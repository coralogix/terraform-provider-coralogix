package coralogix

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"terraform-provider-coralogix/coralogix/clientset"

	. "github.com/ahmetalpbalkan/go-linq"
	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var (
	idRegexp                  = regexp.MustCompile(`^\d+$`)
	validHostedDashboardTypes = []string{"grafana"}
)

func resourceCoralogixHostedDashboard() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceHostedDashboardCreate,
		ReadContext:   resourceHostedDashboardRead,
		UpdateContext: resourceHostedDashboardUpdate,
		DeleteContext: resourceHostedDashboardDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(60 * time.Second),
			Read:   schema.DefaultTimeout(30 * time.Second),
			Update: schema.DefaultTimeout(60 * time.Second),
			Delete: schema.DefaultTimeout(30 * time.Second),
		},

		Schema: HostedDashboardSchema(),

		Description: fmt.Sprintf("Hosted dashboard. Can be one of - %q.", validHostedDashboardTypes),
	}
}

func resourceHostedDashboardCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	hostedDashboardTypeStr := From(validHostedDashboardTypes).FirstWith(func(key interface{}) bool {
		return len(d.Get(key.(string)).([]interface{})) > 0
	}).(string)

	hostedDashboardTypeSchema := d.Get(hostedDashboardTypeStr).([]interface{})[0].(map[string]interface{})

	switch hostedDashboardTypeStr {
	case "grafana":
		return resourceGrafanaDashboardCreate(ctx, d, meta, hostedDashboardTypeSchema)
	default:
		return diag.Errorf("unknown hosted-dashboard type %s", hostedDashboardTypeStr)
	}
}

func resourceHostedDashboardRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	hostedDashboardTypeStr := From(validHostedDashboardTypes).FirstWith(func(key interface{}) bool {
		return len(d.Get(key.(string)).([]interface{})) > 0
	}).(string)

	hostedDashboardTypeSchema := d.Get(hostedDashboardTypeStr).([]interface{})[0].(map[string]interface{})

	switch hostedDashboardTypeStr {
	case "grafana":
		return resourceGrafanaDashboardRead(ctx, d, meta, hostedDashboardTypeSchema)
	default:
		return diag.Errorf("unknown hosted-dashboard type %s", hostedDashboardTypeStr)
	}
}

func resourceHostedDashboardUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	hostedDashboardTypeStr := From(validHostedDashboardTypes).FirstWith(func(key interface{}) bool {
		return len(d.Get(key.(string)).([]interface{})) > 0
	}).(string)

	hostedDashboardTypeSchema := d.Get(hostedDashboardTypeStr).([]interface{})[0].(map[string]interface{})

	switch hostedDashboardTypeStr {
	case "grafana":
		return resourceGrafanaDashboardUpdate(ctx, d, meta, hostedDashboardTypeSchema)
	default:
		return diag.Errorf("unknown hosted-dashboard type %s", hostedDashboardTypeStr)
	}
}

func resourceHostedDashboardDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	hostedDashboardTypeStr := From(validHostedDashboardTypes).FirstWith(func(key interface{}) bool {
		return len(d.Get(key.(string)).([]interface{})) > 0
	}).(string)

	switch hostedDashboardTypeStr {
	case "grafana":
		return resourceGrafanaDashboardDelete(ctx, d, meta)
	default:
		return diag.Errorf("unknown hosted-dashboard type %s", hostedDashboardTypeStr)
	}
}

func HostedDashboardSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"grafana": {
			Type:     schema.TypeList,
			Optional: true,
			MaxItems: 1,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"uid": {
						Type:     schema.TypeString,
						Computed: true,
						Description: "The unique identifier of a dashboard. This is used to construct its URL. " +
							"It's automatically generated if not provided when creating a dashboard. " +
							"The uid allows having consistent URLs for accessing dashboards and when syncing dashboards between multiple Grafana installs. ",
					},
					"dashboard_id": {
						Type:        schema.TypeInt,
						Computed:    true,
						Description: "The numeric ID of the dashboard computed by Grafana.",
					},
					"url": {
						Type:        schema.TypeString,
						Computed:    true,
						Description: "The full URL of the dashboard.",
					},
					"version": {
						Type:     schema.TypeInt,
						Computed: true,
						Description: "Whenever you save a version of your dashboard, a copy of that version is saved " +
							"so that previous versions of your dashboard are not lost.",
					},
					"folder": {
						Type:         schema.TypeString,
						Optional:     true,
						ForceNew:     true,
						Description:  "The id of the folder to save the dashboard in. This attribute is a string to reflect the type of the folder's id.",
						ValidateFunc: validation.StringMatch(idRegexp, "must be a valid folder id"),
						DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
							return old == "0" && new == "" || old == "" && new == "0"
						},
					},
					"config_json": {
						Type:         schema.TypeString,
						Required:     true,
						StateFunc:    normalizeDashboardConfigJSON,
						ValidateFunc: validateDashboardConfigJSON,
						Description:  "The complete dashboard model JSON.",
					},
					"overwrite": {
						Type:        schema.TypeBool,
						Optional:    true,
						Description: "Set to true if you want to overwrite existing dashboard with newer version, same dashboard title in folder or same dashboard uid.",
					},
					"message": {
						Type:        schema.TypeString,
						Optional:    true,
						Description: "Set a commit message for the version history.",
					},
				},
			},
			Description: `Hosted grafana dashboard.
			* [Official documentation](https://grafana.com/docs/grafana/latest/dashboards/)
			* [HTTP API](https://grafana.com/docs/grafana/latest/http_api/dashboard/)`,
			ExactlyOneOf: validHostedDashboardTypes,
		},
	}
}

func resourceGrafanaDashboardCreate(ctx context.Context, d *schema.ResourceData, meta interface{}, hostedGrafanaSchema map[string]interface{}) diag.Diagnostics {
	dashboard, err := makeGrafanaDashboard(hostedGrafanaSchema)
	if err != nil {
		return diag.FromErr(err)
	}

	resp, err := meta.(*clientset.ClientSet).GrafanaDashboards().CreateGrafanaDashboard(ctx, dashboard)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("grafana:" + resp.UID)
	return resourceGrafanaDashboardRead(ctx, d, meta, hostedGrafanaSchema)
}

func resourceGrafanaDashboardRead(ctx context.Context, d *schema.ResourceData, meta interface{}, hostedGrafanaSchema map[string]interface{}) diag.Diagnostics {
	uid := extractOriginalUID(d, "grafana")
	dashboard, err := meta.(*clientset.ClientSet).GrafanaDashboards().GetGrafanaDashboard(ctx, uid)

	var diags diag.Diagnostics
	if err != nil {
		if strings.HasPrefix(err.Error(), "status: 404") {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Warning,
				Summary:  fmt.Sprintf("Dashboard %q is in state, but no longer exists in grafana", uid),
				Detail:   fmt.Sprintf("%q will be recreated when you apply", uid),
			})
			d.SetId("")
			return diags
		} else {
			return diag.FromErr(err)
		}
	}

	d.SetId("grafana:" + dashboard.Model["uid"].(string))
	hostedGrafanaNewSchema := make(map[string]interface{})
	hostedGrafanaNewSchema["uid"] = dashboard.Model["uid"].(string)
	hostedGrafanaNewSchema["dashboard_id"] = int64(dashboard.Model["id"].(float64))
	hostedGrafanaNewSchema["version"] = int64(dashboard.Model["version"].(float64))
	hostedGrafanaNewSchema["url"] = strings.TrimRight(meta.(*clientset.ClientSet).GrafanaDashboards().GetTargetURL(), "/") + dashboard.Meta.URL
	if dashboard.FolderID > 0 {
		hostedGrafanaNewSchema["folder"] = strconv.FormatInt(dashboard.FolderID, 10)
	} else {
		hostedGrafanaNewSchema["folder"] = ""
	}

	configJSONBytes, err := json.Marshal(dashboard.Model)
	if err != nil {
		return diag.FromErr(err)
	}
	remoteDashJSON, err := unmarshalDashboardConfigJSON(string(configJSONBytes))
	if err != nil {
		return diag.FromErr(err)
	}

	configJSON := hostedGrafanaSchema["config_json"].(string)

	// Skip if `uid` is not set in configuration, we need to delete it from the
	// dashboard JSON we just read from the Grafana API. This is so it does not
	// create a diff. We can assume the uid was randomly generated by Grafana or
	// it was removed after dashboard creation. In any case, the user doesn't
	// care to manage it.
	if configJSON != "" {
		configuredDashJSON, err := unmarshalDashboardConfigJSON(configJSON)
		if err != nil {
			return diag.FromErr(err)
		}
		if _, ok := configuredDashJSON["uid"].(string); !ok {
			delete(remoteDashJSON, "uid")
		}
	}
	configJSON = normalizeDashboardConfigJSON(remoteDashJSON)
	hostedGrafanaNewSchema["config_json"] = configJSON

	if err = d.Set("grafana", []interface{}{hostedGrafanaNewSchema}); err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}

	return diags
}

func makeGrafanaDashboard(hostedGrafanaSchema map[string]interface{}) (gapi.Dashboard, error) {
	var parsedFolder int64 = 0
	var err error
	if folderStr := hostedGrafanaSchema["folder"].(string); folderStr != "" {
		parsedFolder, err = strconv.ParseInt(hostedGrafanaSchema["folder"].(string), 10, 64)
		if err != nil {
			return gapi.Dashboard{}, fmt.Errorf("error parsing folder: %s", err)
		}
	}

	dashboard := gapi.Dashboard{
		FolderID:  parsedFolder,
		Overwrite: hostedGrafanaSchema["overwrite"].(bool),
		Message:   hostedGrafanaSchema["message"].(string),
	}
	configJSON := hostedGrafanaSchema["config_json"].(string)
	dashboardJSON, err := unmarshalDashboardConfigJSON(configJSON)
	if err != nil {
		return dashboard, err
	}
	delete(dashboardJSON, "id")
	delete(dashboardJSON, "version")
	dashboard.Model = dashboardJSON
	return dashboard, nil
}

func resourceGrafanaDashboardUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}, hostedGrafanaSchema map[string]interface{}) diag.Diagnostics {
	dashboard, err := makeGrafanaDashboard(hostedGrafanaSchema)
	if err != nil {
		return diag.FromErr(err)
	}
	dashboard.Model["id"] = hostedGrafanaSchema["dashboard_id"].(int)
	dashboard.Overwrite = true
	resp, err := meta.(*clientset.ClientSet).GrafanaDashboards().UpdateGrafanaDashboard(ctx, dashboard)
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId("grafana:" + resp.UID)
	return resourceGrafanaDashboardRead(ctx, d, meta, hostedGrafanaSchema)
}

func resourceGrafanaDashboardDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	uid := extractOriginalUID(d, "grafana")
	err := meta.(*clientset.ClientSet).GrafanaDashboards().DeleteGrafanaDashboard(ctx, uid)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}

func extractOriginalUID(d *schema.ResourceData, dashboardType string) string {
	return strings.Split(d.Id(), fmt.Sprintf("%s:", dashboardType))[1]
}

// validateDashboardConfigJSON is the ValidateFunc for `config_json`. It
// ensures its value is valid JSON.
func validateDashboardConfigJSON(config interface{}, _ string) ([]string, []error) {
	configJSON := config.(string)
	configMap := map[string]interface{}{}
	err := json.Unmarshal([]byte(configJSON), &configMap)
	if err != nil {
		return nil, []error{err}
	}
	return nil, nil
}

// normalizeDashboardConfigJSON is the StateFunc for the `config_json` field.
//
// It removes the following fields:
//
//   - `id`:      an auto-incrementing ID Grafana assigns to dashboards upon
//     creation. We cannot know this before creation and therefore it cannot
//     be managed in code.
//   - `version`: is incremented by Grafana each time a dashboard changes.
func normalizeDashboardConfigJSON(config interface{}) string {
	var dashboardJSON map[string]interface{}
	switch c := config.(type) {
	case map[string]interface{}:
		dashboardJSON = c
	case string:
		var err error
		dashboardJSON, err = unmarshalDashboardConfigJSON(c)
		if err != nil {
			return c
		}
	}

	delete(dashboardJSON, "id")
	delete(dashboardJSON, "version")

	// similarly to uid removal above, remove any attributes panels[].libraryPanel.*
	// from the dashboard JSON other than "name" or "uid".
	// Grafana will populate all other libraryPanel attributes, so delete them to avoid diff.
	panels, hasPanels := dashboardJSON["panels"]
	if hasPanels {
		for _, panel := range panels.([]interface{}) {
			panelMap := panel.(map[string]interface{})
			delete(panelMap, "id")
			if libraryPanel, ok := panelMap["libraryPanel"].(map[string]interface{}); ok {
				for k := range libraryPanel {
					if k != "name" && k != "uid" {
						delete(libraryPanel, k)
					}
				}
			}
		}
	}

	j, _ := json.Marshal(dashboardJSON)
	return string(j)
}

// unmarshalDashboardConfigJSON is a convenience func for unmarshalling
// `config_json` field.
func unmarshalDashboardConfigJSON(configJSON string) (map[string]interface{}, error) {
	dashboardJSON := map[string]interface{}{}
	err := json.Unmarshal([]byte(configJSON), &dashboardJSON)
	if err != nil {
		return nil, err
	}
	return dashboardJSON, nil
}
