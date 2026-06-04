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

package integrations

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestExpandSendLogOmitsURL(t *testing.T) {
	result := expandSendLog(&SendLogModel{
		UUID:    types.StringValue("webhook-id"),
		URL:     types.StringValue("https://example.com/legacy-sendlog-url"),
		Payload: types.StringValue(`{"custom":"payload"}`),
	})

	if result.Url != nil {
		t.Fatalf("expected SendLog URL to be omitted, got %q", *result.Url)
	}
}
