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

package dashboardjson

import (
	"testing"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
)

func TestUnmarshal_SeriesCountLimitAcceptsNumber(t *testing.T) {
	data := []byte(`{
		"legend": {},
		"tooltip": {},
		"queryDefinitions": [
			{
				"id": "306ecf24-3bbf-40ac-9990-d8ba5f6ff6ff",
				"query": {"logs": {}},
				"seriesCountLimit": 20
			}
		]
	}`)

	var target dashboardservice.LineChart
	if err := Unmarshal(data, &target); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}

	if len(target.QueryDefinitions) != 1 {
		t.Fatalf("expected 1 query definition, got %d", len(target.QueryDefinitions))
	}
	limit := target.QueryDefinitions[0].SeriesCountLimit
	if limit == nil || *limit != "20" {
		t.Fatalf("expected seriesCountLimit to be \"20\", got %v", limit)
	}
}

func TestUnmarshal_SeriesCountLimitAcceptsString(t *testing.T) {
	data := []byte(`{
		"legend": {},
		"tooltip": {},
		"queryDefinitions": [
			{
				"id": "306ecf24-3bbf-40ac-9990-d8ba5f6ff6ff",
				"query": {"logs": {}},
				"seriesCountLimit": "20"
			}
		]
	}`)

	var target dashboardservice.LineChart
	if err := Unmarshal(data, &target); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}

	limit := target.QueryDefinitions[0].SeriesCountLimit
	if limit == nil || *limit != "20" {
		t.Fatalf("expected seriesCountLimit to be \"20\", got %v", limit)
	}
}
