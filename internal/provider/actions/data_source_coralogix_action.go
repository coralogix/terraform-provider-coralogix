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

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// actionDataSource is the generated data source. The released data source showed `url_fields` as
// an empty list for an action that has none, where the generated flatten reads an empty list as
// null (the released resource does). Read restores the empty list.
type actionDataSource struct {
	datasource.DataSource
	configure datasource.DataSourceWithConfigure
}

func NewActionDataSource() datasource.DataSource {
	inner := action.NewDataSource(actionHooks)
	return &actionDataSource{DataSource: inner, configure: inner.(datasource.DataSourceWithConfigure)}
}

func (d *actionDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.configure.Configure(ctx, req, resp)
}

func (d *actionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	d.DataSource.Read(ctx, req, resp)
	if resp.Diagnostics.HasError() || resp.State.Raw.IsNull() {
		return
	}
	var urlFields types.List
	if resp.Diagnostics.Append(resp.State.GetAttribute(ctx, path.Root("url_fields"), &urlFields)...); resp.Diagnostics.HasError() || !urlFields.IsNull() {
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("url_fields"), types.ListValueMust(urlFields.ElementType(ctx), nil))...)
}
