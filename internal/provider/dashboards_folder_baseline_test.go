// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
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
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils/schemadump"
)

const (
	dashboardsFolderTypeName  = "coralogix_dashboards_folder"
	dashboardsFolderGoldenDir = "testdata/dashboards_folder_baseline"
	dashboardsFolderParent    = "11111111-1111-4111-8111-111111111111"
)

// These tests record the behavior of the handwritten coralogix_dashboards_folder. After
// the generator switch, the golden diff must show only message wording.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run TestDashboardsFolderBaseline

func newDashboardsFolderHarness(t *testing.T) *grHarness {
	fake := newDashboardsFolderFake(t)
	// The fixture parents exist, as the API checks the parent of a folder.
	fake.seeded[dashboardsFolderParent] = true
	fake.seeded["22222222-2222-4222-8222-222222222222"] = true
	return newProtocolHarness(t, dashboardsFolderTypeName, fake)
}

func (h *grHarness) dashboardsFolders() *dashboardsFolderFake {
	h.t.Helper()
	f, ok := h.api.(*dashboardsFolderFake)
	if !ok {
		h.t.Fatal("expected *dashboardsFolderFake")
	}
	return f
}

func TestDashboardsFolderBaselineSchema(t *testing.T) {
	var current resource.SchemaResponse
	dashboards.NewDashboardsFolderResource().Schema(context.Background(), resource.SchemaRequest{}, &current)
	checkGoldenAt(t, dashboardsFolderGoldenDir, "schema_current.txt", []byte(schemadump.Text(current.Schema)))
}

func TestDashboardsFolderBaselineScenarios(t *testing.T) {
	for name, run := range dashboardsFolderScenarios {
		t.Run(name, func(t *testing.T) {
			h := newDashboardsFolderHarness(t)
			run(h)
			checkGoldenAt(t, dashboardsFolderGoldenDir, "scenario_"+name+".json", maskFolderIDs(marshalGolden(t, h.steps)))
		})
	}
}

var dashboardsFolderScenarios = map[string]func(h *grHarness){
	"minimal": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"name": "minimal"})
		s = h.Refresh("refresh", s)
		h.Destroy("destroy", s)
	},

	"full-lifecycle": func(h *grHarness) {
		s := h.Apply("create with a parent", h.null(), j{"name": "main", "parent_id": dashboardsFolderParent})
		s = h.Refresh("refresh", s)
		s = h.Apply("update: rename", s, j{"name": "main-renamed", "parent_id": dashboardsFolderParent})
		s = h.Apply("update: move to another parent", s, j{"name": "main-renamed", "parent_id": "22222222-2222-4222-8222-222222222222"})
		s = h.Apply("update: remove the parent", s, j{"name": "main-renamed"})
		h.Refresh("refresh", s)
		h.Destroy("destroy", s)
	},

	"config-validation": func(h *grHarness) {
		h.Validate("empty config", j{})
		h.Validate("empty name", j{"name": ""})
		h.Validate("name longer than 100 characters", j{"name": longName(101)})
	},

	"name-omitted": func(h *grHarness) {
		h.Apply("create without a name", h.null(), j{})
	},

	"empty-name": func(h *grHarness) {
		h.Apply("create with an empty name", h.null(), j{"name": ""})
	},

	"empty-parent": func(h *grHarness) {
		h.Apply("create with parent_id = \"\"", h.null(), j{"name": "empty-parent", "parent_id": ""})
		s := h.Apply("create without a parent", h.null(), j{"name": "no-parent"})
		h.Apply("update: set parent_id = \"\"", s, j{"name": "no-parent", "parent_id": ""})
	},

	"update-to-duplicate-name": func(h *grHarness) {
		h.Apply("create the first folder", h.null(), j{"name": "first"})
		s := h.Apply("create the second folder", h.null(), j{"name": "second"})
		s = h.Apply("update: rename to the name of the first", s, j{"name": "first"})
		h.Refresh("refresh", s)
	},

	"import": func(h *grHarness) {
		id := h.dashboardsFolders().seed(j{"name": "seeded", "parentId": dashboardsFolderParent})
		s := h.Import("import", id)
		h.Apply("plan the matching config", s, j{"name": "seeded", "parent_id": dashboardsFolderParent})
	},

	"import-unknown-id": func(h *grHarness) {
		h.Import("import an id that does not exist", "00000000-0000-4000-8000-00000000dead")
	},

	"read-malformed-id": func(h *grHarness) {
		h.Import("import an id that is not a UUID", "not-a-uuid")
	},

	"read-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"name": "gone"})
		h.dashboardsFolders().forget(idOf(s))
		h.Refresh("refresh: folder was deleted outside Terraform", s)
	},

	"delete-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"name": "gone"})
		h.dashboardsFolders().forget(idOf(s))
		h.Destroy("destroy: the API answers 404 for a folder that does not exist", s)
	},

	"update-not-found": func(h *grHarness) {
		s := h.Apply("create", h.null(), j{"name": "gone"})
		h.dashboardsFolders().forget(idOf(s))
		h.Apply("update: folder was deleted outside Terraform", s, j{"name": "gone-renamed"})
	},
}

func longName(n int) string {
	out := make([]byte, n)
	for i := range out {
		out[i] = 'a'
	}
	return string(out)
}

var folderUUID = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)

// maskFolderIDs replaces each folder id that the provider generated with a stable placeholder,
// in order of first appearance. The ids of the fixtures stay as they are.
func maskFolderIDs(golden []byte) []byte {
	seen := map[string]string{}
	return folderUUID.ReplaceAllFunc(golden, func(id []byte) []byte {
		switch string(id) {
		case dashboardsFolderParent, "22222222-2222-4222-8222-222222222222":
			return id
		}
		if strings.HasPrefix(string(id), "00000000-0000-4000-8000-") {
			return id
		}
		if _, ok := seen[string(id)]; !ok {
			seen[string(id)] = fmt.Sprintf("<generated-id-%d>", len(seen)+1)
		}
		return []byte(seen[string(id)])
	})
}
