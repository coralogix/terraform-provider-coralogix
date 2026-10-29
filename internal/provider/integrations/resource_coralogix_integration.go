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

package integrations

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"slices"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	integrationsSDK "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/integration_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/integration"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"
)

// NewIntegrationResource returns the generated coralogix_integration with its handwritten hooks.
// The generated code handles the id, the key, and the version. The hooks handle `parameters`,
// which is a dynamic value that the API holds as a list of typed parameters.
func NewIntegrationResource() resource.Resource {
	h := &integrationHooks{}
	generated := integration.NewResource(integration.Hooks{
		BeforeWrite:   h.beforeWrite,
		BeforeFlatten: h.rememberParameters,
		AfterRead:     h.setStateParameters,
	})
	return &integrationResource{Resource: generated.(*integration.Resource), hooks: h}
}

// integrationResource gives every call that reads the integration its own handoff of the API
// parameters, from BeforeFlatten to AfterRead.
type integrationResource struct {
	*integration.Resource
	hooks *integrationHooks
}

// Configure configures the generated resource, and keeps the client that the test call of
// BeforeWrite uses.
func (r *integrationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.Resource.Configure(ctx, req, resp)
	if cs, ok := req.ProviderData.(*clientset.ClientSet); ok && !resp.Diagnostics.HasError() {
		r.hooks.client = cs.Integrations()
	}
}

func (r *integrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	ctx, handoff := withHandoff(ctx)
	handoff.plan(ctx, req.Plan)
	r.Resource.Create(ctx, req, resp)
}

func (r *integrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	ctx, _ = withHandoff(ctx)
	r.Resource.Read(ctx, req, resp)
}

func (r *integrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	ctx, handoff := withHandoff(ctx)
	handoff.plan(ctx, req.Plan)
	r.Resource.Update(ctx, req, resp)
}

type handoffKey struct{}

// parameterHandoff holds the parameters that one read returned. The hooks get the API value and
// the state in separate steps, and the context of the call, which no other call shares, carries
// the handoff between them.
type parameterHandoff struct {
	parameters []integrationsSDK.Parameter
	// planned is the `parameters` of the plan of a Create or an Update. A value that comes from
	// another resource is unknown in the configuration while Terraform plans, and the plan holds
	// the resolved object, which is what the released resource expanded.
	planned types.Dynamic
}

// plan keeps the parameters of the plan of a write.
func (h *parameterHandoff) plan(ctx context.Context, plan tfsdk.Plan) {
	var planned types.Dynamic
	if !plan.GetAttribute(ctx, path.Root("parameters"), &planned).HasError() {
		h.planned = planned
	}
}

func withHandoff(ctx context.Context) (context.Context, *parameterHandoff) {
	h := &parameterHandoff{}
	return context.WithValue(ctx, handoffKey{}, h), h
}

type integrationHooks struct {
	client *integrationsSDK.IntegrationServiceAPIService
}

// setParameters puts the parameters of the plan, or of the configuration when the plan has none,
// in the Create or Update request.
func (h *integrationHooks) setParameters(ctx context.Context, config tfsdk.Config, body any) diag.Diagnostics {
	var parameters types.Dynamic
	diags := config.GetAttribute(ctx, path.Root("parameters"), &parameters)
	if diags.HasError() {
		return diags
	}
	// The plan has the resolved value when the configuration refers to another resource.
	if handoff, ok := ctx.Value(handoffKey{}).(*parameterHandoff); ok && hasKnownParameters(handoff.planned) {
		parameters = handoff.planned
	}
	list, d := dynamicToParameters(parameters)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	metadata := requestMetadata(body)
	if metadata == nil {
		diags.AddError("Unexpected request body", fmt.Sprintf("The integration request body is %T.", body))
		return diags
	}
	metadata.IntegrationParameters = integrationsSDK.GenericIntegrationParameters{Parameters: list}
	return diags
}

func requestMetadata(body any) *integrationsSDK.IntegrationMetadata {
	switch b := body.(type) {
	case *integrationsSDK.SaveIntegrationRequest:
		return &b.Metadata
	case *integrationsSDK.UpdateIntegrationRequest:
		return &b.Metadata
	}
	return nil
}

// beforeWrite puts the parameters of the configuration in the request, then asks the API to test
// the integration before Create and Update save it.
func (h *integrationHooks) beforeWrite(ctx context.Context, config tfsdk.Config, body any) diag.Diagnostics {
	diags := h.setParameters(ctx, config, body)
	if diags.HasError() {
		return diags
	}
	diags.Append(h.testIntegration(ctx, body)...)
	return diags
}

// testIntegration asks the API to test the integration.
func (h *integrationHooks) testIntegration(ctx context.Context, body any) diag.Diagnostics {
	metadata := requestMetadata(body)
	if metadata == nil {
		return diag.Diagnostics{diag.NewErrorDiagnostic("Unexpected request body", fmt.Sprintf("The integration request body is %T.", body))}
	}
	result, httpResponse, err := h.client.IntegrationServiceTestIntegration(ctx).
		TestIntegrationRequest(integrationsSDK.TestIntegrationRequest{IntegrationData: *metadata}).
		Execute()
	if err != nil {
		return diag.Diagnostics{diag.NewErrorDiagnostic("Testing the integration has failed",
			fmt.Sprintf("API responded with an error: %v", cxsdk.NewAPIError(httpResponse, err)))}
	}
	if result != nil && result.Result != nil && result.Result.Failure != nil {
		return diag.Diagnostics{diag.NewErrorDiagnostic("Invalid integration configuration",
			fmt.Sprintf("API responded with an error: %s", result.Result.Failure.GetErrorMessage()))}
	}
	return nil
}

// rememberParameters keeps the parameters of the API value for setStateParameters of the same call.
func (h *integrationHooks) rememberParameters(ctx context.Context, v *integrationsSDK.DeployedIntegrationInformation) diag.Diagnostics {
	if handoff, ok := ctx.Value(handoffKey{}).(*parameterHandoff); ok {
		handoff.parameters = v.GetParameters()
	}
	return nil
}

// setStateParameters sets `parameters` of the state. The value of the plan (Create and Update) or
// of the prior state (Read) stays: the backend holds more keys than the configuration, and shows
// secrets as a placeholder. Without a known prior value, for example after an import, the state
// holds every parameter of the backend, a secret as <redacted> and an api key as its value.
func (h *integrationHooks) setStateParameters(ctx context.Context, state *tfsdk.State, prior any) diag.Diagnostics {
	var diags diag.Diagnostics
	if p, ok := prior.(interface {
		GetAttribute(context.Context, path.Path, interface{}) diag.Diagnostics
	}); ok && p != nil {
		var parameters types.Dynamic
		diags.Append(p.GetAttribute(ctx, path.Root("parameters"), &parameters)...)
		if diags.HasError() {
			return diags
		}
		if hasKnownParameters(parameters) {
			diags.Append(state.SetAttribute(ctx, path.Root("parameters"), parameters)...)
			return diags
		}
	}
	var backend []integrationsSDK.Parameter
	if handoff, ok := ctx.Value(handoffKey{}).(*parameterHandoff); ok {
		backend = handoff.parameters
	}
	parameters, d := parametersToDynamic(backend, nil)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	diags.Append(state.SetAttribute(ctx, path.Root("parameters"), parameters)...)
	return diags
}

func hasKnownParameters(parameters types.Dynamic) bool {
	if parameters.IsNull() || parameters.IsUnknown() || parameters.UnderlyingValue() == nil {
		return false
	}
	return !parameters.IsUnderlyingValueNull() && !parameters.IsUnderlyingValueUnknown()
}

// dynamicToParameters turns the object of the configuration into API parameters, sorted by key.
func dynamicToParameters(planParameters types.Dynamic) ([]integrationsSDK.Parameter, diag.Diagnostics) {
	parameters := make([]integrationsSDK.Parameter, 0)

	obj, ok := planParameters.UnderlyingValue().(types.Object)
	if !ok {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Parameters have to be an object", fmt.Sprintf("Invalid parameters: %v", planParameters.UnderlyingValue()))}
	}
	attributes := obj.Attributes()
	keys := make([]string, 0, len(attributes))
	for key := range attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		key := key
		param := integrationsSDK.Parameter{}
		switch v := attributes[key].(type) {
		case types.String:
			param.StringValue = v.ValueStringPointer()
			param.Key = &key
		case types.Number:
			f, _ := v.ValueBigFloat().Float64()
			param.NumericValue = &f
			param.Key = &key
		case types.Bool:
			param.BooleanValue = v.ValueBoolPointer()
			param.Key = &key
		case types.List:
			stringlist, diags := collectionToParameters(v.Elements())
			if diags.HasError() {
				return nil, diags
			}
			stringlist.Key = &key
			param = *stringlist
		case types.Tuple:
			stringlist, diags := collectionToParameters(v.Elements())
			if diags.HasError() {
				return nil, diags
			}
			stringlist.Key = &key
			param = *stringlist
		default:
			return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid parameter type", fmt.Sprintf("Invalid parameter type %v: %v", v, obj))}
		}
		parameters = append(parameters, param)
	}
	return parameters, nil
}

func collectionToParameters(elements []attr.Value) (*integrationsSDK.Parameter, diag.Diagnostics) {
	strings := make([]string, len(elements))
	for i, value := range elements {
		switch value := value.(type) {
		case types.String:
			if !value.IsNull() && value.ValueString() != "" {
				strings[i] = value.ValueString()
			}
		default:
			return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid parameter type", fmt.Sprintf("Invalid parameter type %v: %v", value, elements))}
		}
	}
	return &integrationsSDK.Parameter{
		StringList: &integrationsSDK.StringList{
			Values: strings,
		},
	}, nil
}

// parametersToDynamic returns the parameters of the backend as an object. With keys, it holds
// only those keys. With nil keys, it holds all of them.
func parametersToDynamic(parameters []integrationsSDK.Parameter, keys []string) (types.Dynamic, diag.Diagnostics) {
	obj := make(map[string]attr.Value, len(parameters))
	t := make(map[string]attr.Type, len(parameters))
	for _, parameter := range parameters {
		if parameter.Key == nil || keys != nil && !slices.Contains(keys, *parameter.Key) {
			continue
		}
		value, valueType, diags, ok := parameterToAttr(parameter)
		if diags.HasError() {
			return types.Dynamic{}, diags
		}
		if !ok {
			log.Printf("[WARN] Invalid parameter type: %v", utils.FormatJSON(parameter))
			continue
		}
		obj[*parameter.Key] = value
		t[*parameter.Key] = valueType
	}
	val, diags := types.ObjectValue(t, obj)
	return types.DynamicValue(val), diags
}

func parameterToAttr(parameter integrationsSDK.Parameter) (attr.Value, attr.Type, diag.Diagnostics, bool) {
	if parameter.StringList != nil {
		values := make([]attr.Value, len(parameter.StringList.Values))
		assignedTypes := make([]attr.Type, len(parameter.StringList.Values))
		for i, value := range parameter.StringList.Values {
			values[i] = types.StringValue(value)
			assignedTypes[i] = types.StringType
		}
		parameters, diags := types.TupleValue(assignedTypes, values)
		return parameters, types.TupleType{ElemTypes: assignedTypes}, diags, true
	}
	if parameter.BooleanValue != nil {
		return types.BoolPointerValue(parameter.BooleanValue), types.BoolType, nil, true
	}
	if parameter.StringValue != nil {
		return types.StringPointerValue(parameter.StringValue), types.StringType, nil, true
	}
	if parameter.NumericValue != nil {
		return types.NumberValue(big.NewFloat(*parameter.NumericValue)), types.NumberType, nil, true
	}
	if parameter.ApiKey != nil {
		return types.StringPointerValue(parameter.ApiKey.Value), types.StringType, nil, true
	}
	if parameter.SensitiveData != nil {
		return types.StringValue("<redacted>"), types.StringType, nil, true
	}
	return nil, nil, nil, false
}
