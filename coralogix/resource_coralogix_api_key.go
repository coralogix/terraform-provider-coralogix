package coralogix

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
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
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"log"
	"reflect"
	"terraform-provider-coralogix/coralogix/clientset"
	apikeys "terraform-provider-coralogix/coralogix/clientset/grpc/apikeys"
)

var (
	getApiKeyPath    = "com.coralogixapis.aaa.apikeys.v2.ApiKeysService/GetApiKey"
	createApiKeyPath = "com.coralogixapis.aaa.apikeys.v2.ApiKeysService/CreateApiKey"
	deleteApiKeyPath = "com.coralogixapis.aaa.apikeys.v2.ApiKeysService/DeleteApiKey"
	updateApiKeyPath = "com.coralogixapis.aaa.apikeys.v2.ApiKeysService/UpdateApiKey"
)

func NewApiKeyResource() resource.Resource {
	return &ApiKeyResource{}
}

type ApiKeyResource struct {
	client *clientset.ApikeysClient
}

func (r *ApiKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"

}

func (r *ApiKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.ApiKeys()
}

func (r *ApiKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *ApiKeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "ApiKey ID.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Api Key name.",
			},
			"value": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Api Key value.",
			},
			"owner": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"team_id": schema.Int64Attribute{
						Optional: true,
						Validators: []validator.Int64{
							int64validator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("user_id"),
							),
						},
					},
					"user_id": schema.StringAttribute{
						Optional: true,
						Validators: []validator.String{
							stringvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("team_id"),
							),
						},
					},
				},
				Required:            true,
				MarkdownDescription: "Api Key Owner.It can either be a team_id or a user_id ",
			},

			"active": schema.BoolAttribute{
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Api Key Is Active.",
			},
			"hashed": schema.BoolAttribute{
				Computed:            true,
				Optional:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Api Key Is Hashed.",
			},
			"roles": schema.SetAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Api Key Roles",
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
			},
		},
		MarkdownDescription: "Coralogix Api keys.",
	}
}

type ApiKeyModel struct {
	ID     types.String `tfsdk:"id"`
	Name   types.String `tfsdk:"name"`
	Owner  *Owner       `tfsdk:"owner"`
	Active types.Bool   `tfsdk:"active"`
	Hashed types.Bool   `tfsdk:"hashed"`
	Roles  types.Set    `tfsdk:"roles"`
	Value  types.String `tfsdk:"value"`
}

type Owner struct {
	UserId types.String `tfsdk:"user_id"`
	TeamId types.Int64  `tfsdk:"team_id"`
}

func (r *ApiKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var desiredState *ApiKeyModel
	diags := req.Plan.Get(ctx, &desiredState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createApiKeyRequest, diags := makeCreateApiKeyRequest(ctx, desiredState)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	log.Printf("[INFO] Creating new ApiKey: %s", protojson.Format(createApiKeyRequest))
	createApiKeyResp, err := r.client.CreateApiKey(ctx, createApiKeyRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.Unauthenticated {
			resp.Diagnostics.AddError(
				"Error creating Api Key",
				fmt.Sprintf("permission denied for url - %s\ncheck your org-key and permissions", createApiKeyPath),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error creating Api Key",
				formatRpcErrors(err, createApiKeyPath, protojson.Format(createApiKeyRequest)),
			)
		}
		return
	}
	log.Printf("[INFO] Create api key with ID: %s", createApiKeyResp.KeyId)

	currentKeyId := createApiKeyResp.GetKeyId()
	key, diags := r.getKeyInfo(ctx, &currentKeyId, &createApiKeyResp.Value, diags)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
	diags = resp.State.Set(ctx, key)
	resp.Diagnostics.Append(diags...)
}

func (r *ApiKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var currentState *ApiKeyModel
	diags := req.State.Get(ctx, &currentState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	key, diags := r.getKeyInfo(ctx, currentState.ID.ValueStringPointer(), currentState.Value.ValueStringPointer(), diags)

	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, key)
	resp.Diagnostics.Append(diags...)
}

func (r *ApiKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var currentState, desiredState *ApiKeyModel
	diags := req.State.Get(ctx, &currentState)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}
	diags = req.Plan.Get(ctx, &desiredState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := currentState.ID.ValueString()

	var updateApiKeyRequest = apikeys.UpdateApiKeyRequest{
		KeyId: id,
	}
	if currentState.Name.ValueString() != desiredState.Name.ValueString() {
		updateApiKeyRequest.NewName = desiredState.Name.ValueStringPointer()
	}
	if !reflect.DeepEqual(currentState.Roles.Elements(), desiredState.Roles.Elements()) {
		roles, diags := typeStringSliceToStringSlice(ctx, desiredState.Roles.Elements())
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		updateApiKeyRequest.Roles = &apikeys.UpdateApiKeyRequest_Roles{
			Roles: roles,
		}
	}

	if currentState.Active.ValueBool() != desiredState.Active.ValueBool() {
		updateApiKeyRequest.IsActive = desiredState.Active.ValueBoolPointer()
	}

	if currentState.Hashed.ValueBool() != desiredState.Hashed.ValueBool() {
		resp.Diagnostics.AddError(
			"Error updating ApiKey",
			"ApiKey hashing can not be updated.",
		)
		return
	}
	log.Printf("[INFO] Updating  ApiKey %s to  %s", id, protojson.Format(&updateApiKeyRequest))

	_, err := r.client.UpdateApiKey(ctx, &updateApiKeyRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.Unauthenticated {
			resp.Diagnostics.AddError(
				"Error updating Api Key",
				fmt.Sprintf("permission denied for url - %s\ncheck your org-key and permissions", updateApiKeyPath),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error updating Api Key",
				formatRpcErrors(err, updateApiKeyPath, protojson.Format(&updateApiKeyRequest)),
			)
		}
		return
	}

	key, diags := r.getKeyInfo(ctx, &id, currentState.Value.ValueStringPointer(), diags)
	if diags.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, key)...)
}

func (r *ApiKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var currentState *ApiKeyModel
	diags := req.State.Get(ctx, &currentState)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := currentState.ID.ValueString()
	deleteApiKeyRequest, diags := makeDeleteApi(&id)
	_, err := r.client.DeleteApiKey(ctx, deleteApiKeyRequest)

	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.Unauthenticated {
			resp.Diagnostics.AddError(
				"Error getting Api Key",
				fmt.Sprintf("permission denied for url - %s\ncheck your org-key and permissions", deleteApiKeyPath),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error getting Api Key",
				formatRpcErrors(err, deleteApiKeyPath, protojson.Format(deleteApiKeyRequest)),
			)
		}
		return
	}

	log.Printf("[INFO] Dashboard %s deleted", id)
}

func (r *ApiKeyResource) getKeyInfo(ctx context.Context, id *string, keyValue *string, diags diag.Diagnostics) (*ApiKeyModel, diag.Diagnostics) {
	getApiKeyRequest, diags := makeGetApiKeyRequest(id)

	getApiKeyResponse, err := r.client.GetApiKey(ctx, getApiKeyRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.Unauthenticated {
			diags.AddError(
				"Error getting Api Key",
				fmt.Sprintf("permission denied for url - %s\ncheck your org-key and permissions", getApiKeyPath),
			)
		} else {
			diags.AddError(
				"Error getting Api Key",
				formatRpcErrors(err, getApiKeyPath, protojson.Format(getApiKeyRequest)),
			)
		}
		return nil, diags
	}
	key, diags := flattenGetApiKeyResponse(ctx, id, getApiKeyResponse, keyValue)
	if diags.HasError() {
		return nil, diags
	}
	return key, nil
}

func makeGetApiKeyRequest(apiKeyId *string) (*apikeys.GetApiKeyRequest, diag.Diagnostics) {
	return &apikeys.GetApiKeyRequest{
		KeyId: *apiKeyId,
	}, nil
}

func makeDeleteApi(apiKeyId *string) (*apikeys.DeleteApiKeyRequest, diag.Diagnostics) {
	return &apikeys.DeleteApiKeyRequest{
		KeyId: *apiKeyId,
	}, nil
}

func flattenGetApiKeyResponse(ctx context.Context, apiKeyId *string, response *apikeys.GetApiKeyResponse, keyValue *string) (*ApiKeyModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	roles, diags := types.SetValueFrom(ctx, types.StringType, response.KeyInfo.Roles)
	if diags.HasError() {
		return nil, diags
	}

	var key types.String
	if response.KeyInfo.Hashed && keyValue == nil {
		diags.AddError("Key argument is require", "Key value is required")
		return nil, diags
	} else if !response.KeyInfo.Hashed {
		key = types.StringValue(response.GetValue())
	} else {
		key = types.StringValue(*keyValue)
	}

	owner := flattenOwner(response.KeyInfo.Owner)
	return &ApiKeyModel{
		ID:     types.StringValue(*apiKeyId),
		Value:  key,
		Name:   types.StringValue(response.KeyInfo.Name),
		Active: types.BoolValue(response.KeyInfo.Active),
		Hashed: types.BoolValue(response.KeyInfo.Hashed),
		Roles:  roles,
		Owner:  &owner,
	}, nil
}

func makeCreateApiKeyRequest(ctx context.Context, apiKeyModel *ApiKeyModel) (*apikeys.CreateApiKeyRequest, diag.Diagnostics) {
	roles, diags := typeStringSliceToStringSlice(ctx, apiKeyModel.Roles.Elements())

	if diags.HasError() {
		return nil, diags
	}

	owner := extractOwner(apiKeyModel)

	return &apikeys.CreateApiKeyRequest{
		KeyInfo: &apikeys.KeyInfo{
			Name:   apiKeyModel.Name.ValueString(),
			Owner:  &owner,
			Active: apiKeyModel.Active.ValueBool(),
			Hashed: apiKeyModel.Hashed.ValueBool(),
			Roles:  roles,
		},
	}, nil
}

func extractOwner(keyModel *ApiKeyModel) apikeys.Owner {
	if keyModel.Owner.UserId.ValueString() != "" {
		return apikeys.Owner{
			Owner: &apikeys.Owner_UserId{
				UserId: keyModel.Owner.UserId.ValueString(),
			},
		}
	} else {
		return apikeys.Owner{
			Owner: &apikeys.Owner_TeamId{
				TeamId: uint32(keyModel.Owner.TeamId.ValueInt64()),
			},
		}
	}
}

func flattenOwner(owner *apikeys.Owner) Owner {
	var user types.String
	userId := owner.GetUserId()
	if userId == "" {
		user = types.StringNull()
	} else {
		user = types.StringValue(userId)
	}

	return Owner{
		UserId: user,
		TeamId: types.Int64Value(int64(owner.GetTeamId())),
	}

}
