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
	"slices"
	"sort"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	cfgoverlays "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/fleet_manager_configuration_overlays"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
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

const maxOverlayConfigurationBytes = 1048576

var (
	_ resource.ResourceWithConfigure      = &FleetConfigurationOverlayResource{}
	_ resource.ResourceWithImportState    = &FleetConfigurationOverlayResource{}
	_ resource.ResourceWithValidateConfig = &FleetConfigurationOverlayResource{}
	_ planmodifier.String                 = UseStateForUnknownWhenOverlayVersionUnchanged{}
)

type FleetConfigurationOverlayResourceModel struct {
	ID                      types.String `tfsdk:"id"`
	Name                    types.String `tfsdk:"name"`
	Description             types.String `tfsdk:"description"`
	Tags                    types.List   `tfsdk:"tags"`
	PriorityOrder           types.Int64  `tfsdk:"priority_order"`
	RawOverlayConfiguration types.String `tfsdk:"raw_overlay_configuration"`
	Active                  types.Bool   `tfsdk:"active"`
	Targets                 types.Set    `tfsdk:"targets"`
	Version                 types.String `tfsdk:"version"`
	VersionID               types.String `tfsdk:"version_id"`
	OverlayHash             types.String `tfsdk:"overlay_hash"`
	CreatedBy               types.String `tfsdk:"created_by"`
}

func NewFleetConfigurationOverlayResource() resource.Resource {
	return &FleetConfigurationOverlayResource{}
}

type FleetConfigurationOverlayResource struct {
	client *cfgoverlays.FleetManagerConfigurationOverlaysAPIService
}

func (r *FleetConfigurationOverlayResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_fleet_configuration_overlay"
}

func (r *FleetConfigurationOverlayResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.ConfigurationOverlays()
}

func (r *FleetConfigurationOverlayResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	versionModifiers := []planmodifier.String{UseStateForUnknownWhenOverlayVersionUnchanged{}}
	resp.Schema = schema.Schema{
		Version: 0,
		MarkdownDescription: "Fleet Manager configuration overlay: a raw OpenTelemetry Collector YAML fragment that Fleet Manager merges into the targeted remote configurations of configuration groups. " +
			"Changing the YAML or the targets creates a new overlay version. Changing only the name, description, tags, priority or `active` does not. " +
			"Destroy deactivates the overlay and then archives it.\n\n" +
			"Known limitations:\n" +
			"- Targets are remote configuration IDs of one configuration family version. Any change to the targeted `coralogix_fleet_configuration_group` family mints new remote configuration IDs, and the overlay stops applying until its `targets` are updated to the new IDs.\n" +
			"- While an overlay is active, the targeted `coralogix_fleet_configuration_group` reads the overlay-generated family as its latest family and plans a change on every run. Manage the group and an active overlay together only after this is resolved in the backend.\n" +
			"- Overlays managed by Terraform must not have schedules configured in the Coralogix UI.\n" +
			"- The API has no concurrency control; the last write wins.\n\n" +
			"**Note: This resource is in private preview (Beta).**",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Configuration overlay UUID.",
			},
			"name": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(256),
				},
				MarkdownDescription: "Display name, unique among unarchived overlays.",
			},
			"description": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(4096),
				},
				MarkdownDescription: "Human-readable description.",
			},
			"tags": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Validators: []validator.List{
					listvalidator.SizeAtMost(64),
					listvalidator.ValueStringsAre(stringvalidator.LengthAtMost(256)),
				},
				MarkdownDescription: "Tags attached to the configuration overlay.",
			},
			"priority_order": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Default:  int64default.StaticInt64(0),
				Validators: []validator.Int64{
					int64validator.Between(math.MinInt32, math.MaxInt32),
				},
				MarkdownDescription: "Merge precedence on a shared remote configuration: higher values win, and the newer overlay wins a tie. Raw overlays apply after preset overlays. Defaults to 0.",
			},
			"raw_overlay_configuration": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, maxOverlayConfigurationBytes),
				},
				PlanModifiers: []planmodifier.String{
					PreserveStateForEquivalentYAML{},
				},
				MarkdownDescription: "OpenTelemetry Collector YAML fragment merged into each targeted remote configuration. Must not configure the OpAMP extension (`extensions.opamp`). " +
					"The API stores it normalized (sorted keys, no comments); semantically equal YAML does not plan a change. Integers beyond float64 precision may show a permanent diff.",
			},
			"active": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether the latest overlay version is applied to its targets. Requires at least one target when true. Defaults to false.",
			},
			"targets": schema.SetAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Validators: []validator.Set{
					setvalidator.SizeAtMost(256),
				},
				MarkdownDescription: "Remote configuration IDs to apply the overlay to, e.g. `coralogix_fleet_configuration_group.example.family.remote_configuration[0].id`. " +
					"Use remote configuration IDs, not configuration group IDs; their family must be active.",
			},
			"version": schema.StringAttribute{
				Computed:            true,
				PlanModifiers:       versionModifiers,
				MarkdownDescription: "Monotonic number of the latest overlay version.",
			},
			"version_id": schema.StringAttribute{
				Computed:            true,
				PlanModifiers:       versionModifiers,
				MarkdownDescription: "UUID of the latest overlay version.",
			},
			"overlay_hash": schema.StringAttribute{
				Computed:            true,
				PlanModifiers:       versionModifiers,
				MarkdownDescription: "SHA-256 hash of the normalized YAML of the latest overlay version.",
			},
			"created_by": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "User that created the configuration overlay.",
			},
		},
	}
}

func (r *FleetConfigurationOverlayResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var active types.Bool
	var targets types.Set
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("active"), &active)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("targets"), &targets)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if active.IsNull() || active.IsUnknown() || !active.ValueBool() || targets.IsUnknown() {
		return
	}
	if targets.IsNull() || len(targets.Elements()) == 0 {
		resp.Diagnostics.AddAttributeError(path.Root("targets"),
			"Missing targets",
			"An active configuration overlay needs at least one target. Set targets or set active = false.",
		)
	}
}

func (r *FleetConfigurationOverlayResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *FleetConfigurationOverlayResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq, diags := expandOverlayCreateRequest(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	result, httpResponse, err := r.client.
		ConfigurationOverlayServiceCreateConfigurationOverlay(ctx).
		ConfigurationOverlayServiceCreateConfigurationOverlayRequest(createReq).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error creating coralogix_fleet_configuration_overlay",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", createReq),
		)
		return
	}

	state, diags := flattenConfigurationOverlay(ctx, plan, &result.Overlay)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *FleetConfigurationOverlayResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *FleetConfigurationOverlayResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, httpResponse, err := r.client.
		ConfigurationOverlayServiceGetConfigurationOverlay(ctx, state.ID.ValueString()).
		Execute()
	if err != nil {
		if httpResponse != nil && httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				"coralogix_fleet_configuration_overlay is in state, but no longer exists in Coralogix backend",
				"coralogix_fleet_configuration_overlay will be recreated when you apply",
			)
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading coralogix_fleet_configuration_overlay",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
		)
		return
	}

	flattened, diags := flattenConfigurationOverlay(ctx, state, &result.Overlay)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, flattened)...)
}

func (r *FleetConfigurationOverlayResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan *FleetConfigurationOverlayResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var prior *FleetConfigurationOverlayResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq, changed, diags := expandOverlayUpdateRequest(ctx, plan, prior)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	id := prior.ID.ValueString()
	var overlay *cfgoverlays.ConfigurationOverlay
	if changed {
		// Send only changed fields: resending the same YAML or targets mints a
		// new overlay version.
		result, httpResponse, err := r.client.
			ConfigurationOverlayServiceUpdateConfigurationOverlay(ctx, id).
			ConfigurationOverlayServiceUpdateConfigurationOverlayRequest(updateReq).
			Execute()
		if err != nil {
			resp.Diagnostics.AddError("Error updating coralogix_fleet_configuration_overlay",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Update", updateReq),
			)
			return
		}
		overlay = &result.Overlay
	} else {
		// Only null vs empty representation changed; the API rejects empty updates.
		result, httpResponse, err := r.client.
			ConfigurationOverlayServiceGetConfigurationOverlay(ctx, id).
			Execute()
		if err != nil {
			resp.Diagnostics.AddError("Error reading coralogix_fleet_configuration_overlay",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
			)
			return
		}
		overlay = &result.Overlay
	}

	state, diags := flattenConfigurationOverlay(ctx, plan, overlay)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *FleetConfigurationOverlayResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state *FleetConfigurationOverlayResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	// Archive requires an inactive overlay. If archive then fails, the overlay
	// stays inactive in Coralogix and in Terraform state; retrying destroy archives.
	if !state.Active.IsNull() && state.Active.ValueBool() {
		deactivateReq := cfgoverlays.ConfigurationOverlayServiceUpdateConfigurationOverlayRequest{
			Overlay: &cfgoverlays.ConfigurationOverlayServiceUpdateConfigurationOverlayRequestOverlay{
				Active: cfgoverlays.PtrBool(false),
			},
		}
		_, httpResponse, err := r.client.
			ConfigurationOverlayServiceUpdateConfigurationOverlay(ctx, id).
			ConfigurationOverlayServiceUpdateConfigurationOverlayRequest(deactivateReq).
			Execute()
		if err != nil {
			if httpResponse != nil && httpResponse.StatusCode == http.StatusNotFound {
				return
			}
			resp.Diagnostics.AddError("Error deactivating coralogix_fleet_configuration_overlay before archive",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", deactivateReq),
			)
			return
		}
	}

	_, httpResponse, err := r.client.
		ConfigurationOverlayServiceArchiveConfigurationOverlay(ctx, id).
		Execute()
	if err != nil {
		if httpResponse != nil && httpResponse.StatusCode == http.StatusNotFound {
			return
		}
		resp.Diagnostics.AddError("Error archiving coralogix_fleet_configuration_overlay",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", nil),
		)
	}
}

func (r *FleetConfigurationOverlayResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func expandOverlayCreateRequest(ctx context.Context, plan *FleetConfigurationOverlayResourceModel) (cfgoverlays.ConfigurationOverlayServiceCreateConfigurationOverlayRequest, diag.Diagnostics) {
	overlay := cfgoverlays.ConfigurationOverlayCreate{
		Raw: cfgoverlays.RawOverlayPayload{Configuration: plan.RawOverlayConfiguration.ValueString()},
	}
	if !plan.Name.IsNull() && !plan.Name.IsUnknown() {
		overlay.SetName(plan.Name.ValueString())
	}
	if !plan.Description.IsNull() && !plan.Description.IsUnknown() {
		overlay.SetDescription(plan.Description.ValueString())
	}
	tags, diags := expandStringList(ctx, plan.Tags)
	if tags != nil {
		overlay.SetTags(tags)
	}
	if !plan.PriorityOrder.IsNull() && !plan.PriorityOrder.IsUnknown() {
		overlay.SetPriorityOrder(int32(plan.PriorityOrder.ValueInt64()))
	}
	if !plan.Active.IsNull() && !plan.Active.IsUnknown() {
		overlay.SetActive(plan.Active.ValueBool())
	}

	targetIDs, targetDiags := expandStringSet(ctx, plan.Targets)
	diags.Append(targetDiags...)
	targets := make([]cfgoverlays.ConfigurationOverlayTargetCreate, 0, len(targetIDs))
	for _, targetID := range targetIDs {
		targets = append(targets, cfgoverlays.ConfigurationOverlayTargetCreate{RemoteConfigurationId: targetID})
	}

	return cfgoverlays.ConfigurationOverlayServiceCreateConfigurationOverlayRequest{
		Overlay: overlay,
		Targets: targets,
	}, diags
}

// expandOverlayUpdateRequest builds a PATCH body holding only the fields that
// differ between plan and prior state. It reports whether anything changed.
func expandOverlayUpdateRequest(ctx context.Context, plan, prior *FleetConfigurationOverlayResourceModel) (cfgoverlays.ConfigurationOverlayServiceUpdateConfigurationOverlayRequest, bool, diag.Diagnostics) {
	overlay := cfgoverlays.ConfigurationOverlayServiceUpdateConfigurationOverlayRequestOverlay{}
	overlayChanged := false

	// The API clears name and description on an empty string and ignores omitted fields.
	if !plan.Name.Equal(prior.Name) {
		overlay.SetName(plan.Name.ValueString())
		overlayChanged = true
	}
	if !plan.Description.Equal(prior.Description) {
		overlay.SetDescription(plan.Description.ValueString())
		overlayChanged = true
	}
	if !plan.PriorityOrder.Equal(prior.PriorityOrder) {
		overlay.SetPriorityOrder(int32(plan.PriorityOrder.ValueInt64()))
		overlayChanged = true
	}
	if !plan.Active.Equal(prior.Active) {
		overlay.SetActive(plan.Active.ValueBool())
		overlayChanged = true
	}
	if !yamlStringsEqual(plan.RawOverlayConfiguration.ValueString(), prior.RawOverlayConfiguration.ValueString()) {
		overlay.Raw = &cfgoverlays.ConfigurationOverlayServiceUpdateConfigurationOverlayRequestOverlayRaw{
			Configuration: plan.RawOverlayConfiguration.ValueString(),
		}
		overlayChanged = true
	}

	var diags diag.Diagnostics
	planTags, tagDiags := expandStringList(ctx, plan.Tags)
	diags.Append(tagDiags...)
	priorTags, tagDiags := expandStringList(ctx, prior.Tags)
	diags.Append(tagDiags...)
	if !slices.Equal(planTags, priorTags) {
		overlay.Tags = &cfgoverlays.ConfigurationOverlayServiceUpdateConfigurationOverlayRequestOverlayTags{
			Values: nonNilStrings(planTags),
		}
		overlayChanged = true
	}

	updateReq := cfgoverlays.ConfigurationOverlayServiceUpdateConfigurationOverlayRequest{}
	if overlayChanged {
		updateReq.Overlay = &overlay
	}

	targetsReplace, targetsChanged, targetDiags := expandTargetsReplace(ctx, plan.Targets, prior.Targets)
	diags.Append(targetDiags...)
	if targetsChanged {
		updateReq.TargetReplacement = targetsReplace
	}

	return updateReq, overlayChanged || targetsChanged, diags
}

func expandTargetsReplace(ctx context.Context, plan, prior types.Set) (*cfgoverlays.ConfigurationOverlayTargetsReplace, bool, diag.Diagnostics) {
	planIDs, diags := expandStringSet(ctx, plan)
	priorIDs, priorDiags := expandStringSet(ctx, prior)
	diags.Append(priorDiags...)
	if slices.Equal(planIDs, priorIDs) {
		return nil, false, diags
	}
	targets := make([]cfgoverlays.ConfigurationOverlayTargetReplace, 0, len(planIDs))
	for _, targetID := range planIDs {
		targets = append(targets, cfgoverlays.ConfigurationOverlayTargetReplace{RemoteConfigurationId: targetID})
	}
	return &cfgoverlays.ConfigurationOverlayTargetsReplace{Targets: targets}, true, diags
}

// expandStringSet returns the set's values sorted, so two sets compare with slices.Equal.
func expandStringSet(ctx context.Context, set types.Set) ([]string, diag.Diagnostics) {
	if set.IsNull() || set.IsUnknown() {
		return nil, nil
	}
	var values []string
	diags := set.ElementsAs(ctx, &values, false)
	sort.Strings(values)
	return values, diags
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func flattenConfigurationOverlay(ctx context.Context, plan *FleetConfigurationOverlayResourceModel, overlay *cfgoverlays.ConfigurationOverlay) (*FleetConfigurationOverlayResourceModel, diag.Diagnostics) {
	if overlay == nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Empty configuration overlay", "API returned no configuration overlay")}
	}
	if len(overlay.Versions) == 0 {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Missing overlay version", "API returned no configuration overlay version")}
	}
	// Versions are newest first; archived versions are omitted.
	latest := overlay.Versions[0]
	if latest.Raw == nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic(
			"Unsupported configuration overlay",
			fmt.Sprintf("Configuration overlay %s is a preset overlay. coralogix_fleet_configuration_overlay manages raw YAML overlays only.", overlay.GetId()),
		)}
	}
	if plan == nil {
		plan = &FleetConfigurationOverlayResourceModel{}
	}

	tags, diags := flattenStringList(ctx, overlay.Tags, plan.Tags)
	if diags.HasError() {
		return nil, diags
	}

	priority := int64(0)
	if overlay.PriorityOrder != nil {
		priority = int64(*overlay.PriorityOrder)
	}

	return &FleetConfigurationOverlayResourceModel{
		ID:                      types.StringValue(overlay.GetId()),
		Name:                    flattenConfiguredString(overlay.Name, plan.Name),
		Description:             flattenConfiguredString(overlay.Description, plan.Description),
		Tags:                    tags,
		PriorityOrder:           types.Int64Value(priority),
		RawOverlayConfiguration: echoYAML(plan.RawOverlayConfiguration.ValueString(), latest.Raw.Configuration),
		Active:                  types.BoolValue(overlay.GetActive()),
		Targets:                 flattenOverlayTargets(latest.Targets, plan.Targets),
		Version:                 types.StringValue(latest.Version),
		VersionID:               types.StringValue(latest.Id),
		OverlayHash:             types.StringValue(latest.GetOverlayHash()),
		CreatedBy:               types.StringValue(overlay.GetCreatedBy()),
	}, diags
}

// flattenOverlayTargets keeps the IDs the user sent. The overlay* IDs point at
// generated families and change on every activation.
func flattenOverlayTargets(targets []cfgoverlays.ConfigurationOverlayTarget, plan types.Set) types.Set {
	if len(targets) == 0 {
		if plan.IsNull() || plan.IsUnknown() {
			return types.SetNull(types.StringType)
		}
		return types.SetValueMust(types.StringType, []attr.Value{})
	}
	elems := make([]attr.Value, 0, len(targets))
	for _, target := range targets {
		elems = append(elems, types.StringValue(target.SourceRemoteConfigurationId))
	}
	return types.SetValueMust(types.StringType, elems)
}

// UseStateForUnknownWhenOverlayVersionUnchanged keeps the computed version
// attributes when neither the YAML (semantically) nor the targets change. Only
// those two mint a new overlay version.
type UseStateForUnknownWhenOverlayVersionUnchanged struct{}

func (m UseStateForUnknownWhenOverlayVersionUnchanged) Description(_ context.Context) string {
	return "Keeps the previous computed value when the overlay YAML and targets are unchanged."
}

func (m UseStateForUnknownWhenOverlayVersionUnchanged) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m UseStateForUnknownWhenOverlayVersionUnchanged) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if !req.PlanValue.IsUnknown() || req.StateValue.IsNull() || req.StateValue.IsUnknown() {
		return
	}
	if !yamlAttrUnchanged(ctx, req, path.Root("raw_overlay_configuration")) {
		return
	}
	var planTargets, stateTargets types.Set
	if diags := req.Plan.GetAttribute(ctx, path.Root("targets"), &planTargets); diags.HasError() || planTargets.IsUnknown() {
		return
	}
	if diags := req.State.GetAttribute(ctx, path.Root("targets"), &stateTargets); diags.HasError() {
		return
	}
	planIDs, _ := expandStringSet(ctx, planTargets)
	stateIDs, _ := expandStringSet(ctx, stateTargets)
	if slices.Equal(planIDs, stateIDs) {
		resp.PlanValue = req.StateValue
	}
}
