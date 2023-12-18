package coralogix

import (
	"context"
	"fmt"
	"log"

	archiveRetention "terraform-provider-coralogix/coralogix/clientset/grpc/archive-retentions"

	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"google.golang.org/protobuf/encoding/protojson"

	"terraform-provider-coralogix/coralogix/clientset"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var _ datasource.DataSourceWithConfigure = &ArchiveRetentionsDataSource{}

func NewArchiveRetentionsDataSource() datasource.DataSource {
	return &ArchiveRetentionsDataSource{}
}

type ArchiveRetentionsDataSource struct {
	client *clientset.ArchiveRetentionsClient
}

func (d *ArchiveRetentionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_archive_retentions"
}

func (d *ArchiveRetentionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = clientSet.ArchiveRetentions()
}

func (d *ArchiveRetentionsDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r ArchiveRetentionsResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	resp.Schema = convertSchema(resourceResp.Schema)
}

func convertSchema(rs resourceschema.Schema) datasourceschema.Schema {
	attributes := convertAttributes(rs.Attributes)
	return datasourceschema.Schema{
		Attributes:          attributes,
		Description:         rs.Description,
		MarkdownDescription: rs.MarkdownDescription,
		DeprecationMessage:  rs.DeprecationMessage,
	}
}

func (d *ArchiveRetentionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *ArchiveRetentionsResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed archive-retentions value from Coralogix
	log.Print("[INFO] Reading archive-retentions:")
	getArchiveRetentionsReq := &archiveRetention.GetRetentionsRequest{}
	getArchiveRetentionsResp, err := d.client.GetRetentions(ctx, getArchiveRetentionsReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		resp.Diagnostics.AddError(
			"Error reading archive-retentions",
			formatRpcErrors(err, getArchiveRetentionsURL, protojson.Format(getArchiveRetentionsReq)),
		)

		return
	}
	log.Printf("[INFO] Received archive-retentions: %s", protojson.Format(getArchiveRetentionsResp))

	data, diags := flattenArchiveRetentions(ctx, getArchiveRetentionsResp.GetRetentions())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
