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

package dashboards

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	dashboardService "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	dashboardschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestOpenAPIDashboardFromProtoOneOfVariants(t *testing.T) {
	tests := map[string]struct {
		mutate func(*cxsdk.Dashboard)
		assert func(*testing.T, cxsdk.Dashboard)
	}{
		"relative off no folder": {
			mutate: func(d *cxsdk.Dashboard) {},
			assert: func(t *testing.T, d cxsdk.Dashboard) {
				if d.GetTimeFrame() == nil || d.GetAutoRefresh() == nil {
					t.Fatalf("expected time frame and auto refresh after roundtrip")
				}
				if d.GetFolderId() != nil || d.GetFolderPath() != nil {
					t.Fatalf("expected no folder after roundtrip")
				}
			},
		},
		"relative two minutes": {
			mutate: func(d *cxsdk.Dashboard) {
				d.AutoRefresh = &cxsdk.DashboardTwoMinutes{TwoMinutes: &cxsdk.DashboardAutoRefreshTwoMinutes{}}
			},
			assert: func(t *testing.T, d cxsdk.Dashboard) {
				if _, ok := d.GetAutoRefresh().(*cxsdk.DashboardTwoMinutes); !ok {
					t.Fatalf("expected two minute auto refresh, got %T", d.GetAutoRefresh())
				}
			},
		},
		"relative five minutes": {
			mutate: func(d *cxsdk.Dashboard) {
				d.AutoRefresh = &cxsdk.DashboardFiveMinutes{FiveMinutes: &cxsdk.DashboardAutoRefreshFiveMinutes{}}
			},
			assert: func(t *testing.T, d cxsdk.Dashboard) {
				if _, ok := d.GetAutoRefresh().(*cxsdk.DashboardFiveMinutes); !ok {
					t.Fatalf("expected five minute auto refresh, got %T", d.GetAutoRefresh())
				}
			},
		},
		"absolute off": {
			mutate: func(d *cxsdk.Dashboard) {
				d.TimeFrame = testAbsoluteTimeFrame()
			},
			assert: func(t *testing.T, d cxsdk.Dashboard) {
				if _, ok := d.GetTimeFrame().(*cxsdk.DashboardAbsoluteTimeFrame); !ok {
					t.Fatalf("expected absolute time frame, got %T", d.GetTimeFrame())
				}
			},
		},
		"absolute two minutes": {
			mutate: func(d *cxsdk.Dashboard) {
				d.TimeFrame = testAbsoluteTimeFrame()
				d.AutoRefresh = &cxsdk.DashboardTwoMinutes{TwoMinutes: &cxsdk.DashboardAutoRefreshTwoMinutes{}}
			},
			assert: func(t *testing.T, d cxsdk.Dashboard) {
				if _, ok := d.GetTimeFrame().(*cxsdk.DashboardAbsoluteTimeFrame); !ok {
					t.Fatalf("expected absolute time frame, got %T", d.GetTimeFrame())
				}
				if _, ok := d.GetAutoRefresh().(*cxsdk.DashboardTwoMinutes); !ok {
					t.Fatalf("expected two minute auto refresh, got %T", d.GetAutoRefresh())
				}
			},
		},
		"absolute five minutes": {
			mutate: func(d *cxsdk.Dashboard) {
				d.TimeFrame = testAbsoluteTimeFrame()
				d.AutoRefresh = &cxsdk.DashboardFiveMinutes{FiveMinutes: &cxsdk.DashboardAutoRefreshFiveMinutes{}}
			},
			assert: func(t *testing.T, d cxsdk.Dashboard) {
				if _, ok := d.GetTimeFrame().(*cxsdk.DashboardAbsoluteTimeFrame); !ok {
					t.Fatalf("expected absolute time frame, got %T", d.GetTimeFrame())
				}
				if _, ok := d.GetAutoRefresh().(*cxsdk.DashboardFiveMinutes); !ok {
					t.Fatalf("expected five minute auto refresh, got %T", d.GetAutoRefresh())
				}
			},
		},
		"folder id": {
			mutate: func(d *cxsdk.Dashboard) {
				d.FolderId = &cxsdk.UUID{Value: "folder-id"}
			},
			assert: func(t *testing.T, d cxsdk.Dashboard) {
				if d.GetFolderId().GetValue() != "folder-id" {
					t.Fatalf("expected folder id to roundtrip, got %q", d.GetFolderId().GetValue())
				}
			},
		},
		"folder path": {
			mutate: func(d *cxsdk.Dashboard) {
				d.FolderPath = &cxsdk.FolderPath{Segments: []string{"parent", "child"}}
			},
			assert: func(t *testing.T, d cxsdk.Dashboard) {
				if got := strings.Join(d.GetFolderPath().GetSegments(), "/"); got != "parent/child" {
					t.Fatalf("expected folder path to roundtrip, got %q", got)
				}
			},
		},
		"both folder fields": {
			mutate: func(d *cxsdk.Dashboard) {
				d.FolderId = &cxsdk.UUID{Value: "folder-id"}
				d.FolderPath = &cxsdk.FolderPath{Segments: []string{"parent", "child"}}
			},
			assert: func(t *testing.T, d cxsdk.Dashboard) {
				if d.GetFolderId().GetValue() != "folder-id" {
					t.Fatalf("expected folder id to roundtrip, got %q", d.GetFolderId().GetValue())
				}
				if got := strings.Join(d.GetFolderPath().GetSegments(), "/"); got != "parent/child" {
					t.Fatalf("expected folder path to roundtrip, got %q", got)
				}
			},
		},
		"missing auto refresh defaults to off": {
			mutate: func(d *cxsdk.Dashboard) {
				d.AutoRefresh = nil
			},
			assert: func(t *testing.T, d cxsdk.Dashboard) {
				if _, ok := d.GetAutoRefresh().(*cxsdk.DashboardOff); !ok {
					t.Fatalf("expected off auto refresh, got %T", d.GetAutoRefresh())
				}
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			dashboard := testDashboardProto()
			tt.mutate(dashboard)

			openAPIDashboard, diags := openAPIDashboardFromProto(dashboard)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			roundtripped, diags := protoDashboardFromOpenAPI(openAPIDashboard)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics after roundtrip: %v", diags)
			}

			tt.assert(t, *roundtripped)
		})
	}
}

func TestProtoDashboardFromOpenAPIReadableUnsupportedRefreshVariants(t *testing.T) {
	tests := map[string]struct {
		json   string
		assert func(*testing.T, *cxsdk.Dashboard)
	}{
		"one minute": {
			json: `{"name":"read-one-minute","layout":{},"relativeTimeFrame":"900s","oneMinute":{}}`,
			assert: func(t *testing.T, d *cxsdk.Dashboard) {
				if d.GetOneMinute() == nil {
					t.Fatalf("expected oneMinute to roundtrip, got %T", d.GetAutoRefresh())
				}
			},
		},
		"fifteen minutes": {
			json: `{"name":"read-fifteen-minutes","layout":{},"relativeTimeFrame":"900s","fifteenMinutes":{}}`,
			assert: func(t *testing.T, d *cxsdk.Dashboard) {
				if d.GetFifteenMinutes() == nil {
					t.Fatalf("expected fifteenMinutes to roundtrip, got %T", d.GetAutoRefresh())
				}
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			openAPIDashboard := unmarshalOpenAPIDashboard(t, tt.json)
			dashboard, diags := protoDashboardFromOpenAPI(openAPIDashboard)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}
			tt.assert(t, dashboard)
		})
	}
}

func TestOpenAPIDashboardFromProtoMissingTimeFrameDefaultsToRelative(t *testing.T) {
	dashboard := testDashboardProto()
	dashboard.TimeFrame = nil

	openAPIDashboard, diags := openAPIDashboardFromProto(dashboard)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	roundtripped, diags := protoDashboardFromOpenAPI(openAPIDashboard)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics after roundtrip: %v", diags)
	}
	if got := roundtripped.GetRelativeTimeFrame().AsDuration(); got != 15*time.Minute {
		t.Fatalf("expected missing time frame to default to 15m relative, got %s", got)
	}
}

func TestFlattenDashboardOmittedTimeFrameModes(t *testing.T) {
	dashboard := testDashboardProto()

	exposed, diags := flattenDashboard(context.Background(), DashboardResourceModel{}, dashboard, false)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics exposing dashboard time frame: %v", diags)
	}
	if exposed.TimeFrame == nil || exposed.TimeFrame.Relative == nil {
		t.Fatalf("expected data source/import flatten to expose dashboard time frame, got %#v", exposed.TimeFrame)
	}
	if got := exposed.TimeFrame.Relative.Duration.ValueString(); got != "seconds:900" {
		t.Fatalf("expected exposed dashboard time frame to be seconds:900, got %q", got)
	}

	preserved, diags := flattenDashboard(context.Background(), DashboardResourceModel{}, dashboard, true)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics preserving omitted dashboard time frame: %v", diags)
	}
	if preserved.TimeFrame != nil {
		t.Fatalf("expected managed resource flatten to preserve omitted time frame, got %#v", preserved.TimeFrame)
	}
}

func TestShouldPreserveOmittedDashboardTimeFrame(t *testing.T) {
	if shouldPreserveOmittedDashboardTimeFrame(DashboardResourceModel{}) {
		t.Fatal("expected import/data-source shaped state to expose dashboard time frame")
	}
	if !shouldPreserveOmittedDashboardTimeFrame(DashboardResourceModel{Name: types.StringValue("managed")}) {
		t.Fatal("expected managed resource state to preserve omitted dashboard time frame")
	}
}

func TestOpenAPIDashboardOneOfConflictDiagnostics(t *testing.T) {
	tests := map[string]string{
		"both time frame variants": `{
			"name": "bad-time-frame",
			"layout": {},
			"relativeTimeFrame": "900s",
			"absoluteTimeFrame": {
				"from": "2026-05-27T09:00:00Z",
				"to": "2026-05-27T10:00:00Z"
			},
			"off": {}
		}`,
		"both auto refresh variants": `{
			"name": "bad-refresh",
			"layout": {},
			"relativeTimeFrame": "900s",
			"off": {},
			"twoMinutes": {}
		}`,
	}

	for name, raw := range tests {
		t.Run(name, func(t *testing.T) {
			var openAPIDashboard dashboardService.Dashboard
			err := json.Unmarshal([]byte(raw), &openAPIDashboard)
			if err == nil {
				t.Fatal("expected OpenAPI dashboard oneOf unmarshal to fail")
			}

			diagnostic := openAPIDashboardDiagnostic(err)
			if !strings.Contains(diagnostic.Detail(), "exactly one dashboard time frame") {
				t.Fatalf("expected oneOf diagnostic, got %q", diagnostic.Detail())
			}
		})
	}
}

func TestOpenAPIDashboardFromProtoFixtures(t *testing.T) {
	tests := []string{
		"../../../examples/resources/coralogix_dashboard/dashboard.json",
		"../../../examples/resources/coralogix_dashboard/dashboard_with_var_path.json",
		"../../../examples/resources/coralogix_dashboard/dashboard_openapi_minimal_root.json",
		"../../../examples/resources/coralogix_dashboard/dashboard_openapi_minimal_folder_path.json",
	}

	for _, fixture := range tests {
		t.Run(fixture, func(t *testing.T) {
			data, err := os.ReadFile(fixture)
			if err != nil {
				t.Fatal(err)
			}

			dashboard := new(cxsdk.Dashboard)
			if err := dashboardschema.JSONUnmarshal.Unmarshal(data, dashboard); err != nil {
				t.Fatal(err)
			}

			openAPIDashboard, diags := openAPIDashboardFromProto(dashboard)
			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			if _, diags := protoDashboardFromOpenAPI(openAPIDashboard); diags.HasError() {
				t.Fatalf("unexpected diagnostics after roundtrip: %v", diags)
			}
		})
	}
}

func TestDashboardStateUpgradeAndOpenAPIIsolation(t *testing.T) {
	upgraders := DashboardResource{}.UpgradeState(context.Background())
	for _, version := range []int64{1, 2, 3} {
		upgrader, ok := upgraders[version]
		if !ok {
			t.Fatalf("expected state upgrader for version %d", version)
		}
		if upgrader.StateUpgrader == nil {
			t.Fatalf("expected state upgrader function for version %d", version)
		}
	}

	for _, path := range []string{
		"resource_coralogix_dashboard.go",
		"data_source_coralogix_dashboard.go",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}

		source := string(data)
		if strings.Contains(source, "cxsdk.Create"+"DashboardRequest") ||
			strings.Contains(source, "cxsdk.Replace"+"DashboardRequest") ||
			strings.Contains(source, "cxsdk.Delete"+"DashboardRequest") {
			t.Fatalf("normal dashboard CRUD in %s must not use legacy gRPC request types", path)
		}
	}

	resourceSource, err := os.ReadFile("resource_coralogix_dashboard.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"DashboardsServiceCreateDashboard",
		"DashboardsServiceGetDashboard",
		"DashboardsServiceReplaceDashboard",
		"DashboardsServiceDeleteDashboard",
	} {
		if !strings.Contains(string(resourceSource), required) {
			t.Fatalf("expected resource CRUD to use %s", required)
		}
	}
}

func testDashboardProto() *cxsdk.Dashboard {
	return &cxsdk.Dashboard{
		Id:     wrapperspb.String("dashboard-id"),
		Name:   wrapperspb.String("dashboard-name"),
		Layout: &cxsdk.DashboardLayout{},
		TimeFrame: &cxsdk.DashboardRelativeTimeFrame{
			RelativeTimeFrame: durationpb.New(15 * time.Minute),
		},
		AutoRefresh: &cxsdk.DashboardOff{Off: &cxsdk.DashboardAutoRefreshOff{}},
	}
}

func testAbsoluteTimeFrame() *cxsdk.DashboardAbsoluteTimeFrame {
	return &cxsdk.DashboardAbsoluteTimeFrame{
		AbsoluteTimeFrame: &cxsdk.DashboardTimeFrame{
			From: timestamppb.New(time.Date(2026, 5, 27, 9, 0, 0, 0, time.UTC)),
			To:   timestamppb.New(time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC)),
		},
	}
}

func unmarshalOpenAPIDashboard(t *testing.T, raw string) dashboardService.Dashboard {
	t.Helper()

	var openAPIDashboard dashboardService.Dashboard
	if err := json.Unmarshal([]byte(raw), &openAPIDashboard); err != nil {
		t.Fatalf("unexpected OpenAPI dashboard unmarshal error: %v\njson: %s", err, raw)
	}
	return openAPIDashboard
}

func TestOpenAPIDashboardFixtureNames(t *testing.T) {
	for _, fixture := range []string{
		"dashboard.json",
		"dashboard_with_var_path.json",
		"dashboard_openapi_minimal_root.json",
		"dashboard_openapi_minimal_folder_path.json",
	} {
		t.Run(fixture, func(t *testing.T) {
			if !strings.HasSuffix(fixture, ".json") {
				t.Fatalf("unexpected dashboard fixture name: %s", fixture)
			}
			if _, err := os.Stat(fmt.Sprintf("../../../examples/resources/coralogix_dashboard/%s", fixture)); err != nil {
				t.Fatal(err)
			}
		})
	}
}
