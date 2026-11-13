package generator

import (
	"errors"
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
	parts := make([]*typedPart, 0, len(arms))
	for _, arm := range arms {
		armAttrs, armFields, armChecks, err := b.typedArm(p, t, arm, line)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", arm.Name, err)
		}
		parts = append(parts, &typedPart{arm: arm, value: values[arm.Name], attrs: armAttrs, fields: armFields, checks: armChecks})
	}
	if err := b.shareArmChecks(t, parts); err != nil {
		return nil, err
	}
	for _, part := range parts {
		if err := b.armRules(part); err != nil {
			return nil, err
		}
	}
	m := &typedMerge{attrs: attrs, fields: fields, has: map[string][]string{}, required: map[string][]string{}, checked: map[string]map[string][]string{}, rules: map[string][]typedRule{}}
	for _, part := range parts {
		if err := m.add(t, typeName, part); err != nil {
			return nil, err
		}
	}
	attrs, fields = m.attrs, m.fields
	has, required, checked := m.has, m.required, m.checked
	if line.Infer {
		inferTypeAttribute(attrs[0], arms, values, has)
	}
	attrs[0].Typed = &typedInfo{Has: has, Required: required, Checked: checked, Rules: m.rules}
	attrs[0].Validators = append(attrs[0].Validators, typedValidator(attrs, arms, values, has, required, checked, m.rules))
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

// typedInfo is what each arm of a typedArms object has. Every map is keyed by the value of the type.
type typedInfo struct {
	Has      map[string][]string            // the attributes of the arm
	Required map[string][]string            // the attributes that the arm needs
	Checked  map[string]map[string][]string // the validators of an attribute that run for the arm
	Rules    map[string][]typedRule         // the rules over attributes of the arm
}

// fields are the attributes that a config sets for the arm of the type value: the attributes of the
// arm, without the ones that a rule of the arm excludes. A rule that wants exactly one of some
// attributes keeps the first, and a conflict keeps the attribute that the rule names first.
func (t *typedInfo) fields(value string) map[string]bool {
	out := map[string]bool{}
	for _, name := range t.Has[value] {
		out[name] = true
	}
	for _, rule := range t.Rules[value] {
		switch rule.Kind {
		case "exactlyOneOf", "conflictsWith":
			kept := false
			for _, name := range rule.Names {
				switch {
				case !out[name]:
				case kept:
					delete(out, name)
				default:
					kept = true
				}
			}
		}
	}
	return out
}

// typedMerge collects the merged attributes and model fields of a typedArms object, with what each
// arm has, what it requires, and the checks that run for it.
type typedMerge struct {
	attrs    []*tfAttr
	fields   []tfModelField
	has      map[string][]string
	required map[string][]string
	checked  map[string]map[string][]string
	rules    map[string][]typedRule
}

// add merges the attributes of one arm. A merged field is optional, because other arms do not have
// it. The arm that requires it, or that is the scalar itself, still needs it when the type names
// the arm. A field that two arms share must have one Go type.
func (m *typedMerge) add(t *model.Type, typeName string, part *typedPart) error {
	m.has[part.value] = []string{}
	if len(part.checks) > 0 {
		m.checked[part.value] = part.checks
	}
	if len(part.rules) > 0 {
		m.rules[part.value] = part.rules
	}
	for _, a := range part.attrs {
		m.has[part.value] = append(m.has[part.value], a.Name)
		if a.Required || part.arm.Type.Kind != model.Object {
			m.required[part.value] = append(m.required[part.value], a.Name)
		}
		if a.Name == typeName {
			return fmt.Errorf("%s: typedArms.field %q is also a field of the arm %s", t.Schema, typeName, part.arm.Name)
		}
		a.Required, a.Optional, a.TypedArm = false, true, true
		k := slices.IndexFunc(part.fields, func(f tfModelField) bool { return f.TFName == a.Name })
		if k < 0 {
			return fmt.Errorf("%s: the arm %s has no model field for the attribute %s", t.Schema, part.arm.Name, a.Name)
		}
		if j := slices.IndexFunc(m.fields, func(o tfModelField) bool { return o.TFName == a.Name }); j >= 0 {
			if other := m.fields[j]; other.Type != part.fields[k].Type {
				return fmt.Errorf("%s: typedArms: the field %s has the type %s in one arm and %s in another", t.Schema, a.Name, other.Type, part.fields[k].Type)
			}
			continue
		}
		m.attrs = append(m.attrs, a)
		m.fields = append(m.fields, part.fields[k])
	}
	return nil
}

// typedPart is one arm of a typedArms object while its attributes are merged.
type typedPart struct {
	arm    *model.Field
	value  string // the value of the type attribute
	attrs  []*tfAttr
	fields []tfModelField
	checks map[string][]string // the checks that run only when the type names this arm
	rules  []typedRule         // the rules over attributes of this arm that run only when the type names it
}

// typedRule is a rule over attributes of an arm, from a validators line of one of its fields. Names
// has the attribute of the line first, then the attributes that the line names.
type typedRule struct {
	Kind  string
	Names []string
}

// crossValidator returns the kind of a validators entry that names other attributes, and the names.
func crossValidator(v overrides.Validator) (string, []string) {
	switch {
	case len(v.ExactlyOneOf) != 0:
		return "exactlyOneOf", v.ExactlyOneOf
	case len(v.AtLeastOneOf) != 0:
		return "atLeastOneOf", v.AtLeastOneOf
	case len(v.AlsoRequires) != 0:
		return "alsoRequires", v.AlsoRequires
	case len(v.ConflictsWith) != 0:
		return "conflictsWith", v.ConflictsWith
	}
	return "", nil
}

// armRules reads the validators lines of the fields of an object arm. They run only when the type
// names the arm, because the merged field is one copy that every arm shares: a rule over other
// attributes becomes a rule of the arm, and a validator of a text field becomes a check of the arm.
func (b *tfBuilder) armRules(part *typedPart) error {
	if part.arm.Type.Kind != model.Object || part.arm.Type.Schema == "" || inlinedTypedField(b.file, part.arm.Type) != nil {
		return nil
	}
	has := map[string]bool{}
	for _, a := range part.attrs {
		has[a.Name] = true
	}
	for _, a := range part.attrs {
		for _, v := range b.file.Types[part.arm.Type.Schema].Fields[a.Property].Validators {
			if kind, names := crossValidator(v); kind != "" {
				for _, name := range names {
					if !has[name] || name == a.Name {
						return fmt.Errorf("%s: validator target %q is not another attribute of the arm", part.arm.Name, name)
					}
				}
				part.rules = append(part.rules, typedRule{Kind: kind, Names: append([]string{a.Name}, names...)})
				continue
			}
			if a.ValueKind != "String" {
				return fmt.Errorf("%s.%s: a validators line of a typedArms arm field takes a rule over other attributes, or a validator of a text field", part.arm.Name, a.Name)
			}
			expr, err := validatorExpr(a, v, b.file)
			if err != nil {
				return fmt.Errorf("%s.%s: %w", part.arm.Name, a.Name, err)
			}
			if part.checks == nil {
				part.checks = map[string][]string{}
			}
			part.checks[a.Name] = append(part.checks[a.Name], expr)
		}
	}
	return nil
}

// shareArmChecks moves the checks of a field that two or more object arms share into the
// typedArmValidator of each arm. The merged attribute is one copy, so it cannot hold the checks of
// every arm: the enum of one arm would reject the free text of another. A shared text field gets
// per-arm checks. Another kind of shared field must have the same checks in every arm.
func (b *tfBuilder) shareArmChecks(t *model.Type, parts []*typedPart) error {
	first, shared := sharedArmFields(parts)
	for _, part := range parts {
		for _, a := range part.attrs {
			// The type attribute of an inlined typedArms object has structural checks (its values and
			// its arm rules). They state the structure of the request, not a limit, so they stay.
			if !shared[a.Name] || part.arm.Type.Kind != model.Object || len(a.GroupValidators) > 0 {
				continue
			}
			if err := b.shareArmCheck(t, part, a, first[a.Name]); err != nil {
				return err
			}
		}
	}
	return nil
}

// sharedArmFields returns the first attribute of each field name of the object arms, and the names
// that two or more object arms have.
func sharedArmFields(parts []*typedPart) (first map[string]*tfAttr, shared map[string]bool) {
	first, shared = map[string]*tfAttr{}, map[string]bool{}
	for _, part := range parts {
		if part.arm.Type.Kind != model.Object {
			continue
		}
		for _, a := range part.attrs {
			if _, ok := first[a.Name]; ok {
				shared[a.Name] = true
				continue
			}
			first[a.Name] = a
		}
	}
	return first, shared
}

// shareArmCheck moves the checks of the shared field a of one arm into the validator of the arm, or
// reports a shared field that is not text and has other checks than the first arm's.
func (b *tfBuilder) shareArmCheck(t *model.Type, part *typedPart, a, first *tfAttr) error {
	if a.ValueKind != "String" {
		// With no inferred checks, every limit of the contract is dropped anyway.
		if inferred := b.file.Validators.Inferred; (inferred == nil || *inferred) && !slices.Equal(a.Validators, first.Validators) {
			return fmt.Errorf("%s: typedArms: the field %s has other checks in the arm %s than in another arm, and only a text field gets checks for each arm", t.Schema, a.Name, part.arm.Name)
		}
		return nil
	}
	checks, err := b.scalarArmChecks(a)
	if err != nil {
		return fmt.Errorf("%s: %w", part.arm.Name, err)
	}
	if len(checks) > 0 {
		if part.checks == nil {
			part.checks = map[string][]string{}
		}
		part.checks[a.Name] = checks
	}
	return nil
}

// typedArm returns the attributes of one arm, and their model fields: the fields of an object arm,
// one attribute (the line's value) for a scalar arm, and none for an arm without fields.
func (b *tfBuilder) typedArm(p attrPath, t *model.Type, arm *model.Field, line *overrides.TypedArms) ([]*tfAttr, []tfModelField, map[string][]string, error) {
	switch arm.Type.Kind {
	case model.Object:
		if len(arm.Type.Fields) == 0 {
			return nil, nil, nil, nil
		}
		attrs, err := b.objectAttributes(p, arm.Type)
		if err != nil {
			return nil, nil, nil, err
		}
		if inner := inlinedTypedField(b.file, arm.Type); inner != nil {
			// The arm holds one typedArms object: its attributes are the attributes of this one, and
			// this arm allows only the values of its type.
			held := attrs[0].Attributes
			return held, b.modelFieldsOf(typeModelName(inner.Type)), map[string][]string{held[0].Name: {held[0].GroupValidators[0]}}, nil
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
		if len(checks) == 0 {
			return []*tfAttr{a}, []tfModelField{field}, nil, nil
		}
		return []*tfAttr{a}, []tfModelField{field}, map[string][]string{a.Name: checks}, nil
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
	// Infer: an omitted type is picked from the fields that are set (typedArms infer).
	// InferArms is the Go literal of the arms, and InferFields the merged fields, by Terraform name.
	Infer       bool
	InferArms   string
	InferFields []*convField
}

// typedPair is a field of an arm and the field of the merged object that holds it. Two arms can
// call their fields by different API names and show them as one attribute (a name line), so the
// Go names can differ.
type typedPair struct {
	Arm    string // the Go field of the model of the arm
	Merged string // the Go field of the model of the typedArms object
}

// typedInit reads the Fields of Object from an empty value of its SDK type.
type typedInit struct {
	Object *convObject
	Fields []typedPair
}

// typedArm is one arm of a typedArms oneOf.
type typedArm struct {
	Value  string      // the value of the type attribute
	SDK    string      // the Go field of the oneOf struct
	Object *convObject // an object arm; its model holds the fields of the arm
	Empty  bool        // an arm without fields
	Scalar *convField  // a scalar arm: the conversion of the value attribute
	// Inits are the merged fields that no earlier arm has: flatten reads them from an empty arm
	// first, so a field that the selected arm lacks reads as an absent value, with its default.
	Inits []typedInit
	// Inline is the field of an arm that holds one typedArms object, which this arm shows as its
	// own attributes. Inner is that object.
	Inline *convField
	Inner  *convObject
	// Pairs are the fields that the arm adds to the merged fields: the fields of an object arm that
	// are not carried, or the fields of the inner object of an inline arm.
	Pairs []typedPair
	// InitScalar: this arm is the first with the scalar value attribute.
	InitScalar bool
	// Fields are the Terraform names of the fields that the arm has, in order.
	Fields []string
}

// typed turns the oneOf obj into a typedArms object: the type attribute and the merged fields
// replace the arms as its fields. The arms keep their own conversion.
func (b *convBuilder) typed(obj *convObject, t *model.Type) error {
	line := b.file.TypedArmsOf(t.Schema)
	if line == nil || line.AsList {
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
		if err := b.emptyDefaults(obj, cf, arm, fields); err != nil {
			return fmt.Errorf("%s: %w", arm.Name, err)
		}
		var fresh []typedPair
		for _, f := range fields {
			isNew := !slices.ContainsFunc(merged, func(o *convField) bool { return o.TFName == f.TFName })
			var name string
			if merged, name, err = b.mergeTyped(obj, merged, f); err != nil {
				return fmt.Errorf("%s: %w", arm.Name, err)
			}
			a.Fields = append(a.Fields, f.TFName)
			a.Pairs = append(a.Pairs, typedPair{Arm: f.Model, Merged: name})
			if isNew {
				fresh = append(fresh, typedPair{Arm: f.Model, Merged: name})
			}
		}
		if err := typedInits(a, fresh); err != nil {
			return fmt.Errorf("%s: %w", arm.Name, err)
		}
		typed.Arms = append(typed.Arms, a)
		allowed = append(allowed, a.Value)
	}
	typed.Allowed = strings.Join(allowed, ", ")
	if line.Infer {
		typed.Infer, typed.InferFields = true, merged[1:]
		typed.InferArms = inferArmsLiteral(typed.Arms)
	}
	obj.Fields, obj.Typed = merged, typed
	return nil
}

// typedArmOf returns the arm of the field cf, and the fields that it adds to the merged fields.
func typedArmOf(cf *convField, value string, line *overrides.TypedArms) (*typedArm, []*convField) {
	a := &typedArm{Value: value, SDK: cf.SDK}
	switch {
	case cf.Conv == convObj && cf.Object != nil:
		a.Object = cf.Object
		if len(cf.Object.Fields) == 1 && cf.Object.Fields[0].Object != nil && cf.Object.Fields[0].Object.Typed != nil {
			// The arm holds one typedArms object. The merged fields are the fields of that object.
			a.Inline, a.Inner = cf.Object.Fields[0], cf.Object.Fields[0].Object
			var fields []*convField
			for _, f := range a.Inner.Fields {
				fields = append(fields, f)
			}
			return a, fields
		}
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

// mergeTyped adds a copy of f to the merged fields, and returns the Go name that the merged object
// has for it. Fields merge by Terraform name: two arms can call a field by different API names and
// show it as one attribute. The first arm names the merged field. A field that two arms share must
// have one type.
func (b *convBuilder) mergeTyped(obj *convObject, merged []*convField, f *convField) ([]*convField, string, error) {
	c := *f
	c.Carried = true
	i := slices.IndexFunc(merged, func(o *convField) bool { return o.TFName == c.TFName })
	if i < 0 {
		if j := slices.IndexFunc(merged, func(o *convField) bool { return o.Model == c.Model }); j >= 0 {
			return nil, "", fmt.Errorf("typedArms: the fields %s and %s have one Go name, %s", merged[j].TFName, c.TFName, c.Model)
		}
		return append(merged, &c), c.Model, nil
	}
	if merged[i].EmptyDefault == "" {
		merged[i].EmptyDefault = c.EmptyDefault
	}
	first, err := b.attrType(obj, merged[i])
	if err != nil {
		return nil, "", err
	}
	other, err := b.attrType(obj, &c)
	if err != nil {
		return nil, "", err
	}
	if first != other {
		return nil, "", fmt.Errorf("typedArms: the field %s has the type %s in one arm and %s in another", c.TFName, first, other)
	}
	return merged, merged[i].Model, nil
}

// typedValidator is the Go expression of the typedArmValidator of an object whose merged attributes
// are attrs[1:]. has lists the attributes of each arm, by value of the type, and checked the checks
// of a scalar arm. required lists the fields that each arm needs when its type is chosen.
func typedValidator(attrs []*tfAttr, arms []*model.Field, values map[string]string, has, required map[string][]string, checked map[string]map[string][]string, rules map[string][]typedRule) string {
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
		// One entry for each arm: a map literal cannot repeat a key.
		var perAttr []string
		for _, attr := range slices.Sorted(mapsKeys(checked[v])) {
			perAttr = append(perAttr, fmt.Sprintf("%q: {%s}", attr, strings.Join(checked[v][attr], ", ")))
		}
		if len(perAttr) > 0 {
			vals = append(vals, fmt.Sprintf("%q: {%s}", v, strings.Join(perAttr, ", ")))
		}
	}
	out := fmt.Sprintf("%s{Names: []string{%s}, Fields: map[string][]string{%s}", typedArmValidatorName, quote(names), strings.Join(fields, ", "))
	var needs []string
	for _, arm := range arms {
		if v := values[arm.Name]; len(required[v]) > 0 {
			needs = append(needs, fmt.Sprintf("%q: {%s}", v, quote(required[v])))
		}
	}
	if len(needs) > 0 {
		out += ", Required: map[string][]string{" + strings.Join(needs, ", ") + "}"
	}
	if len(vals) > 0 {
		out += ", Values: map[string]map[string][]validator.String{" + strings.Join(vals, ", ") + "}"
	}
	var ruled []string
	for _, arm := range arms {
		v := values[arm.Name]
		var items []string
		for _, r := range rules[v] {
			items = append(items, fmt.Sprintf("{Kind: %q, Names: []string{%s}}", r.Kind, quote(r.Names)))
		}
		if len(items) > 0 {
			ruled = append(ruled, fmt.Sprintf("%q: {%s}", v, strings.Join(items, ", ")))
		}
	}
	if len(ruled) > 0 {
		out += ", Rules: map[string][]typedRule{" + strings.Join(ruled, ", ") + "}"
	}
	return out + "}"
}

// inferTypeAttribute makes the type attribute optional and computed. Its plan modifier picks the
// type of an omitted value from the fields that the configuration sets (typedArms infer).
func inferTypeAttribute(a *tfAttr, arms []*model.Field, values map[string]string, has map[string][]string) {
	a.Required, a.Optional, a.Computed = false, true, true
	var inferArms []string
	for _, arm := range arms {
		v := values[arm.Name]
		inferArm := fmt.Sprintf("{Value: %q", v)
		if len(has[v]) > 0 {
			quoted := make([]string, 0, len(has[v]))
			for _, n := range has[v] {
				quoted = append(quoted, fmt.Sprintf("%q", n))
			}
			inferArm += ", Fields: []string{" + strings.Join(quoted, ", ") + "}"
		}
		inferArms = append(inferArms, inferArm+"}")
	}
	a.Modifiers = append(a.Modifiers, "typedArmInferred{Arms: []typedInferArm{"+strings.Join(inferArms, ", ")+"}}")
	a.Description += " If it is left out, it is the first arm that has a field that is set, and else the arm without fields."
}

// inferArmsLiteral is the Go literal of the arms for inferTypedKind.
func inferArmsLiteral(arms []*typedArm) string {
	var out []string
	for _, a := range arms {
		item := fmt.Sprintf("{Value: %q", a.Value)
		if len(a.Fields) > 0 {
			quoted := make([]string, 0, len(a.Fields))
			for _, n := range a.Fields {
				quoted = append(quoted, fmt.Sprintf("%q", n))
			}
			item += ", Fields: []string{" + strings.Join(quoted, ", ") + "}"
		}
		out = append(out, item+"}")
	}
	return "[]typedInferArm{" + strings.Join(out, ", ") + "}"
}

// inlinedTypedField returns the only field of the object t when its type has a typedArms line, or
// nil. A typedArms arm like that is shown as the attributes of the inner object: the arm {selection:
// <oneOf>} of an operator is the type and the fields of the selection.
func inlinedTypedField(file *overrides.File, t *model.Type) *model.Field {
	if len(t.Fields) != 1 || file.TypedArmsOf(t.Fields[0].Type.Schema) == nil {
		return nil
	}
	return t.Fields[0]
}

// typedInits sets how flatten reads the fresh fields (the merged fields that no earlier arm has) of
// the arm from an empty value, so a field that the selected arm lacks reads as an absent value.
func typedInits(a *typedArm, fresh []typedPair) error {
	switch {
	case a.Scalar != nil:
		if len(fresh) > 0 {
			a.InitScalar = true
		}
	case a.Inner != nil:
		for _, ia := range a.Inner.Typed.Arms {
			if ia.Scalar != nil || ia.Inner != nil {
				return errors.New("typedArms: an arm that holds a typedArms object with scalar or nested arms is not supported")
			}
			var fields []typedPair
			for _, init := range ia.Inits {
				for _, f := range init.Fields {
					// The fresh fields are named after the inner object, which is the arm of this one.
					if i := slices.IndexFunc(fresh, func(p typedPair) bool { return p.Arm == f.Merged }); i >= 0 {
						fields = append(fields, typedPair{Arm: f.Arm, Merged: fresh[i].Merged})
					}
				}
			}
			if len(fields) > 0 {
				a.Inits = append(a.Inits, typedInit{Object: ia.Object, Fields: fields})
			}
		}
	case len(fresh) > 0 && a.Object != nil:
		a.Inits = append(a.Inits, typedInit{Object: a.Object, Fields: fresh})
	}
	return nil
}

// typedArmNeedsPrior reports whether an arm of a typedArms object needs the prior value to read. The
// merged fields are copies of the first arm's, so they do not remember that a later arm needs it.
func typedArmNeedsPrior(t *convTyped) bool {
	for _, a := range t.Arms {
		if a.Object != nil && a.Object.NeedsPrior || a.Scalar != nil && scalarNeedsPrior(a.Scalar) {
			return true
		}
	}
	return false
}

// scalarNeedsPrior reports whether the read of a scalar field needs the prior value (markPriorFields).
func scalarNeedsPrior(f *convField) bool {
	return f.Equality != "" || f.Conv == convFloat64AsFloat32 || f.Conv == convFloat32AsFloat64 || f.Conv == convFloat64AsNumber || f.CaseInsensitive || f.ReadEmptyAsNull && f.Conv == convString
}

// emptyDefaults sets EmptyDefault on the fields of an object arm whose line has default: []. The
// fields are copies of the arm's own, so the arm itself is not changed.
func (b *convBuilder) emptyDefaults(obj *convObject, arm *convField, field *model.Field, fields []*convField) error {
	if arm.Object == nil || field.Type.Schema == "" {
		return nil
	}
	names := apiNames(arm.Object, field.Type)
	for i, f := range fields {
		d, ok := b.file.Types[field.Type.Schema].Fields[names[f]].Default.([]any)
		if !ok || len(d) != 0 {
			continue
		}
		expr, err := b.attrType(obj, f)
		if err != nil {
			return err
		}
		value, ok := emptyCollectionValue(expr)
		if !ok {
			return fmt.Errorf("%s: default: [] needs a list or set", f.TFName)
		}
		c := *f
		c.EmptyDefault = value
		fields[i] = &c
	}
	return nil
}

// emptyCollectionValue turns the attribute type of a list or set, such as
// types.ListType{ElemType: types.StringType}, into the empty value of that type.
func emptyCollectionValue(attrType string) (string, bool) {
	for _, kind := range []string{"List", "Set"} {
		if elem, ok := strings.CutPrefix(attrType, "types."+kind+"Type{ElemType: "); ok {
			return "types." + kind + "ValueMust(" + strings.TrimSuffix(elem, "}") + ", []attr.Value{})", true
		}
	}
	return "", false
}
