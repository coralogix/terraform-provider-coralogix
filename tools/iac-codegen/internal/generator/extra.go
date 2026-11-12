package generator

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

type extraFlatten struct {
	Name   string // Go field
	TFName string // Terraform attribute name
	Expr   string // typed null, for example types.MapNull(types.StringType)
	// Keep: flatten takes the value of the prior object, because the API does not hold the
	// attribute (a string extra attribute with a default, inside an object).
	Keep bool
	// Default is the Go expression of the static default of the attribute. A read without a prior
	// object, such as an import, starts with it.
	Default string
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
			computed, err := attachOneExtra(out, file, component, model, parents, name, extras[name])
			if err != nil {
				return err
			}
			computedExtra = computedExtra || computed
		}
	}
	if computedExtra {
		updateExtraPaths(out, extraHasObject(file))
	}
	// A default that holds the types of the attributes below it is built now: the extra attributes
	// are part of those types.
	if err := resolveStructDefaults(out.Attributes, file); err != nil {
		return err
	}
	// An Optional and Computed attribute or a default can need a package that no other attribute
	// of the schema uses.
	out.PlanModifierPkgs = planModifierPackages(out.Attributes)
	out.DefaultPkgs = defaultPackages(out.Attributes)
	return nil
}

func attachOneExtra(out *tfResource, file *overrides.File, component string, model *tfModel, parents []*tfAttr, name string, extra overrides.ExtraAttribute) (bool, error) {
	location := fmt.Sprintf("types.%s.extraAttributes.%s", component, name)
	if fieldNamed(model, name) || extraAttrNamed(out, file.Resource, component, parents, name) {
		return false, fmt.Errorf("%s: the generated object already has this attribute", location)
	}
	attribute, field, attrType, nullExpr, err := newExtra(name, extra, file)
	if err != nil {
		return false, fmt.Errorf("%s: %w", location, err)
	}
	computed := extra.Computed || extra.Type == "object"
	if component == file.Resource {
		out.Attributes = append(out.Attributes, attribute)
	} else {
		inherited, err := addExtraToParents(parents, attribute, component, name)
		if err != nil {
			return false, err
		}
		computed = computed || inherited
	}
	model.Fields = append(model.Fields, field)
	if err := attachConvExtra(out.Conv, model.Name, name, camelize(name), attrType, nullExpr, extra.Type == "string" && component != file.Resource && extraDefaultExpr(extra) != "", extraDefaultExpr(extra)); err != nil {
		return false, fmt.Errorf("%s: %w", location, err)
	}
	if err := attachExtraBehavior(out, file.Resource, component, name, extra, attrType); err != nil {
		return false, err
	}
	return computed, nil
}

// newExtra returns the schema attribute, the model field, the attr.Type expression, and the typed
// null of an extra attribute, with its validators, default, plan modifiers, and the attributes of
// an object.
func newExtra(name string, extra overrides.ExtraAttribute, file *overrides.File) (*tfAttr, tfModelField, string, string, error) {
	attribute, field, attrType, nullExpr, err := extraAttr(name, extra)
	if err != nil {
		return nil, field, "", "", err
	}
	if err := configureExtraAttribute(attribute, extra, file); err != nil {
		return nil, field, "", "", err
	}
	if extra.Type == "object" {
		if attrType, nullExpr, err = attachObjectChildren(attribute, extra, file); err != nil {
			return nil, field, "", "", err
		}
	}
	return attribute, field, attrType, nullExpr, nil
}

// attachObjectChildren adds the attributes of an object extra attribute, in name order, and returns
// the attr.Type expression and the typed null of the object.
func attachObjectChildren(object *tfAttr, extra overrides.ExtraAttribute, file *overrides.File) (string, string, error) {
	typeExprs := make([]string, 0, len(extra.Attributes))
	for _, name := range extraNames(extra.Attributes) {
		child, _, childType, _, err := newExtra(name, extra.Attributes[name], file)
		if err != nil {
			return "", "", fmt.Errorf("attributes.%s: %w", name, err)
		}
		object.Attributes = append(object.Attributes, child)
		typeExprs = append(typeExprs, fmt.Sprintf("%q: %s", name, childType))
	}
	attrTypes := "map[string]attr.Type{" + strings.Join(typeExprs, ", ") + "}"
	return "types.ObjectType{AttrTypes: " + attrTypes + "}", "types.ObjectNull(" + attrTypes + ")", nil
}

func configureExtraAttribute(attribute *tfAttr, extra overrides.ExtraAttribute, file *overrides.File) error {
	for _, validator := range extra.Validators {
		expr, err := validatorExpr(attribute, validator, file)
		if err != nil {
			return err
		}
		attribute.Validators = append(attribute.Validators, expr)
	}
	if len(extra.ElementValues) != 0 {
		if attribute.Kind != "Set" {
			return fmt.Errorf("elementValues needs a set")
		}
		quoted := make([]string, len(extra.ElementValues))
		for i, value := range extra.ElementValues {
			quoted[i] = fmt.Sprintf("%q", value)
		}
		attribute.Validators = append(attribute.Validators, "setvalidator.ValueStringsAre(stringvalidator.OneOf("+strings.Join(quoted, ", ")+"))")
	}
	if extra.Default != nil {
		expr, err := defaultExpr(attribute, extra.Default)
		if err != nil {
			return fmt.Errorf("default: %w", err)
		}
		attribute.Default, attribute.DefaultRaw = expr, extra.Default
	}
	if extra.Equality != "" {
		// First, as for the equality line of a field.
		attribute.Modifiers = append([]string{"equivalentDocument{equal: " + extra.Equality + "Equal}"}, attribute.Modifiers...)
	}
	if extra.UseStateForUnknown {
		if !extra.Computed || attribute.Kind == "Dynamic" {
			return fmt.Errorf("useStateForUnknown needs a computed attribute")
		}
		attribute.Modifiers = append(attribute.Modifiers, strings.ToLower(attribute.ValueKind)+"planmodifier.UseStateForUnknown()")
	}
	return nil
}

// buildTFResource computed these paths before extra attributes were attached.
// An object extra attribute has required attributes that keepAPIValue must know after an import;
// the other extra attributes never were in RequiredPaths, and the released output keeps that.
func updateExtraPaths(out *tfResource, withRequired bool) {
	out.ComputedPaths = computedPaths(nil, "", out.Attributes)
	if withRequired {
		out.RequiredPaths = requiredPaths(nil, "", out.Attributes)
	}
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
	if attribute.Kind == "SingleNested" && slices.ContainsFunc(parents, computedOnly) {
		return false, fmt.Errorf("types.%s.extraAttributes.%s: an object cannot be inside a computed-only object", component, name)
	}
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

func attachExtraBehavior(out *tfResource, resource, component, name string, extra overrides.ExtraAttribute, attrType string) error {
	location := fmt.Sprintf("types.%s.extraAttributes.%s", component, name)
	if extra.Dynamic && component != resource {
		return fmt.Errorf("%s: dynamic applies to an attribute of the resource %s: the state keeps the value of a dynamic attribute only at the top level", location, resource)
	}
	if extra.Equality != "" && component != resource {
		return fmt.Errorf("%s: equality applies to an attribute of the resource %s: inside the items of a list or set, a plan modifier reads the state and the configuration of another item", location, resource)
	}
	if component == resource {
		keepExtra(out, name, extra, attrType)
	}
	if extra.Equality != "" && out.Conv != nil && !slices.Contains(out.Conv.ExtraEquality, extra.Equality) {
		// The package needs the comparison of that kind, as for an equality line of a field.
		out.Conv.ExtraEquality = append(out.Conv.ExtraEquality, extra.Equality)
	}
	if extra.InUpdate {
		if component != resource {
			return fmt.Errorf("%s: inUpdate applies to an attribute of the resource %s", location, resource)
		}
		if out.Conv == nil || !out.Conv.Replace {
			return fmt.Errorf("%s: inUpdate needs an Update that replaces the resource", location)
		}
		out.Conv.UpdateFields = append(out.Conv.UpdateFields, &maskField{TFName: name})
	}
	return nil
}

// keepExtra makes setState start an extra attribute of the resource with the value of the plan or
// the prior state, which the API does not hold. A dynamic attribute, a configurable string, and an
// Optional and Computed one are kept; a read-only string gets its value from an AfterFlatten hook. An
// object is kept when the prior value has the same type and no unknown value inside.
func keepExtra(out *tfResource, name string, extra overrides.ExtraAttribute, attrType string) {
	if out.CRUD == nil {
		return
	}
	kept := keptExtra{TFName: name, Model: camelize(name)}
	switch {
	case extra.Dynamic:
		kept.Type = "types.Dynamic"
	case extra.Type == "string" && (!extra.Computed || extra.Optional) && !extra.WriteOnly:
		kept.Type = "types.String"
	case extra.Type == "object":
		kept.Type, kept.AttrType = "types.Object", attrType
		out.CRUD.KeptObjects = true
	default:
		return
	}
	out.CRUD.KeptExtras = append(out.CRUD.KeptExtras, kept)
}

// extraHasObject reports whether any extra attribute is an object.
func extraHasObject(file *overrides.File) bool {
	for _, t := range file.Types {
		for _, extra := range t.ExtraAttributes {
			if extra.Type == "object" {
				return true
			}
		}
	}
	return false
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
// null of an extra attribute. The attributes of an object are added by attachObjectChildren.
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
	attribute := &tfAttr{
		Name: name, Required: extra.Required,
		// A computed attribute is read-only, unless optional: true makes it Optional and Computed.
		Optional:  !extra.Required && (!extra.Computed || extra.Optional),
		Computed:  extra.Computed,
		WriteOnly: extra.WriteOnly, Extra: true, Description: extra.MarkdownDescription,
	}
	field := tfModelField{Name: camelize(name), TFName: name}
	var attrType, nullExpr string
	switch extra.Type {
	case "string":
		attribute.Kind, field.Type, attrType, nullExpr = "String", "types.String", "types.StringType", "types.StringNull()"
	case "int64":
		attribute.Kind, field.Type, attrType, nullExpr = "Int64", "types.Int64", "types.Int64Type", "types.Int64Null()"
	case "bool":
		attribute.Kind, field.Type, attrType, nullExpr = "Bool", "types.Bool", "types.BoolType", "types.BoolNull()"
	case "set":
		attribute.Kind, attribute.ElementType, field.Type = "Set", "types.StringType", "types.Set"
		attrType, nullExpr = "types.SetType{ElemType: types.StringType}", "types.SetNull(types.StringType)"
	case "list":
		attribute.Kind, attribute.ElementType, field.Type = "List", "types.StringType", "types.List"
		attrType, nullExpr = "types.ListType{ElemType: types.StringType}", "types.ListNull(types.StringType)"
	case "object":
		// The model holds types.Object, which can be unknown, as an Optional and Computed object is
		// in a plan. attachObjectChildren sets the type and the null of the object.
		attribute.Kind, field.Type = "SingleNested", "types.Object"
	default:
		elem, null, err := extraElem(extra.ElementType)
		if err != nil {
			return nil, tfModelField{}, "", "", err
		}
		attribute.Kind, attribute.ElementType, field.Type = "Map", elem, "types.Map"
		attrType, nullExpr = "types.MapType{ElemType: "+elem+"}", null
	}
	attribute.ValueKind = attribute.Kind
	if attribute.Kind == "SingleNested" {
		attribute.ValueKind = "Object"
	}
	return attribute, field, attrType, nullExpr, nil
}

// extraDefaultExpr is the Go expression of the static string default of an extra attribute, or "".
func extraDefaultExpr(extra overrides.ExtraAttribute) string {
	if text, ok := extra.Default.(string); ok && extra.Type == "string" {
		return "types.StringValue(" + strconv.Quote(text) + ")"
	}
	return ""
}

func attachConvExtra(conv *convData, model, tfName, goName, attrType, nullExpr string, keep bool, defaultExpr string) error {
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
		obj.ExtraFlatten = append(obj.ExtraFlatten, extraFlatten{Name: goName, TFName: tfName, Expr: nullExpr, Keep: keep, Default: defaultExpr})
		if keep {
			// The flatten function reads the attribute from the prior object, and so do the
			// functions of the objects above it.
			obj.NeedsPrior = true
		}
	}
	if matched && keep {
		propagatePrior(conv)
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
