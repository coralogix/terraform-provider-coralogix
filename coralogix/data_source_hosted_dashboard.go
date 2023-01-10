package coralogix

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"terraform-provider-coralogix/coralogix/clientset"
)

func dataSourceCoralogixHostedDashboard() *schema.Resource {
	grafanaDashboardSchema := datasourceSchemaFromResourceSchema(HostedDashboardSchema())
	grafanaDashboardSchema["uid"] = &schema.Schema{
		Type:        schema.TypeString,
		Required:    true,
		Description: "The unique identifier of a dashboard with the dashboard-type prefix (e.g. - grafana:vgvvfknr)",
	}

	return &schema.Resource{
		ReadContext: dataSourceHostedDashboardRead,

		Schema: grafanaDashboardSchema,
	}
}

func dataSourceHostedDashboardRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	hostedDashboardTypeStr, uid := extractDashboardTypeAndUIDFromID(d.Get("uid").(string))

	switch hostedDashboardTypeStr {
	case "grafana":
		return dataSourceHostedGrafanaDashboardRead(ctx, d, meta, uid)
	default:
		return diag.Errorf("unknown hosted-dashboard type %s", hostedDashboardTypeStr)
	}

	return nil

}

func extractDashboardTypeAndUIDFromID(uid string) (string, string) {
	arr := strings.Split(uid, ":")
	return arr[0], arr[1]
}

func dataSourceHostedGrafanaDashboardRead(ctx context.Context, d *schema.ResourceData, meta interface{}, uid string) diag.Diagnostics {
	dashboard, err := meta.(*clientset.ClientSet).GrafanaDashboards().GetGrafanaDashboard(ctx, uid)

	d.SetId("grafana:" + dashboard.Model["uid"].(string))
	hostedGrafanaNewSchema := make(map[string]interface{})
	hostedGrafanaNewSchema["uid"] = dashboard.Model["uid"].(string)
	hostedGrafanaNewSchema["dashboard_id"] = int64(dashboard.Model["id"].(float64))
	configJSONBytes, err := json.Marshal(dashboard.Model)
	if err != nil {
		return diag.FromErr(err)
	}
	hostedGrafanaNewSchema["config_json"] = string(configJSONBytes)
	hostedGrafanaNewSchema["version"] = int64(dashboard.Model["version"].(float64))
	hostedGrafanaNewSchema["title"] = dashboard.Model["title"].(string)
	hostedGrafanaNewSchema["folder"] = dashboard.FolderID
	hostedGrafanaNewSchema["is_starred"] = dashboard.Meta.IsStarred
	hostedGrafanaNewSchema["url"] = strings.TrimRight(meta.(*clientset.ClientSet).GrafanaDashboards().GetTargetURL(), "/") + dashboard.Meta.URL

	d.Set("grafana", []interface{}{hostedGrafanaNewSchema})
	return nil
}
