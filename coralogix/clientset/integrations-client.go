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
	ext "terraform-provider-coralogix/coralogix/clientset/grpc/integrations"
)

// UpdateIntegrationRequest is a request to update an integration.
type UpdateIntegrationRequest = ext.UpdateIntegrationRequest

// SaveIntegrationRequest is a request to create an integration.
type SaveIntegrationRequest = ext.SaveIntegrationRequest

// DeleteIntegrationRequest is a request to delete an integration.
type DeleteIntegrationRequest = ext.DeleteIntegrationRequest

// GetIntegrationDetailsRequest is a request to get integration details.
type GetIntegrationDetailsRequest = ext.GetIntegrationDetailsRequest

// GetIntegrationDefinitionRequest is a request to get an integration definition.
type GetIntegrationDefinitionRequest = ext.GetIntegrationDefinitionRequest

// GetManagedIntegrationStatusRequest is a request to get the status of a managed integration.
type GetManagedIntegrationStatusRequest = ext.GetManagedIntegrationStatusRequest

// GetTemplateRequest is a request to get an integration template.
type GetTemplateRequest = ext.GetTemplateRequest

// GetRumApplicationVersionDataRequest is a request to get RUM application version data.
type GetRumApplicationVersionDataRequest = ext.GetRumApplicationVersionDataRequest

// SyncRumDataRequest is a request to sync RUM data.
type SyncRumDataRequest = ext.SyncRumDataRequest

// TestIntegrationRequest is a request to test an integration.
type TestIntegrationRequest = ext.TestIntegrationRequest

// IntegrationsClient is a client for the Coralogix Extensions API.
type IntegrationsClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

// Create creates a new integration.
func (c IntegrationsClient) Create(ctx context.Context, req *SaveIntegrationRequest) (*ext.SaveIntegrationResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := ext.NewIntegrationServiceClient(conn)

	return client.SaveIntegration(callProperties.Ctx, req, callProperties.CallOptions...)
}

// Update updates an integration
func (c IntegrationsClient) Update(ctx context.Context, req *UpdateIntegrationRequest) (*ext.UpdateIntegrationResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := ext.NewIntegrationServiceClient(conn)

	return client.UpdateIntegration(callProperties.Ctx, req, callProperties.CallOptions...)
}

// Get gets integration details
func (c IntegrationsClient) Get(ctx context.Context, req *GetIntegrationDetailsRequest) (*ext.GetIntegrationDetailsResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := ext.NewIntegrationServiceClient(conn)

	return client.GetIntegrationDetails(callProperties.Ctx, req, callProperties.CallOptions...)
}

// GetDefinition gets an integration definition
func (c IntegrationsClient) GetDefinition(ctx context.Context, req *GetIntegrationDefinitionRequest) (*ext.GetIntegrationDefinitionResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := ext.NewIntegrationServiceClient(conn)

	return client.GetIntegrationDefinition(callProperties.Ctx, req, callProperties.CallOptions...)
}

// GetIntegrationStatus gets the status of a integration
func (c IntegrationsClient) GetIntegrationStatus(ctx context.Context, req *GetManagedIntegrationStatusRequest) (*ext.GetManagedIntegrationStatusResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := ext.NewIntegrationServiceClient(conn)

	return client.GetManagedIntegrationStatus(callProperties.Ctx, req, callProperties.CallOptions...)
}

// Delete deletes an integration
func (c IntegrationsClient) Delete(ctx context.Context, req *DeleteIntegrationRequest) (*ext.DeleteIntegrationResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := ext.NewIntegrationServiceClient(conn)

	return client.DeleteIntegration(callProperties.Ctx, req, callProperties.CallOptions...)
}

// GetTemplate gets an integration template
func (c IntegrationsClient) GetTemplate(ctx context.Context, req *GetTemplateRequest) (*ext.GetTemplateResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := ext.NewIntegrationServiceClient(conn)

	return client.GetTemplate(callProperties.Ctx, req, callProperties.CallOptions...)
}

// GetRumApplicationVersionData gets RUM application version data
func (c IntegrationsClient) GetRumApplicationVersionData(ctx context.Context, req *GetRumApplicationVersionDataRequest) (*ext.GetRumApplicationVersionDataResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := ext.NewIntegrationServiceClient(conn)

	return client.GetRumApplicationVersionData(callProperties.Ctx, req, callProperties.CallOptions...)
}

// SyncRumData syncs RUM data
func (c IntegrationsClient) SyncRumData(ctx context.Context, req *SyncRumDataRequest) (*ext.SyncRumDataResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := ext.NewIntegrationServiceClient(conn)

	return client.SyncRumData(callProperties.Ctx, req, callProperties.CallOptions...)
}

// TestIntegration tests an integration
func (c IntegrationsClient) TestIntegration(ctx context.Context, req *TestIntegrationRequest) (*ext.TestIntegrationResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := ext.NewIntegrationServiceClient(conn)

	return client.TestIntegration(callProperties.Ctx, req, callProperties.CallOptions...)
}

// NewIntegrationsClient creates a new client.
func NewIntegrationsClient(c *CallPropertiesCreator) *IntegrationsClient {
	return &IntegrationsClient{callPropertiesCreator: c}
}
