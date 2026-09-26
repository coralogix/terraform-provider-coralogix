package main

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/model"
)

// wrapper is a Terraform object that the overrides add around API fields
// of one object (D21). Handwritten resources often put the arms of a proto
// oneof in an object named after the oneof, which the spec does not name
// (F62):
//
//	sli: {wrap: [requestBasedMetricSli, windowBasedMetricSli, apmSli], required: true}
//
// makes sli = { request_based_metric_sli = ..., ... }. The wrapped fields
// keep their own overrides. Expand sets them on the same API object.
type wrapper struct {
	Name   string   // the Terraform attribute, the override key
	Fields []string // the wrapped API fields, in API order
	Ov     fieldOverride
}

// wrappers returns the wrappers of t, ordered by their first field.
func (o *overrides) wrappers(t *model.Type) []wrapper {
	if o == nil {
		return nil
	}
	pos := map[string]int{}
	for i, f := range t.Fields {
		pos[f.Name] = i
	}
	var out []wrapper
	for _, name := range sortedKeys(o.Types[t.Schema]) {
		ov := o.Types[t.Schema][name]
		if len(ov.Wrap) == 0 {
			continue
		}
		fields := slices.Clone(ov.Wrap)
		sort.Slice(fields, func(i, j int) bool { return pos[fields[i]] < pos[fields[j]] })
		out = append(out, wrapper{Name: name, Fields: fields, Ov: ov})
	}
	sort.Slice(out, func(i, j int) bool { return pos[out[i].Fields[0]] < pos[out[j].Fields[0]] })
	return out
}

// wrapperOf returns the wrapper of the API field name of t, or "".
func (o *overrides) wrapperOf(t *model.Type, name string) string {
	for _, w := range o.wrappers(t) {
		if slices.Contains(w.Fields, name) {
			return w.Name
		}
	}
	return ""
}

// checkWrappers checks the wrappers of the object t.
func (o *overrides) checkWrappers(t *model.Type) []error {
	var errs []error
	fields := map[string]*model.Field{}
	for _, f := range t.Fields {
		fields[f.Name] = f
	}
	owner := map[string]string{}
	for _, w := range o.wrappers(t) {
		at := fmt.Sprintf("overrides: types.%s.%s", t.Schema, w.Name)
		if err := checkWrapper(w, t, fields, owner, o); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", at, err))
		}
	}
	for _, g := range o.groups(t) {
		in := map[string]bool{}
		for _, a := range g.Arms {
			in[owner[a]] = true
		}
		if len(in) > 1 {
			errs = append(errs, fmt.Errorf("overrides: types.%s: the oneOf arms %v must be in one wrapper or in none", t.Schema, g.Arms))
		}
	}
	return errs
}

func checkWrapper(w wrapper, t *model.Type, fields map[string]*model.Field, owner map[string]string, o *overrides) error {
	switch {
	case t.Kind != model.Object:
		return fmt.Errorf("wrap needs an object, %s is a %s", t.Schema, t.Kind)
	case fields[w.Name] != nil:
		return errors.New("wrap: the key must be a new attribute name, not an API field")
	case !attrNamePattern.MatchString(w.Name):
		return fmt.Errorf("%q is not a valid Terraform attribute name", w.Name)
	case !reflect.DeepEqual(w.Ov, fieldOverride{Wrap: w.Ov.Wrap, Required: w.Ov.Required, Optional: w.Ov.Optional, Computed: w.Ov.Computed}):
		return errors.New("a wrapper can only set wrap, required, optional, and computed")
	}
	for _, name := range w.Ov.Wrap {
		switch {
		case fields[name] == nil:
			return fmt.Errorf("wrap: no field %s", name)
		case o.field(t.Schema, name).Skip:
			return fmt.Errorf("wrap: %s is skipped", name)
		case owner[name] != "":
			return fmt.Errorf("wrap: %s is also in %s", name, owner[name])
		}
		owner[name] = w.Name
	}
	if w.Ov.Computed != nil && *w.Ov.Computed {
		return errors.New("a computed wrapper is not supported")
	}
	return checkFlags(w.Ov, model.Attrs{})
}
