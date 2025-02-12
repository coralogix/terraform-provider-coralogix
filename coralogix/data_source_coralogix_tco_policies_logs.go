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
	"time"

	"terraform-provider-coralogix/coralogix/clientset"
	"terraform-provider-coralogix/coralogix/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

var _ datasource.DataSourceWithConfigure = &TCOPoliciesLogsDataSource{}

func NewTCOPoliciesLogsDataSource() datasource.DataSource {
	return &TCOPoliciesLogsDataSource{}
}

type TCOPoliciesLogsDataSource struct {
	client *cxsdk.TCOPoliciesClient
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

	getPoliciesReq := &cxsdk.GetCompanyPoliciesRequest{SourceType: &logSource}
	log.Printf("[INFO] Reading tco-policies-logs")
	getPoliciesResp, err := d.client.List(ctx, getPoliciesReq)
	for err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if utils.RetryableStatusCode(status.Code(err)) {
			log.Print("[INFO] Retrying to read tco-policies-logs")
			getPoliciesResp, err = d.client.List(ctx, getPoliciesReq)
			continue
		}
		resp.Diagnostics.AddError(
			"Error reading tco-policies",
			utils.FormatRpcErrors(err, getCompanyPoliciesURL, protojson.Format(getPoliciesReq)),
		)
		return
	}
	log.Printf("[INFO] Received tco-policies-logs: %s", protojson.Format(getPoliciesResp))

	state, diags := flattenGetTCOPoliciesLogsList(ctx, getPoliciesResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
