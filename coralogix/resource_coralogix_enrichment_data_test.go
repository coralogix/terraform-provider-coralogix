package coralogix

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"terraform-provider-coralogix/coralogix/clientset"
)

func TestAccCoralogixResourceEnrichmentData(t *testing.T) {
	resourceName := "coralogix_enrichment_data.test"
	name := acctest.RandomWithPrefix("tf-acc-test")
	description := acctest.RandomWithPrefix("tf-acc-test")
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckEnrichmentDataDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceEnrichmentData(name, description),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", name),
					resource.TestCheckResourceAttr(resourceName, "description", description),
					resource.TestCheckResourceAttr(resourceName, "version", "1"),
				),
			},
			{
				ResourceName: resourceName,
				ImportState:  true,
			},
		},
	})
}

func TestAccCoralogixResourceEnrichmentDataWithUploadedFile(t *testing.T) {
	resourceName := "coralogix_enrichment_data.test"
	name := acctest.RandomWithPrefix("tf-acc-test")
	description := acctest.RandomWithPrefix("tf-acc-test")
	filePath := "../examples/enrichment/date-to-day-of-the-week.csv"
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckEnrichmentDataDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceEnrichmentDataWithUploadedFile(name, description, filePath),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", name),
					resource.TestCheckResourceAttr(resourceName, "description", description),
					resource.TestCheckResourceAttr(resourceName, "version", "1"),
					resource.TestCheckResourceAttr(resourceName, "uploaded_file.0.path", filePath),
					resource.TestCheckResourceAttr(resourceName, "uploaded_file.0.updated_from_uploading", "false"),
				),
			},
			{
				ResourceName: resourceName,
				ImportState:  true,
			},
			{
				PreConfig: func() {
					f, _ := os.Open(filePath)
					csvReader := csv.NewReader(f)
					csvReader.Read()
					rec, _ := csvReader.Read()
					csvWriter := csv.NewWriter(f)
					csvWriter.Write(rec)
				},
				PlanOnly: true,
				Config:   testAccCoralogixResourceEnrichmentDataWithUploadedFile(name, description, filePath),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", name),
					resource.TestCheckResourceAttr(resourceName, "description", description),
					resource.TestCheckResourceAttr(resourceName, "version", "1"),
					resource.TestCheckResourceAttr(resourceName, "uploaded_file.0.path", filePath),
					resource.TestCheckResourceAttr(resourceName, "uploaded_file.0.updated_from_uploading", "true"),
					resource.TestCheckResourceAttr(resourceName, "uploaded_file.0.path", filePath)),
			},
			{
				Config: testAccCoralogixResourceEnrichmentDataWithUploadedFile(name, description, filePath),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "name", name),
					resource.TestCheckResourceAttr(resourceName, "description", description),
					resource.TestCheckResourceAttr(resourceName, "version", "1"),
					resource.TestCheckResourceAttr(resourceName, "uploaded_file.0.path", filePath),
					resource.TestCheckResourceAttr(resourceName, "uploaded_file.0.updated_from_uploading", "false"),
				),
			},
		},
	})
}

func testAccCoralogixResourceEnrichmentData(name, description string) string {
	return fmt.Sprintf(`resource "coralogix_enrichment_data" test {
		name         = "%s"
		description  = "%s"
		file_content = "local_id,instance_type\nfoo1,t2.micro\nfoo2,t2.micro\nfoo3,t2.micro\nbar1,m3.large\n"
	}
	`, name, description)
}

func testAccCoralogixResourceEnrichmentDataWithUploadedFile(name, description, filePath string) string {
	return fmt.Sprintf(
		`resource "coralogix_enrichment_data" test {
  					name        = "%s"
  					description = "%s"
  					uploaded_file {
						path = "%s"
  					}
			}
			`, name, description, filePath)
}

func testAccCheckEnrichmentDataDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).Enrichments()

	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_enrichment" {
			continue
		}

		resp, err := client.GetEnrichment(ctx, strToUint32(rs.Primary.ID))
		if err == nil {
			if uint32ToStr(resp.GetId()) == rs.Primary.ID {
				return fmt.Errorf("enrichment still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}
