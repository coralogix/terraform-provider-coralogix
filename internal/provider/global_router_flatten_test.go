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

package provider

import (
	"context"
	"testing"

	global_routers_service "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/global_routers_service"

	"github.com/coralogix/terraform-provider-coralogix/internal/provider/generated/globalrouter"
)

// The pinned contract cannot say that the router id is always in a response, because the id is a
// proto3 optional field. A response without an id must be an error, not a null id in the state:
// Read, Update, and Delete take the id from the state.
func TestGlobalRouterFlattenNeedsAnID(t *testing.T) {
	name := "router"
	_, diags := globalrouter.Flatten(context.Background(), &global_routers_service.GlobalRouter{Name: &name})
	if !diags.HasError() {
		t.Fatal("a response without an id was accepted")
	}

	id := "router-1"
	model, diags := globalrouter.Flatten(context.Background(), &global_routers_service.GlobalRouter{Id: &id, Name: &name})
	if diags.HasError() {
		t.Fatalf("a response with an id was rejected: %v", diags)
	}
	if model.Id.ValueString() != id {
		t.Fatalf("id = %q, want %q", model.Id.ValueString(), id)
	}
}
