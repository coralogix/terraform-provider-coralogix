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
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// UserDataSource is the generated coralogix_user data source with the released
// user_name lookup. The Users Search answers a server-side username filter
// (partial match), so the wrapper resolves a configured user_name to the one
// user whose username equals it case-insensitively, and hands the id to the
// generated read. The lookup cannot be generated yet: the generator refuses a
// name lookup for a resource that reads from its List (CX-71324 documents the
// capability; the recorded baseline scenarios are the semantics contract).
type UserDataSource struct {
	user.DataSource

	client *users_management_service.UsersManagementServiceAPIService
}

// NewUserDataSource returns the wrapped data source. Pass the hooks of the
// resource, as the generated constructor expects.
func NewUserDataSource(hooks user.Hooks) datasource.DataSource {
	return &UserDataSource{DataSource: *user.NewDataSource(hooks).(*user.DataSource)}
}

// Schema is the generated schema with user_name optional: the second lookup key.
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
}

// idAttrAsOptional returns the id attribute as optional, for a data source with two
// lookup keys: exactly one of the two must be set, so neither is required.
func idAttrAsOptional(attr datasourceschema.StringAttribute) datasourceschema.StringAttribute {
	attr.Required = false
	attr.Optional = true
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
	if configuredID.IsNull() || configuredID.IsUnknown() || configuredID.ValueString() == "" {
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("user_name"), &configuredUserName)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if configuredUserName.IsNull() || configuredUserName.IsUnknown() || configuredUserName.ValueString() == "" {
			resp.Diagnostics.AddError("User id or user_name must be set", "")
			return
		}
		resolved, diags := d.userIDByName(ctx, configuredUserName.ValueString())
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		if err := d.rewriteConfig(&req.Config, *resolved); err != nil {
			resp.Diagnostics.AddError("Unable to read coralogix_user", err.Error())
			return
		}
	}

	d.DataSource.Read(ctx, req, resp)
	if resp.Diagnostics.HasError() {
		return
	}

	// The state carries the username the backend answers with; a lookup in
	// another letter case keeps the case of the configuration, as the released
	// data source recorded (the backend folds the case of an SSO login).
	if !configuredUserName.IsNull() && !configuredUserName.IsUnknown() {
		d.keepConfiguredCase(ctx, resp, configuredUserName.ValueString())
	}
}

// userIDByName resolves a user_name to the id of the one user whose username
// equals it case-insensitively. The search narrows server-side with the
// username filter, and the wrapper pages the result, so a user past the first
// page is still found.
func (d *UserDataSource) userIDByName(ctx context.Context, userName string) (*string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if d.client == nil {
		diags.AddError("Unable to read coralogix_user",
			"The data source has no users client. This is a provider bug; please report it.")
		return nil, diags
	}

	var matches []*users_management_service.RbacV2User
	pageToken := int64(0)
	for page := 0; page < 100; page++ {
		request := d.client.UsersMgmtServiceSearchUsers(ctx).Username(userName).PageSize(100)
		if pageToken != 0 {
			request = request.PageToken(pageToken)
		}
		out, _, err := request.Execute()
		if err != nil {
			diags.AddError("Unable to read coralogix_user", fmt.Sprintf("The search for user_name %q failed: %s", userName, err))
			return nil, diags
		}
		for i := range out.GetUsers() {
			if strings.EqualFold(out.Users[i].GetUsername(), userName) {
				matches = append(matches, &out.Users[i])
			}
		}
		pageToken = out.GetNextPageToken()
		if pageToken == 0 || pageToken < 0 || len(out.GetUsers()) == 0 {
			break
		}
	}

	switch len(matches) {
	case 0:
		diags.AddError(fmt.Sprintf("User with user_name %q not found", userName), "")
	case 1:
		if matches[0].GetUserId() == "" {
			diags.AddError(
				fmt.Sprintf("User with user_name %q was returned without an id", userName),
				"Use the user id instead, or report this to the provider developers.",
			)
			return nil, diags
		}
		return &matches[0].UserId, diags
	default:
		ids := make([]string, 0, len(matches))
		for _, match := range matches {
			if id := match.GetUserId(); id != "" {
				ids = append(ids, id)
			}
		}
		diags.AddError(
			fmt.Sprintf("Multiple Users found with user_name %q", userName),
			fmt.Sprintf("Matched user ids: %s. Use the user id instead.", strings.Join(ids, ", ")),
		)
	}
	return nil, diags
}

// rewriteConfig sets the id attribute of the data-source configuration to the
// resolved id, so the generated read runs its usual path. The rest of the
// configuration is untouched.
func (d *UserDataSource) rewriteConfig(config *tfsdk.Config, id string) error {
	object := config.Raw.Type().(tftypes.Object)
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
	var state struct {
		UserName string `tfsdk:"user_name"`
	}
	diags := resp.State.GetAttribute(ctx, path.Root("user_name"), &state.UserName)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || state.UserName == "" || strings.EqualFold(state.UserName, configured) && state.UserName == configured {
		return
	}
	if strings.EqualFold(state.UserName, configured) {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_name"), configured)...)
	}
}
