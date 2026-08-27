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
	"regexp"
	"testing"
	"time"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

const barChartXAxisAttr = "layout.sections.0.rows.0.widgets.0.definition.bar_chart.xaxis"

func barChartTimeBucketsConfig(name, xaxis string) string {
	return fmt.Sprintf(`resource "coralogix_dashboard" "test" {
  name = %q
  time_frame = { relative = { duration = "seconds:900" } }
  layout = { sections = [{ rows = [{
    height = 19
    widgets = [{
      title = "bar chart"
      definition = { bar_chart = {
        query = { metrics = { promql_query = "up" } }
        xaxis = { %s }
      }}
    }]
  }] }] }
}
`, name, xaxis)
}

// The x-axis interval resolution the Coralogix UI writes. Applying, importing
// and re-planning it covers the shape a dashboard built in the UI carries.
func TestAccCoralogixResourceDashboardBarChartTimeBuckets(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDashboardDestroy(t),
		Steps: []resource.TestStep{
			{
				Config: barChartTimeBucketsConfig(name, `time_buckets = {
          auto = { maximum_data_points = 96, minimum_interval = "15s" }
        }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, barChartXAxisAttr+".time_buckets.auto.maximum_data_points", "96"),
					resource.TestCheckResourceAttr(dashboardResourceName, barChartXAxisAttr+".time_buckets.auto.minimum_interval", "15s"),
					resource.TestCheckNoResourceAttr(dashboardResourceName, barChartXAxisAttr+".time"),
				),
			},
			{
				ResourceName:      dashboardResourceName,
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Switching arms, then to the legacy kind, must each settle.
			{
				Config: barChartTimeBucketsConfig(name, `time_buckets = {
          manual             = { interval = "900s", maximum_data_points = 1000, minimum_interval = "15s" }
          use_advanced_limit = true
        }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, barChartXAxisAttr+".time_buckets.manual.interval", "900s"),
					resource.TestCheckResourceAttr(dashboardResourceName, barChartXAxisAttr+".time_buckets.use_advanced_limit", "true"),
					resource.TestCheckNoResourceAttr(dashboardResourceName, barChartXAxisAttr+".time_buckets.auto"),
				),
			},
			{
				Config: barChartTimeBucketsConfig(name, `time = { interval = "15m0s" }`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(dashboardResourceName, barChartXAxisAttr+".time.interval", "15m0s"),
					resource.TestCheckNoResourceAttr(dashboardResourceName, barChartXAxisAttr+".time_buckets"),
				),
			},
		},
	})
}

// The API stores these and the provider used to reject them at plan time.
func TestAccCoralogixResourceDashboardBarChartTimeBucketsRejectsInvalidCombinations(t *testing.T) {
	name := dashboardOpenAPIFixtureName(t.Name())

	for scenario, testCase := range map[string]struct {
		xaxis string
		want  string
	}{
		"both interval kinds": {
			`time = { interval = "15m0s" }
             time_buckets = { auto = {} }`,
			`Only one of these attributes can be configured`,
		},
		"both resolution modes": {
			`time_buckets = {
               auto   = { minimum_interval = "15s" }
               manual = { interval = "900s" }
             }`,
			`Only one of these attributes can be configured`,
		},
		"no interval kind at all": {
			`{}`,
			`No attribute was configured in this one-of group`,
		},
		"shorthand duration the API rewrites": {
			`time_buckets = { auto = { minimum_interval = "1.5s" } }`,
			`stored by the API as "1.500s"`,
		},
		"duration in the wrong dialect": {
			`time_buckets = { auto = { minimum_interval = "15m" } }`,
			`must be a duration in seconds`,
		},
	} {
		t.Run(scenario, func(t *testing.T) {
			xaxis := testCase.xaxis
			if xaxis == `{}` {
				xaxis = ""
			}
			resource.ParallelTest(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{{
					Config:      barChartTimeBucketsConfig(name, xaxis),
					ExpectError: regexp.MustCompile(`(?s)` + testCase.want),
				}},
			})
		})
	}
}

// Every x-axis shape the API stores must be readable. The Coralogix UI writes
// timeBuckets by default and rewrites a stored legacy `time` to it on any save,
// even one that changes something unrelated, so a dashboard managed here became
// unreadable the moment anyone touched it in the UI: the read failed with
// "unknown bar chart x axis type" and plan, refresh, apply and import all
// stopped working. An x-axis or a timeBuckets with no kind selected is stored
// too, and hit the same failure.
func TestAccCoralogixResourceDashboardBarChartReadsEveryStoredXAxis(t *testing.T) {
	client := dashboardOpenAPIAcceptanceClient(t)
	interval, minimum := "900s", "15s"
	maximum := int32(1000)
	advanced := true

	for scenario, axis := range map[string]*dashboardservice.XAxis{
		"time buckets auto": {TimeBuckets: &dashboardservice.IntervalResolution{
			Auto: &dashboardservice.AutoIntervalResolution{MinimumInterval: &minimum},
		}},
		"time buckets manual": {TimeBuckets: &dashboardservice.IntervalResolution{
			Manual: &dashboardservice.ManualIntervalResolution{
				Interval: interval, MaximumDataPoints: &maximum, MinimumInterval: &minimum,
			},
		}},
		"time buckets with the advanced limit": {TimeBuckets: &dashboardservice.IntervalResolution{
			Auto:             &dashboardservice.AutoIntervalResolution{MaximumDataPoints: &maximum},
			UseAdvancedLimit: &advanced,
		}},
		"time buckets with no mode":  {TimeBuckets: &dashboardservice.IntervalResolution{}},
		"x axis with no kind":        {},
		"the deprecated time bucket": {Time: &dashboardservice.XAxisByTime{Interval: &interval}},
	} {
		t.Run(scenario, func(t *testing.T) {
			fixture := dashboardOpenAPIFixtureName(t.Name())
			response, err := dashboardOpenAPICreateDirectFixture(t, client, fixture,
				barChartXAxisRequest(fixture, axis))
			if err != nil {
				t.Fatalf("the API rejected the fixture, so it cannot be a read regression: %v", err)
			}

			resource.Test(t, resource.TestCase{
				PreCheck:                 func() { testAccPreCheck(t) },
				ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
				Steps: []resource.TestStep{{
					Config: `
variable "dashboard_id" { type = string }

data "coralogix_dashboard" "stored" {
  id = var.dashboard_id
}
`,
					ConfigVariables: config.Variables{
						"dashboard_id": config.StringVariable(response.GetDashboardId()),
					},
				}},
			})
		})
	}
}

func barChartXAxisRequest(name string, axis *dashboardservice.XAxis) dashboardservice.CreateDashboardRequestDataStructure {
	start := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	relative, promql, title := "900s", "up", "bar chart"
	height := int32(19)
	sectionID, rowID, widgetID := uuid.NewString(), uuid.NewString(), uuid.NewString()

	return dashboardservice.CreateDashboardRequestDataStructure{
		Dashboard: dashboardservice.Dashboard{
			Name:              name,
			AbsoluteTimeFrame: &dashboardservice.TimeFrame{From: &start, To: &end},
			Off:               map[string]interface{}{},
			Variables:         []dashboardservice.Variable{},
			Filters:           []dashboardservice.FiltersFilter{},
			Annotations:       []dashboardservice.Annotation{},
			Layout: dashboardservice.Layout{Sections: []dashboardservice.Section{{
				Id: &dashboardservice.UUID{Value: &sectionID},
				Rows: []dashboardservice.Row{{
					Id:         &dashboardservice.UUID{Value: &rowID},
					Appearance: &dashboardservice.RowAppearance{Height: &height},
					Widgets: []dashboardservice.Widget{{
						Id:    &dashboardservice.UUID{Value: &widgetID},
						Title: &title,
						Definition: &dashboardservice.WidgetDefinition{BarChart: &dashboardservice.BarChart{
							Query: &dashboardservice.BarChartQuery{Metrics: &dashboardservice.BarChartMetricsQuery{
								PromqlQuery: &dashboardservice.PromQlQuery{Value: &promql},
								Filters:     []dashboardservice.MetricsFilter{},
								TimeFrame:   &dashboardservice.TimeFrameSelect{RelativeTimeFrame: &relative},
							}},
							XAxis: axis,
						}},
					}},
				}},
			}}},
		},
		RequestId: dashboardOpenAPIHydrationRequestID(),
	}
}
