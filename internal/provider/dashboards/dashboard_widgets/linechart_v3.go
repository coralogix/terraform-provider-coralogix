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

package dashboard_widgets

import (
	"fmt"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// LineChartSchemaV3 is a frozen copy of the line chart schema as it stood at resource schema
// version 3. Schema v3 is only ever used to decode prior state, so its shape must not change.
//
// Do not add attributes here and do not redirect this to LineChartSchema(). It was split from
// the shared helper precisely so that widening the current schema cannot alter v3.
func LineChartSchemaV3() schema.Attribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Attributes: map[string]schema.Attribute{
			"legend": LegendSchema(),
			"tooltip": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"show_labels": schema.BoolAttribute{
						Optional: true,
						Computed: true,
						Default:  booldefault.StaticBool(false),
					},
					"type": schema.StringAttribute{
						Optional: true,
						Validators: []validator.String{
							stringvalidator.OneOf(DashboardValidTooltipTypes...),
						},
						MarkdownDescription: fmt.Sprintf("The tooltip type. Valid values are: %s.", strings.Join(DashboardValidTooltipTypes, ", ")),
					},
				},
				Optional: true,
			},
			"stacked_line": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Validators: []validator.String{
					stringvalidator.OneOf(DashboardValidLineChartStackedLineOptions...),
				},
				Default:             stringdefault.StaticString(utils.UNSPECIFIED),
				MarkdownDescription: fmt.Sprintf("Option to show lines as stacked. Possible values: %v", strings.Join(DashboardValidLineChartStackedLineOptions, ", ")),
			},
			"query_definitions": schema.ListNestedAttribute{
				Required: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed: true, PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseNonNullStateForUnknown(),
							},
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
										"filters":      LogsFiltersSchema(),
										"aggregations": LogsAggregationsSchema(),
										"time_frame":   TimeFrameSchema(),
									},
									Optional: true,
								},
								"metrics": schema.SingleNestedAttribute{
									Attributes: map[string]schema.Attribute{
										"promql_query": schema.StringAttribute{
											Required: true,
										},
										"filters": MetricFiltersSchema(),
										"promql_query_type": schema.StringAttribute{
											Optional: true,
											Computed: true,
											Default:  stringdefault.StaticString(utils.UNSPECIFIED),
										},
										"time_frame": TimeFrameSchema(),
									},
									Optional: true,
								},
								"spans": schema.SingleNestedAttribute{
									Attributes: map[string]schema.Attribute{
										"lucene_query": schema.StringAttribute{
											Optional: true,
										},
										"group_by":     SpansFieldsSchema(),
										"aggregations": SpansAggregationsSchema(),
										"filters":      SpansFilterSchema(),
										"time_frame":   TimeFrameSchema(),
									},
									Optional: true,
								},
								"data_prime": schema.SingleNestedAttribute{
									Attributes: map[string]schema.Attribute{
										"query": schema.StringAttribute{
											Optional: true,
										},
										"filters": schema.ListNestedAttribute{
											NestedObject: schema.NestedAttributeObject{
												Attributes: FiltersSourceSchema(),
												Validators: []validator.Object{
													ExactlyOneOfChildren("logs", "metrics", "spans"),
												},
											},
											Optional: true,
										},
										"time_frame": TimeFrameSchema(),
									},
									Optional: true,
								},
							},
							Required: true,
							Validators: []validator.Object{
								ExactlyOneOfChildren("logs", "metrics", "spans", "data_prime"),
							},
						},
						"series_name_template": schema.StringAttribute{
							Optional: true,
						},
						"series_count_limit": schema.Int64Attribute{
							Optional: true,
						},
						"unit": UnitSchema(),
						"scale_type": schema.StringAttribute{
							Optional: true,
							Computed: true,
							Validators: []validator.String{
								stringvalidator.OneOf(DashboardValidScaleTypes...),
							},
							Default:             stringdefault.StaticString(utils.UNSPECIFIED),
							MarkdownDescription: fmt.Sprintf("The scale type. Valid values are: %s.", strings.Join(DashboardValidScaleTypes, ", ")),
						},
						"name": schema.StringAttribute{
							Optional: true,
						},
						"is_visible": schema.BoolAttribute{
							Optional: true,
							Computed: true,
							Default:  booldefault.StaticBool(true),
						},
						"color_scheme": schema.StringAttribute{
							Optional: true,
							Validators: []validator.String{
								stringvalidator.OneOf(DashboardValidColorSchemes...),
							},
						},
						"resolution": schema.SingleNestedAttribute{
							Attributes: map[string]schema.Attribute{
								"interval": schema.StringAttribute{
									Optional: true,
								},
								"buckets_presented": schema.Int64Attribute{
									Optional: true,
								},
							},
							Optional: true,
							Validators: []validator.Object{
								ExactlyOneOfChildren("interval", "buckets_presented"),
							},
						},
						"data_mode_type": schema.StringAttribute{
							Optional: true,
							Computed: true,
							Validators: []validator.String{
								stringvalidator.OneOf(DashboardValidDataModeTypes...),
							},
							Default: stringdefault.StaticString(utils.UNSPECIFIED),
						},
					},
				},
			},
		},
		Validators: []validator.Object{
			objectvalidator.AlsoRequires(
				path.MatchRelative().AtParent().AtParent().AtName("title"),
			),
		},
	}
}
