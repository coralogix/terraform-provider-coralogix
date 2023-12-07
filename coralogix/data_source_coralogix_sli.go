package coralogix

import (
	"context"
	"fmt"
	"log"

	"terraform-provider-coralogix/coralogix/clientset"
	sli "terraform-provider-coralogix/coralogix/clientset/grpc/sli"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var _ datasource.DataSourceWithConfigure = &SLIDataSource{}

func NewSLIDataSource() datasource.DataSource {
	return &SLIDataSource{}
}

type SLIDataSource struct {
	client *clientset.SLIClient
}

func (d *SLIDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_sli"
}

func (d *SLIDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = clientSet.SLIs()
}

func (d *SLIDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r SLIResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	schema := frameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)
	schema.Attributes["service_name"] = datasourceschema.StringAttribute{
		MarkdownDescription: "The service name",
		Required:            true,
	}
	resp.Schema = schema
}

func (d *SLIDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SLIResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed sli value from Coralogix
	id := data.ID.ValueString()
	serviceName := data.ServiceName.ValueString()
	log.Printf("[INFO] Reading sli: %s", id)
	getSLIsReq := &sli.GetSlisRequest{ServiceName: wrapperspb.String(serviceName)}
	getSLIsResp, err := d.client.GetSLIs(ctx, getSLIsReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if status.Code(err) == codes.NotFound {
			data.ID = types.StringNull()
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("SLI %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading SLI",
				formatRpcErrors(err, getSliURL, protojson.Format(getSLIsReq)),
			)
		}
		return
	}

	var SLI *sli.Sli
	for _, sli := range getSLIsResp.GetSlis() {
		if sli.SliId.GetValue() == id {
			SLI = sli
			break
		}
	}
	if SLI == nil {
		data.ID = types.StringNull()
		resp.Diagnostics.AddWarning(
			fmt.Sprintf("SLI %q is in state, but no longer exists in Coralogix backend", id),
			fmt.Sprintf("%s will be recreated when you apply", id),
		)
		return
	}

	log.Printf("[INFO] Received SLI: %s", protojson.Format(SLI))

	data, diags := flattenSLI(ctx, SLI)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
