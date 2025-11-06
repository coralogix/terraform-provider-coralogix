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
	globalRouters "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/global_routers_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	GlobalRouterEntityTypeSchemaToApi = map[string]globalRouters.NotificationCenterEntityType{
		"unspecified":        globalRouters.NOTIFICATIONCENTERENTITYTYPE_ENTITY_TYPE_UNSPECIFIED,
		"alerts":             globalRouters.NOTIFICATIONCENTERENTITYTYPE_ALERTS,
		"cases":              globalRouters.NOTIFICATIONCENTERENTITYTYPE_CASES,
		"test_notifications": globalRouters.NOTIFICATIONCENTERENTITYTYPE_TEST_NOTIFICATIONS,
	}
	GlobalRouterNotificationCenterEntityTypeApiToSchema       = utils.ReverseMap(GlobalRouterEntityTypeSchemaToApi)
	GlobalRouterValidNotificationCenterEntityTypesSchemaToApi = utils.GetKeys(GlobalRouterEntityTypeSchemaToApi)
)

func V1() schema.Schema {
	return schema.Schema{
		Version: 1,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "The ID of the GlobalRouter. Use `router_default` for the default; leave empty for auto generated or provide your own (unique) id.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the GlobalRouter.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Description of the GlobalRouter.",
			},
			"matching_routing_labels": schema.MapAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Routers other than `router_default` require at least one of the following keys to be set: `routing.environment`, `routing.service`, `routing.group`",
			},
			"rules": schema.ListNestedAttribute{
				Optional:    true,
				Description: "Routing rules for the GlobalRouter.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"entity_type": schema.StringAttribute{
							Optional:      true,
							Computed:      true,
							PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
							Validators: []validator.String{
								stringvalidator.OneOf(GlobalRouterValidNotificationCenterEntityTypesSchemaToApi...),
							}},
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
		MarkdownDescription: "Coralogix GlobalRouter. **Note:** This resource is in Beta stage.",
	}
}
