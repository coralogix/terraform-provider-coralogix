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
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var actionDataSourceName = "data." + actionResourceName

func TestAccCoralogixDataSourceAction(t *testing.T) {
	action := actionTestParams{
		name:         acctest.RandomWithPrefix("tf-acc-test"),
		url:          "https://www.google.com/",
		sourceType:   "Logs",
		applications: []string{acctest.RandomWithPrefix("tf-acc-test")},
		subsystems:   []string{acctest.RandomWithPrefix("tf-acc-test")},
		isPrivate:    false,
		isHidden:     false,
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckActionDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceAction(action) +
					testAccCoralogixAction_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(actionDataSourceName, "name", action.name),
				),
			},
		},
	})
}

func testAccCoralogixAction_read() string {
	return `data "coralogix_action" "test" {
             id = coralogix_action.test.id
			}
`
}
