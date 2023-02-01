package coralogix

import (
	"context"
	"log"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"terraform-provider-coralogix/coralogix/clientset"
)

func dataSourceCoralogixWebhook() *schema.Resource {
	webhookSchema := datasourceSchemaFromResourceSchema(WebhookSchema())
	webhookSchema["id"] = &schema.Schema{
		Type:     schema.TypeString,
		Required: true,
	}

	return &schema.Resource{
		ReadContext: dataSourceCoralogixWebhookRead,

		Schema: webhookSchema,
	}
}

func dataSourceCoralogixWebhookRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Get("id").(string)

	log.Printf("[INFO] Reading webhook %s", id)
	webhook, err := meta.(*clientset.ClientSet).Webhooks().GetWebhook(ctx, id)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "webhook", id)
	}
	log.Printf("[INFO] Received webhook: %#v", webhook)

	d.SetId(strconv.Itoa(int(webhook["id"].(float64))))
	return setWebhook(d, webhook)
}
