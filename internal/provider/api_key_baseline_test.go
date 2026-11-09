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
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/aaa"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils/schemadump"
)

const (
	apiKeyTypeName  = "coralogix_api_key"
	apiKeyGoldenDir = "testdata/api_key_baseline"
)

// These tests record the handwritten coralogix_api_key resource and its data source against a fake
// API that follows answers recorded from the real API. After the switch to generated code, the
// golden diff must show only message wording or proven bug fixes.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run 'TestAPIKeyBaseline|TestDataSourceBaseline'
func newAPIKeyHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, apiKeyTypeName, newAPIKeyFake(t))
}

func TestAPIKeyBaselineSchema(t *testing.T) {
	var current resource.SchemaResponse
	aaa.NewApiKeyResource().Schema(context.Background(), resource.SchemaRequest{}, &current)
	checkGoldenAt(t, apiKeyGoldenDir, "schema_current.txt", []byte(schemadump.Text(current.Schema)))
}

func TestAPIKeyBaselineScenarios(t *testing.T) {
	for name, run := range apiKeyScenarios {
		t.Run(name, func(t *testing.T) {
			h := newAPIKeyHarness(t)
			run(h)
			checkGoldenAt(t, apiKeyGoldenDir, "scenario_"+name+".json", marshalGolden(t, h.steps))
		})
	}
}

// apiKeyFirstID is the id that the fake gives the first key of a scenario, as Terraform shows it.
var apiKeyFirstID = apiKeyFakeID(1)

// apiKeyTeam is the owner of every key that a scenario creates: the team of the fake.
func apiKeyTeam() j { return j{"team_id": strconv.Itoa(apiKeyFakeTeamID)} }

// apiKeyConfig is a key of the team with one preset and one permission. Each scenario changes it.
func apiKeyConfig(name string, extra j) j {
	cfg := j{
		"name":        name,
		"owner":       apiKeyTeam(),
		"presets":     []any{"APM"},
		"permissions": []any{"livetail:Read"},
	}
	for k, v := range extra {
		cfg[k] = v
	}
	return cfg
}

const (
	// apiKeyPolicyComplete sets every permission that the API adds to a policy of a team key, so
	// the API stores what the configuration holds, in compact form and in any key order.
	apiKeyPolicyComplete = `{ "version": "2025-01-01", "default": { "permissions": { "team-custom-api-keys:ReadConfig": "grant", "team-custom-api-keys:Manage": "deny", "team-custom-api-keys:ReadAccessPolicy": "grant", "team-custom-api-keys:UpdateAccessPolicy": "deny" } }, "rules": [] }`
	// apiKeyPolicyCompleteReformatted is the same policy with other white space and key order.
	apiKeyPolicyCompleteReformatted = `{"rules":[],"default":{"permissions":{"team-custom-api-keys:UpdateAccessPolicy":"deny","team-custom-api-keys:ReadAccessPolicy":"grant","team-custom-api-keys:Manage":"deny","team-custom-api-keys:ReadConfig":"grant"}},"version":"2025-01-01"}`
	// apiKeyPolicyIncomplete leaves out three permissions, which the API adds as "deny".
	apiKeyPolicyIncomplete = `{"version":"2025-01-01","default":{"permissions":{"team-custom-api-keys:Manage":"deny"}},"rules":[]}`
	// apiKeyPolicyForeign is the policy of the docs example. A team key rejects its permissions.
	apiKeyPolicyForeign = `{ "version": "2025-01-01", "default": { "permissions": { "data-ingest-api-keys:ReadAccessPolicy": "grant", "data-ingest-api-keys:Manage": "deny", "data-ingest-api-keys:UpdateAccessPolicy": "deny", "data-ingest-api-keys:ReadConfig": "grant" } }, "rules": [] }`
)

var apiKeyScenarios = map[string]func(h *grHarness){
	"full-lifecycle": func(h *grHarness) {
		s := h.Apply("create", h.null(), apiKeyConfig("ingest", nil))
		s = h.RefreshPlan("refresh and replan", s)
		s = h.Apply("update: rename", s, apiKeyConfig("ingest-renamed", nil))
		s = h.Apply("update: deactivate", s, apiKeyConfig("ingest-renamed", j{"active": false}))
		s = h.Apply("update: activate again", s, apiKeyConfig("ingest-renamed", j{"active": true}))
		s = h.Apply("update: other presets and permissions", s, apiKeyConfig("ingest-renamed", j{
			"presets": []any{}, "permissions": []any{"livetail:Read", "serverless:Read"},
		}))
		s = h.RefreshPlan("refresh and replan after update", s)
		h.Destroy("destroy", s)
	},

	// The value comes from the Create answer. A rename keeps it.
	"value-kept-on-update": func(h *grHarness) {
		s := h.Apply("create", h.null(), apiKeyConfig("ingest", nil))
		s = h.Apply("update: rename", s, apiKeyConfig("ingest-renamed", nil))
		h.RefreshPlan("refresh and replan", s)
	},

	// Create sends no isActive, so a key is active, and the plan of active = false is not.
	"create-inactive": func(h *grHarness) {
		h.Apply("create with active = false", h.null(), apiKeyConfig("inactive", j{"active": false}))
	},
	"active-drift": func(h *grHarness) {
		s := h.Apply("create", h.null(), apiKeyConfig("drifting", nil))
		h.apiKeys().edit(apiKeyFirstID, func(r *apiKeyRecord) { r.active = false })
		s = h.RefreshPlan("refresh: the key was deactivated outside", s)
		h.Apply("update: back to the configuration", s, apiKeyConfig("drifting", nil))
	},

	// presets and permissions
	"presets-only": func(h *grHarness) {
		s := h.Apply("create with presets only", h.null(), j{"name": "presets", "owner": apiKeyTeam(), "presets": []any{"APM"}, "permissions": []any{}})
		h.RefreshPlan("refresh and replan", s)
	},
	"permissions-only": func(h *grHarness) {
		s := h.Apply("create with permissions only", h.null(), j{"name": "permissions", "owner": apiKeyTeam(), "presets": []any{}, "permissions": []any{"livetail:Read"}})
		h.RefreshPlan("refresh and replan", s)
	},
	"presets-and-permissions-empty": func(h *grHarness) {
		h.Apply("create with both lists empty", h.null(), j{"name": "empty", "owner": apiKeyTeam(), "presets": []any{}, "permissions": []any{}})
	},
	"update-to-both-empty": func(h *grHarness) {
		s := h.Apply("create", h.null(), apiKeyConfig("emptied", nil))
		s = h.Apply("update: both lists empty", s, apiKeyConfig("emptied", j{"presets": []any{}, "permissions": []any{}}))
		h.RefreshPlan("refresh and replan", s)
	},
	"update-clear-presets": func(h *grHarness) {
		s := h.Apply("create", h.null(), apiKeyConfig("clearing", nil))
		s = h.Apply("update: no presets", s, apiKeyConfig("clearing", j{"presets": []any{}}))
		h.RefreshPlan("refresh and replan", s)
	},
	"update-clear-permissions": func(h *grHarness) {
		s := h.Apply("create", h.null(), apiKeyConfig("clearing", nil))
		s = h.Apply("update: no permissions", s, apiKeyConfig("clearing", j{"permissions": []any{}}))
		h.RefreshPlan("refresh and replan", s)
	},
	"presets-and-permissions-omitted": func(h *grHarness) {
		h.Validate("validate without presets and permissions", j{"name": "bare", "owner": apiKeyTeam()})
	},
	// A permission that a preset already holds is returned as a direct permission, too.
	"permission-of-a-preset": func(h *grHarness) {
		s := h.Apply("create", h.null(), apiKeyConfig("overlap", j{"permissions": []any{"serverless:Read"}}))
		h.RefreshPlan("refresh and replan", s)
	},
	// The API matches a preset in any case and returns its name.
	"preset-in-lower-case": func(h *grHarness) {
		h.Apply("create with the preset apm", h.null(), apiKeyConfig("lower", j{"presets": []any{"apm"}}))
	},
	"preset-unknown": func(h *grHarness) {
		h.Apply("create with an unknown preset", h.null(), apiKeyConfig("unknown", j{"presets": []any{"NoSuchPreset"}}))
	},
	"permission-unknown": func(h *grHarness) {
		h.Apply("create with an unknown permission", h.null(), apiKeyConfig("unknown", j{"permissions": []any{"no:Such"}}))
	},
	"update-preset-unknown": func(h *grHarness) {
		s := h.Apply("create", h.null(), apiKeyConfig("unknown", nil))
		h.Apply("update: an unknown preset", s, apiKeyConfig("unknown", j{"presets": []any{"NoSuchPreset"}}))
	},
	"lists-drift": func(h *grHarness) {
		s := h.Apply("create", h.null(), apiKeyConfig("drifting", nil))
		h.apiKeys().edit(apiKeyFirstID, func(r *apiKeyRecord) { r.permissions = []string{"serverless:Read"}; r.presets = nil })
		s = h.RefreshPlan("refresh: the lists changed outside", s)
		h.Apply("update: back to the configuration", s, apiKeyConfig("drifting", nil))
	},

	// owner
	"owner-user-id": func(h *grHarness) {
		h.Apply("create for a user", h.null(), apiKeyConfig("user", j{"owner": j{"user_id": "11111111-1111-4111-8111-111111111111"}}))
	},
	"owner-organisation-id": func(h *grHarness) {
		h.Apply("create for an organisation", h.null(), apiKeyConfig("organisation", j{"owner": j{"organisation_id": "11111111-1111-4111-8111-111111111111"}}))
	},
	"owner-other-team": func(h *grHarness) {
		h.Apply("create for another team", h.null(), apiKeyConfig("other", j{"owner": j{"team_id": "1"}}))
	},
	"owner-team-id-not-a-number": func(h *grHarness) {
		h.Apply("create with a team id that is no number", h.null(), apiKeyConfig("bad-team", j{"owner": j{"team_id": "not-a-number"}}))
	},
	"owner-none": func(h *grHarness) {
		h.Validate("validate an owner without an id", apiKeyConfig("none", j{"owner": j{}}))
	},
	"owner-two": func(h *grHarness) {
		h.Validate("validate an owner with two ids", apiKeyConfig("two", j{"owner": j{"team_id": "1", "user_id": "u"}}))
	},
	"owner-three": func(h *grHarness) {
		h.Validate("validate an owner with three ids", apiKeyConfig("three", j{"owner": j{"team_id": "1", "user_id": "u", "organisation_id": "o"}}))
	},
	"owner-missing": func(h *grHarness) {
		h.Validate("validate without owner", j{"name": "ownerless", "presets": []any{"APM"}, "permissions": []any{}})
	},
	"owner-change-replaces": func(h *grHarness) {
		s := h.Apply("create", h.null(), apiKeyConfig("owned", nil))
		h.Plan("plan: another team", s, apiKeyConfig("owned", j{"owner": j{"team_id": "7"}}))
		h.Plan("plan: a user", s, apiKeyConfig("owned", j{"owner": j{"user_id": "u"}}))
		h.Plan("plan: an organisation", s, apiKeyConfig("owned", j{"owner": j{"organisation_id": "o"}}))
	},

	// name
	"name-missing": func(h *grHarness) {
		h.Validate("validate without name", j{"owner": apiKeyTeam(), "presets": []any{"APM"}, "permissions": []any{}})
	},
	"name-empty": func(h *grHarness) {
		h.Apply("create with an empty name", h.null(), apiKeyConfig("", nil))
	},
	// Names are not unique.
	"name-duplicate": func(h *grHarness) {
		h.apiKeys().seed(apiKeySeed{name: "taken", presets: []string{"APM"}})
		s := h.Apply("create with a name that another key has", h.null(), apiKeyConfig("taken", nil))
		h.RefreshPlan("refresh and replan", s)
	},
	"rename-to-empty-name": func(h *grHarness) {
		s := h.Apply("create", h.null(), apiKeyConfig("named", nil))
		h.Apply("update: rename to an empty name", s, apiKeyConfig("", nil))
	},

	// hashed: the attribute is computed, and Create always sends false.
	"hashed-in-config": func(h *grHarness) {
		h.Validate("validate a config that sets hashed", apiKeyConfig("hash", j{"hashed": true}))
	},
	"hashed-key-outside": func(h *grHarness) {
		id := h.apiKeys().seed(apiKeySeed{name: "hashed", hashed: true, presets: []string{"APM"}})
		h.Import("import a hashed key", id)
	},

	// access_policy
	"policy-omitted": func(h *grHarness) {
		s := h.Apply("create without access_policy", h.null(), apiKeyConfig("plain", nil))
		h.RefreshPlan("refresh and replan", s)
	},
	"policy-empty-string": func(h *grHarness) {
		s := h.Apply("create with access_policy = \"\"", h.null(), apiKeyConfig("plain", j{"access_policy": ""}))
		h.RefreshPlan("refresh and replan", s)
	},
	// The API stores compact JSON in another key order. The state keeps the text of the config.
	"policy-complete": func(h *grHarness) {
		s := h.Apply("create with a complete policy", h.null(), apiKeyConfig("policy", j{"access_policy": apiKeyPolicyComplete}))
		s = h.RefreshPlan("refresh and replan", s)
		h.Plan("plan: the same policy with other white space and key order", s, apiKeyConfig("policy", j{"access_policy": apiKeyPolicyCompleteReformatted}))
	},
	// The API adds the permissions that the policy leaves out, so the stored policy differs from
	// the plan.
	"policy-incomplete": func(h *grHarness) {
		h.Apply("create with a policy that leaves permissions out", h.null(), apiKeyConfig("policy", j{"access_policy": apiKeyPolicyIncomplete}))
	},
	"policy-foreign-permissions": func(h *grHarness) {
		h.Apply("create with the policy of the docs example", h.null(), apiKeyConfig("policy", j{"access_policy": apiKeyPolicyForeign}))
	},
	"policy-not-json": func(h *grHarness) {
		h.Apply("create with a policy that is no JSON", h.null(), apiKeyConfig("policy", j{"access_policy": "not json"}))
	},
	"policy-set-on-update": func(h *grHarness) {
		s := h.Apply("create without policy", h.null(), apiKeyConfig("policy", nil))
		s = h.Apply("update: set a complete policy", s, apiKeyConfig("policy", j{"access_policy": apiKeyPolicyComplete}))
		h.RefreshPlan("refresh and replan", s)
	},
	"policy-cleared-on-update": func(h *grHarness) {
		s := h.Apply("create with a policy", h.null(), apiKeyConfig("policy", j{"access_policy": apiKeyPolicyComplete}))
		s = h.Apply("update: access_policy = \"\"", s, apiKeyConfig("policy", j{"access_policy": ""}))
		h.RefreshPlan("refresh and replan", s)
	},
	// Removing the attribute keeps the policy.
	"policy-removed-on-update": func(h *grHarness) {
		s := h.Apply("create with a policy", h.null(), apiKeyConfig("policy", j{"access_policy": apiKeyPolicyComplete}))
		s = h.Apply("update: remove access_policy", s, apiKeyConfig("policy", nil))
		h.RefreshPlan("refresh and replan", s)
	},
	"policy-untouched-by-rename": func(h *grHarness) {
		s := h.Apply("create with a policy", h.null(), apiKeyConfig("policy", j{"access_policy": apiKeyPolicyComplete}))
		h.Apply("update: rename", s, apiKeyConfig("policy-renamed", j{"access_policy": apiKeyPolicyComplete}))
	},
	"policy-invalid-on-update": func(h *grHarness) {
		s := h.Apply("create", h.null(), apiKeyConfig("policy", nil))
		h.Apply("update: a policy that is no JSON", s, apiKeyConfig("policy", j{"access_policy": "not json"}))
	},
	"policy-changed-outside": func(h *grHarness) {
		s := h.Apply("create with a policy", h.null(), apiKeyConfig("policy", j{"access_policy": apiKeyPolicyComplete}))
		h.apiKeys().edit(apiKeyFirstID, func(r *apiKeyRecord) { r.accessPolicy = "" })
		s = h.RefreshPlan("refresh: the policy was removed outside", s)
		h.Apply("update: back to the configuration", s, apiKeyConfig("policy", j{"access_policy": apiKeyPolicyComplete}))
	},

	// A key that schema version 0 created as hashed has its value only in the state. A refresh and an
	// update keep it. The upgrade leaves the roles as permissions, so the config lists them.
	"hashed-key-refresh-and-update": func(h *grHarness) {
		h.apiKeys().seed(apiKeySeed{name: "hashed", hashed: true, presets: []string{"APM"}})
		s := h.UpgradeV0("upgrade a hashed key", `{"id":"`+apiKeyFirstID+`","name":"hashed","owner":{"team_id":"`+strconv.Itoa(apiKeyFakeTeamID)+`","user_id":null},"active":true,"hashed":true,"value":"old-secret","roles":[]}`)
		s = h.Refresh("refresh: the value stays in the state", s)
		s = h.Apply("update: rename", s, j{"name": "hashed-renamed", "owner": apiKeyTeam(), "presets": []any{"APM"}, "permissions": []any{}})
		h.RefreshPlan("refresh and replan", s)
	},

	// The recording shows that Get returns the value of a key that is not hashed. If it did not, the
	// value of the Create answer would be the only copy, and the state would keep it.
	"get-omits-value": func(h *grHarness) {
		h.apiKeys().omitValue = true
		s := h.Apply("create: Get leaves the value out", h.null(), apiKeyConfig("quiet", nil))
		s = h.RefreshPlan("refresh and replan", s)
		h.Apply("update: rename", s, apiKeyConfig("quiet-renamed", nil))
	},
	"import-get-omits-value": func(h *grHarness) {
		h.apiKeys().omitValue = true
		id := h.apiKeys().seed(apiKeySeed{name: "imported", presets: []string{"APM"}, permissions: []string{"livetail:Read"}})
		h.Import("import: Get leaves the value out", id)
	},

	// The 404 of a gateway without the route is no answer of the key service: the key is not gone.
	"read-route-missing": func(h *grHarness) {
		s := h.Apply("create", h.null(), apiKeyConfig("keeps", nil))
		h.apiKeys().routeMissing = true
		h.Refresh("refresh: the gateway has no such route", s)
	},
	"update-route-missing": func(h *grHarness) {
		s := h.Apply("create", h.null(), apiKeyConfig("keeps", nil))
		h.apiKeys().routeMissing = true
		h.Apply("update: the gateway has no such route", s, apiKeyConfig("keeps-renamed", nil))
	},
	"delete-route-missing": func(h *grHarness) {
		s := h.Apply("create", h.null(), apiKeyConfig("keeps", nil))
		h.apiKeys().routeMissing = true
		h.Destroy("destroy: the gateway has no such route", s)
	},

	// errors
	"create-api-error": func(h *grHarness) {
		h.apiKeys().createStatus = 500
		h.Apply("create: the API answers 500", h.null(), apiKeyConfig("broken", nil))
	},
	// The key exists and the secret is lost: the resource returns before it stores any state.
	"create-read-fails": func(h *grHarness) {
		h.apiKeys().getStatus = 500
		h.Apply("create: Get after Create answers 500", h.null(), apiKeyConfig("unread", nil))
	},
	"read-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), apiKeyConfig("gone", nil))
		h.apiKeys().forgetOutside(apiKeyFirstID)
		h.Refresh("refresh: Get answers 404", s)
	},
	"read-api-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), apiKeyConfig("broken", nil))
		h.apiKeys().getStatus = 500
		h.Refresh("refresh: Get answers 500", s)
	},
	"update-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), apiKeyConfig("gone", nil))
		h.apiKeys().forgetOutside(apiKeyFirstID)
		h.Apply("update: the API answers 404", s, apiKeyConfig("gone-renamed", nil))
	},
	"update-api-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), apiKeyConfig("broken", nil))
		h.apiKeys().updateStatus = 500
		h.Apply("update: the API answers 500", s, apiKeyConfig("broken-renamed", nil))
	},
	"update-read-fails": func(h *grHarness) {
		s := h.Apply("create", h.null(), apiKeyConfig("unread", nil))
		h.apiKeys().getStatus = 500
		h.Apply("update: Get after Update answers 500", s, apiKeyConfig("unread-renamed", nil))
	},
	"delete-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), apiKeyConfig("gone", nil))
		h.apiKeys().forgetOutside(apiKeyFirstID)
		h.Destroy("destroy: the API answers 404", s)
	},
	"delete-api-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), apiKeyConfig("broken", nil))
		h.apiKeys().deleteStatus = 500
		h.Destroy("destroy: the API answers 500", s)
	},

	// import
	"import": func(h *grHarness) {
		id := h.apiKeys().seed(apiKeySeed{name: "imported", presets: []string{"APM"}, permissions: []string{"livetail:Read"}})
		s := h.Import("import", id)
		h.Apply("plan the matching config", s, apiKeyConfig("imported", nil))
	},
	"import-with-policy": func(h *grHarness) {
		id := h.apiKeys().seed(apiKeySeed{name: "imported", presets: []string{"APM"}, permissions: []string{"livetail:Read"},
			accessPolicy: `{"version":"2025-01-01","default":{"permissions":{"team-custom-api-keys:Manage":"deny"}},"rules":[]}`})
		s := h.Import("import", id)
		h.Apply("plan a config without access_policy", s, apiKeyConfig("imported", nil))
	},
	"import-not-found": func(h *grHarness) {
		h.Import("import an id that does not exist", apiKeyFakeID(9))
	},
	"import-invalid-id": func(h *grHarness) {
		h.Import("import an id that is no uuid", "abc")
	},

	// state upgrade from schema version 0, which had roles instead of permissions
	"upgrade-v0": func(h *grHarness) {
		s := h.UpgradeV0("upgrade", `{"id":"`+apiKeyFirstID+`","name":"old","owner":{"team_id":"`+strconv.Itoa(apiKeyFakeTeamID)+`","user_id":null},"active":true,"hashed":false,"value":"old-secret","roles":["Send Data","SCIM"]}`)
		// The upgrade leaves presets null, and the schema requires it, so every config sets it.
		h.Plan("plan the permissions that the roles stand for", s, apiKeyConfig("old", j{"presets": []any{}, "permissions": []any{
			"cloud-metadata-ingress:SendData", "logs.data-ingress:SendData", "metrics.data-ingress:SendData", "spans.data-ingress:SendData",
			"team-groups:Manage", "team-groups:ReadConfig", "team-members:Manage", "team-members:ReadConfig", "team-roles:ReadConfig",
		}}))
	},
	"upgrade-v0-user-owner": func(h *grHarness) {
		h.UpgradeV0("upgrade", `{"id":"`+apiKeyFirstID+`","name":"old","owner":{"team_id":null,"user_id":"u-1"},"active":false,"hashed":true,"value":null,"roles":["Rum Ingress"]}`)
	},
	"upgrade-v0-every-role": func(h *grHarness) {
		h.UpgradeV0("upgrade", `{"id":"`+apiKeyFirstID+`","name":"old","owner":{"team_id":"1","user_id":null},"active":true,"hashed":false,"value":"v","roles":["rum ingress","SEND DATA","Coralogix CLI","scim","Role Management","Trigger Webhook","Legacy API Key","Query Data Legacy"]}`)
	},
	"upgrade-v0-unknown-role": func(h *grHarness) {
		h.UpgradeV0("upgrade", `{"id":"`+apiKeyFirstID+`","name":"old","owner":{"team_id":"1","user_id":null},"active":true,"hashed":false,"value":"v","roles":["No Such Role"]}`)
	},
	"upgrade-v0-no-roles": func(h *grHarness) {
		h.UpgradeV0("upgrade", `{"id":"`+apiKeyFirstID+`","name":"old","owner":{"team_id":"1","user_id":null},"active":true,"hashed":false,"value":"v","roles":[]}`)
	},
	"upgrade-v0-no-owner": func(h *grHarness) {
		h.UpgradeV0("upgrade", `{"id":"`+apiKeyFirstID+`","name":"old","owner":null,"active":true,"hashed":false,"value":"v","roles":["Send Data"]}`)
	},
}

var apiKeyDataSourceScenarios = map[string]func(h *grHarness){
	"read-by-id": func(h *grHarness) {
		id := h.apiKeys().seed(apiKeySeed{name: "seeded", presets: []string{"APM"}, permissions: []string{"livetail:Read"}})
		h.ReadDataSource("read by id", j{"id": id})
	},
	"read-with-policy": func(h *grHarness) {
		id := h.apiKeys().seed(apiKeySeed{name: "seeded", permissions: []string{"livetail:Read"},
			accessPolicy: `{"version":"2025-01-01","default":{"permissions":{"team-custom-api-keys:Manage":"deny"}},"rules":[]}`})
		h.ReadDataSource("read a key with a policy", j{"id": id})
	},
	"read-without-lists": func(h *grHarness) {
		id := h.apiKeys().seed(apiKeySeed{name: "seeded"})
		h.ReadDataSource("read a key without presets and permissions", j{"id": id})
	},
	"read-hashed": func(h *grHarness) {
		id := h.apiKeys().seed(apiKeySeed{name: "hashed", hashed: true, presets: []string{"APM"}})
		h.ReadDataSource("read a hashed key", j{"id": id})
	},
	"read-not-found": func(h *grHarness) {
		h.ReadDataSource("Get answers 404", j{"id": apiKeyFakeID(9)})
	},
	"read-invalid-id": func(h *grHarness) {
		h.ReadDataSource("Get answers 400 for an id that is no uuid", j{"id": "abc"})
	},
	"read-api-error": func(h *grHarness) {
		id := h.apiKeys().seed(apiKeySeed{name: "seeded", presets: []string{"APM"}})
		h.apiKeys().getStatus = 500
		h.ReadDataSource("Get answers 500", j{"id": id})
	},
	"read-without-id": func(h *grHarness) {
		h.ReadDataSource("read without id", j{})
	},
	// The config of the data source can set the other attributes. The secret and the policy of the
	// config take part in the read.
	"read-with-value-in-config": func(h *grHarness) {
		id := h.apiKeys().seed(apiKeySeed{name: "seeded", presets: []string{"APM"}})
		h.ReadDataSource("read with a value in the config", j{"id": id, "value": "from-config"})
	},
}
