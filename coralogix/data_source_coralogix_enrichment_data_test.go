package coralogix

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccCoralogixDataSourceEnrichmentData_basic(t *testing.T) {
	resourceName := "coralogix_enrichment_data.test"
	name := acctest.RandomWithPrefix("tf-acc-test")
	description := acctest.RandomWithPrefix("tf-acc-test")
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	parent := filepath.Dir(wd)
	filePath := parent + "/examples/enrichment/date-to-day-of-the-week.csv"
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckEnrichmentDataDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceEnrichmentData(name, description, filePath) +
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
