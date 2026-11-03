// Copyright 2025 Coralogix Ltd.
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

package acctest

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"

	"github.com/coralogix/terraform-provider-coralogix/internal/clientset"
	"github.com/coralogix/terraform-provider-coralogix/internal/provider"
)

// newClientSet configures the provider from the environment, as the test configs do, and returns its
// clients.
func newClientSet() (*clientset.ClientSet, error) {
	p := provider.OldProvider()
	if diags := p.Configure(context.Background(), &terraform.ResourceConfig{}); diags.HasError() {
		return nil, fmt.Errorf("configure the acceptance client: %s", diags[0].Summary)
	}
	clients, ok := p.Meta().(*clientset.ClientSet)
	if !ok {
		return nil, fmt.Errorf("configure the acceptance client: provider metadata has type %T", p.Meta())
	}
	return clients, nil
}
