// Copyright 2026 Coralogix Ltd.
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

package views

import (
	"context"
	"fmt"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	viewsfolders "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/folders_for_views_service"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = &ViewFolderResource{}
	_ resource.ResourceWithImportState = &ViewFolderResource{}
)

func NewViewFolderResource() resource.Resource {
	return &ViewFolderResource{}
}

type ViewFolderResource struct {
	client *viewsfolders.FoldersForViewsServiceAPIService
}

func (r *ViewFolderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ViewFolderResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.ViewsFolders()
}

type ViewFolderResourceModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

func (r *ViewFolderResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_view_folder"
}

func (r *ViewFolderResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 1,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Unique identifier for the view folder, assigned by Coralogix. Stays the same when the folder is renamed.",
			},
			"name": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 100),
				},
				MarkdownDescription: "Display name of the view folder. Between 1 and 100 characters - the backend silently truncates a longer name, so the provider rejects it instead. Folder names must currently be unique within the team.",
			},
		},
		MarkdownDescription: "Coralogix View Folder. Folders group saved views in the Explore screen. For more info please review - https://coralogix.com/docs/user-guides/monitoring-and-insights/explore-screen/custom-views/.",
	}
}

func (r *ViewFolderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ViewFolderResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rq := extractCreateViewFolder(plan)

	// The create response carries the complete folder, so there is no follow-up read.
	createResult, httpResponse, err := r.client.
		ViewsFoldersServiceCreateViewFolder(ctx).
		CreateViewFolderRequest(rq).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error creating coralogix_view_folder",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq),
		)
		return
	}

	plan = flattenViewFolder(createResult)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *ViewFolderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ViewFolderResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()

	result, httpResponse, err := r.client.ViewsFoldersServiceGetViewFolder(ctx, id).
		Execute()
	if err != nil {
		if cxsdkOpenapi.IsNotFound(cxsdkOpenapi.NewAPIError(httpResponse, err)) {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_view_folder %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error reading coralogix_view_folder",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
			)
		}
		return
	}

	state = flattenViewFolder(result)

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *ViewFolderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ViewFolderResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The replace body is the folder unwrapped, carrying the target id - there is no /{id} on the route.
	rq := extractReplaceViewFolder(plan)

	replaceResult, httpResponse, err := r.client.
		ViewsFoldersServiceReplaceViewFolder(ctx).
		ViewFolder1(rq).
		Execute()
	if err != nil {
		if cxsdkOpenapi.IsNotFound(cxsdkOpenapi.NewAPIError(httpResponse, err)) {
			id := plan.ID.ValueString()
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_view_folder %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error replacing coralogix_view_folder",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Replace", rq),
			)
		}
		return
	}

	plan = flattenViewFolder(replaceResult)

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *ViewFolderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ViewFolderResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()

	if _, httpResponse, err := r.client.ViewsFoldersServiceDeleteViewFolder(ctx, id).Execute(); err != nil {
		apiErr := cxsdkOpenapi.NewAPIError(httpResponse, err)
		if cxsdkOpenapi.IsNotFound(apiErr) {
			return
		}
		resp.Diagnostics.AddError("Error deleting coralogix_view_folder",
			utils.FormatOpenAPIErrors(apiErr, "Delete", nil))
		return
	}
}

// Create, Get and Replace all return *ViewFolder directly - there is no response wrapper to unwrap.
func flattenViewFolder(folder *viewsfolders.ViewFolder) ViewFolderResourceModel {
	return ViewFolderResourceModel{
		ID:   types.StringPointerValue(folder.Id),
		Name: types.StringValue(folder.Name),
	}
}

func extractCreateViewFolder(plan ViewFolderResourceModel) viewsfolders.CreateViewFolderRequest {
	return viewsfolders.CreateViewFolderRequest{
		Name: utils.TypeStringToStringPointer(plan.Name),
	}
}

func extractReplaceViewFolder(plan ViewFolderResourceModel) viewsfolders.ViewFolder1 {
	id := plan.ID.ValueString()
	return viewsfolders.ViewFolder1{
		Id:   &id,
		Name: plan.Name.ValueString(),
	}
}
