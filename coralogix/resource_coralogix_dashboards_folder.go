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

	"terraform-provider-coralogix/coralogix/clientset"
	"terraform-provider-coralogix/coralogix/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	_ resource.ResourceWithConfigure   = &DashboardsFolderResource{}
	_ resource.ResourceWithImportState = &DashboardsFolderResource{}
)

func NewDashboardsFolderResource() resource.Resource {
	return &DashboardsFolderResource{}
}

type DashboardsFolderResource struct {
	client *cxsdk.DashboardsFoldersClient
}

func (r *DashboardsFolderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *DashboardsFolderResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.DashboardsFolders()
}

type DashboardsFolderResourceModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	ParentId types.String `tfsdk:"parent_id"`
}

func (r *DashboardsFolderResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dashboards_folder"
}

func (r *DashboardsFolderResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 1,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Unique identifier for the folder.",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Display name of the folder.",
			},
			"parent_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Parent folder id.",
			},
		},
	}
}

func (r *DashboardsFolderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DashboardsFolderResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	dashboardsFolder := extractCreateDashboardsFolder(plan)
	id := dashboardsFolder.GetId().GetValue()
	dashboardsFolderStr := protojson.Format(dashboardsFolder)
	log.Printf("[INFO] Creating new Dashboards Folder: %s", dashboardsFolderStr)
	_, err := r.client.Create(ctx, &cxsdk.CreateDashboardFolderRequest{Folder: dashboardsFolder})
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError("Error Creating Dashboards Folder",
			utils.FormatRpcErrors(err, cxsdk.DashboardFoldersCreateDashboardFolderRPC, dashboardsFolderStr),
		)
		return
	}
	listResp, err := r.client.List(ctx)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError("Error Listing Dashboards Folders",
			utils.FormatRpcErrors(err, cxsdk.DashboardFoldersListDashboardFoldersRPC, ""),
		)
		return
	}
	dashboardsFolder = nil
	for _, folder := range listResp.GetFolder() {
		if folder.GetId().GetValue() == id {
			dashboardsFolder = folder
			break
		}
	}
	if dashboardsFolder == nil {
		log.Printf("[ERROR] Could not find created folder with id: %s", id)
		resp.Diagnostics.AddError("Error Listing Dashboards Folders",
			fmt.Sprintf("Could not find created folder with id: %s", id),
		)
		return
	}

	log.Printf("[INFO] Submitted new Dashboards Folder: %s", protojson.Format(dashboardsFolder))

	plan = flattenDashboardsFolder(dashboardsFolder)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func flattenDashboardsFolder(folder *cxsdk.DashboardFolder) DashboardsFolderResourceModel {
	return DashboardsFolderResourceModel{
		ID:       utils.WrapperspbStringToTypeString(folder.GetId()),
		Name:     utils.WrapperspbStringToTypeString(folder.GetName()),
		ParentId: utils.WrapperspbStringToTypeString(folder.GetParentId()),
	}
}

func extractCreateDashboardsFolder(plan DashboardsFolderResourceModel) *cxsdk.DashboardFolder {
	return &cxsdk.DashboardFolder{
		Id:       utils.ExpandUuid(plan.ID),
		Name:     utils.TypeStringToWrapperspbString(plan.Name),
		ParentId: utils.TypeStringToWrapperspbString(plan.ParentId),
	}
}

func (r *DashboardsFolderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DashboardsFolderResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	listResp, err := r.client.List(ctx)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		utils.FormatRpcErrors(err, cxsdk.DashboardFoldersListDashboardFoldersRPC, "")
		return
	}
	var dashboardsFolder *cxsdk.DashboardFolder
	for _, folder := range listResp.GetFolder() {
		if folder.GetId().GetValue() == state.ID.ValueString() {
			dashboardsFolder = folder
			break
		}
	}
	if dashboardsFolder == nil {
		log.Printf("[ERROR] Could not find created folder with id: %s", state.ID.ValueString())
		resp.Diagnostics.AddError(fmt.Sprintf("Dashboard folder %q is in state, but no longer exists in Coralogix backend", state.ID.ValueString()),
			fmt.Sprintf("%s will be recreated when you apply", state.ID.ValueString()),
		)
		resp.State.RemoveResource(ctx)
		return
	}

	log.Printf("[INFO] Recived Dashboards Folder: %s", protojson.Format(dashboardsFolder))

	state = flattenDashboardsFolder(dashboardsFolder)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *DashboardsFolderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DashboardsFolderResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	dashboardsFolder := extractCreateDashboardsFolder(plan)
	dashboardsFolderStr := protojson.Format(dashboardsFolder)
	log.Printf("[INFO] Creating new Dashboards Folder: %s", dashboardsFolderStr)
	_, err := r.client.Replace(ctx, &cxsdk.ReplaceDashboardFolderRequest{Folder: dashboardsFolder})
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError("Error Creating Dashboards Folder",
			utils.FormatRpcErrors(err, cxsdk.DashboardFoldersReplaceDashboardFolderRPC, dashboardsFolderStr),
		)
		return
	}
	listResp, err := r.client.List(ctx)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError("Error Listing Dashboards Folders",
			utils.FormatRpcErrors(err, cxsdk.DashboardFoldersListDashboardFoldersRPC, ""),
		)
		return
	}
	dashboardsFolder = nil
	for _, folder := range listResp.GetFolder() {
		if folder.GetId().GetValue() == plan.ID.ValueString() {
			dashboardsFolder = folder
			break
		}
	}
	if dashboardsFolder == nil {
		log.Printf("[ERROR] Could not find created folder with id: %s", plan.ID.ValueString())
		resp.Diagnostics.AddError("Error Listing Dashboards Folders",
			fmt.Sprintf("Could not find created folder with id: %s", plan.ID.ValueString()),
		)
		return
	}

	log.Printf("[INFO] Submitted new Dashboards Folder: %s", protojson.Format(dashboardsFolder))

	plan = flattenDashboardsFolder(dashboardsFolder)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *DashboardsFolderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DashboardsFolderResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	log.Printf("[INFO] Deleting Dashboards Folder %s", id)
	deleteReq := &cxsdk.DeleteDashboardFolderRequest{FolderId: wrapperspb.String(id)}
	if _, err := r.client.Delete(ctx, deleteReq); err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting Dashboard %s", id),
			utils.FormatRpcErrors(err, cxsdk.DashboardFoldersDeleteDashboardFolderRPC, protojson.Format(deleteReq)),
		)
		return
	}
	log.Printf("[INFO] Dashboards Folder %s deleted", id)
}
