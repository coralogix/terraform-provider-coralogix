package coralogix

import (
	"errors"
	"regexp"
	"strings"

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
			State: resourceCoralogixRuleImport,
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
		"name":        d.Get("name").(string),
		"type":        d.Get("type").(string),
		"description": d.Get("description").(string),
		"enabled":     d.Get("enabled").(bool),
		"rule":        d.Get("expression").(string),
		"sourceField": d.Get("source_field").(string),
	}

	if d.Get("type").(string) == "replace" {
		ruleParameters["replaceNewVal"] = d.Get("replace_value").(string)
	}

	if d.Get("type").(string) == "jsonextract" || d.Get("type").(string) == "parse" || d.Get("type").(string) == "replace" {
		ruleParameters["destinationField"] = d.Get("destination_field").(string)
	}

	if d.Get("rule_matcher") != nil && len(d.Get("rule_matcher").(*schema.Set).List()) > 0 {
		ruleParameters["ruleMatchers"] = flattenRuleMatchers(d.Get("rule_matcher").(*schema.Set).List())
	}

	rule, err := apiClient.Post("/external/rule/"+d.Get("rules_group_id").(string), ruleParameters)
	if err != nil {
		return err
	}

	d.SetId(rule["id"].(string))

	return resourceCoralogixRuleRead(d, meta)
}

func resourceCoralogixRuleRead(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	rule, err := apiClient.Get("/external/rule/" + d.Id() + "/group/" + d.Get("rules_group_id").(string))
	if err != nil {
		return err
	}

	d.Set("name", rule["name"].(string))
	d.Set("type", rule["type"].(string))
	d.Set("description", rule["description"].(string))
	d.Set("order", rule["order"].(float64))
	d.Set("enabled", rule["enabled"].(bool))
	d.Set("expression", rule["rule"].(string))
	d.Set("source_field", rule["sourceField"].(string))

	if d.Get("type").(string) == "replace" {
		d.Set("replace_value", rule["replaceNewVal"].(string))
	}

	if d.Get("type").(string) == "jsonextract" || d.Get("type").(string) == "parse" || d.Get("type").(string) == "replace" {
		d.Set("destination_field", rule["destinationField"].(string))
	}

	if rule["ruleMatchers"] != nil {
		d.Set("rule_matcher", flattenRuleMatchers(rule["ruleMatchers"].([]interface{})))
	} else {
		d.Set("rule_matcher", nil)
	}

	return nil
}

func resourceCoralogixRuleUpdate(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	if d.HasChanges("name", "type", "description", "enabled", "rule_matcher", "expression", "source_field", "destination_field", "replace_value") {
		ruleParameters := map[string]interface{}{
			"name":        d.Get("name").(string),
			"type":        d.Get("type").(string),
			"description": d.Get("description").(string),
			"enabled":     d.Get("enabled").(bool),
			"rule":        d.Get("expression").(string),
			"sourceField": d.Get("source_field").(string),
		}

		if d.Get("type").(string) == "replace" {
			ruleParameters["replaceNewVal"] = d.Get("replace_value").(string)
		}

		if d.Get("type").(string) == "jsonextract" || d.Get("type").(string) == "parse" || d.Get("type").(string) == "replace" {
			ruleParameters["destinationField"] = d.Get("destination_field").(string)
		}

		if d.Get("rule_matcher") != nil && len(d.Get("rule_matcher").(*schema.Set).List()) > 0 {
			ruleParameters["ruleMatchers"] = flattenRuleMatchers(d.Get("rule_matcher").(*schema.Set).List())
		}

		_, err := apiClient.Put("/external/rule/"+d.Id()+"/group/"+d.Get("rules_group_id").(string), ruleParameters)
		if err != nil {
			return err
		}
	}

	return resourceCoralogixRuleRead(d, meta)
}

func resourceCoralogixRuleDelete(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	_, err := apiClient.Delete("/external/rule/" + d.Id() + "/group/" + d.Get("rules_group_id").(string))
	if err != nil {
		return err
	}

	return nil
}

func resourceCoralogixRuleImport(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	ids := strings.SplitN(d.Id(), "/", 2)

	if len(ids) != 2 || ids[0] == "" || ids[1] == "" {
		return nil, errors.New("Invalid rule ID")
	}

	d.Set("rules_group_id", ids[0])
	d.SetId(ids[1])

	return []*schema.ResourceData{d}, nil
}
