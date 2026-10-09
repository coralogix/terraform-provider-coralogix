package custom_things_service

import (
	"context"
	"net/http"
)

type ThingKind string

const (
	THINGKIND_THING_KIND_UNSPECIFIED ThingKind = "THING_KIND_UNSPECIFIED"
	THINGKIND_SYSTEM                 ThingKind = "SYSTEM"
	THINGKIND_CUSTOM                 ThingKind = "CUSTOM"
	THINGKIND_LEGACY                 ThingKind = "LEGACY"
)

type AttachmentPolicy string

const (
	ATTACHMENTPOLICY_AUTO     AttachmentPolicy = "AUTO"
	ATTACHMENTPOLICY_ENABLED  AttachmentPolicy = "ENABLED"
	ATTACHMENTPOLICY_DISABLED AttachmentPolicy = "DISABLED"
)

type Note struct {
	Text *string
}

type Attachment struct {
	Policy *AttachmentPolicy
}

type MatchAll struct {
	Marker *string
}

type MatchName struct {
	Name *string
}

type Condition struct {
	MatchAll  *MatchAll
	MatchName *MatchName
}

type CustomThing struct {
	Id         *string
	Name       *string
	Kind       *ThingKind
	Origin     *ThingKind
	Notes      []Note
	Attachment *Attachment
	Condition  *Condition
}

type CreateCustomThingRequest struct {
	Thing *CustomThing
}

type ReplaceCustomThingRequest struct {
	Thing *CustomThing
}

type CreateCustomThingResponse struct {
	Thing *CustomThing
}

type GetCustomThingResponse struct {
	Thing *CustomThing
}

type ReplaceCustomThingResponse struct {
	Thing *CustomThing
}

type CustomThingsServiceAPIService struct{}

type ApiCustomThingsServiceCreateCustomThingRequest struct{}
type ApiCustomThingsServiceGetThingRequest struct{}
type ApiCustomThingsServiceReplaceCustomThingRequest struct{}
type ApiCustomThingsServiceDeleteCustomThingRequest struct{}

func (*CustomThingsServiceAPIService) CustomThingsServiceCreateCustomThing(ctx context.Context) ApiCustomThingsServiceCreateCustomThingRequest {
	_ = ctx
	return ApiCustomThingsServiceCreateCustomThingRequest{}
}

func (*CustomThingsServiceAPIService) CustomThingsServiceGetThing(ctx context.Context, id string) ApiCustomThingsServiceGetThingRequest {
	_, _ = ctx, id
	return ApiCustomThingsServiceGetThingRequest{}
}

func (*CustomThingsServiceAPIService) CustomThingsServiceReplaceCustomThing(ctx context.Context) ApiCustomThingsServiceReplaceCustomThingRequest {
	_ = ctx
	return ApiCustomThingsServiceReplaceCustomThingRequest{}
}

func (*CustomThingsServiceAPIService) CustomThingsServiceDeleteCustomThing(ctx context.Context, id string) ApiCustomThingsServiceDeleteCustomThingRequest {
	_, _ = ctx, id
	return ApiCustomThingsServiceDeleteCustomThingRequest{}
}

func (r ApiCustomThingsServiceCreateCustomThingRequest) CreateCustomThingRequest(createCustomThingRequest CreateCustomThingRequest) ApiCustomThingsServiceCreateCustomThingRequest {
	_ = createCustomThingRequest
	return r
}

func (r ApiCustomThingsServiceReplaceCustomThingRequest) ReplaceCustomThingRequest(replaceCustomThingRequest ReplaceCustomThingRequest) ApiCustomThingsServiceReplaceCustomThingRequest {
	_ = replaceCustomThingRequest
	return r
}

func (ApiCustomThingsServiceCreateCustomThingRequest) Execute() (*CreateCustomThingResponse, *http.Response, error) {
	return nil, nil, nil
}

func (ApiCustomThingsServiceGetThingRequest) Execute() (*GetCustomThingResponse, *http.Response, error) {
	return nil, nil, nil
}

func (ApiCustomThingsServiceReplaceCustomThingRequest) Execute() (*ReplaceCustomThingResponse, *http.Response, error) {
	return nil, nil, nil
}

func (ApiCustomThingsServiceDeleteCustomThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}
