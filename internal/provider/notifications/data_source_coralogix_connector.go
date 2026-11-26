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

	"github.com/hashicorp/terraform-plugin-framework/datasource"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/connector"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"
)

// NewConnectorDataSource returns coralogix_connector, the generated data source of the resource. It
// finds a connector by id or by name, and takes the first connector with the name. The data source
// runs only the BeforeFlatten hook, which the resource does not set.
func NewConnectorDataSource() datasource.DataSource {
	return &connectorDataSource{DataSource: connector.NewDataSource(connector.Hooks{}).(*connector.DataSource)}
}

type connectorDataSource struct{ *connector.DataSource }

// Schema corrects the text of the write-only template attributes. A data source returns every
// template in `config_overrides`, a secret one too.
func (d *connectorDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	d.DataSource.Schema(ctx, req, resp)
	utils.DescribeDataSourceAttribute(resp.Schema,
		"Always null in a data source. Every template is in `config_overrides[].fields[].template`, also one that the resource sets with `config_overrides_templates_wo`. A secret template is returned there.",
		"config_overrides_templates_wo")
	utils.DescribeDataSourceAttribute(resp.Schema, "Always null in a data source.", "config_overrides_templates_wo_versions")
}
