package rename_things_service

import (
	"context"
	"net/http"
)

type RenameParam struct {
	Name  *string
	Value *string
}

type RenameHolder struct {
	Items []RenameParam
}

// RenameSpec is what the Create and Update requests hold in their wrapper. It is not the
// resource: the kind and the version have other names, and the list is one object deeper.
type RenameSpec struct {
	SpecKind    *string
	SpecVersion *string
	Holder      *RenameHolder
}

type RenameThing struct {
	Id         *string
	Kind       *string
	Version    *string
	Parameters []RenameParam
}

type SaveRenameThingRequest struct {
	Spec *RenameSpec
}

// The Update request has the id beside the wrapper.
type UpdateRenameThingRequest struct {
	Id   *string
	Spec *RenameSpec
}

type SaveRenameThingResponse struct {
	ThingId *string
}

type GetRenameThingResponse struct {
	Thing *RenameThing
}

type RenameThingsServiceAPIService struct{}

type ApiRenameThingsServiceSaveRenameThingRequest struct {
	body SaveRenameThingRequest
}
type ApiRenameThingsServiceGetRenameThingRequest struct {
	id string
}
type ApiRenameThingsServiceUpdateRenameThingRequest struct {
	body UpdateRenameThingRequest
}
type ApiRenameThingsServiceDeleteRenameThingRequest struct{}

// Saved and Updated list the bodies that the calls received. Create returns SavedID. Get returns
// Stored, and GetIDs lists the ids that it was asked for.
var (
	Saved   []SaveRenameThingRequest
	Updated []UpdateRenameThingRequest
	SavedID = "thing-1"
	Stored  RenameThing
	GetIDs  []string
)

func (*RenameThingsServiceAPIService) RenameThingsServiceSaveRenameThing(ctx context.Context) ApiRenameThingsServiceSaveRenameThingRequest {
	_ = ctx
	return ApiRenameThingsServiceSaveRenameThingRequest{}
}

func (*RenameThingsServiceAPIService) RenameThingsServiceGetRenameThing(ctx context.Context, thingId string) ApiRenameThingsServiceGetRenameThingRequest {
	_ = ctx
	return ApiRenameThingsServiceGetRenameThingRequest{id: thingId}
}

func (*RenameThingsServiceAPIService) RenameThingsServiceUpdateRenameThing(ctx context.Context) ApiRenameThingsServiceUpdateRenameThingRequest {
	_ = ctx
	return ApiRenameThingsServiceUpdateRenameThingRequest{}
}

func (*RenameThingsServiceAPIService) RenameThingsServiceDeleteRenameThing(ctx context.Context, thingId string) ApiRenameThingsServiceDeleteRenameThingRequest {
	_, _ = ctx, thingId
	return ApiRenameThingsServiceDeleteRenameThingRequest{}
}

func (r ApiRenameThingsServiceSaveRenameThingRequest) SaveRenameThingRequest(saveRenameThingRequest SaveRenameThingRequest) ApiRenameThingsServiceSaveRenameThingRequest {
	r.body = saveRenameThingRequest
	return r
}

func (r ApiRenameThingsServiceUpdateRenameThingRequest) UpdateRenameThingRequest(updateRenameThingRequest UpdateRenameThingRequest) ApiRenameThingsServiceUpdateRenameThingRequest {
	r.body = updateRenameThingRequest
	return r
}

func (r ApiRenameThingsServiceSaveRenameThingRequest) Execute() (*SaveRenameThingResponse, *http.Response, error) {
	Saved = append(Saved, r.body)
	id := SavedID
	return &SaveRenameThingResponse{ThingId: &id}, nil, nil
}

func (r ApiRenameThingsServiceGetRenameThingRequest) Execute() (*GetRenameThingResponse, *http.Response, error) {
	GetIDs = append(GetIDs, r.id)
	stored := Stored
	return &GetRenameThingResponse{Thing: &stored}, nil, nil
}

// Update returns an empty object, as the SDK writes for a response with no fields.
func (r ApiRenameThingsServiceUpdateRenameThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	Updated = append(Updated, r.body)
	return map[string]interface{}{}, nil, nil
}

func (ApiRenameThingsServiceDeleteRenameThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}
