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
	"net/http"
	"slices"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	integrations "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/integration_service"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewIntegrationResource() resource.Resource {
	return &IntegrationResource{}
}

type IntegrationResource struct {
	client *integrations.IntegrationServiceAPIService
}

func (r *IntegrationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration"
}

func (r *IntegrationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clientSet, ok := req.ProviderData.(*clientset.ClientSet)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *clientset.ClientSet, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = clientSet.Integrations()
}

func (r *IntegrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *IntegrationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Integration ID.",
			},
			"integration_key": schema.StringAttribute{
				MarkdownDescription: "Selector for the integration.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"version": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The integration version",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"parameters": schema.DynamicAttribute{
				Required:            true,
				MarkdownDescription: "Parameters required by the integration.",
			},
		},
		MarkdownDescription: "A Coralogix Integration. Check https://coralogix.com/docs/developer-portal/infrastructure-as-code/terraform-provider/integrations/aws-metrics-collector/ for available options.",
	}
}

type IntegrationResourceModel struct {
	ID             types.String  `tfsdk:"id"`
	IntegrationKey types.String  `tfsdk:"integration_key"`
	Version        types.String  `tfsdk:"version"`
	Parameters     types.Dynamic `tfsdk:"parameters"`
}

func (r *IntegrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *IntegrationResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rq, diags := extractCreateIntegration(plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	testResult, _, testErr := r.client.IntegrationServiceTestIntegration(ctx).TestIntegrationRequest(integrations.TestIntegrationRequest{
		IntegrationData: rq.Metadata,
	}).Execute()

	if testErr != nil {
		newDiags := diag.Diagnostics{diag.NewErrorDiagnostic("Testing the integration has failed", fmt.Sprintf("API responded with an error: %v", testErr.Error()))}
		resp.Diagnostics.Append(newDiags...)
		return
	}

	if testResult.Result.TestIntegrationResultFailure != nil {
		// TODO after the data structure is fixed, change to print the error message
		newDiags := diag.Diagnostics{diag.NewErrorDiagnostic("Invalid integration configuration", fmt.Sprintf("API responded with an error: %v", testResult.Result.TestIntegrationResultFailure))}
		resp.Diagnostics.Append(newDiags...)
		return
	}

	result, httpResponse, err := r.client.IntegrationServiceSaveIntegration(ctx).
		SaveIntegrationRequest(*rq).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error creating coralogix_integration",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq),
		)
		return
	}

	readRq := r.client.IntegrationServiceGetDeployedIntegration(ctx, *result.IntegrationId)
	readResult, _, err := readRq.Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error reading coralogix_integration",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
		)
		return
	}

	keys, diags := KeysFromPlan(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	state, e := integrationDetail(readResult, keys)
	state.Parameters = plan.Parameters
	if e.HasError() {
		resp.Diagnostics.Append(e...)
		return
	}

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func KeysFromPlan(ctx context.Context, plan *IntegrationResourceModel) ([]string, diag.Diagnostics) {
	// extract keys first to filter the returned parameters later
	parameters, diags := dynamicToParameters(plan.Parameters)
	keys := make([]string, len(parameters))
	for i, parameter := range parameters {
		if parameter.ParameterStringList != nil {
			keys[i] = *parameter.ParameterStringList.Key
		} else if parameter.ParameterBooleanValue != nil {
			keys[i] = *parameter.ParameterBooleanValue.Key
		} else if parameter.ParameterStringValue != nil {
			keys[i] = *parameter.ParameterStringValue.Key
		} else if parameter.ParameterNumericValue != nil {
			keys[i] = *parameter.ParameterNumericValue.Key
		}
	}
	return keys, diags
}

func extractCreateIntegration(plan *IntegrationResourceModel) (*integrations.SaveIntegrationRequest, diag.Diagnostics) {
	parameters, diags := dynamicToParameters(plan.Parameters)
	if diags.HasError() {
		return nil, diags
	}
	return &integrations.SaveIntegrationRequest{
		Metadata: &integrations.IntegrationMetadata{
			IntegrationKey: plan.IntegrationKey.ValueStringPointer(),
			Version:        plan.Version.ValueStringPointer(),
			IntegrationParameters: &integrations.GenericIntegrationParameters{
				Parameters: parameters,
			},
		},
	}, diag.Diagnostics{}
}

func extractUpdateIntegration(plan *IntegrationResourceModel) (*integrations.UpdateIntegrationRequest, diag.Diagnostics) {

	parameters, diags := dynamicToParameters(plan.Parameters)
	if diags.HasError() {
		return nil, diags
	}
	return &integrations.UpdateIntegrationRequest{
		Id: plan.ID.ValueStringPointer(),
		Metadata: &integrations.IntegrationMetadata{
			IntegrationKey: plan.IntegrationKey.ValueStringPointer(),
			Version:        plan.Version.ValueStringPointer(),
			IntegrationParameters: &integrations.GenericIntegrationParameters{
				Parameters: parameters,
			},
		},
	}, diag.Diagnostics{}
}

func dynamicToParameters(planParameters types.Dynamic) ([]integrations.Parameter, diag.Diagnostics) {
	parameters := make([]integrations.Parameter, 0)

	switch p := planParameters.UnderlyingValue().(type) {
	case types.Object:
		obj := planParameters.UnderlyingValue().(types.Object)
		obj.Attributes()
		for key, value := range obj.Attributes() {
			param := integrations.Parameter{}
			switch v := value.(type) {
			case types.String:
				param.ParameterStringValue = &integrations.ParameterStringValue{
					StringValue: v.ValueStringPointer(),
					Key:         &key,
				}
			case types.Number:
				f, _ := v.ValueBigFloat().Float64()
				param.ParameterNumericValue = &integrations.ParameterNumericValue{
					NumericValue: &f,
					Key:          &key,
				}
			case types.Bool:
				param.ParameterBooleanValue = &integrations.ParameterBooleanValue{
					BooleanValue: v.ValueBoolPointer(),
					Key:          &key,
				}
			case types.List:
				stringlist, diags := collectionToParameters(v.Elements())
				if diags.HasError() {
					return nil, diags
				}

				stringlist.Key = &key
				param.ParameterStringList = stringlist
			case types.Tuple:
				stringlist, diags := collectionToParameters(v.Elements())

				if diags.HasError() {
					return nil, diags
				}

				stringlist.Key = &key
				param.ParameterStringList = stringlist
			default:
				return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid parameter type", fmt.Sprintf("Invalid parameter type %v: %v", v, p))}
			}
			parameters = append(parameters, param)

		}
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Parameters have to be an object", fmt.Sprintf("Invalid parameters: %v", planParameters.UnderlyingValue()))}
	}
	return parameters, diag.Diagnostics{}
}

func collectionToParameters(elements []attr.Value) (*integrations.ParameterStringList, diag.Diagnostics) {
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
	return &integrations.ParameterStringList{
		StringList: &integrations.StringList{
			Values: strings,
		},
	}, nil
}

func integrationDetail(resp *integrations.GetDeployedIntegrationResponse, keys []string) (*IntegrationResourceModel, diag.Diagnostics) {

	integration := resp.Integration
	parameters, diags := parametersToDynamic(integration.GetParameters(), keys)
	if diags.HasError() {
		return nil, diags
	}

	return &IntegrationResourceModel{
		ID:             types.StringPointerValue(integration.Id),
		IntegrationKey: types.StringPointerValue(integration.DefinitionKey),
		Version:        types.StringPointerValue(integration.DefinitionVersion),
		Parameters:     parameters,
	}, diag.Diagnostics{}
}

func parametersToDynamic(parameters []integrations.Parameter, keys []string) (types.Dynamic, diag.Diagnostics) {
	obj := make(map[string]attr.Value, len(parameters))
	t := make(map[string]attr.Type, len(parameters))
	for _, parameter := range parameters {

		if parameter.ParameterStringList != nil && slices.Contains(keys, *parameter.ParameterStringList.Key) {
			v := parameter.ParameterStringList
			values := make([]attr.Value, len(v.StringList.Values))
			assignedTypes := make([]attr.Type, len(v.StringList.Values))
			for i, value := range v.StringList.Values {
				values[i] = types.StringValue(value)
				assignedTypes[i] = types.StringType
			}
			parameters, diags := types.TupleValue(assignedTypes, values)
			if diags.HasError() {
				return types.Dynamic{}, diags
			}
			obj[*parameter.ParameterStringList.Key] = parameters
			t[*parameter.ParameterStringList.Key] = types.TupleType{ElemTypes: assignedTypes}

		} else if parameter.ParameterBooleanValue != nil && slices.Contains(keys, *parameter.ParameterBooleanValue.Key) {
			obj[*parameter.ParameterBooleanValue.Key] = types.BoolPointerValue(parameter.ParameterBooleanValue.BooleanValue)
			t[*parameter.ParameterBooleanValue.Key] = types.BoolType

		} else if parameter.ParameterStringValue != nil && slices.Contains(keys, *parameter.ParameterStringValue.Key) {
			obj[*parameter.ParameterStringValue.Key] = types.StringPointerValue(parameter.ParameterStringValue.StringValue)
			t[*parameter.ParameterStringValue.Key] = types.StringType

		} else if parameter.ParameterNumericValue != nil && slices.Contains(keys, *parameter.ParameterNumericValue.Key) {
			obj[*parameter.ParameterNumericValue.Key] = types.NumberValue(big.NewFloat(*parameter.ParameterNumericValue.NumericValue))
			t[*parameter.ParameterNumericValue.Key] = types.NumberType

		} else if parameter.ParameterApiKey != nil && slices.Contains(keys, *parameter.ParameterApiKey.Key) {
			obj[*parameter.ParameterApiKey.Key] = types.StringPointerValue(parameter.ParameterApiKey.ApiKey.Value)
			t[*parameter.ParameterApiKey.Key] = types.StringType

		} else if parameter.ParameterSensitiveData != nil && slices.Contains(keys, *parameter.ParameterSensitiveData.Key) {
			obj[*parameter.ParameterSensitiveData.Key] = types.StringValue("<redacted>")
			t[*parameter.ParameterSensitiveData.Key] = types.StringType
		} else {
			log.Printf("[WARN] Invalid parameter type: %v", utils.FormatJSON(parameter))
		}
	}
	val, e := types.ObjectValue(t, obj)
	return types.DynamicValue(val), e
}

func (r *IntegrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var plan *IntegrationResourceModel
	diags := req.State.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := plan.ID.ValueString()

	rq := r.client.IntegrationServiceGetDeployedIntegration(ctx, id)
	result, httpResponse, err := rq.Execute()

	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Resource %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error reading coralogix_integration",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
			)
		}
		return
	}

	keys, diags := KeysFromPlan(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	state, e := integrationDetail(result, keys)
	state.Parameters = plan.Parameters
	if e.HasError() {
		resp.Diagnostics.Append(e...)
		return
	}

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *IntegrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan *IntegrationResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := plan.ID.ValueString()

	rq, diags := extractUpdateIntegration(plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	testResult, _, testErr := r.client.IntegrationServiceTestIntegration(ctx).TestIntegrationRequest(integrations.TestIntegrationRequest{
		IntegrationData: rq.Metadata,
	}).Execute()

	if testErr != nil {
		newDiags := diag.Diagnostics{diag.NewErrorDiagnostic("Testing the integration has failed", fmt.Sprintf("API responded with an error: %v", testErr.Error()))}
		resp.Diagnostics.Append(newDiags...)
		return
	}

	if testResult.Result.TestIntegrationResultFailure != nil {
		// TODO after the data structure is fixed, change to print the error message
		newDiags := diag.Diagnostics{diag.NewErrorDiagnostic("Invalid integration configuration", fmt.Sprintf("API responded with an error: %v", testResult.Result.TestIntegrationResultFailure))}
		resp.Diagnostics.Append(newDiags...)
		return
	}

	result, httpResponse, err := r.client.IntegrationServiceUpdateIntegration(ctx).
		UpdateIntegrationRequest(*rq).
		Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error update coralogix_integration",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Update", rq),
		)
		return
	}

	readRq := r.client.IntegrationServiceGetDeployedIntegration(ctx, id)
	readResult, _, err := readRq.Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error reading coralogix_integration",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
		)
		return
	}

	keys, diags := KeysFromPlan(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	state, e := integrationDetail(readResult, keys)
	state.Parameters = plan.Parameters
	if e.HasError() {
		resp.Diagnostics.Append(e...)
		return
	}

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *IntegrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state *IntegrationResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()

	_, httpResponse, err := r.client.
		IntegrationServiceDeleteIntegration(ctx, id).
		Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error deleting coralogix_integration",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", nil),
		)
		return
	}
}
