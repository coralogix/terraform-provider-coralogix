package coralogix

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceCoralogixWebhook() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCoralogixWebhookCreate,
		ReadContext:   resourceCoralogixWebhookRead,
		UpdateContext: resourceCoralogixWebhookUpdate,
		DeleteContext: resourceCoralogixWebhookDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(60 * time.Second),
			Read:   schema.DefaultTimeout(30 * time.Second),
			Update: schema.DefaultTimeout(60 * time.Second),
			Delete: schema.DefaultTimeout(30 * time.Second),
		},

		Schema: WebhookSchema(),

		//Description: "Rule-group is list of rule-subgroups with 'and' (&&) operation between. Api-key is required for this resource.",
	}
}

func resourceCoralogixWebhookCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return nil
}

func resourceCoralogixWebhookRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return nil
}

func resourceCoralogixWebhookUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return nil
}

func resourceCoralogixWebhookDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return nil
}

func WebhookSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{}
}
