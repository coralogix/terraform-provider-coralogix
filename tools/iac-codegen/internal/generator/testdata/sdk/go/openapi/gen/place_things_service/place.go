package place_things_service

import (
	"context"
	"net/http"
)

type PlaceThing struct {
	Id     *string
	Name   *string
	Family *PlaceFamily
	Direct *PlaceSelectorResponse
}

type PlaceFamily struct {
	Raw    *PlaceRaw
	Preset *PlacePreset
}

type PlaceFamilyCreate struct {
	Raw    *PlaceRawCreate
	Preset *PlacePresetCreate
}

type PlaceRaw struct {
	Items []PlaceItem
}

type PlaceRawCreate struct {
	Items []PlaceItemCreate
}

type PlacePreset struct {
	Chart *string
	Items []PlaceItem
}

type PlacePresetCreate struct {
	Chart *string
}

type PlaceItem struct {
	Selector *PlaceSelectorResponse
}

type PlaceItemCreate struct {
	Selector *PlaceSelectorRequest
}

type PlaceSelectorRequest struct {
	Match *string
}

type PlaceSelectorResponse struct {
	Match *string
	Stamp *PlaceStamp
}

type PlaceStamp struct {
	By *string
}

type CreatePlaceThingRequest struct {
	Name   *string
	Family *PlaceFamilyCreate
	Direct *PlaceSelectorResponse
}

type ReplacePlaceThingRequest struct {
	Name   *string
	Family *PlaceFamilyCreate
	Direct *PlaceSelectorResponse
}

type PlaceThingsServiceAPIService struct{}

type ApiPlaceThingsServiceCreatePlaceThingRequest struct{}
type ApiPlaceThingsServiceGetPlaceThingRequest struct{}
type ApiPlaceThingsServiceReplacePlaceThingRequest struct{}
type ApiPlaceThingsServiceDeletePlaceThingRequest struct{}

func (*PlaceThingsServiceAPIService) PlaceThingsServiceCreatePlaceThing(ctx context.Context) ApiPlaceThingsServiceCreatePlaceThingRequest {
	_ = ctx
	return ApiPlaceThingsServiceCreatePlaceThingRequest{}
}

func (*PlaceThingsServiceAPIService) PlaceThingsServiceGetPlaceThing(ctx context.Context, id string) ApiPlaceThingsServiceGetPlaceThingRequest {
	_, _ = ctx, id
	return ApiPlaceThingsServiceGetPlaceThingRequest{}
}

func (*PlaceThingsServiceAPIService) PlaceThingsServiceReplacePlaceThing(ctx context.Context, id string) ApiPlaceThingsServiceReplacePlaceThingRequest {
	_, _ = ctx, id
	return ApiPlaceThingsServiceReplacePlaceThingRequest{}
}

func (*PlaceThingsServiceAPIService) PlaceThingsServiceDeletePlaceThing(ctx context.Context, id string) ApiPlaceThingsServiceDeletePlaceThingRequest {
	_, _ = ctx, id
	return ApiPlaceThingsServiceDeletePlaceThingRequest{}
}

func (r ApiPlaceThingsServiceCreatePlaceThingRequest) CreatePlaceThingRequest(createPlaceThingRequest CreatePlaceThingRequest) ApiPlaceThingsServiceCreatePlaceThingRequest {
	_ = createPlaceThingRequest
	return r
}

func (r ApiPlaceThingsServiceReplacePlaceThingRequest) ReplacePlaceThingRequest(replacePlaceThingRequest ReplacePlaceThingRequest) ApiPlaceThingsServiceReplacePlaceThingRequest {
	_ = replacePlaceThingRequest
	return r
}

func (ApiPlaceThingsServiceCreatePlaceThingRequest) Execute() (*PlaceThing, *http.Response, error) {
	return nil, nil, nil
}

func (ApiPlaceThingsServiceGetPlaceThingRequest) Execute() (*PlaceThing, *http.Response, error) {
	return nil, nil, nil
}

func (ApiPlaceThingsServiceReplacePlaceThingRequest) Execute() (*PlaceThing, *http.Response, error) {
	return nil, nil, nil
}

func (ApiPlaceThingsServiceDeletePlaceThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}
