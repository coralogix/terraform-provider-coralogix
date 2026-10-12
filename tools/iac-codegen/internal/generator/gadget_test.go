package generator

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/model"
)

// Gadget declares server defaults on a top-level enum next to its $ref, on a field that only
// replace sets, and on a nested enum. Each is Optional and Computed with the server-default plan
// modifier, and only the nested one is a request path that drops an unknown value.
func TestGadgetServerDefaultAttributes(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "model", "testdata", "gadget.yaml"))
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
	if text := attr(out.Attributes, "search").Attributes; attr(text, "text").Computed {
		t.Error("search.text has no default, so it is not computed")
	}
}
