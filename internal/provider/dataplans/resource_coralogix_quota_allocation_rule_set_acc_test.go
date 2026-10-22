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

package dataplans_test

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider"
)

const quotaAddress = "coralogix_quota_allocation_rule_set.test"

func quotaRuleConfig(allocationType string) string {
	return `resource "coralogix_quota_allocation_rule_set" "test" {
  rules = [
    {
      allocation = 10
      allocation_type = "` + allocationType + `"
      can_overflow = true
      enabled = true
      entity_type = "logs"
    },
  ]
}`
}

// TestAccCoralogixResourceQuotaAllocationUnspecifiedAlias creates a rule with
// allocation_type = "unspecified", the alias of "percentage" that the API stores. Terraform checks
// the plan of a create against the configuration, and allows the changed value only inside a set.
// Every step plans again after the apply and fails when that plan is not empty. The generated
// acceptance test does not cover it: it sets one value for the attribute.
func TestAccCoralogixResourceQuotaAllocationUnspecifiedAlias(t *testing.T) {
	percentage := resource.TestCheckTypeSetElemNestedAttrs(quotaAddress, "rules.*", map[string]string{
		"entity_type":     "logs",
		"allocation_type": "percentage",
	})
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			if os.Getenv("CORALOGIX_API_KEY") == "" {
				t.Fatal("CORALOGIX_API_KEY must be set for acceptance tests")
			}
		},
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"coralogix": func() (tfprotov6.ProviderServer, error) {
				server, err := provider.MuxServer(context.Background())
				if err != nil {
					return nil, err
				}
				return server(), nil
			},
		},
		Steps: []resource.TestStep{
			{Config: quotaRuleConfig("unspecified"), Check: percentage},
			{Config: quotaRuleConfig("percentage"), Check: percentage},
			{Config: quotaRuleConfig("unspecified"), Check: percentage},
			{ResourceName: quotaAddress, ImportState: true, ImportStateVerify: true},
		},
	})
}
