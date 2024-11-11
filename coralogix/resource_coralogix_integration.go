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
	"log"
	"math/big"
	"terraform-provider-coralogix/coralogix/clientset"
	integrations "terraform-provider-coralogix/coralogix/clientset/grpc/integrations"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/wrapperspb"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	createIntegrationsUrl = integrations.IntegrationService_SaveIntegration_FullMethodName
	deleteIntegrationsUrl = integrations.IntegrationService_DeleteIntegration_FullMethodName
	getIntegrationsUrl    = integrations.IntegrationService_GetDeployedIntegration_FullMethodName
	updateIntegrationsUrl = integrations.IntegrationService_UpdateIntegration_FullMethodName
)

func NewIntegrationResource() resource.Resource {
	return &IntegrationResource{}
}

type IntegrationResource struct {
	client *clientset.IntegrationsClient
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
				MarkdownDescription: "Data required for the integration.",
			},
		},
		MarkdownDescription: "A Coralogix Integration",
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
	createReq, diags := extractCreateIntegration(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	result, testErr := r.client.TestIntegration(ctx, &integrations.TestIntegrationRequest{
		IntegrationData: createReq.Metadata,
	})
	log.Printf("[INFO] Creating new Integration: %s", protojson.Format(createReq))

	if testErr != nil {
		newDiags := diag.Diagnostics{diag.NewErrorDiagnostic("Testing the integration has failed", fmt.Sprintf("API responded with an error: %v", testErr.Error()))}
		resp.Diagnostics.Append(newDiags...)
		return
	}

	fail, hasFailed := result.Result.Result.(*integrations.TestIntegrationResult_Failure_)
	if hasFailed {
		newDiags := diag.Diagnostics{diag.NewErrorDiagnostic("Invalid integration configuration", fmt.Sprintf("API responded with an error: %v", fail.Failure.ErrorMessage))}
		resp.Diagnostics.Append(newDiags...)
		return
	}

	createResp, err := r.client.Create(ctx, createReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error creating Integration",
			formatRpcErrors(err, createIntegrationsUrl, protojson.Format(createReq)),
		)
		return
	}
	log.Printf("[INFO] Submitted new integration: %s", protojson.Format(createResp))

	getIntegrationReq := &integrations.GetDeployedIntegrationRequest{
		IntegrationId: createResp.IntegrationId,
	}
	log.Printf("[INFO] Getting new Integration: %s", protojson.Format(getIntegrationReq))

	getIntegrationResp, err := r.client.Get(ctx, getIntegrationReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error reading Integration",
			formatRpcErrors(err, getIntegrationsUrl, protojson.Format(getIntegrationReq)),
		)
		return
	}
	log.Printf("[INFO] Received Integration: %s", protojson.Format(getIntegrationResp))
	state, e := integrationDetail(getIntegrationResp)
	if e.HasError() {
		resp.Diagnostics.Append(e...)
		return
	}
	// Set state to fully populated data
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func extractCreateIntegration(ctx context.Context, plan *IntegrationResourceModel) (*integrations.SaveIntegrationRequest, diag.Diagnostics) {
	parameters, diags := dynamicToParameters(ctx, plan.Parameters)
	if diags.HasError() {
		return nil, diags
	}
	return &integrations.SaveIntegrationRequest{
		Metadata: &integrations.IntegrationMetadata{
			IntegrationKey: wrapperspb.String(plan.IntegrationKey.ValueString()),
			Version:        wrapperspb.String(plan.Version.ValueString()),
			SpecificData: &integrations.IntegrationMetadata_IntegrationParameters{
				IntegrationParameters: &integrations.GenericIntegrationParameters{
					Parameters: parameters,
				},
			},
		},
	}, diag.Diagnostics{}
}

func extractUpdateIntegration(ctx context.Context, plan *IntegrationResourceModel) (*integrations.UpdateIntegrationRequest, diag.Diagnostics) {

	parameters, diags := dynamicToParameters(ctx, plan.Parameters)
	if diags.HasError() {
		return nil, diags
	}
	return &integrations.UpdateIntegrationRequest{
		Id: wrapperspb.String(plan.ID.ValueString()),
		Metadata: &integrations.IntegrationMetadata{
			IntegrationKey: wrapperspb.String(plan.IntegrationKey.ValueString()),
			Version:        wrapperspb.String(plan.Version.ValueString()),
			SpecificData: &integrations.IntegrationMetadata_IntegrationParameters{
				IntegrationParameters: &integrations.GenericIntegrationParameters{
					Parameters: parameters,
				},
			},
		},
	}, diag.Diagnostics{}
}

func dynamicToParameters(ctx context.Context, planParameters types.Dynamic) ([]*integrations.Parameter, diag.Diagnostics) {
	parameters := make([]*integrations.Parameter, 0)

	switch p := planParameters.UnderlyingValue().(type) {
	case types.Object:
		obj := planParameters.UnderlyingValue().(types.Object)
		obj.Attributes()
		for key, value := range obj.Attributes() {
			switch v := value.(type) {
			case types.String:
				parameters = append(parameters, &integrations.Parameter{
					Key:   key,
					Value: &integrations.Parameter_StringValue{StringValue: wrapperspb.String(v.ValueString())},
				})
			case types.Number:
				f, _ := v.ValueBigFloat().Float64()
				parameters = append(parameters, &integrations.Parameter{
					Key:   key,
					Value: &integrations.Parameter_NumericValue{NumericValue: wrapperspb.Double(f)},
				})
			case types.Bool:
				b := v.ValueBool()
				parameters = append(parameters, &integrations.Parameter{
					Key:   key,
					Value: &integrations.Parameter_BooleanValue{BooleanValue: wrapperspb.Bool(b)},
				})
			case types.Tuple:

				strings := make([]*wrapperspb.StringValue, len(v.Elements()))
				for i, value := range v.Elements() {
					switch value := value.(type) {
					case types.String:
						if !value.IsNull() && value.ValueString() != "" {
							strings[i] = wrapperspb.String(value.ValueString())
						}
					default:
						return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid parameter type", fmt.Sprintf("Invalid parameter type %v: %v", v, p))}
					}

				}

				parameters = append(parameters, &integrations.Parameter{
					Key: key,
					Value: &integrations.Parameter_StringList_{
						StringList: &integrations.Parameter_StringList{
							Values: strings,
						}},
				})
			default:
				return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Invalid parameter type", fmt.Sprintf("Invalid parameter type (Lists have to be tuples) %v: %v", v, p))}
			}
		}
	default:
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Parameters have to be an object", fmt.Sprintf("Invalid parameters: %v", planParameters.UnderlyingValue()))}
	}
	return parameters, diag.Diagnostics{}
}

func integrationDetail(resp *integrations.GetDeployedIntegrationResponse) (*IntegrationResourceModel, diag.Diagnostics) {

	integration := resp.Integration
	parameters, diags := parametersToDynamic(integration.GetParameters())
	if diags.HasError() {
		return nil, diags
	}

	return &IntegrationResourceModel{
		ID:             types.StringValue(integration.Id.Value),
		IntegrationKey: types.StringValue(integration.DefinitionKey.Value),
		Version:        types.StringValue(integration.DefinitionVersion.Value),
		Parameters:     parameters,
	}, diag.Diagnostics{}
}

func parametersToDynamic(parameters []*integrations.Parameter) (types.Dynamic, diag.Diagnostics) {
	obj := make(map[string]attr.Value, len(parameters))
	t := make(map[string]attr.Type, len(parameters))
	for _, parameter := range parameters {
		switch v := parameter.Value.(type) {
		case *integrations.Parameter_StringValue:
			obj[parameter.Key] = types.StringValue(v.StringValue.Value)
			t[parameter.Key] = types.StringType
		case *integrations.Parameter_ApiKey:
			obj[parameter.Key] = types.StringValue(v.ApiKey.Value.Value)
			t[parameter.Key] = types.StringType
		case *integrations.Parameter_SensitiveData:
			obj[parameter.Key] = types.StringValue("<redacted>")
			t[parameter.Key] = types.StringType
		case *integrations.Parameter_NumericValue:
			obj[parameter.Key] = types.NumberValue(big.NewFloat(v.NumericValue.Value))
			t[parameter.Key] = types.NumberType
		case *integrations.Parameter_BooleanValue:
			obj[parameter.Key] = types.BoolValue(v.BooleanValue.Value)
			t[parameter.Key] = types.BoolType
		case *integrations.Parameter_StringList_:
			values := make([]attr.Value, len(v.StringList.Values))
			assignedTypes := make([]attr.Type, len(v.StringList.Values))
			for i, value := range v.StringList.Values {
				values[i] = types.StringValue(value.Value)
				assignedTypes[i] = types.StringType
			}
			parameters, diags := types.TupleValue(assignedTypes, values)
			if diags.HasError() {
				return types.Dynamic{}, diags
			}
			obj[parameter.Key] = parameters
			t[parameter.Key] = types.TupleType{ElemTypes: assignedTypes}
		default:
			obj[parameter.Key] = types.StringValue(protojson.Format(parameter))
			t[parameter.Key] = types.StringType
			log.Printf("[WARN] Invalid parameter type: %v", v)
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

	getIntegrationReq := &integrations.GetDeployedIntegrationRequest{
		IntegrationId: wrapperspb.String(plan.ID.ValueString()),
	}
	log.Printf("[INFO] Reading Integration: %s", protojson.Format(getIntegrationReq))
	getIntegrationResp, err := r.client.Get(ctx, getIntegrationReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Integration %v is in state, but no longer exists in Coralogix backend", plan.ID.ValueString()),
				fmt.Sprintf("%q will be recreated when you apply", plan.ID.ValueString()),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Integration",
				formatRpcErrors(err, getIntegrationsUrl, protojson.Format(getIntegrationReq)),
			)
		}
		return
	}
	log.Printf("[INFO] Received Integration: %s", protojson.Format(getIntegrationResp))

	state, e := integrationDetail(getIntegrationResp)
	if e.HasError() {
		resp.Diagnostics.Append(e...)
		return
	}
	// Set state to fully populated data
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

	updateReq, diags := extractUpdateIntegration(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	log.Printf("[INFO] Updating Integration: %s", protojson.Format(updateReq))

	_, testErr := r.client.TestIntegration(ctx, &integrations.TestIntegrationRequest{
		IntegrationData: updateReq.Metadata,
	})
	if testErr != nil {
		resp.Diagnostics.Append(diags...)
		return
	}

	_, err := r.client.Update(ctx, updateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating Integration",
			formatRpcErrors(err, updateIntegrationsUrl, protojson.Format(updateReq)),
		)
		return
	}

	log.Printf("[INFO] Updated scope: %s", plan.ID.ValueString())

	getIntegrationReq := &integrations.GetDeployedIntegrationRequest{
		IntegrationId: wrapperspb.String(plan.ID.ValueString()),
	}
	getIntegrationResp, err := r.client.Get(ctx, getIntegrationReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error reading Integration",
			formatRpcErrors(err, getIntegrationsUrl, protojson.Format(getIntegrationReq)),
		)
		return
	}
	log.Printf("[INFO] Received Integration: %s", protojson.Format(getIntegrationResp))
	state, e := integrationDetail(getIntegrationResp)
	if e.HasError() {
		resp.Diagnostics.Append(e...)
		return
	}
	// Set state to fully populated data
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

	log.Printf("[INFO] Deleting Integration: %s", state.ID.ValueString())

	deleteReq := &integrations.DeleteIntegrationRequest{IntegrationId: wrapperspb.String(state.ID.ValueString())}
	log.Printf("[INFO] Deleting Integration: %s", protojson.Format(deleteReq))
	_, err := r.client.Delete(ctx, deleteReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error deleting Integration",
			formatRpcErrors(err, deleteIntegrationsUrl, protojson.Format(deleteReq)),
		)
		return
	}
	log.Printf("[INFO] Deleted scope: %s", state.ID.ValueString())
}
