package group_set_things_service

import (
	"context"
	"net/http"
)

type AbsoluteWindow struct {
	End   *string
	Start string
}

// A oneOf whose arms have no fields.
type Mode struct {
	Fast         map[string]interface{}
	Slow         map[string]interface{}
	BalancedMode map[string]interface{}
}

// A oneOf: AbsoluteWindow, or RelativeWindow, a plain duration. Mode is a field of its own.
type Schedule struct {
	AbsoluteWindow *AbsoluteWindow
	RelativeWindow *string
	Mode           *Mode
}

type CreateGroupSetThing struct {
	AbsoluteWindow *AbsoluteWindow
	FiveMinutes    map[string]interface{}
	Groups         []InGroupThing
	Name           *string
	Off            map[string]interface{}
	OneHour        map[string]interface{}
	RelativeWindow *string
	Schedule       *Schedule
}

type UpdateGroupSetThing struct {
	AbsoluteWindow *AbsoluteWindow
	FiveMinutes    map[string]interface{}
	Groups         []InGroupThing
	Name           *string
	Off            map[string]interface{}
	OneHour        map[string]interface{}
	RelativeWindow *string
	Schedule       *Schedule
}

type CreateGroupSetThingResult struct {
	Id string
}

type GroupSetThing struct {
	AbsoluteWindow *AbsoluteWindow
	FiveMinutes    map[string]interface{}
	Groups         map[string]OutGroupThing
	Id             string
	Name           string
	Off            map[string]interface{}
	OneHour        map[string]interface{}
	RelativeWindow *string
	Schedule       *Schedule
}

type InGroupThing struct {
	Interval      *int64
	Items         []InItemThing
	Limit         *string
	Name          string
	PeriodMinutes *int64
}

type OutGroupThing struct {
	Interval      *int64
	Items         []OutItemThing
	LastSeenAt    *string
	Limit         *string
	Name          string
	PeriodMinutes *int64
}

type InItemThing struct {
	Key    string
	Labels map[string]string
}

type OutItemThing struct {
	Key    string
	Labels map[string]string
}

type GroupSetThingsServiceAPIService struct{}

type ApiGroupSetsCreateRequest struct {
	body CreateGroupSetThing
}
type ApiGroupSetsFetchRequest struct {
	id string
}
type ApiGroupSetsUpdateRequest struct {
	id   string
	body UpdateGroupSetThing
}
type ApiGroupSetsDeleteRequest struct{}

// Created and Updated list the bodies that the calls received. Stored is what Fetch returns: the
// service stores an omitted interval as 60, an omitted limit as "0", and omitted labels as {}.
var (
	Created []CreateGroupSetThing
	Updated []UpdateGroupSetThing
	Stored  GroupSetThing
)

func (*GroupSetThingsServiceAPIService) GroupSetsCreate(ctx context.Context) ApiGroupSetsCreateRequest {
	_ = ctx
	return ApiGroupSetsCreateRequest{}
}

func (*GroupSetThingsServiceAPIService) GroupSetsFetch(ctx context.Context, id string) ApiGroupSetsFetchRequest {
	_ = ctx
	return ApiGroupSetsFetchRequest{id: id}
}

func (*GroupSetThingsServiceAPIService) GroupSetsUpdate(ctx context.Context, id string) ApiGroupSetsUpdateRequest {
	_ = ctx
	return ApiGroupSetsUpdateRequest{id: id}
}

func (*GroupSetThingsServiceAPIService) GroupSetsDelete(ctx context.Context, id string) ApiGroupSetsDeleteRequest {
	_, _ = ctx, id
	return ApiGroupSetsDeleteRequest{}
}

func (r ApiGroupSetsCreateRequest) CreateGroupSetThing(createGroupSetThing CreateGroupSetThing) ApiGroupSetsCreateRequest {
	r.body = createGroupSetThing
	return r
}

func (r ApiGroupSetsUpdateRequest) UpdateGroupSetThing(updateGroupSetThing UpdateGroupSetThing) ApiGroupSetsUpdateRequest {
	r.body = updateGroupSetThing
	return r
}

// refresh holds the arms of the refresh that a request sent.
type refresh struct{ off, fiveMinutes, oneHour map[string]interface{} }

func store(id string, name *string, groups []InGroupThing, schedule *Schedule, absolute *AbsoluteWindow, relative *string, arms refresh) {
	Stored = GroupSetThing{Id: id, Name: id, Groups: map[string]OutGroupThing{}, Schedule: schedule, AbsoluteWindow: absolute, RelativeWindow: relative,
		Off: arms.off, FiveMinutes: arms.fiveMinutes, OneHour: arms.oneHour}
	if arms.off == nil && arms.fiveMinutes == nil && arms.oneHour == nil {
		// The service refreshes nothing when no arm is sent, and says so.
		Stored.Off = map[string]interface{}{}
	}
	if name != nil {
		Stored.Name = *name
	}
	for _, g := range groups {
		interval, limit, seen := int64(60), "0", "18446744073709551615"
		if g.Interval != nil {
			interval = *g.Interval
		}
		if g.Limit != nil {
			limit = *g.Limit
		}
		out := OutGroupThing{Name: g.Name, Interval: &interval, Limit: &limit, LastSeenAt: &seen, PeriodMinutes: g.PeriodMinutes, Items: []OutItemThing{}}
		for _, item := range g.Items {
			labels := item.Labels
			if labels == nil {
				labels = map[string]string{}
			}
			out.Items = append(out.Items, OutItemThing{Key: item.Key, Labels: labels})
		}
		// The response map key is independent of the object fields that Terraform exposes.
		Stored.Groups["server:"+g.Name] = out
	}
}

func (r ApiGroupSetsCreateRequest) Execute() (*CreateGroupSetThingResult, *http.Response, error) {
	Created = append(Created, r.body)
	store("set-1", r.body.Name, r.body.Groups, r.body.Schedule, r.body.AbsoluteWindow, r.body.RelativeWindow, refresh{r.body.Off, r.body.FiveMinutes, r.body.OneHour})
	return &CreateGroupSetThingResult{Id: "set-1"}, nil, nil
}

func (r ApiGroupSetsFetchRequest) Execute() (*GroupSetThing, *http.Response, error) {
	stored := Stored
	return &stored, nil, nil
}

func (r ApiGroupSetsUpdateRequest) Execute() (map[string]interface{}, *http.Response, error) {
	Updated = append(Updated, r.body)
	name := &Stored.Name
	if r.body.Name != nil {
		name = r.body.Name
	}
	store(r.id, name, r.body.Groups, r.body.Schedule, r.body.AbsoluteWindow, r.body.RelativeWindow, refresh{r.body.Off, r.body.FiveMinutes, r.body.OneHour})
	return map[string]interface{}{}, nil, nil
}

func (ApiGroupSetsDeleteRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return map[string]interface{}{}, nil, nil
}
