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

package coralogix

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"terraform-provider-coralogix/coralogix/clientset"
	"terraform-provider-coralogix/coralogix/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	gapi "github.com/grafana/grafana-api-golang-client"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"google.golang.org/grpc/codes"
)

func resourceGrafanaFolder() *schema.Resource {
	return &schema.Resource{

		Description: `
* [Official documentation](https://grafana.com/docs/grafana/latest/dashboards/manage-dashboards/)
* [HTTP API](https://grafana.com/docs/grafana/latest/developers/http_api/folder/)
`,

		CreateContext: CreateFolder,
		DeleteContext: DeleteFolder,
		ReadContext:   ReadFolder,
		UpdateContext: UpdateFolder,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"uid": {
				Type:        schema.TypeString,
				Computed:    true,
				Optional:    true,
				Description: "Unique identifier.",
			},
			"title": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The title of the folder.",
			},
			"url": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The full URL of the folder.",
			},
			"prevent_destroy_if_not_empty": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Prevent deletion of the folder if it is not empty (contains dashboards or alert rules).",
			},
		},
	}
}

func isAlreadyExistsErr(err error) bool {
	if err == nil {
		return false
	}
	// SDK maps HTTP codes into gRPC codes for consistency.
	if cxsdk.Code(err) == codes.AlreadyExists {
		return true
	}
	// Be defensive: also catch raw HTTP 409 or text messages.
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "409") || strings.Contains(msg, "already exists")
}

func CreateFolder(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var err error
	var folder gapi.Folder
	folder.Title = d.Get("title").(string)
	if uid, ok := d.GetOk("uid"); ok {
		folder.UID = uid.(string)
	}

	log.Printf("[INFO] Creating grafana-folder: %#v", folder)
	resp, err := meta.(*clientset.ClientSet).Grafana().CreateGrafanaFolder(ctx, folder)
	log.Printf("[INFO] Received err: %#v", err)
	if err != nil {
		if isAlreadyExistsErr(err) {
			log.Printf("[INFO] Received isAlreadyExistsErr: %#v", err)
			return diag.Diagnostics{diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "Grafana folder already exists",
				Detail: fmt.Sprintf(
					"A folder (or dashboard) named %q already exists in the General folder. "+
						"Choose a different `title`, or import the existing folder into state. "+
						"\n\nExample:\n  terraform import coralogix_grafana_folder.this <folder_id>\n\n"+
						"Tip: the folder id is visible in Grafana under /folders/id/<id>.", folder.Title),
			}}
		}
		log.Printf("[INFO] Received error: %#v", err)
		return diag.Errorf("%s", utils.FormatRpcErrors(err, "/grafana/api/folders", fmt.Sprintf("%#v", folder)))
	}
	log.Printf("[INFO] Received grafana-folder: %#v", resp)

	flattenGrafanaFolder(*resp, d, meta)

	return ReadFolder(ctx, d, meta)
}

func expandGrafanaFolder(d *schema.ResourceData) gapi.FolderPayload {
	var folder gapi.FolderPayload
	if v, ok := d.GetOk("title"); ok {
		folder.Title = v.(string)
	}
	if v, ok := d.GetOk("uid"); ok {
		folder.UID = v.(string)
	}
	folder.Overwrite = true
	return folder
}

func flattenGrafanaFolder(folder gapi.Folder, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	d.SetId(fmt.Sprintf("%d", folder.ID))

	if err := d.Set("title", folder.Title); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("uid", folder.UID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("url", strings.TrimRight(meta.(*clientset.ClientSet).Grafana().GetTargetURL(), "/")+folder.URL); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func UpdateFolder(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	folder := expandGrafanaFolder(d)
	log.Printf("[INFO] Updating grafana-folder: %#v", folder)
	resp, err := meta.(*clientset.ClientSet).Grafana().UpdateGrafanaFolder(ctx, folder)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		return diag.FromErr(err)
	}
	flattenGrafanaFolder(*resp, d, meta)

	return ReadFolder(ctx, d, meta)
}

// SplitOrgResourceID splits into two parts (org ID and resource ID) the ID of an org-scoped resource
func SplitOrgResourceID(id string) (int64, string) {
	if strings.ContainsRune(id, ':') {
		parts := strings.SplitN(id, ":", 2)
		orgID, _ := strconv.ParseInt(parts[0], 10, 64)
		return orgID, parts[1]
	}

	return 0, id
}

func ReadFolder(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	folder, err := meta.(*clientset.ClientSet).Grafana().GetGrafanaFolder(ctx, d.Id())
	if err != nil {
		log.Printf("[ERROR] ReadFolder error: %s", err.Error())
		log.Printf("[ERROR] Error type: %T", err)
		// Check if it's a "not found" error or permission error - if so, return warning but keep ID for recreation
		if strings.Contains(strings.ToLower(err.Error()), "not found") ||
			strings.Contains(err.Error(), "status: 404") ||
			strings.Contains(err.Error(), "folders:read") ||
			strings.Contains(err.Error(), "access denied") {
			log.Printf("[INFO] Detected 'not found' or permission error, folder will be recreated")
			return diag.Diagnostics{diag.Diagnostic{
				Severity: diag.Warning,
				Summary:  fmt.Sprintf("Grafana folder %q is in state, but no longer exists or is not accessible in Coralogix backend", d.Id()),
				Detail:   fmt.Sprintf("%s will be recreated when you apply", d.Id()),
			}}
		}
		log.Printf("[ERROR] Not a 'not found' error, returning error")
		return diag.FromErr(err)
	}
	log.Printf("[INFO] Received grafana-folder: %#v", folder)

	return flattenGrafanaFolder(*folder, d, meta)
}

func DeleteFolder(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	folder := expandGrafanaFolder(d)
	log.Printf("[INFO] Deleting grafana-folder id: %s", folder.UID)
	err := meta.(*clientset.ClientSet).Grafana().DeleteGrafanaFolder(ctx, folder.UID)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			d.SetId("")
			return diag.Diagnostics{diag.Diagnostic{
				Severity: diag.Warning,
				Summary:  fmt.Sprintf("grafana-dashboard %q is in state, but no longer exists in Coralogix backend", d.Id()),
				Detail:   fmt.Sprintf("%s will be recreated when you apply", d.Id()),
			}}
		}
		return diag.Errorf("%s", utils.FormatRpcErrors(err, fmt.Sprintf("/grafana/api/folders/%s", folder.UID), fmt.Sprintf("%#v", folder)))
	}

	d.SetId("")
	return nil
}
