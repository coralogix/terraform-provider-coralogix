// Copyright 2024 Coralogix Ltd.
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

package data_exploration

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"

	. "github.com/ahmetalpbalkan/go-linq"
	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	idRegexp                  = regexp.MustCompile(`^\d+$`)
	validHostedDashboardTypes = []string{"grafana"}
)

// getHostedDashboardType determines the dashboard type from either the ID prefix
// (for import operations) or the schema data (for normal CRUD operations).
// During import, the schema is empty, so we extract the type from the ID (e.g., "grafana:uid").
func getHostedDashboardType(d *schema.ResourceData) (string, error) {
	// First, try to extract type from ID prefix (handles import case)
	if id := d.Id(); id != "" && strings.Contains(id, ":") {
		dashboardType := strings.Split(id, ":")[0]
		for _, validType := range validHostedDashboardTypes {
			if dashboardType == validType {
				return dashboardType, nil
			}
		}
	}

	// Fall back to checking schema (for create/update operations)
	result := From(validHostedDashboardTypes).FirstWith(func(key interface{}) bool {
		schemaData := d.Get(key.(string)).([]interface{})
		return len(schemaData) > 0
	})
	if result != nil {
		return result.(string), nil
	}

	return "", fmt.Errorf("unable to determine hosted dashboard type from ID %q or schema", d.Id())
}

// getHostedDashboardSchema retrieves the schema data for the given dashboard type.
// Returns nil if no schema data exists (e.g., during import before Read populates state).
func getHostedDashboardSchema(d *schema.ResourceData, dashboardType string) map[string]interface{} {
	schemaData := d.Get(dashboardType).([]interface{})
	if len(schemaData) == 0 || schemaData[0] == nil {
		return nil
	}
	return schemaData[0].(map[string]interface{})
}

func ResourceCoralogixHostedDashboard() *schema.Resource {
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
	hostedDashboardTypeStr, err := getHostedDashboardType(d)
	if err != nil {
		return diag.FromErr(err)
	}

	hostedDashboardTypeSchema := getHostedDashboardSchema(d, hostedDashboardTypeStr)
	if hostedDashboardTypeSchema == nil {
		return diag.Errorf("no configuration found for hosted dashboard type %q", hostedDashboardTypeStr)
	}

	switch hostedDashboardTypeStr {
	case "grafana":
		return resourceGrafanaDashboardCreate(ctx, d, meta, hostedDashboardTypeSchema)
	default:
		return diag.Errorf("unknown hosted-dashboard type %s", hostedDashboardTypeStr)
	}
}

func resourceHostedDashboardRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	hostedDashboardTypeStr, err := getHostedDashboardType(d)
	if err != nil {
		return diag.FromErr(err)
	}

	// During import, schema is empty - pass nil and let the read function handle it
	hostedDashboardTypeSchema := getHostedDashboardSchema(d, hostedDashboardTypeStr)

	switch hostedDashboardTypeStr {
	case "grafana":
		return resourceGrafanaDashboardRead(ctx, d, meta, hostedDashboardTypeSchema)
	default:
		return diag.Errorf("unknown hosted-dashboard type %s", hostedDashboardTypeStr)
	}
}

func resourceHostedDashboardUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	hostedDashboardTypeStr, err := getHostedDashboardType(d)
	if err != nil {
		return diag.FromErr(err)
	}

	hostedDashboardTypeSchema := getHostedDashboardSchema(d, hostedDashboardTypeStr)
	if hostedDashboardTypeSchema == nil {
		return diag.Errorf("no configuration found for hosted dashboard type %q", hostedDashboardTypeStr)
	}

	switch hostedDashboardTypeStr {
	case "grafana":
		return resourceGrafanaDashboardUpdate(ctx, d, meta, hostedDashboardTypeSchema)
	default:
		return diag.Errorf("unknown hosted-dashboard type %s", hostedDashboardTypeStr)
	}
}

func resourceHostedDashboardDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	hostedDashboardTypeStr, err := getHostedDashboardType(d)
	if err != nil {
		return diag.FromErr(err)
	}

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
						Type:        schema.TypeString,
						Optional:    true,
						ForceNew:    true,
						Description: "The id or UID of the folder to save the dashboard in.",
						DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
							_, old = SplitOrgResourceID(old)
							_, new = SplitOrgResourceID(new)
							return old == "0" && new == "" || old == "" && new == "0" || old == new
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

	resp, err := meta.(*clientset.ClientSet).Grafana().CreateGrafanaDashboard(ctx, dashboard)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("grafana:" + resp.UID)
	return resourceGrafanaDashboardRead(ctx, d, meta, hostedGrafanaSchema)
}

func resourceGrafanaDashboardRead(ctx context.Context, d *schema.ResourceData, meta interface{}, hostedGrafanaSchema map[string]interface{}) diag.Diagnostics {
	uid := extractOriginalUID(d, "grafana")
	dashboard, err := meta.(*clientset.ClientSet).Grafana().GetGrafanaDashboard(ctx, uid)

	var diags diag.Diagnostics
	if err != nil {
		if status.Code(err) == codes.NotFound {
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
	hostedGrafanaNewSchema["url"] = strings.TrimRight(meta.(*clientset.ClientSet).Grafana().GetTargetURL(), "/") + dashboard.Meta.URL

	// If the folder was originally set to a numeric ID, we read the folder ID
	// Otherwise, we read the folder UID
	// During import, hostedGrafanaSchema may be nil, so we check the state directly
	var folderID string
	if schemaData := d.Get("grafana").([]interface{}); len(schemaData) > 0 && schemaData[0] != nil {
		m := schemaData[0].(map[string]interface{})
		if folder, ok := m["folder"]; ok && folder != nil {
			_, folderID = SplitOrgResourceID(folder.(string))
		}
	}
	if idRegexp.MatchString(folderID) && dashboard.Meta.Folder > 0 {
		hostedGrafanaNewSchema["folder"] = strconv.FormatInt(dashboard.Meta.Folder, 10)
	} else {
		hostedGrafanaNewSchema["folder"] = dashboard.Meta.FolderUID
	}

	configJSONBytes, err := json.Marshal(dashboard.Model)
	if err != nil {
		return diag.FromErr(err)
	}
	remoteDashJSON, err := unmarshalDashboardConfigJSON(string(configJSONBytes))
	if err != nil {
		return diag.FromErr(err)
	}

	// Get config_json from schema if available (nil during import)
	var configJSON string
	if hostedGrafanaSchema != nil {
		if cfg, ok := hostedGrafanaSchema["config_json"].(string); ok {
			configJSON = cfg
		}
	}

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
	dashboard := gapi.Dashboard{
		Overwrite: hostedGrafanaSchema["overwrite"].(bool),
		Message:   hostedGrafanaSchema["message"].(string),
	}

	_, folderID := SplitOrgResourceID(hostedGrafanaSchema["folder"].(string))
	if folderInt, err := strconv.ParseInt(folderID, 10, 64); err == nil {
		dashboard.FolderID = folderInt
	} else {
		dashboard.FolderUID = folderID
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
	resp, err := meta.(*clientset.ClientSet).Grafana().UpdateGrafanaDashboard(ctx, dashboard)
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId("grafana:" + resp.UID)
	return resourceGrafanaDashboardRead(ctx, d, meta, hostedGrafanaSchema)
}

func resourceGrafanaDashboardDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	uid := extractOriginalUID(d, "grafana")
	err := meta.(*clientset.ClientSet).Grafana().DeleteGrafanaDashboard(ctx, uid)
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
