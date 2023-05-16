package coralogix

import (
	"context"
	"fmt"
	"os"

	"terraform-provider-coralogix/coralogix/clientset"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

var (
	envToGrpcUrl = map[string]string{
		"APAC1":   "ng-api-grpc.app.coralogix.in:443",
		"APAC2":   "ng-api-grpc.coralogixsg.com:443",
		"EUROPE1": "ng-api-grpc.coralogix.com:443",
		"EUROPE2": "ng-api-grpc.eu2.coralogix.com:443",
		"USA1":    "ng-api-grpc.coralogix.us:443",
	}
	validEnvs = getKeysStrings(envToGrpcUrl)
)

// Provider returns a *schema.Provider.
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"env": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				DefaultFunc:   schema.EnvDefaultFunc("CORALOGIX_ENV", nil),
				ValidateFunc:  validation.StringInSlice(validEnvs, false),
				Description:   fmt.Sprintf("The Coralogix API environment. can be one of %q. environment variable 'CORALOGIX_ENV' can be defined instead.", validEnvs),
				ConflictsWith: []string{"domain"},
			},
			"domain": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				DefaultFunc:   schema.EnvDefaultFunc("CORALOGIX_DOMAIN", nil),
				Description:   "The Coralogix domain. Conflict With 'env'. environment variable 'CORALOGIX_DOMAIN' can be defined instead.",
				ConflictsWith: []string{"env"},
			},
			"api_key": {
				Type:         schema.TypeString,
				Optional:     true,
				Sensitive:    true,
				DefaultFunc:  schema.MultiEnvDefaultFunc([]string{"CORALOGIX_API_KEY"}, nil),
				ValidateFunc: validation.IsUUID,
				Description:  "A key for using coralogix APIs (Auto Generated), appropriate for the defined environment. environment variable 'CORALOGIX_API_KEY' can be defined instead.",
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
			"coralogix_rules_group":           dataSourceCoralogixRulesGroup(),
			"coralogix_alert":                 dataSourceCoralogixAlert(),
			"coralogix_events2metric":         dataSourceCoralogixEvents2Metric(),
			"coralogix_enrichment":            dataSourceCoralogixEnrichment(),
			"coralogix_data_set":              dataSourceCoralogixDataSet(),
			"coralogix_dashboard":             dataSourceCoralogixDashboard(),
			"coralogix_hosted_dashboard":      dataSourceCoralogixHostedDashboard(),
			"coralogix_action":                dataSourceCoralogixAction(),
			"coralogix_recording_rules_group": dataSourceCoralogixRecordingRulesGroup(),
			"coralogix_tco_policy":            dataSourceCoralogixTCOPolicy(),
			"coralogix_tco_policy_override":   dataSourceCoralogixTCOPolicyOverride(),
			"coralogix_webhook":               dataSourceCoralogixWebhook(),
		},

		ResourcesMap: map[string]*schema.Resource{
			"coralogix_rules_group":           resourceCoralogixRulesGroup(),
			"coralogix_alert":                 resourceCoralogixAlert(),
			"coralogix_events2metric":         resourceCoralogixEvents2Metric(),
			"coralogix_enrichment":            resourceCoralogixEnrichment(),
			"coralogix_data_set":              resourceCoralogixDataSet(),
			"coralogix_dashboard":             resourceCoralogixDashboard(),
			"coralogix_hosted_dashboard":      resourceCoralogixHostedDashboard(),
			"coralogix_action":                resourceCoralogixAction(),
			"coralogix_recording_rules_group": resourceCoralogixRecordingRulesGroup(),
			"coralogix_tco_policy":            resourceCoralogixTCOPolicy(),
			"coralogix_tco_policy_override":   resourceCoralogixTCOPolicyOverride(),
			"coralogix_webhook":               resourceCoralogixWebhook(),
		},

		ConfigureContextFunc: func(context context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
			var targetUrl string
			if env, ok := d.GetOk("env"); ok && env.(string) != "" {
				targetUrl = envToGrpcUrl[env.(string)]
			} else if domain, ok := d.GetOk("domain"); ok && domain.(string) != "" {
				targetUrl = fmt.Sprintf("ng-api-grpc.%s:443", domain)
			} else if env = os.Getenv("CORALOGIX_ENV"); env != "" {
				targetUrl = envToGrpcUrl[env.(string)]
			} else if domain = os.Getenv("CORALOGIX_DOMAIN"); domain != "" {
				targetUrl = fmt.Sprintf("ng-api-grpc.%s:443", domain)
			} else {
				return nil, diag.Errorf("At least one of the fields 'env' or 'domain', or one of the environment variables 'CORALOGIX_ENV' or 'CORALOGIX_DOMAIN' have to be define")
			}

			apikey := d.Get("api_key").(string)
			teamsApiKey := d.Get("teams_api_key").(string)
			return clientset.NewClientSet(targetUrl, apikey, teamsApiKey), nil
		},
	}
}
