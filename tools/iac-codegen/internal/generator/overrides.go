package generator

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/issue"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

// overridesPath returns the file to read: the explicit path, else the file in the
// output directory when it exists, else "".
func overridesPath(options Options) string {
	if options.OverridesPath != "" {
		return options.OverridesPath
	}
	if options.OutputDir == "" {
		return ""
	}
	candidate := filepath.Join(options.OutputDir, overrides.FileName)
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate
	}
	return ""
}

// keepOverrides adds the behavior-overrides file of the output directory to the files
// to publish. Publish replaces the whole directory, so the file must go with the new files.
// A file at another path stays where it is.
func keepOverrides(options Options, files map[string][]byte) error {
	path := overridesPath(options)
	if path == "" || filepath.Base(path) != overrides.FileName {
		return nil
	}
	inOutput, err := sameDirectory(filepath.Dir(path), options.OutputDir)
	if err != nil || !inOutput {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read overrides %s: %w", path, err)
	}
	files[overrides.FileName] = data
	return nil
}

func sameDirectory(a, b string) (bool, error) {
	absA, err := filepath.Abs(a)
	if err != nil {
		return false, err
	}
	absB, err := filepath.Abs(b)
	if err != nil {
		return false, err
	}
	return absA == absB, nil
}

// readOverrides reads the file. It returns nil for the empty path: a new resource.
func readOverrides(path string) (*overrides.File, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read overrides %s: %w", path, err)
	}
	file, err := overrides.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("overrides %s: %w", path, err)
	}
	return file, nil
}

// overrideIssues reports every line of the file that does not match the contract.
// A line that matches nothing is an error, so a stale line cannot stay in the file.
func overrideIssues(doc *v3.Document, component string, file *overrides.File) issue.Report {
	var report issue.Report
	if file.Resource != component {
		report = append(report, issue.Issue{
			Code:        "OVERRIDE_RESOURCE_MISMATCH",
			Location:    overrides.FileName,
			Message:     fmt.Sprintf("The file is for resource %q, but the selected resource is %q.", file.Resource, component),
			Remediation: "Use the behavior-overrides file of the selected resource.",
		})
	}
	for _, line := range file.Lines() {
		if problem := lineProblem(doc, line); problem != nil {
			report = append(report, issue.Issue{
				Code:        cmp.Or(problem.code, "OVERRIDE_UNUSED"),
				Location:    overrides.FileName + ":" + line.String(),
				Message:     problem.message,
				Remediation: problem.remediation(line),
			})
		}
	}
	return report.Normalize()
}

// lineIssue says what is wrong with one line of the file. stale names the keys of a field line
// that the contract now states. It is empty when the whole line is wrong.
type lineIssue struct {
	code    string // OVERRIDE_UNUSED when empty
	message string
	stale   []string
	fix     string // what to do, when the issue is not about a stale line
}

func wholeLine(err error) *lineIssue {
	if err == nil {
		return nil
	}
	return &lineIssue{message: err.Error()}
}

// remediation tells what to delete. A field line can set several keys, and the contract can
// state only some of them. The other keys still keep the released behavior.
func (i *lineIssue) remediation(line overrides.Line) string {
	if i.fix != "" {
		return i.fix
	}
	if len(i.stale) == 0 {
		return "Delete the line: the API contract no longer differs here, or fix the name."
	}
	keep := slices.DeleteFunc(slices.Clone(line.Keys), func(key string) bool { return slices.Contains(i.stale, key) })
	if len(keep) == 0 {
		return "Delete the line: the API contract now states everything that it sets."
	}
	return fmt.Sprintf("Delete only the key %s. Keep %s: the contract does not state it.", strings.Join(i.stale, ", "), strings.Join(keep, ", "))
}

func lineProblem(doc *v3.Document, line overrides.Line) *lineIssue {
	if doc.Components == nil || doc.Components.Schemas == nil {
		return wholeLine(errors.New("the API contract has no component schemas"))
	}
	proxy := doc.Components.Schemas.GetOrZero(line.Component)
	if proxy == nil {
		return wholeLine(fmt.Errorf("no component %q", line.Component))
	}
	schema, err := proxy.BuildSchema()
	if err != nil {
		return wholeLine(fmt.Errorf("component %q: %w", line.Component, err))
	}
	switch line.Kind {
	case overrides.KindEnum:
		return enumLineProblem(schema, line)
	case overrides.KindEmptyRequired:
		// A missing required list means that no field is required.
		return wholeLine(fmt.Errorf("component %q: a missing required list already means that no field is required, so no override is needed", line.Component))
	case overrides.KindField:
		return fieldLineProblem(schema, line)
	}
	return nil
}

// enumLineProblem checks an enums line. Each value that the file names must be in the contract,
// and each value of the contract must be accepted or rejected, so a new API value needs a decision.
func enumLineProblem(schema *base.Schema, line overrides.Line) *lineIssue {
	if len(schema.Enum) == 0 {
		return wholeLine(fmt.Errorf("component %q is not an enum", line.Component))
	}
	contract := make([]string, 0, len(schema.Enum))
	for _, n := range schema.Enum {
		contract = append(contract, n.Value)
	}
	for _, value := range slices.Concat(line.EnumValues, line.EnumRejected) {
		if !slices.Contains(contract, value) {
			return wholeLine(fmt.Errorf("enum %q has no value %q", line.Component, value))
		}
	}
	if hasZeroValue(contract) && line.EnumZero == "" {
		return &lineIssue{
			code:    "ENUM_ZERO_UNDECIDED",
			message: fmt.Sprintf("enum %q has the zero value %s, but the line has no zero", line.Component, contract[0]),
			fix:     "Add zero with the Terraform value of the zero value. Without it, a response that has the value fails with Unsupported API value.",
		}
	}
	var undecided []string
	for i, value := range contract {
		zero := i == 0 && hasZeroValue(contract)
		if !zero && !slices.Contains(line.EnumValues, value) && !slices.Contains(line.EnumRejected, value) {
			undecided = append(undecided, value)
		}
	}
	if len(undecided) == 0 {
		return nil
	}
	return &lineIssue{
		code:    "ENUM_VALUE_UNDECIDED",
		message: fmt.Sprintf("the contract of enum %q has values that the file neither accepts nor rejects: %s", line.Component, strings.Join(undecided, ", ")),
		fix:     "Add each value to values (the resource accepts it) or to rejected (it does not).",
	}
}

// hasZeroValue reports whether the first value of the enum is the protobuf zero value.
func hasZeroValue(contract []string) bool {
	return len(contract) > 0 && strings.HasSuffix(contract[0], "_UNSPECIFIED")
}

// fieldLineProblem checks a field line. The field must exist. A readOnly or required key is
// stale when the contract states the same fact.
func fieldLineProblem(schema *base.Schema, line overrides.Line) *lineIssue {
	if schema.Properties == nil || schema.Properties.GetOrZero(line.Field) == nil {
		return wholeLine(fmt.Errorf("component %q has no field %q", line.Component, line.Field))
	}
	if problem := equalityProblem(schema, line); problem != nil {
		return problem
	}
	var stale []string
	if line.ReadOnly && contractReadOnly(schema, line.Field) {
		stale = append(stale, "readOnly")
	}
	if line.Required && slices.Contains(schema.Required, line.Field) {
		stale = append(stale, "required")
	}
	if len(stale) == 0 {
		return nil
	}
	return &lineIssue{
		message: fmt.Sprintf("the contract already states that %q is %s", line.Field, strings.Join(stale, " and ")),
		stale:   stale,
	}
}

// equalityProblem checks the equality key of a field line. The key compares a string that the
// user writes as a YAML or JSON document. On another kind of field, or on a field that only the
// server sets, the generated plan modifier and flatten would have nothing to compare.
func equalityProblem(schema *base.Schema, line overrides.Line) *lineIssue {
	if line.Equality == "" {
		return nil
	}
	built, err := schema.Properties.GetOrZero(line.Field).BuildSchema()
	if err != nil {
		return wholeLine(fmt.Errorf("field %q: %w", line.Field, err))
	}
	if !slices.Equal(built.Type, []string{"string"}) || len(built.Enum) != 0 || built.Format != "" {
		return &lineIssue{
			code:    "OVERRIDE_EQUALITY_NOT_STRING",
			message: fmt.Sprintf("equality: %s compares a string document, but %q is not a plain string (type %v, format %q, %d enum values)", line.Equality, line.Field, built.Type, built.Format, len(built.Enum)),
			fix:     "Delete the equality key, or set it on a string field that holds a YAML or JSON document.",
		}
	}
	if line.ReadOnly || built.ReadOnly != nil && *built.ReadOnly {
		return &lineIssue{
			code:    "OVERRIDE_EQUALITY_COMPUTED",
			message: fmt.Sprintf("equality: %s keeps the text that the user writes, but only the server sets %q", line.Equality, line.Field),
			fix:     "Delete the equality key: Terraform never compares a configured value of a computed-only field.",
		}
	}
	return nil
}

// unwrapUsageIssues reports each unwrap line that decided no place of the resource: a component of
// the list that no field holds, an unwrap: true line that the model rejected first, or an
// unwrap: false line on a field whose component the list does not name. It also reports a field
// line on the property of a wrapper that collapses in every place: Terraform has no attribute
// for that property, so the line would change nothing.
func unwrapUsageIssues(r *model.Resource, file *overrides.File) issue.Report {
	if file == nil {
		return nil
	}
	var report issue.Report
	for _, name := range file.Unwrap {
		if !r.UnwrapUsed["unwrap."+name] {
			report = append(report, issue.Issue{
				Code:        "OVERRIDE_UNUSED",
				Location:    overrides.FileName + ":unwrap." + name,
				Message:     fmt.Sprintf("No field of the resource holds %s in a place without an unwrap key.", name),
				Remediation: "Delete the entry from the unwrap list, or fix the component name.",
			})
		}
	}
	for _, line := range file.Lines() {
		if line.Kind != overrides.KindField {
			continue
		}
		if hidden := hiddenKeys(r, line); len(hidden) != 0 {
			report = append(report, issue.Issue{
				Code:        "OVERRIDE_UNUSED",
				Location:    overrides.FileName + ":" + line.String(),
				Message:     fmt.Sprintf("%s collapses in every place, so Terraform has no %s attribute, and %s change nothing.", line.Component, line.Field, strings.Join(hidden, ", ")),
				Remediation: "Move the keys to the field that holds the wrapper, or delete them.",
			})
		}
		if !slices.Contains(line.Keys, "unwrap") || r.UnwrapUsed[line.Component+"."+line.Field] {
			continue
		}
		message := fmt.Sprintf("unwrap: false keeps the object of %s.%s, but the unwrap list does not name its component.", line.Component, line.Field)
		if *file.Types[line.Component].Fields[line.Field].Unwrap {
			message = fmt.Sprintf("unwrap: true names %s.%s, but the resource does not manage that field.", line.Component, line.Field)
		}
		report = append(report, issue.Issue{
			Code:        "OVERRIDE_UNUSED",
			Location:    overrides.FileName + ":" + line.String() + ".unwrap",
			Message:     message,
			Remediation: "Delete the unwrap key.",
		})
	}
	return report.Normalize()
}

// hiddenKeys returns the keys of a field line on the property of a wrapper that collapses in
// every place. An unwrap key still decides the wrapper inside that property.
func hiddenKeys(r *model.Resource, line overrides.Line) []string {
	if !r.HiddenWrappers[line.Component] {
		return nil
	}
	return slices.DeleteFunc(slices.Clone(line.Keys), func(key string) bool { return key == "unwrap" })
}

func contractReadOnly(schema *base.Schema, field string) bool {
	built, err := schema.Properties.GetOrZero(field).BuildSchema()
	return err == nil && built.ReadOnly != nil && *built.ReadOnly
}

func hasNoRequiredList(schema *base.Schema) bool {
	return schema.GoLow() == nil || schema.GoLow().Required.IsEmpty()
}
