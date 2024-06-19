package coralogix

import (
	"context"
	"fmt"
	"log"

	apikeys "terraform-provider-coralogix/coralogix/clientset/grpc/apikeys"

	"google.golang.org/protobuf/encoding/protojson"

	"terraform-provider-coralogix/coralogix/clientset"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var _ datasource.DataSourceWithConfigure = &ApiKeyDataSource{}

func NewApiKeyDataSource() datasource.DataSource {
	return &ApiKeyDataSource{}
}

type ApiKeyDataSource struct {
	client *clientset.ApikeysClient
}

func (d *ApiKeyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_api_key"
}

func (d *ApiKeyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = clientSet.ApiKeys()
}

func (d *ApiKeyDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r ApiKeyResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	resp.Schema = frameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)
}

func (d *ApiKeyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *ApiKeyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	log.Printf("[INFO] Reading ApiKey")

	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed Action value from Coralogix
	id := data.ID.ValueString()
	log.Printf("[INFO] Reading ApiKey: %s", id)
	getApiKey := &apikeys.GetApiKeyRequest{
		KeyId: id,
	}

	getApiKeyResponse, err := d.client.GetApiKey(ctx, getApiKey)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(err.Error(),
				fmt.Sprintf("Action %q is in state, but no longer exists in Coralogix backend", id),
			)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Action",
				formatRpcErrors(err, getApiKeyPath, protojson.Format(getApiKey)),
			)
		}
		return
	}
	log.Printf("[INFO] Received Action: %s", protojson.Format(getApiKeyResponse))

	if getApiKeyResponse.KeyInfo.Hashed {
		resp.Diagnostics.AddError(
			"Error reading Action",
			"Reading an hashed key is impossible",
		)
		return
	}
	response, diags := flattenGetApiKeyResponse(ctx, &id, getApiKeyResponse, getApiKeyResponse.KeyInfo.Value)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &response)...)
}
