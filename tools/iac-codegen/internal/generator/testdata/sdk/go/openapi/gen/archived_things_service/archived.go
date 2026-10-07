package archived_things_service

import (
	"context"
	"net/http"
)

type ArchivedThing struct {
	Id   *string
	Name *string
}

type CreateArchivedThingRequest struct {
	Name *string
}

type ReplaceArchivedThingRequest struct {
	Name *string
}

// ArchiveError is the error of the archive call, and Archived lists the ids that it received.
// Tests of the generated Delete set and read them.
var (
	ArchiveError error
	Archived     []string
)

type ArchivedThingsServiceAPIService struct{}

type ApiArchivedThingsServiceCreateArchivedThingRequest struct{}
type ApiArchivedThingsServiceGetArchivedThingRequest struct{}
type ApiArchivedThingsServiceReplaceArchivedThingRequest struct{}
type ApiArchivedThingsServiceArchiveArchivedThingRequest struct{ id string }

func (*ArchivedThingsServiceAPIService) ArchivedThingsServiceCreateArchivedThing(ctx context.Context) ApiArchivedThingsServiceCreateArchivedThingRequest {
	_ = ctx
	return ApiArchivedThingsServiceCreateArchivedThingRequest{}
}

func (*ArchivedThingsServiceAPIService) ArchivedThingsServiceGetArchivedThing(ctx context.Context, id string) ApiArchivedThingsServiceGetArchivedThingRequest {
	_, _ = ctx, id
	return ApiArchivedThingsServiceGetArchivedThingRequest{}
}

func (*ArchivedThingsServiceAPIService) ArchivedThingsServiceReplaceArchivedThing(ctx context.Context, id string) ApiArchivedThingsServiceReplaceArchivedThingRequest {
	_, _ = ctx, id
	return ApiArchivedThingsServiceReplaceArchivedThingRequest{}
}

func (*ArchivedThingsServiceAPIService) ArchivedThingsServiceArchiveArchivedThing(ctx context.Context, id string) ApiArchivedThingsServiceArchiveArchivedThingRequest {
	_ = ctx
	return ApiArchivedThingsServiceArchiveArchivedThingRequest{id: id}
}

func (r ApiArchivedThingsServiceCreateArchivedThingRequest) CreateArchivedThingRequest(createArchivedThingRequest CreateArchivedThingRequest) ApiArchivedThingsServiceCreateArchivedThingRequest {
	_ = createArchivedThingRequest
	return r
}

func (r ApiArchivedThingsServiceReplaceArchivedThingRequest) ReplaceArchivedThingRequest(replaceArchivedThingRequest ReplaceArchivedThingRequest) ApiArchivedThingsServiceReplaceArchivedThingRequest {
	_ = replaceArchivedThingRequest
	return r
}

func (ApiArchivedThingsServiceCreateArchivedThingRequest) Execute() (*ArchivedThing, *http.Response, error) {
	return nil, nil, nil
}

func (ApiArchivedThingsServiceGetArchivedThingRequest) Execute() (*ArchivedThing, *http.Response, error) {
	return nil, nil, nil
}

func (ApiArchivedThingsServiceReplaceArchivedThingRequest) Execute() (*ArchivedThing, *http.Response, error) {
	return nil, nil, nil
}

func (r ApiArchivedThingsServiceArchiveArchivedThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	Archived = append(Archived, r.id)
	return nil, nil, ArchiveError
}
