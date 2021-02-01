package coralogix

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceCoralogixRule() *schema.Resource {
	return &schema.Resource{
		Create: resourceCoralogixRuleCreate,
		Read:   resourceCoralogixRuleRead,
		Update: resourceCoralogixRuleUpdate,
		Delete: resourceCoralogixRuleDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"rules_group_id": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.IsUUID,
			},
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringIsNotEmpty,
			},
			"type": {
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: validation.StringInSlice([]string{
					"extract",
					"jsonextract",
					"parse",
					"replace",
					"allow",
					"block",
				}, false),
			},
			"description": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"order": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"enabled": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
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
								"text",
								"category",
								"computerName",
								"className",
								"methodName",
								"threadId",
							}, false),
						},
						"constraint": {
							Type:         schema.TypeString,
							Required:     true,
							ValidateFunc: validation.StringIsValidRegExp,
						},
					},
				},
				Set: schema.HashString,
			},
			"expression": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringIsValidRegExp,
			},
			"source_field": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "text",
				ValidateFunc: validation.Any(
					validation.StringMatch(
						regexp.MustCompile(`^text(\..+)*$`),
						"should starts with \"text\" prefix",
					),
					validation.StringInSlice([]string{
						"severity",
						"category",
						"computerName",
						"className",
						"methodName",
						"threadId",
					}, false),
				),
			},
			"destination_field": {
				Type:     schema.TypeString,
				Optional: true,
				ValidateFunc: validation.Any(
					validation.StringMatch(
						regexp.MustCompile(`^text(\..+)*$`),
						"should starts with \"text\" prefix",
					),
					validation.StringInSlice([]string{
						"severity",
						"category",
						"computerName",
						"className",
						"methodName",
						"threadId",
					}, false),
				),
			},
			"replace_value": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
		},
	}
}

func resourceCoralogixRuleCreate(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	ruleParameters := map[string]interface{}{
		"Name":        d.Get("name").(string),
		"Type":        d.Get("type").(string),
		"Description": d.Get("description").(string),
		"Enabled":     d.Get("enabled").(bool),
		"Rule":        d.Get("expression").(string),
		"SourceField": d.Get("source_field").(string),
	}

	if d.Get("type").(string) == "replace" {
		ruleParameters["ReplaceNewVal"] = d.Get("replace_value").(string)
	}

	if d.Get("type").(string) == "jsonextract" || d.Get("type").(string) == "parse" || d.Get("type").(string) == "replace" {
		ruleParameters["DestinationField"] = d.Get("destination_field").(string)
	}

	if d.Get("rule_matcher") != nil && len(d.Get("rule_matcher").(*schema.Set).List()) > 0 {
		ruleParameters["RuleMatchers"] = flattenRuleMatchers(d.Get("rule_matcher").(*schema.Set).List())
	}

	rule, err := apiClient.Post("/external/action/rule/"+d.Get("rules_group_id").(string), ruleParameters)
	if err != nil {
		return err
	}

	d.SetId(rule["Id"].(string))

	return resourceCoralogixRuleRead(d, meta)
}

func resourceCoralogixRuleRead(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	rule, err := apiClient.Get("/external/action/" + d.Id() + "/rule/" + d.Get("rules_group_id").(string))
	if err != nil {
		return err
	}

	d.Set("name", rule["Name"].(string))
	d.Set("type", rule["Type"].(string))
	d.Set("description", rule["Description"].(string))
	d.Set("order", rule["Order"].(float64))
	d.Set("enabled", rule["Enabled"].(bool))
	d.Set("expression", rule["Rule"].(string))
	d.Set("source_field", rule["SourceField"].(string))

	if d.Get("type").(string) == "replace" {
		d.Set("replace_value", rule["ReplaceNewVal"].(string))
	}

	if d.Get("type").(string) == "jsonextract" || d.Get("type").(string) == "parse" || d.Get("type").(string) == "replace" {
		d.Set("destination_field", rule["DestinationField"].(string))
	}

	if rule["RuleMatchers"] != nil {
		d.Set("rule_matcher", flattenRuleMatchers(rule["RuleMatchers"].([]interface{})))
	} else {
		d.Set("rule_matcher", nil)
	}

	return nil
}

func resourceCoralogixRuleUpdate(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	if d.HasChanges("name", "type", "description", "enabled", "rule_matcher", "expression", "source_field", "destination_field", "replace_value") {
		ruleParameters := map[string]interface{}{
			"Name":        d.Get("name").(string),
			"Type":        d.Get("type").(string),
			"Description": d.Get("description").(string),
			"Enabled":     d.Get("enabled").(bool),
			"Rule":        d.Get("expression").(string),
			"SourceField": d.Get("source_field").(string),
		}

		if d.Get("type").(string) == "replace" {
			ruleParameters["ReplaceNewVal"] = d.Get("replace_value").(string)
		}

		if d.Get("type").(string) == "jsonextract" || d.Get("type").(string) == "parse" || d.Get("type").(string) == "replace" {
			ruleParameters["DestinationField"] = d.Get("destination_field").(string)
		}

		if d.Get("rule_matcher") != nil && len(d.Get("rule_matcher").(*schema.Set).List()) > 0 {
			ruleParameters["RuleMatchers"] = flattenRuleMatchers(d.Get("rule_matcher").(*schema.Set).List())
		}

		_, err := apiClient.Put("/external/action/"+d.Id()+"/rule/"+d.Get("rules_group_id").(string), ruleParameters)
		if err != nil {
			return err
		}
	}

	return resourceCoralogixRuleRead(d, meta)
}

func resourceCoralogixRuleDelete(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	_, err := apiClient.Delete("/external/action/" + d.Id() + "/rule/" + d.Get("rules_group_id").(string))
	if err != nil {
		return err
	}

	return nil
}
