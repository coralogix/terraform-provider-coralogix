// Package provider is a test provider. It registers only the generated
// resource, so the acceptance tests run the generated code alone.
//
// Configuration comes from the environment:
//
//	CORALOGIX_API_KEY  the API key
//	CORALOGIX_ENV      the region, for example EU2
package provider

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/generated/aievaluation"
)

// TypeName is the provider type name. Resource types start with it, for
// example "coralogix_ai_evaluation".
const TypeName = "coralogix"

var _ provider.Provider = (*Provider)(nil)

type Provider struct{}

func New() provider.Provider { return &Provider{} }

func (p *Provider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = TypeName
}

func (p *Provider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{}
}

func (p *Provider) Configure(_ context.Context, _ provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	cs, err := newClientSet()
	if err != nil {
		resp.Diagnostics.AddError("Unable to configure the provider", err.Error())
		return
	}
	resp.ResourceData = cs
}

func (p *Provider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{aievaluation.NewResource}
}

func (p *Provider) DataSources(context.Context) []func() datasource.DataSource {
	return nil
}

// newClientSet builds the SDK client set from the environment.
func newClientSet() (*cxsdk.ClientSet, error) {
	key := os.Getenv("CORALOGIX_API_KEY")
	if key == "" {
		return nil, errors.New("CORALOGIX_API_KEY is not set")
	}
	env := os.Getenv("CORALOGIX_ENV")
	url, ok := cxsdk.URLFromRegion(env)
	if !ok {
		return nil, fmt.Errorf("CORALOGIX_ENV %q is not a known region, for example EU2", env)
	}
	cfg := cxsdk.NewConfigBuilder().WithAPIKey(key).WithURL(url).Build()
	return cxsdk.NewClientSet(cfg), nil
}
