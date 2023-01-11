package clientset

import (
	"context"

	actionsv2 "terraform-provider-coralogix/coralogix/clientset/grpc/com/coralogix/actions/v2"
)

type ActionsClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (a ActionsClient) CreateAction(ctx context.Context, req *actionsv2.CreateActionRequest) (*actionsv2.CreateActionResponse, error) {
	callProperties, err := a.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := actionsv2.NewActionsServiceClient(conn)

	return client.CreateAction(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (a ActionsClient) GetAction(ctx context.Context, req *actionsv2.GetActionRequest) (*actionsv2.GetActionResponse, error) {
	callProperties, err := a.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := actionsv2.NewActionsServiceClient(conn)

	return client.GetAction(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (a ActionsClient) UpdateAction(ctx context.Context, req *actionsv2.ReplaceActionRequest) (*actionsv2.ReplaceActionResponse, error) {
	callProperties, err := a.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := actionsv2.NewActionsServiceClient(conn)

	return client.ReplaceAction(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (a ActionsClient) DeleteAction(ctx context.Context, req *actionsv2.DeleteActionRequest) (*actionsv2.DeleteActionResponse, error) {
	callProperties, err := a.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := actionsv2.NewActionsServiceClient(conn)

	return client.DeleteAction(callProperties.Ctx, req, callProperties.CallOptions...)
}

func NewActionsClient(c *CallPropertiesCreator) *ActionsClient {
	return &ActionsClient{callPropertiesCreator: c}
}
