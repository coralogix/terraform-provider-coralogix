package coralogix

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceCoralogixWebhook() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceCoralogixWebhookRead,

		Schema: map[string]*schema.Schema{
			"id": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"alias": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"type": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"url": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"updated_at": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"created_at": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"company_id": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"pager_duty": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"web_request": {
				Type:     schema.TypeSet,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"uuid": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"method": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"headers": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"payload": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
			"jira": {
				Type:     schema.TypeSet,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"api_token": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"email": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"project_key": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
			"email_group": {
				Type:     schema.TypeSet,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func dataSourceCoralogixWebhookRead(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)
	webhook, err := apiClient.Get(fmt.Sprintf("/external/integrations/%d", d.Get("id").(int)))
	if err != nil {
		return err
	}
	webhookType := transformWebhookId(int(webhook["integration_type_id"].(float64)))
	d.Set("alias", webhook["alias"].(string))
	d.Set("type", webhookType)
	d.Set("url", webhook["url"].(string))
	d.Set("updated_at", webhook["updated_at"].(string))
	d.Set("created_at", webhook["created_at"].(string))
	d.Set("company_id", int(webhook["company_id"].(float64)))
	typeFields := []webhookValue{}
	err = json.Unmarshal([]byte(webhook["integration_type_fields"].(string)), &typeFields)
	if err != nil {
		return err
	}
	switch webhookType {
	case "pager_duty":
		d.Set("pager_duty", typeFields[0].Value)
	case "webhook", "demisto", "sendlog":
		d.Set("web_request", flattenWebhookTypeFieldsWebRequest(typeFields))
	case "jira":
		d.Set("jira", flattenWebhookTypeFieldsJira(typeFields))
	case "email_group":
		d.Set("email_group", typeFields[0].Value)
	}
	d.SetId(strconv.Itoa(d.Get("id").(int)))
	return nil
}
