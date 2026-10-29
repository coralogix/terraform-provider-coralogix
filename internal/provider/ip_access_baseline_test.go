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
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/aaa"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils/schemadump"
)

const (
	ipAccessTypeName  = "coralogix_ip_access"
	ipAccessGoldenDir = "testdata/ip_access_baseline"
	ipAccessBasePath  = "/aaa/team-sec-ip-access/v1"
)

// These tests record the handwritten coralogix_ip_access resource. The data source is
// intentionally excluded. After migration, the golden diff should show only message wording or
// explicitly accepted bug fixes.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run TestIpAccessBaseline
func newIpAccessHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, ipAccessTypeName, newIpAccessFake(t))
}

func TestIpAccessBaselineSchema(t *testing.T) {
	var current resource.SchemaResponse
	aaa.NewIpAccessResource().Schema(context.Background(), resource.SchemaRequest{}, &current)
	checkGoldenAt(t, ipAccessGoldenDir, "schema_current.txt", []byte(schemadump.Text(current.Schema)))
}

func TestIpAccessBaselineScenarios(t *testing.T) {
	for name, run := range ipAccessScenarios {
		t.Run(name, func(t *testing.T) {
			h := newIpAccessHarness(t)
			run(h)
			checkGoldenAt(t, ipAccessGoldenDir, "scenario_"+name+".json", maskIpAccessIDs(marshalGolden(t, h.steps)))
		})
	}
}

var ipAccessScenarios = map[string]func(h *grHarness){
	"default-create-read-destroy": func(h *grHarness) {
		s := h.Apply("create with default support access and empty rules", h.null(), j{"ip_access": l{}})
		s = h.Refresh("refresh", s)
		h.Destroy("destroy", s)
	},
	"full-lifecycle": func(h *grHarness) {
		config := j{
			"enable_coralogix_customer_support_access": "enabled",
			"ip_access": l{j{"enabled": true, "ip_range": "192.0.2.0/24", "name": "office"}},
		}
		s := h.Apply("create", h.null(), config)
		s = h.Refresh("refresh", s)
		updated := j{
			"enable_coralogix_customer_support_access": "disabled",
			"ip_access": l{
				j{"enabled": false, "ip_range": "198.51.100.0/24", "name": "vpn"},
			},
		}
		s = h.Apply("update: replace settings", s, updated)
		s = h.Refresh("refresh after replace", s)
		h.Destroy("destroy", s)
	},
	"import": func(h *grHarness) {
		h.ipAccess().seed(j{
			"enableCoralogixCustomerSupportAccess": "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_ENABLED",
			"ipAccess": map[string]any{
				"entry-1": j{"enabled": true, "ipRange": "203.0.113.0/24", "name": "seeded"},
			},
		})
		s := h.Import("import", "imported-settings")
		h.Apply("plan matching config", s, j{
			"enable_coralogix_customer_support_access": "enabled",
			"ip_access": l{j{"enabled": true, "ip_range": "203.0.113.0/24", "name": "seeded"}},
		})
	},
}

var ipAccessIDPattern = regexp.MustCompile(`settings-id|entry-[0-9]+|imported-settings`)

func maskIpAccessIDs(golden []byte) []byte {
	return ipAccessIDPattern.ReplaceAll(golden, []byte("<masked-id>"))
}
