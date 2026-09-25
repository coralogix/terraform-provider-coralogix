package main

import (
	"errors"
	"fmt"
	"go/types"

	"golang.org/x/tools/go/packages"

	// The generator reads these packages with go/packages. The imports keep
	// the pinned SDK version in go.mod.
	_ "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
)

// loadSDK loads the packages that the refs use, in the version that go.mod pins.
func loadSDK(refs []sdkRef) (map[string]*packages.Package, error) {
	var paths []string
	seen := map[string]bool{}
	for _, ref := range refs {
		if !seen[ref.Pkg] {
			seen[ref.Pkg] = true
			paths = append(paths, ref.Pkg)
		}
	}
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedTypes | packages.NeedModule}
	pkgs, err := packages.Load(cfg, paths...)
	if err != nil {
		return nil, fmt.Errorf("load SDK: %w", err)
	}
	byPath := map[string]*packages.Package{}
	for _, p := range pkgs {
		if len(p.Errors) > 0 {
			return nil, fmt.Errorf("load SDK package %s: %v", p.PkgPath, p.Errors[0])
		}
		byPath[p.PkgPath] = p
	}
	return byPath, nil
}

// checkSDKNames returns one error for each ref that the SDK does not have.
// Each error names the model path and the expected SDK name.
func checkSDKNames(refs []sdkRef, pkgs map[string]*packages.Package) error {
	var errs []error
	for _, ref := range refs {
		if err := checkRef(ref, pkgs[ref.Pkg]); err != nil {
			errs = append(errs, fmt.Errorf("%s: SDK %s %s: %w", ref.Path, ref.Kind, ref.sdkName(), err))
		}
	}
	return errors.Join(errs...)
}

func checkRef(ref sdkRef, pkg *packages.Package) error {
	if pkg == nil {
		return fmt.Errorf("package %s not loaded", ref.Pkg)
	}
	qualifier := func(p *types.Package) string {
		if p == pkg.Types {
			return ""
		}
		return p.Name()
	}
	if ref.Kind == kindField || ref.Kind == kindMethod {
		return checkMember(ref, pkg, qualifier)
	}
	return checkTopLevel(ref, pkg, qualifier)
}

// checkTopLevel checks a package, or a type, const, or func of the package.
func checkTopLevel(ref sdkRef, pkg *packages.Package, qualifier types.Qualifier) error {
	scope := pkg.Types.Scope()
	switch ref.Kind {
	case kindPackage:
		if pkg.Name != ref.Name {
			return fmt.Errorf("package name is %s", pkg.Name)
		}
		return nil
	case kindType:
		if _, ok := scope.Lookup(ref.Name).(*types.TypeName); !ok {
			return errors.New("not found")
		}
		return nil
	case kindConst:
		if _, ok := scope.Lookup(ref.Name).(*types.Const); !ok {
			return errors.New("not found")
		}
		return nil
	case kindFunc:
		fn, ok := scope.Lookup(ref.Name).(*types.Func)
		if !ok {
			return errors.New("not found")
		}
		if s := types.TypeString(fn.Type(), qualifier); s != ref.Want {
			return fmt.Errorf("type is %s, want %s", s, ref.Want)
		}
		return nil
	}
	return fmt.Errorf("unknown kind %q", ref.Kind)
}

// checkMember checks a field or a method of the type ref.Owner.
func checkMember(ref sdkRef, pkg *packages.Package, qualifier types.Qualifier) error {
	owner, ok := pkg.Types.Scope().Lookup(ref.Owner).(*types.TypeName)
	if !ok {
		return fmt.Errorf("type %s not found", ref.Owner)
	}
	var got types.Type
	if ref.Kind == kindField {
		st, ok := owner.Type().Underlying().(*types.Struct)
		if !ok {
			return fmt.Errorf("%s is not a struct", ref.Owner)
		}
		for f := range st.Fields() {
			if f.Name() == ref.Name {
				got = f.Type()
			}
		}
	} else if sel := types.NewMethodSet(types.NewPointer(owner.Type())).Lookup(pkg.Types, ref.Name); sel != nil {
		got = sel.Type()
	}
	if got == nil {
		return errors.New("not found")
	}
	if s := types.TypeString(got, qualifier); s != ref.Want {
		return fmt.Errorf("type is %s, want %s", s, ref.Want)
	}
	return nil
}
