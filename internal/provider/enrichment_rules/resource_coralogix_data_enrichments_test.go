package enrichment_rules

import (
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

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
