// Copyright 2025 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package globalrouterschema

import (
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func RoutingRuleAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"entity_type":    types.StringType,
		"condition":      types.StringType,
		"name":           types.StringType,
		"targets":        types.ListType{ElemType: types.ObjectType{AttrTypes: RoutingTargetAttr()}},
		"custom_details": types.MapType{ElemType: types.StringType},
	}
}

func RoutingTargetAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"connector_id":   types.StringType,
		"preset_id":      types.StringType,
		"custom_details": types.MapType{ElemType: types.StringType},
	}
}

func MessageConfigFieldAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field_name": types.StringType,
		"template":   types.StringType,
	}
}

func ConfigOverridesAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"field_name": types.StringType,
		"template":   types.StringType,
	}
}
