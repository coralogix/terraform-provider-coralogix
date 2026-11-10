package default_things_service

import (
	"context"
	"net/http"
)

type DefaultSeverity string

const (
	DEFAULTSEVERITY_SEVERITY_LOW_OR_UNSPECIFIED DefaultSeverity = "SEVERITY_LOW_OR_UNSPECIFIED"
	DEFAULTSEVERITY_SEVERITY_HIGH               DefaultSeverity = "SEVERITY_HIGH"
)

type DefaultUnit string

const (
	DEFAULTUNIT_UNIT_NEVER_OR_UNSPECIFIED DefaultUnit = "UNIT_NEVER_OR_UNSPECIFIED"
	DEFAULTUNIT_UNIT_HOURS                DefaultUnit = "UNIT_HOURS"
	DEFAULTUNIT_UNIT_DAYS                 DefaultUnit = "UNIT_DAYS"
)

type DefaultRegion struct {
	Name *string
}

type DefaultRouting struct {
	Targets    []string
	Regions    []DefaultRegion
	Severities []DefaultSeverity
}

type DefaultWindow struct {
	Days *int64
}

type DefaultLimits struct {
	Unit    *DefaultUnit
	Enabled *bool
	Count   *int64
	Window  *DefaultWindow
}

type DefaultOverride struct {
	FieldName *string
	Template  *string
}

type DefaultRule struct {
	Name       *string
	Expression *string
}

type DefaultFilters struct {
	Include []string
	Exclude []string
}

type DefaultRemote struct {
	Name      *string
	Filters   *DefaultFilters
	Overrides []DefaultOverride
}

type DefaultGroup struct {
	Name    *string
	Filters *DefaultFilters
}

type DefaultThing struct {
	Id        string
	Name      *string
	Routing   *DefaultRouting
	Limits    *DefaultLimits
	Overrides []DefaultOverride
	Rules     []DefaultRule
	Remotes   []DefaultRemote
	Groups    []DefaultGroup
}

type CreateDefaultThingRequest struct {
	Name      *string
	Routing   *DefaultRouting
	Limits    *DefaultLimits
	Overrides []DefaultOverride
	Rules     []DefaultRule
	Remotes   []DefaultRemote
	Groups    []DefaultGroup
}

type ReplaceDefaultThingRequest struct {
	Name      *string
	Routing   *DefaultRouting
	Limits    *DefaultLimits
	Overrides []DefaultOverride
	Rules     []DefaultRule
	Remotes   []DefaultRemote
	Groups    []DefaultGroup
}

type DefaultThingsServiceAPIService struct{}

type ApiDefaultThingsServiceCreateDefaultThingRequest struct {
	body CreateDefaultThingRequest
}
type ApiDefaultThingsServiceGetDefaultThingRequest struct{}
type ApiDefaultThingsServiceReplaceDefaultThingRequest struct {
	body ReplaceDefaultThingRequest
}
type ApiDefaultThingsServiceDeleteDefaultThingRequest struct{}

// Created and Replaced list the bodies that the calls received. Stored is what Get returns: the
// service returns an omitted object with empty lists, an omitted limits with the stored values (the
// unit never, no flag, a window of 7 days), and an omitted list as an empty one.
var (
	Created  []CreateDefaultThingRequest
	Replaced []ReplaceDefaultThingRequest
	Stored   DefaultThing
)

func (*DefaultThingsServiceAPIService) DefaultThingsServiceCreateDefaultThing(ctx context.Context) ApiDefaultThingsServiceCreateDefaultThingRequest {
	_ = ctx
	return ApiDefaultThingsServiceCreateDefaultThingRequest{}
}

func (*DefaultThingsServiceAPIService) DefaultThingsServiceGetDefaultThing(ctx context.Context, id string) ApiDefaultThingsServiceGetDefaultThingRequest {
	_, _ = ctx, id
	return ApiDefaultThingsServiceGetDefaultThingRequest{}
}

func (*DefaultThingsServiceAPIService) DefaultThingsServiceReplaceDefaultThing(ctx context.Context, id string) ApiDefaultThingsServiceReplaceDefaultThingRequest {
	_, _ = ctx, id
	return ApiDefaultThingsServiceReplaceDefaultThingRequest{}
}

func (*DefaultThingsServiceAPIService) DefaultThingsServiceDeleteDefaultThing(ctx context.Context, id string) ApiDefaultThingsServiceDeleteDefaultThingRequest {
	_, _ = ctx, id
	return ApiDefaultThingsServiceDeleteDefaultThingRequest{}
}

func (r ApiDefaultThingsServiceCreateDefaultThingRequest) CreateDefaultThingRequest(createDefaultThingRequest CreateDefaultThingRequest) ApiDefaultThingsServiceCreateDefaultThingRequest {
	r.body = createDefaultThingRequest
	return r
}

func (r ApiDefaultThingsServiceReplaceDefaultThingRequest) ReplaceDefaultThingRequest(replaceDefaultThingRequest ReplaceDefaultThingRequest) ApiDefaultThingsServiceReplaceDefaultThingRequest {
	r.body = replaceDefaultThingRequest
	return r
}

func store(name *string, routing *DefaultRouting, limits *DefaultLimits, overrides []DefaultOverride, rules []DefaultRule, remotes []DefaultRemote, groups []DefaultGroup) {
	stored := DefaultThing{Id: "thing-1", Name: name, Overrides: []DefaultOverride{}, Rules: []DefaultRule{}, Remotes: []DefaultRemote{}}
	stored.Routing = &DefaultRouting{Targets: []string{}, Regions: []DefaultRegion{}, Severities: []DefaultSeverity{}}
	if routing != nil {
		if routing.Targets != nil {
			stored.Routing.Targets = routing.Targets
		}
		if routing.Regions != nil {
			stored.Routing.Regions = routing.Regions
		}
		if routing.Severities != nil {
			stored.Routing.Severities = routing.Severities
		}
	}
	unit, enabled, days := DEFAULTUNIT_UNIT_NEVER_OR_UNSPECIFIED, false, int64(7)
	stored.Limits = &DefaultLimits{Unit: &unit, Enabled: &enabled, Window: &DefaultWindow{Days: &days}}
	if limits != nil {
		if limits.Unit != nil {
			stored.Limits.Unit = limits.Unit
		}
		if limits.Enabled != nil {
			stored.Limits.Enabled = limits.Enabled
		}
		stored.Limits.Count = limits.Count
		if limits.Window != nil {
			stored.Limits.Window = limits.Window
		}
	}
	if overrides != nil {
		stored.Overrides = overrides
	}
	if rules != nil {
		stored.Rules = rules
	}
	for _, remote := range remotes {
		if remote.Filters == nil {
			remote.Filters = &DefaultFilters{}
		}
		if remote.Filters.Include == nil {
			remote.Filters.Include = []string{}
		}
		if remote.Filters.Exclude == nil {
			remote.Filters.Exclude = []string{}
		}
		if remote.Overrides == nil {
			remote.Overrides = []DefaultOverride{}
		}
		stored.Remotes = append(stored.Remotes, remote)
	}
	for _, group := range groups {
		if group.Filters == nil {
			group.Filters = &DefaultFilters{}
		}
		if group.Filters.Include == nil {
			group.Filters.Include = []string{}
		}
		if group.Filters.Exclude == nil {
			group.Filters.Exclude = []string{}
		}
		stored.Groups = append(stored.Groups, group)
	}
	Stored = stored
}

func (r ApiDefaultThingsServiceCreateDefaultThingRequest) Execute() (*DefaultThing, *http.Response, error) {
	Created = append(Created, r.body)
	store(r.body.Name, r.body.Routing, r.body.Limits, r.body.Overrides, r.body.Rules, r.body.Remotes, r.body.Groups)
	stored := Stored
	return &stored, nil, nil
}

func (ApiDefaultThingsServiceGetDefaultThingRequest) Execute() (*DefaultThing, *http.Response, error) {
	stored := Stored
	return &stored, nil, nil
}

func (r ApiDefaultThingsServiceReplaceDefaultThingRequest) Execute() (*DefaultThing, *http.Response, error) {
	Replaced = append(Replaced, r.body)
	store(r.body.Name, r.body.Routing, r.body.Limits, r.body.Overrides, r.body.Rules, r.body.Remotes, r.body.Groups)
	stored := Stored
	return &stored, nil, nil
}

func (ApiDefaultThingsServiceDeleteDefaultThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return map[string]interface{}{}, nil, nil
}
