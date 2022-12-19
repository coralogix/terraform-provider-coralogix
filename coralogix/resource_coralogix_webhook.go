package coralogix

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
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

		//Description: "Webhook defines integration. Api-key is required for this resource.",
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
	return map[string]*schema.Schema{
		"slack": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"service_key": nil,
				},
			},
			MaxItems: 1,
		},
		"webhook": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: nil,
			},
			MaxItems: 1,
		},
		"pager_duty": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: nil,
			},
			MaxItems: 1,
		},
		"sendlog": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: nil,
			},
			MaxItems: 1,
		},
		"email_group": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"payload": nil,
				},
			},
			MaxItems: 1,
		},
		"microsoft_teams": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: nil,
			},
			MaxItems: 1,
		},
		"jira": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"api_token":   nil,
					"email":       nil,
					"project_key": nil,
				},
			},
			MaxItems: 1,
		},
		"opsgenie": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: nil,
			},
			MaxItems: 1,
		},
		"demisto": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"uuid": {
						Type:         schema.TypeString,
						ValidateFunc: validation.IsUUID,
					},
					"method": {
						Type:         schema.TypeString,
						ValidateFunc: validation.StringInSlice([]string{"get", "post", "put"}, false),
					},
					"headers": {},
					"payload": {},
				},
			},
			MaxItems: 1,
		},
	}
}
