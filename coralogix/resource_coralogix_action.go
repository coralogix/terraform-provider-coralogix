package coralogix

import (
	"context"
	"fmt"
	"log"

	"github.com/golang/protobuf/jsonpb"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-coralogix/coralogix/clientset"
	actions "terraform-provider-coralogix/coralogix/clientset/grpc/actions/v2"
)

var (
	_                                       resource.ResourceWithConfigure   = &ActionResource{}
	_                                       resource.ResourceWithImportState = &ActionResource{}
	actionSchemaSourceTypeToProtoSourceType                                  = map[string]actions.SourceType{
		"Log":     actions.SourceType_SOURCE_TYPE_LOG,
		"DataMap": actions.SourceType_SOURCE_TYPE_DATA_MAP,
	}
	actionProtoSourceTypeToSchemaSourceType = ReverseMap(actionSchemaSourceTypeToProtoSourceType)
	actionValidSourceTypes                  = GetKeys(actionSchemaSourceTypeToProtoSourceType)
)

func NewActionResource() resource.Resource {
	return &ActionResource{}
}

type ActionResource struct {
	client *clientset.ActionsClient
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
					urlValidationFuncFramework{},
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
	jsm := &jsonpb.Marshaler{}
	var plan ActionResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createActionRequest := extractCreateAction(plan)
	actionStr, _ := jsm.MarshalToString(createActionRequest)
	log.Printf("[INFO] Creating new action: %s", actionStr)
	createResp, err := r.client.CreateAction(ctx, createActionRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		resp.Diagnostics.AddError(
			"Error creating Action",
			"Could not create Action, unexpected error: "+err.Error(),
		)
		return
	}
	action := createResp.GetAction()
	actionStr, _ = jsm.MarshalToString(action)
	log.Printf("[INFO] Submitted new action: %#v", action)

	plan = flattenAction(action)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func flattenAction(action *actions.Action) ActionResourceModel {
	return ActionResourceModel{
		ID:           types.StringValue(action.GetId().GetValue()),
		Name:         types.StringValue(action.GetName().GetValue()),
		URL:          types.StringValue(action.GetUrl().GetValue()),
		IsPrivate:    types.BoolValue(action.GetIsPrivate().GetValue()),
		SourceType:   types.StringValue(actionProtoSourceTypeToSchemaSourceType[action.GetSourceType()]),
		Applications: wrappedStringSliceToTypeStringSlice(action.GetApplicationNames()),
		Subsystems:   wrappedStringSliceToTypeStringSlice(action.GetSubsystemNames()),
		CreatedBy:    types.StringValue(action.GetCreatedBy().GetValue()),
		IsHidden:     types.BoolValue(action.GetIsHidden().GetValue()),
	}
}

func (r *ActionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ActionResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed Action value from Coralogix
	id := state.ID.ValueString()
	log.Printf("[INFO] Reading Action: %s", id)
	getActionResp, err := r.client.GetAction(ctx, &actions.GetActionRequest{Id: wrapperspb.String(id)})
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if status.Code(err) == codes.NotFound {
			state.ID = types.StringNull()
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Action %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Action",
				handleRpcErrorNewFramework(err, "Action"),
			)
		}
		return
	}
	action := getActionResp.GetAction()
	log.Printf("[INFO] Received Action: %#v", action)

	state = flattenAction(action)
	//
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r ActionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan ActionResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	actionUpdateReq := extractUpdateAction(plan)
	log.Printf("[INFO] Updating Action: %#v", actionUpdateReq)
	actionUpdateResp, err := r.client.UpdateAction(ctx, actionUpdateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		resp.Diagnostics.AddError(
			"Error updating Action",
			"Could not update Action, unexpected error: "+err.Error(),
		)
		return
	}
	log.Printf("[INFO] Submitted updated Action: %#v", actionUpdateResp)

	// Get refreshed Action value from Coralogix
	id := plan.ID.ValueString()
	getActionResp, err := r.client.GetAction(ctx, &actions.GetActionRequest{Id: wrapperspb.String(id)})
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if status.Code(err) == codes.NotFound {
			plan.ID = types.StringNull()
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Action %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Action",
				handleRpcErrorNewFramework(err, "Action"),
			)
		}
		return
	}
	log.Printf("[INFO] Received Action: %#v", getActionResp)

	plan = flattenAction(getActionResp.GetAction())

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

func (r ActionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ActionResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	log.Printf("[INFO] Deleting Action %s\n", id)
	if _, err := r.client.DeleteAction(ctx, &actions.DeleteActionRequest{Id: wrapperspb.String(id)}); err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting Action %s", state.ID.ValueString()),
			handleRpcErrorNewFramework(err, "Action"),
		)
		return
	}
	log.Printf("[INFO] Action %s deleted\n", id)
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

func extractCreateAction(plan ActionResourceModel) *actions.CreateActionRequest {
	name := typeStringToWrapperspbString(plan.Name)
	url := typeStringToWrapperspbString(plan.URL)
	isPrivate := wrapperspb.Bool(plan.IsPrivate.ValueBool())
	sourceType := actionSchemaSourceTypeToProtoSourceType[plan.SourceType.ValueString()]
	applicationNames := typeStringSliceToWrappedStringSlice(plan.Applications.Elements())
	subsystemNames := typeStringSliceToWrappedStringSlice(plan.Subsystems.Elements())

	return &actions.CreateActionRequest{
		Name:             name,
		Url:              url,
		IsPrivate:        isPrivate,
		SourceType:       sourceType,
		ApplicationNames: applicationNames,
		SubsystemNames:   subsystemNames,
	}
}

func extractUpdateAction(plan ActionResourceModel) *actions.ReplaceActionRequest {
	id := wrapperspb.String(plan.ID.ValueString())
	name := typeStringToWrapperspbString(plan.Name)
	url := typeStringToWrapperspbString(plan.URL)
	isPrivate := wrapperspb.Bool(plan.IsPrivate.ValueBool())
	sourceType := actionSchemaSourceTypeToProtoSourceType[plan.SourceType.ValueString()]
	applicationNames := typeStringSliceToWrappedStringSlice(plan.Applications.Elements())
	subsystemNames := typeStringSliceToWrappedStringSlice(plan.Subsystems.Elements())
	isHidden := wrapperspb.Bool(plan.IsHidden.ValueBool())

	return &actions.ReplaceActionRequest{
		Action: &actions.Action{
			Id:               id,
			Name:             name,
			Url:              url,
			IsPrivate:        isPrivate,
			IsHidden:         isHidden,
			SourceType:       sourceType,
			ApplicationNames: applicationNames,
			SubsystemNames:   subsystemNames,
		},
	}
}
