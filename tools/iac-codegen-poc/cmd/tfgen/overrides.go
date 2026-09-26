package main

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/model"
)

// overrides make the type mode write the Terraform schema of an existing
// handwritten resource, so that switching the resource to the generated code
// changes no user configuration and no state (D21). They are keyed by API
// component and API field, so one line covers every place that uses the
// type. Example:
//
//	wideNumbers: true
//	types:
//	  GlobalRouter:
//	    id: {computed: true, useStateForUnknown: true}
//	    createTime: {skip: true}
//	enums:
//	  EntityType: {terraformNames: true, zero: unspecified}
type overrides struct {
	// WideNumbers writes int32 as Int64 and float as Float64, as most
	// handwritten resources do. The SDK keeps its types.
	WideNumbers bool                                `yaml:"wideNumbers"`
	Types       map[string]map[string]fieldOverride `yaml:"types"`
	Enums       map[string]enumOverride             `yaml:"enums"`
}

// fieldOverride changes one API field. A nil flag keeps the generated value.
type fieldOverride struct {
	Name     string `yaml:"name"` // the Terraform attribute name
	Skip     bool   `yaml:"skip"` // not in the schema, the model, or requests
	Required *bool  `yaml:"required"`
	Optional *bool  `yaml:"optional"`
	Computed *bool  `yaml:"computed"`
	// Default is the Terraform default, as the YAML text of the value. It
	// makes the attribute Optional and Computed.
	Default            *string `yaml:"default"`
	UseStateForUnknown bool    `yaml:"useStateForUnknown"`
	RequiresReplace    bool    `yaml:"requiresReplace"`
	Set                bool    `yaml:"set"` // a list is a Terraform set
	// DeprecationMessage makes Terraform warn when a configuration sets the
	// attribute.
	DeprecationMessage string `yaml:"deprecationMessage"`
	// MissingAsZero reads a value that the API response does not have as
	// the zero value: "", false, 0, or an empty list or map. Handwritten
	// resources often do that (types.StringValue(v.GetName())).
	MissingAsZero bool `yaml:"missingAsZero"`
	// EmptyAsNull reads an empty list, map, or object (no fields set) as
	// null.
	EmptyAsNull bool `yaml:"emptyAsNull"`
	// ReadOnly marks a field that the server sets and the spec does not mark
	// readOnly (F46): the attribute is Computed only, and expand never
	// sends it.
	ReadOnly bool `yaml:"readOnly"`
}

// enumOverride changes the Terraform values of one API enum.
type enumOverride struct {
	// TerraformNames writes the names of the E14 rule instead of the API
	// values: TEXT_ALIGNMENT_LEFT → "left".
	TerraformNames bool `yaml:"terraformNames"`
	// Values are the names that differ from the rule, by API value.
	Values map[string]string `yaml:"values"`
	// Zero is the name of the value that only means "not set". Without it,
	// that value is null in Terraform.
	Zero string `yaml:"zero"`
}

// overridesFlag is the flag of the overrides file.
const overridesFlag = "--overrides"

// loadOverrides reads an overrides file. Unknown keys are errors.
func loadOverrides(path string) (*overrides, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var o overrides
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(true)
	if err := dec.Decode(&o); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &o, nil
}

// field returns the override of the API field name of the component schema.
func (o *overrides) field(schema, name string) fieldOverride {
	if o == nil {
		return fieldOverride{}
	}
	return o.Types[schema][name]
}

// effective returns the override of the field f of the component schema,
// with readOnly also set when the spec marks the field readOnly.
func (o *overrides) effective(schema string, f *model.Field) fieldOverride {
	ov := o.field(schema, f.Name)
	if o != nil && f.Attrs.ReadOnly {
		ov.ReadOnly = true
	}
	return ov
}

// tfName is the Terraform name of the API field name of the component
// schema.
func (o *overrides) tfName(schema, name string) string {
	if n := o.field(schema, name).Name; n != "" {
		return n
	}
	return tfName(name)
}

// fields returns the fields of t that are not skipped.
func (o *overrides) fields(t *model.Type) []*model.Field {
	var out []*model.Field
	for _, f := range t.Fields {
		if !o.field(t.Schema, f.Name).Skip {
			out = append(out, f)
		}
	}
	return out
}

// fieldType returns the type of the field f of the component schema, with a
// list made a set when the override says so. The model is not changed.
func (o *overrides) fieldType(schema string, f *model.Field) *model.Type {
	if !o.field(schema, f.Name).Set || f.Type.Kind != model.List {
		return f.Type
	}
	t := *f.Type
	t.Kind = model.Set
	return &t
}

// groups returns the oneOf groups of t without the skipped arms.
func (o *overrides) groups(t *model.Type) []model.OneOfGroup {
	var out []model.OneOfGroup
	for _, g := range t.Groups {
		var arms []string
		for _, a := range g.Arms {
			if !o.field(t.Schema, a).Skip {
				arms = append(arms, a)
			}
		}
		if len(arms) != 0 {
			out = append(out, model.OneOfGroup{Arms: arms, AllowNone: g.AllowNone})
		}
	}
	return out
}

// named reports whether the enum schema has Terraform names.
func (o *overrides) named(schema string) bool {
	return o != nil && o.Enums[schema].TerraformNames
}

func (o *overrides) wide() bool { return o != nil && o.WideNumbers }

// widen returns t with the wide Terraform type when WideNumbers is set:
// int32 → int64, float → double. The SDK type does not change, so the
// conversion checks the range.
func (o *overrides) widen(t *model.Type) *model.Type {
	if !o.wide() || !isNarrow(t) {
		return t
	}
	w := *t
	w.Format = map[string]string{"int32": "int64", "float": "double"}[t.Format]
	return &w
}

func isNarrow(t *model.Type) bool {
	return t.Kind == model.Integer && !t.WireString && t.Format == "int32" ||
		t.Kind == model.Number && t.Format == "float"
}

// attrNamePattern is a valid Terraform attribute name.
var attrNamePattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// check fails when an override names a component, field, or enum value that
// the types do not have, or when its flags do not make a valid attribute. A
// stale override is an error, so the file cannot hide an API change.
func (o *overrides) check(roots []*model.Type) error {
	if o == nil {
		return nil
	}
	objects, enums := map[string]*model.Type{}, map[string]*model.Type{}
	for _, t := range roots {
		collectTypes(t, objects, enums)
	}
	var errs []error
	for _, schema := range sortedKeys(o.Types) {
		t, ok := objects[schema]
		if !ok {
			errs = append(errs, fmt.Errorf("overrides: types.%s: no such object in the generated types", schema))
			continue
		}
		errs = append(errs, o.checkObject(t)...)
	}
	for _, schema := range sortedKeys(o.Enums) {
		t, ok := enums[schema]
		if !ok {
			errs = append(errs, fmt.Errorf("overrides: enums.%s: no such enum in the generated types", schema))
			continue
		}
		errs = append(errs, o.checkEnum(t)...)
	}
	return errors.Join(errs...)
}

func (o *overrides) checkObject(t *model.Type) []error {
	var errs []error
	fields := map[string]*model.Field{}
	for _, f := range t.Fields {
		fields[f.Name] = f
	}
	for _, name := range sortedKeys(o.Types[t.Schema]) {
		at := fmt.Sprintf("overrides: types.%s.%s", t.Schema, name)
		f, ok := fields[name]
		if !ok {
			errs = append(errs, fmt.Errorf("%s: no such field", at))
			continue
		}
		if err := checkField(o.Types[t.Schema][name], f); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", at, err))
		}
	}
	if len(o.fields(t)) == 0 {
		errs = append(errs, fmt.Errorf("overrides: types.%s: every field is skipped", t.Schema))
	}
	seen := map[string]string{}
	for _, f := range o.fields(t) {
		n := o.tfName(t.Schema, f.Name)
		if prev, ok := seen[n]; ok {
			errs = append(errs, fmt.Errorf("overrides: types.%s: %s and %s both have the Terraform name %q", t.Schema, prev, f.Name, n))
		}
		seen[n] = f.Name
	}
	return errs
}

// checkField checks one field override. The flags must leave a valid
// attribute: Required alone, Optional with or without Computed, or Computed
// alone.
func checkField(ov fieldOverride, f *model.Field) error {
	if ov.Skip && ov != (fieldOverride{Skip: true}) {
		return errors.New("skip cannot be combined with other overrides")
	}
	if ov.Name != "" && !attrNamePattern.MatchString(ov.Name) {
		return fmt.Errorf("name %q is not a valid Terraform attribute name", ov.Name)
	}
	if ov.Set && f.Type.Kind != model.List {
		return fmt.Errorf("set: the field is a %s, not a list", f.Type.Kind)
	}
	if err := checkRead(ov, f.Type); err != nil {
		return err
	}
	return checkFlags(ov, f.Attrs)
}

// checkRead checks the overrides that change how a response is read.
func checkRead(ov fieldOverride, t *model.Type) error {
	object := t.Kind == model.Object || t.Kind == model.OneOf
	collection := t.Kind == model.List || t.Kind == model.Set || t.Kind == model.Map
	switch {
	case ov.MissingAsZero && ov.EmptyAsNull:
		return errors.New("missingAsZero and emptyAsNull cannot be combined")
	case ov.MissingAsZero && object:
		return errors.New("missingAsZero: an object has no zero value")
	case ov.EmptyAsNull && !object && !collection:
		return fmt.Errorf("emptyAsNull: the field is a %s, not a list, map, or object", t.Kind)
	}
	return nil
}

// checkFlags checks that the flags of a field override leave a valid
// attribute.
func checkFlags(ov fieldOverride, a model.Attrs) error {
	if err := checkReadOnly(ov, a); err != nil {
		return err
	}
	req, opt, comp := flags(ov, a)
	switch {
	case req && (opt || comp):
		return errors.New("a required attribute cannot be optional or computed")
	case !req && !opt && !comp:
		return errors.New("the attribute must be required, optional, or computed")
	case ov.Default != nil && (!opt || !comp):
		return errors.New("a default needs an optional and computed attribute")
	}
	return nil
}

// checkReadOnly fails when a read-only field, by the override or by the
// spec, also has flags or a default.
func checkReadOnly(ov fieldOverride, a model.Attrs) error {
	if ov.Required == nil && ov.Optional == nil && ov.Computed == nil && ov.Default == nil {
		return nil
	}
	switch {
	case ov.ReadOnly:
		return errors.New("readOnly cannot be combined with required, optional, computed, or default")
	case a.ReadOnly:
		return errors.New("the spec marks the field readOnly: it cannot be required, optional, computed, or have a default")
	}
	return nil
}

// flags returns Required, Optional, and Computed of a field: the generated
// value, then the override. A default makes the attribute Optional and
// Computed unless the override sets them.
func flags(ov fieldOverride, a model.Attrs) (req, opt, comp bool) {
	req, opt = a.Required, !a.Required
	if a.Default != nil || ov.Default != nil {
		req, opt, comp = false, true, true
	}
	if ov.Required != nil {
		req = *ov.Required
		if req {
			opt, comp = false, false
		} else if ov.Optional == nil && ov.Computed == nil {
			opt = true
		}
	}
	if ov.Optional != nil {
		opt = *ov.Optional
	}
	if ov.Computed != nil {
		comp = *ov.Computed
	}
	if ov.ReadOnly || a.ReadOnly {
		req, opt, comp = false, false, true
	}
	return req, opt, comp
}

func (o *overrides) checkEnum(t *model.Type) []error {
	ov := o.Enums[t.Schema]
	at := "overrides: enums." + t.Schema
	var errs []error
	if !ov.TerraformNames {
		return []error{fmt.Errorf("%s: values and zero need terraformNames: true", at)}
	}
	for _, v := range sortedKeys(ov.Values) {
		if !slices.Contains(t.Values, v) {
			errs = append(errs, fmt.Errorf("%s.values.%s: no such value", at, v))
		}
	}
	if ov.Zero != "" && t.Zero == "" {
		errs = append(errs, fmt.Errorf("%s.zero: the enum has no value that only means \"not set\"", at))
	}
	return errs
}

// collectTypes adds t and every type inside it, by component schema.
func collectTypes(t *model.Type, objects, enums map[string]*model.Type) {
	switch t.Kind {
	case model.List, model.Set, model.Map:
		collectTypes(t.Elem, objects, enums)
	case model.Enum:
		enums[t.Schema] = t
	case model.Object, model.OneOf:
		if _, ok := objects[t.Schema]; ok || t.Schema == "" {
			return
		}
		objects[t.Schema] = t
		for _, f := range t.Fields {
			collectTypes(f.Type, objects, enums)
		}
	}
}
