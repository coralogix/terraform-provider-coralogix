// Copyright 2026 Coralogix Ltd.
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

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var viewFolderDataSourceName = "data." + viewFolderResourceName

// Looked up by id, the data source must return the folder's name.
func TestAccCoralogixDataSourceViewFolder_by_id(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-view-folder-data-by-id")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckViewFolderDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceViewFolder(name) +
					testAccCoralogixDataSourceViewFolder_read_by_id(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(viewFolderDataSourceName, "id"),
					resource.TestCheckResourceAttr(viewFolderDataSourceName, "name", name),
				),
			},
		},
	})
}

// Looked up by name, the data source must return the folder's id.
func TestAccCoralogixDataSourceViewFolder_by_name(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-view-folder-data-by-name")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckViewFolderDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceViewFolder(name) +
					testAccCoralogixDataSourceViewFolder_read_by_name(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(viewFolderDataSourceName, "name", name),
					resource.TestCheckResourceAttrPair(
						viewFolderDataSourceName, "id", viewFolderResourceName, "id"),
				),
			},
		},
	})
}

// A name that matches nothing must produce a diagnostic, not a zero-value folder.
func TestAccCoralogixDataSourceViewFolder_not_found(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-view-folder-data-missing")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccCoralogixDataSourceViewFolder_read_missing(name),
				ExpectError: regexp.MustCompile(`Could not find view folder with name`),
			},
		},
	})
}

func testAccCoralogixDataSourceViewFolder_read_by_id() string {
	return `data "coralogix_view_folder" "test" {
		id = coralogix_view_folder.test.id
	}
`
}

func testAccCoralogixDataSourceViewFolder_read_by_name() string {
	return `data "coralogix_view_folder" "test" {
		name = coralogix_view_folder.test.name
	}
`
}

func testAccCoralogixDataSourceViewFolder_read_missing(name string) string {
	return fmt.Sprintf(`data "coralogix_view_folder" "test" {
		name = %q
	}
`, name)
}
