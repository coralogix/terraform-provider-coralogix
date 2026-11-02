package generator

import (
	"fmt"
	"slices"
	"sort"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

type extraFlatten struct {
	Name   string // Go field
	TFName string // Terraform attribute name
	Expr   string // typed null, for example types.MapNull(types.StringType)
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
	computedExtra := false
	for _, component := range extraComponents(file) {
		extras := file.Types[component].ExtraAttributes
		model, parents, err := extraParents(out, file.Resource, component)
		if err != nil {
			return err
		}
		for _, name := range extraNames(extras) {
			if fieldNamed(model, name) || extraAttrNamed(out, file.Resource, component, parents, name) {
				return fmt.Errorf("types.%s.extraAttributes.%s: the generated object already has this attribute", component, name)
			}
			extra := extras[name]
			computedExtra = computedExtra || extra.Computed
			attribute, field, attrType, nullExpr, err := extraAttr(name, extra)
			if err != nil {
				return fmt.Errorf("types.%s.extraAttributes.%s: %w", component, name, err)
			}
			if component == file.Resource {
				out.Attributes = append(out.Attributes, attribute)
			} else {
				inherited, err := addExtraToParents(parents, attribute, component, name)
				if err != nil {
					return err
				}
				computedExtra = computedExtra || inherited
			}
			model.Fields = append(model.Fields, field)
			if err := attachConvExtra(out.Conv, model.Name, name, camelize(name), attrType, nullExpr); err != nil {
				return fmt.Errorf("types.%s.extraAttributes.%s: %w", component, name, err)
			}
			if err := attachExtraBehavior(out, file.Resource, component, name, extra); err != nil {
				return err
			}
		}
	}
	if computedExtra {
		updateExtraPaths(out)
	}
	return nil
}

// buildTFResource computed these paths before extra attributes were attached.
func updateExtraPaths(out *tfResource) {
	out.ComputedPaths = computedPaths(nil, "", out.Attributes)
	out.ServerPaths = nil
	for _, a := range out.Attributes {
		if !computedOnly(a) {
			out.ServerPaths = serverPaths(out.ServerPaths, a.Name, a.Attributes)
		}
	}
}

func addExtraToParents(parents []*tfAttr, attribute *tfAttr, component, name string) (bool, error) {
	if attribute.WriteOnly && slices.ContainsFunc(parents, computedOnly) {
		return false, fmt.Errorf("types.%s.extraAttributes.%s: writeOnly cannot be used inside a computed-only object", component, name)
	}
	computed := false
	for _, parent := range parents {
		child := *attribute // each schema path can have different computed flags
		if computedOnly(parent) {
			markComputed(&child)
			computed = true
		}
		parent.Attributes = append(parent.Attributes, &child)
	}
	return computed, nil
}

func attachExtraBehavior(out *tfResource, resource, component, name string, extra overrides.ExtraAttribute) error {
	if extra.Dynamic {
		if component != resource {
			return fmt.Errorf("types.%s.extraAttributes.%s: dynamic applies to an attribute of the resource %s: the state keeps the value of a dynamic attribute only at the top level", component, name, resource)
		}
		if out.CRUD != nil {
			out.CRUD.DynamicExtras = append(out.CRUD.DynamicExtras, dynamicExtra{TFName: name, Model: camelize(name)})
		}
	}
	if extra.InUpdate {
		if component != resource {
			return fmt.Errorf("types.%s.extraAttributes.%s: inUpdate applies to an attribute of the resource %s", component, name, resource)
		}
		if out.Conv == nil || !out.Conv.Replace {
			return fmt.Errorf("types.%s.extraAttributes.%s: inUpdate needs an Update that replaces the resource", component, name)
		}
		out.Conv.UpdateFields = append(out.Conv.UpdateFields, &maskField{TFName: name})
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

func extraParents(out *tfResource, resource, component string) (*tfModel, []*tfAttr, error) {
	model := extraModel(out, component)
	if model == nil {
		return nil, nil, fmt.Errorf("types.%s.extraAttributes: no generated model for this component", component)
	}
	if component == resource {
		return model, nil, nil
	}
	parents := nestedAttrParents(out.Attributes, component)
	if len(parents) == 0 {
		return nil, nil, fmt.Errorf("types.%s.extraAttributes: no generated nested object for this component", component)
	}
	return model, parents, nil
}

func extraAttrNamed(out *tfResource, resource, component string, parents []*tfAttr, name string) bool {
	if component == resource {
		return attrNamed(out.Attributes, name)
	}
	for _, parent := range parents {
		if attrNamed(parent.Attributes, name) {
			return true
		}
	}
	return false
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

func nestedAttrParents(attrs []*tfAttr, component string) []*tfAttr {
	var parents []*tfAttr
	for _, a := range attrs {
		if a.ObjectComponent == component {
			parents = append(parents, a)
		}
		parents = append(parents, nestedAttrParents(a.Attributes, component)...)
	}
	return parents
}

func attrNamed(attrs []*tfAttr, name string) bool {
	return slices.ContainsFunc(attrs, func(a *tfAttr) bool { return a.Name == name })
}

func fieldNamed(model *tfModel, name string) bool {
	return slices.ContainsFunc(model.Fields, func(f tfModelField) bool { return f.TFName == name })
}

// extraAttr returns the schema attribute, the model field, the attr.Type expression, and the typed
// null of an extra attribute.
func extraAttr(name string, extra overrides.ExtraAttribute) (*tfAttr, tfModelField, string, string, error) {
	if extra.Dynamic {
		return &tfAttr{
			Name:        name,
			Kind:        "Dynamic",
			ValueKind:   "Dynamic",
			Required:    extra.Required,
			Optional:    !extra.Required,
			Extra:       true,
			Description: extra.MarkdownDescription,
		}, tfModelField{Name: camelize(name), Type: "types.Dynamic", TFName: name}, "types.DynamicType", "types.DynamicNull()", nil
	}
	if extra.Type == "string" {
		return &tfAttr{
			Name: name, Kind: "String", ValueKind: "String",
			Optional: !extra.Computed, Computed: extra.Computed,
			WriteOnly: extra.WriteOnly, Extra: true, Description: extra.MarkdownDescription,
		}, tfModelField{Name: camelize(name), Type: "types.String", TFName: name}, "types.StringType", "types.StringNull()", nil
	}
	if extra.Type == "set" {
		return &tfAttr{
			Name: name, Kind: "Set", ValueKind: "Set",
			Optional: !extra.Computed, Computed: extra.Computed,
			WriteOnly: extra.WriteOnly, Extra: true, ElementType: "types.StringType", Description: extra.MarkdownDescription,
		}, tfModelField{Name: camelize(name), Type: "types.Set", TFName: name}, "types.SetType{ElemType: types.StringType}", "types.SetNull(types.StringType)", nil
	}
	elem, nullExpr, err := extraElem(extra.ElementType)
	if err != nil {
		return nil, tfModelField{}, "", "", err
	}
	return &tfAttr{
		Name:        name,
		Kind:        "Map",
		ValueKind:   "Map",
		Optional:    !extra.Computed,
		Computed:    extra.Computed,
		WriteOnly:   extra.WriteOnly,
		Extra:       true,
		ElementType: elem,
		Description: extra.MarkdownDescription,
	}, tfModelField{Name: camelize(name), Type: "types.Map", TFName: name}, "types.MapType{ElemType: " + elem + "}", nullExpr, nil
}

func attachConvExtra(conv *convData, model, tfName, goName, attrType, nullExpr string) error {
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
				Expr:   attrType,
			})
		}
		obj.ExtraFlatten = append(obj.ExtraFlatten, extraFlatten{Name: goName, TFName: tfName, Expr: nullExpr})
	}
	if !matched {
		matched = attachEmptyConvExtra(conv, model, tfName, goName, attrType, nullExpr)
	}
	if !matched {
		return fmt.Errorf("no flatten conversion for model %s", model)
	}
	return nil
}

// An API object with no fields uses the empty-object conversion instead of its own flatten
// function. The Terraform model can still have extra attributes that need typed nulls.
func attachEmptyConvExtra(conv *convData, model, tfName, goName, attrType, nullExpr string) bool {
	var matched bool
	for _, obj := range conv.Objects {
		if !obj.Flatten {
			continue
		}
		for _, field := range obj.Fields {
			if field.Conv != convEmpty || field.Object.Model != model {
				continue
			}
			if field.ObjectValue {
				field.Object.AttrTypes = append(field.Object.AttrTypes, convAttrType{TFName: tfName, Expr: attrType})
			}
			field.Object.ExtraFlatten = append(field.Object.ExtraFlatten, extraFlatten{Name: goName, TFName: tfName, Expr: nullExpr})
			matched = true
		}
	}
	return matched
}
