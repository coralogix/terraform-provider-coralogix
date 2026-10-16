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
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// The archive targets of logs and of metrics are one per company each, and the API refuses one
// bucket for both. Each archive test touches the other target only when that uses its bucket.
// Deactivating the other target in every run would break the other tests, which can run at the
// same time in another test package.

func testAccArchiveMetricsPreCheck(t *testing.T) {
	testAccPreCheck(t)
	clientSet, err := testAccNewClientSet()
	if err != nil {
		t.Fatalf("error creating the client set: %s", err)
	}
	if err := testAccDisableArchiveMetrics(clientSet); err != nil {
		t.Fatalf("error cleaning the archive metrics target before test: %s", err)
	}
	if err := testAccReleaseArchiveLogsBucket(clientSet, archiveMetricsBucket); err != nil {
		t.Fatalf("error cleaning the archive logs target before test: %s", err)
	}
}

// testAccArchiveLogsPreCheck is testAccArchiveMetricsPreCheck for the archive logs test: it touches
// the metrics target only when that uses the logs bucket.
func testAccArchiveLogsPreCheck(t *testing.T) {
	testAccPreCheck(t)
	clientSet, err := testAccNewClientSet()
	if err != nil {
		t.Fatalf("error creating the client set: %s", err)
	}
	if err := testAccReleaseArchiveMetricsBucket(clientSet, archiveLogsBucket); err != nil {
		t.Fatalf("error cleaning the archive metrics target before test: %s", err)
	}
}

// testAccReleaseArchiveMetricsBucket disables the archive metrics target when it uses bucket, so
// that the archive logs target can use it. Any other metrics target stays as it is.
func testAccReleaseArchiveMetricsBucket(clientSet *clientset.ClientSet, bucket string) error {
	result, httpResponse, err := clientSet.
		ArchiveMetrics().
		MetricsConfiguratorPublicServiceGetTenantConfig(context.Background()).
		Execute()
	if err != nil {
		apiErr := cxsdkOpenapi.NewAPIError(httpResponse, err)
		if cxsdkOpenapi.IsNotFound(apiErr) {
			return nil
		}
		return fmt.Errorf("error reading archive metrics configuration: %w", apiErr)
	}
	config := result.TenantConfig
	if config == nil || config.GetDisabled() || config.S3 == nil || config.S3.GetBucket() != bucket {
		return nil // no S3 target, another bucket, or already disabled
	}
	return testAccDisableArchiveMetrics(clientSet)
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

// testAccDisableArchiveLogs deactivates the archive logs target, whatever its bucket.
func testAccDisableArchiveLogs(clientSet *clientset.ClientSet) error {
	return testAccReleaseArchiveLogsBucket(clientSet, "")
}

// testAccReleaseArchiveLogsBucket deactivates the archive logs target when it uses bucket ("" for
// any bucket), so that the archive metrics target can use it. Any other logs target stays as it is.
func testAccReleaseArchiveLogsBucket(clientSet *clientset.ClientSet, bucket string) error {
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
	if !ok || bucket != "" && s3Target.Bucket != bucket || !result.Target.ArchiveSpec.GetIsActive() {
		return nil // no S3 target, another bucket, or already inactive
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
