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

package slo_mgmt

import (
	"context"

	slos "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/slos_service"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/slov2"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

func NewSLOV2Resource() resource.Resource {
	return slov2.NewResource(slov2.Hooks{BeforeWrite: clearOwnershipOnReplace, BeforeFlatten: normalizeLegacySLO})
}

// clearOwnershipOnReplace sends an empty ownershipTags with a request that has none. A metric SLO
// drops its ownership tags when a replace omits them, but an APM SLO keeps them, so removing
// `ownership_tags` from the configuration of an APM SLO ended in an inconsistent result. An empty
// object clears the tags of both, and on a create it stores none (checked against the API).
func clearOwnershipOnReplace(_ context.Context, _ tfsdk.Config, body any) diag.Diagnostics {
	if slo, ok := body.(*slos.Slo1); ok && slo.OwnershipTags == nil {
		slo.OwnershipTags = &slos.SloOwnershipTags{}
	}
	return nil
}

// normalizeLegacySLO reads the values that an older record does not have as the values that the
// API stores for them today: an omitted productType is UNSPECIFIED, and an omitted
// missingDataStrategy is UNCOUNTED. It also reads an APM filter written before values existed,
// which has only the deprecated singular value, as a filter with that one value, so the next
// replace keeps the meaning of the filter.
func normalizeLegacySLO(ctx context.Context, slo *slos.Slo) diag.Diagnostics {
	if slo.ProductType == nil {
		slo.ProductType = slos.SLOPRODUCTTYPE_SLO_PRODUCT_TYPE_UNSPECIFIED.Ptr()
	}
	if window := slo.WindowBasedMetricSli; window != nil && window.MissingDataStrategy == nil {
		window.MissingDataStrategy = slos.MISSINGDATASTRATEGY_MISSING_DATA_STRATEGY_UNCOUNTED.Ptr()
	}
	return normalizeLegacyFilters(ctx, slo)
}

// normalizeLegacyFilters reads an APM filter written before values existed, which has only the
// deprecated singular value, as a filter with that one value.
func normalizeLegacyFilters(_ context.Context, slo *slos.Slo) diag.Diagnostics {
	for _, sli := range []*slos.ApmSli{slo.ApmSli, slo.ApmSliMetadata} {
		if sli == nil {
			continue
		}
		for i := range sli.Filters {
			filter := &sli.Filters[i]
			if len(filter.Values) == 0 && filter.GetValue() != "" {
				filter.Values = []string{filter.GetValue()}
			}
		}
	}
	return nil
}
