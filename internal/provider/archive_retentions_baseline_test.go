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
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils/schemadump"
)

const arTypeName = "coralogix_archive_retentions"

// These tests record the behavior of the registered coralogix_archive_retentions. After a change,
// the golden diff must show only listed fixes or message wording.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run TestArchiveRetentionsBaseline

func TestArchiveRetentionsBaselineSchema(t *testing.T) {
	checkGoldenAt(t, arGoldenDir, "schema_current.txt", []byte(schemadump.Text(registeredResourceSchema(t, arTypeName))))
}

func TestArchiveRetentionsBaselineScenarios(t *testing.T) {
	for name, run := range arScenarios {
		t.Run(name, func(t *testing.T) {
			fake := newARFake(t)
			h := newProtocolHarness(t, arTypeName, fake)
			run(h, fake)
			checkGoldenAt(t, arGoldenDir, "scenario_"+name+".json", marshalGolden(t, h.steps))
		})
	}
}

// arConfig is a configuration with the given names of the four retentions. A nil name leaves the
// name out.
func arConfig(names ...any) j {
	items := []any{}
	for _, name := range names {
		item := j{}
		if name != nil {
			item["name"] = name
		}
		items = append(items, item)
	}
	return j{"retentions": items}
}

// arNamed is a configuration that names retentions 2 to 4 and leaves the first out, as it must.
func arNamed(second, third, fourth string) j { return arConfig(nil, second, third, fourth) }

var arScenarios = map[string]func(h *grHarness, f *arFake){
	"full-lifecycle": func(h *grHarness, _ *arFake) {
		s := h.Apply("create", h.null(), arNamed("short_term", "mid_term", "long_term"))
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: one name", s, arNamed("short_term", "medium_term", "long_term"))
		s = h.Apply("update: every name", s, arNamed("a1", "b2", "c3"))
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy: the API keeps the names", s)
	},

	// The names that the company already has are the default ones, so the plan shows no change in
	// the API.
	"default-names": func(h *grHarness, _ *arFake) {
		s := h.Apply("create with the names that the company has", h.null(), arNamed("Short", "Intermediate", "Long"))
		h.RefreshPlan("refresh", s)
	},

	// A name left out is unknown in the plan, and Create sends it as "". The API rejects that.
	"names-not-set": func(h *grHarness, _ *arFake) {
		h.Apply("create: no names", h.null(), arConfig(nil, nil, nil, nil))
	},

	"one-name-not-set": func(h *grHarness, _ *arFake) {
		h.Apply("create: the last name is left out", h.null(), arConfig(nil, "short_term", "mid_term", nil))
	},

	"unset-name": func(h *grHarness, _ *arFake) {
		s := h.Apply("create", h.null(), arNamed("short_term", "mid_term", "long_term"))
		s = h.Apply("update: the last name is removed from the config", s, arConfig(nil, "short_term", "mid_term", nil))
		s = h.Apply("update: the last name stays removed, and another name changes", s, arConfig(nil, "medium_term", "mid_term", nil))
		h.RefreshPlan("refresh", s)
	},

	// The company has fewer tags than the configuration.
	"three-tags": func(h *grHarness, f *arFake) {
		f.setCount(3)
		h.Apply("create", h.null(), arNamed("short_term", "mid_term", "long_term"))
	},

	// The company has more tags than the configuration. Create renames the first four.
	"five-tags": func(h *grHarness, f *arFake) {
		f.setCount(5)
		s := h.Apply("create", h.null(), arNamed("short_term", "mid_term", "long_term"))
		h.RefreshPlan("refresh", s)
	},

	// The configured names replace what the company has. Create reads the tags first and sends
	// their ids by position.
	"existing-names": func(h *grHarness, f *arFake) {
		f.rename(1, "old_short")
		f.rename(3, "old_long")
		s := h.Apply("create", h.null(), arNamed("short_term", "mid_term", "long_term"))
		h.RefreshPlan("refresh", s)
	},

	// A company that never read its tags gets the defaults on the first Get.
	"first-read-creates-tags": func(h *grHarness, f *arFake) {
		f.forget()
		s := h.Apply("create", h.null(), arNamed("short_term", "mid_term", "long_term"))
		h.RefreshPlan("refresh", s)
	},

	"drift": func(h *grHarness, f *arFake) {
		s := h.Apply("create", h.null(), arNamed("short_term", "mid_term", "long_term"))
		f.rename(2, "renamed_outside")
		s = h.RefreshPlan("refresh: a name was changed outside Terraform", s)
		s = h.Apply("apply the config again", s, arNamed("short_term", "mid_term", "long_term"))
		h.RefreshPlan("refresh", s)
	},

	"config-validation": func(h *grHarness, _ *arFake) {
		h.Validate("three retentions", arConfig(nil, "a", "b"))
		h.Validate("five retentions", arConfig(nil, "a", "b", "c", "d"))
		h.Validate("no retentions", j{"retentions": []any{}})
		h.Validate("the first name is set", arConfig("Default", "a", "b", "c"))
		h.Validate("set id", j{"id": "mine", "retentions": arNamed("a", "b", "c")["retentions"]})
		h.Validate("set a retention id", j{"retentions": []any{j{}, j{"name": "a", "id": "mine"}, j{"name": "b"}, j{"name": "c"}}})
		h.Validate("set order", j{"retentions": []any{j{}, j{"name": "a", "order": 7.0}, j{"name": "b"}, j{"name": "c"}}})
		h.Validate("a name with a space", arNamed("short term", "b", "c"))
		h.Validate("the first name is unknown", arConfig(unknownJSON{}, "a", "b", "c"))
		h.Validate("a name is unknown", arConfig(nil, unknownJSON{}, "b", "c"))
	},

	// The provider has no name validator; the API rejects these.
	"name-rejected": func(h *grHarness, _ *arFake) {
		h.Apply("create: a name with a space", h.null(), arNamed("short term", "mid_term", "long_term"))
		h.Apply("create: a name of 256 characters", h.null(), arNamed(strings.Repeat("a", 256), "mid_term", "long_term"))
		h.Apply("create: two retentions with one name", h.null(), arNamed("same", "same", "long_term"))
	},

	"name-255": func(h *grHarness, _ *arFake) {
		s := h.Apply("create: a name of 255 characters", h.null(), arNamed(strings.Repeat("a", 255), "mid_term", "long_term"))
		h.RefreshPlan("refresh", s)
	},

	"update-rejected": func(h *grHarness, _ *arFake) {
		s := h.Apply("create", h.null(), arNamed("short_term", "mid_term", "long_term"))
		s = h.Apply("update: two retentions with one name", s, arNamed("same", "same", "long_term"))
		h.RefreshPlan("refresh", s)
	},

	"import": func(h *grHarness, f *arFake) {
		f.rename(1, "short_term")
		f.rename(2, "mid_term")
		f.rename(3, "long_term")
		s := h.Import("import", "archive-retention-settings")
		h.Apply("plan the matching config", s, arNamed("short_term", "mid_term", "long_term"))
	},

	"import-another-id": func(h *grHarness, _ *arFake) {
		h.Import("import with an id that is not the fixed one", "anything")
	},

	// Create reads the tags before it renames them. When that read fails, it sends nothing.
	"create-read-fails": func(h *grHarness, f *arFake) {
		f.failGets(http.StatusInternalServerError, "Internal", "failed to get retentions for company")
		h.Apply("create: the first read fails", h.null(), arNamed("short_term", "mid_term", "long_term"))
	},

	"create-rejected": func(h *grHarness, f *arFake) {
		f.failSets(http.StatusInternalServerError, "Internal", "failed to update retentions for company")
		h.Apply("create: the API fails", h.null(), arNamed("short_term", "mid_term", "long_term"))
	},

	// The resource takes its state from the update answer, not from a read.
	"read-fails-after-update": func(h *grHarness, f *arFake) {
		s := h.Apply("create", h.null(), arNamed("short_term", "mid_term", "long_term"))
		f.failGetsAfterSet(http.StatusInternalServerError, "Internal", "failed to get retentions for company")
		s = h.Apply("update: reads fail after it", s, arNamed("short_term", "medium_term", "long_term"))
		f.healGets()
		h.RefreshPlan("refresh after the API recovers", s)
	},

	// Get creates missing tags, so it never answers 404 for the tags. The case records what the
	// resource does with a 404 of the gateway.
	"route-missing": func(h *grHarness, f *arFake) {
		s := h.Apply("create", h.null(), arNamed("short_term", "mid_term", "long_term"))
		f.failGets(http.StatusNotFound, "Not Found", "Not Found")
		h.RefreshPlan("refresh: the gateway has no such route", s)
	},

	"api-error": func(h *grHarness, f *arFake) {
		s := h.Apply("create", h.null(), arNamed("short_term", "mid_term", "long_term"))
		f.failGets(http.StatusInternalServerError, "Internal", "failed to get retentions for company")
		h.RefreshPlan("refresh: the API fails", s)
	},

	"update-not-found": func(h *grHarness, f *arFake) {
		s := h.Apply("create", h.null(), arNamed("short_term", "mid_term", "long_term"))
		f.failSets(http.StatusNotFound, "Not Found", "Could not set archive retentions, Non existing uuid present on request.")
		h.Apply("update: the API does not find an id", s, arNamed("short_term", "medium_term", "long_term"))
	},

	"destroy-rejected": func(h *grHarness, f *arFake) {
		s := h.Apply("create", h.null(), arNamed("short_term", "mid_term", "long_term"))
		f.failSets(http.StatusInternalServerError, "Internal", "failed to update retentions for company")
		h.Destroy("destroy: the API fails", s)
	},
}

// arDataSourceScenarios read the retention tags of the company. The data source takes no input.
var arDataSourceScenarios = map[string]func(h *grHarness){
	"read": func(h *grHarness) {
		h.ReadDataSource("read the default tags", j{})
	},
	"read-renamed": func(h *grHarness) {
		f := h.archiveRetentions()
		f.rename(1, "short_term")
		f.rename(3, "long_term")
		h.ReadDataSource("read renamed tags", j{})
	},
	"route-missing": func(h *grHarness) {
		h.archiveRetentions().failGets(http.StatusNotFound, "Not Found", "Not Found")
		h.ReadDataSource("read: the gateway has no such route", j{})
	},
	"api-error": func(h *grHarness) {
		h.archiveRetentions().failGets(http.StatusInternalServerError, "Internal", "failed to get retentions for company")
		h.ReadDataSource("read: the API fails", j{})
	},
}

func (h *grHarness) archiveRetentions() *arFake {
	h.t.Helper()
	f, ok := h.api.(*arFake)
	if !ok {
		h.t.Fatal("expected *arFake")
	}
	return f
}
