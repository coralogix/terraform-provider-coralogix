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

package dashboards

import (
	"context"
	"reflect"
	"testing"
	"time"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	dashboardwidgets "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_widgets"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestExpandHorizontalBarChartDataPrimeQueryRequestShape(t *testing.T) {
	ctx := context.Background()
	dataPrime := horizontalBarChartDataPrimeObject(t, &dashboardwidgets.TimeFrameModel{
		Relative: &dashboardwidgets.TimeFrameRelativeModel{Duration: types.StringValue("seconds:900")},
	})
	query := &dashboardwidgets.HorizontalBarChartQueryModel{
		Logs:      types.ObjectNull(barChartLogsQueryAttr()),
		Metrics:   types.ObjectNull(barChartMetricsQueryAttr()),
		Spans:     types.ObjectNull(barChartSpansQueryAttr()),
		DataPrime: dataPrime,
	}

	got, diags := expandHorizontalBarChartQuery(ctx, query)
	if diags.HasError() {
		t.Fatalf("expandHorizontalBarChartQuery() diagnostics = %v", diags)
	}

	queryText := "source logs | groupby $l.applicationname as application, $m.severity as severity aggregate count() as c"
	stackedGroupName := "severity"
	relativeTimeFrame := "900s"
	want := &dashboardservice.HorizontalBarChartQuery{
		Dataprime: &dashboardservice.HorizontalBarChartDataprimeQuery{
			DataprimeQuery:   &dashboardservice.CommonDataprimeQuery{Text: &queryText},
			Filters:          nil,
			GroupNames:       []string{"application"},
			StackedGroupName: &stackedGroupName,
			TimeFrame:        &dashboardservice.TimeFrameSelect{RelativeTimeFrame: &relativeTimeFrame},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expandHorizontalBarChartQuery() = %#v, want %#v", got, want)
	}
}

func TestExpandHorizontalBarChartDataPrimeQueryAbsoluteTimeFrame(t *testing.T) {
	ctx := context.Background()
	dataPrime := horizontalBarChartDataPrimeObject(t, &dashboardwidgets.TimeFrameModel{
		Absolute: &dashboardwidgets.TimeFrameAbsoluteModel{
			Start: types.StringValue("2026-02-01T00:00:00Z"),
			End:   types.StringValue("2026-02-01T00:15:00Z"),
		},
	})

	got, diags := expandHorizontalBarChartDataPrimeQuery(ctx, dataPrime)
	if diags.HasError() {
		t.Fatalf("expandHorizontalBarChartDataPrimeQuery() diagnostics = %v", diags)
	}
	if got == nil || got.TimeFrame == nil || got.TimeFrame.AbsoluteTimeFrame == nil {
		t.Fatalf("expanded absolute time frame = %#v, want populated absolute branch", got)
	}

	wantStart := time.Date(2026, time.February, 1, 0, 0, 0, 0, time.UTC)
	wantEnd := time.Date(2026, time.February, 1, 0, 15, 0, 0, time.UTC)
	if got.TimeFrame.AbsoluteTimeFrame.GetFrom() != wantStart || got.TimeFrame.AbsoluteTimeFrame.GetTo() != wantEnd {
		t.Fatalf("expanded absolute time frame = %#v, want %s to %s", got.TimeFrame.AbsoluteTimeFrame, wantStart, wantEnd)
	}
	if got.TimeFrame.RelativeTimeFrame != nil {
		t.Fatalf("expanded relative time frame = %q, want nil", *got.TimeFrame.RelativeTimeFrame)
	}
}

func TestExpandHorizontalBarChartDataPrimeQueryToleratesNullAndUnknown(t *testing.T) {
	ctx := context.Background()
	for name, value := range map[string]types.Object{
		"null":    types.ObjectNull(barChartDataPrimeQueryAttr()),
		"unknown": types.ObjectUnknown(barChartDataPrimeQueryAttr()),
	} {
		t.Run(name, func(t *testing.T) {
			got, diags := expandHorizontalBarChartDataPrimeQuery(ctx, value)
			if diags.HasError() {
				t.Fatalf("expandHorizontalBarChartDataPrimeQuery() diagnostics = %v", diags)
			}
			if got != nil {
				t.Fatalf("expandHorizontalBarChartDataPrimeQuery() = %#v, want nil", got)
			}
		})
	}
}

func horizontalBarChartDataPrimeObject(t *testing.T, timeFrame *dashboardwidgets.TimeFrameModel) types.Object {
	t.Helper()
	model := dashboardwidgets.BarChartQueryDataPrimeModel{
		Query: types.StringValue("source logs | groupby $l.applicationname as application, $m.severity as severity aggregate count() as c"),
		Filters: types.ListValueMust(
			types.ObjectType{AttrTypes: dashboardwidgets.FilterSourceModelAttr()},
			[]attr.Value{},
		),
		GroupNames:       types.ListValueMust(types.StringType, []attr.Value{types.StringValue("application")}),
		StackedGroupName: types.StringValue("severity"),
		TimeFrame:        timeFrame,
	}

	value, diags := types.ObjectValueFrom(context.Background(), barChartDataPrimeQueryAttr(), model)
	if diags.HasError() {
		t.Fatalf("types.ObjectValueFrom() diagnostics = %v", diags)
	}
	return value
}
