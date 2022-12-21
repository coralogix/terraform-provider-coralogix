package coralogix

import (
	"context"
	"fmt"

	"terraform-provider-coralogix/coralogix/clientset"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var validEnvs = getKeysStrings(clientset.EnvToGrpcUrl)

// Provider returns a *schema.Provider.
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"env": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				DefaultFunc:  schema.EnvDefaultFunc("CORALOGIX_ENV", nil),
				ValidateFunc: validation.StringInSlice(validEnvs, false),
				Description:  fmt.Sprintf("The Coralogix API environment. can be one of %q", validEnvs),
			},
			"api_key": {
				Type:         schema.TypeString,
				Optional:     true,
				Sensitive:    true,
				DefaultFunc:  schema.MultiEnvDefaultFunc([]string{"CORALOGIX_API_KEY"}, nil),
				ValidateFunc: validation.IsUUID,
				Description:  "A key for alerts, rules and tags APIs (Auto Generated), appropriate for the defined environment.",
				AtLeastOneOf: []string{"api_key", "teams_api_key"},
			},
			"teams_api_key": {
				Type:         schema.TypeString,
				Optional:     true,
				Sensitive:    true,
				DefaultFunc:  schema.MultiEnvDefaultFunc([]string{"CORALOGIX_TEAMS_API_KEY"}, nil),
				ValidateFunc: validation.IsUUID,
				Description:  "A key for accessing teams API, appropriate for the defined environment.",
				AtLeastOneOf: []string{"api_key", "teams_api_key"},
			},
		},

		DataSourcesMap: map[string]*schema.Resource{
			"coralogix_rules_group": dataSourceCoralogixRulesGroup(),
			"coralogix_alert":       dataSourceCoralogixAlert(),
			"coralogix_logs2metric": dataSourceCoralogixLogs2Metric(),
			"coralogix_enrichment":  dataSourceCoralogixEnrichment(),
			"coralogix_data_set":    dataSourceCoralogixDataSet(),
			"coralogix_webhook":     dataSourceCoralogixWebhook(),
		},

		ResourcesMap: map[string]*schema.Resource{
			"coralogix_rules_group": resourceCoralogixRulesGroup(),
			"coralogix_alert":       resourceCoralogixAlert(),
			"coralogix_logs2metric": resourceCoralogixLogs2Metric(),
			"coralogix_enrichment":  resourceCoralogixEnrichment(),
			"coralogix_data_set":    resourceCoralogixDataSet(),
			"coralogix_webhook":     resourceCoralogixWebhook(),
		},

		ConfigureContextFunc: func(context context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
			env := d.Get("env").(string)
			apikey := d.Get("api_key").(string)
			teamsApiKey := d.Get("teams_api_key").(string)
			return clientset.NewClientSet(env, apikey, teamsApiKey), nil
		},
	}
}
