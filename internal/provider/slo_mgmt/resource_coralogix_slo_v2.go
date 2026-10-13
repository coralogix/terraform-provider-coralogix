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
)

func NewSLOV2Resource() resource.Resource {
	return slov2.NewResource(slov2.Hooks{BeforeFlatten: normalizeLegacyFilters})
}

// normalizeLegacyFilters reads an APM filter written before values existed, which has only the
// deprecated singular value, as a filter with that one value. The next replace then keeps the
// meaning of the filter.
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
