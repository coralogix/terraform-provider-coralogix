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

package dashboards

import (
	"context"
	"fmt"
	"net/http"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	dbfs "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_folders_service"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = &DashboardsFolderResource{}
	_ resource.ResourceWithImportState = &DashboardsFolderResource{}
)

func NewDashboardsFolderResource() resource.Resource {
	return &DashboardsFolderResource{}
}

type DashboardsFolderResource struct {
	client *dbfs.DashboardFoldersServiceAPIService
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

	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	rq := dbfs.CreateDashboardFolderRequestDataStructure{
		Folder: dashboardsFolder,
	}

	createResult, httpResponse, err := r.client.
		DashboardFoldersServiceCreateDashboardFolder(ctx).
		CreateDashboardFolderRequestDataStructure(rq).
		Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error creating coralogix_dashboard_folder",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq),
		)
		return
	}

	result, httpResponse, err := r.client.DashboardFoldersServiceGetDashboardFolder(ctx, *createResult.FolderId).
		Execute()

	plan = flattenDashboardsFolder(result.Folder)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *DashboardsFolderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DashboardsFolderResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()

	result, httpResponse, err := r.client.DashboardFoldersServiceGetDashboardFolder(ctx, id).
		Execute()
	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_dashboard_folder %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error reading coralogix_dashboard_folder", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil))
		}
		return
	}

	state = flattenDashboardsFolder(result.Folder)

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

	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	rq := dbfs.ReplaceDashboardFolderRequestDataStructure{
		Folder: dashboardsFolder,
	}

	_, httpResponse, err := r.client.
		DashboardFoldersServiceReplaceDashboardFolder(ctx).
		ReplaceDashboardFolderRequestDataStructure(rq).
		Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error replacing coralogix_dashboard_folder",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Replace", rq),
		)
		return
	}

	result, httpResponse, err := r.client.DashboardFoldersServiceGetDashboardFolder(ctx, *dashboardsFolder.Id).
		Execute()

	plan = flattenDashboardsFolder(result.Folder)

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

	if _, httpResponse, err := r.client.DashboardFoldersServiceDeleteDashboardFolder(ctx, id).Execute(); err != nil {
		resp.Diagnostics.AddError("Error deleting coralogix_dashboard_folder",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", nil))
		return
	}
}

func flattenDashboardsFolder(folder *dbfs.DashboardFolder) DashboardsFolderResourceModel {
	return DashboardsFolderResourceModel{
		ID:       types.StringValue(folder.GetId()),
		Name:     types.StringValue(folder.GetName()),
		ParentId: types.StringValue(folder.GetParentId()),
	}
}

func extractCreateDashboardsFolder(plan DashboardsFolderResourceModel) *dbfs.DashboardFolder {
	id := utils.ExpandUuid(plan.ID)
	return &dbfs.DashboardFolder{
		Id:       &id,
		Name:     utils.TypeStringToStringPointer(plan.Name),
		ParentId: utils.TypeStringToStringPointer(plan.ParentId),
	}
}
