package coralogix

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-coralogix/coralogix/clientset"
	dashboards "terraform-provider-coralogix/coralogix/clientset/grpc/coralogix-dashboards/v1"
)

func dataSourceCoralogixDashboard() *schema.Resource {
	dashboardSchema := datasourceSchemaFromResourceSchema(DashboardSchema())
	dashboardSchema["id"] = &schema.Schema{
		Type:     schema.TypeString,
		Required: true,
	}

	return &schema.Resource{
		ReadContext: dataSourceCoralogixDashboardRead,

		Schema: dashboardSchema,
	}
}

func dataSourceCoralogixDashboardRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Get("id").(string)
	log.Printf("[INFO] Reading dashboard %s", id)
	dashboardId := wrapperspb.String(expandUUID(id))
	resp, err := meta.(*clientset.ClientSet).Dashboards().GetDashboard(ctx, &dashboards.GetDashboardRequest{DashboardId: dashboardId})
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "dashboard", id)
	}

	dashboard := resp.GetDashboard()
	log.Printf("[INFO] Received dashboard: %#v", dashboard)

	d.SetId(dashboard.GetId().GetValue())

	return setDashboard(d, dashboard)
}
