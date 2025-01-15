// Copyright 2025 Coralogix Ltd.
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

package dashboardwidgets

import (
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type HexagonModel struct {
	customUnit    types.String `tfsdk:"custom_unit"`
	LegendBy      types.String `tfsdk:"legend_by"`
	Decimal       types.Number `tfsdk:"decimal"`
	DataModeType  types.String `tfsdk:"data_mode_type"`
	Thresholds    types.Set    `tfsdk:"thresholds"` //HexagonThresholdModel
	ThresholdType types.String `tfsdk:"threshold_type"`
	Min           types.Number `tfsdk:"min"`
	Max           types.Number `tfsdk:"max"`
	Unit          types.String `tfsdk:"unit"`
	Legend        *LegendModel `tfsdk:"legend"`
	Query         types.Object `tfsdk:"query"` //HexagonQueryDefinitionModel
}

type HexagonQueryDefinitionModel struct {
	ID                 types.String         `tfsdk:"id"`
	Query              *LineChartQueryModel `tfsdk:"query"`
	SeriesNameTemplate types.String         `tfsdk:"series_name_template"`
	SeriesCountLimit   types.Int64          `tfsdk:"series_count_limit"`
	Unit               types.String         `tfsdk:"unit"`
	ScaleType          types.String         `tfsdk:"scale_type"`
	Name               types.String         `tfsdk:"name"`
	IsVisible          types.Bool           `tfsdk:"is_visible"`
	ColorScheme        types.String         `tfsdk:"color_scheme"`
	Resolution         types.Object         `tfsdk:"resolution"` //LineChartResolutionModel
	DataModeType       types.String         `tfsdk:"data_mode_type"`
}

type HexagonQueryModel struct {
	Logs      *LineChartQueryLogsModel    `tfsdk:"logs"`
	Metrics   *LineChartQueryMetricsModel `tfsdk:"metrics"`
	Spans     *LineChartQuerySpansModel   `tfsdk:"spans"`
	DataPrime *DataPrimeQueryModel        `tfsdk:"dataprime"`
}
type HexagonThresholdModel struct {
	From  types.Number `tfsdk:"from"`
	Color types.String `tfsdk:"color"`
	Label types.String `tfsdk:"label"`
}

func HexagonSchema() schema.Attribute {
	return schema.SingleNestedAttribute{
		Required: true,
		Attributes: map[string]schema.Attribute{
			"min": schema.NumberAttribute{
				Optional: true,
			},
			"max": schema.NumberAttribute{
				Optional: true,
			},
			"decimal": schema.NumberAttribute{
				Optional: true,
			},
			"legend": legendSchema(),
			"legend_by": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("unspecified"),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidLegendBy...),
				},
				MarkdownDescription: fmt.Sprintf("The legend by. Valid values are: %s.", strings.Join(dashboardValidLegendBy, ", ")),
			},
			"unit": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("unspecified"),
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidUnits...),
				},
				MarkdownDescription: fmt.Sprintf("The unit. Valid values are: %s.", strings.Join(dashboardValidUnits, ", ")),
			},
			"data_mode_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidDataModeTypes...),
				},
				Default: stringdefault.StaticString("unspecified"),
			},
			"thresholds": schema.SetNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"from": schema.NumberAttribute{
							Required: true,
						},
						"color": schema.StringAttribute{
							Optional: true,
						},
						"label": schema.StringAttribute{
							Optional: true,
						},
					},
				},
			},
			"threshold_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf(dashboardValidThresholdTypes...),
				},
				Default:             stringdefault.StaticString("unspecified"),
				MarkdownDescription: fmt.Sprintf("The threshold type. Valid values are: %s.", strings.Join(dashboardValidThresholdTypes, ", ")),
			},
			"query": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"logs": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"lucene_query": schema.StringAttribute{
								Optional: true,
							},
							"group_by": schema.ListAttribute{
								ElementType: types.StringType,
								Optional:    true,
							},
							"filters":      logsFiltersSchema(),
							"aggregations": logsAggregationsSchema(),
						},
						Optional: true,
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("metrics"),
								path.MatchRelative().AtParent().AtName("spans"),
								path.MatchRelative().AtParent().AtName("dataprime"),
							),
						},
					},
					"metrics": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"promql_query": schema.StringAttribute{
								Required: true,
							},
							"filters": metricFiltersSchema(),
						},
						Optional: true,
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("logs"),
								path.MatchRelative().AtParent().AtName("spans"),
								path.MatchRelative().AtParent().AtName("dataprime"),
							),
						},
					},
					"spans": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"lucene_query": schema.StringAttribute{
								Optional: true,
							},
							"group_by":     spansFieldsSchema(),
							"aggregations": spansAggregationsSchema(),
							"filters":      spansFilterSchema(),
						},
						Optional: true,
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("metrics"),
								path.MatchRelative().AtParent().AtName("logs"),
								path.MatchRelative().AtParent().AtName("dataprime"),
							),
						},
					},
					"dataprime": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"dataprime_query": schema.StringAttribute{
								Optional: true,
							},
							"timeframe": timeFrameSchema(),
							"filters":   spansFilterSchema(),
						},
						Optional: true,
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("metrics"),
								path.MatchRelative().AtParent().AtName("spans"),
								path.MatchRelative().AtParent().AtName("logs"),
							),
						},
					},
				},
			},
		},
		Validators: []validator.Object{
			objectvalidator.ExactlyOneOf(
				path.MatchRelative().AtParent().AtName("data_table"),
				path.MatchRelative().AtParent().AtName("gauge"),
				path.MatchRelative().AtParent().AtName("line_chart"),
				path.MatchRelative().AtParent().AtName("pie_chart"),
				path.MatchRelative().AtParent().AtName("bar_chart"),
				path.MatchRelative().AtParent().AtName("horizontal_bar_chart"),
				path.MatchRelative().AtParent().AtName("markdown"),
			),
			objectvalidator.AlsoRequires(
				path.MatchRelative().AtParent().AtParent().AtName("title"),
			),
		},
		Optional: true,
	}
}
