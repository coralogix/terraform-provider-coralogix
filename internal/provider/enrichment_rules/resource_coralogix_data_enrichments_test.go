package enrichment_rules

import (
	"context"
	"slices"
	"testing"

	cess "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/custom_enrichments_service"
	ess "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/enrichments_service"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestDataEnrichmentsIDPlanModifierAllowsRecoveryFromNullState(t *testing.T) {
	var schemaResp resource.SchemaResponse
	(&DataEnrichmentsResource{}).Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)
	idAttribute := schemaResp.Schema.Attributes["id"].(schema.StringAttribute)
	modifier := idAttribute.PlanModifiers[0]

	req := planmodifier.StringRequest{
		ConfigValue: types.StringNull(),
		PlanValue:   types.StringUnknown(),
		StateValue:  types.StringNull(),
	}
	resp := planmodifier.StringResponse{PlanValue: req.PlanValue}
	modifier.PlanModifyString(context.Background(), req, &resp)
	if !resp.PlanValue.IsUnknown() {
		t.Fatalf("plan value = %v, want unknown when prior ID is null", resp.PlanValue)
	}

	req.StateValue = types.StringValue("geo_ip")
	resp.PlanValue = req.PlanValue
	modifier.PlanModifyString(context.Background(), req, &resp)
	if got, want := resp.PlanValue.ValueString(), "geo_ip"; got != want {
		t.Fatalf("plan value = %q, want prior ID %q", got, want)
	}
}

func TestDataEnrichmentsEnrichedFieldNameIsOptionalForEveryType(t *testing.T) {
	var resp resource.SchemaResponse
	(&DataEnrichmentsResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)

	for _, enrichmentType := range []string{AWS_TYPE, GEOIP_TYPE, SUSIP_TYPE, CUSTOM_TYPE} {
		t.Run(enrichmentType, func(t *testing.T) {
			typeAttribute := resp.Schema.Attributes[enrichmentType].(schema.SingleNestedAttribute)
			fieldsAttribute := typeAttribute.Attributes["fields"].(schema.ListNestedAttribute)
			fieldNameAttribute := fieldsAttribute.NestedObject.Attributes["enriched_field_name"].(schema.StringAttribute)

			if !fieldNameAttribute.Optional || fieldNameAttribute.Required || fieldNameAttribute.Computed {
				t.Fatalf("enriched_field_name flags = optional:%t required:%t computed:%t, want optional only",
					fieldNameAttribute.Optional,
					fieldNameAttribute.Required,
					fieldNameAttribute.Computed,
				)
			}
		})
	}
}

func TestExtractDataEnrichmentsPreservesNullEnrichedFieldName(t *testing.T) {
	customID := int64(42)
	model := &DataEnrichmentsModel{
		Aws: &AwsEnrichmentFieldsModel{Fields: []AwsEnrichmentFieldModel{{
			Name:     types.StringValue("aws_field"),
			Resource: types.StringValue("ec2"),
		}}},
		GeoIp: &GeoIpEnrichmentFieldsModel{Fields: []GeoIpEnrichmentFieldModel{{
			Name: types.StringValue("geo_field"),
		}}},
		SuspiciousIp: &EnrichmentFieldsModel{Fields: []EnrichmentFieldModel{{
			Name: types.StringValue("suspicious_field"),
		}}},
		Custom: &CustomEnrichmentFieldsModel{
			CustomEnrichmentDataModel: &CustomEnrichmentDataModel{ID: types.Int64Value(customID)},
			Fields:                    []EnrichmentFieldModel{{Name: types.StringValue("custom_field")}},
		},
	}

	got := extractDataEnrichments(model)
	if len(got) != 4 {
		t.Fatalf("request count = %d, want 4", len(got))
	}
	for i, request := range got {
		if request.EnrichedFieldName != nil {
			t.Errorf("request %d enriched_field_name = %q, want nil", i, *request.EnrichedFieldName)
		}
	}
}

func TestExtractDataEnrichmentsKeepsGeoIpWithAsnPerField(t *testing.T) {
	model := &DataEnrichmentsModel{
		GeoIp: &GeoIpEnrichmentFieldsModel{Fields: []GeoIpEnrichmentFieldModel{
			{Name: types.StringValue("first"), Asn: types.BoolValue(true)},
			{Name: types.StringValue("second"), Asn: types.BoolValue(false)},
		}},
	}

	got := extractDataEnrichments(model)
	if len(got) != 2 {
		t.Fatalf("request count = %d, want 2", len(got))
	}
	if got[0].EnrichmentType.GeoIp == got[1].EnrichmentType.GeoIp {
		t.Fatal("Geo IP requests share one enrichment type pointer")
	}
	if withAsn := got[0].EnrichmentType.GeoIp.WithAsn; withAsn == nil || !*withAsn {
		t.Errorf("first with_asn = %v, want true", withAsn)
	}
	if withAsn := got[1].EnrichmentType.GeoIp.WithAsn; withAsn == nil || *withAsn {
		t.Errorf("second with_asn = %v, want false", withAsn)
	}
}

func TestDataEnrichmentFieldsEqualDetectsGeoIpWithAsnChangeBeforeLastField(t *testing.T) {
	plan := &DataEnrichmentsModel{
		GeoIp: &GeoIpEnrichmentFieldsModel{Fields: []GeoIpEnrichmentFieldModel{
			{Name: types.StringValue("first"), Asn: types.BoolValue(true)},
			{Name: types.StringValue("second"), Asn: types.BoolValue(false)},
		}},
	}
	state := &DataEnrichmentsModel{
		GeoIp: &GeoIpEnrichmentFieldsModel{Fields: []GeoIpEnrichmentFieldModel{
			{Name: types.StringValue("first"), Asn: types.BoolValue(false)},
			{Name: types.StringValue("second"), Asn: types.BoolValue(false)},
		}},
	}

	if dataEnrichmentFieldsEqual(plan, state) {
		t.Fatal("fields with a changed non-last Geo IP with_asn value must not be equal")
	}
}

func TestDataEnrichmentFieldsEqualIgnoresComputedFieldIDs(t *testing.T) {
	customID := int64(42)
	plan := &DataEnrichmentsModel{
		Custom: &CustomEnrichmentFieldsModel{
			CustomEnrichmentDataModel: &CustomEnrichmentDataModel{ID: types.Int64Value(customID)},
			Fields: []EnrichmentFieldModel{{
				ID:   types.Int64Unknown(),
				Name: types.StringValue("custom_field"),
			}},
		},
	}
	state := &DataEnrichmentsModel{
		Custom: &CustomEnrichmentFieldsModel{
			CustomEnrichmentDataModel: &CustomEnrichmentDataModel{ID: types.Int64Value(customID)},
			Fields: []EnrichmentFieldModel{{
				ID:   types.Int64Value(1001),
				Name: types.StringValue("custom_field"),
			}},
		},
	}

	if !dataEnrichmentFieldsEqual(plan, state) {
		t.Fatal("fields with the same configuration and different computed IDs must be equal")
	}

	plan.Custom.Fields[0].EnrichedFieldName = types.StringValue("custom_field_enriched")
	if dataEnrichmentFieldsEqual(plan, state) {
		t.Fatal("fields with different enriched_field_name values must not be equal")
	}
}

func TestDataEnrichmentsModelGetFieldsReturnsEveryFieldID(t *testing.T) {
	model := &DataEnrichmentsModel{
		Aws: &AwsEnrichmentFieldsModel{
			Fields: []AwsEnrichmentFieldModel{
				{ID: types.Int64Value(101)},
				{ID: types.Int64Value(102)},
			},
		},
		GeoIp: &GeoIpEnrichmentFieldsModel{
			Fields: []GeoIpEnrichmentFieldModel{
				{ID: types.Int64Value(201)},
				{ID: types.Int64Value(202)},
			},
		},
		SuspiciousIp: &EnrichmentFieldsModel{
			Fields: []EnrichmentFieldModel{
				{ID: types.Int64Value(301)},
				{ID: types.Int64Value(302)},
			},
		},
		Custom: &CustomEnrichmentFieldsModel{
			Fields: []EnrichmentFieldModel{
				{ID: types.Int64Value(401)},
				{ID: types.Int64Value(402)},
			},
		},
	}

	got := ExtractIdsFromEnrichment(model.GetFields())
	want := []uint32{101, 102, 201, 202, 301, 302, 401, 402}
	if !slices.Equal(got, want) {
		t.Fatalf("ExtractIdsFromEnrichment(model.GetFields()) = %v, want %v", got, want)
	}
}

func TestFlattenDataEnrichmentsUsesUniqueCanonicalTypeID(t *testing.T) {
	resourceType := "ec2"
	customID := int64(42)
	enrichments := []ess.Enrichment{
		{EnrichmentType: ess.EnrichmentType{CustomEnrichment: &ess.CustomEnrichmentType{Id: &customID}}, FieldName: "custom"},
		{EnrichmentType: ess.EnrichmentType{SuspiciousIp: map[string]interface{}{}}, FieldName: "suspicious"},
		{EnrichmentType: ess.EnrichmentType{GeoIp: &ess.GeoIpType{}}, FieldName: "geo"},
		{EnrichmentType: ess.EnrichmentType{Aws: &ess.AwsType{ResourceType: &resourceType}}, FieldName: "aws-1"},
		{EnrichmentType: ess.EnrichmentType{Aws: &ess.AwsType{ResourceType: &resourceType}}, FieldName: "aws-2"},
	}

	model := flattenDataEnrichments(enrichments, nil, nil)

	if got, want := model.ID.ValueString(), "aws,geo_ip,suspicious_ip,custom"; got != want {
		t.Errorf("id = %q, want %q", got, want)
	}
}

func TestFlattenDataEnrichmentsUsesCustomDataID(t *testing.T) {
	id := int64(42)
	name := "custom_enrichment"
	contents := "key,value\nfoo,bar\n"

	model := flattenDataEnrichments(nil, &cess.CustomEnrichment{Id: &id, Name: &name}, &contents)

	if got, want := model.ID.ValueString(), "42"; got != want {
		t.Errorf("id = %q, want %q", got, want)
	}
}

func TestEnrichmentTypesFromIDUsesCustomTypeForNumericID(t *testing.T) {
	got := enrichmentTypesFromID("42")
	want := []string{CUSTOM_TYPE}

	if !slices.Equal(got, want) {
		t.Errorf("enrichmentTypesFromID(42) = %v, want %v", got, want)
	}
}

func TestEnrichmentTypesFromModelRecoversTypesWhenIDIsNull(t *testing.T) {
	tests := map[string]struct {
		model *DataEnrichmentsModel
		want  []string
	}{
		"standard": {
			model: &DataEnrichmentsModel{
				ID:           types.StringNull(),
				GeoIp:        &GeoIpEnrichmentFieldsModel{},
				SuspiciousIp: &EnrichmentFieldsModel{},
			},
			want: []string{GEOIP_TYPE, SUSIP_TYPE},
		},
		"custom": {
			model: &DataEnrichmentsModel{
				ID: types.StringNull(),
				Custom: &CustomEnrichmentFieldsModel{
					CustomEnrichmentDataModel: &CustomEnrichmentDataModel{ID: types.Int64Value(42)},
				},
			},
			want: []string{CUSTOM_TYPE},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := enrichmentTypesFromModel(tt.model)
			if !slices.Equal(got, tt.want) {
				t.Errorf("enrichmentTypesFromModel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnrichmentTypesFromIDReturnsNoTypeForEmptyID(t *testing.T) {
	if got := enrichmentTypesFromID(""); len(got) != 0 {
		t.Errorf("enrichmentTypesFromID(\"\") = %v, want no types", got)
	}
}

func TestEnrichmentTypesFromIDDeduplicatesLegacyTypes(t *testing.T) {
	got := enrichmentTypesFromID("geo_ip,geo_ip,suspicious_ip,geo_ip")
	want := []string{GEOIP_TYPE, SUSIP_TYPE}

	if !slices.Equal(got, want) {
		t.Errorf("enrichmentTypesFromID() = %v, want %v", got, want)
	}
}

func TestFilterEnrichmentByTypeAndCustomIDReturnsOnlyRequestedCustomEnrichment(t *testing.T) {
	requestedID := int64(42)
	otherID := int64(99)
	enrichments := []ess.Enrichment{
		{Id: 1, EnrichmentType: ess.EnrichmentType{CustomEnrichment: &ess.CustomEnrichmentType{Id: &requestedID}}},
		{Id: 2, EnrichmentType: ess.EnrichmentType{CustomEnrichment: &ess.CustomEnrichmentType{Id: &otherID}}},
		{Id: 3, EnrichmentType: ess.EnrichmentType{GeoIp: &ess.GeoIpType{}}},
	}

	got := FilterEnrichmentByTypeAndCustomID(enrichments, CUSTOM_TYPE, &requestedID)
	if len(got) != 1 || got[0].Id != 1 {
		t.Fatalf("filtered enrichments = %v, want only enrichment ID 1", got)
	}
}

func TestFilterDataEnrichmentsForModelIsolatesParallelCustomResponses(t *testing.T) {
	requestedID := int64(42)
	otherID := int64(99)
	model := &DataEnrichmentsModel{
		Custom: &CustomEnrichmentFieldsModel{
			CustomEnrichmentDataModel: &CustomEnrichmentDataModel{ID: types.Int64Value(requestedID)},
		},
	}
	enrichments := []ess.Enrichment{
		{Id: 1, FieldName: "requested-1", EnrichmentType: ess.EnrichmentType{CustomEnrichment: &ess.CustomEnrichmentType{Id: &requestedID}}},
		{Id: 2, FieldName: "other", EnrichmentType: ess.EnrichmentType{CustomEnrichment: &ess.CustomEnrichmentType{Id: &otherID}}},
		{Id: 3, FieldName: "requested-2", EnrichmentType: ess.EnrichmentType{CustomEnrichment: &ess.CustomEnrichmentType{Id: &requestedID}}},
	}

	got := filterDataEnrichmentsForModel(enrichments, model)
	if len(got) != 2 || got[0].Id != 1 || got[1].Id != 3 {
		t.Fatalf("filtered enrichments = %v, want enrichment IDs 1 and 3", got)
	}
}
