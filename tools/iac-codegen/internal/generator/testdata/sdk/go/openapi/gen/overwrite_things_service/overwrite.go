package overwrite_things_service

import (
	"context"
	"net/http"
	"strings"
)

type ThingKind string

const (
	THINGKIND_THING_KIND_UNSPECIFIED ThingKind = "THING_KIND_UNSPECIFIED"
	THINGKIND_THING_KIND_A           ThingKind = "THING_KIND_A"
	THINGKIND_THING_KIND_B           ThingKind = "THING_KIND_B"
)

type ThingPriority string

const (
	THINGPRIORITY_THING_PRIORITY_UNSPECIFIED ThingPriority = "THING_PRIORITY_UNSPECIFIED"
	THINGPRIORITY_THING_PRIORITY_LOW         ThingPriority = "THING_PRIORITY_LOW"
	THINGPRIORITY_THING_PRIORITY_HIGH        ThingPriority = "THING_PRIORITY_HIGH"
)

type ThingList struct {
	Things []Thing
}

type Thing struct {
	Id          *string
	Order       *int32
	Name        string
	Description *string
	Enabled     *bool
	Priority    *ThingPriority
	ARules      *Rules
	BRules      *Rules
}

type KindAThingWrite struct {
	Thing  ThingCore
	ARules Rules
}

type ThingCore struct {
	Name        string
	Description *string
	Disabled    *bool
	Priority    *ThingPriority
}

type Rules struct {
	Values *string
}

type OverwriteKindAThingsRequest struct {
	Things []KindAThingWrite
}

type OverwriteResponse struct {
	Created []Thing
}

// Stored are the things of the fake API by kind. An overwrite replaces the things of kind A,
// Get returns the things of the kind that its query names. Writes lists the bodies of the
// overwrites, and Kinds the kinds that each Get sent. Tests of the generated resource set and
// read them. WriteErr and ReadErr, when set, are the errors of every overwrite and every Get.
var (
	Stored   = map[ThingKind][]Thing{}
	Writes   []OverwriteKindAThingsRequest
	Kinds    []ThingKind
	WriteErr error
	ReadErr  error
)

type OverwriteThingsServiceAPIService struct{}

type ApiOverwriteThingsServiceListThingsRequest struct{ kind *ThingKind }
type ApiOverwriteThingsServiceOverwriteKindAThingsRequest struct {
	body OverwriteKindAThingsRequest
}

func (*OverwriteThingsServiceAPIService) OverwriteThingsServiceListThings(ctx context.Context) ApiOverwriteThingsServiceListThingsRequest {
	_ = ctx
	return ApiOverwriteThingsServiceListThingsRequest{}
}

func (r ApiOverwriteThingsServiceListThingsRequest) Kind(kind ThingKind) ApiOverwriteThingsServiceListThingsRequest {
	r.kind = &kind
	return r
}

func (*OverwriteThingsServiceAPIService) OverwriteThingsServiceOverwriteKindAThings(ctx context.Context) ApiOverwriteThingsServiceOverwriteKindAThingsRequest {
	_ = ctx
	return ApiOverwriteThingsServiceOverwriteKindAThingsRequest{}
}

func (r ApiOverwriteThingsServiceOverwriteKindAThingsRequest) OverwriteKindAThingsRequest(overwriteKindAThingsRequest OverwriteKindAThingsRequest) ApiOverwriteThingsServiceOverwriteKindAThingsRequest {
	r.body = overwriteKindAThingsRequest
	return r
}

// Execute returns the things of the kind of the query, and of every kind without one.
func (r ApiOverwriteThingsServiceListThingsRequest) Execute() (*ThingList, *http.Response, error) {
	if ReadErr != nil {
		return nil, nil, ReadErr
	}
	out := &ThingList{Things: []Thing{}}
	if r.kind == nil {
		Kinds = append(Kinds, "")
		for _, kind := range []ThingKind{THINGKIND_THING_KIND_A, THINGKIND_THING_KIND_B} {
			out.Things = append(out.Things, Stored[kind]...)
		}
		return out, nil, nil
	}
	Kinds = append(Kinds, *r.kind)
	out.Things = append(out.Things, Stored[*r.kind]...)
	return out, nil, nil
}

// Execute replaces the things of kind A as the API does: each gets a new id and its position,
// enabled is the opposite of disabled, an absent description is "", and rule values are lower
// case.
func (r ApiOverwriteThingsServiceOverwriteKindAThingsRequest) Execute() (*OverwriteResponse, *http.Response, error) {
	Writes = append(Writes, r.body)
	if WriteErr != nil {
		return nil, nil, WriteErr
	}
	things := []Thing{}
	for i, item := range r.body.Things {
		id, order, enabled, description := "thing-"+string(rune('a'+i)), int32(i+1), true, ""
		if item.Thing.Disabled != nil {
			enabled = !*item.Thing.Disabled
		}
		if item.Thing.Description != nil {
			description = *item.Thing.Description
		}
		rules := item.ARules
		if rules.Values != nil {
			lower := strings.ToLower(*rules.Values)
			rules.Values = &lower
		}
		// A thing without a priority reads back with the zero value.
		priority := THINGPRIORITY_THING_PRIORITY_UNSPECIFIED
		if item.Thing.Priority != nil {
			priority = *item.Thing.Priority
		}
		things = append(things, Thing{Id: &id, Order: &order, Name: item.Thing.Name, Description: &description, Enabled: &enabled, Priority: &priority, ARules: &rules})
	}
	Stored[THINGKIND_THING_KIND_A] = things
	return &OverwriteResponse{Created: things}, nil, nil
}
