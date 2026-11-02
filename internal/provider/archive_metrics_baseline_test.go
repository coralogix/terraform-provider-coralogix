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

const amGoldenDir = "testdata/archive_metrics_baseline"

const amTypeName = "coralogix_archive_metrics"

// These tests record the behavior of the registered coralogix_archive_metrics. After a change, the
// golden diff must show only listed fixes or message wording.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run TestArchiveMetricsBaseline

func TestArchiveMetricsBaselineSchema(t *testing.T) {
	checkGoldenAt(t, amGoldenDir, "schema_current.txt", []byte(schemadump.Text(registeredResourceSchema(t, amTypeName))))
}

func TestArchiveMetricsBaselineScenarios(t *testing.T) {
	for name, run := range amScenarios {
		t.Run(name, func(t *testing.T) {
			fake := newAMFake(t)
			h := newProtocolHarness(t, amTypeName, fake)
			run(h, fake)
			checkGoldenAt(t, amGoldenDir, "scenario_"+name+".json", marshalGolden(t, h.steps))
		})
	}
}

func amS3(bucket string) j { return j{"bucket": bucket, "region": amServerRegion} }

func amIBM() j { return amIBMAt("eu-de", "metrics-bucket") }

func amIBMAt(location, bucket string) j {
	return j{
		"endpoint": "https://s3." + location + ".cloud-object-storage.appdomain.cloud",
		"crn":      "crn:v1:bluemix:public:cloud-object-storage:" + location + ":a/0:0:bucket:" + bucket,
	}
}

func amRetention(raw, fiveMinutes, oneHour float64) j {
	return j{"raw_resolution": raw, "five_minutes_resolution": fiveMinutes, "one_hour_resolution": oneHour}
}

var amScenarios = map[string]func(h *grHarness, f *amFake){
	"minimal": func(h *grHarness, _ *amFake) {
		s := h.Apply("create", h.null(), j{"s3": amS3("metrics-bucket")})
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy: no API call, the archive stays configured", s)
	},

	"ibm": func(h *grHarness, _ *amFake) {
		s := h.Apply("create", h.null(), j{"ibm": amIBM()})
		s = h.RefreshPlan("refresh", s)
		h.Destroy("destroy", s)
	},

	"full-lifecycle": func(h *grHarness, _ *amFake) {
		s := h.Apply("create", h.null(), j{"s3": amS3("metrics-bucket"), "retention_policy": amRetention(10, 20, 30)})
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: retention policy", s, j{"s3": amS3("metrics-bucket"), "retention_policy": amRetention(15, 25, 35)})
		s = h.Apply("update: another bucket", s, j{"s3": amS3("other-bucket"), "retention_policy": amRetention(15, 25, 35)})
		s = h.Apply("update: another region", s, j{"s3": j{"bucket": "other-bucket", "region": "eu-west-1"}, "retention_policy": amRetention(15, 25, 35)})
		s = h.Apply("update: s3 to ibm", s, j{"ibm": amIBM(), "retention_policy": amRetention(15, 25, 35)})
		s = h.Apply("update: ibm endpoint and crn", s, j{"ibm": amIBMAt("us-south", "other-bucket"), "retention_policy": amRetention(15, 25, 35)})
		s = h.Apply("update: ibm to s3", s, j{"s3": amS3("metrics-bucket"), "retention_policy": amRetention(15, 25, 35)})
		h.Destroy("destroy", s)
	},

	"unset-retention-policy": func(h *grHarness, _ *amFake) {
		s := h.Apply("create", h.null(), j{"s3": amS3("metrics-bucket"), "retention_policy": amRetention(10, 20, 30)})
		s = h.Apply("update: retention_policy is removed from the config", s, j{"s3": amS3("metrics-bucket")})
		h.RefreshPlan("refresh", s)
	},

	"add-retention-policy": func(h *grHarness, _ *amFake) {
		s := h.Apply("create without retention_policy", h.null(), j{"s3": amS3("metrics-bucket")})
		s = h.Apply("update: retention_policy is added", s, j{"s3": amS3("metrics-bucket"), "retention_policy": amRetention(10, 20, 30)})
		h.RefreshPlan("refresh", s)
	},

	"ibm-malformed-crn": func(h *grHarness, _ *amFake) {
		h.Apply("create: the CRN has no resource part", h.null(), j{"ibm": j{"endpoint": "https://s3.eu-de.cloud-object-storage.appdomain.cloud", "crn": "x:bucket:y"}})
	},

	"config-validation": func(h *grHarness, _ *amFake) {
		h.Validate("no storage", j{})
		h.Validate("s3 and ibm", j{"s3": amS3("metrics-bucket"), "ibm": amIBM()})
		h.Validate("s3 without region", j{"s3": j{"bucket": "metrics-bucket"}})
		h.Validate("retention policy without raw_resolution", j{"s3": amS3("metrics-bucket"), "retention_policy": j{"five_minutes_resolution": 1.0, "one_hour_resolution": 1.0}})
		h.Validate("set id", j{"s3": amS3("metrics-bucket"), "id": "mine"})
		h.Validate("set prefix", j{"s3": amS3("metrics-bucket"), "prefix": "mine"})
	},

	"create-rejected": func(h *grHarness, f *amFake) {
		f.unwritable = map[string]bool{"metrics-bucket": true}
		h.Apply("create: the API cannot write to the bucket", h.null(), j{"s3": amS3("metrics-bucket")})
	},

	// Create sets the configuration, then reads it. When the read fails, the resource returns the
	// fixed id, the configured storage, and unknown computed values. Terraform keeps that state,
	// tainted.
	"create-read-fails": func(h *grHarness, f *amFake) {
		f.failGetsAfterSet(http.StatusInternalServerError, "Internal", "tenant config store unavailable")
		h.Apply("create: the read after the set fails", h.null(), j{"s3": amS3("metrics-bucket")})
	},

	"existing-config": func(h *grHarness, f *amFake) {
		f.seed(j{"s3": amS3("old-bucket")})
		h.Apply("create sets the configuration that the company already has", h.null(), j{"s3": amS3("metrics-bucket")})
	},

	"import": func(h *grHarness, f *amFake) {
		f.seed(j{"s3": amS3("seeded-bucket"), "retentionPolicy": j{"rawResolution": 10, "fiveMinutesResolution": 20, "oneHourResolution": 30}})
		s := h.Import("import", "archive-metrics-settings")
		h.Apply("plan the matching config", s, j{"s3": amS3("seeded-bucket")})
	},

	"import-ibm": func(h *grHarness, f *amFake) {
		f.seed(j{"ibm": amIBM()})
		s := h.Import("import", "archive-metrics-settings")
		h.Apply("plan the matching config", s, j{"ibm": amIBM()})
	},

	// A configuration made before the custom storage format has an empty prefix, and keeps it.
	"import-legacy": func(h *grHarness, f *amFake) {
		f.seed(j{"s3": amS3("seeded-bucket"), "prefix": ""})
		s := h.Import("import", "archive-metrics-settings")
		s = h.Apply("update: another bucket", s, j{"s3": amS3("other-bucket")})
		h.RefreshPlan("refresh", s)
	},

	"import-zero-retention": func(h *grHarness, f *amFake) {
		f.seed(j{"s3": amS3("seeded-bucket"), "retentionPolicy": j{"rawResolution": 0, "fiveMinutesResolution": 5, "oneHourResolution": 0}})
		s := h.Import("import a configuration with levels of 0", "archive-metrics-settings")
		h.Apply("plan the matching config", s, j{"s3": amS3("seeded-bucket"), "retention_policy": amRetention(0, 5, 0)})
	},

	"import-another-id": func(h *grHarness, f *amFake) {
		f.seed(j{"s3": amS3("seeded-bucket")})
		h.Import("import with an id that is not the fixed one", "anything")
	},

	"import-without-config": func(h *grHarness, _ *amFake) {
		h.Import("import when the company has no configuration", "archive-metrics-settings")
	},

	"read-without-config": func(h *grHarness, f *amFake) {
		s := h.Apply("create", h.null(), j{"s3": amS3("metrics-bucket")})
		f.forget()
		h.RefreshPlan("refresh: the API does not find the configuration", s)
	},

	"read-unsupported-storage": func(h *grHarness, f *amFake) {
		s := h.Apply("create", h.null(), j{"s3": amS3("metrics-bucket")})
		f.seed(j{"gcs": j{"bucket": "gcs-bucket"}})
		h.RefreshPlan("refresh: the configuration uses GCS", s)
	},

	"disabled-outside-terraform": func(h *grHarness, f *amFake) {
		s := h.Apply("create", h.null(), j{"s3": amS3("metrics-bucket")})
		f.disable()
		h.RefreshPlan("refresh: the archive was disabled outside Terraform", s)
	},

	"route-missing": func(h *grHarness, f *amFake) {
		s := h.Apply("create", h.null(), j{"s3": amS3("metrics-bucket")})
		f.failGets(http.StatusNotFound, "Not Found", "Not Found")
		h.RefreshPlan("refresh: the gateway has no such route", s)
	},

	"api-error": func(h *grHarness, f *amFake) {
		s := h.Apply("create", h.null(), j{"s3": amS3("metrics-bucket")})
		f.failGets(http.StatusInternalServerError, "Internal", "tenant config store unavailable")
		h.RefreshPlan("refresh: the API fails", s)
	},

	// The API stores and returns a level of 0. A request or a read that drops a zero shows here.
	"zero-retention": func(h *grHarness, _ *amFake) {
		s := h.Apply("create: raw and one-hour levels are 0", h.null(), j{"s3": amS3("metrics-bucket"), "retention_policy": amRetention(0, 5, 0)})
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: every level is 0", s, j{"s3": amS3("metrics-bucket"), "retention_policy": amRetention(0, 0, 0)})
		s = h.RefreshPlan("refresh", s)
		s = h.Apply("update: back to positive levels", s, j{"s3": amS3("metrics-bucket"), "retention_policy": amRetention(10, 20, 30)})
		h.RefreshPlan("refresh", s)
	},

	// An update that the API rejects changes nothing in the API. The state keeps the last
	// applied values, and the next plan proposes the change again.
	"update-rejected-s3": func(h *grHarness, f *amFake) {
		s := h.Apply("create", h.null(), j{"s3": amS3("metrics-bucket")})
		f.unwritable = map[string]bool{"other-bucket": true}
		s = h.Apply("update: the API cannot write to the new bucket", s, j{"s3": amS3("other-bucket")})
		h.RefreshPlan("refresh", s)
	},

	"update-rejected-ibm": func(h *grHarness, _ *amFake) {
		s := h.Apply("create", h.null(), j{"ibm": amIBM()})
		s = h.Apply("update: the new CRN has no resource part", s, j{"ibm": j{"endpoint": "https://s3.eu-de.cloud-object-storage.appdomain.cloud", "crn": "x:bucket:y"}})
		h.RefreshPlan("refresh", s)
	},

	// The set call stores the configuration, then the read fails. The backend has the new
	// configuration and the state keeps what the resource returned.
	"update-read-fails": func(h *grHarness, f *amFake) {
		s := h.Apply("create", h.null(), j{"s3": amS3("metrics-bucket")})
		f.failGetsAfterSet(http.StatusInternalServerError, "Internal", "tenant config store unavailable")
		s = h.Apply("update: the read after the set fails", s, j{"s3": amS3("other-bucket")})
		f.healGets()
		h.RefreshPlan("refresh after the API recovers", s)
	},

	// The set call does not answer 404 for a missing configuration: it creates one. The case
	// records what the resource does if it did.
	"update-not-found": func(h *grHarness, f *amFake) {
		s := h.Apply("create", h.null(), j{"s3": amS3("metrics-bucket")})
		f.failSets(http.StatusNotFound, "Not Found", "Tenant config not found")
		h.Apply("update: the API does not find the configuration", s, j{"s3": amS3("other-bucket")})
	},
}

// amDataSourceScenarios read the one metrics archive configuration of the company. The data source
// takes no input.
var amDataSourceScenarios = map[string]func(h *grHarness){
	"read": func(h *grHarness) {
		h.archiveMetrics().seed(j{"s3": amS3("seeded-bucket")})
		h.ReadDataSource("read the configuration", j{})
	},
	"read-with-retention-policy": func(h *grHarness) {
		h.archiveMetrics().seed(j{"s3": amS3("seeded-bucket"), "retentionPolicy": j{"rawResolution": 10, "fiveMinutesResolution": 20, "oneHourResolution": 30}})
		h.ReadDataSource("read a configuration with a retention policy", j{})
	},
	"read-zero-retention": func(h *grHarness) {
		h.archiveMetrics().seed(j{"s3": amS3("seeded-bucket"), "retentionPolicy": j{"rawResolution": 0, "fiveMinutesResolution": 5, "oneHourResolution": 0}})
		h.ReadDataSource("read a configuration with levels of 0", j{})
	},
	"read-ibm": func(h *grHarness) {
		h.archiveMetrics().seed(j{"ibm": amIBM()})
		h.ReadDataSource("read an IBM configuration", j{})
	},
	"read-without-config": func(h *grHarness) {
		h.ReadDataSource("read: the API does not find the configuration", j{})
	},
	"route-missing": func(h *grHarness) {
		h.archiveMetrics().failGets(http.StatusNotFound, "Not Found", "Not Found")
		h.ReadDataSource("read: the gateway has no such route", j{})
	},
	"api-error": func(h *grHarness) {
		h.archiveMetrics().failGets(http.StatusInternalServerError, "Internal", "tenant config store unavailable")
		h.ReadDataSource("read: the API fails", j{})
	},
}

func (h *grHarness) archiveMetrics() *amFake {
	h.t.Helper()
	f, ok := h.api.(*amFake)
	if !ok {
		h.t.Fatal("expected *amFake")
	}
	return f
}
