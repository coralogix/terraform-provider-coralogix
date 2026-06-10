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

package dashboard_schema

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type DashboardAccessPolicyCanonicalJSONPlanModifier struct{}

func (m DashboardAccessPolicyCanonicalJSONPlanModifier) Description(_ context.Context) string {
	return "canonicalizes dashboard access policy JSON"
}

func (m DashboardAccessPolicyCanonicalJSONPlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m DashboardAccessPolicyCanonicalJSONPlanModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}

	canonical, diags := CanonicalizeDashboardAccessPolicyJSON(req.PlanValue.ValueString())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.PlanValue = types.StringValue(canonical)
}

func CanonicalizeDashboardAccessPolicyJSON(policy string) (string, diag.Diagnostics) {
	var diags diag.Diagnostics

	decoder := json.NewDecoder(strings.NewReader(policy))
	decoder.UseNumber()

	var value any
	if err := decoder.Decode(&value); err != nil {
		diags.Append(diag.NewErrorDiagnostic(
			"Invalid Dashboard Access Policy",
			fmt.Sprintf("access_policy must be valid JSON: %s", err),
		))
		return "", diags
	}

	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		diags.Append(diag.NewErrorDiagnostic(
			"Invalid Dashboard Access Policy",
			"access_policy must contain exactly one JSON value",
		))
		return "", diags
	}

	canonical, err := json.Marshal(value)
	if err != nil {
		diags.Append(diag.NewErrorDiagnostic(
			"Invalid Dashboard Access Policy",
			fmt.Sprintf("access_policy could not be canonicalized: %s", err),
		))
		return "", diags
	}

	return string(canonical), diags
}
