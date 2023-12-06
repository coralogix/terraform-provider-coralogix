package coralogix

import (
	"context"
	"fmt"
	"log"

	"terraform-provider-coralogix/coralogix/clientset"
	webhooks "terraform-provider-coralogix/coralogix/clientset/grpc/webhooks"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

var _ datasource.DataSourceWithConfigure = &WebhookDataSource{}

func NewWebhookDataSource() datasource.DataSource {
	return &WebhookDataSource{}
}

type WebhookDataSource struct {
	client *clientset.WebhooksClient
}

func (d *WebhookDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook"
}

func (d *WebhookDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	d.client = clientSet.Webhooks()
}

func (d *WebhookDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r WebhookResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	resp.Schema = frameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)
}

func (d *WebhookDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *WebhookResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	//Get refreshed Webhook value from Coralogix
	id := data.ID.ValueString()
	log.Printf("[INFO] Reading Webhook: %s", id)

	getWebhookReq := &webhooks.GetOutgoingWebhookRequest{Id: wrapperspb.String(id)}
	getWebhookResp, err := d.client.GetWebhook(ctx, getWebhookReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		if status.Code(err) == codes.NotFound {
			data.ID = types.StringNull()
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Webhook %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
		} else {
			reqStr := protojson.Format(getWebhookReq)
			resp.Diagnostics.AddError(
				"Error reading Webhook",
				handleRpcErrorNewFramework(err, "Webhook", reqStr),
			)
		}
		return
	}
	log.Printf("[INFO] Received Webhook: %#v", getWebhookResp)

	data, diags := flattenWebhook(ctx, getWebhookResp.GetWebhook())
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}
	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
