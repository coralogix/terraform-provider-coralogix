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

package provider

import (
	"context"
	"fmt"
	"testing"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var (
	archiveLogsResourceName = "coralogix_archive_logs.test"
)

func TestAccCoralogixResourceResourceArchiveLogs(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckArchiveLogsDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceArchiveLogs(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(archiveLogsResourceName, "bucket", "yak-coralogix-bucket"),
					resource.TestCheckResourceAttr(archiveLogsResourceName, "active", "true"),
				),
			},
			{
				ResourceName:      archiveLogsResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccCoralogixResourceArchiveLogsUpdate(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(archiveLogsResourceName, "bucket", "yak-coralogix-bucket"),
					resource.TestCheckResourceAttr(archiveLogsResourceName, "active", "false"),
				),
			},
		},
	})
}

func testAccCheckArchiveLogsDestroy(_ *terraform.State) error {
	clientSet, err := testAccNewClientSet()
	if err != nil {
		return err
	}

	result, httpResponse, err := clientSet.
		ArchiveLogs().
		S3TargetServiceGetTarget(context.Background()).
		Execute()
	if err != nil {
		apiErr := cxsdkOpenapi.NewAPIError(httpResponse, err)
		if cxsdkOpenapi.IsNotFound(apiErr) {
			return nil
		}
		return fmt.Errorf("error reading archive logs target after destroy: %w", apiErr)
	}

	archiveSpec := result.Target.GetArchiveSpec()
	if archiveSpec.GetIsActive() {
		return fmt.Errorf("archive logs target is still active after destroy")
	}
	return nil
}

func testAccCoralogixResourceArchiveLogs() string {
	return `resource "coralogix_archive_logs" "test" {
 	bucket = "yak-coralogix-bucket"
	region = "eu-north-1"
}
`
}

func testAccCoralogixResourceArchiveLogsUpdate() string {
	return `resource "coralogix_archive_logs" "test" {
  		bucket = "yak-coralogix-bucket"
 		active = false
}
`
}
