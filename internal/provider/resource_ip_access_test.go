// Copyright 2025 Coralogix Ltd.
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
	_ "embed"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var IpAccessResource = `

resource "coralogix_ip_access" "ip_access" {
    enable_coralogix_customer_support_access = "enabled"
    ip_access = [
      { enabled = true, ip_range = "100.64.0.0/10", name = "random range from wikipedia" }
    ]
}
`

func TestIpAccessResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: IpAccessResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(apiKeyResourceName, "enable_coralogix_customer_support_access", "enabled"),
					resource.TestCheckTypeSetElemNestedAttrs(recordingRulesGroupsSetResourceName, "ip_access.*",
						map[string]string{
							"enabled":  "true",
							"ip_range": "100.64.0.0/10",
							"name":     "random range from wikipedia",
						},
					),
				),
			},
		},
	})
}
