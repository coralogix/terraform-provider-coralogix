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

package alerts

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/alert"
)

// NewAlertDataSource returns coralogix_alert, the generated data source of the resource. It runs
// the BeforeFlatten and AfterFlatten hooks of the resource.
func NewAlertDataSource() datasource.DataSource {
	return alert.NewDataSource(alertHooks())
}
