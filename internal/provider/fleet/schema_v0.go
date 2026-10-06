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

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Schema version 0 kept collector_version, metadata and remote_configuration
// directly on family. Version 1 moves them under family.raw and adds
// family.preset. The v0 shape below is frozen: only its types matter.

type fleetConfigurationGroupResourceModelV0 struct {
	ID            types.String                          `tfsdk:"id"`
	Name          types.String                          `tfsdk:"name"`
	Description   types.String                          `tfsdk:"description"`
	Tags          types.List                            `tfsdk:"tags"`
	PriorityOrder types.Int64                           `tfsdk:"priority_order"`
	Family        *fleetConfigurationGroupFamilyModelV0 `tfsdk:"family"`
}

type fleetConfigurationGroupFamilyModelV0 struct {
	ID                   types.String                    `tfsdk:"id"`
	Version              types.String                    `tfsdk:"version"`
	Active               types.Bool                      `tfsdk:"active"`
	Description          types.String                    `tfsdk:"description"`
	CollectorVersion     types.String                    `tfsdk:"collector_version"`
	Metadata             types.Map                       `tfsdk:"metadata"`
	RemoteConfigurations []FleetRemoteConfigurationModel `tfsdk:"remote_configuration"`
}

func fleetConfigurationGroupSchemaV0() schema.Schema {
	return schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Computed: true},
			"name":           schema.StringAttribute{Required: true},
			"description":    schema.StringAttribute{Optional: true},
			"tags":           schema.ListAttribute{Optional: true, ElementType: types.StringType},
			"priority_order": schema.Int64Attribute{Optional: true, Computed: true},
			"family": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"id":                schema.StringAttribute{Computed: true},
					"version":           schema.StringAttribute{Computed: true},
					"active":            schema.BoolAttribute{Optional: true, Computed: true},
					"description":       schema.StringAttribute{Optional: true},
					"collector_version": schema.StringAttribute{Optional: true, Computed: true},
					"metadata":          schema.MapAttribute{Optional: true, ElementType: types.StringType},
					"remote_configuration": schema.ListNestedAttribute{
						Required: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"id":                schema.StringAttribute{Computed: true},
								"hash":              schema.StringAttribute{Computed: true},
								"name":              schema.StringAttribute{Required: true},
								"raw_configuration": schema.StringAttribute{Required: true},
								"agent_selector":    schema.MapAttribute{Optional: true, ElementType: types.StringType},
							},
						},
					},
				},
			},
		},
	}
}

// upgradeFleetConfigurationGroupStateV0 moves the v0 family fields under
// family.raw. Every v0 family was defined by its remote configurations, and the
// moved values keep their types, so they carry over unchanged and in order.
func upgradeFleetConfigurationGroupStateV0(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	var prior fleetConfigurationGroupResourceModelV0
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	upgraded := FleetConfigurationGroupResourceModel{
		ID:            prior.ID,
		Name:          prior.Name,
		Description:   prior.Description,
		Tags:          prior.Tags,
		PriorityOrder: prior.PriorityOrder,
	}
	if prior.Family != nil {
		upgraded.Family = &FleetConfigurationGroupFamilyModel{
			ID:          prior.Family.ID,
			Version:     prior.Family.Version,
			Active:      prior.Family.Active,
			Description: prior.Family.Description,
			Raw: &FleetRawFamilyModel{
				CollectorVersion:     prior.Family.CollectorVersion,
				Metadata:             prior.Family.Metadata,
				RemoteConfigurations: prior.Family.RemoteConfigurations,
			},
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, upgraded)...)
}
