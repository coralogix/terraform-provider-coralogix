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

package coralogix

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"terraform-provider-coralogix/coralogix/clientset"
	webhooks "terraform-provider-coralogix/coralogix/clientset/grpc/webhooks"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"google.golang.org/protobuf/encoding/protojson"

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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	_                           resource.ResourceWithConfigure   = &WebhookResource{}
	_                           resource.ResourceWithImportState = &WebhookResource{}
	webhooksSchemaToProtoMethod                                  = map[string]webhooks.GenericWebhookConfig_MethodType{
		"get":  webhooks.GenericWebhookConfig_GET,
		"post": webhooks.GenericWebhookConfig_POST,
		"put":  webhooks.GenericWebhookConfig_PUT,
	}
	webhooksProtoToSchemaMethod                = ReverseMap(webhooksSchemaToProtoMethod)
	webhooksValidMethods                       = GetKeys(webhooksSchemaToProtoMethod)
	webhooksSchemaToProtoSlackConfigDigestType = map[string]webhooks.SlackConfig_DigestType{
		"error_and_critical_logs": webhooks.SlackConfig_ERROR_AND_CRITICAL_LOGS,
		"flow_anomalies":          webhooks.SlackConfig_FLOW_ANOMALIES,
		"spike_anomalies":         webhooks.SlackConfig_SPIKE_ANOMALIES,
		"data_usage":              webhooks.SlackConfig_DATA_USAGE,
	}
	webhooksProtoToSchemaSlackConfigDigestType = ReverseMap(webhooksSchemaToProtoSlackConfigDigestType)
	webhooksValidSlackConfigDigestTypes        = GetKeys(webhooksSchemaToProtoSlackConfigDigestType)
	webhooksProtoToSchemaSlackAttachmentType   = map[string]webhooks.SlackConfig_AttachmentType{
		"empty":           webhooks.SlackConfig_EMPTY,
		"metric_snapshot": webhooks.SlackConfig_METRIC_SNAPSHOT,
		"logs":            webhooks.SlackConfig_LOGS,
	}
	webhooksSchemaToProtoSlackAttachmentType = ReverseMap(webhooksProtoToSchemaSlackAttachmentType)
	webhooksValidSlackAttachmentTypes        = GetKeys(webhooksProtoToSchemaSlackAttachmentType)
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
	createWebhookURL = "com.coralogix.outgoing_webhooks.v1.OutgoingWebhooksService/CreateOutgoingWebhook"
	getWebhookURL    = "com.coralogix.outgoing_webhooks.v1.OutgoingWebhooksService/GetOutgoingWebhook"
	updateWebhookURL = "com.coralogix.outgoing_webhooks.v1.OutgoingWebhooksService/UpdateOutgoingWebhook"
	deleteWebhookURL = "com.coralogix.outgoing_webhooks.v1.OutgoingWebhooksService/DeleteOutgoingWebhook"
)

func NewWebhookResource() resource.Resource {
	return &WebhookResource{}
}

type WebhookResource struct {
	client *clientset.WebhooksClient
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
	MsTeamsWorkflow *MsTeamsWorkflowModel `tfsdk:"microsoft_teams"`
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
						path.MatchRelative().AtParent().AtName("jira"),
						path.MatchRelative().AtParent().AtName("opsgenie"),
						path.MatchRelative().AtParent().AtName("demisto"),
						path.MatchRelative().AtParent().AtName("event_bridge"),
					),
				},
				Optional:            true,
				MarkdownDescription: "Email group webhook.",
			},
			"microsoft_teams": schema.SingleNestedAttribute{
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
						path.MatchRelative().AtParent().AtName("jira"),
						path.MatchRelative().AtParent().AtName("opsgenie"),
						path.MatchRelative().AtParent().AtName("demisto"),
						path.MatchRelative().AtParent().AtName("event_bridge"),
					),
				},
				Optional:            true,
				MarkdownDescription: "Microsoft Teams Workflow webhook.",
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
						Optional: true,
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

	createWebhookRequest, diags := extractCreateWebhookRequest(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	webhookStr := protojson.Format(createWebhookRequest)
	log.Printf("[INFO] Creating new webhook: %s", webhookStr)
	createResp, err := r.client.CreateWebhook(ctx, createWebhookRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error creating Webhook",
			formatRpcErrors(err, createWebhookURL, webhookStr),
		)
		return
	}
	id := createResp.Id.GetValue()
	log.Printf("[INFO] Submitted new webhook, id - %s", id)

	readWebhookRequest := &webhooks.GetOutgoingWebhookRequest{
		Id: wrapperspb.String(id),
	}
	getWebhookResp, err := r.client.GetWebhook(ctx, readWebhookRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error reading Webhook",
			formatRpcErrors(err, getWebhookURL, protojson.Format(readWebhookRequest)),
		)
		return
	}

	getWebhookStr := protojson.Format(getWebhookResp)
	log.Printf("[INFO] Reading webhook - %s", getWebhookStr)

	plan, diags = flattenWebhook(ctx, getWebhookResp.GetWebhook())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	// Set state to fully populated data
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
	readWebhookRequest := &webhooks.GetOutgoingWebhookRequest{
		Id: wrapperspb.String(id),
	}

	log.Printf("[INFO] Reading Webhook: %s", id)
	getWebhookResp, err := r.client.GetWebhook(ctx, readWebhookRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Webhook %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Webhook",
				formatRpcErrors(err, getWebhookURL, protojson.Format(readWebhookRequest)),
			)
		}
		return
	}

	log.Printf("[INFO] Reading webhook - %s", protojson.Format(getWebhookResp))

	state, diags = flattenWebhook(ctx, getWebhookResp.GetWebhook())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// Set state to fully populated data
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

	webhookUpdateReq, diags := extractUpdateWebhookRequest(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	log.Printf("[INFO] Updating Webhook: %s", protojson.Format(webhookUpdateReq))
	webhookUpdateResp, err := r.client.UpdateWebhook(ctx, webhookUpdateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating Webhook",
			formatRpcErrors(err, updateWebhookURL, protojson.Format(webhookUpdateReq)),
		)
		return
	}
	log.Printf("[INFO] Submitted updated Webhhok: %s", protojson.Format(webhookUpdateResp))

	// Get refreshed Webhook value from Coralogix
	id := plan.ID.ValueString()
	getWebhookReq := &webhooks.GetOutgoingWebhookRequest{Id: wrapperspb.String(id)}
	getWebhookResp, err := r.client.GetWebhook(ctx, getWebhookReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if status.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Webhook %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Webhook",
				formatRpcErrors(err, getWebhookURL, protojson.Format(getWebhookReq)),
			)
		}
		return
	}
	log.Printf("[INFO] Received Webhook: %s", protojson.Format(getWebhookResp))

	plan, diags = flattenWebhook(ctx, getWebhookResp.GetWebhook())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	// Set state to fully populated data
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
	log.Printf("[INFO] Deleting Webhook: %s", id)
	deleteReq := &webhooks.DeleteOutgoingWebhookRequest{Id: wrapperspb.String(id)}
	_, err := r.client.DeleteWebhook(ctx, deleteReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error deleting Webhook",
			formatRpcErrors(err, deleteWebhookURL, protojson.Format(deleteReq)),
		)
		return
	}
	log.Printf("[INFO] Deleted Webhook: %s", id)
}

func extractCreateWebhookRequest(ctx context.Context, plan *WebhookResourceModel) (*webhooks.CreateOutgoingWebhookRequest, diag.Diagnostics) {
	data := &webhooks.OutgoingWebhookInputData{
		Name: typeStringToWrapperspbString(plan.Name),
	}

	data, diagnostics := expandWebhookType(ctx, plan, data)
	if diagnostics.HasError() {
		return nil, diagnostics
	}

	return &webhooks.CreateOutgoingWebhookRequest{
		Data: data,
	}, nil
}

func extractUpdateWebhookRequest(ctx context.Context, plan *WebhookResourceModel) (*webhooks.UpdateOutgoingWebhookRequest, diag.Diagnostics) {
	data := &webhooks.OutgoingWebhookInputData{
		Name: typeStringToWrapperspbString(plan.Name),
	}

	data, diagnostics := expandWebhookType(ctx, plan, data)
	if diagnostics.HasError() {
		return nil, diagnostics
	}

	return &webhooks.UpdateOutgoingWebhookRequest{
		Id:   plan.ID.ValueString(),
		Data: data,
	}, nil
}

func expandWebhookType(ctx context.Context, plan *WebhookResourceModel, data *webhooks.OutgoingWebhookInputData) (*webhooks.OutgoingWebhookInputData, diag.Diagnostics) {
	var diags diag.Diagnostics
	if plan.CustomWebhook != nil {
		data.Config, data.Url, diags = expandGenericWebhook(ctx, plan.CustomWebhook)
		data.Type = webhooks.WebhookType_GENERIC
	} else if plan.Slack != nil {
		data.Config, data.Url, diags = expandSlack(ctx, plan.Slack)
		data.Type = webhooks.WebhookType_SLACK
	} else if plan.PagerDuty != nil {
		data.Config = expandPagerDuty(plan.PagerDuty)
		data.Type = webhooks.WebhookType_PAGERDUTY
	} else if plan.SendLog != nil {
		data.Config, data.Url = expandSendLog(plan.SendLog)
		data.Type = webhooks.WebhookType_SEND_LOG
	} else if plan.EmailGroup != nil {
		data.Config, diags = expandEmailGroup(ctx, plan.EmailGroup)
		data.Type = webhooks.WebhookType_EMAIL_GROUP
	} else if plan.MsTeamsWorkflow != nil {
		data.Config, data.Url = expandMicrosoftTeamsWorkflow(plan.MsTeamsWorkflow)
		data.Type = webhooks.WebhookType_MS_TEAMS_WORKFLOW
	} else if plan.Jira != nil {
		data.Config, data.Url = expandJira(plan.Jira)
		data.Type = webhooks.WebhookType_JIRA
	} else if plan.Opsgenie != nil {
		data.Config, data.Url = expandOpsgenie(plan.Opsgenie)
		data.Type = webhooks.WebhookType_OPSGENIE
	} else if plan.Demisto != nil {
		data.Config, data.Url = expandDemisto(plan.Demisto)
		data.Type = webhooks.WebhookType_DEMISTO
	} else if plan.EventBridge != nil {
		data.Config = expandEventBridge(plan.EventBridge)
		data.Type = webhooks.WebhookType_AWS_EVENT_BRIDGE
	} else {
		diags.AddError("Error expanding webhook type", "Unknown webhook type")

	}

	if diags.HasError() {
		return nil, diags
	}

	return data, nil
}

func expandEventBridge(bridge *EventBridgeModel) *webhooks.OutgoingWebhookInputData_AwsEventBridge {
	return &webhooks.OutgoingWebhookInputData_AwsEventBridge{
		AwsEventBridge: &webhooks.AwsEventBridgeConfig{
			EventBusArn: typeStringToWrapperspbString(bridge.EventBusARN),
			Detail:      typeStringToWrapperspbString(bridge.Detail),
			DetailType:  typeStringToWrapperspbString(bridge.DetailType),
			Source:      typeStringToWrapperspbString(bridge.Source),
			RoleName:    typeStringToWrapperspbString(bridge.RoleName),
		},
	}
}

func expandMicrosoftTeamsWorkflow(microsoftTeams *MsTeamsWorkflowModel) (*webhooks.OutgoingWebhookInputData_MsTeamsWorkflow, *wrapperspb.StringValue) {
	var url *wrapperspb.StringValue
	if planUrl := microsoftTeams.URL; !(planUrl.IsNull() || planUrl.IsUnknown()) {
		url = wrapperspb.String(planUrl.ValueString())
	}

	return &webhooks.OutgoingWebhookInputData_MsTeamsWorkflow{
		MsTeamsWorkflow: &webhooks.MSTeamsWorkflowConfig{},
	}, url
}

func expandSlack(ctx context.Context, slack *SlackModel) (*webhooks.OutgoingWebhookInputData_Slack, *wrapperspb.StringValue, diag.Diagnostics) {
	digests, diags := expandDigests(ctx, slack.NotifyAbout)
	if diags.HasError() {
		return nil, nil, diags
	}

	attachments, diags := expandSlackAttachments(ctx, slack.Attachments)
	if diags.HasError() {
		return nil, nil, diags
	}

	var url *wrapperspb.StringValue
	if planUrl := slack.URL; !(planUrl.IsNull() || planUrl.IsUnknown()) {
		url = wrapperspb.String(planUrl.ValueString())
	}

	return &webhooks.OutgoingWebhookInputData_Slack{
		Slack: &webhooks.SlackConfig{
			Digests:     digests,
			Attachments: attachments,
		},
	}, url, nil
}

func expandSlackAttachments(ctx context.Context, attachmentsList types.List) ([]*webhooks.SlackConfig_Attachment, diag.Diagnostics) {
	var attachmentsObjects []types.Object
	var expandedAttachments []*webhooks.SlackConfig_Attachment
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
		expandedAttachment := &webhooks.SlackConfig_Attachment{
			Type:     webhooksProtoToSchemaSlackAttachmentType[attachmentModel.Type.ValueString()],
			IsActive: typeBoolToWrapperspbBool(attachmentModel.Active),
		}
		expandedAttachments = append(expandedAttachments, expandedAttachment)
	}
	return expandedAttachments, diags
}

func expandDigests(ctx context.Context, digestsSet types.Set) ([]*webhooks.SlackConfig_Digest, diag.Diagnostics) {
	digests := digestsSet.Elements()
	expandedDigests := make([]*webhooks.SlackConfig_Digest, 0, len(digests))
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

func expandDigest(digest webhooks.SlackConfig_DigestType) *webhooks.SlackConfig_Digest {
	return &webhooks.SlackConfig_Digest{
		Type:     digest,
		IsActive: wrapperspb.Bool(true),
	}
}

func expandGenericWebhook(ctx context.Context, genericWebhook *CustomWebhookModel) (*webhooks.OutgoingWebhookInputData_GenericWebhook, *wrapperspb.StringValue, diag.Diagnostics) {
	headers, diags := typeMapToStringMap(ctx, genericWebhook.Headers)
	if diags.HasError() {
		return nil, nil, diags
	}

	var url *wrapperspb.StringValue
	if planUrl := genericWebhook.URL; !(planUrl.IsNull() || planUrl.IsUnknown()) {
		url = wrapperspb.String(planUrl.ValueString())
	}

	return &webhooks.OutgoingWebhookInputData_GenericWebhook{
		GenericWebhook: &webhooks.GenericWebhookConfig{
			Uuid:    expandUuid(genericWebhook.UUID),
			Method:  webhooksSchemaToProtoMethod[genericWebhook.Method.ValueString()],
			Headers: headers,
			Payload: typeStringToWrapperspbString(genericWebhook.Payload),
		},
	}, url, nil
}

func expandPagerDuty(pagerDuty *PagerDutyModel) *webhooks.OutgoingWebhookInputData_PagerDuty {
	return &webhooks.OutgoingWebhookInputData_PagerDuty{
		PagerDuty: &webhooks.PagerDutyConfig{
			ServiceKey: typeStringToWrapperspbString(pagerDuty.ServiceKey),
		},
	}
}

func expandSendLog(sendLog *SendLogModel) (*webhooks.OutgoingWebhookInputData_SendLog, *wrapperspb.StringValue) {
	var url *wrapperspb.StringValue
	if planUrl := sendLog.URL; !(planUrl.IsNull() || planUrl.IsUnknown()) {
		url = wrapperspb.String(planUrl.ValueString())
	}

	return &webhooks.OutgoingWebhookInputData_SendLog{
		SendLog: &webhooks.SendLogConfig{
			Uuid:    expandUuid(sendLog.UUID),
			Payload: typeStringToWrapperspbString(sendLog.Payload),
		},
	}, url
}

func expandEmailGroup(ctx context.Context, emailGroup *EmailGroupModel) (*webhooks.OutgoingWebhookInputData_EmailGroup, diag.Diagnostics) {
	emailAddresses, diags := typeStringSliceToWrappedStringSlice(ctx, emailGroup.Emails.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &webhooks.OutgoingWebhookInputData_EmailGroup{
		EmailGroup: &webhooks.EmailGroupConfig{
			EmailAddresses: emailAddresses,
		},
	}, nil
}

func expandJira(jira *JiraModel) (*webhooks.OutgoingWebhookInputData_Jira, *wrapperspb.StringValue) {
	var url *wrapperspb.StringValue
	if planUrl := jira.URL; !(planUrl.IsNull() || planUrl.IsUnknown()) {
		url = wrapperspb.String(planUrl.ValueString())
	}

	return &webhooks.OutgoingWebhookInputData_Jira{
		Jira: &webhooks.JiraConfig{
			ApiToken:   typeStringToWrapperspbString(jira.ApiKey),
			Email:      typeStringToWrapperspbString(jira.Email),
			ProjectKey: typeStringToWrapperspbString(jira.ProjectID),
		},
	}, url
}

func expandOpsgenie(opsgenie *OpsgenieModel) (*webhooks.OutgoingWebhookInputData_Opsgenie, *wrapperspb.StringValue) {
	var url *wrapperspb.StringValue
	if planUrl := opsgenie.URL; !(planUrl.IsNull() || planUrl.IsUnknown()) {
		url = wrapperspb.String(planUrl.ValueString())
	}

	return &webhooks.OutgoingWebhookInputData_Opsgenie{
		Opsgenie: &webhooks.OpsgenieConfig{},
	}, url
}

func expandDemisto(demisto *DemistoModel) (*webhooks.OutgoingWebhookInputData_Demisto, *wrapperspb.StringValue) {
	var url *wrapperspb.StringValue
	if planUrl := demisto.URL; !(planUrl.IsNull() || planUrl.IsUnknown()) {
		url = wrapperspb.String(planUrl.ValueString())
	}

	return &webhooks.OutgoingWebhookInputData_Demisto{
		Demisto: &webhooks.DemistoConfig{
			Uuid:    expandUuid(demisto.UUID),
			Payload: typeStringToWrapperspbString(demisto.Payload),
		},
	}, url
}

func flattenWebhook(ctx context.Context, webhook *webhooks.OutgoingWebhook) (*WebhookResourceModel, diag.Diagnostics) {
	result := &WebhookResourceModel{
		ID:         wrapperspbStringToTypeString(webhook.Id),
		ExternalID: types.StringValue(strconv.Itoa(int(webhook.GetExternalId().GetValue()))),
		Name:       wrapperspbStringToTypeString(webhook.Name),
	}

	url := webhook.GetUrl()
	var diags diag.Diagnostics
	switch configType := webhook.Config.(type) {
	case *webhooks.OutgoingWebhook_Slack:
		result.Slack, diags = flattenSlack(ctx, configType.Slack, url)
	case *webhooks.OutgoingWebhook_GenericWebhook:
		result.CustomWebhook, diags = flattenGenericWebhook(ctx, configType.GenericWebhook, url)
	case *webhooks.OutgoingWebhook_PagerDuty:
		result.PagerDuty = flattenPagerDuty(configType.PagerDuty)
	case *webhooks.OutgoingWebhook_SendLog:
		result.SendLog = flattenSendLog(configType.SendLog, url)
	case *webhooks.OutgoingWebhook_EmailGroup:
		result.EmailGroup = flattenEmailGroup(configType.EmailGroup)
	case *webhooks.OutgoingWebhook_MsTeamsWorkflow:
		result.MsTeamsWorkflow = flattenMsTeamsWorkflow(configType.MsTeamsWorkflow, url)
	case *webhooks.OutgoingWebhook_Jira:
		result.Jira = flattenJira(configType.Jira, url)
	case *webhooks.OutgoingWebhook_Opsgenie:
		result.Opsgenie = flattenOpsgenie(configType.Opsgenie, url)
	case *webhooks.OutgoingWebhook_Demisto:
		result.Demisto = flattenDemisto(configType.Demisto, url)
	case *webhooks.OutgoingWebhook_AwsEventBridge:
		result.EventBridge = flattenEventBridge(configType.AwsEventBridge)
	default:
		diags.AddError("Error flattening webhook", fmt.Sprintf("Unknown webhook type: %T", configType))
	}

	return result, diags
}

func flattenGenericWebhook(ctx context.Context, genericWebhook *webhooks.GenericWebhookConfig, url *wrapperspb.StringValue) (*CustomWebhookModel, diag.Diagnostics) {
	headers, diags := types.MapValueFrom(ctx, types.StringType, genericWebhook.Headers)
	return &CustomWebhookModel{
		UUID:    wrapperspbStringToTypeString(genericWebhook.Uuid),
		Method:  types.StringValue(webhooksProtoToSchemaMethod[genericWebhook.Method]),
		Headers: headers,
		Payload: wrapperspbStringToTypeString(genericWebhook.Payload),
		URL:     wrapperspbStringToTypeString(url),
	}, diags
}

func flattenSlack(ctx context.Context, slack *webhooks.SlackConfig, url *wrapperspb.StringValue) (*SlackModel, diag.Diagnostics) {
	digests, diags := flattenDigests(ctx, slack.GetDigests())
	if diags.HasError() {
		return nil, diags
	}

	attachments, diags := flattenSlackAttachments(ctx, slack.GetAttachments())
	if diags.HasError() {
		return nil, diags
	}

	return &SlackModel{
		NotifyAbout: digests,
		URL:         wrapperspbStringToTypeString(url),
		Attachments: attachments,
	}, nil
}

func flattenSlackAttachments(ctx context.Context, attachments []*webhooks.SlackConfig_Attachment) (types.List, diag.Diagnostics) {
	if len(attachments) == 0 {
		return types.ListNull(types.ObjectType{AttrTypes: slackAttachmentsAttr()}), nil
	}

	attachmentsElements := make([]SlackAttachmentModel, 0, len(attachments))
	for _, attachment := range attachments {
		flattenedAttachment := SlackAttachmentModel{
			Type:   types.StringValue(webhooksSchemaToProtoSlackAttachmentType[attachment.GetType()]),
			Active: types.BoolValue(attachment.GetIsActive().GetValue()),
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

func flattenDigests(ctx context.Context, digests []*webhooks.SlackConfig_Digest) (types.Set, diag.Diagnostics) {
	if len(digests) == 0 {
		return types.SetNull(types.StringType), nil
	}

	digestsElements := make([]attr.Value, 0, len(digests))
	for _, digest := range digests {
		flattenedDigest := flattenDigest(digest)
		digestsElements = append(digestsElements, flattenedDigest)
	}

	return types.SetValueFrom(ctx, types.StringType, digestsElements)
}

func flattenDigest(digest *webhooks.SlackConfig_Digest) types.String {
	return types.StringValue(webhooksProtoToSchemaSlackConfigDigestType[digest.GetType()])
}

func flattenPagerDuty(pagerDuty *webhooks.PagerDutyConfig) *PagerDutyModel {
	return &PagerDutyModel{
		ServiceKey: wrapperspbStringToTypeString(pagerDuty.ServiceKey),
	}
}

func flattenSendLog(sendLog *webhooks.SendLogConfig, url *wrapperspb.StringValue) *SendLogModel {
	return &SendLogModel{
		UUID:    wrapperspbStringToTypeString(sendLog.Uuid),
		Payload: wrapperspbStringToTypeString(sendLog.Payload),
		URL:     wrapperspbStringToTypeString(url),
	}
}

func flattenEmailGroup(emailGroup *webhooks.EmailGroupConfig) *EmailGroupModel {
	return &EmailGroupModel{
		Emails: wrappedStringSliceToTypeStringList(emailGroup.EmailAddresses),
	}
}

func flattenMsTeamsWorkflow(_ *webhooks.MSTeamsWorkflowConfig, url *wrapperspb.StringValue) *MsTeamsWorkflowModel {
	return &MsTeamsWorkflowModel{
		URL: wrapperspbStringToTypeString(url),
	}
}

func flattenJira(jira *webhooks.JiraConfig, url *wrapperspb.StringValue) *JiraModel {
	return &JiraModel{
		ApiKey:    wrapperspbStringToTypeString(jira.ApiToken),
		Email:     wrapperspbStringToTypeString(jira.Email),
		ProjectID: wrapperspbStringToTypeString(jira.ProjectKey),
		URL:       wrapperspbStringToTypeString(url),
	}
}

func flattenOpsgenie(_ *webhooks.OpsgenieConfig, url *wrapperspb.StringValue) *OpsgenieModel {
	return &OpsgenieModel{
		URL: wrapperspbStringToTypeString(url),
	}
}

func flattenDemisto(demisto *webhooks.DemistoConfig, url *wrapperspb.StringValue) *DemistoModel {
	return &DemistoModel{
		UUID:    wrapperspbStringToTypeString(demisto.Uuid),
		Payload: wrapperspbStringToTypeString(demisto.Payload),
		URL:     wrapperspbStringToTypeString(url),
	}
}

func flattenEventBridge(bridge *webhooks.AwsEventBridgeConfig) *EventBridgeModel {
	return &EventBridgeModel{
		EventBusARN: wrapperspbStringToTypeString(bridge.EventBusArn),
		Detail:      wrapperspbStringToTypeString(bridge.Detail),
		DetailType:  wrapperspbStringToTypeString(bridge.DetailType),
		Source:      wrapperspbStringToTypeString(bridge.Source),
		RoleName:    wrapperspbStringToTypeString(bridge.RoleName),
	}
}
