package coralogix

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var enrichmentDataSourceName = "data." + enrichmentResourceName

func TestAccCoralogixDataSourceEnrichment_basic(t *testing.T) {
	fieldName := "coralogix.metadata.sdkId"
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckDataSetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceGeoIpEnrichment(fieldName) +
					testAccCoralogixDataSourceEnrichment_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(enrichmentDataSourceName, "id"),
					resource.TestCheckResourceAttr(enrichmentDataSourceName, "geo_ip.0.fields.0.name", fieldName),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceEnrichment_read() string {
	return `data "coralogix_enrichment" "test" {
	id = coralogix_enrichment.test.id
}
`
}
