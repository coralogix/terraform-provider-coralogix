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
	archiveRetention "terraform-provider-coralogix/coralogix/clientset/grpc/archive-retentions"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	_                          resource.ResourceWithConfigure   = &ArchiveRetentionsResource{}
	_                          resource.ResourceWithImportState = &ArchiveRetentionsResource{}
	getArchiveRetentionsURL                                     = "com.coralogix.archive.v1.RetentionsService/GetRetentions"
	updateArchiveRetentionsURL                                  = "com.coralogix.archive.v1.RetentionsService/UpdateRetentions"
)

func NewArchiveRetentionsResource() resource.Resource {
	return &ArchiveRetentionsResource{}
}

type ArchiveRetentionsResource struct {
	client *clientset.ArchiveRetentionsClient
}

func (r *ArchiveRetentionsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_archive_retentions"
}

func (r *ArchiveRetentionsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ArchiveRetentionsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"retentions": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The retention id.",
						},
						"order": schema.Int64Attribute{
							Computed:            true,
							MarkdownDescription: "The retention order. Computed by the order of the retention in the retentions list definition.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							Optional:            true,
							MarkdownDescription: "The retention name. If not set, the retention will be named by backend.",
						},
						"editable": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Is the retention editable.",
						},
					},
				},
				Required: true,
				Validators: []validator.List{
					listvalidator.SizeBetween(4, 4),
					retentionsValidator{},
				},
				MarkdownDescription: "List of 4 retentions. The first retention is the default retention and can't be renamed.",
			},
		},
		MarkdownDescription: "Coralogix archive-retention. For more info please review - https://coralogix.com/docs/archive-setup-grpc-api/.",
	}
}

type retentionsValidator struct{}

func (r retentionsValidator) Description(_ context.Context) string {
	return "Retentions validator"
}

func (r retentionsValidator) MarkdownDescription(_ context.Context) string {
	return "Retentions validator"
}

func (r retentionsValidator) ValidateList(ctx context.Context, req validator.ListRequest, resp *validator.ListResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		resp.Diagnostics.AddError("error on validating retentions", "retentions can not be null or unknown")
	}

	var retentionsObjects []types.Object
	diag := req.ConfigValue.ElementsAs(ctx, &retentionsObjects, true)
	if diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}

	if length := len(retentionsObjects); length != 4 {
		resp.Diagnostics.AddError("error on validating retentions", fmt.Sprintf("retentions list must have 4 elements but got %d", length))
	}

	var archiveRetentionResourceModel ArchiveRetentionResourceModel
	retentionsObjects[0].As(ctx, &archiveRetentionResourceModel, basetypes.ObjectAsOptions{})
	if !archiveRetentionResourceModel.Name.IsNull() {
		resp.Diagnostics.AddError("error on validating retentions", "first retention's name can't be set")
	}
}

type ArchiveRetentionsResourceModel struct {
	Retentions types.List   `tfsdk:"retentions"` //ArchiveRetentionResourceModel
	ID         types.String `tfsdk:"id"`
}

type ArchiveRetentionResourceModel struct {
	ID       types.String `tfsdk:"id"`
	Order    types.Int64  `tfsdk:"order"`
	Name     types.String `tfsdk:"name"`
	Editable types.Bool   `tfsdk:"editable"`
}

func (r *ArchiveRetentionsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *ArchiveRetentionsResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	log.Print("[INFO] Reading archive-retentions")
	getArchiveRetentionsReq := &archiveRetention.GetRetentionsRequest{}
	getArchiveRetentionsResp, err := r.client.GetRetentions(ctx, getArchiveRetentionsReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		formatRpcErrors(err, getArchiveRetentionsURL, protojson.Format(getArchiveRetentionsReq))
		return
	}
	log.Printf("[INFO] Received archive-retentions: %s", protojson.Format(getArchiveRetentionsResp))

	createArchiveRetentions, diags := extractCreateArchiveRetentions(ctx, plan, getArchiveRetentionsResp.GetRetentions())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	archiveRetentionsStr := protojson.Format(createArchiveRetentions)
	log.Printf("[INFO] Updating archive-retentions: %s", archiveRetentionsStr)
	updateResp, err := r.client.UpdateRetentions(ctx, createArchiveRetentions)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		diags.AddError(
			"Error creating archive-retentions",
			formatRpcErrors(err, updateArchiveRetentionsURL, archiveRetentionsStr),
		)
		return
	}
	log.Printf("[INFO] Submitted updated archive-retentions: %s", protojson.Format(updateResp))

	plan, diags = flattenArchiveRetentions(ctx, updateResp.GetRetentions())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func flattenArchiveRetentions(ctx context.Context, retentions []*archiveRetention.Retention) (*ArchiveRetentionsResourceModel, diag.Diagnostics) {
	if len(retentions) == 0 {
		r, _ := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: archiveRetentionAttributes()}, []types.Object{})
		return &ArchiveRetentionsResourceModel{
			Retentions: r,
			ID:         types.StringValue(""),
		}, nil
	}

	var diags diag.Diagnostics
	var retentionsObjects []types.Object
	for _, retention := range retentions {
		retentionModel := ArchiveRetentionResourceModel{
			ID:       wrapperspbStringToTypeString(retention.GetId()),
			Order:    wrapperspbInt32ToTypeInt64(retention.GetOrder()),
			Name:     wrapperspbStringToTypeString(retention.GetName()),
			Editable: wrapperspbBoolToTypeBool(retention.GetEditable()),
		}
		retentionObject, flattenDiags := types.ObjectValueFrom(ctx, archiveRetentionAttributes(), retentionModel)
		if flattenDiags.HasError() {
			diags.Append(flattenDiags...)
			continue
		}
		retentionsObjects = append(retentionsObjects, retentionObject)
	}
	if diags.HasError() {
		return nil, diags
	}

	flattenedRetentions, diags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: archiveRetentionAttributes()}, retentionsObjects)
	if diags.HasError() {
		return nil, diags
	}

	return &ArchiveRetentionsResourceModel{
		Retentions: flattenedRetentions,
		ID:         types.StringValue(""),
	}, nil
}

func archiveRetentionAttributes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":       types.StringType,
		"order":    types.Int64Type,
		"name":     types.StringType,
		"editable": types.BoolType,
	}
}

func extractCreateArchiveRetentions(ctx context.Context, plan *ArchiveRetentionsResourceModel, exitingRetentions []*archiveRetention.Retention) (*archiveRetention.UpdateRetentionsRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	var retentions []*archiveRetention.RetentionUpdateElement
	var retentionsObjects []types.Object
	plan.Retentions.ElementsAs(ctx, &retentionsObjects, true)
	for i, retentionObject := range retentionsObjects {
		var retentionModel ArchiveRetentionResourceModel
		if dg := retentionObject.As(ctx, &retentionModel, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		retentions = append(retentions, &archiveRetention.RetentionUpdateElement{
			Id:   wrapperspb.String(exitingRetentions[i].GetId().GetValue()),
			Name: typeStringToWrapperspbString(retentionModel.Name),
		})

	}
	retentions[0].Name = wrapperspb.String("Default")
	return &archiveRetention.UpdateRetentionsRequest{
		RetentionUpdateElements: retentions,
	}, diags
}

func extractUpdateArchiveRetentions(ctx context.Context, plan, state *ArchiveRetentionsResourceModel) (*archiveRetention.UpdateRetentionsRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	var planRetentionsObjects, stateRetentionsObjects []types.Object
	plan.Retentions.ElementsAs(ctx, &planRetentionsObjects, true)
	state.Retentions.ElementsAs(ctx, &stateRetentionsObjects, true)

	var retentions []*archiveRetention.RetentionUpdateElement
	for i := range planRetentionsObjects {
		var planRetentionModel, stateRetentionModel ArchiveRetentionResourceModel
		if dg := planRetentionsObjects[i].As(ctx, &planRetentionModel, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		if dg := stateRetentionsObjects[i].As(ctx, &stateRetentionModel, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		retentions = append(retentions, &archiveRetention.RetentionUpdateElement{
			Id:   typeStringToWrapperspbString(stateRetentionModel.ID),
			Name: typeStringToWrapperspbString(planRetentionModel.Name),
		})
	}
	retentions[0].Name = wrapperspb.String("Default")
	return &archiveRetention.UpdateRetentionsRequest{
		RetentionUpdateElements: retentions,
	}, diags
}

func (r *ArchiveRetentionsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *ArchiveRetentionsResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	log.Print("[INFO] Reading archive-retentions")
	getArchiveRetentionsReq := &archiveRetention.GetRetentionsRequest{}
	getArchiveRetentionsResp, err := r.client.GetRetentions(ctx, getArchiveRetentionsReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		formatRpcErrors(err, getArchiveRetentionsURL, protojson.Format(getArchiveRetentionsReq))
		return
	}
	log.Printf("[INFO] Received archive-retentions: %s", protojson.Format(getArchiveRetentionsResp))

	state, diags = flattenArchiveRetentions(ctx, getArchiveRetentionsResp.GetRetentions())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r *ArchiveRetentionsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan *ArchiveRetentionsResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state *ArchiveRetentionsResourceModel
	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	archiveRetentionsUpdateReq, diags := extractUpdateArchiveRetentions(ctx, plan, state)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	log.Printf("[INFO] Updating archive-retentions: %s", protojson.Format(archiveRetentionsUpdateReq))
	archiveRetentionsUpdateResp, err := r.client.UpdateRetentions(ctx, archiveRetentionsUpdateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating archive-retentions",
			formatRpcErrors(err, updateArchiveRetentionsURL, protojson.Format(archiveRetentionsUpdateReq)),
		)
		return
	}
	log.Printf("[INFO] Submitted updated archive-retentions: %s", protojson.Format(archiveRetentionsUpdateResp))

	// Get refreshed archive-retentions value from Coralogix
	getArchiveRetentionsReq := &archiveRetention.GetRetentionsRequest{}
	getArchiveRetentionsResp, err := r.client.GetRetentions(ctx, getArchiveRetentionsReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error reading archive-retentions",
			formatRpcErrors(err, getArchiveRetentionsURL, protojson.Format(getArchiveRetentionsReq)),
		)
		return
	}
	log.Printf("[INFO] Received archive-retentions: %s", protojson.Format(getArchiveRetentionsResp))

	plan, diags = flattenArchiveRetentions(ctx, getArchiveRetentionsResp.GetRetentions())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *ArchiveRetentionsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ArchiveRetentionsResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	log.Print("[INFO] Deleting archive-retentions")
	deleteReq := &archiveRetention.UpdateRetentionsRequest{}
	if _, err := r.client.UpdateRetentions(ctx, deleteReq); err != nil {
		resp.Diagnostics.AddError(
			"Error Deleting archive-retentions",
			formatRpcErrors(err, updateArchiveRetentionsURL, protojson.Format(deleteReq)),
		)
		return
	}
	log.Print("[INFO] archive-retentions were deleted")
}

func (r *ArchiveRetentionsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.ArchiveRetentions()
}
