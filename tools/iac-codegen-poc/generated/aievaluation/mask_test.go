// This file is handwritten. It tests the generated update mask code in
// mask.go.

package aievaluation

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// stored returns the model of an existing evaluation, as in the state.
func stored() *AiEvaluationModel {
	return &AiEvaluationModel{
		Id:          types.StringValue("e1"),
		Application: types.StringValue("app"),
		Subsystem:   types.StringValue("sub"),
		Target:      types.StringValue("PROMPT"),
		IsEnabled:   types.BoolValue(true),
		Threshold:   types.Float64Value(0.5),
		Config: &EvaluationConfigModel{
			AllowedTopics: &AllowedTopicsConfigModel{Topics: strSet("a", "b")},
		},
		UpdatedAt: types.StringValue("2026-09-25T10:00:00Z"),
	}
}

func toState(t *testing.T, m *AiEvaluationModel) tfsdk.State {
	t.Helper()
	ctx := context.Background()
	s := tfsdk.State{Schema: Schema(), Raw: tftypes.NewValue(Schema().Type().TerraformType(ctx), nil)}
	if diags := s.Set(ctx, m); diags.HasError() {
		t.Fatalf("set state: %v", diags)
	}
	return s
}

func toPlan(t *testing.T, m *AiEvaluationModel) tfsdk.Plan {
	t.Helper()
	s := toState(t, m)
	return tfsdk.Plan{Schema: s.Schema, Raw: s.Raw}
}

// TestUpdateRequest changes the stored model into a plan, and checks the
// update mask and the Update body.
func TestUpdateRequest(t *testing.T) {
	cases := []struct {
		name   string
		change func(m *AiEvaluationModel)
		mask   []string
		body   string // "" when no request is sent
	}{
		{
			name:   "no change",
			change: func(m *AiEvaluationModel) {},
		},
		{
			name:   "same set in another order",
			change: func(m *AiEvaluationModel) { m.Config.AllowedTopics.Topics = strSet("b", "a") },
		},
		{
			name: "immutable and computed fields are never in the mask",
			change: func(m *AiEvaluationModel) {
				m.Application = types.StringValue("other")
				m.Subsystem = types.StringValue("other")
				m.Target = types.StringValue("RESPONSE")
				m.UpdatedAt = types.StringUnknown()
			},
		},
		{
			name:   "change",
			change: func(m *AiEvaluationModel) { m.IsEnabled = types.BoolValue(false) },
			mask:   []string{"isEnabled"},
			body:   `{"updateMask":"isEnabled","isEnabled":false,"threshold":0.5,"config":{"allowedTopics":{"topics":["a","b"]}}}`,
		},
		{
			name:   "clear",
			change: func(m *AiEvaluationModel) { m.Threshold = types.Float64Null() },
			mask:   []string{"threshold"},
			body:   `{"updateMask":"threshold","isEnabled":true,"config":{"allowedTopics":{"topics":["a","b"]}}}`,
		},
		{
			name:   "change to zero",
			change: func(m *AiEvaluationModel) { m.Threshold = types.Float64Value(0) },
			mask:   []string{"threshold"},
			body:   `{"updateMask":"threshold","isEnabled":true,"threshold":0,"config":{"allowedTopics":{"topics":["a","b"]}}}`,
		},
		{
			name:   "change inside config",
			change: func(m *AiEvaluationModel) { m.Config.AllowedTopics.Topics = strSet("a", "c") },
			mask:   []string{"config"},
			body:   `{"updateMask":"config","isEnabled":true,"threshold":0.5,"config":{"allowedTopics":{"topics":["a","c"]}}}`,
		},
		{
			name:   "change of config arm",
			change: func(m *AiEvaluationModel) { m.Config = &EvaluationConfigModel{Toxicity: &ToxicityConfigModel{}} },
			mask:   []string{"config"},
			body:   `{"updateMask":"config","isEnabled":true,"threshold":0.5,"config":{"toxicity":{}}}`,
		},
		{
			name: "all Update fields, in model order",
			change: func(m *AiEvaluationModel) {
				m.Threshold = types.Float64Null()
				m.IsEnabled = types.BoolValue(false)
				m.Config.AllowedTopics.Topics = strSet()
			},
			mask: []string{"config", "isEnabled", "threshold"},
			body: `{"updateMask":"config,isEnabled,threshold","isEnabled":false,"config":{"allowedTopics":{"topics":[]}}}`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			m := stored()
			c.change(m)
			plan, state := toPlan(t, m), toState(t, stored())

			mask, diags := updateMask(ctx, plan, state)
			assertNoDiags(t, diags)
			if !reflect.DeepEqual(mask, c.mask) {
				t.Errorf("mask: got %q, want %q", mask, c.mask)
			}

			body, diags := updateRequest(ctx, plan, state)
			assertNoDiags(t, diags)
			if c.body == "" {
				if body != nil {
					t.Errorf("got a request, want none: %+v", body)
				}
				return
			}
			assertJSON(t, body, c.body)
		})
	}
}

// TestUpdateMaskFromNull checks that a field with no value in the state and
// a value in the plan is in the mask.
func TestUpdateMaskFromNull(t *testing.T) {
	old := stored()
	old.Threshold = types.Float64Null()
	mask, diags := updateMask(context.Background(), toPlan(t, stored()), toState(t, old))
	assertNoDiags(t, diags)
	if want := []string{"threshold"}; !reflect.DeepEqual(mask, want) {
		t.Errorf("mask: got %q, want %q", mask, want)
	}
}
