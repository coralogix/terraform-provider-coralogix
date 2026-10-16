package generator

import (
	"fmt"
	"slices"
	"strings"
	"unicode"

	"github.com/coralogix/coralogix-management-sdk/iac/issue"
	"github.com/coralogix/coralogix-management-sdk/iac/model"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

// providerModulePath is the Go module of the provider that the generated code is part of.
const providerModulePath = "github.com/coralogix/terraform-provider-coralogix"

// terraformInt64SafeDecimalDigits: a decimal uint64 with at most 18 digits always fits in
// Terraform Int64.
const terraformInt64SafeDecimalDigits int64 = 18

// terraformContractIssues runs the contract checks of the iac model and the checks that only
// Terraform needs, and returns one sorted report. The Terraform checks read the type that the
// contract checks survey, so both see the same resource.
func terraformContractIssues(doc *v3.Document, component string, ids model.OperationIDs, p model.Policy, file *overrides.File) issue.Report {
	report := omittedMaskIssues(model.ValidateWithPolicy(doc, component, ids, p), file)
	if t, _ := model.SurveyWithPolicy(doc, component, p); t != nil {
		report = append(report, terraformNameCollisions(p, "components.schemas."+component, t)...)
		report = append(report, uint64RangeIssues(component, t)...)
	}
	return report.Normalize()
}

// resolveResource maps an explicit Terraform resource name or exact component
// name to one unique OpenAPI component. It never selects among multiple matches.
func resolveResource(doc *v3.Document, selection string) (string, issue.Report) {
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

// terraformNameCollisions reports the fields of an object that have the same Terraform name.
func terraformNameCollisions(p model.Policy, location string, t *model.Type) issue.Report {
	var report issue.Report
	if t == nil {
		return nil
	}
	if t.Kind == model.List || t.Kind == model.Set || t.Kind == model.Map {
		return terraformNameCollisions(p, location+"[]", t.Elem)
	}
	if t.Kind != model.Object && t.Kind != model.OneOf {
		return nil
	}
	terraformNames := map[string]string{}
	for _, field := range t.Fields {
		if lists(p.Skip, t.Schema, field.Name) || lists(p.FixedRequest, t.Schema, field.Name) {
			continue // a skipped field or a fixed request value has no Terraform attribute
		}
		terraformName := terraformName(p, t.Schema, field.Name)
		if previous, ok := terraformNames[terraformName]; ok && previous != field.Name {
			report = append(report, issue.Issue{Code: "TERRAFORM_NAME_COLLISION", Location: location, Message: fmt.Sprintf("Fields %q and %q both normalize to %q.", previous, field.Name, terraformName), Remediation: "Rename a source API field, or give one a name line, so every Terraform name is unique."})
		} else {
			terraformNames[terraformName] = field.Name
		}
		report = append(report, terraformNameCollisions(p, location+"."+field.Name, field.Type)...)
	}
	return report
}

// uint64RangeIssues reports a uint64 that may not fit Terraform Int64: it must travel as a
// string of at most 18 decimal digits. path is the path of the type in the spec, as the model
// names it.
func uint64RangeIssues(path string, t *model.Type) issue.Report {
	if t == nil {
		return nil
	}
	switch t.Kind {
	case model.List, model.Set:
		return uint64RangeIssues(path+"[]", t.Elem)
	case model.Map:
		return uint64RangeIssues(path+"{}", t.Elem)
	case model.Object, model.OneOf:
		var report issue.Report
		for _, field := range t.Fields {
			report = append(report, uint64RangeIssues(path+"."+field.Name, field.Type)...)
		}
		return report
	}
	if t.Format != "uint64" || t.WireString && t.MaxLength != nil && *t.MaxLength <= terraformInt64SafeDecimalDigits {
		return nil
	}
	return issue.Report{{
		Code:        "UINT64_RANGE_UNSUPPORTED",
		Location:    path,
		Message:     fmt.Sprintf("%s: uint64 needs string type and maxLength at most %d to fit Terraform Int64", path, terraformInt64SafeDecimalDigits),
		Remediation: "Correct the source OpenAPI contract to use a supported, deterministic schema shape.",
	}}
}

// terraformName returns the Terraform attribute name of field of component: the name of a
// name line, or the name derived from the JSON name.
func terraformName(p model.Policy, component, field string) string {
	if n, ok := p.Names[component+"."+field]; ok {
		return n
	}
	return TerraformName(field)
}

// terraformType returns the released Terraform type of field of component, or "" for the type of
// the contract.
func terraformType(p model.Policy, component, field string) string {
	if p.AsInt64(component, field) {
		return "Int64"
	}
	return ""
}

// lists reports whether list, a list of "Component.field", has field of component.
func lists(list []string, component, field string) bool {
	return component != "" && slices.Contains(list, component+"."+field)
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

// omittedMaskIssues drops the contract issues about the update mask paths when the resource sends
// no update mask (updateMask: omit). The body then holds the whole value of each field it sends,
// a changed oneOf arm included, so no mask path has to name the arm. buildMask skips the path
// checks for the same reason.
func omittedMaskIssues(report issue.Report, file *overrides.File) issue.Report {
	if file == nil || file.UpdateMask != overrides.UpdateMaskOmit {
		return report
	}
	var out issue.Report
	for _, i := range report {
		if i.Code != "UPDATE_MASK_NESTED_ONEOF_UNSUPPORTED" {
			out = append(out, i)
		}
	}
	return out
}
