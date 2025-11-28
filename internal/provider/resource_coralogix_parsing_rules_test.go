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
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var parsingRulesGroupResourceName = "coralogix_parsing_rules.test"

type parsingRuleParams struct {
	name, description string
}

type parsingRuleGroupParams struct {
	parsingRuleParams
	name, description, creator string
}

func TestAccCoralogixResourceParsingRules_severities(t *testing.T) {
	var parsingRulesGroupResourceName = "coralogix_parsing_rules.bug_example"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckParsingRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceParsingRulessSeverities(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "name", "Example parse-json-field rule-group from terraform"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "description", "rule_group created by coralogix terraform provider"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "severities.#", "3"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse_json_field.name", "Example parse-json-field rule from terraform"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse_json_field.description", "rule created by coralogix terraform provider"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse_json_field.source_field", "text"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse_json_field.destination_field", "text"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse_json_field.keep_destination_field", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse_json_field.keep_destination_field", "true"),
				),
			},
			{
				ResourceName:      parsingRulesGroupResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceParsingRules_block(t *testing.T) {
	r := getRandomParsingRule()

	keepBlockedLogs := "true"
	regEx := `sql_error_code\\s*=\\s*28000`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckParsingRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceParsingRulesBlock(r, regEx, keepBlockedLogs),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "id"),
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "order"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "active", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "hidden", "false"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "name", r.name),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "creator", r.creator),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "description", r.description),
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.block.id"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.block.order", "1"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.block.active", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.block.name", r.parsingRuleParams.name),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.block.description", r.parsingRuleParams.description),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.block.source_field", "text"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.block.keep_blocked_logs", keepBlockedLogs),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.block.blocking_all_matching_blocks", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.block.regular_expression", "sql_error_code\\s*=\\s*28000"),
				),
			},
			{
				ResourceName:      parsingRulesGroupResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceParsingRules_allow(t *testing.T) {
	r := getRandomParsingRule()

	keepBlockedLogs := "true"
	regEx := `sql_error_code\\s*=\\s*28000`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckParsingRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceParsingRulesAllow(r, regEx, keepBlockedLogs),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "id"),
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "order"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "active", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "hidden", "false"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "name", r.name),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "creator", r.creator),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "description", r.description),
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.block.id"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.block.order", "1"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.block.active", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.block.name", r.parsingRuleParams.name),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.block.description", r.parsingRuleParams.description),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.block.keep_blocked_logs", keepBlockedLogs),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.block.blocking_all_matching_blocks", "false"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.block.regular_expression", "sql_error_code\\s*=\\s*28000"),
				),
			},
			{
				ResourceName:      parsingRulesGroupResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceParsingRules_jsonExtract(t *testing.T) {
	r := getRandomParsingRule()

	jsonKey := "worker"
	destinationField := "Category"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckParsingRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceParsingRulesJsonExtract(r, jsonKey, destinationField),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "id"),
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "order"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "active", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "hidden", "false"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "name", r.name),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "creator", r.creator),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "description", r.description),
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.json_extract.id"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.json_extract.order", "1"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.json_extract.active", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.json_extract.name", r.parsingRuleParams.name),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.json_extract.description", r.parsingRuleParams.description),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.json_extract.destination_field", destinationField),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.json_extract.json_key", jsonKey),
				),
			},
			{
				ResourceName:      parsingRulesGroupResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceParsingRules_replace(t *testing.T) {
	r := getRandomParsingRule()

	regEx := ".*{"
	replacementString := "{"

	resourceName := "coralogix_parsing_rules.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckParsingRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceParsingRulesReplace(r, regEx, replacementString),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "id"),
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "order"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "active", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "hidden", "false"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "name", r.name),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "creator", r.creator),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "description", r.description),
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.replace.id"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.replace.order", "1"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.replace.active", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.replace.name", r.parsingRuleParams.name),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.replace.description", r.parsingRuleParams.description),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.replace.source_field", "text"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.replace.destination_field", "text"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.replace.regular_expression", regEx),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.replace.replacement_string", replacementString),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceParsingRules_extractTimestamp(t *testing.T) {
	r := getRandomParsingRule()

	timeFormat := ""

	fieldFormatStandard := "NanoTS"

	resourceName := "coralogix_parsing_rules.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckParsingRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceParsingRulesExtractTimestamp(r, timeFormat, fieldFormatStandard),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "id"),
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "order"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "active", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "hidden", "false"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "name", r.name),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "creator", r.creator),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "description", r.description),
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.extract_timestamp.id"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.extract_timestamp.order", "1"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.extract_timestamp.active", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.extract_timestamp.name", r.parsingRuleParams.name),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.extract_timestamp.description", r.parsingRuleParams.description),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.extract_timestamp.time_format", timeFormat),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.extract_timestamp.field_format_standard", fieldFormatStandard),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceParsingRules_removeFields(t *testing.T) {
	r := getRandomParsingRule()

	excludedFields := `["coralogix.metadata.applicationName", "coralogix.metadata.className"]`

	resourceName := "coralogix_parsing_rules.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckParsingRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceParsingRulesRemoveFields(r, excludedFields),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "id"),
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "order"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "active", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "hidden", "false"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "name", r.name),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "creator", r.creator),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "description", r.description),
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.remove_fields.id"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.remove_fields.order", "1"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.remove_fields.active", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.remove_fields.name", r.parsingRuleParams.name),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.remove_fields.description", r.parsingRuleParams.description),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.remove_fields.excluded_fields.0", "coralogix.metadata.applicationName"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.remove_fields.excluded_fields.1", "coralogix.metadata.className"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceParsingRules_jsonStringify(t *testing.T) {
	r := getRandomParsingRule()

	keepSourceField := "true"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckParsingRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceParsingRulesJsonStringify(r, keepSourceField),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "id"),
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "order"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "active", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "hidden", "false"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "name", r.name),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "creator", r.creator),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "description", r.description),
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.json_stringify.id"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.json_stringify.order", "1"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.json_stringify.active", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.json_stringify.name", r.parsingRuleParams.name),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.json_stringify.description", r.parsingRuleParams.description),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.json_stringify.source_field", "text"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.json_stringify.destination_field", "text"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.json_stringify.keep_source_field", keepSourceField),
				),
			},
			{
				ResourceName:      parsingRulesGroupResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceParsingRules_extract(t *testing.T) {
	r := getRandomParsingRule()

	regEx := `\\b(?P<severity>DEBUG|TRACE|INFO|WARN|ERROR|FATAL|EXCEPTION|[Dd]ebug|[Tt]race|[Ii]nfo|[Ww]arn|[Ee]rror|[Ff]atal|[Ee]xception)\\b`

	resourceName := "coralogix_parsing_rules.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckParsingRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceParsingRulesExtract(r, regEx),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "id"),
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "order"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "active", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "hidden", "false"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "name", r.name),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "creator", r.creator),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "description", r.description),
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.extract.id"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.extract.order", "1"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.extract.active", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.extract.name", r.parsingRuleParams.name),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.extract.description", r.parsingRuleParams.description),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.extract.source_field", "text"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.extract.regular_expression",
						"\\b(?P<severity>DEBUG|TRACE|INFO|WARN|ERROR|FATAL|EXCEPTION|[Dd]ebug|[Tt]race|[Ii]nfo|[Ww]arn|[Ee]rror|[Ff]atal|[Ee]xception)\\b"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceParsingRules_parse(t *testing.T) {
	r := getRandomParsingRule()

	regEx := `(?P<remote_addr>\\d{1,3}.\\d{1,3}.\\d{1,3}.\\d{1,3})\\s*-\\s*(?P<user>[^ ]+)\\s*\\[(?P<timestemp>\\d{4}-\\d{2}\\-\\d{2}T\\d{2}\\:\\d{2}\\:\\d{2}\\.\\d{1,6}Z)\\]\\s*\\\\\\\"(?P<method>[A-z]+)\\s[\\/\\\\]+(?P<request>[^\\s]+)\\s*(?P<protocol>[A-z0-9\\/\\.]+)\\\\\\\"\\s*(?P<status>\\d+)\\s*(?P<body_bytes_sent>\\d+)?\\s*?\\\\\\\"(?P<http_referer>[^\"]+)\\\"\\s*\\\\\\\"(?P<http_user_agent>[^\"]+)\\\"\\s(?P<request_time>\\d{1,6})\\s*(?P<response_time>\\d{1,6})`

	resourceName := "coralogix_parsing_rules.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckParsingRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceParsingRulesParse(r, regEx),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "id"),
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "order"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "active", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "hidden", "false"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "name", r.name),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "creator", r.creator),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "description", r.description),
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse.id"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse.order", "1"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse.active", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse.name", r.parsingRuleParams.name),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse.description", r.parsingRuleParams.description),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse.source_field", "text"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse.destination_field", "text"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse.regular_expression",
						"(?P<remote_addr>\\d{1,3}.\\d{1,3}.\\d{1,3}.\\d{1,3})\\s*-\\s*(?P<user>[^ ]+)\\s*\\[(?P<timestemp>\\d{4}-\\d{2}\\-\\d{2}T\\d{2}\\:\\d{2}\\:\\d{2}\\.\\d{1,6}Z)\\]\\s*\\\\\\\"(?P<method>[A-z]+)\\s[\\/\\\\]+(?P<request>[^\\s]+)\\s*(?P<protocol>[A-z0-9\\/\\.]+)\\\\\\\"\\s*(?P<status>\\d+)\\s*(?P<body_bytes_sent>\\d+)?\\s*?\\\\\\\"(?P<http_referer>[^\"]+)\\\"\\s*\\\\\\\"(?P<http_user_agent>[^\"]+)\\\"\\s(?P<request_time>\\d{1,6})\\s*(?P<response_time>\\d{1,6})"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceParsingRules_parseJsonField(t *testing.T) {
	r := getRandomParsingRule()
	keepSourceField := utils.SelectRandomlyFromSlice([]string{"true", "false"})
	keepDestinationField := utils.SelectRandomlyFromSlice([]string{"true", "false"})
	resourceName := "coralogix_parsing_rules.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckParsingRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceParsingRulesParseJsonField(r, keepSourceField, keepDestinationField),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "id"),
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "order"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "active", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "hidden", "false"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "name", r.name),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "creator", r.creator),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "description", r.description),
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse_json_field.id"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse_json_field.order", "1"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse_json_field.active", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse_json_field.name", r.parsingRuleParams.name),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse_json_field.description", r.parsingRuleParams.description),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse_json_field.source_field", "text"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse_json_field.destination_field", "text"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse_json_field.keep_source_field", keepSourceField),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse_json_field.keep_destination_field", keepDestinationField),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceParsingRules_rules_combination(t *testing.T) {
	r := getRandomParsingRule()
	resourceName := "coralogix_parsing_rules.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckParsingRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceParsingRulesCombination(r),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "id"),
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "order"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "active", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "hidden", "false"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "name", r.name),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "creator", r.creator),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "description", r.description),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.#", "3"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse.name", "rule1"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse.order", "1"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.1.extract.name", "rule2"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.1.extract.order", "2"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.2.parse.name", "rule3"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.2.parse.order", "3"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.1.rules.0.extract_timestamp.name", "rule1"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccCoralogixResourceParsingRules_update(t *testing.T) {
	r1 := getRandomParsingRule()
	r2 := getRandomParsingRule()
	resourceName := "coralogix_parsing_rules.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckParsingRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceParsingRulesCombination(r1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "id"),
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "order"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "active", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "hidden", "false"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "name", r1.name),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "creator", r1.creator),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "description", r1.description),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.#", "3"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse.name", "rule1"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.1.extract.name", "rule2"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.2.parse.name", "rule3"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.1.rules.0.extract_timestamp.name", "rule1"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccCoralogixResourceParsingRulesCombination(r2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "id"),
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "order"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "active", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "hidden", "false"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "name", r2.name),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "creator", r2.creator),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "description", r2.description),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.#", "3"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse.name", "rule1"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.1.extract.name", "rule2"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.2.parse.name", "rule3"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.1.rules.0.extract_timestamp.name", "rule1"),
				),
			},
		},
	})
}

func TestAccCoralogixResourceParsingRules_update_order_inside_rule_group(t *testing.T) {
	r := getRandomParsingRule()
	resourceName := "coralogix_parsing_rules.test"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckParsingRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceParsingRulesCombination(r),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "id"),
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "order"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "active", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "hidden", "false"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "name", r.name),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "creator", r.creator),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "description", r.description),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.#", "3"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse.name", "rule1"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.parse.order", "1"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.1.extract.name", "rule2"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.1.extract.order", "2"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.2.parse.name", "rule3"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.2.parse.order", "3"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.1.rules.0.extract_timestamp.name", "rule1"),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccCoralogixResourceParsingRulesCombinationDifferentOrders(r),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "id"),
					resource.TestCheckResourceAttrSet(parsingRulesGroupResourceName, "order"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "active", "true"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "hidden", "false"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "name", r.name),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "creator", r.creator),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "description", r.description),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.#", "3"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.extract.name", "rule2"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.0.extract.order", "1"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.1.parse.name", "rule3"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.1.parse.order", "2"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.2.parse.name", "rule1"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.0.rules.2.parse.order", "3"),
					resource.TestCheckResourceAttr(parsingRulesGroupResourceName, "rule_subgroups.1.rules.0.extract_timestamp.name", "rule1"),
				),
			},
		},
	})
}

func TestAccCoralogixResourceParsingRulesOrder(t *testing.T) {
	firstRuleGroupOrder := acctest.RandIntRange(1, 2)
	secondRuleGroupOrder := 2
	if firstRuleGroupOrder == 2 {
		secondRuleGroupOrder = 1
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckParsingRuleDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccCoralogixResourceParsingRulesGroupsOrders(firstRuleGroupOrder, secondRuleGroupOrder),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("coralogix_parsing_rules.test1", "order", strconv.Itoa(firstRuleGroupOrder)),
					resource.TestCheckResourceAttr("coralogix_parsing_rules.test2", "order", strconv.Itoa(secondRuleGroupOrder)),
				),
			},
		},
	})
}

func getRandomParsingRule() *parsingRuleGroupParams {
	return &parsingRuleGroupParams{
		name:        acctest.RandomWithPrefix("tf-acc-test"),
		description: acctest.RandomWithPrefix("tf-acc-test"),
		creator:     acctest.RandomWithPrefix("tf-acc-test"),
		parsingRuleParams: parsingRuleParams{
			name:        acctest.RandomWithPrefix("tf-acc-test"),
			description: acctest.RandomWithPrefix("tf-acc-test"),
		},
	}
}

func testAccCheckParsingRuleDestroy(s *terraform.State) error {
	client := testAccProvider.Meta().(*clientset.ClientSet).ParsingRuleGroups()

	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_parsing_rules" {
			continue
		}

		resp, _, err := client.RuleGroupsServiceGetRuleGroup(ctx, rs.Primary.ID).Execute()
		if err == nil {
			if *resp.RuleGroup.Id == rs.Primary.ID {
				return fmt.Errorf("RuleGroup still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

/*func testAccCoralogixResourceParsingRulesMinimal(name string) string {
    return fmt.Sprintf(`resource "coralogix_parsing_rules" "test" {
  name         = "%s"
  description  = "rule group from terraform provider"
 }
`, name)
}*/

func testAccCoralogixResourceParsingRulesBlock(r *parsingRuleGroupParams, regEx, keepBlockedLogs string) string {
	return fmt.Sprintf(`resource "coralogix_parsing_rules" "test" {
  name         = "%s"
  description  = "%s"
  creator      = "%s"
  rule_subgroups = [{
    rules = [{
        block = {
            name               	= "%s"
            description        	= "%s"
            source_field 		= "text"
            regular_expression	= "%s"
            keep_blocked_logs  	= "%s"
        }
    }]
  }]
 }
`, r.name, r.description, r.creator, r.parsingRuleParams.name, r.parsingRuleParams.description, regEx, keepBlockedLogs)
}

func testAccCoralogixResourceParsingRulesJsonExtract(r *parsingRuleGroupParams, jsonKey, destinationField string) string {
	return fmt.Sprintf(`resource "coralogix_parsing_rules" "test" {
  name         = "%s"
  description  = "%s"
  creator      = "%s"
  rule_subgroups = [{
    rules = [{
        json_extract = {
            name               	= "%s"
            description        	= "%s"
            json_key     		= "%s"
            destination_field  	= "%s"
        }
    }]
  }]
 }
`, r.name, r.description, r.creator, r.parsingRuleParams.name, r.parsingRuleParams.description, jsonKey, destinationField)
}

func testAccCoralogixResourceParsingRulesReplace(r *parsingRuleGroupParams, regEx, replacementString string) string {
	return fmt.Sprintf(`resource "coralogix_parsing_rules" "test" {
  name         = "%s"
  description  = "%s"
  creator      = "%s"
  rule_subgroups = [{
    rules = [{
        replace = {
            name               	= "%s"
            description        	= "%s"
            source_field       	= "text"
            destination_field  	= "text"
            regular_expression	= "%s"
            replacement_string  = "%s"
        }
    }
  }
 }
`, r.name, r.description, r.creator, r.parsingRuleParams.name, r.parsingRuleParams.description, regEx, replacementString)
}

func testAccCoralogixResourceParsingRulesAllow(r *parsingRuleGroupParams, regEx, keepBlockedLogs string) string {
	return fmt.Sprintf(`resource "coralogix_parsing_rules" "test" {
  name         = "%s"
  description  = "%s"
  creator      = "%s"
  rule_subgroups = [{
    rules = [{
        block = {
            name               	= "%s"
            description        	= "%s"
            source_field 			= "text"
            regular_expression	= "%s"
            keep_blocked_logs     = "%s"
            blocking_all_matching_blocks = false
        }
   }]
  }]
 }
`, r.name, r.description, r.creator, r.parsingRuleParams.name, r.parsingRuleParams.description, regEx, keepBlockedLogs)
}

func testAccCoralogixResourceParsingRulesExtractTimestamp(r *parsingRuleGroupParams, timeFormat, fieldFormatStandard string) string {
	return fmt.Sprintf(`resource "coralogix_parsing_rules" "test" {
name         = "%s"
description  = "%s"
creator      = "%s"
rule_subgroups = [{
    rules = [{
        extract_timestamp = {
            name                  = "%s"
            description           = "%s"
            source_field          = "text"
            time_format        	  = "%s"
            field_format_standard = "%s"
        }
    }]
}]
}
`, r.name, r.description, r.creator, r.parsingRuleParams.name, r.parsingRuleParams.description, timeFormat, fieldFormatStandard)
}

func testAccCoralogixResourceParsingRulesRemoveFields(r *parsingRuleGroupParams, excludedFields string) string {
	return fmt.Sprintf(`resource "coralogix_parsing_rules" "test" {
  name         = "%s"
  description  = "%s"
  creator      = "%s"
  rule_subgroups = [{
    rules = [{
        remove_fields = {
            name               = "%s"
            description        = "%s"
            excluded_fields    = %s
        }
   }]
  }]
 }
`, r.name, r.description, r.creator, r.parsingRuleParams.name, r.parsingRuleParams.description, excludedFields)
}

func testAccCoralogixResourceParsingRulesJsonStringify(r *parsingRuleGroupParams, keepSourceField string) string {
	return fmt.Sprintf(`resource "coralogix_parsing_rules" "test" {
  name         = "%s"
  description  = "%s"
  creator      = "%s"
  rule_subgroups = [{
    rules = [{
        json_stringify = {
            name               = "%s"
            description        = "%s"
            source_field       = "text"
            destination_field  = "text"
            keep_source_field  = "%s"
        }   
   }]
  }]
 }
`, r.name, r.description, r.creator, r.parsingRuleParams.name, r.parsingRuleParams.description, keepSourceField)
}

func testAccCoralogixResourceParsingRulesExtract(r *parsingRuleGroupParams, regEx string) string {
	return fmt.Sprintf(`resource "coralogix_parsing_rules" "test" {
  name         = "%s"
  description  = "%s"
  creator      = "%s"
   rule_subgroups = [{
    rules = [{
        extract {
            name               = "%s"
            description        = "%s"
            source_field       = "text"
            regular_expression = "%s"
        }
    }]
  }]
 }
`, r.name, r.description, r.creator, r.parsingRuleParams.name, r.parsingRuleParams.description, regEx)
}

func testAccCoralogixResourceParsingRulesParse(r *parsingRuleGroupParams, regEx string) string {
	return fmt.Sprintf(`resource "coralogix_parsing_rules" "test" {
  name         = "%s"
  description  = "%s"
  creator      = "%s"
  rule_subgroups = [{
    rules = [{
        parse = {
            name               = "%s"
            description        = "%s"
            source_field       = "text"
            destination_field  = "text"
            regular_expression  = "%s"
        }
    }]
  }]
 }
`, r.name, r.description, r.creator, r.parsingRuleParams.name, r.parsingRuleParams.description, regEx)
}

func testAccCoralogixResourceParsingRulesParseJsonField(r *parsingRuleGroupParams, keepSourceField, keepDestinationField string) string {
	return fmt.Sprintf(`resource "coralogix_parsing_rules" "test" {
  name         = "%s"
  description  = "%s"
  creator      = "%s"
  rule_subgroups = [{
    rules = [{
        parse_json_field {
            name               = "%s"
            description        = "%s"
            source_field       = "text"
            destination_field  = "text"
            keep_source_field  = "%s"
            keep_destination_field = "%s"
        }   
    }]
  }]
 }
`, r.name, r.description, r.creator, r.parsingRuleParams.name, r.parsingRuleParams.description, keepSourceField, keepDestinationField)
}

func testAccCoralogixResourceParsingRulesCombination(r *parsingRuleGroupParams) string {
	return fmt.Sprintf(`resource "coralogix_parsing_rules" "test" {
  name         = "%s"
  description  = "%s"
  creator      = "%s"
  rule_subgroups = [{
    rules = [{
        parse = {
            name               = "rule1"
            description        = "description"
            source_field       = "text"
            destination_field  = "text"
            regular_expression  = "(?P<remote_addr>\\d{1,3}.\\d{1,3}.\\d{1,3}.\\d{1,3})\\s*-\\s*(?P<user>[^ ]+)\\s*\\[(?P<timestemp>\\d{4}-\\d{2}\\-\\d{2}T\\d{2}\\:\\d{2}\\:\\d{2}\\.\\d{1,6}Z)\\]\\s*\\\\\\\"(?P<method>[A-z]+)\\s[\\/\\\\]+(?P<request>[^\\s]+)\\s*(?P<protocol>[A-z0-9\\/\\.]+)\\\\\\\"\\s*(?P<status>\\d+)\\s*(?P<body_bytes_sent>\\d+)?\\s*?\\\\\\\"(?P<http_referer>[^\"]+)\\\"\\s*\\\\\\\"(?P<http_user_agent>[^\"]+)\\\"\\s(?P<request_time>\\d{1,6})\\s*(?P<response_time>\\d{1,6})"
        }
    },{
        extract = {
            name               = "rule2"
            description        = "description"
            source_field       = "text"
            regular_expression  = "(?P<remote_addr>\\d{1,3}.\\d{1,3}.\\d{1,3}.\\d{1,3})\\s*-\\s*(?P<user>[^ ]+)\\s*\\[(?P<timestemp>\\d{4}-\\d{2}\\-\\d{2}T\\d{2}\\:\\d{2}\\:\\d{2}\\.\\d{1,6}Z)\\]\\s*\\\\\\\"(?P<method>[A-z]+)\\s[\\/\\\\]+(?P<request>[^\\s]+)\\s*(?P<protocol>[A-z0-9\\/\\.]+)\\\\\\\"\\s*(?P<status>\\d+)\\s*(?P<body_bytes_sent>\\d+)?\\s*?\\\\\\\"(?P<http_referer>[^\"]+)\\\"\\s*\\\\\\\"(?P<http_user_agent>[^\"]+)\\\"\\s(?P<request_time>\\d{1,6})\\s*(?P<response_time>\\d{1,6})"
        }
    },{
        parse = {
            name               = "rule3"
            description        = "description"
            source_field       = "text"
                destination_field  = "text"
            regular_expression  = "(?P<remote_addr>\\d{1,3}.\\d{1,3}.\\d{1,3}.\\d{1,3})\\s*-\\s*(?P<user>[^ ]+)\\s*\\[(?P<timestemp>\\d{4}-\\d{2}\\-\\d{2}T\\d{2}\\:\\d{2}\\:\\d{2}\\.\\d{1,6}Z)\\]\\s*\\\\\\\"(?P<method>[A-z]+)\\s[\\/\\\\]+(?P<request>[^\\s]+)\\s*(?P<protocol>[A-z0-9\\/\\.]+)\\\\\\\"\\s*(?P<status>\\d+)\\s*(?P<body_bytes_sent>\\d+)?\\s*?\\\\\\\"(?P<http_referer>[^\"]+)\\\"\\s*\\\\\\\"(?P<http_user_agent>[^\"]+)\\\"\\s(?P<request_time>\\d{1,6})\\s*(?P<response_time>\\d{1,6})"
        }
    },{
        extract_timestamp = {
            name               	= "rule1"
            description        	= "description"
            source_field 			= "text"
            time_format        	= "2006-01-02T15:04:05.999999999Z07:00"
            field_format_standard = "Golang"
        }
   }
  }
 }
`, r.name, r.description, r.creator)
}

func testAccCoralogixResourceParsingRulesCombinationDifferentOrders(r *parsingRuleGroupParams) string {
	return fmt.Sprintf(`resource "coralogix_parsing_rules" "test" {
  name         = "%s"
  description  = "%s"
  creator      = "%s"
  rule_subgroups = [{
    rules = [{
        extract = {
            name               = "rule2"
            description        = "description"
            source_field       = "text"
            regular_expression  = "(?P<remote_addr>\\d{1,3}.\\d{1,3}.\\d{1,3}.\\d{1,3})\\s*-\\s*(?P<user>[^ ]+)\\s*\\[(?P<timestemp>\\d{4}-\\d{2}\\-\\d{2}T\\d{2}\\:\\d{2}\\:\\d{2}\\.\\d{1,6}Z)\\]\\s*\\\\\\\"(?P<method>[A-z]+)\\s[\\/\\\\]+(?P<request>[^\\s]+)\\s*(?P<protocol>[A-z0-9\\/\\.]+)\\\\\\\"\\s*(?P<status>\\d+)\\s*(?P<body_bytes_sent>\\d+)?\\s*?\\\\\\\"(?P<http_referer>[^\"]+)\\\"\\s*\\\\\\\"(?P<http_user_agent>[^\"]+)\\\"\\s(?P<request_time>\\d{1,6})\\s*(?P<response_time>\\d{1,6})"
        }
    }, {
        parse = {
            name               = "rule3"
            description        = "description"
            source_field       = "text"
                destination_field  = "text"
            regular_expression  = "(?P<remote_addr>\\d{1,3}.\\d{1,3}.\\d{1,3}.\\d{1,3})\\s*-\\s*(?P<user>[^ ]+)\\s*\\[(?P<timestemp>\\d{4}-\\d{2}\\-\\d{2}T\\d{2}\\:\\d{2}\\:\\d{2}\\.\\d{1,6}Z)\\]\\s*\\\\\\\"(?P<method>[A-z]+)\\s[\\/\\\\]+(?P<request>[^\\s]+)\\s*(?P<protocol>[A-z0-9\\/\\.]+)\\\\\\\"\\s*(?P<status>\\d+)\\s*(?P<body_bytes_sent>\\d+)?\\s*?\\\\\\\"(?P<http_referer>[^\"]+)\\\"\\s*\\\\\\\"(?P<http_user_agent>[^\"]+)\\\"\\s(?P<request_time>\\d{1,6})\\s*(?P<response_time>\\d{1,6})"
        }
    }, {
        parse = {
            name               = "rule1"
            description        = "description"
            source_field       = "text"
            destination_field  = "text"
            regular_expression  = "(?P<remote_addr>\\d{1,3}.\\d{1,3}.\\d{1,3}.\\d{1,3})\\s*-\\s*(?P<user>[^ ]+)\\s*\\[(?P<timestemp>\\d{4}-\\d{2}\\-\\d{2}T\\d{2}\\:\\d{2}\\:\\d{2}\\.\\d{1,6}Z)\\]\\s*\\\\\\\"(?P<method>[A-z]+)\\s[\\/\\\\]+(?P<request>[^\\s]+)\\s*(?P<protocol>[A-z0-9\\/\\.]+)\\\\\\\"\\s*(?P<status>\\d+)\\s*(?P<body_bytes_sent>\\d+)?\\s*?\\\\\\\"(?P<http_referer>[^\"]+)\\\"\\s*\\\\\\\"(?P<http_user_agent>[^\"]+)\\\"\\s(?P<request_time>\\d{1,6})\\s*(?P<response_time>\\d{1,6})"
        }
    }, {
        extract_timestamp = {
            name               	= "rule1"
            description        	= "description"
            source_field 			= "text"
            time_format        	= "2006-01-02T15:04:05.999999999Z07:00"
            field_format_standard = "Golang"
        }
   }]
  }]
 }
`, r.name, r.description, r.creator)
}

func testAccCoralogixResourceParsingRulesGroupsOrders(firstRuleGroupOrder, secondRuleGroupOrder int) string {
	return fmt.Sprintf(`resource "coralogix_parsing_rules" "test1" {
  name         = "name1"
  description  = "description1"
  creator      = "creator1"
 order = %d
   rule_subgroups = [{
    rules = [{
        extract = {
            name               = "rule2"
            description        = "description"
            source_field       = "text"
            regular_expression  = "(?P<remote_addr>\\d{1,3}.\\d{1,3}.\\d{1,3}.\\d{1,3})\\s*-\\s*(?P<user>[^ ]+)\\s*\\[(?P<timestemp>\\d{4}-\\d{2}\\-\\d{2}T\\d{2}\\:\\d{2}\\:\\d{2}\\.\\d{1,6}Z)\\]\\s*\\\\\\\"(?P<method>[A-z]+)\\s[\\/\\\\]+(?P<request>[^\\s]+)\\s*(?P<protocol>[A-z0-9\\/\\.]+)\\\\\\\"\\s*(?P<status>\\d+)\\s*(?P<body_bytes_sent>\\d+)?\\s*?\\\\\\\"(?P<http_referer>[^\"]+)\\\"\\s*\\\\\\\"(?P<http_user_agent>[^\"]+)\\\"\\s(?P<request_time>\\d{1,6})\\s*(?P<response_time>\\d{1,6})"
        }
    }]
  }]
}

resource "coralogix_parsing_rules" "test2" {
  name         = "name2"
  description  = "description2"
  creator      = "creator2"
  order = %d
  rule_subgroups = [{
    rules = [{
        extract = {
            name               = "rule2"
            description        = "description"
            source_field       = "text"
            regular_expression  = "(?P<remote_addr>\\d{1,3}.\\d{1,3}.\\d{1,3}.\\d{1,3})\\s*-\\s*(?P<user>[^ ]+)\\s*\\[(?P<timestemp>\\d{4}-\\d{2}\\-\\d{2}T\\d{2}\\:\\d{2}\\:\\d{2}\\.\\d{1,6}Z)\\]\\s*\\\\\\\"(?P<method>[A-z]+)\\s[\\/\\\\]+(?P<request>[^\\s]+)\\s*(?P<protocol>[A-z0-9\\/\\.]+)\\\\\\\"\\s*(?P<status>\\d+)\\s*(?P<body_bytes_sent>\\d+)?\\s*?\\\\\\\"(?P<http_referer>[^\"]+)\\\"\\s*\\\\\\\"(?P<http_user_agent>[^\"]+)\\\"\\s(?P<request_time>\\d{1,6})\\s*(?P<response_time>\\d{1,6})"
        }
    }]
  }]
}
`, firstRuleGroupOrder, secondRuleGroupOrder)
}

func testAccCoralogixResourceParsingRulessSeverities() string {
	return `resource "coralogix_parsing_rules" "bug_example" {
  name         = "Example parse-json-field rule-group from terraform"
  description  = "rule_group created by coralogix terraform provider"
  applications = ["test"]
  subsystems   = ["example"]
  order = 1
  severities =  ["critical", "debug", "error"]
  rule_subgroups = [{
    rules = [{
        parse_json_field = {
          name                   = "Example parse-json-field rule from terraform"
          description            = "rule created by coralogix terraform provider"
          source_field           = "text"
          destination_field      = "text"
          keep_source_field      = "true"
          keep_destination_field = "true"
        }
    }]
  }] 
}`

}
