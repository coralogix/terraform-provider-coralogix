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

	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"terraform-provider-coralogix/coralogix/clientset"
	tcopolicies "terraform-provider-coralogix/coralogix/clientset/grpc/tco-policies"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"google.golang.org/grpc/status"
)

var _ datasource.DataSourceWithConfigure = &TCOPoliciesTracesDataSource{}

func NewTCOPoliciesTracesDataSource() datasource.DataSource {
	return &TCOPoliciesTracesDataSource{}
}

type TCOPoliciesTracesDataSource struct {
	client *clientset.TCOPoliciesClient
}

func (d *TCOPoliciesTracesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tco_policies_traces"
}

func (d *TCOPoliciesTracesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *TCOPoliciesTracesDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r TCOPoliciesTracesResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	attributes := convertAttributes(resourceResp.Schema.Attributes)

	resp.Schema = datasourceschema.Schema{
		Attributes:          attributes,
		Description:         resourceResp.Schema.Description,
		MarkdownDescription: resourceResp.Schema.MarkdownDescription,
		DeprecationMessage:  resourceResp.Schema.DeprecationMessage,
	}
}

func (d *TCOPoliciesTracesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	getPoliciesReq := &tcopolicies.GetCompanyPoliciesRequest{SourceType: &tracesSource}
	log.Printf("[INFO] Reading tco-policies-traces")
	getPoliciesResp, err := d.client.GetTCOPolicies(ctx, getPoliciesReq)
	for err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if retryableStatusCode(status.Code(err)) {
			log.Print("[INFO] Retrying to read tco-policies-traces")
			getPoliciesResp, err = d.client.GetTCOPolicies(ctx, getPoliciesReq)
			continue
		}
		resp.Diagnostics.AddError(
			"Error reading tco-policies-traces",
			formatRpcErrors(err, getCompanyPoliciesURL, protojson.Format(getPoliciesReq)),
		)
		return
	}
	log.Printf("[INFO] Received tco-policies-traces: %s", protojson.Format(getPoliciesResp))

	state, diags := flattenGetTCOTracesPoliciesList(ctx, getPoliciesResp)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
