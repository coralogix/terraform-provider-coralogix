package coralogix

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"terraform-provider-coralogix/coralogix/clientset"
)

func TestAccCoralogixResourceGeoIpeEnrichment(t *testing.T) {
	resourceName := "coralogix_enrichment.test"
	fieldName := "coralogix.metadata.sdkId"
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckEnrichmentDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceGeoIpEnrichment(fieldName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "geo_ip.0.fields.0.name", fieldName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceSuspiciousIpEnrichment(t *testing.T) {
	resourceName := "coralogix_enrichment.test"
	fieldName := "coralogix.metadata.sdkId"
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckEnrichmentDestroy,
		Steps: []resource.TestStep{
			{

				Config: testAccCoralogixResourceSuspiciousIpEnrichment(fieldName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "suspicious_ip.0.fields.0.name", fieldName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

//func TestAccCoralogixResourceAwsEnrichment(t *testing.T) {
//	resourceName := "coralogix_enrichment.test"
//	fieldName := "coralogix.metadata.sdkId"
//	resourceType := ""
//	resource.Test(t, resource.TestCase{
//		PreCheck:          func() { testAccPreCheck(t) },
//		ProviderFactories: testAccProviderFactories,
//		CheckDestroy:      testAccCheckEnrichmentDestroy,
//		Steps: []resource.TestStep{
//			{
//				Config: testAccCoralogixResourceAwsEnrichment(fieldName, resourceType),
//				Check: resource.ComposeAggregateTestCheckFunc(
//					resource.TestCheckResourceAttrSet(resourceName, "id"),
//					resource.TestCheckResourceAttr(resourceName, "aws.0.fields.0.name", fieldName),
//					resource.TestCheckResourceAttr(resourceName, "aws.0.fields.0.resource_type", resourceType),
//				),
//			},
//			{
//				ResourceName:      resourceName,
//				ImportState:       true,
//				ImportStateVerify: true,
//			},
//		},
//	})
//}

func TestAccCoralogixResourceCustomEnrichment(t *testing.T) {
	resourceName := "coralogix_enrichment.test"
	fieldName := "coralogix.metadata.sdkId"
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      testAccCheckEnrichmentDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceCustomEnrichment(fieldName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttr(resourceName, "custom.0.fields.0.name", fieldName),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCoralogixResourceGeoIpEnrichment(fieldName string) string {
	return fmt.Sprintf(`resource "coralogix_enrichment" test {
  			geo_ip {
    			fields {
      				name = "%s"
    			}
			}
		}
		`, fieldName)
}

func testAccCoralogixResourceSuspiciousIpEnrichment(fieldName string) string {
	return fmt.Sprintf(`resource "coralogix_enrichment" test {
			suspicious_ip {
				fields {
      				name = "%s"
    			}
			}
		}
		`, fieldName)
}

//func testAccCoralogixResourceAwsEnrichment(fieldName, resourceType string) string {
//	return fmt.Sprintf(`resource "coralogix_enrichment" test{
//			aws{
//				fields {
//					name = "%s"
//					resource_type = "%s"
//				}
//			}
//	}
//	`, fieldName, resourceType)
//}

func testAccCoralogixResourceCustomEnrichment(fieldName string) string {
	return fmt.Sprintf(`resource "coralogix_data_set" test {
		name         = "custom enrichment"
		description  = "description"
		file_content = "local_id,instance_type\nfoo1,t2.micro\nfoo2,t2.micro\nfoo3,t2.micro\nbar1,m3.large\n"
	}

	resource "coralogix_enrichment" test{
		custom{
			custom_enrichment_id = coralogix_data_set.test.id
			fields {
					name = "%s"
				}
		}
	}
	`, fieldName)
}

func testAccCheckEnrichmentDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).Enrichments()

	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_enrichment" {
			continue
		}

		resp, err := client.GetEnrichmentsByType(ctx, rs.Primary.ID)
		if err == nil {
			if len(resp) != 0 {
				return fmt.Errorf("enrichment still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func testAccCheckCustomEnrichmentDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).Enrichments()

	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_enrichment" {
			continue
		}

		resp, err := client.GetCustomEnrichments(ctx, strToUint32(rs.Primary.ID))
		if err == nil {
			if len(resp) != 0 {
				return fmt.Errorf("enrichment still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}