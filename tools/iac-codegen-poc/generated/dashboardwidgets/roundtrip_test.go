// This file is handwritten. It runs real dashboard widgets through the
// generated types (D20): the widget definitions in testdata (copies of the
// provider's dashboard test fixtures in the API shape) are read by the SDK,
// flattened, stored in a Terraform state built from the generated schema,
// read back, and expanded. The API values must not change.

package dashboardwidgets_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	sdk "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/generated/dashboardwidgets"
)

// host is a handwritten model that embeds the generated widget definition.
type host struct {
	Definition *dashboardwidgets.WidgetDefinitionModel `tfsdk:"definition"`
}

func hostSchema() schema.Schema {
	return schema.Schema{Attributes: map[string]schema.Attribute{
		"definition": schema.SingleNestedAttribute{Optional: true, Attributes: dashboardwidgets.WidgetDefinitionAttributes()},
	}}
}

// definitions collects the "definition" objects of every widget in v.
func definitions(v any, out *[]json.RawMessage) {
	switch x := v.(type) {
	case map[string]any:
		for k, e := range x {
			if k == "definition" {
				b, _ := json.Marshal(e)
				*out = append(*out, b)
			}
			definitions(e, out)
		}
	case []any:
		for _, e := range x {
			definitions(e, out)
		}
	}
}

func TestRealWidgets(t *testing.T) {
	files, err := filepath.Glob("testdata/*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("no fixtures: %v", err)
	}
	seen := map[string]bool{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var doc any
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		var defs []json.RawMessage
		definitions(doc, &defs)
		for i, raw := range defs {
			name := filepath.Base(f) + "#" + string(rune('0'+i))
			var def sdk.WidgetDefinition
			if err := json.Unmarshal(raw, &def); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			for k := range map[string]any(mustMap(t, raw)) {
				seen[k] = true
			}
			roundTrip(t, name, &def)
		}
	}
	t.Logf("widget kinds in the fixtures: %v", keys(seen))
}

func roundTrip(t *testing.T, name string, def *sdk.WidgetDefinition) {
	t.Helper()
	ctx := context.Background()
	p := path.Root("definition")
	m, diags := dashboardwidgets.FlattenWidgetDefinition(ctx, p, def)
	if diags.HasError() {
		t.Fatalf("%s: flatten: %v", name, diags)
	}
	s := hostSchema()
	state := tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
	if diags := state.Set(ctx, &host{Definition: m}); diags.HasError() {
		t.Fatalf("%s: state: %v", name, diags)
	}
	var back host
	if diags := state.Get(ctx, &back); diags.HasError() {
		t.Fatalf("%s: read the state: %v", name, diags)
	}
	out, diags := dashboardwidgets.ExpandWidgetDefinition(ctx, p, back.Definition)
	if diags.HasError() {
		t.Fatalf("%s: expand: %v", name, diags)
	}
	want, _ := json.Marshal(def)
	got, _ := json.Marshal(out)
	if string(got) != string(want) {
		t.Errorf("%s differs:\n got %s\nwant %s", name, got, want)
	}
}

func mustMap(t *testing.T, raw json.RawMessage) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func keys(m map[string]bool) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
