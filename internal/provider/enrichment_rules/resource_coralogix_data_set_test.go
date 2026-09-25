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

package enrichment_rules

import (
	"context"
	"testing"
)

// TestExpandFileContentReadError verifies that a file read error surfaces as a
// returned error rather than crashing the provider process via log.Fatal.
// Opening a directory succeeds, but reading from it fails, which drives the
// read loop's default case.
func TestExpandFileContentReadError(t *testing.T) {
	dir := t.TempDir()

	r := ResourceCoralogixDataSet()
	d := r.Data(nil)
	if err := d.Set("uploaded_file", []interface{}{
		map[string]interface{}{
			"path": dir,
		},
	}); err != nil {
		t.Fatalf("failed to set uploaded_file: %s", err)
	}

	_, _, err := expandFileContent(d)
	if err == nil {
		t.Fatalf("expected an error reading a directory, got nil")
	}
}

// TestImportDataSet verifies that the import state function accepts numeric IDs
// and rejects non-numeric ones instead of silently importing data set id 0.
func TestImportDataSet(t *testing.T) {
	t.Run("valid numeric id", func(t *testing.T) {
		r := ResourceCoralogixDataSet()
		d := r.Data(nil)
		d.SetId("12345")

		got, err := importDataSet(context.Background(), d, nil)
		if err != nil {
			t.Fatalf("expected no error for numeric id, got %s", err)
		}
		if len(got) != 1 || got[0].Id() != "12345" {
			t.Fatalf("expected the resource data to pass through unchanged, got %#v", got)
		}
	})

	t.Run("non-numeric id is rejected", func(t *testing.T) {
		r := ResourceCoralogixDataSet()
		d := r.Data(nil)
		d.SetId("not-a-number")

		got, err := importDataSet(context.Background(), d, nil)
		if err == nil {
			t.Fatalf("expected an error for non-numeric id, got nil")
		}
		if got != nil {
			t.Fatalf("expected nil resource data on error, got %#v", got)
		}
	})
}
