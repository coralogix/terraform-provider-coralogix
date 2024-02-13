package coralogix

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"log"
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

	systemRoles, diags := d.fetchAllSystemRoles(ctx)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	d.parentRolesMapping = systemRoles

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
	id := data.ID.ValueInt64()
	log.Printf("[INFO] Reading Custom Role: %s", id)
	getApiKey := &roles.GetCustomRoleRequest{
		RoleId: uint32(id),
	}

	createCustomRoleResponse, err := d.client.GetRole(ctx, getApiKey)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Custom role  %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading custom role",
				formatRpcErrors(err, getActionURL, protojson.Format(getApiKey)),
			)
		}
		return
	}
	log.Printf("[INFO] Received Custom Role: %s", protojson.Format(createCustomRoleResponse))

	model, diags := d.flatterCustomRole(ctx, createCustomRoleResponse.GetRole())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func (c *CustomRoleDataSource) fetchAllSystemRoles(ctx context.Context) (map[string]*roles.SystemRole, diag.Diagnostics) {
	var diags diag.Diagnostics
	role, err := c.client.ListSystemRole(ctx, &roles.ListSystemRolesRequest{})
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.PermissionDenied || status.Code(err) == codes.Unauthenticated {
			diags.AddError(
				"Error getting Custom Role",
				fmt.Sprintf("permission denied for url - %s\ncheck your org-key and permissions", getSystemRolesPath),
			)
		} else {
			diags.AddError(
				"Error getting Custom Role",
				formatRpcErrors(err, getApiKeyPath, protojson.Format(&roles.ListSystemRolesRequest{})),
			)
		}
		return nil, diags
	}

	thisMap := make(map[string]*roles.SystemRole)
	for _, elem := range role.GetRoles() {
		thisMap[elem.Name] = elem
	}

	return thisMap, nil
}

func (c *CustomRoleDataSource) flatterCustomRole(ctx context.Context, customRole *roles.CustomRole) (*RolesModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	conditionSatisfied := ""
	for key, val := range c.parentRolesMapping {
		if val.RoleId == customRole.ParentRoleId {
			conditionSatisfied = key
			break
		}
	}
	if len(conditionSatisfied) == 0 {
		diags.AddError("Invalid parent role id", "Parent role not found!")
		return nil, diags
	}

	permissions, diags := types.SetValueFrom(ctx, types.StringType, customRole.Permissions)
	if diags.HasError() {
		return nil, diags
	}

	model := RolesModel{
		ID:          types.Int64Value(int64(customRole.RoleId)),
		TeamId:      types.Int64Value(int64(customRole.TeamId)),
		ParentRole:  types.StringValue(conditionSatisfied),
		Permissions: permissions,
		Description: types.StringValue(customRole.Description),
		Name:        types.StringValue(customRole.Name),
	}

	return &model, nil
}
