package main

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/model"
)

// Inlined objects (D21, F67). Handwritten resources often show the fields of
// a nested API object as fields of the parent: the API sends
// {"name": "k", "keyPermissions": {"permissions": ["a"]}}, and Terraform has
// name = "k" and permissions = ["a"]. The override is on the field that
// holds the object, so each parent decides:
//
//	types: {KeyInfo: {keyPermissions: {inline: true}}}
//
// The inlined fields keep their own overrides, keyed by the nested component
// (types: {KeyInfo.KeyPermissions: {permissions: {set: true}}}). Rules:
//   - expand: the nested object is sent when at least one inlined attribute
//     is set (not null or unknown); else it is not sent.
//   - flatten: a missing object reads as an object with no fields, so each
//     inlined attribute is null, or what its read override says.
//   - An inlined field is required only when the holding field is required
//     too: else the object, and so the field, can be missing.

// inlined reports whether the field f of the component schema is inlined.
func (o *overrides) inlined(schema string, f *model.Field) bool {
	return o != nil && o.field(schema, f.Name).Inline
}

// inlinedFields returns the fields of the object that holder holds, without
// the skipped ones, as the parent sees them (see the rules).
func (o *overrides) inlinedFields(holder *model.Field) []*model.Field {
	var out []*model.Field
	for _, f := range o.fields(holder.Type) {
		c := *f
		c.Attrs.Required = f.Attrs.Required && holder.Attrs.Required
		out = append(out, &c)
	}
	return out
}

// attrName is one attribute of an object after the overrides, for the name
// checks: its Terraform name, its model Go name, and the API field for the
// error.
type attrName struct{ tf, goName, label string }

// attrNames returns the attributes that the field f of t becomes: one, or
// the inlined fields.
func (o *overrides) attrNames(t *model.Type, f *model.Field) []attrName {
	if !o.inlined(t.Schema, f) || f.Type.Kind != model.Object {
		return []attrName{{o.tfName(t.Schema, f.Name), camelize(f.Name), f.Name}}
	}
	var out []attrName
	for _, nf := range o.fields(f.Type) {
		out = append(out, attrName{o.tfName(f.Type.Schema, nf.Name), camelize(nf.Name), f.Name + "." + nf.Name})
	}
	return out
}

// checkInline checks the inline override of the field f of the object t.
func (o *overrides) checkInline(t *model.Type, f *model.Field) error {
	nt := f.Type
	switch {
	case !reflect.DeepEqual(o.field(t.Schema, f.Name), fieldOverride{Inline: true}):
		return errors.New("inline cannot be combined with other overrides: the inlined fields have their own")
	case t.Kind == model.OneOf || slices.ContainsFunc(t.Groups, func(g model.OneOfGroup) bool { return slices.Contains(g.Arms, f.Name) }):
		return errors.New("inline: a oneOf arm cannot be inlined")
	case o.wrapperOf(t, f.Name) != "":
		return errors.New("inline: a wrapped field cannot be inlined")
	case f.Attrs.ReadOnly:
		return errors.New("inline: a read-only field cannot be inlined")
	case nt.Kind != model.Object:
		return fmt.Errorf("inline needs an object, the field is a %s", nt.Kind)
	case o.unwrapped(nt.Schema):
		return fmt.Errorf("inline: %s is unwrapped", nt.Schema)
	case len(o.fields(nt)) == 0:
		return fmt.Errorf("inline: %s has no fields", nt.Schema)
	case len(nt.Groups) != 0:
		return fmt.Errorf("inline: %s has oneOf groups, which is not supported", nt.Schema)
	case len(o.wrappers(nt)) != 0:
		return fmt.Errorf("inline: %s has wrappers, which is not supported", nt.Schema)
	case slices.ContainsFunc(nt.Fields, func(nf *model.Field) bool { return o.inlined(nt.Schema, nf) }):
		return fmt.Errorf("inline: %s has an inlined field, which is not supported", nt.Schema)
	}
	return nil
}

// checkNames fails when two attributes of the object t, after renames and
// inlined objects, have one Terraform name or one model Go name. The
// wrappers are attributes too.
func (o *overrides) checkNames(t *model.Type) []error {
	var errs []error
	seen, seenGo := map[string]string{}, map[string]string{}
	for _, w := range o.wrappers(t) {
		seen[w.Name] = "the wrapper " + w.Name
	}
	for _, f := range o.fields(t) {
		for _, a := range o.attrNames(t, f) {
			if prev, ok := seen[a.tf]; ok {
				errs = append(errs, fmt.Errorf("overrides: types.%s: %s and %s both have the Terraform name %q", t.Schema, prev, a.label, a.tf))
			} else if prev, ok := seenGo[a.goName]; ok {
				errs = append(errs, fmt.Errorf("overrides: types.%s: %s and %s both have the model field %s", t.Schema, prev, a.label, a.goName))
			}
			seen[a.tf], seenGo[a.goName] = a.label, a.label
		}
	}
	return errs
}

// inlineAttributes returns the attributes and the model fields that the
// inlined field holder adds to the parent at the path p.
func (b *tfBuilder) inlineAttributes(p attrPath, holder *model.Field) (fieldParts, error) {
	var out fieldParts
	for _, f := range b.ov.inlinedFields(holder) {
		name := b.ov.tfName(holder.Type.Schema, f.Name)
		a, mf, err := b.field(append(append(attrPath{}, p...), name), holder.Type.Schema, f)
		if err != nil {
			return out, fmt.Errorf("%s: %w", f.Name, err)
		}
		out.attrs, out.fields = append(out.attrs, a), append(out.fields, mf)
	}
	return out, nil
}

// inline returns the conversion of the inlined field holder of parent,
// whose SDK struct is at owner (an SDK name path). Its object has the
// parent model and the SDK struct of the nested object.
func (b *convBuilder) inline(parent *convObject, owner string, holder *model.Field) (*convField, error) {
	ref, err := b.ix.fieldRef(owner + "." + holder.Name)
	if err != nil {
		return nil, err
	}
	nested, err := b.ix.schemaRef(holder.Type.Schema)
	if err != nil {
		return nil, err
	}
	cf := &convField{TFName: tfName(holder.Name), SDK: ref.Name, Conv: convInline}
	switch ref.Want {
	case "*" + nested.Name:
	case nested.Name:
		cf.Value = true
	default:
		return nil, fmt.Errorf("SDK field %s has type %s, the inline conversion needs *%s", ref.sdkName(), ref.Want, nested.Name)
	}
	obj := &convObject{Func: parent.Func + "Inline" + camelize(holder.Name), Model: parent.Model, SDK: nested.Name, Inline: true}
	var unset []string
	for _, f := range b.ov.inlinedFields(holder) {
		icf, err := b.objectField(nested.Path, holder.Type.Schema, f)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		obj.Fields = append(obj.Fields, icf)
		if !icf.ReadOnly {
			unset = append(unset, unsetCheck(icf))
		}
	}
	obj.UnsetCheck = "true"
	if len(unset) != 0 {
		obj.UnsetCheck = strings.Join(unset, " && ")
	}
	b.objects = append(b.objects, obj)
	cf.Object = obj
	return cf, nil
}

// unsetCheck is the Go condition that is true when expand does not send the
// model field of cf: nil for a model struct pointer, else null or unknown.
func unsetCheck(cf *convField) string {
	pointer := cf.Conv == convObj || cf.Conv == convEmpty || cf.Conv == convUnwrap && strings.HasPrefix(cf.Object.ValueType, "*")
	if pointer {
		return "m." + cf.Model + " == nil"
	}
	return "unset(m." + cf.Model + ")"
}

// withInlined returns the fields of obj with each inlined field replaced by
// the fields of its object: the attributes of the Terraform object.
func withInlined(obj *convObject) []*convField {
	var out []*convField
	for _, f := range obj.Fields {
		if f.Conv == convInline {
			out = append(out, f.Object.Fields...)
			continue
		}
		out = append(out, f)
	}
	return out
}
