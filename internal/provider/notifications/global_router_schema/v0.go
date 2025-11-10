// Copyright 2025 Coralogix Ltd.
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

package globalrouterschema

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func V0() schema.Schema {
	return schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "The ID of the GlobalRouter.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the GlobalRouter.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description of the GlobalRouter.",
			},
			"entity_type": schema.StringAttribute{
				Optional: true,
				Computed: true,
				// Validators: []validator.String{
				// 	stringvalidator.OneOf(validNotificationsEntityTypes...),
				// },
				// Description: "Type of the entity. Valid values are: " + strings.Join(validNotificationsEntityTypes, ", "),
			},
			"rules": schema.ListNestedAttribute{
				Optional:    true,
				Description: "Routing rules for the GlobalRouter.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"condition": schema.StringAttribute{
							Required: true,
						},
						"targets": schema.ListNestedAttribute{
							Optional:    true,
							Description: "Routing targets for the rule.",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"connector_id": schema.StringAttribute{
										Required:    true,
										Description: "ID of the connector.",
									},
									"preset_id": schema.StringAttribute{
										Optional:    true,
										Description: "ID of the preset.",
									},
									"custom_details": schema.MapAttribute{
										Optional:    true,
										ElementType: types.StringType,
										Description: "Custom details for the target.",
									},
								},
							},
						},
						"custom_details": schema.MapAttribute{
							Optional:    true,
							ElementType: types.StringType,
							Description: "Custom details for the rule.",
						},
						"name": schema.StringAttribute{
							Required:    true,
							Description: "Name of the routing rule.",
						},
					},
				},
			},
			"fallback": schema.ListNestedAttribute{
				Optional:    true,
				Description: "Fallback routing targets.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"connector_id": schema.StringAttribute{
							Required:    true,
							Description: "ID of the connector.",
						},
						"preset_id": schema.StringAttribute{
							Optional:    true,
							Description: "ID of the preset.",
						},
						"custom_details": schema.MapAttribute{
							Optional:    true,
							ElementType: types.StringType,
							Description: "Custom details for the target.",
						},
					},
				},
			},
			"entity_labels": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
			},
		},
		MarkdownDescription: "Coralogix GlobalRouter. **Note:** This resource is in alpha stage.",
	}
}
