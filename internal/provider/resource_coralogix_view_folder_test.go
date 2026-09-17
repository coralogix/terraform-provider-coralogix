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
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var (
	viewFolderResourceName = "coralogix_view_folder.test"
	// The validator's own message - the name must never reach the API.
	regexpViewFolderNameLength = regexp.MustCompile(`string length must be between 1 and 100`)
)

// The folder id is server-assigned and must survive a rename, and the name must
// round-trip exactly - the plan has to converge after both steps.
func TestAccCoralogixResourceViewFolder(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-view-folder")
	renamed := acctest.RandomWithPrefix("tf-acc-view-folder-renamed")
	var folderID string

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckViewFolderDestroy,
		Steps: []resource.TestStep{
			// 1. Create round-trip: the name reads back as written and an id is assigned.
			{
				Config: testAccCoralogixResourceViewFolder(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(viewFolderResourceName, "id"),
					resource.TestCheckResourceAttr(viewFolderResourceName, "name", name),
					testAccCaptureViewFolderID(&folderID),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// 2. Rename in place: updated via PUT, never replaced, id unchanged.
			//    An envelope-wrapped PUT body would fail here and nowhere else.
			{
				Config: testAccCoralogixResourceViewFolder(renamed),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(viewFolderResourceName, plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(viewFolderResourceName, "name", renamed),
					testAccCheckViewFolderIDUnchanged(&folderID),
				),
			},
			// 3. Import by id.
			{
				ResourceName:      viewFolderResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// Deleting the folder out of band must make the next refresh drop it from state
// (the 404 -> RemoveResource path) rather than fail the run.
func TestAccCoralogixResourceViewFolder_externalDeletion(t *testing.T) {
	name := acctest.RandomWithPrefix("tf-acc-view-folder-ext-del")
	var folderID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckViewFolderDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceViewFolder(name),
				Check:  testAccCaptureViewFolderID(&folderID),
			},
			{
				PreConfig: func() { testAccDeleteViewFolderOutOfBand(t, &folderID) },
				Config:    testAccCoralogixResourceViewFolder(name),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(viewFolderResourceName, plancheck.ResourceActionCreate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(viewFolderResourceName, "id"),
					resource.TestCheckResourceAttr(viewFolderResourceName, "name", name),
				),
			},
		},
	})
}

// The backend silently truncates a name longer than 100 characters, so the
// validator has to reject it before any API call is made.
func TestAccCoralogixResourceViewFolder_nameTooLong(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccCoralogixResourceViewFolder(strings.Repeat("x", 101)),
				ExpectError: regexpViewFolderNameLength,
			},
		},
	})
}

func testAccCoralogixResourceViewFolder(name string) string {
	return fmt.Sprintf(`resource "coralogix_view_folder" "test" {
			name = %q
		}
`, name)
}

func testAccCaptureViewFolderID(target *string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		id, err := testAccViewFolderIDFromState(state)
		if err != nil {
			return err
		}
		*target = id
		return nil
	}
}

func testAccCheckViewFolderIDUnchanged(previous *string) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		id, err := testAccViewFolderIDFromState(state)
		if err != nil {
			return err
		}
		if id != *previous {
			return fmt.Errorf("view folder id = %q, want the previous step's %q", id, *previous)
		}
		return nil
	}
}

func testAccViewFolderIDFromState(state *terraform.State) (string, error) {
	rs, ok := state.RootModule().Resources[viewFolderResourceName]
	if !ok {
		return "", fmt.Errorf("%s not found in state", viewFolderResourceName)
	}
	if rs.Primary.ID == "" {
		return "", fmt.Errorf("%s has no id in state", viewFolderResourceName)
	}
	return rs.Primary.ID, nil
}

func testAccDeleteViewFolderOutOfBand(t *testing.T, id *string) {
	t.Helper()
	clientSet, err := testAccNewClientSet()
	if err != nil {
		t.Fatalf("out-of-band delete: %s", err)
	}
	if _, httpResponse, err := clientSet.ViewsFolders().
		ViewsFoldersServiceDeleteViewFolder(context.Background(), *id).
		Execute(); err != nil {
		apiErr := cxsdkOpenapi.NewAPIError(httpResponse, err)
		t.Fatalf("out-of-band delete of view folder: %s", utils.FormatOpenAPIErrors(apiErr, "Delete", *id))
	}
}

func testAccCheckViewFolderDestroy(s *terraform.State) error {
	clientSet, err := testAccNewClientSet()
	if err != nil {
		return err
	}
	ctx := context.Background()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_view_folder" {
			continue
		}

		_, httpResponse, err := clientSet.ViewsFolders().
			ViewsFoldersServiceGetViewFolder(ctx, rs.Primary.ID).
			Execute()
		if err == nil {
			return fmt.Errorf("view folder still exists: %s", rs.Primary.ID)
		}

		apiErr := cxsdkOpenapi.NewAPIError(httpResponse, err)
		if cxsdkOpenapi.IsNotFound(apiErr) {
			continue
		}
		return fmt.Errorf("error checking view folder destroy for %s: %s",
			rs.Primary.ID, utils.FormatOpenAPIErrors(apiErr, "Get", rs.Primary.ID))
	}

	return nil
}
