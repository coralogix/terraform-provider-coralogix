package coralogix

import (
	"context"
	"fmt"
	"log"

	"terraform-provider-coralogix/coralogix/clientset"
	dashboards "terraform-provider-coralogix/coralogix/clientset/grpc/dashboards"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var _ datasource.DataSourceWithConfigure = &DashboardDataSource{}

func NewDashboardDataSource() datasource.DataSource {
	return &DashboardDataSource{}
}

type DashboardDataSource struct {
	client *clientset.DashboardsClient
}

func (d *DashboardDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dashboard"
}

func (d *DashboardDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = clientSet.Dashboards()
}

func (d *DashboardDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r DashboardResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	resp.Schema = frameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)
}

func (d *DashboardDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DashboardResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed Dashboard value from Coralogix
	id := data.ID.ValueString()
	log.Printf("[INFO] Reading Dashboard: %s", id)
	getDashboardReq := &dashboards.GetDashboardRequest{DashboardId: wrapperspb.String(id)}
	getDashboardResp, err := d.client.GetDashboard(ctx, getDashboardReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(err.Error(),
				fmt.Sprintf("Dashboard %q is in state, but no longer exists in Coralogix backend", id))
		} else {
			resp.Diagnostics.AddError(
				"Error reading Dashboard",
				formatRpcErrors(err, getDashboardURL, protojson.Format(getDashboardReq)),
			)
		}
		return
	}
	log.Printf("[INFO] Received Dashboard: %s", protojson.Format(getDashboardResp))

	dashboard, diags := flattenDashboard(ctx, DashboardResourceModel{}, getDashboardResp.GetDashboard())
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

	data = *dashboard
	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
