package generator

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/issue"
	"github.com/coralogix/coralogix-management-sdk/iac/model"
)

// Gadget declares server defaults on a top-level enum next to its $ref, on a field that only
// replace sets, and on a nested enum. Each is Optional and Computed with the server-default plan
// modifier, and only the nested one is a request path that drops an unknown value.
func TestGadgetServerDefaultAttributes(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "gadget.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := model.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	r, err := model.Build(doc, "Gadget")
	if err != nil {
		t.Fatal(err)
	}
	out, err := buildTFResource(r, "gadget")
	if err != nil {
		t.Fatal(err)
	}
	attr := func(attrs []*tfAttr, name string) *tfAttr {
		i := slices.IndexFunc(attrs, func(a *tfAttr) bool { return a.Name == name })
		if i < 0 {
			t.Fatalf("no attribute %s", name)
		}
		return attrs[i]
	}
	for name, a := range map[string]*tfAttr{
		"mode":          attr(out.Attributes, "mode"),
		"compact":       attr(out.Attributes, "compact"),
		"search.syntax": attr(attr(out.Attributes, "search").Attributes, "syntax"),
	} {
		if !a.Optional || !a.Computed || len(a.Modifiers) != 1 {
			t.Errorf("%s: optional %t computed %t modifiers %v, want Optional, Computed, and the server-default modifier", name, a.Optional, a.Computed, a.Modifiers)
		}
	}
	if want := []defaultPath{{Path: "search.syntax", Value: `types.StringValue("GADGET_SYNTAX_PLAIN")`}}; !slices.Equal(out.DefaultPaths, want) {
		t.Errorf("DefaultPaths = %v, want %v", out.DefaultPaths, want)
	}
	// Only the object that holds the default reads "" as empty.
	if want := []string{"search"}; !slices.Equal(out.DefaultObjects(), want) {
		t.Errorf("DefaultObjects = %v, want %v", out.DefaultObjects(), want)
	}
	if text := attr(out.Attributes, "search").Attributes; attr(text, "text").Computed {
		t.Error("search.text has no default, so it is not computed")
	}
}

// A nested component that the requests and the response share binds the requests with its
// required list. A required integer without presence there needs a line: required: true says
// that Terraform sends it every time, and the line is not stale.
func TestRequiredLineStatesPresenceOfASharedComponent(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "gadget.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	spec := strings.Replace(string(data), "      required: [text]\n      properties:\n", "      required: [text, count]\n      properties:\n        count: {type: integer, format: int32, minimum: 0}\n", 1)
	if spec == string(data) {
		t.Fatal("the fixture changed: update the replaced text")
	}
	head := "resource: Gadget\nmode: existing\nvalidators:\n  inferred: true\n"
	for name, test := range map[string]struct {
		file string
		code string
	}{
		"no line":       {head, "REQUIRED_SCALAR_PRESENCE_UNKNOWN"},
		"required line": {head + "types:\n  GadgetSearch:\n    fields:\n      count: {required: true}\n", ""},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := validateOpenAPIWith([]byte(spec), "Gadget", model.OperationIDs{}, "candidate.invalid/coralogix-management-sdk", providerModulePath, mustParse(t, test.file))
			if test.code == "" {
				if err != nil {
					t.Fatalf("err = %v, want none", err)
				}
				return
			}
			if !slices.ContainsFunc(reportOf(t, err), func(i issue.Issue) bool { return i.Code == test.code }) {
				t.Fatalf("err = %v, want %s", err, test.code)
			}
		})
	}
}

// Flatten cannot match a set element that the API returns without a nested default to the
// configured element, so a nested server default inside a set is an error. In a list it works.
func TestNestedServerDefaultInsideASetIsAnError(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "gadget.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for collection, wantErr := range map[string]bool{"set": true, "list": false} {
		extra := ""
		if collection == "set" {
			extra = ", x-coralogix-collection: set"
		}
		spec := strings.ReplaceAll(string(data), "search: {$ref: '#/components/schemas/GadgetSearch'}",
			"search: {type: array"+extra+", items: {$ref: '#/components/schemas/GadgetSearch'}}")
		doc, err := model.Load([]byte(spec))
		if err != nil {
			t.Fatal(err)
		}
		r, err := model.Build(doc, "Gadget")
		if err != nil {
			t.Fatal(err)
		}
		_, err = buildTFResource(r, "gadget")
		if got := err != nil && strings.Contains(err.Error(), "inside a set is unsupported"); got != wantErr {
			t.Errorf("%s: err = %v, want the set error %t", collection, err, wantErr)
		}
	}
}
