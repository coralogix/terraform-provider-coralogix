package generator

import (
	"errors"
	"fmt"
	"go/types"
	"os"
	"strings"

	"golang.org/x/tools/go/packages"
)

// loadSDK loads the packages that the refs use from the selected module graph.
func loadSDK(refs []sdkRef, dir string) (map[string]*packages.Package, error) {
	var paths []string
	seen := map[string]bool{}
	for _, ref := range refs {
		if !seen[ref.Pkg] {
			seen[ref.Pkg] = true
			paths = append(paths, ref.Pkg)
		}
	}
	cfg := &packages.Config{
		Dir:  dir,
		Mode: packages.NeedName | packages.NeedTypes | packages.NeedModule,
		Env:  offlineGoEnv(),
	}
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

func offlineGoEnv() []string {
	env := make([]string, 0, len(os.Environ())+2)
	for _, item := range os.Environ() {
		if strings.HasPrefix(item, "GOPROXY=") || strings.HasPrefix(item, "GOSUMDB=") {
			continue
		}
		env = append(env, item)
	}
	return append(env, "GOPROXY=off", "GOSUMDB=off")
}

func checkRef(ref *sdkRef, pkg *packages.Package) error {
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
func checkTopLevel(ref *sdkRef, pkg *packages.Package, qualifier types.Qualifier) error {
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
func checkMember(ref *sdkRef, pkg *packages.Package, qualifier types.Qualifier) error {
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
	if got == nil && ref.ByType {
		return findMethodByType(ref, owner, qualifier)
	}
	if got == nil {
		return errors.New("not found")
	}
	switch s := types.TypeString(got, qualifier); {
	case s == ref.Want:
	case ref.WantValue != "" && s == ref.WantValue:
		ref.Want = s
	default:
		return fmt.Errorf("type is %s, want %s", s, ref.Want)
	}
	return nil
}

// findMethodByType sets ref.Name to the one exported method of owner whose
// type is ref.Want.
func findMethodByType(ref *sdkRef, owner *types.TypeName, qualifier types.Qualifier) error {
	var found []string
	for sel := range types.NewMethodSet(types.NewPointer(owner.Type())).Methods() {
		if sel.Obj().Exported() && types.TypeString(sel.Type(), qualifier) == ref.Want {
			found = append(found, sel.Obj().Name())
		}
	}
	if len(found) != 1 {
		return fmt.Errorf("not found, and %d methods of %s have type %s: %v", len(found), ref.Owner, ref.Want, found)
	}
	ref.Name = found[0]
	return nil
}
