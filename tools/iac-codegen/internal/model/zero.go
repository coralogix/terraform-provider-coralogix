package model

// zeroDefaults decides how the generated resource reads a request scalar without presence and
// without a declared default. Proto3 does not send a zero scalar, and the API returns one, so
// omission and zero store the same value: zero. A field whose zero value is valid gets zero as
// its default in Create, Update, and Get, and the generated resource handles it as a declared
// server default. A field whose zero value is not valid reads zero as an omitted value. A field
// with a line in the behavior-overrides file keeps its released behavior.
func (r *Resource) zeroDefaults() {
	seen := map[*Type]bool{}
	for _, f := range r.Fields {
		if !r.Policy.released(r.Name, f.Name) {
			if get := f.Get; get != nil {
				zeroDefault(f.Type, f.Create, f.Update, get)
			}
		}
		r.nestedZeroDefaults(f.Type, seen)
	}
}

func (r *Resource) nestedZeroDefaults(t *Type, seen map[*Type]bool) {
	if t == nil || seen[t] {
		return
	}
	seen[t] = true
	switch t.Kind {
	case List, Set, Map:
		r.nestedZeroDefaults(t.Elem, seen)
		return
	case Object, OneOf:
	default:
		return
	}
	arms := map[string]bool{}
	for _, g := range t.Groups {
		for _, arm := range g.Arms {
			arms[arm] = true
		}
	}
	for _, f := range t.Fields {
		// A oneOf arm uses the presence of its union: a set arm is the selected one.
		if t.Kind != OneOf && !arms[f.Name] && !r.Policy.released(t.Schema, f.Name) {
			zeroDefault(f.Type, f.Create, f.Update, &f.Attrs)
		}
		r.nestedZeroDefaults(f.Type, seen)
	}
}

// zeroDefault applies zeroDefaults to one field: its type, and its attributes in Create, Update,
// and Get. A nil request attribute means that the request does not have the field.
func zeroDefault(t *Type, create, update, get *Attrs) {
	switch t.Kind {
	case Bool, Integer, Number, String:
	default:
		return // an enum reads its zero value as null already
	}
	if len(t.Wrappers) != 0 || create == nil && update == nil {
		return // a wrapper states presence, and a field that no request sends is computed
	}
	for _, a := range []*Attrs{create, update} {
		if a != nil && (a.Presence || a.Required || a.Default != nil) {
			return
		}
	}
	if get.Default != nil {
		return
	}
	if !t.ZeroValid {
		t.ReadZeroAsNull = true
		return
	}
	zero := t.Zero
	for _, a := range []*Attrs{create, update, get} {
		if a != nil {
			a.Default = &zero
		}
	}
	t.ImplicitDefault = true
}
