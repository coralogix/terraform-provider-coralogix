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

package alertschema

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestAnalyticsNoDataPolicyNotEmpty guards the validator that rejects an empty
// no_data_policy block. Without it, `no_data_policy = {}` round-trips as null and
// fails apply with "inconsistent result after apply".
func TestAnalyticsNoDataPolicyNotEmpty(t *testing.T) {
	ctx := context.Background()
	obj := func(state, autoRetire attr.Value) types.Object {
		return types.ObjectValueMust(NoDataPolicyAttr(), map[string]attr.Value{
			"state":               state,
			"auto_retire_seconds": autoRetire,
		})
	}

	cases := []struct {
		name      string
		value     types.Object
		wantError bool
	}{
		{"null block is fine", types.ObjectNull(NoDataPolicyAttr()), false},
		{"unknown block is fine", types.ObjectUnknown(NoDataPolicyAttr()), false},
		{"empty block is rejected", obj(types.StringNull(), types.Int64Null()), true},
		{"state set is fine", obj(types.StringValue("ALERTING"), types.Int64Null()), false},
		{"auto_retire set is fine", obj(types.StringNull(), types.Int64Value(3600)), false},
		{"unknown child is fine", obj(types.StringUnknown(), types.Int64Null()), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := &validator.ObjectResponse{}
			analyticsNoDataPolicyNotEmpty{}.ValidateObject(ctx,
				validator.ObjectRequest{ConfigValue: tc.value}, resp)
			if got := resp.Diagnostics.HasError(); got != tc.wantError {
				t.Errorf("HasError() = %v, want %v (%v)", got, tc.wantError, resp.Diagnostics)
			}
		})
	}
}
