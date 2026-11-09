// Copyright 2026 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package alerts

import (
	"context"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	alertapi "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/alert_definitions_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"
)

// getAlert reads the alert with include_access_permissions, so the response carries the access policy.
// A key without alerts:ReadAccessPolicy can get a 403 for the flag instead of a response without the
// policy, so the call is repeated without it: the policy is then not readable.
func getAlert(ctx context.Context, client *alertapi.AlertDefinitionsServiceAPIService, id string) (*alertapi.GetAlertDefResponse, *http.Response, error) {
	out, httpResp, err := client.AlertDefsServiceGetAlertDef(ctx, id).IncludeAccessPermissions(true).Execute()
	if err != nil && httpResp != nil && httpResp.StatusCode == http.StatusForbidden {
		return client.AlertDefsServiceGetAlertDef(ctx, id).Execute()
	}
	return out, httpResp, err
}

// afterGetAlert sets access_policy from the access block of the Get response. The state holds the
// plan or the prior state value, or null on import and in the data source.
func afterGetAlert(ctx context.Context, state *tfsdk.State, response *alertapi.GetAlertDefResponse) diag.Diagnostics {
	var current types.String
	diags := state.GetAttribute(ctx, path.Root("access_policy"), &current)
	if diags.HasError() {
		return diags
	}
	policy, readable := accessPolicyFromGetResponse(response)
	diags.Append(state.SetAttribute(ctx, path.Root("access_policy"), foldAccessPolicy(current, policy, readable))...)
	return diags
}

// accessPolicyFromGetResponse extracts the raw access policy from a Get response that opted into
// include_access_permissions. Three shapes reach the caller:
//   - access with a policy: the caller can read policies and one is set;
//   - access without a policy: the caller can read and the alert has no policy. This is an authoritative
//     "no policy", and the state clears;
//   - no access: the caller lacks alerts:ReadAccessPolicy (or the backend omits it), so the policy is
//     unknown and the state keeps its prior value.
func accessPolicyFromGetResponse(response *alertapi.GetAlertDefResponse) (policy *string, readable bool) {
	if response == nil {
		return nil, false
	}
	access, ok := response.GetAccessOk()
	if !ok || access == nil {
		return nil, false
	}
	p, ok := access.GetAccessPolicyOk()
	if !ok {
		return nil, true
	}
	return p, true
}

// foldAccessPolicy folds the access policy of the backend into the state value: a configured text that is
// semantically equal to the backend's is kept as written, otherwise the backend text is stored in the
// canonical shape that jsonencode produces.
//
// readable reports whether the response answers the policy question. Then no policy means that the alert
// has none, and a prior non-empty value is cleared, so a policy removed outside Terraform shows as drift and
// the next apply sets it again. When the read could not see policies (a key without alerts:ReadAccessPolicy),
// no policy means unknown, not deleted, and the prior value stays.
func foldAccessPolicy(current types.String, policy *string, readable bool) types.String {
	if policy == nil {
		if readable || current.IsNull() || current.IsUnknown() {
			return types.StringNull()
		}
		return current
	}
	if !current.IsNull() && !current.IsUnknown() && utils.JSONStringsEqual(current.ValueString(), *policy) {
		return current
	}
	canonical, err := utils.CanonicalJSON(*policy)
	if err != nil {
		// The API returned text that is not JSON. Store it unchanged instead of failing the read.
		return types.StringValue(*policy)
	}
	return types.StringValue(canonical)
}
