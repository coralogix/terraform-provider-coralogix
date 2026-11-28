package generator

import (
	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

// objectValueSet holds the nested object fields of components that a place computes and another
// place writes, as "<component>.<field>". One model struct serves every place of a component, so
// the field needs one Go type in all of them: a types.Object, which holds null, unknown, and known
// values. The set is empty for a component whose places agree, so the field keeps its pointer.
type objectValueSet map[string]bool

// objectValueFields returns the nested object fields of r that need a types.Object because the
// places of their component disagree. It walks the same types as sentFields.
func objectValueFields(r *model.Resource, file *overrides.File) objectValueSet {
	places := map[string]*[2]bool{} // by field: a place computes it [0], a place writes it [1]
	for _, f := range r.Fields {
		collectObjectPlaces(f.Type, places)
	}
	if file != nil {
		for _, d := range file.Distributed {
			collectObjectPlaces(d.Field.Type, places)
		}
	}
	out := objectValueSet{}
	for key, seen := range places {
		if seen[0] && seen[1] {
			out[key] = true
		}
	}
	return out
}

func collectObjectPlaces(t *model.Type, places map[string]*[2]bool) {
	if t == nil {
		return
	}
	collectObjectPlaces(t.Elem, places)
	for _, f := range t.Fields {
		if t.Schema != "" && (f.Type.Kind == model.Object || f.Type.Kind == model.OneOf) {
			key := t.Schema + "." + f.Name
			if places[key] == nil {
				places[key] = &[2]bool{}
			}
			if f.Behavior == model.Computed || f.Attrs.Computed {
				places[key][0] = true
			} else {
				places[key][1] = true
			}
		}
		collectObjectPlaces(f.Type, places)
	}
}

// has reports whether the field of the component is a types.Object in every place.
func (s objectValueSet) has(component, field string) bool { return s[component+"."+field] }
