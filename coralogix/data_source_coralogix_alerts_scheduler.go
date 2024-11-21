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
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var _ datasource.DataSourceWithConfigure = &AlertsSchedulerDataSource{}

func NewAlertsSchedulerDataSource() datasource.DataSource {
	return &AlertsSchedulerDataSource{}
}

type AlertsSchedulerDataSource struct {
	client *cxsdk.AlertSchedulerClient
}

func (d *AlertsSchedulerDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_alerts_scheduler"
}

func (d *AlertsSchedulerDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = clientSet.AlertSchedulers()
}

func (d *AlertsSchedulerDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r AlertsSchedulerResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	resp.Schema = frameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)
}

func (d *AlertsSchedulerDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *AlertsSchedulerResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed alerts-scheduler value from Coralogix
	id := data.ID.ValueString()
	log.Printf("[INFO] Reading alerts-scheduler: %s", id)
	getAlertsSchedulerReq := &cxsdk.GetAlertSchedulerRuleRequest{AlertSchedulerRuleId: id}
	getAlertsSchedulerResp, err := d.client.Get(ctx, getAlertsSchedulerReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error reading alerts-scheduler",
			formatRpcErrors(err, cxsdk.GetAlertSchedulerRuleRPC, protojson.Format(getAlertsSchedulerReq)),
		)
		return
	}
	log.Printf("[INFO] Received alerts-scheduler: %s", protojson.Format(getAlertsSchedulerResp))

	data, diags := flattenAlertScheduler(ctx, getAlertsSchedulerResp.GetAlertSchedulerRule())
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
