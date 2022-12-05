package coralogix

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccCoralogixDataSourceDataSet_basic(t *testing.T) {
	resourceName := "coralogix_data_set.test"
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
		CheckDestroy:      testAccCheckDataSetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceDataSet(name, description, filePath) +
					testAccCoralogixDataSourceDataSet_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(fmt.Sprintf("data.%s", resourceName), "name", name),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceDataSet_read() string {
	return `data "coralogix_data_set" "test" {
	id = coralogix_data_set.test.id
}
`
}
