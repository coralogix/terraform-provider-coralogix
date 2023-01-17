package coralogix

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-coralogix/coralogix/clientset"
	actionsv2 "terraform-provider-coralogix/coralogix/clientset/grpc/actions/v2"
)

var (
	actionSchemaSourceTypeToProtoSourceType = map[string]string{
		"Log":     "SOURCE_TYPE_LOG",
		"DataMap": "SOURCE_TYPE_DATA_MAP",
	}
	actionProtoSourceTypeToSchemaSourceType = reverseMapStrings(actionSchemaSourceTypeToProtoSourceType)
	actionValidSourceTypes                  = getKeysStrings(actionSchemaSourceTypeToProtoSourceType)
)

func resourceCoralogixAction() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCoralogixActionCreate,
		ReadContext:   resourceCoralogixActionRead,
		UpdateContext: resourceCoralogixActionUpdate,
		DeleteContext: resourceCoralogixActionDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(60 * time.Second),
			Read:   schema.DefaultTimeout(30 * time.Second),
			Update: schema.DefaultTimeout(60 * time.Second),
			Delete: schema.DefaultTimeout(30 * time.Second),
		},

		Schema: ActionSchema(),

		Description: "Coralogix action. Api-key is required for this resource. For more info please review - https://coralogix.com/docs/coralogix-action-extension/.",
	}
}

func resourceCoralogixActionCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	createActionRequest := extractCreateAction(d)

	log.Printf("[INFO] Creating new action: %#v", createActionRequest)
	resp, err := meta.(*clientset.ClientSet).Actions().CreateAction(ctx, createActionRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return diag.FromErr(err)
	}

	action := resp.GetAction()
	log.Printf("[INFO] Submitted new action: %#v", action)
	d.SetId(action.GetId().GetValue())

	return resourceCoralogixActionRead(ctx, d, meta)
}

func resourceCoralogixActionRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := wrapperspb.String(d.Id())
	getActionRequest := &actionsv2.GetActionRequest{
		Id: wrapperspb.String(d.Id()),
	}

	log.Printf("[INFO] Reading action %s", id)
	resp, err := meta.(*clientset.ClientSet).Actions().GetAction(ctx, getActionRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "action", id.GetValue())
	}

	action := resp.GetAction()
	log.Printf("[INFO] Received action: %#v", action)

	return setAction(d, action)
}

func resourceCoralogixActionUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	updateActionRequest := extractUpdateAction(d)

	log.Printf("[INFO] Updating action: %#v", updateActionRequest)
	resp, err := meta.(*clientset.ClientSet).Actions().UpdateAction(ctx, updateActionRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return diag.FromErr(err)
	}

	action := resp.GetAction()
	log.Printf("[INFO] Submitted new action: %#v", action)
	d.SetId(action.GetId().GetValue())

	return resourceCoralogixActionRead(ctx, d, meta)
}

func resourceCoralogixActionDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := wrapperspb.String(d.Id())
	deleteActionRequest := &actionsv2.DeleteActionRequest{
		Id: id,
	}

	log.Printf("[INFO] Deleting action %s", id)
	_, err := meta.(*clientset.ClientSet).Actions().DeleteAction(ctx, deleteActionRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "action", id.GetValue())
	}
	log.Printf("[INFO] action %s deleted", id)

	d.SetId("")
	return nil
}

func ActionSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"name": {
			Type:         schema.TypeString,
			Required:     true,
			ValidateFunc: validation.StringIsNotEmpty,
			Description:  "Action name.",
		},
		"url": {
			Type:             schema.TypeString,
			Required:         true,
			ValidateDiagFunc: urlValidationFunc(),
			Description:      "URL for the external tool.",
		},
		"is_private": {
			Type:        schema.TypeBool,
			Optional:    true,
			Default:     true,
			Description: "Determines weather the action will be shared with the entire team. Can be set to false only by admin.",
		},
		"is_hidden": {
			Type:        schema.TypeBool,
			Optional:    true,
			Default:     false,
			Description: "Determines weather the action will be shown at the action menu.",
		},
		"source_type": {
			Type:         schema.TypeString,
			Required:     true,
			ValidateFunc: validation.StringInSlice(actionValidSourceTypes, false),
			Description:  fmt.Sprintf("By selecting the data type, you can make sure that the action will be displayed only in the relevant context. Can be one of %q", actionValidSourceTypes),
		},
		"applications": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
			Set:         schema.HashString,
			Description: "Applies the action for specific applications.",
		},
		"subsystems": {
			Type:     schema.TypeSet,
			Optional: true,
			Elem: &schema.Schema{
				Type: schema.TypeString,
			},
			Set:         schema.HashString,
			Description: "Applies the action for specific subsystems.",
		},
		"created_by": {
			Type:     schema.TypeString,
			Computed: true,
		},
	}
}

func extractCreateAction(d *schema.ResourceData) *actionsv2.CreateActionRequest {
	name := wrapperspb.String(d.Get("name").(string))
	url := wrapperspb.String(d.Get("url").(string))
	isPrivate := wrapperspb.Bool(d.Get("is_private").(bool))
	sourceType := expandActionSourceType(d.Get("source_type").(string))
	applicationNames := interfaceSliceToWrappedStringSlice(d.Get("applications").(*schema.Set).List())
	subsystemNames := interfaceSliceToWrappedStringSlice(d.Get("subsystems").(*schema.Set).List())

	return &actionsv2.CreateActionRequest{
		Name:             name,
		Url:              url,
		IsPrivate:        isPrivate,
		SourceType:       sourceType,
		ApplicationNames: applicationNames,
		SubsystemNames:   subsystemNames,
	}
}

func extractUpdateAction(d *schema.ResourceData) *actionsv2.ReplaceActionRequest {
	id := wrapperspb.String(d.Id())
	name := wrapperspb.String(d.Get("name").(string))
	url := wrapperspb.String(d.Get("url").(string))
	isPrivate := wrapperspb.Bool(d.Get("is_private").(bool))
	isHidden := wrapperspb.Bool(d.Get("is_hidden").(bool))
	sourceType := expandActionSourceType(d.Get("source_type").(string))
	applicationNames := interfaceSliceToWrappedStringSlice(d.Get("applications").(*schema.Set).List())
	subsystemNames := interfaceSliceToWrappedStringSlice(d.Get("subsystems").(*schema.Set).List())

	return &actionsv2.ReplaceActionRequest{
		Action: &actionsv2.Action{
			Id:               id,
			Name:             name,
			Url:              url,
			IsPrivate:        isPrivate,
			IsHidden:         isHidden,
			SourceType:       sourceType,
			ApplicationNames: applicationNames,
			SubsystemNames:   subsystemNames,
		},
	}
}

func expandActionSourceType(s string) actionsv2.SourceType {
	sourceTypeStr := actionSchemaSourceTypeToProtoSourceType[s]
	sourceTypeValue := actionsv2.SourceType_value[sourceTypeStr]
	return actionsv2.SourceType(sourceTypeValue)
}

func setAction(d *schema.ResourceData, action *actionsv2.Action) diag.Diagnostics {
	if err := d.Set("name", action.GetName().GetValue()); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("url", action.GetUrl().GetValue()); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("is_private", action.GetIsPrivate().GetValue()); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("source_type", flattenActionSourceType(action.GetSourceType().String())); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("applications", wrappedStringSliceToStringSlice(action.GetApplicationNames())); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("subsystems", wrappedStringSliceToStringSlice(action.GetSubsystemNames())); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("is_hidden", action.GetIsHidden().GetValue()); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("created_by", action.GetCreatedBy().GetValue()); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func flattenActionSourceType(s string) string {
	return actionProtoSourceTypeToSchemaSourceType[s]
}
