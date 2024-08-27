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

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ datasource.DataSourceWithConfigure = &ApiKeyDataSource{}

func NewApiKeyDataSource() datasource.DataSource {
	return &ApiKeyDataSource{}
}

type ApiKeyDataSource struct {
	client *cxsdk.ApikeysClient
}

func (d *ApiKeyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}

func (d *ApiKeyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clientSet, ok := req.ProviderData.(*cxsdk.ClientSet)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *cxsdk.ClientSet, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = clientSet.APIKeys()
}

func (d *ApiKeyDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r ApiKeyResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	resp.Schema = frameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)
}

func (d *ApiKeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *ApiKeyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	log.Printf("[INFO] Reading ApiKey")

	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed API Keys value from Coralogix
	id := data.ID.ValueString()
	log.Printf("[INFO] Reading ApiKey: %s", id)
	getApiKey := &cxsdk.GetAPIKeyRequest{
		KeyId: id,
	}

	getApiKeyResponse, err := d.client.Get(ctx, getApiKey)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(err.Error(),
				fmt.Sprintf("API Keys %q is in state, but no longer exists in Coralogix backend", id),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading API Keys",
				formatRpcErrors(err, cxsdk.GetAPIKeyRpc, protojson.Format(getApiKey)),
			)
		}
		return
	}
	log.Printf("[INFO] Received API Keys: %s", protojson.Format(getApiKeyResponse))

	if getApiKeyResponse.KeyInfo.Hashed {
		resp.Diagnostics.AddError(
			"Error reading API Keys",
			"Reading an hashed key is impossible",
		)
		return
	}
	response, diags := flattenGetApiKeyResponse(ctx, &id, getApiKeyResponse, getApiKeyResponse.KeyInfo.Value)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &response)...)
}
