package model

import (
	"fmt"
	"slices"
)

// unwrap replaces the type of each wrapper that the policy names with the type of the value
// inside it, and records the wrapper on the new type. It works from the inside out, so a
// wrapper inside a wrapper collapses first, and the outer type records both.
func (r *Resource) unwrap() error {
	u := &unwrapper{policy: r.Policy, used: map[string]bool{}}
	for _, f := range r.Fields {
		t, err := u.value(fieldLocation(r.Name, f.Name), r.Name, f.Name, f.Type)
		if err != nil {
			return err
		}
		if len(t.Wrappers) != 0 && f.Name == r.IDParam {
			return unwrapError(fieldLocation(r.Name, f.Name), "UNWRAP_COMPONENT_UNSUPPORTED",
				"The resource id cannot be inside a wrapper: the generated calls send it as a path parameter.",
				"Delete the unwrap line or the list entry that names the wrapper of the id.")
		}
		f.Type = t
	}
	r.UnwrapUsed = u.used
	return nil
}

type unwrapper struct {
	policy Policy
	used   map[string]bool
}

// value returns the type of the value of field of the owner component, at location at, after
// the wrappers inside it and the wrapper of the value itself, or of its items, collapse.
func (u *unwrapper) value(at, owner, field string, t *Type) (*Type, error) {
	if err := u.inside(at, t); err != nil {
		return nil, err
	}
	switch t.Kind {
	case List, Set:
		if !u.decide(owner, field, t.Elem.Schema) {
			return t, nil
		}
		elem, err := collapse(at+"[]", owner, field, t.Elem)
		if err != nil {
			return nil, err
		}
		out := *t
		out.Elem = elem
		return &out, nil
	case Map:
		if u.decide(owner, field, t.Elem.Schema) {
			return nil, unwrapError(at, "UNWRAP_COMPONENT_UNSUPPORTED", "A map of wrapped values is not supported.",
				"Delete the unwrap line or the list entry that names the wrapper of the map values.")
		}
		return t, nil
	}
	if !u.decide(owner, field, t.Schema) {
		return t, nil
	}
	return collapse(at, owner, field, t)
}

// inside collapses the wrappers of the fields of t, and of the items of a list, set, or map.
func (u *unwrapper) inside(at string, t *Type) error {
	switch t.Kind {
	case List, Set:
		return u.inside(at+"[]", t.Elem)
	case Map:
		return u.inside(at+"{}", t.Elem)
	case Object, OneOf:
	default:
		return nil
	}
	for _, f := range t.Fields {
		ft, err := u.value(at+"."+f.Name, t.Schema, f.Name, f.Type)
		if err != nil {
			return err
		}
		f.Type = ft
	}
	return nil
}

// decide reports whether the wrapper component of the value of field collapses, and records the
// line of the policy that decided it. A false field line decides only when the list names the
// component: otherwise the object stays without it.
func (u *unwrapper) decide(owner, field, component string) bool {
	key := owner + "." + field
	listed := component != "" && slices.Contains(u.policy.Unwrap, component)
	on, lined := u.policy.UnwrapFields[key]
	switch {
	case !u.policy.Existing:
		return false
	case lined:
		if on || listed {
			u.used[key] = true
		}
		return on
	case listed:
		u.used["unwrap."+component] = true
		return true
	}
	return false
}

// collapse returns the type of the value inside the wrapper w, with w recorded as its outer
// wrapper. The wrapper must be an object with exactly one property, and a request that sends
// the wrapper must send that property.
func collapse(at, owner, field string, w *Type) (*Type, error) {
	if w.Kind != Object || w.Schema == "" {
		return nil, unwrapError(at, "UNWRAP_FIELD_UNSUPPORTED",
			fmt.Sprintf("%s.%s holds %s, not an object component with one property, so it has no wrapper to remove.", owner, field, typeLabel(w)),
			"Delete the unwrap line of the field, or the list entry that names the component.")
	}
	if reason := wrapperProblem(w); reason != "" {
		return nil, unwrapError(at, "UNWRAP_COMPONENT_UNSUPPORTED",
			fmt.Sprintf("%s is not a wrapper: %s.", w.Schema, reason),
			"Delete the unwrap line or the list entry of the component.")
	}
	inner := w.Fields[0]
	out := *inner.Type
	out.Wrappers = append([]Wrapper{{
		Schema: w.Schema, CreateSchema: w.CreateSchema, UpdateSchema: w.UpdateSchema,
		Field: inner.Name, Attrs: inner.Attrs, Create: inner.Create, Update: inner.Update,
	}}, inner.Type.Wrappers...)
	return &out, nil
}

// wrapperProblem returns why w cannot collapse, or "".
func wrapperProblem(w *Type) string {
	switch {
	case len(w.Fields) != 1:
		return fmt.Sprintf("it has %d properties, want 1", len(w.Fields))
	case len(w.Groups) != 0 || w.Discriminator != "":
		return "it has a oneOf"
	}
	inner := w.Fields[0]
	switch {
	case inner.Attrs.Default != nil:
		return fmt.Sprintf("its property %s has a server default", inner.Name)
	case w.CreateSchema != "" && inner.Create == nil:
		return fmt.Sprintf("Create sends it without its property %s", inner.Name)
	case w.UpdateSchema != "" && inner.Update == nil:
		return fmt.Sprintf("Update sends it without its property %s", inner.Name)
	}
	return ""
}

func typeLabel(t *Type) string {
	if t.Schema != "" {
		return fmt.Sprintf("%s %s", t.Kind, t.Schema)
	}
	return string(t.Kind)
}

func unwrapError(at, code, message, remediation string) error {
	return &contractError{code: code, location: at, message: message, remediation: remediation}
}
