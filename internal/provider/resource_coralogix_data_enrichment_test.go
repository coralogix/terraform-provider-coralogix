package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var dataEnrichmentResourceName = "coralogix_data_enrichments.test"

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
	return fmt.Sprintf(`resource "coralogix_data_set" test {
        name         = "custom enrichment"
        description  = "description"
        file_content = "local_id,instance_type\nfoo1,t2.micro\nfoo2,t2.micro\nfoo3,t2.micro\nbar1,m3.large\n"
    }

    resource "coralogix_data_enrichments" test{
        custom {
            custom_enrichment_id = coralogix_data_set.test.id
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
