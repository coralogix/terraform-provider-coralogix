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

package integrations

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/integration"
)

// NewIntegrationDataSource returns the generated data source. The generated data source reads the
// id, the key, and the version. It does not run AfterRead, so this wrapper shows `parameters`
// itself: every parameter that the backend holds, a secret as <redacted>.
func NewIntegrationDataSource() datasource.DataSource {
	h := &integrationHooks{}
	generated := integration.NewDataSource(integration.Hooks{BeforeFlatten: h.rememberParameters})
	return &integrationDataSource{DataSource: generated.(*integration.DataSource)}
}

type integrationDataSource struct {
	*integration.DataSource
}

func (d *integrationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	ctx, handoff := withHandoff(ctx)
	d.DataSource.Read(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		return
	}
	parameters, diags := parametersToDynamic(handoff.parameters, nil)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("parameters"), parameters)...)
}
