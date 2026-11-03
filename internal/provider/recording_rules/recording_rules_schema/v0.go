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

package recordingrulesschema

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// V0 is the schema of version 0 of coralogix_recording_rules_groups_set: `group` and `rule`
// instead of `groups` and `rules`, and a rule with only record, expr, and labels. State of this
// version is upgraded by reading the set from the API; yaml_content keeps its value.
func V0() schema.Schema {
	return schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"yaml_content": schema.StringAttribute{
				Optional: true,
			},
			"group": schema.SetNestedAttribute{
				Optional: true,
				Computed: true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required: true,
						},
						"interval": schema.Int64Attribute{
							Required: true,
						},
						"limit": schema.Int64Attribute{
							Optional: true,
						},
						"rule": schema.SetNestedAttribute{
							Required: true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"record": schema.StringAttribute{
										Required: true,
									},
									"expr": schema.StringAttribute{
										Required: true,
									},
									"labels": schema.MapAttribute{
										ElementType: types.StringType,
										Optional:    true,
									},
								},
							},
						},
					},
				},
			},
			"name": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
		},
	}
}
