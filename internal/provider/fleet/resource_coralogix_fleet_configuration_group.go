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

package fleet

import (
	"context"
	"fmt"
	"net/http"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	cfggroups "github.com/coralogix/terraform-provider-coralogix/internal/openapi/configuration_group_service"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.ResourceWithConfigure   = &FleetConfigurationGroupResource{}
	_ resource.ResourceWithImportState = &FleetConfigurationGroupResource{}
)

type FleetConfigurationGroupResourceModel struct {
	ID            types.String                        `tfsdk:"id"`
	Name          types.String                        `tfsdk:"name"`
	Description   types.String                        `tfsdk:"description"`
	Tags          types.List                          `tfsdk:"tags"`
	PriorityOrder types.Int64                         `tfsdk:"priority_order"`
	Family        *FleetConfigurationGroupFamilyModel `tfsdk:"family"`
}

type FleetConfigurationGroupFamilyModel struct {
	ID                   types.String                    `tfsdk:"id"`
	Version              types.String                    `tfsdk:"version"`
	Active               types.Bool                      `tfsdk:"active"`
	Description          types.String                    `tfsdk:"description"`
	CollectorVersion     types.String                    `tfsdk:"collector_version"`
	Metadata             types.Map                       `tfsdk:"metadata"`
	RemoteConfigurations []FleetRemoteConfigurationModel `tfsdk:"remote_configuration"`
}

type FleetRemoteConfigurationModel struct {
	ID               types.String `tfsdk:"id"`
	Hash             types.String `tfsdk:"hash"`
	Name             types.String `tfsdk:"name"`
	RawConfiguration types.String `tfsdk:"raw_configuration"`
	AgentSelector    types.Map    `tfsdk:"agent_selector"`
}

func NewFleetConfigurationGroupResource() resource.Resource {
	return &FleetConfigurationGroupResource{}
}

type FleetConfigurationGroupResource struct {
	client *cfggroups.FleetManagerConfigurationGroupsAPIService
}

func (r *FleetConfigurationGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_fleet_configuration_group"
}

func (r *FleetConfigurationGroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.ConfigurationGroups()
}

func (r *FleetConfigurationGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:             0,
		MarkdownDescription: "Fleet Manager configuration group with its latest family and remote OpenTelemetry Collector YAML. Destroy archives the group. **Note: This resource is in Beta stage.**",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Configuration group UUID.",
			},
			"name": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Display name.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Human-readable description.",
			},
			"tags": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Tags attached to the configuration group.",
			},
			"priority_order": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(0),
				MarkdownDescription: "Selection precedence. Higher values win on ties. Defaults to 0.",
			},
			"family": schema.SingleNestedAttribute{
				Required:            true,
				MarkdownDescription: "Latest configuration family for this group.",
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Configuration family UUID. Replace may mint a new version.",
					},
					"version": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Monotonic family version within the group.",
					},
					"active": schema.BoolAttribute{
						Optional:            true,
						Computed:            true,
						Default:             booldefault.StaticBool(true),
						MarkdownDescription: "Whether this family is active. Defaults to true.",
					},
					"description": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Human-readable family description.",
					},
					"collector_version": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Collector semantic version this family targets, without a leading v prefix.",
					},
					"metadata": schema.MapAttribute{
						Optional:            true,
						ElementType:         types.StringType,
						MarkdownDescription: "Metadata stored with this configuration family.",
					},
					"remote_configuration": schema.ListNestedAttribute{
						Required: true,
						Validators: []validator.List{
							listvalidator.SizeAtLeast(1),
						},
						MarkdownDescription: "Remote OpenTelemetry Collector configurations in this family.",
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"id": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "Remote configuration UUID. Replace may mint a new version.",
								},
								"hash": schema.StringAttribute{
									Computed:            true,
									MarkdownDescription: "SHA-256 hash of the normalized raw configuration. Replace may mint a new version.",
								},
								"name": schema.StringAttribute{
									Required: true,
									Validators: []validator.String{
										stringvalidator.LengthAtLeast(1),
									},
									MarkdownDescription: "Remote configuration name.",
								},
								"raw_configuration": schema.StringAttribute{
									Required: true,
									PlanModifiers: []planmodifier.String{
										PreserveStateForEquivalentYAML{},
									},
									MarkdownDescription: "OpenTelemetry Collector configuration YAML. The supervisor-managed OpAMP extension must not be configured. Semantically equal YAML does not plan.",
								},
								"agent_selector": schema.MapAttribute{
									Optional:            true,
									ElementType:         types.StringType,
									MarkdownDescription: "Flat agent attributes that match agents for this configuration.",
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *FleetConfigurationGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *FleetConfigurationGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq, diags := expandCreateRequest(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	result, httpResponse, err := r.client.
		ConfigurationGroupServiceCreateConfigurationGroup(ctx).
		ConfigurationGroupServiceCreateConfigurationGroupRequest(createReq).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error creating coralogix_fleet_configuration_group",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", createReq),
		)
		return
	}

	state, diags := flattenConfigurationGroup(ctx, plan, result.Group)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *FleetConfigurationGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *FleetConfigurationGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	group, httpResponse, err := r.getLatestFamily(ctx, state.ID.ValueString())
	if err != nil {
		if httpResponse != nil && httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				"coralogix_fleet_configuration_group is in state, but no longer exists in Coralogix backend",
				"coralogix_fleet_configuration_group will be recreated when you apply",
			)
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading coralogix_fleet_configuration_group",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
		)
		return
	}

	flattened, diags := flattenConfigurationGroup(ctx, state, group)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, flattened)...)
}

func (r *FleetConfigurationGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan *FleetConfigurationGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	replaceReq, diags := expandReplaceRequest(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	result, httpResponse, err := r.client.
		ConfigurationGroupServiceReplaceConfigurationGroup(ctx, plan.ID.ValueString()).
		ConfigurationGroupServiceReplaceConfigurationGroupRequest(replaceReq).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error updating coralogix_fleet_configuration_group",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Update", replaceReq),
		)
		return
	}

	state, diags := flattenConfigurationGroup(ctx, plan, result.Group)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *FleetConfigurationGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state *FleetConfigurationGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	if err := deactivateFamilyIfActive(ctx, r.client, state); err != nil {
		resp.Diagnostics.AddError("Error archiving coralogix_fleet_configuration_group", err.Error())
		return
	}

	_, httpResponse, err := r.client.
		ConfigurationGroupServiceArchiveConfigurationGroup(ctx, id).
		Execute()
	if err != nil {
		if httpResponse != nil && httpResponse.StatusCode == http.StatusNotFound {
			return
		}
		resp.Diagnostics.AddError("Error archiving coralogix_fleet_configuration_group",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", nil),
		)
	}
}

func (r *FleetConfigurationGroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *FleetConfigurationGroupResource) getLatestFamily(ctx context.Context, id string) (*cfggroups.ConfigurationGroup, *http.Response, error) {
	result, httpResponse, err := r.client.
		ConfigurationGroupServiceGetConfigurationGroup(ctx, id).
		LatestFamilyOnly(true).
		Execute()
	if err != nil {
		return nil, httpResponse, err
	}
	if result == nil || result.Group == nil {
		return nil, httpResponse, fmt.Errorf("configuration group %s was empty", id)
	}
	return result.Group, httpResponse, nil
}

func deactivateFamilyIfActive(ctx context.Context, client *cfggroups.FleetManagerConfigurationGroupsAPIService, state *FleetConfigurationGroupResourceModel) error {
	if state.Family == nil || state.Family.Active.IsNull() || !state.Family.Active.ValueBool() {
		return nil
	}

	inactive := *state
	family := *state.Family
	family.Active = types.BoolValue(false)
	inactive.Family = &family

	replaceReq, diags := expandReplaceRequest(ctx, &inactive)
	if diags.HasError() {
		return fmt.Errorf("preparing deactivate request: %s", diags.Errors()[0].Detail())
	}

	_, httpResponse, err := client.
		ConfigurationGroupServiceReplaceConfigurationGroup(ctx, state.ID.ValueString()).
		ConfigurationGroupServiceReplaceConfigurationGroupRequest(replaceReq).
		Execute()
	if err != nil {
		if httpResponse != nil && httpResponse.StatusCode == http.StatusNotFound {
			return nil
		}
		return fmt.Errorf("deactivating family before archive: %w", cxsdkOpenapi.NewAPIError(httpResponse, err))
	}
	return nil
}

func expandCreateRequest(ctx context.Context, plan *FleetConfigurationGroupResourceModel) (cfggroups.ConfigurationGroupServiceCreateConfigurationGroupRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	group := cfggroups.NewConfigurationGroupCreate()
	group.SetName(plan.Name.ValueString())
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		group.SetDescription(plan.Description.ValueString())
	}
	tags, tagDiags := expandStringList(ctx, plan.Tags)
	diags.Append(tagDiags...)
	if tags != nil {
		group.SetTags(tags)
	}
	if !plan.PriorityOrder.IsNull() && !plan.PriorityOrder.IsUnknown() {
		group.SetPriorityOrder(int32(plan.PriorityOrder.ValueInt64()))
	}

	family, familyDiags := expandFamilyCreate(ctx, plan.Family)
	diags.Append(familyDiags...)
	if familyDiags.HasError() {
		return cfggroups.ConfigurationGroupServiceCreateConfigurationGroupRequest{}, diags
	}
	group.SetFamily(*family)

	req := cfggroups.NewConfigurationGroupServiceCreateConfigurationGroupRequest()
	req.SetGroup(*group)
	return *req, diags
}

func expandReplaceRequest(ctx context.Context, plan *FleetConfigurationGroupResourceModel) (cfggroups.ConfigurationGroupServiceReplaceConfigurationGroupRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	group := cfggroups.NewConfigurationGroupServiceReplaceConfigurationGroupRequestGroup()
	group.SetName(plan.Name.ValueString())
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		group.SetDescription(plan.Description.ValueString())
	} else {
		group.SetDescription("")
	}
	tags, tagDiags := expandStringList(ctx, plan.Tags)
	diags.Append(tagDiags...)
	if tags == nil {
		tags = []string{}
	}
	group.SetTags(tags)
	if !plan.PriorityOrder.IsNull() && !plan.PriorityOrder.IsUnknown() {
		group.SetPriorityOrder(int32(plan.PriorityOrder.ValueInt64()))
	}

	family, familyDiags := expandFamilyReplace(ctx, plan.Family)
	diags.Append(familyDiags...)
	if familyDiags.HasError() {
		return cfggroups.ConfigurationGroupServiceReplaceConfigurationGroupRequest{}, diags
	}
	group.SetFamily(*family)

	req := cfggroups.NewConfigurationGroupServiceReplaceConfigurationGroupRequest()
	req.SetGroup(*group)
	return *req, diags
}

func expandFamilyCreate(ctx context.Context, family *FleetConfigurationGroupFamilyModel) (*cfggroups.ConfigurationFamilyCreate, diag.Diagnostics) {
	if family == nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Missing family", "family is required")}
	}
	out := cfggroups.NewConfigurationFamilyCreate()
	if !family.Active.IsNull() && !family.Active.IsUnknown() {
		out.SetActive(family.Active.ValueBool())
	}
	if !family.Description.IsNull() && !family.Description.IsUnknown() {
		out.SetDescription(family.Description.ValueString())
	}
	if !family.CollectorVersion.IsNull() && !family.CollectorVersion.IsUnknown() {
		out.SetCollectorVersion(family.CollectorVersion.ValueString())
	}
	metadata, diags := expandStringMap(ctx, family.Metadata)
	if diags.HasError() {
		return nil, diags
	}
	if metadata != nil {
		out.SetMetadata(metadata)
	}
	remotes, remoteDiags := expandRemoteCreates(ctx, family.RemoteConfigurations)
	diags.Append(remoteDiags...)
	if remoteDiags.HasError() {
		return nil, diags
	}
	out.SetRemoteConfigurations(remotes)
	return out, diags
}

func expandFamilyReplace(ctx context.Context, family *FleetConfigurationGroupFamilyModel) (*cfggroups.ConfigurationGroupServiceReplaceConfigurationGroupRequestGroupFamily, diag.Diagnostics) {
	if family == nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Missing family", "family is required")}
	}
	out := cfggroups.NewConfigurationGroupServiceReplaceConfigurationGroupRequestGroupFamily()
	if !family.Active.IsNull() && !family.Active.IsUnknown() {
		out.SetActive(family.Active.ValueBool())
	}
	if !family.Description.IsNull() && !family.Description.IsUnknown() {
		out.SetDescription(family.Description.ValueString())
	} else {
		out.SetDescription("")
	}
	if !family.CollectorVersion.IsNull() && !family.CollectorVersion.IsUnknown() {
		out.SetCollectorVersion(family.CollectorVersion.ValueString())
	}
	metadata, diags := expandStringMap(ctx, family.Metadata)
	if diags.HasError() {
		return nil, diags
	}
	if metadata == nil {
		metadata = map[string]string{}
	}
	out.SetMetadata(metadata)
	remotes, remoteDiags := expandRemoteReplaces(ctx, family.RemoteConfigurations)
	diags.Append(remoteDiags...)
	if remoteDiags.HasError() {
		return nil, diags
	}
	out.SetRemoteConfigurations(remotes)
	return out, diags
}

func expandRemoteCreates(ctx context.Context, remotes []FleetRemoteConfigurationModel) ([]cfggroups.RemoteConfigurationCreate, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := make([]cfggroups.RemoteConfigurationCreate, 0, len(remotes))
	for _, remote := range remotes {
		item := cfggroups.NewRemoteConfigurationCreate()
		item.SetName(remote.Name.ValueString())
		item.SetRawConfiguration(remote.RawConfiguration.ValueString())
		selector, selectorDiags := expandAgentSelector(ctx, remote.AgentSelector)
		diags.Append(selectorDiags...)
		if selector != nil {
			item.SetAgentSelector(*selector)
		}
		out = append(out, *item)
	}
	return out, diags
}

func expandRemoteReplaces(ctx context.Context, remotes []FleetRemoteConfigurationModel) ([]cfggroups.RemoteConfigurationReplace, diag.Diagnostics) {
	var diags diag.Diagnostics
	out := make([]cfggroups.RemoteConfigurationReplace, 0, len(remotes))
	for _, remote := range remotes {
		item := cfggroups.NewRemoteConfigurationReplace()
		item.SetName(remote.Name.ValueString())
		item.SetRawConfiguration(remote.RawConfiguration.ValueString())
		selector, selectorDiags := expandAgentSelector(ctx, remote.AgentSelector)
		diags.Append(selectorDiags...)
		if selector != nil {
			item.SetAgentSelector(*selector)
		}
		out = append(out, *item)
	}
	return out, diags
}

func expandAgentSelector(ctx context.Context, selector types.Map) (*cfggroups.AgentSelectorRequest, diag.Diagnostics) {
	attrs, diags := expandStringMap(ctx, selector)
	if diags.HasError() || attrs == nil {
		return nil, diags
	}
	req := cfggroups.NewAgentSelectorRequest()
	req.SetAttributes(attrs)
	return req, diags
}

func expandStringList(ctx context.Context, list types.List) ([]string, diag.Diagnostics) {
	if list.IsNull() || list.IsUnknown() {
		return nil, nil
	}
	var values []string
	diags := list.ElementsAs(ctx, &values, false)
	return values, diags
}

func expandStringMap(ctx context.Context, m types.Map) (map[string]string, diag.Diagnostics) {
	if m.IsNull() || m.IsUnknown() {
		return nil, nil
	}
	values := make(map[string]string)
	diags := m.ElementsAs(ctx, &values, false)
	return values, diags
}

func flattenConfigurationGroup(ctx context.Context, plan *FleetConfigurationGroupResourceModel, group *cfggroups.ConfigurationGroup) (*FleetConfigurationGroupResourceModel, diag.Diagnostics) {
	if group == nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Empty configuration group", "API returned no configuration group")}
	}

	tags, diags := flattenStringList(ctx, group.Tags, plan.Tags)
	if diags.HasError() {
		return nil, diags
	}

	var planFamily *FleetConfigurationGroupFamilyModel
	if plan != nil {
		planFamily = plan.Family
	}
	family, familyDiags := flattenFamily(ctx, planFamily, group.Families)
	diags.Append(familyDiags...)
	if familyDiags.HasError() {
		return nil, diags
	}

	description := types.StringNull()
	if group.Description != nil && *group.Description != "" {
		description = types.StringValue(*group.Description)
	} else if plan != nil && !plan.Description.IsNull() && plan.Description.ValueString() == "" {
		description = types.StringNull()
	}

	priority := int64(0)
	if group.PriorityOrder != nil {
		priority = int64(*group.PriorityOrder)
	}

	return &FleetConfigurationGroupResourceModel{
		ID:            types.StringValue(group.GetId()),
		Name:          types.StringValue(group.GetName()),
		Description:   description,
		Tags:          tags,
		PriorityOrder: types.Int64Value(priority),
		Family:        family,
	}, diags
}

func flattenFamily(ctx context.Context, plan *FleetConfigurationGroupFamilyModel, families []cfggroups.ConfigurationFamily) (*FleetConfigurationGroupFamilyModel, diag.Diagnostics) {
	if len(families) == 0 {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Missing family", "API returned no configuration family")}
	}
	family := families[0]
	planMetadata := types.MapNull(types.StringType)
	if plan != nil {
		planMetadata = plan.Metadata
	}
	metadata, diags := flattenStringMap(ctx, family.Metadata, planMetadata)
	if diags.HasError() {
		return nil, diags
	}

	var planRemotes []FleetRemoteConfigurationModel
	if plan != nil {
		planRemotes = plan.RemoteConfigurations
	}
	remotes, remoteDiags := flattenRemotes(ctx, planRemotes, family.RemoteConfigurations)
	diags.Append(remoteDiags...)
	if remoteDiags.HasError() {
		return nil, diags
	}

	description := types.StringNull()
	if family.Description != nil && *family.Description != "" {
		description = types.StringValue(*family.Description)
	}
	collectorVersion := types.StringNull()
	if family.CollectorVersion != nil && *family.CollectorVersion != "" {
		collectorVersion = types.StringValue(*family.CollectorVersion)
	}

	return &FleetConfigurationGroupFamilyModel{
		ID:                   types.StringValue(family.GetId()),
		Version:              types.StringValue(family.GetVersion()),
		Active:               types.BoolValue(family.GetActive()),
		Description:          description,
		CollectorVersion:     collectorVersion,
		Metadata:             metadata,
		RemoteConfigurations: remotes,
	}, diags
}

func flattenRemotes(ctx context.Context, plan []FleetRemoteConfigurationModel, remotes []cfggroups.RemoteConfiguration) ([]FleetRemoteConfigurationModel, diag.Diagnostics) {
	byName := make(map[string]FleetRemoteConfigurationModel, len(plan))
	for _, remote := range plan {
		byName[remote.Name.ValueString()] = remote
	}

	var diags diag.Diagnostics
	out := make([]FleetRemoteConfigurationModel, 0, len(remotes))
	for _, remote := range remotes {
		planned, ok := byName[remote.GetName()]
		var plannedYAML string
		var plannedSelector types.Map
		if ok {
			plannedYAML = planned.RawConfiguration.ValueString()
			plannedSelector = planned.AgentSelector
		}
		selector, selectorDiags := flattenAgentSelector(ctx, remote.AgentSelector, plannedSelector)
		diags.Append(selectorDiags...)
		out = append(out, FleetRemoteConfigurationModel{
			ID:               types.StringValue(remote.GetId()),
			Hash:             types.StringValue(remote.GetHash()),
			Name:             types.StringValue(remote.GetName()),
			RawConfiguration: echoYAML(plannedYAML, remote.GetRawConfiguration()),
			AgentSelector:    selector,
		})
	}
	return out, diags
}

func flattenAgentSelector(ctx context.Context, selector *cfggroups.AgentSelectorResponse, plan types.Map) (types.Map, diag.Diagnostics) {
	var attrs map[string]string
	if selector != nil {
		attrs = selector.Attributes
	}
	return flattenStringMap(ctx, attrs, plan)
}

func flattenStringList(_ context.Context, values []string, plan types.List) (types.List, diag.Diagnostics) {
	if len(values) == 0 {
		if plan.IsNull() {
			return types.ListNull(types.StringType), nil
		}
		return types.ListValueMust(types.StringType, []attr.Value{}), nil
	}
	elems := make([]attr.Value, 0, len(values))
	for _, value := range values {
		elems = append(elems, types.StringValue(value))
	}
	return types.ListValue(types.StringType, elems)
}

func flattenStringMap(_ context.Context, values map[string]string, plan types.Map) (types.Map, diag.Diagnostics) {
	if len(values) == 0 {
		if plan.IsNull() {
			return types.MapNull(types.StringType), nil
		}
		return types.MapValueMust(types.StringType, map[string]attr.Value{}), nil
	}
	elems := make(map[string]attr.Value, len(values))
	for key, value := range values {
		elems[key] = types.StringValue(value)
	}
	return types.MapValue(types.StringType, elems)
}
