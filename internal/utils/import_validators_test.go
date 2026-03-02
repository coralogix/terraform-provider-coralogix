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

package utils_test

import (
	"context"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// makeConfig builds a tfsdk.Config from a flat map of string values and a schema.
func makeConfig(t *testing.T, s schema.Schema, attrTypes map[string]tftypes.Type, vals map[string]tftypes.Value) tfsdk.Config {
	t.Helper()
	raw := tftypes.NewValue(tftypes.Object{AttributeTypes: attrTypes}, vals)
	return tfsdk.Config{Raw: raw, Schema: s}
}

// TestRequiredForCreateValidator_SkipsOnImport documents the known limitation of
// the LEGACY ConfigValidator approach: it CANNOT distinguish an import path from
// a create path because it has no access to prior state.  It will fire even when
// id is set in config.  New resources should use RequiredOnCreate (ModifyPlan)
// instead — see TestRequiredOnCreate_SkipsOnImport.
func TestRequiredForCreateValidator_SkipsOnImport(t *testing.T) {
	s := schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{Computed: true},
			"name": schema.StringAttribute{Optional: true, Computed: true},
		},
	}
	attrTypes := map[string]tftypes.Type{
		"id":   tftypes.String,
		"name": tftypes.String,
	}
	vals := map[string]tftypes.Value{
		"id":   tftypes.NewValue(tftypes.String, "some-uuid"), // id is set → import path
		"name": tftypes.NewValue(tftypes.String, nil),         // null
	}

	cfg := makeConfig(t, s, attrTypes, vals)
	req := resource.ValidateConfigRequest{Config: cfg}
	resp := &resource.ValidateConfigResponse{}

	v := utils.RequiredForCreateValidator(path.Root("name"))
	v.ValidateResource(context.Background(), req, resp)

	// Legacy ConfigValidators CANNOT skip on import — they always fire.
	// This is the known limitation documented in RequiredForCreateValidator's docstring.
	if !resp.Diagnostics.HasError() {
		t.Error("expected legacy validator to fire even on import path (known limitation)")
	}
}

// TestRequiredForCreateValidator_ErrorsOnCreate verifies that the validator fires
// when id is null (create path) and the required field is missing.
func TestRequiredForCreateValidator_ErrorsOnCreate(t *testing.T) {
	s := schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{Computed: true},
			"name": schema.StringAttribute{Optional: true, Computed: true},
		},
	}
	attrTypes := map[string]tftypes.Type{
		"id":   tftypes.String,
		"name": tftypes.String,
	}
	vals := map[string]tftypes.Value{
		"id":   tftypes.NewValue(tftypes.String, nil), // null → create path
		"name": tftypes.NewValue(tftypes.String, nil), // null → missing
	}

	cfg := makeConfig(t, s, attrTypes, vals)
	req := resource.ValidateConfigRequest{Config: cfg}
	resp := &resource.ValidateConfigResponse{}

	v := utils.RequiredForCreateValidator(path.Root("name"))
	v.ValidateResource(context.Background(), req, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("expected validation error on create path with missing required field, got none")
	}
}

// TestRequiredForCreateValidator_PassesOnCreate verifies that the validator does NOT
// fire when id is null (create path) but the required field is set.
func TestRequiredForCreateValidator_PassesOnCreate(t *testing.T) {
	s := schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":   schema.StringAttribute{Computed: true},
			"name": schema.StringAttribute{Optional: true, Computed: true},
		},
	}
	attrTypes := map[string]tftypes.Type{
		"id":   tftypes.String,
		"name": tftypes.String,
	}
	vals := map[string]tftypes.Value{
		"id":   tftypes.NewValue(tftypes.String, nil),    // null → create path
		"name": tftypes.NewValue(tftypes.String, "test"), // set
	}

	cfg := makeConfig(t, s, attrTypes, vals)
	req := resource.ValidateConfigRequest{Config: cfg}
	resp := &resource.ValidateConfigResponse{}

	v := utils.RequiredForCreateValidator(path.Root("name"))
	v.ValidateResource(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("expected no errors when required field is set on create, got: %v", resp.Diagnostics)
	}
}

// TestExactlyOneOfForCreateValidator_SkipsOnImport documents the known limitation of
// the LEGACY ExactlyOneOfForCreateValidator: it CANNOT skip on import because
// ConfigValidators have no access to prior state.  New resources should use
// ExactlyOneOfOnCreate (ModifyPlan) instead.
func TestExactlyOneOfForCreateValidator_SkipsOnImport(t *testing.T) {
	objType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}
	s := schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":     schema.StringAttribute{Computed: true},
			"type_a": schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{}},
			"type_b": schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{}},
		},
	}
	attrTypes := map[string]tftypes.Type{
		"id":     tftypes.String,
		"type_a": objType,
		"type_b": objType,
	}
	vals := map[string]tftypes.Value{
		"id":     tftypes.NewValue(tftypes.String, "some-uuid"), // id set → import path
		"type_a": tftypes.NewValue(objType, nil),                // null
		"type_b": tftypes.NewValue(objType, nil),                // null
	}

	cfg := makeConfig(t, s, attrTypes, vals)
	req := resource.ValidateConfigRequest{Config: cfg}
	resp := &resource.ValidateConfigResponse{}

	v := utils.ExactlyOneOfForCreateValidator(path.Root("type_a"), path.Root("type_b"))
	v.ValidateResource(context.Background(), req, resp)

	// Legacy ConfigValidators CANNOT skip on import — they always fire.
	// This is the known limitation documented in ExactlyOneOfForCreateValidator's docstring.
	if !resp.Diagnostics.HasError() {
		t.Error("expected legacy validator to fire even on import path (known limitation)")
	}
}

// TestExactlyOneOfForCreateValidator_ErrorsOnCreate verifies that the validator fires
// when no type block is set on a create path.
func TestExactlyOneOfForCreateValidator_ErrorsOnCreate(t *testing.T) {
	objType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}
	s := schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":     schema.StringAttribute{Computed: true},
			"type_a": schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{}},
			"type_b": schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{}},
		},
	}
	attrTypes := map[string]tftypes.Type{
		"id":     tftypes.String,
		"type_a": objType,
		"type_b": objType,
	}
	vals := map[string]tftypes.Value{
		"id":     tftypes.NewValue(tftypes.String, nil), // null → create path
		"type_a": tftypes.NewValue(objType, nil),        // null
		"type_b": tftypes.NewValue(objType, nil),        // null
	}

	cfg := makeConfig(t, s, attrTypes, vals)
	req := resource.ValidateConfigRequest{Config: cfg}
	resp := &resource.ValidateConfigResponse{}

	v := utils.ExactlyOneOfForCreateValidator(path.Root("type_a"), path.Root("type_b"))
	v.ValidateResource(context.Background(), req, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("expected validation error on create path with no type block set, got none")
	}
}

// TestExactlyOneOfForCreateValidator_ErrorsOnMultiple verifies that setting more than
// one type block on create also produces an error.
func TestExactlyOneOfForCreateValidator_ErrorsOnMultiple(t *testing.T) {
	objType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}
	s := schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":     schema.StringAttribute{Computed: true},
			"type_a": schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{}},
			"type_b": schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{}},
		},
	}
	attrTypes := map[string]tftypes.Type{
		"id":     tftypes.String,
		"type_a": objType,
		"type_b": objType,
	}
	vals := map[string]tftypes.Value{
		"id":     tftypes.NewValue(tftypes.String, nil),
		"type_a": tftypes.NewValue(objType, map[string]tftypes.Value{}), // set
		"type_b": tftypes.NewValue(objType, map[string]tftypes.Value{}), // set
	}

	cfg := makeConfig(t, s, attrTypes, vals)
	req := resource.ValidateConfigRequest{Config: cfg}
	resp := &resource.ValidateConfigResponse{}

	v := utils.ExactlyOneOfForCreateValidator(path.Root("type_a"), path.Root("type_b"))
	v.ValidateResource(context.Background(), req, resp)

	if !resp.Diagnostics.HasError() {
		t.Error("expected validation error when multiple type blocks are set, got none")
	}
}

// TestExactlyOneOfForCreateValidator_PassesOnCreate verifies that exactly one type block
// set on create passes validation.
func TestExactlyOneOfForCreateValidator_PassesOnCreate(t *testing.T) {
	objType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{}}
	s := schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":     schema.StringAttribute{Computed: true},
			"type_a": schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{}},
			"type_b": schema.SingleNestedAttribute{Optional: true, Attributes: map[string]schema.Attribute{}},
		},
	}
	attrTypes := map[string]tftypes.Type{
		"id":     tftypes.String,
		"type_a": objType,
		"type_b": objType,
	}
	vals := map[string]tftypes.Value{
		"id":     tftypes.NewValue(tftypes.String, nil),
		"type_a": tftypes.NewValue(objType, map[string]tftypes.Value{}), // set
		"type_b": tftypes.NewValue(objType, nil),                        // null
	}

	cfg := makeConfig(t, s, attrTypes, vals)
	req := resource.ValidateConfigRequest{Config: cfg}
	resp := &resource.ValidateConfigResponse{}

	v := utils.ExactlyOneOfForCreateValidator(path.Root("type_a"), path.Root("type_b"))
	v.ValidateResource(context.Background(), req, resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("expected no errors when exactly one type block is set on create, got: %v", resp.Diagnostics)
	}
}
