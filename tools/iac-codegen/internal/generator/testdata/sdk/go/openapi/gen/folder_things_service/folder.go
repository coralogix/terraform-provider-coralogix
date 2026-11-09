package folder_things_service

import (
	"context"
	"errors"
	"net/http"
)

type FolderThing struct {
	Id       *string
	Name     *string
	ParentId *string
}

// The Create and Replace requests wrap the resource and have an idempotency key that the
// resource does not send.
type CreateFolderThingBody struct {
	RequestId *string
	Thing     *FolderThing
}

type ReplaceFolderThingBody struct {
	RequestId *string
	Thing     *FolderThing
}

// The Create response holds only the id.
type CreateFolderThingResponse struct {
	ThingId *string
}

// The Get response holds a revision next to the thing. The server returns it only for a request
// that asks for it with IncludeRevision.
type GetFolderThingResponse struct {
	Thing    *FolderThing
	Revision *string
}

type FolderThingsServiceAPIService struct{}

type ApiFolderThingsServiceCreateFolderThingRequest struct {
	body CreateFolderThingBody
}
type ApiFolderThingsServiceGetFolderThingRequest struct {
	id      string
	include bool
}
type ApiFolderThingsServiceReplaceFolderThingRequest struct {
	body ReplaceFolderThingBody
}
type ApiFolderThingsServiceDeleteFolderThingRequest struct{}

// Created and Replaced list the bodies that the calls received. Create returns CreatedID. Get
// returns Stored, and GetIDs lists the ids that it was asked for.
var (
	Created   []CreateFolderThingBody
	Replaced  []ReplaceFolderThingBody
	CreatedID = "folder-1"
	Stored    FolderThing
	GetIDs    []string
	// StoredRevision is the revision that Get returns for a request that includes it. GetIncludes
	// lists whether each Get asked for it. With RejectInclude, a Get that asks for it fails with
	// a 403 and the error GetForbidden.
	StoredRevision string
	GetIncludes    []bool
	RejectInclude  bool
	GetForbidden   = errors.New("forbidden")
	// GetErr makes Get fail, and DeleteErr makes Delete fail.
	GetErr, DeleteErr, ReplaceErr error
)

// The path parameter is thing_id in the contract, which openapi-generator writes as thingId.
func (*FolderThingsServiceAPIService) FolderThingsServiceCreateFolderThing(ctx context.Context) ApiFolderThingsServiceCreateFolderThingRequest {
	_ = ctx
	return ApiFolderThingsServiceCreateFolderThingRequest{}
}

func (*FolderThingsServiceAPIService) FolderThingsServiceGetFolderThing(ctx context.Context, thingId string) ApiFolderThingsServiceGetFolderThingRequest {
	_ = ctx
	return ApiFolderThingsServiceGetFolderThingRequest{id: thingId}
}

func (*FolderThingsServiceAPIService) FolderThingsServiceReplaceFolderThing(ctx context.Context) ApiFolderThingsServiceReplaceFolderThingRequest {
	_ = ctx
	return ApiFolderThingsServiceReplaceFolderThingRequest{}
}

func (*FolderThingsServiceAPIService) FolderThingsServiceDeleteFolderThing(ctx context.Context, thingId string) ApiFolderThingsServiceDeleteFolderThingRequest {
	_, _ = ctx, thingId
	return ApiFolderThingsServiceDeleteFolderThingRequest{}
}

// IncludeRevision asks the server for the revision.
func (r ApiFolderThingsServiceGetFolderThingRequest) IncludeRevision(includeRevision bool) ApiFolderThingsServiceGetFolderThingRequest {
	r.include = includeRevision
	return r
}

func (r ApiFolderThingsServiceCreateFolderThingRequest) CreateFolderThingBody(createFolderThingBody CreateFolderThingBody) ApiFolderThingsServiceCreateFolderThingRequest {
	r.body = createFolderThingBody
	return r
}

func (r ApiFolderThingsServiceReplaceFolderThingRequest) ReplaceFolderThingBody(replaceFolderThingBody ReplaceFolderThingBody) ApiFolderThingsServiceReplaceFolderThingRequest {
	r.body = replaceFolderThingBody
	return r
}

func (r ApiFolderThingsServiceCreateFolderThingRequest) Execute() (*CreateFolderThingResponse, *http.Response, error) {
	Created = append(Created, r.body)
	id := CreatedID
	return &CreateFolderThingResponse{ThingId: &id}, nil, nil
}

func (r ApiFolderThingsServiceGetFolderThingRequest) Execute() (*GetFolderThingResponse, *http.Response, error) {
	GetIDs = append(GetIDs, r.id)
	GetIncludes = append(GetIncludes, r.include)
	if GetErr != nil {
		return nil, nil, GetErr
	}
	stored := Stored
	out := &GetFolderThingResponse{Thing: &stored}
	if r.include {
		if RejectInclude {
			return nil, &http.Response{StatusCode: http.StatusForbidden}, GetForbidden
		}
		revision := StoredRevision
		out.Revision = &revision
	}
	return out, nil, nil
}

// Replace returns an empty object, as the SDK writes for a response with no fields.
func (r ApiFolderThingsServiceReplaceFolderThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	Replaced = append(Replaced, r.body)
	return map[string]interface{}{}, nil, ReplaceErr
}

func (ApiFolderThingsServiceDeleteFolderThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, DeleteErr
}
