package coralogix

import (
	"context"
	"log"
	"time"

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

		Description: "Coralogix Dashboard. Api-key is required for this resource." +
			" More info: https://coralogix.com/docs/dashboard-widgets/ .",
	}
}

func resourceCoralogixDashboardCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	createDashboardRequest, err := extractCreateDashboardRequest(d)
	if err != nil {
		return diag.FromErr(err)
	}

	log.Printf("[INFO] Creating new dashboardv1: %#v", createDashboardRequest)
	DashboardResp, err := meta.(*clientset.ClientSet).Dashboards().CreateDashboard(ctx, createDashboardRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err, "dashboardv1")
	}
	Dashboard := DashboardResp.ProtoReflect()
	log.Printf("[INFO] Submitted new dashboardv1: %#v", Dashboard)
	d.SetId(createDashboardRequest.GetDashboard().GetId().GetValue())

	return resourceCoralogixDashboardRead(ctx, d, meta)
}

func extractCreateDashboardRequest(d *schema.ResourceData) (*dashboardv1.CreateDashboardRequest, error) {
	name := wrapperspb.String(d.Get("name").(string))
	description := wrapperspb.String(d.Get("description").(string))

	createDashboardRequest := &dashboardv1.CreateDashboardRequest{
		RequestId: nil,
		Dashboard: &dashboardv1.Dashboard{
			Id:          nil,
			Name:        name,
			Description: description,
			Layout: &dashboardv1.Layout{
				Sections: []*dashboardv1.Section{
					{
						Id:   nil,
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
		},
	}

	return createDashboardRequest, nil
}

func resourceCoralogixDashboardRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	log.Printf("[INFO] Reading dashboardv1 %s", id)
	resp, err := meta.(*clientset.ClientSet).Dashboards().GetDashboard(ctx, &dashboardv1.GetDashboardRequest{DashboardId: nil})
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "dashboardv1", id)
	}
	dashboard := resp.GetDashboard()
	log.Printf("[INFO] Received dashboardv1: %#v", dashboard)

	return setDashboard(d, dashboard)
}

func setDashboard(d *schema.ResourceData, dashboard *dashboardv1.Dashboard) diag.Diagnostics {
	return nil
}

func resourceCoralogixDashboardUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return nil
}

func resourceCoralogixDashboardDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return nil
}

func DashboardSchema() map[string]*schema.Schema {
	return nil
}
