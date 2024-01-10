package coralogix

import (
	"context"
	"fmt"
	"log"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"terraform-provider-coralogix/coralogix/clientset"
	alertsSchedulers "terraform-provider-coralogix/coralogix/clientset/grpc/alerts-scheduler"
)

var _ datasource.DataSourceWithConfigure = &AlertsSchedulerDataSource{}

func NewAlertsSchedulerDataSource() datasource.DataSource {
	return &AlertsSchedulerDataSource{}
}

type AlertsSchedulerDataSource struct {
	client *clientset.AlertsSchedulersClient
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
	getAlertsSchedulerReq := &alertsSchedulers.GetAlertSchedulerRuleRequest{AlertSchedulerRuleId: id}
	getAlertsSchedulerResp, err := d.client.GetAlertScheduler(ctx, getAlertsSchedulerReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if status.Code(err) == codes.NotFound {
			data.ID = types.StringNull()
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("alerts-scheduler %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading alerts-scheduler",
				formatRpcErrors(err, getAlertsSchedulerURL, protojson.Format(getAlertsSchedulerReq)),
			)
		}
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
