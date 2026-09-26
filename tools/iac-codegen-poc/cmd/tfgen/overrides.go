package main

import (
	"errors"
	"fmt"
	"os"
	"reflect"
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
//	unwrap: [LuceneQuery]
type overrides struct {
	// WideNumbers writes int32 as Int64 and float as Float64, as most
	// handwritten resources do. The SDK keeps its types.
	WideNumbers bool                                `yaml:"wideNumbers"`
	Types       map[string]map[string]fieldOverride `yaml:"types"`
	Enums       map[string]enumOverride             `yaml:"enums"`
	// Unwrap lists objects with one field that Terraform shows as that
	// field (see unwrap.go).
	Unwrap []string `yaml:"unwrap"`
	// CustomPackage is the import path of the handwritten converters of the
	// custom fields (see custom.go).
	CustomPackage string `yaml:"customPackage"`
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
	// UseNonNullStateForUnknown keeps a known, non-null state value in the
	// plan (the framework plan modifier of that name).
	UseNonNullStateForUnknown bool `yaml:"useNonNullStateForUnknown"`
	RequiresReplace           bool `yaml:"requiresReplace"`
	Set                       bool `yaml:"set"` // a list is a Terraform set
	// DeprecationMessage makes Terraform warn when a configuration sets the
	// attribute.
	DeprecationMessage string `yaml:"deprecationMessage"`
	// MissingAsZero reads a value that the API response does not have as
	// the zero value: "", false, 0, or an empty list or map. A missing
	// object reads as an empty one, whose fields follow their own read
	// overrides, and expand sends an empty one as a missing one. Handwritten
	// resources often do that (types.StringValue(v.GetName())).
	MissingAsZero bool `yaml:"missingAsZero"`
	// EmptyAsNull reads an empty list, map, or object (no fields set) as
	// null.
	EmptyAsNull bool `yaml:"emptyAsNull"`
	// ReadOnly marks a field that the server sets and the spec does not mark
	// readOnly (F46): the attribute is Computed only, and expand never
	// sends it.
	ReadOnly bool `yaml:"readOnly"`
	// Wrap makes the key a new Terraform object that holds these API fields
	// (see wrapper).
	Wrap []string `yaml:"wrap"`
	// Inline shows the fields of this object field as fields of the parent
	// (see inline.go).
	Inline bool `yaml:"inline"`
	// Sensitive hides the value in plans and logs (a secret).
	Sensitive bool `yaml:"sensitive"`
	// Int64 reads a string field with the pattern of a 64-bit number as an
	// Int64 attribute: ^-?[0-9]+$ is an int64, ^[0-9]+$ a uint64 (D7). The
	// spec has no format (F68).
	Int64 bool `yaml:"int64"`
	// String makes an int64 field a String attribute with the decimal
	// number, as some handwritten resources have (F68).
	String bool `yaml:"string"`
	// Custom keeps the handwritten attribute and converters of a field whose
	// Terraform shape no rule converts (see custom.go).
	Custom *customOverride `yaml:"custom"`
	// NamesArm marks an enum field that names the set arm of the oneOf
	// group of its object. It is not an attribute; expand sets it from the
	// arm (see custom.go).
	NamesArm bool `yaml:"namesArm"`
	// Wide writes one int32 as Int64, or one float as Float64, as
	// wideNumbers does for every field.
	Wide bool `yaml:"wide"`
	// DefaultObject makes the default of an object attribute the object of
	// the defaults of its fields, and null for the fields without one. Like
	// default, it makes the attribute Optional and Computed.
	DefaultObject bool `yaml:"defaultObject"`
}

// numberPatterns are the formats of the patterns of a 64-bit number that
// JSON sends as a string.
var numberPatterns = map[string]string{`^-?[0-9]+$`: "int64", `^[0-9]+$`: "uint64"}

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
	// AcceptZero false keeps Zero for reading, but not in the names that a
	// configuration may use (the server may reject the value).
	AcceptZero *bool `yaml:"acceptZero"`
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

// fields returns the fields of t that are attributes: not skipped, and not
// set from the arm (namesArm).
func (o *overrides) fields(t *model.Type) []*model.Field {
	var out []*model.Field
	for _, f := range t.Fields {
		if ov := o.field(t.Schema, f.Name); !ov.Skip && !ov.NamesArm {
			out = append(out, f)
		}
	}
	return out
}

// fieldType returns the type of the field f of the component schema, with a
// list made a set, and a string made a 64-bit number that JSON sends as a
// string, when the override says so. The model is not changed.
func (o *overrides) fieldType(schema string, f *model.Field) *model.Type {
	ov := o.field(schema, f.Name)
	switch {
	case ov.Set && f.Type.Kind == model.List:
		t := *f.Type
		t.Kind = model.Set
		return &t
	case ov.Int64 && f.Type.Kind == model.String:
		// The format that the OpenAPI generator leaves out (F68).
		return &model.Type{Kind: model.Integer, Format: numberPatterns[f.Type.Pattern], WireString: true}
	}
	return f.Type
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
	return widenType(t)
}

// widenType returns the wide type of the narrow number t.
func widenType(t *model.Type) *model.Type {
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
	errs = append(errs, o.checkUnwrap(roots, objects)...)
	errs = append(errs, o.checkCustomPackage()...)
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
		switch {
		case !ok && len(o.Types[t.Schema][name].Wrap) != 0:
			continue // checkWrappers
		case !ok:
			errs = append(errs, fmt.Errorf("%s: no such field", at))
			continue
		}
		if err := o.checkUsedField(o.Types[t.Schema][name], f); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", at, err))
			continue
		}
		if ok, err := o.checkShapeOverride(t, f); ok {
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", at, err))
			}
			continue
		}
		if err := checkField(o.Types[t.Schema][name], f); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", at, err))
		}
	}
	if len(o.fields(t)) == 0 {
		errs = append(errs, fmt.Errorf("overrides: types.%s: every field is skipped", t.Schema))
	}
	errs = append(errs, o.checkWrappers(t)...)
	return append(errs, o.checkNames(t)...)
}

// checkField checks one field override. The flags must leave a valid
// attribute: Required alone, Optional with or without Computed, or Computed
// alone.
func checkField(ov fieldOverride, f *model.Field) error {
	if len(ov.Wrap) != 0 {
		return errors.New("wrap: the key must be a new attribute name, not an API field")
	}
	if ov.Skip && !reflect.DeepEqual(ov, fieldOverride{Skip: true}) {
		return errors.New("skip cannot be combined with other overrides")
	}
	if ov.Name != "" && !attrNamePattern.MatchString(ov.Name) {
		return fmt.Errorf("name %q is not a valid Terraform attribute name", ov.Name)
	}
	if err := checkShape(ov, f.Type); err != nil {
		return err
	}
	if err := checkNumberText(ov, f.Type); err != nil {
		return err
	}
	if err := checkRead(ov, f.Type); err != nil {
		return err
	}
	return checkFlags(ov, f.Attrs)
}

// checkShape checks the overrides that need a kind of field: set, wide, and
// defaultObject.
func checkShape(ov fieldOverride, t *model.Type) error {
	switch {
	case ov.Set && t.Kind != model.List:
		return fmt.Errorf("set: the field is a %s, not a list", t.Kind)
	case ov.Wide && !isNarrow(t):
		return fmt.Errorf("wide: the field is a %s, not an int32 or a float", typeName(t))
	case ov.DefaultObject && (t.Kind != model.Object && t.Kind != model.OneOf || ov.Default != nil):
		return fmt.Errorf("defaultObject needs an object with no default, the field is a %s", t.Kind)
	}
	return nil
}

// checkNumberText checks the overrides int64 and string (F68): int64 needs a
// string with the pattern of a 64-bit number, and string needs an int64 JSON
// number.
func checkNumberText(ov fieldOverride, t *model.Type) error {
	switch {
	case ov.Int64 && ov.String:
		return errors.New("int64 and string cannot be combined")
	case ov.Int64 && (t.Kind != model.String || t.Format != "" || numberPatterns[t.Pattern] == ""):
		return fmt.Errorf("int64: the field is a %s with the pattern %q, it must be a string with the pattern %s", typeName(t), t.Pattern, strings.Join(sortedKeys(numberPatterns), " or "))
	case ov.String && (t.Kind != model.Integer || t.WireString || t.Format != "int64"):
		return fmt.Errorf("string: the field is a %s, it must be an int64 JSON number", typeName(t))
	}
	return nil
}

// checkRead checks the overrides that change how a response is read.
func checkRead(ov fieldOverride, t *model.Type) error {
	object := t.Kind == model.Object || t.Kind == model.OneOf
	collection := t.Kind == model.List || t.Kind == model.Set || t.Kind == model.Map
	switch {
	case ov.MissingAsZero && ov.EmptyAsNull:
		return errors.New("missingAsZero and emptyAsNull cannot be combined")
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
	case (ov.Default != nil || ov.DefaultObject) && !comp:
		return errors.New("a default needs a computed attribute")
	}
	return nil
}

// checkReadOnly fails when a read-only field, by the override or by the
// spec, also has flags or a default.
func checkReadOnly(ov fieldOverride, a model.Attrs) error {
	if ov.Required == nil && ov.Optional == nil && ov.Computed == nil && ov.Default == nil && !ov.DefaultObject {
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
	if a.Default != nil || ov.Default != nil || ov.DefaultObject {
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
	if ov.AcceptZero != nil && ov.Zero == "" {
		errs = append(errs, fmt.Errorf("%s.acceptZero needs zero", at))
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
