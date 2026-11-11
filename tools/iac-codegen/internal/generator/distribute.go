package generator

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

// distributeFields applies the distributeTo lines of the file to the resource. A distributed list is
// no attribute of the resource: its items are attributes of the component that they name. So the
// resource loses the list, and the list items lose the field that names the component (the
// generated code sets and reads it). Both go to file.Distributed, which the schema builder and the
// conversion builder read. It runs after the SDK names are resolved, because those names still need
// the list and the field.
func distributeFields(r *model.Resource, file *overrides.File) error {
	if file == nil {
		return nil
	}
	file.Distributed = nil
	for _, component := range slices.Sorted(maps.Keys(file.Types)) {
		for _, field := range slices.Sorted(maps.Keys(file.Types[component].Fields)) {
			if file.Types[component].Fields[field].DistributeTo != nil && component != r.Name {
				return fmt.Errorf("types.%s.fields.%s: distributeTo works on a top-level field of the resource %s", component, field, r.Name)
			}
		}
	}
	for _, name := range file.DistributedFields(r.Name) {
		line := *file.Types[r.Name].Fields[name].DistributeTo
		d, err := distribution(r, file, name, line)
		if err != nil {
			return fmt.Errorf("types.%s.fields.%s: %w", r.Name, name, err)
		}
		file.Distributed = append(file.Distributed, d)
	}
	for _, d := range file.Distributed {
		r.Fields = slices.DeleteFunc(r.Fields, func(f *model.ResourceField) bool { return f == d.Field })
		d.Field.Type.Elem.Fields = slices.DeleteFunc(d.Field.Type.Elem.Fields, func(f *model.Field) bool { return f == d.Hidden })
	}
	return nil
}

// distribution checks one distributeTo line against the resource and returns what it moves. It
// changes nothing.
func distribution(r *model.Resource, file *overrides.File, name string, line overrides.Distribute) (*overrides.Distribution, error) {
	i := slices.IndexFunc(r.Fields, func(f *model.ResourceField) bool { return f.Name == name })
	if i < 0 {
		return nil, fmt.Errorf("the resource %s has no field %s", r.Name, name)
	}
	f := r.Fields[i]
	if err := checkDistributedList(r, f); err != nil {
		return nil, err
	}
	action := f.Type.Elem
	hidden, err := stringField(action, line.By.ListField, "by.listField")
	if err != nil {
		return nil, err
	}
	others := slices.Delete(slices.Clone(r.Fields), i, i+1)
	item := componentType(others, line.Item)
	if item == nil {
		return nil, fmt.Errorf("item %s is no component that the resource holds beside %s", line.Item, name)
	}
	if componentType(others, action.Schema) != nil {
		return nil, fmt.Errorf("%s is held in other places too, and distributeTo takes %s out of its items", action.Schema, line.By.ListField)
	}
	if _, err := stringField(item, line.By.ItemField, "by.itemField"); err != nil {
		return nil, err
	}
	for _, f := range item.Fields {
		if terraformName(r.Policy, item.Schema, f.Name) == line.As {
			return nil, fmt.Errorf("%s has the attribute %s already, so as cannot name it", item.Schema, line.As)
		}
	}
	for _, d := range file.Distributed {
		if d.Line.Item == line.Item && d.Line.As == line.As {
			return nil, fmt.Errorf("%s gets the attribute %s from the field %s already", line.Item, line.As, d.Field.Name)
		}
	}
	return &overrides.Distribution{Line: line, Field: f, Hidden: hidden}, nil
}

// checkDistributedList checks that f is a list of objects that a request sends, which an Update
// that replaces the resource sends whole.
func checkDistributedList(r *model.Resource, f *model.ResourceField) error {
	switch {
	case f.Type.Kind != model.List || f.Type.Elem == nil || f.Type.Elem.Kind != model.Object || f.Type.Elem.Schema == "":
		return fmt.Errorf("distributeTo needs a list of objects of a component, but %s is not one", f.Name)
	case len(f.Type.Wrappers) != 0 || f.Type.Projected != nil || f.Type.Keyed != nil:
		return fmt.Errorf("distributeTo needs a plain list of objects, but %s has wrappers, an itemName, or a keyedBy line", f.Name)
	case f.Behavior != model.Normal || f.Create == nil || f.Update == nil:
		return fmt.Errorf("distributeTo needs a list that Create and Update send and Get returns, but %s is %s", f.Name, f.Behavior)
	case f.Create.Required || f.Update.Required:
		return fmt.Errorf("distributeTo needs a list that Create and Update leave optional: a resource without items would send no list, but %s is required", f.Name)
	case !r.Replace:
		return fmt.Errorf("distributeTo needs an Update that replaces the whole resource, because %s holds the items of every component", f.Name)
	}
	return nil
}

// stringField returns the field of the object t that the key names, which must hold a string.
func stringField(t *model.Type, name, key string) (*model.Field, error) {
	i := slices.IndexFunc(t.Fields, func(f *model.Field) bool { return f.Name == name })
	if i < 0 {
		return nil, fmt.Errorf("%s: %s has no field %s", key, t.Schema, name)
	}
	f := t.Fields[i]
	if f.Type.Kind != model.String || isTime(f.Type) {
		return nil, fmt.Errorf("%s: the field %s of %s is no plain string", key, name, t.Schema)
	}
	return f, nil
}

// componentType returns the first object of the component schema in the types of the fields.
func componentType(fields []*model.ResourceField, schema string) *model.Type {
	seen := map[*model.Type]bool{}
	var walk func(t *model.Type) *model.Type
	walk = func(t *model.Type) *model.Type {
		if t == nil || seen[t] {
			return nil
		}
		seen[t] = true
		if t.Kind == model.Object && t.Schema == schema {
			return t
		}
		if found := walk(t.Elem); found != nil {
			return found
		}
		for _, f := range t.Fields {
			if found := walk(f.Type); found != nil {
				return found
			}
		}
		return nil
	}
	for _, f := range fields {
		if found := walk(f.Type); found != nil {
			return found
		}
	}
	return nil
}

// distributedItems returns the distributions of the file whose Item is the component schema.
func distributedItems(file *overrides.File, schema string) []*overrides.Distribution {
	if file == nil {
		return nil
	}
	var out []*overrides.Distribution
	for _, d := range file.Distributed {
		if d.Line.Item == schema {
			out = append(out, d)
		}
	}
	return out
}

// keepSteps splits a keep path into the API names of its steps.
func keepSteps(keep string) []string {
	if keep == "" {
		return nil
	}
	return strings.Split(keep, ".")
}

// distributedAttributes returns the attributes and the model fields that the distributeTo lines give
// the object t: the list of the items that name it. The attribute has the Terraform name of the line.
func (b *tfBuilder) distributedAttributes(p attrPath, t *model.Type) ([]*tfAttr, []tfModelField, error) {
	var attrs []*tfAttr
	var fields []tfModelField
	for _, d := range distributedItems(b.file, t.Schema) {
		f := d.Field
		child := append(append(attrPath{}, p...), d.Line.As)
		// The line of the list is the line of the attribute, so its texts apply to it.
		a, err := b.attribute(child, b.file.Resource, f.Name, "", f.Type, fieldAttrs(f), false)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		a.Name = d.Line.As
		field := b.modelField(b.file.Resource, f.Name, f.Type, f.Behavior, false, false)
		field.Name, field.TFName = camelize(d.Line.As), d.Line.As
		attrs, fields = append(attrs, a), append(fields, field)
	}
	return attrs, fields, nil
}
