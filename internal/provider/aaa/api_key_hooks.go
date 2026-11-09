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

package aaa

import (
	"context"

	apiKeys "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/api_keys_service"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The API does not return the value of a hashed key. A key that Terraform created with schema version 0
// can be hashed, and its value is only in the state, so the resource keeps it. A hashed key that has
// no value in the state, an imported one, fails the read, as it did before.
const (
	hashedKeySummary = "Key value is required"
	hashedKeyDetail  = "Hashed API key has no recoverable value from the backend and none was provided."
)

type priorAttributes interface {
	GetAttribute(context.Context, path.Path, any) diag.Diagnostics
}

// keepStoredKeyValue keeps the value that the plan or the state before the read holds when the API
// returns none, and fails for a hashed key that has no value to keep.
func keepStoredKeyValue(ctx context.Context, state *tfsdk.State, prior any) diag.Diagnostics {
	var diags diag.Diagnostics
	var value types.String
	var hashed types.Bool
	diags.Append(state.GetAttribute(ctx, path.Root("hashed"), &hashed)...)
	diags.Append(state.GetAttribute(ctx, path.Root("value"), &value)...)
	if diags.HasError() || (!value.IsNull() && value.ValueString() != "") {
		return diags
	}
	if src, ok := prior.(priorAttributes); ok {
		var previous types.String
		if d := src.GetAttribute(ctx, path.Root("value"), &previous); !d.HasError() && !previous.IsNull() && !previous.IsUnknown() && previous.ValueString() != "" {
			return state.SetAttribute(ctx, path.Root("value"), previous)
		}
	}
	if hashed.ValueBool() {
		diags.AddError(hashedKeySummary, hashedKeyDetail)
	}
	return diags
}

// rejectHashedKey stops the data source for a hashed key: it has no value to show, and the data
// source has no state to take it from.
func rejectHashedKey(_ context.Context, key *apiKeys.KeyInfo) diag.Diagnostics {
	var diags diag.Diagnostics
	if key != nil && key.Hashed {
		diags.AddError(hashedKeySummary, hashedKeyDetail)
	}
	return diags
}

// omitUnconfiguredAccessPolicy leaves access_policy out of an Update when the configuration does not
// set it. The attribute keeps the policy of the state, so the plan holds it, but the API checks a
// permission on the key for any request that carries accessPolicy, also an empty one: a caller that
// may rename, deactivate, or change the lists of a key gets a 403 for the whole request. The released
// resource sent the policy only when it changed.
func omitUnconfiguredAccessPolicy(ctx context.Context, config tfsdk.Config, body any) diag.Diagnostics {
	update, ok := body.(*apiKeys.UpdateApiKeyRequest)
	if !ok {
		return nil
	}
	var policy types.String
	diags := config.GetAttribute(ctx, path.Root("access_policy"), &policy)
	if !diags.HasError() && policy.IsNull() {
		update.AccessPolicy = nil
	}
	return diags
}
