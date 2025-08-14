// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package coralogix

import (
	"context"
	"fmt"
	"os"
	"strings"

	"terraform-provider-coralogix/coralogix/clientset"
	"terraform-provider-coralogix/coralogix/utils"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"golang.org/x/exp/slices"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	oldSchema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
)

var (
	terraformEnvironmentAliasToGrpcUrl = map[string]string{
		"APAC1":   "ng-api-grpc.app.coralogix.in:443",
		"AP1":     "ng-api-grpc.app.coralogix.in:443",
		"APAC2":   "ng-api-grpc.coralogixsg.com:443",
		"AP2":     "ng-api-grpc.coralogixsg.com:443",
		"APAC3":   "ng-api-grpc.ap3.coralogix.com:443",
		"AP3":     "ng-api-grpc.ap3.coralogix.com:443",
		"EUROPE1": "ng-api-grpc.coralogix.com:443",
		"EU1":     "ng-api-grpc.coralogix.com:443",
		"EUROPE2": "ng-api-grpc.eu2.coralogix.com:443",
		"EU2":     "ng-api-grpc.eu2.coralogix.com:443",
		"USA1":    "ng-api-grpc.coralogix.us:443",
		"US1":     "ng-api-grpc.coralogix.us:443",
		"USA2":    "ng-api-grpc.cx498.coralogix.com:443",
		"US2":     "ng-api-grpc.cx498.coralogix.com:443",
	}
	validEnvironmentAliases                   = utils.GetKeys(terraformEnvironmentAliasToGrpcUrl)
	terraformEnvironmentAliasToSdkEnvironment = map[string]string{
		"APAC1":   "AP1",
		"AP1":     "AP1",
		"APAC2":   "AP2",
		"AP2":     "AP2",
		"APAC3":   "AP3",
		"AP3":     "AP3",
		"EUROPE1": "EU1",
		"EU1":     "EU1",
		"EUROPE2": "EU2",
		"EU2":     "EU2",
		"USA1":    "US1",
		"US1":     "US1",
		"USA2":    "US2",
		"US2":     "US2",
	}
)

// OldProvider returns a *schema.Provider.
func OldProvider() *oldSchema.Provider {
	return &oldSchema.Provider{
		Schema: map[string]*oldSchema.Schema{
			"env": {
				Type:     oldSchema.TypeString,
				Optional: true,
				//ForceNew: true,
				//DefaultFunc:   oldSchema.EnvDefaultFunc("CORALOGIX_ENV", nil),
				ValidateFunc:  validation.StringInSlice(validEnvironmentAliases, true),
				Description:   fmt.Sprintf("The Coralogix API environment. can be one of %q. environment variable 'CORALOGIX_ENV' can be defined instead.", validEnvironmentAliases),
				ConflictsWith: []string{"domain"},
			},
			"domain": {
				Type:     oldSchema.TypeString,
				Optional: true,
				//ForceNew: true,
				//DefaultFunc:   oldSchema.EnvDefaultFunc("CORALOGIX_DOMAIN", nil),
				Description:   "The Coralogix domain. Conflict With 'env'. environment variable 'CORALOGIX_DOMAIN' can be defined instead.",
				ConflictsWith: []string{"env"},
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
			"coralogix_rules_group":      dataSourceCoralogixRulesGroup(),
			"coralogix_enrichment":       dataSourceCoralogixEnrichment(),
			"coralogix_data_set":         dataSourceCoralogixDataSet(),
			"coralogix_hosted_dashboard": dataSourceCoralogixHostedDashboard(),
		},

		ResourcesMap: map[string]*oldSchema.Resource{
			"coralogix_rules_group":      resourceCoralogixRulesGroup(),
			"coralogix_enrichment":       resourceCoralogixEnrichment(),
			"coralogix_data_set":         resourceCoralogixDataSet(),
			"coralogix_hosted_dashboard": resourceCoralogixHostedDashboard(),
			"coralogix_grafana_folder":   resourceGrafanaFolder(),
		},

		ConfigureContextFunc: func(context context.Context, d *oldSchema.ResourceData) (interface{}, diag.Diagnostics) {
			var targetUrl string
			var cxEnv string
			if env, ok := d.GetOk("env"); ok && env.(string) != "" {
				if url, ok := terraformEnvironmentAliasToGrpcUrl[strings.ToUpper(env.(string))]; !ok {
					return nil, diag.Errorf("The Coralogix env must be one of %q", validEnvironmentAliases)
				} else {
					targetUrl = url
					cxEnv = env.(string)
				}
			} else if domain, ok := d.GetOk("domain"); ok && domain.(string) != "" {
				targetUrl = fmt.Sprintf("ng-api-grpc.%s:443", domain)
				cxEnv = targetUrl
			} else if env = strings.ToUpper(os.Getenv("CORALOGIX_ENV")); env != "" {
				if url, ok := terraformEnvironmentAliasToGrpcUrl[env.(string)]; !ok {
					return nil, diag.Errorf("The Coralogix env must be one of %q", validEnvironmentAliases)
				} else {
					targetUrl = url
					cxEnv = env.(string)
				}
			} else if domain := os.Getenv("CORALOGIX_DOMAIN"); domain != "" {
				targetUrl = fmt.Sprintf("ng-api-grpc.%s:443", domain)
			} else {
				return nil, diag.Errorf("At least one of the fields 'env' or 'domain', or one of the environment variables 'CORALOGIX_ENV' or 'CORALOGIX_DOMAIN' have to be defined")
			}

			apiKey := os.Getenv("CORALOGIX_API_KEY")
			if apiKey == "" {
				apiKey = d.Get("api_key").(string)
			}

			if apiKey == "" {
				return nil, diag.Errorf("At least one of the field 'api_key' or environment variable 'CORALOGIX_API_KEY' have to be defined")
			}
			if cxEnv == "" || len(cxEnv) > 3 {
				cxEnv = terraformEnvironmentAliasToSdkEnvironment[cxEnv]
			}

			return clientset.NewClientSet(cxEnv, apiKey, targetUrl), nil
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
				Optional: true,
				Validators: []validator.String{
					stringvalidator.OneOfCaseInsensitive(validEnvironmentAliases...),
					stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("domain")),
				},
				Description: fmt.Sprintf("The Coralogix API environment. can be one of %q. environment variable 'CORALOGIX_ENV' can be defined instead.", validEnvironmentAliases),
			},
			"domain": schema.StringAttribute{
				Optional: true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("domain")),
				},
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
	terraformEnvironmentAlias := os.Getenv("CORALOGIX_ENV")
	apiKey := os.Getenv("CORALOGIX_API_KEY")

	if !config.Domain.IsNull() {
		domain = config.Domain.ValueString()
	}

	if !config.Env.IsNull() {
		terraformEnvironmentAlias = config.Env.ValueString()
	}
	terraformEnvironmentAlias = strings.ToUpper(terraformEnvironmentAlias)

	if !config.ApiKey.IsNull() {
		apiKey = config.ApiKey.ValueString()
	}

	// If any of the expected configurations are missing, return
	// errors with provider-specific guidance.

	if domain == "" && terraformEnvironmentAlias == "" {
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

	if domain != "" && terraformEnvironmentAlias != "" {
		resp.Diagnostics.AddError("Conflicting attributes \"env\" and \"domain\"",
			"Only one of \"env\" need to be set."+
				"ensure CORALOGIX_ENV and CORALOGIX_DOMAIN are not set together as well.",
		)
	} else if domain == "" && !slices.Contains(validEnvironmentAliases, terraformEnvironmentAlias) {
		resp.Diagnostics.AddAttributeError(path.Root("env"), "Invalid Coralogix env", fmt.Sprintf("The Coralogix env must be one of %q", validEnvironmentAliases))
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
	if terraformEnvironmentAlias != "" {
		targetUrl = terraformEnvironmentAliasToGrpcUrl[terraformEnvironmentAlias]
	} else {
		targetUrl = fmt.Sprintf("ng-api-grpc.%s:443", domain)
	}

	sdkEnvironment := terraformEnvironmentAliasToSdkEnvironment[terraformEnvironmentAlias]
	if domain != "" {
		sdkEnvironment = domain
	}
	clientSet := clientset.NewClientSet(sdkEnvironment, apiKey, targetUrl)
	resp.DataSourceData = clientSet
	resp.ResourceData = clientSet
}

func (p *coralogixProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewEvents2MetricDataSource,
		NewActionDataSource,
		NewTCOPoliciesLogsDataSource,
		NewTCOPoliciesTracesDataSource,
		NewDashboardDataSource,
		NewWebhookDataSource,
		NewRecordingRuleGroupSetDataSource,
		NewArchiveRetentionsDataSource,
		NewArchiveMetricsDataSource,
		NewArchiveLogsDataSource,
		NewAlertsSchedulerDataSource,
		NewSLODataSource,
		// NewSLOV2DataSource,
		NewDashboardsFoldersDataSource,
		NewApiKeyDataSource,
		NewCustomRoleDataSource,
		NewGroupDataSource,
		NewUserDataSource,
		NewTeamDataSource,
		NewScopeDataSource,
		NewIntegrationDataSource,
		NewAlertDataSource,
		NewConnectorDataSource,
		NewGlobalRouterDataSource,
		NewPresetDataSource,
		NewGroupV2DataSource,
	}
}

func (p *coralogixProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewEvents2MetricResource,
		NewActionResource,
		NewTCOPoliciesLogsResource,
		NewTCOPoliciesTracesResource,
		NewDashboardResource,
		NewWebhookResource,
		NewRecordingRuleGroupSetResource,
		NewArchiveRetentionsResource,
		NewArchiveMetricsResource,
		NewArchiveLogsResource,
		NewAlertsSchedulerResource,
		NewTeamResource,
		NewApiKeyResource,
		NewSLOResource,
		// NewSLOV2Resource,
		NewDashboardsFolderResource,
		NewCustomRoleSource,
		NewGroupResource,
		NewGroupAttachmentResource,
		NewUserResource,
		NewScopeResource,
		NewIntegrationResource,
		NewAlertResource,
		NewConnectorResource,
		NewGlobalRouterResource,
		NewPresetResource,
		NewGroupV2Resource,
	}
}
