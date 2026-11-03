package webhook

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	webhooks "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/outgoing_webhooks_service"
	generatedwebhook "github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/webhook"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// NewResourceWithBehavior adds the Terraform-only webhook fields to generated CRUD.
func NewResourceWithBehavior() resource.Resource {
	return &webhookResource{Resource: generatedwebhook.NewResource(webhookHooks()).(*generatedwebhook.Resource)}
}

type webhookResource struct{ *generatedwebhook.Resource }

// The schema validates the arms, as it did in the released resource.
func (r *webhookResource) ConfigValidators(context.Context) []resource.ConfigValidator { return nil }

func (r *webhookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	r.Resource.ImportState(ctx, req, resp)
	resp.Diagnostics.AddWarning("An imported credential is written to state",
		"Importing reads the webhook's current configuration from Coralogix, including any credential it carries, and writes it to state. "+
			"Treat an imported credential as exposed and rotate it.\n\n"+
			"To keep it out of state from then on, move the value into the matching write-only attribute "+
			"(jira.api_token_wo, pager_duty.service_key_wo, or custom.headers_wo), set its version, and apply. "+
			"That apply removes the imported value from state. Write-only attributes need Terraform 1.11 or later.")
}

func (r *webhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.Resource.Create(ctx, req, resp)
	resp.Diagnostics = legacyWebhookDiagnostics("Create", resp.Diagnostics, "")
}

func (r *webhookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	r.Resource.Read(ctx, req, resp)
	var id types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &id)...)
	resp.Diagnostics = legacyWebhookDiagnostics("Read", resp.Diagnostics, id.ValueString())
}

func (r *webhookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.Resource.Update(ctx, req, resp)
	resp.Diagnostics = legacyWebhookDiagnostics("Update", resp.Diagnostics, "")
}

func (r *webhookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	r.Resource.Delete(ctx, req, resp)
	resp.Diagnostics = legacyWebhookDiagnostics("Delete", resp.Diagnostics, "")
}

func legacyWebhookDiagnostics(operation string, given diag.Diagnostics, id string) diag.Diagnostics {
	var out diag.Diagnostics
	for _, item := range given {
		if operation == "Read" && item.Summary() == "Resource disappeared during read" {
			out.AddWarning(fmt.Sprintf("Resource %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id))
			continue
		}
		if strings.HasPrefix(item.Summary(), "Unable to ") {
			summary, detail := legacyWebhookDiagnostic(operation, item.Detail())
			out.AddError(summary, detail)
			continue
		}
		out.Append(item)
	}
	return out
}

func legacyWebhookDiagnostic(operation, detail string) (string, string) {
	summary := "Error " + map[string]string{"Create": "creating", "Read": "reading", "Update": "updating", "Delete": "deleting"}[operation] + " coralogix_webhook"
	if operation == "Create" && strings.Contains(detail, "but could not read it: ") {
		detail = strings.SplitN(detail, "but could not read it: ", 2)[1]
		return "Error reading coralogix_webhook", "error - " + detail + "\noperation - Read"
	}
	if operation == "Update" && strings.Contains(detail, "Webhook with id ") && strings.HasPrefix(detail, "id ") && strings.Contains(detail, "500 ") {
		summary = "Error reading coralogix_webhook, state not updated"
	}
	if operation != "Create" && strings.HasPrefix(detail, "id ") && strings.Contains(detail, ": ") {
		detail = strings.SplitN(detail, ": ", 2)[1]
	}
	return summary, legacyWebhookError(operation, detail)
}

func legacyWebhookError(operation, detail string) string {
	category := ""
	switch {
	case strings.HasPrefix(detail, "400 "), strings.HasPrefix(detail, "422 "):
		category = "invalid argument error.\n"
	case strings.HasPrefix(detail, "500 "), strings.HasPrefix(detail, "502 "), strings.HasPrefix(detail, "503 "):
		category = "internal error in Coralogix backend.\n"
	}
	return category + "error - " + detail + "\noperation - " + operation
}

// NewDataSourceWithBehavior uses the same response conversion as the resource.
func NewDataSourceWithBehavior() datasource.DataSource {
	return &webhookDataSource{DataSource: generatedwebhook.NewDataSource(webhookHooks()).(*generatedwebhook.DataSource)}
}

type webhookDataSource struct{ *generatedwebhook.DataSource }

func (d *webhookDataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	for _, name := range []string{"id", "name"} {
		var value types.String
		resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(name), &value)...)
		if !value.IsNull() && !value.IsUnknown() && strings.TrimSpace(value.ValueString()) == "" {
			resp.Diagnostics.AddAttributeError(path.Root(name), "Invalid Attribute Value Match",
				fmt.Sprintf("Attribute %s must not be empty or contain only whitespace, got: %s", name, value.ValueString()))
		}
	}
}

func (d *webhookDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	d.DataSource.Read(ctx, req, resp)
	var name, id types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("name"), &name)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("id"), &id)...)
	if !resp.Diagnostics.HasError() {
		return
	}
	var rewritten diag.Diagnostics
	for _, item := range resp.Diagnostics {
		switch {
		case !name.IsNull() && strings.Contains(item.Summary(), "not unique"):
			rewritten.AddError("Multiple Webhooks Found", fmt.Sprintf("Multiple webhooks found with name %q", name.ValueString()))
		case !name.IsNull() && strings.Contains(item.Summary(), "not found"):
			rewritten.AddError("Webhook Not Found", fmt.Sprintf("No webhook found with name %q", name.ValueString()))
		case item.Summary() == "coralogix_webhook not found":
			rewritten.AddWarning(fmt.Sprintf("coralogix_webhook %q is in state, but no longer exists in Coralogix backend", id.ValueString()),
				fmt.Sprintf("%s will be recreated when you apply", id.ValueString()))
			rewritten.AddError("Error listing Webhooks", "error - 404 Not Found\noperation - Read")
			resp.State.RemoveResource(ctx)
		case item.Summary() == "Unable to list coralogix_webhook":
			rewritten.AddError("Error listing Webhooks", legacyWebhookError("Read", item.Detail()))
		case item.Summary() == "Unable to read coralogix_webhook":
			detail := item.Detail()
			if strings.HasPrefix(detail, "id ") && strings.Contains(detail, ": ") {
				detail = strings.SplitN(detail, ": ", 2)[1]
			}
			rewritten.AddError("Error reading coralogix_webhook", legacyWebhookError("Read", detail))
			if !id.IsNull() && strings.HasPrefix(detail, "500 ") {
				rewritten.AddError("Error listing Webhooks", "error - 500 Internal Server Error\noperation - Read")
			}
		default:
			rewritten.Append(item)
		}
	}
	resp.Diagnostics = rewritten
}

func webhookHooks() generatedwebhook.Hooks {
	return generatedwebhook.Hooks{BeforeWrite: webhookBeforeWrite, AfterFlatten: webhookAfterFlatten, AfterRead: webhookAfterRead, ValidateConfig: webhookValidateConfig}
}

func webhookValidateConfig(ctx context.Context, config tfsdk.Config) diag.Diagnostics {
	var diags diag.Diagnostics
	model := &generatedwebhook.OutgoingWebhookModel{}
	diags.Append(config.GetAttribute(ctx, path.Root("name"), &model.Name)...)
	var object types.Object
	for _, name := range []string{"jira", "pager_duty", "custom"} {
		diags.Append(config.GetAttribute(ctx, path.Root(name), &object)...)
		if diags.HasError() || object.IsNull() || object.IsUnknown() {
			continue
		}
		switch name {
		case "jira":
			model.Jira = &generatedwebhook.JiraConfigModel{}
			diags.Append(object.As(ctx, model.Jira, basetypes.ObjectAsOptions{})...)
		case "pager_duty":
			model.PagerDuty = &generatedwebhook.PagerDutyConfigModel{}
			diags.Append(object.As(ctx, model.PagerDuty, basetypes.ObjectAsOptions{})...)
		case "custom":
			model.GenericWebhook = &generatedwebhook.GenericWebhookConfigModel{}
			diags.Append(object.As(ctx, model.GenericWebhook, basetypes.ObjectAsOptions{})...)
		}
	}
	if diags.HasError() {
		return diags
	}
	diags.Append(webhookCredentialWarnings(model)...)
	if model.GenericWebhook != nil {
		diags.Append(validateWebhookHeaders(model.GenericWebhook)...)
	}
	return diags
}

func validateWebhookHeaders(custom *generatedwebhook.GenericWebhookConfigModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if custom.HeadersWo.IsUnknown() || custom.HeadersWoVersions.IsUnknown() {
		return diags
	}
	names := custom.HeadersWo.Elements()
	versions := custom.HeadersWoVersions.Elements()
	conflicts := conflictingWebhookHeaders(custom)
	if len(conflicts) > 0 {
		diags.AddAttributeError(path.Root("custom").AtName("headers_wo"), "Header set both ways",
			fmt.Sprintf("%s collides with another header name. Keep a secret header in headers_wo only, and give it one spelling: HTTP header names are case-insensitive, so headers and headers_wo must not name the same header even in different case.", strings.Join(conflicts, ", ")))
	}
	for name, secret := range names {
		if secret.IsNull() {
			diags.AddAttributeError(path.Root("custom").AtName("headers_wo"), "Null write-only header value",
				fmt.Sprintf("Header %q has a null value in headers_wo. A write-only header needs a value to send, and a null one would be sent as an empty header. Give it a value, or leave it out of headers_wo along with its version.", name))
		}
	}
	if diags.HasError() {
		return diags
	}
	diags.Append(validateWebhookHeaderVersions(names, versions)...)
	return diags
}

func conflictingWebhookHeaders(custom *generatedwebhook.GenericWebhookConfigModel) []string {
	seen := make(map[string]bool)
	if !custom.Headers.IsNull() && !custom.Headers.IsUnknown() {
		for name := range custom.Headers.Elements() {
			seen[strings.ToLower(name)] = true
		}
	}
	var conflicts []string
	for name := range custom.HeadersWo.Elements() {
		lower := strings.ToLower(name)
		if seen[lower] {
			conflicts = append(conflicts, name)
		}
		seen[lower] = true
	}
	sort.Strings(conflicts)
	return conflicts
}

func validateWebhookHeaderVersions(names map[string]attr.Value, versions map[string]attr.Value) diag.Diagnostics {
	var diags diag.Diagnostics
	for name := range names {
		version, ok := versions[name]
		if !ok {
			diags.AddAttributeError(path.Root("custom").AtName("headers_wo_versions"), "Missing write-only header version",
				fmt.Sprintf("Header %q is set in headers_wo but has no entry in headers_wo_versions. Terraform holds no copy of a write-only value, so the version is what tells it the secret changed.", name))
		} else if version.IsNull() {
			diags.AddAttributeError(path.Root("custom").AtName("headers_wo_versions"), "Null write-only header version",
				fmt.Sprintf("Header %q has a null entry in headers_wo_versions. A null version records nothing, so a later change to the secret alone would never be sent: Terraform holds no copy of the value and would see no reason to update. Give the header a number.", name))
		}
	}
	for name := range versions {
		if _, ok := names[name]; !ok {
			diags.AddAttributeError(path.Root("custom").AtName("headers_wo_versions"), "Version without a write-only header",
				fmt.Sprintf("Header %q has an entry in headers_wo_versions but is not set in headers_wo.", name))
		}
	}
	return diags
}

var webhookCredentialHeaderNames = map[string]bool{
	"authorization": true, "proxy-authorization": true, "x-api-key": true,
	"api-key": true, "x-auth-token": true, "cookie": true,
}

const webhookWriteOnlyAdvice = " Terraform writes it to state. Use %s with %s to send the secret without storing it. Write-only attributes need Terraform 1.11 or later. Terraform shows one warning per distinct message, so check any other webhook with the same name as well."

func webhookWarningSummary(config *generatedwebhook.OutgoingWebhookModel, credential string) string {
	if config.Name.IsNull() || config.Name.IsUnknown() {
		return credential + " is stored in state"
	}
	return fmt.Sprintf("%s for %q is stored in state", credential, config.Name.ValueString())
}

func plainWebhookCredentialHeaderNames(config *generatedwebhook.OutgoingWebhookModel) []string {
	if config == nil || config.GenericWebhook == nil {
		return nil
	}
	headers := config.GenericWebhook.Headers
	if headers.IsNull() || headers.IsUnknown() {
		return nil
	}
	var names []string
	for name := range headers.Elements() {
		if webhookCredentialHeaderNames[strings.ToLower(name)] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func webhookCredentialWarnings(config *generatedwebhook.OutgoingWebhookModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if config == nil {
		return diags
	}
	if config.Jira != nil && !config.Jira.ApiToken.IsNull() && !config.Jira.ApiToken.IsUnknown() {
		diags.AddAttributeWarning(path.Root("jira").AtName("api_token"), webhookWarningSummary(config, "Jira API token"),
			"The token is set through api_token."+fmt.Sprintf(webhookWriteOnlyAdvice, "api_token_wo", "api_token_wo_version"))
	}
	if config.PagerDuty != nil && !config.PagerDuty.ServiceKey.IsNull() && !config.PagerDuty.ServiceKey.IsUnknown() {
		diags.AddAttributeWarning(path.Root("pager_duty").AtName("service_key"), webhookWarningSummary(config, "PagerDuty service key"),
			"The key is set through service_key."+fmt.Sprintf(webhookWriteOnlyAdvice, "service_key_wo", "service_key_wo_version"))
	}
	if names := plainWebhookCredentialHeaderNames(config); len(names) > 0 {
		diags.AddAttributeWarning(path.Root("custom").AtName("headers"), webhookWarningSummary(config, "Credential-carrying webhook header"),
			fmt.Sprintf("%s is set through headers and appears to carry a credential.", strings.Join(names, ", "))+fmt.Sprintf(webhookWriteOnlyAdvice, "headers_wo", "headers_wo_versions"))
	}
	return diags
}

func webhookBeforeWrite(ctx context.Context, config tfsdk.Config, request any) diag.Diagnostics {
	var model generatedwebhook.OutgoingWebhookModel
	diags := config.Get(ctx, &model)
	if diags.HasError() {
		return diags
	}
	_, create := request.(*webhooks.CreateOutgoingWebhookRequest)
	if model.MicrosoftTeams != nil && create {
		diags.AddError("Error expanding webhook type", "Unknown webhook type")
		return diags
	}
	var data *webhooks.OutgoingWebhookInputData
	switch body := request.(type) {
	case *webhooks.CreateOutgoingWebhookRequest:
		data = &body.Data
	case *webhooks.UpdateOutgoingWebhookRequest:
		data = &body.Data
	default:
		diags.AddError("Invalid webhook request", "The generated request has an unknown type.")
		return diags
	}
	if kind := webhookType(data); kind != "" {
		data.Type = &kind
	}
	webhookWriteCollections(data)
	webhookWriteURL(&model, data)
	diags.Append(webhookWriteCustom(ctx, &model, data)...)
	webhookWriteUUIDs(data)
	webhookWriteCredentials(&model, data)
	diags.Append(webhookWriteSlack(ctx, &model, data)...)
	return diags
}

func webhookWriteCollections(data *webhooks.OutgoingWebhookInputData) {
	if data.EmailGroup != nil && data.EmailGroup.EmailAddresses == nil {
		data.EmailGroup.EmailAddresses = []string{}
	}
	if data.Slack != nil && data.Slack.Digests == nil {
		data.Slack.Digests = []webhooks.Digest{}
	}
}

func webhookWriteURL(model *generatedwebhook.OutgoingWebhookModel, data *webhooks.OutgoingWebhookInputData) {
	for _, url := range []types.String{
		webhookURL(model.GenericWebhook), webhookURL(model.Slack), webhookURL(model.Jira),
		webhookURL(model.MsTeamsWorkflow), webhookURL(model.Opsgenie), webhookURL(model.Demisto), webhookURL(model.SendLog),
	} {
		if !url.IsNull() && !url.IsUnknown() {
			data.Url = url.ValueStringPointer()
			break
		}
	}
}

func webhookWriteCustom(ctx context.Context, model *generatedwebhook.OutgoingWebhookModel, data *webhooks.OutgoingWebhookInputData) diag.Diagnostics {
	var diags diag.Diagnostics
	if data.GenericWebhook != nil {
		if model.GenericWebhook != nil && !model.GenericWebhook.HeadersWo.IsNull() {
			var headers map[string]string
			diags.Append(model.GenericWebhook.HeadersWo.ElementsAs(ctx, &headers, false)...)
			if data.GenericWebhook.Headers == nil {
				data.GenericWebhook.Headers = make(map[string]string)
			}
			for name, value := range headers {
				data.GenericWebhook.Headers[name] = value
			}
		}
	}
	return diags
}

func webhookWriteUUIDs(data *webhooks.OutgoingWebhookInputData) {
	if data.GenericWebhook != nil && data.GenericWebhook.Uuid == nil {
		uuid := utils.UuidCreateIfNull(types.StringNull())
		data.GenericWebhook.Uuid = &uuid
	}
	if data.SendLog != nil && data.SendLog.Uuid == nil {
		uuid := utils.UuidCreateIfNull(types.StringNull())
		data.SendLog.Uuid = &uuid
	}
	if data.Demisto != nil && data.Demisto.Uuid == nil {
		uuid := utils.UuidCreateIfNull(types.StringNull())
		data.Demisto.Uuid = &uuid
	}
}

func webhookWriteCredentials(model *generatedwebhook.OutgoingWebhookModel, data *webhooks.OutgoingWebhookInputData) {
	if data.Jira != nil && model.Jira != nil && !model.Jira.ApiTokenWo.IsNull() {
		data.Jira.ApiToken = model.Jira.ApiTokenWo.ValueString()
	}
	if data.PagerDuty != nil && model.PagerDuty != nil && !model.PagerDuty.ServiceKeyWo.IsNull() {
		data.PagerDuty.ServiceKey = model.PagerDuty.ServiceKeyWo.ValueString()
	}
}

func webhookWriteSlack(ctx context.Context, model *generatedwebhook.OutgoingWebhookModel, data *webhooks.OutgoingWebhookInputData) diag.Diagnostics {
	var diags diag.Diagnostics
	if data.Slack != nil && model.Slack != nil && !model.Slack.NotifyOn.IsNull() {
		var names []string
		diags.Append(model.Slack.NotifyOn.ElementsAs(ctx, &names, false)...)
		for _, name := range names {
			active := true
			data.Slack.Digests = append(data.Slack.Digests, webhooks.Digest{Type: webhooks.DigestType(strings.ToUpper(name)), IsActive: &active})
		}
	}
	return diags
}

// The API marks type read-only, but the released provider sends it on writes.
func webhookType(data *webhooks.OutgoingWebhookInputData) webhooks.WebhookType {
	switch {
	case data.GenericWebhook != nil:
		return webhooks.WEBHOOKTYPE_GENERIC
	case data.Slack != nil:
		return webhooks.WEBHOOKTYPE_SLACK
	case data.PagerDuty != nil:
		return webhooks.WEBHOOKTYPE_PAGERDUTY
	case data.SendLog != nil:
		return webhooks.WEBHOOKTYPE_SEND_LOG
	case data.EmailGroup != nil:
		return webhooks.WEBHOOKTYPE_EMAIL_GROUP
	case data.MsTeamsWorkflow != nil:
		return webhooks.WEBHOOKTYPE_MS_TEAMS_WORKFLOW
	case data.Jira != nil:
		return webhooks.WEBHOOKTYPE_JIRA
	case data.Opsgenie != nil:
		return webhooks.WEBHOOKTYPE_OPSGENIE
	case data.Demisto != nil:
		return webhooks.WEBHOOKTYPE_DEMISTO
	case data.AwsEventBridge != nil:
		return webhooks.WEBHOOKTYPE_AWS_EVENT_BRIDGE
	}
	return ""
}

func webhookURL(m any) types.String {
	switch v := m.(type) {
	case *generatedwebhook.GenericWebhookConfigModel:
		if v != nil {
			return v.Url
		}
	case *generatedwebhook.SlackConfigModel:
		if v != nil {
			return v.Url
		}
	case *generatedwebhook.JiraConfigModel:
		if v != nil {
			return v.Url
		}
	case *generatedwebhook.MSTeamsWorkflowConfigModel:
		if v != nil {
			return v.Url
		}
	case *generatedwebhook.OpsgenieConfigModel:
		if v != nil {
			return v.Url
		}
	case *generatedwebhook.DemistoConfigModel:
		if v != nil {
			return v.Url
		}
	case *generatedwebhook.SendLogConfigModel:
		if v != nil {
			return v.Url
		}
	}
	return types.StringNull()
}

func webhookAfterFlatten(ctx context.Context, model *generatedwebhook.OutgoingWebhookModel, value *webhooks.OutgoingWebhook) diag.Diagnostics {
	var diags diag.Diagnostics
	model.ExternalId = types.StringValue(strconv.FormatInt(value.ExternalId, 10))
	url := types.StringPointerValue(value.Url)
	if model.GenericWebhook != nil {
		model.GenericWebhook.Url = url
	}
	if model.Slack != nil {
		model.Slack.Url = url
		if len(value.Slack.Digests) > 0 {
			var names []string
			for _, digest := range value.Slack.Digests {
				if digest.GetIsActive() {
					names = append(names, strings.ToLower(string(digest.Type)))
				}
			}
			model.Slack.NotifyOn, diags = types.SetValueFrom(ctx, types.StringType, names)
		}
	}
	if model.Jira != nil {
		model.Jira.Url = url
	}
	if model.MsTeamsWorkflow != nil {
		model.MsTeamsWorkflow.Url = url
	}
	if model.MicrosoftTeams != nil {
		model.MicrosoftTeams.Url = url
	}
	if model.Opsgenie != nil {
		model.Opsgenie.Url = url
	}
	if model.Demisto != nil {
		model.Demisto.Url = url
	}
	if model.SendLog != nil {
		model.SendLog.Url = url
	}
	return diags
}

func webhookAfterRead(ctx context.Context, state *tfsdk.State, prior any) diag.Diagnostics {
	var current, previous generatedwebhook.OutgoingWebhookModel
	diags := state.Get(ctx, &current)
	if diags.HasError() {
		return diags
	}
	switch p := prior.(type) {
	case tfsdk.Plan:
		diags.Append(p.Get(ctx, &previous)...)
	case tfsdk.State:
		diags.Append(p.Get(ctx, &previous)...)
	default:
		return diags
	}
	if diags.HasError() {
		return diags
	}
	webhookKeepCredentialVersions(&current, &previous)
	diags.Append(webhookStripWriteOnlyHeaders(ctx, &current, &previous)...)
	if !diags.HasError() {
		diags.Append(state.Set(ctx, &current)...)
	}
	return diags
}

func webhookKeepCredentialVersions(current, previous *generatedwebhook.OutgoingWebhookModel) {
	if current.Jira != nil && previous.Jira != nil && !previous.Jira.ApiTokenWoVersion.IsNull() {
		current.Jira.ApiToken = types.StringNull()
		current.Jira.ApiTokenWoVersion = previous.Jira.ApiTokenWoVersion
	}
	if current.PagerDuty != nil && previous.PagerDuty != nil && !previous.PagerDuty.ServiceKeyWoVersion.IsNull() {
		current.PagerDuty.ServiceKey = types.StringNull()
		current.PagerDuty.ServiceKeyWoVersion = previous.PagerDuty.ServiceKeyWoVersion
	}
}

func webhookStripWriteOnlyHeaders(ctx context.Context, current, previous *generatedwebhook.OutgoingWebhookModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if current.GenericWebhook != nil && previous.GenericWebhook != nil && !previous.GenericWebhook.HeadersWoVersions.IsNull() {
		var headers map[string]string
		diags.Append(current.GenericWebhook.Headers.ElementsAs(ctx, &headers, false)...)
		for name := range previous.GenericWebhook.HeadersWoVersions.Elements() {
			delete(headers, name)
		}
		var mapDiags diag.Diagnostics
		current.GenericWebhook.Headers, mapDiags = types.MapValueFrom(ctx, types.StringType, headers)
		diags.Append(mapDiags...)
		current.GenericWebhook.HeadersWoVersions = previous.GenericWebhook.HeadersWoVersions
	}
	return diags
}
