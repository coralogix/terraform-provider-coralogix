package generator

import (
	"slices"
	"strings"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

const deprecationSpec = `openapi: 3.1.0
info: {title: t, version: 1.0.0}
paths: {}
components:
  schemas:
    Target:
      type: object
      deprecated: true
      properties:
        url: {type: string}
    Thing:
      type: object
      properties:
        plain: {type: string}
        inline: {type: string, deprecated: true}
        ref:
          $ref: '#/components/schemas/Target'
          deprecated: true
        wrapped:
          allOf:
            - $ref: '#/components/schemas/Target'
          deprecated: true
        target:
          $ref: '#/components/schemas/Target'
        notDeprecated: {type: string, deprecated: false}
`

// The contract marks a field deprecated on the property itself: inline, next to a $ref, or on an
// allOf wrapper. A property that only names a deprecated component is not deprecated, and neither is
// a property with deprecated: false.
func TestContractDeprecationsReadThePropertyItself(t *testing.T) {
	got := contractDeprecations([]byte(deprecationSpec))
	for _, want := range []string{"Thing.inline", "Thing.ref", "Thing.wrapped"} {
		if !got[want] {
			t.Errorf("%s is not deprecated, want it to be: %v", want, got)
		}
	}
	for _, none := range []string{"Thing.plain", "Thing.target", "Thing.notDeprecated", "Target.url"} {
		if got[none] {
			t.Errorf("%s is deprecated, want it not to be", none)
		}
	}
}

// A deprecated attribute that the user can set gets the warning, also inside an object and a list of
// objects and as an arm of a oneOf. A Required attribute, an attribute that only the server sets, and an
// attribute with a line of its own get none: the line wins.
func TestMarkDeprecationsFollowsTheMode(t *testing.T) {
	deprecated := map[string]bool{"Thing.a": true, "Thing.req": true, "Thing.srv": true, "Thing.own": true, "Inner.b": true, "Inner.arm": true}
	attrs := []*tfAttr{
		{Name: "a", Component: "Thing", Property: "a", Optional: true},
		{Name: "both", Component: "Thing", Property: "a", Optional: true, Computed: true},
		{Name: "req", Component: "Thing", Property: "req", Required: true},
		{Name: "srv", Component: "Thing", Property: "srv", Computed: true},
		{Name: "own", Component: "Thing", Property: "own", Optional: true, DeprecationMessage: "Use another."},
		{Name: "fine", Component: "Thing", Property: "fine", Optional: true},
		{Name: "obj", Component: "Thing", Property: "obj", Optional: true, Attributes: []*tfAttr{
			{Name: "b", Component: "Inner", Property: "b", Optional: true},
			{Name: "arm", Component: "Inner", Property: "arm", Optional: true, OneOfGroup: "arm,other"},
		}},
	}
	markDeprecations(attrs, deprecated, nil)
	want := map[string]string{"a": overrides.DeprecationText, "both": overrides.DeprecationText, "req": "", "srv": "", "own": "Use another.", "fine": ""}
	for _, a := range attrs[:6] {
		if a.DeprecationMessage != want[a.Name] {
			t.Errorf("%s: message %q, want %q", a.Name, a.DeprecationMessage, want[a.Name])
		}
	}
	for _, a := range attrs[6].Attributes {
		if a.DeprecationMessage != overrides.DeprecationText {
			t.Errorf("nested %s: message %q, want the warning", a.Name, a.DeprecationMessage)
		}
	}
}

// A deprecation line that says what the generator writes is stale. A line with another text is not.
func TestDeprecationLineThatRepeatsTheWarningIsUnused(t *testing.T) {
	const head = "resource: LegacyThing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n  LegacyRule:\n    fields:\n      name: {deprecation: %s}\n"
	for name, text := range map[string]string{"the warning": `"` + overrides.DeprecationText + `"`, "another text": `"Use another field."`} {
		t.Run(name, func(t *testing.T) {
			file := mustParse(t, strings.Replace(head, "%s", text, 1))
			_, err := validateOpenAPIWith(legacySpec(t), "LegacyThing", model.OperationIDs{}, "sdk", "provider", file)
			unused := slices.Contains(eligibilityCodes(t, err), "OVERRIDE_UNUSED")
			if unused != (name == "the warning") {
				t.Fatalf("err = %v, OVERRIDE_UNUSED = %v", err, unused)
			}
		})
	}
}
