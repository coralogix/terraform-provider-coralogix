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

package slo_mgmt

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/slov2"
)

// NewSLOV2DataSource returns coralogix_slo_v2, the generated data source of the resource. It reads
// an older record as the resource does.
func NewSLOV2DataSource() datasource.DataSource {
	return slov2.NewDataSource(slov2.Hooks{BeforeFlatten: normalizeLegacySLO})
}
