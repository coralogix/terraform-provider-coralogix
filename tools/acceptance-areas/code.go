package main

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// goFile holds what the checks need to know about one Go source file.
type goFile struct {
	path    string
	dir     string
	pkg     string
	tests   []string        // top-level Test, Example, and Fuzz functions
	decls   []string        // package-level names, without methods
	uses    map[string]bool // names used here but not declared in this file
	imports []string        // import paths inside this module
}

// gitFiles lists the files of the work tree: tracked files plus new files that git does
// not ignore.
func gitFiles() ([]string, error) {
	out, err := exec.Command("git", "ls-files", "--cached", "--others", "--exclude-standard").Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	var files []string
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		if _, err := os.Stat(sc.Text()); err == nil {
			files = append(files, sc.Text())
		}
	}
	return files, sc.Err()
}

func modulePath() (string, error) {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(line, "module "); ok {
			return strings.TrimSpace(rest), nil
		}
	}
	return "", fmt.Errorf("go.mod has no module line")
}

func scanGo(files []string, module string) ([]*goFile, error) {
	var out []*goFile
	fset := token.NewFileSet()
	for _, p := range files {
		if !strings.HasSuffix(p, ".go") || !strings.HasPrefix(p, "internal/") {
			continue
		}
		// Mode 0 keeps object resolution on. f.Unresolved needs it.
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			return nil, err
		}
		gf := &goFile{path: p, dir: path.Dir(p), pkg: f.Name.Name, uses: map[string]bool{}}
		for _, imp := range f.Imports {
			ip, _ := strconv.Unquote(imp.Path.Value)
			if rest, ok := strings.CutPrefix(ip, module+"/"); ok {
				gf.imports = append(gf.imports, rest)
			}
		}
		collectDecls(gf, f)
		// Unresolved names are the ones that the parser did not find in this file: names from
		// other files of the package, builtins, and package names.
		for _, id := range f.Unresolved {
			gf.uses[id.Name] = true
		}
		// Keys of composite literals are not resolved, and may name a constant of another file.
		ast.Inspect(f, func(n ast.Node) bool {
			if kv, ok := n.(*ast.KeyValueExpr); ok {
				if id, ok := kv.Key.(*ast.Ident); ok {
					gf.uses[id.Name] = true
				}
			}
			return true
		})
		out = append(out, gf)
	}
	return out, nil
}

func collectDecls(gf *goFile, f *ast.File) {
	isTestFile := strings.HasSuffix(gf.path, "_test.go")
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Recv != nil {
				continue
			}
			gf.decls = append(gf.decls, d.Name.Name)
			if isTestFile && isTestName(d.Name.Name) {
				gf.tests = append(gf.tests, d.Name.Name)
			}
		case *ast.GenDecl:
			for _, s := range d.Specs {
				switch s := s.(type) {
				case *ast.TypeSpec:
					gf.decls = append(gf.decls, s.Name.Name)
				case *ast.ValueSpec:
					for _, n := range s.Names {
						gf.decls = append(gf.decls, n.Name)
					}
				}
			}
		}
	}
}

// isTestName follows the rule of `go test`: the prefix is followed by the end of the name
// or by a character that is not a lower-case letter.
func isTestName(name string) bool {
	for _, p := range []string{"Test", "Example", "Fuzz"} {
		rest, ok := strings.CutPrefix(name, p)
		if !ok || name == "TestMain" {
			continue
		}
		if rest == "" {
			return true
		}
		r, _ := utf8.DecodeRuneInString(rest)
		return !unicode.IsLower(r)
	}
	return false
}

// refEdge says that a file uses code from another file.
type refEdge struct {
	from, to string
	via      string // the name or the import path
}

// refEdges finds the uses of declarations of other files in the same package, and the
// imports of other packages of this module.
func refEdges(files []*goFile) []refEdge {
	type pkgKey struct{ dir, pkg string }
	declared := map[pkgKey]map[string]string{} // package -> name -> declaring file
	byDir := map[string][]*goFile{}
	for _, f := range files {
		k := pkgKey{f.dir, f.pkg}
		if declared[k] == nil {
			declared[k] = map[string]string{}
		}
		for _, d := range f.decls {
			if d == "_" || d == "init" {
				continue
			}
			if _, dup := declared[k][d]; !dup {
				declared[k][d] = f.path
			}
		}
		if !strings.HasSuffix(f.path, "_test.go") {
			byDir[f.dir] = append(byDir[f.dir], f)
		}
	}
	var edges []refEdge
	for _, f := range files {
		names := make([]string, 0, len(f.uses))
		for n := range f.uses {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			if to, ok := declared[pkgKey{f.dir, f.pkg}][n]; ok && to != f.path {
				edges = append(edges, refEdge{from: f.path, to: to, via: n})
			}
		}
		for _, imp := range f.imports {
			for _, target := range byDir[imp] {
				edges = append(edges, refEdge{from: f.path, to: target.path, via: imp})
			}
		}
	}
	return edges
}
