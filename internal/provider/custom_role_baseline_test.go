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
	customRoleTypeName  = "coralogix_custom_role"
	customRoleGoldenDir = "testdata/custom_role_baseline"
	customRoleBasePath  = "/aaa/custom-roles/v1"

	crUpdateConfig = "spans.events2metrics:UpdateConfig"
	crReadConfig   = "spans.events2metrics:ReadConfig"
	crAlertsRead   = "alerts:ReadConfig"
)

// These tests record the handwritten coralogix_custom_role resource and its data source against a
// fake API that follows answers recorded from the real API. After the switch to generated code,
// the golden diff must show only message wording or proven bug fixes.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run 'TestCustomRoleBaseline|TestDataSourceBaseline/custom_role'
func newCustomRoleHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, customRoleTypeName, newCustomRoleFake(t))
}

func TestCustomRoleBaselineSchema(t *testing.T) {
	var current resource.SchemaResponse
	aaa.NewCustomRoleSource().Schema(context.Background(), resource.SchemaRequest{}, &current)
	checkGoldenAt(t, customRoleGoldenDir, "schema_current.txt", []byte(schemadump.Text(current.Schema)))
}

func TestCustomRoleBaselineScenarios(t *testing.T) {
	for name, run := range customRoleScenarios {
		t.Run(name, func(t *testing.T) {
			h := newCustomRoleHarness(t)
			run(h)
			checkGoldenAt(t, customRoleGoldenDir, "scenario_"+name+".json", marshalGolden(t, h.steps))
		})
	}
}

// crConfig is the configuration of a role with the parent "Standard User".
func crConfig(name string, permissions ...string) j {
	elements := make(l, 0, len(permissions))
	for _, permission := range permissions {
		elements = append(elements, permission)
	}
	return j{"name": name, "description": "made with Terraform", "parent_role": "Standard User", "permissions": elements}
}

// crWith is a copy of config with the attributes of changes.
func crWith(config j, changes j) j {
	out := j{}
	for key, value := range config {
		out[key] = value
	}
	for key, value := range changes {
		out[key] = value
	}
	return out
}

// crFakeID is the id that the fake gives the n-th role of a scenario, as Terraform shows it.
func crFakeID(n int) string {
	return strconv.Itoa(customRoleFirstID + n - 1)
}

var customRoleScenarios = map[string]func(h *grHarness){
	"full-lifecycle": func(h *grHarness) {
		s := h.Apply("create", h.null(), crConfig("auditors", crUpdateConfig))
		s = h.RefreshPlan("refresh and replan", s)
		s = h.Apply("update: rename", s, crConfig("auditors-renamed", crUpdateConfig))
		s = h.Apply("update: change the description", s, crWith(crConfig("auditors-renamed", crUpdateConfig), j{"description": "changed"}))
		s = h.Apply("update: add a permission", s, crWith(crConfig("auditors-renamed", crUpdateConfig, crReadConfig), j{"description": "changed"}))
		s = h.RefreshPlan("refresh and replan after update", s)
		h.Destroy("destroy", s)
	},
	"permissions-removed-on-update": func(h *grHarness) {
		s := h.Apply("create with two permissions", h.null(), crConfig("auditors", crUpdateConfig, crReadConfig))
		s = h.Apply("update: remove one permission", s, crConfig("auditors", crReadConfig))
		h.RefreshPlan("refresh and replan", s)
	},
	// The API keeps the case of a permission. The state keeps the case of the configuration.
	"permissions-upper-case": func(h *grHarness) {
		s := h.Apply("create with upper case permissions", h.null(), crConfig("shouting", "SPANS.EVENTS2METRICS:UPDATECONFIG"))
		s = h.RefreshPlan("refresh and replan", s)
		s = h.Apply("update: add an upper case permission", s, crConfig("shouting", "SPANS.EVENTS2METRICS:UPDATECONFIG", "SPANS.EVENTS2METRICS:READCONFIG"))
		h.RefreshPlan("refresh and replan after update", s)
	},
	"permissions-case-changed-to-config": func(h *grHarness) {
		s := h.Apply("create", h.null(), crConfig("shouting", crUpdateConfig))
		s = h.Apply("update: change only the case", s, crConfig("shouting", "SPANS.EVENTS2METRICS:UPDATECONFIG"))
		h.RefreshPlan("refresh and replan", s)
	},
	// The role was created outside Terraform with another case than the configuration. The refresh
	// shows the case of the configuration.
	"permissions-case-differs-outside": func(h *grHarness) {
		s := h.Apply("create", h.null(), crConfig("outside", crUpdateConfig))
		h.customRoles().roles[customRoleFirstID].permissions = []string{"SPANS.EVENTS2METRICS:UPDATECONFIG"}
		s = h.Refresh("refresh: the API has another case", s)
		h.RefreshPlan("replan", s)
	},
	"permissions-changed-outside": func(h *grHarness) {
		s := h.Apply("create", h.null(), crConfig("outside", crUpdateConfig))
		h.customRoles().roles[customRoleFirstID].permissions = []string{crUpdateConfig, crAlertsRead}
		s = h.Refresh("refresh: the API has another permission", s)
		h.Apply("apply the configuration again", s, crConfig("outside", crUpdateConfig))
	},
	// The role lost a permission outside Terraform. The refresh shows the reduced set, and the
	// plan puts the permission back.
	"permissions-removed-outside": func(h *grHarness) {
		s := h.Apply("create with two permissions", h.null(), crConfig("outside", crUpdateConfig, crReadConfig))
		h.customRoles().roles[customRoleFirstID].permissions = []string{crUpdateConfig}
		s = h.Refresh("refresh: the API has fewer permissions", s)
		h.Apply("apply the configuration again", s, crConfig("outside", crUpdateConfig, crReadConfig))
	},
	"description-changed-outside": func(h *grHarness) {
		s := h.Apply("create", h.null(), crConfig("outside", crUpdateConfig))
		h.customRoles().roles[customRoleFirstID].description = "changed in the UI"
		s = h.Refresh("refresh: the API has another description", s)
		h.Apply("apply the configuration again", s, crConfig("outside", crUpdateConfig))
	},
	"description-empty": func(h *grHarness) {
		s := h.Apply("create with an empty description", h.null(), crWith(crConfig("quiet", crUpdateConfig), j{"description": ""}))
		h.RefreshPlan("refresh and replan", s)
	},
	"name-empty": func(h *grHarness) {
		s := h.Apply("create with an empty name", h.null(), crConfig("", crUpdateConfig))
		h.RefreshPlan("refresh and replan", s)
	},
	"name-missing": func(h *grHarness) {
		h.Validate("validate without name", j{"description": "d", "parent_role": "Standard User", "permissions": l{crUpdateConfig}})
	},
	"description-missing": func(h *grHarness) {
		h.Validate("validate without description", j{"name": "n", "parent_role": "Standard User", "permissions": l{crUpdateConfig}})
	},
	"parent-role-missing": func(h *grHarness) {
		h.Validate("validate without parent_role", j{"name": "n", "description": "d", "permissions": l{crUpdateConfig}})
	},
	"permissions-missing": func(h *grHarness) {
		h.Validate("validate without permissions", j{"name": "n", "description": "d", "parent_role": "Standard User"})
	},
	// The API accepts a role without permissions, and the resource refuses it.
	"permissions-empty": func(h *grHarness) {
		h.Validate("validate an empty permissions set", crConfig("n"))
	},
	"permissions-empty-string": func(h *grHarness) {
		h.Apply("create with an empty permission", h.null(), crConfig("n", ""))
	},
	// Terraform refuses a set with the same element twice.
	"permissions-duplicates": func(h *grHarness) {
		h.Apply("create with the same permission twice", h.null(), crConfig("n", crUpdateConfig, crUpdateConfig))
	},
	// The API keeps one of two permissions that differ only in case, the first in ASCII order, so
	// the state has one element where the plan has two.
	"permissions-differ-only-in-case": func(h *grHarness) {
		h.Apply("create with a permission in two cases", h.null(), crConfig("n", crUpdateConfig, "SPANS.EVENTS2METRICS:UPDATECONFIG"))
	},
	"permissions-differ-only-in-case-on-update": func(h *grHarness) {
		s := h.Apply("create", h.null(), crConfig("n", crUpdateConfig))
		h.Apply("update: add the same permission in another case", s, crConfig("n", crUpdateConfig, "SPANS.EVENTS2METRICS:UPDATECONFIG"))
	},
	"permission-unknown": func(h *grHarness) {
		h.Apply("create with an unknown permission", h.null(), crConfig("n", "nope:Nope"))
	},
	"permission-unknown-on-update": func(h *grHarness) {
		s := h.Apply("create", h.null(), crConfig("n", crUpdateConfig))
		h.Apply("update: add an unknown permission", s, crConfig("n", crUpdateConfig, "nope:Nope"))
	},
	"parent-role-unknown": func(h *grHarness) {
		h.Apply("create with an unknown parent role", h.null(), crWith(crConfig("n", crUpdateConfig), j{"parent_role": "Nope Role"}))
	},
	"parent-role-lower-case": func(h *grHarness) {
		h.Apply("create with a parent role in lower case", h.null(), crWith(crConfig("n", crUpdateConfig), j{"parent_role": "standard user"}))
	},
	"parent-role-other": func(h *grHarness) {
		s := h.Apply("create", h.null(), crWith(crConfig("n", crUpdateConfig), j{"parent_role": "Read-Only User"}))
		h.RefreshPlan("refresh and replan", s)
	},
	// Update does not send the parent role, so the API keeps it.
	"parent-role-change": func(h *grHarness) {
		s := h.Apply("create", h.null(), crConfig("n", crUpdateConfig))
		h.Apply("update: change parent_role", s, crWith(crConfig("n", crUpdateConfig), j{"parent_role": "Read-Only User"}))
	},
	"name-duplicate": func(h *grHarness) {
		h.customRoles().seed("taken", []string{crUpdateConfig})
		h.Apply("create with a name that another role has", h.null(), crConfig("taken", crUpdateConfig))
	},
	"rename-to-taken-name": func(h *grHarness) {
		h.customRoles().seed("taken", []string{crUpdateConfig})
		s := h.Apply("create", h.null(), crConfig("mine", crUpdateConfig))
		h.Apply("update: rename to a name that another role has", s, crConfig("taken", crUpdateConfig))
	},
	"rename-to-empty-name": func(h *grHarness) {
		s := h.Apply("create", h.null(), crConfig("mine", crUpdateConfig))
		h.Apply("update: rename to an empty name", s, crConfig("", crUpdateConfig))
	},
	"create-api-error": func(h *grHarness) {
		h.customRoles().createStatus = 500
		h.Apply("create: the API answers 500", h.null(), crConfig("n", crUpdateConfig))
	},
	"create-read-fails": func(h *grHarness) {
		h.customRoles().getStatus = 500
		s := h.Apply("create: Get after Create answers 500", h.null(), crConfig("n", crUpdateConfig))
		h.customRoles().getStatus = 0
		h.Apply("apply again", s, crConfig("n", crUpdateConfig))
	},
	"read-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), crConfig("gone", crUpdateConfig))
		h.customRoles().forgetOutside(customRoleFirstID)
		h.Refresh("refresh: Get answers 404", s)
	},
	"read-api-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), crConfig("broken", crUpdateConfig))
		h.customRoles().getStatus = 500
		h.Refresh("refresh: Get answers 500", s)
	},
	"update-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), crConfig("gone", crUpdateConfig))
		h.customRoles().forgetOutside(customRoleFirstID)
		h.Apply("update: the API answers 404", s, crConfig("gone-renamed", crUpdateConfig))
	},
	"update-api-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), crConfig("broken", crUpdateConfig))
		h.customRoles().updateStatus = 500
		h.Apply("update: the API answers 500", s, crConfig("broken-renamed", crUpdateConfig))
	},
	"update-read-fails": func(h *grHarness) {
		s := h.Apply("create", h.null(), crConfig("unread", crUpdateConfig))
		h.customRoles().getStatus = 500
		h.Apply("update: Get after Update answers 500", s, crConfig("unread-renamed", crUpdateConfig))
	},
	// The Update succeeds and the Get after it fails once. Without a refresh, the state keeps the
	// old name, so the next apply sends the Update again (terraform apply -refresh=false).
	"update-read-fails-then-succeeds": func(h *grHarness) {
		s := h.Apply("create", h.null(), crConfig("unread", crUpdateConfig))
		h.customRoles().getStatus = 500
		s = h.Apply("update: Get after Update answers 500", s, crConfig("unread-renamed", crUpdateConfig))
		h.customRoles().getStatus = 0
		s = h.Apply("apply again: Get answers", s, crConfig("unread-renamed", crUpdateConfig))
		h.RefreshPlan("refresh and replan", s)
	},
	// The same failure, then a normal run: the plan refreshes the role first, so the state has the
	// new name and the apply sends no second Update.
	"update-read-fails-then-refresh": func(h *grHarness) {
		s := h.Apply("create", h.null(), crConfig("unread", crUpdateConfig))
		h.customRoles().getStatus = 500
		s = h.Apply("update: Get after Update answers 500", s, crConfig("unread-renamed", crUpdateConfig))
		h.customRoles().getStatus = 0
		s = h.Refresh("refresh: Get answers", s)
		s = h.Apply("apply again after the refresh", s, crConfig("unread-renamed", crUpdateConfig))
		h.RefreshPlan("refresh and replan", s)
	},
	"delete-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), crConfig("gone", crUpdateConfig))
		h.customRoles().forgetOutside(customRoleFirstID)
		h.Destroy("destroy: the API answers 404", s)
	},
	"delete-api-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), crConfig("broken", crUpdateConfig))
		h.customRoles().deleteStatus = 500
		h.Destroy("destroy: the API answers 500", s)
	},
	"import": func(h *grHarness) {
		id := h.customRoles().seed("imported", []string{crUpdateConfig, crReadConfig})
		s := h.Import("import", strconv.FormatInt(id, 10))
		h.Apply("plan the matching config", s, crConfig("imported", crUpdateConfig, crReadConfig))
	},
	// The seeded role has the description "seeded", and the configuration has another.
	"import-then-update": func(h *grHarness) {
		id := h.customRoles().seed("imported", []string{crUpdateConfig})
		s := h.Import("import", strconv.FormatInt(id, 10))
		h.Apply("update to the configuration", s, crConfig("imported", crUpdateConfig, crReadConfig))
	},
	"import-upper-case": func(h *grHarness) {
		id := h.customRoles().seed("imported", []string{"SPANS.EVENTS2METRICS:UPDATECONFIG"})
		s := h.Import("import a role with upper case permissions", strconv.FormatInt(id, 10))
		h.Apply("plan a config with lower case permissions", s, crWith(crConfig("imported", crUpdateConfig), j{"description": "seeded"}))
	},
	"import-not-a-number": func(h *grHarness) {
		h.Import("import a non-numeric id", "not-a-number")
	},
	"import-not-found": func(h *grHarness) {
		h.Import("import an id that does not exist", crFakeID(9))
	},
	"import-api-error": func(h *grHarness) {
		id := h.customRoles().seed("imported", []string{crUpdateConfig})
		h.customRoles().getStatus = 500
		h.Import("import: Get answers 500", strconv.FormatInt(id, 10))
	},
}

// The data source finds a role by id with Get, and by name in the list. The list shows the
// permissions of the role together with those of its parent role.
var customRoleDataSourceScenarios = map[string]func(h *grHarness){
	"read-by-id": func(h *grHarness) {
		id := h.customRoles().seed("seeded", []string{crUpdateConfig, crReadConfig})
		h.ReadDataSource("read by id", j{"id": strconv.FormatInt(id, 10)})
	},
	"read-by-name": func(h *grHarness) {
		h.customRoles().seed("other", []string{crAlertsRead})
		h.customRoles().seed("seeded", []string{crUpdateConfig})
		h.ReadDataSource("read by name", j{"name": "seeded"})
	},
	"duplicate-name": func(h *grHarness) {
		h.customRoles().seed("seeded", []string{crUpdateConfig})
		h.customRoles().seedDuplicate("seeded")
		h.ReadDataSource("read a name that two roles have", j{"name": "seeded"})
	},
	"not-found": func(h *grHarness) {
		h.ReadDataSource("read an id that does not exist", j{"id": crFakeID(9)})
	},
	"name-not-found": func(h *grHarness) {
		h.customRoles().seed("seeded", []string{crUpdateConfig})
		h.ReadDataSource("read a name that does not exist", j{"name": "missing"})
	},
	"id-and-name": func(h *grHarness) {
		id := h.customRoles().seed("seeded", []string{crUpdateConfig})
		h.ReadDataSource("config sets id and name", j{"id": strconv.FormatInt(id, 10), "name": "seeded"})
	},
	"no-id-or-name": func(h *grHarness) {
		h.ReadDataSource("config sets neither id nor name", j{})
	},
	"id-not-a-number": func(h *grHarness) {
		h.ReadDataSource("read an id that is no number", j{"id": "not-a-number"})
	},
	// A name that is empty is no name: the data source asks for an id or a name.
	"name-empty": func(h *grHarness) {
		h.customRoles().seed("", []string{crUpdateConfig})
		h.ReadDataSource("read the empty name", j{"name": ""})
	},
	"read-by-id-api-error": func(h *grHarness) {
		id := h.customRoles().seed("seeded", []string{crUpdateConfig})
		h.customRoles().getStatus = 500
		h.ReadDataSource("Get answers 500", j{"id": strconv.FormatInt(id, 10)})
	},
	"read-by-name-api-error": func(h *grHarness) {
		h.customRoles().seed("seeded", []string{crUpdateConfig})
		h.customRoles().listStatus = 500
		h.ReadDataSource("the list answers 500", j{"name": "seeded"})
	},
	// The role has no permissions of its own, so Get shows none and the list shows its parent's.
	"read-without-permissions": func(h *grHarness) {
		id := h.customRoles().seed("seeded", nil)
		h.ReadDataSource("read by id: the role has no permissions", j{"id": strconv.FormatInt(id, 10)})
		h.ReadDataSource("read by name: the role has no permissions", j{"name": "seeded"})
	},
}
