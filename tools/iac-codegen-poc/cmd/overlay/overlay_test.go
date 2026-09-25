package main

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/sdkspec"
)

const sample = `openapi: 3.1.0
paths:
  /things/v1/{id}:
    patch:
      requestBody:
        content:
          application/json:
            schema:
              properties:
                name:
                  description: >-
                    A folded description that
                    spans two lines.
                  type: string
                size:
                  type: string
              type: object
components:
  schemas:
    v3.Thing:
      type: object
      example: {a: 1}
`

const body = "paths./things/v1/{id}.patch.requestBody.content.application/json.schema"

func mustApply(t *testing.T, src, overlay string) string {
	t.Helper()
	entries, err := parseEntries([]byte(overlay))
	if err != nil {
		t.Fatalf("parse entries: %v", err)
	}
	out, err := apply([]byte(src), entries)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	return string(out)
}

func applyErr(t *testing.T, src, overlay string) error {
	t.Helper()
	entries, err := parseEntries([]byte(overlay))
	if err != nil {
		return err
	}
	_, err = apply([]byte(src), entries)
	if err == nil {
		t.Fatal("want an error, got none")
	}
	return err
}

func TestParsePath(t *testing.T) {
	for _, tc := range []struct {
		path string
		want []string
	}{
		{"a.b.c", []string{"a", "b", "c"}},
		{"paths./ai/v3/{id}.patch.content.application/json", []string{"paths", "/ai/v3/{id}", "patch", "content", "application/json"}},
		{"components.schemas[v3.FilterOperator].type", []string{"components", "schemas", "v3.FilterOperator", "type"}},
		{"components.schemas.[v3.X][a.b]", []string{"components", "schemas", "v3.X", "a.b"}},
	} {
		got, err := parsePath(tc.path)
		if err != nil {
			t.Errorf("parsePath(%q): %v", tc.path, err)
			continue
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("parsePath(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
	for _, bad := range []string{"", "a..b", "a.", ".a", "a[b", "a[]", "a[b]c"} {
		if _, err := parsePath(bad); err == nil {
			t.Errorf("parsePath(%q): want an error", bad)
		}
	}
}

func TestFormatPath(t *testing.T) {
	keys := []string{"components", "schemas", "v3.X", "/a/{id}"}
	got := formatPath(keys)
	if got != "components.schemas[v3.X]./a/{id}" {
		t.Errorf("formatPath = %q", got)
	}
	if back, _ := parsePath(got); !slices.Equal(back, keys) {
		t.Errorf("parsePath(formatPath) = %q, want %q", back, keys)
	}
}

func TestSetAddsKeyAtEndOfMapping(t *testing.T) {
	got := mustApply(t, sample, `
- path: `+body+`.properties.size.format
  set: uint64
- path: `+body+`.required
  set: [name]
`)
	want := strings.Replace(sample, `
                  type: string
              type: object
`, `
                  type: string
                  format: uint64
              type: object
              required:
                - name
`, 1)
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestSetReplacesValue(t *testing.T) {
	got := mustApply(t, sample, `
- path: `+body+`.properties.name.description
  set: Short.
- path: components.schemas[v3.Thing].type
  set: string
`)
	want := strings.Replace(sample, `                  description: >-
                    A folded description that
                    spans two lines.
`, "                  description: Short.\n", 1)
	want = strings.Replace(want, "      type: object\n      example", "      type: string\n      example", 1)
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRemove(t *testing.T) {
	// name is in the middle of its mapping; size is the last key before a
	// key of a parent mapping.
	got := mustApply(t, sample, `
- path: `+body+`.properties.name
  remove: true
`)
	want := strings.Replace(sample, `                name:
                  description: >-
                    A folded description that
                    spans two lines.
                  type: string
`, "", 1)
	if got != want {
		t.Errorf("remove name, got:\n%s", got)
	}

	got = mustApply(t, sample, `
- path: `+body+`.properties.size
  remove: true
`)
	want = strings.Replace(sample, "                size:\n                  type: string\n", "", 1)
	if got != want {
		t.Errorf("remove size, got:\n%s", got)
	}
}

func TestEntriesApplyInOrder(t *testing.T) {
	size := body + ".properties.size"
	for _, tc := range []struct {
		name, overlay, want string
	}{
		{"set twice", `
- {path: '` + size + `.format', set: int64}
- {path: '` + size + `.format', set: uint64}
`, "                  type: string\n                  format: uint64\n              type: object"},
		{"set then remove", `
- {path: '` + size + `.format', set: int64}
- {path: '` + size + `.format', remove: true}
`, "                size:\n                  type: string\n              type: object"},
		{"remove then set moves the key to the end", `
- {path: '` + body + `.properties.name.description', remove: true}
- {path: '` + body + `.properties.name.description', set: New.}
`, "                name:\n                  type: string\n                  description: New.\n                size:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := mustApply(t, sample, tc.overlay); !strings.Contains(got, tc.want) {
				t.Errorf("got:\n%s\nwant it to contain:\n%s", got, tc.want)
			}
		})
	}
}

func TestApplyErrors(t *testing.T) {
	for _, tc := range []struct {
		name, overlay, want string
	}{
		{"missing key in path", `
- {path: '` + body + `.type', set: object}
- {path: 'paths./things/v1/{id}.get.responses', set: {}}
`, `entry 2 (line 3, path "paths./things/v1/{id}.get.responses"): key "get" not found in "paths./things/v1/{id}"`},
		{"remove a missing key", `
- {path: '` + body + `.required', remove: true}
`, `entry 1 (line 2, path "` + body + `.required"): key "required" not found`},
		{"path through a scalar", `
- {path: openapi.version, set: x}
`, `entry 1 (line 2, path "openapi.version"): "openapi" is not a mapping`},
		{"flow style parent", `
- {path: 'components.schemas[v3.Thing].example.b', set: 2}
`, "flow style"},
		{"remove the last key", `
- {path: '` + body + `.properties.size', remove: true}
- {path: '` + body + `.properties.name', remove: true}
`, "entry 2 (line 3, path"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := applyErr(t, sample, tc.overlay); !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestParseEntriesErrors(t *testing.T) {
	for _, tc := range []struct {
		name, overlay, want string
	}{
		{"not a list", "path: a\nset: 1\n", "must be a list"},
		{"entry is not a mapping", "- a.b\n", "entry 1 (line 1"},
		{"unknown field", "- {path: a, set: 1}\n- {path: a, sett: 1}\n", `entry 2 (line 2, path "a"): unknown field "sett"`},
		{"set and remove", "- {path: a, set: 1, remove: true}\n", "exactly one of set and remove"},
		{"no operation", "- {path: a}\n", "exactly one of set and remove"},
		{"remove false", "- {path: a, remove: false}\n", "remove must be true"},
		{"no path", "- {set: 1}\n", "path is missing"},
		{"path not a string", "- {path: [a], set: 1}\n", "path must be a string"},
		{"bad path", "- {path: a..b, set: 1}\n", `entry 1 (line 1, path "a..b"): empty key`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseEntries([]byte(tc.overlay))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

// TestPatchedSpecIsUpToDate fails when spec/openapi.patched.yaml is not the
// output of the current overlay on the pinned SDK spec, or when two runs give different bytes.
func TestPatchedSpecIsUpToDate(t *testing.T) {
	src, err := sdkspec.Read()
	if err != nil {
		t.Fatal(err)
	}
	overlay, err := os.ReadFile("../../spec/overlay.yaml")
	if err != nil {
		t.Fatal(err)
	}
	committed, err := os.ReadFile("../../spec/openapi.patched.yaml")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := parseEntries(overlay)
	if err != nil {
		t.Fatal(err)
	}
	first, err := apply(src, entries)
	if err != nil {
		t.Fatal(err)
	}
	second, err := apply(src, entries)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Error("two runs give different output")
	}
	if !bytes.Equal(first, committed) {
		t.Error("spec/openapi.patched.yaml is out of date; run cmd/overlay again")
	}
}
