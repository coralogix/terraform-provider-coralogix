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
	archiveLogs "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/target_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var (
	archiveLogsResourceName = "coralogix_archive_logs.test"
)

func TestAccCoralogixResourceResourceArchiveLogs(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccArchivePreCheck(t) },
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

func testAccArchivePreCheck(t *testing.T) {
	testAccPreCheck(t)
	if err := testAccCleanupArchiveTargets(); err != nil {
		t.Fatalf("error cleaning archive targets before test: %s", err)
	}
}

func testAccCleanupArchiveTargets() error {
	clientSet, err := testAccNewClientSet()
	if err != nil {
		return err
	}

	if err := testAccDisableArchiveMetrics(clientSet); err != nil {
		return err
	}
	return testAccDisableArchiveLogs(clientSet)
}

func testAccDisableArchiveMetrics(clientSet *clientset.ClientSet) error {
	_, httpResponse, err := clientSet.
		ArchiveMetrics().
		MetricsConfiguratorPublicServiceDisableArchive(context.Background()).
		Execute()
	if err != nil && !cxsdkOpenapi.IsNotFound(cxsdkOpenapi.NewAPIError(httpResponse, err)) {
		return fmt.Errorf("error deactivating archive metrics configuration: %w", cxsdkOpenapi.NewAPIError(httpResponse, err))
	}
	return nil
}

func testAccDisableArchiveLogs(clientSet *clientset.ClientSet) error {
	result, httpResponse, err := clientSet.
		ArchiveLogs().
		S3TargetServiceGetTarget(context.Background()).
		Execute()
	if err != nil {
		apiErr := cxsdkOpenapi.NewAPIError(httpResponse, err)
		if cxsdkOpenapi.IsNotFound(apiErr) {
			return nil
		}
		return fmt.Errorf("error reading archive logs target: %w", apiErr)
	}

	s3Target, ok := result.Target.GetS3Ok()
	if !ok {
		return fmt.Errorf("archive logs target is not an S3 target")
	}

	_, httpResponse, err = clientSet.
		ArchiveLogs().
		S3TargetServiceSetTarget(context.Background()).
		SetTargetResponse(archiveLogs.SetTargetResponse{
			IsActive: false,
			S3:       *s3Target,
		}).
		Execute()
	if err != nil && !cxsdkOpenapi.IsNotFound(cxsdkOpenapi.NewAPIError(httpResponse, err)) {
		return fmt.Errorf("error deactivating archive logs target: %w", cxsdkOpenapi.NewAPIError(httpResponse, err))
	}
	return nil
}

func testAccCheckArchiveLogsDestroy(_ *terraform.State) error {
	clientSet, err := testAccNewClientSet()
	if err != nil {
		return err
	}
	if err := testAccDisableArchiveLogs(clientSet); err != nil {
		return fmt.Errorf("error deactivating archive logs target after destroy: %w", err)
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

func testAccCheckArchiveMetricsDestroy(_ *terraform.State) error {
	clientSet, err := testAccNewClientSet()
	if err != nil {
		return err
	}
	if err := testAccDisableArchiveMetrics(clientSet); err != nil {
		return fmt.Errorf("error deactivating archive metrics configuration after destroy: %w", err)
	}

	result, httpResponse, err := clientSet.
		ArchiveMetrics().
		MetricsConfiguratorPublicServiceGetTenantConfig(context.Background()).
		Execute()
	if err != nil {
		apiErr := cxsdkOpenapi.NewAPIError(httpResponse, err)
		if cxsdkOpenapi.IsNotFound(apiErr) {
			return nil
		}
		return fmt.Errorf("error reading archive metrics configuration after destroy: %w", apiErr)
	}

	if result.TenantConfig != nil && !result.TenantConfig.GetDisabled() {
		return fmt.Errorf("archive metrics target is still active after destroy")
	}
	return nil
}
