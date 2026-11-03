package provider

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"testing"
)

// Keep the released recordings intact. Each migration recording may differ only
// at the paths listed here. The test fails when a later edit widens that set.
func TestWebhookMigrationDifferences(t *testing.T) {
	allowed := map[string][]string{
		"edge_create-read-error.json": {"/0/state"},
		"edge_delete-missing.json":    {"/1/applyDiagnostics", "/1/state"},
		"edge_empty-collections.json": {
			"/1/inconsistentResult", "/1/replanDiff", "/1/replanEmpty", "/1/requests/0/body/data/slack/attachments",
			"/1/state/slack/attachments", "/1/state/slack/notify_on", "/2/replanDiff", "/2/replanEmpty",
			"/2/state/slack/attachments", "/2/state/slack/notify_on",
		},
		"edge_empty-emails.json": {
			"/0/inconsistentResult", "/0/replanDiff", "/0/replanEmpty", "/0/state/email_group/emails",
			"/1/replanDiff", "/1/replanEmpty", "/1/state/email_group/emails", "/3/inconsistentResult",
			"/3/replanDiff", "/3/replanEmpty", "/3/state/email_group/emails", "/4/replanDiff", "/4/replanEmpty",
			"/4/state/email_group/emails",
		},
		"edge_import-legacy-teams.json": {
			"/2/applyDiagnostics/0", "/2/requests/0/body/data", "/3/applyDiagnostics/0", "/3/requests/0/body/data",
		},
		"edge_missing-name-create.json": {"/0/requests/0/body/data/name", "/2/requests/0/body/data/name"},
		"edge_update-missing.json":      {"/1/applyDiagnostics", "/1/state"},
		"optional_jira-email.json":      {"/0/requests/0/body/data/jira/email", "/2/requests/0/body/data/jira/email"},
		"optional_jira-project.json":    {"/0/requests/0/body/data/jira/projectKey", "/2/requests/0/body/data/jira/projectKey"},
		"optional_jira-token.json":      {"/0/requests/0/body/data/jira/apiToken", "/2/requests/0/body/data/jira/apiToken"},
		"optional_pager-duty-key.json":  {"/0/requests/0/body/data/pagerDuty/serviceKey", "/2/requests/0/body/data/pagerDuty/serviceKey"},
	}
	entries, err := os.ReadDir(webhookMigrationDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(allowed) {
		t.Fatalf("migration recordings: got %d, want %d", len(entries), len(allowed))
	}
	for _, entry := range entries {
		name := entry.Name()
		want, ok := allowed[name]
		if !ok {
			t.Errorf("unapproved migration recording %s", name)
			continue
		}
		old := readWebhookRecording(t, filepath.Join(webhookGoldenDir, name))
		new := readWebhookRecording(t, filepath.Join(webhookMigrationDir, name))
		var got []string
		webhookDifferencePaths(old, new, "", &got)
		sort.Strings(got)
		sort.Strings(want)
		if !slices.Equal(got, want) {
			t.Errorf("%s: changed paths = %v, want %v", name, got, want)
		}
	}
}

func readWebhookRecording(t *testing.T, name string) any {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func webhookDifferencePaths(old, new any, path string, out *[]string) {
	if reflect.DeepEqual(old, new) {
		return
	}
	if a, ok := old.(map[string]any); ok {
		if b, ok := new.(map[string]any); ok {
			keys := make(map[string]bool)
			for key := range a {
				keys[key] = true
			}
			for key := range b {
				keys[key] = true
			}
			for key := range keys {
				left, leftOK := a[key]
				right, rightOK := b[key]
				if leftOK != rightOK {
					*out = append(*out, path+"/"+key)
				} else {
					webhookDifferencePaths(left, right, path+"/"+key, out)
				}
			}
			return
		}
	}
	if a, ok := old.([]any); ok {
		if b, ok := new.([]any); ok && len(a) == len(b) {
			for i := range a {
				webhookDifferencePaths(a[i], b[i], path+"/"+strconv.Itoa(i), out)
			}
			return
		}
	}
	*out = append(*out, path)
}
