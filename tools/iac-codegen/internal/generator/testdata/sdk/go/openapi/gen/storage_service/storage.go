package storage_service

import (
	"context"
	"net/http"
)

type StorageConfig struct {
	Gcs    *Gcs
	Ibm    *Ibm
	Prefix *string
	S3     *S3
}

type S3 struct {
	Bucket string
}

type Ibm struct {
	Endpoint string
}

type Gcs struct {
	Bucket string
}

type SetConfigRequest struct {
	Gcs *Gcs
	Ibm *Ibm
	S3  *S3
}

type GetConfigResponse struct {
	Config *StorageConfig
}

// APIError is an error answer of the fake API.
type APIError struct {
	Status int
	Text   string
}

func (e APIError) Error() string   { return e.Text }
func (e APIError) StatusCode() int { return e.Status }
func (e APIError) Body() []byte    { return []byte(e.Text) }

// Stored is the configuration of the fake API, nil when the company has none: Get then answers
// 404. GetErr, when set, answers every Get, and SetErr every Set, which then stores nothing. Set
// replaces the configuration, sets the prefix, and returns an empty object. Sets lists the bodies
// of Set. Tests of the generated resource set and read them.
var (
	Stored *StorageConfig
	GetErr error
	SetErr error
	Sets   []SetConfigRequest
)

type StorageServiceAPIService struct{}

type ApiStorageServiceGetConfigRequest struct{}
type ApiStorageServiceSetConfigRequest struct{ body SetConfigRequest }

func (*StorageServiceAPIService) StorageServiceGetConfig(ctx context.Context) ApiStorageServiceGetConfigRequest {
	_ = ctx
	return ApiStorageServiceGetConfigRequest{}
}

func (*StorageServiceAPIService) StorageServiceSetConfig(ctx context.Context) ApiStorageServiceSetConfigRequest {
	_ = ctx
	return ApiStorageServiceSetConfigRequest{}
}

func (r ApiStorageServiceSetConfigRequest) SetConfigRequest(setConfigRequest SetConfigRequest) ApiStorageServiceSetConfigRequest {
	r.body = setConfigRequest
	return r
}

func (ApiStorageServiceGetConfigRequest) Execute() (*GetConfigResponse, *http.Response, error) {
	if GetErr != nil {
		return nil, nil, GetErr
	}
	if Stored == nil {
		return nil, nil, APIError{Status: http.StatusNotFound, Text: "Not Found: Tenant config not found"}
	}
	config := *Stored
	return &GetConfigResponse{Config: &config}, nil, nil
}

func (r ApiStorageServiceSetConfigRequest) Execute() (map[string]interface{}, *http.Response, error) {
	Sets = append(Sets, r.body)
	if SetErr != nil {
		return nil, nil, SetErr
	}
	prefix := "cx_metrics/"
	Stored = &StorageConfig{S3: r.body.S3, Ibm: r.body.Ibm, Gcs: r.body.Gcs, Prefix: &prefix}
	return map[string]interface{}{}, nil, nil
}
