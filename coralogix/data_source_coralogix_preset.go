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
	"encoding/json"
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"terraform-provider-coralogix/coralogix/clientset"
	"terraform-provider-coralogix/coralogix/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var _ datasource.DataSourceWithConfigure = &PresetDataSource{}

func NewPresetDataSource() datasource.DataSource {
	return &PresetDataSource{}
}

type PresetDataSource struct {
	client *cxsdk.NotificationsClient
}

func (d *PresetDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_preset"
}

func (d *PresetDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clientSet, ok := req.ProviderData.(*clientset.ClientSet)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *clientset.ClientSet, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = clientSet.GetNotifications()
}

func (d *PresetDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r PresetResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	resp.Schema = utils.FrameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)

	if idAttr, ok := resp.Schema.Attributes["id"].(schema.StringAttribute); ok {
		idAttr.Required = false
		idAttr.Optional = true
		idAttr.Validators = []validator.String{
			stringvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("name")),
		}
		resp.Schema.Attributes["id"] = idAttr
	}

	if nameAttr, ok := resp.Schema.Attributes["name"].(schema.StringAttribute); ok {
		nameAttr.Required = false
		nameAttr.Optional = true
		resp.Schema.Attributes["name"] = nameAttr
	}
}

func (d *PresetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *PresetResourceModel
	diags := req.Config.Get(ctx, &data)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	var presetID string
	//Get refreshed preset value from Coralogix
	if name := data.Name.ValueString(); name != "" {
		log.Printf("[INFO] Listing presets to find by name: %s", name)
		listPresetReq := &cxsdk.ListPresetSummariesRequest{EntityType: cxsdk.EntityTypeAlerts}
		listPresetResp, err := d.client.ListPresetSummaries(ctx, listPresetReq)
		if err != nil {
			log.Printf("[ERROR] Received error when listing presets: %s", err.Error())
			listPresetReqStr, _ := json.Marshal(listPresetResp)
			resp.Diagnostics.AddError(
				"Error listing presets",
				utils.FormatRpcErrors(err, "List", string(listPresetReqStr)),
			)
			return
		}

		for _, preset := range listPresetResp.PresetSummaries {
			if preset.Name == data.Name.ValueString() {
				presetID = preset.Id
				break
			}
		}

		if presetID == "" {
			resp.Diagnostics.AddError(fmt.Sprintf("preset with name %q not found", name), "")
			return
		}
	} else if id := data.ID.ValueString(); id != "" {
		presetID = id
	} else {
		resp.Diagnostics.AddError("preset id or name must be set", "")
		return
	}

	getPresetResp, err := d.client.GetPreset(ctx, &cxsdk.GetPresetRequest{Id: presetID})
	if err != nil {
		resp.Diagnostics.AddError("Failed to get preset", err.Error())
		return
	}
	if getPresetResp == nil {
		resp.Diagnostics.AddError("preset not found", "preset not found")
		return
	}

	data, diags = flattenPreset(ctx, getPresetResp.Preset)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
