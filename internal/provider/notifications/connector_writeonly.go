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

	connectors "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/connectors_service"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type priorAttributes interface {
	GetAttribute(context.Context, path.Path, any) diag.Diagnostics
}

func mergeWriteOnlyIntoRequest(ctx context.Context, config tfsdk.Config, body any) diag.Diagnostics {
	var secrets types.Map
	diags := config.GetAttribute(ctx, path.Root("connector_config").AtName("field_values_wo"), &secrets)
	if diags.HasError() {
		return diags
	}
	if secrets.IsNull() || secrets.IsUnknown() {
		return diags
	}
	values := map[string]string{}
	diags.Append(secrets.ElementsAs(ctx, &values, false)...)
	if diags.HasError() {
		return diags
	}
	switch req := body.(type) {
	case *connectors.CreateConnectorRequest:
		mergeSecretFields(&req.Connector, values)
	case *connectors.ReplaceConnectorRequest:
		mergeSecretFields(&req.Connector, values)
	}
	return diags
}

func restoreWriteOnlyAfterRead(ctx context.Context, state *tfsdk.State, prior any) diag.Diagnostics {
	if state == nil || prior == nil {
		return nil
	}
	src, ok := prior.(priorAttributes)
	if !ok {
		return nil
	}

	var versions types.Map
	diags := src.GetAttribute(ctx, path.Root("connector_config").AtName("field_values_wo_versions"), &versions)
	if diags.HasError() {
		return diags
	}

	if versions.IsNull() || versions.IsUnknown() {
		versions = types.MapNull(types.Int64Type)
	}
	names := map[string]struct{}{}
	if !versions.IsNull() && !versions.IsUnknown() {
		for name := range versions.Elements() {
			names[name] = struct{}{}
		}
	}

	var cfg types.Object
	diags.Append(state.GetAttribute(ctx, path.Root("connector_config"), &cfg)...)
	if diags.HasError() || cfg.IsNull() || cfg.IsUnknown() {
		return diags
	}

	var fields types.Set
	diags.Append(state.GetAttribute(ctx, path.Root("connector_config").AtName("fields"), &fields)...)
	if diags.HasError() {
		return diags
	}
	fields, d := omitWriteOnlyFields(ctx, fields, names)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	diags.Append(state.SetAttribute(ctx, path.Root("connector_config").AtName("fields"), fields)...)
	diags.Append(state.SetAttribute(ctx, path.Root("connector_config").AtName("field_values_wo"), types.MapNull(types.StringType))...)
	diags.Append(state.SetAttribute(ctx, path.Root("connector_config").AtName("field_values_wo_versions"), versions)...)
	return diags
}
