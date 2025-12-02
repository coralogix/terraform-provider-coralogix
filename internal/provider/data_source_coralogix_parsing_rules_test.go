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
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var parsingRulesDataSourceName = "data." + parsingRulesGroupResourceName

func TestAccCoralogixDataSourceParsingRules_basic(t *testing.T) {
	r := getRandomParsingRule()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixDataSourceParsingRules_basic(r) +
					testAccCoralogixDataSourceParsingRules_read(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(parsingRulesDataSourceName, "name", r.name),
					resource.TestCheckResourceAttr(parsingRulesDataSourceName, "rule_subgroups.0.rules.0.extract.name", r.parsingRuleParams.name),
				),
			},
		},
	})
}

func testAccCoralogixDataSourceParsingRules_basic(r *parsingRuleGroupParams) string {
	return fmt.Sprintf(`resource "coralogix_parsing_rules" "test" {
  name         = "%s"
  description  = "%s"
  creator      = "%s"
  rule_subgroups = [{
    rules = [{
     	extract = {
			name               = "%s"
			description        = "%s"
			source_field       = "text"
			regular_expression = "(?P<remote_addr>\\d{1,3}.\\d{1,3}.\\d{1,3}.\\d{1,3})\\s*-\\s*(?P<user>[^ ]+)\\s*\\[(?P<timestemp>\\d{4}-\\d{2}\\-\\d{2}T\\d{2}\\:\\d{2}\\:\\d{2}\\.\\d{1,6}Z)\\]\\s*\\\\\\\"(?P<method>[A-z]+)\\s[\\/\\\\]+(?P<request>[^\\s]+)\\s*(?P<protocol>[A-z0-9\\/\\.]+)\\\\\\\"\\s*(?P<status>\\d+)\\s*(?P<body_bytes_sent>\\d+)?\\s*?\\\\\\\"(?P<http_referer>[^\"]+)\\\"\\s*\\\\\\\"(?P<http_user_agent>[^\"]+)\\\"\\s(?P<request_time>\\d{1,6})\\s*(?P<response_time>\\d{1,6})"
    	}
  	}]
  }]
 }
`, r.name, r.description, r.creator, r.parsingRuleParams.name, r.parsingRuleParams.description)
}

func testAccCoralogixDataSourceParsingRules_read() string {
	return `data "coralogix_parsing_rules" "test" {
	id = coralogix_parsing_rules.test.id
}
`
}
