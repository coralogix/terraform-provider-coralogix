// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an AS IS BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"net/http"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils/schemadump"
)

const alGoldenDir = "testdata/archive_logs_baseline"

const alTypeName = "coralogix_archive_logs"

// These tests record the behavior of the registered coralogix_archive_logs. After a change, the
// golden diff must show only listed fixes or message wording.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run TestArchiveLogsBaseline

func TestArchiveLogsBaselineSchema(t *testing.T) {
	checkGoldenAt(t, alGoldenDir, "schema_current.txt", []byte(schemadump.Text(registeredResourceSchema(t, alTypeName))))
}

func TestArchiveLogsBaselineScenarios(t *testing.T) {
	for name, run := range alScenarios {
		t.Run(name, func(t *testing.T) {
			fake := newALFake(t)
			h := newProtocolHarness(t, alTypeName, fake)
			run(h, fake)
			checkGoldenAt(t, alGoldenDir, "scenario_"+name+".json", marshalGolden(t, h.steps))
		})
	}
}

var alScenarios = map[string]func(h *grHarness, f *alFake){
	"minimal": func(h *grHarness, _ *alFake) {
		s := h.Apply("create", h.null(), j{"bucket": "archive-bucket"})
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	},

	"full-lifecycle": func(h *grHarness, _ *alFake) {
		s := h.Apply("create", h.null(), j{"bucket": "archive-bucket", "region": "eu-north-1", "active": true})
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: deactivate", s, j{"bucket": "archive-bucket", "region": "eu-north-1", "active": false})
		s = h.Apply("update: another bucket", s, j{"bucket": "other-bucket", "region": "eu-north-1", "active": false})
		h.Destroy("destroy", s)
	},

	"unset-optional-attributes": func(h *grHarness, _ *alFake) {
		s := h.Apply("create", h.null(), j{"bucket": "archive-bucket", "region": "eu-north-1", "active": false})
		s = h.Apply("update: only bucket is left", s, j{"bucket": "archive-bucket"})
		h.RefreshPlan("refresh", s)
	},

	"config-validation": func(h *grHarness, _ *alFake) {
		h.Validate("missing bucket", j{})
		h.Validate("set id", j{"bucket": "archive-bucket", "id": "mine"})
		h.Validate("set enable_tags", j{"bucket": "archive-bucket", "enable_tags": true})
	},

	"invalid-bucket": func(h *grHarness, _ *alFake) {
		h.Apply("create: the API rejects the bucket", h.null(), j{"bucket": "Invalid_Bucket"})
	},

	"import": func(h *grHarness, f *alFake) {
		f.seed(map[string]any{"bucket": "seeded-bucket", "region": "eu-north-1"}, true)
		s := h.Import("import", "archive-logs-settings")
		h.Apply("plan the matching config", s, j{"bucket": "seeded-bucket"})
	},

	"import-another-id": func(h *grHarness, f *alFake) {
		f.seed(map[string]any{"bucket": "seeded-bucket", "region": "eu-north-1"}, false)
		s := h.Import("import with an id that is not the fixed one", "anything")
		h.Apply("plan the matching config", s, j{"bucket": "seeded-bucket", "active": false})
	},

	"import-without-target": func(h *grHarness, _ *alFake) {
		h.Import("import when the company has no target", "archive-logs-settings")
	},

	"read-without-target": func(h *grHarness, f *alFake) {
		s := h.Apply("create", h.null(), j{"bucket": "archive-bucket"})
		f.forget()
		h.RefreshPlan("refresh: the API returns an empty IBM COS target", s)
	},

	"other-region": func(h *grHarness, _ *alFake) {
		h.Apply("create: the API accepts only the region of the environment", h.null(), j{"bucket": "archive-bucket", "region": "ap-southeast-2"})
	},

	"enable-tags-kept": func(h *grHarness, f *alFake) {
		f.seed(map[string]any{"bucket": "seeded-bucket", "region": "eu-north-1"}, true)
		f.target["enableTags"] = true
		s := h.Import("import a target with tags enabled", "archive-logs-settings")
		h.Apply("update: the API keeps enable_tags", s, j{"bucket": "seeded-bucket", "active": false})
	},

	"route-missing": func(h *grHarness, f *alFake) {
		s := h.Apply("create", h.null(), j{"bucket": "archive-bucket"})
		f.failGets(http.StatusNotFound, "Not Found", "Not Found")
		h.RefreshPlan("refresh: the gateway has no such route", s)
	},

	"api-error": func(h *grHarness, f *alFake) {
		s := h.Apply("create", h.null(), j{"bucket": "archive-bucket"})
		f.failGets(http.StatusInternalServerError, "Internal", "archive target store unavailable")
		h.RefreshPlan("refresh: the API fails", s)
	},

	// The API never answers Set with a 404, because the target always exists. The case records
	// what the resource does if it did.
	"update-not-found": func(h *grHarness, f *alFake) {
		s := h.Apply("create", h.null(), j{"bucket": "archive-bucket"})
		f.failSets(http.StatusNotFound, "Not Found", "archive target not found")
		h.Apply("update: the API does not find the target", s, j{"bucket": "archive-bucket", "active": false})
	},

	"existing-target": func(h *grHarness, f *alFake) {
		f.seed(map[string]any{"bucket": "old-bucket", "region": "eu-north-1"}, true)
		h.Apply("create sets the target that the company already has", h.null(), j{"bucket": "archive-bucket"})
	},
}

// alDataSourceScenarios read the one archive target of the company. The data source takes no input.
var alDataSourceScenarios = map[string]func(h *grHarness){
	"read": func(h *grHarness) {
		h.archiveLogs().seed(map[string]any{"bucket": "seeded-bucket", "region": "eu-north-1"}, true)
		h.ReadDataSource("read the archive target", j{})
	},
	"read-without-target": func(h *grHarness) {
		h.ReadDataSource("read: the API returns an empty IBM COS target", j{})
	},
	"route-missing": func(h *grHarness) {
		h.archiveLogs().failGets(http.StatusNotFound, "Not Found", "Not Found")
		h.ReadDataSource("read: the gateway has no such route", j{})
	},
	"api-error": func(h *grHarness) {
		h.archiveLogs().failGets(http.StatusInternalServerError, "Internal", "archive target store unavailable")
		h.ReadDataSource("read: the API fails", j{})
	},
}

func (h *grHarness) archiveLogs() *alFake {
	h.t.Helper()
	f, ok := h.api.(*alFake)
	if !ok {
		h.t.Fatal("expected *alFake")
	}
	return f
}
