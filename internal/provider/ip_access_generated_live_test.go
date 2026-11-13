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

package provider

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const (
	ipAccessGeneratedLiveOptIn       = "IP_ACCESS_GENERATED_LIVE"
	ipAccessGeneratedLiveAck         = "IP_ACCESS_GENERATED_LIVE_ALLOW_MUTATION"
	ipAccessGeneratedLiveAckValue    = "I_UNDERSTAND_THIS_USES_A_DISPOSABLE_TEAM"
	ipAccessGeneratedResourceAddress = "coralogix_ip_access.generated_live"
	ipAccessReleasedProviderSource   = "registry.terraform.io/coralogix/coralogix"
	ipAccessReleasedProviderVersion  = "= 3.25.0"
)

// TestIpAccessGeneratedLiveAcceptance creates the resource with released provider 3.25.0, checks
// that the generated provider plans no change, then exercises generated update, import, data-source
// reads, and cleanup against EU2. It is opt-in and requires an empty disposable team with support
// access disabled; it creates only disabled documentation ranges.
func TestIpAccessGeneratedLiveAcceptance(t *testing.T) {
	if os.Getenv(ipAccessGeneratedLiveOptIn) != "1" {
		t.Skipf("set %s=1 to run against the disposable EU2 team", ipAccessGeneratedLiveOptIn)
	}
	if os.Getenv(ipAccessGeneratedLiveAck) != ipAccessGeneratedLiveAckValue {
		t.Fatalf("set %s=%s only when using the disposable EU2 team", ipAccessGeneratedLiveAck, ipAccessGeneratedLiveAckValue)
	}
	apiKey := os.Getenv("CORALOGIX_API_KEY")
	if apiKey == "" {
		t.Fatal("CORALOGIX_API_KEY must be set for the generated live acceptance test")
	}
	if !strings.EqualFold(os.Getenv("CORALOGIX_ENV"), "EU2") {
		t.Fatal("CORALOGIX_ENV must be EU2 for the generated live acceptance test")
	}
	if os.Getenv("TF_ACC_PROVIDER_NAMESPACE") != "coralogix" {
		t.Fatal("TF_ACC_PROVIDER_NAMESPACE must be coralogix for the released-provider upgrade step")
	}
	base, ok := cxsdk.URLFromRegion("eu2")
	if !ok {
		t.Fatal("SDK does not recognize EU2")
	}
	ensureIpAccessFidelityTeamSafe(t, base, apiKey)

	name := "codex-" + uuid.NewString()[:8]
	firstRange := "192.0.2.0/24"
	updatedRange := "198.51.100.0/24"
	resourceConfig := func(ipRange string, withDataSources bool) string {
		config := fmt.Sprintf(`resource "coralogix_ip_access" "generated_live" {
  enable_coralogix_customer_support_access = "disabled"
  ip_access = [{
    enabled = false
    ip_range = %q
    name     = %q
  }]
}`, ipRange, name)
		if withDataSources {
			config += `

data "coralogix_ip_access" "by_id" {
  id = coralogix_ip_access.generated_live.id
}

data "coralogix_ip_access" "without_id" {
  depends_on = [coralogix_ip_access.generated_live]
}`
		}
		return config
	}

	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		CheckDestroy: checkIpAccessGeneratedLiveDestroy(base, apiKey),
		Steps: []resource.TestStep{
			{
				Config: resourceConfig(firstRange, false),
				ExternalProviders: map[string]resource.ExternalProvider{
					"coralogix": {Source: ipAccessReleasedProviderSource, VersionConstraint: ipAccessReleasedProviderVersion},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(ipAccessGeneratedResourceAddress, "id"),
					resource.TestCheckResourceAttr(ipAccessGeneratedResourceAddress, "ip_access.#", "1"),
					resource.TestCheckResourceAttr(ipAccessGeneratedResourceAddress, "ip_access.0.ip_range", firstRange),
				),
			},
			{
				Config:                   resourceConfig(firstRange, false),
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(ipAccessGeneratedResourceAddress, plancheck.ResourceActionNoop)},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(ipAccessGeneratedResourceAddress, "id"),
					resource.TestCheckResourceAttr(ipAccessGeneratedResourceAddress, "ip_access.#", "1"),
					resource.TestCheckResourceAttr(ipAccessGeneratedResourceAddress, "ip_access.0.ip_range", firstRange),
				),
			},
			{
				Config:                   resourceConfig(firstRange, false),
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet(ipAccessGeneratedResourceAddress, "id"),
					resource.TestCheckResourceAttr(ipAccessGeneratedResourceAddress, "enable_coralogix_customer_support_access", "disabled"),
					resource.TestCheckResourceAttr(ipAccessGeneratedResourceAddress, "ip_access.#", "1"),
					resource.TestCheckResourceAttr(ipAccessGeneratedResourceAddress, "ip_access.0.enabled", "false"),
					resource.TestCheckResourceAttr(ipAccessGeneratedResourceAddress, "ip_access.0.ip_range", firstRange),
					resource.TestCheckResourceAttr(ipAccessGeneratedResourceAddress, "ip_access.0.name", name),
				),
			},
			{
				Config:                   resourceConfig(updatedRange, false),
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(ipAccessGeneratedResourceAddress, "ip_access.#", "1"),
					resource.TestCheckResourceAttr(ipAccessGeneratedResourceAddress, "ip_access.0.ip_range", updatedRange),
					resource.TestCheckResourceAttr(ipAccessGeneratedResourceAddress, "ip_access.0.name", name),
				),
			},
			{
				Config:                   resourceConfig(updatedRange, true),
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.coralogix_ip_access.by_id", "id", ipAccessGeneratedResourceAddress, "id"),
					resource.TestCheckResourceAttr("data.coralogix_ip_access.by_id", "ip_access.#", "1"),
					resource.TestCheckResourceAttrSet("data.coralogix_ip_access.without_id", "id"),
					resource.TestCheckResourceAttr("data.coralogix_ip_access.without_id", "enable_coralogix_customer_support_access", "disabled"),
					resource.TestCheckResourceAttr("data.coralogix_ip_access.without_id", "ip_access.#", "1"),
				),
			},
			{
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				ResourceName:             ipAccessGeneratedResourceAddress,
				ImportState:              true,
				ImportStateVerify:        true,
			},
		},
	})
}

func checkIpAccessGeneratedLiveDestroy(base, apiKey string) func(*terraform.State) error {
	return func(*terraform.State) error {
		status, body := sendIpAccessFidelityRequestNoTest(base, apiKey, http.MethodGet, "", nil)
		if status != http.StatusOK {
			return fmt.Errorf("post-destroy Get answered %d: %s", status, compact(body))
		}
		settings := asObject(asObject(body)["settings"])
		entries := asObject(settings["ipAccess"])
		access, _ := settings["enableCoralogixCustomerSupportAccess"].(string)
		if len(entries) != 0 || access != "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_DISABLED" {
			return fmt.Errorf("post-destroy settings are not empty and disabled; refusing to accept live test cleanup")
		}
		return nil
	}
}
