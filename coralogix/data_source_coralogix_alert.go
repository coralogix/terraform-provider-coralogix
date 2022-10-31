package coralogix

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func dataSourceCoralogixAlert() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceCoralogixAlertRead,

		Schema: map[string]*schema.Schema{
			"unique_identifier": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.IsUUID,
			},
			"name": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"severity": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"enabled": {
				Type:     schema.TypeBool,
				Computed: true,
			},
			"type": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"description": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"filter": {
				Type:     schema.TypeSet,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"text": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"applications": {
							Type:     schema.TypeSet,
							Computed: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"subsystems": {
							Type:     schema.TypeSet,
							Computed: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"severities": {
							Type:     schema.TypeSet,
							Computed: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"alias": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
			"ratio": {
				Type:     schema.TypeSet,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"text": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"applications": {
							Type:     schema.TypeSet,
							Computed: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"subsystems": {
							Type:     schema.TypeSet,
							Computed: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"severities": {
							Type:     schema.TypeSet,
							Computed: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"alias": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"group_by": {
							Type:     schema.TypeSet,
							Computed: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
			"metric": {
				Type:     schema.TypeSet,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"field": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"source": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"arithmetic_operator": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"arithmetic_operator_modifier": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"sample_threshold_percentage": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"non_null_percentage": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"swap_null_values": {
							Type:     schema.TypeBool,
							Computed: true,
						},
						"promql_text": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
			"condition": {
				Type:     schema.TypeSet,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"type": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"threshold": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"timeframe": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"group_by": {
							Type:       schema.TypeString,
							Deprecated: "group_by is no longer being used and will be deprecated in the next release. Please use 'group_by_array'",
							Computed:   true,
						},
						"group_by_array": {
							Type:     schema.TypeList,
							Optional: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"relative_timeframe": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"unique_count_key": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
			"schedule": {
				Type:     schema.TypeSet,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"days": {
							Type:     schema.TypeSet,
							Computed: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"start": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"end": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
			"content": {
				Type:     schema.TypeSet,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"notifications": {
				Type:     schema.TypeSet,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"emails": {
							Type:     schema.TypeSet,
							Computed: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
						"integrations": {
							Type:     schema.TypeSet,
							Computed: true,
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
				Computed: true,
			},
			"notify_group_by_only": {
				Type:     schema.TypeBool,
				Computed: true,
			},
			"notify_per_group_by": {
				Type:     schema.TypeBool,
				Computed: true,
			},
		},
	}
}

func dataSourceCoralogixAlertRead(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	alertsList, err := apiClient.Get("/external/alerts")
	if err != nil {
		return err
	}

	alert, err := getAlertByID(alertsList["alerts"].([]interface{}), d.Get("unique_identifier").(string))
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
	// a change for group_by and group_by_array - will be changed when group_by is removed
	d.Set("condition", flattenAlertCondition(alert, true))
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
