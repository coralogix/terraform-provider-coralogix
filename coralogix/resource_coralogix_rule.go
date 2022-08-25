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
					"timestampextract",
					"removefields",
					"block",
					"allow",
					"jsonstringify",
					"jsonparse",
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
				Type:       schema.TypeSet,
				Optional:   true,
				Deprecated: "rule_matcher is no longer being used and will be deprecated in the next release.",
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
				Optional:     true,
				Default:      ".*",
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
			"replace_value": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"format_standard": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
				ValidateFunc: validation.StringInSlice([]string{
					"javasdf",
					"golang",
					"strftime",
					"secondsts",
					"millits",
					"microts",
					"nanots",
				}, false),
			},
			"time_format": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
			},
			"keep_blocked_logs": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"delete_source": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"overwrite_destinaton": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"escaped_value": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
		},
	}
}

func resourceCoralogixRuleCreate(d *schema.ResourceData, meta interface{}) error {
	if err := ruleValuesValidation(d); err != nil {
		return err
	}
	apiClient := meta.(*Client)
	ruleType := d.Get("type").(string)
	ruleParameters := map[string]interface{}{
		"name":        d.Get("name").(string),
		"type":        ruleType,
		"description": d.Get("description").(string),
		"enabled":     d.Get("enabled").(bool),
		"sourceField": d.Get("source_field").(string),
	}
	if ruleType != "timestampextract" {
		ruleParameters["rule"] = d.Get("expression").(string)
	} else {
		ruleParameters["formatStandard"] = d.Get("format_standard").(string)
		ruleParameters["timeFormat"] = d.Get("time_format").(string)
	}
	if ruleType == "replace" {
		ruleParameters["replaceNewVal"] = d.Get("replace_value").(string)
	}

	if ruleType == "jsonextract" || ruleType == "parse" || ruleType == "replace" {
		ruleParameters["destinationField"] = d.Get("destination_field").(string)
	}

	if ruleType == "block" || ruleType == "allow" {
		ruleParameters["keepBlockedLogs"] = d.Get("keep_blocked_logs")
	}

	if ruleType == "jsonstringify" || ruleType == "jsonparse" {
		ruleParameters["deleteSource"] = d.Get("delete_source").(bool)
		ruleParameters["escapedValue"] = d.Get("escaped_value").(bool)
	}

	if ruleType == "jsonparse" {
		ruleParameters["overrideDest"] = d.Get("overwrite_destinaton").(bool)
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
	ruleType := rule["type"]

	d.Set("name", rule["name"].(string))
	d.Set("type", ruleType)
	d.Set("description", rule["description"].(string))
	d.Set("order", rule["order"].(float64))
	d.Set("enabled", rule["enabled"].(bool))
	d.Set("source_field", rule["sourceField"].(string))

	if ruleType != "timestampextract" {
		d.Set("expression", rule["rule"].(string))
	} else {
		d.Set("format_standard", rule["formatStandard"].(string))
		d.Set("time_format", rule["timeFormat"].(string))
	}

	if ruleType == "replace" {
		d.Set("replace_value", rule["replaceNewVal"].(string))
	}

	if ruleType == "jsonextract" || ruleType == "parse" || ruleType == "replace" {
		d.Set("destination_field", rule["destinationField"].(string))
	}

	if ruleType == "block" || ruleType == "allow" {
		d.Set("keep_blocked_logs", rule["keepBlockedLogs"])
	}

	if ruleType == "jsonstringify" || ruleType == "jsonparse" {
		d.Set("delete_source", rule["deleteSource"])
		d.Set("escaped_value", rule["escapedValue"])
	}

	if ruleType == "jsonparse" {
		d.Set("overwrite_destinaton", rule["overrideDest"])
	}

	if rule["ruleMatchers"] != nil {
		d.Set("rule_matcher", flattenRuleMatchers(rule["ruleMatchers"].([]interface{})))
	} else {
		d.Set("rule_matcher", nil)
	}

	return nil
}

func resourceCoralogixRuleUpdate(d *schema.ResourceData, meta interface{}) error {
	if err := ruleValuesValidation(d); err != nil {
		return err
	}
	apiClient := meta.(*Client)
	ruleType := d.Get("type").(string)
	if d.HasChanges("name", "type", "description", "enabled", "expression", "source_field", "destination_field", "replace_value", "keep_blocked_logs", "delete_source", "overwrite_destinaton", "escaped_value", "format_standard", "time_format") {
		ruleParameters := map[string]interface{}{
			"name":        d.Get("name").(string),
			"type":        ruleType,
			"description": d.Get("description").(string),
			"enabled":     d.Get("enabled").(bool),
			"rule":        d.Get("expression").(string),
			"sourceField": d.Get("source_field").(string),
		}

		if ruleType != "timestampextract" {
			ruleParameters["rule"] = d.Get("expression").(string)
		} else {
			ruleParameters["formatStandard"] = d.Get("format_standard").(string)
			ruleParameters["timeFormat"] = d.Get("time_format").(string)
		}

		if ruleType == "replace" {
			ruleParameters["replaceNewVal"] = d.Get("replace_value").(string)
		}

		if ruleType == "jsonextract" || ruleType == "parse" || ruleType == "replace" {
			ruleParameters["destinationField"] = d.Get("destination_field").(string)
		}

		if ruleType == "block" || ruleType == "allow" {
			ruleParameters["keepBlockedLogs"] = d.Get("keep_blocked_logs")
		}

		if ruleType == "jsonstringify" || ruleType == "jsonparse" {
			ruleParameters["deleteSource"] = d.Get("delete_source").(bool)
			ruleParameters["escapedValue"] = d.Get("escaped_value").(bool)
		}

		if ruleType == "jsonparse" {
			ruleParameters["overrideDest"] = d.Get("overwrite_destinaton").(bool)
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
		return nil, errors.New("invalid rule ID")
	}

	d.Set("rules_group_id", ids[0])
	d.SetId(ids[1])

	return []*schema.ResourceData{d}, nil
}
