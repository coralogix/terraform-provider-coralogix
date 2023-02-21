package coralogix

import (
	"context"
	"fmt"
	"log"
	"time"

	"terraform-provider-coralogix/coralogix/clientset"
	dashboards "terraform-provider-coralogix/coralogix/clientset/grpc/coralogix-dashboards/v1"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	dashboardSchemaRowStyleToProtoRowStyle = map[string]string{
		"Unspecified": "ROW_STYLE_UNSPECIFIED",
		"One_Line":    "ROW_STYLE_ONE_LINE",
		"Two_Line":    "ROW_STYLE_TWO_LINE",
		"Condensed":   "ROW_STYLE_CONDENSED",
		"Json":        "ROW_STYLE_JSON",
	}
	dashboardProtoRowStyleToSchemaRowStyle         = reverseMapStrings(dashboardSchemaRowStyleToProtoRowStyle)
	dashboardValidRowStyle                         = getKeysStrings(dashboardSchemaRowStyleToProtoRowStyle)
	dashboardSchemaLegendColumnToProtoLegendColumn = map[string]string{
		"Unspecified": "LEGEND_COLUMN_UNSPECIFIED",
		"Min":         "LEGEND_COLUMN_MIN",
		"Max":         "LEGEND_COLUMN_MAX",
		"Sum":         "LEGEND_COLUMN_SUM",
		"Avg":         "LEGEND_COLUMN_AVG",
		"Last":        "LEGEND_COLUMN_LAST",
	}
	dashboardProtoLegendColumnToSchemaLegendColumn     = reverseMapStrings(dashboardSchemaLegendColumnToProtoLegendColumn)
	dashboardValidLegendColumn                         = getKeysStrings(dashboardSchemaLegendColumnToProtoLegendColumn)
	dashboardSchemaOrderDirectionToProtoOrderDirection = map[string]string{
		"Unspecified": "ORDER_DIRECTION_UNSPECIFIED",
		"Asc":         "ORDER_DIRECTION_ASC",
		"Desc":        "ORDER_DIRECTION_DESC",
	}
	dashboardProtoOrderDirectionToSchemaOrderDirection = reverseMapStrings(dashboardSchemaOrderDirectionToProtoOrderDirection)
	dashboardValidOrderDirection                       = getKeysStrings(dashboardSchemaOrderDirectionToProtoOrderDirection)
	dashboardSchemaAggregationToProtoAggregation       = map[string]string{
		"Unspecified": "AGGREGATION_UNSPECIFIED",
		"Last":        "AGGREGATION_LAST",
		"Min":         "AGGREGATION_MIN",
		"Max":         "AGGREGATION_MAX",
		"Avg":         "AGGREGATION_AVG",
	}
	dashboardProtoAggregationToSchemaAggregation = reverseMapStrings(dashboardSchemaAggregationToProtoAggregation)
	dashboardValidAggregation                    = getKeysStrings(dashboardSchemaAggregationToProtoAggregation)
	dashboardSchemaGaugeUnitToProtoGaugeUnit     = map[string]string{
		"Unspecified": "Gauge_UNIT_UNSPECIFIED",
		"Number":      "Gauge_UNIT_NUMBER",
		"Percent":     "Gauge_UNIT_PERCENT",
	}
	dashboardProtoGaugeUnitToSchemaGaugeUnit = reverseMapStrings(dashboardSchemaGaugeUnitToProtoGaugeUnit)
	dashboardValidGaugeUnit                  = getKeysStrings(dashboardSchemaGaugeUnitToProtoGaugeUnit)
)

func resourceCoralogixDashboard() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCoralogixDashboardCreate,
		ReadContext:   resourceCoralogixDashboardRead,
		UpdateContext: resourceCoralogixDashboardUpdate,
		DeleteContext: resourceCoralogixDashboardDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(60 * time.Second),
			Read:   schema.DefaultTimeout(30 * time.Second),
			Update: schema.DefaultTimeout(60 * time.Second),
			Delete: schema.DefaultTimeout(30 * time.Second),
		},

		Schema: DashboardSchema(),

		Description: "Coralogix Dashboard.",
	}
}

func resourceCoralogixDashboardCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	dashboard, diags := extractDashboard(d)
	if diags != nil {
		return diags
	}
	createDashboardRequest := &dashboards.CreateDashboardRequest{
		Dashboard: dashboard,
	}

	log.Printf("[INFO] Creating new dashboard: %#v", createDashboardRequest)
	DashboardResp, err := meta.(*clientset.ClientSet).Dashboards().CreateDashboard(ctx, createDashboardRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err, "dashboard")
	}

	Dashboard := DashboardResp.ProtoReflect()
	log.Printf("[INFO] Submitted new dashboard: %#v", Dashboard)
	d.SetId(createDashboardRequest.GetDashboard().GetId().GetValue())

	return resourceCoralogixDashboardRead(ctx, d, meta)
}

func resourceCoralogixDashboardRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	dashboardId := wrapperspb.String(expandUUID(id))
	log.Printf("[INFO] Reading dashboard %s", id)
	resp, err := meta.(*clientset.ClientSet).Dashboards().GetDashboard(ctx, &dashboards.GetDashboardRequest{DashboardId: dashboardId})
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "dashboard", id)
	}

	dashboard := resp.GetDashboard()
	log.Printf("[INFO] Received dashboard: %#v", dashboard)

	return setDashboard(d, dashboard)
}

func resourceCoralogixDashboardUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	dashboard, diags := extractDashboard(d)
	if diags != nil {
		return diags
	}
	updateDashboardRequest := &dashboards.ReplaceDashboardRequest{
		Dashboard: dashboard,
	}

	log.Printf("[INFO] Updating dashboard: %#v", updateDashboardRequest)
	DashboardResp, err := meta.(*clientset.ClientSet).Dashboards().UpdateDashboard(ctx, updateDashboardRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err, "dashboard")
	}

	Dashboard := DashboardResp.ProtoReflect()
	log.Printf("[INFO] Submitted updated dashboard: %#v", Dashboard)
	d.SetId(updateDashboardRequest.GetDashboard().GetId().GetValue())

	return resourceCoralogixDashboardRead(ctx, d, meta)
}

func resourceCoralogixDashboardDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	log.Printf("[INFO] Deleting dashboard %s\n", id)

	deleteAlertRequest := &dashboards.DeleteDashboardRequest{DashboardId: wrapperspb.String(id)}
	_, err := meta.(*clientset.ClientSet).Dashboards().DeleteDashboard(ctx, deleteAlertRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v\n", err)
		return handleRpcErrorWithID(err, "dashboard", id)
	}
	log.Printf("[INFO] dashboard %s deleted\n", id)

	d.SetId("")
	return nil
}

func DashboardSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"name": {
			Type:         schema.TypeString,
			Optional:     true,
			ValidateFunc: validation.StringIsNotEmpty,
			Description:  "Dashboard name.",
		},
		"description": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Dashboard description.",
		},
		"layout": {
			Type:     schema.TypeList,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"sections": {
						Type: schema.TypeList,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"id": {
									Type:     schema.TypeString,
									Computed: true,
								},
								"rows": {
									Type: schema.TypeList,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											"id": {
												Type:     schema.TypeString,
												Computed: true,
											},
											"appearance": {
												Type:     schema.TypeList,
												Required: true,
												MaxItems: 1,
												Elem: &schema.Resource{
													Schema: map[string]*schema.Schema{
														"height": {
															Type:     schema.TypeInt,
															Required: true,
														},
													},
												},
											},
											"widgets": {
												Type: schema.TypeList,
												Elem: &schema.Resource{
													Schema: map[string]*schema.Schema{
														"id": {
															Type:     schema.TypeString,
															Computed: true,
														},
														"title": {
															Type:     schema.TypeString,
															Optional: true,
														},
														"description": {
															Type:     schema.TypeString,
															Optional: true,
														},
														"definition": {
															Type:     schema.TypeList,
															MaxItems: 1,
															Elem: &schema.Resource{
																Schema: map[string]*schema.Schema{
																	"line_chart": {
																		Type:     schema.TypeList,
																		MaxItems: 1,
																		Elem: &schema.Resource{
																			Schema: map[string]*schema.Schema{
																				"query": {
																					Type:     schema.TypeList,
																					MaxItems: 1,
																					Elem: &schema.Resource{
																						Schema: map[string]*schema.Schema{
																							"logs": {
																								Type:     schema.TypeList,
																								MaxItems: 1,
																								Elem: &schema.Resource{
																									Schema: map[string]*schema.Schema{
																										"lucene_query": {
																											Type:     schema.TypeString,
																											Optional: true,
																										},
																										"group_by": {
																											Type: schema.TypeList,
																											Elem: &schema.Schema{
																												Type: schema.TypeString,
																											},
																											Optional: true,
																										},
																										"aggregations": {
																											Type: schema.TypeList,
																											Elem: &schema.Resource{
																												Schema: map[string]*schema.Schema{
																													"count": {
																														Type:     schema.TypeList,
																														MaxItems: 1,
																														Elem: &schema.Resource{
																															Schema: map[string]*schema.Schema{},
																														},
																														Optional: true,
																													},
																													"count_distinct": {
																														Type:     schema.TypeList,
																														MaxItems: 1,
																														Elem: &schema.Resource{
																															Schema: map[string]*schema.Schema{
																																"field": {
																																	Type:     schema.TypeString,
																																	Required: true,
																																},
																															},
																														},
																														Optional: true,
																													},
																													"sum": {
																														Type:     schema.TypeList,
																														MaxItems: 1,
																														Elem: &schema.Resource{
																															Schema: map[string]*schema.Schema{
																																"field": {
																																	Type:     schema.TypeString,
																																	Required: true,
																																},
																															},
																														},
																														Optional: true,
																													},
																													"average": {
																														Type:     schema.TypeList,
																														MaxItems: 1,
																														Elem: &schema.Resource{
																															Schema: map[string]*schema.Schema{
																																"field": {
																																	Type:     schema.TypeString,
																																	Required: true,
																																},
																															},
																														},
																														Optional: true,
																													},
																													"min": {
																														Type:     schema.TypeList,
																														MaxItems: 1,
																														Elem: &schema.Resource{
																															Schema: map[string]*schema.Schema{
																																"field": {
																																	Type:     schema.TypeString,
																																	Required: true,
																																},
																															},
																														},
																														Optional: true,
																													},
																													"max": {
																														Type:     schema.TypeList,
																														MaxItems: 1,
																														Elem: &schema.Resource{
																															Schema: map[string]*schema.Schema{
																																"field": {
																																	Type:     schema.TypeString,
																																	Required: true,
																																},
																															},
																														},
																														Optional: true,
																													},
																												},
																											},
																											Optional: true,
																										},
																									},
																								},
																								Optional: true,
																							},
																							"metrics": {
																								Type:     schema.TypeList,
																								MaxItems: 1,
																								Elem: &schema.Resource{
																									Schema: map[string]*schema.Schema{
																										"promql_query": {
																											Type:     schema.TypeString,
																											Required: true,
																										},
																									},
																								},
																								Optional: true,
																							},
																						},
																					},
																					Optional: true,
																				},
																				"legend": {
																					Type:     schema.TypeList,
																					MaxItems: 1,
																					Elem: &schema.Resource{
																						Schema: map[string]*schema.Schema{
																							"is_visible": {
																								Type:     schema.TypeBool,
																								Required: true,
																							},
																							"columns": {
																								Type: schema.TypeList,
																								Elem: &schema.Schema{
																									Type:         schema.TypeString,
																									ValidateFunc: validation.StringInSlice(dashboardValidLegendColumn, false),
																								},
																								Optional: true,
																							},
																						},
																					},
																					Optional: true,
																				},
																				"series_name_template": {
																					Type:     schema.TypeString,
																					Optional: true,
																				},
																				"series_count_limit": {
																					Type:     schema.TypeInt,
																					Optional: true,
																				},
																			},
																		},
																		Optional: true,
																	},
																	"data_table": {
																		Type:     schema.TypeList,
																		MaxItems: 1,
																		Elem: &schema.Resource{
																			Schema: map[string]*schema.Schema{
																				"query": {
																					Type:     schema.TypeList,
																					MaxItems: 1,
																					Elem: &schema.Resource{
																						Schema: map[string]*schema.Schema{
																							"logs": {
																								Type:     schema.TypeList,
																								MaxItems: 1,
																								Elem: &schema.Resource{
																									Schema: map[string]*schema.Schema{
																										"lucene_query": {
																											Type:     schema.TypeString,
																											Optional: true,
																										},
																										"filters": {
																											Type: schema.TypeList,
																											Elem: &schema.Resource{
																												Schema: map[string]*schema.Schema{
																													"field": {
																														Type:     schema.TypeString,
																														Required: true,
																													},
																													"operator": {
																														Type:     schema.TypeList,
																														MaxItems: 1,
																														Elem: &schema.Resource{
																															Schema: map[string]*schema.Schema{
																																"equals": {
																																	Type:     schema.TypeList,
																																	MaxItems: 1,
																																	Elem: &schema.Resource{
																																		Schema: map[string]*schema.Schema{
																																			"selection": {
																																				Type:     schema.TypeList,
																																				MaxItems: 1,
																																				Elem: &schema.Resource{
																																					Schema: map[string]*schema.Schema{
																																						"all": {
																																							Type:     schema.TypeBool,
																																							Optional: true,
																																						},
																																						"list": {
																																							Type: schema.TypeList,
																																							Elem: &schema.Schema{
																																								Type: schema.TypeString,
																																							},
																																							Optional: true,
																																						},
																																					},
																																				},
																																				Optional: true,
																																			},
																																		},
																																	},
																																	Optional: true,
																																},
																															},
																														},
																														Required: true,
																													},
																												},
																											},
																											Optional: true,
																										},
																									},
																								},
																								Optional: true,
																							},
																						},
																					},
																					Optional: true,
																				},
																				"results_per_page": {
																					Type:     schema.TypeInt,
																					Optional: true,
																				},
																				"row_style": {
																					Type:         schema.TypeString,
																					ValidateFunc: validation.StringInSlice(dashboardValidRowStyle, false),
																					Required:     true,
																				},
																				"columns": {
																					Type: schema.TypeList,
																					Elem: &schema.Resource{
																						Schema: map[string]*schema.Schema{
																							"field": {
																								Type:     schema.TypeString,
																								Required: true,
																							},
																							"width": {
																								Type:     schema.TypeInt,
																								Optional: true,
																							},
																						},
																					},
																					Optional: true,
																				},
																				"order_by": {
																					Type:     schema.TypeList,
																					MaxItems: 1,
																					Elem: &schema.Resource{
																						Schema: map[string]*schema.Schema{
																							"field": {
																								Type:     schema.TypeString,
																								Required: true,
																							},
																							"order_direction": {
																								Type:         schema.TypeString,
																								Required:     true,
																								ValidateFunc: validation.StringInSlice(dashboardValidOrderDirection, false),
																							},
																						},
																					},
																					Optional: true,
																				},
																			},
																		},
																		Optional: true,
																	},
																	"gauge": {
																		Type:     schema.TypeList,
																		MaxItems: 1,
																		Elem: &schema.Resource{
																			Schema: map[string]*schema.Schema{
																				"query": {
																					Type:     schema.TypeList,
																					MaxItems: 1,
																					Elem: &schema.Resource{
																						Schema: map[string]*schema.Schema{
																							"metrics": {
																								Type:     schema.TypeList,
																								MaxItems: 1,
																								Elem: &schema.Resource{
																									Schema: map[string]*schema.Schema{
																										"promql_query": {
																											Type:     schema.TypeString,
																											Required: true,
																										},
																										"aggregation": {
																											Type:         schema.TypeString,
																											Required:     true,
																											ValidateFunc: validation.StringInSlice(dashboardValidAggregation, false),
																										},
																									},
																								},
																								Optional: true,
																							},
																						},
																					},
																					Optional: true,
																				},
																				"min": {
																					Type:     schema.TypeFloat,
																					Optional: true,
																				},
																				"max": {
																					Type:     schema.TypeFloat,
																					Optional: true,
																				},
																				"show_inner_arc": {
																					Type:     schema.TypeBool,
																					Optional: true,
																				},
																				"show_outer_arc": {
																					Type:     schema.TypeBool,
																					Optional: true,
																				},
																				"unit": {
																					Type:         schema.TypeString,
																					ValidateFunc: validation.StringInSlice(dashboardValidGaugeUnit, false),
																					Optional:     true,
																				},
																				"thresholds": {
																					Type: schema.TypeList,
																					Elem: &schema.Resource{
																						Schema: map[string]*schema.Schema{
																							"from": {
																								Type:     schema.TypeFloat,
																								Required: true,
																							},
																							"color": {
																								Type:     schema.TypeString,
																								Required: true,
																							},
																						},
																					},
																					Optional: true,
																				},
																			},
																		},
																		Optional: true,
																	},
																},
															},
															Optional: true,
														},
														"appearance": {
															Type:     schema.TypeList,
															MaxItems: 1,
															Elem: &schema.Resource{
																Schema: map[string]*schema.Schema{
																	"width": {
																		Type:     schema.TypeInt,
																		Required: true,
																	},
																},
															},
															Optional: true,
														},
													},
												},
												Optional: true,
											},
										},
									},
									Optional: true,
								},
							},
						},
						Optional: true,
					},
				},
			},
			Optional: true,
		},
		"variables": {
			Type: schema.TypeList,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"name": {
						Type:     schema.TypeString,
						Required: true,
					},
					"display_name": {
						Type:     schema.TypeString,
						Optional: true,
					},
					"definition": {
						Type:     schema.TypeList,
						MaxItems: 1,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"constant": {
									Type:     schema.TypeString,
									Optional: true,
								},
								"multi_select": {
									Type:     schema.TypeList,
									MaxItems: 1,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											"source": {
												Type:     schema.TypeList,
												MaxItems: 1,
												Elem: &schema.Resource{
													Schema: map[string]*schema.Schema{
														"logs_path": {
															Type:     schema.TypeString,
															Optional: true,
														},
														"metric_label": {
															Type:     schema.TypeList,
															MaxItems: 1,
															Elem: &schema.Resource{
																Schema: map[string]*schema.Schema{
																	"metric_name": {
																		Type:     schema.TypeString,
																		Required: true,
																	},
																	"label": {
																		Type:     schema.TypeString,
																		Required: true,
																	},
																},
															},
															Optional: true,
														},
														"constant_list": {
															Type: schema.TypeList,
															Elem: &schema.Schema{
																Type: schema.TypeString,
															},
															Optional: true,
														},
													},
												},
												Required: true,
											},
											"selection": {
												Type:     schema.TypeList,
												MaxItems: 1,
												Elem: &schema.Resource{
													Schema: map[string]*schema.Schema{
														"all": {
															Type:     schema.TypeBool,
															Optional: true,
														},
														"list": {
															Type: schema.TypeList,
															Elem: &schema.Schema{
																Type: schema.TypeString,
															},
															Optional: true,
														},
													},
												},
												Optional: true,
											},
											"values_order_direction": {
												Type:         schema.TypeString,
												Optional:     true,
												ValidateFunc: validation.StringInSlice(dashboardValidOrderDirection, false),
											},
										},
									},
									Optional: true,
								},
							},
						},
						Required: true,
					},
				},
			},
			Optional: true,
		},
		"filters": {
			Type: schema.TypeList,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"source": {
						Type:     schema.TypeList,
						MaxItems: 1,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"logs": {
									Type:     schema.TypeList,
									MaxItems: 1,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											"field": {
												Type:     schema.TypeString,
												Required: true,
											},
											"operator": {
												Type:     schema.TypeList,
												MaxItems: 1,
												Elem: &schema.Resource{
													Schema: map[string]*schema.Schema{
														"equals": {
															Type:     schema.TypeList,
															MaxItems: 1,
															Elem: &schema.Resource{
																Schema: map[string]*schema.Schema{
																	"selection": {
																		Type:     schema.TypeList,
																		MaxItems: 1,
																		Elem: &schema.Resource{
																			Schema: map[string]*schema.Schema{
																				"all": {
																					Type:     schema.TypeBool,
																					Optional: true,
																				},
																				"list": {
																					Type: schema.TypeList,
																					Elem: &schema.Schema{
																						Type: schema.TypeString,
																					},
																					Optional: true,
																				},
																			},
																		},
																		Required: true,
																	},
																},
															},
															Optional: true,
														},
													},
												},
												Required: true,
											},
										},
									},
									Optional: true,
								},
							},
						},
						Required: true,
					},
					"enabled": {
						Type:     schema.TypeBool,
						Optional: true,
						Default:  true,
					},
					"collapsed": {
						Type:     schema.TypeBool,
						Optional: true,
						Default:  false,
					},
				},
			},
			Optional: true,
		},
		"content_json": {
			Type:             schema.TypeString,
			Optional:         true,
			ConflictsWith:    []string{"layout", "name", "layout", "variables", "filters"},
			ValidateDiagFunc: dashboardContentJsonValidationFunc(),
			Description:      "an option to set the dashboard content from a json file.",
			DiffSuppressFunc: SuppressEquivalentJSONDiffs,
		},
	}
}

func extractDashboard(d *schema.ResourceData) (*dashboards.Dashboard, diag.Diagnostics) {
	if contentJson, ok := d.GetOk("content_json"); ok {
		dashboard := new(dashboards.Dashboard)
		err := protojson.Unmarshal([]byte(contentJson.(string)), dashboard)
		diags := diag.FromErr(err)
		return dashboard, diags
	}

	id := wrapperspb.String(expandUUID(d.Id()))
	name := wrapperspb.String(d.Get("name").(string))
	description := wrapperspb.String(d.Get("description").(string))
	layout, diags := expandLayout(d.Get("layout"))
	variables, dgs := expandVariables(d.Get("variables"))
	diags = append(diags, dgs...)
	filters, dgs := expandDashboardFilters(d.Get("filters"))
	diags = append(diags, dgs...)

	return &dashboards.Dashboard{
		Id:          id,
		Name:        name,
		Description: description,
		Layout:      layout,
		Variables:   variables,
		Filters:     filters,
	}, diags
}

func expandUUID(v interface{}) string {
	var id string
	if v == nil || v.(string) == "" {
		id = RandStringBytes(21)
	} else {
		id = v.(string)
	}
	return id
}

func expandLayout(v interface{}) (*dashboards.Layout, diag.Diagnostics) {
	var m map[string]interface{}
	if v == nil {
		return nil, nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil, nil
	} else {
		m = l[0].(map[string]interface{})
	}

	sections, diags := expandSections(m["sections"])
	return &dashboards.Layout{
		Sections: sections,
	}, diags

}

func expandVariables(i interface{}) ([]*dashboards.Variable, diag.Diagnostics) {
	if i == nil {
		return nil, nil
	}
	variables := i.([]interface{})
	result := make([]*dashboards.Variable, 0, len(variables))
	var diags diag.Diagnostics
	for _, v := range variables {
		variable, dgs := expandVariable(v)
		result = append(result, variable)
		diags = append(diags, dgs...)
	}
	return result, diags
}

func expandDashboardFilters(v interface{}) ([]*dashboards.Filter, diag.Diagnostics) {
	if v == nil {
		return nil, nil
	}
	filters := v.([]interface{})
	result := make([]*dashboards.Filter, 0, len(filters))
	var diags diag.Diagnostics
	for _, f := range filters {
		filter, dgs := expandDashboardFilter(f)
		result = append(result, filter)
		diags = append(diags, dgs...)
	}
	return result, diags
}

func expandDashboardFilter(v interface{}) (*dashboards.Filter, diag.Diagnostics) {
	m := v.(map[string]interface{})
	source := expandFilterSource(m["source"])
	enabled := wrapperspb.Bool(m["enabled"].(bool))
	collapsed := wrapperspb.Bool(m["collapsed"].(bool))
	return &dashboards.Filter{
		Source:    source,
		Enabled:   enabled,
		Collapsed: collapsed,
	}, nil
}

func expandFilterSource(v interface{}) *dashboards.Filter_Source {
	var m map[string]interface{}
	if v == nil {
		return nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil
	} else {
		m = l[0].(map[string]interface{})
	}

	logs := expandFilterSourceLogs(m["logs"])
	return &dashboards.Filter_Source{
		Value: &dashboards.Filter_Source_Logs{
			Logs: logs,
		},
	}
}

func expandFilterSourceLogs(v interface{}) *dashboards.Filter_LogFilter {
	var m map[string]interface{}
	if v == nil {
		return nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil
	} else {
		m = l[0].(map[string]interface{})
	}

	field := wrapperspb.String(m["field"].(string))
	operator := expandLogsOperator(m["operator"])
	return &dashboards.Filter_LogFilter{
		Field:    field,
		Operator: operator,
	}
}

func expandLogsOperator(v interface{}) *dashboards.Filter_LogFilter_Operator {
	var m map[string]interface{}
	if v == nil {
		return nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil
	} else {
		m = l[0].(map[string]interface{})
	}

	equals := expandOperatorEquals(m["equals"])
	return &dashboards.Filter_LogFilter_Operator{
		Value: &dashboards.Filter_LogFilter_Operator_Equals{
			Equals: equals,
		},
	}
}

func expandOperatorEquals(v interface{}) *dashboards.Filter_Equals {
	var m map[string]interface{}
	if v == nil {
		return nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil
	} else {
		m = l[0].(map[string]interface{})
	}

	if all, ok := m["all"].(bool); ok && all {
		return &dashboards.Filter_Equals{
			Selection: &dashboards.Filter_Selection{
				Value: &dashboards.Filter_Selection_All{},
			},
		}
	} else if list, ok := m["list"]; ok {
		values := interfaceSliceToWrappedStringSlice(list.([]interface{}))
		return &dashboards.Filter_Equals{
			Selection: &dashboards.Filter_Selection{
				Value: &dashboards.Filter_Selection_List{
					List: &dashboards.Filter_Selection_ListSelection{
						Values: values,
					},
				},
			},
		}
	}

	return nil
}

func expandSections(v interface{}) ([]*dashboards.Section, diag.Diagnostics) {
	if v == nil {
		return nil, nil
	}
	sections := v.([]interface{})
	result := make([]*dashboards.Section, 0, len(sections))
	var diags diag.Diagnostics
	for _, s := range sections {
		section, ds := expandSection(s)
		if ds != nil {
			diags = append(diags, ds...)
		}
		result = append(result, section)
	}
	return result, diags
}

func expandSection(v interface{}) (*dashboards.Section, diag.Diagnostics) {
	m := v.(map[string]interface{})
	uuid := &dashboards.UUID{Value: expandUUID(m["id"])}
	rows, diags := expandRows(m["rows"])
	return &dashboards.Section{
		Id:   uuid,
		Rows: rows,
	}, diags
}

func expandRows(v interface{}) ([]*dashboards.Row, diag.Diagnostics) {
	if v == nil {
		return nil, nil
	}
	rows := v.([]interface{})
	result := make([]*dashboards.Row, 0, len(rows))
	var diags diag.Diagnostics
	for _, r := range rows {
		row, ds := expandRow(r)
		if ds != nil {
			diags = append(diags, ds...)
		}
		result = append(result, row)
	}
	return result, diags
}

func expandRow(v interface{}) (*dashboards.Row, diag.Diagnostics) {
	m := v.(map[string]interface{})
	uuid := &dashboards.UUID{Value: expandUUID(m["id"])}
	appearance := expandRowAppearance(m["appearance"])
	widgets, diags := expandWidgets(m["widgets"])
	return &dashboards.Row{
		Id:         uuid,
		Appearance: appearance,
		Widgets:    widgets,
	}, diags
}

func expandRowAppearance(v interface{}) *dashboards.Row_Appearance {
	var m map[string]interface{}
	if v == nil {
		return nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil
	} else {
		m = l[0].(map[string]interface{})
	}

	height := wrapperspb.Int32(int32(m["height"].(int)))
	return &dashboards.Row_Appearance{
		Height: height,
	}
}

func expandWidgets(v interface{}) ([]*dashboards.Widget, diag.Diagnostics) {
	if v == nil {
		return nil, nil
	}
	widgets := v.([]interface{})
	result := make([]*dashboards.Widget, 0, len(widgets))
	var diags diag.Diagnostics
	for _, w := range widgets {
		widget, err := expandWidget(w)
		if err != nil {
			diags = append(diags, diag.FromErr(err)...)
		}
		result = append(result, widget)
	}
	return result, diags
}

func expandWidget(v interface{}) (*dashboards.Widget, error) {
	m := v.(map[string]interface{})
	id := &dashboards.UUID{Value: expandUUID(m["id"])}
	title := wrapperspb.String(m["title"].(string))
	description := wrapperspb.String(m["description"].(string))
	definition, err := expandWidgetDefinition(m["definition"])
	if err != nil {
		return nil, err
	}
	appearance := expandWidgetAppearance(m["appearance"])
	return &dashboards.Widget{
		Id:          id,
		Title:       title,
		Description: description,
		Definition:  definition,
		Appearance:  appearance,
	}, nil
}

func expandWidgetDefinition(v interface{}) (*dashboards.Widget_Definition, error) {
	var m map[string]interface{}
	if v == nil {
		return nil, nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil, nil
	} else {
		m = l[0].(map[string]interface{})
	}

	if l, ok := m["line_chart"]; ok && len(l.([]interface{})) != 0 {
		lineChart, err := expandLineChart(l.([]interface{})[0])
		if err != nil {
			return nil, err
		}
		return &dashboards.Widget_Definition{
			Value: lineChart,
		}, nil
	} else if l, ok = m["data_table"]; ok && len(l.([]interface{})) != 0 {
		dataTable := expandDataTable(l.([]interface{})[0])
		return &dashboards.Widget_Definition{
			Value: dataTable,
		}, nil
	} else if l, ok = m["gauge"]; ok && len(l.([]interface{})) != 0 {
		gauge := expandGauge(l.([]interface{})[0])
		return &dashboards.Widget_Definition{
			Value: gauge,
		}, nil
	}

	return nil, nil
}

func expandGauge(v interface{}) *dashboards.Widget_Definition_Gauge {
	m := v.(map[string]interface{})
	query := expandGaugeQuery(m["query"])
	min := wrapperspb.Double(m["min"].(float64))
	max := wrapperspb.Double(m["max"].(float64))
	showInnerArc := wrapperspb.Bool(m["show_inner_arc"].(bool))
	showOuterArc := wrapperspb.Bool(m["show_outer_arc"].(bool))
	unit := expandGaugeUnit(m["unit"])
	thresholds := expandGaugeThresholds(m["thresholds"])

	return &dashboards.Widget_Definition_Gauge{
		Gauge: &dashboards.Gauge{
			Query:        query,
			Min:          min,
			Max:          max,
			ShowInnerArc: showInnerArc,
			ShowOuterArc: showOuterArc,
			Unit:         unit,
			Thresholds:   thresholds,
		},
	}
}

func expandGaugeThresholds(v interface{}) []*dashboards.Gauge_Threshold {
	l := v.([]interface{})
	result := make([]*dashboards.Gauge_Threshold, 0, len(l))
	for _, gaugeThreshold := range l {
		threshold := expandThreshold(gaugeThreshold)
		result = append(result, threshold)
	}
	return result
}

func expandThreshold(v interface{}) *dashboards.Gauge_Threshold {
	m := v.(map[string]interface{})
	from := wrapperspb.Double(m["from"].(float64))
	color := wrapperspb.String(m["color"].(string))
	return &dashboards.Gauge_Threshold{
		From:  from,
		Color: color,
	}
}

func expandGaugeUnit(v interface{}) dashboards.Gauge_Unit {
	s := v.(string)
	unitStr := dashboardSchemaGaugeUnitToProtoGaugeUnit[s]
	return dashboards.Gauge_Unit(dashboards.Gauge_Unit_value[unitStr])
}

func expandGaugeQuery(v interface{}) *dashboards.Gauge_Query {
	var m map[string]interface{}
	if v == nil {
		return nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil
	} else {
		m = l[0].(map[string]interface{})
	}

	metrics := expandGaugeQueryMetric(m["metrics"])
	return &dashboards.Gauge_Query{
		Value: &dashboards.Gauge_Query_Metrics{
			Metrics: metrics,
		},
	}
}

func expandGaugeQueryMetric(v interface{}) *dashboards.Gauge_MetricsQuery {
	var m map[string]interface{}
	if v == nil {
		return nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil
	} else {
		m = l[0].(map[string]interface{})
	}

	promqlQuery := expandPromqlQuery(m["promql_query"])
	aggregation := expandGaugeAggregation(m["aggregation"])
	return &dashboards.Gauge_MetricsQuery{
		PromqlQuery: promqlQuery,
		Aggregation: aggregation,
	}
}

func expandPromqlQuery(v interface{}) *dashboards.PromQlQuery {
	s := v.(string)
	value := wrapperspb.String(s)
	return &dashboards.PromQlQuery{
		Value: value,
	}
}

func expandGaugeAggregation(v interface{}) dashboards.Gauge_Aggregation {
	s := v.(string)
	gaugeAggregationStr := dashboardSchemaAggregationToProtoAggregation[s]
	return dashboards.Gauge_Aggregation(dashboards.Gauge_Aggregation_value[gaugeAggregationStr])
}

func expandLineChart(v interface{}) (*dashboards.Widget_Definition_LineChart, error) {
	m := v.(map[string]interface{})
	query, err := expandLineChartQuery(m["query"])
	if err != nil {
		return nil, err
	}
	legend := expandLegend(m["legend"])
	seriesNameTemplate := wrapperspb.String(m["series_name_template"].(string))
	return &dashboards.Widget_Definition_LineChart{
		LineChart: &dashboards.LineChart{
			Query:              query,
			Legend:             legend,
			SeriesNameTemplate: seriesNameTemplate,
		},
	}, nil
}

func expandLineChartQuery(v interface{}) (*dashboards.LineChart_Query, error) {
	var m map[string]interface{}
	if v == nil {
		return nil, fmt.Errorf("line chart query cannot be empty")
	}
	if l := v.([]interface{}); len(l) == 0 || l[0] == nil {
		return nil, fmt.Errorf("line chart query cannot be empty")
	} else {
		m = l[0].(map[string]interface{})
	}

	if l, ok := m["logs"]; ok && len(l.([]interface{})) != 0 {
		lineChartQueryLogs := expandLineChartQueryLogs(l.([]interface{})[0])
		return &dashboards.LineChart_Query{
			Value: lineChartQueryLogs,
		}, nil
	} else if l, ok = m["metrics"]; ok && len(l.([]interface{})) != 0 {
		lineChartQueryMetrics := expandLineChartQueryMetric(l.([]interface{})[0])
		return &dashboards.LineChart_Query{
			Value: lineChartQueryMetrics,
		}, nil
	}

	return nil, fmt.Errorf("line chart query cannot be empty")
}

func expandLineChartQueryLogs(v interface{}) *dashboards.LineChart_Query_Logs {
	if v == nil {
		return &dashboards.LineChart_Query_Logs{}
	}
	m := v.(map[string]interface{})
	luceneQuery := &dashboards.LuceneQuery{Value: wrapperspb.String(m["lucene_query"].(string))}
	groupBy := interfaceSliceToWrappedStringSlice(m["group_by"].([]interface{}))
	aggregations := expandAggregations(m["aggregations"])
	return &dashboards.LineChart_Query_Logs{
		Logs: &dashboards.LineChart_LogsQuery{
			LuceneQuery:  luceneQuery,
			GroupBy:      groupBy,
			Aggregations: aggregations,
		},
	}
}

func expandAggregations(v interface{}) []*dashboards.LogsAggregation {
	if v == nil {
		return nil
	}
	aggregations := v.([]interface{})
	result := make([]*dashboards.LogsAggregation, 0, len(aggregations))
	for _, a := range aggregations {
		aggregation := expandAggregation(a)
		result = append(result, aggregation)
	}
	return result
}

func expandAggregation(v interface{}) *dashboards.LogsAggregation {
	if v == nil {
		return nil
	}
	m := v.(map[string]interface{})

	if l, ok := m["count"]; ok && len(l.([]interface{})) != 0 {
		return &dashboards.LogsAggregation{
			Value: &dashboards.LogsAggregation_Count_{
				Count: &dashboards.LogsAggregation_Count{},
			},
		}
	} else if l, ok = m["count_distinct"]; ok && len(l.([]interface{})) != 0 {
		m = l.([]interface{})[0].(map[string]interface{})
		field := wrapperspb.String(m["field"].(string))
		return &dashboards.LogsAggregation{
			Value: &dashboards.LogsAggregation_CountDistinct_{
				CountDistinct: &dashboards.LogsAggregation_CountDistinct{
					Field: field,
				},
			},
		}
	} else if l, ok = m["sum"]; ok && len(l.([]interface{})) != 0 {
		m = l.([]interface{})[0].(map[string]interface{})
		field := wrapperspb.String(m["field"].(string))
		return &dashboards.LogsAggregation{
			Value: &dashboards.LogsAggregation_Sum_{
				Sum: &dashboards.LogsAggregation_Sum{
					Field: field,
				},
			},
		}
	} else if l, ok = m["average"]; ok && len(l.([]interface{})) != 0 {
		m = l.([]interface{})[0].(map[string]interface{})
		field := wrapperspb.String(m["field"].(string))
		return &dashboards.LogsAggregation{
			Value: &dashboards.LogsAggregation_Average_{
				Average: &dashboards.LogsAggregation_Average{
					Field: field,
				},
			},
		}
	} else if l, ok = m["min"]; ok && len(l.([]interface{})) != 0 {
		m = l.([]interface{})[0].(map[string]interface{})
		field := wrapperspb.String(m["field"].(string))
		return &dashboards.LogsAggregation{
			Value: &dashboards.LogsAggregation_Min_{
				Min: &dashboards.LogsAggregation_Min{
					Field: field,
				},
			},
		}
	} else if l, ok = m["max"]; ok && len(l.([]interface{})) != 0 {
		m = l.([]interface{})[0].(map[string]interface{})
		field := wrapperspb.String(m["field"].(string))
		return &dashboards.LogsAggregation{
			Value: &dashboards.LogsAggregation_Max_{
				Max: &dashboards.LogsAggregation_Max{
					Field: field,
				},
			},
		}
	}

	return nil
}

func expandLineChartQueryMetric(v interface{}) *dashboards.LineChart_Query_Metrics {
	if v == nil {
		return &dashboards.LineChart_Query_Metrics{}
	}
	m := v.(map[string]interface{})
	promqlQuery := wrapperspb.String(m["promql_query"].(string))
	return &dashboards.LineChart_Query_Metrics{
		Metrics: &dashboards.LineChart_MetricsQuery{
			PromqlQuery: &dashboards.PromQlQuery{
				Value: promqlQuery,
			},
		},
	}
}

func expandLegend(v interface{}) *dashboards.Legend {
	var m map[string]interface{}
	if v == nil {
		return nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil
	} else {
		m = l[0].(map[string]interface{})
	}

	isVisible := wrapperspb.Bool(m["is_visible"].(bool))
	columns := expandLegendColumns(m["columns"])

	return &dashboards.Legend{
		IsVisible: isVisible,
		Columns:   columns,
	}
}

func expandLegendColumns(v interface{}) []dashboards.Legend_LegendColumn {
	if v == nil {
		return nil
	}
	legendColumns := v.([]interface{})
	result := make([]dashboards.Legend_LegendColumn, 0, len(legendColumns))
	for _, lc := range legendColumns {
		legend := expandLegendColumn(lc.(string))
		result = append(result, legend)
	}
	return result
}

func expandLegendColumn(legendColumn string) dashboards.Legend_LegendColumn {
	legendColumnStr := dashboardSchemaLegendColumnToProtoLegendColumn[legendColumn]
	legendColumnValue := dashboards.Legend_LegendColumn_value[legendColumnStr]
	return dashboards.Legend_LegendColumn(legendColumnValue)
}

func expandDataTable(v interface{}) *dashboards.Widget_Definition_DataTable {
	m := v.(map[string]interface{})
	query := expandDataTableQuery(m["query"])
	resultsPerPage := wrapperspb.Int32(int32(m["results_per_page"].(int)))
	rowStyle := expandRowStyle(m["row_style"].(string))
	columns := expandDataTableColumns(m["columns"])

	return &dashboards.Widget_Definition_DataTable{
		DataTable: &dashboards.DataTable{
			Query:          query,
			ResultsPerPage: resultsPerPage,
			RowStyle:       rowStyle,
			Columns:        columns,
		},
	}
}

func expandDataTableColumns(v interface{}) []*dashboards.DataTable_Column {
	if v == nil {
		return nil
	}
	dataTableColumns := v.([]interface{})
	result := make([]*dashboards.DataTable_Column, 0, len(dataTableColumns))
	for _, dtc := range dataTableColumns {
		dataTableColumn := expandDataTableColumn(dtc)
		result = append(result, dataTableColumn)
	}
	return result
}

func expandDataTableColumn(v interface{}) *dashboards.DataTable_Column {
	if v == nil {
		return nil
	}
	m := v.(map[string]interface{})

	field := wrapperspb.String(m["field"].(string))
	return &dashboards.DataTable_Column{
		Field: field,
	}

}

func expandRowStyle(s string) dashboards.RowStyle {
	rowStyleStr := dashboardSchemaRowStyleToProtoRowStyle[s]
	rowStyleValue := dashboards.RowStyle_value[rowStyleStr]
	return dashboards.RowStyle(rowStyleValue)
}

func expandDataTableQuery(v interface{}) *dashboards.DataTable_Query {
	var m map[string]interface{}
	if v == nil {
		return nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil
	} else {
		m = l[0].(map[string]interface{})
	}
	logsMap := m["logs"].([]interface{})[0].(map[string]interface{})

	luceneQuery := expandLuceneQuery(logsMap["lucene_query"])
	filters := expandSearchFilters(logsMap["filters"])
	return &dashboards.DataTable_Query{
		Value: &dashboards.DataTable_Query_Logs{
			Logs: &dashboards.DataTable_LogsQuery{
				LuceneQuery: luceneQuery,
				Filters:     filters,
			},
		},
	}
}

func expandLuceneQuery(v interface{}) *dashboards.LuceneQuery {
	query := v.(string)
	return &dashboards.LuceneQuery{
		Value: wrapperspb.String(query),
	}
}

func expandSearchFilters(v interface{}) []*dashboards.Filter_LogFilter {
	if v == nil {
		return nil
	}
	filters := v.([]interface{})
	result := make([]*dashboards.Filter_LogFilter, 0, len(filters))
	for _, f := range filters {
		filter := expandSearchFilter(f)
		result = append(result, filter)
	}
	return result
}

func expandSearchFilter(v interface{}) *dashboards.Filter_LogFilter {
	if v == nil {
		return nil
	}
	m := v.(map[string]interface{})
	field := wrapperspb.String(m["field"].(string))
	operator := expandFilterOperator(m["operator"])
	return &dashboards.Filter_LogFilter{
		Field:    field,
		Operator: operator,
	}
}

func expandFilterOperator(v interface{}) *dashboards.Filter_LogFilter_Operator {
	var m map[string]interface{}
	if v == nil {
		return nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil
	} else {
		m = l[0].(map[string]interface{})
	}

	if l, ok := m["equals"]; ok && len(l.([]interface{})) != 0 {
		m = l.([]interface{})[0].(map[string]interface{})
		selection := expandFilterSelection(m["selection"])
		return &dashboards.Filter_LogFilter_Operator{
			Value: &dashboards.Filter_LogFilter_Operator_Equals{
				Equals: &dashboards.Filter_Equals{
					Selection: selection,
				},
			},
		}
	}

	return nil
}

func expandFilterSelection(v interface{}) *dashboards.Filter_Selection {
	var m map[string]interface{}
	if v == nil {
		return nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil
	} else {
		m = l[0].(map[string]interface{})
	}

	if all, ok := m["all"].(bool); ok && all {
		return &dashboards.Filter_Selection{
			Value: &dashboards.Filter_Selection_All{
				All: &dashboards.Filter_Selection_AllSelection{},
			},
		}
	} else if list, ok := m["list"].([]interface{}); ok {
		values := interfaceSliceToWrappedStringSlice(list)
		return &dashboards.Filter_Selection{
			Value: &dashboards.Filter_Selection_List{
				List: &dashboards.Filter_Selection_ListSelection{
					Values: values,
				},
			},
		}
	}

	return nil
}

func expandWidgetAppearance(v interface{}) *dashboards.Widget_Appearance {
	var m map[string]interface{}
	if v == nil {
		return nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil
	} else {
		m = l[0].(map[string]interface{})
	}

	width := wrapperspb.Int32(int32(m["width"].(int)))
	return &dashboards.Widget_Appearance{
		Width: width,
	}
}

func expandVariable(v interface{}) (*dashboards.Variable, diag.Diagnostics) {
	if v == nil {
		return nil, nil
	}
	m := v.(map[string]interface{})
	name := wrapperspb.String(m["name"].(string))
	definition, diags := expandVariableDefinition(m["definition"])
	return &dashboards.Variable{
		Name:       name,
		Definition: definition,
	}, diags
}

func expandVariableDefinition(v interface{}) (*dashboards.Variable_Definition, diag.Diagnostics) {
	var m map[string]interface{}
	if v == nil {
		return nil, nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil, nil
	} else {
		m = l[0].(map[string]interface{})
	}

	if constant, ok := m["constant"]; ok && constant.(string) != "" {
		value := wrapperspb.String(constant.(string))
		return &dashboards.Variable_Definition{
			Value: &dashboards.Variable_Definition_Constant{
				Constant: &dashboards.Constant{
					Value: value,
				},
			},
		}, nil
	} else if l, ok := m["multi_select"]; ok && len(l.([]interface{})) != 0 {
		multiSelect := l.([]interface{})[0].(map[string]interface{})
		source, diags := expandSource(multiSelect["source"])
		selection := expandVariableSelection(multiSelect["selection"])
		valuesOrderDirection := expandValuesOrderDirection(multiSelect["values_order_direction"])
		return &dashboards.Variable_Definition{
			Value: &dashboards.Variable_Definition_MultiSelect{
				MultiSelect: &dashboards.MultiSelect{
					Source:               source,
					Selection:            selection,
					ValuesOrderDirection: valuesOrderDirection,
				},
			},
		}, diags
	}

	return nil, diag.Errorf("variable definition must contain exactly one of \"constant\" or \"multi_select\"")
}

func expandValuesOrderDirection(v interface{}) dashboards.OrderDirection {
	s := v.(string)
	orderDirectionStr := dashboards.OrderDirection_value[dashboardSchemaOrderDirectionToProtoOrderDirection[s]]
	return dashboards.OrderDirection(orderDirectionStr)
}

func expandVariableSelection(v interface{}) *dashboards.MultiSelect_Selection {
	var m map[string]interface{}
	if v == nil {
		return nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil
	} else {
		m = l[0].(map[string]interface{})
	}

	if all, ok := m["all"].(bool); ok && all {
		return &dashboards.MultiSelect_Selection{
			Value: &dashboards.MultiSelect_Selection_All{
				All: &dashboards.MultiSelect_Selection_AllSelection{},
			},
		}
	} else if list, ok := m["list"].([]interface{}); ok {
		values := interfaceSliceToWrappedStringSlice(list)
		return &dashboards.MultiSelect_Selection{
			Value: &dashboards.MultiSelect_Selection_List{
				List: &dashboards.MultiSelect_Selection_ListSelection{
					Values: values,
				},
			},
		}
	}

	return nil
}

func expandSource(v interface{}) (*dashboards.MultiSelect_Source, diag.Diagnostics) {
	var m map[string]interface{}
	if v == nil {
		return nil, nil
	}
	if l := v.([]interface{}); len(l) == 0 {
		return nil, nil
	} else {
		m = l[0].(map[string]interface{})
	}

	if logPath, ok := m["logs_path"]; ok && logPath.(string) != "" {
		value := wrapperspb.String(logPath.(string))
		return &dashboards.MultiSelect_Source{
			Value: &dashboards.MultiSelect_Source_LogsPath{
				LogsPath: &dashboards.MultiSelect_LogsPathSource{
					Value: value,
				},
			},
		}, nil
	} else if l, ok := m["metric_label"]; ok && len(l.([]interface{})) != 0 {
		metricLabel := l.([]interface{})[0].(map[string]interface{})
		metricName := wrapperspb.String(metricLabel["metric_name"].(string))
		label := wrapperspb.String(metricLabel["label"].(string))
		return &dashboards.MultiSelect_Source{
			Value: &dashboards.MultiSelect_Source_MetricLabel{
				MetricLabel: &dashboards.MultiSelect_MetricLabelSource{
					MetricName: metricName,
					Label:      label,
				},
			},
		}, nil
	} else if constantList, ok := m["constant_list"].([]interface{}); ok {
		values := interfaceSliceToWrappedStringSlice(constantList)
		return &dashboards.MultiSelect_Source{
			Value: &dashboards.MultiSelect_Source_ConstantList{
				ConstantList: &dashboards.MultiSelect_ConstantListSource{
					Values: values,
				},
			},
		}, nil
	}

	return nil, diag.Errorf("source must contain exactly one of \"logs_path\", \"metric_label\" or \"constant_list\"")
}

func setDashboard(d *schema.ResourceData, dashboard *dashboards.Dashboard) diag.Diagnostics {
	if _, ok := d.GetOk("content_json"); ok {
		contentJson, err := protojson.Marshal(dashboard)
		if err != nil {
			return diag.FromErr(err)
		}

		if err = d.Set("content_json", string(contentJson)); err != nil {
			return diag.FromErr(err)
		}

		return nil
	}

	if err := d.Set("name", dashboard.GetName().GetValue()); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("description", dashboard.GetDescription().GetValue()); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("layout", flattenLayout(dashboard.GetLayout())); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("variables", flattenVariables(dashboard.GetVariables())); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("filters", flattenDashboardFilters(dashboard.GetFilters())); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func flattenLayout(layout *dashboards.Layout) interface{} {
	sections := flattenSections(layout.GetSections())
	return []interface{}{
		map[string]interface{}{
			"sections": sections,
		},
	}
}

func flattenVariables(variables []*dashboards.Variable) interface{} {
	result := make([]interface{}, 0, len(variables))
	for _, v := range variables {
		variable := flattenVariable(v)
		result = append(result, variable)
	}
	return result
}

func flattenDashboardFilters(filters []*dashboards.Filter) interface{} {
	result := make([]interface{}, 0, len(filters))
	for _, f := range filters {
		variable := flattenDashboardFilter(f)
		result = append(result, variable)
	}
	return result
}

func flattenSections(sections []*dashboards.Section) interface{} {
	result := make([]interface{}, 0, len(sections))
	for _, s := range sections {
		section := flattenSection(s)
		result = append(result, section)
	}
	return result
}

func flattenSection(section *dashboards.Section) interface{} {
	id := section.GetId().GetValue()
	rows := flattenRows(section.GetRows())
	return map[string]interface{}{
		"id":   id,
		"rows": rows,
	}
}

func flattenRows(rows []*dashboards.Row) interface{} {
	result := make([]interface{}, 0, len(rows))
	for _, r := range rows {
		row := flattenRow(r)
		result = append(result, row)
	}
	return result
}

func flattenRow(row *dashboards.Row) interface{} {
	id := row.GetId().GetValue()
	appearance := flattenRowAppearance(row.GetAppearance())
	widgets := flattenWidgets(row.GetWidgets())
	return map[string]interface{}{
		"id":         id,
		"appearance": appearance,
		"widgets":    widgets,
	}
}

func flattenRowAppearance(appearance *dashboards.Row_Appearance) interface{} {
	return []interface{}{
		map[string]interface{}{
			"height": appearance.GetHeight().GetValue(),
		},
	}
}

func flattenWidgets(widgets []*dashboards.Widget) interface{} {
	result := make([]interface{}, 0, len(widgets))
	for _, w := range widgets {
		widget := flattenWidget(w)
		result = append(result, widget)
	}
	return result
}

func flattenWidget(widget *dashboards.Widget) interface{} {
	id := widget.GetId().GetValue()
	title := widget.GetTitle().GetValue()
	description := widget.GetDescription().GetValue()
	definition := flattenWidgetDefinition(widget.GetDefinition())
	appearance := flattenWidgetAppearance(widget.GetAppearance())
	return map[string]interface{}{
		"id":          id,
		"title":       title,
		"description": description,
		"definition":  definition,
		"appearance":  appearance,
	}
}

func flattenWidgetDefinition(definition *dashboards.Widget_Definition) interface{} {
	var widgetDefinition map[string]interface{}
	switch definitionValue := definition.GetValue().(type) {
	case *dashboards.Widget_Definition_LineChart:
		lineChart := flattenLineChart(definitionValue.LineChart)
		widgetDefinition = map[string]interface{}{
			"line_chart": lineChart,
		}
	case *dashboards.Widget_Definition_DataTable:
		dataTable := flattenDataTable(definitionValue.DataTable)
		widgetDefinition = map[string]interface{}{
			"data_table": dataTable,
		}
	}

	return []interface{}{
		widgetDefinition,
	}
}

func flattenLineChart(lineChart *dashboards.LineChart) interface{} {
	query := flattenLineChartQuery(lineChart.GetQuery())
	legend := flattenLegend(lineChart.GetLegend())
	seriesNameTemplate := lineChart.GetSeriesNameTemplate().GetValue()
	return []interface{}{
		map[string]interface{}{
			"query":                query,
			"legend":               legend,
			"series_name_template": seriesNameTemplate,
		},
	}
}

func flattenLineChartQuery(query *dashboards.LineChart_Query) interface{} {
	var queryMap interface{}
	switch queryValue := query.GetValue().(type) {
	case *dashboards.LineChart_Query_Logs:
		queryMap = map[string]interface{}{
			"logs": flattenLineChartLogsQuery(queryValue.Logs),
		}
	case *dashboards.LineChart_Query_Metrics:
		queryMap = map[string]interface{}{
			"metrics": flattenLineChartMetricsQuery(queryValue.Metrics),
		}
	}

	return []interface{}{
		queryMap,
	}
}

func flattenLineChartLogsQuery(logs *dashboards.LineChart_LogsQuery) interface{} {
	luceneQuery := logs.GetLuceneQuery().GetValue().GetValue()
	groupBy := wrappedStringSliceToStringSlice(logs.GetGroupBy())
	aggregations := flattenAggregations(logs.GetAggregations())
	return []interface{}{
		map[string]interface{}{
			"lucene_query": luceneQuery,
			"group_by":     groupBy,
			"aggregations": aggregations,
		},
	}
}

func flattenAggregations(aggregations []*dashboards.LogsAggregation) interface{} {
	result := make([]interface{}, 0, len(aggregations))
	for _, a := range aggregations {
		aggregation := flattenAggregation(a)
		result = append(result, aggregation)
	}
	return result
}

func flattenAggregation(aggregation *dashboards.LogsAggregation) interface{} {
	switch aggregationValue := aggregation.GetValue().(type) {
	case *dashboards.LogsAggregation_Count_:
		return map[string]interface{}{
			"count": []interface{}{
				map[string]interface{}{},
			},
		}
	case *dashboards.LogsAggregation_CountDistinct_:
		return map[string]interface{}{
			"count_distinct": []interface{}{
				map[string]interface{}{
					"field": aggregationValue.CountDistinct.GetField().GetValue(),
				},
			},
		}
	case *dashboards.LogsAggregation_Sum_:
		return map[string]interface{}{
			"sum": []interface{}{
				map[string]interface{}{
					"field": aggregationValue.Sum.GetField().GetValue(),
				},
			},
		}
	case *dashboards.LogsAggregation_Average_:
		return map[string]interface{}{
			"average": []interface{}{
				map[string]interface{}{
					"field": aggregationValue.Average.GetField().GetValue(),
				},
			},
		}
	case *dashboards.LogsAggregation_Min_:
		return map[string]interface{}{
			"min": []interface{}{
				map[string]interface{}{
					"field": aggregationValue.Min.GetField().GetValue(),
				},
			},
		}
	case *dashboards.LogsAggregation_Max_:
		return map[string]interface{}{
			"max": []interface{}{
				map[string]interface{}{
					"field": aggregationValue.Max.GetField().GetValue(),
				},
			},
		}
	}

	return nil
}

func flattenLineChartMetricsQuery(metrics *dashboards.LineChart_MetricsQuery) interface{} {
	promqlQuery := metrics.GetPromqlQuery().GetValue().GetValue()
	return []interface{}{
		map[string]interface{}{
			"promql_query": promqlQuery,
		},
	}
}

func flattenLegend(legend *dashboards.Legend) interface{} {
	isVisible := legend.IsVisible.GetValue()
	columns := flattenLegendColumns(legend.GetColumns())
	return []interface{}{
		map[string]interface{}{
			"is_visible": isVisible,
			"columns":    columns,
		},
	}
}

func flattenLegendColumns(columns []dashboards.Legend_LegendColumn) interface{} {
	result := make([]string, 0, len(columns))
	for _, c := range columns {
		column := flattenLegendColumn(c)
		result = append(result, column)
	}

	return result
}

func flattenLegendColumn(column dashboards.Legend_LegendColumn) string {
	columnStr := dashboards.Legend_LegendColumn_name[int32(column)]
	return dashboardProtoLegendColumnToSchemaLegendColumn[columnStr]
}

func flattenDataTable(dataTable *dashboards.DataTable) interface{} {
	query := flattenDataTableQuery(dataTable.GetQuery())
	resultsPerPage := dataTable.GetResultsPerPage().GetValue()
	rowStyle := flattenRowStyle(dataTable.GetRowStyle())
	columns := flattenDataTableColumns(dataTable.GetColumns())
	return []interface{}{
		map[string]interface{}{
			"query":            query,
			"results_per_page": resultsPerPage,
			"row_style":        rowStyle,
			"columns":          columns,
		},
	}
}

func flattenDataTableColumns(columns []*dashboards.DataTable_Column) interface{} {
	result := make([]interface{}, 0, len(columns))
	for _, c := range columns {
		column := flattenDataTableColumn(c)
		result = append(result, column)
	}

	return result
}

func flattenDataTableColumn(column *dashboards.DataTable_Column) interface{} {
	field := column.GetField().GetValue()
	return map[string]interface{}{
		"field": field,
	}
}

func flattenRowStyle(rowStyle dashboards.RowStyle) string {
	rowStyleStr := dashboards.RowStyle_name[int32(rowStyle)]
	return dashboardProtoRowStyleToSchemaRowStyle[rowStyleStr]
}

func flattenDataTableQuery(query *dashboards.DataTable_Query) interface{} {
	logs := flattenDataTableLogsQuery(query.GetLogs())
	return []interface{}{
		map[string]interface{}{
			"logs": logs,
		},
	}
}

func flattenDataTableLogsQuery(logs *dashboards.DataTable_LogsQuery) interface{} {
	luceneQuery := logs.GetLuceneQuery().GetValue().GetValue()
	filters := flattenDataTableFilters(logs.GetFilters())
	return []interface{}{
		map[string]interface{}{
			"lucene_query": luceneQuery,
			"filters":      filters,
		},
	}
}

func flattenDataTableFilters(filters []*dashboards.Filter_LogFilter) interface{} {
	result := make([]interface{}, 0, len(filters))
	for _, f := range filters {
		filter := flattenDataTableFilter(f)
		result = append(result, filter)
	}
	return result
}

func flattenDataTableFilter(filter *dashboards.Filter_LogFilter) interface{} {
	field := filter.GetField().GetValue()
	operator := flattenDataTableFilterOperator(filter.GetOperator())
	return map[string]interface{}{
		"field":    field,
		"operator": operator,
	}
}

func flattenDataTableFilterOperator(operator *dashboards.Filter_LogFilter_Operator) interface{} {
	equals := flattenEquals(operator.GetEquals())
	return []interface{}{
		map[string]interface{}{
			"equals": equals,
		},
	}
}

func flattenEquals(equals *dashboards.Filter_Equals) interface{} {
	selection := flattenSelection(equals.GetSelection())
	return []interface{}{
		map[string]interface{}{
			"selection": selection,
		},
	}
}

func flattenSelection(selection *dashboards.Filter_Selection) interface{} {
	switch selectionType := selection.GetValue().(type) {
	case *dashboards.Filter_Selection_All:
		return []interface{}{
			map[string]interface{}{
				"all": true,
			},
		}
	case *dashboards.Filter_Selection_List:
		list := wrappedStringSliceToStringSlice(selectionType.List.GetValues())
		return []interface{}{
			map[string]interface{}{
				"list": list,
			},
		}
	}

	return nil
}

func flattenWidgetAppearance(appearance *dashboards.Widget_Appearance) interface{} {
	return []interface{}{
		map[string]interface{}{
			"width": appearance.GetWidth().GetValue(),
		},
	}
}

func flattenVariable(variable *dashboards.Variable) interface{} {
	name := variable.GetName().GetValue()
	definition := flattenVariableDefinition(variable.GetDefinition())
	return map[string]interface{}{
		"name":       name,
		"definition": definition,
	}
}

func flattenVariableDefinition(definition *dashboards.Variable_Definition) interface{} {
	var definitionMap map[string]interface{}
	switch definitionValue := definition.GetValue().(type) {
	case *dashboards.Variable_Definition_Constant:
		constant := flattenConstant(definitionValue.Constant)
		definitionMap = map[string]interface{}{
			"constant": constant,
		}
	case *dashboards.Variable_Definition_MultiSelect:
		multiSelect := flattenMultiSelect(definitionValue.MultiSelect)
		definitionMap = map[string]interface{}{
			"multi_select": multiSelect,
		}
	}
	return []interface{}{
		definitionMap,
	}
}

func flattenConstant(constant *dashboards.Constant) interface{} {
	return []interface{}{
		map[string]interface{}{
			"value": constant.GetValue().GetValue(),
		},
	}
}

func flattenMultiSelect(multiSelect *dashboards.MultiSelect) interface{} {
	selection := flattenMultiSelectSelection(multiSelect.GetSelection())
	source := flattenMultiSelectSource(multiSelect.GetSource())
	return []interface{}{
		map[string]interface{}{
			"selection": selection,
			"source":    source,
		},
	}
}

func flattenMultiSelectSource(source *dashboards.MultiSelect_Source) interface{} {
	var sourceMap map[string]interface{}
	switch sourceValue := source.GetValue().(type) {
	case *dashboards.MultiSelect_Source_LogsPath:
		logsPath := flattenLogPathSource(sourceValue.LogsPath)
		sourceMap = map[string]interface{}{
			"log_path": logsPath,
		}
	case *dashboards.MultiSelect_Source_MetricLabel:
		metricLabel := flattenMetricLabelSource(sourceValue.MetricLabel)
		sourceMap = map[string]interface{}{
			"metric_label": metricLabel,
		}
	case *dashboards.MultiSelect_Source_ConstantList:
		constantList := wrappedStringSliceToStringSlice(sourceValue.ConstantList.GetValues())
		sourceMap = map[string]interface{}{
			"constant_list": constantList,
		}
	}
	return []interface{}{
		sourceMap,
	}
}

func flattenMultiSelectSelection(selection *dashboards.MultiSelect_Selection) interface{} {
	switch selectionType := selection.GetValue().(type) {
	case *dashboards.MultiSelect_Selection_All:
		return []interface{}{
			map[string]interface{}{
				"all": true,
			},
		}
	case *dashboards.MultiSelect_Selection_List:
		list := wrappedStringSliceToStringSlice(selectionType.List.GetValues())
		return []interface{}{
			map[string]interface{}{
				"list": list,
			},
		}
	}

	return nil
}

func flattenLogPathSource(logPath *dashboards.MultiSelect_LogsPathSource) interface{} {
	value := logPath.GetValue().GetValue()
	return []interface{}{
		map[string]interface{}{
			"value": value,
		},
	}
}

func flattenMetricLabelSource(metricLabel *dashboards.MultiSelect_MetricLabelSource) interface{} {
	metricName := metricLabel.GetMetricName().GetValue()
	label := metricLabel.GetLabel().GetValue()
	return []interface{}{
		map[string]interface{}{
			"metric_name": metricName,
			"label":       label,
		},
	}
}

func flattenDashboardFilter(filter *dashboards.Filter) interface{} {
	source := flattenFilterSource(filter.GetSource())
	enabled := filter.GetEnabled().GetValue()
	collapsed := filter.GetCollapsed().GetValue()

	return map[string]interface{}{
		"source":    source,
		"enabled":   enabled,
		"collapsed": collapsed,
	}
}

func flattenFilterSource(source *dashboards.Filter_Source) interface{} {
	logs := flattenLogFilter(source.GetLogs())
	return []interface{}{
		map[string]interface{}{
			"logs": logs,
		},
	}
}

func flattenLogFilter(logs *dashboards.Filter_LogFilter) interface{} {
	field := logs.GetField().GetValue()
	operator := flattenLogFilterOperator(logs.GetOperator())
	return []interface{}{
		map[string]interface{}{
			"field":    field,
			"operator": operator,
		},
	}
}

func flattenLogFilterOperator(operator *dashboards.Filter_LogFilter_Operator) interface{} {
	equal := flattenEquals(operator.GetEquals())
	return []interface{}{
		map[string]interface{}{
			"equal": equal,
		},
	}
}

func dashboardContentJsonValidationFunc() schema.SchemaValidateDiagFunc {
	return func(v interface{}, _ cty.Path) diag.Diagnostics {
		err := protojson.Unmarshal([]byte(v.(string)), &dashboards.Dashboard{})
		if err != nil {
			return diag.Errorf("json content is not matching layout schema. got an err while unmarshalling - %s", err)
		}
		return nil
	}
}
