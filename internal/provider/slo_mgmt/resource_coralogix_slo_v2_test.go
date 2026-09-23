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
	"testing"

	slos "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/slos_service"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func ptr[T any](v T) *T { return &v }

func TestSLOV2APMServicesRequiresExactlyOne(t *testing.T) {
	ctx := context.Background()
	var schemaResponse frameworkresource.SchemaResponse
	(&SLOV2Resource{}).Schema(ctx, frameworkresource.SchemaRequest{}, &schemaResponse)

	sli := schemaResponse.Schema.Attributes["sli"].(resourceschema.SingleNestedAttribute)
	apm := sli.Attributes["apm_sli"].(resourceschema.SingleNestedAttribute)
	services := apm.Attributes["services"].(resourceschema.ListAttribute)

	for _, test := range []struct {
		name      string
		values    []attr.Value
		wantError bool
	}{
		{name: "empty", values: nil, wantError: true},
		{name: "one", values: []attr.Value{types.StringValue("checkout")}, wantError: false},
		{name: "two", values: []attr.Value{types.StringValue("checkout"), types.StringValue("payments")}, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := validator.ListRequest{ConfigValue: types.ListValueMust(types.StringType, test.values)}
			var response validator.ListResponse
			for _, listValidator := range services.Validators {
				listValidator.ValidateList(ctx, request, &response)
			}

			if got := response.Diagnostics.HasError(); got != test.wantError {
				t.Fatalf("services validator has error = %v, want %v: %v", got, test.wantError, response.Diagnostics)
			}
		})
	}
}

func baseSLO() *slos.Slo {
	return &slos.Slo{
		Id:                        ptr("11111111-2222-3333-4444-555555555555"),
		Name:                      ptr("slo"),
		TargetThresholdPercentage: ptr(float32(99)),
		SloTimeFrame:              slos.SLOTIMEFRAME_SLO_TIME_FRAME_7_DAYS.Ptr(),
	}
}

func windowSLI() *slos.WindowBasedMetricSli {
	return &slos.WindowBasedMetricSli{
		Query:              &slos.Metric{Query: ptr("avg(avg_over_time(request_duration_seconds[1m]))")},
		Window:             slos.WINDOWSLOWINDOW_WINDOW_SLO_WINDOW_1_MINUTE.Ptr(),
		ComparisonOperator: slos.COMPARISONOPERATOR_COMPARISON_OPERATOR_LESS_THAN.Ptr(),
		Threshold:          ptr(float32(0.232)),
	}
}

func sliObject(t *testing.T, model *SLOV2ResourceModel) SLIModel {
	t.Helper()
	var sli SLIModel
	if diags := model.SLI.As(context.Background(), &sli, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("converting sli: %v", diags)
	}
	return sli
}

// A response without missingDataStrategy - the enum has no unspecified member -
// must flatten to the value the backend actually stored, not to "".
func TestFlattenWindowBasedSLIDefaultsMissingDataStrategy(t *testing.T) {
	slo := baseSLO()
	slo.WindowBasedMetricSli = windowSLI()

	model, diags := flattenSLOV2(context.Background(), slo)
	if diags.HasError() {
		t.Fatalf("flatten: %v", diags)
	}

	var window WindowBasedMetricSliModel
	if diags := sliObject(t, model).WindowBasedMetricSli.As(context.Background(), &window, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("converting window sli: %v", diags)
	}

	if got := window.MissingDataStrategy.ValueString(); got != "uncounted" {
		t.Errorf("missing_data_strategy = %q, want %q", got, "uncounted")
	}
	if got := model.ProductType.ValueString(); got != "unspecified" {
		t.Errorf("product_type = %q, want %q", got, "unspecified")
	}
	if !model.OwnershipTags.IsNull() {
		t.Errorf("ownership_tags = %v, want null", model.OwnershipTags)
	}
	if !model.ApmSliMetadata.IsNull() {
		t.Errorf("apm_sli_metadata = %v, want null", model.ApmSliMetadata)
	}
}

// An APM read carries empty filters and groupingKeys arrays that no
// configuration can express; they have to flatten back to null.
func TestFlattenApmSLINormalizesInjectedEmpties(t *testing.T) {
	slo := baseSLO()
	slo.ProductType = slos.SLOPRODUCTTYPE_SLO_PRODUCT_TYPE_APM.Ptr()
	slo.Grouping = &slos.V1Grouping{Labels: []string{"service_name"}}
	slo.ApmSli = &slos.ApmSli{
		Services:     []string{"checkout"},
		ErrorConfig:  map[string]interface{}{},
		Filters:      []slos.ApmFilter{},
		GroupingKeys: []string{},
	}
	slo.ApmSliMetadata = &slos.ApmSli{
		Services:    []string{"checkout"},
		ErrorConfig: map[string]interface{}{},
	}

	model, diags := flattenSLOV2(context.Background(), slo)
	if diags.HasError() {
		t.Fatalf("flatten: %v", diags)
	}

	var apm ApmSliModel
	if diags := sliObject(t, model).ApmSli.As(context.Background(), &apm, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("converting apm sli: %v", diags)
	}

	if !apm.Filters.IsNull() {
		t.Errorf("filters = %v, want null", apm.Filters)
	}
	if !apm.GroupingKeys.IsNull() {
		t.Errorf("grouping_keys = %v, want null", apm.GroupingKeys)
	}
	if apm.ErrorConfig.IsNull() {
		t.Error("error_config = null, want the empty object")
	}
	if !apm.LatencyConfig.IsNull() {
		t.Errorf("latency_config = %v, want null", apm.LatencyConfig)
	}
	if got := model.ProductType.ValueString(); got != "apm" {
		t.Errorf("product_type = %q, want %q", got, "apm")
	}
	if model.ApmSliMetadata.IsNull() {
		t.Error("apm_sli_metadata = null, want the backend's mirror")
	}
}

// Filters written before `values` existed carry only the deprecated singular
// `value`; an import has to turn that into a one-element values collection, and
// the empty `value` the backend injects next to a populated `values` has to be
// ignored.
func TestFlattenApmSLINormalizesDeprecatedFilterValue(t *testing.T) {
	slo := baseSLO()
	slo.ApmSli = &slos.ApmSli{
		Services: []string{"checkout"},
		LatencyConfig: &slos.ApmLatencySli{
			TimeWindow: slos.WINDOWSLOWINDOW_WINDOW_SLO_WINDOW_5_MINUTES.Ptr(),
			Threshold:  ptr(float32(500)),
			Quantile:   &slos.ApmLatencyQuantile{Percentile: ptr(float32(0.95))},
		},
		Filters: []slos.ApmFilter{
			{Key: ptr("legacy"), Value: ptr("only-value")},
			{Key: ptr("modern"), Value: ptr(""), Values: []string{"500", "503"}},
		},
	}

	model, diags := flattenSLOV2(context.Background(), slo)
	if diags.HasError() {
		t.Fatalf("flatten: %v", diags)
	}

	var apm ApmSliModel
	if diags := sliObject(t, model).ApmSli.As(context.Background(), &apm, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("converting apm sli: %v", diags)
	}

	filters := apm.Filters.Elements()
	if len(filters) != 2 {
		t.Fatalf("filters = %d, want 2", len(filters))
	}

	var legacy ApmFilterModel
	if diags := filters[0].(types.Object).As(context.Background(), &legacy, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("converting legacy filter: %v", diags)
	}
	if got := len(legacy.Values.Elements()); got != 1 {
		t.Fatalf("legacy filter values = %d, want 1", got)
	}
	if got := legacy.Values.Elements()[0].(types.String).ValueString(); got != "only-value" {
		t.Errorf("legacy filter value = %q, want %q", got, "only-value")
	}

	var modern ApmFilterModel
	if diags := filters[1].(types.Object).As(context.Background(), &modern, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("converting modern filter: %v", diags)
	}
	if got := len(modern.Values.Elements()); got != 2 {
		t.Errorf("modern filter values = %d, want 2", got)
	}

	var latency ApmLatencySliModel
	if diags := apm.LatencyConfig.As(context.Background(), &latency, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("converting latency config: %v", diags)
	}
	if got := latency.Threshold.ValueFloat32(); got != 500 {
		t.Errorf("threshold = %v, want 500", got)
	}
	if !latency.Average.IsNull() {
		t.Errorf("average = %v, want null", latency.Average)
	}
}

// The backend injects an empty array into whichever of staticValues/labelKeys
// the caller did not set; keeping it would show as drift against a
// configuration that sets only one of them.
func TestFlattenOwnershipTagsNormalizesInjectedEmpties(t *testing.T) {
	slo := baseSLO()
	slo.WindowBasedMetricSli = windowSLI()
	slo.OwnershipTags = &slos.SloOwnershipTags{
		Environment: &slos.SloOwnershipTag{
			StaticValues:   []string{"prod", "staging"},
			LabelKeys:      []string{},
			ResolvedValues: []string{},
		},
		Team: &slos.SloOwnershipTag{
			StaticValues:   []string{},
			LabelKeys:      []string{"owning_team"},
			ResolvedValues: []string{},
		},
	}

	model, diags := flattenSLOV2(context.Background(), slo)
	if diags.HasError() {
		t.Fatalf("flatten: %v", diags)
	}

	var tags OwnershipTagsModel
	if diags := model.OwnershipTags.As(context.Background(), &tags, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("converting ownership tags: %v", diags)
	}
	if !tags.Service.IsNull() {
		t.Errorf("service = %v, want null", tags.Service)
	}

	var environment OwnershipTagModel
	if diags := tags.Environment.As(context.Background(), &environment, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("converting environment: %v", diags)
	}
	if !environment.LabelKeys.IsNull() {
		t.Errorf("environment.label_keys = %v, want null", environment.LabelKeys)
	}
	if !environment.ResolvedValues.IsNull() {
		t.Errorf("environment.resolved_values = %v, want null", environment.ResolvedValues)
	}
	if got := len(environment.StaticValues.Elements()); got != 2 {
		t.Errorf("environment.static_values = %d, want 2", got)
	}

	var team OwnershipTagModel
	if diags := tags.Team.As(context.Background(), &team, basetypes.ObjectAsOptions{}); diags.HasError() {
		t.Fatalf("converting team: %v", diags)
	}
	if !team.StaticValues.IsNull() {
		t.Errorf("team.static_values = %v, want null", team.StaticValues)
	}
}

// extractSLOV2Payload copies field by field, so a field left out of it is
// silently dropped from every create and replace.
func TestExtractSLOV2PayloadCarriesEveryWritableField(t *testing.T) {
	slo := baseSLO()
	slo.WindowBasedMetricSli = windowSLI()
	slo.WindowBasedMetricSli.MissingDataStrategy = slos.MISSINGDATASTRATEGY_MISSING_DATA_STRATEGY_GOOD.Ptr()
	slo.ProductType = slos.SLOPRODUCTTYPE_SLO_PRODUCT_TYPE_APM.Ptr()
	slo.OwnershipTags = &slos.SloOwnershipTags{Team: &slos.SloOwnershipTag{StaticValues: []string{"sre"}}}
	slo.ApmSli = &slos.ApmSli{Services: []string{"checkout"}, ErrorConfig: map[string]interface{}{}}
	// Server-owned; must never reach the request body.
	slo.ApmSliMetadata = &slos.ApmSli{Services: []string{"checkout"}}

	payload := extractSLOV2Payload(slo)

	if payload.ProductType == nil || *payload.ProductType != slos.SLOPRODUCTTYPE_SLO_PRODUCT_TYPE_APM {
		t.Errorf("productType = %v, want APM", payload.ProductType)
	}
	if payload.OwnershipTags == nil || payload.OwnershipTags.Team == nil {
		t.Error("ownershipTags dropped from the payload")
	}
	if payload.ApmSli == nil {
		t.Error("apmSli dropped from the payload")
	}
	if payload.WindowBasedMetricSli == nil || payload.WindowBasedMetricSli.GetMissingDataStrategy() != slos.MISSINGDATASTRATEGY_MISSING_DATA_STRATEGY_GOOD {
		t.Error("missingDataStrategy dropped from the payload")
	}
	if payload.ApmSliMetadata != nil {
		t.Error("apmSliMetadata must not be sent; the backend rejects it")
	}
}

// An omitted enum has to stay out of the request body so the backend can apply
// its own default, while a known one is always sent - a replace that leaves
// missingDataStrategy out resets the stored value to uncounted.
func TestExtractWindowBasedSLIOmitsUnknownMissingDataStrategy(t *testing.T) {
	ctx := context.Background()

	query, diags := types.ObjectValueFrom(ctx, sloMetricQueryAttr(), SLOMetricQueryModel{Query: types.StringValue("up")})
	if diags.HasError() {
		t.Fatalf("building query: %v", diags)
	}

	for _, tc := range []struct {
		name     string
		strategy types.String
		want     *slos.MissingDataStrategy
	}{
		{"unknown", types.StringUnknown(), nil},
		{"null", types.StringNull(), nil},
		{"known", types.StringValue("bad"), slos.MISSINGDATASTRATEGY_MISSING_DATA_STRATEGY_BAD.Ptr()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sliObj, diags := types.ObjectValueFrom(ctx, windowBasedMetricSliAttr(), WindowBasedMetricSliModel{
				Query:               query,
				Window:              types.StringValue("1_minute"),
				ComparisonOperator:  types.StringValue("less_than"),
				Threshold:           types.Float32Value(1),
				MissingDataStrategy: tc.strategy,
			})
			if diags.HasError() {
				t.Fatalf("building sli: %v", diags)
			}

			sli, diags := extractWindowBasedSLI(ctx, sliObj)
			if diags.HasError() {
				t.Fatalf("extract: %v", diags)
			}

			switch {
			case tc.want == nil && sli.MissingDataStrategy != nil:
				t.Errorf("missingDataStrategy = %v, want it omitted", *sli.MissingDataStrategy)
			case tc.want != nil && (sli.MissingDataStrategy == nil || *sli.MissingDataStrategy != *tc.want):
				t.Errorf("missingDataStrategy = %v, want %v", sli.MissingDataStrategy, *tc.want)
			}
		})
	}
}

// A dimension with no values is dropped by the backend, and an ownership block
// with no dimensions is dropped whole; neither may reach the request body.
func TestExtractOwnershipTagsOmitsValuelessDimensions(t *testing.T) {
	ctx := context.Background()

	empty, diags := types.ObjectValueFrom(ctx, ownershipTagAttr(), OwnershipTagModel{
		StaticValues:   types.ListNull(types.StringType),
		LabelKeys:      types.ListNull(types.StringType),
		ResolvedValues: types.ListNull(types.StringType),
	})
	if diags.HasError() {
		t.Fatalf("building dimension: %v", diags)
	}

	tags, diags := types.ObjectValueFrom(ctx, ownershipTagsAttr(), OwnershipTagsModel{
		Service:     empty,
		Environment: types.ObjectNull(ownershipTagAttr()),
		Team:        types.ObjectNull(ownershipTagAttr()),
	})
	if diags.HasError() {
		t.Fatalf("building ownership tags: %v", diags)
	}

	extracted, diags := extractOwnershipTags(ctx, tags)
	if diags.HasError() {
		t.Fatalf("extract: %v", diags)
	}
	if extracted != nil {
		t.Errorf("ownershipTags = %v, want it omitted", extracted)
	}
}
