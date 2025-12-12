package provider

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var dataEnrichmentResourceName = "coralogix_data_enrichments.test"

func TestAccCoralogixResourceCustomDataEnrichments(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-test")
	description := acctest.RandomWithPrefix("tf-acc-test")
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	parent := filepath.Dir(filepath.Dir(wd))
	filePath := parent + "/examples/resources/coralogix_data_set/date-to-day-of-the-week.csv"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceCustomDataEnrichments(name, description, fmt.Sprintf("file(%v)", filePath)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dataEnrichmentResourceName, "id"),
					resource.TestCheckResourceAttr(dataEnrichmentResourceName, "name", name),
					resource.TestCheckResourceAttr(dataEnrichmentResourceName, "description", description),
					resource.TestCheckResourceAttr(dataEnrichmentResourceName, "version", "1"),
				),
			},
			{
				ResourceName: dataEnrichmentResourceName,
				ImportState:  true,
			},
		},
	})
}

func TestAccCoralogixResourceCustomDataEnrichmentsWithUploadedFile(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-test")
	description := acctest.RandomWithPrefix("tf-acc-test")
	updatedTestData := "\"Date,day of week\\n7/30/21,Friday\\n7/31/21,Saturday\\n8/1/21,Sunday\\n\""

	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	parent := filepath.Dir(filepath.Dir(wd))
	filePath := parent + "/examples/resources/coralogix_data_set/date-to-day-of-the-week.csv"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceCustomDataEnrichments(name, description, fmt.Sprintf("file(\"%v\")", filePath)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dataEnrichmentResourceName, "custom.custom_enrichment_data.id"),
					resource.TestCheckResourceAttrSet(dataEnrichmentResourceName, "custom.custom_enrichment_data.contents"),
					resource.TestCheckResourceAttr(dataEnrichmentResourceName, "custom.custom_enrichment_data.version", "1"),
					resource.TestCheckResourceAttr(dataEnrichmentResourceName, "custom.custom_enrichment_data.name", name),
					resource.TestCheckResourceAttr(dataEnrichmentResourceName, "custom.custom_enrichment_data.description", description),
				),
			},
			{
				ResourceName: dataEnrichmentResourceName,
				ImportState:  true,
			},
			{
				Config: testAccCoralogixResourceCustomDataEnrichments(name, description, updatedTestData),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dataEnrichmentResourceName, "id"),
					resource.TestCheckResourceAttr(dataEnrichmentResourceName, "custom.custom_enrichment_data.name", name),
					resource.TestCheckResourceAttr(dataEnrichmentResourceName, "custom.custom_enrichment_data.description", description),
					resource.TestCheckResourceAttr(dataEnrichmentResourceName, "custom.custom_enrichment_data.version", "2"),
				),
			},
			{
				PlanOnly: true,
				Config:   testAccCoralogixResourceCustomDataEnrichments(name, description, updatedTestData),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dataEnrichmentResourceName, "custom.custom_enrichment_data.id"),
					resource.TestCheckResourceAttr(dataEnrichmentResourceName, "custom.custom_enrichment_data.name", name),
					resource.TestCheckResourceAttr(dataEnrichmentResourceName, "custom.custom_enrichment_data.description", description),
					resource.TestCheckResourceAttr(dataEnrichmentResourceName, "custom.custom_enrichment_data.version", "2"),
				),
			},
		},
	})
}

func TestAccCoralogixResourceGeoIpDataEnrichment(t *testing.T) {
	fieldName := "coralogix.metadata.sdkId"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceGeoIpDataEnrichment(fieldName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dataEnrichmentResourceName, "geo_ip.fields.0.id"),
					resource.TestCheckResourceAttr(dataEnrichmentResourceName, "geo_ip.fields.0.name", fieldName),
				),
			},
			{
				ResourceName:      dataEnrichmentResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceSuspiciousIpDataEnrichment(t *testing.T) {
	fieldName := "coralogix.metadata.sdkId"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceSuspiciousIpDataEnrichment(fieldName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dataEnrichmentResourceName, "id"),
					resource.TestCheckResourceAttr(dataEnrichmentResourceName, "suspicious_ip.fields.0.name", fieldName),
				),
			},
			{
				ResourceName:      dataEnrichmentResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceGeoIpAndSuspiciousIpDataEnrichment(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceGeoIpSusIpDataEnrichments(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dataEnrichmentResourceName, "id"),
					resource.TestCheckResourceAttr(dataEnrichmentResourceName, "geo_ip.fields.0.name", "coralogix.metadata.sdkId"),
					resource.TestCheckResourceAttr(dataEnrichmentResourceName, "suspicious_ip.fields.0.name", "coralogix.metadata.requestId"),
				),
			},
			{
				ResourceName:      dataEnrichmentResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceCustomDataEnrichment(t *testing.T) {
	fieldName := "coralogix.metadata.sdkId"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceCustomDataEnrichment(fieldName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(dataEnrichmentResourceName, "id"),
					resource.TestCheckResourceAttrSet(dataEnrichmentResourceName, "custom.custom_enrichment_data.id"),
					resource.TestCheckResourceAttrSet(dataEnrichmentResourceName, "custom.custom_enrichment_data.version"),
					resource.TestCheckResourceAttrSet(dataEnrichmentResourceName, "custom.custom_enrichment_data.contents"),
					resource.TestCheckResourceAttr(dataEnrichmentResourceName, "custom.custom_enrichment_data.name", "custom enrichment"),
					resource.TestCheckResourceAttr(dataEnrichmentResourceName, "custom.custom_enrichment_data.description", "description"),
					resource.TestCheckResourceAttr(dataEnrichmentResourceName, "custom.fields.0.name", fieldName),
				),
			},
			{
				ResourceName:      dataEnrichmentResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCoralogixResourceGeoIpDataEnrichment(fieldName string) string {
	return fmt.Sprintf(`resource "coralogix_data_enrichments" test {
              geo_ip = {
                fields = [{
					name = "%s"
					enriched_field_name = "field_enriched"
                }]
            }
        }
        `, fieldName)
}

func testAccCoralogixResourceSuspiciousIpDataEnrichment(fieldName string) string {
	return fmt.Sprintf(`resource "coralogix_data_enrichments" test {
            suspicious_ip = {
                fields = [{
					name = "%s"
					enriched_field_name = "field_enriched"
                }]
            }
        }
        `, fieldName)
}

func testAccCoralogixResourceCustomDataEnrichment(fieldName string) string {
	return fmt.Sprintf(`
    resource "coralogix_data_enrichments" test{
        custom = {
            custom_enrichment_data = {
				name         = "custom enrichment"
				description  = "description"
				contents = "local_id,instance_type\nfoo1,t2.micro\nfoo2,t2.micro\nfoo3,t2.micro\nbar1,m3.large\n"			
			}
            fields = [{
				name = "%s"
				enriched_field_name = "field_enriched"
			}]
        }
    }
    `, fieldName)
}

func testAccCoralogixResourceGeoIpSusIpDataEnrichments() string {
	return `resource "coralogix_data_enrichments" test {
	geo_ip = {
		fields = [{
			name = "coralogix.metadata.sdkId"
			enriched_field_name = "field_enriched"
		}]
	}
	suspicious_ip = {
		fields = [{
			name = "coralogix.metadata.requestId"
			enriched_field_name = "sus_ip_field_enriched"
		}]
	}
}`
}

func popLineFromCsvFile(path string) {
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

func testAccCoralogixResourceCustomDataEnrichments(name, description, fileContents string) string {
	return fmt.Sprintf(`
	resource "coralogix_data_enrichments" test{
        custom = {
            custom_enrichment_data = {
				name         = "%s"
				description  = "%s"
				contents     = %s
			}
            fields = []
        }
    }
	`, name, description, fileContents)
}
