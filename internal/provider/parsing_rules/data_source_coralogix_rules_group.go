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

package parsing_rules

import (
	"context"
	"log"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/utils"

	cxsdk "github.com/coralogix/coralogix-management-sdk/go"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func DataSourceCoralogixRulesGroup() *schema.Resource {
	rulesGroupSchema := utils.DatasourceSchemaFromResourceSchema(RulesGroupSchema())
	rulesGroupSchema["id"] = &schema.Schema{
		Type:     schema.TypeString,
		Required: true,
	}

	return &schema.Resource{
		ReadContext: dataSourceCoralogixRulesGroupRead,

		Schema: rulesGroupSchema,
	}
}

func dataSourceCoralogixRulesGroupRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	id := d.Get("id").(string)
	getRuleGroupRequest := &cxsdk.GetRuleGroupRequest{
		GroupId: id,
	}

	log.Printf("[INFO] Reading rule-group %s", id)
	ruleGroupResp, err := meta.(*clientset.ClientSet).RuleGroups().Get(ctx, getRuleGroupRequest)
	if err != nil {
		reqStr := protojson.Format(getRuleGroupRequest)
		log.Printf("[ERROR] Received error: %s", err.Error())
		return diag.Errorf("%s", utils.FormatRpcErrors(err, cxsdk.RuleGroupsGetRuleGroupRPC, reqStr))
	}
	ruleGroup := ruleGroupResp.GetRuleGroup()
	log.Printf("[INFO] Received rule-group: %s", protojson.Format(ruleGroup))

	d.SetId(ruleGroup.GetId().GetValue())

	return setRuleGroup(d, ruleGroup)
}
