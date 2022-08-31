package coralogix

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

type webhookValue struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

func resourceCoralogixWebhook() *schema.Resource {
	return &schema.Resource{
		Create: resourceCoralogixWebhookCreate,
		Read:   resourceCoralogixWebhookRead,
		Update: resourceCoralogixWebhookUpdate,
		Delete: resourceCoralogixWebhookDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"alias": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringIsNotEmpty,
			},
			"type": {
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: validation.StringInSlice([]string{
					"slack",
					"pager_duty",
					"microsoft_teams",
					"webhook",
					"jira",
					"demisto",
					"email_group",
					"sendlog",
					"opsgenie",
				}, false),
			},
			"url": {
				Type:         schema.TypeString,
				ValidateFunc: validation.IsURLWithHTTPorHTTPS,
				Optional:     true,
				Default:      "",
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
				Optional: true,
				Default:  "",
			},
			"web_request": {
				Type:     schema.TypeSet,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"uuid": {
							Type:     schema.TypeString,
							Required: true,
						},
						"method": {
							Type:     schema.TypeString,
							Required: true,
							ValidateFunc: validation.StringInSlice([]string{
								"get",
								"post",
								"put",
							}, false),
						},
						"headers": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "{}",
						},
						"payload": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "{}",
						},
					},
				},
			},
			"jira": {
				Type:     schema.TypeSet,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"api_token": {
							Type:     schema.TypeString,
							Required: true,
						},
						"email": {
							Type:     schema.TypeString,
							Required: true,
						},
						"project_key": {
							Type:     schema.TypeString,
							Required: true,
						},
					},
				},
			},
			"email_group": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func resourceCoralogixWebhookCreate(d *schema.ResourceData, meta interface{}) error {
	if err := webhookValuesValidation(d); err != nil {
		return err
	}
	apiClient := meta.(*Client)
	webhookType := d.Get("type").(string)
	webhookTypeFields := make([]webhookValue, 0, 4)
	switch webhookType {
	case "pager_duty":
		webhookTypeFields = append(webhookTypeFields, webhookValue{Name: "serviceKey", Value: d.Get("pager_duty").(string)})
	case "webhook", "demisto", "sendlog":
		webRequest := getFirstOrNil(d.Get("web_request").(*schema.Set).List()).(map[string]interface{})
		webhookTypeFields = append(webhookTypeFields, webhookValue{Name: "uuid", Value: webRequest["uuid"].(string)})
		webhookTypeFields = append(webhookTypeFields, webhookValue{Name: "method", Value: webRequest["method"].(string)})
		// using json unmarshal to not send double escaped json onto the api
		webhookTypeFields = append(webhookTypeFields, webhookValue{Name: "headers", Value: ""})
		if err := json.Unmarshal([]byte(webRequest["headers"].(string)), &webhookTypeFields[2].Value); err != nil {
			return fmt.Errorf("error while decoding json in 'web_request.headers'. err: %s", err.Error())
		}
		webhookTypeFields = append(webhookTypeFields, webhookValue{Name: "payload", Value: ""})
		if err := json.Unmarshal([]byte(webRequest["payload"].(string)), &webhookTypeFields[3].Value); err != nil {
			return fmt.Errorf("error while decoding json in 'web_request.payload'. err: %s", err.Error())
		}
	case "jira":
		jira := getFirstOrNil(d.Get("jira").(*schema.Set).List()).(map[string]interface{})
		webhookTypeFields = append(webhookTypeFields, webhookValue{Name: "apiToken", Value: jira["api_token"].(string)})
		webhookTypeFields = append(webhookTypeFields, webhookValue{Name: "email", Value: jira["email"].(string)})
		webhookTypeFields = append(webhookTypeFields, webhookValue{Name: "projectKey", Value: jira["project_key"].(string)})
	case "email_group":
		emailGroup := (d.Get("email_group").(*schema.Set)).List()
		webhookTypeFields = append(webhookTypeFields, webhookValue{Name: "payload", Value: emailGroup})
	}
	webhookTypeFieldsJson, _ := json.Marshal(webhookTypeFields)
	webhookParameters := map[string]interface{}{
		"alias":                   d.Get("alias").(string),
		"integration_type":        getWebhookJsonFields(webhookType),
		"integration_type_id":     transformWebhookName(webhookType),
		"integration_type_fields": string(webhookTypeFieldsJson),
		"url":                     d.Get("url").(string),
	}
	webhook, err := apiClient.Post("/external/integrations", webhookParameters)
	if err != nil {
		return err
	}

	d.SetId(fmt.Sprintf("%.0f", webhook["id"].(float64)))

	return resourceCoralogixWebhookRead(d, meta)
}

func resourceCoralogixWebhookRead(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)
	webhook, err := apiClient.Get(fmt.Sprintf("/external/integrations/%s", d.Id()))
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
	return nil
}

func resourceCoralogixWebhookUpdate(d *schema.ResourceData, meta interface{}) error {
	// until the integration api allows put calls its the same as creating a new resource only adding the id
	if err := webhookValuesValidation(d); err != nil {
		return err
	}
	apiClient := meta.(*Client)
	webhookType := d.Get("type").(string)
	webhookTypeFields := make([]webhookValue, 0, 4)
	switch webhookType {
	case "pager_duty":
		webhookTypeFields = append(webhookTypeFields, webhookValue{Name: "serviceKey", Value: d.Get("pager_duty").(string)})
	case "webhook", "demisto", "sendlog":
		webRequest := getFirstOrNil(d.Get("web_request").(*schema.Set).List()).(map[string]interface{})
		webhookTypeFields = append(webhookTypeFields, webhookValue{Name: "uuid", Value: webRequest["uuid"].(string)})
		webhookTypeFields = append(webhookTypeFields, webhookValue{Name: "method", Value: webRequest["method"].(string)})
		// using json unmarshal to not send double escaped json onto the api
		webhookTypeFields = append(webhookTypeFields, webhookValue{Name: "headers", Value: ""})
		if err := json.Unmarshal([]byte(webRequest["headers"].(string)), &webhookTypeFields[2].Value); err != nil {
			return fmt.Errorf("error while decoding json in 'web_request.headers'. err: %s", err.Error())
		}
		webhookTypeFields = append(webhookTypeFields, webhookValue{Name: "payload", Value: ""})
		if err := json.Unmarshal([]byte(webRequest["payload"].(string)), &webhookTypeFields[3].Value); err != nil {
			return fmt.Errorf("error while decoding json in 'web_request.payload'. err: %s", err.Error())
		}
	case "jira":
		jira := getFirstOrNil(d.Get("jira").(*schema.Set).List()).(map[string]interface{})
		webhookTypeFields = append(webhookTypeFields, webhookValue{Name: "apiToken", Value: jira["api_token"].(string)})
		webhookTypeFields = append(webhookTypeFields, webhookValue{Name: "email", Value: jira["email"].(string)})
		webhookTypeFields = append(webhookTypeFields, webhookValue{Name: "projectKey", Value: jira["project_key"].(string)})
	case "email_group":
		emailGroup := (d.Get("email_group").(*schema.Set)).List()
		webhookTypeFields = append(webhookTypeFields, webhookValue{Name: "payload", Value: emailGroup})
	}
	webhookTypeFieldsJson, _ := json.Marshal(webhookTypeFields)
	id, _ := strconv.Atoi(d.Id())
	webhookParameters := map[string]interface{}{
		"alias":                   d.Get("alias").(string),
		"integration_type":        getWebhookJsonFields(webhookType),
		"integration_type_id":     transformWebhookName(webhookType),
		"integration_type_fields": string(webhookTypeFieldsJson),
		"url":                     d.Get("url").(string),
		"id":                      id,
	}
	_, err := apiClient.Post("/external/integrations", webhookParameters)
	if err != nil {
		return err
	}
	// cannot set as return value is not valid for now (api side)
	//d.SetId(fmt.Sprintf("%.0f", webhook["id"].(float64)))

	return resourceCoralogixWebhookRead(d, meta)
}

func resourceCoralogixWebhookDelete(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)
	_, err := apiClient.Delete(fmt.Sprintf("/external/integrations/%s", d.Id()))
	if err != nil {
		return err
	}
	return nil
}
