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

package utils

// Import-aware plan-time validation for the Coralogix Terraform provider.
//
// # Problem
//
// schema-level Required/ExactlyOneOf validators run at plan time on the HCL config
// alone — they have no access to Terraform state. This means they fire for BOTH:
//   - New creates (desired: fire with clear error)
//   - Plans after `terraform import` or `import {}` blocks (desired: skip)
//
// ConfigValidators have the same limitation: they receive only the config, not state.
//
// # Solution
//
// Use resource.ResourceWithModifyPlan, which receives the prior state alongside the
// config. The prior state is:
//   - null  → new resource creation (validate required fields)
//   - non-null → update or post-import plan (skip — resource already exists)
//
// For `import {}` blocks, Terraform reads the resource from the API before calling
// ModifyPlan, so req.State is populated with the live resource data.
//
// # Usage
//
// In your resource, implement resource.ResourceWithModifyPlan:
//
//	func (r *MyResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
//	    utils.RequiredOnCreate(ctx, req, resp, path.Root("name"), path.Root("url"))
//	    utils.ExactlyOneOfOnCreate(ctx, req, resp,
//	        path.Root("type_a"),
//	        path.Root("type_b"),
//	    )
//	}

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// IsNewResource returns true when ModifyPlan is running for a new resource
// (no prior state exists). Returns false for updates, post-import plans, and destroys.
func IsNewResource(req resource.ModifyPlanRequest) bool {
	// Destroy plans have a null planned state — not a create.
	if req.Plan.Raw.IsNull() {
		return false
	}
	// Creates have no prior state.
	return req.State.Raw.IsNull()
}

// isNewResource is the unexported alias kept for internal use within this package.
func isNewResource(req resource.ModifyPlanRequest) bool {
	return IsNewResource(req)
}

// RequiredOnCreate adds plan-time errors when the given string attribute paths
// are null or unknown during a new resource creation. Skips for updates and
// post-import plans where the resource already exists in state.
func RequiredOnCreate(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse, paths ...path.Path) {
	if !isNewResource(req) {
		return
	}
	for _, p := range paths {
		var val types.String
		diags := req.Config.GetAttribute(ctx, p, &val)
		if diags.HasError() || val.IsNull() || val.IsUnknown() {
			resp.Diagnostics.AddAttributeError(
				p,
				"Missing required attribute",
				fmt.Sprintf("The attribute %q is required when creating this resource. It may be omitted when importing.", p),
			)
		}
	}
}

// RequiredAttributeOnCreate adds plan-time errors when the given attribute paths
// (any type — string, number, list, object, etc.) are null or unknown during a
// new resource creation. Skips for updates and post-import plans.
func RequiredAttributeOnCreate(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse, paths ...path.Path) {
	if !isNewResource(req) {
		return
	}
	for _, p := range paths {
		if isConfigAttrNullOrUnknown(ctx, req, p) {
			resp.Diagnostics.AddAttributeError(
				p,
				"Missing required attribute",
				fmt.Sprintf("The attribute %q is required when creating this resource. It may be omitted when importing.", p),
			)
		}
	}
}

// isConfigAttrNullOrUnknown returns true if the attribute at path p is null or
// unknown in the plan config.  It tries the most common concrete types in order
// so it works regardless of the attribute's schema type (String, Float32/64,
// Int64, Bool, List, Set, Object, Map).  If no type matches, it conservatively
// returns false (do not block the plan).
func isConfigAttrNullOrUnknown(ctx context.Context, req resource.ModifyPlanRequest, p path.Path) bool {
	// Object / nested block
	var obj types.Object
	if diags := req.Config.GetAttribute(ctx, p, &obj); !diags.HasError() {
		return obj.IsNull() || obj.IsUnknown()
	}
	// List
	var lst types.List
	if diags := req.Config.GetAttribute(ctx, p, &lst); !diags.HasError() {
		return lst.IsNull() || lst.IsUnknown()
	}
	// Set
	var set types.Set
	if diags := req.Config.GetAttribute(ctx, p, &set); !diags.HasError() {
		return set.IsNull() || set.IsUnknown()
	}
	// String
	var str types.String
	if diags := req.Config.GetAttribute(ctx, p, &str); !diags.HasError() {
		return str.IsNull() || str.IsUnknown()
	}
	// Float64
	var f64 types.Float64
	if diags := req.Config.GetAttribute(ctx, p, &f64); !diags.HasError() {
		return f64.IsNull() || f64.IsUnknown()
	}
	// Float32
	var f32 types.Float32
	if diags := req.Config.GetAttribute(ctx, p, &f32); !diags.HasError() {
		return f32.IsNull() || f32.IsUnknown()
	}
	// Int64
	var i64 types.Int64
	if diags := req.Config.GetAttribute(ctx, p, &i64); !diags.HasError() {
		return i64.IsNull() || i64.IsUnknown()
	}
	// Bool
	var b types.Bool
	if diags := req.Config.GetAttribute(ctx, p, &b); !diags.HasError() {
		return b.IsNull() || b.IsUnknown()
	}
	// Map
	var m types.Map
	if diags := req.Config.GetAttribute(ctx, p, &m); !diags.HasError() {
		return m.IsNull() || m.IsUnknown()
	}
	// Unknown type — be conservative, do not block
	return false
}

// ExactlyOneOfOnCreate adds a plan-time error when none or more than one of the
// given attribute paths are non-null during a new resource creation.
// Skips for updates and post-import plans.
func ExactlyOneOfOnCreate(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse, paths ...path.Path) {
	if !isNewResource(req) {
		return
	}

	setCount := 0
	var setPaths []string
	allPaths := make([]string, len(paths))

	for i, p := range paths {
		allPaths[i] = p.String()
		if !isConfigAttrNullOrUnknown(ctx, req, p) {
			setCount++
			setPaths = append(setPaths, p.String())
		}
	}

	if setCount == 0 {
		resp.Diagnostics.AddError(
			"Missing required attribute",
			fmt.Sprintf(
				"Exactly one of [%s] must be set when creating this resource, but none were set. It may be omitted when importing.",
				strings.Join(allPaths, ", "),
			),
		)
	} else if setCount > 1 {
		resp.Diagnostics.AddError(
			"Conflicting attributes",
			fmt.Sprintf(
				"Exactly one of [%s] must be set when creating this resource, but %d were set: [%s].",
				strings.Join(allPaths, ", "),
				setCount,
				strings.Join(setPaths, ", "),
			),
		)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Legacy ConfigValidator types — kept for unit tests and backwards compatibility.
// New resources should use the ModifyPlan helpers above instead.
// ─────────────────────────────────────────────────────────────────────────────

// RequiredForCreateValidator returns a ConfigValidator. Prefer RequiredOnCreate
// (ModifyPlan-based) for new resources — ConfigValidators cannot see prior state
// and cannot distinguish creates from post-import plans.
func RequiredForCreateValidator(paths ...path.Path) resource.ConfigValidator {
	return &requiredForCreateValidator{paths: paths}
}

type requiredForCreateValidator struct {
	paths []path.Path
}

func (v *requiredForCreateValidator) Description(_ context.Context) string {
	strs := make([]string, len(v.paths))
	for i, p := range v.paths {
		strs[i] = p.String()
	}
	return fmt.Sprintf("Attributes %s are required when creating or updating this resource.", strings.Join(strs, ", "))
}

func (v *requiredForCreateValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v *requiredForCreateValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	for _, p := range v.paths {
		var val types.String
		diags := req.Config.GetAttribute(ctx, p, &val)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			continue
		}
		if val.IsNull() || val.IsUnknown() {
			resp.Diagnostics.AddAttributeError(
				p,
				"Missing required attribute",
				fmt.Sprintf("The attribute %q is required when creating or updating this resource.", p),
			)
		}
	}
}

// RequiredAttributeForCreateValidator returns a ConfigValidator. Prefer
// RequiredAttributeOnCreate (ModifyPlan-based) for new resources.
func RequiredAttributeForCreateValidator(paths ...path.Path) resource.ConfigValidator {
	return &requiredAttributeForCreateValidator{paths: paths}
}

type requiredAttributeForCreateValidator struct {
	paths []path.Path
}

func (v *requiredAttributeForCreateValidator) Description(_ context.Context) string {
	strs := make([]string, len(v.paths))
	for i, p := range v.paths {
		strs[i] = p.String()
	}
	return fmt.Sprintf("Attributes %s are required when creating or updating this resource.", strings.Join(strs, ", "))
}

func (v *requiredAttributeForCreateValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v *requiredAttributeForCreateValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	for _, p := range v.paths {
		var obj types.Object
		diags := req.Config.GetAttribute(ctx, p, &obj)
		if diags.HasError() {
			resp.Diagnostics.AddAttributeError(
				p,
				"Missing required attribute",
				fmt.Sprintf("The attribute %q is required when creating or updating this resource.", p),
			)
			continue
		}
		if obj.IsNull() || obj.IsUnknown() {
			resp.Diagnostics.AddAttributeError(
				p,
				"Missing required attribute",
				fmt.Sprintf("The attribute %q is required when creating or updating this resource.", p),
			)
		}
	}
}

// ExactlyOneOfForCreateValidator returns a ConfigValidator. Prefer
// ExactlyOneOfOnCreate (ModifyPlan-based) for new resources.
func ExactlyOneOfForCreateValidator(paths ...path.Path) resource.ConfigValidator {
	return &exactlyOneOfForCreateValidator{paths: paths}
}

type exactlyOneOfForCreateValidator struct {
	paths []path.Path
}

func (v *exactlyOneOfForCreateValidator) Description(_ context.Context) string {
	strs := make([]string, len(v.paths))
	for i, p := range v.paths {
		strs[i] = p.String()
	}
	return fmt.Sprintf("Exactly one of [%s] must be set when creating or updating this resource.", strings.Join(strs, ", "))
}

func (v *exactlyOneOfForCreateValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v *exactlyOneOfForCreateValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	setCount := 0
	var setPaths []string

	for _, p := range v.paths {
		var obj types.Object
		diags := req.Config.GetAttribute(ctx, p, &obj)
		if diags.HasError() {
			var str types.String
			diags2 := req.Config.GetAttribute(ctx, p, &str)
			if diags2.HasError() {
				continue
			}
			if !str.IsNull() && !str.IsUnknown() {
				setCount++
				setPaths = append(setPaths, p.String())
			}
			continue
		}
		if !obj.IsNull() && !obj.IsUnknown() {
			setCount++
			setPaths = append(setPaths, p.String())
		}
	}

	allPaths := make([]string, len(v.paths))
	for i, p := range v.paths {
		allPaths[i] = p.String()
	}

	if setCount == 0 {
		resp.Diagnostics.AddError(
			"Missing required attribute",
			fmt.Sprintf(
				"Exactly one of [%s] must be set when creating or updating this resource, but none were set.",
				strings.Join(allPaths, ", "),
			),
		)
	} else if setCount > 1 {
		resp.Diagnostics.AddError(
			"Conflicting attributes",
			fmt.Sprintf(
				"Exactly one of [%s] must be set when creating or updating this resource, but %d were set: [%s].",
				strings.Join(allPaths, ", "),
				setCount,
				strings.Join(setPaths, ", "),
			),
		)
	}
}
