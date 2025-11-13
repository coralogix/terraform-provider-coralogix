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

package actions

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"

	actionss "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/actions_service"
)

var (
	_                                       resource.ResourceWithConfigure   = &ActionResource{}
	_                                       resource.ResourceWithImportState = &ActionResource{}
	actionSchemaSourceTypeToProtoSourceType                                  = map[string]actionss.V2SourceType{
		"Log":     actionss.V2SOURCETYPE_SOURCE_TYPE_LOG,
		"DataMap": actionss.V2SOURCETYPE_SOURCE_TYPE_DATA_MAP,
	}
	actionProtoSourceTypeToSchemaSourceType = utils.ReverseMap(actionSchemaSourceTypeToProtoSourceType)
	actionValidSourceTypes                  = utils.GetKeys(actionSchemaSourceTypeToProtoSourceType)
)

func NewActionResource() resource.Resource {
	return &ActionResource{}
}

type ActionResource struct {
	client *actionss.ActionsServiceAPIService
}

func (r *ActionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_action"
}

func (r *ActionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.Actions()
}

func (r *ActionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Action ID.",
			},
			"name": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Action name.",
			},
			"url": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					utils.UrlValidationFuncFramework{},
				},
				MarkdownDescription: "URL for the external tool.",
			},
			"is_private": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Determines weather the action will be shared with the entire team. Can be set to false only by admin.",
			},
			"is_hidden": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Determines weather the action will be shown at the action menu.",
			},
			"source_type": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(actionValidSourceTypes...),
				},
				MarkdownDescription: fmt.Sprintf("By selecting the data type, you can make sure that the action will be displayed only in the relevant context. Can be one of %q", actionValidSourceTypes),
			},
			"applications": schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
				MarkdownDescription: "Applies the action for specific applications.",
			},
			"subsystems": schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
				MarkdownDescription: "Applies the action for specific subsystems.",
			},
			"created_by": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "The user who created the action.",
			},
		},
		MarkdownDescription: "Coralogix action. For more info please review - https://coralogix.com/docs/coralogix-action-extension/.",
	}
}

func (r *ActionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ActionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ActionResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rq, diags := extractCreateAction(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	log.Printf("[INFO] Creating new coralogix_action: %s", utils.FormatJSON(rq))
	result, httpResponse, err := r.client.
		ActionsServiceCreateAction(ctx).
		ActionsServiceCreateActionRequest(*rq).
		Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error creating coralogix_action",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq),
		)
		return
	}
	log.Printf("[INFO] Created new coralogix_action: %s", utils.FormatJSON(result))
	action := result.GetAction()

	plan = flattenAction(&action)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func flattenAction(action *actionss.V2Action) ActionResourceModel {
	return ActionResourceModel{
		ID:           types.StringValue(action.GetId()),
		Name:         types.StringValue(action.GetName()),
		URL:          types.StringValue(action.GetUrl()),
		IsPrivate:    types.BoolValue(action.GetIsPrivate()),
		SourceType:   types.StringValue(actionProtoSourceTypeToSchemaSourceType[action.GetSourceType()]),
		Applications: utils.StringSliceToTypeStringSet(action.GetApplicationNames()),
		Subsystems:   utils.StringSliceToTypeStringSet(action.GetSubsystemNames()),
		CreatedBy:    types.StringValue(action.GetCreatedBy()),
		IsHidden:     types.BoolValue(action.GetIsHidden()),
	}
}

func (r *ActionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ActionResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()

	rq := r.client.
		ActionsServiceGetAction(ctx, id)

	log.Printf("[INFO] Reading coralogix_action: %s", utils.FormatJSON(rq))
	result, httpResponse, err := rq.
		Execute()

	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_action %v is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%v will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error reading coralogix_action", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil))
		}
		return
	}
	log.Printf("[INFO] Replaced new coralogix_action: %s", utils.FormatJSON(result))
	state = flattenAction(result.Action)

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r ActionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ActionResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rq, diags := extractUpdateAction(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	log.Printf("[INFO] Replacing new coralogix_action: %s", utils.FormatJSON(rq))
	result, httpResponse, err := r.client.ActionsServiceReplaceAction(ctx).
		ActionsServiceReplaceActionRequest(*rq).
		Execute()
	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_action %v is in state, but no longer exists in Coralogix backend", rq.Action.Id),
				fmt.Sprintf("%v will be recreated when you apply", rq.Action.Id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error updating coralogix_action", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Update", rq))
		}
		return
	}
	log.Printf("[INFO] Replaced new coralogix_action: %s", utils.FormatJSON(result))

	plan = flattenAction(result.Action)

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r ActionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ActionResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	rq := r.client.ActionsServiceDeleteAction(ctx, id)
	log.Printf("[INFO] Deleting coralogix_action: %s", utils.FormatJSON(rq))

	result, httpResponse, err := rq.Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error deleting coralogix_action",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", nil),
		)
		return
	}
	log.Printf("[INFO] Deleted coralogix_action: %s", utils.FormatJSON(result))
}

type ActionResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	URL          types.String `tfsdk:"url"`
	IsPrivate    types.Bool   `tfsdk:"is_private"`
	SourceType   types.String `tfsdk:"source_type"`
	Applications types.Set    `tfsdk:"applications"`
	Subsystems   types.Set    `tfsdk:"subsystems"`
	CreatedBy    types.String `tfsdk:"created_by"`
	IsHidden     types.Bool   `tfsdk:"is_hidden"`
}

func extractCreateAction(ctx context.Context, plan ActionResourceModel) (*actionss.ActionsServiceCreateActionRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	sourceType := actionSchemaSourceTypeToProtoSourceType[plan.SourceType.ValueString()]
	applicationNames, dgs := utils.TypeStringSliceToStringSlice(ctx, plan.Applications.Elements())
	diags = append(diags, dgs...)
	subsystemNames, dgs := utils.TypeStringSliceToStringSlice(ctx, plan.Subsystems.Elements())
	diags = append(diags, dgs...)

	return &actionss.ActionsServiceCreateActionRequest{
		Name:             plan.Name.ValueStringPointer(),
		Url:              plan.URL.ValueStringPointer(),
		IsPrivate:        plan.IsPrivate.ValueBoolPointer(),
		SourceType:       &sourceType,
		ApplicationNames: applicationNames,
		SubsystemNames:   subsystemNames,
	}, diags
}

func extractUpdateAction(ctx context.Context, plan ActionResourceModel) (*actionss.ActionsServiceReplaceActionRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	sourceType := actionSchemaSourceTypeToProtoSourceType[plan.SourceType.ValueString()]
	applicationNames, dgs := utils.TypeStringSliceToStringSlice(ctx, plan.Applications.Elements())
	diags = append(diags, dgs...)

	subsystemNames, dgs := utils.TypeStringSliceToStringSlice(ctx, plan.Subsystems.Elements())
	diags = append(diags, dgs...)

	return &actionss.ActionsServiceReplaceActionRequest{
		Action: &actionss.V2Action{
			Id:               plan.ID.ValueStringPointer(),
			Name:             plan.Name.ValueStringPointer(),
			Url:              plan.URL.ValueStringPointer(),
			IsPrivate:        plan.IsPrivate.ValueBoolPointer(),
			IsHidden:         plan.IsHidden.ValueBoolPointer(),
			SourceType:       &sourceType,
			ApplicationNames: applicationNames,
			SubsystemNames:   subsystemNames,
		},
	}, diags
}
