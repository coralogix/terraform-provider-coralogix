package coralogix

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func dataSourceCoralogixAlert() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceCoralogixAlertRead,

		Schema: map[string]*schema.Schema{
			"alert_id": {
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
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
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
		},
	}
}

func dataSourceCoralogixAlertRead(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	alertsList, err := apiClient.Get("/external/alerts")
	if err != nil {
		return err
	}

	alert, err := getAlertByID(alertsList["alerts"].([]interface{}), d.Get("alert_id").(string))
	if err != nil {
		return err
	}

	d.Set("name", alert["name"].(string))
	d.Set("severity", alert["severity"].(string))
	d.Set("enabled", alert["is_active"].(bool))
	d.Set("type", alert["log_filter"].(map[string]interface{})["filter_type"].(string))
	d.Set("filter", []interface{}{flattenAlertFilter(alert)})
	d.Set("condition", []interface{}{flattenAlertCondition(alert)})
	d.Set("notifications", []interface{}{flattenAlertNotifications(alert)})

	d.SetId(alert["id"].(string))

	return nil
}
