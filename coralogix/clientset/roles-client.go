package clientset

import (
	"context"
	roles "terraform-provider-coralogix/coralogix/clientset/grpc/roles"
)

type RolesClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (t RolesClient) CreateRole(ctx context.Context, req *roles.CreateRoleRequest) (*roles.CreateRoleResponse, error) {
	callProperties, err := t.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := roles.NewRoleManagementServiceClient(conn)

	return client.CreateRole(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (t RolesClient) UpdateRole(ctx context.Context, req *roles.UpdateRoleRequest) (*roles.UpdateRoleResponse, error) {
	callProperties, err := t.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}
	conn := callProperties.Connection
	defer conn.Close()

	client := roles.NewRoleManagementServiceClient(conn)

	return client.UpdateRole(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (t RolesClient) DeleteRole(ctx context.Context, req *roles.DeleteRoleRequest) (*roles.DeleteRoleResponse, error) {
	callProperties, err := t.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}
	conn := callProperties.Connection
	defer conn.Close()

	client := roles.NewRoleManagementServiceClient(conn)

	return client.DeleteRole(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (t RolesClient) GetRole(ctx context.Context, req *roles.GetCustomRoleRequest) (*roles.GetCustomRoleResponse, error) {
	callProperties, err := t.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := roles.NewRoleManagementServiceClient(conn)

	return client.GetCustomRole(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (t RolesClient) ListCustomRole(ctx context.Context, req *roles.ListCustomRolesRequest) (*roles.ListCustomRolesResponse, error) {
	callProperties, err := t.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := roles.NewRoleManagementServiceClient(conn)

	return client.ListCustomRoles(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewRolesClient(c *CallPropertiesCreator) *RolesClient {
	return &RolesClient{callPropertiesCreator: c}
}
