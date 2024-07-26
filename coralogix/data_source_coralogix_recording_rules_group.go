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
	rrgs "terraform-provider-coralogix/coralogix/clientset/grpc/recording-rules-groups-sets/v1"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ datasource.DataSourceWithConfigure = &RecordingRuleGroupSetDataSource{}

func NewRecordingRuleGroupSetDataSource() datasource.DataSource {
	return &RecordingRuleGroupSetDataSource{}
}

type RecordingRuleGroupSetDataSource struct {
	client *clientset.RecordingRulesGroupsSetsClient
}

func (d *RecordingRuleGroupSetDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_recording_rules_groups_set"
}

func (d *RecordingRuleGroupSetDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = clientSet.RecordingRuleGroupsSets()
}

func (d *RecordingRuleGroupSetDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r RecordingRuleGroupSetResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	resp.Schema = frameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)
}

func (d *RecordingRuleGroupSetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *RecordingRuleGroupSetResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed recording-rule-group-set value from Coralogix
	id := data.ID.ValueString()
	log.Printf("[INFO] Reading recording-rule-group-set: %s", id)
	getReq := &rrgs.FetchRuleGroupSet{Id: id}
	getResp, err := d.client.GetRecordingRuleGroupsSet(ctx, getReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				err.Error(),
				fmt.Sprintf("recording-rule-group-set %q is in state, but no longer exists in Coralogix backend", id),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading recording-rule-group-set",
				formatRpcErrors(err, getRuleGroupURL, protojson.Format(getReq)),
			)
		}
		return
	}
	log.Printf("[INFO] Received recording-rule-group-set: %s", protojson.Format(getResp))

	data, diags := flattenRecordingRuleGroupSet(ctx, &RecordingRuleGroupSetResourceModel{}, getResp)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
