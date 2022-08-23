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
			"description": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"enabled": {
				Type:     schema.TypeBool,
				Computed: true,
			},
			"creator": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"order": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"rules": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"group": {
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
									"keep_blocked_logs": {
										Type:     schema.TypeBool,
										Computed: true,
									},
									"delete_source": {
										Type:     schema.TypeBool,
										Computed: true,
									},
									"escaped_value": {
										Type:     schema.TypeBool,
										Computed: true,
									},
									"override_destination": {
										Type:     schema.TypeBool,
										Computed: true,
									},
								},
							},
						},
					},
				},
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
			"created_at": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"updated_at": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func dataSourceCoralogixRulesGroupRead(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	ruleGroup, err := apiClient.Get("/external/group/" + d.Get("rules_group_id").(string))
	if err != nil {
		return err
	}

	d.Set("name", ruleGroup["name"].(string))
	d.Set("description", ruleGroup["description"].(string))
	d.Set("enabled", ruleGroup["enabled"].(bool))
	d.Set("creator", ruleGroup["creator"].(string))
	d.Set("order", ruleGroup["order"].(float64))
	d.Set("rules", flattenRules(ruleGroup["rulesGroups"].([]interface{})))
	d.Set("rule_matcher", flattenRuleMatchers(ruleGroup["ruleMatchers"]))
	d.Set("updated_at", ruleGroup["updatedAt"].(string))
	d.Set("created_at", ruleGroup["createdAt"].(string))
	d.SetId(ruleGroup["id"].(string))
	return nil
}
