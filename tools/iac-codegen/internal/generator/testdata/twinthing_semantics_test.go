package parsething

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"example.com/iac-test-sdk/go/openapi/gen/parse_things_service"
)

func ptr[T any](v T) *T { return &v }

func expandStep(m *ParseStepModel) (*parse_things_service.CreateParseStep, diag.Diagnostics) {
	var diags diag.Diagnostics
	step := expandCreateParseStep(context.Background(), path.Empty(), m, &diags)
	return step, diags
}

func flattenStep(step *parse_things_service.ParseStep) (*ParseStepModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	model := flattenParseStep(context.Background(), path.Empty(), step, &diags)
	return model, diags
}

func wantError(t *testing.T, diags diag.Diagnostics, text string) {
	t.Helper()
	if !diags.HasError() {
		t.Fatalf("no error, want one with %q", text)
	}
	if !strings.Contains(diags.Errors()[0].Detail(), text) {
		t.Fatalf("error %q, want %q", diags.Errors()[0].Detail(), text)
	}
}

// The flag keep_matching = true sends the shown arm with the field that only it has. A field that
// only the folded arm has is refused.
func TestFoldShownArmSendsItsOwnFields(t *testing.T) {
	step, diags := expandStep(&ParseStepModel{KeepParams: &ParseKeepParamsModel{
		Rule: types.StringValue("x"), KeepLogs: types.BoolValue(true), Label: types.StringValue("l"),
		Limits: &ParseLimitsModel{Max: types.Int64Value(3)}, KeepMatching: types.BoolValue(true),
	}})
	if diags.HasError() {
		t.Fatal(diags)
	}
	keep := step.Params.KeepParams
	if step.Params.DropParams != nil || keep == nil || *keep.Rule != "x" || *keep.Label != "l" || *keep.Limits.Max != 3 {
		t.Fatalf("sent %+v", step.Params)
	}

	_, diags = expandStep(&ParseStepModel{KeepParams: &ParseKeepParamsModel{Reason: types.StringValue("r"), KeepMatching: types.BoolValue(true)}})
	wantError(t, diags, "reason applies only when keep_matching is false")
	_, diags = expandStep(&ParseStepModel{KeepParams: &ParseKeepParamsModel{Extra: &ParseExtraModel{}, KeepMatching: types.BoolValue(true)}})
	wantError(t, diags, "extra applies only when keep_matching is false")
}

// The flag keep_matching = false sends the folded arm: the common fields are copied, and the fields
// that only the folded arm has are written to it. A field that only the shown arm has is refused,
// not dropped.
func TestFoldFoldedArmSendsTheCommonAndItsOwnFields(t *testing.T) {
	step, diags := expandStep(&ParseStepModel{KeepParams: &ParseKeepParamsModel{
		Rule: types.StringValue("x"), KeepLogs: types.BoolValue(false), Reason: types.StringValue("r"),
		Threshold: types.Int64Value(7), Extra: &ParseExtraModel{Detail: types.StringValue("d")}, KeepMatching: types.BoolValue(false),
	}})
	if diags.HasError() {
		t.Fatal(diags)
	}
	drop := step.Params.DropParams
	if step.Params.KeepParams != nil || drop == nil || *drop.Rule != "x" || *drop.KeepLogs || *drop.Reason != "r" || *drop.Threshold != 7 || *drop.Extra.Detail != "d" {
		t.Fatalf("sent %+v", step.Params)
	}

	_, diags = expandStep(&ParseStepModel{KeepParams: &ParseKeepParamsModel{Label: types.StringValue("l"), KeepMatching: types.BoolValue(false)}})
	wantError(t, diags, "label applies only when keep_matching is true")
	_, diags = expandStep(&ParseStepModel{KeepParams: &ParseKeepParamsModel{Limits: &ParseLimitsModel{}, KeepMatching: types.BoolValue(false)}})
	wantError(t, diags, "limits applies only when keep_matching is true")
}

// A null flag is true, as without the extra fields. A null or unknown field is no set field.
func TestFoldNullFlagSelectsTheShownArm(t *testing.T) {
	step, diags := expandStep(&ParseStepModel{KeepParams: &ParseKeepParamsModel{
		Rule: types.StringValue("x"), Reason: types.StringUnknown(), Threshold: types.Int64Null(),
	}})
	if diags.HasError() {
		t.Fatal(diags)
	}
	if step.Params.KeepParams == nil || step.Params.DropParams != nil {
		t.Fatalf("sent %+v", step.Params)
	}
}

// The read fills each field from the arm that the response has. A field of the other arm is null.
func TestFoldReadFillsTheFieldsOfTheArmThatTheResponseHas(t *testing.T) {
	dropped, diags := flattenStep(&parse_things_service.ParseStep{Params: &parse_things_service.ParseParams{DropParams: &parse_things_service.ParseDropParams{
		Rule: ptr("x"), KeepLogs: ptr(false), Reason: ptr("r"), Threshold: ptr(int64(7)), Extra: &parse_things_service.ParseExtra{Detail: ptr("d")},
	}}})
	if diags.HasError() {
		t.Fatal(diags)
	}
	k := dropped.KeepParams
	if k == nil || k.KeepMatching.ValueBool() || k.Rule.ValueString() != "x" || k.Reason.ValueString() != "r" || k.Threshold.ValueInt64() != 7 || k.Extra == nil || k.Extra.Detail.ValueString() != "d" {
		t.Fatalf("drop read = %+v", k)
	}
	if !k.Label.IsNull() || k.Limits != nil || !k.Strict.IsNull() {
		t.Fatalf("the fields of the shown arm are %+v %+v %+v, want null", k.Label, k.Limits, k.Strict)
	}

	kept, diags := flattenStep(&parse_things_service.ParseStep{Params: &parse_things_service.ParseParams{KeepParams: &parse_things_service.ParseKeepParams{
		Rule: ptr("x"), Label: ptr("l"), Limits: &parse_things_service.ParseLimits{Max: ptr(int64(3))},
	}}})
	if diags.HasError() {
		t.Fatal(diags)
	}
	k = kept.KeepParams
	if k == nil || !k.KeepMatching.ValueBool() || k.Label.ValueString() != "l" || k.Limits == nil || k.Limits.Max.ValueInt64() != 3 {
		t.Fatalf("keep read = %+v", k)
	}
	if !k.Reason.IsNull() || !k.Threshold.IsNull() || k.Extra != nil || !k.Hard.IsNull() {
		t.Fatalf("the fields of the folded arm are %+v %+v %+v %+v, want null", k.Reason, k.Threshold, k.Extra, k.Hard)
	}
}

// An arm without a fold is unchanged.
func TestFoldLeavesOtherArmsAlone(t *testing.T) {
	step, diags := expandStep(&ParseStepModel{CopyParams: &ParseCopyParamsModel{Target: types.StringValue("t")}})
	if diags.HasError() || step.Params.CopyParams == nil || *step.Params.CopyParams.Target != "t" {
		t.Fatalf("sent %+v %v", step.Params, diags)
	}
}
