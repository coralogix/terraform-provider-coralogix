package main

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/model"
)

// typesData is the template data of the type mode (D20): the Terraform
// schema, the models, and expand and flatten of component types, with no
// resource. Handwritten code embeds them, for example a new widget in the
// handwritten dashboard resource.
type typesData struct {
	Package       string
	TimeValidator bool   // an attribute uses rfc3339Validator
	Command       string // the tfgen arguments that write the package
	Roots         []*typeRoot
	Models        []*tfModel
	Conv          *convData
	// Enums are the enum names for handwritten code (F54). SDKPkg is the
	// import path of their SDK package.
	Enums  []*typeEnum
	SDKPkg string
	// CustomPkg is the import path of the handwritten custom attributes
	// (custom.go); "" for none.
	CustomPkg string
}

// typeEnum is one enum of --enums: the Terraform name of each API value.
type typeEnum struct {
	Name   string // Go name of the exported names, for example "TextAlignment"
	SDK    string // qualified SDK type, for example "dashboard_service.TextAlignment"
	Schema string // the component schema
	Prefix string // the prefix of the API values
	Items  []enumItem
	// Overridden is true when the overrides give some values other names,
	// or name the value that only means "not set" (D21).
	Overridden bool
}

type enumItem struct {
	TFName string // for example "left"
	Const  string // qualified SDK constant
	Value  string // the API value
	// ReadOnly is true for the zero value when the overrides do not accept
	// it in a configuration: it is read, but not in <E>Names.
	ReadOnly bool
}

// typeRoot is one type that the handwritten code uses. The types inside it
// are generated too, but only for the roots.
type typeRoot struct {
	Name       string // Go name of the exported functions, for example "Panel"
	Obj        *convObject
	Attributes []*tfAttr
}

// typeTagFlag is the flag that selects the SDK package of the types.
const typeTagFlag = "--tag"

// typeInputs are the component schemas of the type mode: the types, and the
// enums whose Terraform names handwritten code needs.
type typeInputs struct {
	roots, enums []string
	// overrides is the path of the overrides file (D21); "" for none.
	overrides string
}

// checkedTypeNames builds the model of each root and each enum, and returns
// them with their SDK names. It fails when the pinned SDK does not have one
// of the names.
func checkedTypeNames(spec string, in typeInputs, tag, sdkModule string) ([]*model.Type, []*model.Type, []sdkRef, error) {
	if tag == "" {
		// The SDK copies a component into each package that uses it, so the
		// component alone does not name the package.
		return nil, nil, nil, fmt.Errorf("%s is required with --types and --enums: the tag of the operations whose SDK package has the types", typeTagFlag)
	}
	data, err := os.ReadFile(spec)
	if err != nil {
		return nil, nil, nil, err
	}
	doc, err := model.Load(data)
	if err != nil {
		return nil, nil, nil, err
	}
	seen := map[string]bool{}
	var types, enums []*model.Type
	for _, list := range []struct {
		names []string
		build func(*v3.Document, string) (*model.Type, error)
		out   *[]*model.Type
	}{{in.roots, model.BuildType, &types}, {in.enums, model.BuildEnum, &enums}} {
		for _, name := range list.names {
			if seen[name] {
				return nil, nil, nil, fmt.Errorf("%s is listed twice", name)
			}
			seen[name] = true
			t, err := list.build(doc, name)
			if err != nil {
				return nil, nil, nil, err
			}
			*list.out = append(*list.out, t)
		}
	}
	refs, err := resolveTypeSDKNames(append(slices.Clone(types), enums...), tag, sdkModule)
	if err != nil {
		return nil, nil, nil, err
	}
	pkgs, err := loadSDK(refs)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := checkSDKNames(refs, pkgs); err != nil {
		return nil, nil, nil, fmt.Errorf("SDK package %s (from %s %q): %w", refs[0].Pkg, typeTagFlag, tag, err)
	}
	return types, enums, refs, nil
}

// resolveTypeSDKNames lists the SDK names of the roots and of the types
// inside them, in the SDK package of the tag.
func resolveTypeSDKNames(roots []*model.Type, tag, module string) ([]sdkRef, error) {
	pkgName := strings.ToLower(strings.ReplaceAll(tag, " ", "_"))
	s := &resolver{pkg: sdkGenRoot(module) + "/" + pkgName, seen: map[string]bool{}, bodies: map[string]string{}}
	s.add(sdkRef{Path: "resource", Kind: kindPackage, Name: pkgName, Rule: ruleTag})
	for _, t := range roots {
		// nested adds the type, its fields, and the types inside it.
		if err := s.nested("types."+t.Schema, t); err != nil {
			return nil, err
		}
	}
	return s.refs, nil
}

// buildTypes maps the roots, the enums, and their SDK names to the template
// data. Each type is generated once, also when several roots use it. ov are
// the overrides; nil for none.
func buildTypes(roots, enums []*model.Type, refs []sdkRef, ov *overrides, pkg, command string) (*typesData, error) {
	if ov == nil {
		// The type mode always applies the spec readOnly (see effective).
		ov = &overrides{}
	}
	out := &typesData{Package: pkg, Command: command, CustomPkg: ov.CustomPackage}
	if err := ov.check(append(slices.Clone(roots), enums...)); err != nil {
		return nil, err
	}
	ix, err := indexRefs(refs)
	if err != nil {
		return nil, err
	}
	out.SDKPkg = ix.pkg.Pkg
	names := map[string][]string{}
	for _, t := range withNamedEnums(roots, enums, ov) {
		e, err := enumNames(t, ix, ov)
		if err != nil {
			return nil, err
		}
		out.Enums = append(out.Enums, e)
		for _, it := range e.Items {
			if !it.ReadOnly {
				names[t.Schema] = append(names[t.Schema], it.TFName)
			}
		}
	}
	tb := &tfBuilder{seen: map[string]bool{}, ov: ov, enumNames: names}
	for _, t := range roots {
		attrs, err := tb.objectAttributes(attrPath{"root", tfName(t.Schema)}, t)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", t.Schema, err)
		}
		out.Roots = append(out.Roots, &typeRoot{Name: camelize(t.Schema), Attributes: attrs})
	}
	if len(tb.validators) != 0 {
		return nil, errors.New("internal error: a resource validator in the type mode")
	}
	out.Models = tb.models
	for _, r := range out.Roots {
		out.TimeValidator = out.TimeValidator || usesValidator(r.Attributes, timeValidator)
	}
	if out.Conv, err = typesConv(roots, ix, ov, out.Roots); err != nil {
		return nil, err
	}
	return out, nil
}

// withNamedEnums returns the enums of --enums, then the enums that the
// overrides give Terraform names, in name order. The generated code of an
// enum with names uses its name maps, so enums.go must have them.
func withNamedEnums(roots, enums []*model.Type, ov *overrides) []*model.Type {
	if ov == nil {
		return enums
	}
	out := slices.Clone(enums)
	listed := map[string]bool{}
	for _, t := range enums {
		listed[t.Schema] = true
	}
	objects, found := map[string]*model.Type{}, map[string]*model.Type{}
	for _, t := range roots {
		collectTypes(t, objects, found)
	}
	for _, schema := range sortedKeys(ov.Enums) {
		if t, ok := found[schema]; ok && ov.named(schema) && !listed[schema] {
			out = append(out, t)
		}
	}
	return out
}

// typesConv returns the expand and flatten data of the roots, and sets the
// convObject of each typeRoot.
func typesConv(roots []*model.Type, ix *refIndex, ov *overrides, out []*typeRoot) (*convData, error) {
	cb := &convBuilder{ix: ix, bySchema: map[string]*convObject{}, ov: ov}
	for i, t := range roots {
		obj, err := cb.nested(t)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", t.Schema, err)
		}
		// Handwritten code sends and reads a type, so it needs both.
		for _, expand := range []bool{true, false} {
			if err := cb.mark(obj, expand); err != nil {
				return nil, fmt.Errorf("%s: %w", t.Schema, err)
			}
		}
		out[i].Obj = obj
	}
	// Every type exports its attribute types: handwritten code needs them to
	// build a types.Object, or a list or map of them, of any generated model.
	// An unwrapped object has no model, so it has no attribute types, and an
	// inlined object has the model of its parent.
	for _, obj := range cb.objects {
		obj.AttrTypesFunc = obj.Func + "AttrTypes"
	}
	for _, obj := range cb.objects {
		if obj.Unwrap || obj.Inline {
			continue
		}
		if err := cb.attrTypes(obj); err != nil {
			return nil, err
		}
	}
	return &convData{SDKPkg: ix.pkg.Pkg, SDKName: ix.pkg.Name, Objects: cb.objects, Exported: true, CustomPkg: ov.CustomPackage}, nil
}

// enumNames returns the Terraform name of each value of the enum t. The name
// is the API value without the enum prefix and without _OR_UNSPECIFIED or
// _UNSPECIFIED, in lower case: TEXT_ALIGNMENT_LEFT → "left",
// ANNOTATION_ORIENTATION_VERTICAL_UNSPECIFIED → "vertical". Handwritten
// resources use these names (F54). The overrides can give a value another
// name, and name the value that only means "not set" (D21). Two values with
// one name are an error.
func enumNames(t *model.Type, ix *refIndex, ov *overrides) (*typeEnum, error) {
	ref, err := ix.schemaRef(t.Schema)
	if err != nil {
		return nil, err
	}
	e := &typeEnum{Name: camelize(t.Schema), SDK: ix.pkg.Name + "." + ref.Name, Schema: t.Schema, Prefix: t.EnumPrefix}
	var eo enumOverride
	if ov != nil {
		eo = ov.Enums[t.Schema]
	}
	values := t.Values
	if eo.Zero != "" {
		values = append(slices.Clone(values), t.Zero)
	}
	e.Overridden = len(eo.Values) != 0 || eo.Zero != ""
	byName := map[string]string{}
	for _, v := range values {
		name := enumRuleName(t, v)
		switch {
		case v == t.Zero:
			name = eo.Zero
		case eo.Values[v] != "":
			name = eo.Values[v]
		}
		if prev, ok := byName[name]; ok {
			return nil, fmt.Errorf("%s: %s and %s both have the Terraform name %q", t.Schema, prev, v, name)
		}
		byName[name] = v
		item := enumItem{TFName: name, Const: ix.pkg.Name + "." + enumConstName(ref.Name, v), Value: v}
		item.ReadOnly = v == t.Zero && eo.AcceptZero != nil && !*eo.AcceptZero
		e.Items = append(e.Items, item)
	}
	return e, nil
}

// enumRuleName is the Terraform name of the enum value v by the rule of
// enumNames.
func enumRuleName(t *model.Type, v string) string {
	name := strings.TrimPrefix(v, t.EnumPrefix)
	for _, suffix := range []string{"_OR_UNSPECIFIED", "_UNSPECIFIED"} {
		name = strings.TrimSuffix(name, suffix)
	}
	return strings.ToLower(name)
}

// typeFiles maps each output file of the type mode to its template. They are
// written with --types; enums.go is written with --enums.
var typeFiles = map[string]string{
	"schema.go":  "types_schema.go.tmpl",
	"model.go":   "types_model.go.tmpl",
	"convert.go": "types_convert.go.tmpl",
}

// generateTypes returns the generated files of the type mode, by file name.
func generateTypes(roots, enums []*model.Type, refs []sdkRef, ov *overrides, pkg, command string) (map[string][]byte, error) {
	data, err := buildTypes(roots, enums, refs, ov, pkg, command)
	if err != nil {
		return nil, err
	}
	files := map[string]string{}
	if len(roots) != 0 {
		files = maps.Clone(typeFiles)
	}
	if len(data.Enums) != 0 {
		files["enums.go"] = "types_enums.go.tmpl"
	}
	return render(files, data)
}
