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
	"log"
	"net/http"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdkOpenapi "github.com/coralogix/coralogix-management-sdk/go/openapi/cxsdk"
	webhooks "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/outgoing_webhooks_service"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

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
	ServiceKey types.String `tfsdk:"service_key"`
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
	ApiKey    types.String `tfsdk:"api_token"`
	Email     types.String `tfsdk:"email"`
	ProjectID types.String `tfsdk:"project_key"`
	URL       types.String `tfsdk:"url"`
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

func (r *WebhookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
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
						ElementType:         types.StringType,
						MarkdownDescription: "Webhook headers. Map of string to string.",
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
						MarkdownDescription: "PagerDuty service key.",
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
						Required:            true,
						MarkdownDescription: "Webhook URL.",
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
						MarkdownDescription: "Jira API token.",
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

func (r *WebhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan *WebhookResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
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
	log.Printf("[INFO] Creating new coralogix_webhook: %s", utils.FormatJSON(rq))
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
	log.Printf("[INFO] Created new coralogix_webhook: %s", utils.FormatJSON(createResult))

	readRq := r.client.OutgoingWebhooksServiceGetOutgoingWebhook(ctx, *createResult.Id)

	log.Printf("[INFO] Reading new coralogix_webhook: %s", utils.FormatJSON(rq))

	result, _, err := readRq.Execute()
	if err != nil {
		resp.Diagnostics.AddError("Error reading coralogix_webhook",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Read", nil),
		)
		return
	}
	log.Printf("[INFO] Read coralogix_webhook: %s", utils.FormatJSON(result))

	plan, diags = flattenWebhook(ctx, result.Webhook)

	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	diags = resp.State.Set(ctx, plan)
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

	log.Printf("[INFO] Reading coralogix_webhook: %s", utils.FormatJSON(rq))

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
	log.Printf("[INFO] Read coralogix_webhook: %s", utils.FormatJSON(result))

	state, diags = flattenWebhook(ctx, result.Webhook)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
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

	data, diags := expandWebhookType(ctx, plan)

	rq := webhooks.UpdateOutgoingWebhookRequest{
		Data: data,
		Id:   &id,
	}
	log.Printf("[INFO] Updating coralogix_webhook: %s", utils.FormatJSON(rq))
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
			resp.Diagnostics.AddError("Error updating coralogix_webhook", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Update", nil))
		}
		return
	}

	result, httpResponse, err := r.client.OutgoingWebhooksServiceGetOutgoingWebhook(ctx, id).Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error reading coralogix_webhook, state not updated", utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Update", nil))
		return
	}

	log.Printf("[INFO] Updated coralogix_webhook: %s", utils.FormatJSON(result))
	plan, diags = flattenWebhook(ctx, result.Webhook)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	diags = resp.State.Set(ctx, plan)
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

	log.Printf("[INFO] Deleting coralogix_webhook %s", id)

	result, httpResponse, err := r.client.
		OutgoingWebhooksServiceDeleteOutgoingWebhook(ctx, id).
		Execute()

	if err != nil {
		resp.Diagnostics.AddError("Error deleting coralogix_webhook",
			utils.FormatOpenAPIErrors(cxsdkOpenapi.NewAPIError(httpResponse, err), "Delete", nil),
		)
		return
	}
	log.Printf("[INFO] Deleted coralogix_webhook: %s", utils.FormatJSON(result))
}

func expandWebhookType(ctx context.Context, plan *WebhookResourceModel) (*webhooks.OutgoingWebhookInputData, diag.Diagnostics) {
	var diags diag.Diagnostics
	data := webhooks.OutgoingWebhookInputData{}
	if plan.CustomWebhook != nil {
		data.OutgoingWebhookInputDataGenericWebhook, diags = expandGenericWebhook(ctx, plan.CustomWebhook)
		data.OutgoingWebhookInputDataGenericWebhook.Name = plan.Name.ValueStringPointer()
	} else if plan.Slack != nil {
		data.OutgoingWebhookInputDataSlack, diags = expandSlack(ctx, plan.Slack)
		data.OutgoingWebhookInputDataSlack.Name = plan.Name.ValueStringPointer()
	} else if plan.PagerDuty != nil {
		data.OutgoingWebhookInputDataPagerDuty = expandPagerDuty(plan.PagerDuty)
		data.OutgoingWebhookInputDataPagerDuty.Name = plan.Name.ValueStringPointer()
	} else if plan.SendLog != nil {
		data.OutgoingWebhookInputDataSendLog = expandSendLog(plan.SendLog)
		data.OutgoingWebhookInputDataSendLog.Name = plan.Name.ValueStringPointer()
	} else if plan.EmailGroup != nil {
		data.OutgoingWebhookInputDataEmailGroup, diags = expandEmailGroup(ctx, plan.EmailGroup)
		data.OutgoingWebhookInputDataEmailGroup.Name = plan.Name.ValueStringPointer()
	} else if plan.MsTeamsWorkflow != nil {
		data.OutgoingWebhookInputDataMsTeamsWorkflow = expandMicrosoftTeamsWorkflow(plan.MsTeamsWorkflow)
		data.OutgoingWebhookInputDataMsTeamsWorkflow.Name = plan.Name.ValueStringPointer()
	} else if plan.Jira != nil {
		data.OutgoingWebhookInputDataJira = expandJira(plan.Jira)
		data.OutgoingWebhookInputDataJira.Name = plan.Name.ValueStringPointer()
	} else if plan.Opsgenie != nil {
		data.OutgoingWebhookInputDataOpsgenie = expandOpsgenie(plan.Opsgenie)
		data.OutgoingWebhookInputDataOpsgenie.Name = plan.Name.ValueStringPointer()
	} else if plan.Demisto != nil {
		data.OutgoingWebhookInputDataDemisto = expandDemisto(plan.Demisto)
		data.OutgoingWebhookInputDataDemisto.Name = plan.Name.ValueStringPointer()
	} else if plan.EventBridge != nil {
		data.OutgoingWebhookInputDataAwsEventBridge = expandEventBridge(plan.EventBridge)
		data.OutgoingWebhookInputDataAwsEventBridge.Name = plan.Name.ValueStringPointer()
	} else {
		diags.AddError("Error expanding webhook type", "Unknown webhook type")
	}

	if diags.HasError() {
		return nil, diags
	}

	return &data, nil
}

func expandEventBridge(bridge *EventBridgeModel) *webhooks.OutgoingWebhookInputDataAwsEventBridge {
	ty := webhooks.WEBHOOKTYPE_AWS_EVENT_BRIDGE
	return &webhooks.OutgoingWebhookInputDataAwsEventBridge{
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

func expandMicrosoftTeamsWorkflow(microsoftTeams *MsTeamsWorkflowModel) *webhooks.OutgoingWebhookInputDataMsTeamsWorkflow {
	ty := webhooks.WEBHOOKTYPE_MS_TEAMS_WORKFLOW
	return &webhooks.OutgoingWebhookInputDataMsTeamsWorkflow{
		MsTeamsWorkflow: map[string]any{},
		Type:            &ty,
		Url:             utils.StringNullIfUnknown(microsoftTeams.URL),
	}
}

func expandSlack(ctx context.Context, slack *SlackModel) (*webhooks.OutgoingWebhookInputDataSlack, diag.Diagnostics) {
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
	return &webhooks.OutgoingWebhookInputDataSlack{
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

func expandGenericWebhook(ctx context.Context, genericWebhook *CustomWebhookModel) (*webhooks.OutgoingWebhookInputDataGenericWebhook, diag.Diagnostics) {
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
	return &webhooks.OutgoingWebhookInputDataGenericWebhook{
		Type: &ty,
		Url:  url,
		GenericWebhook: &webhooks.GenericWebhookConfig{
			Uuid:    &uuid,
			Method:  &method,
			Headers: &headers,
			Payload: genericWebhook.Payload.ValueStringPointer(),
		},
	}, nil
}

func expandPagerDuty(pagerDuty *PagerDutyModel) *webhooks.OutgoingWebhookInputDataPagerDuty {
	ty := webhooks.WEBHOOKTYPE_PAGERDUTY
	return &webhooks.OutgoingWebhookInputDataPagerDuty{
		Type: &ty,
		PagerDuty: &webhooks.PagerDutyConfig{
			ServiceKey: pagerDuty.ServiceKey.ValueStringPointer(),
		},
	}
}

func expandSendLog(sendLog *SendLogModel) *webhooks.OutgoingWebhookInputDataSendLog {
	uuid := utils.UuidCreateIfNull(sendLog.UUID)

	ty := webhooks.WEBHOOKTYPE_SEND_LOG
	return &webhooks.OutgoingWebhookInputDataSendLog{
		Type: &ty,
		SendLog: &webhooks.SendLogConfig{
			Payload: sendLog.Payload.ValueStringPointer(),
			Uuid:    &uuid,
		},
		Url: sendLog.URL.ValueStringPointer(),
	}
}

func expandEmailGroup(ctx context.Context, emailGroup *EmailGroupModel) (*webhooks.OutgoingWebhookInputDataEmailGroup, diag.Diagnostics) {
	emailAddresses, diags := utils.TypeStringSliceToStringSlice(ctx, emailGroup.Emails.Elements())
	if diags.HasError() {
		return nil, diags
	}

	ty := webhooks.WEBHOOKTYPE_EMAIL_GROUP
	return &webhooks.OutgoingWebhookInputDataEmailGroup{
		Type: &ty,
		EmailGroup: &webhooks.EmailGroupConfig{
			EmailAddresses: emailAddresses,
		},
	}, nil
}

func expandJira(jira *JiraModel) *webhooks.OutgoingWebhookInputDataJira {
	ty := webhooks.WEBHOOKTYPE_JIRA
	return &webhooks.OutgoingWebhookInputDataJira{
		Type: &ty,
		Url:  utils.StringNullIfUnknown(jira.URL),
		Jira: &webhooks.JiraConfig{
			ApiToken:   jira.ApiKey.ValueStringPointer(),
			Email:      jira.Email.ValueStringPointer(),
			ProjectKey: jira.ProjectID.ValueStringPointer(),
		},
	}
}

func expandOpsgenie(opsgenie *OpsgenieModel) *webhooks.OutgoingWebhookInputDataOpsgenie {
	ty := webhooks.WEBHOOKTYPE_OPSGENIE
	return &webhooks.OutgoingWebhookInputDataOpsgenie{
		Opsgenie: map[string]any{},
		Type:     &ty,
		Url:      utils.StringNullIfUnknown(opsgenie.URL)}
}

func expandDemisto(demisto *DemistoModel) *webhooks.OutgoingWebhookInputDataDemisto {
	uuid := utils.UuidCreateIfNull(demisto.UUID)
	ty := webhooks.WEBHOOKTYPE_DEMISTO
	return &webhooks.OutgoingWebhookInputDataDemisto{
		Type: &ty,
		Url:  utils.StringNullIfUnknown(demisto.URL),
		Demisto: &webhooks.DemistoConfig{
			Uuid:    &uuid,
			Payload: demisto.Payload.ValueStringPointer(),
		},
	}
}

func flattenWebhook(ctx context.Context, webhook *webhooks.OutgoingWebhook) (*WebhookResourceModel, diag.Diagnostics) {
	result := &WebhookResourceModel{}

	var diags diag.Diagnostics

	if webhook.OutgoingWebhookAwsEventBridge != nil {
		result.EventBridge, result.ID, result.ExternalID, result.Name = flattenEventBridge(webhook.OutgoingWebhookAwsEventBridge)
	} else if webhook.OutgoingWebhookDemisto != nil {
		result.Demisto, result.ID, result.ExternalID, result.Name = flattenDemisto(webhook.OutgoingWebhookDemisto)
	} else if webhook.OutgoingWebhookEmailGroup != nil {
		result.EmailGroup, result.ID, result.ExternalID, result.Name = flattenEmailGroup(webhook.OutgoingWebhookEmailGroup)
	} else if webhook.OutgoingWebhookGenericWebhook != nil {
		result.CustomWebhook, result.ID, result.ExternalID, result.Name, diags = flattenGenericWebhook(ctx, webhook.OutgoingWebhookGenericWebhook)
	} else if webhook.OutgoingWebhookJira != nil {
		result.Jira, result.ID, result.ExternalID, result.Name = flattenJira(webhook.OutgoingWebhookJira)
	} else if webhook.OutgoingWebhookMicrosoftTeams != nil {
		result.MsTeams, result.ID, result.ExternalID, result.Name = flattenMicrosoftTeams(webhook.OutgoingWebhookMicrosoftTeams)
	} else if webhook.OutgoingWebhookMsTeamsWorkflow != nil {
		result.MsTeamsWorkflow, result.ID, result.ExternalID, result.Name = flattenMsTeamsWorkflow(webhook.OutgoingWebhookMsTeamsWorkflow)
	} else if webhook.OutgoingWebhookOpsgenie != nil {
		result.Opsgenie, result.ID, result.ExternalID, result.Name = flattenOpsgenie(webhook.OutgoingWebhookOpsgenie)
	} else if webhook.OutgoingWebhookPagerDuty != nil {
		result.PagerDuty, result.ID, result.ExternalID, result.Name = flattenPagerDuty(webhook.OutgoingWebhookPagerDuty)
	} else if webhook.OutgoingWebhookSendLog != nil {
		result.SendLog, result.ID, result.ExternalID, result.Name = flattenSendLog(webhook.OutgoingWebhookSendLog)
	} else if webhook.OutgoingWebhookSlack != nil {
		result.Slack, result.ID, result.ExternalID, result.Name, diags = flattenSlack(ctx, webhook.OutgoingWebhookSlack)
	} else {
		diags.AddError("Error flattening webhook", fmt.Sprintf("Unknown webhook type: %v", *webhook))
	}
	return result, diags
}

func flattenGenericWebhook(ctx context.Context, genericWebhook *webhooks.OutgoingWebhookGenericWebhook) (*CustomWebhookModel, types.String, types.String, types.String, diag.Diagnostics) {
	headers, diags := types.MapValueFrom(ctx, types.StringType, genericWebhook.GenericWebhook.Headers)
	return &CustomWebhookModel{
		UUID:    types.StringPointerValue(genericWebhook.GenericWebhook.Uuid),
		Method:  types.StringValue(webhooksProtoToSchemaMethod[*genericWebhook.GenericWebhook.Method]),
		Headers: headers,
		Payload: types.StringPointerValue(genericWebhook.GenericWebhook.Payload),
		URL:     types.StringPointerValue(genericWebhook.Url),
	}, types.StringPointerValue(genericWebhook.Id), utils.Int64ToStringValue(genericWebhook.ExternalId), types.StringPointerValue(genericWebhook.Name), diags
}

func flattenSlack(ctx context.Context, slack *webhooks.OutgoingWebhookSlack) (*SlackModel, types.String, types.String, types.String, diag.Diagnostics) {
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

func flattenPagerDuty(pagerDuty *webhooks.OutgoingWebhookPagerDuty) (*PagerDutyModel, types.String, types.String, types.String) {
	return &PagerDutyModel{
		ServiceKey: types.StringPointerValue(pagerDuty.PagerDuty.ServiceKey),
	}, types.StringPointerValue(pagerDuty.Id), utils.Int64ToStringValue(pagerDuty.ExternalId), types.StringPointerValue(pagerDuty.Name)
}

func flattenSendLog(sendLog *webhooks.OutgoingWebhookSendLog) (*SendLogModel, types.String, types.String, types.String) {
	return &SendLogModel{
		UUID:    types.StringPointerValue(sendLog.SendLog.Uuid),
		Payload: types.StringPointerValue(sendLog.SendLog.Payload),
		URL:     types.StringPointerValue(sendLog.Url),
	}, types.StringPointerValue(sendLog.Id), utils.Int64ToStringValue(sendLog.ExternalId), types.StringPointerValue(sendLog.Name)
}

func flattenEmailGroup(emailGroup *webhooks.OutgoingWebhookEmailGroup) (*EmailGroupModel, types.String, types.String, types.String) {
	return &EmailGroupModel{
		Emails: utils.StringSliceToTypeStringList(emailGroup.EmailGroup.EmailAddresses),
	}, types.StringPointerValue(emailGroup.Id), utils.Int64ToStringValue(emailGroup.ExternalId), types.StringPointerValue(emailGroup.Name)
}

func flattenMsTeamsWorkflow(msteamswf *webhooks.OutgoingWebhookMsTeamsWorkflow) (*MsTeamsWorkflowModel, types.String, types.String, types.String) {
	return &MsTeamsWorkflowModel{
		URL: types.StringPointerValue(msteamswf.Url),
	}, types.StringPointerValue(msteamswf.Id), utils.Int64ToStringValue(msteamswf.ExternalId), types.StringPointerValue(msteamswf.Name)
}

// Legacy webhook, is converted to MS Teams Workflow webhook
func flattenMicrosoftTeams(msteams *webhooks.OutgoingWebhookMicrosoftTeams) (*MsTeamsWorkflowModel, types.String, types.String, types.String) {
	return &MsTeamsWorkflowModel{
		URL: types.StringPointerValue(msteams.Url),
	}, types.StringPointerValue(msteams.Id), utils.Int64ToStringValue(msteams.ExternalId), types.StringPointerValue(msteams.Name)
}

func flattenJira(jira *webhooks.OutgoingWebhookJira) (*JiraModel, types.String, types.String, types.String) {
	return &JiraModel{
		ApiKey:    types.StringPointerValue(jira.Jira.ApiToken),
		Email:     types.StringPointerValue(jira.Jira.Email),
		ProjectID: types.StringPointerValue(jira.Jira.ProjectKey),
		URL:       types.StringPointerValue(jira.Url),
	}, types.StringPointerValue(jira.Id), utils.Int64ToStringValue(jira.ExternalId), types.StringPointerValue(jira.Name)
}

func flattenOpsgenie(opsgenie *webhooks.OutgoingWebhookOpsgenie) (*OpsgenieModel, types.String, types.String, types.String) {
	return &OpsgenieModel{
		URL: types.StringPointerValue(opsgenie.Url),
	}, types.StringPointerValue(opsgenie.Id), utils.Int64ToStringValue(opsgenie.ExternalId), types.StringPointerValue(opsgenie.Name)
}

func flattenDemisto(demisto *webhooks.OutgoingWebhookDemisto) (*DemistoModel, types.String, types.String, types.String) {
	return &DemistoModel{
		UUID:    types.StringPointerValue(demisto.Demisto.Uuid),
		Payload: types.StringPointerValue(demisto.Demisto.Payload),
		URL:     types.StringPointerValue(demisto.Url),
	}, types.StringPointerValue(demisto.Id), utils.Int64ToStringValue(demisto.ExternalId), types.StringPointerValue(demisto.Name)
}

func flattenEventBridge(bridge *webhooks.OutgoingWebhookAwsEventBridge) (*EventBridgeModel, types.String, types.String, types.String) {
	return &EventBridgeModel{
		EventBusARN: types.StringPointerValue(bridge.AwsEventBridge.EventBusArn),
		Detail:      types.StringPointerValue(bridge.AwsEventBridge.Detail),
		DetailType:  types.StringPointerValue(bridge.AwsEventBridge.DetailType),
		Source:      types.StringPointerValue(bridge.AwsEventBridge.Source),
		RoleName:    types.StringPointerValue(bridge.AwsEventBridge.RoleName),
	}, types.StringPointerValue(bridge.Id), utils.Int64ToStringValue(bridge.ExternalId), types.StringPointerValue(bridge.Name)
}
