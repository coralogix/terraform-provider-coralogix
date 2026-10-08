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

package notifications

import (
	"context"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/preset"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &PresetResource{}
	_ resource.ResourceWithImportState = &PresetResource{}
	_ resource.ResourceWithConfigure   = &PresetResource{}
)

func NewPresetResource() resource.Resource {
	return &PresetResource{Resource: preset.NewResource(preset.Hooks{
		AfterRead: emptyConfigOverrides,
	}).(*preset.Resource)}
}

// emptyConfigOverrides stores a missing override list as an empty list. The
// released resource did that, and an import compares against it.
func emptyConfigOverrides(ctx context.Context, state *tfsdk.State, _ any) diag.Diagnostics {
	var model preset.PresetModel
	diags := state.Get(ctx, &model)
	if diags.HasError() {
		return diags
	}
	if !model.ConfigOverrides.IsNull() {
		return nil
	}
	model.ConfigOverrides = types.ListValueMust(model.ConfigOverrides.ElementType(ctx), []attr.Value{})
	return state.Set(ctx, &model)
}

func normalizeConfigOverrides(ctx context.Context, model *preset.PresetModel) {
	if model == nil || !model.ConfigOverrides.IsNull() {
		return
	}
	model.ConfigOverrides = types.ListValueMust(model.ConfigOverrides.ElementType(ctx), []attr.Value{})
}

// PresetResource is the generated preset. Changing id recreates the resource;
// the generator does not add that modifier.
type PresetResource struct {
	*preset.Resource
}

func (r *PresetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	s := preset.Schema()
	if idAttr, ok := s.Attributes["id"].(schema.StringAttribute); ok {
		idAttr.PlanModifiers = []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
			stringplanmodifier.RequiresReplace(),
		}
		s.Attributes["id"] = idAttr
	}
	resp.Schema = s
}
