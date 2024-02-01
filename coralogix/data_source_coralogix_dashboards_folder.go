package coralogix

import (
	"context"
	"fmt"
	"log"

	dashboards "terraform-provider-coralogix/coralogix/clientset/grpc/dashboards"

	"google.golang.org/protobuf/encoding/protojson"

	"terraform-provider-coralogix/coralogix/clientset"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var _ datasource.DataSourceWithConfigure = &DashboardsFolderDataSource{}

func NewDashboardsFoldersDataSource() datasource.DataSource {
	return &DashboardsFolderDataSource{}
}

type DashboardsFolderDataSource struct {
	client *clientset.DashboardsFoldersClient
}

func (d *DashboardsFolderDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dashboards_folder"
}

func (d *DashboardsFolderDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = clientSet.DashboardsFolders()
}

func (d *DashboardsFolderDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r ArchiveRetentionsResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	resp.Schema = convertSchemaWithoutID(resourceResp.Schema)
}

func (d *DashboardsFolderDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *DashboardsFolderResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed dashboards-folder value from Coralogix
	log.Print("[INFO] Reading dashboards-folders")
	getDashboardsFolders, err := d.client.GetDashboardsFolders(ctx, &dashboards.ListDashboardFoldersRequest{})
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error listing dashboards-folders",
			formatRpcErrors(err, getDashboardURL, protojson.Format(&dashboards.ListDashboardFoldersRequest{})),
		)

		return
	}
	log.Printf("[INFO] Received dashboards-folders: %s", protojson.Format(getDashboardsFolders))
	var dashboardsFolder *dashboards.DashboardFolder
	for _, folder := range getDashboardsFolders.GetFolder() {
		if folder.GetId().GetValue() == data.ID.ValueString() {
			dashboardsFolder = folder
			break
		}
	}
	if dashboardsFolder == nil {
		log.Printf("[ERROR] Could not find created folder with id: %s", data.ID.ValueString())
		resp.Diagnostics.AddError(
			"Error reading dashboards-folders",
			fmt.Sprintf("Could not find created folder with id: %s", data.ID.ValueString()),
		)
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
