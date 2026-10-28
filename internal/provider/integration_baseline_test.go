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
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/integrations"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils/schemadump"
)

// These tests record the behavior of the handwritten coralogix_integration. After the generator
// switch, the golden diff must show only message wording and listed fixes.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run TestIntegrationBaseline

func TestIntegrationBaselineSchema(t *testing.T) {
	var current resource.SchemaResponse
	integrations.NewIntegrationResource().Schema(context.Background(), resource.SchemaRequest{}, &current)
	checkGoldenAt(t, integrationGoldenDir, "schema_current.txt", []byte(schemadump.Text(current.Schema)))
}

func TestIntegrationBaselineScenarios(t *testing.T) {
	for name, run := range integrationScenarios {
		t.Run(name, func(t *testing.T) {
			h := newIntegrationHarness(t)
			run(h)
			for i := range h.steps {
				sort.Strings(h.steps[i].ValidateDiags)
			}
			checkGoldenAt(t, integrationGoldenDir, "scenario_"+name+".json", marshalGolden(t, h.steps))
		})
	}
}

const (
	integrationOKRole   = "arn:aws:iam::123456789012:role/S3Access"
	integrationBadRole  = integrationRoleFailsTest
	integrationCatalog  = "aws-resource-catalog"
	integrationCollect  = "aws-metrics-collector"
	integrationVersion1 = "0.1.0"
)

func integrationConfig(key string, parameters j) j {
	return j{"integration_key": key, "version": integrationVersion1, "parameters": parameters}
}

func catalogConfig(name string) j {
	return integrationConfig(integrationCatalog, j{"IntegrationName": name, "AwsRoleArn": integrationOKRole})
}

func collectorParameters(name string) j {
	return j{
		"ApplicationName":  "cxsdk",
		"SubsystemName":    name,
		"MetricNamespaces": l{"AWS/S3"},
		"AwsRoleArn":       integrationOKRole,
		"IntegrationName":  name,
		"AwsRegion":        "eu-north-1",
		"WithAggregations": false,
		"EnrichWithTags":   true,
	}
}

var integrationScenarios = map[string]func(h *grHarness){
	"minimal": func(h *grHarness) {
		s := h.Apply("create", h.null(), catalogConfig("a"))
		s = h.Refresh("refresh", s)
		h.Destroy("destroy", s)
	},

	"full-lifecycle": func(h *grHarness) {
		s := h.Apply("create", h.null(), catalogConfig("a"))
		s = h.Refresh("refresh", s)
		h.RefreshPlan("plan the applied config again", s)
		s = h.Apply("update: change a value", s, catalogConfig("b"))
		s = h.Refresh("refresh", s)
		s = h.Apply("apply the same config", s, catalogConfig("b"))
		h.Destroy("destroy", s)
	},

	// String, list of strings, and bool values. A key that the config leaves out stays in the
	// backend, and the state shows only the keys of the config.
	"parameter-types": func(h *grHarness) {
		s := h.Apply("create", h.null(), integrationConfig(integrationCollect, collectorParameters("a")))
		s = h.Refresh("refresh", s)
		changed := collectorParameters("a")
		changed["MetricNamespaces"] = l{"AWS/S3", "AWS/EC2"}
		changed["WithAggregations"] = true
		s = h.Apply("update: change a list and a bool", s, integrationConfig(integrationCollect, changed))
		fewer := collectorParameters("a")
		delete(fewer, "EnrichWithTags")
		delete(fewer, "AwsRegion")
		s = h.Apply("update: leave two keys out of the config", s, integrationConfig(integrationCollect, fewer))
		s = h.Refresh("refresh: the backend still holds the two keys", s)
		h.Apply("update: add the two keys again", s, integrationConfig(integrationCollect, collectorParameters("a")))
	},

	"empty-list-parameter": func(h *grHarness) {
		p := collectorParameters("a")
		p["MetricNamespaces"] = l{}
		s := h.Apply("create: an empty list", h.null(), integrationConfig(integrationCollect, p))
		h.Refresh("refresh", s)
	},

	// The request shows how a number is sent.
	"number-parameter": func(h *grHarness) {
		p := collectorParameters("a")
		p["Count"] = 3.0
		h.Apply("create: a number value", h.null(), integrationConfig(integrationCollect, p))
	},

	"parameters-not-an-object": func(h *grHarness) {
		h.t.Helper()
		h.Apply("create: parameters is a string", h.null(), j{"integration_key": integrationCatalog, "version": integrationVersion1, "parameters": "x"})
	},

	"parameter-with-nested-object": func(h *grHarness) {
		p := catalogConfig("a")
		p["parameters"].(j)["Nested"] = j{"a": "b"}
		h.Apply("create: a value that is an object", h.null(), p)
	},

	"config-validation": func(h *grHarness) {
		h.Validate("missing integration_key", j{"version": integrationVersion1, "parameters": j{}})
		h.Validate("missing version", j{"integration_key": integrationCatalog, "parameters": j{}})
		h.Validate("missing parameters", j{"integration_key": integrationCatalog, "version": integrationVersion1})
		h.Validate("valid", catalogConfig("a"))
	},

	"test-failure-on-create": func(h *grHarness) {
		p := collectorParameters("a")
		p["AwsRoleArn"] = integrationBadRole
		h.Apply("create: the test result is a failure, so nothing is saved", h.null(), integrationConfig(integrationCollect, p))
	},

	"test-failure-on-update": func(h *grHarness) {
		s := h.Apply("create", h.null(), integrationConfig(integrationCollect, collectorParameters("a")))
		p := collectorParameters("a")
		p["AwsRoleArn"] = integrationBadRole
		h.Apply("update: the test result is a failure, so nothing is updated", s, integrationConfig(integrationCollect, p))
	},

	"test-api-error": func(h *grHarness) {
		h.integrations().testStatus = 500
		h.Apply("create: the test call answers 500", h.null(), catalogConfig("a"))
	},

	"create-invalid-parameter": func(h *grHarness) {
		p := catalogConfig("a")
		p["parameters"].(j)["Unknown"] = "x"
		h.Apply("create: the API refuses an unknown parameter", h.null(), p)
	},

	"create-unknown-integration": func(h *grHarness) {
		h.Apply("create: the integration key does not exist", h.null(), integrationConfig("does-not-exist", j{"IntegrationName": "a"}))
	},

	"create-api-error": func(h *grHarness) {
		h.integrations().failStatus = 500
		h.Apply("create: the save answers 500", h.null(), catalogConfig("a"))
	},

	"update-api-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), catalogConfig("a"))
		h.integrations().failStatus = 500
		s = h.Apply("update: the API answers 500", s, catalogConfig("b"))
		h.integrations().failStatus = 0
		h.Refresh("refresh", s)
	},

	"update-invalid-version": func(h *grHarness) {
		s := h.Apply("create", h.null(), catalogConfig("a"))
		c := catalogConfig("a")
		c["version"] = "9.9.9"
		h.Apply("update: a version change updates in place, and the API refuses this version", s, c)
	},

	"change-integration-key": func(h *grHarness) {
		s := h.Apply("create", h.null(), catalogConfig("a"))
		h.Apply("update: a new integration_key replaces the resource", s, integrationConfig(integrationCollect, collectorParameters("a")))
	},

	"delete-error": func(h *grHarness) {
		s := h.Apply("create", h.null(), catalogConfig("a"))
		h.integrations().failStatus = 500
		h.Destroy("destroy: the API answers 500", s)
	},

	"read-after-delete-outside": func(h *grHarness) {
		s := h.Apply("create", h.null(), catalogConfig("a"))
		h.integrations().deleteOutside(h.integrations().onlyID())
		h.Refresh("refresh: the integration was deleted outside Terraform", s)
	},

	"update-after-delete-outside": func(h *grHarness) {
		s := h.Apply("create", h.null(), catalogConfig("a"))
		h.integrations().deleteOutside(h.integrations().onlyID())
		h.Apply("update: the integration was deleted outside Terraform", s, catalogConfig("b"))
	},

	"destroy-after-delete-outside": func(h *grHarness) {
		s := h.Apply("create", h.null(), catalogConfig("a"))
		h.integrations().deleteOutside(h.integrations().onlyID())
		h.Destroy("destroy: the integration was deleted outside Terraform", s)
	},

	"read-api-404": func(h *grHarness) {
		s := h.Apply("create", h.null(), catalogConfig("a"))
		h.integrations().notFound = true
		h.Refresh("refresh: the API answers 404 for every integration", s)
	},

	// A value changed outside Terraform does not show: a refresh keeps the value of the config.
	"drift-outside": func(h *grHarness) {
		s := h.Apply("create", h.null(), catalogConfig("a"))
		h.integrations().modifyOutside(h.integrations().onlyID(), "IntegrationName", "changed-outside")
		s = h.Refresh("refresh: a value changed outside Terraform", s)
		h.RefreshPlan("plan the applied config again", s)
	},

	// An import has no config, so the state holds every parameter of the backend. A secret reads
	// back as <redacted>, and an API key as its value.
	"import": func(h *grHarness) {
		id := h.integrations().seed(integrationCatalog, integrationVersion1,
			map[string]any{"key": "IntegrationName", "stringValue": "a"},
			map[string]any{"key": "AwsRoleArn", "stringValue": integrationOKRole},
			map[string]any{"key": "Secret", "sensitiveData": map[string]any{}},
			map[string]any{"key": "Token", "apiKey": map[string]any{"id": "1", "value": "token-value"}},
			map[string]any{"key": "Count", "numericValue": 3},
			map[string]any{"key": "List", "stringList": map[string]any{"values": []any{"x", "y"}}},
		)
		s := h.Import("import", id)
		h.Apply("plan the matching config after the import", s, catalogConfig("a"))
	},

	"import-unknown-id": func(h *grHarness) {
		h.Import("import an id that does not exist", "does-not-exist")
	},
}
