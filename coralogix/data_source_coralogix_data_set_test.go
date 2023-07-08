package coralogix

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var dataSetDataSourceName = "data." + dataSetResourceName

func TestAccCoralogixDataSourceDataSet_basic(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-test")
	description := acctest.RandomWithPrefix("tf-acc-test")
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	parent := filepath.Dir(wd)
	filePath := parent + "/examples/data_set/date-to-day-of-the-week.csv"

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckDataSetDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceDataSet(name, description, filePath) +
					testAccCoralogixDataSourceDataSet_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dataSetDataSourceName, "name", name),
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
