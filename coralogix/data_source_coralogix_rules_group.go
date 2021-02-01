package coralogix

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func dataSourceCoralogixRulesGroup() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceCoralogixRulesGroupRead,

		Schema: map[string]*schema.Schema{
			"rules_group_id": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.IsUUID,
			},
			"name": {
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
			"rules": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:     schema.TypeString,
							Computed: true,
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
				},
			},
		},
	}
}

func dataSourceCoralogixRulesGroupRead(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	ruleGroup, err := apiClient.Get("/external/actions/rule/" + d.Get("rules_group_id").(string))
	if err != nil {
		return err
	}

	d.Set("name", ruleGroup["Name"].(string))
	d.Set("order", ruleGroup["Order"].(float64))
	d.Set("enabled", ruleGroup["Enabled"].(bool))
	d.Set("rules", flattenRules(ruleGroup["Rules"].([]interface{})))

	d.SetId(ruleGroup["Id"].(string))

	return nil
}
