package clientset

import (
	"example.com/iac-test-sdk/go/openapi/gen/archived_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/config_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/legacy_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/settings_service"
	"example.com/iac-test-sdk/go/openapi/gen/things_service"
	"example.com/iac-test-sdk/go/openapi/gen/wrap_things_service"
)

type ClientSet struct {
	Client         *things_service.ThingsServiceAPIService
	SettingsClient *settings_service.SettingsServiceAPIService
	Legacy         *legacy_things_service.LegacyThingsServiceAPIService
	Archived       *archived_things_service.ArchivedThingsServiceAPIService
	Config         *config_things_service.ConfigThingsServiceAPIService
	Wrap           *wrap_things_service.WrapThingsServiceAPIService
}

func (c *ClientSet) Things() *things_service.ThingsServiceAPIService { return c.Client }

func (c *ClientSet) Settings() *settings_service.SettingsServiceAPIService { return c.SettingsClient }

func (c *ClientSet) LegacyThings() *legacy_things_service.LegacyThingsServiceAPIService {
	return c.Legacy
}

func (c *ClientSet) ArchivedThings() *archived_things_service.ArchivedThingsServiceAPIService {
	return c.Archived
}

func (c *ClientSet) ConfigThings() *config_things_service.ConfigThingsServiceAPIService {
	return c.Config
}

func (c *ClientSet) WrapThings() *wrap_things_service.WrapThingsServiceAPIService {
	return c.Wrap
}
