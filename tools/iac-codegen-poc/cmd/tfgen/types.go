package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

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

// checkedTypeNames builds the model of each root and returns it with its SDK
// names. It fails when the pinned SDK does not have one of the names.
func checkedTypeNames(spec string, roots []string, tag, sdkModule string) ([]*model.Type, []sdkRef, error) {
	if tag == "" {
		// The SDK copies a component into each package that uses it, so the
		// component alone does not name the package.
		return nil, nil, fmt.Errorf("%s is required with --types: the tag of the operations whose SDK package has the types", typeTagFlag)
	}
	data, err := os.ReadFile(spec)
	if err != nil {
		return nil, nil, err
	}
	doc, err := model.Load(data)
	if err != nil {
		return nil, nil, err
	}
	var types []*model.Type
	seen := map[string]bool{}
	for _, name := range roots {
		if seen[name] {
			return nil, nil, fmt.Errorf("--types: %s is listed twice", name)
		}
		seen[name] = true
		t, err := model.BuildType(doc, name)
		if err != nil {
			return nil, nil, err
		}
		types = append(types, t)
	}
	refs, err := resolveTypeSDKNames(types, tag, sdkModule)
	if err != nil {
		return nil, nil, err
	}
	pkgs, err := loadSDK(refs)
	if err != nil {
		return nil, nil, err
	}
	if err := checkSDKNames(refs, pkgs); err != nil {
		return nil, nil, fmt.Errorf("SDK package %s (from %s %q): %w", refs[0].Pkg, typeTagFlag, tag, err)
	}
	return types, refs, nil
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

// buildTypes maps the roots and their SDK names to the template data. Each
// type is generated once, also when several roots use it.
func buildTypes(roots []*model.Type, refs []sdkRef, pkg, command string) (*typesData, error) {
	out := &typesData{Package: pkg, Command: command}
	tb := &tfBuilder{seen: map[string]bool{}}
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

	ix, err := indexRefs(refs)
	if err != nil {
		return nil, err
	}
	cb := &convBuilder{ix: ix, bySchema: map[string]*convObject{}}
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
		out.Roots[i].Obj = obj
	}
	// Every type exports its attribute types: handwritten code needs them to
	// build a types.Object, or a list or map of them, of any generated model.
	for _, obj := range cb.objects {
		obj.AttrTypesFunc = obj.Func + "AttrTypes"
	}
	for _, obj := range cb.objects {
		if err := cb.attrTypes(obj); err != nil {
			return nil, err
		}
	}
	out.Conv = &convData{SDKPkg: ix.pkg.Pkg, SDKName: ix.pkg.Name, Objects: cb.objects, Exported: true}
	return out, nil
}

// typeFiles maps each output file of the type mode to its template.
var typeFiles = map[string]string{
	"schema.go":  "types_schema.go.tmpl",
	"model.go":   "types_model.go.tmpl",
	"convert.go": "types_convert.go.tmpl",
}

// generateTypes returns the generated files of the type mode, by file name.
func generateTypes(roots []*model.Type, refs []sdkRef, pkg, command string) (map[string][]byte, error) {
	data, err := buildTypes(roots, refs, pkg, command)
	if err != nil {
		return nil, err
	}
	return render(typeFiles, data)
}
