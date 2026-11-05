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

// Package acctest holds the handwritten helpers that the generated acceptance tests call, such as
// the leftovers functions that an acceptance.yaml names.
package acctest

import (
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/internal/acctest/leftovers"
)

// RemoveLeftoverGlobalRouters deletes every global router that holds exactly the routing labels of a
// coralogix_global_router in configs. See leftovers.GlobalRouters.
func RemoveLeftoverGlobalRouters(t *testing.T, configs ...string) {
	t.Helper()
	clients, err := newClientSet()
	if err != nil {
		t.Fatal(err)
	}
	leftovers.GlobalRouters(t, clients, configs...)
}
