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

package dataplans

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	tcoPolicys "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/policies_service"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var _ datasource.DataSourceWithConfigure = &TCOPoliciesLogsDataSource{}

func NewTCOPoliciesLogsDataSource() datasource.DataSource {
	return &TCOPoliciesLogsDataSource{}
}

type TCOPoliciesLogsDataSource struct {
	client *tcoPolicys.PoliciesServiceAPIService
}

func (d *TCOPoliciesLogsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tco_policies_logs"
}

func (d *TCOPoliciesLogsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clientSet, ok := req.ProviderData.(*clientset.ClientSet)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *clientset.ClientSet, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = clientSet.TCOPolicies()
}

func (d *TCOPoliciesLogsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r TCOPoliciesLogsResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	attributes := utils.ConvertAttributes(resourceResp.Schema.Attributes)

	resp.Schema = datasourceschema.Schema{
		Attributes:          attributes,
		Description:         resourceResp.Schema.Description,
		MarkdownDescription: resourceResp.Schema.MarkdownDescription,
		DeprecationMessage:  resourceResp.Schema.DeprecationMessage,
	}
}

func (d *TCOPoliciesLogsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	log.Printf("[INFO] Reading coralogix_tco_policies_logs")
	result, httpResponse, err := d.client.PoliciesServiceGetCompanyPolicies(ctx).SourceType(LogSource).Execute()
	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				"coralogix_tco_policies_logs is in state, but no longer exists in Coralogix backend",
				"coralogix_tco_policies_logs will be recreated when you apply",
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error reading coralogix_tco_policies_logs",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
			)
		}
		return
	}
	log.Printf("[INFO] Read coralogix_tco_policies_logs: %s", utils.FormatJSON(result))

	state, diags := flattenGetTCOPoliciesLogsList(ctx, result)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
