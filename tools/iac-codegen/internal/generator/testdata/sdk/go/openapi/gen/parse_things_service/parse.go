package parse_things_service

import (
	"context"
	"net/http"
)

type ParseSeverityValue string

const (
	PARSESEVERITYVALUE_SEVERITY_DEBUG_OR_UNSPECIFIED ParseSeverityValue = "SEVERITY_DEBUG_OR_UNSPECIFIED"
	PARSESEVERITYVALUE_SEVERITY_INFO                 ParseSeverityValue = "SEVERITY_INFO"
	PARSESEVERITYVALUE_SEVERITY_ERROR                ParseSeverityValue = "SEVERITY_ERROR"
)

type ParsePickField string

const (
	PARSEPICKFIELD_PICK_FIELD_CATEGORY_OR_UNSPECIFIED ParsePickField = "PICK_FIELD_CATEGORY_OR_UNSPECIFIED"
	PARSEPICKFIELD_PICK_FIELD_THREADID                ParsePickField = "PICK_FIELD_THREADID"
	PARSEPICKFIELD_PICK_FIELD_TEXT                    ParsePickField = "PICK_FIELD_TEXT"
)

type ParseAppName struct {
	Value *string
}

type ParseSeverity struct {
	Value *ParseSeverityValue
}

type ParseMatcher struct {
	AppName  *ParseAppName
	Severity *ParseSeverity
}

type ParseKeepParams struct {
	Rule     *string
	KeepLogs *bool
}

type ParseDropParams struct {
	Rule     *string
	KeepLogs *bool
}

type ParseCopyParams struct {
	Target       *string
	DeleteSource *bool
	Escaped      *bool
}

type ParsePickParams struct {
	Rule  *string
	Field *ParsePickField
}

type ParseParams struct {
	KeepParams *ParseKeepParams
	DropParams *ParseDropParams
	CopyParams *ParseCopyParams
	PickParams *ParsePickParams
}

type CreateParseStep struct {
	Name    *string
	Enabled *bool
	Order   *int64
	Source  *string
	Params  *ParseParams
}

type ParseStep struct {
	Id      *string
	Name    *string
	Enabled *bool
	Order   *int64
	Source  *string
	Params  *ParseParams
}

type CreateParseGroup struct {
	Enabled *bool
	Order   *int64
	Steps   []CreateParseStep
}

type ParseGroup struct {
	Id      *string
	Enabled *bool
	Order   *int64
	Steps   []ParseStep
}

type ParseTeamId struct {
	Id *int64
}

type ParseThing struct {
	Id       *string
	Name     *string
	Enabled  *bool
	Order    *int64
	Note     *string
	Matchers []ParseMatcher
	Groups   []ParseGroup
}

// The Create and Replace bodies are the same inline object, with a team id that the read does not
// have.
type CreateParseThingBody struct {
	Name     *string
	Enabled  *bool
	Order    *int64
	Note     *string
	Matchers []ParseMatcher
	Groups   []CreateParseGroup
	TeamId   *ParseTeamId
}

type ReplaceParseThingBody struct {
	Name     *string
	Enabled  *bool
	Order    *int64
	Note     *string
	Matchers []ParseMatcher
	Groups   []CreateParseGroup
	TeamId   *ParseTeamId
}

type CreateParseThingResponse struct {
	Thing *ParseThing
}

type ReplaceParseThingResponse struct {
	Thing *ParseThing
}

type GetParseThingResponse struct {
	Thing *ParseThing
}

type ParseThingsServiceAPIService struct{}

type ApiParseThingsServiceCreateParseThingRequest struct {
	body CreateParseThingBody
}
type ApiParseThingsServiceGetParseThingRequest struct {
	id string
}
type ApiParseThingsServiceReplaceParseThingRequest struct {
	id   string
	body ReplaceParseThingBody
}
type ApiParseThingsServiceDeleteParseThingRequest struct{}

// Created and Replaced list the bodies that the calls received. Stored is the thing that Get
// returns; Respond, when set, makes it from a body, as the API does (ids, orders, defaults).
var (
	Created  []CreateParseThingBody
	Replaced []ReplaceParseThingBody
	Stored   ParseThing
	Respond  func(name *string, enabled *bool, order *int64, note *string, matchers []ParseMatcher, groups []CreateParseGroup) ParseThing
	GetErr   error
)

func (*ParseThingsServiceAPIService) ParseThingsServiceCreateParseThing(ctx context.Context) ApiParseThingsServiceCreateParseThingRequest {
	_ = ctx
	return ApiParseThingsServiceCreateParseThingRequest{}
}

func (*ParseThingsServiceAPIService) ParseThingsServiceGetParseThing(ctx context.Context, thingId string) ApiParseThingsServiceGetParseThingRequest {
	_ = ctx
	return ApiParseThingsServiceGetParseThingRequest{id: thingId}
}

func (*ParseThingsServiceAPIService) ParseThingsServiceReplaceParseThing(ctx context.Context, thingId string) ApiParseThingsServiceReplaceParseThingRequest {
	_ = ctx
	return ApiParseThingsServiceReplaceParseThingRequest{id: thingId}
}

func (*ParseThingsServiceAPIService) ParseThingsServiceDeleteParseThing(ctx context.Context, thingId string) ApiParseThingsServiceDeleteParseThingRequest {
	_, _ = ctx, thingId
	return ApiParseThingsServiceDeleteParseThingRequest{}
}

func (r ApiParseThingsServiceCreateParseThingRequest) CreateParseThingBody(createParseThingBody CreateParseThingBody) ApiParseThingsServiceCreateParseThingRequest {
	r.body = createParseThingBody
	return r
}

func (r ApiParseThingsServiceReplaceParseThingRequest) ReplaceParseThingBody(replaceParseThingBody ReplaceParseThingBody) ApiParseThingsServiceReplaceParseThingRequest {
	r.body = replaceParseThingBody
	return r
}

func store(name *string, enabled *bool, order *int64, note *string, matchers []ParseMatcher, groups []CreateParseGroup) *ParseThing {
	if Respond != nil {
		Stored = Respond(name, enabled, order, note, matchers, groups)
	} else {
		id := "thing-1"
		Stored = ParseThing{Id: &id, Name: name, Enabled: enabled, Order: order, Note: note, Matchers: matchers}
	}
	stored := Stored
	return &stored
}

func (r ApiParseThingsServiceCreateParseThingRequest) Execute() (*CreateParseThingResponse, *http.Response, error) {
	Created = append(Created, r.body)
	b := r.body
	return &CreateParseThingResponse{Thing: store(b.Name, b.Enabled, b.Order, b.Note, b.Matchers, b.Groups)}, nil, nil
}

func (r ApiParseThingsServiceGetParseThingRequest) Execute() (*GetParseThingResponse, *http.Response, error) {
	if GetErr != nil {
		return nil, nil, GetErr
	}
	stored := Stored
	return &GetParseThingResponse{Thing: &stored}, nil, nil
}

func (r ApiParseThingsServiceReplaceParseThingRequest) Execute() (*ReplaceParseThingResponse, *http.Response, error) {
	Replaced = append(Replaced, r.body)
	b := r.body
	return &ReplaceParseThingResponse{Thing: store(b.Name, b.Enabled, b.Order, b.Note, b.Matchers, b.Groups)}, nil, nil
}

func (ApiParseThingsServiceDeleteParseThingRequest) Execute() (map[string]interface{}, *http.Response, error) {
	return nil, nil, nil
}
