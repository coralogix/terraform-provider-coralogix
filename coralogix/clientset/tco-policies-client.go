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

	return client.CreatePolicy(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (t TCOPoliciesClient) GetTCOPolicy(ctx context.Context, req *tcopolicies.GetPolicyRequest) (*tcopolicies.GetPolicyResponse, error) {
	callProperties, err := t.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := tcopolicies.NewPoliciesServiceClient(conn)

	return client.GetPolicy(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (t TCOPoliciesClient) UpdateTCOPolicy(ctx context.Context, req *tcopolicies.UpdatePolicyRequest) (*tcopolicies.UpdatePolicyResponse, error) {
	callProperties, err := t.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := tcopolicies.NewPoliciesServiceClient(conn)

	return client.UpdatePolicy(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (t TCOPoliciesClient) DeleteTCOPolicy(ctx context.Context, req *tcopolicies.DeletePolicyRequest) (*tcopolicies.DeletePolicyResponse, error) {
	callProperties, err := t.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := tcopolicies.NewPoliciesServiceClient(conn)

	return client.DeletePolicy(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (t TCOPoliciesClient) GetTCOPolicies(ctx context.Context, req *tcopolicies.GetCompanyPoliciesRequest) (*tcopolicies.GetCompanyPoliciesResponse, error) {
	callProperties, err := t.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := tcopolicies.NewPoliciesServiceClient(conn)

	return client.GetCompanyPolicies(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (t TCOPoliciesClient) ReorderTCOPolicies(ctx context.Context, req *tcopolicies.ReorderPoliciesRequest) (*tcopolicies.ReorderPoliciesResponse, error) {
	callProperties, err := t.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := tcopolicies.NewPoliciesServiceClient(conn)

	return client.ReorderPolicies(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewTCOPoliciesClient(c *CallPropertiesCreator) *TCOPoliciesClient {
	return &TCOPoliciesClient{callPropertiesCreator: c}
}
