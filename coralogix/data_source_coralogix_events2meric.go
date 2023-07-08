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
	e2m "terraform-provider-coralogix/coralogix/clientset/grpc/events2metrics/v2"
)

var _ datasource.DataSourceWithConfigure = &Events2MetricDataSource{}

func NewEvents2MetricDataSource() datasource.DataSource {
	return &Events2MetricDataSource{}
}

type Events2MetricDataSource struct {
	client *clientset.Events2MetricsClient
}

func (d *Events2MetricDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_events2metric"
}

func (d *Events2MetricDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = clientSet.Events2Metrics()
}

func (d *Events2MetricDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r Events2MetricResource
	var resourceResp resource.SchemaResponse
	r.Schema(nil, resource.SchemaRequest{}, &resourceResp)

	resp.Schema = frameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)
}

func (d *Events2MetricDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data Events2MetricResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed Events2Metric value from Coralogix
	id := data.ID.ValueString()
	log.Printf("[INFO] Reading Events2metric: %s", id)
	getE2MResp, err := d.client.GetEvents2Metric(ctx, &e2m.GetE2MRequest{Id: wrapperspb.String(id)})
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if status.Code(err) == codes.NotFound {
			data.ID = types.StringNull()
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Events2Metric %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Events2Metric",
				handleRpcErrorNewFramework(err, "Events2metric"),
			)
		}
		return
	}
	log.Printf("[INFO] Received Events2metric: %#v", getE2MResp)

	data = flattenE2M(ctx, getE2MResp.GetE2M())

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
