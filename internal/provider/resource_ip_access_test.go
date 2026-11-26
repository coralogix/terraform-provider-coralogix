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
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
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
	disposable := strings.EqualFold(os.Getenv("CORALOGIX_ENV"), "EU2") &&
		os.Getenv(ipAccessGeneratedLiveAck) == ipAccessGeneratedLiveAckValue
	if err := clearIpAccessTestFixture(base, os.Getenv("CORALOGIX_API_KEY"), disposable); err != nil {
		t.Fatal(err)
	}
}

func clearIpAccessTestFixture(base, apiKey string, disposable bool) error {
	status, body := sendIpAccessRequest(base, apiKey, http.MethodGet, "", nil)
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
	for _, raw := range entries {
		entry := asObject(raw)
		if entry["enabled"] != false {
			return fmt.Errorf("IP access preflight found an enabled or invalid entry; refusing to delete")
		}
		owned := entry["name"] == "random range from wikipedia" &&
			entry["ipRange"] == "100.64.0.0/10"
		if !owned && !disposable {
			return fmt.Errorf("IP access preflight found an unowned entry; refusing to delete")
		}
	}
	status, _ = sendIpAccessRequest(base, apiKey, http.MethodDelete, "", nil)
	if status != http.StatusOK && status != http.StatusNoContent && status != http.StatusNotFound {
		return fmt.Errorf("IP access preflight DELETE failed with status %d", status)
	}
	return nil
}

func TestClearIpAccessTestFixture(t *testing.T) {
	for _, test := range []struct {
		name           string
		entry          map[string]any
		extraEntry     map[string]any
		getStatus      int
		allow          bool
		supportEnabled bool
		delete         bool
	}{
		{name: "absent", getStatus: http.StatusNotFound},
		{name: "empty", delete: true},
		{name: "old test fixture", entry: map[string]any{
			"name": "random range from wikipedia", "ipRange": "100.64.0.0/10", "enabled": false,
		}, delete: true},
		{name: "unowned entry", entry: map[string]any{
			"name": "other", "ipRange": "100.64.0.0/10", "enabled": false,
		}},
		{name: "disposable unowned entry", entry: map[string]any{
			"name": "other", "ipRange": "100.64.0.0/10", "enabled": false,
		}, allow: true, delete: true},
		{name: "disposable multiple entries", entry: map[string]any{
			"name": "other", "ipRange": "100.64.0.0/10", "enabled": false,
		}, extraEntry: map[string]any{
			"name": "another", "ipRange": "192.0.2.0/24", "enabled": false,
		}, allow: true, delete: true},
		{name: "disposable enabled entry", entry: map[string]any{
			"name": "other", "ipRange": "100.64.0.0/10", "enabled": true,
		}, allow: true},
		{name: "disposable enabled support access", allow: true, supportEnabled: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			settings := map[string]any{
				"ipAccess":                             map[string]any{},
				"enableCoralogixCustomerSupportAccess": "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_DISABLED",
			}
			if test.supportEnabled {
				settings["enableCoralogixCustomerSupportAccess"] = "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_ENABLED"
			}
			if test.entry != nil {
				entries := map[string]any{"test-entry": test.entry}
				if test.extraEntry != nil {
					entries["extra-entry"] = test.extraEntry
				}
				settings["ipAccess"] = entries
			}
			var methods []string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				methods = append(methods, r.Method)
				w.Header().Set("Content-Type", "application/json")
				switch {
				case r.URL.Path != ipAccessBasePath:
					w.WriteHeader(http.StatusNotFound)
				case r.Method == http.MethodGet && test.getStatus != 0:
					w.WriteHeader(test.getStatus)
				case r.Method == http.MethodGet:
					_ = json.NewEncoder(w).Encode(map[string]any{"settings": settings})
				case r.Method == http.MethodDelete:
					_ = json.NewEncoder(w).Encode(map[string]any{})
				default:
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			}))
			t.Cleanup(server.Close)
			err := clearIpAccessTestFixture(server.URL, "test-key", test.allow)
			if (err == nil) != (test.delete || test.getStatus == http.StatusNotFound) {
				t.Fatalf("clear error = %v, want delete = %v", err, test.delete)
			}
			deleted := false
			for _, method := range methods {
				deleted = deleted || method == http.MethodDelete
			}
			if deleted != test.delete {
				t.Fatalf("deleted = %v, want %v", deleted, test.delete)
			}
		})
	}
}

// ipAccessBasePath is the route of the team IP access settings.
const ipAccessBasePath = "/aaa/team-sec-ip-access/v1"

// sendIpAccessRequest calls the IP access API directly and returns the status and the parsed body.
func sendIpAccessRequest(base, apiKey, method, path string, body any) (int, any) {
	url := strings.TrimRight(base, "/") + ipAccessBasePath + path
	var reader io.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	}
	req, _ := http.NewRequest(method, url, reader)
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err.Error()
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var parsed any
	if len(bytes.TrimSpace(raw)) == 0 {
		parsed = map[string]any{}
	} else if json.Unmarshal(raw, &parsed) != nil {
		parsed = map[string]any{"raw": string(raw)}
	}
	return resp.StatusCode, parsed
}

func asObject(value any) map[string]any {
	if object, ok := value.(map[string]any); ok {
		return object
	}
	return map[string]any{}
}
