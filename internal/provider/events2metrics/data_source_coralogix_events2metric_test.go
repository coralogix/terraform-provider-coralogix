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

package events2metrics

import (
	"context"
	"testing"

	e2ms "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/events2metrics_service"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The data source shows the description as the API stores it, as before the resource was
// generated: a stored "" is "", not the null that the resource reads for it.
func TestDataSourceShowsTheStoredDescription(t *testing.T) {
	for name, test := range map[string]struct {
		stored *string
		want   types.String
	}{
		"empty":  {stored: new(string), want: types.StringValue("")},
		"absent": {stored: nil, want: types.StringNull()},
		"set":    {stored: func() *string { s := "d"; return &s }(), want: types.StringValue("d")},
	} {
		t.Run(name, func(t *testing.T) {
			data, diags := flattenDataSource(context.Background(), &e2ms.E2M{Id: "id", Name: "n", Description: test.stored})
			if diags.HasError() {
				t.Fatal(diags)
			}
			if !data.Description.Equal(test.want) {
				t.Fatalf("description = %v, want %v", data.Description, test.want)
			}
		})
	}
}
