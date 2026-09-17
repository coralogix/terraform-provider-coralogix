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
	description := "runbook for disk pressure"
	// The data source echoes the backend value, so dpxl_filter reads back with
	// the `<v1> ` prefix the API stores rather than the configured form.
	dpxlFilter := "<v1> $d.severity == 'ERROR'"
	urlFields := []actionURLFieldTestParams{
		{name: "env", required: true},
		{name: "trace", required: false},
	}

	action := actionTestParams{
		name:         acctest.RandomWithPrefix("tf-acc-test"),
		url:          "https://www.google.com/search?q={{$p.selected_value}}",
		sourceType:   "Log",
		applications: []string{acctest.RandomWithPrefix("tf-acc-test")},
		subsystems:   []string{acctest.RandomWithPrefix("tf-acc-test")},
		isPrivate:    false,
		isHidden:     false,
		description:  &description,
		dpxlFilter:   &dpxlFilter,
		urlFields:    &urlFields,
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
					resource.TestCheckResourceAttr(actionDataSourceName, "description", description),
					resource.TestCheckResourceAttr(actionDataSourceName, "dpxl_filter", dpxlFilter),
					resource.TestCheckResourceAttr(actionDataSourceName, "url_fields.#", "2"),
					resource.TestCheckResourceAttr(actionDataSourceName, "url_fields.0.name", "env"),
					resource.TestCheckResourceAttr(actionDataSourceName, "url_fields.0.required", "true"),
					resource.TestCheckResourceAttr(actionDataSourceName, "url_fields.1.name", "trace"),
					resource.TestCheckResourceAttr(actionDataSourceName, "url_fields.1.required", "false"),
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
