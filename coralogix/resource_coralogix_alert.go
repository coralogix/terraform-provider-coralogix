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
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"text": {
							Type:     schema.TypeString,
							Required: true,
						},
						"applications": {
							Type:     schema.TypeSet,
							Required: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"subsystems": {
							Type:     schema.TypeSet,
							Required: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"severities": {
							Type:     schema.TypeSet,
							Required: true,
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
				ForceNew: true,
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
							Type:     schema.TypeSet,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
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
								"prometheus",
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
								"HOUR",
								"DAY",
							}, false),
						},
						"relative_timeframe": {
							Type:     schema.TypeString,
							Optional: true,
							ForceNew: true,
							ValidateFunc: validation.StringInSlice([]string{
								"HOUR",
								"DAY",
								"WEEK",
								"MONTH",
							}, false),
							Default: "",
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
							Type:     schema.TypeSet,
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
				Type:     schema.TypeSet,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"notifications": {
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"emails": {
							Type:     schema.TypeSet,
							Optional: true,
							ForceNew: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"integrations": {
							Type:     schema.TypeSet,
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
	alertType := d.Get("type").(string)
	filter := getFirstOrNil(d.Get("filter").(*schema.Set).List())
	var newFilter = make(map[string]interface{}, 7)
	newFilter["filter_type"] = alertType
	newFilter["text"] = filter.(map[string]interface{})["text"].(string)
	newFilter["severity"] = filter.(map[string]interface{})["severities"].(*schema.Set).List()
	newFilter["application_name"] = filter.(map[string]interface{})["applications"].(*schema.Set).List()
	newFilter["subsystem_name"] = filter.(map[string]interface{})["subsystems"].(*schema.Set).List()
	newFilter["alias"] = filter.(map[string]interface{})["alias"].(string)
	condition := getFirstOrNil(d.Get("condition").(*schema.Set).List())
	if condition == nil {
		if alertType != "text" {
			str := "alert of type " + d.Get("type").(string) + " must have condition block"
			return errors.New(str)
		}
	}
	ratio := getFirstOrNil(d.Get("ratio").(*schema.Set).List())
	if alertType == "ratio" {
		if ratio == nil {
			return errors.New("alert of type ratio must have ratio block")
		}
		// specific check until filter block is optional completly
		if newFilter["alias"] == "" {
			return errors.New("alert of type ratio must have alias defined on filter block")
		}
		ratio := ratio.(map[string]interface{})
		newRatio := make(map[string]interface{}, 6)
		newRatio["severity"] = ratio["severities"].(*schema.Set).List()
		newRatio["application_name"] = ratio["applications"].(*schema.Set).List()
		newRatio["subsystem_name"] = ratio["subsystems"].(*schema.Set).List()
		newRatio["group_by"] = ratio["group_by"].(*schema.Set).List()
		newRatio["text"] = ratio["text"]
		newRatio["alias"] = ratio["alias"]
		newFilter["ratioAlerts"] = []interface{}{newRatio}
	}
	metric := getFirstOrNil(d.Get("metric").(*schema.Set).List())
	if alertType == "metric" {
		if metric == nil {
			return errors.New("alert of type metric must have metric block")
		}
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
	content := getFirstOrNil(d.Get("content").(*schema.Set).List())
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
		"name":                 d.Get("name").(string),
		"severity":             d.Get("severity").(string),
		"is_active":            d.Get("enabled").(bool),
		"description":          d.Get("description").(string),
		"log_filter":           newFilter,
		"condition":            condition,
		"notifications":        newNotification,
		"active_when":          newSchedule,
		"notif_payload_filter": content,
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
	d.Set("filter", flattenAlertFilter(alert))
	d.Set("metric", flattenAlertMetric(alert))
	d.Set("ratio", flattenAlertRatio(alert))
	d.Set("condition", flattenAlertCondition(alert))
	d.Set("notifications", flattenAlertNotifications(alert))
	d.Set("schedule", flattenAlertSchedule(alert))
	if content := alert["notif_payload_filter"]; content != nil && len(content.([]interface{})) > 0 {
		d.Set("content", content)
	}
	d.Set("description", alert["description"].(string))
	d.SetId(alert["unique_identifier"].(string))
	return nil
}

func resourceCoralogixAlertUpdate(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	if d.HasChanges("name", "severity", "enabled", "type", "description") {
		alert, err := apiClient.Put("/external/alerts", map[string]interface{}{
			"unique_identifier": d.Id(),
			"name":              d.Get("name").(string),
			"description":       d.Get("description").(string),
			"severity":          d.Get("severity").(string),
			"is_active":         d.Get("enabled").(bool),
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

	_, err := apiClient.Request("DELETE", "/external/alerts", map[string]interface{}{"unique_identifier": d.Id()})
	if err != nil {
		return err
	}

	return nil
}
