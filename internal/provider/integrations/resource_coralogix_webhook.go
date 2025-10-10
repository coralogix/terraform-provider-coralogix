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
	"strconv"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"

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
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	_                           resource.ResourceWithConfigure   = &WebhookResource{}
	_                           resource.ResourceWithImportState = &WebhookResource{}
	webhooksSchemaToProtoMethod                                  = map[string]cxsdk.GenericWebhookConfigMethodType{
		"get":  cxsdk.GenericWebhookConfigGet,
		"post": cxsdk.GenericWebhookConfigPost,
		"put":  cxsdk.GenericWebhookConfigPut,
	}
	webhooksProtoToSchemaMethod                = utils.ReverseMap(webhooksSchemaToProtoMethod)
	webhooksValidMethods                       = utils.GetKeys(webhooksSchemaToProtoMethod)
	webhooksSchemaToProtoSlackConfigDigestType = map[string]cxsdk.SlackConfigDigestType{
		"error_and_critical_logs": cxsdk.SlackConfigErrorAndCriticalLogs,
		"flow_anomalies":          cxsdk.SlackConfigFlowAnomalies,
		"spike_anomalies":         cxsdk.SlackConfigSpikeAnomalies,
		"data_usage":              cxsdk.SlackConfigDataUsage,
	}
	webhooksProtoToSchemaSlackConfigDigestType = utils.ReverseMap(webhooksSchemaToProtoSlackConfigDigestType)
	webhooksValidSlackConfigDigestTypes        = utils.GetKeys(webhooksSchemaToProtoSlackConfigDigestType)
	webhooksProtoToSchemaSlackAttachmentType   = map[string]cxsdk.SlackConfigAttachmentType{
		"empty":           cxsdk.SlackConfigEmpty,
		"metric_snapshot": cxsdk.SlackConfigMetricSnapshot,
		"logs":            cxsdk.SlackConfigLogs,
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
	client *cxsdk.WebhooksClient
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

	createWebhookRequest, diags := extractCreateWebhookRequest(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	webhookStr := protojson.Format(createWebhookRequest)
	log.Printf("[INFO] Creating new webhook: %s", webhookStr)
	createResp, err := r.client.Create(ctx, createWebhookRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error creating Webhook",
			utils.FormatRpcErrors(err, cxsdk.OutgoingWebhookCreateRPC, webhookStr),
		)
		return
	}
	id := createResp.Id.GetValue()
	log.Printf("[INFO] Submitted new webhook, id - %s", id)

	readWebhookRequest := &cxsdk.GetOutgoingWebhookRequest{
		Id: wrapperspb.String(id),
	}
	getWebhookResp, err := r.client.Get(ctx, readWebhookRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error reading Webhook",
			utils.FormatRpcErrors(err, cxsdk.OutgoingWebhookGetRPC, protojson.Format(readWebhookRequest)),
		)
		return
	}

	getWebhookStr := protojson.Format(getWebhookResp)
	log.Printf("[INFO] Reading webhook - %s", getWebhookStr)

	plan, diags = flattenWebhookWrite(ctx, getWebhookResp.GetWebhook())
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
	readWebhookRequest := &cxsdk.GetOutgoingWebhookRequest{
		Id: wrapperspb.String(id),
	}

	log.Printf("[INFO] Reading Webhook: %s", id)
	getWebhookResp, err := r.client.Get(ctx, readWebhookRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Webhook %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Webhook",
				utils.FormatRpcErrors(err, cxsdk.OutgoingWebhookGetRPC, protojson.Format(readWebhookRequest)),
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
	webhookUpdateResp, err := r.client.Update(ctx, webhookUpdateReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error updating Webhook",
			utils.FormatRpcErrors(err, cxsdk.OutgoingWebhookUpdateRPC, protojson.Format(webhookUpdateReq)),
		)
		return
	}
	log.Printf("[INFO] Submitted updated Webhhok: %s", protojson.Format(webhookUpdateResp))

	// Get refreshed Webhook value from Coralogix
	id := plan.ID.ValueString()
	getWebhookReq := &cxsdk.GetOutgoingWebhookRequest{Id: wrapperspb.String(id)}
	getWebhookResp, err := r.client.Get(ctx, getWebhookReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		if cxsdk.Code(err) == codes.NotFound {
			resp.Diagnostics.AddWarning(
				fmt.Sprintf("Webhook %q is in state, but no longer exists in Coralogix backend", id),
				fmt.Sprintf("%s will be recreated when you apply", id),
			)
			resp.State.RemoveResource(ctx)
		} else {
			resp.Diagnostics.AddError(
				"Error reading Webhook",
				utils.FormatRpcErrors(err, cxsdk.OutgoingWebhookGetRPC, protojson.Format(getWebhookReq)),
			)
		}
		return
	}
	log.Printf("[INFO] Received Webhook: %s", protojson.Format(getWebhookResp))

	plan, diags = flattenWebhookWrite(ctx, getWebhookResp.GetWebhook())
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
	deleteReq := &cxsdk.DeleteOutgoingWebhookRequest{Id: wrapperspb.String(id)}
	_, err := r.client.Delete(ctx, deleteReq)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		resp.Diagnostics.AddError(
			"Error deleting Webhook",
			utils.FormatRpcErrors(err, cxsdk.OutgoingWebhookDeleteRPC, protojson.Format(deleteReq)),
		)
		return
	}
	log.Printf("[INFO] Deleted Webhook: %s", id)
}

func extractCreateWebhookRequest(ctx context.Context, plan *WebhookResourceModel) (*cxsdk.CreateOutgoingWebhookRequest, diag.Diagnostics) {
	data := &cxsdk.OutgoingWebhookInputData{
		Name: utils.TypeStringToWrapperspbString(plan.Name),
	}

	data, diagnostics := expandWebhookType(ctx, plan, data)
	if diagnostics.HasError() {
		return nil, diagnostics
	}

	return &cxsdk.CreateOutgoingWebhookRequest{
		Data: data,
	}, nil
}

func extractUpdateWebhookRequest(ctx context.Context, plan *WebhookResourceModel) (*cxsdk.UpdateOutgoingWebhookRequest, diag.Diagnostics) {
	data := &cxsdk.OutgoingWebhookInputData{
		Name: utils.TypeStringToWrapperspbString(plan.Name),
	}

	data, diagnostics := expandWebhookType(ctx, plan, data)
	if diagnostics.HasError() {
		return nil, diagnostics
	}

	return &cxsdk.UpdateOutgoingWebhookRequest{
		Id:   plan.ID.ValueString(),
		Data: data,
	}, nil
}

func expandWebhookType(ctx context.Context, plan *WebhookResourceModel, data *cxsdk.OutgoingWebhookInputData) (*cxsdk.OutgoingWebhookInputData, diag.Diagnostics) {
	var diags diag.Diagnostics
	if plan.CustomWebhook != nil {
		data.Config, data.Url, diags = expandGenericWebhook(ctx, plan.CustomWebhook)
		data.Type = cxsdk.WebhookTypeGeneric
	} else if plan.Slack != nil {
		data.Config, data.Url, diags = expandSlack(ctx, plan.Slack)
		data.Type = cxsdk.WebhookTypeSlack
	} else if plan.PagerDuty != nil {
		data.Config = expandPagerDuty(plan.PagerDuty)
		data.Type = cxsdk.WebhookTypePagerduty
	} else if plan.SendLog != nil {
		data.Config, data.Url = expandSendLog(plan.SendLog)
		data.Type = cxsdk.WebhookTypeSendLog
	} else if plan.EmailGroup != nil {
		data.Config, diags = expandEmailGroup(ctx, plan.EmailGroup)
		data.Type = cxsdk.WebhookTypeEmailGroup
	} else if plan.MsTeamsWorkflow != nil {
		data.Config, data.Url = expandMicrosoftTeamsWorkflow(plan.MsTeamsWorkflow)
		data.Type = cxsdk.WebhookTypeMicrosoftTeamsWorkflow
	} else if plan.Jira != nil {
		data.Config, data.Url = expandJira(plan.Jira)
		data.Type = cxsdk.WebhookTypeJira
	} else if plan.Opsgenie != nil {
		data.Config, data.Url = expandOpsgenie(plan.Opsgenie)
		data.Type = cxsdk.WebhookTypeOpsgenie
	} else if plan.Demisto != nil {
		data.Config, data.Url = expandDemisto(plan.Demisto)
		data.Type = cxsdk.WebhookTypeDemisto
	} else if plan.EventBridge != nil {
		data.Config = expandEventBridge(plan.EventBridge)
		data.Type = cxsdk.WebhookTypeAwsEventBridge
	} else {
		diags.AddError("Error expanding webhook type", "Unknown webhook type")

	}

	if diags.HasError() {
		return nil, diags
	}

	return data, nil
}

func expandEventBridge(bridge *EventBridgeModel) *cxsdk.AwsEventBridgeWebhookInputData {
	return &cxsdk.AwsEventBridgeWebhookInputData{
		AwsEventBridge: &cxsdk.AwsEventBridgeConfig{
			EventBusArn: utils.TypeStringToWrapperspbString(bridge.EventBusARN),
			Detail:      utils.TypeStringToWrapperspbString(bridge.Detail),
			DetailType:  utils.TypeStringToWrapperspbString(bridge.DetailType),
			Source:      utils.TypeStringToWrapperspbString(bridge.Source),
			RoleName:    utils.TypeStringToWrapperspbString(bridge.RoleName),
		},
	}
}

func expandMicrosoftTeamsWorkflow(microsoftTeams *MsTeamsWorkflowModel) (*cxsdk.MsTeamsWorkflowInputData, *wrapperspb.StringValue) {
	var url *wrapperspb.StringValue
	if planUrl := microsoftTeams.URL; !(planUrl.IsNull() || planUrl.IsUnknown()) {
		url = wrapperspb.String(planUrl.ValueString())
	}

	return &cxsdk.MsTeamsWorkflowInputData{
		MsTeamsWorkflow: &cxsdk.MSTeamsWorkflowConfig{},
	}, url
}

func expandMicrosoftTeams(microsoftTeams *MsTeamsWorkflowModel) (*cxsdk.MicrosoftTeamsWebhookInputData, *wrapperspb.StringValue) {
	var url *wrapperspb.StringValue
	if planUrl := microsoftTeams.URL; !(planUrl.IsNull() || planUrl.IsUnknown()) {
		url = wrapperspb.String(planUrl.ValueString())
	}

	return &cxsdk.MicrosoftTeamsWebhookInputData{
		MicrosoftTeams: &cxsdk.MicrosoftTeamsConfig{},
	}, url
}

func expandSlack(ctx context.Context, slack *SlackModel) (*cxsdk.SlackWebhookInputData, *wrapperspb.StringValue, diag.Diagnostics) {
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

	return &cxsdk.SlackWebhookInputData{
		Slack: &cxsdk.SlackConfig{
			Digests:     digests,
			Attachments: attachments,
		},
	}, url, nil
}

func expandSlackAttachments(ctx context.Context, attachmentsList types.List) ([]*cxsdk.SlackConfigAttachment, diag.Diagnostics) {
	var attachmentsObjects []types.Object
	var expandedAttachments []*cxsdk.SlackConfigAttachment
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
		expandedAttachment := &cxsdk.SlackConfigAttachment{
			Type:     webhooksProtoToSchemaSlackAttachmentType[attachmentModel.Type.ValueString()],
			IsActive: utils.TypeBoolToWrapperspbBool(attachmentModel.Active),
		}
		expandedAttachments = append(expandedAttachments, expandedAttachment)
	}
	return expandedAttachments, diags
}

func expandDigests(ctx context.Context, digestsSet types.Set) ([]*cxsdk.SlackConfigDigest, diag.Diagnostics) {
	digests := digestsSet.Elements()
	expandedDigests := make([]*cxsdk.SlackConfigDigest, 0, len(digests))
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

func expandDigest(digest cxsdk.SlackConfigDigestType) *cxsdk.SlackConfigDigest {
	return &cxsdk.SlackConfigDigest{
		Type:     digest,
		IsActive: wrapperspb.Bool(true),
	}
}

func expandGenericWebhook(ctx context.Context, genericWebhook *CustomWebhookModel) (*cxsdk.GenericWebhookInputData, *wrapperspb.StringValue, diag.Diagnostics) {
	headers, diags := utils.TypeMapToStringMap(ctx, genericWebhook.Headers)
	if diags.HasError() {
		return nil, nil, diags
	}

	var url *wrapperspb.StringValue
	if planUrl := genericWebhook.URL; !(planUrl.IsNull() || planUrl.IsUnknown()) {
		url = wrapperspb.String(planUrl.ValueString())
	}

	return &cxsdk.GenericWebhookInputData{
		GenericWebhook: &cxsdk.GenericWebhookConfig{
			Uuid:    utils.ExpandUuid(genericWebhook.UUID),
			Method:  webhooksSchemaToProtoMethod[genericWebhook.Method.ValueString()],
			Headers: headers,
			Payload: utils.TypeStringToWrapperspbString(genericWebhook.Payload),
		},
	}, url, nil
}

func expandPagerDuty(pagerDuty *PagerDutyModel) *cxsdk.PagerDutyWebhookInputData {
	return &cxsdk.PagerDutyWebhookInputData{
		PagerDuty: &cxsdk.PagerDutyConfig{
			ServiceKey: utils.TypeStringToWrapperspbString(pagerDuty.ServiceKey),
		},
	}
}

func expandSendLog(sendLog *SendLogModel) (*cxsdk.SendLogWebhookInputData, *wrapperspb.StringValue) {
	var url *wrapperspb.StringValue
	if planUrl := sendLog.URL; !(planUrl.IsNull() || planUrl.IsUnknown()) {
		url = wrapperspb.String(planUrl.ValueString())
	}

	return &cxsdk.SendLogWebhookInputData{
		SendLog: &cxsdk.SendLogConfig{
			Uuid:    utils.ExpandUuid(sendLog.UUID),
			Payload: utils.TypeStringToWrapperspbString(sendLog.Payload),
		},
	}, url
}

func expandEmailGroup(ctx context.Context, emailGroup *EmailGroupModel) (*cxsdk.EmailGroupWebhookInputData, diag.Diagnostics) {
	emailAddresses, diags := utils.TypeStringSliceToWrappedStringSlice(ctx, emailGroup.Emails.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &cxsdk.EmailGroupWebhookInputData{
		EmailGroup: &cxsdk.EmailGroupConfig{
			EmailAddresses: emailAddresses,
		},
	}, nil
}

func expandJira(jira *JiraModel) (*cxsdk.JiraWebhookInputData, *wrapperspb.StringValue) {
	var url *wrapperspb.StringValue
	if planUrl := jira.URL; !(planUrl.IsNull() || planUrl.IsUnknown()) {
		url = wrapperspb.String(planUrl.ValueString())
	}

	return &cxsdk.JiraWebhookInputData{
		Jira: &cxsdk.JiraConfig{
			ApiToken:   utils.TypeStringToWrapperspbString(jira.ApiKey),
			Email:      utils.TypeStringToWrapperspbString(jira.Email),
			ProjectKey: utils.TypeStringToWrapperspbString(jira.ProjectID),
		},
	}, url
}

func expandOpsgenie(opsgenie *OpsgenieModel) (*cxsdk.OpsgenieWebhookInputData, *wrapperspb.StringValue) {
	var url *wrapperspb.StringValue
	if planUrl := opsgenie.URL; !(planUrl.IsNull() || planUrl.IsUnknown()) {
		url = wrapperspb.String(planUrl.ValueString())
	}

	return &cxsdk.OpsgenieWebhookInputData{
		Opsgenie: &cxsdk.OpsgenieConfig{},
	}, url
}

func expandDemisto(demisto *DemistoModel) (*cxsdk.DemistoWebhookInputData, *wrapperspb.StringValue) {
	var url *wrapperspb.StringValue
	if planUrl := demisto.URL; !(planUrl.IsNull() || planUrl.IsUnknown()) {
		url = wrapperspb.String(planUrl.ValueString())
	}

	return &cxsdk.DemistoWebhookInputData{
		Demisto: &cxsdk.DemistoConfig{
			Uuid:    utils.ExpandUuid(demisto.UUID),
			Payload: utils.TypeStringToWrapperspbString(demisto.Payload),
		},
	}, url
}

// Temporary function to prevent the creation of depreacted resources
func flattenWebhookWrite(ctx context.Context, webhook *cxsdk.OutgoingWebhook) (*WebhookResourceModel, diag.Diagnostics) {
	result := &WebhookResourceModel{
		ID:         utils.WrapperspbStringToTypeString(webhook.Id),
		ExternalID: types.StringValue(strconv.Itoa(int(webhook.GetExternalId().GetValue()))),
		Name:       utils.WrapperspbStringToTypeString(webhook.Name),
	}

	url := webhook.GetUrl()
	var diags diag.Diagnostics
	switch configType := webhook.Config.(type) {
	case *cxsdk.SlackWebhook:
		result.Slack, diags = flattenSlack(ctx, configType.Slack, url)
	case *cxsdk.GenericWebhook:
		result.CustomWebhook, diags = flattenGenericWebhook(ctx, configType.GenericWebhook, url)
	case *cxsdk.PagerDutyWebhook:
		result.PagerDuty = flattenPagerDuty(configType.PagerDuty)
	case *cxsdk.SendLogWebhook:
		result.SendLog = flattenSendLog(configType.SendLog, url)
	case *cxsdk.EmailGroupWebhook:
		result.EmailGroup = flattenEmailGroup(configType.EmailGroup)
	case *cxsdk.MsTeamsWorkflowWebhook:
		result.MsTeamsWorkflow = flattenMsTeamsWorkflow(configType.MsTeamsWorkflow, url)
	case *cxsdk.JiraWebhook:
		result.Jira = flattenJira(configType.Jira, url)
	case *cxsdk.OpsgenieWebhook:
		result.Opsgenie = flattenOpsgenie(configType.Opsgenie, url)
	case *cxsdk.DemistoWebhook:
		result.Demisto = flattenDemisto(configType.Demisto, url)
	case *cxsdk.AwsEventBridgeWebhook:
		result.EventBridge = flattenEventBridge(configType.AwsEventBridge)
	default:
		diags.AddError("Error flattening webhook", fmt.Sprintf("Unknown webhook type: %T", configType))
	}

	return result, diags
}

func flattenWebhook(ctx context.Context, webhook *cxsdk.OutgoingWebhook) (*WebhookResourceModel, diag.Diagnostics) {
	result := &WebhookResourceModel{
		ID:         utils.WrapperspbStringToTypeString(webhook.Id),
		ExternalID: types.StringValue(strconv.Itoa(int(webhook.GetExternalId().GetValue()))),
		Name:       utils.WrapperspbStringToTypeString(webhook.Name),
	}

	url := webhook.GetUrl()
	var diags diag.Diagnostics
	switch configType := webhook.Config.(type) {
	case *cxsdk.SlackWebhook:
		result.Slack, diags = flattenSlack(ctx, configType.Slack, url)
	case *cxsdk.GenericWebhook:
		result.CustomWebhook, diags = flattenGenericWebhook(ctx, configType.GenericWebhook, url)
	case *cxsdk.PagerDutyWebhook:
		result.PagerDuty = flattenPagerDuty(configType.PagerDuty)
	case *cxsdk.SendLogWebhook:
		result.SendLog = flattenSendLog(configType.SendLog, url)
	case *cxsdk.EmailGroupWebhook:
		result.EmailGroup = flattenEmailGroup(configType.EmailGroup)
	case *cxsdk.MsTeamsWorkflowWebhook:
		result.MsTeamsWorkflow = flattenMsTeamsWorkflow(configType.MsTeamsWorkflow, url)
	case *cxsdk.MicrosoftTeamsWebhook:
		result.MsTeams = flattenMicrosoftTeams(configType.MicrosoftTeams, url)
	case *cxsdk.JiraWebhook:
		result.Jira = flattenJira(configType.Jira, url)
	case *cxsdk.OpsgenieWebhook:
		result.Opsgenie = flattenOpsgenie(configType.Opsgenie, url)
	case *cxsdk.DemistoWebhook:
		result.Demisto = flattenDemisto(configType.Demisto, url)
	case *cxsdk.AwsEventBridgeWebhook:
		result.EventBridge = flattenEventBridge(configType.AwsEventBridge)
	default:
		diags.AddError("Error flattening webhook", fmt.Sprintf("Unknown webhook type: %T", configType))
	}

	return result, diags
}

func flattenGenericWebhook(ctx context.Context, genericWebhook *cxsdk.GenericWebhookConfig, url *wrapperspb.StringValue) (*CustomWebhookModel, diag.Diagnostics) {
	headers, diags := types.MapValueFrom(ctx, types.StringType, genericWebhook.Headers)
	return &CustomWebhookModel{
		UUID:    utils.WrapperspbStringToTypeString(genericWebhook.Uuid),
		Method:  types.StringValue(webhooksProtoToSchemaMethod[genericWebhook.Method]),
		Headers: headers,
		Payload: utils.WrapperspbStringToTypeString(genericWebhook.Payload),
		URL:     utils.WrapperspbStringToTypeString(url),
	}, diags
}

func flattenSlack(ctx context.Context, slack *cxsdk.SlackConfig, url *wrapperspb.StringValue) (*SlackModel, diag.Diagnostics) {
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
		URL:         utils.WrapperspbStringToTypeString(url),
		Attachments: attachments,
	}, nil
}

func flattenSlackAttachments(ctx context.Context, attachments []*cxsdk.SlackConfigAttachment) (types.List, diag.Diagnostics) {
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

func flattenDigests(ctx context.Context, digests []*cxsdk.SlackConfigDigest) (types.Set, diag.Diagnostics) {
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

func flattenDigest(digest *cxsdk.SlackConfigDigest) types.String {
	return types.StringValue(webhooksProtoToSchemaSlackConfigDigestType[digest.GetType()])
}

func flattenPagerDuty(pagerDuty *cxsdk.PagerDutyConfig) *PagerDutyModel {
	return &PagerDutyModel{
		ServiceKey: utils.WrapperspbStringToTypeString(pagerDuty.ServiceKey),
	}
}

func flattenSendLog(sendLog *cxsdk.SendLogConfig, url *wrapperspb.StringValue) *SendLogModel {
	return &SendLogModel{
		UUID:    utils.WrapperspbStringToTypeString(sendLog.Uuid),
		Payload: utils.WrapperspbStringToTypeString(sendLog.Payload),
		URL:     utils.WrapperspbStringToTypeString(url),
	}
}

func flattenEmailGroup(emailGroup *cxsdk.EmailGroupConfig) *EmailGroupModel {
	return &EmailGroupModel{
		Emails: utils.WrappedStringSliceToTypeStringList(emailGroup.EmailAddresses),
	}
}

func flattenMsTeamsWorkflow(_ *cxsdk.MSTeamsWorkflowConfig, url *wrapperspb.StringValue) *MsTeamsWorkflowModel {
	return &MsTeamsWorkflowModel{
		URL: utils.WrapperspbStringToTypeString(url),
	}
}

func flattenMicrosoftTeams(microsoftTeamsConfig *cxsdk.MicrosoftTeamsConfig, url *wrapperspb.StringValue) *MsTeamsWorkflowModel {
	return &MsTeamsWorkflowModel{
		URL: utils.WrapperspbStringToTypeString(url),
	}
}

func flattenJira(jira *cxsdk.JiraConfig, url *wrapperspb.StringValue) *JiraModel {
	return &JiraModel{
		ApiKey:    utils.WrapperspbStringToTypeString(jira.ApiToken),
		Email:     utils.WrapperspbStringToTypeString(jira.Email),
		ProjectID: utils.WrapperspbStringToTypeString(jira.ProjectKey),
		URL:       utils.WrapperspbStringToTypeString(url),
	}
}

func flattenOpsgenie(_ *cxsdk.OpsgenieConfig, url *wrapperspb.StringValue) *OpsgenieModel {
	return &OpsgenieModel{
		URL: utils.WrapperspbStringToTypeString(url),
	}
}

func flattenDemisto(demisto *cxsdk.DemistoConfig, url *wrapperspb.StringValue) *DemistoModel {
	return &DemistoModel{
		UUID:    utils.WrapperspbStringToTypeString(demisto.Uuid),
		Payload: utils.WrapperspbStringToTypeString(demisto.Payload),
		URL:     utils.WrapperspbStringToTypeString(url),
	}
}

func flattenEventBridge(bridge *cxsdk.AwsEventBridgeConfig) *EventBridgeModel {
	return &EventBridgeModel{
		EventBusARN: utils.WrapperspbStringToTypeString(bridge.EventBusArn),
		Detail:      utils.WrapperspbStringToTypeString(bridge.Detail),
		DetailType:  utils.WrapperspbStringToTypeString(bridge.DetailType),
		Source:      utils.WrapperspbStringToTypeString(bridge.Source),
		RoleName:    utils.WrapperspbStringToTypeString(bridge.RoleName),
	}
}
