// This file is handwritten. It tests the generated expand and flatten code
// in convert.go.

package aievaluation

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/coralogix/coralogix-management-sdk/go/openapi/gen/ai_evaluations_service"
)

func strSet(vs ...string) types.Set {
	elems := []attr.Value{}
	for _, v := range vs {
		elems = append(elems, types.StringValue(v))
	}
	return types.SetValueMust(types.StringType, elems)
}

type example struct {
	conversation string
	score        int64
}

func examples(es ...example) types.List {
	elemType := types.ObjectType{AttrTypes: customEvaluationExampleAttrTypes()}
	elems := []attr.Value{}
	for _, e := range es {
		elems = append(elems, types.ObjectValueMust(elemType.AttrTypes, map[string]attr.Value{
			"conversation": types.StringValue(e.conversation),
			"score":        types.Int64Value(e.score),
		}))
	}
	return types.ListValueMust(elemType, elems)
}

// withConfig returns a model that has only config set. A zero Terraform
// value is null.
func withConfig(c EvaluationConfigModel) *AiEvaluationModel {
	return &AiEvaluationModel{Config: &c}
}

// assertJSON checks that v marshals to the same JSON value as want.
func assertJSON(t *testing.T, v any, want string) {
	t.Helper()
	got, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("bad want JSON %s: %v", want, err)
	}
	if !reflect.DeepEqual(g, w) {
		t.Errorf("JSON:\n got  %s\n want %s", got, want)
	}
}

func assertNoDiags(t *testing.T, diags diag.Diagnostics) {
	t.Helper()
	if diags.HasError() {
		t.Fatalf("diagnostics: %v", diags)
	}
}

// assertError checks that diags has exactly one error, at path p.
func assertError(t *testing.T, diags diag.Diagnostics, p path.Path, detail string) {
	t.Helper()
	errs := diags.Errors()
	if len(errs) != 1 {
		t.Fatalf("got %d errors, want 1: %v", len(errs), diags)
	}
	d, ok := errs[0].(diag.DiagnosticWithPath)
	if !ok || !d.Path().Equal(p) {
		t.Errorf("error %q is not at path %s", errs[0].Detail(), p)
	}
	if !strings.Contains(errs[0].Detail(), detail) {
		t.Errorf("error %q does not contain %q", errs[0].Detail(), detail)
	}
}

func TestExpandCreate(t *testing.T) {
	cases := []struct {
		name string
		m    *AiEvaluationModel
		want string
	}{
		{
			name: "null values are not sent",
			m:    &AiEvaluationModel{},
			want: `{}`,
		},
		{
			name: "zero values are sent",
			m: &AiEvaluationModel{
				Application: types.StringValue("app"),
				Subsystem:   types.StringValue(""),
				IsEnabled:   types.BoolValue(false),
				Threshold:   types.Float64Value(0),
				Target:      types.StringValue("RESPONSE"),
				Config: &EvaluationConfigModel{
					PromptInjection: &PromptInjectionConfigModel{AdditionalContext: types.StringValue("")},
				},
			},
			want: `{"application":"app","subsystem":"","isEnabled":false,"threshold":0,"target":"RESPONSE",
				"config":{"promptInjection":{"additionalContext":""}}}`,
		},
		{
			name: "computed fields are not sent",
			m: &AiEvaluationModel{
				Id: types.StringValue("id"), CompanyId: types.StringValue("c"), CreatedBy: types.StringValue("u"),
				CreatedAt: types.StringValue("2026-09-25T00:00:00Z"), UpdatedAt: types.StringValue("2026-09-25T00:00:00Z"),
			},
			want: `{}`,
		},
		{
			name: "oneOf arm with a null field",
			m:    withConfig(EvaluationConfigModel{PromptInjection: &PromptInjectionConfigModel{}}),
			want: `{"config":{"promptInjection":{}}}`,
		},
		{
			name: "empty oneOf arm",
			m:    withConfig(EvaluationConfigModel{Sexism: &SexismConfigModel{}}),
			want: `{"config":{"sexism":{}}}`,
		},
		{
			name: "no oneOf arm",
			m:    withConfig(EvaluationConfigModel{}),
			want: `{"config":{}}`,
		},
		{
			name: "set of strings",
			m:    withConfig(EvaluationConfigModel{AllowedTopics: &AllowedTopicsConfigModel{Topics: strSet("b", "a")}}),
			want: `{"config":{"allowedTopics":{"topics":["b","a"]}}}`,
		},
		{
			name: "empty set",
			m:    withConfig(EvaluationConfigModel{SqlAllowedTables: &SqlAllowedTablesConfigModel{Tables: strSet()}}),
			want: `{"config":{"sqlAllowedTables":{"tables":[]}}}`,
		},
		{
			name: "null set",
			m:    withConfig(EvaluationConfigModel{Competition: &CompetitionConfigModel{Competitors: types.SetNull(types.StringType)}}),
			want: `{"config":{"competition":{}}}`,
		},
		{
			name: "set of enum values",
			m:    withConfig(EvaluationConfigModel{Pii: &PiiConfigModel{Categories: strSet("EMAIL_ADDRESS", "US_SSN")}}),
			want: `{"config":{"pii":{"categories":["EMAIL_ADDRESS","US_SSN"]}}}`,
		},
		{
			name: "uint64 as a string, and zero",
			m: withConfig(EvaluationConfigModel{SqlLoad: &SqlLoadConfigModel{
				JoinLimit:         types.Int64Value(9223372036854775807),
				CteLimit:          types.Int64Value(0),
				AllowRecursiveCte: types.BoolValue(false),
			}}),
			want: `{"config":{"sqlLoad":{"joinLimit":"9223372036854775807","cteLimit":"0","allowRecursiveCte":false}}}`,
		},
		{
			name: "list keeps the order",
			m: withConfig(EvaluationConfigModel{CustomEvaluation: &CustomEvaluationConfigModel{
				Instructions:              types.StringValue("do"),
				ShouldIncludeSystemPrompt: types.BoolValue(true),
				Examples:                  examples(example{"b", 2}, example{"a", 0}),
			}}),
			want: `{"config":{"customEvaluation":{"instructions":"do","shouldIncludeSystemPrompt":true,
				"examples":[{"conversation":"b","score":"2"},{"conversation":"a","score":"0"}]}}}`,
		},
		{
			name: "empty list",
			m:    withConfig(EvaluationConfigModel{CustomEvaluation: &CustomEvaluationConfigModel{Examples: examples()}}),
			want: `{"config":{"customEvaluation":{"examples":[]}}}`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, diags := expandCreate(context.Background(), c.m)
			assertNoDiags(t, diags)
			assertJSON(t, got, c.want)
		})
	}
}

// TestExpandUpdate checks that the Update body never has application,
// subsystem, or target (D9), and has no update mask.
func TestExpandUpdate(t *testing.T) {
	m := &AiEvaluationModel{
		Application: types.StringValue("app"),
		Subsystem:   types.StringValue("sub"),
		Target:      types.StringValue("PROMPT"),
		IsEnabled:   types.BoolValue(true),
		Threshold:   types.Float64Value(0.5),
		Config:      &EvaluationConfigModel{Toxicity: &ToxicityConfigModel{}},
	}
	got, diags := expandUpdate(context.Background(), m)
	assertNoDiags(t, diags)
	assertJSON(t, got, `{"isEnabled":true,"threshold":0.5,"config":{"toxicity":{}}}`)
}

func TestExpandErrors(t *testing.T) {
	cases := []struct {
		name string
		m    *AiEvaluationModel
		path path.Path
	}{
		{
			name: "negative uint64",
			m:    withConfig(EvaluationConfigModel{SqlLoad: &SqlLoadConfigModel{JoinLimit: types.Int64Value(-1)}}),
			path: path.Root("config").AtName("sql_load").AtName("join_limit"),
		},
		{
			name: "negative uint64 in a list",
			m: withConfig(EvaluationConfigModel{CustomEvaluation: &CustomEvaluationConfigModel{
				Examples: examples(example{"a", 1}, example{"b", -2}),
			}}),
			path: path.Root("config").AtName("custom_evaluation").AtName("examples").AtListIndex(1).AtName("score"),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, diags := expandCreate(context.Background(), c.m)
			assertError(t, diags, c.path, "negative")
		})
	}
}

func unmarshalEvaluation(t *testing.T, s string) *ai_evaluations_service.AiEvaluation {
	t.Helper()
	var v ai_evaluations_service.AiEvaluation
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	return &v
}

// TestRoundTrip reads an API response with the SDK, flattens it, and expands
// it into a Create body. The body must be the response without the computed
// fields.
func TestRoundTrip(t *testing.T) {
	computed := `"id":"e1","companyId":"c1","createdBy":"u1","createdAt":"2026-09-25T10:00:00Z","updatedAt":"2026-09-25T10:00:00.5Z"`
	configs := []string{
		`{"allowedTopics":{"topics":["a","b"]}}`,
		`{"restrictedTopics":{"topics":[]}}`,
		`{"competition":{}}`,
		`{"pii":{"categories":["PHONE_NUMBER","IBAN_CODE"]}}`,
		`{"promptInjection":{"additionalContext":""}}`,
		`{"sqlLoad":{"joinLimit":"3","cteLimit":"0","allowRecursiveCte":false}}`,
		`{"customEvaluation":{"instructions":"i","policyType":"p","safe":"s","violates":"v","shouldIncludeSystemPrompt":false,
			"examples":[{"conversation":"x","score":"1"},{"conversation":"y"}]}}`,
		`{"hallucinationCompleteness":{}}`,
		`{"sqlReadOnly":{}}`,
		`{}`,
	}
	for _, config := range configs {
		t.Run(config, func(t *testing.T) {
			body := `{"application":"app","subsystem":"sub","target":"CONVERSATION","isEnabled":false,"threshold":0,"config":` + config + `}`
			resp := unmarshalEvaluation(t, `{`+computed+`,`+body[1:])
			m, diags := flatten(context.Background(), resp)
			assertNoDiags(t, diags)
			got, diags := expandCreate(context.Background(), m)
			assertNoDiags(t, diags)
			assertJSON(t, got, body)
		})
	}
}

func TestFlatten(t *testing.T) {
	ctx := context.Background()
	m, diags := flatten(ctx, unmarshalEvaluation(t, `{
		"id":"e1","isEnabled":false,"createdAt":"2026-09-25T10:00:00.5Z",
		"config":{"sqlLoad":{"joinLimit":"18"}}}`))
	assertNoDiags(t, diags)
	checks := []struct {
		name      string
		got, want attr.Value
	}{
		{"present id", m.Id, types.StringValue("e1")},
		{"false is a value", m.IsEnabled, types.BoolValue(false)},
		{"missing string is null", m.Application, types.StringNull()},
		{"missing number is null", m.Threshold, types.Float64Null()},
		{"missing enum is null", m.Target, types.StringNull()},
		{"time", m.CreatedAt, types.StringValue("2026-09-25T10:00:00.5Z")},
		{"missing time is null", m.UpdatedAt, types.StringNull()},
		{"uint64", m.Config.SqlLoad.JoinLimit, types.Int64Value(18)},
		{"missing uint64 is null", m.Config.SqlLoad.CteLimit, types.Int64Null()},
	}
	for _, c := range checks {
		if !c.got.Equal(c.want) {
			t.Errorf("%s: got %s, want %s", c.name, c.got, c.want)
		}
	}
	if m.Config.Toxicity != nil || m.Config.Pii != nil {
		t.Error("missing arms: got set, want nil")
	}

	m, diags = flatten(ctx, unmarshalEvaluation(t, `{"config":{"sexism":{}}}`))
	assertNoDiags(t, diags)
	if m.Config.Sexism == nil {
		t.Error("empty arm sexism: got nil, want set")
	}

	m, diags = flatten(ctx, unmarshalEvaluation(t, `{"config":{"allowedTopics":{"topics":[]}}}`))
	assertNoDiags(t, diags)
	if got := m.Config.AllowedTopics.Topics; !got.Equal(strSet()) {
		t.Errorf("empty set: got %s, want []", got)
	}

	m, diags = flatten(ctx, unmarshalEvaluation(t, `{"config":{"allowedTopics":{}}}`))
	assertNoDiags(t, diags)
	if got := m.Config.AllowedTopics.Topics; !got.IsNull() {
		t.Errorf("missing set: got %s, want null", got)
	}

	m, diags = flatten(ctx, unmarshalEvaluation(t, `{"config":{"customEvaluation":{}}}`))
	assertNoDiags(t, diags)
	if got := m.Config.CustomEvaluation.Examples; !got.IsNull() || !got.ElementType(ctx).Equal(types.ObjectType{AttrTypes: customEvaluationExampleAttrTypes()}) {
		t.Errorf("missing list: got %s, want a null list of examples", got)
	}
}

func TestFlattenErrors(t *testing.T) {
	scorePath := path.Root("config").AtName("custom_evaluation").AtName("examples").AtListIndex(0).AtName("score")
	cases := []struct {
		name   string
		resp   string
		path   path.Path
		detail string
	}{
		{"uint64 above the Int64 maximum", `{"config":{"sqlLoad":{"cteLimit":"9223372036854775808"}}}`,
			path.Root("config").AtName("sql_load").AtName("cte_limit"), "larger than the Terraform maximum"},
		{"uint64 not a number", `{"config":{"customEvaluation":{"examples":[{"score":"x"}]}}}`,
			scorePath, "not an unsigned number"},
		{"uint64 negative", `{"config":{"customEvaluation":{"examples":[{"score":"-1"}]}}}`,
			scorePath, "not an unsigned number"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, diags := flatten(context.Background(), unmarshalEvaluation(t, c.resp))
			assertError(t, diags, c.path, c.detail)
		})
	}

	if _, diags := flatten(context.Background(), nil); !diags.HasError() {
		t.Error("nil response: got no error")
	}
}
