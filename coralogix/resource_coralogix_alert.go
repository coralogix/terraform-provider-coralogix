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
					"unique_count",
					"relative_time",
					"metric",
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
			"metric": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"field": {
							Type:         schema.TypeString,
							Required:     true,
							ForceNew:     true,
							ValidateFunc: validation.StringIsNotEmpty,
						},
						"source": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
							ValidateFunc: validation.StringInSlice([]string{
								"logs2metrics",
								"Prometheus",
							}, false),
						},
						"arithmetic_operator": {
							Type:         schema.TypeInt,
							Required:     true,
							ForceNew:     true,
							ValidateFunc: validation.IntBetween(0, 5),
						},
						"arithmetic_operator_modifier": {
							Type:         schema.TypeInt,
							Optional:     true,
							ForceNew:     true,
							Default:      0,
							ValidateFunc: validation.IntAtLeast(0),
						},
						"sample_threshold_percentage": {
							Type:     schema.TypeInt,
							Required: true,
							ForceNew: true,
							ValidateFunc: validation.All(
								validation.IntBetween(0, 90),
								validation.IntDivisibleBy(10),
							),
						},
						"non_null_percentage": {
							Type:     schema.TypeInt,
							Required: true,
							ForceNew: true,
							ValidateFunc: validation.All(
								validation.IntBetween(0, 100),
								validation.IntDivisibleBy(10),
							),
						},
						"swap_null_values": {
							Type:     schema.TypeBool,
							Optional: true,
							ForceNew: true,
							Default:  false,
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
							ForceNew: true,
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
							ForceNew:     true,
							ValidateFunc: validation.IntAtLeast(0),
						},
						"timeframe": {
							Type:     schema.TypeString,
							Required: true,
							ForceNew: true,
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
							ForceNew: true,
							Default:  "",
						},
						"unique_count_key": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
							Default:  "",
						},
					},
				},
			},
			"schedule": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"days": {
							Type:     schema.TypeList,
							Required: true,
							ForceNew: true,
							MinItems: 1,
							MaxItems: 7,
							Elem: &schema.Schema{
								Type: schema.TypeString,
								ValidateFunc: validation.StringInSlice([]string{
									"Mo",
									"Tu",
									"We",
									"Th",
									"Fr",
									"Sa",
									"Su",
								}, false),
							},
						},
						"start": {
							Type:         schema.TypeString,
							Required:     true,
							ForceNew:     true,
							ValidateFunc: validation.StringIsNotEmpty,
						},
						"end": {
							Type:         schema.TypeString,
							Required:     true,
							ForceNew:     true,
							ValidateFunc: validation.StringIsNotEmpty,
						},
					},
				},
			},
			"content": {
				Type:     schema.TypeList,
				Optional: true,
				Default:  nil,
				ForceNew: true,
				MinItems: 1,
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: validation.StringIsNotEmpty,
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
			"condition_type":   "",
			"threshold":        0,
			"timeframe":        "",
			"group_by":         "",
			"unique_count_key": "",
		}
	}

	metric := getFirstOrNil(d.Get("metric").(*schema.Set).List())
	if d.Get("type").(string) == "metric" && metric != nil {
		metric := metric.(map[string]interface{})
		condition := condition.(map[string]interface{})
		condition["metric_field"] = metric["field"]
		condition["metric_source"] = metric["source"]
		condition["arithmetic_operator"] = metric["arithmetic_operator"]
		condition["arithmetic_operator_modifier"] = metric["arithmetic_operator_modifier"]
		condition["sample_threshold_percentage"] = metric["sample_threshold_percentage"]
		condition["non_null_percentage"] = metric["non_null_percentage"]
		condition["swap_null_values"] = metric["swap_null_values"]
	}

	alertParameters := map[string]interface{}{
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
	}

	schedule := getFirstOrNil(d.Get("schedule").(*schema.Set).List())
	if schedule != nil {
		alertParameters["active_when"] = map[string]interface{}{
			"timeframes": []interface{}{
				map[string]interface{}{
					"days_of_week":    transformWeekList(d.Get("schedule").(*schema.Set).List()[0].(map[string]interface{})["days"].([]interface{})),
					"activity_starts": d.Get("schedule").(*schema.Set).List()[0].(map[string]interface{})["start"].(string),
					"activity_ends":   d.Get("schedule").(*schema.Set).List()[0].(map[string]interface{})["end"].(string),
				},
			},
		}
	}

	if d.Get("content") != nil {
		alertParameters["notif_payload_filter"] = d.Get("content").([]interface{})
	}

	alert, err := apiClient.Post("/external/alerts", alertParameters)
	if err != nil {
		return err
	}

	d.SetId(alert["unique_identifier"].([]interface{})[0].(string))

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
	d.Set("enabled", alert["is_active"].(bool))
	d.Set("type", alert["log_filter"].(map[string]interface{})["filter_type"].(string))
	d.Set("filter", []interface{}{flattenAlertFilter(alert)})
	d.Set("metric", []interface{}{flattenAlertMetric(alert)})
	d.Set("condition", []interface{}{flattenAlertCondition(alert)})
	d.Set("notifications", []interface{}{flattenAlertNotifications(alert)})

	if alert["description"] != nil {
		d.Set("description", alert["description"].(string))
	} else {
		d.Set("description", "")
	}

	if alert["notif_payload_filter"] != nil && len(alert["notif_payload_filter"].([]interface{})) > 0 {
		d.Set("content", alert["notif_payload_filter"])
	}

	if alert["active_when"] != nil && len(alert["active_when"].(map[string]interface{})["timeframes"].([]interface{})) > 0 {
		d.Set("schedule", []interface{}{flattenAlertSchedule(alert)})
	}

	d.SetId(alert["unique_identifier"].(string))

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
		d.SetId(alert["unique_identifier"].(string))
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
