package coralogix

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceCoralogixAlert() *schema.Resource {
	return &schema.Resource{
		Create: resourceCoralogixAlertCreate,
		Read:   resourceCoralogixAlertRead,
		Update: resourceCoralogixAlertUpdate,
		Delete: resourceCoralogixAlertDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringIsNotEmpty,
			},
			"severity": {
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: validation.StringInSlice([]string{
					"info",
					"warning",
					"critical",
				}, false),
			},
			"enabled": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"type": {
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: validation.StringInSlice([]string{
					"text",
					"ratio",
				}, false),
			},
			"description": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"filter": {
				Type:     schema.TypeSet,
				Required: true,
				ForceNew: true,
				MinItems: 1,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"text": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "",
						},
						"applications": {
							Type:     schema.TypeSet,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"subsystems": {
							Type:     schema.TypeSet,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"severities": {
							Type:     schema.TypeSet,
							Optional: true,
							MaxItems: 6,
							Elem: &schema.Schema{
								Type: schema.TypeString,
								ValidateFunc: validation.StringInSlice([]string{
									"debug",
									"verbose",
									"info",
									"warning",
									"error",
									"critical",
								}, false),
							},
						},
					},
				},
			},
			"condition": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"condition_type": {
							Type:     schema.TypeString,
							Required: true,
							ValidateFunc: validation.StringInSlice([]string{
								"less_than",
								"more_than",
								"more_than_usual",
								"new_value",
							}, false),
						},
						"threshold": {
							Type:         schema.TypeInt,
							Required:     true,
							ValidateFunc: validation.IntAtLeast(0),
						},
						"timeframe": {
							Type:     schema.TypeString,
							Required: true,
							ValidateFunc: validation.StringInSlice([]string{
								"5MIN",
								"10MIN",
								"30MIN",
								"1H",
								"2H",
								"3H",
								"4H",
								"6H",
								"12H",
								"24H",
								"48H",
								"72H",
								"1W",
								"1M",
								"2M",
								"3M",
							}, false),
						},
						"group_by": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "",
						},
					},
				},
			},
			"notifications": {
				Type:     schema.TypeSet,
				Optional: true,
				Default:  nil,
				ForceNew: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"emails": {
							Type:     schema.TypeList,
							Optional: true,
							ForceNew: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"integrations": {
							Type:     schema.TypeList,
							Optional: true,
							ForceNew: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
		},
	}
}

func resourceCoralogixAlertCreate(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	condition := getFirstOrNil(d.Get("condition").(*schema.Set).List())
	if condition == nil {
		condition = map[string]interface{}{
			"condition_type": "",
			"threshold":      0,
			"timeframe":      "",
			"group_by":       "",
		}
	}

	alert, err := apiClient.Post("/external/alerts", map[string]interface{}{
		"name":        d.Get("name").(string),
		"severity":    d.Get("severity").(string),
		"is_active":   d.Get("enabled").(bool),
		"description": d.Get("description").(string),
		"log_filter": map[string]interface{}{
			"filter_type":      d.Get("type").(string),
			"text":             d.Get("filter").(*schema.Set).List()[0].(map[string]interface{})["text"].(string),
			"severity":         d.Get("filter").(*schema.Set).List()[0].(map[string]interface{})["severities"].(*schema.Set).List(),
			"application_name": d.Get("filter").(*schema.Set).List()[0].(map[string]interface{})["applications"].(*schema.Set).List(),
			"subsystem_name":   d.Get("filter").(*schema.Set).List()[0].(map[string]interface{})["subsystems"].(*schema.Set).List(),
		},
		"condition":     condition,
		"notifications": getFirstOrNil(d.Get("notifications").(*schema.Set).List()),
	})
	if err != nil {
		return err
	}

	d.SetId(alert["alert_id"].([]interface{})[0].(string))

	return resourceCoralogixAlertRead(d, meta)
}

func resourceCoralogixAlertRead(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	alertsList, err := apiClient.Get("/external/alerts")
	if err != nil {
		return err
	}

	alert, err := getAlertByID(alertsList["alerts"].([]interface{}), d.Id())
	if err != nil {
		return err
	}

	d.Set("name", alert["name"].(string))
	d.Set("severity", alert["severity"].(string))
	d.Set("description", alert["description"].(string))
	d.Set("enabled", alert["is_active"].(bool))
	d.Set("type", alert["log_filter"].(map[string]interface{})["filter_type"].(string))
	d.Set("filter", []interface{}{flattenAlertFilter(alert)})
	d.Set("condition", []interface{}{flattenAlertCondition(alert)})
	d.Set("notifications", []interface{}{flattenAlertNotifications(alert)})

	d.SetId(alert["id"].(string))

	return nil
}

func resourceCoralogixAlertUpdate(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	if d.HasChanges("name", "severity", "enabled", "type", "description") {
		alert, err := apiClient.Put("/external/alerts", map[string]interface{}{
			"id":          d.Id(),
			"name":        d.Get("name").(string),
			"description": d.Get("description").(string),
			"severity":    d.Get("severity").(string),
			"is_active":   d.Get("enabled").(bool),
		})
		if err != nil {
			return err
		}
		d.SetId(alert["alert_id"].(string))
	}

	return resourceCoralogixAlertRead(d, meta)
}

func resourceCoralogixAlertDelete(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	_, err := apiClient.Request("DELETE", "/external/alerts", map[string]interface{}{"id": d.Id()})
	if err != nil {
		return err
	}

	return nil
}
