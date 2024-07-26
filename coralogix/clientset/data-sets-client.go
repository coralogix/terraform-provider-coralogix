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

package clientset

import (
	"context"

	enrichment "terraform-provider-coralogix/coralogix/clientset/grpc/enrichment/v1"
)

type DataSetClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (d DataSetClient) CreatDataSet(ctx context.Context, req *enrichment.CreateCustomEnrichmentRequest) (*enrichment.CreateCustomEnrichmentResponse, error) {
	callProperties, err := d.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := enrichment.NewCustomEnrichmentServiceClient(conn)

	return client.CreateCustomEnrichment(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (d DataSetClient) GetDataSet(ctx context.Context, req *enrichment.GetCustomEnrichmentRequest) (*enrichment.GetCustomEnrichmentResponse, error) {
	callProperties, err := d.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := enrichment.NewCustomEnrichmentServiceClient(conn)

	return client.GetCustomEnrichment(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (d DataSetClient) UpdateDataSet(ctx context.Context, req *enrichment.UpdateCustomEnrichmentRequest) (*enrichment.UpdateCustomEnrichmentResponse, error) {
	callProperties, err := d.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := enrichment.NewCustomEnrichmentServiceClient(conn)

	return client.UpdateCustomEnrichment(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (d DataSetClient) DeleteDataSet(ctx context.Context, req *enrichment.DeleteCustomEnrichmentRequest) (*enrichment.DeleteCustomEnrichmentResponse, error) {
	callProperties, err := d.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()

	client := enrichment.NewCustomEnrichmentServiceClient(conn)

	return client.DeleteCustomEnrichment(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewDataSetClient(c *CallPropertiesCreator) *DataSetClient {
	return &DataSetClient{callPropertiesCreator: c}
}
