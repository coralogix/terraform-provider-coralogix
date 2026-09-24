// Copyright 2026 Coralogix Ltd.
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

package enrichment_rules

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"testing"

	ess "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/enrichments_service"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// enrichmentSchema pulls the real resource schema, the same way the existing
// plan-modifier test does, so Config/Plan/State fixtures are built from the
// production schema rather than a hand-rolled copy.
func enrichmentSchema(t *testing.T) schema.Schema {
	t.Helper()
	var schemaResp resource.SchemaResponse
	(&DataEnrichmentsResource{}).Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("schema diagnostics: %v", schemaResp.Diagnostics)
	}
	return schemaResp.Schema
}

// modelToRaw converts a DataEnrichmentsModel into the tftypes-backed Raw value
// the framework carries inside a Config/Plan/State, by round-tripping it
// through an empty tfsdk.State whose Set encodes the model against the schema.
// This reuses the dashboard idiom's end goal (a schema-typed Raw value) without
// hand-encoding every nested attribute.
func modelToRaw(t *testing.T, s schema.Schema, model *DataEnrichmentsModel) tfsdk.State {
	t.Helper()
	state := tfsdk.State{Schema: s}
	diags := state.Set(context.Background(), model)
	if diags.HasError() {
		t.Fatalf("encode model into state: %v", diags)
	}
	return state
}

func configFrom(t *testing.T, s schema.Schema, model *DataEnrichmentsModel) tfsdk.Config {
	t.Helper()
	st := modelToRaw(t, s, model)
	return tfsdk.Config{Schema: s, Raw: st.Raw}
}

func planFrom(t *testing.T, s schema.Schema, model *DataEnrichmentsModel) tfsdk.Plan {
	t.Helper()
	st := modelToRaw(t, s, model)
	return tfsdk.Plan{Schema: s, Raw: st.Raw}
}

func stateFrom(t *testing.T, s schema.Schema, model *DataEnrichmentsModel) tfsdk.State {
	t.Helper()
	return modelToRaw(t, s, model)
}

func geoIpField(id int64, name string) GeoIpEnrichmentFieldModel {
	return GeoIpEnrichmentFieldModel{
		ID:                types.Int64Value(id),
		Name:              types.StringValue(name),
		Asn:               types.BoolNull(),
		EnrichedFieldName: types.StringNull(),
		SelectedColumns:   types.SetNull(types.StringType),
	}
}

// errorDiagnostics returns only the Error-severity diagnostics.
func errorDiagnostics(d diag.Diagnostics) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, one := range d {
		if one.Severity() == diag.SeverityError {
			out = append(out, one)
		}
	}
	return out
}

// TestExtractHelpers_NilCustomEnrichmentData exercises the nil-safety of the
// three extract helpers when the custom block is present but its required
// custom_enrichment_data child is nil — the shape ValidateConfig rejects but
// which the helpers must survive without dereferencing a nil model.
func TestExtractHelpers_NilCustomEnrichmentData(t *testing.T) {
	customNoData := &CustomEnrichmentFieldsModel{
		CustomEnrichmentDataModel: nil,
		Fields: []EnrichmentFieldModel{
			{
				ID:                types.Int64Value(0),
				Name:              types.StringValue("ignored"),
				EnrichedFieldName: types.StringNull(),
				SelectedColumns:   types.SetNull(types.StringType),
			},
		},
	}

	t.Run("custom_only_nil_data", func(t *testing.T) {
		plan := &DataEnrichmentsModel{Custom: customNoData}

		if got := extractCustomEnrichmentsDataCreate(plan); got != nil {
			t.Fatalf("extractCustomEnrichmentsDataCreate = %v, want nil", got)
		}
		if got := extractCustomEnrichmentsDataUpdate(plan); got != nil {
			t.Fatalf("extractCustomEnrichmentsDataUpdate = %v, want nil", got)
		}
		got := extractDataEnrichments(plan)
		if len(got) != 0 {
			t.Fatalf("extractDataEnrichments = %v, want empty when only custom-without-data is set", got)
		}
	})

	t.Run("geoip_and_suspicious_with_nil_custom_data", func(t *testing.T) {
		plan := &DataEnrichmentsModel{
			GeoIp: &GeoIpEnrichmentFieldsModel{
				Fields: []GeoIpEnrichmentFieldModel{geoIpField(11, "src_ip")},
			},
			SuspiciousIp: &EnrichmentFieldsModel{
				Fields: []EnrichmentFieldModel{
					{
						ID:                types.Int64Value(22),
						Name:              types.StringValue("dst_ip"),
						EnrichedFieldName: types.StringNull(),
						SelectedColumns:   types.SetNull(types.StringType),
					},
				},
			},
			Custom: customNoData,
		}

		// Custom helpers still skip the malformed custom block.
		if got := extractCustomEnrichmentsDataCreate(plan); got != nil {
			t.Fatalf("extractCustomEnrichmentsDataCreate = %v, want nil", got)
		}
		if got := extractCustomEnrichmentsDataUpdate(plan); got != nil {
			t.Fatalf("extractCustomEnrichmentsDataUpdate = %v, want nil", got)
		}

		// extractDataEnrichments still emits geo_ip + suspicious_ip requests
		// while skipping the custom fields (guarded by CustomEnrichmentDataModel).
		got := extractDataEnrichments(plan)
		names := fieldNameSet(got)
		if len(got) != 2 {
			t.Fatalf("extractDataEnrichments emitted %d requests (%v), want 2 (geo_ip + suspicious_ip)", len(got), names)
		}
		for _, want := range []string{"src_ip", "dst_ip"} {
			if !names[want] {
				t.Fatalf("extractDataEnrichments missing %q; got %v", want, names)
			}
		}
	})
}

// TestValidateConfig_CustomRequiresEnrichmentData covers the AlsoRequires-style
// dependency ValidateConfig enforces: a custom block without
// custom_enrichment_data is an error; with it, no error.
func TestValidateConfig_CustomRequiresEnrichmentData(t *testing.T) {
	ctx := context.Background()
	s := enrichmentSchema(t)
	r := &DataEnrichmentsResource{}

	t.Run("custom_without_data", func(t *testing.T) {
		model := &DataEnrichmentsModel{
			ID: types.StringNull(),
			Custom: &CustomEnrichmentFieldsModel{
				CustomEnrichmentDataModel: nil,
				Fields:                    []EnrichmentFieldModel{},
			},
		}
		resp := &resource.ValidateConfigResponse{}
		r.ValidateConfig(ctx, resource.ValidateConfigRequest{Config: configFrom(t, s, model)}, resp)

		errs := errorDiagnostics(resp.Diagnostics)
		if len(errs) < 1 {
			t.Fatalf("expected >=1 error diagnostic for custom-without-data, got: %v", resp.Diagnostics)
		}
		withPath, ok := errs[0].(diag.DiagnosticWithPath)
		if !ok {
			t.Fatalf("error diagnostic has no path: %#v", errs[0])
		}
		if got := withPath.Path().String(); got != CUSTOM_TYPE {
			t.Fatalf("diagnostic path = %q, want %q", got, CUSTOM_TYPE)
		}
	})

	t.Run("custom_with_data", func(t *testing.T) {
		model := &DataEnrichmentsModel{
			ID: types.StringNull(),
			Custom: &CustomEnrichmentFieldsModel{
				CustomEnrichmentDataModel: &CustomEnrichmentDataModel{
					ID:          types.Int64Value(5),
					Name:        types.StringValue("lookup"),
					Description: types.StringValue("desc"),
					Version:     types.Int64Value(1),
					Contents:    types.StringValue("a,b\n1,2\n"),
				},
				Fields: []EnrichmentFieldModel{},
			},
		}
		resp := &resource.ValidateConfigResponse{}
		r.ValidateConfig(ctx, resource.ValidateConfigRequest{Config: configFrom(t, s, model)}, resp)

		if errs := errorDiagnostics(resp.Diagnostics); len(errs) != 0 {
			t.Fatalf("expected 0 error diagnostics for custom-with-data, got: %v", errs)
		}
	})
}

// fieldNameSet returns the set of FieldNames present in the request models,
// the identity used to compare enrichment sets without depending on ordering
// or on server-assigned ids (which the request models do not carry).
func fieldNameSet(models []ess.EnrichmentRequestModel) map[string]bool {
	out := make(map[string]bool, len(models))
	for _, m := range models {
		out[m.FieldName] = true
	}
	return out
}

func sortedNames(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// TestUpdate_RollbackOnAddFailure drives the Update remove -> add -> rollback
// flow through the addEnrichmentsFn/removeEnrichmentsFn test seams. The fixture
// uses geo_ip fields only (no custom block), so extractCustomEnrichmentsDataUpdate
// returns nil and the nil custom_enrichments_client is never dereferenced. The
// plan differs from state (different enriched_field_name) so fieldsChanged is
// true, and state carries field ids > 0 so the remove path runs.
func TestUpdate_RollbackOnAddFailure(t *testing.T) {
	ctx := context.Background()
	s := enrichmentSchema(t)

	// State: two geo_ip fields with ids > 0.
	stateModel := &DataEnrichmentsModel{
		ID: types.StringValue(GEOIP_TYPE),
		GeoIp: &GeoIpEnrichmentFieldsModel{
			Fields: []GeoIpEnrichmentFieldModel{
				geoIpField(101, "src_ip"),
				geoIpField(102, "dst_ip"),
			},
		},
	}
	// Plan: same fields but a changed enriched_field_name so fieldsChanged=true.
	planField := geoIpField(0, "src_ip")
	planField.EnrichedFieldName = types.StringValue("src_ip_geo")
	planModel := &DataEnrichmentsModel{
		ID: types.StringValue(GEOIP_TYPE),
		GeoIp: &GeoIpEnrichmentFieldsModel{
			Fields: []GeoIpEnrichmentFieldModel{
				planField,
				geoIpField(0, "dst_ip"),
			},
		},
	}

	// The enrichment set the rollback must re-add is exactly the state's set.
	wantRollbackNames := fieldNameSet(extractDataEnrichments(stateModel))

	newUpdateReqResp := func() (resource.UpdateRequest, *resource.UpdateResponse) {
		return resource.UpdateRequest{
				Plan:  planFrom(t, s, planModel),
				State: stateFrom(t, s, stateModel),
			}, &resource.UpdateResponse{
				State: stateFrom(t, s, stateModel),
			}
	}

	const originalErrText = "boom: add enrichments failed"

	t.Run("remove_ok_add_fails_then_rollback", func(t *testing.T) {
		var removeCalls, addCalls int
		var rollbackReq *ess.EnrichmentsCreationRequest
		r := &DataEnrichmentsResource{
			removeEnrichmentsFn: func(_ context.Context, _ []int64) (*ess.RemoveEnrichmentsResponse, *http.Response, error) {
				removeCalls++
				return &ess.RemoveEnrichmentsResponse{}, &http.Response{StatusCode: http.StatusOK}, nil
			},
			addEnrichmentsFn: func(_ context.Context, rq *ess.EnrichmentsCreationRequest) (*ess.AddEnrichmentsResponse, *http.Response, error) {
				addCalls++
				if addCalls == 1 {
					// first (real) add fails
					return nil, &http.Response{StatusCode: http.StatusInternalServerError}, errors.New(originalErrText)
				}
				// second call is the rollback re-add; succeed.
				rollbackReq = rq
				return &ess.AddEnrichmentsResponse{}, &http.Response{StatusCode: http.StatusOK}, nil
			},
		}

		req, resp := newUpdateReqResp()
		r.Update(ctx, req, resp)

		if removeCalls != 1 {
			t.Fatalf("removeEnrichmentsFn called %d times, want 1", removeCalls)
		}
		if addCalls != 2 {
			t.Fatalf("addEnrichmentsFn called %d times, want 2 (add + rollback)", addCalls)
		}
		if rollbackReq == nil {
			t.Fatal("rollback add did not receive a request")
		}
		gotNames := fieldNameSet(rollbackReq.RequestEnrichments)
		if len(gotNames) != len(wantRollbackNames) {
			t.Fatalf("rollback field set = %v, want %v", sortedNames(gotNames), sortedNames(wantRollbackNames))
		}
		for name := range wantRollbackNames {
			if !gotNames[name] {
				t.Fatalf("rollback field set missing %q: got %v", name, sortedNames(gotNames))
			}
		}
		errs := errorDiagnostics(resp.Diagnostics)
		if len(errs) == 0 {
			t.Fatal("expected an error diagnostic carrying the original add error")
		}
		if !containsErr(errs, originalErrText) {
			t.Fatalf("diagnostics do not carry original error %q: %v", originalErrText, errs)
		}
	})

	t.Run("add_fails_on_both_calls_rollback_swallowed", func(t *testing.T) {
		var addCalls int
		r := &DataEnrichmentsResource{
			removeEnrichmentsFn: func(_ context.Context, _ []int64) (*ess.RemoveEnrichmentsResponse, *http.Response, error) {
				return &ess.RemoveEnrichmentsResponse{}, &http.Response{StatusCode: http.StatusOK}, nil
			},
			addEnrichmentsFn: func(_ context.Context, _ *ess.EnrichmentsCreationRequest) (*ess.AddEnrichmentsResponse, *http.Response, error) {
				addCalls++
				return nil, &http.Response{StatusCode: http.StatusInternalServerError}, errors.New(originalErrText)
			},
		}

		req, resp := newUpdateReqResp()
		// Must not panic even though the rollback add also fails.
		r.Update(ctx, req, resp)

		if addCalls != 2 {
			t.Fatalf("addEnrichmentsFn called %d times, want 2 (add + failed rollback)", addCalls)
		}
		errs := errorDiagnostics(resp.Diagnostics)
		if len(errs) == 0 {
			t.Fatal("expected the original error diagnostic to be returned")
		}
		if !containsErr(errs, originalErrText) {
			t.Fatalf("diagnostics do not carry original error %q: %v", originalErrText, errs)
		}
	})
}

func containsErr(errs []diag.Diagnostic, substr string) bool {
	for _, e := range errs {
		if strings.Contains(e.Detail(), substr) || strings.Contains(e.Summary(), substr) {
			return true
		}
	}
	return false
}
