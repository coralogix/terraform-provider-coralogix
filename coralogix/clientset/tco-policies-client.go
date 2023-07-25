package clientset

import (
	"context"

	tcopolicies "terraform-provider-coralogix/coralogix/clientset/grpc/tco-policies"
)

type TCOPoliciesClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (t TCOPoliciesClient) CreateTCOPolicy(ctx context.Context, req *tcopolicies.CreatePolicyRequest) (*tcopolicies.CreatePolicyResponse, error) {
	callProperties, err := t.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := tcopolicies.NewPoliciesServiceClient(conn)

	return client.CreatePolicy(ctx, req, callProperties.CallOptions...)
}

func (t TCOPoliciesClient) GetTCOPolicy(ctx context.Context, req *tcopolicies.GetPolicyRequest) (*tcopolicies.GetPolicyResponse, error) {
	callProperties, err := t.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := tcopolicies.NewPoliciesServiceClient(conn)

	return client.GetPolicy(ctx, req, callProperties.CallOptions...)
}

func (t TCOPoliciesClient) UpdateTCOPolicy(ctx context.Context, req *tcopolicies.UpdatePolicyRequest) (*tcopolicies.UpdatePolicyResponse, error) {
	callProperties, err := t.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := tcopolicies.NewPoliciesServiceClient(conn)

	return client.UpdatePolicy(ctx, req, callProperties.CallOptions...)
}

func (t TCOPoliciesClient) DeleteTCOPolicy(ctx context.Context, req *tcopolicies.DeletePolicyRequest) (*tcopolicies.DeletePolicyResponse, error) {
	callProperties, err := t.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := tcopolicies.NewPoliciesServiceClient(conn)

	return client.DeletePolicy(ctx, req, callProperties.CallOptions...)
}

func (t TCOPoliciesClient) GetTCOPolicies(ctx context.Context, req *tcopolicies.GetCompanyPoliciesRequest) (*tcopolicies.GetCompanyPoliciesResponse, error) {
	callProperties, err := t.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := tcopolicies.NewPoliciesServiceClient(conn)

	return client.GetCompanyPolicies(ctx, req, callProperties.CallOptions...)
}

func (t TCOPoliciesClient) ReorderTCOPolicies(ctx context.Context, req *tcopolicies.ReorderPoliciesRequest) (*tcopolicies.ReorderPoliciesResponse, error) {
	callProperties, err := t.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := tcopolicies.NewPoliciesServiceClient(conn)

	return client.ReorderPolicies(ctx, req, callProperties.CallOptions...)
}

func NewTCOPoliciesClient(c *CallPropertiesCreator) *TCOPoliciesClient {
	return &TCOPoliciesClient{callPropertiesCreator: c}
}
