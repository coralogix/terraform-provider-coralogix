package coralogix

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"terraform-provider-coralogix/coralogix/clientset"
	actionsv2 "terraform-provider-coralogix/coralogix/clientset/grpc/actions/v2"
)

func dataSourceCoralogixAction() *schema.Resource {
	actionSchema := datasourceSchemaFromResourceSchema(ActionSchema())
	actionSchema["id"] = &schema.Schema{
		Type:     schema.TypeString,
		Required: true,
	}

	return &schema.Resource{
		ReadContext: dataSourceCoralogixActionRead,

		Schema: actionSchema,
	}
}

func dataSourceCoralogixActionRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := wrapperspb.String(d.Get("id").(string))
	getActionRequest := &actionsv2.GetActionRequest{
		Id: id,
	}

	log.Printf("[INFO] Reading action %s", id)
	actionResp, err := meta.(*clientset.ClientSet).Actions().GetAction(ctx, getActionRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "action", id.GetValue())
	}

	action := actionResp.GetAction()
	log.Printf("[INFO] Received action: %#v", action)

	d.SetId(action.GetId().GetValue())

	return setAction(d, action)
}
