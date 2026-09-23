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
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

var actionResourceName = "coralogix_action.test"

type actionURLFieldTestParams struct {
	name     string
	required bool
}

// A nil description/dpxlFilter/urlFields omits the attribute from the config,
// which is how the clear-on-omit contract is exercised.
type actionTestParams struct {
	name, url, sourceType    string
	applications, subsystems []string
	isPrivate, isHidden      bool
	description, dpxlFilter  *string
	urlFields                *[]actionURLFieldTestParams
}

func TestAccCoralogixResourceAction(t *testing.T) {
	// The prefixed form of dpxl_filter is what the backend stores, so
	// ImportStateVerify below can compare state against config directly.
	description := "runbook for disk pressure"
	dpxlFilter := "<v1> $d.severity == 'ERROR'"
	urlFields := []actionURLFieldTestParams{
		{name: "env", required: true},
		{name: "trace", required: false},
	}

	action := actionTestParams{
		name:         "google search action",
		url:          "https://www.google.com/",
		sourceType:   "Log",
		applications: []string{acctest.RandomWithPrefix("tf-acc-test")},
		subsystems:   []string{acctest.RandomWithPrefix("tf-acc-test")},
		isPrivate:    false,
		isHidden:     false,
		description:  &description,
		dpxlFilter:   &dpxlFilter,
		urlFields:    &urlFields,
	}

	updatedAction := actionTestParams{
		name:         "bing search action",
		url:          "https://www.bing.com/search?q={{$p.selected_value}}",
		sourceType:   "DataMap",
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
				Config: testAccCoralogixResourceAction(action),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(actionResourceName, "id"),
					resource.TestCheckResourceAttr(actionResourceName, "name", action.name),
					resource.TestCheckResourceAttr(actionResourceName, "url", action.url),
					resource.TestCheckResourceAttr(actionResourceName, "source_type", action.sourceType),
					resource.TestCheckResourceAttr(actionResourceName, "applications.0", action.applications[0]),
					resource.TestCheckResourceAttr(actionResourceName, "subsystems.0", action.subsystems[0]),
					resource.TestCheckResourceAttr(actionResourceName, "is_private", fmt.Sprintf("%t", action.isPrivate)),
					resource.TestCheckResourceAttr(actionResourceName, "is_hidden", fmt.Sprintf("%t", action.isHidden)),
					resource.TestCheckResourceAttr(actionResourceName, "description", description),
					resource.TestCheckResourceAttr(actionResourceName, "dpxl_filter", dpxlFilter),
					resource.TestCheckResourceAttr(actionResourceName, "url_fields.#", "2"),
					resource.TestCheckResourceAttr(actionResourceName, "url_fields.0.name", "env"),
					resource.TestCheckResourceAttr(actionResourceName, "url_fields.0.required", "true"),
					resource.TestCheckResourceAttr(actionResourceName, "url_fields.1.name", "trace"),
					resource.TestCheckResourceAttr(actionResourceName, "url_fields.1.required", "false"),
				),
			},
			{
				ResourceName:      actionResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: testAccCoralogixResourceAction(updatedAction),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(actionResourceName, "id"),
					resource.TestCheckResourceAttr(actionResourceName, "name", updatedAction.name),
					resource.TestCheckResourceAttr(actionResourceName, "url", updatedAction.url),
					resource.TestCheckResourceAttr(actionResourceName, "source_type", updatedAction.sourceType),
					resource.TestCheckResourceAttr(actionResourceName, "applications.0", updatedAction.applications[0]),
					resource.TestCheckResourceAttr(actionResourceName, "subsystems.0", updatedAction.subsystems[0]),
					resource.TestCheckResourceAttr(actionResourceName, "is_private", fmt.Sprintf("%t", updatedAction.isPrivate)),
					resource.TestCheckResourceAttr(actionResourceName, "is_hidden", fmt.Sprintf("%t", updatedAction.isHidden)),
					// updatedAction omits all three, and the replace endpoint
					// merges omitted optional scalars, so this step is the
					// clear-on-omit contract.
					resource.TestCheckNoResourceAttr(actionResourceName, "description"),
					resource.TestCheckNoResourceAttr(actionResourceName, "dpxl_filter"),
					resource.TestCheckNoResourceAttr(actionResourceName, "url_fields.#"),
				),
			},
		},
	})
}

// TestAccCoralogixResourceActionOptionalFields covers the convergence behavior
// of description, dpxl_filter and url_fields: the required `<v1> ` prefix on
// dpxl_filter, explicit empties, and url_fields ordering. It reuses
// testAccPreCheck and testAccCheckActionDestroy.
func TestAccCoralogixResourceActionOptionalFields(t *testing.T) {
	prefixedFilter := "<v1> $d.severity == 'ERROR'"
	description := "runbook for disk pressure"
	changedDescription := "runbook for memory pressure"
	changedFilter := "<v1> $d.severity == 'WARNING'"
	emptyDescription := ""

	roundTrip := []actionURLFieldTestParams{
		{name: "env", required: true},
		{name: "trace", required: false},
	}
	ordered := []actionURLFieldTestParams{
		{name: "zeta", required: true},
		{name: "alpha", required: false},
		{name: "mike", required: true},
	}
	noURLFields := []actionURLFieldTestParams{}

	base := actionTestParams{
		name:         acctest.RandomWithPrefix("tf-acc-test"),
		url:          "https://www.google.com/search?q={{$p.selected_value}}",
		sourceType:   "Log",
		applications: []string{acctest.RandomWithPrefix("tf-acc-test")},
		subsystems:   []string{acctest.RandomWithPrefix("tf-acc-test")},
		isPrivate:    false,
		isHidden:     false,
	}

	withFields := func(desc, filter *string, fields *[]actionURLFieldTestParams) actionTestParams {
		params := base
		params.description = desc
		params.dpxlFilter = filter
		params.urlFields = fields
		return params
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckActionDestroy,
		Steps: []resource.TestStep{
			// Round-trip a prefixed dpxl_filter: state keeps the configured form.
			{
				Config: testAccCoralogixResourceAction(withFields(&description, &prefixedFilter, &roundTrip)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(actionResourceName, "description", description),
					resource.TestCheckResourceAttr(actionResourceName, "dpxl_filter", prefixedFilter),
					resource.TestCheckResourceAttr(actionResourceName, "url_fields.#", "2"),
					resource.TestCheckResourceAttr(actionResourceName, "url_fields.0.name", "env"),
					resource.TestCheckResourceAttr(actionResourceName, "url_fields.1.name", "trace"),
				),
			},
			// The phantom-diff failure mode only shows on a second plan.
			{
				Config:   testAccCoralogixResourceAction(withFields(&description, &prefixedFilter, &roundTrip)),
				PlanOnly: true,
			},
			// Set -> change.
			{
				Config: testAccCoralogixResourceAction(withFields(&changedDescription, &changedFilter, &ordered)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(actionResourceName, "description", changedDescription),
					resource.TestCheckResourceAttr(actionResourceName, "dpxl_filter", changedFilter),
					// Submitted order is preserved, not sorted.
					resource.TestCheckResourceAttr(actionResourceName, "url_fields.#", "3"),
					resource.TestCheckResourceAttr(actionResourceName, "url_fields.0.name", "zeta"),
					resource.TestCheckResourceAttr(actionResourceName, "url_fields.1.name", "alpha"),
					resource.TestCheckResourceAttr(actionResourceName, "url_fields.2.name", "mike"),
				),
			},
			// Explicit empties stay as configured.
			{
				Config: testAccCoralogixResourceAction(withFields(&emptyDescription, &emptyDescription, &noURLFields)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(actionResourceName, "description", ""),
					resource.TestCheckResourceAttr(actionResourceName, "dpxl_filter", ""),
					resource.TestCheckResourceAttr(actionResourceName, "url_fields.#", "0"),
				),
			},
			{
				Config:   testAccCoralogixResourceAction(withFields(&emptyDescription, &emptyDescription, &noURLFields)),
				PlanOnly: true,
			},
			// Set -> remove: the attributes must clear, not merge.
			{
				Config: testAccCoralogixResourceAction(withFields(&description, &prefixedFilter, &roundTrip)),
				Check:  resource.TestCheckResourceAttr(actionResourceName, "url_fields.#", "2"),
			},
			{
				Config: testAccCoralogixResourceAction(withFields(nil, nil, nil)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(actionResourceName, "description"),
					resource.TestCheckNoResourceAttr(actionResourceName, "dpxl_filter"),
					resource.TestCheckNoResourceAttr(actionResourceName, "url_fields.#"),
				),
			},
			{
				Config:   testAccCoralogixResourceAction(withFields(nil, nil, nil)),
				PlanOnly: true,
			},
		},
	})
}

func testAccCheckActionDestroy(s *terraform.State) error {
	meta := testAccProvider.Meta()

	if meta == nil {
		return nil
	}
	client := testAccProvider.Meta().(*clientset.ClientSet).Actions()
	ctx := context.TODO()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "coralogix_action" {
			continue
		}

		resp, _, err := client.ActionsServiceGetAction(ctx, rs.Primary.ID).Execute()
		if err == nil && resp.Action != nil && resp.Action.Id != nil {
			if *resp.Action.Id == rs.Primary.ID {
				return fmt.Errorf("action still exists: %s", rs.Primary.ID)
			}
		}
	}

	return nil
}

func testAccCoralogixResourceAction(action actionTestParams) string {
	return fmt.Sprintf(
		`resource "coralogix_action" "test" {
  						name               = "%s"
  						url			       = "%s"
  						source_type		   = "%s"
  						applications       =  %s
  						subsystems 		   =  %s
  						is_private         =  %t
%s}
`, action.name, action.url, action.sourceType, utils.SliceToString(action.applications), utils.SliceToString(action.subsystems), action.isPrivate, actionOptionalFieldsHCL(action))
}

func actionOptionalFieldsHCL(action actionTestParams) string {
	var hcl string
	if action.description != nil {
		hcl += fmt.Sprintf("  						description        =  %q\n", *action.description)
	}
	if action.dpxlFilter != nil {
		hcl += fmt.Sprintf("  						dpxl_filter        =  %q\n", *action.dpxlFilter)
	}
	if action.urlFields != nil {
		if len(*action.urlFields) == 0 {
			hcl += "  						url_fields         =  []\n"
			return hcl
		}
		hcl += "  						url_fields         =  [\n"
		for _, field := range *action.urlFields {
			hcl += fmt.Sprintf("  							{ name = %q, required = %t },\n", field.name, field.required)
		}
		hcl += "  						]\n"
	}
	return hcl
}
