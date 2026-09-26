// This file is handwritten. It tests two shapes that the real API uses: a
// oneOf with a discriminator field (as dashboards SortStrategy), and a
// base64 string with format byte (as custom enrichments File.binary).

package fakeboard

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestDiscriminator checks that the discriminator is a normal string field
// beside the oneOf arms: expand sends it as it is, and the arms keep their
// oneOf rules. The spec does not say who sets it (F36).
func TestDiscriminator(t *testing.T) {
	m := withGroups(t)
	m.Panels = panels(t, map[string]PanelModel{"cpu": {
		Query: types.StringValue("q"),
		Sort:  &SortStrategyModel{ByValue: &EveryModel{Minutes: types.Int32Value(1)}, StrategyType: types.StringValue("BY_VALUE")},
	}})
	body, diags := expandCreate(context.Background(), m)
	assertNoDiags(t, diags)
	assertJSON(t, body.Panels["cpu"].Sort, `{"byValue":{"minutes":1},"strategyType":"BY_VALUE"}`)

	v := unmarshalBoard(t, `{"id":"b1","name":"ops","layout":{"title":"T","relativeTime":{}},
		"panels":{"cpu":{"query":"q","sort":{"byName":{},"strategyType":"BY_NAME"}}}}`)
	back, diags := flatten(context.Background(), v)
	assertNoDiags(t, diags)
	var ps map[string]PanelModel
	assertNoDiags(t, back.Panels.ElementsAs(context.Background(), &ps, false))
	if s := ps["cpu"].Sort; s == nil || s.ByName == nil || s.ByValue != nil || s.StrategyType.ValueString() != "BY_NAME" {
		t.Errorf("sort = %+v", s)
	}

	m.Panels = panels(t, map[string]PanelModel{"cpu": {
		Query: types.StringValue("q"),
		Sort:  &SortStrategyModel{ByName: &BoldStyleModel{}, ByValue: &EveryModel{}},
	}})
	if diags := validateAll(t, m); !diags.HasError() {
		t.Error("two sort arms: no error")
	}
}

// TestByteString checks that a format byte field is base64 text: it is sent
// and read back unchanged.
func TestByteString(t *testing.T) {
	m := withGroups(t)
	m.Icon = types.StringValue("aGVsbG8=")
	body, diags := expandCreate(context.Background(), m)
	assertNoDiags(t, diags)
	assertJSON(t, body.Icon, `"aGVsbG8="`)

	back, diags := flatten(context.Background(), unmarshalBoard(t, `{"id":"b1","name":"ops","icon":"aGVsbG8="}`))
	assertNoDiags(t, diags)
	if !back.Icon.Equal(types.StringValue("aGVsbG8=")) {
		t.Errorf("icon = %s", back.Icon)
	}
}
