// Copyright 2026 Coralogix Ltd.
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

package fleet

import (
	"context"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/configurationgroup"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// TypeName is the released name of the configuration group resource and data
// source. The generated resource is named after the API component instead.
const TypeName = "fleet_configuration_group"

var (
	_ resource.ResourceWithConfigure        = &FleetConfigurationGroupResource{}
	_ resource.ResourceWithImportState      = &FleetConfigurationGroupResource{}
	_ resource.ResourceWithConfigValidators = &FleetConfigurationGroupResource{}
	_ resource.ResourceWithModifyPlan       = &FleetConfigurationGroupResource{}
)

func NewFleetConfigurationGroupResource() resource.Resource {
	return &FleetConfigurationGroupResource{Resource: &configurationgroup.Resource{}}
}

// FleetConfigurationGroupResource is the generated resource under its released name.
type FleetConfigurationGroupResource struct {
	*configurationgroup.Resource
}

func (r *FleetConfigurationGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_" + TypeName
}
