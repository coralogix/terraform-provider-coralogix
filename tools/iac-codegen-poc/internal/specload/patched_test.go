package specload_test

import (
	"os"
	"slices"
	"testing"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
)

const patchedSpec = "../../spec/openapi.patched.yaml"

// TestPatchedSpec checks that the patched spec loads and has the O1–O5 values
// in the request bodies (README.md, "Overlay").
func TestPatchedSpec(t *testing.T) {
	doc := loadPatched(t)
	create := libopenapiBody(t, doc.Paths.PathItems.GetOrZero(pathCollection).Post)
	update := libopenapiBody(t, doc.Paths.PathItems.GetOrZero(pathItem).Patch)

	t.Run("O1 create required", func(t *testing.T) {
		if want := []string{"application", "subsystem", "config"}; !slices.Equal(create.Required, want) {
			t.Errorf("required = %v, want %v", create.Required, want)
		}
	})
	t.Run("O2 create presence", func(t *testing.T) {
		checkPresence(t, create, []string{"target", "threshold"}, true)
		checkPresence(t, create, []string{"application", "subsystem", "config", "isEnabled"}, false)
	})
	t.Run("O3 create isEnabled default", func(t *testing.T) {
		if d := mustProp(t, create, "isEnabled").Default; d == nil || d.Value != "false" {
			t.Errorf("default = %v, want false", d)
		}
	})
	t.Run("O4 update properties", func(t *testing.T) {
		want := []string{"config", "isEnabled", "threshold", "updateMask"}
		if got := libopenapiKeys(update); !slices.Equal(got, want) {
			t.Errorf("update properties = %v, want %v", got, want)
		}
	})
	t.Run("O5 update presence", func(t *testing.T) {
		checkPresence(t, update, []string{"config", "isEnabled", "threshold"}, true)
	})
}

// TestPatchedSpecComponents checks the O6–O7 values in the component schemas.
func TestPatchedSpecComponents(t *testing.T) {
	doc := loadPatched(t)
	t.Run("O6 sets", func(t *testing.T) {
		for _, c := range [][2]string{
			{"AllowedTopicsConfig", "topics"},
			{"RestrictedTopicsConfig", "topics"},
			{"CompetitionConfig", "competitors"},
			{"SqlAllowedTablesConfig", "tables"},
			{"SqlRestrictedTablesConfig", "tables"},
			{"PiiConfig", "categories"},
		} {
			p := mustProp(t, mustComponent(t, doc, c[0]), c[1])
			if p.UniqueItems == nil || !*p.UniqueItems {
				t.Errorf("%s.%s: uniqueItems is not true", c[0], c[1])
			}
			if n := p.Extensions.GetOrZero("x-coralogix-collection"); n == nil || n.Value != "set" {
				t.Errorf("%s.%s: x-coralogix-collection is not set", c[0], c[1])
			}
		}
		if p := mustProp(t, mustComponent(t, doc, "CustomEvaluationConfig"), "examples"); p.UniqueItems != nil {
			t.Error("CustomEvaluationConfig.examples: must stay a list (D6)")
		}
	})
	t.Run("O7 uint64", func(t *testing.T) {
		for _, c := range [][2]string{
			{"SqlLoadConfig", "joinLimit"},
			{"SqlLoadConfig", "cteLimit"},
			{"CustomEvaluationExample", "score"},
		} {
			p := mustProp(t, mustComponent(t, doc, c[0]), c[1])
			if p.Format != "uint64" || !slices.Equal(p.Type, []string{"string"}) {
				t.Errorf("%s.%s: format %q, type %v, want uint64 and [string]", c[0], c[1], p.Format, p.Type)
			}
		}
	})
}

func loadPatched(t *testing.T) *v3.Document {
	t.Helper()
	data, err := os.ReadFile(patchedSpec)
	if err != nil {
		t.Fatal(err)
	}
	return loadLibopenapi(t, data)
}

func mustProp(t *testing.T, s *base.Schema, name string) *base.Schema {
	t.Helper()
	p := s.Properties.GetOrZero(name)
	if p == nil {
		t.Fatalf("property %q not found", name)
	}
	return p.Schema()
}

func mustComponent(t *testing.T, doc *v3.Document, name string) *base.Schema {
	t.Helper()
	c := doc.Components.Schemas.GetOrZero(name)
	if c == nil {
		t.Fatalf("component %q not found", name)
	}
	return c.Schema()
}

// checkPresence checks that each of names in s has x-coralogix-presence: true
// when want is true, and has no presence marker when want is false.
func checkPresence(t *testing.T, s *base.Schema, names []string, want bool) {
	t.Helper()
	for _, name := range names {
		n := mustProp(t, s, name).Extensions.GetOrZero("x-coralogix-presence")
		if got := n != nil && n.Value == "true"; got != want {
			t.Errorf("%s: x-coralogix-presence = %v, want %v", name, got, want)
		}
	}
}
