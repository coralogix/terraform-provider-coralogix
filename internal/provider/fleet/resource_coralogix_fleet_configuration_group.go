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
	"math"
	"net/http"
	"sort"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	cfggroups "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/fleet_manager_configuration_groups"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
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
	_ resource.ResourceWithConfigure    = &FleetConfigurationGroupResource{}
	_ resource.ResourceWithImportState  = &FleetConfigurationGroupResource{}
	_ resource.ResourceWithUpgradeState = &FleetConfigurationGroupResource{}
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
	ID          types.String            `tfsdk:"id"`
	Version     types.String            `tfsdk:"version"`
	Active      types.Bool              `tfsdk:"active"`
	Description types.String            `tfsdk:"description"`
	Preset      *FleetPresetFamilyModel `tfsdk:"preset"`
	Raw         *FleetRawFamilyModel    `tfsdk:"raw"`
}

type FleetPresetFamilyModel struct {
	ChartName             types.String `tfsdk:"chart_name"`
	ChartVersion          types.String `tfsdk:"chart_version"`
	IntegrationVersion    types.String `tfsdk:"integration_version"`
	Metadata              types.Map    `tfsdk:"metadata"`
	ObservabilityFeatures types.String `tfsdk:"observability_features"`
	// Computed: Coralogix generates the remote configurations from the preset.
	RemoteConfigurations types.List `tfsdk:"remote_configuration"`
}

type FleetRawFamilyModel struct {
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
		Version:             1,
		MarkdownDescription: "Fleet Manager configuration group with its latest family. A family is either a `preset` (a configuration template Coralogix renders into remote configurations) or `raw` (remote OpenTelemetry Collector YAML). Destroy deactivates the latest family and then archives the group. **Note: This resource is in private preview (Beta).**",
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
				Optional: true,
				Computed: true,
				Default:  int64default.StaticInt64(0),
				Validators: []validator.Int64{
					int64validator.Between(math.MinInt32, math.MaxInt32),
				},
				MarkdownDescription: "Selection precedence. Higher values win on ties. Defaults to 0.",
			},
			"family": schema.SingleNestedAttribute{
				Required:            true,
				MarkdownDescription: "Latest configuration family for this group. Exactly one of `preset` or `raw` must be set.",
				Attributes: map[string]schema.Attribute{
					"id": schema.StringAttribute{
						Computed: true,
						PlanModifiers: []planmodifier.String{
							UseStateForUnknownWhenFamilyUnchanged{Levels: 1},
						},
						MarkdownDescription: "Configuration family UUID. Replace may mint a new version.",
					},
					"version": schema.StringAttribute{
						Computed: true,
						PlanModifiers: []planmodifier.String{
							UseStateForUnknownWhenFamilyUnchanged{Levels: 1},
						},
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
					"preset": presetFamilySchema(),
					"raw":    rawFamilySchema(),
				},
			},
		},
	}
}

func presetFamilySchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional: true,
		Validators: []validator.Object{
			objectvalidator.ExactlyOneOf(path.MatchRoot("family").AtName("raw")),
		},
		MarkdownDescription: "Configuration template settings. Coralogix generates the remote configurations from them. Conflicts with `raw`.",
		Attributes: map[string]schema.Attribute{
			"chart_name": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(chartNameSchemaValues()...),
				},
				MarkdownDescription: fmt.Sprintf("Configuration template type: `otel_integration` for Kubernetes, `otel_ecs_ec2` for ECS on EC2, or a `*_standalone` template for hosts. Valid values: %s.", strings.Join(chartNameSchemaValues(), ", ")),
			},
			"chart_version": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				MarkdownDescription: "Configuration template semantic version. It determines the collector version, the generated configuration, and which `integration_version` values are supported.",
			},
			"integration_version": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					UseStateForUnknownWhenFamilyUnchanged{Levels: 2},
				},
				MarkdownDescription: "Version of the observability features format. When omitted, Coralogix resolves the default for `chart_name` and `chart_version`, and resolves it again when the family changes. " +
					"Removing it from configuration keeps the current value until another `preset` change replaces the family.",
			},
			"metadata": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Environment setup values for the template, such as `ClusterName`, `KubernetesRunningOn`, `ApplicationName`, or `SubsystemName`. Set observability features in `observability_features`, not under an `ObservabilityFeatures` key here.",
			},
			"observability_features": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					PreserveStateForEquivalentJSON{},
				},
				MarkdownDescription: "Observability feature settings as a JSON object string, for example `jsonencode({...})`. The available features depend on `chart_name` and `integration_version`. Semantically equal JSON does not plan.",
			},
			"remote_configuration": schema.ListNestedAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.List{
					UseStateForUnknownWhenFamilyUnchanged{Levels: 2},
				},
				MarkdownDescription: "Remote configurations Coralogix generated from the template settings.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Remote configuration UUID.",
						},
						"hash": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "SHA-256 hash of the normalized raw configuration.",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Remote configuration name.",
						},
						"raw_configuration": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Generated OpenTelemetry Collector configuration YAML.",
						},
						"agent_selector": schema.MapAttribute{
							Computed:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "Flat agent attributes that match agents for this configuration.",
						},
					},
				},
			},
		},
	}
}

func rawFamilySchema() schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Optional:            true,
		MarkdownDescription: "Family defined directly by its remote OpenTelemetry Collector configurations. Conflicts with `preset`.",
		Attributes: map[string]schema.Attribute{
			"collector_version": schema.StringAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					UseStateForUnknownWhenFamilyUnchanged{Levels: 2},
				},
				MarkdownDescription: "Collector semantic version this family targets, without a leading v prefix. " +
					"Removing it from configuration keeps the current value until another `raw` change replaces the family, " +
					"which sends the family without it and clears it.",
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
					listvalidator.SizeAtMost(128),
				},
				MarkdownDescription: "Remote OpenTelemetry Collector configurations in this family.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed: true,
							PlanModifiers: []planmodifier.String{
								UseStateForUnknownWhenFamilyUnchanged{Levels: 4},
							},
							MarkdownDescription: "Remote configuration UUID. Replace may mint a new version.",
						},
						"hash": schema.StringAttribute{
							Computed: true,
							PlanModifiers: []planmodifier.String{
								UseStateForUnknownWhenFamilyUnchanged{Levels: 4},
							},
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
							Validators: []validator.String{
								stringvalidator.LengthAtLeast(1),
							},
							PlanModifiers: []planmodifier.String{
								PreserveStateForEquivalentYAML{},
							},
							MarkdownDescription: "OpenTelemetry Collector configuration YAML. The supervisor-managed OpAMP extension must not be configured. Semantically equal YAML does not plan.",
						},
						"agent_selector": schema.MapAttribute{
							Optional:    true,
							ElementType: types.StringType,
							MarkdownDescription: "Flat agent attributes that match agents for this configuration. " +
								"The API may copy raw.collector_version onto service.version when that key is omitted; " +
								"the resource drops that injected key unless configuration sets it. Data-source reads keep the remote map.",
						},
					},
				},
			},
		},
	}
}

func (r *FleetConfigurationGroupResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	schemaV0 := fleetConfigurationGroupSchemaV0()
	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema:   &schemaV0,
			StateUpgrader: upgradeFleetConfigurationGroupStateV0,
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

	state, diags := flattenConfigurationGroup(ctx, plan, result.Group, true)
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

	flattened, diags := flattenConfigurationGroup(ctx, state, group, true)
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
	var prior *FleetConfigurationGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	replaceReq, diags := expandReplaceRequest(ctx, plan, prior)
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

	state, diags := flattenConfigurationGroup(ctx, plan, result.Group, true)
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
	// Archive requires an inactive family. Deactivate first; if archive then
	// fails, the group stays inactive in Coralogix and in Terraform state.
	// Retrying destroy archives. A later apply still manages the group and
	// will send active=true from the schema default.
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

	replaceReq, diags := expandReplaceRequest(ctx, &inactive, nil)
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

func expandReplaceRequest(ctx context.Context, plan, prior *FleetConfigurationGroupResourceModel) (cfggroups.ConfigurationGroupServiceReplaceConfigurationGroupRequest, diag.Diagnostics) {
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

	// Omit an unchanged family so group-only updates keep existing family IDs.
	// Sending family on every PUT can mint a new version even when content is equal.
	var priorFamily *FleetConfigurationGroupFamilyModel
	if prior != nil {
		priorFamily = prior.Family
	}
	if !familyConfigUnchanged(plan.Family, priorFamily) {
		family, familyDiags := expandFamilyReplace(ctx, plan.Family)
		diags.Append(familyDiags...)
		if familyDiags.HasError() {
			return cfggroups.ConfigurationGroupServiceReplaceConfigurationGroupRequest{}, diags
		}
		group.SetFamily(*family)
	}

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

	var diags diag.Diagnostics
	switch {
	case family.Preset != nil:
		metadata, metadataDiags := expandStringMap(ctx, family.Preset.Metadata)
		diags.Append(metadataDiags...)
		preset := cfggroups.NewPresetConfigurationFamilyCreate(
			chartNameToAPI(family.Preset.ChartName.ValueString()),
			family.Preset.ChartVersion.ValueString(),
			family.Preset.ObservabilityFeatures.ValueString(),
		)
		if !family.Preset.IntegrationVersion.IsNull() && !family.Preset.IntegrationVersion.IsUnknown() {
			preset.SetIntegrationVersion(family.Preset.IntegrationVersion.ValueString())
		}
		if metadata != nil {
			preset.SetMetadata(metadata)
		}
		out.SetPreset(*preset)
	case family.Raw != nil:
		metadata, metadataDiags := expandStringMap(ctx, family.Raw.Metadata)
		diags.Append(metadataDiags...)
		remotes, remoteDiags := expandRemoteCreates(ctx, family.Raw.RemoteConfigurations)
		diags.Append(remoteDiags...)
		raw := cfggroups.NewRawConfigurationFamilyCreate(remotes)
		if !family.Raw.CollectorVersion.IsNull() && !family.Raw.CollectorVersion.IsUnknown() {
			raw.SetCollectorVersion(family.Raw.CollectorVersion.ValueString())
		}
		if metadata != nil {
			raw.SetMetadata(metadata)
		}
		out.SetRaw(*raw)
	default:
		diags.AddError("Missing family type", "exactly one of family.preset or family.raw is required")
	}
	if diags.HasError() {
		return nil, diags
	}
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

	var diags diag.Diagnostics
	switch {
	case family.Preset != nil:
		metadata, metadataDiags := expandStringMap(ctx, family.Preset.Metadata)
		diags.Append(metadataDiags...)
		if metadata == nil {
			metadata = map[string]string{}
		}
		preset := cfggroups.NewPresetConfigurationFamilyReplace(
			chartNameToAPI(family.Preset.ChartName.ValueString()),
			family.Preset.ChartVersion.ValueString(),
			family.Preset.ObservabilityFeatures.ValueString(),
		)
		if !family.Preset.IntegrationVersion.IsNull() && !family.Preset.IntegrationVersion.IsUnknown() {
			preset.SetIntegrationVersion(family.Preset.IntegrationVersion.ValueString())
		}
		preset.SetMetadata(metadata)
		out.SetPreset(*preset)
	case family.Raw != nil:
		metadata, metadataDiags := expandStringMap(ctx, family.Raw.Metadata)
		diags.Append(metadataDiags...)
		if metadata == nil {
			metadata = map[string]string{}
		}
		remotes, remoteDiags := expandRemoteReplaces(ctx, family.Raw.RemoteConfigurations)
		diags.Append(remoteDiags...)
		raw := cfggroups.NewRawConfigurationFamilyReplace(remotes)
		if !family.Raw.CollectorVersion.IsNull() && !family.Raw.CollectorVersion.IsUnknown() {
			raw.SetCollectorVersion(family.Raw.CollectorVersion.ValueString())
		}
		raw.SetMetadata(metadata)
		out.SetRaw(*raw)
	default:
		diags.AddError("Missing family type", "exactly one of family.preset or family.raw is required")
	}
	if diags.HasError() {
		return nil, diags
	}
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

func flattenConfigurationGroup(ctx context.Context, plan *FleetConfigurationGroupResourceModel, group *cfggroups.ConfigurationGroup, dropInjectedSelector bool) (*FleetConfigurationGroupResourceModel, diag.Diagnostics) {
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
	family, familyDiags := flattenFamily(ctx, planFamily, group.Families, dropInjectedSelector)
	diags.Append(familyDiags...)
	if familyDiags.HasError() {
		return nil, diags
	}

	planDescription := types.StringNull()
	if plan != nil {
		planDescription = plan.Description
	}
	description := flattenConfiguredString(group.Description, planDescription)

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

func flattenFamily(ctx context.Context, plan *FleetConfigurationGroupFamilyModel, families []cfggroups.ConfigurationFamily, dropInjectedSelector bool) (*FleetConfigurationGroupFamilyModel, diag.Diagnostics) {
	if len(families) == 0 {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Missing family", "API returned no configuration family")}
	}
	family := families[0]

	planDescription := types.StringNull()
	var planPreset *FleetPresetFamilyModel
	var planRaw *FleetRawFamilyModel
	if plan != nil {
		planDescription = plan.Description
		planPreset = plan.Preset
		planRaw = plan.Raw
	}

	out := &FleetConfigurationGroupFamilyModel{
		ID:          types.StringValue(family.GetId()),
		Version:     types.StringValue(family.GetVersion()),
		Active:      types.BoolValue(family.GetActive()),
		Description: flattenConfiguredString(family.Description, planDescription),
	}

	var diags diag.Diagnostics
	switch {
	case family.Preset != nil:
		out.Preset, diags = flattenPresetFamily(ctx, planPreset, family.Preset, family.GetCollectorVersion())
	case family.Raw != nil:
		collectorVersion := family.Raw.GetCollectorVersion()
		if collectorVersion == "" {
			collectorVersion = family.GetCollectorVersion()
		}
		out.Raw, diags = flattenRawFamily(ctx, planRaw, family.Raw, collectorVersion, dropInjectedSelector)
	default:
		diags.AddError("Unknown family type", fmt.Sprintf("API returned configuration family %s with neither preset nor raw settings", family.GetId()))
	}
	if diags.HasError() {
		return nil, diags
	}
	return out, diags
}

func flattenPresetFamily(ctx context.Context, plan *FleetPresetFamilyModel, preset *cfggroups.PresetConfigurationFamily, collectorVersion string) (*FleetPresetFamilyModel, diag.Diagnostics) {
	planMetadata := types.MapNull(types.StringType)
	planFeatures := ""
	if plan != nil {
		planMetadata = plan.Metadata
		planFeatures = plan.ObservabilityFeatures.ValueString()
	}
	metadata, diags := flattenStringMap(ctx, preset.Metadata, planMetadata)
	if diags.HasError() {
		return nil, diags
	}

	// Generated remotes are not configurable: sort them by name and keep the
	// selector the API returns.
	remotes, remoteDiags := flattenRemotes(ctx, nil, preset.RemoteConfigurations, collectorVersion, false)
	diags.Append(remoteDiags...)
	if remoteDiags.HasError() {
		return nil, diags
	}
	remoteList, listDiags := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: remoteConfigurationAttrTypes()}, remotes)
	diags.Append(listDiags...)
	if listDiags.HasError() {
		return nil, diags
	}

	return &FleetPresetFamilyModel{
		ChartName:             chartNameFromAPI(preset.ChartName),
		ChartVersion:          types.StringValue(preset.GetChartVersion()),
		IntegrationVersion:    types.StringValue(preset.GetIntegrationVersion()),
		Metadata:              metadata,
		ObservabilityFeatures: echoJSON(planFeatures, preset.GetObservabilityFeatures()),
		RemoteConfigurations:  remoteList,
	}, diags
}

func flattenRawFamily(ctx context.Context, plan *FleetRawFamilyModel, raw *cfggroups.RawConfigurationFamily, collectorVersion string, dropInjectedSelector bool) (*FleetRawFamilyModel, diag.Diagnostics) {
	planMetadata := types.MapNull(types.StringType)
	var planRemotes []FleetRemoteConfigurationModel
	if plan != nil {
		planMetadata = plan.Metadata
		planRemotes = plan.RemoteConfigurations
	}
	metadata, diags := flattenStringMap(ctx, raw.Metadata, planMetadata)
	if diags.HasError() {
		return nil, diags
	}

	remotes, remoteDiags := flattenRemotes(ctx, planRemotes, raw.RemoteConfigurations, collectorVersion, dropInjectedSelector)
	diags.Append(remoteDiags...)
	if remoteDiags.HasError() {
		return nil, diags
	}

	collectorVersionValue := types.StringNull()
	if collectorVersion != "" {
		collectorVersionValue = types.StringValue(collectorVersion)
	}

	return &FleetRawFamilyModel{
		CollectorVersion:     collectorVersionValue,
		Metadata:             metadata,
		RemoteConfigurations: remotes,
	}, diags
}

func remoteConfigurationAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":                types.StringType,
		"hash":              types.StringType,
		"name":              types.StringType,
		"raw_configuration": types.StringType,
		"agent_selector":    types.MapType{ElemType: types.StringType},
	}
}

// chartNameSchemaValues maps CHART_NAME_OTEL_INTEGRATION to otel_integration
// for every concrete chart name the SDK knows.
func chartNameSchemaValues() []string {
	out := make([]string, 0, len(cfggroups.AllowedChartNameEnumValues))
	for _, name := range cfggroups.AllowedChartNameEnumValues {
		if name == cfggroups.CHARTNAME_CHART_NAME_UNSPECIFIED {
			continue
		}
		out = append(out, strings.ToLower(strings.TrimPrefix(string(name), chartNamePrefix)))
	}
	return out
}

const chartNamePrefix = "CHART_NAME_"

func chartNameToAPI(name string) cfggroups.ChartName {
	return cfggroups.ChartName(chartNamePrefix + strings.ToUpper(name))
}

func chartNameFromAPI(name *cfggroups.ChartName) types.String {
	if name == nil || *name == cfggroups.CHARTNAME_CHART_NAME_UNSPECIFIED {
		return types.StringNull()
	}
	return types.StringValue(strings.ToLower(strings.TrimPrefix(string(*name), chartNamePrefix)))
}

func flattenRemotes(ctx context.Context, plan []FleetRemoteConfigurationModel, remotes []cfggroups.RemoteConfiguration, collectorVersion string, dropInjectedSelector bool) ([]FleetRemoteConfigurationModel, diag.Diagnostics) {
	apiByName := make(map[string]cfggroups.RemoteConfiguration, len(remotes))
	for _, remote := range remotes {
		apiByName[remote.GetName()] = remote
	}
	planByName := make(map[string]FleetRemoteConfigurationModel, len(plan))
	for _, planned := range plan {
		planByName[planned.Name.ValueString()] = planned
	}

	order := make([]string, 0, len(remotes))
	seen := make(map[string]struct{}, len(remotes))
	for _, planned := range plan {
		name := planned.Name.ValueString()
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		order = append(order, name)
	}
	extra := make([]string, 0, len(remotes))
	for _, remote := range remotes {
		name := remote.GetName()
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		extra = append(extra, name)
	}
	if len(plan) == 0 {
		sort.Strings(extra)
	}
	order = append(order, extra...)

	var diags diag.Diagnostics
	out := make([]FleetRemoteConfigurationModel, 0, len(order))
	for _, name := range order {
		remote, ok := apiByName[name]
		if !ok {
			continue
		}
		planned := planByName[name]
		selector, selectorDiags := flattenAgentSelector(ctx, remote.AgentSelector, planned.AgentSelector, collectorVersion, dropInjectedSelector)
		diags.Append(selectorDiags...)
		out = append(out, FleetRemoteConfigurationModel{
			ID:               types.StringValue(remote.GetId()),
			Hash:             types.StringValue(remote.GetHash()),
			Name:             types.StringValue(remote.GetName()),
			RawConfiguration: echoYAML(planned.RawConfiguration.ValueString(), remote.GetRawConfiguration()),
			AgentSelector:    selector,
		})
	}
	return out, diags
}

const collectorVersionSelectorKey = "service.version"

func flattenAgentSelector(ctx context.Context, selector *cfggroups.AgentSelectorResponse, plan types.Map, collectorVersion string, dropInjectedSelector bool) (types.Map, diag.Diagnostics) {
	var attrs map[string]string
	if selector != nil {
		attrs = selector.Attributes
	}
	return flattenStringMap(ctx, selectorAttrsForState(attrs, plan, collectorVersion, dropInjectedSelector), plan)
}

// The API copies collectorVersion onto agentSelector as service.version when
// omitted. Drop that injected key on resource reads unless the user configured
// it, so state matches configuration. Data-source reads keep the remote map.
func selectorAttrsForState(api map[string]string, plan types.Map, collectorVersion string, dropInjectedSelector bool) map[string]string {
	if len(api) == 0 || !dropInjectedSelector {
		return api
	}
	if !plan.IsNull() && !plan.IsUnknown() {
		if _, configured := plan.Elements()[collectorVersionSelectorKey]; configured {
			return api
		}
	}
	injected, ok := api[collectorVersionSelectorKey]
	if !ok || collectorVersion == "" || injected != collectorVersion {
		return api
	}
	out := make(map[string]string, len(api)-1)
	for key, value := range api {
		if key == collectorVersionSelectorKey {
			continue
		}
		out[key] = value
	}
	return out
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

func flattenConfiguredString(api *string, plan types.String) types.String {
	if api != nil && *api != "" {
		return types.StringValue(*api)
	}
	// Preserve only an explicit configured empty string so description = ""
	// round-trips. A nonempty prior value against an empty API result is drift.
	if !plan.IsNull() && !plan.IsUnknown() && plan.ValueString() == "" {
		return plan
	}
	return types.StringNull()
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
