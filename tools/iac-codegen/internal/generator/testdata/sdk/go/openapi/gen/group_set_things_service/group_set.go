package group_set_things_service

import (
	"context"
	"net/http"
)

type CreateGroupSetThing struct {
	Groups []InGroupThing
	Name   *string
}

type UpdateGroupSetThing struct {
	Groups []InGroupThing
	Name   *string
}

type CreateGroupSetThingResult struct {
	Id string
}

type GroupSetThing struct {
	Groups []OutGroupThing
	Id     string
	Name   string
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

func store(id string, name *string, groups []InGroupThing) {
	Stored = GroupSetThing{Id: id, Name: id}
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
		Stored.Groups = append(Stored.Groups, out)
	}
}

func (r ApiGroupSetsCreateRequest) Execute() (*CreateGroupSetThingResult, *http.Response, error) {
	Created = append(Created, r.body)
	store("set-1", r.body.Name, r.body.Groups)
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
	store(r.id, name, r.body.Groups)
	return map[string]interface{}{}, nil, nil
}

func (ApiGroupSetsDeleteRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return map[string]interface{}{}, nil, nil
}
