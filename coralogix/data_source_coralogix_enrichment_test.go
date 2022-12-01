package coralogix

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccCoralogixDataSourceEnrichment_basic(t *testing.T) {
	resourceName := "coralogix_enrichment.test"
	fieldName := "coralogix.metadata.sdkId"
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckEnrichmentDataDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceGeoIpEnrichment(fieldName) +
					testAccCoralogixDataSourceEnrichment_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(fmt.Sprintf("data.%s", resourceName), "geo_ip.0.field_name", fieldName),
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
