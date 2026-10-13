package model

import (
	"fmt"
	"slices"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	"github.com/pb33f/libopenapi/orderedmap"
	"go.yaml.in/yaml/v4"
)

// foldCreateBody returns the Create body as the resource sees it when a createAs line folds
// top-level Create fields into an object field. A released API can take a value flat on Create
// and inside an object on Update and Get, for example permutationsLimit and permutations.limit.
// The body loses the flat fields and gains the object field, whose properties are the flat
// fields under their names in the object. Validation and Build both read the body this way, so
// the object field has the normal lifecycle and its properties are matched as usual.
func (p Policy) foldCreateBody(component string, body *base.Schema) (*base.Schema, error) {
	if body == nil || len(p.CreateAs) == 0 {
		return body, nil
	}
	folds := p.foldsOf(component)
	if len(folds) == 0 {
		return body, nil
	}
	out := *body
	out.Properties = orderedmap.New[string, *base.SchemaProxy]()
	flat := map[string]bool{}
	for _, f := range folds {
		for _, name := range f.flat() {
			flat[name] = true
		}
	}
	for name, proxy := range body.Properties.FromOldest() {
		if !flat[name] {
			out.Properties.Set(name, proxy)
		}
	}
	out.Required = slices.DeleteFunc(slices.Clone(body.Required), func(name string) bool { return flat[name] })
	for _, f := range folds {
		if propertyOf(body, f.field) != nil {
			return nil, fmt.Errorf("createAs on %s.%s: the Create body already has %q", component, f.field, f.field)
		}
		object := &base.Schema{
			Type:       []string{"object"},
			Properties: orderedmap.New[string, *base.SchemaProxy](),
			Extensions: orderedmap.New[string, *yaml.Node](),
		}
		for _, nested := range f.nested() {
			name := f.into[nested]
			proxy := propertyOf(body, name)
			if proxy == nil {
				return nil, fmt.Errorf("createAs on %s.%s: the Create body has no field %q", component, f.field, name)
			}
			object.Properties.Set(nested, proxy)
			if slices.Contains(body.Required, name) {
				object.Required = append(object.Required, nested)
			}
		}
		out.Properties.Set(f.field, base.CreateSchemaProxy(object))
	}
	return &out, nil
}

// fold is one createAs line: Create sends the property nested of the object field as the
// top-level Create field into[nested].
type fold struct {
	field string
	into  map[string]string
}

func (f fold) nested() []string {
	out := make([]string, 0, len(f.into))
	for nested := range f.into {
		out = append(out, nested)
	}
	slices.Sort(out)
	return out
}

func (f fold) flat() []string {
	out := make([]string, 0, len(f.into))
	for _, name := range f.into {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

// foldsOf returns the createAs lines of the top-level fields of component, in field order.
func (p Policy) foldsOf(component string) []fold {
	var out []fold
	for key, into := range p.CreateAs {
		comp, field, ok := cutKey(key)
		if ok && comp == component {
			out = append(out, fold{field: field, into: into})
		}
	}
	slices.SortFunc(out, func(a, b fold) int {
		if a.field < b.field {
			return -1
		}
		if a.field > b.field {
			return 1
		}
		return 0
	})
	return out
}

// CreateAsOf returns how Create sends the properties of a top-level object field: the flat
// Create field of each property. It is nil for a field without a createAs line.
func (p Policy) CreateAsOf(component, field string) map[string]string {
	return p.CreateAs[component+"."+field]
}

func cutKey(key string) (string, string, bool) {
	for i := len(key) - 1; i >= 0; i-- {
		if key[i] == '.' {
			return key[:i], key[i+1:], true
		}
	}
	return "", "", false
}

// CreateFold is a property of an object field that Create sends as a top-level field.
type CreateFold struct {
	Property string // the property of the object, for example limit
	Field    string // the top-level Create field, for example permutationsLimit
	Type     *Type  // the type of the property in the Create request
	Required bool   // the Create request requires the top-level field
}

// CreateFolds returns the properties of f that Create sends as top-level fields, in the order
// of the object. It is nil for a field without a createAs line.
func (r *Resource) CreateFolds(f *ResourceField) []CreateFold {
	into := r.Policy.CreateAsOf(r.Name, f.Name)
	if len(into) == 0 || f.Create == nil {
		return nil
	}
	var out []CreateFold
	for _, nf := range f.Type.CreateType().Fields {
		if flat, ok := into[nf.Name]; ok {
			out = append(out, CreateFold{Property: nf.Name, Field: flat, Type: nf.Type, Required: nf.Attrs.Required})
		}
	}
	return out
}
