// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
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

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-coralogix/coralogix/clientset"
	"terraform-provider-coralogix/coralogix/utils"
)

var _ resource.Resource = (*ViewsFolderResource)(nil)

func NewViewsFolderResource() resource.Resource {
	return &ViewsFolderResource{}
}

type ViewsFolderResource struct {
	client *cxsdk.ViewFoldersClient
}

func (r *ViewsFolderResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_views_folder"
}

func (r *ViewsFolderResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ViewsFolderResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = ViewsFolderResourceSchema(ctx)
}

func (r *ViewsFolderResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ViewsFolderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *ViewsFolderModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create API call logic
	createRequest := &cxsdk.CreateViewFolderRequest{
		Name: utils.TypeStringToWrapperspbString(data.Name),
	}
	viewFolderStr := protojson.Format(createRequest)
	log.Printf("[INFO] Creating new views-folder: %s", viewFolderStr)
	createResponse, err := r.client.Create(ctx, createRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err)
		resp.Diagnostics.AddError("Error creating Views-Folder",
			utils.FormatRpcErrors(err, cxsdk.CreateActionRPC, viewFolderStr),
		)
		return
	}
	log.Printf("[INFO] Views-Folder created successfully: %s", protojson.Format(createResponse))

	// Save data into Terraform state
	data = flattenViewsFolder(createResponse.Folder)
	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
}

func extractViewsFolder(data *ViewsFolderModel) *cxsdk.ViewFolder {
	return &cxsdk.ViewFolder{
		Id:   utils.TypeStringToWrapperspbString(data.Id),
		Name: utils.TypeStringToWrapperspbString(data.Name),
	}
}

func flattenViewsFolder(viewsFolder *cxsdk.ViewFolder) *ViewsFolderModel {
	return &ViewsFolderModel{
		Id:   utils.WrapperspbStringToTypeString(viewsFolder.Id),
		Name: utils.WrapperspbStringToTypeString(viewsFolder.Name),
	}
}

func (r *ViewsFolderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *ViewsFolderModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := data.Id.ValueString()
	readReq := &cxsdk.GetViewFolderRequest{
		Id: wrapperspb.String(id),
	}
	log.Printf("[INFO] Reading views-folder with ID: %s", id)
	readResp, err := r.client.Get(ctx, readReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Views-Folder %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading views-folder",
				utils.FormatRpcErrors(err, cxsdk.GetViewFolderRPC, protojson.Format(readReq)),
			)
		}
		return
	}
	log.Printf("[INFO] Views-Folder read successfully: %s", protojson.Format(readResp.Folder))

	// Flatten the response into the model
	data = flattenViewsFolder(readResp.Folder)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ViewsFolderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *ViewsFolderModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	updateReq := &cxsdk.ReplaceViewFolderRequest{
		Folder: extractViewsFolder(data),
	}

	log.Printf("[INFO] Updating views-folder in state: %s", protojson.Format(updateReq))
	updateResp, err := r.client.Replace(ctx, updateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating views-folder in state",
			utils.FormatRpcErrors(err, cxsdk.ReplaceViewFolderRPC, protojson.Format(updateReq)),
		)
		return
	}
	log.Printf("[INFO] Views-Folder updated in state successfully: %s", protojson.Format(updateResp.Folder))

	// Flatten the response into the model
	data = flattenViewsFolder(updateResp.Folder)
	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ViewsFolderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *ViewsFolderModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	id := data.Id.ValueString()
	_, err := r.client.Delete(ctx, &cxsdk.DeleteViewFolderRequest{Id: wrapperspb.String(id)})
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Views-Folder %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be removed from state", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error deleting views-folder",
				utils.FormatRpcErrors(err, cxsdk.DeleteViewFolderRPC, id),
			)
		}
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
}

func ViewsFolderResourceSchema(ctx context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				Description:         "id",
				MarkdownDescription: "id",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				Description:         "Name of the views-folder",
				MarkdownDescription: "Name of the views-folder",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
		},
	}
}

type ViewsFolderModel struct {
	Id   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}
