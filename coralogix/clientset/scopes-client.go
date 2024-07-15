package clientset

import (
	"context"
	scopes "terraform-provider-coralogix/coralogix/clientset/grpc/scopes"
)

type CreateScopeRequest = scopes.CreateScopeRequest
type GetTeamScopesByIdsRequest = scopes.GetTeamScopesByIdsRequest
type GetTeamScopesRequest = scopes.GetTeamScopesRequest
type UpdateScopeRequest = scopes.UpdateScopeRequest
type DeleteScopeRequest = scopes.DeleteScopeRequest

type Filter = scopes.Filter

const (
	EntityType_UNSPECIFIED = scopes.EntityType_UNSPECIFIED
	EntityType_LOGS        = scopes.EntityType_LOGS
	EntityType_SPANS       = scopes.EntityType_SPANS
)

type ScopesClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

// Create a new scope
func (c ScopesClient) Create(ctx context.Context, req *CreateScopeRequest) (*scopes.CreateScopeResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := scopes.NewScopesServiceClient(conn)

	return client.CreateScope(callProperties.Ctx, req, callProperties.CallOptions...)
}

// Get a scope by its ID
func (c ScopesClient) Get(ctx context.Context, req *GetTeamScopesByIdsRequest) (*scopes.GetScopesResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := scopes.NewScopesServiceClient(conn)

	return client.GetTeamScopesByIds(callProperties.Ctx, req, callProperties.CallOptions...)
}

// List all scopes for the current team
func (c ScopesClient) List(ctx context.Context, req *GetTeamScopesRequest) (*scopes.GetScopesResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := scopes.NewScopesServiceClient(conn)

	return client.GetTeamScopes(callProperties.Ctx, req, callProperties.CallOptions...)
}

// Update a scope
func (c ScopesClient) Update(ctx context.Context, req *UpdateScopeRequest) (*scopes.UpdateScopeResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := scopes.NewScopesServiceClient(conn)

	return client.UpdateScope(callProperties.Ctx, req, callProperties.CallOptions...)
}

// Delete a scope
func (c ScopesClient) Delete(ctx context.Context, req *DeleteScopeRequest) (*scopes.DeleteScopeResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := scopes.NewScopesServiceClient(conn)

	return client.DeleteScope(callProperties.Ctx, req, callProperties.CallOptions...)
}

// Create a new ScopesClient
func NewScopesClient(c *CallPropertiesCreator) *ScopesClient {
	return &ScopesClient{callPropertiesCreator: c}
}
