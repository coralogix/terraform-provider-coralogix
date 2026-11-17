// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package slo_mgmt

import (
	"context"
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/slov2"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestHasRangeSelector(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  bool
	}{
		{"range", `sum(rate(m[1m]))`, true},
		{"range with labels", `sum(increase(m{status="success"}[1m])) by (svc)`, true},
		{"other spelling", `sum(rate(m[60s]))`, true},
		{"wrong range is left to the backend", `sum(rate(m[5m]))`, true},
		{"subquery", `max_over_time(rate(m[1m])[5m:1m])`, true},
		{"subquery without inner range", `max_over_time(m[5m:1m])`, true},
		{"space before bracket", `sum(rate(m [1m]))`, true},
		{"no range", `sum(increase(m{status="success"}))`, false},
		{"empty selector", `sum(increase(m{}))`, false},
		{"empty", ``, false},
		{"bracket only in a label regex", `sum(increase(m{x=~"[a-z]+"}))`, false},
		{"bracket only in a single-quoted label", `sum(increase(m{x=~'[a-z]+'}))`, false},
		{"bracket only in a backtick label", "sum(increase(m{x=~`[a-z]+`}))", false},
		{"escaped quote keeps the string open", `sum(increase(m{x="a\"[1m]"}))`, false},
		{"backslash does not escape in a backtick string", "sum(increase(m{x=~`a\\`}[1m]))", true},
		{"string closes, then a range", `sum(rate(m{x=~"[a-z]+"}[1m]))`, true},
		{"bracket in a comment", "sum(increase(m)) # [1m]\n", false},
		{"range after a comment line", "# note\nsum(rate(m[1m]))", true},
		{"unterminated string", `sum(increase(m{x="[1m]))`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasRangeSelector(tc.query); got != tc.want {
				t.Errorf("hasRangeSelector(%q) = %v, want %v", tc.query, got, tc.want)
			}
		})
	}
}

// nullObject returns a value of the type with every attribute null, with the given attributes set.
// A nested map sets attributes of a nested object.
func nullObject(t *testing.T, typ tftypes.Type, set map[string]any) tftypes.Value {
	t.Helper()
	object, ok := typ.(tftypes.Object)
	if !ok {
		t.Fatalf("%v is not an object type", typ)
	}
	values := make(map[string]tftypes.Value, len(object.AttributeTypes))
	for name, attrType := range object.AttributeTypes {
		switch v := set[name].(type) {
		case nil:
			values[name] = tftypes.NewValue(attrType, nil)
		case map[string]any:
			values[name] = nullObject(t, attrType, v)
		case tftypes.Value:
			values[name] = v
		default:
			t.Fatalf("unsupported value %T for %s", v, name)
		}
	}
	return tftypes.NewValue(object, values)
}

func sloConfig(t *testing.T, sli map[string]any) tfsdk.Config {
	t.Helper()
	schema := slov2.Schema()
	typ := schema.Type().TerraformType(context.Background())
	raw := nullObject(t, typ, map[string]any{
		"name":                        tftypes.NewValue(tftypes.String, "slo"),
		"target_threshold_percentage": tftypes.NewValue(tftypes.Number, 99),
		"sli":                         sli,
	})
	return tfsdk.Config{Schema: schema, Raw: raw}
}

func requestBased(good, total tftypes.Value) map[string]any {
	return map[string]any{"request_based_metric_sli": map[string]any{
		"good_events":  map[string]any{"query": good},
		"total_events": map[string]any{"query": total},
	}}
}

func str(s string) tftypes.Value { return tftypes.NewValue(tftypes.String, s) }

func TestValidateRequestBasedQueryRanges(t *testing.T) {
	unknown := tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
	null := tftypes.NewValue(tftypes.String, nil)
	withRange := str(`sum(rate(m[1m]))`)
	cases := []struct {
		name string
		sli  map[string]any
		// errors lists the attribute paths that must carry an error.
		errors []string
	}{
		{"both have a range", requestBased(withRange, withRange), nil},
		{"good has none", requestBased(str(`sum(increase(m{status="success"}))`), withRange),
			[]string{`sli.request_based_metric_sli.good_events.query`}},
		{"total has none", requestBased(withRange, str(`sum(increase(m{}))`)),
			[]string{`sli.request_based_metric_sli.total_events.query`}},
		{"both have none", requestBased(str(`sum(increase(m))`), str(`sum(increase(n))`)),
			[]string{`sli.request_based_metric_sli.good_events.query`, `sli.request_based_metric_sli.total_events.query`}},
		{"a range only in a label value counts as none", requestBased(str(`sum(increase(m{x=~"[a-z]+"}))`), withRange),
			[]string{`sli.request_based_metric_sli.good_events.query`}},
		{"a subquery counts as a range", requestBased(str(`max_over_time(rate(m[1m])[5m:1m])`), withRange), nil},
		{"an unknown query is skipped", requestBased(unknown, unknown), nil},
		{"a null query is skipped", requestBased(null, null), nil},
		{"an unknown and a bad one", requestBased(unknown, str(`sum(increase(m))`)),
			[]string{`sli.request_based_metric_sli.total_events.query`}},
		{"a window-based query is not checked", map[string]any{"window_based_metric_sli": map[string]any{
			"query": map[string]any{"query": str(`avg(avg_over_time(m))`)},
		}}, nil},
		{"no request-based arm", map[string]any{"apm_sli": map[string]any{}}, nil},
		{"no sli", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diags := validateRequestBasedQueryRanges(context.Background(), sloConfig(t, tc.sli))
			got := map[string]bool{}
			for _, d := range diags {
				if d.Severity().String() != "Error" {
					continue
				}
				withPath, ok := d.(diag.DiagnosticWithPath)
				if !ok {
					t.Fatalf("diagnostic %v has no attribute path", d)
				}
				got[withPath.Path().String()] = true
				if !strings.Contains(d.Detail(), "[1m]") {
					t.Errorf("the message does not name the [1m] range: %s", d.Detail())
				}
			}
			if len(got) != len(tc.errors) {
				t.Fatalf("errors at %v, want %v", got, tc.errors)
			}
			for _, p := range tc.errors {
				if !got[p] {
					t.Errorf("no error at %s, got %v", p, got)
				}
			}
		})
	}
}

// The resource that the provider registers runs the check while Terraform validates the configuration.
func TestSLOV2ResourceValidatesQueryRanges(t *testing.T) {
	ctx := context.Background()
	r, ok := NewSLOV2Resource().(resource.ResourceWithValidateConfig)
	if !ok {
		t.Fatal("the resource does not validate its configuration")
	}
	validate := func(good string) *resource.ValidateConfigResponse {
		resp := &resource.ValidateConfigResponse{}
		r.ValidateConfig(ctx, resource.ValidateConfigRequest{Config: sloConfig(t, requestBased(str(good), str(`sum(rate(m[1m]))`)))}, resp)
		return resp
	}
	if resp := validate(`sum(rate(m[1m]))`); resp.Diagnostics.HasError() {
		t.Errorf("a query with a range is rejected: %v", resp.Diagnostics)
	}
	resp := validate(`sum(increase(m))`)
	if !resp.Diagnostics.HasError() {
		t.Fatal("a query without a range is accepted")
	}
	if got := resp.Diagnostics[0].Summary(); got != "Query has no range selector" {
		t.Errorf("summary = %q", got)
	}
}
