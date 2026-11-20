package mask_things_service

import (
	"context"
	"net/http"
	"time"
)

type MaskThingStringField string

const (
	MASKTHINGSTRINGFIELD_MASK_THING_STRING_FIELD_UNSPECIFIED MaskThingStringField = "MASK_THING_STRING_FIELD_UNSPECIFIED"
	MASKTHINGSTRINGFIELD_MASK_THING_STRING_FIELD_PLATFORM    MaskThingStringField = "MASK_THING_STRING_FIELD_PLATFORM"
	MASKTHINGSTRINGFIELD_MASK_THING_STRING_FIELD_COUNTRY     MaskThingStringField = "MASK_THING_STRING_FIELD_COUNTRY"
)

type MaskThingStringFilter struct {
	Field MaskThingStringField
	Value string
}

type MaskThingBoolFilter struct {
	Value *bool
}

// A oneOf: one arm is set.
type MaskThingFilter struct {
	StringFilter *MaskThingStringFilter
	BoolFilter   *MaskThingBoolFilter
}

type MaskThingFixed struct {
	Start string
	End   string
}

type MaskThingRelative struct {
	Minutes int32
}

type MaskThingRange struct {
	Fixed    *MaskThingFixed
	Relative *MaskThingRelative
}

type MaskThingLimits struct {
	Max     *int32
	Min     *int32
	Applied *int32
}

type MaskThingLimitsInput struct {
	Max *int32
	Min *int32
}

type MaskThing struct {
	Id          string
	CreateTime  time.Time
	Owner       *string
	Name        string
	Description *string
	Range       MaskThingRange
	Limits      *MaskThingLimits
	Filters     []MaskThingFilter
}

type CreateMaskThingRequest struct {
	Name        string
	Description *string
	Range       MaskThingRange
	Limits      *MaskThingLimitsInput
	Filters     []MaskThingFilter
}

type UpdateMaskThingRequest struct {
	Name        *string
	Description *string
	Range       *MaskThingRange
	Limits      *MaskThingLimitsInput
	Filters     []MaskThingFilter
}

type MaskThingsServiceAPIService struct{}

type ApiMaskThingsServiceCreateMaskThingRequest struct{}
type ApiMaskThingsServiceGetMaskThingRequest struct{}
type ApiMaskThingsServiceUpdateMaskThingRequest struct{}
type ApiMaskThingsServiceDeleteMaskThingRequest struct{}

func (*MaskThingsServiceAPIService) MaskThingsServiceCreateMaskThing(ctx context.Context) ApiMaskThingsServiceCreateMaskThingRequest {
	_ = ctx
	return ApiMaskThingsServiceCreateMaskThingRequest{}
}

func (*MaskThingsServiceAPIService) MaskThingsServiceGetMaskThing(ctx context.Context, id string) ApiMaskThingsServiceGetMaskThingRequest {
	_, _ = ctx, id
	return ApiMaskThingsServiceGetMaskThingRequest{}
}

func (*MaskThingsServiceAPIService) MaskThingsServiceUpdateMaskThing(ctx context.Context, id string) ApiMaskThingsServiceUpdateMaskThingRequest {
	_, _ = ctx, id
	return ApiMaskThingsServiceUpdateMaskThingRequest{}
}

func (*MaskThingsServiceAPIService) MaskThingsServiceDeleteMaskThing(ctx context.Context, id string) ApiMaskThingsServiceDeleteMaskThingRequest {
	_, _ = ctx, id
	return ApiMaskThingsServiceDeleteMaskThingRequest{}
}

func (r ApiMaskThingsServiceCreateMaskThingRequest) CreateMaskThingRequest(createMaskThingRequest CreateMaskThingRequest) ApiMaskThingsServiceCreateMaskThingRequest {
	_ = createMaskThingRequest
	return r
}

func (r ApiMaskThingsServiceUpdateMaskThingRequest) UpdateMaskThingRequest(updateMaskThingRequest UpdateMaskThingRequest) ApiMaskThingsServiceUpdateMaskThingRequest {
	_ = updateMaskThingRequest
	return r
}

func (r ApiMaskThingsServiceUpdateMaskThingRequest) UpdateMask(updateMask string) ApiMaskThingsServiceUpdateMaskThingRequest {
	_ = updateMask
	return r
}

func (ApiMaskThingsServiceCreateMaskThingRequest) Execute() (*MaskThing, *http.Response, error) {
	return nil, nil, nil
}

func (ApiMaskThingsServiceGetMaskThingRequest) Execute() (*MaskThing, *http.Response, error) {
	return nil, nil, nil
}

func (ApiMaskThingsServiceUpdateMaskThingRequest) Execute() (*MaskThing, *http.Response, error) {
	return nil, nil, nil
}

func (ApiMaskThingsServiceDeleteMaskThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}
