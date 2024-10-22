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

	"terraform-provider-coralogix/coralogix/clientset"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func dataSourceCoralogixDataSet() *schema.Resource {
	dataSetSchema := datasourceSchemaFromResourceSchema(DataSetSchema())
	dataSetSchema["id"] = &schema.Schema{
		Type:     schema.TypeString,
		Required: true,
	}

	return &schema.Resource{
		ReadContext: dataSourceCoralogixDataSetRead,

		Schema: dataSetSchema,
	}
}

func dataSourceCoralogixDataSetRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Get("id").(string)
	req := &cxsdk.GetDataSetRequest{Id: wrapperspb.UInt32(strToUint32(id))}
	log.Printf("[INFO] Reading custom-enrichment-data %s", id)
	enrichmentResp, err := meta.(*clientset.ClientSet).DataSet().Get(ctx, req)
	if err != nil {
		log.Printf("[ERROR] Received error: %s", err.Error())
		reqStr := protojson.Format(req)
		return diag.Errorf(formatRpcErrors(err, cxsdk.GetDataSetRPC, reqStr))
	}
	log.Printf("[INFO] Received custom-enrichment-data: %s", protojson.Format(enrichmentResp))

	d.SetId(uint32ToStr(enrichmentResp.GetCustomEnrichment().GetId()))

	return setDataSet(d, enrichmentResp.GetCustomEnrichment())
}
