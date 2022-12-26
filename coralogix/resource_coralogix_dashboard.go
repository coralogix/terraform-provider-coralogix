package coralogix

import (
	"context"
	"log"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"terraform-provider-coralogix/coralogix/clientset"
	dashboardv1 "terraform-provider-coralogix/coralogix/clientset/grpc/com/coralogix/coralogix-dashboards"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"google.golang.org/protobuf/types/known/wrapperspb"
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

		Description: "Coralogix Dashboard. Api-key is required for this resource.",
	}
}

func resourceCoralogixDashboardCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	dashboard, err := extractDashboard(d)
	if err != nil {
		return diag.FromErr(err)
	}
	createDashboardRequest := &dashboardv1.CreateDashboardRequest{
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

func extractDashboard(d *schema.ResourceData) (*dashboardv1.Dashboard, error) {
	name := wrapperspb.String(d.Get("name").(string))
	description := wrapperspb.String(d.Get("description").(string))

	return &dashboardv1.Dashboard{
		Name:        name,
		Description: description,
		Layout: &dashboardv1.Layout{
			Sections: []*dashboardv1.Section{
				{
					Rows: []*dashboardv1.Row{{}},
				}},
		},
		Variables: []*dashboardv1.Variable{
			{
				Name: name,
				Definition: &dashboardv1.Variable_Definition{
					Value: &dashboardv1.Variable_Definition_MultiSelect{
						MultiSelect: &dashboardv1.MultiSelect{
							Selected: nil,
							Source: &dashboardv1.MultiSelect_Source{
								Value: &dashboardv1.MultiSelect_Source_LogsPath{
									LogsPath: &dashboardv1.MultiSelect_LogsPathSource{
										Value: nil,
									},
								},
							},
						},
					},
				},
			},
		},
	}, nil
}

func resourceCoralogixDashboardRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	log.Printf("[INFO] Reading dashboard %s", id)
	resp, err := meta.(*clientset.ClientSet).Dashboards().GetDashboard(ctx, &dashboardv1.GetDashboardRequest{DashboardId: nil})
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "dashboard", id)
	}

	dashboard := resp.GetDashboard()
	log.Printf("[INFO] Received dashboard: %#v", dashboard)

	return setDashboard(d, dashboard)
}

func setDashboard(d *schema.ResourceData, dashboard *dashboardv1.Dashboard) diag.Diagnostics {
	return nil
}

func resourceCoralogixDashboardUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	dashboard, err := extractDashboard(d)
	if err != nil {
		return diag.FromErr(err)
	}
	updateDashboardRequest := &dashboardv1.ReplaceDashboardRequest{
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
	deleteAlertRequest := &dashboardv1.DeleteDashboardRequest{DashboardId: &dashboardv1.UUID{Value: id}}
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
			Required:     true,
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
									Type:     schema.TypeList,
									MaxItems: 1,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											"id": {
												Type:     schema.TypeString,
												Computed: true,
											},
											"appearance": {
												Type:     schema.TypeList,
												Computed: true,
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
																Schema: map[string]*schema.Schema{},
															},
														},
														"appearance": {
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
																											Type:     schema.TypeList,
																											Optional: true,
																											Elem: &schema.Schema{
																												Type: schema.TypeString,
																											},
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
																													},
																												},
																											},
																										},
																									},
																								},
																							},
																							"metrics": {},
																						},
																					},
																				},
																				"legend": {
																					Type:     schema.TypeList,
																					MaxItems: 1,
																					Elem: &schema.Resource{
																						Schema: map[string]*schema.Schema{
																							"is_visible": {
																								Type: schema.TypeBool,
																							},
																							"columns": {
																								Type: schema.TypeList,
																								Elem: &schema.Schema{
																									Type:         schema.TypeString,
																									ValidateFunc: validation.StringInSlice([]string{}, false),
																								},
																							},
																						},
																					},
																				},
																				"series_name_template": {
																					Type:     schema.TypeString,
																					Optional: true,
																				},
																			},
																		},
																	},
																	"data_table": {},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			Optional:    true,
			Description: "Dashboard description.",
		},
		"variables": {
			Type: schema.TypeList,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"name": {},
					"definition": {
						Type: schema.TypeList,
						Elem: &schema.Resource{
							Schema: map[string]*schema.Schema{
								"constant": {
									Type:     schema.TypeList,
									MaxItems: 1,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											"value": {
												Type:     schema.TypeString,
												Required: true,
											},
										},
									},
									Optional: true,
								},
								"multi_select": {
									Type:     schema.TypeList,
									MaxItems: 1,
									Elem: &schema.Resource{
										Schema: map[string]*schema.Schema{
											"selected": {
												Elem: &schema.Schema{
													Type: schema.TypeString,
												},
											},
											"source": {
												Type:     schema.TypeList,
												MaxItems: 1,
												Elem: &schema.Resource{
													Schema: map[string]*schema.Schema{},
												},
											},
										},
									},
									Optional: true,
								},
							},
						},
					},
				},
			},
			Optional:    true,
			Description: "Dashboard description.",
		},
		dashboardv1.Dashboard{},
	}
}
