package coralogix

import (
	"context"
	"fmt"

	"terraform-provider-coralogix-v2/coralogix/clientset"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"golang.org/x/exp/maps"
)

/*
TODO
grpc -> sdk
docker
workflow
*/

var (
	envToGrpcUrl = map[string]string{
		"APAC1":   "ng-api-grpc.app.coralogix.in:443",
		"APAC2":   "ng-api-grpc.coralogixsg.com:443",
		"EUROPE1": "ng-api-grpc.coralogix.com:443",
		"EUROPE2": "ng-api-grpc.eu2.coralogix.com:443",
		"USA1":    "ng-api-grpc.coralogix.us:443",
	}
	validEnvs = maps.Keys(envToGrpcUrl)
)

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
		},

		ResourcesMap: map[string]*schema.Resource{
			"coralogix_rules_group": resourceCoralogixRulesGroup(),
			"coralogix_alert":       resourceCoralogixAlert(),
			"coralogix_logs2metric": resourceCoralogixLogs2Metric(),
		},

		ConfigureContextFunc: func(context context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
			targetUrl := envToGrpcUrl[d.Get("env").(string)]
			apikey := d.Get("api_key").(string)
			teamsApiKey := d.Get("teams_api_key").(string)
			return clientset.NewClientSet(targetUrl, apikey, teamsApiKey), nil
		},
	}
}