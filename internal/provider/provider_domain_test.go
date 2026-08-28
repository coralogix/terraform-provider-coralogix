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

package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	fwschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	oldSchema "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func frameworkProviderSchema(t *testing.T, ctx context.Context) fwschema.Schema {
	t.Helper()
	resp := &fwprovider.SchemaResponse{}
	NewCoralogixProvider().Schema(ctx, fwprovider.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("provider Schema() returned diagnostics: %v", resp.Diagnostics)
	}
	return resp.Schema
}

func frameworkProviderConfig(t *testing.T, ctx context.Context, env, domain *string) tfsdk.Config {
	t.Helper()
	s := frameworkProviderSchema(t, ctx)
	stringValue := func(v *string) tftypes.Value {
		if v == nil {
			return tftypes.NewValue(tftypes.String, nil)
		}
		return tftypes.NewValue(tftypes.String, *v)
	}
	return tfsdk.Config{
		Schema: s,
		Raw: tftypes.NewValue(s.Type().TerraformType(ctx), map[string]tftypes.Value{
			"env":     stringValue(env),
			"domain":  stringValue(domain),
			"api_key": tftypes.NewValue(tftypes.String, nil),
		}),
	}
}

// The framework provider reads CORALOGIX_DOMAIN itself, so a malformed value never reaches
// the schema validators. Configure() has to surface it as a diagnostic rather than build a
// client against a malformed gRPC target.
func TestCoralogixProviderConfigureRejectsMalformedDomainEnvVar(t *testing.T) {
	t.Setenv("CORALOGIX_ENV", "")
	t.Setenv("CORALOGIX_DOMAIN", " ")
	t.Setenv("CORALOGIX_API_KEY", "dummy-key")

	ctx := context.Background()
	resp := &fwprovider.ConfigureResponse{}
	NewCoralogixProvider().Configure(ctx, fwprovider.ConfigureRequest{
		Config: frameworkProviderConfig(t, ctx, nil, nil),
	}, resp)

	if !resp.Diagnostics.HasError() {
		t.Fatal("Configure() must report an error diagnostic for a malformed CORALOGIX_DOMAIN")
	}
	if resp.ResourceData != nil || resp.DataSourceData != nil {
		t.Fatal("Configure() must not hand out a client set after an error diagnostic")
	}

	var messages []string
	for _, d := range resp.Diagnostics.Errors() {
		messages = append(messages, d.Summary()+" "+d.Detail())
	}
	joined := strings.Join(messages, "\n")
	for _, want := range []string{"CORALOGIX_DOMAIN", `" "`} {
		if !strings.Contains(joined, want) {
			t.Errorf("diagnostics %q do not mention %s", joined, want)
		}
	}
}

// The SDKv2 half of the muxed provider reads CORALOGIX_DOMAIN in its own configure func and
// must return the failure as a diagnostic, not panic or dial a malformed target.
func TestOldProviderConfigureRejectsMalformedDomainEnvVar(t *testing.T) {
	t.Setenv("CORALOGIX_ENV", "")
	t.Setenv("CORALOGIX_DOMAIN", " ")
	t.Setenv("CORALOGIX_API_KEY", "dummy-key")

	p := OldProvider()
	clientSet, diags := p.ConfigureContextFunc(context.Background(), oldSchema.TestResourceDataRaw(t, p.Schema, map[string]interface{}{}))

	if !diags.HasError() {
		t.Fatal("ConfigureContextFunc must report an error diagnostic for a malformed CORALOGIX_DOMAIN")
	}
	if clientSet != nil {
		t.Fatal("ConfigureContextFunc must not return a client set after an error diagnostic")
	}

	var messages []string
	for _, d := range diags {
		messages = append(messages, d.Summary+" "+d.Detail)
	}
	joined := strings.Join(messages, "\n")
	for _, want := range []string{"CORALOGIX_DOMAIN", `" "`} {
		if !strings.Contains(joined, want) {
			t.Errorf("diagnostics %q do not mention %s", joined, want)
		}
	}
}

// domain's own ConflictsWith validator has to name env; pointing it at domain makes the
// framework validators library skip it as a self-reference and emit nothing.
func TestCoralogixProviderDomainConflictsWithEnv(t *testing.T) {
	ctx := context.Background()
	env, domain := "EU2", "coralogix.com"
	config := frameworkProviderConfig(t, ctx, &env, &domain)

	attribute, ok := config.Schema.(fwschema.Schema).Attributes["domain"].(fwschema.StringAttribute)
	if !ok {
		t.Fatalf("domain attribute is not a StringAttribute")
	}
	if len(attribute.Validators) == 0 {
		t.Fatal("domain attribute has no validators")
	}

	resp := &validator.StringResponse{}
	for _, v := range attribute.Validators {
		v.ValidateString(ctx, validator.StringRequest{
			Path:           path.Root("domain"),
			PathExpression: path.MatchRoot("domain"),
			Config:         config,
			ConfigValue:    types.StringValue(domain),
		}, resp)
	}

	if !resp.Diagnostics.HasError() {
		t.Fatal("setting both env and domain must produce a conflict diagnostic on domain")
	}
	if joined := resp.Diagnostics.Errors()[0].Detail(); !strings.Contains(joined, "env") {
		t.Errorf("conflict diagnostic %q does not name env", joined)
	}
}
