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
	"fmt"
	"log"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	_ datasource.DataSourceWithConfigure = &ScopeDataSource{}
)

func NewScopeDataSource() datasource.DataSource {
	return &ScopeDataSource{}
}

type ScopeDataSource struct {
	client *cxsdk.ScopesClient
}

func (d *ScopeDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_scope"
}

func (d *ScopeDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = clientSet.Scopes()
}

func (d *ScopeDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r ScopeResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	resp.Schema = utils.FrameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)
}

func (d *ScopeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *ScopeResourceModel
	diags := req.Config.Get(ctx, &data)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	getScopeReq := &cxsdk.GetTeamScopesByIDsRequest{
		Ids: []string{data.ID.ValueString()},
	}
	log.Printf("[INFO] Reading Scopes: %s", protojson.Format(getScopeReq))
	getScopeResp, err := d.client.Get(ctx, getScopeReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error reading Scope",
			utils.FormatRpcErrors(err, cxsdk.GetTeamScopesByIDsRPC, protojson.Format(getScopeReq)),
		)
		return
	}
	log.Printf("[INFO] Received Scope: %s", protojson.Format(getScopeResp))

	data = &(flattenScope(getScopeResp)[0])
	// Set state to fully populated data
	diags = resp.State.Set(ctx, &data)
	resp.Diagnostics.Append(diags...)
}
