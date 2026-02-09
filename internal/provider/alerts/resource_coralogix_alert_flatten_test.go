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

package alerts

import (
	"context"
	"testing"

	alerts "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/alert_definitions_service"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	alerttypes "github.com/coralogix/terraform-provider-coralogix/internal/provider/alerts/alert_types"
)

func TestFlattenLogsUniqueCount_WithNilMaxUniqueCountPerGroupByKey(t *testing.T) {
	ctx := context.Background()

	// Create LogsUniqueCountType with MaxUniqueCountPerGroupByKey set to nil
	uniqueCount := &alerts.LogsUniqueCountType{
		Rules:                       []alerts.LogsUniqueCountRule{},
		MaxUniqueCountPerGroupByKey: nil, // This is what we're testing
		LogsFilter:                  nil, // Can be nil, will return null object
		NotificationPayloadFilter:   nil,
		UniqueCountKeypath:          nil,
	}

	// This should not panic
	var result types.Object
	var diags diag.Diagnostics
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		result, diags = flattenLogsUniqueCount(ctx, uniqueCount)
	}()

	if panicked {
		t.Fatal("flattenLogsUniqueCount panicked when MaxUniqueCountPerGroupByKey is nil")
	}

	// Verify no errors
	if diags.HasError() {
		t.Fatalf("Expected no diagnostics errors, got: %v", diags)
	}

	// Verify result is not null
	if result.IsNull() {
		t.Error("Expected result to not be null")
	}

	// Verify the object can be read (basic sanity check)
	if result.IsUnknown() {
		t.Error("Expected result to not be unknown")
	}
}

func TestFlattenLogsUniqueCountRuleCondition_WithNilMaxUniqueCount(t *testing.T) {
	ctx := context.Background()

	// Create LogsUniqueCountCondition with MaxUniqueCount set to nil
	condition := &alerts.LogsUniqueCountCondition{
		MaxUniqueCount: nil, // This is what we're testing
		TimeWindow: &alerts.LogsUniqueValueTimeWindow{
			LogsUniqueValueTimeWindowSpecificValue: alerts.LOGSUNIQUEVALUETIMEWINDOWVALUE_LOGS_UNIQUE_VALUE_TIME_WINDOW_VALUE_MINUTE_1_OR_UNSPECIFIED.Ptr(),
		},
	}

	// This should not panic
	var result types.Object
	var diags diag.Diagnostics
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		result, diags = flattenLogsUniqueCountRuleCondition(ctx, condition)
	}()

	if panicked {
		t.Fatal("flattenLogsUniqueCountRuleCondition panicked when MaxUniqueCount is nil")
	}

	// Verify no errors
	if diags.HasError() {
		t.Fatalf("Expected no diagnostics errors, got: %v", diags)
	}

	// Verify result is not null
	if result.IsNull() {
		t.Error("Expected result to not be null")
	}

	// Verify the object can be read (basic sanity check)
	if result.IsUnknown() {
		t.Error("Expected result to not be unknown")
	}
}

func TestFlattenFlowStage_WithNilTimeframeMs(t *testing.T) {
	ctx := context.Background()

	// Create FlowStages with TimeframeMs set to nil
	// Note: flattenFlowStagesGroups requires FlowStagesGroups to be set up properly
	stage := &alerts.FlowStages{
		TimeframeMs:   nil, // This is what we're testing
		TimeframeType: alerts.TIMEFRAMETYPE_TIMEFRAME_TYPE_UNSPECIFIED.Ptr(),
		FlowStagesGroups: &alerts.FlowStagesGroups{
			Groups: []alerts.FlowStagesGroup{}, // Empty groups slice
		},
	}

	// This should not panic
	var result *alerttypes.FlowStageModel
	var diags diag.Diagnostics
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		result, diags = flattenFlowStage(ctx, stage)
	}()

	if panicked {
		t.Fatal("flattenFlowStage panicked when TimeframeMs is nil")
	}

	// Verify no errors
	if diags.HasError() {
		t.Fatalf("Expected no diagnostics errors, got: %v", diags)
	}

	// Verify result is not nil
	if result == nil {
		t.Fatal("Expected result to not be nil")
	}

	// Verify timeframe_ms is set to 0 (default value)
	if result.TimeframeMs.IsNull() {
		t.Error("Expected TimeframeMs to not be null")
	}
	if result.TimeframeMs.IsUnknown() {
		t.Error("Expected TimeframeMs to not be unknown")
	}
	if !result.TimeframeMs.IsNull() && !result.TimeframeMs.IsUnknown() {
		if result.TimeframeMs.ValueInt64() != 0 {
			t.Errorf("Expected TimeframeMs to be 0 when nil, got: %d", result.TimeframeMs.ValueInt64())
		}
	}
}

func TestFlattenSloTimeDuration_WithNilTimeDuration(t *testing.T) {
	ctx := context.Background()

	// Call with nil TimeDuration pointer
	var td *alerts.TimeDuration = nil

	// This should not panic
	var result types.Object
	var diags diag.Diagnostics
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		result, diags = flattenSloTimeDuration(ctx, td)
	}()

	if panicked {
		t.Fatal("flattenSloTimeDuration panicked when TimeDuration is nil")
	}

	// Verify no errors
	if diags.HasError() {
		t.Fatalf("Expected no diagnostics errors, got: %v", diags)
	}

	// Verify result is null (as per the function implementation)
	if !result.IsNull() {
		t.Error("Expected result to be null when TimeDuration is nil")
	}
}

func TestFlattenSloTimeDuration_WithNilDuration(t *testing.T) {
	ctx := context.Background()

	// Create TimeDuration with Duration set to nil
	td := &alerts.TimeDuration{
		Duration: nil, // This is what we're testing
		Unit:     alerts.DURATIONUNIT_DURATION_UNIT_UNSPECIFIED.Ptr(),
	}

	// This should not panic
	var result types.Object
	var diags diag.Diagnostics
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		result, diags = flattenSloTimeDuration(ctx, td)
	}()

	if panicked {
		t.Fatal("flattenSloTimeDuration panicked when Duration is nil")
	}

	// Verify no errors
	if diags.HasError() {
		t.Fatalf("Expected no diagnostics errors, got: %v", diags)
	}

	// Verify result is not null
	if result.IsNull() {
		t.Error("Expected result to not be null")
	}

	// Verify the object can be read (basic sanity check)
	if result.IsUnknown() {
		t.Error("Expected result to not be unknown")
	}
}
