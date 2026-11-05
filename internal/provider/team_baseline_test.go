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
	teamTypeName  = "coralogix_team"
	teamGoldenDir = "testdata/team_baseline"
	teamBasePath  = "/aaa/teams/v2"
)

// These tests record the handwritten coralogix_team resource against a fake API that follows
// answers recorded from the real API. After the switch to generated code, the golden diff must
// show only message wording or proven bug fixes. The data source is new with the generated
// resource: the released provider registered none.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run TestTeamBaseline
func newTeamHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, teamTypeName, newTeamFake(t))
}

func TestTeamBaselineSchema(t *testing.T) {
	var current resource.SchemaResponse
	aaa.NewTeamResource().Schema(context.Background(), resource.SchemaRequest{}, &current)
	checkGoldenAt(t, teamGoldenDir, "schema_current.txt", []byte(schemadump.Text(current.Schema)))
}

func TestTeamBaselineScenarios(t *testing.T) {
	for name, run := range teamScenarios {
		t.Run(name, func(t *testing.T) {
			h := newTeamHarness(t)
			run(h)
			checkGoldenAt(t, teamGoldenDir, "scenario_"+name+".json", marshalGolden(t, h.steps))
		})
	}
}

// teamFakeID is the id that the fake gives the n-th team of a scenario, as Terraform shows it.
func teamFakeID(n int) string {
	return strconv.Itoa(teamFirstID + n - 1)
}

var teamScenarios = map[string]func(h *grHarness){
	"full-lifecycle": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"name": "platform", "daily_quota": 0.5})
		s = h.RefreshPlan("refresh and replan", s)
		s = h.Apply("update: rename", s, j{"name": "platform-renamed", "daily_quota": 0.5})
		s = h.Apply("update: change quota", s, j{"name": "platform-renamed", "daily_quota": 0.25})
		s = h.RefreshPlan("refresh and replan after update", s)
		h.Destroy("destroy", s)
	},
	// An increase reaches the configured quota. A decrease (full-lifecycle) leaves 0.01 more.
	"quota-increase": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"name": "growing", "daily_quota": 0.25})
		s = h.Apply("update: raise the quota", s, j{"name": "growing", "daily_quota": 0.5})
		h.RefreshPlan("refresh and replan", s)
	},
	// The minimum quota reads back as float32(0.02), which state shows as 0.02. Delete then fails:
	// the API refuses to give the minimum quota back to the default team.
	"quota-omitted": func(h *grHarness) {
		s := h.Apply("create without daily_quota", h.null(), j{"name": "minimum"})
		s = h.RefreshPlan("refresh and replan", s)
		h.Destroy("destroy: the API refuses the minimum quota", s)
	},
	// Removing daily_quota keeps the state value, and Update sends it again.
	"quota-removed-on-update": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"name": "removed", "daily_quota": 0.5})
		s = h.Apply("update: remove daily_quota and rename", s, j{"name": "removed-renamed"})
		h.RefreshPlan("refresh and replan", s)
	},
	// A quota with more than three decimals. The handwritten resource rounded the read to three
	// decimals, which differed from the plan.
	"quota-more-than-three-decimals": func(h *grHarness) {
		h.Apply("create with 0.1234567", h.null(), j{"name": "precise", "daily_quota": 0.1234567})
	},
	"quota-more-than-three-decimals-on-update": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"name": "precise", "daily_quota": 0.5})
		h.Apply("update to 0.1234567", s, j{"name": "precise", "daily_quota": 0.1234567})
	},
	// The API ignores a quota of 0.01 or less on Create and stores the minimum quota.
	"quota-at-most-minimum": func(h *grHarness) {
		h.Apply("create with 0.005", h.null(), j{"name": "tiny", "daily_quota": 0.005})
	},
	"quota-zero-on-update": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"name": "zero", "daily_quota": 0.5})
		h.Apply("update to 0", s, j{"name": "zero", "daily_quota": 0.0})
	},
	"name-missing": func(h *grHarness) {
		h.Validate("validate without name", j{"daily_quota": 0.5})
	},
	"name-empty": func(h *grHarness) {
		h.Apply("create with an empty name", h.null(), j{"name": "", "daily_quota": 0.5})
	},
	"name-duplicate": func(h *grHarness) {
		h.team().seed("taken", 0.5)
		h.Apply("create with a name that another team has", h.null(), j{"name": "taken", "daily_quota": 0.5})
	},
	"rename-to-taken-name": func(h *grHarness) {
		h.team().seed("taken", 0.5)
		s := h.Apply("create", h.null(), j{"name": "mine", "daily_quota": 0.5})
		h.Apply("update: rename to a name that another team has", s, j{"name": "taken", "daily_quota": 0.5})
	},
	"rename-to-empty-name": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"name": "mine", "daily_quota": 0.5})
		h.Apply("update: rename to an empty name", s, j{"name": "", "daily_quota": 0.5})
	},
	"create-read-fails": func(h *grHarness) {
		h.team().getStatus = 500
		h.Apply("create: Get after Create answers 500", h.null(), j{"name": "unread", "daily_quota": 0.5})
	},
	"read-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"name": "gone", "daily_quota": 0.5})
		h.team().forgetOutside(teamFirstID)
		h.Refresh("refresh: Get answers 404", s)
	},
	// The API answers 400, not 404, for a team that was deleted, so the refresh fails.
	"read-after-delete-outside": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"name": "deleted", "daily_quota": 0.5})
		h.team().deleteOutside(teamFirstID)
		h.Refresh("refresh: Get answers 400 for a deleted team", s)
	},
	"read-api-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"name": "broken", "daily_quota": 0.5})
		h.team().getStatus = 500
		h.Refresh("refresh: Get answers 500", s)
	},
	"update-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"name": "gone", "daily_quota": 0.5})
		h.team().forgetOutside(teamFirstID)
		h.Apply("update: the API answers 404", s, j{"name": "gone-renamed", "daily_quota": 0.5})
	},
	"update-after-delete-outside": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"name": "deleted", "daily_quota": 0.5})
		h.team().deleteOutside(teamFirstID)
		h.Apply("update: the API answers 412 for a deleted team", s, j{"name": "deleted-renamed", "daily_quota": 0.5})
	},
	"update-read-fails": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"name": "unread", "daily_quota": 0.5})
		h.team().getStatus = 500
		h.Apply("update: Get after Update answers 500", s, j{"name": "unread-renamed", "daily_quota": 0.5})
	},
	"delete-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"name": "gone", "daily_quota": 0.5})
		h.team().forgetOutside(teamFirstID)
		h.Destroy("destroy: the API answers 404", s)
	},
	"delete-after-delete-outside": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"name": "deleted", "daily_quota": 0.5})
		h.team().deleteOutside(teamFirstID)
		h.Destroy("destroy: the API answers 400 for a deleted team", s)
	},
	"delete-api-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"name": "broken", "daily_quota": 0.5})
		h.team().deleteStatus = 500
		h.Destroy("destroy: the API answers 500", s)
	},
	"import": func(h *grHarness) {
		id := h.team().seed("imported", 0.3)
		s := h.Import("import", strconv.FormatInt(id, 10))
		h.Apply("plan the matching config", s, j{"name": "imported", "daily_quota": 0.3})
	},
	"import-without-quota-in-config": func(h *grHarness) {
		id := h.team().seed("imported", 0.3)
		s := h.Import("import", strconv.FormatInt(id, 10))
		h.Apply("plan a config without daily_quota", s, j{"name": "imported"})
	},
	"import-not-a-number": func(h *grHarness) {
		h.Import("import a non-numeric id", "not-a-number")
	},
	"import-not-found": func(h *grHarness) {
		h.Import("import an id that does not exist", teamFakeID(9))
	},
}

var teamDataSourceScenarios = map[string]func(h *grHarness){
	"read-by-id": func(h *grHarness) {
		id := h.team().seed("seeded", 0.3)
		h.ReadDataSource("read by id", j{"id": strconv.FormatInt(id, 10)})
	},
	"read-not-found": func(h *grHarness) {
		h.ReadDataSource("Get answers 404", j{"id": teamFakeID(9)})
	},
	"read-api-error": func(h *grHarness) {
		id := h.team().seed("seeded", 0.3)
		h.team().getStatus = 500
		h.ReadDataSource("Get answers 500", j{"id": strconv.FormatInt(id, 10)})
	},
	"read-not-a-number": func(h *grHarness) {
		h.ReadDataSource("read a non-numeric id", j{"id": "not-a-number"})
	},
}
