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

package enrichment_rules

import (
	"context"
	"fmt"
	"log"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func DataSourceCoralogixEnrichment() *schema.Resource {
	enrichmentSchema := utils.DatasourceSchemaFromResourceSchema(EnrichmentSchema())
	enrichmentSchema["id"] = &schema.Schema{
		Type:     schema.TypeString,
		Required: true,
	}

	return &schema.Resource{
		ReadContext: dataSourceCoralogixEnrichmentRead,

		Schema:             enrichmentSchema,
		DeprecationMessage: "This data source will be phased out in 5.0.0. Please use `coralogix_data_enrichments` instead.",
		Description:        "**DEPRECATED**. Please use `coralogix_data_enrichments` instead.",
	}
}

func dataSourceCoralogixEnrichmentRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Get("id").(string)
	log.Print("[INFO] Reading enrichment")
	var enrichmentResp []*cxsdk.Enrichment
	var err error
	var enrichmentType string
	if id == "geo_ip" || id == "suspicious_ip" || id == "aws" {
		enrichmentType = id
		enrichmentResp, err = EnrichmentsByType(ctx, meta.(*clientset.ClientSet).Enrichments(), id)
	} else {
		enrichmentType = "custom"
		enrichmentResp, err = EnrichmentsByID(ctx, meta.(*clientset.ClientSet).Enrichments(), utils.StrToUint32(id))
	}
	if err != nil {
		reqStr := protojson.Format(&cxsdk.GetEnrichmentsRequest{})
		log.Printf("[ERROR] Received error: %s", err.Error())
		return diag.Errorf("%s", utils.FormatRpcErrors(err, cxsdk.GetEnrichmentsRPC, reqStr))
	}

	var enrichmentStr string
	for _, enrichment := range enrichmentResp {
		enrichmentStr += fmt.Sprintf("%s\n", protojson.Format(enrichment))
	}
	log.Printf("[INFO] Received enrichment: %s", enrichmentStr)
	d.SetId(id)
	return setEnrichment(d, enrichmentType, enrichmentResp)
}
