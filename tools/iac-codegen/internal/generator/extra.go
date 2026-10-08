package generator

import (
	"fmt"
	"slices"
	"sort"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

type extraFlatten struct {
	Name string // Go field
	Expr string // typed null, for example types.MapNull(types.StringType)
}

func extraElem(elementType string) (tfType, nullExpr string, err error) {
	switch elementType {
	case "string":
		return "types.StringType", "types.MapNull(types.StringType)", nil
	case "int64":
		return "types.Int64Type", "types.MapNull(types.Int64Type)", nil
	default:
		return "", "", fmt.Errorf("elementType is %q, want string or int64", elementType)
	}
}

// attachExtraAttributes adds the extraAttributes of the behavior-overrides file
// to the schema, the model, and flatten. They are not API fields.
func attachExtraAttributes(out *tfResource, file *overrides.File) error {
	if file == nil {
		return nil
	}
	for _, component := range extraComponents(file) {
		extras := file.Types[component].ExtraAttributes
		parent, model, err := extraParent(out, file.Resource, component)
		if err != nil {
			return err
		}
		for _, name := range extraNames(extras) {
			if attrNamed(parent, name) || fieldNamed(model, name) {
				return fmt.Errorf("types.%s.extraAttributes.%s: the generated object already has this attribute", component, name)
			}
			extra := extras[name]
			elem, nullExpr, err := extraElem(extra.ElementType)
			if err != nil {
				return fmt.Errorf("types.%s.extraAttributes.%s: %w", component, name, err)
			}
			parent = append(parent, &tfAttr{
				Name:        name,
				Kind:        "Map",
				ValueKind:   "Map",
				Optional:    true,
				WriteOnly:   extra.WriteOnly,
				ElementType: elem,
				Description: extra.MarkdownDescription,
			})
			model.Fields = append(model.Fields, tfModelField{
				Name:   camelize(name),
				Type:   "types.Map",
				TFName: name,
			})
			if err := attachConvExtra(out.Conv, model.Name, name, camelize(name), elem, nullExpr); err != nil {
				return fmt.Errorf("types.%s.extraAttributes.%s: %w", component, name, err)
			}
		}
		if err := setExtraParent(out, file.Resource, component, parent); err != nil {
			return err
		}
	}
	return nil
}

func extraComponents(file *overrides.File) []string {
	var names []string
	for name, t := range file.Types {
		if len(t.ExtraAttributes) != 0 {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func extraNames(extras map[string]overrides.ExtraAttribute) []string {
	names := make([]string, 0, len(extras))
	for name := range extras {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func extraParent(out *tfResource, resource, component string) ([]*tfAttr, *tfModel, error) {
	model := extraModel(out, component)
	if model == nil {
		return nil, nil, fmt.Errorf("types.%s.extraAttributes: no generated model for this component", component)
	}
	if component == resource {
		return out.Attributes, model, nil
	}
	attrs := nestedAttrsOf(out.Attributes, component)
	if attrs == nil {
		return nil, nil, fmt.Errorf("types.%s.extraAttributes: no generated nested object for this component", component)
	}
	return attrs, model, nil
}

func setExtraParent(out *tfResource, resource, component string, attrs []*tfAttr) error {
	if component == resource {
		out.Attributes = attrs
		return nil
	}
	if setNestedAttrs(out.Attributes, component, attrs) {
		return nil
	}
	return fmt.Errorf("types.%s.extraAttributes: no generated nested object for this component", component)
}

func extraModel(out *tfResource, component string) *tfModel {
	name := modelTypeName(component)
	for _, m := range out.Models {
		if m.Name == name {
			return m
		}
	}
	return nil
}

func nestedAttrsOf(attrs []*tfAttr, component string) []*tfAttr {
	for _, a := range attrs {
		if len(a.Attributes) > 0 && a.Attributes[0].Component == component {
			return a.Attributes
		}
		if found := nestedAttrsOf(a.Attributes, component); found != nil {
			return found
		}
	}
	return nil
}

func setNestedAttrs(attrs []*tfAttr, component string, next []*tfAttr) bool {
	for _, a := range attrs {
		if len(a.Attributes) > 0 && a.Attributes[0].Component == component {
			a.Attributes = next
			return true
		}
		if setNestedAttrs(a.Attributes, component, next) {
			return true
		}
	}
	return false
}

func attrNamed(attrs []*tfAttr, name string) bool {
	return slices.ContainsFunc(attrs, func(a *tfAttr) bool { return a.Name == name })
}

func fieldNamed(model *tfModel, name string) bool {
	return slices.ContainsFunc(model.Fields, func(f tfModelField) bool { return f.TFName == name })
}

func attachConvExtra(conv *convData, model, tfName, goName, elem, nullExpr string) error {
	if conv == nil {
		return nil
	}
	var matched bool
	for _, obj := range conv.Objects {
		if obj.Model != model || !obj.Flatten {
			continue
		}
		matched = true
		if obj.AttrTypes != nil {
			obj.AttrTypes = append(obj.AttrTypes, convAttrType{
				TFName: tfName,
				Expr:   "types.MapType{ElemType: " + elem + "}",
			})
		}
		obj.ExtraFlatten = append(obj.ExtraFlatten, extraFlatten{Name: goName, Expr: nullExpr})
	}
	if !matched {
		return fmt.Errorf("no flatten conversion for model %s", model)
	}
	return nil
}
