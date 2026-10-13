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

package views

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/view"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// viewQuickSelectionSeconds are the quick time ranges that the views service stores. It stores
// any other value as the nearest one, so Terraform would read back another value than it planned.
var viewQuickSelectionSeconds = []int64{60, 120, 300, 900, 1800, 3600, 7200, 21600, 43200, 86400, 172800, 259200, 432000, 604800}

// NewViewResource returns the generated coralogix_view with the checks of values that the views
// service changes on write. The API contract cannot state them without a breaking change.
func NewViewResource() resource.Resource {
	return &viewResource{Resource: view.NewResource(view.Hooks{}).(*view.Resource)}
}

type viewResource struct {
	*view.Resource
}

var _ resource.ResourceWithConfigValidators = (*viewResource)(nil)

func (r *viewResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return append(r.Resource.ConfigValidators(ctx), viewStoredValues{})
}

// viewStoredValues rejects values that the views service would store differently: a quick
// selection that is not a preset, and a selected filter value that is false (the service keeps
// only true values).
type viewStoredValues struct{}

func (viewStoredValues) Description(context.Context) string {
	return "quick_selection.seconds is a stored preset, and selected_values holds only true values"
}

func (v viewStoredValues) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (viewStoredValues) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	secondsPath := path.Root("time_selection").AtName("quick_selection").AtName("seconds")
	var seconds types.Int64
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, secondsPath, &seconds)...)
	if !seconds.IsNull() && !seconds.IsUnknown() && !slices.Contains(viewQuickSelectionSeconds, seconds.ValueInt64()) {
		presets := make([]string, len(viewQuickSelectionSeconds))
		for i, s := range viewQuickSelectionSeconds {
			presets[i] = fmt.Sprint(s)
		}
		resp.Diagnostics.AddAttributeError(secondsPath, "Unsupported quick selection",
			fmt.Sprintf("The views service stores %d as the nearest preset. Use one of: %s.", seconds.ValueInt64(), strings.Join(presets, ", ")))
	}

	filtersPath := path.Root("filters").AtName("filters")
	var filters types.List
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, filtersPath, &filters)...)
	if filters.IsNull() || filters.IsUnknown() {
		return
	}
	for i, elem := range filters.Elements() {
		filter, ok := elem.(types.Object)
		if !ok || filter.IsNull() || filter.IsUnknown() {
			continue
		}
		values, ok := filter.Attributes()["selected_values"].(types.Map)
		if !ok || values.IsNull() || values.IsUnknown() {
			continue
		}
		for key, value := range values.Elements() {
			if b, ok := value.(types.Bool); ok && !b.IsNull() && !b.IsUnknown() && !b.ValueBool() {
				resp.Diagnostics.AddAttributeError(filtersPath.AtListIndex(i).AtName("selected_values").AtMapKey(key), "Unsupported selected value",
					"The views service keeps only the values set to true. Remove the entry instead of setting it to false.")
			}
		}
	}
}
