package coralogix

import (
	"context"
	"fmt"
	"log"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
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
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-coralogix/coralogix/clientset"
	"terraform-provider-coralogix/coralogix/utils"
)

func NewConnectorResource() resource.Resource {
	return &ConnectorResource{}
}

type ConnectorResource struct {
	client *cxsdk.NotificationsClient
}

func (r *ConnectorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_connector"
}

func (r *ConnectorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.GetNotifications()
}

func (r *ConnectorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Connector ID.",
			},
			"name": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Connector name.",
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
				MarkdownDescription: "Determines weather the Connector will be shared with the entire team. Can be set to false only by admin.",
			},
			"is_hidden": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Determines weather the Connector will be shown at the Connector menu.",
			},
			"source_type": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(ConnectorValidSourceTypes...),
				},
				MarkdownDescription: fmt.Sprintf("By selecting the data type, you can make sure that the Connector will be displayed only in the relevant context. Can be one of %q", ConnectorValidSourceTypes),
			},
			"applications": schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
				MarkdownDescription: "Applies the Connector for specific applications.",
			},
			"subsystems": schema.SetAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
				MarkdownDescription: "Applies the Connector for specific subsystems.",
			},
			"created_by": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "The user who created the Connector.",
			},
		},
		MarkdownDescription: "Coralogix Connector. For more info please review - https://coralogix.com/docs/coralogix-Connector-extension/.",
	}
}

func (r *ConnectorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ConnectorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ConnectorResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createConnectorRequest, diags := extractCreateConnector(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	ConnectorStr := protojson.Format(createConnectorRequest)
	log.Printf("[INFO] Creating new Connector: %s", ConnectorStr)
	createResp, err := r.client.Create(ctx, createConnectorRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err)
		resp.Diagnostics.AddError("Error creating Connector",
			utils.FormatRpcErrors(err, cxsdk.CreateConnectorRPC, ConnectorStr),
		)
		return
	}
	Connector := createResp.GetConnector()
	log.Printf("[INFO] Submitted new Connector: %s", protojson.Format(Connector))

	plan = flattenConnector(Connector)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func flattenConnector(Connector *cxsdk.Connector) ConnectorResourceModel {
	return ConnectorResourceModel{
		ID:           types.StringValue(Connector.GetId().GetValue()),
		Name:         types.StringValue(Connector.GetName().GetValue()),
		URL:          types.StringValue(Connector.GetUrl().GetValue()),
		IsPrivate:    types.BoolValue(Connector.GetIsPrivate().GetValue()),
		SourceType:   types.StringValue(ConnectorProtoSourceTypeToSchemaSourceType[Connector.GetSourceType()]),
		Applications: utils.WrappedStringSliceToTypeStringSet(Connector.GetApplicationNames()),
		Subsystems:   utils.WrappedStringSliceToTypeStringSet(Connector.GetSubsystemNames()),
		CreatedBy:    types.StringValue(Connector.GetCreatedBy().GetValue()),
		IsHidden:     types.BoolValue(Connector.GetIsHidden().GetValue()),
	}
}

func (r *ConnectorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ConnectorResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed Connector value from Coralogix
	id := state.ID.ValueString()
	log.Printf("[INFO] Reading Connector: %s", id)
	getConnectorReq := &cxsdk.GetConnectorRequest{Id: wrapperspb.String(id)}
	getConnectorResp, err := r.client.Get(ctx, getConnectorReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Connector %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Connector",
				utils.FormatRpcErrors(err, cxsdk.GetConnectorRPC, protojson.Format(getConnectorReq)),
			)
		}
		return
	}
	Connector := getConnectorResp.GetConnector()
	log.Printf("[INFO] Received Connector: %s", protojson.Format(Connector))

	state = flattenConnector(Connector)
	//
	diags = resp.State.Set(ctx, &state)
	resp.Diagnostics.Append(diags...)
}

func (r ConnectorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan ConnectorResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	ConnectorUpdateReq, diags := extractUpdateConnector(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	log.Printf("[INFO] Updating Connector: %s", protojson.Format(ConnectorUpdateReq))
	ConnectorUpdateResp, err := r.client.Replace(ctx, ConnectorUpdateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating Connector",
			utils.FormatRpcErrors(err, cxsdk.ReplaceConnectorRPC, protojson.Format(ConnectorUpdateReq)),
		)
		return
	}
	log.Printf("[INFO] Submitted updated Connector: %s", protojson.Format(ConnectorUpdateResp))

	// Get refreshed Connector value from Coralogix
	id := plan.ID.ValueString()
	getConnectorReq := &cxsdk.GetConnectorRequest{Id: wrapperspb.String(id)}
	getConnectorResp, err := r.client.Get(ctx, getConnectorReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Connector %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Connector",
				utils.FormatRpcErrors(err, cxsdk.GetConnectorRPC, protojson.Format(getConnectorReq)),
			)
		}
		return
	}
	log.Printf("[INFO] Received Connector: %s", protojson.Format(getConnectorResp))

	plan = flattenConnector(getConnectorResp.GetConnector())

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r ConnectorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ConnectorResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	log.Printf("[INFO] Deleting Connector %s", id)
	deleteReq := &cxsdk.DeleteConnectorRequest{Id: wrapperspb.String(id)}
	if _, err := r.client.Delete(ctx, deleteReq); err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting Connector %s", id),
			utils.FormatRpcErrors(err, cxsdk.DeleteConnectorRPC, protojson.Format(deleteReq)),
		)
		return
	}
	log.Printf("[INFO] Connector %s deleted", id)
}

type ConnectorResourceModel struct {
	ID            types.String `tfsdk:"id"`
	UserFacingId  types.String `tfsdk:"user_facing_id"`
	TeamId        types.Int64  `tfsdk:"team_id"`
	Name          types.String `tfsdk:"name"`
	Description   types.String `tfsdk:"description"`
	ConnectorType types.Object `tfsdk:"connector_type"` // ConnectorTypeModel
}

type ConnectorTypeModel struct {
	GenericHttps types.Object `tfsdk:"generic_https"` // GenericHttpsConnectorModel
	Slack        types.Object `tfsdk:"slack"`         // SlackConnectorModel
}

type GenericHttpsConnectorModel struct {
}

type SlackConnectorModel struct {
	CommonFields types.Object `tfsdk:"common_fields"` // ConnectorSlackCommonFields

	Overrides types.List `tfsdk:"overrides"` // ConnectorSlackOverride
}

type ConnectorSlackCommonFields struct {
	RawConfig *ConnectorSlackConfig `json:"rawConfig"`

	StructuredConfig *ConnectorSlackConfig `json:"structuredConfig"`
}

type ConnectorSlackConfig struct {
	Integration *SlackIntegrationRef `json:"integration"`

	FallbackChannel string `json:"fallbackChannel"`

	// +optional
	Channel *string `json:"channel,omitempty"`
}

type SlackIntegrationRef struct {
	BackendRef *SlackIntegrationBackendRef `json:"backendRef"`
}

type SlackIntegrationBackendRef struct {
	Id string `json:"id"`
}

type ConnectorSlackOverride struct {
	RawConfig *ConnectorSlackConfigOverride `json:"rawConfig"`

	StructuredConfig *ConnectorSlackConfigOverride `json:"structuredConfig"`

	EntityType string `json:"entityType"`
}

type ConnectorSlackConfigOverride struct {
	Channel string `json:"channel"`
}

func extractCreateConnector(ctx context.Context, plan ConnectorResourceModel) (*cxsdk.CreateConnectorRequest, diag.Diagnostics) {
	connectorConfigs, connectorOverrides := extractConnectorConfigs(ctx, plan.ConnectorType)
	return &cxsdk.CreateConnectorRequest{
		Connector: &cxsdk.Connector{
			UserFacingId:     utils.TypeStringToStringPointer(plan.UserFacingId),
			Name:             plan.Name.ValueString(),
			Description:      plan.Description.ValueString(),
			ConnectorConfigs: connectorConfigs,
			ConfigOverrides:  connectorOverrides,
		},
	}, nil
}

func extractConnectorConfigs(ctx context.Context, connectorType types.Object) ([]*cxsdk.ConnectorConfig, []*cxsdk.EntityTypeConfigOverrides) {
	var connectorConfigs []*cxsdk.ConnectorConfig
	var connectorOverrides []*cxsdk.EntityTypeConfigOverrides

	connectorType.ToTerraformValue(ctx, &connectorType)

}

func extractUpdateConnector(ctx context.Context, plan ConnectorResourceModel) (*cxsdk.ReplaceConnectorRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	id := wrapperspb.String(plan.ID.ValueString())
	name := utils.TypeStringToWrapperspbString(plan.Name)
	url := utils.TypeStringToWrapperspbString(plan.URL)
	isPrivate := wrapperspb.Bool(plan.IsPrivate.ValueBool())
	sourceType := ConnectorSchemaSourceTypeToProtoSourceType[plan.SourceType.ValueString()]
	applicationNames, dgs := utils.TypeStringSliceToWrappedStringSlice(ctx, plan.Applications.Elements())
	if dgs.HasError() {
		diags = append(diags, dgs...)
	}
	subsystemNames, dgs := utils.TypeStringSliceToWrappedStringSlice(ctx, plan.Subsystems.Elements())
	if dgs.HasError() {
		diags = append(diags, dgs...)
	}
	isHidden := wrapperspb.Bool(plan.IsHidden.ValueBool())

	return &cxsdk.ReplaceConnectorRequest{
		Connector: &cxsdk.Connector{
			Id:               id,
			Name:             name,
			Url:              url,
			IsPrivate:        isPrivate,
			IsHidden:         isHidden,
			SourceType:       sourceType,
			ApplicationNames: applicationNames,
			SubsystemNames:   subsystemNames,
		},
	}, diags
}
