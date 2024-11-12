// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package coralogix

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"terraform-provider-coralogix/coralogix/clientset"
	enrichmentv1 "terraform-provider-coralogix/coralogix/clientset/grpc/enrichment/v1"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"google.golang.org/protobuf/types/known/wrapperspb"
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
	filePath := parent + "/examples/resources/coralogix_data_set/date-to-day-of-the-week.csv"
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
	filePath := parent + "/examples/resources/coralogix_data_set/date-to-day-of-the-week.csv"
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
