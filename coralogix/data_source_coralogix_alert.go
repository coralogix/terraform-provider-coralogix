package coralogix

import (
	"context"
	"log"

	"google.golang.org/protobuf/encoding/protojson"

	"terraform-provider-coralogix/coralogix/clientset"
	alertsv1 "terraform-provider-coralogix/coralogix/clientset/grpc/alerts/v2"

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
	getAlertRequest := &alertsv1.GetAlertByUniqueIdRequest{
		Id: id,
	}

	log.Printf("[INFO] Reading alert %s", id)
	alertResp, err := meta.(*clientset.ClientSet).Alerts().GetAlert(ctx, getAlertRequest)
	if err != nil {
		reqStr := protojson.Format(getAlertRequest)
		log.Printf("[ERROR] Received error: %s", err.Error())
		return diag.Errorf(formatRpcErrors(err, getAlertURL, reqStr))
	}
	alert := alertResp.GetAlert()
	log.Printf("[INFO] Received alert: %s", protojson.Format(alert))

	d.SetId(alert.GetId().GetValue())

	return setAlert(d, alert)
}
