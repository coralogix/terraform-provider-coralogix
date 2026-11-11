package clientset

import (
	"example.com/iac-test-sdk/go/openapi/gen/action_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/archived_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/config_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/custom_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/default_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/echo_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/eval_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/filter_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/folder_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/group_set_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/key_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/legacy_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/method_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/overwrite_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/parse_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/rename_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/retag_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/rulebook_service"
	"example.com/iac-test-sdk/go/openapi/gen/settings_service"
	"example.com/iac-test-sdk/go/openapi/gen/shared_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/storage_service"
	"example.com/iac-test-sdk/go/openapi/gen/suppressed_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/things_service"
	"example.com/iac-test-sdk/go/openapi/gen/typed_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/union_wrap_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/upsert_service"
	"example.com/iac-test-sdk/go/openapi/gen/wrap_things_service"
	"example.com/iac-test-sdk/go/openapi/gen/wrapped_things_service"
)

type ClientSet struct {
	Overwrite      *overwrite_things_service.OverwriteThingsServiceAPIService
	ActionThing    *action_things_service.ActionThingsServiceAPIService
	Folder         *folder_things_service.FolderThingsServiceAPIService
	WrappedID      *wrapped_things_service.WrappedThingsServiceAPIService
	Client         *things_service.ThingsServiceAPIService
	SettingsClient *settings_service.SettingsServiceAPIService
	Legacy         *legacy_things_service.LegacyThingsServiceAPIService
	Archived       *archived_things_service.ArchivedThingsServiceAPIService
	Config         *config_things_service.ConfigThingsServiceAPIService
	Wrap           *wrap_things_service.WrapThingsServiceAPIService
	UnionWrap      *union_wrap_things_service.UnionWrapThingsServiceAPIService
	Custom         *custom_things_service.CustomThingsServiceAPIService
	Shared         *shared_things_service.SharedThingsServiceAPIService
	Echo           *echo_things_service.EchoThingsServiceAPIService
	Eval           *eval_things_service.EvalThingsServiceAPIService
	Suppressed     *suppressed_things_service.SuppressedThingsServiceAPIService
	RulebookClient *rulebook_service.RulebookServiceAPIService
	UpsertClient   *upsert_service.UpsertServiceAPIService
	StorageClient  *storage_service.StorageServiceAPIService
	Rename         *rename_things_service.RenameThingsServiceAPIService
	Parse          *parse_things_service.ParseThingsServiceAPIService
	Filter         *filter_things_service.FilterThingsServiceAPIService
	Typed          *typed_things_service.TypedThingsServiceAPIService
	GroupSet       *group_set_things_service.GroupSetThingsServiceAPIService
	Retag          *retag_things_service.RetagThingsServiceAPIService
	Default        *default_things_service.DefaultThingsServiceAPIService
	Key            *key_things_service.KeyThingsServiceAPIService
	Method         *method_things_service.MethodThingsServiceAPIService
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

func (c *ClientSet) DefaultThings() *default_things_service.DefaultThingsServiceAPIService {
	return c.Default
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
func (c *ClientSet) Storage() *storage_service.StorageServiceAPIService {
	return c.StorageClient
}

func (c *ClientSet) FolderThings() *folder_things_service.FolderThingsServiceAPIService {
	return c.Folder
}

func (c *ClientSet) WrappedThings() *wrapped_things_service.WrappedThingsServiceAPIService {
	return c.WrappedID
}

func (c *ClientSet) MethodThings() *method_things_service.MethodThingsServiceAPIService {
	return c.Method
}

func (c *ClientSet) KeyThings() *key_things_service.KeyThingsServiceAPIService {
	return c.Key
}

func (c *ClientSet) RenameThings() *rename_things_service.RenameThingsServiceAPIService {
	return c.Rename
}

func (c *ClientSet) ActionThings() *action_things_service.ActionThingsServiceAPIService {
	return c.ActionThing
}

func (c *ClientSet) TypedThings() *typed_things_service.TypedThingsServiceAPIService {
	return c.Typed
}

func (c *ClientSet) FilterThings() *filter_things_service.FilterThingsServiceAPIService {
	return c.Filter
}

func (c *ClientSet) ParseThings() *parse_things_service.ParseThingsServiceAPIService {
	return c.Parse
}

func (c *ClientSet) GroupSetThings() *group_set_things_service.GroupSetThingsServiceAPIService {
	return c.GroupSet
}

func (c *ClientSet) OverwriteThings() *overwrite_things_service.OverwriteThingsServiceAPIService {
	return c.Overwrite
}

func (c *ClientSet) RetagThings() *retag_things_service.RetagThingsServiceAPIService {
	return c.Retag
}

func (c *ClientSet) UnionWrapThings() *union_wrap_things_service.UnionWrapThingsServiceAPIService {
	return c.UnionWrap
}
