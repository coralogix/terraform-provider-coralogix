package coralogix

import (
	"context"
	"encoding/json"
	"strings"

	"terraform-provider-coralogix/coralogix/clientset"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceCoralogixHostedDashboard() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceHostedDashboardRead,

		Schema: map[string]*schema.Schema{
			"grafana": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"uid": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The uid of the Grafana dashboard.",
						},
						"dashboard_id": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "The numeric ID of the dashboard computed by Grafana.",
						},
						"config_json": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The complete dashboard model JSON.",
						},
						"version": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "The numerical version of the Grafana dashboard.",
						},
						"title": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The title of the Grafana dashboard.",
						},
						"folder": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "The numerical ID of the folder where the Grafana dashboard is found.",
						},
						"is_starred": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "Whether or not the Grafana dashboard is starred. Starred Dashboards will show up on your own Home Dashboard by default, and are a convenient way to mark Dashboards that you’re interested in.",
						},
						"url": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The full URL of the dashboard.",
						},
					},
				},
				Description: `Hosted grafana dashboard.
			* [Official documentation](https://grafana.com/docs/grafana/latest/dashboards/)
			* [HTTP API](https://grafana.com/docs/grafana/latest/http_api/dashboard/)`,
			},
			"uid": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The unique identifier of a dashboard with the dashboard-type prefix (e.g. - grafana:vgvvfknr)",
			},
		},
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
}

func extractDashboardTypeAndUIDFromID(uid string) (string, string) {
	arr := strings.Split(uid, ":")
	return arr[0], arr[1]
}

func dataSourceHostedGrafanaDashboardRead(ctx context.Context, d *schema.ResourceData, meta interface{}, uid string) diag.Diagnostics {
	dashboard, err := meta.(*clientset.ClientSet).Grafana().GetGrafanaDashboard(ctx, uid)
	if err != nil {
		return diag.FromErr(err)
	}

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
	hostedGrafanaNewSchema["url"] = strings.TrimRight(meta.(*clientset.ClientSet).Grafana().GetTargetURL(), "/") + dashboard.Meta.URL

	if err = d.Set("grafana", []interface{}{hostedGrafanaNewSchema}); err != nil {
		return diag.FromErr(err)
	}

	return nil
}
