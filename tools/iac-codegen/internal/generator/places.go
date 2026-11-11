package generator

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

// placeVariant is the variant of the model struct of a component at a place: the parts of the API
// path of the holder, so that two places have two model structs.
func placeVariant(place string) string {
	var out strings.Builder
	out.WriteString("At")
	for _, part := range strings.Split(place, ".") {
		out.WriteString(camelize(part))
	}
	return out.String()
}

// placedType returns t with the variant of its place when a types.<Type>.at line names the holder:
// the component and field that hold t, written Component.field. A list, set, or map of such objects
// gets the variant on its items. The component then has its own model struct and its own names at
// that place.
func placedType(file *overrides.File, t *model.Type, holder string) *model.Type {
	if file == nil || t == nil {
		return t
	}
	switch t.Kind {
	case model.List, model.Set, model.Map:
		if elem := placedType(file, t.Elem, holder); elem != t.Elem {
			c := *t
			c.Elem = elem
			return &c
		}
	case model.Object, model.OneOf:
		if _, ok := file.Types[t.Schema].At[holder]; ok && t.Variant == "" {
			c := *t
			c.Variant = placeVariant(holder)
			return &c
		}
	}
	return t
}

// placeFieldName returns the name that a types.<Type>.at line gives the field of the object t at
// its place, and whether there is one.
func placeFieldName(file *overrides.File, t *model.Type, field string) (string, bool) {
	if file == nil || t == nil || t.Variant == "" {
		return "", false
	}
	for place, line := range file.Types[t.Schema].At {
		if placeVariant(place) != t.Variant {
			continue
		}
		if f, ok := line.Fields[field]; ok {
			return f.Name, true
		}
	}
	return "", false
}

// markPlace records that the builder found the place of a types.<Type>.at line.
func (b *tfBuilder) markPlace(place string) {
	if b.places == nil {
		b.places = map[string]bool{}
	}
	b.places[place] = true
}

// checkPlacesUsed reports a types.<Type>.at line whose holder the resource does not have, such as a
// misspelled component or field: the names of the line would be dropped without a word.
func (b *tfBuilder) checkPlacesUsed() error {
	if b.file == nil {
		return nil
	}
	for _, name := range slices.Sorted(maps.Keys(b.file.Types)) {
		for _, place := range slices.Sorted(maps.Keys(b.file.Types[name].At)) {
			if !b.places[place] {
				return fmt.Errorf("types.%s.at.%s: no field of that holder holds %s", name, place, name)
			}
		}
	}
	return nil
}

// checkPlaceFields reports a field of an at line that the object t does not have, for t at a place.
func checkPlaceFields(file *overrides.File, t *model.Type) error {
	if file == nil || t.Variant == "" {
		return nil
	}
	for place, line := range file.Types[t.Schema].At {
		if placeVariant(place) != t.Variant {
			continue
		}
		for field := range line.Fields {
			if !slices.ContainsFunc(t.Fields, func(f *model.Field) bool { return f.Name == field }) {
				return fmt.Errorf("types.%s.at.%s.fields.%s: %s has no field %s", t.Schema, place, field, t.Schema, field)
			}
		}
	}
	return nil
}
