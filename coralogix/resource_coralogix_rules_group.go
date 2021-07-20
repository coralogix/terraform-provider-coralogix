package coralogix

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceCoralogixRulesGroup() *schema.Resource {
	return &schema.Resource{
		Create: resourceCoralogixRulesGroupCreate,
		Read:   resourceCoralogixRulesGroupRead,
		Update: resourceCoralogixRulesGroupUpdate,
		Delete: resourceCoralogixRulesGroupDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringIsNotEmpty,
			},
			"description": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"enabled": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"creator": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "Coralogix Terraform Provider",
			},
			"order": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"rule_matcher": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"field": {
							Type:     schema.TypeString,
							Required: true,
							ValidateFunc: validation.StringInSlice([]string{
								"severity",
								"applicationName",
								"subsystemName",
							}, false),
						},
						"constraint": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringIsNotEmpty,
						},
					},
				},
				Set: schema.HashString,
			},
		},
	}
}

func resourceCoralogixRulesGroupCreate(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	rulesGroupParameters := map[string]interface{}{
		"name":        d.Get("name").(string),
		"description": d.Get("description").(string),
		"enabled":     d.Get("enabled").(bool),
		"creator":     d.Get("creator").(string),
	}

	if d.Get("rule_matcher") != nil && len(d.Get("rule_matcher").(*schema.Set).List()) > 0 {
		rulesGroupParameters["ruleMatchers"] = flattenRuleMatchers(d.Get("rule_matcher").(*schema.Set).List())
	}

	ruleGroup, err := apiClient.Post("/external/group", rulesGroupParameters)
	if err != nil {
		return err
	}

	d.SetId(ruleGroup["id"].(string))

	return resourceCoralogixRulesGroupRead(d, meta)
}

func resourceCoralogixRulesGroupRead(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	ruleGroup, err := apiClient.Get("/external/group/" + d.Id())
	if err != nil {
		return err
	}

	d.Set("name", ruleGroup["name"].(string))
	d.Set("description", ruleGroup["description"].(string))
	d.Set("enabled", ruleGroup["enabled"].(bool))
	d.Set("creator", ruleGroup["creator"].(string))
	d.Set("order", ruleGroup["order"].(float64))

	if ruleGroup["ruleMatchers"] != nil {
		d.Set("rule_matcher", flattenRuleMatchers(ruleGroup["ruleMatchers"].([]interface{})))
	} else {
		d.Set("rule_matcher", nil)
	}

	return nil
}

func resourceCoralogixRulesGroupUpdate(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	if d.HasChanges("name", "enabled", "description", "creator", "rule_matcher") {
		rulesGroupParameters := map[string]interface{}{
			"name":        d.Get("name").(string),
			"description": d.Get("description").(string),
			"enabled":     d.Get("enabled").(bool),
			"creator":     d.Get("creator").(string),
		}

		if d.Get("rule_matcher") != nil && len(d.Get("rule_matcher").(*schema.Set).List()) > 0 {
			rulesGroupParameters["ruleMatchers"] = flattenRuleMatchers(d.Get("rule_matcher").(*schema.Set).List())
		}

		_, err := apiClient.Put("/external/group/"+d.Id(), rulesGroupParameters)
		if err != nil {
			return err
		}
	}

	return resourceCoralogixRulesGroupRead(d, meta)
}

func resourceCoralogixRulesGroupDelete(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	_, err := apiClient.Delete("/external/group/" + d.Id())
	if err != nil {
		return err
	}

	return nil
}
