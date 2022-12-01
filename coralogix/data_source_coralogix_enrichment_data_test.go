package coralogix

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccCoralogixDataSourceEnrichmentData_basic(t *testing.T) {
	resourceName := "coralogix_enrichment_data.test"
	name := acctest.RandomWithPrefix("tf-acc-test")
	description := acctest.RandomWithPrefix("tf-acc-test")
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckEnrichmentDataDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceEnrichmentData(name, description) +
					testAccCoralogixDataSourceEnrichmentData_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(fmt.Sprintf("data.%s", resourceName), "name", name),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceEnrichmentData_read() string {
	return `data "coralogix_enrichment_data" "test" {
	id = coralogix_enrichment_data.test.id
}
`
}
