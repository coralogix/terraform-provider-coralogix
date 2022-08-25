package coralogix

import (
	"regexp"

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
									"overwrite_destinaton": {
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
							ValidateFunc: validation.StringMatch(regexp.MustCompile(`^[a-z0-9_-]*$`), "only lowercase alphanumeric characters, hyphens and underscores allowed in 'constraint'"),
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
	d.Set("updated_at", ruleGroup["updatedAt"].(string))
	d.Set("created_at", ruleGroup["createdAt"].(string))
	d.Set("rules", flattenRules(ruleGroup["rulesGroups"].([]interface{})))
	d.Set("rule_matcher", flattenRuleMatchers(ruleGroup["ruleMatchers"]))

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
