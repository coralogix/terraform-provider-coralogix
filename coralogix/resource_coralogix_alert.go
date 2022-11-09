package coralogix

import (
	"errors"

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
					"error",
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
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"text": {
							Type:     schema.TypeString,
							Optional: true,
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
						"alias": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			"ratio": {
				Type:     schema.TypeSet,
				Optional: true,
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
						"alias": {
							Type:     schema.TypeString,
							Required: true,
						},
						"group_by": {
							Type:     schema.TypeList,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
			"metric": {
				Type:     schema.TypeSet,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"field": {
							Type:         schema.TypeString,
							Optional:     true,
							ValidateFunc: validation.StringIsNotEmpty,
						},
						"source": {
							Type:     schema.TypeString,
							Optional: true,
							ValidateFunc: validation.StringInSlice([]string{
								"logs2metrics",
								"prometheus",
							}, false),
						},
						"arithmetic_operator": {
							Type:         schema.TypeInt,
							Optional:     true,
							Default:      0,
							ValidateFunc: validation.IntBetween(0, 5),
						},
						"arithmetic_operator_modifier": {
							Type:         schema.TypeInt,
							Optional:     true,
							Default:      0,
							ValidateFunc: validation.IntBetween(1, 99),
						},
						"sample_threshold_percentage": {
							Type:     schema.TypeInt,
							Required: true,
							ValidateFunc: validation.All(
								validation.IntBetween(0, 90),
								validation.IntDivisibleBy(10),
							),
						},
						"non_null_percentage": {
							Type:     schema.TypeInt,
							Required: true,
							ValidateFunc: validation.All(
								validation.IntBetween(0, 100),
								validation.IntDivisibleBy(10),
							),
						},
						"swap_null_values": {
							Type:     schema.TypeBool,
							Optional: true,
							Default:  false,
						},
						"promql_text": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "",
						},
					},
				},
			},
			"condition": {
				Type:     schema.TypeSet,
				Optional: true,
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
							Type:         schema.TypeFloat,
							Required:     true,
							ValidateFunc: validation.FloatAtLeast(0),
						},
						"timeframe": {
							Type:     schema.TypeString,
							Required: true,
							ValidateFunc: validation.StringInSlice([]string{
								"1MIN",
								"5MIN",
								"10MIN",
								"15MIN",
								"30MIN",
								"1H",
								"2H",
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
								"HOUR",
								"DAY",
							}, false),
						},
						"relative_timeframe": {
							Type:     schema.TypeString,
							Optional: true,
							ValidateFunc: validation.StringInSlice([]string{
								"HOUR",
								"DAY",
								"WEEK",
								"MONTH",
							}, false),
							Default: "",
						},
						"group_by": {
							Type:       schema.TypeString,
							Optional:   true,
							Default:    "",
							Deprecated: "group_by is no longer being used and will be deprecated in the next release. Please use 'group_by_array'",
						},
						"group_by_array": {
							Type:     schema.TypeList,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"unique_count_key": {
							Type:     schema.TypeString,
							Optional: true,
							Default:  "",
						},
					},
				},
			},
			"schedule": {
				Type:     schema.TypeSet,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"days": {
							Type:     schema.TypeSet,
							Required: true,
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
							ValidateFunc: validation.StringIsNotEmpty,
						},
						"end": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringIsNotEmpty,
						},
					},
				},
			},
			"content": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"notifications": {
				Type:     schema.TypeSet,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"emails": {
							Type:     schema.TypeSet,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"integrations": {
							Type:     schema.TypeSet,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
			"alert_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"notify_every": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  60,
			},
			"notify_group_by_only": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"notify_per_group_by": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
		},
	}
}

func resourceCoralogixAlertCreate(d *schema.ResourceData, meta interface{}) error {
	if err := alertValuesValidation(d); err != nil {
		return err
	}
	apiClient := meta.(*Client)
	alertType := d.Get("type").(string)
	notifyEvery := d.Get("notify_every").(int)
	notifyPerGroupBy := d.Get("notify_per_group_by").(bool)
	notifyGroupByOnly := d.Get("notify_group_by_only").(bool)
	filter := getFirstOrNil(d.Get("filter").(*schema.Set).List())
	var newFilter = make(map[string]interface{}, 7)
	newFilter["filter_type"] = alertType
	newFilter["text"] = filter.(map[string]interface{})["text"].(string)
	newFilter["severity"] = filter.(map[string]interface{})["severities"].(*schema.Set).List()
	newFilter["application_name"] = filter.(map[string]interface{})["applications"].(*schema.Set).List()
	newFilter["subsystem_name"] = filter.(map[string]interface{})["subsystems"].(*schema.Set).List()
	newFilter["alias"] = filter.(map[string]interface{})["alias"].(string)
	condition := getFirstOrNil(d.Get("condition").(*schema.Set).List())
	var newCondition = make(map[string]interface{}, 6)
	if condition != nil {
		condition := condition.(map[string]interface{})
		newCondition["condition_type"] = condition["condition_type"].(string)
		newCondition["threshold"] = condition["threshold"].(float64)
		newCondition["timeframe"] = condition["timeframe"].(string)
		newCondition["relative_timeframe"] = condition["relative_timeframe"].(string)
		newCondition["unique_count_key"] = condition["unique_count_key"].(string)
		if condition["group_by"] != "" {
			newCondition["group_by"] = condition["group_by"]
		} else if len(condition["group_by_array"].([]interface{})) != 0 {
			// new_value alert accept only one string
			if newCondition["condition_type"] == "new_value" {
				newCondition["group_by"] = condition["group_by_array"].([]interface{})[0]
			} else {
				newCondition["group_by"] = condition["group_by_array"].([]interface{})
			}
		}
	} else {
		newCondition = nil
	}
	if alertType == "ratio" {
		ratio := getFirstOrNil(d.Get("ratio").(*schema.Set).List()).(map[string]interface{})
		newRatio := make(map[string]interface{}, 6)
		newRatio["severity"] = ratio["severities"].(*schema.Set).List()
		newRatio["application_name"] = ratio["applications"].(*schema.Set).List()
		newRatio["subsystem_name"] = ratio["subsystems"].(*schema.Set).List()
		newRatio["group_by"] = ratio["group_by"].([]interface{})
		newRatio["text"] = ratio["text"]
		newRatio["alias"] = ratio["alias"]
		newFilter["ratioAlerts"] = []interface{}{newRatio}
	}
	if alertType == "metric" {
		metric := getFirstOrNil(d.Get("metric").(*schema.Set).List()).(map[string]interface{})
		if value := metric["promql_text"]; value != "" {
			newCondition["promql_text"] = value
		} else {
			newCondition["metric_field"] = metric["field"]
			newCondition["metric_source"] = metric["source"]
			newCondition["arithmetic_operator"] = metric["arithmetic_operator"]
		}
		newCondition["arithmetic_operator_modifier"] = metric["arithmetic_operator_modifier"]
		newCondition["sample_threshold_percentage"] = metric["sample_threshold_percentage"]
		newCondition["non_null_percentage"] = metric["non_null_percentage"]
		newCondition["swap_null_values"] = metric["swap_null_values"]
	}
	var newSchedule map[string]interface{}
	schedule := getFirstOrNil(d.Get("schedule").(*schema.Set).List())
	if schedule != nil {
		schedule := schedule.(map[string]interface{})
		newSchedule = make(map[string]interface{}, 1)
		newSchedule["timeframes"] = []interface{}{map[string]interface{}{
			"days_of_week":    transformWeekList(schedule["days"].(*schema.Set).List()),
			"activity_starts": schedule["start"].(string),
			"activity_ends":   schedule["end"].(string),
		},
		}
	}
	content := d.Get("content").(*schema.Set).List()
	var newNotification map[string]interface{}
	notification := getFirstOrNil(d.Get("notifications").(*schema.Set).List())
	if notification != nil {
		notification := notification.(map[string]interface{})
		newNotification = make(map[string]interface{}, 2)
		if _, ok := notification["emails"]; ok {
			newNotification["emails"] = notification["emails"].(*schema.Set).List()
		}
		if _, ok := notification["integrations"]; ok {
			newNotification["integrations"] = notification["integrations"].(*schema.Set).List()
		}
	}
	alertParameters := map[string]interface{}{
		"name":                        d.Get("name").(string),
		"severity":                    d.Get("severity").(string),
		"is_active":                   d.Get("enabled").(bool),
		"description":                 d.Get("description").(string),
		"log_filter":                  newFilter,
		"condition":                   newCondition,
		"notifications":               newNotification,
		"active_when":                 newSchedule,
		"notif_payload_filter":        content,
		"notify_every":                notifyEvery,
		"notify_group_by_only_alerts": notifyGroupByOnly,
		"notify_per_group_by_value":   notifyPerGroupBy,
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

	// sometimes response is status 200 but no alerts
	if alertsList == nil {
		return errors.New("error while getting alerts list")
	}

	alert, err := getAlertByID(alertsList["alerts"].([]interface{}), d.Id())
	if err != nil {
		return err
	}
	d.Set("alert_id", alert["id"].(string))
	d.Set("name", alert["name"].(string))
	d.Set("severity", alert["severity"].(string))
	d.Set("enabled", alert["is_active"].(bool))
	d.Set("type", alert["log_filter"].(map[string]interface{})["filter_type"].(string))
	d.Set("filter", flattenAlertFilter(alert))
	d.Set("metric", flattenAlertMetric(alert))
	d.Set("ratio", flattenAlertRatio(alert))
	// a check for group_by_array and group_by. will be changed when we remove group_by
	condition := getFirstOrNil(d.Get("condition").(*schema.Set).List())
	group_by_array_flag := true
	if condition != nil {
		if condition.(map[string]interface{})["group_by"] != "" {
			// the data came from group_by
			group_by_array_flag = false
		}
		d.Set("condition", flattenAlertCondition(alert, group_by_array_flag))
	} else {
		d.Set("condition", flattenAlertCondition(alert, group_by_array_flag))
	}
	d.Set("notifications", flattenAlertNotifications(alert))
	d.Set("schedule", flattenAlertSchedule(alert))
	if content := alert["notif_payload_filter"]; content != nil && len(content.([]interface{})) > 0 {
		d.Set("content", content)
	}
	d.Set("description", alert["description"].(string))
	d.Set("notify_every", alert["notify_every"].(float64))
	d.Set("notify_group_by_only", alert["notify_group_by_only_alerts"].(bool))
	d.Set("notify_per_group_by", alert["notify_per_group_by_value"].(bool))
	d.SetId(alert["unique_identifier"].(string))
	return nil
}

func resourceCoralogixAlertUpdate(d *schema.ResourceData, meta interface{}) error {
	if err := alertValuesValidation(d); err != nil {
		return err
	}
	apiClient := meta.(*Client)
	alertUpdateParameters := make(map[string]interface{}, 0)
	alertType := d.Get("type").(string)
	// top level fields
	if d.HasChange("name") {
		alertUpdateParameters["name"] = d.Get("name").(string)
	}
	if d.HasChange("severity") {
		alertUpdateParameters["severity"] = d.Get("severity").(string)
	}
	if d.HasChange("enabled") {
		alertUpdateParameters["is_active"] = d.Get("enabled").(bool)
	}
	if d.HasChange("description") {
		alertUpdateParameters["description"] = d.Get("description").(string)
	}
	if d.HasChange("notify_every") {
		alertUpdateParameters["notify_every"] = d.Get("notify_every").(int)
	}
	if d.HasChange("notify_group_by_only") {
		alertUpdateParameters["notify_group_by_only_alerts"] = d.Get("notify_group_by_only").(bool)
	}
	if d.HasChange("notify_per_group_by") {
		alertUpdateParameters["notify_per_group_by_value"] = d.Get("notify_group_by_only").(bool)
	}
	if d.HasChange("content") {
		if contentKey, ok := d.GetOk("content"); ok {
			content := contentKey.(*schema.Set).List()
			alertUpdateParameters["notif_payload_filter"] = content
		} else {
			alertUpdateParameters["notif_payload_filter"] = []interface{}{}
		}
	}
	// log_filter field
	if d.HasChanges("type", "filter", "ratio") {
		filter := getFirstOrNil(d.Get("filter").(*schema.Set).List())
		var newFilter = make(map[string]interface{}, 7)
		newFilter["filter_type"] = alertType
		newFilter["text"] = filter.(map[string]interface{})["text"].(string)
		newFilter["severity"] = filter.(map[string]interface{})["severities"].(*schema.Set).List()
		newFilter["application_name"] = filter.(map[string]interface{})["applications"].(*schema.Set).List()
		newFilter["subsystem_name"] = filter.(map[string]interface{})["subsystems"].(*schema.Set).List()
		newFilter["alias"] = filter.(map[string]interface{})["alias"].(string)
		if d.HasChange("ratio") {
			if ratioKey, ok := d.GetOk("ratio"); ok {
				ratio := getFirstOrNil(ratioKey.(*schema.Set).List()).(map[string]interface{})
				newRatio := make(map[string]interface{}, 6)
				newRatio["severity"] = ratio["severities"].(*schema.Set).List()
				newRatio["application_name"] = ratio["applications"].(*schema.Set).List()
				newRatio["subsystem_name"] = ratio["subsystems"].(*schema.Set).List()
				newRatio["group_by"] = ratio["group_by"].([]interface{})
				newRatio["text"] = ratio["text"]
				newRatio["alias"] = ratio["alias"]
				newFilter["ratioAlerts"] = []interface{}{newRatio}
			}
		}
		alertUpdateParameters["log_filter"] = newFilter
	}
	// condition field
	if d.HasChanges("condition", "metric") {
		if conditionKey, ok := d.GetOk("condition"); ok {
			condition := getFirstOrNil(conditionKey.(*schema.Set).List()).(map[string]interface{})
			newCondition := make(map[string]interface{}, 6)
			newCondition["condition_type"] = condition["condition_type"].(string)
			newCondition["threshold"] = condition["threshold"].(float64)
			newCondition["timeframe"] = condition["timeframe"].(string)
			newCondition["relative_timeframe"] = condition["relative_timeframe"].(string)
			newCondition["unique_count_key"] = condition["unique_count_key"].(string)
			if condition["group_by"] != "" {
				newCondition["group_by"] = condition["group_by"]
			} else if len(condition["group_by_array"].([]interface{})) != 0 {
				// new_value alert accept only one string
				if newCondition["condition_type"] == "new_value" {
					newCondition["group_by"] = condition["group_by_array"].([]interface{})[0]
				} else {
					newCondition["group_by"] = condition["group_by_array"].([]interface{})
				}
			} else {
				newCondition["group_by"] = ""
			}
			alertUpdateParameters["condition"] = newCondition
		} else {
			alertUpdateParameters["condition"] = nil
		}
		if d.HasChange("metric") && alertUpdateParameters["condition"] != nil {
			// check if the change is to metric
			if metricKey, ok := d.GetOk("metric"); ok {
				if logFilter, ok := alertUpdateParameters["log_filter"]; ok {
					// cannot send these fields with metric , already validated that they are zero-valued.
					delete(logFilter.(map[string]interface{}), "severity")
					delete(logFilter.(map[string]interface{}), "application_name")
					delete(logFilter.(map[string]interface{}), "subsystem_name")
					delete(logFilter.(map[string]interface{}), "alias")
				}
				metric := getFirstOrNil(metricKey.(*schema.Set).List()).(map[string]interface{})
				condition := alertUpdateParameters["condition"].(map[string]interface{})
				if value := metric["promql_text"]; value != "" {
					condition["promql_text"] = value
				} else {
					condition["metric_field"] = metric["field"]
					condition["metric_source"] = metric["source"]
					condition["arithmetic_operator"] = metric["arithmetic_operator"]
				}
				condition["arithmetic_operator_modifier"] = metric["arithmetic_operator_modifier"]
				condition["sample_threshold_percentage"] = metric["sample_threshold_percentage"]
				condition["non_null_percentage"] = metric["non_null_percentage"]
				condition["swap_null_values"] = metric["swap_null_values"]
			}
		}
	}
	// active_when field
	if d.HasChange("schedule") {
		if scheduleKey, ok := d.GetOk("schedule"); ok {
			schedule := getFirstOrNil(scheduleKey.(*schema.Set).List()).(map[string]interface{})
			newSchedule := make(map[string]interface{}, 1)
			newSchedule["timeframes"] = []interface{}{map[string]interface{}{
				"days_of_week":    transformWeekList(schedule["days"].(*schema.Set).List()),
				"activity_starts": schedule["start"].(string),
				"activity_ends":   schedule["end"].(string),
			},
			}
			alertUpdateParameters["active_when"] = newSchedule
		} else {
			alertUpdateParameters["active_when"] = map[string]interface{}{
				"timeframes": []interface{}{},
			}
		}
	}
	// notification field
	if d.HasChange("notifications") {
		newNotifications := make(map[string]interface{}, 2)
		if notificationsKey, ok := d.GetOk("notifications"); ok {
			notifications := getFirstOrNil(notificationsKey.(*schema.Set).List()).(map[string]interface{})
			if _, ok := notifications["emails"]; ok {
				newNotifications["emails"] = notifications["emails"].(*schema.Set).List()
			} else {
				newNotifications["emails"] = []interface{}{}
			}
			if _, ok := notifications["integrations"]; ok {
				newNotifications["integrations"] = notifications["integrations"].(*schema.Set).List()
			} else {
				newNotifications["integrations"] = []interface{}{}
			}
			alertUpdateParameters["notifications"] = newNotifications
		} else {
			newNotifications["emails"] = []interface{}{}
			newNotifications["integrations"] = []interface{}{}
			alertUpdateParameters["notifications"] = newNotifications
		}
	}
	// updating uses an alert id and not unique_identifier
	if len(alertUpdateParameters) > 0 {
		alertUpdateParameters["id"] = d.Get("alert_id").(string)
		alert, err := apiClient.Put("/external/alerts", alertUpdateParameters)
		if err != nil {
			return err
		}
		d.SetId(alert["unique_identifier"].(string))
	}
	return resourceCoralogixAlertRead(d, meta)
}

func resourceCoralogixAlertDelete(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	_, err := apiClient.Request("DELETE", "/external/alerts", map[string]interface{}{"unique_identifier": d.Id()})
	if err != nil {
		return err
	}

	return nil
}
