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
	"terraform-provider-coralogix/coralogix/clientset"
	tcopolicies "terraform-provider-coralogix/coralogix/clientset/grpc/tco-policies"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

var _ datasource.DataSourceWithConfigure = &TCOPolicyDataSource{}

func NewTCOPolicyDataSource() datasource.DataSource {
	return &TCOPolicyDataSource{}
}

type TCOPolicyDataSource struct {
	client *clientset.TCOPoliciesClient
}

func (d *TCOPolicyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tco_policy_logs"
}

func (d *TCOPolicyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = clientSet.TCOPolicies()
}

func (d *TCOPolicyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r TCOPolicyResource
	var resourceResp resource.SchemaResponse
	r.Schema(nil, resource.SchemaRequest{}, &resourceResp)

	resp.Schema = frameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)
}

func (d *TCOPolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data TCOPolicyResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed tco-policy value from Coralogix
	id := data.ID.ValueString()
	log.Printf("[INFO] Reading tco-policy: %s", id)
	getPolicyResp, err := d.client.GetTCOPolicy(ctx, &tcopolicies.GetPolicyRequest{Id: wrapperspb.String(id)})
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if status.Code(err) == codes.NotFound {
			data.ID = types.StringNull()
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("tco-policy %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading tco-policy",
				handleRpcErrorNewFramework(err, "tco-policy"),
			)
		}
		return
	}
	log.Printf("[INFO] Received tco-policy: %#v", getPolicyResp)

	data = flattenTCOPolicy(getPolicyResp.GetPolicy())

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
