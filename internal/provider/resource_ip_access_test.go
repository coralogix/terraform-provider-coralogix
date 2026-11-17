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
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var IpAccessResource = `

resource "coralogix_ip_access" "test" {
    enable_coralogix_customer_support_access = "disabled"
    ip_access = [{ 
	   enabled = false, 
	   ip_range = "100.64.0.0/10", 
	   name = "random range from wikipedia" 
	}]
}
`

var ipAccessResourceName = "coralogix_ip_access.test"

func TestIpAccessResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { precheckIpAccessResource(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: IpAccessResource,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(ipAccessResourceName, "enable_coralogix_customer_support_access", "disabled"),
					resource.TestCheckTypeSetElemNestedAttrs(ipAccessResourceName, "ip_access.*",
						map[string]string{
							"enabled":  "false",
							"ip_range": "100.64.0.0/10",
							"name":     "random range from wikipedia",
						},
					),
				),
			},
			{
				Config: strings.ReplaceAll(IpAccessResource, "100.64.0.0/10", "10.0.0.0/8"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(ipAccessResourceName, "enable_coralogix_customer_support_access", "disabled"),
					resource.TestCheckTypeSetElemNestedAttrs(ipAccessResourceName, "ip_access.*", map[string]string{
						"enabled": "false", "ip_range": "10.0.0.0/8", "name": "random range from wikipedia",
					}),
				),
			},
			{
				ResourceName:      ipAccessResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config:   strings.ReplaceAll(IpAccessResource, "100.64.0.0/10", "10.0.0.0/8"),
				PlanOnly: true,
			},
		},
	})
}

func precheckIpAccessResource(t *testing.T) {
	t.Helper()
	testAccPreCheck(t)
	base, ok := cxsdk.URLFromRegion(strings.ToLower(os.Getenv("CORALOGIX_ENV")))
	if !ok {
		base = cxsdk.URLFromDomain(os.Getenv("CORALOGIX_DOMAIN"))
	}
	if err := clearIpAccessTestFixture(base, os.Getenv("CORALOGIX_API_KEY")); err != nil {
		t.Fatal(err)
	}
}

func clearIpAccessTestFixture(base, apiKey string) error {
	status, body := sendIpAccessFidelityRequestNoTest(base, apiKey, http.MethodGet, "", nil)
	if status == http.StatusNotFound {
		return nil
	}
	if status != http.StatusOK {
		return fmt.Errorf("IP access preflight GET failed with status %d", status)
	}
	settings := asObject(asObject(body)["settings"])
	access, _ := settings["enableCoralogixCustomerSupportAccess"].(string)
	if access != "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_DISABLED" {
		return fmt.Errorf("IP access preflight found enabled support access; refusing to delete")
	}
	entries := asObject(settings["ipAccess"])
	if len(entries) > 1 {
		return fmt.Errorf("IP access preflight found other entries; refusing to delete")
	}
	for _, raw := range entries {
		entry := asObject(raw)
		if entry["name"] != "random range from wikipedia" ||
			entry["ipRange"] != "100.64.0.0/10" || entry["enabled"] != false {
			return fmt.Errorf("IP access preflight found an unowned entry; refusing to delete")
		}
	}
	status, _ = sendIpAccessFidelityRequestNoTest(base, apiKey, http.MethodDelete, "", nil)
	if status != http.StatusOK && status != http.StatusNoContent && status != http.StatusNotFound {
		return fmt.Errorf("IP access preflight DELETE failed with status %d", status)
	}
	return nil
}

func TestClearIpAccessTestFixture(t *testing.T) {
	for _, test := range []struct {
		name      string
		entry     map[string]any
		getStatus int
		delete    bool
	}{
		{name: "absent", getStatus: http.StatusNotFound},
		{name: "empty", delete: true},
		{name: "old test fixture", entry: map[string]any{
			"name": "random range from wikipedia", "ipRange": "100.64.0.0/10", "enabled": false,
		}, delete: true},
		{name: "unowned entry", entry: map[string]any{
			"name": "other", "ipRange": "100.64.0.0/10", "enabled": false,
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			fake := newIpAccessFidelityFake()
			fake.getStatus = test.getStatus
			if test.entry != nil {
				fake.settings["ipAccess"] = map[string]any{"test-entry": test.entry}
			}
			server := httptest.NewServer(fake)
			t.Cleanup(server.Close)
			err := clearIpAccessTestFixture(server.URL, "test-key")
			if (err == nil) != (test.delete || test.getStatus == http.StatusNotFound) {
				t.Fatalf("clear error = %v, want delete = %v", err, test.delete)
			}
			deleted := false
			for _, request := range fake.requests {
				deleted = deleted || request.Method == http.MethodDelete
			}
			if deleted != test.delete {
				t.Fatalf("deleted = %v, want %v", deleted, test.delete)
			}
		})
	}
}
