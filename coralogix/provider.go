package coralogix

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	oldSchema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"terraform-provider-coralogix/coralogix/clientset"

	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
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
func Provider() *oldSchema.Provider {
	return &oldSchema.Provider{
		Schema: map[string]*oldSchema.Schema{
			"env": {
				Type:     oldSchema.TypeString,
				Optional: true,
				//ForceNew: true,
				//DefaultFunc:   oldSchema.EnvDefaultFunc("CORALOGIX_ENV", nil),
				//ValidateFunc:  validation.StringInSlice(validEnvs, false),
				Description: fmt.Sprintf("The Coralogix API environment. can be one of %q. environment variable 'CORALOGIX_ENV' can be defined instead.", validEnvs),
				//ConflictsWith: []string{"domain"},
			},
			"domain": {
				Type:     oldSchema.TypeString,
				Optional: true,
				//ForceNew: true,
				//DefaultFunc:   oldSchema.EnvDefaultFunc("CORALOGIX_DOMAIN", nil),
				Description: "The Coralogix domain. Conflict With 'env'. environment variable 'CORALOGIX_DOMAIN' can be defined instead.",
				//ConflictsWith: []string{"env"},
			},
			"api_key": {
				Type:      oldSchema.TypeString,
				Optional:  true,
				Sensitive: true,
				//DefaultFunc:  oldSchema.MultiEnvDefaultFunc([]string{"CORALOGIX_API_KEY"}, nil),
				//ValidateFunc: validation.IsUUID,
				Description: "A key for using coralogix APIs (Auto Generated), appropriate for the defined environment. environment variable 'CORALOGIX_API_KEY' can be defined instead.",
			},
		},

		DataSourcesMap: map[string]*oldSchema.Resource{
			"coralogix_rules_group":                dataSourceCoralogixRulesGroup(),
			"coralogix_alert":                      dataSourceCoralogixAlert(),
			"coralogix_enrichment":                 dataSourceCoralogixEnrichment(),
			"coralogix_data_set":                   dataSourceCoralogixDataSet(),
			"coralogix_dashboard":                  dataSourceCoralogixDashboard(),
			"coralogix_hosted_dashboard":           dataSourceCoralogixHostedDashboard(),
			"coralogix_action":                     dataSourceCoralogixAction(),
			"coralogix_recording_rules_groups_set": dataSourceCoralogixRecordingRulesGroupsSet(),
			"coralogix_tco_policy":                 dataSourceCoralogixTCOPolicy(),
			"coralogix_tco_policy_override":        dataSourceCoralogixTCOPolicyOverride(),
			"coralogix_webhook":                    dataSourceCoralogixWebhook(),
		},

		ResourcesMap: map[string]*oldSchema.Resource{
			"coralogix_rules_group":                resourceCoralogixRulesGroup(),
			"coralogix_alert":                      resourceCoralogixAlert(),
			"coralogix_enrichment":                 resourceCoralogixEnrichment(),
			"coralogix_data_set":                   resourceCoralogixDataSet(),
			"coralogix_dashboard":                  resourceCoralogixDashboard(),
			"coralogix_hosted_dashboard":           resourceCoralogixHostedDashboard(),
			"coralogix_action":                     resourceCoralogixAction(),
			"coralogix_recording_rules_groups_set": resourceCoralogixRecordingRulesGroupsSet(),
			"coralogix_tco_policy":                 resourceCoralogixTCOPolicy(),
			"coralogix_tco_policy_override":        resourceCoralogixTCOPolicyOverride(),
			"coralogix_webhook":                    resourceCoralogixWebhook(),
		},

		ConfigureContextFunc: func(context context.Context, d *oldSchema.ResourceData) (interface{}, diag.Diagnostics) {
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
			return clientset.NewClientSet(targetUrl, apikey, ""), nil
		},
	}
}

type coralogixProviderModel struct {
	Env    types.String `tfsdk:"env"`
	Domain types.String `tfsdk:"domain"`
	ApiKey types.String `tfsdk:"api_key"`
}

var (
	_ provider.Provider = &coralogixProvider{}
)

func NewCoralogixProvider() provider.Provider {
	return &coralogixProvider{}
}

type coralogixProvider struct{}

func (p *coralogixProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "coralogix"
}

func (p *coralogixProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"env": schema.StringAttribute{
				Optional:    true,
				Description: fmt.Sprintf("The Coralogix API environment. can be one of %q. environment variable 'CORALOGIX_ENV' can be defined instead.", validEnvs),
			},
			"domain": schema.StringAttribute{
				Optional:    true,
				Description: "The Coralogix domain. Conflict With 'env'. environment variable 'CORALOGIX_DOMAIN' can be defined instead.",
			},
			"api_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "A key for using coralogix APIs (Auto Generated), appropriate for the defined environment. environment variable 'CORALOGIX_API_KEY' can be defined instead.",
			},
		},
	}
}

func (p *coralogixProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) { // Retrieve provider data from configuration
	var config coralogixProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If practitioner provided a configuration value for any of the
	// attributes, it must be a known value.

	if config.Domain.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("domain"),
			"Unknown Coralogix Domain",
			"The provider cannot create the Coralogix API client as there is an unknown configuration value for the Coralogix domain. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the CORALOGIX_DOMAIN environment variable.",
		)
	}

	if config.Env.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("env"),
			"Unknown Coralogix Env",
			"The provider cannot create the Coralogix API client as there is an unknown configuration value for the Coralogix environment. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the CORALOGIX_ENV environment variable.",
		)
	}

	if config.ApiKey.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Unknown Coralogix API API-Key",
			"The provider cannot create the Coralogix API client as there is an unknown configuration value for the Coralogix API-Key. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the CORALOGIX_API_KEY environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Default values to environment variables, but override
	// with Terraform configuration value if set.

	domain := os.Getenv("CORALOGIX_DOMAIN")
	env := os.Getenv("CORALOGIX_ENV")
	apiKey := os.Getenv("CORALOGIX_API_KEY")

	if !config.Domain.IsNull() {
		domain = config.Domain.ValueString()
	}

	if !config.Env.IsNull() {
		env = config.Env.ValueString()
	}

	if !config.ApiKey.IsNull() {
		apiKey = config.ApiKey.ValueString()
	}

	// If any of the expected configurations are missing, return
	// errors with provider-specific guidance.

	if domain == "" && env == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("domain"),
			"Missing Coralogix domain",
			"The provider cannot create the Coralogix API client as there is a missing or empty value for the Coralogix domain. "+
				"Set the domain value in the configuration or use the CORALOGIX_DOMAIN environment variable. "+
				"If either is already set, ensure the value is not empty."+
				"Coralogix env can be set instead",
		)
		resp.Diagnostics.AddAttributeError(
			path.Root("env"),
			"Missing Coralogix API Host",
			"The provider cannot create the Coralogix API client as there is a missing or empty value for the Coralogix environment. "+
				"Set the env value in the configuration or use the CORALOGIX_ENV environment variable. "+
				"If either is already set, ensure the value is not empty."+
				"Coralogix domain can be set instead",
		)
	}

	if domain != "" && env != "" {
		resp.Diagnostics.AddError("Conflicting attributes \"env\" and \"domain\"",
			"Only one of \"env\" need to be set."+
				"ensure CORALOGIX_ENV and CORALOGIX_DOMAIN are not set together as well.",
		)
	}

	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing Coralogix API API-Key",
			"The provider cannot create the Coralogix API client as there is a missing or empty value for the Coralogix API-Key. "+
				"Set the api_key value in the configuration or use the CORALOGIX_API_KEY environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	var targetUrl string
	if env != "" {
		targetUrl = envToGrpcUrl[env]
	} else {
		targetUrl = fmt.Sprintf("ng-api-grpc.%s:443", domain)
	}

	clientSet := clientset.NewClientSet(targetUrl, apiKey, "")
	resp.DataSourceData = clientSet
	resp.ResourceData = clientSet
}

//func (p coralogixProvider) ConfigValidators(ctx context.Context) []provider.ConfigValidator {
//	return []provider.ConfigValidator{
//		providervalidator.Conflicting(
//			path.MatchRoot("env"),
//			path.MatchRoot("domain"),
//		),
//	}
//}

func (p *coralogixProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func (p *coralogixProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewEvents2MetricResource,
	}
}
