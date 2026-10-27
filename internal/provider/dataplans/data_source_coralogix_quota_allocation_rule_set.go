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

package dataplans

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/quotaallocationruleset"
)

// NewQuotaAllocationRuleSetDataSource returns coralogix_quota_allocation_rule_set, the generated
// data source of the resource. It keeps the rules that Coralogix manages (cx_managed), which the
// resource drops from its state, and reads an unspecified allocation type as percentage, as the
// released data source did.
func NewQuotaAllocationRuleSetDataSource() datasource.DataSource {
	return quotaallocationruleset.NewDataSource(quotaallocationruleset.Hooks{BeforeFlatten: normalizeQuotaAllocationTypes})
}
