package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/model"
)

// Custom fields (D21). Some handwritten attributes have a Terraform shape
// that no rule converts: alerts of_the_last is one string, "10_MINUTES" or
// "90s", for an API oneOf of an enum and a duration. The override
//
//	customPackage: <import path>
//	types:
//	  MetricThresholdCondition:
//	    ofTheLast: {custom: {type: String, shape: 1a2b3c4d5e6f}}
//
// keeps the handwritten attribute and converters of that one field. The
// generated code calls, in the custom package, for the name
// <component><Field> (MetricThresholdConditionOfTheLast):
//
//	func <name>Attribute() schema.Attribute
//	func Expand<name>(p path.Path, v types.<type>, diags *diag.Diagnostics) <the SDK field type>
//	func Flatten<name>(p path.Path, v *<the SDK type>, diags *diag.Diagnostics) types.<type>
//
// The custom package must not import the generated one. shape is the start
// of the SHA-256 of the API type of the field (model.TypeText): when the API
// type changes, the generator stops and shows the new type, so the
// handwritten converter cannot miss a change.

// customOverride is the custom override of one field.
type customOverride struct {
	Type  string `yaml:"type"`  // the Terraform value type, for example String
	Shape string `yaml:"shape"` // shapeOf the API type
}

// customTypes are the Terraform value types of a custom field.
var customTypes = []string{"Bool", "Float32", "Float64", "Int32", "Int64", "Number", "String"}

// shapeOf returns the first 12 hex digits of the SHA-256 of the text of t.
func shapeOf(t *model.Type) string {
	sum := sha256.Sum256([]byte(model.TypeText(t)))
	return hex.EncodeToString(sum[:])[:12]
}

// customName is the Go name of the custom field f of the component schema.
func customName(schema string, f *model.Field) string { return camelize(schema) + camelize(f.Name) }

// checkShapeOverride checks the overrides that change the shape of a field
// instead of its attribute: inline, custom, and namesArm. ok is false for
// another override.
func (o *overrides) checkShapeOverride(t *model.Type, f *model.Field) (ok bool, err error) {
	ov := o.field(t.Schema, f.Name)
	switch {
	case ov.Inline:
		return true, o.checkInline(t, f)
	case ov.Custom != nil:
		return true, o.checkCustom(t, f)
	case ov.NamesArm:
		return true, o.checkNamesArm(t, f)
	}
	return false, nil
}

func (o *overrides) checkCustom(t *model.Type, f *model.Field) error {
	ov := o.field(t.Schema, f.Name)
	isArm := func(g model.OneOfGroup) bool { return slices.Contains(g.Arms, f.Name) }
	switch {
	case !reflect.DeepEqual(ov, fieldOverride{Custom: ov.Custom, Name: ov.Name}):
		return errors.New("custom can only be combined with name: the handwritten attribute has the rest")
	case ov.Name != "" && !attrNamePattern.MatchString(ov.Name):
		return fmt.Errorf("name %q is not a valid Terraform attribute name", ov.Name)
	case t.Kind == model.OneOf || slices.ContainsFunc(t.Groups, isArm):
		return errors.New("custom: a oneOf arm cannot be custom")
	case !slices.Contains(customTypes, ov.Custom.Type):
		return fmt.Errorf("custom: type %q is not one of %v", ov.Custom.Type, customTypes)
	}
	if got := shapeOf(f.Type); ov.Custom.Shape != got {
		return fmt.Errorf("custom: the API type is not the one that the handwritten converter was written for (shape %s, the file has %q). "+
			"Check %s in the custom package, then set shape: %s. The API type is now:\n%s",
			got, ov.Custom.Shape, customName(t.Schema, f), got, model.TypeText(f.Type))
	}
	return nil
}

// checkCustomPackage fails when a custom field has no package, or the
// package has no custom field.
func (o *overrides) checkCustomPackage() []error {
	has := false
	for _, fields := range o.Types {
		for _, ov := range fields {
			has = has || ov.Custom != nil
		}
	}
	switch {
	case has && o.CustomPackage == "":
		return []error{errors.New("overrides: a custom field needs customPackage, the import path of its converters")}
	case !has && o.CustomPackage != "":
		return []error{errors.New("overrides: customPackage is set, but no field is custom")}
	}
	return nil
}

// customAttribute returns the handwritten attribute and the model field of
// the custom field f of the component schema.
func (b *tfBuilder) customAttribute(name, schema string, f *model.Field) (*tfAttr, tfModelField) {
	ov := b.ov.field(schema, f.Name)
	a := &tfAttr{Name: name, Custom: "custom." + customName(schema, f) + "Attribute()"}
	return a, tfModelField{Name: camelize(f.Name), Type: "types." + ov.Custom.Type, TFName: name}
}

// customField returns the conversion of the custom field f of the component
// schema, whose SDK struct is at owner. The SDK type of the field is the
// one of the handwritten converters; the Go compiler checks them.
func (b *convBuilder) customField(owner, schema string, f *model.Field) (*convField, error) {
	ref, err := b.ix.fieldRef(owner + "." + f.Name)
	if err != nil {
		return nil, err
	}
	ov := b.ov.effective(schema, f)
	return &convField{TFName: b.ov.tfName(schema, f.Name), Model: camelize(f.Name), SDK: ref.Name, Conv: convCustom,
		Custom: customName(schema, f), CustomType: ov.Custom.Type, Value: !strings.HasPrefix(ref.Want, "*"), ReadOnly: ov.ReadOnly}, nil
}

// Arms named by an enum (D21). The alerts API has one field for each alert
// type, of which one is set, and a type field that names it:
//
//	{"metricThreshold": {...}, "type": "ALERT_DEF_TYPE_METRIC_THRESHOLD"}
//
// The override
//
//	types:
//	  AlertDefProperties:
//	    type: {namesArm: true}
//
// makes type no attribute, and expand sets it from the set arm. The enum
// name rule (enumRuleName) must pair each arm with exactly one enum value,
// and each value with an arm: metricThreshold with
// ALERT_DEF_TYPE_METRIC_THRESHOLD, logsImmediate with
// ALERT_DEF_TYPE_LOGS_IMMEDIATE_OR_UNSPECIFIED. So a new arm without a
// value, or a new value without an arm, stops the generator.

// armNames is the namesArm field of a convObject.
type armNames struct {
	SDK   string // the SDK field of the enum
	Value bool   // the SDK field is a value, not a pointer
	Arms  []armName
}

type armName struct {
	SDK   string // the SDK field of the arm
	Const string // the qualified SDK constant of its enum value
}

func (o *overrides) checkNamesArm(t *model.Type, f *model.Field) error {
	if !reflect.DeepEqual(o.field(t.Schema, f.Name), fieldOverride{NamesArm: true}) {
		return errors.New("namesArm cannot be combined with other overrides: the field is not an attribute")
	}
	_, err := armValues(t, f)
	return err
}

// armValues returns the enum value of each arm of the only oneOf group of
// t, for the namesArm field f.
func armValues(t *model.Type, f *model.Field) (map[string]string, error) {
	if f.Type.Kind != model.Enum {
		return nil, fmt.Errorf("namesArm needs an enum, the field is a %s", f.Type.Kind)
	}
	if len(t.Groups) != 1 {
		return nil, fmt.Errorf("namesArm needs exactly one oneOf group in %s, it has %d", t.Schema, len(t.Groups))
	}
	values := map[string]string{} // Terraform name → enum value
	for _, v := range f.Type.Values {
		values[enumRuleName(f.Type, v)] = v
	}
	out := map[string]string{}
	var missing []string
	for _, arm := range t.Groups[0].Arms {
		v, ok := values[tfName(arm)]
		if !ok {
			missing = append(missing, "the arm "+arm+" has no value")
			continue
		}
		out[arm] = v
		delete(values, tfName(arm))
	}
	for _, name := range sortedKeys(values) {
		missing = append(missing, "the value "+values[name]+" has no arm")
	}
	if len(missing) != 0 {
		return nil, fmt.Errorf("namesArm: %s", strings.Join(missing, "; "))
	}
	return out, nil
}

// namesArm sets the namesArm field of obj, the object of t whose SDK struct
// is at owner, when t has one. Only the arms that the overrides do not skip
// are listed: the others are never set.
func (b *convBuilder) namesArm(t *model.Type, obj *convObject, owner string) error {
	for _, f := range t.Fields {
		if !b.ov.field(t.Schema, f.Name).NamesArm {
			continue
		}
		ref, err := b.ix.fieldRef(owner + "." + f.Name)
		if err != nil {
			return err
		}
		enum, err := b.ix.schemaRef(f.Type.Schema)
		if err != nil {
			return err
		}
		values, err := armValues(t, f)
		if err != nil {
			return err
		}
		obj.NamesArm = &armNames{SDK: ref.Name, Value: !strings.HasPrefix(ref.Want, "*")}
		for _, g := range b.ov.groups(t) {
			for _, arm := range g.Arms {
				aref, err := b.ix.fieldRef(owner + "." + arm)
				if err != nil {
					return err
				}
				if !strings.HasPrefix(aref.Want, "*") {
					return fmt.Errorf("namesArm: the SDK field %s is a value, so it is always set", aref.sdkName())
				}
				obj.NamesArm.Arms = append(obj.NamesArm.Arms, armName{SDK: aref.Name, Const: b.qualify(enumConstName(enum.Name, values[arm]))})
			}
		}
	}
	return nil
}
