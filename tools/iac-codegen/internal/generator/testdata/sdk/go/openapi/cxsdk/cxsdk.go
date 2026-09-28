package cxsdk

import (
	"net/http"

	"example.com/iac-test-sdk/go/openapi/gen/things_service"
)

type ClientSet struct {
	Client *things_service.ThingsServiceAPIService
}

func (c *ClientSet) Things() *things_service.ThingsServiceAPIService { return c.Client }

func NewAPIError(resp *http.Response, err error) error {
	_ = resp
	return err
}

func Code(err error) int {
	_ = err
	return 0
}
