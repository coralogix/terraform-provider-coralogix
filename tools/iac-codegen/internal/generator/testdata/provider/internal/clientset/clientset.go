package clientset

import "example.com/iac-test-sdk/go/openapi/gen/things_service"

type ClientSet struct {
	Client *things_service.ThingsServiceAPIService
}

func (c *ClientSet) Things() *things_service.ThingsServiceAPIService { return c.Client }
