package coralogix

import (
	// "time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// Provider returns a *schema.Provider.
func Provider() *schema.Provider {
	// time.Sleep(time.Second * 5)
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"url": {
				Type:         schema.TypeString,
				Optional:     true,
				DefaultFunc:  schema.EnvDefaultFunc("CORALOGIX_URL", "https://api.coralogix.com/api/v1"),
				ValidateFunc: validation.IsURLWithHTTPS,
				Description:  "The Coralogix API URL.",
			},

			"api_key": {
				Type:         schema.TypeString,
				Required:     true,
				Sensitive:    true,
				DefaultFunc:  schema.EnvDefaultFunc("CORALOGIX_API_KEY", nil),
				ValidateFunc: validation.IsUUID,
				Description:  "The Coralogix API key.",
			},

			"timeout": {
				Type:         schema.TypeInt,
				Optional:     true,
				DefaultFunc:  schema.EnvDefaultFunc("CORALOGIX_API_TIMEOUT", 30),
				ValidateFunc: validation.IntBetween(10, 300),
				Description:  "The Coralogix API timeout.",
			},
		},

		DataSourcesMap: map[string]*schema.Resource{
			"coralogix_alert":       dataSourceCoralogixAlert(),
			"coralogix_rules_group": dataSourceCoralogixRulesGroup(),
			"coralogix_rule":        dataSourceCoralogixRule(),
		},

		ResourcesMap: map[string]*schema.Resource{
			"coralogix_alert":       resourceCoralogixAlert(),
			"coralogix_rules_group": resourceCoralogixRulesGroup(),
			"coralogix_rule":        resourceCoralogixRule(),
		},

		ConfigureFunc: func(d *schema.ResourceData) (interface{}, error) {
			url := d.Get("url").(string)
			apiKey := d.Get("api_key").(string)
			timeout := d.Get("timeout").(int)
			return NewClient(url, apiKey, timeout)
		},
	}
}
