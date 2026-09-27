package main

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/model"
)

// Type strings (D21). Handwritten resources often show an API oneOf by the
// name of the set arm. The dashboards API sends
// {"aggregation": {"percentile": {"field": "x", "percent": 95}}}, and
// Terraform has aggregation = {type = "percentile", field = "x", percent = 95};
// it sends {"colorsBy": {"stack": {}}}, and Terraform has colors_by = "stack".
// The overrides lines
//
//	typeStrings:
//	  LogsAggregation: {names: {average: avg}}
//	  ColorsBy: {plain: true}
//
// make every field of that oneOf type, and every list of it, show it so. The
// key is the component, as for unwrap: every use has the same form. Rules:
//   - The name of an arm is its Terraform name (countDistinct →
//     count_distinct), or the one in names.
//   - plain: the attribute is the name itself, a String. Every arm must be an
//     empty object, so a new arm field stops the generator.
//   - Else the attribute is an object: type (required) names the arm, and the
//     fields of all arms are its other attributes, merged by Terraform name.
//     A merged field must have the same API type and overrides in each arm.
//     They are optional; a validator rejects a field of another arm, and
//     requires the required fields of the arm.
//   - expand sends the arm that type names, with its fields. flatten reads the
//     set arm; the fields of the other arms are null, and a oneOf with no arm
//     set reads as null.
//   - An arm can be skipped (types: {LogsAggregation: {newArm: {skip: true}}}).
//     Its fields keep the overrides of their own component, except the flags
//     and defaults.
//
// Empty objects as bools (D21). The API sends {"mappedValues": {}}, and
// Terraform has mapped_values = true. The field override bool: true makes
// the attribute a Bool: true sends {}, false or null sends nothing, and a
// present object reads as true.

// typeStringOverride is the typeStrings override of one oneOf component.
type typeStringOverride struct {
	// Plain makes the attribute the name of the set arm itself.
	Plain bool `yaml:"plain"`
	// Names are the Terraform names that differ from the rule, by API arm.
	Names map[string]string `yaml:"names"`
}

// typeStringAttr is the Terraform name of the attribute that names the arm.
const typeStringAttr = "type"

// typeString returns the typeStrings override of the component schema, or
// nil.
func (o *overrides) typeString(schema string) *typeStringOverride {
	if o == nil || schema == "" {
		return nil
	}
	if ts, ok := o.TypeStrings[schema]; ok {
		return &ts
	}
	return nil
}

// typeStringObject reports whether t is shown as an object with type.
func (o *overrides) typeStringObject(t *model.Type) bool {
	ts := o.typeString(t.Schema)
	return ts != nil && !ts.Plain
}

// armName is the Terraform name of the arm of the typeString oneOf schema.
func (o *overrides) armName(schema, arm string) string {
	if n := o.TypeStrings[schema].Names[arm]; n != "" {
		return n
	}
	return tfName(arm)
}

// armNames returns the Terraform names of the arms of the typeString t, in
// API order.
func (o *overrides) armNames(t *model.Type) []string {
	var out []string
	for _, arm := range o.fields(t) {
		out = append(out, o.armName(t.Schema, arm.Name))
	}
	return out
}

// mergedField is one attribute of a typeString object besides type: a field
// of one or more arms.
type mergedField struct {
	tfName string
	goName string   // the model field, from the API name of the first arm that has it
	arms   []string // the Terraform names of the arms that have it
	// required are the arms that require it.
	required []string
	first    *model.Field // the field in the first arm that has it
	schema   string       // the component of that arm
}

// mergedFields returns the attributes of the typeString object t besides
// type, in the order of the arms and their fields.
func (o *overrides) mergedFields(t *model.Type) []*mergedField {
	var out []*mergedField
	byName := map[string]*mergedField{}
	for _, arm := range o.fields(t) {
		name := o.armName(t.Schema, arm.Name)
		for _, f := range o.fields(arm.Type) {
			n := o.tfName(arm.Type.Schema, f.Name)
			m, ok := byName[n]
			if !ok {
				m = &mergedField{tfName: n, goName: camelize(f.Name), first: f, schema: arm.Type.Schema}
				byName[n] = m
				out = append(out, m)
			}
			m.arms = append(m.arms, name)
			if f.Attrs.Required {
				m.required = append(m.required, name)
			}
		}
	}
	return out
}

// optionalField returns a copy of f that is not required: in a typeString
// object, the validator requires it for its arm only.
func optionalField(f *model.Field) *model.Field {
	c := *f
	c.Attrs.Required = false
	return &c
}

// usesTypeString returns the typeString oneOf that the type t is, or that
// its items or values are, or "".
func (o *overrides) usesTypeString(t *model.Type) string {
	if (t.Kind == model.List || t.Kind == model.Set || t.Kind == model.Map) && t.Elem != nil {
		t = t.Elem
	}
	if t.Kind == model.OneOf && o.typeString(t.Schema) != nil {
		return t.Schema
	}
	return ""
}

// typeStringType returns the Terraform type of a plain typeString t: an enum
// of the arm names. Other types do not change.
func (o *overrides) typeStringType(t *model.Type) *model.Type {
	if ts := o.typeString(t.Schema); t.Kind == model.OneOf && ts != nil && ts.Plain {
		return &model.Type{Kind: model.Enum, Values: o.armNames(t)}
	}
	return t
}

// checkTypeStrings checks the typeStrings overrides (see the rules).
func (o *overrides) checkTypeStrings(roots []*model.Type, objects map[string]*model.Type) []error {
	var errs []error
	for _, schema := range sortedKeys(o.TypeStrings) {
		at := "overrides: typeStrings." + schema
		t, ok := objects[schema]
		switch {
		case !ok:
			errs = append(errs, fmt.Errorf("%s: no such object in the generated types", at))
			continue
		case slices.ContainsFunc(roots, func(r *model.Type) bool { return r.Schema == schema }):
			errs = append(errs, fmt.Errorf("%s: a root type cannot be a typeString", at))
			continue
		case t.Kind != model.OneOf:
			errs = append(errs, fmt.Errorf("%s: the object is not a oneOf", at))
			continue
		case o.unwrapped(schema):
			errs = append(errs, fmt.Errorf("%s: the object is also unwrapped", at))
			continue
		}
		for _, name := range sortedKeys(o.Types[schema]) {
			if !reflect.DeepEqual(o.Types[schema][name], fieldOverride{Skip: true}) {
				errs = append(errs, fmt.Errorf("overrides: types.%s.%s: an arm of a typeString can only be skipped", schema, name))
			}
		}
		errs = append(errs, o.checkArmNames(t, at)...)
		for _, arm := range o.fields(t) {
			if err := o.checkArm(t, arm); err != nil {
				errs = append(errs, fmt.Errorf("%s.%s: %w", at, arm.Name, err))
			}
		}
		if len(errs) == 0 && !o.TypeStrings[schema].Plain {
			errs = append(errs, o.checkMerged(t, at)...)
		}
	}
	return errs
}

// checkArmNames checks the names of the arms of the typeString t.
func (o *overrides) checkArmNames(t *model.Type, at string) []error {
	var errs []error
	arms := fieldNames(o.fields(t))
	for _, arm := range sortedKeys(o.TypeStrings[t.Schema].Names) {
		if !slices.Contains(arms, arm) {
			errs = append(errs, fmt.Errorf("%s.names.%s: no such arm", at, arm))
		}
	}
	seen := map[string]string{}
	for _, arm := range arms {
		name := o.armName(t.Schema, arm)
		if prev, ok := seen[name]; ok {
			errs = append(errs, fmt.Errorf("%s: the arms %s and %s both have the name %q", at, prev, arm, name))
		}
		seen[name] = arm
		if !attrNamePattern.MatchString(name) {
			errs = append(errs, fmt.Errorf("%s: the name %q of the arm %s is not a Terraform name", at, name, arm))
		}
	}
	return errs
}

// checkArm checks one arm of the typeString t: an object with no oneOf,
// whose fields have no flags, defaults, or shape overrides. A plain
// typeString needs an empty object.
func (o *overrides) checkArm(t *model.Type, arm *model.Field) error {
	at := arm.Type
	switch {
	case at.Kind != model.Object || len(at.Groups) != 0:
		return fmt.Errorf("the arm is a %s, a typeString arm must be an object with no oneOf", typeName(at))
	case len(o.wrappers(at)) != 0:
		return fmt.Errorf("%s has wrappers, which is not supported in a typeString arm", at.Schema)
	case o.TypeStrings[t.Schema].Plain && len(o.fields(at)) != 0:
		return fmt.Errorf("a plain typeString needs empty arms, and %s has the fields %v", at.Schema, fieldNames(o.fields(at)))
	}
	for _, f := range o.fields(at) {
		ov := o.effective(at.Schema, f)
		kept := fieldOverride{Name: ov.Name, Set: ov.Set, Int64: ov.Int64, String: ov.String, Wide: ov.Wide, Bool: ov.Bool,
			MissingAsZero: ov.MissingAsZero, EmptyAsNull: ov.EmptyAsNull, Sensitive: ov.Sensitive, DeprecationMessage: ov.DeprecationMessage}
		if !reflect.DeepEqual(ov, kept) {
			return fmt.Errorf("types.%s.%s: a field of a typeString arm cannot have flags, defaults, readOnly, or shape overrides", at.Schema, f.Name)
		}
	}
	return nil
}

// checkMerged checks the attributes of the typeString object t: a field of
// several arms has the same API type and overrides in each, and no name is
// type or two model fields.
func (o *overrides) checkMerged(t *model.Type, at string) []error {
	var errs []error
	merged := o.mergedFields(t)
	goNames := map[string]string{"Type": typeStringAttr}
	for _, m := range merged {
		if m.tfName == typeStringAttr {
			errs = append(errs, fmt.Errorf("%s: the arm field %s.%s has the name %q, which names the arm", at, m.schema, m.first.Name, typeStringAttr))
		}
		if prev, ok := goNames[m.goName]; ok {
			errs = append(errs, fmt.Errorf("%s: %s and %s both have the model field %s", at, prev, m.tfName, m.goName))
		}
		goNames[m.goName] = m.tfName
	}
	byName := map[string]*mergedField{}
	for _, m := range merged {
		byName[m.tfName] = m
	}
	for _, arm := range o.fields(t) {
		for _, f := range o.fields(arm.Type) {
			m := byName[o.tfName(arm.Type.Schema, f.Name)]
			if m.first == f {
				continue
			}
			if model.TypeText(f.Type) != model.TypeText(m.first.Type) || !reflect.DeepEqual(o.field(arm.Type.Schema, f.Name), o.field(m.schema, m.first.Name)) {
				errs = append(errs, fmt.Errorf("%s: %s.%s and %s.%s are both %q, but their API types or overrides differ",
					at, m.schema, m.first.Name, arm.Type.Schema, f.Name, m.tfName))
			}
		}
	}
	return errs
}

// checkTypeStringUse fails for an override that a field of a typeString type
// cannot have: a typeString reads a oneOf with no arm as null, so it has no
// missingAsZero, and it is not inlined.
func (o *overrides) checkTypeStringUse(ov fieldOverride, f *model.Field) error {
	s := o.usesTypeString(f.Type)
	switch {
	case s == "":
		return nil
	case ov.MissingAsZero:
		return fmt.Errorf("%s is a typeString: missingAsZero is not supported", s)
	case ov.Bool:
		return fmt.Errorf("%s is a typeString: bool is not supported", s)
	}
	return nil
}

// typeStringAttributes returns the attributes of the typeString object t at
// the path p, and adds its model struct.
func (b *tfBuilder) typeStringAttributes(p attrPath, t *model.Type) ([]*tfAttr, error) {
	if b.ov.typeString(t.Schema).Plain {
		return nil, fmt.Errorf("%s is a plain typeString: a list, set, or map of it is not supported", t.Schema)
	}
	names := b.ov.armNames(t)
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = strconv.Quote(n)
	}
	attrs := []*tfAttr{{Name: typeStringAttr, Kind: "String", ValueKind: "String", Required: true,
		Description: fmt.Sprintf("The field of %s that is set: %s.", t.Schema, strings.Join(names, ", ")),
		Validators:  []string{"stringvalidator.OneOf(" + strings.Join(quoted, ", ") + ")"}}}
	fields := []tfModelField{{Name: "Type", Type: "types.String", TFName: typeStringAttr}}
	for _, m := range b.ov.mergedFields(t) {
		a, mf, err := b.field(append(append(attrPath{}, p...), m.tfName), m.schema, optionalField(m.first))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", m.first.Name, err)
		}
		a.Description = strings.TrimSpace(fmt.Sprintf("%s Only with type %s.", a.Description, strings.Join(m.arms, ", ")))
		mf.Name = m.goName
		attrs, fields = append(attrs, a), append(fields, mf)
	}
	if name := modelTypeName(t.Schema); !b.seen[name] {
		b.seen[name] = true
		b.models = append(b.models, &tfModel{Name: name, Schema: t.Schema, Fields: fields})
	}
	return attrs, nil
}

// typeStringValidator returns the Go expression of the validator of the
// typeString object t: the attributes of each arm, and the required ones.
func (b *tfBuilder) typeStringValidator(t *model.Type) string {
	fields, required := map[string][]string{}, map[string][]string{}
	for _, name := range b.ov.armNames(t) {
		fields[name] = nil
	}
	for _, m := range b.ov.mergedFields(t) {
		for _, arm := range m.arms {
			fields[arm] = append(fields[arm], m.tfName)
		}
		for _, arm := range m.required {
			required[arm] = append(required[arm], m.tfName)
		}
	}
	return fmt.Sprintf("typeStringValidator{fields: %s, required: %s}", stringsMapExpr(fields), stringsMapExpr(required))
}

// stringsMapExpr is the Go expression of m, with the keys in order.
func stringsMapExpr(m map[string][]string) string {
	var entries []string
	for _, k := range sortedKeys(m) {
		quoted := make([]string, len(m[k]))
		for i, v := range m[k] {
			quoted[i] = strconv.Quote(v)
		}
		entries = append(entries, fmt.Sprintf("%q: {%s}", k, strings.Join(quoted, ", ")))
	}
	return "map[string][]string{" + strings.Join(entries, ", ") + "}"
}

// usesTypeStringValidator reports whether an attribute in attrs, at any
// depth, has the validator of a typeString object.
func usesTypeStringValidator(attrs []*tfAttr) bool {
	has := func(vals []string) bool {
		return slices.ContainsFunc(vals, func(v string) bool { return strings.HasPrefix(v, "typeStringValidator{") })
	}
	for _, a := range attrs {
		if has(a.Validators) || has(a.ElemValidators) || usesTypeStringValidator(a.Attributes) {
			return true
		}
	}
	return false
}

// typeStringData is the conversion data of a typeString oneOf.
type typeStringData struct {
	Plain bool
	Arms  []*typeStringArm
}

// typeStringArm is one arm of a typeString oneOf.
type typeStringArm struct {
	Name string // the Terraform name
	SDK  string // the SDK field of the arm
	// Object is the conversion of the fields of the arm: its model is the
	// model of the typeString object. nil for an empty arm.
	Object *convObject
}

// typeStringObject returns the convObject of the typeString oneOf t, and
// fills it on the first call. Its expand and flatten have the signatures of
// an object (object form) or of an unwrapped object with a types.String
// value (plain), so the field conversions of objects work for it.
func (b *convBuilder) typeStringObject(t *model.Type) (*convObject, error) {
	ref, err := b.ix.schemaRef(t.Schema)
	if err != nil {
		return nil, err
	}
	if obj, ok := b.bySchema[t.Schema]; ok {
		return obj, nil
	}
	obj := b.object(t.Schema, ref)
	ts := &typeStringData{Plain: b.ov.typeString(t.Schema).Plain}
	obj.TypeString = ts
	b.objects = append(b.objects, obj)
	goNames := map[string]string{}
	if !ts.Plain {
		obj.Fields = []*convField{{TFName: typeStringAttr, Model: "Type", Conv: convString}}
		for _, m := range b.ov.mergedFields(t) {
			goNames[m.tfName] = m.goName
		}
	}
	merged := map[string]bool{}
	for _, arm := range b.ov.fields(t) {
		a, err := b.typeStringArm(obj, ref.Path, arm)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", arm.Name, err)
		}
		a.Name = b.ov.armName(t.Schema, arm.Name)
		ts.Arms = append(ts.Arms, a)
		if a.Object == nil {
			continue
		}
		for _, cf := range a.Object.Fields {
			cf.Model = goNames[cf.TFName]
			if !merged[cf.TFName] {
				merged[cf.TFName] = true
				obj.Fields = append(obj.Fields, cf)
			}
		}
	}
	return obj, nil
}

// typeStringArm returns the conversion of the arm of the typeString obj,
// whose SDK struct is at owner.
func (b *convBuilder) typeStringArm(obj *convObject, owner string, arm *model.Field) (*typeStringArm, error) {
	ref, err := b.ix.fieldRef(owner + "." + arm.Name)
	if err != nil {
		return nil, err
	}
	out := &typeStringArm{SDK: ref.Name}
	fields := b.ov.fields(arm.Type)
	if len(fields) == 0 {
		if ref.Want != "map[string]interface{}" {
			return nil, fmt.Errorf("SDK field %s has type %s, an empty arm needs map[string]interface{}", ref.sdkName(), ref.Want)
		}
		return out, nil
	}
	nested, err := b.ix.schemaRef(arm.Type.Schema)
	if err != nil {
		return nil, err
	}
	if ref.Want != "*"+nested.Name {
		return nil, fmt.Errorf("SDK field %s has type %s, the arm needs *%s", ref.sdkName(), ref.Want, nested.Name)
	}
	out.Object = &convObject{Func: obj.Func + "Arm" + camelize(arm.Name), Model: obj.Model, SDK: nested.Name, Arm: true}
	for _, f := range fields {
		cf, err := b.objectField(nested.Path, arm.Type.Schema, optionalField(f))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		out.Object.Fields = append(out.Object.Fields, cf)
	}
	b.objects = append(b.objects, out.Object)
	return out, nil
}

// nullInit is a field of a typeString object model that flatten first sets
// to its null value: the fields of the arms that are not set stay null.
type nullInit struct {
	Model, GoType, AttrType string
}

// NullInits returns the fields of the typeString object obj besides type
// whose zero Go value is not a Terraform null: collections and objects that
// are types.Object. A zero types.String, types.Bool, or number is null, and a
// nil model pointer too.
func (obj *convObject) NullInits() ([]nullInit, error) {
	var out []nullInit
	for i, cf := range obj.Fields {
		goType, err := modelGoType(cf)
		if err != nil {
			return nil, err
		}
		if i == 0 || strings.HasPrefix(goType, "*") || zeroIsNull[goType] {
			continue
		}
		if i >= len(obj.AttrTypes) || obj.AttrTypes[i].TFName != cf.TFName {
			return nil, errors.New("internal error: the attribute types of a typeString object are not in field order")
		}
		out = append(out, nullInit{Model: cf.Model, GoType: goType, AttrType: obj.AttrTypes[i].Expr})
	}
	return out, nil
}

// zeroIsNull are the model Go types whose zero value is a Terraform null.
var zeroIsNull = map[string]bool{"types.String": true, "types.Bool": true, "types.Int32": true, "types.Int64": true,
	"types.Float32": true, "types.Float64": true, "types.Number": true}
