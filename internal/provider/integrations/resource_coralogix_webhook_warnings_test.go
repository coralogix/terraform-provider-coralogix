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
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func headerMap(t *testing.T, values map[string]string) types.Map {
	t.Helper()
	m, diags := types.MapValueFrom(context.Background(), types.StringType, values)
	if diags.HasError() {
		t.Fatalf("build header map: %v", diags)
	}
	return m
}

func TestPlainWebhookCredentialHeaderNames(t *testing.T) {
	for name, tc := range map[string]struct {
		headers *map[string]string
		want    []string
	}{
		"a well-known auth header is reported": {
			headers: &map[string]string{"Authorization": "Bearer x"},
			want:    []string{"Authorization"},
		},
		"casing does not matter": {
			headers: &map[string]string{"x-api-key": "x"},
			want:    []string{"x-api-key"},
		},
		// Warning on every header would fire on Content-Type and be ignored.
		"an ordinary header is not reported": {
			headers: &map[string]string{"Content-Type": "application/json"},
		},
		"only the credential header of several": {
			headers: &map[string]string{"Content-Type": "application/json", "X-Auth-Token": "x"},
			want:    []string{"X-Auth-Token"},
		},
		"no headers at all": {},
	} {
		t.Run(name, func(t *testing.T) {
			config := &WebhookResourceModel{}
			if tc.headers != nil {
				config.CustomWebhook = &CustomWebhookModel{Headers: headerMap(t, *tc.headers)}
			}
			got := plainWebhookCredentialHeaderNames(config)
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// Terraform's console renderer shows one warning per summary and suppresses the
// rest, so two credentials sharing a summary would hide the second warning.
func TestWebhookCredentialWarningSummariesAreDistinct(t *testing.T) {
	summaries := map[string]int{}
	for _, warning := range webhookCredentialWarnings(&WebhookResourceModel{
		Jira:          &JiraModel{ApiKey: types.StringValue("token")},
		PagerDuty:     &PagerDutyModel{ServiceKey: types.StringValue("key")},
		CustomWebhook: &CustomWebhookModel{Headers: headerMap(t, map[string]string{"Authorization": "x"})},
	}) {
		summaries[warning.Summary()]++
	}
	if len(summaries) != 3 {
		t.Errorf("expected 3 distinct summaries, got %d: %v", len(summaries), summaries)
	}
	for summary, count := range summaries {
		if count > 1 {
			t.Errorf("summary %q used %d times; only the first would be shown", summary, count)
		}
	}
}

func TestWebhookCredentialWarningsStayQuietForWriteOnly(t *testing.T) {
	for name, config := range map[string]*WebhookResourceModel{
		"jira supplied write-only": {
			Jira: &JiraModel{ApiTokenWO: types.StringValue("secret"), ApiTokenWOVersion: types.Int64Value(1)},
		},
		"pager duty supplied write-only": {
			PagerDuty: &PagerDutyModel{ServiceKeyWO: types.StringValue("secret"), ServiceKeyWOVersion: types.Int64Value(1)},
		},
		"only ordinary headers": {
			CustomWebhook: &CustomWebhookModel{Headers: headerMap(t, map[string]string{"Content-Type": "application/json"})},
		},
	} {
		t.Run(name, func(t *testing.T) {
			if got := webhookCredentialWarnings(config); len(got) > 0 {
				t.Errorf("expected no warnings, got %v", got)
			}
		})
	}
}
