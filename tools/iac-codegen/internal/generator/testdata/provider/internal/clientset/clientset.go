package clientset

import (
	"example.com/iac-test-sdk/go/openapi/gen/action_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/archived_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/config_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/custom_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/echo_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/eval_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/folder_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/legacy_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/rename_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/rulebook_service"
	"example.com/iac-test-sdk/go/openapi/gen/settings_service"
	"example.com/iac-test-sdk/go/openapi/gen/shared_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/suppressed_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/things_service"
	"example.com/iac-test-sdk/go/openapi/gen/upsert_service"
	"example.com/iac-test-sdk/go/openapi/gen/wrap_things_service"
)

type ClientSet struct {
	ActionThing    *action_things_service.ActionThingsServiceAPIService
	Folder         *folder_things_service.FolderThingsServiceAPIService
	Client         *things_service.ThingsServiceAPIService
	SettingsClient *settings_service.SettingsServiceAPIService
	Legacy         *legacy_things_service.LegacyThingsServiceAPIService
	Archived       *archived_things_service.ArchivedThingsServiceAPIService
	Config         *config_things_service.ConfigThingsServiceAPIService
	Wrap           *wrap_things_service.WrapThingsServiceAPIService
	Custom         *custom_things_service.CustomThingsServiceAPIService
	Shared         *shared_things_service.SharedThingsServiceAPIService
	Echo           *echo_things_service.EchoThingsServiceAPIService
	Eval           *eval_things_service.EvalThingsServiceAPIService
	Suppressed     *suppressed_things_service.SuppressedThingsServiceAPIService
	RulebookClient *rulebook_service.RulebookServiceAPIService
	UpsertClient   *upsert_service.UpsertServiceAPIService
	Rename         *rename_things_service.RenameThingsServiceAPIService
}

func (c *ClientSet) Things() *things_service.ThingsServiceAPIService { return c.Client }

func (c *ClientSet) Rulebook() *rulebook_service.RulebookServiceAPIService { return c.RulebookClient }

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

func (c *ClientSet) CustomThings() *custom_things_service.CustomThingsServiceAPIService {
	return c.Custom
}

func (c *ClientSet) SharedThings() *shared_things_service.SharedThingsServiceAPIService {
	return c.Shared
}

func (c *ClientSet) EchoThings() *echo_things_service.EchoThingsServiceAPIService {
	return c.Echo
}

func (c *ClientSet) EvalThings() *eval_things_service.EvalThingsServiceAPIService {
	return c.Eval
}

func (c *ClientSet) SuppressedThings() *suppressed_things_service.SuppressedThingsServiceAPIService {
	return c.Suppressed
}
func (c *ClientSet) Upsert() *upsert_service.UpsertServiceAPIService { return c.UpsertClient }

func (c *ClientSet) FolderThings() *folder_things_service.FolderThingsServiceAPIService {
	return c.Folder
}

func (c *ClientSet) RenameThings() *rename_things_service.RenameThingsServiceAPIService {
	return c.Rename
}

func (c *ClientSet) ActionThings() *action_things_service.ActionThingsServiceAPIService {
	return c.ActionThing
}
