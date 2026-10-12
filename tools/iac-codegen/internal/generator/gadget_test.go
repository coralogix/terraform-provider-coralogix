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
	// An integer enum accepts only its values, and a map of booleans with the enum [true]
	// accepts only true values.
	if got := attr(out.Attributes, "level").Validators; !slices.Equal(got, []string{"int32validator.OneOf(1, 5)"}) {
		t.Errorf("level validators = %v, want int32validator.OneOf(1, 5)", got)
	}
	flags := attr(attr(out.Attributes, "search").Attributes, "flags")
	if !slices.Equal(flags.Validators, []string{"mapBoolValuesAre(boolvalidator.Equals(true))"}) || !out.MapBoolValues {
		t.Errorf("search.flags validators = %v, MapBoolValues = %t; want mapBoolValuesAre(boolvalidator.Equals(true))", flags.Validators, out.MapBoolValues)
	}
}

// The made-up values of the acceptance test keep to the values that a validator lists: the
// first and the second integer, and the one boolean in both configs.
func TestAcceptanceValuesFollowListedValues(t *testing.T) {
	level := &tfAttr{Name: "level", Kind: "Int32", Validators: []string{"int32validator.OneOf(1, 5)"}}
	only := &tfAttr{Name: "flag", Kind: "Bool", Validators: []string{"boolvalidator.Equals(true)"}}
	for _, test := range []struct {
		a    *tfAttr
		mode accMode
		want string
	}{
		{level, accFull, "1"}, {level, accUpdated, "5"},
		{only, accFull, "true"}, {only, accUpdated, "true"},
	} {
		got, _, err := scalarValue(test.a, test.mode)
		if err != nil || got != test.want {
			t.Errorf("%s in mode %v = %q, %v; want %q", test.a.Name, test.mode, got, err, test.want)
		}
	}
}
