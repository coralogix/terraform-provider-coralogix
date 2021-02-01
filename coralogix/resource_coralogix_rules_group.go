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
			"order": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"enabled": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
		},
	}
}

func resourceCoralogixRulesGroupCreate(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	ruleGroup, err := apiClient.Post("/external/actions/rule", map[string]interface{}{
		"Name": d.Get("name").(string),
	})
	if err != nil {
		return err
	}

	d.SetId(ruleGroup["Id"].(string))

	if !d.Get("enabled").(bool) {
		_, err = apiClient.Put("/external/actions/rule/"+d.Id(), map[string]interface{}{
			"Name":    d.Get("name").(string),
			"Order":   ruleGroup["Order"].(float64),
			"Enabled": d.Get("enabled").(bool),
		})
		if err != nil {
			return err
		}
	}

	return resourceCoralogixRulesGroupRead(d, meta)
}

func resourceCoralogixRulesGroupRead(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	ruleGroup, err := apiClient.Get("/external/actions/rule/" + d.Id())
	if err != nil {
		return err
	}

	d.Set("name", ruleGroup["Name"].(string))
	d.Set("order", ruleGroup["Order"].(float64))
	d.Set("enabled", ruleGroup["Enabled"].(bool))

	return nil
}

func resourceCoralogixRulesGroupUpdate(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	if d.HasChanges("name", "enabled") {
		_, err := apiClient.Put("/external/actions/rule/"+d.Id(), map[string]interface{}{
			"Name":    d.Get("name").(string),
			"Order":   d.Get("order").(int),
			"Enabled": d.Get("enabled").(bool),
		})
		if err != nil {
			return err
		}
	}

	return resourceCoralogixRulesGroupRead(d, meta)
}

func resourceCoralogixRulesGroupDelete(d *schema.ResourceData, meta interface{}) error {
	apiClient := meta.(*Client)

	_, err := apiClient.Delete("/external/actions/rule/" + d.Id())
	if err != nil {
		return err
	}

	return nil
}
