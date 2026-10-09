package generator

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/model"
)

func wrapThingSpec(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "model", "testdata", "wrapthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func wrapThingOverrides(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "wrapthing-overrides.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// wrapThingCodes validates the wrapthing contract with its overrides file, changed by each pair of
// old and new text.
func wrapThingCodes(t *testing.T, replacements ...string) []string {
	t.Helper()
	text := wrapThingOverrides(t)
	for i := 0; i < len(replacements); i += 2 {
		if !strings.Contains(text, replacements[i]) {
			t.Fatalf("the overrides file has no %q", replacements[i])
		}
		text = strings.Replace(text, replacements[i], replacements[i+1], 1)
	}
	_, err := validateOpenAPIWith(wrapThingSpec(t), "WrapThing", model.OperationIDs{}, "sdk", "provider", mustParse(t, text))
	return eligibilityCodes(t, err)
}

func TestUnwrapLinesThatDecideNothingAreUnused(t *testing.T) {
	if codes := wrapThingCodes(t); len(codes) != 0 {
		t.Fatalf("codes %v, want none for the valid file", codes)
	}
	tests := map[string][]string{
		"component no field holds": {"ListSelection]", "ListSelection, Missing]"},
		"false line, not listed":   {"      mainLabel:\n", "      extraLabel:\n        unwrap: false\n      mainLabel:\n"},
	}
	for name, replacements := range tests {
		t.Run(name, func(t *testing.T) {
			if codes := wrapThingCodes(t, replacements...); !slices.Equal(codes, []string{"OVERRIDE_UNUSED"}) {
				t.Fatalf("codes %v, want [OVERRIDE_UNUSED]", codes)
			}
		})
	}
}

func TestUnwrapRejectsKeysThatReadTheWrappedField(t *testing.T) {
	codes := wrapThingCodes(t, "      mainLabel:\n", "      widgetIds:\n        keepPriorOrder: true\n      mainLabel:\n")
	if !slices.Contains(codes, "UNWRAP_COMBINATION_UNSUPPORTED") {
		t.Fatalf("codes %v do not contain UNWRAP_COMBINATION_UNSUPPORTED", codes)
	}
}

func TestUnwrapRejectsAListedComponentThatIsNoWrapper(t *testing.T) {
	codes := wrapThingCodes(t, "ListSelection]", "ListSelection, SimpleFilter]")
	if !slices.Contains(codes, "UNWRAP_COMPONENT_UNSUPPORTED") {
		t.Fatalf("codes %v do not contain UNWRAP_COMPONENT_UNSUPPORTED", codes)
	}
}

// Terraform does not show the wrappers of an object, but the update mask names them.
func TestMaskPathsNameTheWrappers(t *testing.T) {
	filter := &model.Type{Kind: model.Object, Schema: "SimpleFilter",
		Fields:   []*model.Field{{Name: "text", Type: &model.Type{Kind: model.String}}},
		Wrappers: []model.Wrapper{{Schema: "FilterHolder", Field: "filter"}},
	}
	tree := maskTree("filter", filter)
	var paths []string
	if err := checkMaskPaths(tree, "", func(p string) bool { paths = append(paths, p); return true }); err != nil {
		t.Fatal(err)
	}
	if want := []string{"filter", "filter.filter.text"}; !slices.Equal(paths, want) {
		t.Fatalf("paths %v, want %v", paths, want)
	}
	if tree.Children[0].TFName != "text" {
		t.Fatalf("Terraform name %q, want text", tree.Children[0].TFName)
	}
}

// A wrapper field that is a value, not a pointer, takes the struct, and unwrap reads it without a
// nil check.
func TestWrapHelpersOfValueFields(t *testing.T) {
	steps := []wrapStep{
		{Type: "sdk.Outer", Field: "Middle", Value: true},
		{Type: "sdk.Middle", Field: "Inner"},
		{Type: "sdk.Inner", Field: "Value", Value: true},
	}
	if got, want := wrapBuild(steps), "&sdk.Outer{Middle: sdk.Middle{Inner: &sdk.Inner{Value: *v}}}"; got != want {
		t.Errorf("wrapBuild = %s, want %s", got, want)
	}
	if got, want := wrapNilCheck(steps), "w == nil || w.Middle.Inner == nil"; got != want {
		t.Errorf("wrapNilCheck = %s, want %s", got, want)
	}
	if got, want := wrapRead(steps, true), "&w.Middle.Inner.Value"; got != want {
		t.Errorf("wrapRead = %s, want %s", got, want)
	}
	if got, want := wrapRead(steps[:2], false), "w.Middle.Inner"; got != want {
		t.Errorf("wrapRead of a slice = %s, want %s", got, want)
	}
}

func TestQualifyType(t *testing.T) {
	b := &convBuilder{ix: &refIndex{pkg: sdkRef{Name: "sdk"}}}
	for in, want := range map[string]string{
		"*string": "*string", "[]string": "[]string", "*Thing": "*sdk.Thing", "[]Thing": "[]sdk.Thing",
		"*time.Time": "*time.Time", "map[string]interface{}": "map[string]interface{}", "map[string]Kind": "map[string]sdk.Kind",
	} {
		if got := b.qualifyType(in); got != want {
			t.Errorf("qualifyType(%q) = %q, want %q", in, got, want)
		}
	}
}
