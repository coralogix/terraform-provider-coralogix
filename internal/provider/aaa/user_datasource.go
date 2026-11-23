package aaa

import (
	"context"
	"fmt"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/user"

	"github.com/coralogix/coralogix-management-sdk/go/openapi/gen/users_management_service"
	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// UserDataSource is the generated coralogix_user data source with the released
// user_name lookup. The Users Search narrows server-side with a username filter
// (the backend applies an exact, case-insensitive equality), so the wrapper
// resolves a configured user_name to the one user whose username equals it
// case-insensitively, and hands the id to the generated read. The lookup cannot be generated yet: the generator refuses a
// name lookup for a resource that reads from its List, which is a V6 generator
// follow-up; the recorded baseline scenarios are the semantics contract.
type UserDataSource struct {
	user.DataSource

	client *users_management_service.UsersManagementServiceAPIService
}

// NewUserDataSource returns the wrapped data source. Pass the hooks of the
// resource, as the generated constructor expects.
func NewUserDataSource(hooks user.Hooks) datasource.DataSource {
	return &UserDataSource{DataSource: *user.NewDataSource(hooks).(*user.DataSource)}
}

// Schema is the generated schema with two lookup keys: the id and the
// user_name, exactly one of the two set, as the released data source had.
func (d *UserDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	d.DataSource.Schema(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		return
	}
	rawID, ok := resp.Schema.Attributes["id"].(datasourceschema.StringAttribute)
	if !ok {
		resp.Diagnostics.AddError("Data source schema error",
			"The generated coralogix_user schema has no id string attribute.")
		return
	}
	resp.Schema.Attributes["id"] = idAttrAsOptional(rawID)

	userName, ok := resp.Schema.Attributes["user_name"].(datasourceschema.StringAttribute)
	if !ok {
		resp.Diagnostics.AddError("Data source schema error",
			"The generated coralogix_user schema has no user_name string attribute.")
		return
	}
	userName.Optional = true
	userName.Computed = true
	userName.Validators = []validator.String{stringvalidator.LengthAtLeast(1)}
	userName.MarkdownDescription = "User name (email). Exactly one of `id` or `user_name` must be set. " +
		"The lookup is case-insensitive, since SSO login can normalize letter case in the backend."
	resp.Schema.Attributes["user_name"] = userName

	// emails is read-only, as the released schema had it: the set is derived from
	// the username, and a configuration never sets it. The generator's extra
	// attributes are Optional+Computed (its parser refuses a computed-only object
	// set), so the wrapper narrows the set and every descendant here.
	if emails, ok := resp.Schema.Attributes["emails"].(datasourceschema.SetNestedAttribute); ok {
		emails.Optional = false
		emails.Computed = true
		nested := emails.NestedObject.Attributes
		if primary, ok := nested["primary"].(datasourceschema.BoolAttribute); ok {
			primary.Optional = false
			primary.Computed = true
			nested["primary"] = primary
		}
		for _, name := range []string{"type", "value"} {
			if at, ok := nested[name].(datasourceschema.StringAttribute); ok {
				at.Optional = false
				at.Computed = true
				nested[name] = at
			}
		}
		emails.NestedObject.Attributes = nested
		resp.Schema.Attributes["emails"] = emails
	}
}

// idAttrAsOptional returns the id attribute as optional and computed, for a data
// source with two lookup keys: exactly one of the two must be set, so neither is
// required, and the read fills the id when the lookup is the user_name.
func idAttrAsOptional(attr datasourceschema.StringAttribute) datasourceschema.StringAttribute {
	attr.Required = false
	attr.Optional = true
	attr.Computed = true
	attr.Validators = []validator.String{stringvalidator.LengthAtLeast(1)}
	attr.MarkdownDescription = "User ID. Exactly one of `id` or `user_name` must be set."
	return attr
}

// ConfigValidators requires exactly one lookup key, as the released data source did.
func (d *UserDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(
			path.MatchRoot("id"),
			path.MatchRoot("user_name"),
		),
	}
}

func (d *UserDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.DataSource.Configure(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		return
	}
	if cs, ok := req.ProviderData.(*clientset.ClientSet); ok {
		d.client = cs.Users()
	}
}

// Read resolves a configured user_name to the user's id first, then runs the
// generated read with that id. The resolution searches with the username filter
// the API documents and matches case-insensitively, as the released lookup did.
func (d *UserDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var configuredID, configuredUserName types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &configuredID)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("user_name"), &configuredUserName)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// ExactlyOneOf skips its check while a value is unknown, which a reference to
	// another resource's id is at validate time. The released read rejected a
	// configuration that sets both by the time the read runs, and this one does too.
	idSet := isKnownLookup(configuredID)
	userNameSet := isKnownLookup(configuredUserName)
	switch {
	case idSet && userNameSet:
		resp.Diagnostics.AddError(
			"Exactly one of id or user_name must be set",
			"Both were set by the time the User data source was read. Remove one of them — "+
				"the two are alternative lookup keys, not filters to combine.",
		)
		return
	case userNameSet:
		resolved, diags := resolveUserByUsername(ctx, d.client, configuredUserName.ValueString(), "Unable to read coralogix_user")
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		if err := rewriteConfig(&req.Config, resolved.GetUserId()); err != nil {
			resp.Diagnostics.AddError("Unable to read coralogix_user", err.Error())
			return
		}
		// The generated read that follows searches again: narrow it with the
		// username the resolution found, instead of paging the whole team.
		ctx = context.WithValue(ctx, userReadNameKey{}, configuredUserName.ValueString())
	case !idSet:
		resp.Diagnostics.AddError("User id or user_name must be set", "")
		return
	}

	d.DataSource.Read(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		return
	}

	// The state carries the username the backend answers with; a lookup in
	// another letter case keeps the case of the configuration, as the released
	// data source recorded (the backend folds the case of an SSO login).
	if userNameSet {
		d.keepConfiguredCase(ctx, resp, configuredUserName.ValueString())
	}
}

// isKnownLookup reports whether a configured lookup key holds a usable value.
func isKnownLookup(value types.String) bool {
	return !value.IsNull() && !value.IsUnknown() && value.ValueString() != ""
}

// rewriteConfig sets the id attribute of the data-source configuration to the
// resolved id, so the generated read runs its usual path. The rest of the
// configuration is untouched.
func rewriteConfig(config *tfsdk.Config, id string) error {
	object, ok := config.Raw.Type().(tftypes.Object)
	if !ok {
		return fmt.Errorf("the configuration is %T, not an object", config.Raw.Type())
	}
	attrs := make(map[string]tftypes.Value, len(object.AttributeTypes))
	for name, attrType := range object.AttributeTypes {
		value, _, err := tftypes.WalkAttributePath(config.Raw, tftypes.NewAttributePath().WithAttributeName(name))
		if err != nil {
			value = tftypes.NewValue(attrType, nil)
		}
		attrs[name] = value.(tftypes.Value)
	}
	attrs["id"] = tftypes.NewValue(object.AttributeTypes["id"], id)
	config.Raw = tftypes.NewValue(config.Raw.Type(), attrs)
	return nil
}

// keepConfiguredCase writes the case of the configured user_name into the state,
// when the state username equals it case-insensitively.
func (d *UserDataSource) keepConfiguredCase(ctx context.Context, resp *datasource.ReadResponse, configured string) {
	var stateUserName types.String
	diags := resp.State.GetAttribute(ctx, path.Root("user_name"), &stateUserName)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || !isKnownLookup(stateUserName) {
		return
	}
	if strings.EqualFold(stateUserName.ValueString(), configured) && stateUserName.ValueString() != configured {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_name"), configured)...)
	}
}
