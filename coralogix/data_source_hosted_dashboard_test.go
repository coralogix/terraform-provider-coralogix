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

package coralogix

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var hostedDashboardDataSourceName = "data." + hostedDashboardResourceName

func TestAccCoralogixDataSourceGrafanaDashboard_basic(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	parent := filepath.Dir(wd)
	filePath := parent + "/examples/hosted_dashboard/grafana_acc_dashboard.json"

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: testAccProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceGrafanaDashboard(filePath) +
					testAccCoralogixDataSourceGrafanaDashboard_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(hostedDashboardDataSourceName, "uid"),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceGrafanaDashboard_read() string {
	return `data "coralogix_hosted_dashboard" "test" {
		uid = coralogix_hosted_dashboard.test.id
}
`
}
