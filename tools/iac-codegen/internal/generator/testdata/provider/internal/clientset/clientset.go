package clientset

import (
	"example.com/iac-test-sdk/go/openapi/gen/settings_service"
	"example.com/iac-test-sdk/go/openapi/gen/things_service"
)

type ClientSet struct {
	Client         *things_service.ThingsServiceAPIService
	SettingsClient *settings_service.SettingsServiceAPIService
}

func (c *ClientSet) Things() *things_service.ThingsServiceAPIService { return c.Client }

func (c *ClientSet) Settings() *settings_service.SettingsServiceAPIService { return c.SettingsClient }
