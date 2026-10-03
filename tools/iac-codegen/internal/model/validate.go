package model

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/issue"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// ResolveResource maps an explicit Terraform resource name or exact component
// name to one unique OpenAPI component. It never selects among multiple matches.
func ResolveResource(doc *v3.Document, selection string) (string, issue.Report) {
	if doc.Components == nil || doc.Components.Schemas == nil {
		return "", issue.Report{{Code: "RESOURCE_SCHEMA_NOT_FOUND", Location: "components.schemas", Message: "The OpenAPI document has no component schemas.", Remediation: "Add the selected resource schema to the source API contract."}}
	}
	if doc.Components.Schemas.GetOrZero(selection) != nil {
		return selection, nil
	}
	var matches []string
	for name := range doc.Components.Schemas.FromOldest() {
		if TerraformName(name) == selection {
			matches = append(matches, name)
		}
	}
	slices.Sort(matches)
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", issue.Report{{Code: "RESOURCE_SCHEMA_NOT_FOUND", Location: "components.schemas", Message: fmt.Sprintf("No component has the exact name or Terraform name %q.", selection), Remediation: "Select an existing complete resource component."}}
	default:
		return "", issue.Report{{Code: "RESOURCE_SCHEMA_AMBIGUOUS", Location: "components.schemas", Message: fmt.Sprintf("More than one component normalizes to %q: %s.", selection, strings.Join(matches, ", ")), Remediation: "Use the exact OpenAPI component name or rename the colliding components."}}
	}
}

// Validate collects every detectable eligibility issue before generation.
func Validate(doc *v3.Document, name string, ids OperationIDs) issue.Report {
	var report issue.Report
	ops, opIssues := validateOperations(doc, name, ids)
	report = append(report, opIssues...)

	t, surveyIssues := Survey(doc, name)
	for _, err := range surveyIssues {
		report = append(report, schemaIssue(err))
	}
	if t != nil {
		report = append(report, nameCollisions("components.schemas."+name, t)...)
		report = append(report, componentNameCollisions(t)...)
	}
	if len(ops) == len(verbs) {
		report = append(report, requestSchemaSeparationIssues(name, ops)...)
		report = append(report, requiredDeclarationIssues(name, ops)...)
		report = append(report, validateFieldContracts(name, ops)...)
		report = append(report, responseWrapperIssues(name, ops)...)
	}
	if len(ops) == len(verbs) {
		r, err := BuildWithOperationIDs(doc, name, ids)
		if err != nil {
			report = append(report, buildIssue(err))
		} else {
			report = append(report, validateBuiltResource(r)...)
		}
	}
	return report.Normalize()
}

func requestSchemaSeparationIssues(name string, ops map[verb]foundOp) issue.Report {
	create, createErr := rootSchemaIdentity(bodyProxy(ops[opCreate].op))
	update, updateErr := rootSchemaIdentity(bodyProxy(ops[opUpdate].op))
	resource, resourceErr := rootSchemaIdentity(responseResourceProxy(ops[opGet].op, name))
	if createErr != nil || updateErr != nil || resourceErr != nil {
		return nil
	}
	createLocation := "paths.create." + ops[opCreate].op.OperationId + ".requestBody"
	updateLocation := "paths.update." + ops[opUpdate].op.OperationId + ".requestBody"
	var report issue.Report
	if create == resource {
		report = append(report, reusedRequestSchemaIssue(createLocation, "Create", "resource response"))
	}
	if update == resource {
		report = append(report, reusedRequestSchemaIssue(updateLocation, "Update", "resource response"))
	}
	if create == update {
		report = append(report, reusedRequestSchemaIssue(updateLocation, "Update", "Create"))
	}
	return report
}

func rootSchemaIdentity(proxy *base.SchemaProxy) (string, error) {
	if proxy == nil {
		return "", fmt.Errorf("missing schema")
	}
	schema, err := schemaOf(proxy)
	if err != nil {
		return "", err
	}
	inner, err := singleAllOf(schema, "schema")
	if err == nil {
		return rootSchemaIdentity(inner)
	}
	if reference := proxy.GetReference(); reference != "" {
		return "reference:" + reference, nil
	}
	return fmt.Sprintf("inline:%p", schema.GoLow()), nil
}

func reusedRequestSchemaIssue(location, role, reused string) issue.Issue {
	return issue.Issue{
		Code:        "REQUEST_SCHEMA_REUSED",
		Location:    location,
		Message:     fmt.Sprintf("The %s request reuses the %s schema.", role, reused),
		Remediation: "Define separate Create, Update, and resource response schemas. Inline request schemas with distinct identities are supported.",
	}
}

func requiredDeclarationIssues(name string, ops map[verb]foundOp) issue.Report {
	seen := map[*base.Schema]bool{}
	roots := []struct {
		location string
		proxy    *base.SchemaProxy
	}{
		{"paths.create." + ops[opCreate].op.OperationId + ".requestBody", bodyProxy(ops[opCreate].op)},
		{"paths.update." + ops[opUpdate].op.OperationId + ".requestBody", bodyProxy(ops[opUpdate].op)},
		{"components.schemas." + name, responseResourceProxy(ops[opGet].op, name)},
	}
	var report issue.Report
	for _, root := range roots {
		report = append(report, requiredDeclarationIssuesAt(root.location, root.proxy, seen)...)
	}
	return report
}

func requiredDeclarationIssuesAt(location string, proxy *base.SchemaProxy, seen map[*base.Schema]bool) issue.Report {
	if proxy == nil {
		return nil
	}
	schema, err := schemaOf(proxy)
	if err != nil || seen[schema] {
		return nil
	}
	seen[schema] = true
	location = referencedSchemaLocation(proxy, location)
	var report issue.Report
	if fixedObject(schema) && (schema.GoLow() == nil || schema.GoLow().Required.IsEmpty()) {
		report = append(report, issue.Issue{
			Code:        "REQUIRED_DECLARATION_MISSING",
			Location:    location,
			Message:     "The object does not declare which fields are required.",
			Remediation: "Add an explicit required list. Use required: [] when every field is optional.",
		})
	}
	for index, inner := range schema.AllOf {
		report = append(report, requiredDeclarationIssuesAt(fmt.Sprintf("%s.allOf[%d]", location, index), inner, seen)...)
	}
	for _, field := range propertyNames(schema) {
		report = append(report, requiredDeclarationIssuesAt(location+"."+field, propertyOf(schema, field), seen)...)
	}
	if schema.Items != nil && schema.Items.IsA() {
		report = append(report, requiredDeclarationIssuesAt(location+"[]", schema.Items.A, seen)...)
	}
	if schema.AdditionalProperties != nil && schema.AdditionalProperties.IsA() {
		report = append(report, requiredDeclarationIssuesAt(location+"{}", schema.AdditionalProperties.A, seen)...)
	}
	return report
}

func referencedSchemaLocation(proxy *base.SchemaProxy, fallback string) string {
	if name, err := componentName(proxy.GetReference()); err == nil {
		return "components.schemas." + name
	}
	return fallback
}

func fixedObject(schema *base.Schema) bool {
	if !slices.Equal(schema.Type, []string{"object"}) {
		return false
	}
	return schema.AdditionalProperties == nil || !schema.AdditionalProperties.IsA() && !schema.AdditionalProperties.B
}

func validateOperations(doc *v3.Document, name string, ids OperationIDs) (map[verb]foundOp, issue.Report) {
	found := map[verb]foundOp{}
	if doc.Paths == nil {
		return found, issue.Report{{Code: "RESOURCE_LIFECYCLE_INCOMPLETE", Location: "paths", Message: "The OpenAPI document has no paths.", Remediation: "Add one complete Create, Get, Update or Replace, and Delete lifecycle."}}
	}
	explicit := map[verb]string{opCreate: ids.Create, opGet: ids.Get, opUpdate: ids.Update, opDelete: ids.Delete}
	var report issue.Report
	for _, role := range verbs {
		var candidates []foundOp
		for path, item := range doc.Paths.PathItems.FromOldest() {
			for method, op := range item.GetOperations().FromOldest() {
				match := explicit[role] != "" && op.OperationId == explicit[role]
				if explicit[role] == "" {
					match = strings.HasSuffix(op.OperationId, "_"+string(role)+name)
					if role == opUpdate {
						match = match || strings.HasSuffix(op.OperationId, "_"+string(opReplace)+name)
					}
				}
				if match {
					candidates = append(candidates, foundOp{path: path, method: strings.ToUpper(method), item: item, op: op})
				}
			}
		}
		location := "paths." + strings.ToLower(string(role))
		switch len(candidates) {
		case 0:
			wanted := "a unique operationId suffix"
			if explicit[role] != "" {
				wanted = fmt.Sprintf("operationId %q", explicit[role])
			}
			report = append(report, issue.Issue{Code: "OPERATION_NOT_FOUND", Location: location, Message: fmt.Sprintf("No %s operation matches %s.", role, wanted), Remediation: "Fix the source API contract or select the exact operation ID."})
		case 1:
			candidate := candidates[0]
			candidateIssues := operationCandidateIssues(location, role, candidate)
			report = append(report, candidateIssues...)
			if len(candidateIssues) == 0 {
				found[role] = candidate
			}
		default:
			var names []string
			for _, candidate := range candidates {
				names = append(names, candidate.op.OperationId)
			}
			slices.Sort(names)
			report = append(report, issue.Issue{Code: "OPERATION_AMBIGUOUS", Location: location, Message: fmt.Sprintf("More than one %s operation matches: %s.", role, strings.Join(names, ", ")), Remediation: "Select one exact operation ID or make discovery unique in the source API contract."})
		}
	}
	if len(found) != len(verbs) {
		report = append(report, issue.Issue{Code: "RESOURCE_LIFECYCLE_INCOMPLETE", Location: "paths", Message: "The resource does not have one eligible operation for every lifecycle step.", Remediation: "Provide one Create, Get, Update or Replace, and Delete operation."})
	}
	return found, report
}

func operationCandidateIssues(location string, role verb, candidate foundOp) issue.Report {
	if !slices.Contains(verbMethods[role], candidate.method) {
		return issue.Report{{
			Code:        "OPERATION_METHOD_INCOMPATIBLE",
			Location:    location + "." + candidate.op.OperationId,
			Message:     fmt.Sprintf("The method is %s. The %s operation needs %s.", candidate.method, role, strings.Join(verbMethods[role], " or ")),
			Remediation: "Use the required HTTP method in the source API contract.",
		}}
	}
	return requiredParameterIssues(location, role, candidate)
}

func requiredParameterIssues(location string, role verb, op foundOp) issue.Report {
	var report issue.Report
	for _, p := range unsupportedRequiredParameters(role, op) {
		report = append(report, issue.Issue{
			Code:        "REQUIRED_PARAMETER_UNSUPPORTED",
			Location:    fmt.Sprintf("%s.%s.parameters.%s.%s", location, op.op.OperationId, p.In, p.Name),
			Message:     fmt.Sprintf("The operation requires the unsupported %s parameter %q.", p.In, p.Name),
			Remediation: "Remove the required parameter or wait until the generator can model and send it.",
		})
	}
	return report
}

func validateFieldContracts(name string, ops map[verb]foundOp) issue.Report {
	create, createErr := schemaOf(bodyProxy(ops[opCreate].op))
	update, updateErr := schemaOf(bodyProxy(ops[opUpdate].op))
	getProxy := ops[opGet].op
	_ = getProxy
	if createErr != nil || updateErr != nil {
		return nil
	}
	// The component is validated by Survey. Build performs the detailed
	// request/response match after this pass.
	resourceProxy := responseResourceProxy(ops[opGet].op, name)
	get, getErr := schemaOf(resourceProxy)
	if getErr != nil {
		return nil
	}
	names := append(propertyNames(get), propertyNames(create)...)
	names = append(names, propertyNames(update)...)
	slices.Sort(names)
	names = slices.Compact(names)
	var report issue.Report
	bodyOnlyID := ""
	if len(pathParams(ops[opUpdate])) == 0 {
		if params := pathParams(ops[opGet]); len(params) == 1 {
			bodyOnlyID = params[0].Name
		}
	}
	for _, field := range names {
		if field == bodyOnlyID {
			continue // Build reports the unsupported Update identity contract.
		}
		report = append(report, validateFieldContract(name, field, create, update, get)...)
	}
	report = append(report, resourceIDIssues(name, ops, get)...)
	report = append(report, rootGroupContractIssues(name, create, update, get)...)
	return report
}

func resourceIDIssues(name string, ops map[verb]foundOp, get *base.Schema) issue.Report {
	params := pathParams(ops[opGet])
	if len(params) != 1 {
		return nil // Build reports an unsupported resource identity shape.
	}
	id := params[0].Name
	property := propertyOf(get, id)
	if property == nil {
		return nil
	}
	location := "components.schemas." + name + "." + id
	var report issue.Report
	if !slices.Contains(get.Required, id) {
		report = append(report, issue.Issue{
			Code:        "RESOURCE_ID_OPTIONAL",
			Location:    location,
			Message:     fmt.Sprintf("The Get response does not require the resource id field %q.", id),
			Remediation: "List the resource id field in the Get response schema's required fields.",
		})
	}
	resourceType, err := typeOf(property, location, walk{})
	if err != nil {
		return report
	}
	resourceKind, supported := supportedIDType(resourceType)
	if !supported {
		report = append(report, unsupportedIDTypeIssue(location, resourceType))
		return report
	}
	for _, role := range []verb{opGet, opUpdate, opDelete} {
		op := ops[role]
		params := pathParams(op)
		if len(params) != 1 {
			continue // Build reports the unsupported parameter count.
		}
		parameter := params[0]
		parameterLocation := fmt.Sprintf("paths.%s.%s.parameters.%s", strings.ToLower(string(role)), op.op.OperationId, parameter.Name)
		parameterType, err := typeOf(parameter.Schema, parameterLocation, walk{})
		if err != nil {
			report = append(report, issue.Issue{Code: "RESOURCE_ID_TYPE_UNSUPPORTED", Location: parameterLocation, Message: err.Error(), Remediation: "Use a string, int32, or int64 resource ID."})
			continue
		}
		parameterKind, supported := supportedIDType(parameterType)
		switch {
		case !supported:
			report = append(report, unsupportedIDTypeIssue(parameterLocation, parameterType))
		case parameterKind != resourceKind:
			report = append(report, issue.Issue{Code: "RESOURCE_ID_TYPE_INCONSISTENT", Location: parameterLocation, Message: fmt.Sprintf("The path ID type is %s, but the response ID field type is %s.", parameterKind, resourceKind), Remediation: "Use the same ID type in Get, Update, Delete, and the resource response."})
		}
	}
	return report
}

func supportedIDType(t *Type) (string, bool) {
	switch {
	case t.Kind == String && t.Format != "date-time":
		return "string", true
	case t.Kind == Integer && !t.WireString && (t.Format == "int32" || t.Format == "int64"):
		return t.Format, true
	default:
		return fmt.Sprintf("%s/%s", t.Kind, t.Format), false
	}
}

func unsupportedIDTypeIssue(location string, t *Type) issue.Issue {
	kind, _ := supportedIDType(t)
	return issue.Issue{Code: "RESOURCE_ID_TYPE_UNSUPPORTED", Location: location, Message: fmt.Sprintf("The resource ID type %s is not supported.", kind), Remediation: "Use a string, int32, or int64 resource ID."}
}

func responseWrapperIssues(name string, ops map[verb]foundOp) issue.Report {
	var report issue.Report
	for _, role := range []verb{opCreate, opGet, opUpdate} {
		op := ops[role]
		if op.op == nil || op.op.Responses == nil || op.op.Responses.Codes == nil {
			continue
		}
		response := op.op.Responses.Codes.GetOrZero("200")
		if response == nil || response.Content == nil {
			continue
		}
		media := response.Content.GetOrZero(jsonMedia)
		if media == nil || media.Schema == nil || !media.Schema.IsReference() {
			continue
		}
		component, err := componentName(media.Schema.GetReference())
		if err != nil || component == name {
			continue
		}
		report = append(report, issue.Issue{
			Code:        "RESPONSE_WRAPPER_UNSUPPORTED",
			Location:    fmt.Sprintf("paths.%s.%s.responses.200", strings.ToLower(string(role)), op.op.OperationId),
			Message:     fmt.Sprintf("The success response is %s instead of the direct %s resource.", component, name),
			Remediation: "Return the resource directly in REST. Set google.api.http response_body to the resource field in the protobuf response message.",
		})
	}
	return report
}

func validateFieldContract(name, field string, create, update, get *base.Schema) issue.Report {
	cp := requestContractProperty(create, field)
	up := requestContractProperty(update, field)
	gp := propertyOf(get, field)
	location := "components.schemas." + name + "." + field
	if _, err := Classify(cp != nil, up != nil, gp != nil); err != nil {
		return issue.Report{{Code: "FIELD_LIFECYCLE_UNSUPPORTED", Location: location, Message: fmt.Sprintf("The field locations are Create=%t, Update=%t, Get=%t.", cp != nil, up != nil, gp != nil), Remediation: "Use a managed, immutable, or computed field lifecycle."}}
	}
	var report issue.Report
	if cp != nil && up != nil && !slices.Contains(create.Required, field) && slices.Contains(update.Required, field) {
		report = append(report, issue.Issue{
			Code:        "FIELD_REQUIREDNESS_UNSUPPORTED",
			Location:    location,
			Message:     "The field is optional in Create but required in Update.",
			Remediation: "Make the field optional in Update, or require it in Create so Terraform always has a value to send.",
		})
	}
	report = append(report, fieldTypeIssues(location, gp, cp, up)...)
	report = append(report, fieldDefaultContractIssues(location, field, create, update, get, cp, up, gp)...)
	report = append(report, fieldPresenceIssue(location+".create", field, create, cp)...)
	report = append(report, fieldPresenceIssue(location+".update", field, update, up)...)
	report = append(report, nestedPresenceIssues(location+".create", cp, map[*base.Schema]bool{})...)
	report = append(report, nestedPresenceIssues(location+".update", up, map[*base.Schema]bool{})...)
	report = append(report, nestedReadOnlyIssues(location+".create", cp, map[*base.Schema]bool{})...)
	report = append(report, nestedReadOnlyIssues(location+".update", up, map[*base.Schema]bool{})...)
	report = append(report, unsupportedSchemaIssues(location+".create", cp, map[*base.Schema]bool{})...)
	report = append(report, unsupportedSchemaIssues(location+".update", up, map[*base.Schema]bool{})...)
	report = append(report, unsupportedSchemaIssues(location+".get", gp, map[*base.Schema]bool{})...)
	return report
}

func fieldDefaultContractIssues(location, field string, create, update, get *base.Schema, cp, up, gp *base.SchemaProxy) issue.Report {
	var report issue.Report
	createDefault := schemaDefault(cp)
	updateDefault := schemaDefault(up)
	report = append(report, defaultValueIssues(location+".create", cp)...)
	report = append(report, defaultValueIssues(location+".update", up)...)
	createOptional := cp != nil && !slices.Contains(create.Required, field)
	getRequired := gp != nil && slices.Contains(get.Required, field)
	if createOptional && getRequired && createDefault == nil {
		report = append(report, issue.Issue{Code: "FIELD_SERVER_DEFAULT_UNDECLARED", Location: location, Message: "The field is optional in Create but required in Get, so the server supplies a value without a declared default.", Remediation: "Declare the exact OpenAPI default, or separate the client-owned request field from the server-owned response field."})
	}
	if createDefault != nil {
		switch {
		case !createOptional || !getRequired:
			report = append(report, issue.Issue{Code: "FIELD_DEFAULT_CONTRACT_INCONSISTENT", Location: location, Message: "A declared server default needs an optional Create field and a required Get field.", Remediation: "Make the field optional in Create, required in Get, and let the server return the declared default."})
		case up != nil && (slices.Contains(update.Required, field) || updateDefault == nil || *updateDefault != *createDefault):
			report = append(report, issue.Issue{Code: "FIELD_DEFAULT_CONTRACT_INCONSISTENT", Location: location, Message: "The mutable field does not declare the same optional default in Create and Update.", Remediation: "Declare the same typed OpenAPI default on the optional Create and Update fields."})
		}
	} else if updateDefault != nil {
		report = append(report, issue.Issue{Code: "FIELD_DEFAULT_CONTRACT_INCONSISTENT", Location: location, Message: "The Update field declares a default that Create does not declare.", Remediation: "Declare the same typed OpenAPI default on the optional Create and Update fields."})
	}
	for _, candidate := range []struct {
		name  string
		proxy *base.SchemaProxy
	}{{"create", cp}, {"update", up}} {
		report = append(report, nestedDefaultIssues(location+"."+candidate.name, candidate.proxy, true, map[*base.Schema]bool{})...)
	}
	return report
}

func defaultValueIssues(location string, proxy *base.SchemaProxy) issue.Report {
	if proxy == nil {
		return nil
	}
	schema, err := schemaOf(proxy)
	if err != nil || schema.Default == nil || len(schema.Type) != 1 {
		return nil
	}
	if err := validateScalarDefault(schema); err != nil {
		return issue.Report{{
			Code:        "FIELD_DEFAULT_INVALID",
			Location:    location,
			Message:     "The declared default is invalid: " + err.Error() + ".",
			Remediation: "Declare a scalar default that has the field type and satisfies its limits.",
		}}
	}
	return nil
}

func validateScalarDefault(schema *base.Schema) error {
	switch schema.Type[0] {
	case "string":
		return validateStringDefault(schema)
	case "boolean":
		return validateBooleanDefault(schema)
	case "integer":
		return validateIntegerDefault(schema)
	case "number":
		return validateNumberDefault(schema)
	default:
		return fmt.Errorf("defaults for %s fields are not supported", schema.Type[0])
	}
}

func validateStringDefault(schema *base.Schema) error {
	node := schema.Default
	if node.Tag != "!!str" {
		return fmt.Errorf("%q is not a string", node.Value)
	}
	length := int64(utf8.RuneCountInString(node.Value))
	if schema.MinLength != nil && length < *schema.MinLength || schema.MaxLength != nil && length > *schema.MaxLength {
		return fmt.Errorf("the string %q does not satisfy the length limits", node.Value)
	}
	for _, candidate := range schema.Enum {
		if candidate.Tag == node.Tag && candidate.Value == node.Value {
			return nil
		}
	}
	if len(schema.Enum) != 0 {
		return fmt.Errorf("%q is not an enum value", node.Value)
	}
	return nil
}

func validateBooleanDefault(schema *base.Schema) error {
	node := schema.Default
	if node.Tag != "!!bool" {
		return fmt.Errorf("%q is not a boolean", node.Value)
	}
	if _, err := strconv.ParseBool(node.Value); err != nil {
		return fmt.Errorf("%q is not a boolean", node.Value)
	}
	return nil
}

func validateIntegerDefault(schema *base.Schema) error {
	node := schema.Default
	if node.Tag != "!!int" {
		return fmt.Errorf("%q is not an integer", node.Value)
	}
	bits := 64
	if schema.Format == "int32" {
		bits = 32
	}
	value, err := strconv.ParseInt(node.Value, 10, bits)
	if err != nil || schema.Format == "uint64" && value < 0 {
		return fmt.Errorf("%q is not a supported %s integer", node.Value, schema.Format)
	}
	return defaultNumberLimits(schema, float64(value))
}

func validateNumberDefault(schema *base.Schema) error {
	node := schema.Default
	if node.Tag != "!!int" && node.Tag != "!!float" {
		return fmt.Errorf("%q is not a number", node.Value)
	}
	value, err := strconv.ParseFloat(node.Value, 64)
	if err != nil {
		return fmt.Errorf("%q is not a number", node.Value)
	}
	return defaultNumberLimits(schema, value)
}

func defaultNumberLimits(schema *base.Schema, value float64) error {
	if schema.Minimum != nil && value < *schema.Minimum || schema.Maximum != nil && value > *schema.Maximum {
		return fmt.Errorf("the numeric default %s does not satisfy the range limits", schema.Default.Value)
	}
	return nil
}

func schemaDefault(proxy *base.SchemaProxy) *string {
	if proxy == nil {
		return nil
	}
	schema, err := schemaOf(proxy)
	if err != nil || schema.Default == nil {
		return nil
	}
	value := schema.Default.Value
	return &value
}

func nestedDefaultIssues(location string, proxy *base.SchemaProxy, root bool, seen map[*base.Schema]bool) issue.Report {
	if proxy == nil {
		return nil
	}
	schema, err := schemaOf(proxy)
	if err != nil || seen[schema] {
		return nil
	}
	seen[schema] = true
	var report issue.Report
	if !root && schema.Default != nil {
		report = append(report, issue.Issue{Code: "NESTED_FIELD_DEFAULT_UNSUPPORTED", Location: location, Message: "A nested request field declares a server default.", Remediation: "Move the defaulted value to a top-level field or wait for nested server-default support."})
	}
	for _, name := range propertyNames(schema) {
		report = append(report, nestedDefaultIssues(location+"."+name, propertyOf(schema, name), false, seen)...)
	}
	if schema.Items != nil && schema.Items.IsA() {
		report = append(report, nestedDefaultIssues(location+"[]", schema.Items.A, false, seen)...)
	}
	if schema.AdditionalProperties != nil && schema.AdditionalProperties.IsA() {
		report = append(report, nestedDefaultIssues(location+"{}", schema.AdditionalProperties.A, false, seen)...)
	}
	return report
}

func unsupportedSchemaIssues(location string, proxy *base.SchemaProxy, seen map[*base.Schema]bool) issue.Report {
	if proxy == nil {
		return nil
	}
	schema, err := schemaOf(proxy)
	if err != nil || seen[schema] {
		return nil
	}
	seen[schema] = true
	var report issue.Report
	if schema.WriteOnly != nil && *schema.WriteOnly {
		report = append(report, issue.Issue{Code: "FIELD_WRITE_ONLY_UNSUPPORTED", Location: location, Message: "The field is writeOnly, but this generator cannot preserve or rotate a value that the API does not return.", Remediation: "Use a handwritten resource until generic write-only state and version handling is supported."})
	}
	if schema.Pattern != "" {
		report = append(report, issue.Issue{Code: "STRING_PATTERN_UNSUPPORTED", Location: location, Message: fmt.Sprintf("The field declares the unsupported pattern %q.", schema.Pattern), Remediation: "Remove the pattern or wait for generated regular-expression validation support."})
	}
	for _, name := range propertyNames(schema) {
		report = append(report, unsupportedSchemaIssues(location+"."+name, propertyOf(schema, name), seen)...)
	}
	if schema.Items != nil && schema.Items.IsA() {
		report = append(report, unsupportedSchemaIssues(location+"[]", schema.Items.A, seen)...)
	}
	if schema.AdditionalProperties != nil && schema.AdditionalProperties.IsA() {
		report = append(report, unsupportedSchemaIssues(location+"{}", schema.AdditionalProperties.A, seen)...)
	}
	return report
}

func rootGroupContractIssues(name string, create, update, get *base.Schema) issue.Report {
	createGroups, createErr := rootContractGroups(create)
	updateGroups, updateErr := rootContractGroups(update)
	getGroups, getErr := rootContractGroups(get)
	if createErr != nil || updateErr != nil || getErr != nil {
		return nil // The schema survey and build report malformed oneOf shapes.
	}
	if reflect.DeepEqual(createGroups, updateGroups) && reflect.DeepEqual(createGroups, getGroups) {
		return nil
	}
	return issue.Report{{
		Code:        "ROOT_ONEOF_LIFECYCLE_INCONSISTENT",
		Location:    "components.schemas." + name,
		Message:     fmt.Sprintf("The root oneOf groups differ across Create=%v, Update=%v, and Get=%v.", createGroups, updateGroups, getGroups),
		Remediation: "Use the same root oneOf groups in Create, Update, and Get. Keep server-only unions outside the configurable resource shape.",
	}}
}

func rootContractGroups(schema *base.Schema) ([]OneOfGroup, error) {
	if schema == nil {
		return nil, fmt.Errorf("missing schema")
	}
	schemas := []*base.Schema{schema}
	for _, proxy := range schema.AllOf {
		entry, err := schemaOf(proxy)
		if err != nil {
			return nil, err
		}
		schemas = append(schemas, entry)
	}
	var groups []OneOfGroup
	for _, entry := range schemas {
		if len(entry.OneOf) == 0 {
			continue
		}
		group, err := oneOfArms(entry)
		if err != nil {
			return nil, err
		}
		slices.Sort(group.Arms)
		groups = append(groups, group)
	}
	slices.SortFunc(groups, func(a, b OneOfGroup) int {
		return strings.Compare(strings.Join(a.Arms, "\x00"), strings.Join(b.Arms, "\x00"))
	})
	return groups, nil
}

func nestedReadOnlyIssues(location string, proxy *base.SchemaProxy, seen map[*base.Schema]bool) issue.Report {
	if proxy == nil {
		return nil
	}
	schema, err := schemaOf(proxy)
	if err != nil || seen[schema] {
		return nil
	}
	seen[schema] = true
	var report issue.Report
	for _, name := range propertyNames(schema) {
		child := propertyOf(schema, name)
		childSchema, childErr := schemaOf(child)
		if childErr != nil {
			continue
		}
		childLocation := location + "." + name
		if childSchema.ReadOnly != nil && *childSchema.ReadOnly {
			report = append(report, issue.Issue{Code: "FIELD_LIFECYCLE_UNSUPPORTED", Location: childLocation, Message: "A nested request field is readOnly and cannot be removed by the current resource renderer.", Remediation: "Use separate request and response schemas so server-only fields are absent from request objects."})
			continue
		}
		report = append(report, nestedReadOnlyIssues(childLocation, child, seen)...)
	}
	if schema.Items != nil && schema.Items.IsA() {
		report = append(report, nestedReadOnlyIssues(location+"[]", schema.Items.A, seen)...)
	}
	if schema.AdditionalProperties != nil && schema.AdditionalProperties.IsA() {
		report = append(report, nestedReadOnlyIssues(location+"{}", schema.AdditionalProperties.A, seen)...)
	}
	return report
}

func fieldTypeIssues(location string, candidates ...*base.SchemaProxy) issue.Report {
	var canonical *Type
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		current, err := typeOf(candidate, location, walk{})
		if err != nil {
			continue
		}
		if canonical == nil {
			canonical = current
			continue
		}
		if !reflect.DeepEqual(canonical, current) {
			return issue.Report{{Code: "FIELD_TYPE_INCONSISTENT", Location: location, Message: "The Create, Update, and Get schemas do not use the same field type.", Remediation: "Use one compatible schema for the field in every lifecycle operation."}}
		}
	}
	return nil
}

func fieldPresenceIssue(location, field string, parent *base.Schema, proxy *base.SchemaProxy) issue.Report {
	if proxy == nil {
		return nil
	}
	schema, err := schemaOf(proxy)
	if err != nil {
		return nil
	}
	value := valueSchema(schema)
	required := slices.Contains(parent.Required, field)
	if required {
		if report := requiredEmptyIssue(location, value); report != nil {
			return report
		}
	}
	// A list or map has no presence: empty and missing are the same. A
	// message always has presence, so it needs no marker.
	if !scalarSchema(value) || required && !scalarZeroValueValid(schema) {
		return nil
	}
	presence := schema.Extensions.GetOrZero(extPresence)
	if presence != nil && presence.Value == "true" {
		return nil
	}
	if required {
		return issue.Report{{
			Code:        "REQUIRED_SCALAR_PRESENCE_UNKNOWN",
			Location:    location,
			Message:     "The required bool or number accepts its zero value but does not preserve field presence.",
			Remediation: "Use a presence-tracking protobuf scalar and expose x-coralogix-presence: true in OpenAPI.",
		}}
	}
	return issue.Report{{Code: "FIELD_PRESENCE_UNKNOWN", Location: location, Message: "The optional request field does not state whether omission differs from an explicit zero or empty value.", Remediation: "Add x-coralogix-presence: true to the source API contract."}}
}

// requiredEmptyIssue reports a required string, list, or map that accepts an
// empty value. Required means not empty: an empty value is an optional field.
func requiredEmptyIssue(location string, schema *base.Schema) issue.Report {
	var message, remediation string
	switch {
	case plainStringSchema(schema) && (schema.MinLength == nil || *schema.MinLength < 1):
		message, remediation = `The required string accepts "".`, "Add openapiv3_field min_length: 1, or make the field optional."
	case slices.Equal(schema.Type, []string{"array"}) && (schema.MinItems == nil || *schema.MinItems < 1):
		message, remediation = "The required list accepts [].", "Add openapiv3_field min_items: 1, or make the field optional."
	case isMapSchema(schema) && (schema.MinProperties == nil || *schema.MinProperties < 1):
		message, remediation = "The required map accepts {}.", "Add openapiv3_field min_properties: 1, or make the field optional."
	default:
		return nil
	}
	return issue.Report{{Code: "REQUIRED_VALUE_CAN_BE_EMPTY", Location: location, Message: message, Remediation: remediation}}
}

// valueSchema returns the schema that an allOf wrapper of one $ref names, or
// schema itself.
func valueSchema(schema *base.Schema) *base.Schema {
	if inner, err := singleAllOf(schema, "schema"); err == nil {
		if resolved, resolveErr := schemaOf(inner); resolveErr == nil {
			return resolved
		}
	}
	return schema
}

// scalarSchema reports whether schema is a string, bool, number, or enum. A
// list, a map, and an object are not scalars.
func scalarSchema(schema *base.Schema) bool {
	if len(schema.Type) != 1 {
		return len(schema.Enum) != 0
	}
	switch schema.Type[0] {
	case "string", "boolean", "integer", "number":
		return true
	}
	return false
}

// plainStringSchema reports whether schema is a protobuf string: a string with
// no enum and no format. A 64-bit number is a string with a format.
func plainStringSchema(schema *base.Schema) bool {
	return slices.Equal(schema.Type, []string{"string"}) && len(schema.Enum) == 0 && schema.Format == ""
}

func scalarZeroValueValid(schema *base.Schema) bool {
	if inner, err := singleAllOf(schema, "schema"); err == nil {
		resolved, resolveErr := schemaOf(inner)
		return resolveErr == nil && scalarZeroValueValid(resolved)
	}
	if len(schema.Enum) != 0 || len(schema.Type) != 1 {
		return false
	}
	switch schema.Type[0] {
	case "boolean":
		return true
	case "integer", "number":
		return numericZeroValueValid(schema)
	case "string":
		return stringZeroValueValid(schema)
	default:
		return false
	}
}

func numericZeroValueValid(schema *base.Schema) bool {
	return (schema.Minimum == nil || *schema.Minimum <= 0) && (schema.Maximum == nil || *schema.Maximum >= 0)
}

func stringZeroValueValid(schema *base.Schema) bool {
	value := ""
	switch schema.Format {
	case "":
	case "int64", "uint64":
		value = "0"
	default:
		return false
	}
	length := int64(len([]rune(value)))
	if schema.MinLength != nil && length < *schema.MinLength || schema.MaxLength != nil && length > *schema.MaxLength {
		return false
	}
	if schema.Pattern == "" {
		return true
	}
	re, err := regexp.Compile(schema.Pattern)
	return err == nil && re.MatchString(value)
}

func nestedPresenceIssues(location string, proxy *base.SchemaProxy, seen map[*base.Schema]bool) issue.Report {
	if proxy == nil {
		return nil
	}
	schema, err := schemaOf(proxy)
	if err != nil || seen[schema] {
		return nil
	}
	seen[schema] = true
	var report issue.Report
	grouped := groupedFields(schema)
	for _, name := range propertyNames(schema) {
		child := propertyOf(schema, name)
		childSchema, childErr := schemaOf(child)
		if childErr == nil && childSchema.ReadOnly != nil && *childSchema.ReadOnly {
			continue
		}
		if !grouped[name] {
			report = append(report, fieldPresenceIssue(location+"."+name, name, schema, child)...)
		}
		report = append(report, nestedPresenceIssues(location+"."+name, child, seen)...)
	}
	if schema.Items != nil && schema.Items.IsA() {
		report = append(report, nestedPresenceIssues(location+"[]", schema.Items.A, seen)...)
	}
	if schema.AdditionalProperties != nil && schema.AdditionalProperties.IsA() {
		report = append(report, nestedPresenceIssues(location+"{}", schema.AdditionalProperties.A, seen)...)
	}
	return report
}

func groupedFields(schema *base.Schema) map[string]bool {
	grouped := map[string]bool{}
	schemas := []*base.Schema{schema}
	for _, proxy := range schema.AllOf {
		entry, err := schemaOf(proxy)
		if err == nil {
			schemas = append(schemas, entry)
		}
	}
	for _, entry := range schemas {
		if len(entry.OneOf) == 0 {
			continue
		}
		group, err := oneOfArms(entry)
		if err != nil {
			continue
		}
		for _, name := range group.Arms {
			grouped[name] = true
		}
	}
	return grouped
}

func requestContractProperty(parent *base.Schema, name string) *base.SchemaProxy {
	proxy := propertyOf(parent, name)
	if proxy == nil {
		return nil
	}
	schema, err := schemaOf(proxy)
	if err == nil && schema.ReadOnly != nil && *schema.ReadOnly {
		return nil
	}
	return proxy
}

func responseResourceProxy(op *v3.Operation, name string) *base.SchemaProxy {
	if op == nil || op.Responses == nil || op.Responses.Codes == nil {
		return nil
	}
	resp := op.Responses.Codes.GetOrZero("200")
	if resp == nil {
		return nil
	}
	if resp.Content == nil {
		return nil
	}
	media := resp.Content.GetOrZero(jsonMedia)
	if media == nil || media.Schema == nil {
		return nil
	}
	if media.Schema.GetReference() == componentPrefix+name {
		return media.Schema
	}
	s, err := schemaOf(media.Schema)
	if err != nil {
		return nil
	}
	for _, field := range propertyNames(s) {
		proxy := propertyOf(s, field)
		if unwrapRef(proxy) == componentPrefix+name {
			return proxy
		}
	}
	return nil
}

func validateBuiltResource(r *Resource) issue.Report {
	var report issue.Report
	if r.Replace {
		return nil
	}
	if r.UpdateMaskPattern == "" {
		report = append(report, issue.Issue{Code: "UPDATE_MASK_CONTRACT_MISSING", Location: "paths.update." + r.Update.OperationID, Message: "The PATCH update mask has no pattern that defines accepted mask paths.", Remediation: "Add the authoritative update-mask path pattern to the source API contract."})
		return report
	}
	_, leaf, err := UpdateMaskRule(r.UpdateMaskPattern)
	if err != nil {
		return append(report, issue.Issue{Code: "UPDATE_MASK_CONTRACT_INVALID", Location: "paths.update." + r.Update.OperationID, Message: err.Error() + ".", Remediation: "Use a mask pattern that accepts field names, rejects *, and defines whether dotted paths are supported."})
	}
	if leaf {
		return report
	}
	for _, field := range r.Fields {
		if field.Update == nil {
			continue
		}
		paths := oneOfMaskPaths(field.Name, field.Type)
		if len(paths) == 0 {
			continue
		}
		report = append(report, issue.Issue{
			Code:        "UPDATE_MASK_NESTED_ONEOF_UNSUPPORTED",
			Location:    "components.schemas." + r.Name + "." + field.Name,
			Message:     fmt.Sprintf("The update mask accepts only top-level paths, but changing this oneOf needs an arm path such as %q.", paths[0]),
			Remediation: "Allow dotted update-mask paths so the generator can send the selected oneOf arm.",
		})
	}
	return report
}

func oneOfMaskPaths(prefix string, t *Type) []string {
	if t == nil {
		return nil
	}
	var paths []string
	if t.Kind == OneOf {
		for _, field := range t.Fields {
			paths = append(paths, prefix+"."+field.Name)
		}
		return paths
	}
	if t.Kind != Object {
		return nil
	}
	for _, group := range t.Groups {
		for _, arm := range group.Arms {
			paths = append(paths, prefix+"."+arm)
		}
	}
	for _, field := range t.Fields {
		paths = append(paths, oneOfMaskPaths(prefix+"."+field.Name, field.Type)...)
	}
	return paths
}

func nameCollisions(location string, t *Type) issue.Report {
	var report issue.Report
	if t == nil {
		return nil
	}
	if t.Kind == List || t.Kind == Set || t.Kind == Map {
		return nameCollisions(location+"[]", t.Elem)
	}
	if t.Kind != Object && t.Kind != OneOf {
		return nil
	}
	terraformNames := map[string]string{}
	goNames := map[string]string{}
	for _, field := range t.Fields {
		terraformName := TerraformName(field.Name)
		if previous, ok := terraformNames[terraformName]; ok && previous != field.Name {
			report = append(report, issue.Issue{Code: "TERRAFORM_NAME_COLLISION", Location: location, Message: fmt.Sprintf("Fields %q and %q both normalize to %q.", previous, field.Name, terraformName), Remediation: "Rename a source API field so every Terraform name is unique."})
		} else {
			terraformNames[terraformName] = field.Name
		}
		goName := GoName(field.Name)
		if previous, ok := goNames[goName]; ok && previous != field.Name {
			report = append(report, issue.Issue{Code: "GO_NAME_COLLISION", Location: location, Message: fmt.Sprintf("Fields %q and %q both generate the Go name %q.", previous, field.Name, goName), Remediation: "Rename a source API field so every generated Go field name is unique."})
		} else {
			goNames[goName] = field.Name
		}
		report = append(report, nameCollisions(location+"."+field.Name, field.Type)...)
	}
	return report
}

func componentNameCollisions(root *Type) issue.Report {
	generatedNames := map[string]string{}
	visitedSchemas := map[string]bool{}
	visitedTypes := map[*Type]bool{}
	var report issue.Report
	var walk func(*Type)
	walk = func(t *Type) {
		if t == nil || visitedTypes[t] {
			return
		}
		visitedTypes[t] = true
		if t.Schema != "" && !visitedSchemas[t.Schema] {
			visitedSchemas[t.Schema] = true
			generated := GoName(t.Schema)
			if previous, ok := generatedNames[generated]; ok && previous != t.Schema {
				report = append(report, issue.Issue{
					Code:        "GO_COMPONENT_NAME_COLLISION",
					Location:    "components.schemas",
					Message:     fmt.Sprintf("Components %q and %q both generate the Go name %q.", previous, t.Schema, generated),
					Remediation: "Rename a source API component so every generated Go type name is unique.",
				})
			} else {
				generatedNames[generated] = t.Schema
			}
		}
		walk(t.Elem)
		for _, field := range t.Fields {
			walk(field.Type)
		}
	}
	walk(root)
	return report
}

// TerraformName converts an OpenAPI name to the exact Terraform name used by
// validation and rendering. Acronym runs stay together: HTTPServer becomes
// http_server. Terraform allows only a-z, 0-9, and _, and no leading digit, so
// any other character becomes _: foo-bar and foo.bar become foo_bar. The name
// collision check then finds foo-bar beside foo_bar.
func TerraformName(name string) string {
	var out strings.Builder
	runes := []rune(name)
	for i, r := range runes {
		if unicode.IsUpper(r) && i > 0 {
			previousLower := !unicode.IsUpper(runes[i-1])
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if previousLower || nextLower {
				out.WriteByte('_')
			}
		}
		out.WriteRune(terraformNameRune(unicode.ToLower(r)))
	}
	result := out.String()
	if result == "" || result[0] >= '0' && result[0] <= '9' {
		return "_" + result
	}
	return result
}

func terraformNameRune(r rune) rune {
	if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' {
		return r
	}
	return '_'
}

// GoName converts an OpenAPI name to the exact Go name used by SDK and model
// field rendering: foo-bar and foo_bar both become FooBar.
func GoName(name string) string {
	var out strings.Builder
	for part := range strings.FieldsFuncSeq(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		out.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return out.String()
}

func schemaIssue(err error) issue.Issue {
	message := err.Error()
	code := schemaIssueCode(message)
	return issue.Issue{Code: code, Location: firstLocation(message), Message: message, Remediation: "Correct the source OpenAPI contract to use a supported, deterministic schema shape."}
}

func schemaIssueCode(message string) string {
	if code := structuralSchemaIssueCode(message); code != "" {
		return code
	}
	if code := constraintSchemaIssueCode(message); code != "" {
		return code
	}
	return "UNION_SHAPE_UNSUPPORTED"
}

func structuralSchemaIssueCode(message string) string {
	switch {
	case strings.Contains(message, "recursive schema"):
		return "SCHEMA_RECURSIVE"
	case strings.Contains(message, "additionalProperties") || strings.Contains(message, "map without"):
		return "MAP_VALUE_TYPE_UNSUPPORTED"
	case strings.Contains(message, "uniqueItems") || strings.Contains(message, extCollection):
		return "COLLECTION_SEMANTICS_UNKNOWN"
	case strings.Contains(message, "nullable"):
		return "FIELD_NULLABILITY_AMBIGUOUS"
	case strings.Contains(message, "$ref") || strings.Contains(message, "reference"):
		return "REFERENCE_UNRESOLVED"
	default:
		return ""
	}
}

func constraintSchemaIssueCode(message string) string {
	switch {
	case strings.Contains(message, "minProperties") || strings.Contains(message, "maxProperties"):
		return "OBJECT_PROPERTY_COUNT_UNSUPPORTED"
	case strings.Contains(message, "exclusiveMinimum") || strings.Contains(message, "exclusiveMaximum"):
		return "NUMERIC_EXCLUSIVE_BOUND_UNSUPPORTED"
	case strings.Contains(message, "uint64") && strings.Contains(message, "Terraform Int64"):
		return "UINT64_RANGE_UNSUPPORTED"
	case strings.Contains(message, "enum") && (strings.Contains(message, "UNSPECIFIED") || strings.Contains(message, "zero value") || strings.Contains(message, "zero-value prefix")):
		return "ENUM_ZERO_INVALID"
	default:
		return ""
	}
}

func buildIssue(err error) issue.Issue {
	message := err.Error()
	code := "RESOURCE_LIFECYCLE_INCOMPLETE"
	remediation := "Correct the source API contract so the complete resource lifecycle is deterministic."
	switch {
	case strings.Contains(message, "id in the request body is not supported"):
		code = "UPDATE_ID_IN_BODY_UNSUPPORTED"
		remediation = "Put the resource id in the Update path. Add body-id compatibility only during existing-resource migration."
	case strings.Contains(message, "update mask") || strings.Contains(message, updateMaskField):
		code = "CLEAR_BEHAVIOR_UNKNOWN"
	case strings.Contains(message, "type differs"):
		code = "FIELD_TYPE_INCONSISTENT"
	case strings.Contains(message, "unsupported field location"):
		code = "FIELD_LIFECYCLE_UNSUPPORTED"
	case strings.Contains(message, "minProperties") || strings.Contains(message, "maxProperties"):
		code = "OBJECT_PROPERTY_COUNT_UNSUPPORTED"
	case strings.Contains(message, "exclusiveMinimum") || strings.Contains(message, "exclusiveMaximum"):
		code = "NUMERIC_EXCLUSIVE_BOUND_UNSUPPORTED"
	}
	return issue.Issue{Code: code, Location: firstLocation(message), Message: message, Remediation: remediation}
}

func firstLocation(message string) string {
	if before, _, ok := strings.Cut(message, ":"); ok {
		return strings.TrimSpace(before)
	}
	return "openapi"
}
