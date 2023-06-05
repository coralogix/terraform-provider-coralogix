package coralogix

//func dataSourceCoralogixEvents2Metric() datasource.DataSource {
//	events2MetricSchema := datasourceSchemaFromResourceSchema(Events2MetricSchema())
//	events2MetricSchema["id"] = &schema.Schema{
//		Type:     schema.TypeString,
//		Required: true,
//	}
//
//	return datasource.DataSource{
//		ReadContext: dataSourceCoralogixEvents2MetricRead,
//
//		Schema: events2MetricSchema,
//	}
//}
//
//func dataSourceCoralogixEvents2MetricRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
//	id := d.Get("id").(string)
//	getE2MRequest := &e2m.GetE2MRequest{
//		Id: wrapperspb.String(id),
//	}
//
//	log.Printf("[INFO] Reading events2Metric %s", id)
//	getE2MResp, err := meta.(*clientset.ClientSet).Events2Metrics().GetEvents2Metric(ctx, getE2MRequest)
//	if err != nil {
//		log.Printf("[ERROR] Received error: %#v", err)
//		return handleRpcErrorWithID(err, "events2Metric", id)
//	}
//
//	e2m := getE2MResp.GetE2M()
//	log.Printf("[INFO] Received events2Metric: %#v", e2m)
//
//	d.SetId(e2m.GetId().GetValue())
//
//	return setEvents2Metric(d, e2m)
//}
