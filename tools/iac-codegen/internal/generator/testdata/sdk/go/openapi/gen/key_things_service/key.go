package key_things_service

import (
	"context"
	"net/http"
)

type CreateKeyThingRequestKeyPermissions struct {
	Presets     []string
	Permissions []string
}

// A required field is a value, as openapi-generator writes it.
type CreateKeyThingRequest struct {
	Name           string
	Hashed         *bool
	KeyPermissions CreateKeyThingRequestKeyPermissions
}

type CreateKeyThingResponse struct {
	KeyId string
}

// PresetThing is how the response holds a preset: an object with its name and its permissions.
type PresetThing struct {
	Name        string
	Permissions []string
}

type KeyThingKeyPermissions struct {
	Presets     []PresetThing
	Permissions []string
}

type KeyThing struct {
	Id             string
	Name           string
	Enabled        bool
	Hashed         bool
	Value          *string
	KeyPermissions KeyThingKeyPermissions
}

type GetKeyThingResponse struct {
	KeyInfo KeyThing
}

// The Update request holds each list in a one-property object, at the top level.
type KeyThingPresets struct {
	Presets []string
}

type KeyThingPermissions struct {
	Permissions []string
}

type UpdateKeyThingRequest struct {
	NewName     *string
	IsEnabled   *bool
	Presets     *KeyThingPresets
	Permissions *KeyThingPermissions
}

type KeyThingsServiceAPIService struct{}

type ApiKeyThingsServiceCreateKeyThingRequest struct {
	body CreateKeyThingRequest
}
type ApiKeyThingsServiceGetKeyThingRequest struct {
	id string
}
type ApiKeyThingsServiceUpdateKeyThingRequest struct {
	id   string
	body UpdateKeyThingRequest
}
type ApiKeyThingsServiceDeleteKeyThingRequest struct{}

// Created and Updated list the bodies that the calls received. Create returns CreatedID, and Get
// returns Stored.
var (
	Created   []CreateKeyThingRequest
	Updated   []UpdateKeyThingRequest
	UpdatedID []string
	CreatedID = "key-1"
	Stored    KeyThing
)

func (*KeyThingsServiceAPIService) KeyThingsServiceCreateKeyThing(ctx context.Context) ApiKeyThingsServiceCreateKeyThingRequest {
	_ = ctx
	return ApiKeyThingsServiceCreateKeyThingRequest{}
}

func (*KeyThingsServiceAPIService) KeyThingsServiceGetKeyThing(ctx context.Context, keyId string) ApiKeyThingsServiceGetKeyThingRequest {
	_ = ctx
	return ApiKeyThingsServiceGetKeyThingRequest{id: keyId}
}

func (*KeyThingsServiceAPIService) KeyThingsServiceUpdateKeyThing(ctx context.Context, keyId string) ApiKeyThingsServiceUpdateKeyThingRequest {
	_ = ctx
	return ApiKeyThingsServiceUpdateKeyThingRequest{id: keyId}
}

func (*KeyThingsServiceAPIService) KeyThingsServiceDeleteKeyThing(ctx context.Context, keyId string) ApiKeyThingsServiceDeleteKeyThingRequest {
	_, _ = ctx, keyId
	return ApiKeyThingsServiceDeleteKeyThingRequest{}
}

func (r ApiKeyThingsServiceCreateKeyThingRequest) CreateKeyThingRequest(createKeyThingRequest CreateKeyThingRequest) ApiKeyThingsServiceCreateKeyThingRequest {
	r.body = createKeyThingRequest
	return r
}

func (r ApiKeyThingsServiceUpdateKeyThingRequest) UpdateKeyThingRequest(updateKeyThingRequest UpdateKeyThingRequest) ApiKeyThingsServiceUpdateKeyThingRequest {
	r.body = updateKeyThingRequest
	return r
}

func (r ApiKeyThingsServiceCreateKeyThingRequest) Execute() (*CreateKeyThingResponse, *http.Response, error) {
	Created = append(Created, r.body)
	id := CreatedID
	return &CreateKeyThingResponse{KeyId: id}, nil, nil
}

func (r ApiKeyThingsServiceGetKeyThingRequest) Execute() (*GetKeyThingResponse, *http.Response, error) {
	stored := Stored
	return &GetKeyThingResponse{KeyInfo: stored}, nil, nil
}

// Update returns an empty object, as the SDK writes for a response with no fields.
func (r ApiKeyThingsServiceUpdateKeyThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	Updated = append(Updated, r.body)
	UpdatedID = append(UpdatedID, r.id)
	return map[string]interface{}{}, nil, nil
}

func (ApiKeyThingsServiceDeleteKeyThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}
