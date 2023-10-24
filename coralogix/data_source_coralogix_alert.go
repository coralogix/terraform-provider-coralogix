package coralogix

import (
	"context"
	"log"

	alerts "github.com/coralogix/coralogix-sdk-demo/alerts/v2"
	"terraform-provider-coralogix/coralogix/clientset"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func dataSourceCoralogixAlert() *schema.Resource {
	alertSchema := datasourceSchemaFromResourceSchema(AlertSchema())
	alertSchema["id"] = &schema.Schema{
		Type:     schema.TypeString,
		Required: true,
	}

	return &schema.Resource{
		ReadContext: dataSourceCoralogixAlertRead,

		Schema: alertSchema,
	}
}

func dataSourceCoralogixAlertRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := wrapperspb.String(d.Get("id").(string))
	getAlertRequest := &alerts.GetAlertByUniqueIdRequest{
		Id: id,
	}

	log.Printf("[INFO] Reading alert %s", id)
	alertResp, err := meta.(*clientset.ClientSet).Alerts().GetAlert(ctx, getAlertRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "alert", id.GetValue())
	}
	alert := alertResp.GetAlert()
	log.Printf("[INFO] Received alert: %#v", alert)

	d.SetId(alert.GetId().GetValue())

	return setAlert(d, alert)
}
