package coralogix

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"terraform-provider-coralogix-v2/coralogix/clientset"
	logs2metric "terraform-provider-coralogix-v2/coralogix/clientset/grpc/com/coralogix/logs2metrics/v2"
)

func dataSourceCoralogixLogs2Metric() *schema.Resource {
	logs2MetricSchema := datasourceSchemaFromResourceSchema(Logs2MetricSchema())
	logs2MetricSchema["id"] = &schema.Schema{
		Type:     schema.TypeString,
		Required: true,
	}

	return &schema.Resource{
		ReadContext: dataSourceCoralogixLogs2MetricRead,

		Schema: logs2MetricSchema,
	}
}

func dataSourceCoralogixLogs2MetricRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Get("id").(string)
	getLogs2MetricRequest := &logs2metric.GetL2MRequest{
		Id: id,
	}

	log.Printf("[INFO] Reading logs2Metric %s", id)
	logs2MetricResp, err := meta.(*clientset.ClientSet).Logs2Metrics().GetLogs2Metric(ctx, getLogs2MetricRequest)
	if err != nil {
		log.Printf("[ERROR] Received error: %#v", err)
		return handleRpcErrorWithID(err, "logs2Metric", id)
	}

	log.Printf("[INFO] Received logs2Metric: %#v", logs2MetricResp)

	d.SetId(logs2MetricResp.GetId())

	return setLogs2Metric(d, logs2MetricResp)
}