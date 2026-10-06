package generator

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

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/acceptance"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
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

var formatGenerated = formatSource

// render returns the generated files of the resource, by file name.
// renderWith is render for a resource with a behavior-overrides file (nil for a new resource).
func renderWith(r *model.Resource, refs []sdkRef, pkg string, file *overrides.File) (map[string][]byte, error) {
	return renderAll(r, refs, pkg, file, nil, nil, "")
}

// renderAll is renderWith that also writes the acceptance test, when acc is not nil, and the upgrade
// attributes file of its upgrade test. prior is that file of the last run, or nil. providerModule
// is the module path of the provider; the test imports its provider package.
func renderAll(r *model.Resource, refs []sdkRef, pkg string, file *overrides.File, acc *acceptance.File, prior *acceptance.UpgradeAttributes, providerModule string) (map[string][]byte, error) {
	data, err := buildTFResourceWith(r, pkg, file)
	if err != nil {
		return nil, err
	}
	if data.Conv, err = buildConvWith(r, refs, file); err != nil {
		return nil, err
	}
	if data.CRUD, err = buildCRUDWith(r, refs, file); err != nil {
		return nil, err
	}
	files := maps.Clone(generatedFiles)
	if data.Conv.UsesEquality() {
		if data.HasServerDefaults {
			// ModifyPlan keeps the state in place of every unknown that the configuration does
			// not set. A server default plans unknown on purpose, and would be lost.
			return nil, fmt.Errorf("equality and a server default in one resource are not supported; in existing mode, a default line in %s replaces the server default", overrides.FileName)
		}
		files["equality.go"] = "equality.go.tmpl"
	}
	if data.Conv.Replace {
		// A full replace has no update mask (E11).
		delete(files, "mask.go")
		files["replace.go"] = "replace.go.tmpl"
	}
	out := map[string][]byte{}
	order := []string{"schema.go", "model.go", "convert.go", "mask.go", "resource.go", "replace.go"}
	for _, file := range order {
		tmpl, ok := files[file]
		if !ok {
			continue
		}
		var buf bytes.Buffer
		if err := templates.ExecuteTemplate(&buf, tmpl, data); err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		src, err := formatGenerated(file, buf.Bytes())
		if err != nil {
			return nil, err
		}
		out[file] = src
	}
	if acc != nil {
		accData, err := buildAcceptance(data, providerModule, acc, prior)
		if err != nil {
			return nil, err
		}
		if out["acceptance_test.go"], err = renderAcceptance(accData); err != nil {
			return nil, err
		}
		if accData.UpgradeAttributes != nil {
			if out[acceptance.UpgradeFileName], err = accData.UpgradeAttributes.Marshal(); err != nil {
				return nil, err
			}
		}
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
