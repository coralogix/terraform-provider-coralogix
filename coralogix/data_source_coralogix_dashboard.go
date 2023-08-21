package coralogix

import (
	"context"
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-coralogix/coralogix/clientset"
	dashboards "terraform-provider-coralogix/coralogix/clientset/grpc/coralogix-dashboards/v1"
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

func (d *DashboardDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r DashboardResource
	var resourceResp resource.SchemaResponse
	r.Schema(nil, resource.SchemaRequest{}, &resourceResp)

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
	getDashboardResp, err := d.client.GetDashboard(ctx, &dashboards.GetDashboardRequest{DashboardId: wrapperspb.String(id)})
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if status.Code(err) == codes.NotFound {
			data.ID = types.StringNull()
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Dashboard %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Dashboard",
				handleRpcErrorNewFramework(err, "Dashboard"),
			)
		}
		return
	}
	log.Printf("[INFO] Received Dashboard: %#v", getDashboardResp)

	dashboard, diags := flattenDashboard(ctx, DashboardResourceModel{}, getDashboardResp.GetDashboard())
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

	data = *dashboard
	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
