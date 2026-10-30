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

package actions

import (
	"context"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/action"

	actionss "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/actions_service"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// actionHooks are the hooks of the resource and the data source. The API omits createdBy for an
// action that an API key made, and the released resource and data source read that as "". The
// released Create omitted a description and a dpxl_filter that the configuration does not set,
// while Update sends "" to clear them (the API keeps a value that a request omits).
var actionHooks = action.Hooks{
	BeforeWrite: func(ctx context.Context, config tfsdk.Config, body any) diag.Diagnostics {
		create, ok := body.(*actionss.ActionsServiceCreateActionRequest)
		if !ok {
			return nil
		}
		var diags diag.Diagnostics
		for attribute, field := range map[string]**string{"description": &create.Description, "dpxl_filter": &create.DpxlFilter} {
			var configured types.String
			diags.Append(config.GetAttribute(ctx, path.Root(attribute), &configured)...)
			if configured.IsNull() {
				*field = nil
			}
		}
		return diags
	},
	BeforeFlatten: func(_ context.Context, v *actionss.V2Action) diag.Diagnostics {
		if v.CreatedBy == nil {
			empty := ""
			v.CreatedBy = &empty
		}
		return nil
	},
}
