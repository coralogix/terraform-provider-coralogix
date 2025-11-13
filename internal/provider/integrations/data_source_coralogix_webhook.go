// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package integrations

import (
	"context"
	"fmt"
	"log"
	"net/http"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	webhooks "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/outgoing_webhooks_service"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var _ datasource.DataSourceWithConfigure = &WebhookDataSource{}

func NewWebhookDataSource() datasource.DataSource {
	return &WebhookDataSource{}
}

type WebhookDataSource struct {
	client *webhooks.OutgoingWebhooksServiceAPIService
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

	resp.Schema = utils.FrameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)

	if idAttr, ok := resp.Schema.Attributes["id"].(schema.StringAttribute); ok {
		idAttr.Required = false
		idAttr.Optional = true
		idAttr.Validators = []validator.String{
			stringvalidator.ExactlyOneOf(path.MatchRelative().AtParent().AtName("name")),
		}
		resp.Schema.Attributes["id"] = idAttr
	}

	if nameAttr, ok := resp.Schema.Attributes["name"].(schema.StringAttribute); ok {
		nameAttr.Required = false
		nameAttr.Optional = true
		resp.Schema.Attributes["name"] = nameAttr
	}
}

func (d *WebhookDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data *WebhookResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := data.ID.ValueString()
	name := data.Name.ValueString()

	var result *webhooks.GetOutgoingWebhookResponse
	var err error

	if id != "" {
		result, err = d.fetchWebhookByID(ctx, id, resp)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error listing Webhooks",
				utils.FormatOpenAPIErrors(err, "Read", nil))
			return
		}

	} else if name != "" {
		log.Printf("[INFO] Listing Webhooks to find by name: %s", name)
		listResult, httpResponse, err := d.client.OutgoingWebhooksServiceListAllOutgoingWebhooks(ctx).Execute()
		if err != nil {
			log.Printf("[ERROR] Received error when listing webhooks: %s", err.Error())
			resp.Diagnostics.AddError(
				"Error listing Webhooks",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil))
			return
		}

		var webhookID string
		var found bool
		for _, webhookSummary := range listResult.GetDeployed() {
			if webhookSummary.GetName() == name {
				if found {
					resp.Diagnostics.AddError(
						"Multiple Webhooks Found",
						fmt.Sprintf("Multiple webhooks found with name %q", name),
					)
					return
				}
				found = true
				log.Printf("[INFO] Found Webhook ID by name: %s", webhookSummary.GetId())
				webhookID = webhookSummary.GetId()
			}
		}

		if webhookID == "" {
			resp.Diagnostics.AddError(
				"Webhook Not Found",
				fmt.Sprintf("No webhook found with name %q", name),
			)
			return
		}

		result, err = d.fetchWebhookByID(ctx, webhookID, resp)
		if err != nil {
			return
		}
	}
	data, diags := flattenWebhook(ctx, result.Webhook)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *WebhookDataSource) fetchWebhookByID(ctx context.Context, id string, resp *datasource.ReadResponse) (*webhooks.GetOutgoingWebhookResponse, error) {
	rq := d.client.OutgoingWebhooksServiceGetOutgoingWebhook(ctx, id)

	log.Printf("[INFO] Reading new coralogix_webhook: %s", utils.FormatJSON(rq))

	result, httpResponse, err := rq.Execute()
	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("coralogix_webhook %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error reading coralogix_webhook",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
			)
		}
		return nil, err
	}
	log.Printf("[INFO] Read coralogix_webhook: %s", utils.FormatJSON(result))
	return result, nil
}
