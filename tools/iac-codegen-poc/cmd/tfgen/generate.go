package main

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"maps"
	"strconv"
	"text/template"

	"golang.org/x/tools/go/ast/astutil"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/model"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

var templates = template.Must(template.ParseFS(templateFS, "templates/*.tmpl"))

// generatedFiles maps each output file to its template.
var generatedFiles = map[string]string{
	"schema.go":   "schema.go.tmpl",
	"model.go":    "model.go.tmpl",
	"convert.go":  "convert.go.tmpl",
	"mask.go":     "mask.go.tmpl",
	"resource.go": "resource.go.tmpl",
}

// generate returns the generated files of the resource, by file name. refs
// are the checked SDK names of the resource. With acc values, it also writes
// the acceptance test.
func generate(r *model.Resource, refs []sdkRef, pkg string, acc *accValues) (map[string][]byte, error) {
	data, err := buildTFResource(r, pkg)
	if err != nil {
		return nil, err
	}
	if data.Conv, err = buildConv(r, refs); err != nil {
		return nil, err
	}
	if data.CRUD, err = buildCRUD(r, refs); err != nil {
		return nil, err
	}
	files := maps.Clone(generatedFiles)
	if acc != nil {
		if data.Acc, err = buildAcc(r, acc); err != nil {
			return nil, fmt.Errorf("acceptance test values: %w", err)
		}
		files["acc_test.go"] = "acc_test.go.tmpl"
	}
	out := map[string][]byte{}
	for file, tmpl := range files {
		var buf bytes.Buffer
		if err := templates.ExecuteTemplate(&buf, tmpl, data); err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		src, err := formatSource(file, buf.Bytes())
		if err != nil {
			return nil, err
		}
		out[file] = src
	}
	return out, nil
}

// formatSource removes the unused imports and formats the file. A template
// lists every import that the file can use.
func formatSource(file string, src []byte) ([]byte, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse generated %s: %w\n%s", file, err, src)
	}
	var unused []string
	for _, imp := range f.Imports {
		p, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil, err
		}
		if !astutil.UsesImport(f, p) {
			unused = append(unused, p)
		}
	}
	for _, p := range unused {
		astutil.DeleteImport(fset, f, p)
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		return nil, fmt.Errorf("format generated %s: %w", file, err)
	}
	return buf.Bytes(), nil
}
