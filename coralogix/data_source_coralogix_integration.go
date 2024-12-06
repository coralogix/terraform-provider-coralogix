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

package coralogix

import (
	"context"
	"fmt"
	"log"

	"terraform-provider-coralogix/coralogix/clientset"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	_                 datasource.DataSourceWithConfigure = &IntegrationDataSource{}
	getIntegrationURL                                    = cxsdk.GetDeployedIntegrationRPC
)

func NewIntegrationDataSource() datasource.DataSource {
	return &IntegrationDataSource{}
}

type IntegrationDataSource struct {
	client *cxsdk.IntegrationsClient
}

func (d *IntegrationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration"
}

func (d *IntegrationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = clientSet.Integrations()
}

func (d *IntegrationDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r IntegrationResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	resp.Schema = frameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)
}

func (d *IntegrationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *IntegrationResourceModel
	diags := req.Config.Get(ctx, &data)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	getIntegrationReq := &cxsdk.GetDeployedIntegrationRequest{
		IntegrationId: wrapperspb.String(data.ID.ValueString()),
	}
	log.Printf("[INFO] Reading Integrations: %s", protojson.Format(getIntegrationReq))
	getIntegrationResp, err := d.client.Get(ctx, getIntegrationReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error reading Integration",
			formatRpcErrors(err, getIntegrationURL, protojson.Format(getIntegrationReq)),
		)
		return
	}
	log.Printf("[INFO] Received Integration: %s", protojson.Format(getIntegrationResp))
	keys, diags := KeysFromPlan(ctx, data)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	state, e := integrationDetail(getIntegrationResp, keys)
	if e.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}
