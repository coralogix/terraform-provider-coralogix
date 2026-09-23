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
	"net/http"
	"regexp"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
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

type ActionResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	URL          types.String `tfsdk:"url"`
	Description  types.String `tfsdk:"description"`
	DpxlFilter   types.String `tfsdk:"dpxl_filter"`
	URLFields    types.List   `tfsdk:"url_fields"`
	IsPrivate    types.Bool   `tfsdk:"is_private"`
	SourceType   types.String `tfsdk:"source_type"`
	Applications types.Set    `tfsdk:"applications"`
	Subsystems   types.Set    `tfsdk:"subsystems"`
	CreatedBy    types.String `tfsdk:"created_by"`
	IsHidden     types.Bool   `tfsdk:"is_hidden"`
}

type ActionURLFieldModel struct {
	Name     types.String `tfsdk:"name"`
	Required types.Bool   `tfsdk:"required"`
}

// actionURLFieldAttributeTypes must stay in sync with the `url_fields` nested
// object in Schema; the framework rejects an object whose schema and attribute
// types disagree.
func actionURLFieldAttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"name":     types.StringType,
		"required": types.BoolType,
	}
}

func actionURLFieldElementType() attr.Type {
	return types.ObjectType{AttrTypes: actionURLFieldAttributeTypes()}
}

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
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Free-text description of the action. Removing this line clears the description.",
			},
			"dpxl_filter": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^(<v1>|$)`),
						"must be empty or start with a version prefix, e.g. `<v1> $d.severity == 'ERROR'`",
					),
				},
				MarkdownDescription: "DPXL expression that scopes when the action is offered. " +
					"The expression must include a version prefix, e.g. `<v1> $d.severity == 'ERROR'`. " +
					"Removing this line clears the filter.",
			},
			"url_fields": schema.ListNestedAttribute{
				Optional: true,
				Validators: []validator.List{
					listvalidator.SizeAtMost(1000),
				},
				MarkdownDescription: "Declarations for the `{{placeholder}}` slots in `url`, in the configured order.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.LengthAtLeast(1),
							},
							MarkdownDescription: "URL field name. Must be unique within `url_fields`.",
						},
						"required": schema.BoolAttribute{
							Required:            true,
							MarkdownDescription: "Whether the field must be present for the action to be invokable.",
						},
					},
				},
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
				DeprecationMessage:  "`is_hidden` is a per-user UI preference, not a property of the action. It will be removed in a future version.",
				MarkdownDescription: "Deprecated: `is_hidden` is a per-user UI preference, not a property of the action. It will be removed in a future version.",
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
				Computed:    true,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
				MarkdownDescription: "Applies the action for specific applications.",
			},
			"subsystems": schema.SetAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
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
	action := result.GetAction()

	state, diags := flattenAction(ctx, &plan, &action)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// flattenAction converts an API action into the Terraform model. A non-nil plan
// makes the conversion config-aware, which is what the resource needs to
// converge: the API merges omitted optional scalars and canonicalizes
// `dpxlFilter`, so the configured value wins whenever it is equivalent to the
// stored one. A nil plan flattens the API values verbatim, for the read-only
// data source.
func flattenAction(ctx context.Context, plan *ActionResourceModel, action *actionss.V2Action) (ActionResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	var description, dpxlFilter types.String
	var urlFields types.List
	if plan == nil {
		description = flattenAPIString(action.Description)
		dpxlFilter = flattenAPIString(action.DpxlFilter)
		var dgs diag.Diagnostics
		urlFields, dgs = urlFieldsToList(ctx, action.GetUrlFields())
		diags.Append(dgs...)
	} else {
		description = flattenConfiguredString(action.Description, plan.Description)
		dpxlFilter = flattenConfiguredString(action.DpxlFilter, plan.DpxlFilter)
		var dgs diag.Diagnostics
		urlFields, dgs = flattenURLFields(ctx, action.GetUrlFields(), plan.URLFields)
		diags.Append(dgs...)
	}

	return ActionResourceModel{
		ID:           types.StringValue(action.GetId()),
		Name:         types.StringValue(action.GetName()),
		URL:          types.StringValue(action.GetUrl()),
		Description:  description,
		DpxlFilter:   dpxlFilter,
		URLFields:    urlFields,
		IsPrivate:    types.BoolValue(action.GetIsPrivate()),
		SourceType:   types.StringValue(actionProtoSourceTypeToSchemaSourceType[action.GetSourceType()]),
		Applications: utils.StringSliceToTypeStringSet(action.GetApplicationNames()),
		Subsystems:   utils.StringSliceToTypeStringSet(action.GetSubsystemNames()),
		CreatedBy:    types.StringValue(action.GetCreatedBy()),
		IsHidden:     types.BoolValue(action.GetIsHidden()),
	}, diags
}

// flattenAPIString echoes the backend value, keeping an explicit "" as "".
func flattenAPIString(api *string) types.String {
	if api == nil {
		return types.StringNull()
	}
	return types.StringValue(*api)
}

// flattenConfiguredString maps an absent or empty API value to null, preserving
// only an explicit configured empty string so `description = ""` round-trips.
// The update path always sends "" to clear the field, and null != "" in
// Terraform, so without this an unset attribute would diff forever.
func flattenConfiguredString(api *string, plan types.String) types.String {
	if api != nil && *api != "" {
		return types.StringValue(*api)
	}
	if !plan.IsNull() && !plan.IsUnknown() && plan.ValueString() == "" {
		return plan
	}
	return types.StringNull()
}

// flattenURLFields reconciles the API's concrete [] for an omitted url_fields
// with a null configuration, while keeping an explicit url_fields = [] known.
// Order is preserved: the API returns items in submitted order.
func flattenURLFields(ctx context.Context, fields []actionss.UrlField, plan types.List) (types.List, diag.Diagnostics) {
	if len(fields) == 0 && plan.IsNull() {
		return types.ListNull(actionURLFieldElementType()), nil
	}
	return urlFieldsToList(ctx, fields)
}

func urlFieldsToList(ctx context.Context, fields []actionss.UrlField) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics

	elements := make([]attr.Value, 0, len(fields))
	for _, field := range fields {
		element, dgs := types.ObjectValueFrom(ctx, actionURLFieldAttributeTypes(), ActionURLFieldModel{
			Name:     types.StringValue(field.GetName()),
			Required: types.BoolValue(field.GetRequired()),
		})
		diags.Append(dgs...)
		if dgs.HasError() {
			return types.ListNull(actionURLFieldElementType()), diags
		}
		elements = append(elements, element)
	}

	list, dgs := types.ListValue(actionURLFieldElementType(), elements)
	diags.Append(dgs...)
	return list, diags
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

	result, httpResponse, err := rq.
		Execute()

	if err != nil {
		if httpResponse != nil && httpResponse.StatusCode == http.StatusNotFound {
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
	newState, diags := flattenAction(ctx, &state, result.Action)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
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
	result, httpResponse, err := r.client.ActionsServiceReplaceAction(ctx).
		ActionsServiceReplaceActionRequest(*rq).
		Execute()
	if err != nil {
		if httpResponse != nil && httpResponse.StatusCode == http.StatusNotFound {
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

	state, diags := flattenAction(ctx, &plan, result.Action)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
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

	_, httpResponse, err := rq.Execute()

	if err != nil {
		if httpResponse != nil && httpResponse.StatusCode == http.StatusNotFound {
			return
		}
		resp.Diagnostics.AddError("Error deleting coralogix_action",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", nil),
		)
		return
	}
}

func extractCreateAction(ctx context.Context, plan ActionResourceModel) (*actionss.ActionsServiceCreateActionRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	sourceType := actionSchemaSourceTypeToProtoSourceType[plan.SourceType.ValueString()]
	applicationNames, dgs := utils.TypeStringElementsToStringSlice(ctx, plan.Applications.Elements())
	diags = append(diags, dgs...)
	subsystemNames, dgs := utils.TypeStringElementsToStringSlice(ctx, plan.Subsystems.Elements())
	diags = append(diags, dgs...)

	urlFields, dgs := expandURLFields(ctx, plan.URLFields)
	diags = append(diags, dgs...)

	rq := &actionss.ActionsServiceCreateActionRequest{
		Name:             plan.Name.ValueStringPointer(),
		Url:              plan.URL.ValueStringPointer(),
		IsPrivate:        plan.IsPrivate.ValueBoolPointer(),
		SourceType:       &sourceType,
		ApplicationNames: applicationNames,
		SubsystemNames:   subsystemNames,
		UrlFields:        urlFields,
	}
	// Omitted on create stays absent; the API injects nothing.
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		rq.Description = plan.Description.ValueStringPointer()
	}
	if !plan.DpxlFilter.IsNull() && !plan.DpxlFilter.IsUnknown() {
		rq.DpxlFilter = plan.DpxlFilter.ValueStringPointer()
	}

	return rq, diags
}

func extractUpdateAction(ctx context.Context, plan ActionResourceModel) (*actionss.ActionsServiceReplaceActionRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	sourceType := actionSchemaSourceTypeToProtoSourceType[plan.SourceType.ValueString()]
	applicationNames, dgs := utils.TypeStringElementsToStringSlice(ctx, plan.Applications.Elements())
	diags = append(diags, dgs...)

	subsystemNames, dgs := utils.TypeStringElementsToStringSlice(ctx, plan.Subsystems.Elements())
	diags = append(diags, dgs...)

	urlFields, dgs := expandURLFields(ctx, plan.URLFields)
	diags = append(diags, dgs...)
	// The replace endpoint merges omitted optional scalars instead of clearing
	// them, so description and dpxl_filter are always sent — "" when unset —
	// and url_fields goes out as an explicit [] rather than being omitted.
	if urlFields == nil {
		urlFields = []actionss.UrlField{}
	}
	description := ""
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		description = plan.Description.ValueString()
	}
	dpxlFilter := ""
	if !plan.DpxlFilter.IsNull() && !plan.DpxlFilter.IsUnknown() {
		dpxlFilter = plan.DpxlFilter.ValueString()
	}

	return &actionss.ActionsServiceReplaceActionRequest{
		Action: &actionss.V2Action{
			Id:   plan.ID.ValueStringPointer(),
			Name: plan.Name.ValueStringPointer(),
			Url:  plan.URL.ValueStringPointer(),
			// isHidden is required on replace; omitting it is a 400.
			IsPrivate:        plan.IsPrivate.ValueBoolPointer(),
			IsHidden:         plan.IsHidden.ValueBoolPointer(),
			Description:      &description,
			DpxlFilter:       &dpxlFilter,
			UrlFields:        urlFields,
			SourceType:       &sourceType,
			ApplicationNames: applicationNames,
			SubsystemNames:   subsystemNames,
		},
	}, diags
}

func expandURLFields(ctx context.Context, list types.List) ([]actionss.UrlField, diag.Diagnostics) {
	if list.IsNull() || list.IsUnknown() {
		return nil, nil
	}

	var models []ActionURLFieldModel
	diags := list.ElementsAs(ctx, &models, false)
	if diags.HasError() {
		return nil, diags
	}

	fields := make([]actionss.UrlField, 0, len(models))
	for _, model := range models {
		fields = append(fields, actionss.UrlField{
			Name:     model.Name.ValueString(),
			Required: model.Required.ValueBool(),
		})
	}
	return fields, diags
}
