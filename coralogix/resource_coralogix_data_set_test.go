package coralogix

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-coralogix/coralogix/clientset"
	enrichmentv1 "terraform-provider-coralogix/coralogix/clientset/grpc/enrichment/v1"
)

var dataSetResourceName = "coralogix_data_set.test"

func TestAccCoralogixResourceDataSet(t *testing.T) {
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
				Config: testAccCoralogixResourceDataSet(name, description, filePath),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dataSetResourceName, "id"),
					resource.TestCheckResourceAttr(dataSetResourceName, "name", name),
					resource.TestCheckResourceAttr(dataSetResourceName, "description", description),
					resource.TestCheckResourceAttr(dataSetResourceName, "version", "1"),
				),
			},
			{
				ResourceName: dataSetResourceName,
				ImportState:  true,
			},
		},
	})
}

func TestAccCoralogixResourceDataSetWithUploadedFile(t *testing.T) {
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
				Config: testAccCoralogixResourceDataSetWithUploadedFile(name, description, filePath),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dataSetResourceName, "id"),
					resource.TestCheckResourceAttr(dataSetResourceName, "name", name),
					resource.TestCheckResourceAttr(dataSetResourceName, "description", description),
					resource.TestCheckResourceAttr(dataSetResourceName, "version", "1"),
					resource.TestCheckResourceAttr(dataSetResourceName, "uploaded_file.0.path", filePath),
					resource.TestCheckResourceAttr(dataSetResourceName, "uploaded_file.0.updated_from_uploading", "false"),
				),
			},
			{
				ResourceName: dataSetResourceName,
				ImportState:  true,
			},
			{
				PreConfig: func() { removeLineFromCsvFile(filePath) },
				PlanOnly:  true,
				Config:    testAccCoralogixResourceDataSetWithUploadedFile(name, description, filePath),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dataSetResourceName, "id"),
					resource.TestCheckResourceAttr(dataSetResourceName, "name", name),
					resource.TestCheckResourceAttr(dataSetResourceName, "description", description),
					resource.TestCheckResourceAttr(dataSetResourceName, "version", "1"),
					resource.TestCheckResourceAttr(dataSetResourceName, "uploaded_file.0.path", filePath),
					resource.TestCheckResourceAttr(dataSetResourceName, "uploaded_file.0.updated_from_uploading", "true"),
					resource.TestCheckResourceAttr(dataSetResourceName, "uploaded_file.0.path", filePath)),
			},
			{
				Config: testAccCoralogixResourceDataSetWithUploadedFile(name, description, filePath),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dataSetResourceName, "id"),
					resource.TestCheckResourceAttr(dataSetResourceName, "name", name),
					resource.TestCheckResourceAttr(dataSetResourceName, "description", description),
					resource.TestCheckResourceAttr(dataSetResourceName, "version", "1"),
					resource.TestCheckResourceAttr(dataSetResourceName, "uploaded_file.0.path", filePath),
					resource.TestCheckResourceAttr(dataSetResourceName, "uploaded_file.0.updated_from_uploading", "false"),
				),
			},
		},
	})
}

func removeLineFromCsvFile(path string) {
	f, _ := os.Open(path)
	csvReader := csv.NewReader(f)
	_, err := csvReader.Read()
	if err != nil {
		panic(err)
	}
	rec, err := csvReader.Read()
	if err != nil {
		panic(err)
	}
	csvWriter := csv.NewWriter(f)
	err = csvWriter.Write(rec)
	if err != nil {
		panic(err)
	}
}

func testAccCoralogixResourceDataSet(name, description, filePath string) string {
	return fmt.Sprintf(`resource "coralogix_data_set" test {
		name         = "%s"
		description  = "%s"
		file_content = file("%s")
	}
	`, name, description, filePath)
}

func testAccCoralogixResourceDataSetWithUploadedFile(name, description, filePath string) string {
	return fmt.Sprintf(
		`resource "coralogix_data_set" test {
  					name        = "%s"
  					description = "%s"
  					uploaded_file {
						path = "%s"
  					}
			}
			`, name, description, filePath)
}

func testAccCheckDataSetDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).DataSet()

	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_enrichment" {
			continue
		}

		resp, err := client.GetDataSet(ctx, &enrichmentv1.GetCustomEnrichmentRequest{Id: wrapperspb.UInt32(strToUint32(rs.Primary.ID))})
		if err == nil {
			if uint32ToStr(resp.GetCustomEnrichment().GetId()) == rs.Primary.ID {
				return fmt.Errorf("enrichment still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}
