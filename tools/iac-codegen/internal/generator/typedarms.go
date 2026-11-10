package generator

import (
	"fmt"
	"slices"
	"strings"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

// typedArmValidatorName is the Go type that the schema file defines for a typedArms object.
const typedArmValidatorName = "typedArmValidator"

// typedValues returns the value of the type attribute for each arm of the oneOf t: the names entry
// of the line, or the Terraform name of the arm. It reports a names entry that is no arm.
func typedValues(t *model.Type, line *overrides.TypedArms, arms []*model.Field, tfName func(string) string) (map[string]string, error) {
	values := map[string]string{}
	owner := map[string]string{}
	for _, arm := range arms {
		value := line.Names[arm.Name]
		if value == "" {
			value = tfName(arm.Name)
		}
		if other, ok := owner[value]; ok {
			return nil, fmt.Errorf("%s: typedArms: the arms %s and %s both have the value %q", t.Schema, other, arm.Name, value)
		}
		values[arm.Name], owner[value] = value, arm.Name
	}
	for _, name := range slices.Sorted(mapsKeys(line.Names)) {
		if _, ok := values[name]; !ok {
			return nil, fmt.Errorf("%s: typedArms.names.%s is no arm of the oneOf", t.Schema, name)
		}
	}
	return values, nil
}

func mapsKeys[V any](m map[string]V) func(yield func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// typedArms are the arms of the oneOf t, without its discriminator.
func typedArms(t *model.Type) []*model.Field {
	var arms []*model.Field
	for _, f := range t.Fields {
		if f.Name != t.Discriminator {
			arms = append(arms, f)
		}
	}
	return arms
}

// typedAttributes are the attributes of the oneOf t with a typedArms line: the required type
// attribute, which names the arm, and the fields of all arms, merged by name and all optional. An
// arm that is a scalar has one attribute, with the name of the line's value. The model struct of
// each object arm is made too, because the conversion of the arm reads and writes it.
func (b *tfBuilder) typedAttributes(p attrPath, t *model.Type, line *overrides.TypedArms) ([]*tfAttr, error) {
	arms := typedArms(t)
	values, err := typedValues(t, line, arms, func(arm string) string { return terraformName(b.policy, t.Schema, arm) })
	if err != nil {
		return nil, err
	}
	quoted := make([]string, 0, len(arms))
	for _, arm := range arms {
		quoted = append(quoted, fmt.Sprintf("%q", values[arm.Name]))
	}
	typeName := line.TypeField()
	oneOf := "stringvalidator.OneOf(" + strings.Join(quoted, ", ") + ")"
	attrs := []*tfAttr{{
		Name: typeName, Kind: "String", ValueKind: "String", Required: true, Component: t.Schema, Property: typeName,
		// The values of the type state the structure of the request, not a limit of the contract.
		Validators: []string{oneOf}, GroupValidators: []string{oneOf},
		Description: "Which arm of the oneOf the object holds. One of " + strings.Join(quoted, ", ") + ".",
	}}
	fields := []tfModelField{{Name: camelize(typeName), Type: "types.String", TFName: typeName}}
	has := map[string][]string{}
	checked := map[string]map[string][]string{}
	for _, arm := range arms {
		armAttrs, armFields, checks, err := b.typedArm(p, t, arm, line)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", arm.Name, err)
		}
		has[values[arm.Name]] = []string{}
		if len(checks) > 0 {
			checked[values[arm.Name]] = map[string][]string{armAttrs[0].Name: checks}
		}
		for _, a := range armAttrs {
			has[values[arm.Name]] = append(has[values[arm.Name]], a.Name)
			if a.Name == typeName {
				return nil, fmt.Errorf("%s: typedArms.field %q is also a field of the arm %s", t.Schema, typeName, arm.Name)
			}
			a.Required, a.Optional = false, true
			k := slices.IndexFunc(armFields, func(f tfModelField) bool { return f.TFName == a.Name })
			if k < 0 {
				return nil, fmt.Errorf("%s: the arm %s has no model field for the attribute %s", t.Schema, arm.Name, a.Name)
			}
			if j := slices.IndexFunc(fields, func(o tfModelField) bool { return o.TFName == a.Name }); j >= 0 {
				if other := fields[j]; other.Type != armFields[k].Type {
					return nil, fmt.Errorf("%s: typedArms: the field %s has the type %s in one arm and %s in another", t.Schema, a.Name, other.Type, armFields[k].Type)
				}
				continue
			}
			attrs = append(attrs, a)
			fields = append(fields, armFields[k])
		}
	}
	attrs[0].Validators = append(attrs[0].Validators, typedValidator(attrs, arms, values, has, checked))
	attrs[0].GroupValidators = append(attrs[0].GroupValidators, attrs[0].Validators[len(attrs[0].Validators)-1])
	name := typeModelName(t)
	if err := b.claimModel(name, "the component "+t.Schema); err != nil {
		return nil, err
	}
	if !b.seen[name] {
		b.seen[name] = true
		b.models = append(b.models, &tfModel{Name: name, Fields: fields})
	}
	return attrs, nil
}

// typedArm returns the attributes of one arm, and their model fields: the fields of an object arm,
// one attribute (the line's value) for a scalar arm, and none for an arm without fields.
func (b *tfBuilder) typedArm(p attrPath, t *model.Type, arm *model.Field, line *overrides.TypedArms) ([]*tfAttr, []tfModelField, []string, error) {
	switch arm.Type.Kind {
	case model.Object:
		if len(arm.Type.Fields) == 0 {
			return nil, nil, nil, nil
		}
		attrs, err := b.objectAttributes(p, arm.Type)
		if err != nil {
			return nil, nil, nil, err
		}
		return attrs, b.modelFieldsOf(typeModelName(arm.Type)), nil, nil
	case model.String, model.Enum, model.Bool, model.Number, model.Integer:
		only := b.sent.responseOnly(t.Schema, arm)
		a, err := b.attribute(append(append(attrPath{}, p...), line.ValueField()), t.Schema, arm.Name, arm.Description, arm.Type, nestedAttrs(arm), only)
		if err != nil {
			return nil, nil, nil, err
		}
		a.Name = line.ValueField()
		field := b.modelField(t.Schema, arm.Name, arm.Type, arm.Behavior, only, false)
		field.Name, field.TFName = camelize(a.Name), a.Name
		checks, err := b.scalarArmChecks(a)
		if err != nil {
			return nil, nil, nil, err
		}
		return []*tfAttr{a}, []tfModelField{field}, checks, nil
	}
	return nil, nil, nil, fmt.Errorf("typedArms needs arms that are objects or scalars, not a %s", arm.Type.Kind)
}

// scalarArmChecks moves the checks of a scalar arm from its attribute to the typedArmValidator,
// which runs them only when the type names the arm: the attribute is shared with the other scalar
// arms. A mapped enum is checked as its enums line states. The limits of the contract stay checks
// when the file keeps them.
func (b *tfBuilder) scalarArmChecks(a *tfAttr) ([]string, error) {
	var checks []string
	if _, mapped := b.file.Enums[a.EnumSchema]; mapped && a.EnumSchema != "" && a.ValueKind == "String" {
		expr, err := enumValidatorExpr(a, b.file, true)
		if err != nil {
			return nil, err
		}
		checks = append(checks, expr)
	} else if inferred := b.file.Validators.Inferred; inferred == nil || *inferred {
		checks = append(checks, a.Validators...)
	}
	a.Validators, a.EnumSchema = nil, ""
	return checks, nil
}

// modelFieldsOf returns the fields of the model struct with the name, which the builder made.
func (b *tfBuilder) modelFieldsOf(name string) []tfModelField {
	for _, m := range b.models {
		if m.Name == name {
			return m.Fields
		}
	}
	return nil
}

// convTyped is a oneOf that Terraform shows as a type attribute plus the merged fields of its arms.
type convTyped struct {
	Pkg       string // the name of the SDK package
	TypeField *convField
	Arms      []*typedArm
	Allowed   string // the values of the type attribute, for an error text
}

// typedArm is one arm of a typedArms oneOf.
type typedArm struct {
	Value  string      // the value of the type attribute
	SDK    string      // the Go field of the oneOf struct
	Object *convObject // an object arm; its model holds the fields of the arm
	Empty  bool        // an arm without fields
	Scalar *convField  // a scalar arm: the conversion of the value attribute
}

// typed turns the oneOf obj into a typedArms object: the type attribute and the merged fields
// replace the arms as its fields. The arms keep their own conversion.
func (b *convBuilder) typed(obj *convObject, t *model.Type) error {
	line := b.file.TypedArmsOf(t.Schema)
	if line == nil {
		return nil
	}
	if !b.resource.Replace {
		// A PATCH mask names API fields, not the arms that Terraform shows.
		return fmt.Errorf("%s: typedArms needs an Update that replaces the resource; the update mask names API fields", t.Schema)
	}
	values, err := typedValues(t, line, typedArms(t), func(arm string) string { return terraformName(b.resource.Policy, t.Schema, arm) })
	if err != nil {
		return err
	}
	typeField := &convField{TFName: line.TypeField(), Model: camelize(line.TypeField()), Conv: convString, Carried: true}
	typed := &convTyped{Pkg: b.ix.pkg.Name, TypeField: typeField}
	merged := []*convField{typeField}
	var allowed []string
	for _, arm := range typedArms(t) {
		cf := fieldByModel(obj.Fields, camelize(arm.Name))
		if cf == nil {
			return fmt.Errorf("%s: no conversion of the arm %s", t.Schema, arm.Name)
		}
		a, fields := typedArmOf(cf, values[arm.Name], line)
		for _, f := range fields {
			if merged, err = b.mergeTyped(obj, merged, f); err != nil {
				return fmt.Errorf("%s: %w", arm.Name, err)
			}
		}
		typed.Arms = append(typed.Arms, a)
		allowed = append(allowed, a.Value)
	}
	typed.Allowed = strings.Join(allowed, ", ")
	obj.Fields, obj.Typed = merged, typed
	return nil
}

// typedArmOf returns the arm of the field cf, and the fields that it adds to the merged fields.
func typedArmOf(cf *convField, value string, line *overrides.TypedArms) (*typedArm, []*convField) {
	a := &typedArm{Value: value, SDK: cf.SDK}
	switch {
	case cf.Conv == convObj && cf.Object != nil:
		a.Object = cf.Object
		var fields []*convField
		for _, f := range cf.Object.Fields {
			if !f.Carried {
				fields = append(fields, f)
			}
		}
		return a, fields
	case cf.Conv == convEmpty:
		a.Empty = true
		return a, nil
	}
	scalar := *cf
	scalar.TFName, scalar.Model = line.ValueField(), camelize(line.ValueField())
	a.Scalar = &scalar
	return a, []*convField{&scalar}
}

// mergeTyped adds a copy of f to the merged fields. A field that two arms share must have one type.
func (b *convBuilder) mergeTyped(obj *convObject, merged []*convField, f *convField) ([]*convField, error) {
	c := *f
	c.Carried = true
	i := slices.IndexFunc(merged, func(o *convField) bool { return o.Model == c.Model })
	if i < 0 {
		return append(merged, &c), nil
	}
	first, err := b.attrType(obj, merged[i])
	if err != nil {
		return nil, err
	}
	other, err := b.attrType(obj, &c)
	if err != nil {
		return nil, err
	}
	if first != other {
		return nil, fmt.Errorf("typedArms: the field %s has the type %s in one arm and %s in another", c.TFName, first, other)
	}
	return merged, nil
}

// typedValidator is the Go expression of the typedArmValidator of an object whose merged attributes
// are attrs[1:]. has lists the attributes of each arm, by value of the type, and checked the checks
// of a scalar arm.
func typedValidator(attrs []*tfAttr, arms []*model.Field, values map[string]string, has map[string][]string, checked map[string]map[string][]string) string {
	quote := func(names []string) string {
		quoted := make([]string, 0, len(names))
		for _, n := range names {
			quoted = append(quoted, fmt.Sprintf("%q", n))
		}
		return strings.Join(quoted, ", ")
	}
	var names []string
	for _, a := range attrs[1:] {
		names = append(names, a.Name)
	}
	var fields, vals []string
	for _, arm := range arms {
		v := values[arm.Name]
		fields = append(fields, fmt.Sprintf("%q: {%s}", v, quote(has[v])))
		for _, attr := range slices.Sorted(mapsKeys(checked[v])) {
			vals = append(vals, fmt.Sprintf("%q: {%q: {%s}}", v, attr, strings.Join(checked[v][attr], ", ")))
		}
	}
	out := fmt.Sprintf("%s{Names: []string{%s}, Fields: map[string][]string{%s}", typedArmValidatorName, quote(names), strings.Join(fields, ", "))
	if len(vals) > 0 {
		out += ", Values: map[string]map[string][]validator.String{" + strings.Join(vals, ", ") + "}"
	}
	return out + "}"
}
