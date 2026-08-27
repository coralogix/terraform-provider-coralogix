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

package dashboard_widgets

import (
	"context"
	"strings"
	"testing"

	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// These attributes are sent and read back without conversion, so the only safe
// values are the ones the API returns unchanged. Every accepted value below was
// applied against a live environment and came back verbatim; every rejected one
// was rewritten by the API, which Terraform then reports as a change forever.
func TestWireDurationAcceptsOnlyWhatTheApiReturns(t *testing.T) {
	for value, wantError := range map[string]string{
		"900s":         "",
		"15s":          "",
		"0s":           "",
		"86400s":       "",
		"1.500s":       "",
		"0.250s":       "",
		"1.000000001s": "",
		"1.5s":         `stored by the API as "1.500s"`,
		"1.0s":         `stored by the API as "1s"`,
		"900.000s":     `stored by the API as "900s"`,
		"15m":          "must be a duration in seconds",
		"5m0s":         "must be a duration in seconds",
		"seconds:900":  "must be a duration in seconds",
		"":             "must be a duration in seconds",
	} {
		t.Run(value, func(t *testing.T) {
			response := validator.StringResponse{}
			wireDurationValidator{}.ValidateString(context.Background(), validator.StringRequest{
				Path:        path.Root("minimum_interval"),
				ConfigValue: types.StringValue(value),
			}, &response)

			switch {
			case wantError == "" && response.Diagnostics.HasError():
				t.Errorf("%q must be accepted, got %v", value, response.Diagnostics)
			case wantError != "" && !response.Diagnostics.HasError():
				t.Errorf("%q must be rejected", value)
			case wantError != "" && !strings.Contains(response.Diagnostics.Errors()[0].Detail(), wantError):
				t.Errorf("%q must be rejected with %q, got %q", value, wantError, response.Diagnostics.Errors()[0].Detail())
			}
		})
	}

	// A value that has not resolved yet is not a mistake, and rejecting an
	// unset attribute would make the whole block unusable.
	t.Run("null and unknown pass through", func(t *testing.T) {
		for name, value := range map[string]types.String{
			"null":    types.StringNull(),
			"unknown": types.StringUnknown(),
		} {
			response := validator.StringResponse{}
			wireDurationValidator{}.ValidateString(context.Background(), validator.StringRequest{
				Path: path.Root("minimum_interval"), ConfigValue: value,
			}, &response)
			if response.Diagnostics.HasError() {
				t.Errorf("a %s value must pass through, got %v", name, response.Diagnostics)
			}
		}
	})
}

// AtMostOneOfChildren exists because the API accepts neither arm: requiring
// exactly one would reject a resolution the API stores and returns.
func TestAtMostOneOfChildrenAllowsNoneButNotTwo(t *testing.T) {
	object := func(auto, manual attr.Value) types.Object {
		return types.ObjectValueMust(
			map[string]attr.Type{"auto": types.StringType, "manual": types.StringType},
			map[string]attr.Value{"auto": auto, "manual": manual},
		)
	}

	for scenario, testCase := range map[string]struct {
		value     types.Object
		wantError bool
	}{
		"neither set":     {object(types.StringNull(), types.StringNull()), false},
		"only auto":       {object(types.StringValue("a"), types.StringNull()), false},
		"only manual":     {object(types.StringNull(), types.StringValue("m")), false},
		"both set":        {object(types.StringValue("a"), types.StringValue("m")), true},
		"one unknown":     {object(types.StringUnknown(), types.StringNull()), false},
		"set and unknown": {object(types.StringValue("a"), types.StringUnknown()), false},
	} {
		t.Run(scenario, func(t *testing.T) {
			response := validator.ObjectResponse{}
			AtMostOneOfChildren("auto", "manual").ValidateObject(context.Background(), validator.ObjectRequest{
				Path: path.Root("time_buckets"), ConfigValue: testCase.value,
			}, &response)
			if got := response.Diagnostics.HasError(); got != testCase.wantError {
				t.Errorf("expected error=%v, got %v", testCase.wantError, response.Diagnostics)
			}
		})
	}
}

// Omitting both modes is a documented way to leave the choice to the backend, so
// `time_buckets = {}` is a configuration a user can write and the API stores it.
// Dropping it on read made the x-axis vanish, and the apply then failed with an
// inconsistent result. Only an absent resolution reads back as absent.
func TestFlattenIntervalResolutionKeepsAResolutionWithNoMode(t *testing.T) {
	if got := FlattenIntervalResolution(nil); got != nil {
		t.Errorf("an absent resolution must stay absent, got %v", got)
	}

	got := FlattenIntervalResolution(&dashboardservice.IntervalResolution{})
	switch {
	case got == nil:
		t.Fatal("a resolution with no mode must be kept, or `time_buckets = {}` cannot round-trip")
	case got.Auto != nil || got.Manual != nil:
		t.Errorf("no mode was stored, so none may be set, got %v", got)
	case !got.UseAdvancedLimit.IsNull():
		t.Errorf("no advanced limit was stored, so it must stay null, got %v", got.UseAdvancedLimit)
	}

	// A resolution carrying only the advanced limit is a real stored shape and
	// must survive, or the check above would be passing on a dropped field.
	advanced := true
	if got := FlattenIntervalResolution(&dashboardservice.IntervalResolution{UseAdvancedLimit: &advanced}); got == nil || !got.UseAdvancedLimit.ValueBool() {
		t.Errorf("use_advanced_limit alone must read back, got %v", got)
	}
}

// The union is re-checked here because AtMostOneOfChildren returns early while
// either arm is unknown, and the API rejects both arms when the request is
// marshalled, naming generated JSON fields rather than any HCL path.
func TestExpandIntervalResolutionRejectsBothModes(t *testing.T) {
	_, diags := ExpandIntervalResolution(&IntervalResolutionModel{
		Auto:   &AutoIntervalResolutionModel{},
		Manual: &ManualIntervalResolutionModel{Interval: types.StringValue("900s")},
	})
	if !diags.HasError() {
		t.Fatal("both modes together must be rejected")
	}

	resolution, diags := ExpandIntervalResolution(&IntervalResolutionModel{
		Auto: &AutoIntervalResolutionModel{MinimumInterval: types.StringValue("15s")},
	})
	if diags.HasError() || resolution == nil || resolution.Auto == nil || resolution.Manual != nil {
		t.Errorf("auto alone must convert, got %v %v", resolution, diags)
	}
}
