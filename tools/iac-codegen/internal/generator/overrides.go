package generator

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/coralogix/coralogix-management-sdk/iac/issue"
	"github.com/coralogix/coralogix-management-sdk/iac/model"
	iacoverrides "github.com/coralogix/coralogix-management-sdk/iac/overrides"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"github.com/pb33f/libopenapi/orderedmap"
	"go.yaml.in/yaml/v4"
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
		if problem := lineProblem(doc, line, component, file); problem != nil {
			report = append(report, issue.Issue{
				Code:        cmp.Or(problem.code, "OVERRIDE_UNUSED"),
				Location:    overrides.FileName + ":" + line.String(),
				Message:     problem.message,
				Remediation: problem.remediation(line),
			})
		}
	}
	report = append(report, dataSourceIssues(file)...)
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

// lineProblem checks one line. resource is the component of the resource response.
func lineProblem(doc *v3.Document, line overrides.Line, resource string, file *overrides.File) *lineIssue {
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
	if line.Kind == overrides.KindField && line.Component == file.Resource {
		if schema, err = withInlinedFields(schema, file); err != nil {
			return wholeLine(err)
		}
	}
	switch line.Kind {
	case overrides.KindEnum:
		return enumLineProblem(schema, line)
	case overrides.KindEmptyRequired:
		// A missing required list means that no field is required.
		return wholeLine(fmt.Errorf("component %q: a missing required list already means that no field is required, so no override is needed", line.Component))
	case overrides.KindField:
		return fieldLineProblem(schema, line, line.Component != resource)
	case overrides.KindGroup:
		for _, field := range line.GroupFields {
			if schema.Properties == nil || schema.Properties.GetOrZero(field) == nil {
				return wholeLine(fmt.Errorf("component %q has no field %q", line.Component, field))
			}
		}
	}
	return nil
}

// withInlinedFields returns the resource component with the fields of its inlined objects: they
// are top-level attributes, so their lines are lines of the resource component. The model checks
// that an inlined field is an object.
func withInlinedFields(schema *base.Schema, file *overrides.File) (*base.Schema, error) {
	var inlined []string
	for _, line := range file.Lines() {
		if line.Kind == overrides.KindField && line.Component == file.Resource && line.Inline {
			inlined = append(inlined, line.Field)
		}
	}
	if len(inlined) == 0 || schema.Properties == nil {
		return schema, nil
	}
	out := *schema
	out.Properties = orderedmap.New[string, *base.SchemaProxy]()
	out.Required = slices.Clone(schema.Required)
	for name, proxy := range schema.Properties.FromOldest() {
		out.Properties.Set(name, proxy)
	}
	for _, name := range inlined {
		proxy := schema.Properties.GetOrZero(name)
		if proxy == nil {
			continue // the line of the inlined field reports it
		}
		object, err := proxy.BuildSchema()
		if err != nil {
			return nil, fmt.Errorf("field %q: %w", name, err)
		}
		if len(object.AllOf) == 1 {
			if object, err = object.AllOf[0].BuildSchema(); err != nil {
				return nil, fmt.Errorf("field %q: %w", name, err)
			}
		}
		if object.Properties == nil {
			continue
		}
		for child, childProxy := range object.Properties.FromOldest() {
			if out.Properties.GetOrZero(child) == nil {
				out.Properties.Set(child, childProxy)
			}
		}
		out.Required = append(out.Required, object.Required...)
	}
	return &out, nil
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
	if line.EnumZero != "" && !hasZeroValue(contract) {
		return &lineIssue{
			code:    "ENUM_ZERO_INVALID",
			message: fmt.Sprintf("enum %q starts with %s, a business value, not a *_UNSPECIFIED sentinel", line.Component, contract[0]),
			fix:     "Remove zero and put that value in values. A missing field reads back as that first value, not as an enum zero.",
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
// shared is true for a nested component, which the requests and the response share, so its
// required list binds the requests too.
func fieldLineProblem(schema *base.Schema, line overrides.Line, shared bool) *lineIssue {
	if schema.Properties == nil || schema.Properties.GetOrZero(line.Field) == nil {
		return wholeLine(fmt.Errorf("component %q has no field %q", line.Component, line.Field))
	}
	if problem := equalityProblem(schema, line); problem != nil {
		return problem
	}
	var stale []string
	if line.ReadOnly && iacoverrides.ContractReadOnly(schema, line.Field) {
		stale = append(stale, "readOnly")
	}
	if line.Required && slices.Contains(schema.Required, line.Field) && !(shared && requiredLineStatesPresence(schema, line.Field)) {
		stale = append(stale, "required")
	}
	if line.Optional && shared && !slices.Contains(schema.Required, line.Field) {
		// The resource component has request components of its own: the contract check reports
		// an optional line that relaxes none of them.
		stale = append(stale, "optional")
	}
	if line.Collection != "" {
		if problem := collectionProblem(schema, line); problem != nil {
			return problem
		}
	}
	if line.Optional {
		if problem := optionalProblem(schema, line); problem != nil {
			return problem
		}
	}
	if line.Name != "" {
		if problem := nameProblem(schema, line); problem != nil {
			return problem
		}
	}
	if len(line.RequestValueBy) != 0 {
		if problem := requestValueByProblem(schema, line); problem != nil {
			return problem
		}
	}
	if line.KeyedBy != nil {
		if problem := keyedByProblem(schema, line); problem != nil {
			return problem
		}
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

// requiredLineStatesPresence reports whether a required key on a field that the contract already
// requires is what states its presence: a bool or number whose zero value is valid and that has
// no presence marker and no server default. Terraform requires the attribute and sends it every time, so omission
// never happens, but the contract cannot say so without a presence-tracking scalar.
func requiredLineStatesPresence(schema *base.Schema, field string) bool {
	built, err := schema.Properties.GetOrZero(field).BuildSchema()
	if err != nil || len(built.Type) != 1 || len(built.Enum) != 0 || built.Default != nil {
		return false
	}
	if presence := built.Extensions.GetOrZero("x-coralogix-presence"); presence != nil && presence.Value == "true" {
		return false
	}
	switch built.Type[0] {
	case "boolean":
		return true
	case "integer", "number":
		return (built.Minimum == nil || *built.Minimum <= 0) && (built.Maximum == nil || *built.Maximum >= 0)
	}
	return false
}

func hasNoRequiredList(schema *base.Schema) bool {
	return schema.GoLow() == nil || schema.GoLow().Required.IsEmpty()
}

// requestValueByProblem checks a requestValueBy line. The field is a string, each key is an
// object field of the component, and each value is a value of the enum of the field when the
// field is an enum.
func requestValueByProblem(schema *base.Schema, line overrides.Line) *lineIssue {
	built, err := schema.Properties.GetOrZero(line.Field).BuildSchema()
	if err != nil {
		return wholeLine(fmt.Errorf("field %q: %w", line.Field, err))
	}
	// The renderer sends each value as a string.
	if !slices.Equal(built.Type, []string{"string"}) {
		return wholeLine(fmt.Errorf("requestValueBy needs a string or a string enum, but %q is %v", line.Field, built.Type))
	}
	for _, field := range slices.Sorted(maps.Keys(line.RequestValueBy)) {
		proxy := schema.Properties.GetOrZero(field)
		if proxy == nil {
			return wholeLine(fmt.Errorf("requestValueBy names %q, which component %q does not have", field, line.Component))
		}
		s, err := proxy.BuildSchema()
		if err != nil {
			return wholeLine(fmt.Errorf("field %q: %w", field, err))
		}
		// The renderer tests the field for nil, so it must be an object, also behind a $ref.
		if !slices.Equal(s.Type, []string{"object"}) && len(s.AllOf) == 0 && (len(s.Type) != 0 || s.Properties == nil) {
			return wholeLine(fmt.Errorf("requestValueBy names %q, which is not an object", field))
		}
		value := line.RequestValueBy[field]
		if len(built.Enum) != 0 && !slices.ContainsFunc(built.Enum, func(n *yaml.Node) bool { return n.Value == value }) {
			return wholeLine(fmt.Errorf("requestValueBy sends %q, which the enum of %q does not have", value, line.Field))
		}
	}
	return nil
}

// keyedByProblem checks a keyedBy line. The field is a list of objects. The key is a required
// string property of the object. The value, when the line names one, is a required string
// property, and the object has no other property.
func keyedByProblem(schema *base.Schema, line overrides.Line) *lineIssue {
	built, err := schema.Properties.GetOrZero(line.Field).BuildSchema()
	if err != nil {
		return wholeLine(fmt.Errorf("field %q: %w", line.Field, err))
	}
	if !slices.Equal(built.Type, []string{"array"}) || built.Items == nil || built.Items.A == nil {
		return wholeLine(fmt.Errorf("keyedBy needs a list of objects, but %q is %v", line.Field, built.Type))
	}
	item, err := built.Items.A.BuildSchema()
	if err != nil {
		return wholeLine(fmt.Errorf("field %q: %w", line.Field, err))
	}
	if item.Properties == nil {
		return wholeLine(fmt.Errorf("keyedBy needs a list of objects, but the items of %q have no properties", line.Field))
	}
	isString := func(name string) bool {
		proxy := item.Properties.GetOrZero(name)
		if proxy == nil {
			return false
		}
		s, err := proxy.BuildSchema()
		return err == nil && slices.Equal(s.Type, []string{"string"}) && len(s.Enum) == 0
	}
	key, value := line.KeyedBy.Key, line.KeyedBy.Value
	if line.KeyedBy.Object {
		return keyedObjectProblem(item, line, isString)
	}
	if !isString(key) || !slices.Contains(item.Required, key) {
		return wholeLine(fmt.Errorf("keyedBy: the key %q is not a required string property of the items of %q", key, line.Field))
	}
	if value != "" && (!isString(value) || item.Properties.Len() != 2) {
		return wholeLine(fmt.Errorf("keyedBy: the value %q must be a string property, and the items of %q must have only the key and the value", value, line.Field))
	}
	if value != "" && !slices.Contains(item.Required, value) {
		// A map of strings has no place for an item without a value.
		return wholeLine(fmt.Errorf("keyedBy: the value %q is not a required property of the items of %q", value, line.Field))
	}
	return nil
}

// keyedObjectProblem checks a keyedBy object line. The key is a required enum property of the
// item. Each arm names an enum value and a property of the item. keyAs is a string property.
func keyedObjectProblem(item *base.Schema, line overrides.Line, isString func(string) bool) *lineIssue {
	k := line.KeyedBy
	proxy := item.Properties.GetOrZero(k.Key)
	var values []string
	if proxy != nil {
		if s, err := proxy.BuildSchema(); err == nil {
			for _, n := range s.Enum {
				values = append(values, n.Value)
			}
		}
	}
	if len(values) == 0 || !slices.Contains(item.Required, k.Key) {
		return wholeLine(fmt.Errorf("keyedBy: the key %q is not a required enum property of the items of %q", k.Key, line.Field))
	}
	// The object has an attribute per value that the enums line of the key accepts, so an arm of
	// another value, or a key without an enums line, would change nothing.
	enum := ""
	if proxy.IsReference() {
		enum = strings.TrimPrefix(proxy.GetReference(), "#/components/schemas/")
	}
	accepted, ok := line.Accepted[enum]
	if !ok || len(accepted) == 0 {
		return wholeLine(fmt.Errorf("keyedBy: the key %q needs an enums line with values for its enum %q", k.Key, enum))
	}
	// Each value becomes an attribute and a Go field, so two values must not normalize to one name.
	tfNames, goNames := map[string]string{}, map[string]string{}
	for _, v := range slices.Sorted(maps.Keys(accepted)) {
		tf, gn := TerraformName(accepted[v]), model.GoName(accepted[v])
		if other, ok := tfNames[tf]; ok {
			return wholeLine(fmt.Errorf("keyedBy: values %q and %q both become the attribute %q", other, v, tf))
		}
		if other, ok := goNames[gn]; ok {
			return wholeLine(fmt.Errorf("keyedBy: values %q and %q both become the Go field %q", other, v, gn))
		}
		tfNames[tf], goNames[gn] = v, v
	}
	groups, err := oneOfArmGroups(item)
	if err != nil {
		return wholeLine(fmt.Errorf("field %q: %w", line.Field, err))
	}
	var groupArms []string
	for _, g := range groups {
		groupArms = append(groupArms, g.arms...)
	}
	// A value without an arm gets an attribute with no arm of the union, so a union that
	// requires an arm needs one for every value that the resource accepts.
	for _, g := range groups {
		if g.allowNone {
			continue
		}
		for _, v := range slices.Sorted(maps.Keys(accepted)) {
			if !slices.Contains(g.arms, k.Arms[v]) {
				return wholeLine(fmt.Errorf("keyedBy: %q has no arm in arms, but the items of %q require one of %v", v, line.Field, g.arms))
			}
		}
	}
	for _, v := range slices.Sorted(maps.Keys(k.Arms)) {
		if _, ok := accepted[v]; !ok {
			return wholeLine(fmt.Errorf("keyedBy: arms names %q, which the enums line of %q does not accept", v, k.Key))
		}
		if item.Properties.GetOrZero(k.Arms[v]) == nil {
			return wholeLine(fmt.Errorf("keyedBy: arms names %q, which the items of %q do not have", k.Arms[v], line.Field))
		}
		// The keyed object drops every oneOf arm but the one of its value, so an arm must be one.
		if !slices.Contains(groupArms, k.Arms[v]) {
			return wholeLine(fmt.Errorf("keyedBy: arms names %q, which is not a oneOf arm of the items of %q", k.Arms[v], line.Field))
		}
	}
	if k.KeyAs != "" && !isString(k.KeyAs) {
		return wholeLine(fmt.Errorf("keyedBy: keyAs %q is not a string property of the items of %q", k.KeyAs, line.Field))
	}
	return nil
}

// oneOfArmGroup is a oneOf group of a contract schema: its arms, and whether it allows no arm.
type oneOfArmGroup struct {
	arms      []string
	allowNone bool
}

// oneOfArmGroups returns the oneOf groups of s: its own oneOf, and the oneOf of each allOf
// entry. An arm is the one required property of a oneOf entry; the "no arm" entry has none, and
// its presence lets the group have no arm.
func oneOfArmGroups(s *base.Schema) ([]oneOfArmGroup, error) {
	schemas := []*base.Schema{s}
	for _, proxy := range s.AllOf {
		e, err := proxy.BuildSchema()
		if err != nil {
			return nil, err
		}
		schemas = append(schemas, e)
	}
	var groups []oneOfArmGroup
	for _, e := range schemas {
		if len(e.OneOf) == 0 {
			continue
		}
		var g oneOfArmGroup
		for _, proxy := range e.OneOf {
			entry, err := proxy.BuildSchema()
			if err != nil {
				return nil, err
			}
			switch {
			case entry.Not != nil:
				g.allowNone = true
			case len(entry.Required) == 1:
				g.arms = append(g.arms, entry.Required[0])
			}
		}
		groups = append(groups, g)
	}
	return groups, nil
}

// optionalProblem checks an optional line. The SDK holds a field that the contract requires as a
// value, which Create would send as its zero value when Terraform leaves the field out. Only a
// list or a map is nil when it is left out, so only they can be optional, or a field whose line
// also has a static default: Terraform then always has the value to send.
func optionalProblem(schema *base.Schema, line overrides.Line) *lineIssue {
	built, err := schema.Properties.GetOrZero(line.Field).BuildSchema()
	if err != nil {
		return wholeLine(fmt.Errorf("field %q: %w", line.Field, err))
	}
	isMap := slices.Equal(built.Type, []string{"object"}) && built.AdditionalProperties != nil && built.AdditionalProperties.A != nil
	if !slices.Equal(built.Type, []string{"array"}) && !isMap && !slices.Contains(line.Keys, "default") {
		return wholeLine(fmt.Errorf("optional needs a list or a map, or a default on the line, but %q is %v: the SDK holds a required value, which Create sends as its zero value", line.Field, built.Type))
	}
	return nil
}

// nameProblem checks a name line. The name must differ from the name that the JSON name
// gives, and from the name of every other field of the component that has an attribute.
func nameProblem(schema *base.Schema, line overrides.Line) *lineIssue {
	if line.Name == TerraformName(line.Field) {
		return &lineIssue{message: fmt.Sprintf("%q is already the Terraform name of %q", line.Name, line.Field), stale: []string{"name"}}
	}
	for field := range schema.Properties.KeysFromOldest() {
		if field != line.Field && TerraformName(field) == line.Name && !slices.Contains(line.NoAttribute, field) {
			return wholeLine(fmt.Errorf("name %q is the Terraform name of field %q", line.Name, field))
		}
	}
	return nil
}

// collectionProblem checks a collection: set line. It needs a list that the contract does not
// already mark as a set.
func collectionProblem(schema *base.Schema, line overrides.Line) *lineIssue {
	built, err := schema.Properties.GetOrZero(line.Field).BuildSchema()
	if err != nil {
		return wholeLine(fmt.Errorf("field %q: %w", line.Field, err))
	}
	if !slices.Equal(built.Type, []string{"array"}) {
		return wholeLine(fmt.Errorf("collection: set needs a list, but %q is %v", line.Field, built.Type))
	}
	if n := built.Extensions.GetOrZero("x-coralogix-collection"); n != nil && n.Value == "set" {
		return &lineIssue{message: fmt.Sprintf("the contract already states that %q is a set", line.Field), stale: []string{"collection"}}
	}
	return nil
}
