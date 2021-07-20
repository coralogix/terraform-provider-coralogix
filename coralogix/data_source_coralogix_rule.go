package coralogix

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func dataSourceCoralogixRule() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceCoralogixRuleRead,

		Schema: map[string]*schema.Schema{
			"rule_id": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.IsUUID,
			},
			"rules_group_id": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.IsUUID,
			},
			"name": {
				Type:     schema.TypeString,
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
			"order": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"enabled": {
				Type:     schema.TypeBool,
				Computed: true,
			},
			"rule_matcher": {
				Type:     schema.TypeSet,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"field": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"constraint": {
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
			"expression": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"source_field": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"destination_field": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"replace_value": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func dataSourceCoralogixRuleRead(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	rule, err := apiClient.Get("/external/action/" + d.Get("rule_id").(string) + "/rule/" + d.Get("rules_group_id").(string))
	if err != nil {
		return err
	}

	d.Set("name", rule["name"].(string))
	d.Set("type", rule["type"].(string))
	d.Set("description", rule["description"].(string))
	d.Set("order", rule["order"].(float64))
	d.Set("enabled", rule["enabled"].(bool))
	d.Set("rule_matcher", flattenRuleMatchers(rule["ruleMatchers"].([]interface{})))
	d.Set("expression", rule["rule"].(string))
	d.Set("source_field", rule["sourceField"].(string))
	d.Set("destination_field", rule["destinationField"].(string))
	d.Set("replace_value", rule["replaceNewVal"].(string))

	d.SetId(rule["id"].(string))

	return nil
}
