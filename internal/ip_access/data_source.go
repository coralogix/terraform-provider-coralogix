package ip_access

import (
	"context"
	"fmt"
	"terraform-provider-coralogix/coralogix/clientset"
	"terraform-provider-coralogix/coralogix/utils"

	ipaccess "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/ip_access_service"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var _ datasource.DataSourceWithConfigure = &IpAccessDataSource{}

type IpAccessDataSource struct {
	client *ipaccess.IPAccessServiceAPIService
}

func (r *IpAccessDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ipaccess"
}

func (r *IpAccessDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var m *IpAccessResource
	var resourceResp resource.SchemaResponse
	m.Schema(ctx, resource.SchemaRequest{}, &resourceResp)
	resp.Schema = utils.FrameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)
}

func (r *IpAccessDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data IpAccessResource

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	result, _, err := r.client.
		IpAccessServiceGetCompanyIpAccessSettings(ctx).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error read resource",
			utils.FormatOpenAPIErrors(err, "Read", nil),
		)
		return
	}
	state := flattenReadResponse(result)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *IpAccessDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	r.client = clientSet.IpAccess()
}
