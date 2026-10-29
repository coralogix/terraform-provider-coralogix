// Copyright 2026 Coralogix Ltd.
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
	"fmt"
	"maps"
	"net/http"
	"testing"
)

// These tests record the data sources of the generated resources through the plugin protocol,
// against the fake backends of the resource baselines. When a data source becomes generated code,
// the golden diff must show only listed fixes or message wording.
//
//	UPDATE_GOLDEN=1 go test ./internal/provider -run TestDataSourceBaseline

const (
	cgTypeName = "coralogix_configuration_group"
	vfTypeName = "coralogix_view_folder"

	cgGoldenDir = "testdata/configuration_group_baseline"
	vfGoldenDir = "testdata/view_folder_baseline"
)

// dataSourceBaseline is the data source of one resource: its golden directory, a harness with the
// fake backend, and the scenarios.
type dataSourceBaseline struct {
	dir       string
	harness   func(t *testing.T) *grHarness
	scenarios map[string]func(h *grHarness)
}

var dataSourceBaselines = map[string]dataSourceBaseline{
	"configuration_group": {cgGoldenDir, newCGHarness, cgDataSourceScenarios},
	"alerts_scheduler": {asGoldenDir, func(t *testing.T) *grHarness {
		return newProtocolHarness(t, asTypeName, newASFake(t))
	}, asDataSourceScenarios},
	"slo_v2":        {sloGoldenDir, newSLOHarness, sloDataSourceScenarios},
	"events2metric": {e2mGoldenDir, newE2MHarness, e2mDataSourceScenarios},
	"connector":     {cxGoldenDir, newCXHarness, cxDataSourceScenarios},
	"global_router": {grGoldenDir, newGRHarness, grDataSourceScenarios},
	"preset":        {presetGoldenDir, newPresetHarness, presetDataSourceScenarios},
	"view_folder":   {vfGoldenDir, newVFHarness, vfDataSourceScenarios},
	"archive_logs": {alGoldenDir, func(t *testing.T) *grHarness {
		return newProtocolHarness(t, alTypeName, newALFake(t))
	}, alDataSourceScenarios},
	"quota_allocation_rule_set": {quotaGoldenDir, newQuotaHarness, quotaDataSourceScenarios},
	"action":                    {actionGoldenDir, newActionHarness, actionDataSourceScenarios},
	"integration":               {integrationGoldenDir, newIntegrationHarness, integrationDataSourceScenarios},
}

func TestDataSourceBaselineSchema(t *testing.T) {
	for name, baseline := range dataSourceBaselines {
		t.Run(name, func(t *testing.T) {
			h := baseline.harness(t)
			if h.dsSchema == nil {
				t.Fatalf("%s has no data source", h.typeName)
			}
			checkGoldenAt(t, baseline.dir, "datasource_schema.txt", []byte(dataSourceSchemaText(h.dsSchema)))
		})
	}
}

func TestDataSourceBaselineScenarios(t *testing.T) {
	for name, baseline := range dataSourceBaselines {
		for scenario, run := range baseline.scenarios {
			t.Run(name+"/"+scenario, func(t *testing.T) {
				h := baseline.harness(t)
				run(h)
				checkGoldenAt(t, baseline.dir, "datasource_"+scenario+".json", marshalGolden(t, h.steps))
			})
		}
	}
}

// missingID is an id that no fake has. It has the form of an id, because an SDK can check the form
// before the request.
const missingID = "00000000-0000-4000-8000-999999999999"

// byID is the data source config that looks up id.
func byID(id string) j { return j{"id": id} }

// nameLookupScenarios are the scenarios of a data source that finds a resource by id or by name.
// seed stores a resource named name and returns its id; duplicate stores a copy of id with a new id
// and the same name.
func nameLookupScenarios(seed func(h *grHarness, name string) string, duplicate func(h *grHarness, id string) string) map[string]func(h *grHarness) {
	return map[string]func(h *grHarness){
		"read-by-id": func(h *grHarness) {
			h.ReadDataSource("read by id", byID(seed(h, "seeded")))
		},
		"read-by-name": func(h *grHarness) {
			seed(h, "other")
			seed(h, "seeded")
			h.ReadDataSource("read by name", j{"name": "seeded"})
		},
		"duplicate-name": func(h *grHarness) {
			first := seed(h, "seeded")
			duplicate(h, first)
			h.ReadDataSource("read a name that two resources have", j{"name": "seeded"})
		},
		"not-found": func(h *grHarness) {
			h.ReadDataSource("read an id that does not exist", byID(missingID))
		},
		"name-not-found": func(h *grHarness) {
			seed(h, "seeded")
			h.ReadDataSource("read a name that does not exist", j{"name": "missing"})
		},
		"id-and-name": func(h *grHarness) {
			h.ReadDataSource("config sets id and name", j{"id": "x", "name": "x"})
		},
		"no-id-or-name": func(h *grHarness) {
			h.ReadDataSource("config sets neither id nor name", j{})
		},
	}
}

// idLookupScenarios are the scenarios of a data source that finds a resource by id only. seed
// stores a resource and returns its id.
func idLookupScenarios(seed func(h *grHarness) string) map[string]func(h *grHarness) {
	return map[string]func(h *grHarness){
		"read-by-id": func(h *grHarness) {
			h.ReadDataSource("read by id", byID(seed(h)))
		},
		"not-found": func(h *grHarness) {
			h.ReadDataSource("read an id that does not exist", byID(missingID))
		},
	}
}

// --- configuration group ---

func newCGHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, cgTypeName, newRecordFake(t, "/fleet-management/configuration-groups/v1", "", "",
		"Configuration group with id %s not found"))
}

func (h *grHarness) records() *recordFake {
	h.t.Helper()
	f, ok := h.api.(*recordFake)
	if !ok {
		h.t.Fatal("expected *recordFake")
	}
	return f
}

var cgDataSourceScenarios = idLookupScenarios(func(h *grHarness) string {
	return h.records().seed(j{
		"id": "11111111-1111-4111-8111-111111111111", "name": "seeded", "description": "made outside Terraform",
		"archived": false, "priorityOrder": float64(2), "tags": l{"team-a"},
		"family": j{
			"id": "22222222-2222-4222-8222-222222222222", "version": "3", "active": false, "description": "raw family",
			"raw": j{"remoteConfigurations": l{j{
				"id": "33333333-3333-4333-8333-333333333333", "name": "base", "hash": "h1",
				"rawConfiguration": "receivers:\n  otlp: {}\n",
			}}},
		},
	})
})

// --- alerts scheduler ---

var asDataSourceScenarios = func() map[string]func(h *grHarness) {
	out := idLookupScenarios(func(h *grHarness) string {
		return h.schedulers().seed(j{
			"name": "seeded", "description": "made outside Terraform", "enabled": true,
			"filter": j{"whatExpression": "source logs | filter true", "alertUniqueIds": j{"value": l{"alert-1"}}},
			"schedule": j{
				"scheduleOperation": "SCHEDULE_OPERATION_MUTE",
				"recurring":         j{"alwaysActive": j{}},
			},
		})
	})
	// The data source reads a start time in the form that the API returns, as the resource does.
	out["read-start-time"] = func(h *grHarness) {
		id := h.schedulers().seed(j{
			"name": "one-time", "enabled": true,
			"filter": j{"whatExpression": "source logs | filter true", "alertUniqueIds": j{"value": l{"alert-1"}}},
			"schedule": j{
				"scheduleOperation": "SCHEDULE_OPERATION_ACTIVATE",
				"oneTime": j{"timeframe": j{
					"startTime": "2030-01-04T00:00:00.000Z", "timezone": "UTC+2",
					"duration": j{"forOver": float64(2), "frequency": "DURATION_FREQUENCY_HOUR"},
				}},
			},
		})
		h.ReadDataSource("read a one-time rule", byID(id))
	}
	return out
}()

// --- SLO v2 ---

var sloDataSourceScenarios = func() map[string]func(h *grHarness) {
	out := idLookupScenarios(func(h *grHarness) string {
		return h.slos().seed(j{
			"name": "seeded", "description": "made outside Terraform", "targetThresholdPercentage": float64(99),
			"sloTimeFrame": "SLO_TIME_FRAME_14_DAYS",
			"windowBasedMetricSli": j{
				"query": j{"query": "avg(up)"}, "window": "WINDOW_SLO_WINDOW_5_MINUTES",
				"comparisonOperator": "COMPARISON_OPERATOR_LESS_THAN", "threshold": 0.2,
			},
		})
	})
	// The data source reads an older record as the resource does.
	out["read-legacy-filter-value"] = func(h *grHarness) {
		id := h.slos().seedRaw(j{
			"name": "legacy", "targetThresholdPercentage": float64(99), "sloTimeFrame": "SLO_TIME_FRAME_7_DAYS",
			"apmSli": j{
				"services": l{"checkout"}, "errorConfig": j{}, "groupingKeys": l{},
				"filters": l{j{"key": "environment", "value": "prod"}},
			},
		})
		h.ReadDataSource("read a record without productType and with a deprecated filter value", byID(id))
	}
	return out
}()

// --- events2metric ---

var e2mDataSourceScenarios = func() map[string]func(h *grHarness) {
	out := idLookupScenarios(func(h *grHarness) string {
		return h.events2metrics().seed(j{
			"name": "seeded", "type": "E2M_TYPE_LOGS2METRICS", "description": "made outside Terraform",
			"logsQuery":         j{"lucene": "x", "severityFilters": l{"SEVERITY_INFO"}},
			"metricFields":      l{j{"targetBaseMetricName": "f", "sourceField": "f", "aggregations": l{}}},
			"metricLabels":      l{j{"targetLabel": "L", "sourceField": "l"}},
			"permutationsLimit": float64(7),
		})
	})
	// The data source shows the description that the API stores, also when it is empty.
	out["read-empty-description"] = func(h *grHarness) {
		id := h.events2metrics().seed(j{
			"name": "bare", "type": "E2M_TYPE_LOGS2METRICS", "description": "", "logsQuery": j{},
		})
		h.ReadDataSource("read an events2metric with an empty description", byID(id))
	}
	return out
}()

// --- connector ---

var cxDataSourceScenarios = nameLookupScenarios(
	func(h *grHarness, name string) string {
		return h.connectors().seed(j{
			"name": name, "type": "GENERIC_HTTPS", "description": "made outside Terraform",
			"connectorConfig": j{"fields": l{
				j{"fieldName": "method", "value": "post"},
				j{"fieldName": "url", "value": "https://example.com/hook"},
			}},
		})
	},
	func(h *grHarness, id string) string { return h.connectors().duplicate(id) },
)

// --- global router ---

var grDataSourceScenarios = nameLookupScenarios(
	func(h *grHarness, name string) string {
		return h.routers().seed(j{
			"name": name, "description": "made outside Terraform",
			"routingLabels": j{"environment": name},
			"rules": l{j{"name": "r", "condition": "true", "entityType": "CASES", "targets": l{
				j{"connectorId": "c1"}, j{"connectorId": "c2", "presetId": "p"},
			}}},
		})
	},
	func(h *grHarness, id string) string { return h.routers().duplicate(id) },
)

// --- preset ---

var presetDataSourceScenarios = nameLookupScenarios(
	func(h *grHarness, name string) string {
		return h.presets().seed(j{
			"name": name, "entityType": "ALERTS", "connectorType": "GENERIC_HTTPS",
			"parentId": presetSystemParent, "description": "made outside Terraform",
			"configOverrides": l{j{
				"conditionType": j{"matchEntityType": j{}},
				"payloadType":   "generic_https_default",
				"messageConfig": j{"fields": l{j{"fieldName": "body", "template": "{}"}}},
			}},
		})
	},
	func(h *grHarness, id string) string { return h.presets().duplicate(id) },
)

// --- view folder ---

func newVFHarness(t *testing.T) *grHarness {
	return newProtocolHarness(t, vfTypeName, newRecordFake(t, "/data-exploration/views/v1/folders",
		"/data-exploration/views/v1/folders", "folders", "View folder with id %s not found"))
}

var vfDataSourceScenarios = nameLookupScenarios(
	func(h *grHarness, name string) string {
		return h.records().seed(j{"id": vfNextID(h), "name": name})
	},
	func(h *grHarness, id string) string {
		copied := maps.Clone(h.records().records[id])
		copied["id"] = vfNextID(h)
		return h.records().seed(copied)
	},
)

// vfNextID is the id of the next folder. The ids grow, so the List returns the folders in the
// order that the test seeds them.
func vfNextID(h *grHarness) string {
	return fmt.Sprintf("aaaaaaaa-0000-4000-8000-%012d", len(h.records().records)+1)
}

// --- quota allocation rule set ---

// The data source reads the singleton with no configuration, and shows the rules of Coralogix
// (cx_managed) next to the rules of the customer.
var quotaDataSourceScenarios = map[string]func(h *grHarness){
	"read-empty": func(h *grHarness) {
		h.ReadDataSource("read the rule set before any rule exists", j{})
	},
	"read-managed-and-customer": func(h *grHarness) {
		h.quota().seedManaged(j{"entityType": "alerts", "allocation": 5, "enabled": true, "canOverflow": false, "allocationType": "QUOTA_ALLOCATION_TYPE_PERCENTAGE"})
		h.quota().seedCustomer(
			j{"entityType": "logs", "allocation": 60.5, "enabled": true, "canOverflow": true},
			j{"entityType": "spans", "allocation": 2, "enabled": false, "canOverflow": false, "allocationType": "QUOTA_ALLOCATION_TYPE_LOCKED_UNITS"},
		)
		h.ReadDataSource("read managed and customer rules", j{})
	},
	"read-unspecified-type": func(h *grHarness) {
		h.quota().seedManaged(j{"entityType": "alerts", "allocation": 5, "enabled": true, "canOverflow": false, "allocationType": "QUOTA_ALLOCATION_TYPE_UNSPECIFIED"})
		h.quota().seedCustomer(j{"entityType": "logs", "allocation": 10, "enabled": true, "canOverflow": false})
		h.ReadDataSource("read rules whose type the API returns as unspecified or leaves out", j{})
	},
	"not-found": func(h *grHarness) {
		h.quota().notFound = true
		h.ReadDataSource("read when the API answers 404", j{})
	},
}

// --- integration ---

// The data source reads a deployed integration by id. A configured `parameters` object filters the
// parameters that the data source shows to its keys.
var integrationDataSourceScenarios = map[string]func(h *grHarness){
	"read-by-id": func(h *grHarness) {
		id := h.integrations().seed(integrationCatalog, integrationVersion1,
			j{"key": "IntegrationName", "stringValue": "seeded"},
			j{"key": "AwsRoleArn", "stringValue": integrationOKRole})
		h.ReadDataSource("read by id", byID(id))
	},
	"read-by-id-filtered-by-parameter-keys": func(h *grHarness) {
		id := h.integrations().seed(integrationCollect, integrationVersion1,
			j{"key": "IntegrationName", "stringValue": "seeded"},
			j{"key": "AwsRoleArn", "stringValue": integrationOKRole},
			j{"key": "AwsRegion", "stringValue": "eu-north-1"},
			j{"key": "MetricNamespaces", "stringList": j{"values": l{"AWS/S3"}}},
			j{"key": "WithAggregations", "booleanValue": false})
		h.ReadDataSource("read with parameters set: the state keeps the configured value", j{
			"id":         id,
			"parameters": j{"IntegrationName": "x", "AwsRegion": "y"},
		})
	},
	"read-by-id-with-unwanted-attributes": func(h *grHarness) {
		id := h.integrations().seed(integrationCatalog, integrationVersion1,
			j{"key": "IntegrationName", "stringValue": "seeded"},
			j{"key": "AwsRoleArn", "stringValue": integrationOKRole})
		h.ReadDataSource("config sets the computed integration_key", j{"id": id, "integration_key": "other"})
	},
	"not-found": func(h *grHarness) {
		h.ReadDataSource("read an id that does not exist", byID(missingID))
	},
	"read-api-error": func(h *grHarness) {
		h.integrations().notFound = false
		id := h.integrations().seed(integrationCatalog, integrationVersion1,
			j{"key": "IntegrationName", "stringValue": "seeded"},
			j{"key": "AwsRoleArn", "stringValue": integrationOKRole})
		h.integrations().readStatus = http.StatusInternalServerError
		h.ReadDataSource("read when the API answers 500", byID(id))
	},
	"no-id": func(h *grHarness) {
		h.ReadDataSource("config sets no id", j{})
	},
}
