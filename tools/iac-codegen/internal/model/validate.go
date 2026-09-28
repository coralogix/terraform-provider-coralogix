package model

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"unicode"

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
		if terraformName(name) == selection {
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
	}
	if len(ops) == len(verbs) {
		report = append(report, validateFieldContracts(name, ops)...)
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
			if !slices.Contains(verbMethods[role], candidate.method) {
				report = append(report, issue.Issue{Code: "OPERATION_METHOD_INCOMPATIBLE", Location: location + "." + candidate.op.OperationId, Message: fmt.Sprintf("The method is %s. The %s operation needs %s.", candidate.method, role, strings.Join(verbMethods[role], " or ")), Remediation: "Use the required HTTP method in the source API contract."})
			} else {
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
	for _, field := range names {
		if field == updateMaskField {
			continue
		}
		report = append(report, validateFieldContract(name, field, create, update, get)...)
	}
	return report
}

func validateFieldContract(name, field string, create, update, get *base.Schema) issue.Report {
	cp := requestContractProperty(create, field)
	up := requestContractProperty(update, field)
	gp := get.Properties.GetOrZero(field)
	location := "components.schemas." + name + "." + field
	if _, err := Classify(cp != nil, up != nil, gp != nil); err != nil {
		return issue.Report{{Code: "FIELD_LIFECYCLE_UNSUPPORTED", Location: location, Message: fmt.Sprintf("The field locations are Create=%t, Update=%t, Get=%t.", cp != nil, up != nil, gp != nil), Remediation: "Use a managed, immutable, or computed field lifecycle."}}
	}
	report := fieldTypeIssues(location, gp, cp, up)
	report = append(report, fieldPresenceIssue(location+".create", field, create, cp)...)
	report = append(report, fieldPresenceIssue(location+".update", field, update, up)...)
	report = append(report, nestedReadOnlyIssues(location+".create", cp, map[*base.Schema]bool{})...)
	report = append(report, nestedReadOnlyIssues(location+".update", up, map[*base.Schema]bool{})...)
	return report
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
		child := schema.Properties.GetOrZero(name)
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
	if proxy == nil || slices.Contains(parent.Required, field) {
		return nil
	}
	schema, err := schemaOf(proxy)
	if err != nil {
		return nil
	}
	presence := schema.Extensions.GetOrZero(extPresence)
	if presence != nil && presence.Value == "true" {
		return nil
	}
	code := "FIELD_PRESENCE_UNKNOWN"
	if slices.Equal(schema.Type, []string{"array"}) || slices.Equal(schema.Type, []string{"object"}) && schema.AdditionalProperties != nil {
		code = "COLLECTION_NULL_EMPTY_AMBIGUOUS"
	}
	return issue.Report{{Code: code, Location: location, Message: "The optional request field does not state whether omission differs from an explicit zero or empty value.", Remediation: "Add x-coralogix-presence: true to the source API contract."}}
}

func requestContractProperty(parent *base.Schema, name string) *base.SchemaProxy {
	proxy := parent.Properties.GetOrZero(name)
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
		proxy := s.Properties.GetOrZero(field)
		if unwrapRef(proxy) == componentPrefix+name {
			return proxy
		}
	}
	return nil
}

func validateBuiltResource(r *Resource) issue.Report {
	var report issue.Report
	if !r.Replace && r.UpdateMaskPattern == "" {
		report = append(report, issue.Issue{Code: "UPDATE_MASK_CONTRACT_MISSING", Location: "paths.update." + r.Update.OperationID, Message: "The PATCH update mask has no pattern that defines accepted mask paths.", Remediation: "Add the authoritative update-mask path pattern to the source API contract."})
	}
	return report
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
	seen := map[string]string{}
	for _, field := range t.Fields {
		name := terraformName(field.Name)
		if previous, ok := seen[name]; ok && previous != field.Name {
			report = append(report, issue.Issue{Code: "TERRAFORM_NAME_COLLISION", Location: location, Message: fmt.Sprintf("Fields %q and %q both normalize to %q.", previous, field.Name, name), Remediation: "Rename a source API field so every Terraform name is unique."})
		} else {
			seen[name] = field.Name
		}
		report = append(report, nameCollisions(location+"."+field.Name, field.Type)...)
	}
	return report
}

func terraformName(name string) string {
	var out []rune
	for i, r := range name {
		if unicode.IsUpper(r) && i > 0 {
			out = append(out, '_')
		}
		out = append(out, unicode.ToLower(r))
	}
	return string(out)
}

func schemaIssue(err error) issue.Issue {
	message := err.Error()
	code := "UNION_SHAPE_UNSUPPORTED"
	switch {
	case strings.Contains(message, "recursive schema"):
		code = "SCHEMA_RECURSIVE"
	case strings.Contains(message, "additionalProperties") || strings.Contains(message, "map without"):
		code = "MAP_VALUE_TYPE_UNSUPPORTED"
	case strings.Contains(message, "uniqueItems") || strings.Contains(message, extCollection):
		code = "COLLECTION_SEMANTICS_UNKNOWN"
	case strings.Contains(message, "nullable"):
		code = "FIELD_NULLABILITY_AMBIGUOUS"
	case strings.Contains(message, "$ref") || strings.Contains(message, "reference"):
		code = "REFERENCE_UNRESOLVED"
	}
	return issue.Issue{Code: code, Location: firstLocation(message), Message: message, Remediation: "Correct the source OpenAPI contract to use a supported, deterministic schema shape."}
}

func buildIssue(err error) issue.Issue {
	message := err.Error()
	code := "RESOURCE_LIFECYCLE_INCOMPLETE"
	switch {
	case strings.Contains(message, "update mask") || strings.Contains(message, updateMaskField):
		code = "CLEAR_BEHAVIOR_UNKNOWN"
	case strings.Contains(message, "type differs"):
		code = "FIELD_TYPE_INCONSISTENT"
	case strings.Contains(message, "unsupported field location"):
		code = "FIELD_LIFECYCLE_UNSUPPORTED"
	}
	return issue.Issue{Code: code, Location: firstLocation(message), Message: message, Remediation: "Correct the source API contract so the complete resource lifecycle is deterministic."}
}

func firstLocation(message string) string {
	if before, _, ok := strings.Cut(message, ":"); ok {
		return strings.TrimSpace(before)
	}
	return "openapi"
}
