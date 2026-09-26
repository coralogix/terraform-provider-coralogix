package main

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/model"
)

// Unwrapped objects (D21). Many API objects have only one field, value, and
// handwritten resources show them as that field: the dashboards API sends
// {"luceneQuery": {"value": "status:500"}}, and Terraform has
// lucene_query = "status:500". The overrides line
//
//	unwrap: [LuceneQuery]
//
// makes every field of type LuceneQuery, and every list or set of it, an
// attribute of the type of LuceneQuery.value. The attribute keeps the name,
// the description, the flags, and the overrides of the field that uses the
// object; its type and validators come from value. Rules:
//   - expand: a null value sends no object; a value sends {"value": ...}.
//   - flatten: a missing object reads as an object with no value, so both
//     are null. The read overrides of value (missingAsZero, emptyAsNull)
//     apply to both.

// unwrapped reports whether the object schema is shown as its only field.
func (o *overrides) unwrapped(schema string) bool {
	return o != nil && schema != "" && slices.Contains(o.Unwrap, schema)
}

// unwrap returns the Terraform type of t: the type of the only field of an
// unwrapped object, or a list or set of it. Other types do not change.
func (o *overrides) unwrap(t *model.Type) *model.Type {
	switch {
	case t.Kind == model.Object && o.unwrapped(t.Schema):
		return o.unwrap(o.fieldType(t.Schema, t.Fields[0]))
	case (t.Kind == model.List || t.Kind == model.Set) && t.Elem.Kind == model.Object && o.unwrapped(t.Elem.Schema):
		c := *t
		c.Elem = o.unwrap(t.Elem)
		return &c
	}
	return t
}

// tfType returns the Terraform type of the field f of the component schema:
// fieldType, then unwrap.
func (o *overrides) tfType(schema string, f *model.Field) *model.Type {
	return o.unwrap(o.fieldType(schema, f))
}

// usesUnwrapped returns the unwrapped object that the type t is, or that
// its items are, or "".
func (o *overrides) usesUnwrapped(t *model.Type) string {
	if (t.Kind == model.List || t.Kind == model.Set) && t.Elem != nil {
		t = t.Elem
	}
	if t.Kind == model.Object && o.unwrapped(t.Schema) {
		return t.Schema
	}
	return ""
}

// checkUsedField fails for a read override on a field of an unwrapped type:
// the read overrides of the value apply to every field of that type, so
// they are on the field of the unwrapped object.
func (o *overrides) checkUsedField(ov fieldOverride, f *model.Field) error {
	if s := o.usesUnwrapped(f.Type); s != "" && (ov.MissingAsZero || ov.EmptyAsNull) {
		return fmt.Errorf("%s is unwrapped: put missingAsZero and emptyAsNull on types.%s.%s", s, s, f.Type.Fields[0].Name)
	}
	return nil
}

// checkUnwrap checks the unwrap list: each entry is an object of the
// generated types with exactly one field, and not a root. The field of an
// unwrapped object can only have the overrides set, missingAsZero, and
// emptyAsNull: the others are on the fields that use the object.
func (o *overrides) checkUnwrap(roots []*model.Type, objects map[string]*model.Type) []error {
	var errs []error
	seen := map[string]bool{}
	for _, schema := range o.Unwrap {
		at := "overrides: unwrap." + schema
		t, ok := objects[schema]
		switch {
		case seen[schema]:
			errs = append(errs, fmt.Errorf("%s: listed twice", at))
			continue
		case !ok:
			errs = append(errs, fmt.Errorf("%s: no such object in the generated types", at))
			continue
		case slices.ContainsFunc(roots, func(r *model.Type) bool { return r.Schema == schema }):
			errs = append(errs, fmt.Errorf("%s: a root type cannot be unwrapped", at))
			continue
		case t.Kind != model.Object || len(t.Groups) != 0:
			errs = append(errs, fmt.Errorf("%s: a oneOf cannot be unwrapped", at))
			continue
		case len(t.Fields) != 1:
			errs = append(errs, fmt.Errorf("%s: the object has %d fields, unwrap needs exactly one", at, len(t.Fields)))
			continue
		}
		seen[schema] = true
		for _, name := range sortedKeys(o.Types[schema]) {
			ov := o.Types[schema][name]
			if !reflect.DeepEqual(ov, fieldOverride{Set: ov.Set, MissingAsZero: ov.MissingAsZero, EmptyAsNull: ov.EmptyAsNull}) {
				errs = append(errs, fmt.Errorf("overrides: types.%s.%s: the field of an unwrapped object can only have set, missingAsZero, and emptyAsNull", schema, name))
			}
		}
	}
	return errs
}
