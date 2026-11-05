package retag_things_service

import (
	"context"
	"errors"
	"net/http"
)

type TagList struct {
	Tags []Tag
}

type RenameTagsResponse struct {
	Tags []Tag
}

type Tag struct {
	Id    string
	Order int32
	Name  string
}

type TagUpdate struct {
	Id   string
	Name string
}

type RenameTagsRequest struct {
	TagUpdates []TagUpdate
}

// Stored are the tags of the fake API. A rename changes the name of each tag that it names by id and
// returns those tags. Writes lists the bodies of the renames, and Gets counts the reads. Tests of the
// generated resource set and read them. WriteErr and ReadErr, when set, are the errors of every
// rename and every Get.
var (
	Stored   []Tag
	Writes   []RenameTagsRequest
	Gets     int
	WriteErr error
	ReadErr  error
)

type RetagThingsServiceAPIService struct{}

type ApiRetagThingsServiceListTagsRequest struct{}
type ApiRetagThingsServiceRenameTagsRequest struct {
	body RenameTagsRequest
}

func (*RetagThingsServiceAPIService) RetagThingsServiceListTags(ctx context.Context) ApiRetagThingsServiceListTagsRequest {
	_ = ctx
	return ApiRetagThingsServiceListTagsRequest{}
}

func (*RetagThingsServiceAPIService) RetagThingsServiceRenameTags(ctx context.Context) ApiRetagThingsServiceRenameTagsRequest {
	_ = ctx
	return ApiRetagThingsServiceRenameTagsRequest{}
}

func (r ApiRetagThingsServiceRenameTagsRequest) RenameTagsRequest(renameTagsRequest RenameTagsRequest) ApiRetagThingsServiceRenameTagsRequest {
	r.body = renameTagsRequest
	return r
}

func (r ApiRetagThingsServiceListTagsRequest) Execute() (*TagList, *http.Response, error) {
	Gets++
	if ReadErr != nil {
		return nil, nil, ReadErr
	}
	return &TagList{Tags: append([]Tag{}, Stored...)}, nil, nil
}

func (r ApiRetagThingsServiceRenameTagsRequest) Execute() (*RenameTagsResponse, *http.Response, error) {
	Writes = append(Writes, r.body)
	if WriteErr != nil {
		return nil, nil, WriteErr
	}
	out := &RenameTagsResponse{Tags: []Tag{}}
	for _, u := range r.body.TagUpdates {
		found := false
		for i := range Stored {
			if Stored[i].Id == u.Id {
				Stored[i].Name, found = u.Name, true
				out.Tags = append(out.Tags, Stored[i])
			}
		}
		if !found {
			return nil, nil, errors.New("404 Not Found: unknown tag id " + u.Id)
		}
	}
	return out, nil, nil
}
