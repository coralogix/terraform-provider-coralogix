// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package integrations

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	webhooks "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/outgoing_webhooks_service"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_                           resource.ResourceWithConfigure   = &WebhookResource{}
	_                           resource.ResourceWithImportState = &WebhookResource{}
	webhooksSchemaToProtoMethod                                  = map[string]webhooks.MethodType{
		"get":  webhooks.METHODTYPE_GET,
		"post": webhooks.METHODTYPE_POST,
		"put":  webhooks.METHODTYPE_PUT,
	}
	webhooksProtoToSchemaMethod                = utils.ReverseMap(webhooksSchemaToProtoMethod)
	webhooksValidMethods                       = utils.GetKeys(webhooksSchemaToProtoMethod)
	webhooksSchemaToProtoSlackConfigDigestType = map[string]webhooks.DigestType{
		"error_and_critical_logs": webhooks.DIGESTTYPE_ERROR_AND_CRITICAL_LOGS,
		"flow_anomalies":          webhooks.DIGESTTYPE_FLOW_ANOMALIES,
		"spike_anomalies":         webhooks.DIGESTTYPE_SPIKE_ANOMALIES,
		"data_usage":              webhooks.DIGESTTYPE_DATA_USAGE,
	}
	webhooksProtoToSchemaSlackConfigDigestType = utils.ReverseMap(webhooksSchemaToProtoSlackConfigDigestType)
	webhooksValidSlackConfigDigestTypes        = utils.GetKeys(webhooksSchemaToProtoSlackConfigDigestType)
	webhooksProtoToSchemaSlackAttachmentType   = map[string]webhooks.AttachmentType{
		"empty":           webhooks.ATTACHMENTTYPE_EMPTY,
		"metric_snapshot": webhooks.ATTACHMENTTYPE_METRIC_SNAPSHOT,
		"logs":            webhooks.ATTACHMENTTYPE_LOGS,
	}
	webhooksSchemaToProtoSlackAttachmentType = utils.ReverseMap(webhooksProtoToSchemaSlackAttachmentType)
	webhooksValidSlackAttachmentTypes        = utils.GetKeys(webhooksProtoToSchemaSlackAttachmentType)
	customDefaultPayload                     = `{
    "uuid": "",
    "alert_id": "$ALERT_ID",
    "name": "$ALERT_NAME",
    "description": "$ALERT_DESCRIPTION",
    "threshold": "$ALERT_THRESHOLD",
    "timewindow": "$ALERT_TIMEWINDOW_MINUTES",
    "group_by_labels": "$ALERT_GROUPBY_LABELS",
    "alert_Webhook": "$ALERT_Webhook",
    "alert_url": "$ALERT_URL",
    "log_url": "$LOG_URL",
    "icon_url": "$CORALOGIX_ICON_URL",
    "service": "$SERVICE",
    "duration": "$DURATION",
    "errors": "$ERRORS",
    "spans": "$SPANS",
    "fields": [
      {
        "key": "team",
        "value": "$TEAM_NAME"
      },
      {
        "key": "application",
        "value": "$APPLICATION_NAME"
      },
      {
        "key": "subsystem",
        "value": "$SUBSYSTEM_NAME"
      },
      {
        "key": "severity",
        "value": "$EVENT_SEVERITY"
      },
      {
        "key": "computer",
        "value": "$COMPUTER_NAME"
      },
      {
        "key": "ipAddress",
        "value": "$IP_ADDRESS"
      },
      {
        "key": "timestamp",
        "value": "$EVENT_TIMESTAMP"
      },
      {
        "key": "hitCount",
        "value": "$HIT_COUNT"
      },
      {
        "key": "text",
        "value": "$LOG_TEXT"
      },
      {
        "key": "Custom field",
        "value": "$JSON_KEY"
      },
      {
        "key": "Group-by Key1",
        "value": "$GROUP_BY_KEY_1"
      },
      {
        "key": "Group-by Value1",
        "value": "$GROUP_BY_VALUE_1"
      },
      {
        "key": "Group-by Key2",
        "value": "$GROUP_BY_KEY_2"
      },
      {
        "key": "Group-by Value2",
        "value": "$GROUP_BY_VALUE_2"
      },
      {
        "key": "metricKey",
        "value": "$METRIC_KEY"
      },
      {
        "key": "metricOperator",
        "value": "$METRIC_OPERATOR"
      },
      {
        "key": "timeframe",
        "value": "$TIMEFRAME"
      },
      {
        "key": "timeframePercentageOverThreshold",
        "value": "$TIMEFRAME_OVER_THRESHOLD"
      },
      {
        "key": "metricCriteria",
        "value": "$METRIC_CRITERIA"
      },
      {
        "key": "ratioQueryOne",
        "value": "$RATIO_QUERY_ONE"
      },
      {
        "key": "ratioQueryTwo",
        "value": "$RATIO_QUERY_TWO"
      },
      {
        "key": "ratioTimeframe",
        "value": "$RATIO_TIMEFRAME"
      },
      {
        "key": "ratioGroupByKeys",
        "value": "$RATIO_GROUP_BY_KEYS"
      },
      {
        "key": "ratioGroupByTable",
        "value": "$RATIO_GROUP_BY_TABLE"
      },
      {
        "key": "uniqueCountValuesList",
        "value": "$UNIQUE_COUNT_VALUES_LIST"
      },
      {
        "key": "newValueTrackedKey",
        "value": "$NEW_VALUE_TRACKED_KEY"
      },
      {
        "key": "metaLabels",
        "value": "$META_LABELS"
      }
    ]
  }`
	sendLockDefaultPayload = `{
    "applicationName": "$APPLICATION_NAME",
    "subsystemName": "$SUBSYSTEM_NAME",
    "computerName": "$COMPUTER_NAME",
    "logEntries": [
      {
        "severity": 3,
        "timestamp": "$EVENT_TIMESTAMP_MS",
        "text": {
          "integration_text": "Insert your desired integration description",
          "alert_severity": "$EVENT_SEVERITY",
          "alert_id": "$ALERT_ID",
          "alert_name": "$ALERT_NAME",
          "alert_url": "$ALERT_URL",
          "hit_count": "$HIT_COUNT"
        }
      }
    ]
  }`
	demistoDefaultPayload = `{
    "applicationName": "Coralogix Alerts",
    "subsystemName": "Coralogix Alerts",
    "computerName": "$COMPUTER_NAME",
    "logEntries": [
      {
        "severity": 3,
        "timestamp": "$EVENT_TIMESTAMP_MS",
        "text": {
          "integration_text": "Security Incident",
          "alert_application": "$APPLICATION_NAME",
          "alert_subsystem": "$SUBSYSTEM_NAME",
          "alert_severity": "$EVENT_SEVERITY",
          "alert_id": "$ALERT_ID",
          "alert_name": "$ALERT_NAME",
          "alert_url": "$ALERT_URL",
          "hit_count": "$HIT_COUNT",
          "alert_type_id": "53d222e2-e7b2-4fa6-80d4-9935425d47dd"
        }
      }
    ]
  }`
)

func NewWebhookResource() resource.Resource {
	return &WebhookResource{}
}

type WebhookResource struct {
	client *webhooks.OutgoingWebhooksServiceAPIService
}

type WebhookResourceModel struct {
	ID              types.String          `tfsdk:"id"`
	ExternalID      types.String          `tfsdk:"external_id"`
	Name            types.String          `tfsdk:"name"`
	CustomWebhook   *CustomWebhookModel   `tfsdk:"custom"`
	Slack           *SlackModel           `tfsdk:"slack"`
	PagerDuty       *PagerDutyModel       `tfsdk:"pager_duty"`
	SendLog         *SendLogModel         `tfsdk:"sendlog"`
	EmailGroup      *EmailGroupModel      `tfsdk:"email_group"`
	MsTeamsWorkflow *MsTeamsWorkflowModel `tfsdk:"microsoft_teams_workflow"`
	MsTeams         *MsTeamsWorkflowModel `tfsdk:"microsoft_teams"`
	Jira            *JiraModel            `tfsdk:"jira"`
	Opsgenie        *OpsgenieModel        `tfsdk:"opsgenie"`
	Demisto         *DemistoModel         `tfsdk:"demisto"`
	EventBridge     *EventBridgeModel     `tfsdk:"event_bridge"`
}

type CustomWebhookModel struct {
	UUID    types.String `tfsdk:"uuid"`
	Method  types.String `tfsdk:"method"`
	Headers types.Map    `tfsdk:"headers"`
	Payload types.String `tfsdk:"payload"`
	URL     types.String `tfsdk:"url"`
	// The versions map is keyed by header name, so state records which headers
	// are write-only. Read has no config to consult, and needs that to know
	// which values the API echoed back must be kept out of state.
	HeadersWO         types.Map `tfsdk:"headers_wo"`
	HeadersWOVersions types.Map `tfsdk:"headers_wo_versions"`
}

type SlackModel struct {
	NotifyAbout types.Set    `tfsdk:"notify_on"` //types.String
	URL         types.String `tfsdk:"url"`
	Attachments types.List   `tfsdk:"attachments"` //SlackAttachmentModel
}

type SlackAttachmentModel struct {
	Type   types.String `tfsdk:"type"`
	Active types.Bool   `tfsdk:"active"`
}

type PagerDutyModel struct {
	ServiceKey          types.String `tfsdk:"service_key"`
	ServiceKeyWO        types.String `tfsdk:"service_key_wo"`
	ServiceKeyWOVersion types.Int64  `tfsdk:"service_key_wo_version"`
}

type SendLogModel struct {
	UUID    types.String `tfsdk:"uuid"`
	Payload types.String `tfsdk:"payload"`
	URL     types.String `tfsdk:"url"`
}

type EmailGroupModel struct {
	Emails types.List `tfsdk:"emails"` //types.String
}

type MsTeamsWorkflowModel struct {
	URL types.String `tfsdk:"url"`
}

type JiraModel struct {
	ApiKey            types.String `tfsdk:"api_token"`
	Email             types.String `tfsdk:"email"`
	ProjectID         types.String `tfsdk:"project_key"`
	URL               types.String `tfsdk:"url"`
	ApiTokenWO        types.String `tfsdk:"api_token_wo"`
	ApiTokenWOVersion types.Int64  `tfsdk:"api_token_wo_version"`
}

type OpsgenieModel struct {
	URL types.String `tfsdk:"url"`
}

type DemistoModel struct {
	UUID    types.String `tfsdk:"uuid"`
	Payload types.String `tfsdk:"payload"`
	URL     types.String `tfsdk:"url"`
}

type EventBridgeModel struct {
	EventBusARN types.String `tfsdk:"event_bus_arn"`
	Detail      types.String `tfsdk:"detail"`
	DetailType  types.String `tfsdk:"detail_type"`
	Source      types.String `tfsdk:"source"`
	RoleName    types.String `tfsdk:"role_name"`
}

// webhookCredentialWarnings reports credentials given through an ordinary
// attribute, which puts them in state, and names the write-only alternative.
func webhookCredentialWarnings(config *WebhookResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if config == nil {
		return diags
	}

	if jira := config.Jira; jira != nil && !jira.ApiKey.IsNull() && !jira.ApiKey.IsUnknown() {
		diags.AddAttributeWarning(
			path.Root("jira").AtName("api_token"),
			"Jira API token is stored in state",
			"The Jira API token is set through api_token."+fmt.Sprintf(webhookWriteOnlyAdvice, "api_token_wo", "api_token_wo_version"),
		)
	}
	if pagerDuty := config.PagerDuty; pagerDuty != nil && !pagerDuty.ServiceKey.IsNull() && !pagerDuty.ServiceKey.IsUnknown() {
		diags.AddAttributeWarning(
			path.Root("pager_duty").AtName("service_key"),
			"PagerDuty service key is stored in state",
			"The PagerDuty service key is set through service_key."+fmt.Sprintf(webhookWriteOnlyAdvice, "service_key_wo", "service_key_wo_version"),
		)
	}
	if names := plainWebhookCredentialHeaderNames(config); len(names) > 0 {
		diags.AddAttributeWarning(
			path.Root("custom").AtName("headers"),
			"Webhook header carrying a credential is stored in state",
			fmt.Sprintf("%s appears to carry a credential and is set through headers.", strings.Join(names, ", "))+
				fmt.Sprintf(webhookWriteOnlyAdvice, "headers_wo", "headers_wo_versions"),
		)
	}
	return diags
}

func plainWebhookCredentialHeaderNames(config *WebhookResourceModel) []string {
	if config.CustomWebhook == nil {
		return nil
	}
	headers := config.CustomWebhook.Headers
	if headers.IsNull() || headers.IsUnknown() {
		return nil
	}
	var names []string
	for name := range headers.Elements() {
		if _, ok := webhookCredentialHeaderNames[strings.ToLower(name)]; ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func (r *WebhookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)

	// Only the ID is known here, so the warning cannot say whether this
	// particular webhook carries a credential. It is worth saying anyway: the
	// import is the point at which the value lands in state, and by the time a
	// plan shows it the secret has already been written.
	resp.Diagnostics.AddWarning(
		"An imported credential is written to state",
		"Importing reads the webhook's current configuration from Coralogix, including any credential it carries, and writes it to state. "+
			"Treat an imported credential as exposed and rotate it.\n\n"+
			"To keep it out of state from then on, move the value into the matching write-only attribute "+
			"(jira.api_token_wo, pager_duty.service_key_wo, or custom.headers_wo), set its version, and apply. "+
			"That apply removes the imported value from state. Write-only attributes need Terraform 1.11 or later.",
	)
}

func (r *WebhookResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	clientSet, ok := req.ProviderData.(*clientset.ClientSet)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *clientset.ClientSet, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = clientSet.Webhooks()
}

func (r *WebhookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook"
}

func (r *WebhookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 0,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Webhook ID.",
			},
			"external_id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				MarkdownDescription: "Webhook external ID. Using to linq webhook to alert.",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Webhook name.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					stringvalidator.RegexMatches(
						regexp.MustCompile(`\S`),
						"must not be empty or contain only whitespace",
					),
				},
			},
			"custom": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"uuid": schema.StringAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Webhook UUID. Computed automatically.",
					},
					"method": schema.StringAttribute{
						Optional: true,
						Validators: []validator.String{
							stringvalidator.OneOf(webhooksValidMethods...),
						},
						MarkdownDescription: fmt.Sprintf("Webhook method. can be one of: %s", strings.Join(webhooksValidMethods, ", ")),
					},
					"headers": schema.MapAttribute{
						Optional:            true,
						Computed:            true,
						ElementType:         types.StringType,
						MarkdownDescription: "Webhook headers. Map of string to string. A header carrying a secret belongs in `headers_wo` instead, so its value is never written to state.",
					},
					"headers_wo": schema.MapAttribute{
						Optional:    true,
						WriteOnly:   true,
						ElementType: types.StringType,
						Validators: []validator.Map{
							mapvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName("headers_wo_versions")),
						},
						MarkdownDescription: "Webhook headers whose values are secret, keyed by header name. Values are sent to Coralogix and never written to state. Every key needs a matching entry in `headers_wo_versions`, and a key here must not also appear in `headers`. Terraform never stores a write-only value, so it cannot detect that the secret changed: increment `headers_wo_versions` to send a rotated one. Requires Terraform 1.11 or later. Importing brings the value into state, because an import has neither configuration nor prior state to say the value is managed this way; one apply afterwards removes it again, so treat an imported secret as exposed and rotate it. Reading the same webhook through `data.coralogix_webhook` does return this value, because a data source reads from the API and has no configuration telling it which value is managed write-only.",
					},
					"headers_wo_versions": schema.MapAttribute{
						Optional:    true,
						ElementType: types.Int64Type,
						Validators: []validator.Map{
							mapvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName("headers_wo")),
						},
						MarkdownDescription: "Version of each `headers_wo` value, keyed by header name. Increment a header's version to send a rotated secret.",
					},
					"payload": schema.StringAttribute{
						Optional:            true,
						Computed:            true,
						Default:             stringdefault.StaticString(customDefaultPayload),
						MarkdownDescription: "Webhook payload. JSON string.",
					},
					"url": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Webhook URL.",
					},
				},
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(
						path.MatchRelative().AtParent().AtName("slack"),
						path.MatchRelative().AtParent().AtName("pager_duty"),
						path.MatchRelative().AtParent().AtName("sendlog"),
						path.MatchRelative().AtParent().AtName("email_group"),
						path.MatchRelative().AtParent().AtName("microsoft_teams"),
						path.MatchRelative().AtParent().AtName("microsoft_teams_workflow"),
						path.MatchRelative().AtParent().AtName("jira"),
						path.MatchRelative().AtParent().AtName("opsgenie"),
						path.MatchRelative().AtParent().AtName("demisto"),
						path.MatchRelative().AtParent().AtName("event_bridge"),
					),
				},
				Optional:            true,
				MarkdownDescription: "Generic webhook.",
			},
			"slack": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"notify_on": schema.SetAttribute{
						Optional:    true,
						ElementType: types.StringType,
						Validators: []validator.Set{
							setvalidator.ValueStringsAre(stringvalidator.OneOf(webhooksValidSlackConfigDigestTypes...)),
						},
						MarkdownDescription: fmt.Sprintf("Slack notifications. can be one of: %s", strings.Join(webhooksValidSlackConfigDigestTypes, ", ")),
					},
					"url": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Slack URL.",
					},
					"attachments": schema.ListNestedAttribute{
						Optional: true,
						Computed: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"type": schema.StringAttribute{
									Required: true,
									Validators: []validator.String{
										stringvalidator.OneOf(webhooksValidSlackAttachmentTypes...),
									},
									MarkdownDescription: fmt.Sprintf("Slack attachment type. can be one of: %s", strings.Join(webhooksValidSlackAttachmentTypes, ", ")),
								},
								"active": schema.BoolAttribute{
									Optional:            true,
									Computed:            true,
									Default:             booldefault.StaticBool(true),
									MarkdownDescription: "Determines if the attachment is active. Default is true.",
								},
							},
						},
						MarkdownDescription: "Slack attachments.",
					},
				},
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(
						path.MatchRelative().AtParent().AtName("custom"),
						path.MatchRelative().AtParent().AtName("pager_duty"),
						path.MatchRelative().AtParent().AtName("sendlog"),
						path.MatchRelative().AtParent().AtName("email_group"),
						path.MatchRelative().AtParent().AtName("microsoft_teams"),
						path.MatchRelative().AtParent().AtName("microsoft_teams_workflow"),
						path.MatchRelative().AtParent().AtName("jira"),
						path.MatchRelative().AtParent().AtName("opsgenie"),
						path.MatchRelative().AtParent().AtName("demisto"),
						path.MatchRelative().AtParent().AtName("event_bridge"),
					),
				},
				Optional:            true,
				MarkdownDescription: "Slack webhook.",
			},
			"pager_duty": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"service_key": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "PagerDuty service key. Use `service_key_wo` instead to keep it out of state.",
					},
					"service_key_wo": schema.StringAttribute{
						Optional:  true,
						WriteOnly: true,
						Validators: []validator.String{
							stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("service_key")),
							stringvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName("service_key_wo_version")),
						},
						MarkdownDescription: "PagerDuty service key, sent to Coralogix and never written to state. Terraform never stores a write-only value, so it cannot detect that the secret changed: increment `service_key_wo_version` to send a rotated one. Requires Terraform 1.11 or later. Importing brings the value into state, because an import has neither configuration nor prior state to say the value is managed this way; one apply afterwards removes it again, so treat an imported secret as exposed and rotate it. Reading the same webhook through `data.coralogix_webhook` does return this value, because a data source reads from the API and has no configuration telling it which value is managed write-only.",
					},
					"service_key_wo_version": schema.Int64Attribute{
						Optional: true,
						Validators: []validator.Int64{
							int64validator.AlsoRequires(path.MatchRelative().AtParent().AtName("service_key_wo")),
						},
						MarkdownDescription: "Version of `service_key_wo`. Increment it to send a rotated service key.",
					},
				},
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(
						path.MatchRelative().AtParent().AtName("custom"),
						path.MatchRelative().AtParent().AtName("slack"),
						path.MatchRelative().AtParent().AtName("sendlog"),
						path.MatchRelative().AtParent().AtName("email_group"),
						path.MatchRelative().AtParent().AtName("microsoft_teams"),
						path.MatchRelative().AtParent().AtName("microsoft_teams_workflow"),
						path.MatchRelative().AtParent().AtName("jira"),
						path.MatchRelative().AtParent().AtName("opsgenie"),
						path.MatchRelative().AtParent().AtName("demisto"),
						path.MatchRelative().AtParent().AtName("event_bridge"),
					),
				},
				Optional:            true,
				MarkdownDescription: "PagerDuty webhook.",
			},
			"sendlog": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"uuid": schema.StringAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Webhook UUID. Computed automatically.",
					},
					"url": schema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Webhook URL returned by the service when present. SendLog webhooks do not support configuring a URL.",
					},
					"payload": schema.StringAttribute{
						Optional:            true,
						Computed:            true,
						Default:             stringdefault.StaticString(sendLockDefaultPayload),
						MarkdownDescription: "Webhook payload. JSON string.",
					},
				},
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(
						path.MatchRelative().AtParent().AtName("custom"),
						path.MatchRelative().AtParent().AtName("slack"),
						path.MatchRelative().AtParent().AtName("pager_duty"),
						path.MatchRelative().AtParent().AtName("email_group"),
						path.MatchRelative().AtParent().AtName("microsoft_teams"),
						path.MatchRelative().AtParent().AtName("microsoft_teams_workflow"),
						path.MatchRelative().AtParent().AtName("jira"),
						path.MatchRelative().AtParent().AtName("opsgenie"),
						path.MatchRelative().AtParent().AtName("demisto"),
						path.MatchRelative().AtParent().AtName("event_bridge"),
					),
				},
				Optional:            true,
				MarkdownDescription: "Send log webhook.",
			},
			"email_group": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"emails": schema.ListAttribute{
						Optional:            true,
						Computed:            true,
						ElementType:         types.StringType,
						MarkdownDescription: "Emails list.",
					},
				},
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(
						path.MatchRelative().AtParent().AtName("custom"),
						path.MatchRelative().AtParent().AtName("slack"),
						path.MatchRelative().AtParent().AtName("pager_duty"),
						path.MatchRelative().AtParent().AtName("sendlog"),
						path.MatchRelative().AtParent().AtName("microsoft_teams"),
						path.MatchRelative().AtParent().AtName("microsoft_teams_workflow"),
						path.MatchRelative().AtParent().AtName("jira"),
						path.MatchRelative().AtParent().AtName("opsgenie"),
						path.MatchRelative().AtParent().AtName("demisto"),
						path.MatchRelative().AtParent().AtName("event_bridge"),
					),
				},
				Optional:            true,
				MarkdownDescription: "Email group webhook.",
			},
			"microsoft_teams_workflow": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"url": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Microsoft Teams Workflow URL.",
					},
				},
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(
						path.MatchRelative().AtParent().AtName("custom"),
						path.MatchRelative().AtParent().AtName("slack"),
						path.MatchRelative().AtParent().AtName("pager_duty"),
						path.MatchRelative().AtParent().AtName("sendlog"),
						path.MatchRelative().AtParent().AtName("email_group"),
						path.MatchRelative().AtParent().AtName("microsoft_teams"),
						path.MatchRelative().AtParent().AtName("jira"),
						path.MatchRelative().AtParent().AtName("opsgenie"),
						path.MatchRelative().AtParent().AtName("demisto"),
						path.MatchRelative().AtParent().AtName("event_bridge"),
					),
				},
				Optional:            true,
				MarkdownDescription: "Microsoft Teams Workflow webhook.",
			},
			"microsoft_teams": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"url": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Microsoft Teams URL.",
					},
				},
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(
						path.MatchRelative().AtParent().AtName("custom"),
						path.MatchRelative().AtParent().AtName("slack"),
						path.MatchRelative().AtParent().AtName("pager_duty"),
						path.MatchRelative().AtParent().AtName("sendlog"),
						path.MatchRelative().AtParent().AtName("email_group"),
						path.MatchRelative().AtParent().AtName("microsoft_teams_workflow"),
						path.MatchRelative().AtParent().AtName("jira"),
						path.MatchRelative().AtParent().AtName("opsgenie"),
						path.MatchRelative().AtParent().AtName("demisto"),
						path.MatchRelative().AtParent().AtName("event_bridge"),
					),
				},
				Optional:            true,
				MarkdownDescription: "Microsoft Teams webhook. (Deprecated, please use microsoft_teams_workflow)",
				DeprecationMessage:  "Please use microsoft_teams_workflow",
			},

			"jira": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"api_token": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Jira API token. Use `api_token_wo` instead to keep it out of state.",
					},
					"api_token_wo": schema.StringAttribute{
						Optional:  true,
						WriteOnly: true,
						Validators: []validator.String{
							stringvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("api_token")),
							stringvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName("api_token_wo_version")),
						},
						MarkdownDescription: "Jira API token, sent to Coralogix and never written to state. Terraform never stores a write-only value, so it cannot detect that the secret changed: increment `api_token_wo_version` to send a rotated one. Requires Terraform 1.11 or later. Importing brings the value into state, because an import has neither configuration nor prior state to say the value is managed this way; one apply afterwards removes it again, so treat an imported secret as exposed and rotate it. Reading the same webhook through `data.coralogix_webhook` does return this value, because a data source reads from the API and has no configuration telling it which value is managed write-only.",
					},
					"api_token_wo_version": schema.Int64Attribute{
						Optional: true,
						Validators: []validator.Int64{
							int64validator.AlsoRequires(path.MatchRelative().AtParent().AtName("api_token_wo")),
						},
						MarkdownDescription: "Version of `api_token_wo`. Increment it to send a rotated API token.",
					},
					"email": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "email.",
					},
					"project_key": schema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Jira project key.",
					},
					"url": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Jira URL.",
					},
				},
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(
						path.MatchRelative().AtParent().AtName("custom"),
						path.MatchRelative().AtParent().AtName("slack"),
						path.MatchRelative().AtParent().AtName("pager_duty"),
						path.MatchRelative().AtParent().AtName("sendlog"),
						path.MatchRelative().AtParent().AtName("email_group"),
						path.MatchRelative().AtParent().AtName("microsoft_teams"),
						path.MatchRelative().AtParent().AtName("microsoft_teams_workflow"),
						path.MatchRelative().AtParent().AtName("opsgenie"),
						path.MatchRelative().AtParent().AtName("demisto"),
						path.MatchRelative().AtParent().AtName("event_bridge"),
					),
				},
				Optional:            true,
				MarkdownDescription: "Jira webhook.",
			},
			"opsgenie": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"url": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Opsgenie URL.",
					},
				},
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(
						path.MatchRelative().AtParent().AtName("custom"),
						path.MatchRelative().AtParent().AtName("slack"),
						path.MatchRelative().AtParent().AtName("pager_duty"),
						path.MatchRelative().AtParent().AtName("sendlog"),
						path.MatchRelative().AtParent().AtName("email_group"),
						path.MatchRelative().AtParent().AtName("microsoft_teams"),
						path.MatchRelative().AtParent().AtName("microsoft_teams_workflow"),
						path.MatchRelative().AtParent().AtName("jira"),
						path.MatchRelative().AtParent().AtName("demisto"),
						path.MatchRelative().AtParent().AtName("event_bridge"),
					),
				},
				Optional:            true,
				MarkdownDescription: "Opsgenie webhook.",
			},
			"demisto": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"uuid": schema.StringAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Webhook UUID. Computed automatically.",
					},
					"payload": schema.StringAttribute{
						Optional:            true,
						Computed:            true,
						Default:             stringdefault.StaticString(demistoDefaultPayload),
						MarkdownDescription: "Webhook payload. JSON string.",
					},
					"url": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Microsoft Teams URL.",
					},
				},
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(
						path.MatchRelative().AtParent().AtName("custom"),
						path.MatchRelative().AtParent().AtName("slack"),
						path.MatchRelative().AtParent().AtName("pager_duty"),
						path.MatchRelative().AtParent().AtName("sendlog"),
						path.MatchRelative().AtParent().AtName("email_group"),
						path.MatchRelative().AtParent().AtName("microsoft_teams"),
						path.MatchRelative().AtParent().AtName("microsoft_teams_workflow"),
						path.MatchRelative().AtParent().AtName("jira"),
						path.MatchRelative().AtParent().AtName("opsgenie"),
						path.MatchRelative().AtParent().AtName("event_bridge"),
					),
				},
				Optional:            true,
				MarkdownDescription: "Demisto webhook.",
			},
			"event_bridge": schema.SingleNestedAttribute{
				Attributes: map[string]schema.Attribute{
					"event_bus_arn": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Corresponds to the event bus, which will receive notifications. The policy attached must contain permission to publish.",
					},
					"detail": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Event bridge message. JSON string. More details about the token [\"here\"](https://coralogix.com/docs/user-guides/alerting/outbound-webhooks/generic-outbound-webhooks-alert-webhooks/#placeholders)",
					},
					"detail_type": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Free text to be included in the event.",
					},
					"source": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Free text is used to identify the messages Coralogix sends.",
					},
					"role_name": schema.StringAttribute{
						Required:            true,
						MarkdownDescription: "Corresponds to the AWS IAM role that will be created in your account.",
					},
				},
				Optional: true,
				Validators: []validator.Object{
					objectvalidator.ExactlyOneOf(
						path.MatchRelative().AtParent().AtName("custom"),
						path.MatchRelative().AtParent().AtName("slack"),
						path.MatchRelative().AtParent().AtName("pager_duty"),
						path.MatchRelative().AtParent().AtName("sendlog"),
						path.MatchRelative().AtParent().AtName("email_group"),
						path.MatchRelative().AtParent().AtName("microsoft_teams"),
						path.MatchRelative().AtParent().AtName("microsoft_teams_workflow"),
						path.MatchRelative().AtParent().AtName("jira"),
						path.MatchRelative().AtParent().AtName("opsgenie"),
						path.MatchRelative().AtParent().AtName("demisto"),
					),
				},
			},
		},
		MarkdownDescription: "Coralogix webhook. For more info please review - https://coralogix.com/docs/coralogix-Webhook-extension/.",
	}
}

// A write-only value reaches the provider only through the configuration, and
// must not survive into state. Both halves are handled here: the value is moved
// into the field the request is built from, and afterwards cleared from what
// gets stored. The API echoes every one of these secrets back on read, so
// clearing it is what keeps it out of state.
//
// Which secrets are write-only is recorded by the version attributes, which are
// ordinary attributes and so are present in plan and state alike. Read has no
// configuration to consult and relies on that.

func injectWebhookWriteOnlySecrets(ctx context.Context, plan, config *WebhookResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if config == nil {
		return diags
	}

	if plan.Jira != nil && config.Jira != nil && !config.Jira.ApiTokenWO.IsNull() {
		plan.Jira.ApiKey = config.Jira.ApiTokenWO
	}
	if plan.PagerDuty != nil && config.PagerDuty != nil && !config.PagerDuty.ServiceKeyWO.IsNull() {
		plan.PagerDuty.ServiceKey = config.PagerDuty.ServiceKeyWO
	}
	if plan.CustomWebhook != nil && config.CustomWebhook != nil && !config.CustomWebhook.HeadersWO.IsNull() {
		merged, mergeDiags := mergeWebhookWriteOnlyHeaders(ctx, plan.CustomWebhook.Headers, config.CustomWebhook.HeadersWO)
		diags.Append(mergeDiags...)
		if diags.HasError() {
			return diags
		}
		plan.CustomWebhook.Headers = merged
	}
	return diags
}

func mergeWebhookWriteOnlyHeaders(ctx context.Context, headers, writeOnly types.Map) (types.Map, diag.Diagnostics) {
	merged := make(map[string]string)
	if !headers.IsNull() && !headers.IsUnknown() {
		values, diags := utils.TypeMapToStringMap(ctx, headers)
		if diags.HasError() {
			return types.MapNull(types.StringType), diags
		}
		for name, value := range values {
			merged[name] = value
		}
	}
	values, diags := utils.TypeMapToStringMap(ctx, writeOnly)
	if diags.HasError() {
		return types.MapNull(types.StringType), diags
	}
	for name, value := range values {
		merged[name] = value
	}
	return types.MapValueFrom(ctx, types.StringType, merged)
}

// stripWebhookWriteOnlySecrets clears the secrets the configuration supplies
// write-only from freshly flattened state, and carries the version attributes
// across, since the API knows nothing about them.
func stripWebhookWriteOnlySecrets(ctx context.Context, state, prior *WebhookResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	if state == nil || prior == nil {
		return diags
	}

	if state.Jira != nil && prior.Jira != nil && !prior.Jira.ApiTokenWOVersion.IsNull() {
		state.Jira.ApiKey = types.StringNull()
		state.Jira.ApiTokenWOVersion = prior.Jira.ApiTokenWOVersion
	}
	if state.PagerDuty != nil && prior.PagerDuty != nil && !prior.PagerDuty.ServiceKeyWOVersion.IsNull() {
		state.PagerDuty.ServiceKey = types.StringNull()
		state.PagerDuty.ServiceKeyWOVersion = prior.PagerDuty.ServiceKeyWOVersion
	}
	if state.CustomWebhook != nil && prior.CustomWebhook != nil && !prior.CustomWebhook.HeadersWOVersions.IsNull() {
		remaining, removeDiags := withoutWebhookWriteOnlyHeaders(ctx, state.CustomWebhook.Headers, prior.CustomWebhook.HeadersWOVersions)
		diags.Append(removeDiags...)
		if diags.HasError() {
			return diags
		}
		state.CustomWebhook.Headers = remaining
		state.CustomWebhook.HeadersWOVersions = prior.CustomWebhook.HeadersWOVersions
	}
	return diags
}

func withoutWebhookWriteOnlyHeaders(ctx context.Context, headers, versions types.Map) (types.Map, diag.Diagnostics) {
	values, diags := utils.TypeMapToStringMap(ctx, headers)
	if diags.HasError() {
		return types.MapNull(types.StringType), diags
	}
	for name := range versions.Elements() {
		delete(values, name)
	}
	return types.MapValueFrom(ctx, types.StringType, values)
}

// writeOnlyWebhookHeaderNameConflicts reports write-only header names that
// collide with another header. Which value would win is not something to leave
// to the merge order, and HTTP header names are case-insensitive, so two
// spellings of one name are a collision too: the API stores both, and the
// recipient decides which applies.
//
// The comparison is deliberately not applied to the versions map. Those keys
// have to match headers_wo exactly, because the value stripped from state is
// deleted by the key the API echoes back, and the API preserves the case it
// was given.
func writeOnlyWebhookHeaderNameConflicts(headers, writeOnly types.Map) []string {
	if writeOnly.IsNull() || writeOnly.IsUnknown() {
		return nil
	}

	seen := make(map[string]int)
	if !headers.IsNull() && !headers.IsUnknown() {
		for name := range headers.Elements() {
			seen[strings.ToLower(name)]++
		}
	}

	names := make([]string, 0, len(writeOnly.Elements()))
	for name := range writeOnly.Elements() {
		names = append(names, name)
	}
	sort.Strings(names)

	var conflicts []string
	for _, name := range names {
		lower := strings.ToLower(name)
		if seen[lower] > 0 {
			conflicts = append(conflicts, name)
		}
		seen[lower]++
	}
	return conflicts
}

// Header names that carry a credential often enough to be worth naming. A
// warning on every headers entry would fire on Content-Type and be ignored,
// so the check is deliberately a short list rather than a guess at intent.
// Each of these warnings needs its own summary: Terraform's console renderer
// shows one warning per summary and suppresses the rest, so sharing a summary
// would hide every warning after the first.
var webhookCredentialHeaderNames = map[string]struct{}{
	"authorization":       {},
	"proxy-authorization": {},
	"x-api-key":           {},
	"api-key":             {},
	"x-auth-token":        {},
	"cookie":              {},
}

const webhookWriteOnlyAdvice = " Terraform writes it to state. Use %s with %s to send the secret without storing it. Write-only attributes need Terraform 1.11 or later."

func (r *WebhookResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config *WebhookResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() || config == nil {
		return
	}

	resp.Diagnostics.Append(webhookCredentialWarnings(config)...)

	if config.CustomWebhook == nil {
		return
	}

	if conflicts := writeOnlyWebhookHeaderNameConflicts(config.CustomWebhook.Headers, config.CustomWebhook.HeadersWO); len(conflicts) > 0 {
		resp.Diagnostics.AddAttributeError(
			path.Root("custom").AtName("headers_wo"),
			"Header set both ways",
			fmt.Sprintf("%s collides with another header name. Keep a secret header in headers_wo only, and give it one spelling: HTTP header names are case-insensitive, so headers and headers_wo must not name the same header even in different case.", strings.Join(conflicts, ", ")),
		)
	}

	names := config.CustomWebhook.HeadersWO.Elements()
	versions := config.CustomWebhook.HeadersWOVersions.Elements()
	for name := range names {
		if _, ok := versions[name]; !ok {
			resp.Diagnostics.AddAttributeError(
				path.Root("custom").AtName("headers_wo_versions"),
				"Missing write-only header version",
				fmt.Sprintf("Header %q is set in headers_wo but has no entry in headers_wo_versions. Terraform holds no copy of a write-only value, so the version is what tells it the secret changed.", name),
			)
		}
	}
	for name := range versions {
		if _, ok := names[name]; !ok {
			resp.Diagnostics.AddAttributeError(
				path.Root("custom").AtName("headers_wo_versions"),
				"Version without a write-only header",
				fmt.Sprintf("Header %q has an entry in headers_wo_versions but is not set in headers_wo.", name),
			)
		}
	}
}

func (r *WebhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *WebhookResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var config *WebhookResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(injectWebhookWriteOnlySecrets(ctx, plan, config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	data, diags := expandWebhookType(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	rq := webhooks.CreateOutgoingWebhookRequest{
		Data: data,
	}
	createResult, httpResponse, err := r.client.
		OutgoingWebhooksServiceCreateOutgoingWebhook(ctx).
		CreateOutgoingWebhookRequest(rq).
		Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error creating coralogix_webhook",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Create", rq),
		)
		return
	}
	readRq := r.client.OutgoingWebhooksServiceGetOutgoingWebhook(ctx, *createResult.Id)

	result, _, err := readRq.Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error reading coralogix_webhook",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
		)
		return
	}

	state, diags := flattenWebhook(ctx, result.Webhook)

	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	resp.Diagnostics.Append(stripWebhookWriteOnlySecrets(ctx, state, plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r *WebhookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state *WebhookResourceModel

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()
	rq := r.client.OutgoingWebhooksServiceGetOutgoingWebhook(ctx, id)

	result, httpResponse, err := rq.Execute()
	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Resource %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error reading coralogix_webhook",
				utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
			)
		}
		return
	}

	prior := state
	state, diags = flattenWebhook(ctx, result.Webhook)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	resp.Diagnostics.Append(stripWebhookWriteOnlySecrets(ctx, state, prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r WebhookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Retrieve values from plan
	var plan *WebhookResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := plan.ID.ValueString()

	var config *WebhookResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(injectWebhookWriteOnlySecrets(ctx, plan, config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	data, diags := expandWebhookType(ctx, plan)

	rq := webhooks.UpdateOutgoingWebhookRequest{
		Data: data,
		Id:   &id,
	}
	_, httpResponse, err := r.client.
		OutgoingWebhooksServiceUpdateOutgoingWebhook(ctx).
		UpdateOutgoingWebhookRequest(rq).
		Execute()

	if err != nil {
		if httpResponse.StatusCode == http.StatusNotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("webhook %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError("Error updating coralogix_webhook", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Update", rq))
		}
		return
	}

	result, httpResponse, err := r.client.OutgoingWebhooksServiceGetOutgoingWebhook(ctx, id).Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error reading coralogix_webhook, state not updated", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Update", rq))
		return
	}

	state, diags := flattenWebhook(ctx, result.Webhook)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	resp.Diagnostics.Append(stripWebhookWriteOnlySecrets(ctx, state, plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
}

func (r WebhookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state WebhookResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ID.ValueString()

	_, httpResponse, err := r.client.
		OutgoingWebhooksServiceDeleteOutgoingWebhook(ctx, id).
		Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error deleting coralogix_webhook",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", nil),
		)
		return
	}
}

func expandWebhookType(ctx context.Context, plan *WebhookResourceModel) (*webhooks.OutgoingWebhookInputData, diag.Diagnostics) {
	var diags diag.Diagnostics
	var data *webhooks.OutgoingWebhookInputData
	if plan.CustomWebhook != nil {
		data, diags = expandGenericWebhook(ctx, plan.CustomWebhook)
	} else if plan.Slack != nil {
		data, diags = expandSlack(ctx, plan.Slack)
	} else if plan.PagerDuty != nil {
		data = expandPagerDuty(plan.PagerDuty)
	} else if plan.SendLog != nil {
		data = expandSendLog(plan.SendLog)
	} else if plan.EmailGroup != nil {
		data, diags = expandEmailGroup(ctx, plan.EmailGroup)
	} else if plan.MsTeamsWorkflow != nil {
		data = expandMicrosoftTeamsWorkflow(plan.MsTeamsWorkflow)
	} else if plan.Jira != nil {
		data = expandJira(plan.Jira)
	} else if plan.Opsgenie != nil {
		data = expandOpsgenie(plan.Opsgenie)
	} else if plan.Demisto != nil {
		data = expandDemisto(plan.Demisto)
	} else if plan.EventBridge != nil {
		data = expandEventBridge(plan.EventBridge)
	} else {
		diags.AddError("Error expanding webhook type", "Unknown webhook type")
	}

	if diags.HasError() {
		return nil, diags
	}

	data.Name = plan.Name.ValueStringPointer()
	return data, nil
}

func expandEventBridge(bridge *EventBridgeModel) *webhooks.OutgoingWebhookInputData {
	ty := webhooks.WEBHOOKTYPE_AWS_EVENT_BRIDGE
	return &webhooks.OutgoingWebhookInputData{
		Type: &ty,
		AwsEventBridge: &webhooks.AwsEventBridgeConfig{
			EventBusArn: bridge.EventBusARN.ValueStringPointer(),
			Detail:      bridge.Detail.ValueStringPointer(),
			DetailType:  bridge.DetailType.ValueStringPointer(),
			Source:      bridge.Source.ValueStringPointer(),
			RoleName:    bridge.RoleName.ValueStringPointer(),
		},
	}
}

func expandMicrosoftTeamsWorkflow(microsoftTeams *MsTeamsWorkflowModel) *webhooks.OutgoingWebhookInputData {
	ty := webhooks.WEBHOOKTYPE_MS_TEAMS_WORKFLOW
	return &webhooks.OutgoingWebhookInputData{
		MsTeamsWorkflow: map[string]any{},
		Type:            &ty,
		Url:             utils.StringNullIfUnknown(microsoftTeams.URL),
	}
}

func expandSlack(ctx context.Context, slack *SlackModel) (*webhooks.OutgoingWebhookInputData, diag.Diagnostics) {
	digests, diags := expandDigests(ctx, slack.NotifyAbout)
	if diags.HasError() {
		return nil, diags
	}

	attachments, diags := expandSlackAttachments(ctx, slack.Attachments)
	if diags.HasError() {
		return nil, diags
	}

	var url *string
	if planUrl := slack.URL; !(planUrl.IsNull() || planUrl.IsUnknown()) {
		url = planUrl.ValueStringPointer()
	}
	ty := webhooks.WEBHOOKTYPE_SLACK
	return &webhooks.OutgoingWebhookInputData{
		Url: url,
		Slack: &webhooks.SlackConfig{
			Digests:     digests,
			Attachments: attachments,
		},
		Type: &ty,
	}, nil
}

func expandSlackAttachments(ctx context.Context, attachmentsList types.List) ([]webhooks.Attachment, diag.Diagnostics) {
	var attachmentsObjects []types.Object
	var expandedAttachments []webhooks.Attachment
	diags := attachmentsList.ElementsAs(ctx, &attachmentsObjects, true)
	if diags.HasError() {
		return nil, diags
	}

	for _, attachmentObject := range attachmentsObjects {
		var attachmentModel SlackAttachmentModel
		if dg := attachmentObject.As(ctx, &attachmentModel, basetypes.ObjectAsOptions{}); dg.HasError() {
			diags.Append(dg...)
			continue
		}
		ty := webhooksProtoToSchemaSlackAttachmentType[attachmentModel.Type.ValueString()]
		expandedAttachments = append(expandedAttachments, webhooks.Attachment{
			Type:     &ty,
			IsActive: attachmentModel.Active.ValueBoolPointer(),
		})
	}
	return expandedAttachments, diags
}

func expandDigests(ctx context.Context, digestsSet types.Set) ([]webhooks.Digest, diag.Diagnostics) {
	digests := digestsSet.Elements()
	expandedDigests := make([]webhooks.Digest, 0, len(digests))
	var diags diag.Diagnostics
	for _, digest := range digests {
		val, err := digest.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Error expanding digest", err.Error())
			continue
		}
		var str string
		if err = val.As(&str); err != nil {
			diags.AddError("Error expanding digest", err.Error())
			continue
		}
		digestType := webhooksSchemaToProtoSlackConfigDigestType[str]
		expandedDigests = append(expandedDigests, expandDigest(digestType))
	}
	return expandedDigests, diags
}

func expandDigest(digest webhooks.DigestType) webhooks.Digest {
	isActive := true
	return webhooks.Digest{
		Type:     &digest,
		IsActive: &isActive,
	}
}

func expandGenericWebhook(ctx context.Context, genericWebhook *CustomWebhookModel) (*webhooks.OutgoingWebhookInputData, diag.Diagnostics) {
	headers, diags := utils.TypeMapToStringMap(ctx, genericWebhook.Headers)
	if diags.HasError() {
		return nil, diags
	}

	var url *string
	if planUrl := genericWebhook.URL; !(planUrl.IsNull() || planUrl.IsUnknown()) {
		url = planUrl.ValueStringPointer()
	}
	method := webhooksSchemaToProtoMethod[genericWebhook.Method.ValueString()]
	uuid := utils.UuidCreateIfNull(genericWebhook.UUID)
	ty := webhooks.WEBHOOKTYPE_GENERIC
	return &webhooks.OutgoingWebhookInputData{
		Type: &ty,
		Url:  url,
		GenericWebhook: &webhooks.GenericWebhookConfig{
			Uuid:    &uuid,
			Method:  &method,
			Headers: headers,
			Payload: genericWebhook.Payload.ValueStringPointer(),
		},
	}, nil
}

func expandPagerDuty(pagerDuty *PagerDutyModel) *webhooks.OutgoingWebhookInputData {
	ty := webhooks.WEBHOOKTYPE_PAGERDUTY
	return &webhooks.OutgoingWebhookInputData{
		Type: &ty,
		PagerDuty: &webhooks.PagerDutyConfig{
			ServiceKey: pagerDuty.ServiceKey.ValueStringPointer(),
		},
	}
}

func expandSendLog(sendLog *SendLogModel) *webhooks.OutgoingWebhookInputData {
	uuid := utils.UuidCreateIfNull(sendLog.UUID)

	ty := webhooks.WEBHOOKTYPE_SEND_LOG
	return &webhooks.OutgoingWebhookInputData{
		Type: &ty,
		SendLog: &webhooks.SendLogConfig{
			Payload: sendLog.Payload.ValueStringPointer(),
			Uuid:    &uuid,
		},
	}
}

func expandEmailGroup(ctx context.Context, emailGroup *EmailGroupModel) (*webhooks.OutgoingWebhookInputData, diag.Diagnostics) {
	emailAddresses, diags := utils.TypeStringElementsToStringSlice(ctx, emailGroup.Emails.Elements())
	if diags.HasError() {
		return nil, diags
	}

	ty := webhooks.WEBHOOKTYPE_EMAIL_GROUP
	return &webhooks.OutgoingWebhookInputData{
		Type: &ty,
		EmailGroup: &webhooks.EmailGroupConfig{
			EmailAddresses: emailAddresses,
		},
	}, nil
}

func expandJira(jira *JiraModel) *webhooks.OutgoingWebhookInputData {
	ty := webhooks.WEBHOOKTYPE_JIRA
	return &webhooks.OutgoingWebhookInputData{
		Type: &ty,
		Url:  utils.StringNullIfUnknown(jira.URL),
		Jira: &webhooks.JiraConfig{
			ApiToken:   jira.ApiKey.ValueStringPointer(),
			Email:      jira.Email.ValueStringPointer(),
			ProjectKey: jira.ProjectID.ValueStringPointer(),
		},
	}
}

func expandOpsgenie(opsgenie *OpsgenieModel) *webhooks.OutgoingWebhookInputData {
	ty := webhooks.WEBHOOKTYPE_OPSGENIE
	return &webhooks.OutgoingWebhookInputData{
		Opsgenie: map[string]any{},
		Type:     &ty,
		Url:      utils.StringNullIfUnknown(opsgenie.URL)}
}

func expandDemisto(demisto *DemistoModel) *webhooks.OutgoingWebhookInputData {
	uuid := utils.UuidCreateIfNull(demisto.UUID)
	ty := webhooks.WEBHOOKTYPE_DEMISTO
	return &webhooks.OutgoingWebhookInputData{
		Type: &ty,
		Url:  utils.StringNullIfUnknown(demisto.URL),
		Demisto: &webhooks.DemistoConfig{
			Uuid:    &uuid,
			Payload: demisto.Payload.ValueStringPointer(),
		},
	}
}

func flattenWebhook(ctx context.Context, webhook *webhooks.OutgoingWebhook) (*WebhookResourceModel, diag.Diagnostics) {
	if webhook == nil {
		return nil, diag.Diagnostics{
			diag.NewErrorDiagnostic(
				"Error flattening webhook",
				"Received nil webhook from API",
			),
		}
	}

	result := &WebhookResourceModel{}

	var diags diag.Diagnostics

	if webhook.AwsEventBridge != nil {
		result.EventBridge, result.ID, result.ExternalID, result.Name = flattenEventBridge(webhook)
	} else if webhook.Demisto != nil {
		result.Demisto, result.ID, result.ExternalID, result.Name = flattenDemisto(webhook)
	} else if webhook.EmailGroup != nil {
		result.EmailGroup, result.ID, result.ExternalID, result.Name = flattenEmailGroup(webhook)
	} else if webhook.GenericWebhook != nil {
		result.CustomWebhook, result.ID, result.ExternalID, result.Name, diags = flattenGenericWebhook(ctx, webhook)
	} else if webhook.Jira != nil {
		result.Jira, result.ID, result.ExternalID, result.Name = flattenJira(webhook)
	} else if webhook.MicrosoftTeams != nil {
		result.MsTeams, result.ID, result.ExternalID, result.Name = flattenMicrosoftTeams(webhook)
	} else if webhook.MsTeamsWorkflow != nil {
		result.MsTeamsWorkflow, result.ID, result.ExternalID, result.Name = flattenMsTeamsWorkflow(webhook)
	} else if webhook.Opsgenie != nil {
		result.Opsgenie, result.ID, result.ExternalID, result.Name = flattenOpsgenie(webhook)
	} else if webhook.PagerDuty != nil {
		result.PagerDuty, result.ID, result.ExternalID, result.Name = flattenPagerDuty(webhook)
	} else if webhook.SendLog != nil {
		result.SendLog, result.ID, result.ExternalID, result.Name = flattenSendLog(webhook)
	} else if webhook.Slack != nil {
		result.Slack, result.ID, result.ExternalID, result.Name, diags = flattenSlack(ctx, webhook)
	} else {
		diags.AddError("Error flattening webhook", fmt.Sprintf("Unknown webhook type: %v", *webhook))
	}
	return result, diags
}

func flattenGenericWebhook(ctx context.Context, genericWebhook *webhooks.OutgoingWebhook) (*CustomWebhookModel, types.String, types.String, types.String, diag.Diagnostics) {
	headers, diags := types.MapValueFrom(ctx, types.StringType, genericWebhook.GenericWebhook.Headers)
	return &CustomWebhookModel{
		UUID:    types.StringPointerValue(genericWebhook.GenericWebhook.Uuid),
		Method:  types.StringValue(webhooksProtoToSchemaMethod[*genericWebhook.GenericWebhook.Method]),
		Headers: headers,
		Payload: types.StringPointerValue(genericWebhook.GenericWebhook.Payload),
		URL:     types.StringPointerValue(genericWebhook.Url),
		// A zero-value Map carries no element type and fails state conversion.
		HeadersWO:         types.MapNull(types.StringType),
		HeadersWOVersions: types.MapNull(types.Int64Type),
	}, types.StringPointerValue(genericWebhook.Id), utils.Int64ToStringValue(genericWebhook.ExternalId), types.StringPointerValue(genericWebhook.Name), diags
}

func flattenSlack(ctx context.Context, slack *webhooks.OutgoingWebhook) (*SlackModel, types.String, types.String, types.String, diag.Diagnostics) {
	digests, diags := flattenDigests(ctx, slack.Slack.Digests)
	if diags.HasError() {
		return nil, types.StringNull(), types.StringNull(), types.StringNull(), diags
	}

	attachments, diags := flattenSlackAttachments(ctx, slack.Slack.Attachments)
	if diags.HasError() {
		return nil, types.StringNull(), types.StringNull(), types.StringNull(), diags
	}

	return &SlackModel{
		NotifyAbout: digests,
		URL:         types.StringPointerValue(slack.Url),
		Attachments: attachments,
	}, types.StringPointerValue(slack.Id), utils.Int64ToStringValue(slack.ExternalId), types.StringPointerValue(slack.Name), nil
}

func flattenSlackAttachments(ctx context.Context, attachments []webhooks.Attachment) (types.List, diag.Diagnostics) {
	if len(attachments) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: slackAttachmentsAttr()}), nil
	}

	attachmentsElements := make([]SlackAttachmentModel, 0, len(attachments))
	for _, attachment := range attachments {
		flattenedAttachment := SlackAttachmentModel{
			Type:   types.StringValue(webhooksSchemaToProtoSlackAttachmentType[attachment.GetType()]),
			Active: types.BoolValue(attachment.GetIsActive()),
		}
		attachmentsElements = append(attachmentsElements, flattenedAttachment)
	}

	return types.ListValueFrom(ctx, types.ObjectType{AttrTypes: slackAttachmentsAttr()}, attachmentsElements)
}

func slackAttachmentsAttr() map[string]attr.Type {
	return map[string]attr.Type{
		"type":   types.StringType,
		"active": types.BoolType,
	}
}

func flattenDigests(ctx context.Context, digests []webhooks.Digest) (types.Set, diag.Diagnostics) {
	if len(digests) == 0 {
		return types.SetNull(types.StringType), nil
	}

	digestsElements := make([]attr.Value, 0, len(digests))
	for _, digest := range digests {
		flattenedDigest := flattenDigest(&digest)
		digestsElements = append(digestsElements, flattenedDigest)
	}

	return types.SetValueFrom(ctx, types.StringType, digestsElements)
}

func flattenDigest(digest *webhooks.Digest) types.String {
	return types.StringValue(webhooksProtoToSchemaSlackConfigDigestType[digest.GetType()])
}

func flattenPagerDuty(pagerDuty *webhooks.OutgoingWebhook) (*PagerDutyModel, types.String, types.String, types.String) {
	return &PagerDutyModel{
		ServiceKey: types.StringPointerValue(pagerDuty.PagerDuty.ServiceKey),
	}, types.StringPointerValue(pagerDuty.Id), utils.Int64ToStringValue(pagerDuty.ExternalId), types.StringPointerValue(pagerDuty.Name)
}

func flattenSendLog(sendLog *webhooks.OutgoingWebhook) (*SendLogModel, types.String, types.String, types.String) {
	return &SendLogModel{
		UUID:    types.StringPointerValue(sendLog.SendLog.Uuid),
		Payload: types.StringPointerValue(sendLog.SendLog.Payload),
		URL:     types.StringPointerValue(sendLog.Url),
	}, types.StringPointerValue(sendLog.Id), utils.Int64ToStringValue(sendLog.ExternalId), types.StringPointerValue(sendLog.Name)
}

func flattenEmailGroup(emailGroup *webhooks.OutgoingWebhook) (*EmailGroupModel, types.String, types.String, types.String) {
	return &EmailGroupModel{
		Emails: utils.StringSliceToTypeStringList(emailGroup.EmailGroup.EmailAddresses),
	}, types.StringPointerValue(emailGroup.Id), utils.Int64ToStringValue(emailGroup.ExternalId), types.StringPointerValue(emailGroup.Name)
}

func flattenMsTeamsWorkflow(msteamswf *webhooks.OutgoingWebhook) (*MsTeamsWorkflowModel, types.String, types.String, types.String) {
	return &MsTeamsWorkflowModel{
		URL: types.StringPointerValue(msteamswf.Url),
	}, types.StringPointerValue(msteamswf.Id), utils.Int64ToStringValue(msteamswf.ExternalId), types.StringPointerValue(msteamswf.Name)
}

// Legacy webhook, is converted to MS Teams Workflow webhook
func flattenMicrosoftTeams(msteams *webhooks.OutgoingWebhook) (*MsTeamsWorkflowModel, types.String, types.String, types.String) {
	return &MsTeamsWorkflowModel{
		URL: types.StringPointerValue(msteams.Url),
	}, types.StringPointerValue(msteams.Id), utils.Int64ToStringValue(msteams.ExternalId), types.StringPointerValue(msteams.Name)
}

func flattenJira(jira *webhooks.OutgoingWebhook) (*JiraModel, types.String, types.String, types.String) {
	return &JiraModel{
		ApiKey:    types.StringPointerValue(jira.Jira.ApiToken),
		Email:     types.StringPointerValue(jira.Jira.Email),
		ProjectID: types.StringPointerValue(jira.Jira.ProjectKey),
		URL:       types.StringPointerValue(jira.Url),
	}, types.StringPointerValue(jira.Id), utils.Int64ToStringValue(jira.ExternalId), types.StringPointerValue(jira.Name)
}

func flattenOpsgenie(opsgenie *webhooks.OutgoingWebhook) (*OpsgenieModel, types.String, types.String, types.String) {
	return &OpsgenieModel{
		URL: types.StringPointerValue(opsgenie.Url),
	}, types.StringPointerValue(opsgenie.Id), utils.Int64ToStringValue(opsgenie.ExternalId), types.StringPointerValue(opsgenie.Name)
}

func flattenDemisto(demisto *webhooks.OutgoingWebhook) (*DemistoModel, types.String, types.String, types.String) {
	return &DemistoModel{
		UUID:    types.StringPointerValue(demisto.Demisto.Uuid),
		Payload: types.StringPointerValue(demisto.Demisto.Payload),
		URL:     types.StringPointerValue(demisto.Url),
	}, types.StringPointerValue(demisto.Id), utils.Int64ToStringValue(demisto.ExternalId), types.StringPointerValue(demisto.Name)
}

func flattenEventBridge(bridge *webhooks.OutgoingWebhook) (*EventBridgeModel, types.String, types.String, types.String) {
	return &EventBridgeModel{
		EventBusARN: types.StringPointerValue(bridge.AwsEventBridge.EventBusArn),
		Detail:      types.StringPointerValue(bridge.AwsEventBridge.Detail),
		DetailType:  types.StringPointerValue(bridge.AwsEventBridge.DetailType),
		Source:      types.StringPointerValue(bridge.AwsEventBridge.Source),
		RoleName:    types.StringPointerValue(bridge.AwsEventBridge.RoleName),
	}, types.StringPointerValue(bridge.Id), utils.Int64ToStringValue(bridge.ExternalId), types.StringPointerValue(bridge.Name)
}
