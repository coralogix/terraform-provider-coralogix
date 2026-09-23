package enrichment_rules

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func TestExpandAwsUsesPublicResourceAttribute(t *testing.T) {
	fields := schema.NewSet(hashAwsFields(), []interface{}{
		map[string]interface{}{
			"name":     "coralogix.metadata.aws_resource_id",
			"resource": "ec2",
		},
	})

	got := expandAws(map[string]interface{}{"fields": fields})
	if len(got) != 1 {
		t.Fatalf("expanded enrichments = %d, want 1", len(got))
	}
	if resourceType := got[0].GetEnrichmentType().GetAws().GetResourceType().GetValue(); resourceType != "ec2" {
		t.Fatalf("resource = %q, want ec2", resourceType)
	}
}
