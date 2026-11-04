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

package metrics

import (
	"context"

	ams "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/metrics_data_archive_service"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/archivemetrics"
)

// ArchiveMetricsHooks are the handwritten steps of the generated coralogix_archive_metrics.
// A response with neither S3 nor IBM storage is an error, as in the released resource: the
// generated flatten would read it as an archive without storage.
func ArchiveMetricsHooks() archivemetrics.Hooks {
	return archivemetrics.Hooks{BeforeFlatten: requireStorage}
}

func requireStorage(_ context.Context, v *ams.TenantConfigV2) diag.Diagnostics {
	var diags diag.Diagnostics
	if v.S3 == nil && v.Ibm == nil && v.Gcs == nil {
		diags.AddError("Error reading coralogix_archive_metrics",
			"the tenant configuration uses an unsupported storage backend; only IBM and S3 are supported")
	}
	return diags
}

// NewArchiveMetricsResource returns the generated coralogix_archive_metrics.
func NewArchiveMetricsResource() resource.Resource {
	return archivemetrics.NewResource(ArchiveMetricsHooks())
}

// NewArchiveMetricsDataSource returns coralogix_archive_metrics, the generated data source of the
// resource. It reads the one metrics archive of the company and takes no input.
func NewArchiveMetricsDataSource() datasource.DataSource {
	return archivemetrics.NewDataSource(ArchiveMetricsHooks())
}
