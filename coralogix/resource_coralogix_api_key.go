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
	"reflect"
	"strconv"
	"strings"

	"terraform-provider-coralogix/coralogix/clientset"
	apikeys "terraform-provider-coralogix/coralogix/clientset/grpc/apikeys"

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
	"golang.org/x/exp/slices"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	getApiKeyPath    = apikeys.ApiKeysService_GetApiKey_FullMethodName
	createApiKeyPath = apikeys.ApiKeysService_CreateApiKey_FullMethodName
	deleteApiKeyPath = apikeys.ApiKeysService_DeleteApiKey_FullMethodName
	updateApiKeyPath = apikeys.ApiKeysService_UpdateApiKey_FullMethodName
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

func resourceSchemaV1() schema.Schema {
	return schema.Schema{
		Version: 1,
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
					"team_id": schema.StringAttribute{
						Optional: true,
						Validators: []validator.String{
							stringvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("user_id"),
								path.MatchRelative().AtParent().AtName("organisation_id"),
							),
						},
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"user_id": schema.StringAttribute{
						Optional: true,
						Validators: []validator.String{
							stringvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("team_id"),
								path.MatchRelative().AtParent().AtName("organisation_id"),
							),
						},
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"organisation_id": schema.StringAttribute{
						Optional: true,
						Validators: []validator.String{
							stringvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("team_id"),
								path.MatchRelative().AtParent().AtName("user_id")),
						},
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
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
				MarkdownDescription: "Api Key Is Hashed.",
			},
			"presets": schema.SetAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Api Key Presets",
			},
			"permissions": schema.SetAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Api Key Permissions",
			},
		},
		MarkdownDescription: "Coralogix Api keys.",
	}
}

func resourceSchemaV0() schema.Schema {
	return schema.Schema{

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
					"team_id": schema.StringAttribute{
						Optional: true,
						Validators: []validator.String{
							stringvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("user_id"),
							),
						},
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
						},
					},
					"user_id": schema.StringAttribute{
						Optional: true,
						Validators: []validator.String{
							stringvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("team_id"),
							),
						},
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.RequiresReplace(),
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

func (r *ApiKeyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchemaV1()
}

type ApiKeyModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Owner       *Owner       `tfsdk:"owner"`
	Active      types.Bool   `tfsdk:"active"`
	Hashed      types.Bool   `tfsdk:"hashed"`
	Permissions types.Set    `tfsdk:"permissions"`
	Presets     types.Set    `tfsdk:"presets"`
	Value       types.String `tfsdk:"value"`
}

type Owner struct {
	UserId         types.String `tfsdk:"user_id"`
	TeamId         types.String `tfsdk:"team_id"`
	OrganisationId types.String `tfsdk:"organisation_id"`
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
		resp.Diagnostics.AddError(
			"Error creating Api Key",
			formatRpcErrors(err, createApiKeyPath, protojson.Format(createApiKeyRequest)),
		)
		return
	}
	log.Printf("[INFO] Create api key with ID: %s", createApiKeyResp.KeyId)

	currentKeyId := createApiKeyResp.GetKeyId()
	key, diags := r.getKeyInfo(ctx, &currentKeyId, &createApiKeyResp.Value)
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

	key, diags := r.getKeyInfo(ctx, currentState.ID.ValueStringPointer(), currentState.Value.ValueStringPointer())
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

	if !reflect.DeepEqual(currentState.Permissions.Elements(), desiredState.Permissions.Elements()) {
		permissions, diags := typeStringSliceToStringSlice(ctx, desiredState.Permissions.Elements())
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		updateApiKeyRequest.Permissions = &apikeys.UpdateApiKeyRequest_Permissions{
			Permissions: permissions,
		}
	}

	if !reflect.DeepEqual(currentState.Presets.Elements(), desiredState.Presets.Elements()) {
		presets, diags := typeStringSliceToStringSlice(ctx, desiredState.Presets.Elements())
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		updateApiKeyRequest.Presets = &apikeys.UpdateApiKeyRequest_Presets{
			Presets: presets,
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
		resp.Diagnostics.AddError(
			"Error updating Api Key",
			formatRpcErrors(err, updateApiKeyPath, protojson.Format(&updateApiKeyRequest)),
		)
		return
	}

	key, diags := r.getKeyInfo(ctx, &id, currentState.Value.ValueStringPointer())
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
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	_, err := r.client.DeleteApiKey(ctx, deleteApiKeyRequest)

	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error getting Api Key",
			formatRpcErrors(err, deleteApiKeyPath, protojson.Format(deleteApiKeyRequest)),
		)
		return
	}

	log.Printf("[INFO] Api Key %s deleted", id)
}

func (r *ApiKeyResource) getKeyInfo(ctx context.Context, id *string, keyValue *string) (*ApiKeyModel, diag.Diagnostics) {
	getApiKeyRequest, diags := makeGetApiKeyRequest(id)
	if diags.HasError() {
		return nil, diags
	}
	log.Printf("[INFO] Get api key with ID: %s", getApiKeyRequest)
	getApiKeyResponse, err := r.client.GetApiKey(ctx, getApiKeyRequest)

	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.NotFound {
			diags.AddError(
				"Error getting Api Key",
				fmt.Sprintf("Api Key with id %s not found", *id),
			)
		} else {
			diags.AddError(
				"Error getting Api Key",
				formatRpcErrors(err, getApiKeyPath, protojson.Format(getApiKeyRequest)),
			)
		}
		return nil, diags
	}
	log.Printf("[INFO] Got api key info: %s", protojson.Format(getApiKeyResponse))
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

	permissions := stringSliceToTypeStringSet(response.KeyInfo.KeyPermissions.Permissions)
	if permissions.IsNull() {
		permissions = types.SetValueMust(types.StringType, []attr.Value{})
	}
	presetNames := make([]attr.Value, len(response.KeyInfo.KeyPermissions.Presets))
	for i, p := range response.KeyInfo.KeyPermissions.Presets {
		presetNames[i] = types.StringValue(p.Name)
	}

	presets, diags := types.SetValueFrom(ctx, types.StringType, presetNames)
	if diags.HasError() {
		return nil, diags
	}

	var key types.String
	if response.KeyInfo.Hashed && keyValue == nil {
		diags.AddError("Key argument is require", "Key value is required")
		return nil, diags
	} else if !response.KeyInfo.Hashed {
		key = types.StringValue(response.KeyInfo.GetValue())
	} else {
		key = types.StringValue(*keyValue)
	}

	owner := flattenOwner(response.KeyInfo.Owner)
	return &ApiKeyModel{
		ID:          types.StringValue(*apiKeyId),
		Value:       key,
		Name:        types.StringValue(response.KeyInfo.Name),
		Active:      types.BoolValue(response.KeyInfo.Active),
		Hashed:      types.BoolValue(response.KeyInfo.Hashed),
		Permissions: permissions,
		Presets:     presets,
		Owner:       &owner,
	}, nil
}

func makeCreateApiKeyRequest(ctx context.Context, apiKeyModel *ApiKeyModel) (*apikeys.CreateApiKeyRequest, diag.Diagnostics) {
	permissions, diags := typeStringSliceToStringSlice(ctx, apiKeyModel.Permissions.Elements())
	if diags.HasError() {
		return nil, diags
	}

	presets, diags := typeStringSliceToStringSlice(ctx, apiKeyModel.Presets.Elements())
	if diags.HasError() {
		return nil, diags
	}

	owner, diags := extractOwner(apiKeyModel)
	if diags.HasError() {
		return nil, diags
	}

	return &apikeys.CreateApiKeyRequest{
		Name:  apiKeyModel.Name.ValueString(),
		Owner: &owner,
		KeyPermissions: &apikeys.CreateApiKeyRequest_KeyPermissions{
			Presets:     presets,
			Permissions: permissions,
		},
		Hashed: false, // this has to be false or the GetApiKey will fail (encrypted keys are not readable)
	}, nil
}

func extractOwner(keyModel *ApiKeyModel) (apikeys.Owner, diag.Diagnostics) {
	var diags diag.Diagnostics
	if keyModel.Owner.UserId.ValueString() != "" {
		return apikeys.Owner{
			Owner: &apikeys.Owner_UserId{
				UserId: keyModel.Owner.UserId.ValueString(),
			},
		}, diags
	} else {
		if keyModel.Owner.OrganisationId.ValueString() != "" {
			return apikeys.Owner{
				Owner: &apikeys.Owner_OrganisationId{
					OrganisationId: keyModel.Owner.OrganisationId.ValueString(),
				},
			}, diags
		} else {
			teamId, err := strconv.Atoi(keyModel.Owner.TeamId.ValueString())
			if err != nil {
				diags.AddError("Invalid team id", "Team id must be a int")
			}
			return apikeys.Owner{
				Owner: &apikeys.Owner_TeamId{
					TeamId: uint32(teamId),
				},
			}, diags
		}
	}
}

func flattenOwner(owner *apikeys.Owner) Owner {
	switch owner.Owner.(type) {
	case *apikeys.Owner_TeamId:
		return Owner{
			TeamId: types.StringValue(strconv.Itoa(int(owner.GetTeamId()))),
		}
	case *apikeys.Owner_UserId:
		return Owner{
			UserId: types.StringValue(owner.GetUserId()),
		}
	case *apikeys.Owner_OrganisationId:
		return Owner{
			OrganisationId: types.StringValue(owner.GetOrganisationId()),
		}
	default:
		return Owner{}
	}
}

func (r *ApiKeyResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	schemaV0 := resourceSchemaV0()

	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema: &schemaV0,

			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				type ApiKeyModelV0 struct {
					ID     types.String `tfsdk:"id"`
					Name   types.String `tfsdk:"name"`
					Owner  *Owner       `tfsdk:"owner"`
					Active types.Bool   `tfsdk:"active"`
					Hashed types.Bool   `tfsdk:"hashed"`
					Value  types.String `tfsdk:"value"`
					Roles  types.Set    `tfsdk:"roles"`
				}

				var dataV0 ApiKeyModelV0

				resp.Diagnostics.Append(req.State.Get(ctx, &dataV0)...)
				if resp.Diagnostics.HasError() {
					return
				}
				permissions, diags := mapRolesToPermissions(dataV0.Roles)

				if diags.HasError() {
					resp.Diagnostics.Append(diags...)
					return
				}

				dataV1 := ApiKeyModel{
					ID:          dataV0.ID,
					Name:        dataV0.Name,
					Owner:       dataV0.Owner,
					Active:      dataV0.Active,
					Hashed:      dataV0.Hashed,
					Value:       dataV0.Value,
					Permissions: permissions,
					Presets:     types.SetNull(types.StringType),
				}

				diags = resp.State.Set(ctx, dataV1)
				resp.Diagnostics.Append(diags...)
			},
		},
	}
}

func mapRolesToPermissions(roles types.Set) (types.Set, diag.Diagnostics) {
	permissions := []string{}
	for _, role := range roles.Elements() {
		mappedPermissions, diags := mapRoleToPermission(role.(types.String))
		if diags.HasError() {
			return types.SetNull(types.StringType), diags
		}
		for _, m := range mappedPermissions {
			if !slices.Contains(permissions, m) {
				permissions = append(permissions, m)
			}
		}
	}
	return stringSliceToTypeStringSet(permissions), diag.Diagnostics{}
}

func mapRoleToPermission(role types.String) ([]string, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	switch r := strings.ToLower(role.ValueString()); r {
	case "rum ingress":
		return []string{"rum-ingress:SendData"}, diags
	case "send data":
		return []string{
			"cloud-metadata-ingress:SendData", "logs.data-ingress:SendData", "metrics.data-ingress:SendData", "spans.data-ingress:SendData"}, diags
	case "coralogix cli":
		return []string{"data-usage:Read", "org-quota:Manage", "org-quota:Read", "org-teams:Manage", "org-teams:ReadConfig", "team-members:Manage", "team-members:ReadConfig", "team-scim:Manage", "team-scim:ReadConfig", "team-sso:Manage", "team-sso:ReadConfig", "team-quota:Manage", "team-quota:Read"}, diags
	case "scim":
		return []string{"team-groups:Manage", "team-groups:ReadConfig", "team-members:Manage", "team-members:ReadConfig", "team-roles:ReadConfig"}, diags
	case "role management":
		return []string{"team-roles:Manage", "team-roles:ReadConfig"}, diags
	case "trigger webhook":
		return []string{"contextual-data:SendData"}, diags
	case "legacy api key":
		return []string{"alerts:ReadConfig", "alerts:UpdateConfig", "cloud-metadata-enrichment:ReadConfig", "cloud-metadata-enrichment:UpdateConfig", "data-usage:Read", "geo-enrichment:ReadConfig", "geo-enrichment:UpdateConfig", "grafana:Read", "grafana:Update", "logs.data-setup#low:ReadConfig", "logs.data-setup#low:UpdateConfig", "logs.events2metrics:ReadConfig", "logs.events2metrics:UpdateConfig", "logs.tco:ReadPolicies", "logs.tco:UpdatePolicies", "metrics.data-analytics#high:Read", "metrics.data-analytics#low:Read", "metrics.data-setup#high:ReadConfig", "metrics.data-setup#high:UpdateConfig", "metrics.data-setup#low:ReadConfig", "metrics.data-setup#low:UpdateConfig", "metrics.recording-rules:ReadConfig", "metrics.recording-rules:UpdateConfig", "metrics.tco:ReadPolicies", "metrics.tco:UpdatePolicies", "outbound-webhooks:ReadConfig", "outbound-webhooks:UpdateConfig", "parsing-rules:ReadConfig", "parsing-rules:UpdateConfig", "security-enrichment:ReadConfig", "security-enrichment:UpdateConfig", "serverless:Read", "service-catalog:ReadDimensionsConfig", "service-catalog:ReadSLIConfig", "service-catalog:UpdateDimensionsConfig", "service-catalog:UpdateSLIConfig", "service-map:Read", "source-mapping:UploadMapping", "spans.data-api#high:ReadData", "spans.data-api#low:ReadData", "spans.data-setup#low:ReadConfig", "spans.data-setup#low:UpdateConfig", "spans.events2metrics:ReadConfig", "spans.events2metrics:UpdateConfig", "spans.tco:ReadPolicies", "spans.tco:UpdatePolicies", "team-actions:ReadConfig", "team-actions:UpdateConfig", "team-api-keys-security-settings:Manage", "team-api-keys-security-settings:ReadConfig", "team-api-keys:Manage", "team-api-keys:ReadConfig", "team-custom-enrichment:ReadConfig", "team-custom-enrichment:ReadData", "team-custom-enrichment:UpdateConfig", "team-custom-enrichment:UpdateData", "team-dashboards:Read", "team-dashboards:Update", "user-actions:ReadConfig", "user-actions:UpdateConfig", "user-dashboards:Read", "user-dashboards:Update", "version-benchmark-tags:Read", "logs.alerts:ReadConfig", "logs.alerts:UpdateConfig", "spans.alerts:ReadConfig", "spans.alerts:UpdateConfig", "metrics.alerts:ReadConfig", "metrics.alerts:UpdateConfig", "livetail:Read", "service-catalog:Read", "version-benchmark-tags:Update", "service-catalog:ReadApdexConfig", "service-catalog:UpdateApdexConfig", "service-catalog:Update", "team-quota:Manage", "team-quota:Read"}, diags
	case "query data legacy":
		return []string{"logs.data-api#high:ReadData", "logs.data-api#low:ReadData", "metrics.data-api#high:ReadData", "metrics.data-api#low:ReadData", "opensearch-dashboards:Read", "opensearch-dashboards:Update", "snowbit.cspm:Read", "snowbit.sspm:Read", "spans.data-api#high:ReadData", "spans.data-api#low:ReadData", "livetail:Read"}, diags
	}
	diags.AddError("Invalid role", fmt.Sprintf("Unable to translate role '%v' into permissions", role))
	return []string{}, diags
}
