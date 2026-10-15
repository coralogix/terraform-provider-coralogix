package model

import (
	"errors"
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
	return ValidateWithPolicy(doc, name, ids, Policy{})
}

// ValidateWithPolicy is Validate under the given rule set.
func ValidateWithPolicy(doc *v3.Document, name string, ids OperationIDs, p Policy) issue.Report {
	var report issue.Report
	ops, opIssues := validateOperations(doc, name, ids, p)
	report = append(report, opIssues...)

	t, surveyIssues := SurveyWithPolicy(doc, name, p)
	for _, err := range surveyIssues {
		report = append(report, schemaIssue(err))
	}
	if t != nil {
		report = append(report, nameCollisions(p, "components.schemas."+name, t)...)
		report = append(report, componentNameCollisions(t)...)
		report = append(report, skippedArmIssues(p, t)...)
	}
	if len(ops) == len(verbs) {
		report = append(report, requestSchemaSeparationIssues(p, name, ops)...)
		report = append(report, validateFieldContracts(p, name, ops)...)
		report = append(report, responseWrapperIssues(p, name, ops)...)
	}
	if len(ops) == len(verbs) {
		r, err := BuildWithPolicy(doc, name, ids, p)
		if err != nil {
			report = append(report, buildIssue(err))
		} else {
			report = append(report, validateBuiltResource(r)...)
			report = append(report, componentNameCollisions(resourceType(r))...)
		}
	}
	return report.Normalize()
}

// resourceType is the type of the resource response, with the request
// components that Build merged into it.
func resourceType(r *Resource) *Type {
	t := &Type{Kind: Object, Schema: r.Name}
	for _, f := range r.Fields {
		t.Fields = append(t.Fields, &Field{Name: f.Name, Type: f.Type})
	}
	return t
}

func requestSchemaSeparationIssues(p Policy, name string, ops map[verb]foundOp) issue.Report {
	if p.Existing {
		return nil // a released payload can be shared by the request and the resource
	}
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

// referencedComponent returns the component name of a $ref schema, or "".
func referencedComponent(proxy *base.SchemaProxy) string {
	if proxy == nil {
		return ""
	}
	name, err := componentName(proxy.GetReference())
	if err != nil {
		return ""
	}
	return name
}

func validateOperations(doc *v3.Document, name string, ids OperationIDs, p Policy) (map[verb]foundOp, issue.Report) {
	found := map[verb]foundOp{}
	if doc.Paths == nil {
		return found, issue.Report{{Code: "RESOURCE_LIFECYCLE_INCOMPLETE", Location: "paths", Message: "The OpenAPI document has no paths.", Remediation: "Add one complete Create, Get, Update or Replace, and Delete lifecycle."}}
	}
	report := deleteOverrideConflictIssues(ids, p)
	if len(report) != 0 {
		return found, report
	}
	ids, _ = p.operationIDs(ids) // deleteOverrideConflictIssues reports the conflict.
	explicit := map[verb]string{opCreate: ids.Create, opGet: ids.Get, opUpdate: ids.Update, opDelete: ids.Delete}
	for _, role := range verbs {
		candidates := matchingOperations(doc, name, role, explicit[role])
		report = append(report, validateRole(role, candidates, explicit[role], p, found)...)
	}
	if len(found) != len(verbs) {
		report = append(report, issue.Issue{Code: "RESOURCE_LIFECYCLE_INCOMPLETE", Location: "paths", Message: "The resource does not have one eligible operation for every lifecycle step.", Remediation: "Provide one Create, Get, Update or Replace, and Delete operation."})
	}
	return found, report
}

// validateRole checks the operations that match one lifecycle step. It adds the one eligible
// operation to found.
func validateRole(role verb, candidates []foundOp, explicit string, p Policy, found map[verb]foundOp) issue.Report {
	location := "paths." + strings.ToLower(string(role))
	switch len(candidates) {
	case 0:
		if role == opDelete && p.DeleteOperation != "" {
			return issue.Report{deleteOverrideNotFoundIssue(p)}
		}
		wanted := "a unique operationId suffix"
		if explicit != "" {
			wanted = fmt.Sprintf("operationId %q", explicit)
		}
		return issue.Report{{Code: "OPERATION_NOT_FOUND", Location: location, Message: fmt.Sprintf("No %s operation matches %s.", role, wanted), Remediation: "Fix the source API contract or select the exact operation ID."}}
	case 1:
		candidate := candidates[0]
		report := operationCandidateIssues(location, role, candidate, p)
		if role == opDelete && p.DeleteOperation != "" && len(report) == 0 {
			report = deleteActionIssues(location, candidate, found)
		}
		if len(report) == 0 {
			found[role] = candidate
		}
		return report
	default:
		var names []string
		for _, candidate := range candidates {
			names = append(names, candidate.op.OperationId)
		}
		slices.Sort(names)
		return issue.Report{{Code: "OPERATION_AMBIGUOUS", Location: location, Message: fmt.Sprintf("More than one %s operation matches: %s.", role, strings.Join(names, ", ")), Remediation: "Select one exact operation ID or make discovery unique in the source API contract."}}
	}
}

func operationCandidateIssues(location string, role verb, candidate foundOp, p Policy) issue.Report {
	if methods := p.methods(role); !slices.Contains(methods, candidate.method) {
		code, remediation := "OPERATION_METHOD_INCOMPATIBLE", "Use the required HTTP method in the source API contract."
		if role == opDelete && p.DeleteOperation != "" {
			code, remediation = "DELETE_OVERRIDE_METHOD_INCOMPATIBLE", "Name a POST operation in api.delete.operation, or delete the line when the API has a DELETE."
		}
		return issue.Report{{
			Code:        code,
			Location:    location + "." + candidate.op.OperationId,
			Message:     fmt.Sprintf("The method is %s. The %s operation needs %s.", candidate.method, role, strings.Join(methods, " or ")),
			Remediation: remediation,
		}}
	}
	return requiredParameterIssues(location, role, candidate)
}

// deleteOverrideLocation is the line of the behavior-overrides file that names the Delete operation.
const deleteOverrideLocation = "behavior-overrides.yaml:api.delete.operation"

// deleteOverrideConflictIssues reports a Delete override that the --delete-operation flag
// contradicts: both name the Delete operation.
func deleteOverrideConflictIssues(ids OperationIDs, p Policy) issue.Report {
	if p.DeleteOperation == "" || ids.Delete == "" {
		return nil
	}
	return issue.Report{{
		Code:        "DELETE_OVERRIDE_CONFLICT",
		Location:    deleteOverrideLocation,
		Message:     fmt.Sprintf("The overrides name the Delete operation %q, and --delete-operation names %q.", p.DeleteOperation, ids.Delete),
		Remediation: "Name the Delete operation in one place.",
	}}
}

func deleteOverrideNotFoundIssue(p Policy) issue.Issue {
	return issue.Issue{
		Code:        "DELETE_OVERRIDE_OPERATION_NOT_FOUND",
		Location:    deleteOverrideLocation,
		Message:     fmt.Sprintf("No operation has the operationId %q.", p.DeleteOperation),
		Remediation: "Name an operation of the API contract, or delete the line.",
	}
}

// deleteActionIssues checks the operation of a Delete override against Get: a POST on the Get
// path plus one segment, with the id path parameter of Get and no request body. The generated
// call sends only the id, so any other input would be lost.
func deleteActionIssues(location string, candidate foundOp, found map[verb]foundOp) issue.Report {
	get, ok := found[opGet]
	if !ok {
		return nil // The Get issue is reported.
	}
	location += "." + candidate.op.OperationId
	var report issue.Report
	getID, getErr := idParam(get)
	id, err := idParam(candidate)
	switch {
	case getErr != nil:
		report = append(report, issue.Issue{Code: "DELETE_OVERRIDE_ID_INCOMPATIBLE", Location: location + ".parameters", Message: fmt.Sprintf("Get has no single id path parameter (%s). A Delete override needs the resource id.", getErr), Remediation: "Use a Delete override only for a resource with an id in the Get path."})
	case err != nil:
		report = append(report, issue.Issue{Code: "DELETE_OVERRIDE_ID_INCOMPATIBLE", Location: location + ".parameters", Message: fmt.Sprintf("The operation has %s. It needs only the id path parameter %q of Get.", err, getID), Remediation: "Give the operation exactly the id path parameter of Get."})
	case id != getID:
		report = append(report, issue.Issue{Code: "DELETE_OVERRIDE_ID_INCOMPATIBLE", Location: location + ".parameters.path." + id, Message: fmt.Sprintf("The id path parameter is %q. Get uses %q.", id, getID), Remediation: "Give the operation exactly the id path parameter of Get."})
	}
	if !deleteActionPath(get.path, candidate.path) {
		report = append(report, issue.Issue{Code: "DELETE_OVERRIDE_PATH_INCOMPATIBLE", Location: location, Message: fmt.Sprintf("The path is %s. A Delete override needs the Get path plus one segment, for example %s/archive.", candidate.path, get.path), Remediation: "Name an operation on the Get path plus one segment."})
	}
	if candidate.op.RequestBody != nil {
		report = append(report, issue.Issue{Code: "DELETE_OVERRIDE_BODY_UNSUPPORTED", Location: location + ".requestBody", Message: "The operation has a request body. The generated Delete sends only the id.", Remediation: "Remove the request body from the operation, or wait until the generator can send it."})
	}
	return report
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

func validateFieldContracts(p Policy, name string, ops map[verb]foundOp) issue.Report {
	create, createErr := schemaOf(p.requestBody(ops[opCreate].op))
	if createErr == nil {
		folded, err := p.foldCreateBody(name, create)
		if err != nil {
			return issue.Report{{Code: "CREATE_AS_UNSUPPORTED", Location: fieldLocation(name, "createAs"), Message: err.Error(), Remediation: "Name an object field of the resource and the top-level Create fields that hold its properties."}}
		}
		create = folded
	}
	update, updateErr := schemaOf(p.requestBody(ops[opUpdate].op))
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
	idParam, bodyOnlyID := "", ""
	if params := pathParams(ops[opGet]); len(params) == 1 {
		// The id property, which api.idProperty can name apart from the path parameter.
		idParam = p.idProperty(ops)
		if len(pathParams(ops[opUpdate])) == 0 && !p.UpdateIDInBody {
			bodyOnlyID = idParam
		}
	}
	for _, field := range names {
		if field == bodyOnlyID {
			continue // Build reports the unsupported Update identity contract.
		}
		if p.UpdateMaskInBody && isUpdateMaskName(field) {
			continue // The mask stays in the JSON body and is not a Terraform field.
		}
		report = append(report, validateFieldContract(p, name, idParam, field, create, update, get)...)
	}
	report = append(report, resourceIDIssues(p, name, ops, get)...)
	report = append(report, bodyIDIssues(p, ops, create, update)...)
	report = append(report, serverSetIDIssues(p, name, ops, create, update)...)
	report = append(report, skippedIDIssues(p, name, ops)...)
	report = append(report, rootGroupContractIssues(name, create, update, get)...)
	return report
}

// bodyIDIssues checks the ids that a behavior-overrides policy says travel in a request body.
// Without the id property, the generated call sends nothing that names the resource, and the
// override has no effect. The generator fails closed, so a stale or wrong override is an error.
func bodyIDIssues(p Policy, ops map[verb]foundOp, create, update *base.Schema) issue.Report {
	id := p.idProperty(ops)
	if id == "" {
		return nil // A singleton has no id, and Build reports an unsupported identity shape.
	}
	var report issue.Report
	if p.ClientSetID && bodyProperty(create, id) == nil {
		report = append(report, issue.Issue{
			Code:        "CLIENT_SET_ID_NOT_IN_CREATE",
			Location:    "paths.create." + ops[opCreate].op.OperationId + ".requestBody",
			Message:     fmt.Sprintf("The override clientSetID says that the user can choose the id, but the Create body has no %q property.", id),
			Remediation: "Delete clientSetID from the overrides, or add the id property to the Create body.",
		})
	}
	if p.UpdateIDInBody && bodyProperty(update, id) == nil {
		report = append(report, issue.Issue{
			Code:        "UPDATE_ID_NOT_IN_BODY",
			Location:    "paths.update." + ops[opUpdate].op.OperationId + ".requestBody",
			Message:     fmt.Sprintf("The override updateIDInBody says that Update sends the id in the body, but the Update body has no %q property.", id),
			Remediation: "Delete updateIDInBody from the overrides, or add the id property to the Update body.",
		})
	}
	return report
}

// serverSetIDIssues checks that serverSetID describes an id that only the server sets: the
// Create and Update bodies do not have it, or it is read-only there or by a readOnly line.
// Otherwise the user could set the id in Terraform, and a body-only Update would send the
// planned id instead of the state id, so a change could name another resource.
func serverSetIDIssues(p Policy, name string, ops map[verb]foundOp, create, update *base.Schema) issue.Report {
	id := p.idProperty(ops)
	if !p.ServerSetID || id == "" || p.readOnly(name, id) {
		return nil
	}
	var report issue.Report
	for _, body := range []struct {
		op     verb
		schema *base.Schema
	}{{opCreate, create}, {opUpdate, update}} {
		if !userSetsProperty(body.schema, id) {
			continue
		}
		report = append(report, issue.Issue{
			Code:        "SERVER_SET_ID_IN_REQUEST",
			Location:    "paths." + strings.ToLower(string(body.op)) + "." + ops[body.op].op.OperationId + ".requestBody",
			Message:     fmt.Sprintf("The override serverSetID says that only the server sets the id, but the %s body has a %q property that is not read-only.", body.op, id),
			Remediation: fmt.Sprintf("Add readOnly: true to the %q line of %s, mark the property read-only in the contract, or delete serverSetID.", id, name),
		})
	}
	return report
}

// bodyProperty returns the property of a request body. The body can be allOf: [$ref], as the
// OpenAPI fork writes a reference with a description: the property is then on the referenced
// schema.
func bodyProperty(body *base.Schema, name string) *base.SchemaProxy {
	object, _, _ := validationObject(body)
	return propertyOf(object, name)
}

// userSetsProperty reports whether the request body has the property and does not mark it
// read-only. The property can be allOf: [$ref] too, with readOnly on the wrapper or on the
// referenced schema.
func userSetsProperty(body *base.Schema, name string) bool {
	property := bodyProperty(body, name)
	if property == nil {
		return false
	}
	s, err := schemaOf(property)
	if err != nil {
		return true
	}
	inner, _, _ := validationObject(s)
	return !isReadOnly(s) && !isReadOnly(inner)
}

func isReadOnly(s *base.Schema) bool {
	return s != nil && s.ReadOnly != nil && *s.ReadOnly
}

// skippedIDIssues reports a skip override on the resource id. Every call after Create names the
// resource by its id, so a resource without the id attribute cannot read, update, delete, or import.
func skippedIDIssues(p Policy, name string, ops map[verb]foundOp) issue.Report {
	id := p.idProperty(ops)
	if id == "" || !p.skips(name, id) {
		return nil
	}
	return issue.Report{{
		Code:        "RESOURCE_ID_SKIPPED",
		Location:    "components.schemas." + name + "." + id,
		Message:     fmt.Sprintf("The override skips the resource id field %q. Read, Update, Delete, and import need it.", id),
		Remediation: "Delete the skip line of the resource id from the overrides.",
	}}
}

// skippedArmIssues reports a skip override on an arm of a oneOf group. Skipping an arm changes
// what the group means: the remaining arm would become required, or the schema could not be built.
// The one exception is an empty object arm of a group that allows no arm, such as an Empty
// "none" marker: it holds no value, and Terraform selecting no arm leaves the group as it allows.
func skippedArmIssues(p Policy, root *Type) issue.Report {
	var report issue.Report
	seen := map[*Type]bool{}
	var visit func(t *Type)
	visit = func(t *Type) {
		if t == nil || seen[t] {
			return
		}
		seen[t] = true
		visit(t.Elem)
		if t.Kind != Object && t.Kind != OneOf {
			return
		}
		for _, arm := range oneOfArmNames(t) {
			if p.skips(t.Schema, arm) && !skippableArm(t, arm) {
				report = append(report, issue.Issue{
					Code:        "ONEOF_ARM_SKIPPED",
					Location:    "components.schemas." + t.Schema + "." + arm,
					Message:     fmt.Sprintf("The override skips %q, an arm of a oneOf group of %s.", arm, t.Schema),
					Remediation: "Delete the skip line, or skip the parent field of the whole group.",
				})
			}
		}
		for _, f := range t.Fields {
			visit(f.Type)
		}
	}
	visit(root)
	return report
}

// oneOfArmNames returns the field names that are arms of a oneOf group of the type.
func oneOfArmNames(t *Type) []string {
	var arms []string
	if t.Kind == OneOf {
		for _, f := range t.Fields {
			arms = append(arms, f.Name)
		}
	}
	for _, g := range t.Groups {
		arms = append(arms, g.Arms...)
	}
	slices.Sort(arms)
	return slices.Compact(arms)
}

// idProperty returns the property of the resource that holds the id: the property that the policy
// names, or the one named like the single path parameter of Get. It is "" when Get does not have
// exactly one path parameter.
func (p Policy) idProperty(ops map[verb]foundOp) string {
	params := pathParams(ops[opGet])
	if len(params) != 1 {
		return ""
	}
	if p.IDProperty != "" {
		return p.IDProperty
	}
	return params[0].Name
}

func resourceIDIssues(p Policy, name string, ops map[verb]foundOp, get *base.Schema) issue.Report {
	id := p.idProperty(ops)
	if id == "" {
		return nil // Build reports an unsupported resource identity shape.
	}
	if params := pathParams(ops[opGet]); p.IDProperty != "" && params[0].Name == p.IDProperty {
		return issue.Report{{
			Code:        "ID_PROPERTY_UNNEEDED",
			Location:    "components.schemas." + name + "." + id,
			Message:     fmt.Sprintf("The override idProperty names %q, which is already the name of the Get path parameter.", id),
			Remediation: "Delete api.idProperty from the overrides.",
		}}
	}
	property := propertyOf(get, id)
	if property == nil {
		return nil
	}
	location := "components.schemas." + name + "." + id
	var report issue.Report
	// A released resource can share its schema between the request and the response, so the
	// contract cannot require the id that only the server sets. serverSetID states that the
	// API returns it anyway, and flatten checks it.
	required := slices.Contains(get.Required, id)
	if required && p.ServerSetID {
		report = append(report, issue.Issue{
			Code:        "SERVER_SET_ID_UNNEEDED",
			Location:    location,
			Message:     fmt.Sprintf("The override serverSetID says that the Get response cannot require %q, but it does.", id),
			Remediation: "Delete serverSetID from the overrides.",
		})
	}
	if !required && !p.ClientSetID && !p.ServerSetID {
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

func responseWrapperIssues(p Policy, name string, ops map[verb]foundOp) issue.Report {
	if p.Existing {
		return nil // a released API can wrap the resource in one response property
	}
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
		if media == nil {
			continue
		}
		ref := responseRef(media.Schema)
		if ref == nil {
			continue
		}
		component, err := componentName(ref.GetReference())
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

func validateFieldContract(p Policy, name, idParam, field string, create, update, get *base.Schema) issue.Report {
	if p.skips(name, field) {
		return skippedFieldIssues(name, field, create, update)
	}
	var report issue.Report
	if slices.Contains(p.Optional, name+"."+field) && !requires(create, field) && !requires(update, field) {
		// The requests of the resource have their own components, so only they say whether the
		// line relaxes anything; the response component that the file names does not.
		report = append(report, issue.Issue{
			Code:        "OVERRIDE_UNUSED",
			Location:    "behavior-overrides.yaml:types." + name + ".fields." + field,
			Message:     fmt.Sprintf("the Create and Update requests already leave %q optional", field),
			Remediation: "Delete only the key optional.",
		})
	}
	create, update = p.withOptional(name, create), p.withOptional(name, update)
	cp, up := requestContractProperty(create, field), requestContractProperty(update, field)
	if p.readOnly(name, field) {
		cp, up = nil, nil
	}
	gp := propertyOf(get, field)
	location := fieldLocation(name, field)
	if _, err := classifyTopLevel(p, idParam, field, cp != nil, up != nil, gp != nil, schemaDefault(up) != nil); err != nil {
		return issue.Report{{Code: "FIELD_LIFECYCLE_UNSUPPORTED", Location: location, Message: fmt.Sprintf("The field locations are Create=%t, Update=%t, Get=%t.", cp != nil, up != nil, gp != nil), Remediation: "Use a managed, immutable, or computed field lifecycle."}}
	}
	// A computed line on a field that Get requires has a value in state. With a plan modifier that
	// keeps it, the plan has it too, so Update can always send it. Without one, a change of another
	// attribute plans the field unknown, and Update would leave out a field that it requires.
	kept := p.computed(name, field) && p.keepsState(name, field) && slices.Contains(get.Required, field)
	if cp != nil && up != nil && !slices.Contains(create.Required, field) && slices.Contains(update.Required, field) && !kept {
		report = append(report, issue.Issue{
			Code:        "FIELD_REQUIREDNESS_UNSUPPORTED",
			Location:    location,
			Message:     "The field is optional in Create but required in Update.",
			Remediation: "Make the field optional in Update, or require it in Create so Terraform always has a value to send.",
		})
	}
	report = append(report, fieldTypeIssues(p, name, field, location, gp, cp, up)...)
	report = append(report, fieldDefaultContractIssues(p, name, location, field, create, update, get, cp, up, gp)...)
	// A field with a line in the behavior-overrides file keeps its released behavior, so
	// the contract need not say whether omission differs from an empty value.
	if !p.released(name, field) {
		report = append(report, fieldPresenceIssue(location+".create", field, create, cp, schemaDefault(cp))...)
		// Generation reads the Create default. A field that only Update has declares it there.
		updateDefault := schemaDefault(up)
		if cp != nil {
			updateDefault = schemaDefault(cp)
		}
		if !createRequires(create, cp, field) {
			report = append(report, fieldPresenceIssue(location+".update", field, update, up, updateDefault)...)
		}
	}
	// The response decides which wrappers collapse, as in the generated resource: a request can
	// name its components differently.
	wrapped := p.unwraps(name, field, wrapperComponent(gp))
	// A skipped field is never sent, so the fields inside it need no presence.
	if !p.skips(name, field) {
		report = append(report, nestedPresenceIssues(p, location+".create", cp, gp, nil, wrapped, map[presenceVisit]bool{})...)
		report = append(report, nestedPresenceIssues(p, location+".update", up, gp, cp, wrapped, map[presenceVisit]bool{})...)
	}
	report = append(report, nestedReadOnlyIssues(p, location+".create", cp, map[*base.Schema]bool{})...)
	report = append(report, nestedReadOnlyIssues(p, location+".update", up, map[*base.Schema]bool{})...)
	report = append(report, unsupportedSchemaIssues(p, location+".create", cp, true, map[*base.Schema]bool{})...)
	report = append(report, unsupportedSchemaIssues(p, location+".update", up, true, map[*base.Schema]bool{})...)
	report = append(report, unsupportedSchemaIssues(p, location+".get", gp, false, map[*base.Schema]bool{})...)
	return report
}

// skippedFieldIssues checks a top-level field that the resource does not manage. The
// generated requests never send it, so no request may require it.
func skippedFieldIssues(name, field string, create, update *base.Schema) issue.Report {
	for _, body := range []*base.Schema{create, update} {
		// A read-only property is required in responses only: the request does not send it.
		if body != nil && slices.Contains(body.Required, field) && requestContractProperty(body, field) != nil {
			return issue.Report{{
				Code:        "SKIP_REQUIRED_FIELD",
				Location:    fieldLocation(name, field),
				Message:     "The field is skipped, but a request requires it.",
				Remediation: "Delete the skip line: the generated request must send the field.",
			}}
		}
	}
	return nil
}

func fieldDefaultContractIssues(p Policy, name, location, field string, create, update, get *base.Schema, cp, up, gp *base.SchemaProxy) issue.Report {
	var report issue.Report
	createDefault := schemaDefault(cp)
	updateDefault := schemaDefault(up)
	report = append(report, defaultValueIssues(location+".create", cp)...)
	report = append(report, defaultValueIssues(location+".update", up)...)
	if p.OverridesDefault(name, field) {
		return report // the default line of the overrides replaces the declared default
	}
	createOptional := cp != nil && !slices.Contains(create.Required, field)
	getRequired := gp != nil && slices.Contains(get.Required, field)
	// A released field states how an omitted value behaves, so it needs no declared default.
	// A declared default is still checked: the generated resource uses it.
	if createOptional && getRequired && createDefault == nil && !p.released(name, field) {
		report = append(report, issue.Issue{Code: "FIELD_SERVER_DEFAULT_UNDECLARED", Location: location, Message: "The field is optional in Create but required in Get, so the server supplies a value without a declared default.", Remediation: "Declare the exact OpenAPI default, or separate the client-owned request field from the server-owned response field."})
	}
	// A field that only Update has declares its server default there: Create stores it.
	updateOnly := cp == nil && up != nil
	if updateOnly {
		createDefault = updateDefault
	}
	// The response may state the default with required or with the same default. Without
	// either, an omitted value reads as null: the response can omit a value that is not the
	// default, for example a record that the request enum cannot express.
	getDefault := schemaDefault(gp)
	if createDefault != nil && getDefault != nil && *getDefault != *createDefault {
		report = append(report, issue.Issue{Code: "FIELD_DEFAULT_CONTRACT_INCONSISTENT", Location: location, Message: "The Get field declares a different default than the request.", Remediation: "Declare the same default in every schema of the field."})
	}
	updateRequired := up != nil && slices.Contains(update.Required, field)
	report = append(report, declaredDefaultIssues(location, createDefault, updateDefault, (createOptional || updateOnly) && gp != nil, updateRequired)...)
	for _, candidate := range []struct {
		name  string
		proxy *base.SchemaProxy
	}{{"create", cp}, {"update", up}} {
		report = append(report, nestedDefaultIssues(p, location+"."+candidate.name, candidate.proxy, false, map[*base.Schema]bool{})...)
	}
	return report
}

// declaredDefaultIssues checks that a declared server default fits the field: optional in Create
// and Update, returned by Get, and the same default in Create and Update.
func declaredDefaultIssues(location string, createDefault, updateDefault *string, serverOwned, updateRequired bool) issue.Report {
	switch {
	case createDefault == nil && updateDefault != nil:
		return issue.Report{{Code: "FIELD_DEFAULT_CONTRACT_INCONSISTENT", Location: location, Message: "The Update field declares a default that Create does not declare.", Remediation: "Declare the default on the optional Create field, or remove it from Update."}}
	case createDefault == nil:
		return nil
	case !serverOwned:
		return issue.Report{{Code: "FIELD_DEFAULT_CONTRACT_INCONSISTENT", Location: location, Message: "A declared server default needs an optional Create field that Get returns.", Remediation: "Make the field optional in Create, and return it in Get."}}
	case updateRequired || updateDefault != nil && *updateDefault != *createDefault:
		// Update may omit the default: generation reads only the Create default.
		return issue.Report{{Code: "FIELD_DEFAULT_CONTRACT_INCONSISTENT", Location: location, Message: "The mutable field is required in Update, or Update declares a different default than Create.", Remediation: "Make the field optional in Update. Omit its default there, or declare the Create default."}}
	}
	return nil
}

func defaultValueIssues(location string, proxy *base.SchemaProxy) issue.Report {
	if proxy == nil {
		return nil
	}
	schema, err := schemaOf(proxy)
	if err != nil || schema.Default == nil {
		return nil
	}
	if target := refSiblingTarget(proxy, schema); target != nil {
		// A default next to a $ref has the type and the limits of the target, as typeOf reads them.
		merged := *target
		merged.Default = schema.Default
		schema = &merged
	}
	if len(schema.Type) != 1 {
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

// permissivePattern is the pattern that the API generator adds to every string. It accepts
// every value, so it is not a constraint.
const permissivePattern = `^[\s\S]*$`

// goPattern reports whether the generated validator can use pattern: Go
// compiles it, or it is empty.
func goPattern(pattern string) bool {
	_, err := regexp.Compile(pattern)
	return err == nil
}

// omissionIsDefault reports whether omitting an Update field with server default d stores the
// same value as sending the value that the generated SDK leaves out: the zero value.
func omissionIsDefault(proxy *base.SchemaProxy, d *string) bool {
	if d == nil || proxy == nil {
		return false
	}
	schema, err := schemaOf(proxy)
	if err != nil {
		return false
	}
	if target := refSiblingTarget(proxy, schema); target != nil {
		schema = target
	}
	zero, ok := scalarZero(valueSchema(schema))
	if !ok {
		return true // the zero value is not a value that Terraform sends
	}
	if *d == zero {
		return true
	}
	// A number default can spell zero another way, such as 0.0.
	n, err := strconv.ParseFloat(*d, 64)
	return zero == "0" && err == nil && n == 0 && !plainStringSchema(valueSchema(schema))
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

// nestedDefaultIssues reports the nested request fields whose server default the generated resource
// cannot honor. viaAllOf is true below an allOf wrapper.
func nestedDefaultIssues(p Policy, location string, proxy *base.SchemaProxy, viaAllOf bool, seen map[*base.Schema]bool) issue.Report {
	if proxy == nil {
		return nil
	}
	schema, err := schemaOf(proxy)
	if err != nil || seen[schema] {
		return nil
	}
	seen[schema] = true
	object, ref, required := validationObject(schema)
	var report issue.Report
	viaAllOf = viaAllOf || ref != nil
	for _, name := range propertyNames(object) {
		child := propertyOf(object, name)
		if !p.Existing && nestedDefault(child) {
			// A new resource plans an omitted nested field with a server default as unknown,
			// and the response returns the default: merge requires the same default there. The
			// resource model does not read through allOf, so a default there is unsupported. A
			// required field is never omitted, so its default says nothing.
			switch {
			case viaAllOf || allOfDefault(child):
				report = append(report, issue.Issue{Code: "NESTED_FIELD_DEFAULT_UNSUPPORTED", Location: location + "." + name, Message: "A nested request field declares a server default through allOf.", Remediation: "Use a $ref with a sibling default, or remove the default."})
			case slices.Contains(required, name):
				report = append(report, issue.Issue{Code: "NESTED_FIELD_DEFAULT_UNSUPPORTED", Location: location + "." + name, Message: "A required nested request field declares a server default.", Remediation: "Make the field optional, or remove its default."})
			}
		}
		report = append(report, nestedDefaultIssues(p, location+"."+name, child, viaAllOf, seen)...)
	}
	if object.Items != nil && object.Items.IsA() {
		report = append(report, nestedDefaultIssues(p, location+"[]", object.Items.A, viaAllOf, seen)...)
	}
	if object.AdditionalProperties != nil && object.AdditionalProperties.IsA() {
		report = append(report, nestedDefaultIssues(p, location+"{}", object.AdditionalProperties.A, viaAllOf, seen)...)
	}
	return report
}

// nestedDefault reports whether a nested field declares a server default, on the field or on
// the schema that its allOf wrapper names.
func nestedDefault(proxy *base.SchemaProxy) bool {
	schema, err := schemaOf(proxy)
	if err != nil {
		return false
	}
	object, _, _ := validationObject(schema)
	return schema.Default != nil || object.Default != nil
}

// allOfDefault reports whether the field is an allOf wrapper of one $ref.
func allOfDefault(proxy *base.SchemaProxy) bool {
	schema, err := schemaOf(proxy)
	if err != nil {
		return false
	}
	_, ref, _ := validationObject(schema)
	return ref != nil
}

// unsupportedSchemaIssues reports the keywords below proxy that the generator
// cannot honor. request is false for the resource response. A pattern becomes a
// Go regular expression validator of the configuration. The configuration sets
// only the values that a request sends, so a response pattern is no issue.
func unsupportedSchemaIssues(p Policy, location string, proxy *base.SchemaProxy, request bool, seen map[*base.Schema]bool) issue.Report {
	if proxy == nil {
		return nil
	}
	schema, err := schemaOf(proxy)
	if err != nil || seen[schema] {
		return nil
	}
	seen[schema] = true
	object, _, _ := validationObject(schema)
	var report issue.Report
	for _, target := range []*base.Schema{schema, object} {
		if target.WriteOnly != nil && *target.WriteOnly {
			report = append(report, issue.Issue{Code: "FIELD_WRITE_ONLY_UNSUPPORTED", Location: location, Message: "The field is writeOnly, but this generator cannot preserve or rotate a value that the API does not return.", Remediation: "Use a handwritten resource until generic write-only state and version handling is supported."})
		}
		if request && !p.Existing {
			report = append(report, patternIssues(location, target)...)
		}
		if target == object {
			break
		}
	}
	for _, name := range propertyNames(object) {
		report = append(report, unsupportedSchemaIssues(p, location+"."+name, propertyOf(object, name), request, seen)...)
	}
	if object.Items != nil && object.Items.IsA() {
		report = append(report, unsupportedSchemaIssues(p, location+"[]", object.Items.A, request, seen)...)
	}
	if object.AdditionalProperties != nil && object.AdditionalProperties.IsA() {
		report = append(report, unsupportedSchemaIssues(p, location+"{}", object.AdditionalProperties.A, request, seen)...)
	}
	return report
}

// patternIssues reports a request pattern that the generated validator cannot
// enforce: Go (RE2) cannot compile it, or it is on an enum, whose attribute
// gets only the OneOf validator.
func patternIssues(location string, schema *base.Schema) issue.Report {
	switch {
	case !goPattern(schema.Pattern):
		return issue.Report{{Code: "STRING_PATTERN_UNSUPPORTED", Location: location, Message: fmt.Sprintf("Go regular expressions (RE2) cannot compile the pattern %q.", schema.Pattern), Remediation: "Use a pattern without lookaround or backreferences."}}
	case len(schema.Enum) != 0 && schema.Pattern != "" && schema.Pattern != permissivePattern:
		return issue.Report{{Code: "STRING_PATTERN_UNSUPPORTED", Location: location, Message: fmt.Sprintf("The enum declares the pattern %q. The enum values already limit the value.", schema.Pattern), Remediation: "Remove the pattern from the enum."}}
	}
	return nil
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

func nestedReadOnlyIssues(p Policy, location string, proxy *base.SchemaProxy, seen map[*base.Schema]bool) issue.Report {
	if proxy == nil {
		return nil
	}
	schema, err := schemaOf(proxy)
	if err != nil || seen[schema] {
		return nil
	}
	seen[schema] = true
	object, ref, _ := validationObject(schema)
	component := proxy
	if ref != nil {
		component = ref
	}
	var report issue.Report
	for _, name := range propertyNames(object) {
		child := propertyOf(object, name)
		childSchema, childErr := schemaOf(child)
		if childErr != nil {
			continue
		}
		childLocation := location + "." + name
		if p.skips(referencedComponent(component), name) {
			continue // the resource does not manage this field, so it is never sent
		}
		if childSchema.ReadOnly != nil && *childSchema.ReadOnly {
			// requestValue sends a fixed value, so the field is not a server-only
			// property the renderer has to drop.
			if p.fixedRequest(referencedComponent(component), name) {
				continue
			}
			if p.Existing {
				// A released API can share the component with the response. The request does
				// not send the field, so it is computed.
				continue
			}
			report = append(report, issue.Issue{Code: "FIELD_LIFECYCLE_UNSUPPORTED", Location: childLocation, Message: "A nested request field is readOnly and cannot be removed by the current resource renderer.", Remediation: "Use separate request and response schemas so server-only fields are absent from request objects."})
			continue
		}
		report = append(report, nestedReadOnlyIssues(p, childLocation, child, seen)...)
	}
	if object.Items != nil && object.Items.IsA() {
		report = append(report, nestedReadOnlyIssues(p, location+"[]", object.Items.A, seen)...)
	}
	if object.AdditionalProperties != nil && object.AdditionalProperties.IsA() {
		report = append(report, nestedReadOnlyIssues(p, location+"{}", object.AdditionalProperties.A, seen)...)
	}
	return report
}

// fieldTypeIssues merges the Create and Update types of a field into its type
// in the resource response, as Build does. A schema that is not supported is
// left out: unsupportedSchemaIssues reports it.
func fieldTypeIssues(p Policy, name, field, location string, gp, cp, up *base.SchemaProxy) issue.Report {
	if gp == nil {
		return nil
	}
	get, err := typeOf(gp, location, walk{policy: &p})
	p.asSet(name, field, get)
	if err != nil {
		return nil
	}
	var requests [2]*Type
	for i, proxy := range []*base.SchemaProxy{cp, up} {
		if proxy == nil {
			continue
		}
		if requests[i], err = typeOf(proxy, location, walk{policy: &p}); err != nil {
			return nil
		}
		p.asSet(name, field, requests[i])
	}
	var contract *contractError
	if err := mergeRequests(location, get, requests[0], requests[1]); errors.As(err, &contract) {
		return issue.Report{contract.issue()}
	}
	return nil
}

// fieldPresenceIssue reports a request scalar that must preserve field presence. serverDefault is
// the declared default that the server stores for an omitted value, or nil.
func fieldPresenceIssue(location, field string, parent *base.Schema, proxy *base.SchemaProxy, serverDefault *string) issue.Report {
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
	// Proto3 does not send a zero scalar, so the server reads it as omitted. Without a declared
	// default, omission and zero both store zero, and the generated resource keeps the prior form
	// on read. A default that is the zero value, or a zero that is not valid (an enum zero), also
	// stores the same value. Only a valid zero next to another default, such as false next to a
	// default of true, cannot be sent without presence.
	if serverDefault == nil || omissionIsDefault(proxy, serverDefault) {
		return nil
	}
	return issue.Report{{Code: "FIELD_PRESENCE_UNKNOWN", Location: location, Message: fmt.Sprintf("The optional request field has the default %s, so the server stores an explicit zero value as that default.", *serverDefault), Remediation: "Add x-coralogix-presence: true to the source API contract (a proto3 optional field)."}}
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

// validationObject returns the object that a validation walk reads. A schema
// that is only allOf of one $ref uses the inner properties. required is the
// union of the required lists on the way. ref is that $ref, so the component
// name is the inner component. Build still reads schema.Properties: the
// wrapper required list is empty, and a required inner field would look optional.
func validationObject(schema *base.Schema) (object *base.Schema, ref *base.SchemaProxy, required []string) {
	if schema == nil {
		return nil, nil, nil
	}
	required = append([]string{}, schema.Required...)
	object = schema
	seen := map[*base.Schema]bool{}
	for !seen[object] {
		seen[object] = true
		inner, err := singleAllOf(object, "schema")
		if err != nil {
			break
		}
		resolved, err := schemaOf(inner)
		if err != nil {
			break
		}
		ref = inner
		for _, name := range resolved.Required {
			if !slices.Contains(required, name) {
				required = append(required, name)
			}
		}
		object = resolved
	}
	return object, ref, required
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

// scalarZeroValueValid reports whether a client can set the protobuf zero value of a scalar.
func scalarZeroValueValid(schema *base.Schema) bool {
	_, ok := scalarZero(schema)
	return ok
}

// scalarZero returns the protobuf zero value of a scalar, as the text that a default holds, and
// whether a client can set it. An enum zero is never set, and neither is a zero outside the
// limits of the field. Matches settable_zero in scripts/lint_iac_contract.py of
// cx-management-apis.
func scalarZero(schema *base.Schema) (string, bool) {
	if inner, err := singleAllOf(schema, "schema"); err == nil {
		resolved, resolveErr := schemaOf(inner)
		if resolveErr != nil {
			return "", false
		}
		return scalarZero(resolved)
	}
	if len(schema.Enum) != 0 || len(schema.Type) != 1 {
		return "", false
	}
	switch schema.Type[0] {
	case "boolean":
		return "false", true
	case "integer", "number":
		return "0", numberAcceptsZero(schema)
	case "string":
		return stringZero(schema)
	default:
		return "", false
	}
}

// numberAcceptsZero reports whether the bounds of a number accept 0: inclusive bounds, and
// exclusive bounds in both the OpenAPI 3.1 form (a number) and the 3.0 form (true next to
// minimum or maximum). Matches number_accepts_zero in the API lint.
func numberAcceptsZero(schema *base.Schema) bool {
	if schema.Minimum != nil && *schema.Minimum > 0 || schema.Maximum != nil && *schema.Maximum < 0 {
		return false
	}
	for _, bound := range []struct {
		exclusive *base.DynamicValue[bool, float64]
		inclusive *float64
		excludes  func(float64) bool
	}{
		{schema.ExclusiveMinimum, schema.Minimum, func(v float64) bool { return v >= 0 }},
		{schema.ExclusiveMaximum, schema.Maximum, func(v float64) bool { return v <= 0 }},
	} {
		switch {
		case bound.exclusive == nil:
		case bound.exclusive.IsA() && bound.exclusive.A && bound.inclusive != nil && *bound.inclusive == 0:
			return false
		case bound.exclusive.IsB() && bound.excludes(bound.exclusive.B):
			return false
		}
	}
	return true
}

// stringZero returns the protobuf zero value of a string and whether a client can set it. A
// 64-bit number is a string whose zero is "0": with an int64 or uint64 format, or as a digit
// string with no format. Matches the string part of settable_zero in the API lint.
func stringZero(schema *base.Schema) (string, bool) {
	value := ""
	switch {
	case schema.Format == "int64" || schema.Format == "uint64":
		value = "0"
	case schema.Format == "" && digitString(schema.Pattern):
		value = "0"
	case schema.Format == "":
	default:
		return "", false
	}
	length := int64(len([]rune(value)))
	if schema.MinLength != nil && length < *schema.MinLength || schema.MaxLength != nil && length > *schema.MaxLength {
		return "", false
	}
	if schema.Pattern == "" {
		return value, true
	}
	re, err := regexp.Compile(schema.Pattern)
	return value, err == nil && re.MatchString(value)
}

// int64Patterns are the patterns that the OpenAPI generator (protoc-gen-openapiv3,
// stringInt64Pattern) writes for a 64-bit number sent as a string without a format. Matches
// INT64_PATTERNS in the API lint.
var int64Patterns = []string{`^[0-9]+$`, `^-?[0-9]+$`}

// digitString reports whether a string is a 64-bit number that the OpenAPI generator writes as a
// digit string without a format. Its protobuf zero is "0". Only the exact generated patterns
// count: a pattern such as ^[A-Z0-9]+$ also accepts "0", but its field is a protobuf string.
// Matches digit_string in the API lint.
func digitString(pattern string) bool {
	return slices.Contains(int64Patterns, pattern)
}

// createRequires reports whether Create requires the field. Terraform then requires it, and a
// required value is never empty, so Update always sends a value: omission and an empty value
// never differ there, and Update needs no presence mark.
func createRequires(create *base.Schema, cp *base.SchemaProxy, field string) bool {
	return cp != nil && slices.Contains(create.Required, field)
}

// asksPresence reports whether a nested field must state presence. A field with a line keeps its
// released behavior. A field that Create requires in the same place needs no mark in Update, as
// createRequires says.
func asksPresence(p Policy, component *base.SchemaProxy, name string, createRequired []string) bool {
	return !p.released(referencedComponent(component), name) && !slices.Contains(createRequired, name)
}

// unsentField reports whether Terraform never sends a nested request field: the field is
// readOnly, or a line skips it.
func unsentField(p Policy, component *base.SchemaProxy, name string, child *base.SchemaProxy) bool {
	if p.skips(referencedComponent(component), name) {
		return true
	}
	schema, err := schemaOf(child)
	return err == nil && schema.ReadOnly != nil && *schema.ReadOnly
}

// presenceVisit is one request schema that nestedPresenceIssues checked, with the response
// schema in the same place, as a wrapper that collapses or as an object. A component can be
// both, in two places.
type presenceVisit struct {
	schema, get, create *base.Schema
	wrapped             bool
}

// nestedPresenceIssues reports the nested request fields that do not state presence. get is the
// response schema in the same place. wrapped is true when proxy, or the items of the list that
// proxy is, is a wrapper that collapses: the wrapper states the presence of its property,
// because Terraform null sends no wrapper. The response components decide which wrappers
// collapse, as they do in the generated resource. create is the Create schema in the same
// place when proxy is an Update schema, and nil otherwise.
func nestedPresenceIssues(p Policy, location string, proxy, get, create *base.SchemaProxy, wrapped bool, seen map[presenceVisit]bool) issue.Report {
	if proxy == nil {
		return nil
	}
	schema, err := schemaOf(proxy)
	if err != nil {
		return nil
	}
	getSchema, _ := schemaOf(get)
	createSchema, _ := schemaOf(create)
	visit := presenceVisit{schema, getSchema, createSchema, wrapped}
	if seen[visit] {
		return nil
	}
	seen[visit] = true
	object, ref, required := validationObject(schema)
	component := proxy
	if ref != nil {
		component = ref
	}
	parent := *object
	parent.Required = p.requiredOf(referencedComponent(component), required)
	getObject, _, _ := validationObject(getSchema)
	createObject, _, createRequired := validationObject(createSchema)
	var report issue.Report
	grouped := groupedFields(schema)
	owner, _ := componentName(unwrapRef(get))
	for _, name := range propertyNames(object) {
		child := propertyOf(object, name)
		if unsentField(p, component, name, child) {
			continue
		}
		createChild := propertyOf(createObject, name)
		if !wrapped && !grouped[name] && asksPresence(p, component, name, createRequired) {
			report = append(report, fieldPresenceIssue(location+"."+name, name, &parent, child, requestDefault(child, createChild))...)
		}
		getChild := propertyOf(getObject, name)
		report = append(report, nestedPresenceIssues(p, location+"."+name, child, getChild, createChild, p.unwraps(owner, name, wrapperComponent(getChild)), seen)...)
	}
	if object.Items != nil && object.Items.IsA() {
		report = append(report, nestedPresenceIssues(p, location+"[]", object.Items.A, itemsOf(getObject), itemsOf(createObject), wrapped, seen)...)
	}
	if object.AdditionalProperties != nil && object.AdditionalProperties.IsA() {
		report = append(report, nestedPresenceIssues(p, location+"{}", object.AdditionalProperties.A, nil, nil, false, seen)...)
	}
	return report
}

// requestDefault returns the declared default of a nested request field. In Update, create is the
// same field in Create, and generation reads its default.
func requestDefault(field, create *base.SchemaProxy) *string {
	if create != nil {
		return schemaDefault(create)
	}
	return schemaDefault(field)
}

// itemsOf returns the items of the list schema s, or nil.
func itemsOf(s *base.Schema) *base.SchemaProxy {
	if s == nil || s.Items == nil || !s.Items.IsA() {
		return nil
	}
	return s.Items.A
}

// wrapperComponent returns the component of a property that could be a wrapper: the property
// itself, or the items of a list. It returns "" for any other schema.
func wrapperComponent(proxy *base.SchemaProxy) string {
	if proxy == nil {
		return ""
	}
	if name, err := componentName(unwrapRef(proxy)); err == nil {
		return name
	}
	s, err := schemaOf(proxy)
	if err != nil {
		return ""
	}
	name, err := componentName(unwrapRef(itemsOf(s)))
	if err != nil {
		return ""
	}
	return name
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
	schema := media.Schema
	if ref := responseRef(schema); ref != nil {
		schema = ref
	}
	if schema.GetReference() == componentPrefix+name {
		return schema
	}
	s, err := schemaOf(schema)
	if err != nil {
		return nil
	}
	for _, field := range propertyNames(s) {
		proxy := propertyOf(s, field)
		if unwrapRef(proxy) == componentPrefix+name {
			// The property can be "allOf: [$ref: X]". Return the $ref, whose schema has the fields.
			if !proxy.IsReference() {
				return proxy.Schema().AllOf[0]
			}
			return proxy
		}
	}
	return nil
}

func validateBuiltResource(r *Resource) issue.Report {
	if r.Replace {
		return nil
	}
	if r.UpdateMaskPattern == "" {
		return issue.Report{{Code: "UPDATE_MASK_CONTRACT_MISSING", Location: "paths.update." + r.Update.OperationID, Message: "The PATCH update mask has no pattern that defines accepted mask paths.", Remediation: "Add the authoritative update-mask path pattern to the source API contract."}}
	}
	_, leaf, err := UpdateMaskRule(r.UpdateMaskPattern)
	if err != nil {
		return issue.Report{{Code: "UPDATE_MASK_CONTRACT_INVALID", Location: "paths.update." + r.Update.OperationID, Message: err.Error() + ".", Remediation: "Use a mask pattern that accepts field names and comma-separated lists of them, rejects *, and defines whether dotted paths are supported."}}
	}
	if leaf {
		return nil
	}
	return nestedOneOfMaskIssues(r)
}

func nestedOneOfMaskIssues(r *Resource) issue.Report {
	var report issue.Report
	for _, field := range r.Fields {
		if field.Update == nil {
			continue
		}
		paths := oneOfMaskPaths(field.Name, field.Type.UpdateType())
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
	if len(t.Wrappers) != 0 {
		prefix += "." + t.WrapperPath()
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

func nameCollisions(p Policy, location string, t *Type) issue.Report {
	var report issue.Report
	if t == nil {
		return nil
	}
	if t.Kind == List || t.Kind == Set || t.Kind == Map {
		return nameCollisions(p, location+"[]", t.Elem)
	}
	if t.Kind != Object && t.Kind != OneOf {
		return nil
	}
	terraformNames := map[string]string{}
	goNames := map[string]string{}
	for _, field := range t.Fields {
		if p.skips(t.Schema, field.Name) || p.fixedRequest(t.Schema, field.Name) {
			continue // a skipped field or a fixed request value has no Terraform attribute or Go field
		}
		terraformName := p.TerraformName(t.Schema, field.Name)
		if previous, ok := terraformNames[terraformName]; ok && previous != field.Name {
			report = append(report, issue.Issue{Code: "TERRAFORM_NAME_COLLISION", Location: location, Message: fmt.Sprintf("Fields %q and %q both normalize to %q.", previous, field.Name, terraformName), Remediation: "Rename a source API field, or give one a name line, so every Terraform name is unique."})
		} else {
			terraformNames[terraformName] = field.Name
		}
		goName := GoName(field.Name)
		if previous, ok := goNames[goName]; ok && previous != field.Name {
			report = append(report, issue.Issue{Code: "GO_NAME_COLLISION", Location: location, Message: fmt.Sprintf("Fields %q and %q both generate the Go name %q.", previous, field.Name, goName), Remediation: "Rename a source API field so every generated Go field name is unique."})
		} else {
			goNames[goName] = field.Name
		}
		report = append(report, nameCollisions(p, location+"."+field.Name, field.Type)...)
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
		// A request component of an object names its expand function. A wrapper that collapsed
		// is no longer a type of the graph, but the generated code still names its components.
		schemas := []string{t.Schema, t.CreateSchema, t.UpdateSchema}
		for _, w := range t.Wrappers {
			schemas = append(schemas, w.Schema, w.CreateSchema, w.UpdateSchema)
		}
		for _, schema := range schemas {
			if schema == "" || visitedSchemas[schema] {
				continue
			}
			visitedSchemas[schema] = true
			generated := GoName(schema)
			if previous, ok := generatedNames[generated]; ok && previous != schema {
				report = append(report, issue.Issue{
					Code:        "GO_COMPONENT_NAME_COLLISION",
					Location:    "components.schemas",
					Message:     fmt.Sprintf("Components %q and %q both generate the Go name %q.", previous, schema, generated),
					Remediation: "Rename a source API component so every generated Go type name is unique.",
				})
			} else {
				generatedNames[generated] = schema
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
	var contract *contractError
	if errors.As(err, &contract) {
		return contract.issue()
	}
	message := err.Error()
	code := "RESOURCE_LIFECYCLE_INCOMPLETE"
	remediation := "Correct the source API contract so the complete resource lifecycle is deterministic."
	switch {
	case strings.Contains(message, "id in the request body is not supported"):
		code = "UPDATE_ID_IN_BODY_UNSUPPORTED"
		remediation = "Put the resource id in the Update path. Add body-id compatibility only during existing-resource migration."
	case strings.Contains(message, "update mask") || strings.Contains(message, updateMaskField) || strings.Contains(message, updateMaskProtoField):
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

// skippableArm reports whether arm is an empty object arm of a group of t that allows no arm.
func skippableArm(t *Type, arm string) bool {
	field := slices.IndexFunc(t.Fields, func(f *Field) bool { return f.Name == arm })
	if field < 0 || t.Fields[field].Type.Kind != Object || len(t.Fields[field].Type.Fields) != 0 {
		return false
	}
	switch t.Kind {
	case Object:
		return slices.ContainsFunc(t.Groups, func(g OneOfGroup) bool { return slices.Contains(g.Arms, arm) && g.AllowNone })
	case OneOf:
		// A type that only has the arms of one group keeps the group in its own AllowNone.
		return t.AllowNone
	}
	return false
}

// requires reports whether the request body s requires field.
func requires(s *base.Schema, field string) bool {
	return s != nil && slices.Contains(s.Required, field)
}
