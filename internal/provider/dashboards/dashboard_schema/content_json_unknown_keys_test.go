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

package dashboard_schema

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/coralogix/coralogix-management-sdk/go/openapi/dashboardjson"
	dashboardservice "github.com/coralogix/coralogix-management-sdk/go/openapi/gen/dashboard_service"
)

func TestUnknownContentJSONKeys(t *testing.T) {
	for name, tc := range map[string]struct {
		content string
		want    []string
	}{
		"clean dashboard": {
			content: `{"name":"x","description":"d","layout":{"sections":[]}}`,
		},
		"misspelled top-level field": {
			content: `{"name":"x","descriptoin":"typo"}`,
			want:    []string{"descriptoin"},
		},
		"unknown field nested in a list element": {
			content: `{"layout":{"sections":[{"id":{"value":"a"},"rowz":[]}]}}`,
			want:    []string{"layout.sections[0].rowz"},
		},
		"several are all reported, sorted": {
			content: `{"zzz":1,"aaa":2}`,
			want:    []string{"aaa", "zzz"},
		},
		// The decoder accepts both spellings, so neither may be reported.
		"protobuf snake_case spelling is accepted": {
			content: `{"folder_id":{"value":"f"}}`,
		},
		"camelCase spelling is accepted": {
			content: `{"folderId":{"value":"f"}}`,
		},
		// A free-form map's keys are user data, not schema fields.
		"keys inside an untyped map are not reported": {
			content: `{"layout":{"sections":[{"options":{"internal":{"anythingAtAll":true}}}]}}`,
		},
		"malformed json is left to the error path": {
			content: `{"name":`,
		},
	} {
		t.Run(name, func(t *testing.T) {
			got := unknownContentJSONKeys(tc.content)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// Whatever this reports must actually be discarded by the decoder, otherwise the
// warning fires on a configuration that works.
func TestUnknownContentJSONKeysAgreeWithTheDecoder(t *testing.T) {
	for _, content := range []string{
		`{"name":"x","layout":{"sections":[]},"folder_id":{"value":"f"}}`,
		`{"name":"x","layout":{"sections":[]},"folderId":{"value":"f"}}`,
		`{"name":"x","layout":{"sections":[{"rows":[]}]}}`,
		`{"name":"x","layout":{"sections":[]},"absoluteTimeFrame":{"from":"2026-01-01T00:00:00Z","to":"2026-01-02T00:00:00Z"}}`,
	} {
		if got := unknownContentJSONKeys(content); len(got) > 0 {
			t.Errorf("reported %v for content the decoder accepts: %s", got, content)
		}
		if err := dashboardjson.Unmarshal([]byte(content), &dashboardservice.Dashboard{}); err != nil {
			t.Errorf("decoder rejected %s: %v", content, err)
		}
	}
}

// The validator must warn, never fail: a configuration carrying an unrecognised
// key still applies, and only the key is dropped.
func TestContentJsonValidatorWarnsOnUnknownKeys(t *testing.T) {
	for name, tc := range map[string]struct {
		content      string
		wantWarnings int
		wantErrors   int
	}{
		"unknown key warns": {
			content:      `{"name":"x","layout":{"sections":[]},"descriptoin":"typo"}`,
			wantWarnings: 1,
		},
		"clean content is silent": {
			content: `{"name":"x","layout":{"sections":[]},"description":"fine"}`,
		},
		"malformed json still errors": {
			content:    `{"name":`,
			wantErrors: 1,
		},
	} {
		t.Run(name, func(t *testing.T) {
			resp := &validator.StringResponse{}
			ContentJsonValidator{}.ValidateString(context.Background(), validator.StringRequest{
				Path:        path.Root("content_json"),
				ConfigValue: types.StringValue(tc.content),
			}, resp)

			if got := resp.Diagnostics.WarningsCount(); got != tc.wantWarnings {
				t.Errorf("warnings: got %d want %d (%v)", got, tc.wantWarnings, resp.Diagnostics.Warnings())
			}
			if got := resp.Diagnostics.ErrorsCount(); got != tc.wantErrors {
				t.Errorf("errors: got %d want %d (%v)", got, tc.wantErrors, resp.Diagnostics.Errors())
			}
			if tc.wantWarnings > 0 && !strings.Contains(resp.Diagnostics.Warnings()[0].Detail(), "descriptoin") {
				t.Errorf("warning does not name the offending key: %s", resp.Diagnostics.Warnings()[0].Detail())
			}
		})
	}
}
