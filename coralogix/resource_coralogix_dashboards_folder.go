package coralogix

import (
	"context"
	"fmt"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-coralogix/coralogix/clientset"
	dashboards "terraform-provider-coralogix/coralogix/clientset/grpc/dashboards"
)

var (
	createDashboardsFolderURL                                = "com.coralogixapis.dashboards.v1.services.DashboardFoldersService/CreateDashboardFolder"
	listDashboardsFoldersURL                                 = "com.coralogixapis.dashboards.v1.services.DashboardFoldersService/ListDashboardFolders"
	deleteDashboardsFolderURL                                = "com.coralogixapis.dashboards.v1.services.DashboardFoldersService/DeleteDashboardFolder"
	updateDashboardsFolderURL                                = "com.coralogixapis.dashboards.v1.services.DashboardFoldersService/ReplaceDashboardFolder"
	_                         resource.ResourceWithConfigure = &DashboardsFolderResource{}
)

func NewDashboardsFolderResource() resource.Resource {
	return &DashboardsFolderResource{}
}

type DashboardsFolderResource struct {
	client *clientset.DashboardsFoldersClient
}

func (r *DashboardsFolderResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = clientSet.DashboardsFolders()
}

type DashboardsFolderResourceModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

func (r *DashboardsFolderResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dashboards_folder"
}

func (r *DashboardsFolderResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 1,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Unique identifier for the folder.",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Display name of the folder.",
			},
		},
	}
	return
}

func (r *DashboardsFolderResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DashboardsFolderResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	dashboardsFolder := extractCreateDashboardsFolder(plan)
	id := dashboardsFolder.GetId().GetValue()
	dashboardsFolderStr := protojson.Format(dashboardsFolder)
	log.Printf("[INFO] Creating new Dashboards Folder: %s", dashboardsFolderStr)
	_, err := r.client.CreateDashboardsFolder(ctx, &dashboards.CreateDashboardFolderRequest{Folder: dashboardsFolder})
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError("Error Creating Dashboards Folder",
			formatRpcErrors(err, createDashboardsFolderURL, dashboardsFolderStr),
		)
		return
	}
	listResp, err := r.client.GetDashboardsFolders(ctx, &dashboards.ListDashboardFoldersRequest{})
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError("Error Listing Dashboards Folders",
			formatRpcErrors(err, listDashboardsFoldersURL, protojson.Format(&dashboards.ListDashboardFoldersRequest{})),
		)
		return
	}
	dashboardsFolder = nil
	for _, folder := range listResp.GetFolder() {
		if folder.GetId().GetValue() == id {
			dashboardsFolder = folder
			break
		}
	}
	if dashboardsFolder == nil {
		log.Printf("[ERROR] Could not find created folder with id: %s", id)
		resp.Diagnostics.AddError("Error Listing Dashboards Folders",
			fmt.Sprintf("Could not find created folder with id: %s", id),
		)
		return
	}

	log.Printf("[INFO] Submitted new Dashboards Folder: %s", protojson.Format(dashboardsFolder))

	plan = flattenDashboardsFolder(dashboardsFolder)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func flattenDashboardsFolder(folder *dashboards.DashboardFolder) DashboardsFolderResourceModel {
	return DashboardsFolderResourceModel{
		ID:   wrapperspbStringToTypeString(folder.GetId()),
		Name: wrapperspbStringToTypeString(folder.GetName()),
	}
}

func extractCreateDashboardsFolder(plan DashboardsFolderResourceModel) *dashboards.DashboardFolder {
	return &dashboards.DashboardFolder{
		Id:   expandUuid(plan.ID),
		Name: typeStringToWrapperspbString(plan.Name),
	}
}

func (r *DashboardsFolderResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DashboardsFolderResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	listResp, err := r.client.GetDashboardsFolders(ctx, &dashboards.ListDashboardFoldersRequest{})
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		formatRpcErrors(err, listDashboardsFoldersURL, protojson.Format(&dashboards.ListDashboardFoldersRequest{}))
		return
	}
	var dashboardsFolder *dashboards.DashboardFolder
	for _, folder := range listResp.GetFolder() {
		if folder.GetId().GetValue() == state.ID.ValueString() {
			dashboardsFolder = folder
			break
		}
	}
	if dashboardsFolder == nil {
		log.Printf("[ERROR] Could not find created folder with id: %s", state.ID.ValueString())
		resp.Diagnostics.AddError("Error Listing Dashboards Folders",
			fmt.Sprintf("Could not find created folder with id: %s", state.ID.ValueString()),
		)
		return
	}

	log.Printf("[INFO] Recived Dashboards Folder: %s", protojson.Format(dashboardsFolder))

	state = flattenDashboardsFolder(dashboardsFolder)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *DashboardsFolderResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DashboardsFolderResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	dashboardsFolder := extractCreateDashboardsFolder(plan)
	dashboardsFolderStr := protojson.Format(dashboardsFolder)
	log.Printf("[INFO] Creating new Dashboards Folder: %s", dashboardsFolderStr)
	_, err := r.client.UpdateDashboardsFolder(ctx, &dashboards.ReplaceDashboardFolderRequest{Folder: dashboardsFolder})
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError("Error Creating Dashboards Folder",
			formatRpcErrors(err, updateDashboardsFolderURL, dashboardsFolderStr),
		)
		return
	}
	listResp, err := r.client.GetDashboardsFolders(ctx, &dashboards.ListDashboardFoldersRequest{})
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError("Error Listing Dashboards Folders",
			formatRpcErrors(err, listDashboardsFoldersURL, protojson.Format(&dashboards.ListDashboardFoldersRequest{})),
		)
		return
	}
	dashboardsFolder = nil
	for _, folder := range listResp.GetFolder() {
		if folder.GetId().GetValue() == plan.ID.ValueString() {
			dashboardsFolder = folder
			break
		}
	}
	if dashboardsFolder == nil {
		log.Printf("[ERROR] Could not find created folder with id: %s", plan.ID.ValueString())
		resp.Diagnostics.AddError("Error Listing Dashboards Folders",
			fmt.Sprintf("Could not find created folder with id: %s", plan.ID.ValueString()),
		)
		return
	}

	log.Printf("[INFO] Submitted new Dashboards Folder: %s", protojson.Format(dashboardsFolder))

	plan = flattenDashboardsFolder(dashboardsFolder)

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
}

func (r *DashboardsFolderResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DashboardsFolderResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	log.Printf("[INFO] Deleting Dashboards Folder %s", id)
	deleteReq := &dashboards.DeleteDashboardFolderRequest{FolderId: wrapperspb.String(id)}
	if _, err := r.client.DeleteDashboardsFolder(ctx, deleteReq); err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error Deleting Dashboard %s", id),
			formatRpcErrors(err, deleteDashboardsFolderURL, protojson.Format(deleteReq)),
		)
		return
	}
	log.Printf("[INFO] Dashboards Folder %s deleted", id)
}
