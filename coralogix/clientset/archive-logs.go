package clientset

import (
	"context"

	archiveLogs "terraform-provider-coralogix/coralogix/clientset/grpc/archive-logs"
)

type ArchiveLogsClient struct {
	callPropertiesCreator *CallPropertiesCreator
}

func (c ArchiveLogsClient) UpdateArchiveLogs(ctx context.Context, req *archiveLogs.SetTargetRequest) (*archiveLogs.SetTargetResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := archiveLogs.NewTargetServiceClient(conn)

	return client.SetTarget(callProperties.Ctx, req, callProperties.CallOptions...)
}

func (c ArchiveLogsClient) GetArchiveLogs(ctx context.Context) (*archiveLogs.GetTargetResponse, error) {
	callProperties, err := c.callPropertiesCreator.GetCallProperties(ctx)
	if err != nil {
		return nil, err
	}

	conn := callProperties.Connection
	defer conn.Close()
	client := archiveLogs.NewTargetServiceClient(conn)

	return client.GetTarget(callProperties.Ctx, &archiveLogs.GetTargetRequest{}, callProperties.CallOptions...)
}

func NewArchiveLogsClient(c *CallPropertiesCreator) *ArchiveLogsClient {
	return &ArchiveLogsClient{callPropertiesCreator: c}
}
