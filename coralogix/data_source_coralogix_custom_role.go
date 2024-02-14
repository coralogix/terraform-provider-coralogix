package coralogix

import (
	"context"
	"fmt"
	"log"
	"strconv"
	roles "terraform-provider-coralogix/coralogix/clientset/grpc/roles"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"terraform-provider-coralogix/coralogix/clientset"
)

var _ datasource.DataSourceWithConfigure = &CustomRoleDataSource{}

func NewCustomRoleDataSource() datasource.DataSource {
	return &CustomRoleDataSource{}
}

type CustomRoleDataSource struct {
	client             *clientset.RolesClient
	parentRolesMapping map[string]*roles.SystemRole
}

func (d *CustomRoleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_role"
}

func (d *CustomRoleDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = clientSet.CustomRoles()
}

func (d *CustomRoleDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r ApiKeyResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	resp.Schema = frameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)
}

func (d *CustomRoleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *RolesModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	log.Printf("[INFO] Reading Custom Role")

	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed Action value from Coralogix
	roleId, err := strconv.Atoi(data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid Id", "Custom role id must be an int")
		return
	}
	log.Printf("[INFO] Reading Custom Role: %v", roleId)
	getCustomRoleReuest := &roles.GetCustomRoleRequest{
		RoleId: uint32(roleId),
	}

	createCustomRoleResponse, err := d.client.GetRole(ctx, getCustomRoleReuest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Custom role  %q is in state, but no longer exists in Coralogix backend", roleId),
				fmt.Sprintf("%v will be recreated when you apply", roleId),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading custom role",
				formatRpcErrors(err, getRolePath, protojson.Format(getCustomRoleReuest)),
			)
		}
		return
	}
	log.Printf("[INFO] Received Custom Role: %s", protojson.Format(createCustomRoleResponse))

	model, diags := flatterCustomRole(ctx, createCustomRoleResponse.GetRole())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}
