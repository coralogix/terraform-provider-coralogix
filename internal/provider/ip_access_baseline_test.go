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
	"encoding/json"
	"regexp"
	"sort"
	"strings"
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

// These tests record the handwritten coralogix_ip_access resource and paired data source. The
// data source remains handwritten for this migration; its schema and read behavior are baselined.
// After migration, the golden diff should show only message wording or explicitly accepted bugs.
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

func TestIpAccessBaselineDataSourceSchema(t *testing.T) {
	h := newIpAccessHarness(t)
	if h.dsSchema == nil {
		t.Fatal("coralogix_ip_access data source is not registered")
	}
	checkGoldenAt(t, ipAccessGoldenDir, "data_source_schema_current.txt", []byte(dataSourceSchemaText(h.dsSchema)))
}

func TestIpAccessBaselineScenarios(t *testing.T) {
	// A read after Delete is intentionally not a 404 case: the live Get response
	// is 200 with an empty ipAccess map, so the singleton remains in state.
	for name, run := range ipAccessScenarios {
		t.Run(name, func(t *testing.T) {
			h := newIpAccessHarness(t)
			run(h)
			canonicalizeIpAccessStepValues(h.steps)
			checkGoldenAt(t, ipAccessGoldenDir, "scenario_"+name+".json", maskIpAccessIDs(marshalGolden(t, h.steps)))
		})
	}
}

func TestIpAccessBaselineDataSourceScenarios(t *testing.T) {
	for name, run := range ipAccessDataSourceScenarios {
		t.Run(name, func(t *testing.T) {
			h := newIpAccessHarness(t)
			run(h)
			canonicalizeIpAccessStepValues(h.steps)
			checkGoldenAt(t, ipAccessGoldenDir, "data_source_scenario_"+name+".json", maskIpAccessIDs(marshalGolden(t, h.steps)))
		})
	}
}

var ipAccessScenarios = map[string]func(h *grHarness){
	"default-create-read-destroy": func(h *grHarness) {
		s := h.Apply("create with default support access and empty rules", h.null(), j{"ip_access": l{}})
		s = h.RefreshPlan("refresh and replan defaults", s)
		h.Destroy("destroy", s)
	},
	"ip-access-omitted-on-create": func(h *grHarness) {
		// The Optional+Computed set is unknown when omitted; the handwritten slice
		// model cannot consume that value, so Create fails before an API request.
		h.Apply("create with ip_access omitted", h.null(), j{})
	},
	"ip-access-omitted-on-update": func(h *grHarness) {
		s := h.Apply("create with one rule", h.null(), j{"ip_access": l{
			j{"enabled": false, "ip_range": "192.0.2.0/24", "name": "first"},
		}})
		// Omission also produces an unknown set on Update; record the conversion
		// error and confirm a refresh keeps the existing backend rule.
		s = h.Apply("update with ip_access omitted", s, j{
			"enable_coralogix_customer_support_access": "disabled",
		})
		h.RefreshPlan("refresh and replan after omitted ip_access", s)
	},
	// This fake-backed scenario exercises ENABLED support access for Terraform schema
	// coverage. The live recorder keeps support access DISABLED, so the backend's response
	// to ENABLED remains unverified and is not treated as a fidelity fact.
	"full-lifecycle": func(h *grHarness) {
		config := j{
			"enable_coralogix_customer_support_access": "enabled",
			"ip_access": l{
				j{"enabled": true, "ip_range": "192.0.2.0/24", "name": "office"},
				j{"enabled": false, "ip_range": "198.51.100.0/24"},
			},
		}
		s := h.Apply("create", h.null(), config)
		s = h.Refresh("refresh", s)
		updated := j{
			"enable_coralogix_customer_support_access": "disabled",
			"ip_access": l{j{"enabled": false, "ip_range": "203.0.113.0/24"}},
		}
		s = h.Apply("update: replace two rules with one and omit name", s, updated)
		s = h.Refresh("refresh after replace", s)
		s = h.Apply("update: clear rules and omit support access", s, j{"ip_access": l{}})
		h.RefreshPlan("refresh after clearing rules", s)
		h.Destroy("destroy", s)
	},
	"create-invalid-cidr": func(h *grHarness) {
		h.Apply("create: invalid CIDR", h.null(), j{"ip_access": l{
			j{"enabled": false, "ip_range": "not-a-cidr"},
		}})
	},
	"create-conflict": func(h *grHarness) {
		h.ipAccess().seed(j{
			"enableCoralogixCustomerSupportAccess": "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_DISABLED",
			"ipAccess": map[string]any{
				"entry-1": j{"enabled": false, "ipRange": "192.0.2.0/24", "name": "existing"},
			},
		})
		h.Apply("create: settings already exist", h.null(), j{"ip_access": l{
			j{"enabled": false, "ip_range": "198.51.100.0/24", "name": "new"},
		}})
	},
	"replace-invalid-cidr": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"ip_access": l{
			j{"enabled": false, "ip_range": "192.0.2.0/24", "name": "existing"},
		}})
		h.Apply("replace: invalid CIDR", s, j{"ip_access": l{
			j{"enabled": false, "ip_range": "not-a-cidr"},
		}})
	},
	"read-after-delete-outside": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"ip_access": l{
			j{"enabled": false, "ip_range": "192.0.2.0/24", "name": "first"},
		}})
		h.ipAccess().clearOutside()
		h.Refresh("refresh: Get still returns settings with no rules", s)
	},
	"read-api-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"ip_access": l{
			j{"enabled": false, "ip_range": "192.0.2.0/24", "name": "first"},
		}})
		h.ipAccess().getStatus = 404
		h.Refresh("refresh: Get answers 404", s)
	},
	"read-api-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"ip_access": l{
			j{"enabled": false, "ip_range": "192.0.2.0/24", "name": "first"},
		}})
		h.ipAccess().getStatus = 500
		h.Refresh("refresh: Get answers 500", s)
	},
	"update-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"ip_access": l{
			j{"enabled": false, "ip_range": "192.0.2.0/24", "name": "first"},
		}})
		h.ipAccess().replaceStatus = 404
		h.Apply("update: API answers 404", s, j{"ip_access": l{
			j{"enabled": false, "ip_range": "198.51.100.0/24", "name": "replacement"},
		}})
	},
	"delete-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"ip_access": l{
			j{"enabled": false, "ip_range": "192.0.2.0/24", "name": "first"},
		}})
		h.ipAccess().deleteStatus = 404
		h.Destroy("destroy: API answers 404", s)
	},
	"delete-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"ip_access": l{
			j{"enabled": false, "ip_range": "192.0.2.0/24", "name": "first"},
		}})
		h.ipAccess().deleteStatus = 500
		h.Destroy("destroy: API answers 500", s)
	},
	"import": func(h *grHarness) {
		h.ipAccess().seed(j{
			"enableCoralogixCustomerSupportAccess": "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_ENABLED",
			"ipAccess": map[string]any{
				"entry-1": j{"enabled": true, "ipRange": "203.0.113.0/24", "name": "seeded"},
			},
		})
		const importedID = "imported-settings"
		s := h.Import("import", importedID)
		// The import protocol request carries an ID that the refresh then replaces
		// with the backend settings ID. Record both so a migration cannot preserve
		// the imported value unnoticed.
		h.steps[len(h.steps)-2].Config = j{"id": importedID}
		h.Apply("plan matching config", s, j{
			"enable_coralogix_customer_support_access": "enabled",
			"ip_access": l{j{"enabled": true, "ip_range": "203.0.113.0/24", "name": "seeded"}},
		})
	},
}

var ipAccessDataSourceScenarios = map[string]func(h *grHarness){
	"read-by-id": func(h *grHarness) {
		h.ipAccess().seed(j{
			"enableCoralogixCustomerSupportAccess": "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_ENABLED",
			"ipAccess": map[string]any{
				"entry-1": j{"enabled": false, "ipRange": "192.0.2.0/24", "name": "seeded"},
			},
		})
		h.ReadDataSource("read configured id", j{"id": "configured-id"})
	},
	"read-without-id": func(h *grHarness) {
		h.ipAccess().seed(j{
			"enableCoralogixCustomerSupportAccess": "CORALOGIX_CUSTOMER_SUPPORT_ACCESS_DISABLED",
			"ipAccess":                             map[string]any{},
		})
		h.ReadDataSource("read without id", j{})
	},
	"read-api-not-found": func(h *grHarness) {
		h.ipAccess().getStatus = 404
		h.ReadDataSource("Get answers 404", j{"id": "configured-id"})
	},
	"read-api-error": func(h *grHarness) {
		h.ipAccess().getStatus = 500
		h.ReadDataSource("Get answers 500", j{"id": "configured-id"})
	},
}

var ipAccessIDPattern = regexp.MustCompile(`entry-[0-9]+`)

func maskIpAccessIDs(golden []byte) []byte {
	golden = []byte(strings.ReplaceAll(string(golden), "imported-settings", "<import-id>"))
	golden = []byte(strings.ReplaceAll(string(golden), "settings-id", "<settings-id>"))
	return ipAccessIDPattern.ReplaceAll(golden, []byte("<masked-id>"))
}

func canonicalizeIpAccessStepValues(steps []grStep) {
	for i := range steps {
		canonicalizeIpAccessSets(steps[i].Plan)
		canonicalizeIpAccessSets(steps[i].State)
	}
}

func canonicalizeIpAccessSets(value any) {
	switch value := value.(type) {
	case map[string]any:
		for key, item := range value {
			if key == "ip_access" {
				if elements, ok := item.([]any); ok {
					sort.Slice(elements, func(i, j int) bool {
						left, _ := json.Marshal(elements[i])
						right, _ := json.Marshal(elements[j])
						return string(left) < string(right)
					})
					value[key] = elements
				}
				continue
			}
			canonicalizeIpAccessSets(item)
		}
	case []any:
		for _, item := range value {
			canonicalizeIpAccessSets(item)
		}
	}
}
