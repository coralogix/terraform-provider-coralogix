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
