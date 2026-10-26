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

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/alertsscheduler"
)

// NewAlertsSchedulerDataSource returns coralogix_alerts_scheduler, the generated data source of the
// resource. It runs the BeforeFlatten hook of the resource.
func NewAlertsSchedulerDataSource() datasource.DataSource {
	return alertsscheduler.NewDataSource(schedulerHooks)
}
