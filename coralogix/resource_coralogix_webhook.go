package coralogix

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	. "github.com/ahmetalpbalkan/go-linq"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"terraform-provider-coralogix/coralogix/clientset"
)

var (
	validWebhookTypes = []string{"slack", "custom", "pager_duty", "email_group", "microsoft_teams", "jira", "opsgenie", "sendlog", "demisto"}
	validMethods      = []string{"get", "post", "put"}
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

		Description: "Webhook defines integration. More info - https://coralogix.com/integrations/ (Alerting section).",
	}
}

func resourceCoralogixWebhookCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	body, err := extractCreateWebhookRequest(d)
	if err != nil {
		return diag.FromErr(err)
	}

	log.Printf("[INFO] Creating new webhook: %#v", body)
	resp, err := meta.(*clientset.ClientSet).Webhooks().CreateWebhook(ctx, body)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err, "webhook")
	}
	log.Printf("[INFO] Submitted new webhook: %#v", resp)

	var m map[string]interface{}
	if err = json.Unmarshal([]byte(resp), &m); err != nil {
		return diag.FromErr(err)
	}
	id := strconv.Itoa(int(m["id"].(float64)))
	d.SetId(id)
	return resourceCoralogixWebhookRead(ctx, d, meta)
}

func resourceCoralogixWebhookRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	log.Printf("[INFO] Reading webhook %s", id)
	resp, err := meta.(*clientset.ClientSet).Webhooks().GetWebhook(ctx, id)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err, "webhook")
	}
	log.Printf("[INFO] Received webhook: %#v", resp)

	var m map[string]interface{}
	if err = json.Unmarshal([]byte(resp), &m); err != nil {
		return diag.FromErr(err)
	}
	return setWebhook(d, m)
}

func resourceCoralogixWebhookUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	body, err := extractCreateWebhookRequest(d)
	if err != nil {
		return diag.FromErr(err)
	}

	log.Printf("[INFO] Updating webhook: %#v", body)
	resp, err := meta.(*clientset.ClientSet).Webhooks().UpdateWebhook(ctx, body)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcError(err, "webhook")
	}
	log.Printf("[INFO] Submitted updated webhook: %#v", resp)
	return resourceCoralogixWebhookRead(ctx, d, meta)
}

func resourceCoralogixWebhookDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Id()
	log.Printf("[INFO] Deleting webhook %s\n", id)
	_, err := meta.(*clientset.ClientSet).Webhooks().DeleteWebhook(ctx, id)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v\n", err)
		return handleRpcErrorWithID(err, "webhook", id)
	}
	log.Printf("[INFO] webhook %s deleted\n", id)

	d.SetId("")
	return nil
}

func WebhookSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"name": {
			Type:     schema.TypeString,
			Required: true,
		},
		"slack": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"url": {
						Type:             schema.TypeString,
						Optional:         true,
						ValidateDiagFunc: urlValidationFunc(),
					},
				},
			},
			MaxItems:     1,
			ExactlyOneOf: validWebhookTypes,
		},
		"custom": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"url": {
						Type:             schema.TypeString,
						Required:         true,
						ValidateDiagFunc: urlValidationFunc(),
					},
					"uuid": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"method": {
						Type:         schema.TypeString,
						ValidateFunc: validation.StringInSlice(validMethods, false),
						Required:     true,
					},
					"headers": {
						Type:     schema.TypeString,
						Computed: true,
						Optional: true,
					},
					"payload": {
						Type:     schema.TypeString,
						Optional: true,
					},
				},
			},
			MaxItems:     1,
			ExactlyOneOf: validWebhookTypes,
		},
		"pager_duty": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"service_key": {
						Type:     schema.TypeString,
						Optional: true,
					},
				},
			},
			MaxItems:     1,
			ExactlyOneOf: validWebhookTypes,
		},
		"email_group": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"emails": {
						Type:     schema.TypeSet,
						Required: true,
						Elem: &schema.Schema{
							Type:             schema.TypeString,
							ValidateDiagFunc: mailValidationFunc(),
						},
						Set: schema.HashString,
					},
				},
			},
			MaxItems:     1,
			ExactlyOneOf: validWebhookTypes,
		},
		"microsoft_teams": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"url": {
						Type:             schema.TypeString,
						Required:         true,
						ValidateDiagFunc: urlValidationFunc(),
					},
				},
			},
			MaxItems:     1,
			ExactlyOneOf: validWebhookTypes,
		},
		"jira": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"url": {
						Type:             schema.TypeString,
						Required:         true,
						ValidateDiagFunc: urlValidationFunc(),
					},
					"api_token": {
						Type:     schema.TypeString,
						Required: true,
					},
					"email": {
						Type:             schema.TypeString,
						Required:         true,
						ValidateDiagFunc: mailValidationFunc(),
					},
					"project_key": {
						Type:     schema.TypeString,
						Required: true,
					},
				},
			},
			MaxItems:     1,
			ExactlyOneOf: validWebhookTypes,
		},
		"opsgenie": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"url": {
						Type:             schema.TypeString,
						Required:         true,
						ValidateDiagFunc: urlValidationFunc(),
					},
				},
			},
			MaxItems:     1,
			ExactlyOneOf: validWebhookTypes,
		},
		"sendlog": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"url": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"uuid": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"payload": {
						Type:     schema.TypeString,
						Optional: true,
					},
				},
			},
			MaxItems:     1,
			ExactlyOneOf: validWebhookTypes,
		},
		"demisto": {
			Type:     schema.TypeList,
			Optional: true,
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"url": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"uuid": {
						Type:     schema.TypeString,
						Computed: true,
					},
					"payload": {
						Type:     schema.TypeString,
						Optional: true,
					},
				},
			},
			MaxItems:     1,
			ExactlyOneOf: validWebhookTypes,
		},
	}
}

func extractCreateWebhookRequest(d *schema.ResourceData) (string, error) {
	webhookTypeStr := From(validWebhookTypes).FirstWith(func(key interface{}) bool {
		return len(d.Get(key.(string)).([]interface{})) > 0
	}).(string)
	webhookType := d.Get(webhookTypeStr).([]interface{})[0].(map[string]interface{})

	var webhookTypeMap map[string]interface{}
	switch webhookTypeStr {
	case "slack":
		webhookTypeMap = expandSlack(webhookType)
	case "custom":
		webhookTypeMap = expandWebhook(webhookType)
	case "pager_duty":
		webhookTypeMap = expandPagerDuty(webhookType)
	case "sendlog":
		webhookTypeMap = expandSendlog(webhookType)
	case "email_group":
		webhookTypeMap = expandEmailGroup(webhookType)
	case "microsoft_teams":
		webhookTypeMap = expandMicrosoftTeams(webhookType)
	case "jira":
		webhookTypeMap = expandJira(webhookType)
	case "opsgenie":
		webhookTypeMap = expandOpsgenie(webhookType)
	case "demisto":
		webhookTypeMap = expandDemisto(webhookType)
	}

	webhookTypeMap["alias"] = d.Get("name").(string)

	if d.Id() != "" {
		if n, err := strconv.Atoi(d.Id()); err != nil {
			return "", err
		} else {
			webhookTypeMap["id"] = n
		}
	}

	if webhookRequestBody, err := json.Marshal(webhookTypeMap); err != nil {
		return "", err
	} else {
		return string(webhookRequestBody), nil
	}
}

func expandSlack(webhookType map[string]interface{}) map[string]interface{} {
	url := webhookType["url"].(string)
	return map[string]interface{}{
		"integration_type_id": 0,
		"integration_type": map[string]interface{}{
			"label": "Slack",
			"icon":  "/assets/settings/slack-48.png",
			"id":    0,
		},
		"url": url,
	}
}

func expandWebhook(webhookType map[string]interface{}) map[string]interface{} {
	url := webhookType["url"].(string)
	method := valueFormat(webhookType["method"].(string))
	integrationTypeFields := toArrayFormat([]string{
		integrationTypeFieldsFormat("method", method),
		integrationTypeFieldsFormat("headers", webhookType["headers"].(string)),
		integrationTypeFieldsFormat("payload", webhookType["payload"].(string)),
	})
	return map[string]interface{}{
		"url":                     url,
		"integration_type_fields": integrationTypeFields,
		"integration_type_id":     1,
		"integration_type": map[string]interface{}{
			"label": "WebHook",
			"icon":  "/assets/webhook.png",
			"id":    1,
		},
	}
}

func expandPagerDuty(webhookType map[string]interface{}) map[string]interface{} {
	serviceKey := valueFormat(webhookType["service_key"].(string))
	integrationTypeFields := toArrayFormat([]string{
		integrationTypeFieldsFormat("serviceKey", serviceKey),
	})
	return map[string]interface{}{
		"integration_type_fields": integrationTypeFields,
		"integration_type_id":     2,
		"integration_type": map[string]interface{}{
			"label": "PagerDuty",
			"icon":  "/assets/settings/pagerDuty.png",
			"id":    2,
		},
	}
}

func expandSendlog(webhookType map[string]interface{}) map[string]interface{} {
	url := webhookType["url"].(string)
	integrationTypeFields := toArrayFormat([]string{
		integrationTypeFieldsFormat("payload", webhookType["payload"].(string)),
	})
	return map[string]interface{}{
		"url":                     url,
		"integration_type_fields": integrationTypeFields,
		"integration_type_id":     3,
		"integration_type": map[string]interface{}{
			"label": "SendLog",
			"icon":  "/assets/invite.png",
			"id":    3,
		},
	}
}

func expandEmailGroup(m map[string]interface{}) map[string]interface{} {
	emails := interfaceSliceToStringSlice(m["emails"].(*schema.Set).List())
	emailsStr := sliceToString(emails)
	integrationTypeFields := toArrayFormat([]string{
		integrationTypeFieldsFormat("payload", emailsStr),
	})
	return map[string]interface{}{
		"integration_type_fields": integrationTypeFields,
		"integration_type_id":     4,
		"integration_type": map[string]interface{}{
			"label": "Email Group",
			"icon":  "/assets/settings/pagerDuty.png",
			"id":    4,
		},
	}
}

func expandMicrosoftTeams(webhookType map[string]interface{}) map[string]interface{} {
	url := webhookType["url"].(string)
	return map[string]interface{}{
		"url":                 url,
		"integration_type_id": 5,
		"integration_type": map[string]interface{}{
			"label": "Microsoft Teams",
			"icon":  "/assets/settings/teams.png",
			"id":    5,
		},
	}
}

func expandJira(webhookType map[string]interface{}) map[string]interface{} {
	url := webhookType["url"].(string)
	integrationTypeFields := toArrayFormat([]string{
		integrationTypeFieldsFormat("apiToken", valueFormat(webhookType["api_token"].(string))),
		integrationTypeFieldsFormat("email", valueFormat(webhookType["email"].(string))),
		integrationTypeFieldsFormat("projectKey", valueFormat(webhookType["project_key"].(string))),
	})
	return map[string]interface{}{
		"url":                     url,
		"integration_type_fields": integrationTypeFields,
		"integration_type_id":     6,
		"integration_type": map[string]interface{}{
			"label": "Jira",
			"icon":  "/assets/settings/jira.png",
			"id":    6,
		},
	}
}

func expandOpsgenie(webhookType map[string]interface{}) map[string]interface{} {
	url := webhookType["url"].(string)
	return map[string]interface{}{
		"url":                 url,
		"integration_type_id": 7,
		"integration_type": map[string]interface{}{
			"label": "Opsgenie",
			"icon":  "/assets/settings/opsgenie.png",
			"id":    7,
		},
	}
}

func expandDemisto(webhookType map[string]interface{}) map[string]interface{} {
	integrationTypeFields := toArrayFormat([]string{
		integrationTypeFieldsFormat("payload", webhookType["payload"].(string)),
	})
	return map[string]interface{}{
		"integration_type_fields": integrationTypeFields,
		"integration_type_id":     8,
		"integration_type": map[string]interface{}{
			"label": "Demisto",
			"icon":  "/assets/settings/demisto.png",
			"id":    8,
		},
	}
}

func setWebhook(d *schema.ResourceData, resp map[string]interface{}) diag.Diagnostics {
	var webhookTypeStr string
	var webhook interface{}
	switch resp["integration_type_id"].(float64) {
	case 0:
		webhookTypeStr = "slack"
		webhook = flattenSlack(resp)
	case 1:
		webhookTypeStr = "custom"
		webhook = flattenWebhook(resp)
	case 2:
		webhookTypeStr = "pager_duty"
		webhook = flattenPagerDuty(resp)
	case 3:
		webhookTypeStr = "sendlog"
		webhook = flattenSendlog(resp)
	case 4:
		webhookTypeStr = "email_group"
		webhook = flattenEmailGroup(resp)
	case 5:
		webhookTypeStr = "microsoft_teams"
		webhook = flattenMicrosoftTeams(resp)
	case 6:
		webhookTypeStr = "jira"
		webhook = flattenJira(resp)
	case 7:
		webhookTypeStr = "opsgenie"
		webhook = flattenOpsgenie(resp)
	case 8:
		webhookTypeStr = "demisto"
		webhook = flattenDemisto(resp)
	}

	if err := d.Set(webhookTypeStr, webhook); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("name", resp["alias"]); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func flattenSlack(resp map[string]interface{}) interface{} {
	return []map[string]interface{}{
		{
			"url": resp["url"],
		},
	}
}

func flattenWebhook(resp map[string]interface{}) interface{} {
	integrationTypeFieldsStr := resp["integration_type_fields"].(string)
	integrationTypeFields := extractIntegrationTypeFields(integrationTypeFieldsStr)
	payload := marshalMap(integrationTypeFields["payload"])
	headers := marshalMap(integrationTypeFields["headers"])
	return []map[string]interface{}{
		{
			"url":     resp["url"],
			"uuid":    integrationTypeFields["uuid"],
			"method":  integrationTypeFields["method"],
			"headers": headers,
			"payload": payload,
		},
	}
}

func flattenPagerDuty(resp map[string]interface{}) interface{} {
	integrationTypeFieldsStr := resp["integration_type_fields"].(string)
	integrationTypeFields := extractIntegrationTypeFields(integrationTypeFieldsStr)
	serviceKey := integrationTypeFields["serviceKey"].(string)
	return []map[string]interface{}{
		{
			"service_key": serviceKey,
		},
	}
}

func flattenSendlog(resp map[string]interface{}) interface{} {
	integrationTypeFieldsStr := resp["integration_type_fields"].(string)
	integrationTypeFields := extractIntegrationTypeFields(integrationTypeFieldsStr)
	payload := marshalMap(integrationTypeFields["payload"])
	return []map[string]interface{}{
		{
			"url":     resp["url"],
			"payload": payload,
		},
	}
}

func flattenEmailGroup(resp map[string]interface{}) interface{} {
	integrationTypeFieldsStr := resp["integration_type_fields"].(string)
	integrationTypeFields := extractIntegrationTypeFields(integrationTypeFieldsStr)
	return []map[string]interface{}{
		{
			"emails": integrationTypeFields["payload"],
		},
	}
}

func flattenMicrosoftTeams(resp map[string]interface{}) interface{} {
	return []map[string]interface{}{
		{
			"url": resp["url"],
		},
	}
}

func flattenJira(resp map[string]interface{}) interface{} {
	integrationTypeFieldsStr := resp["integration_type_fields"].(string)
	integrationTypeFields := extractIntegrationTypeFields(integrationTypeFieldsStr)
	return []map[string]interface{}{
		{
			"api_token":   integrationTypeFields["apiToken"],
			"email":       integrationTypeFields["email"],
			"project_key": integrationTypeFields["projectKey"],
			"url":         resp["url"],
		},
	}
}

func flattenOpsgenie(resp map[string]interface{}) interface{} {
	return []map[string]interface{}{
		{
			"url": resp["url"],
		},
	}
}

func flattenDemisto(resp map[string]interface{}) interface{} {
	integrationTypeFieldsStr := resp["integration_type_fields"].(string)
	integrationTypeFields := extractIntegrationTypeFields(integrationTypeFieldsStr)
	payload := marshalMap(integrationTypeFields["payload"])
	return []map[string]interface{}{
		{
			"url":     resp["url"],
			"payload": payload,
		},
	}
}

func integrationTypeFieldsFormat(key, value string) string {
	return fmt.Sprintf("{\"name\":\"%s\",\"value\":%s}", key, value)
}

func valueFormat(str string) string {
	return fmt.Sprintf("\"%s\"", str)
}

func toArrayFormat(integrationTypeFields []string) string {
	return fmt.Sprintf("[%s]", strings.Join(integrationTypeFields, ", "))
}

func extractIntegrationTypeFields(str string) map[string]interface{} {
	var fields []map[string]interface{}
	json.Unmarshal([]byte(str), &fields)
	results := make(map[string]interface{})
	for _, field := range fields {
		name := field["name"].(string)
		value := field["value"]
		results[name] = value
	}
	return results
}

func marshalMap(v interface{}) string {
	payload, _ := json.Marshal(v)
	return string(payload)
}
