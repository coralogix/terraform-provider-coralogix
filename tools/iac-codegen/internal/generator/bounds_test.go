package generator

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"go.yaml.in/yaml/v4"
)

// exclusiveBoundsExact keeps the values that a schema accepts and gives them an inclusive form.
func TestExclusiveBoundsExact(t *testing.T) {
	tests := []struct {
		name   string
		schema string
		want   map[string]any // the keys of the schema after the rewrite
	}{
		{
			name:   "number with a step: the least multiple above the bound",
			schema: `{type: number, multipleOf: 0.001, exclusiveMinimum: 0}`,
			want:   map[string]any{"type": "number", "multipleOf": 0.001, "minimum": 0.001},
		},
		{
			name:   "number with a step: the greatest multiple below the bound",
			schema: `{type: number, multipleOf: 0.5, exclusiveMaximum: 3}`,
			want:   map[string]any{"type": "number", "multipleOf": 0.5, "maximum": 2.5},
		},
		{
			name:   "bound that is not a multiple of the step",
			schema: `{type: number, multipleOf: 0.5, exclusiveMinimum: 0.7, exclusiveMaximum: 2.2}`,
			want:   map[string]any{"type": "number", "multipleOf": 0.5, "minimum": 1, "maximum": 2},
		},
		{
			name:   "integer",
			schema: `{type: integer, format: int32, exclusiveMinimum: 0, exclusiveMaximum: 11}`,
			want:   map[string]any{"type": "integer", "format": "int32", "minimum": 1, "maximum": 10},
		},
		{
			name:   "integer with a bound that is not whole",
			schema: `{type: integer, exclusiveMinimum: 0.5, exclusiveMaximum: 9.5}`,
			want:   map[string]any{"type": "integer", "minimum": 1, "maximum": 9},
		},
		{
			name:   "integer with a fractional step accepts every integer",
			schema: `{type: integer, multipleOf: 0.5, exclusiveMinimum: 0}`,
			want:   map[string]any{"type": "integer", "multipleOf": 0.5, "minimum": 1},
		},
		{
			name:   "integer with a fractional step moves by the integers that are multiples of it",
			schema: `{type: integer, multipleOf: 1.5, exclusiveMinimum: 0}`,
			want:   map[string]any{"type": "integer", "multipleOf": 1.5, "minimum": 3},
		},
		{
			name:   "integer with a whole step",
			schema: `{type: integer, multipleOf: 2, exclusiveMaximum: 10}`,
			want:   map[string]any{"type": "integer", "multipleOf": 2, "maximum": 8},
		},
		{
			name:   "a step finer than float64 at the bound has no safe form",
			schema: `{type: number, multipleOf: 1e-20, exclusiveMinimum: 1}`,
			want:   map[string]any{"type": "number", "multipleOf": 1e-20, "exclusiveMinimum": 1},
		},
		{
			name:   "OpenAPI 3.0 form",
			schema: `{type: integer, minimum: 5, exclusiveMinimum: true, maximum: 8, exclusiveMaximum: true}`,
			want:   map[string]any{"type": "integer", "minimum": 6, "maximum": 7},
		},
		{
			name:   "OpenAPI 3.0 form that is false changes nothing",
			schema: `{type: integer, minimum: 5, exclusiveMinimum: false}`,
			want:   map[string]any{"type": "integer", "minimum": 5},
		},
		{
			name:   "the tighter of an inclusive and an exclusive bound stays",
			schema: `{type: integer, minimum: 3, exclusiveMinimum: 0, maximum: 4, exclusiveMaximum: 100}`,
			want:   map[string]any{"type": "integer", "minimum": 3, "maximum": 4},
		},
		{
			name:   "a number with no step has no exact form",
			schema: `{type: number, exclusiveMinimum: 0}`,
			want:   map[string]any{"type": "number", "exclusiveMinimum": 0},
		},
		{
			name:   "a string is not a number",
			schema: `{type: string, exclusiveMinimum: 0}`,
			want:   map[string]any{"type": "string", "exclusiveMinimum": 0},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			spec := "components:\n  schemas:\n    Thing: " + test.schema + "\n"
			out, err := exclusiveBoundsExact([]byte(spec))
			if err != nil {
				t.Fatal(err)
			}
			var got struct {
				Components struct {
					Schemas map[string]map[string]any
				}
			}
			if err := yaml.Unmarshal(out, &got); err != nil {
				t.Fatal(err)
			}
			if schema := got.Components.Schemas["Thing"]; !sameNumbers(schema, test.want) {
				t.Errorf("schema = %v, want %v", schema, test.want)
			}
		})
	}
}

func sameNumbers(got, want map[string]any) bool {
	if len(got) != len(want) {
		return false
	}
	for key, w := range want {
		g, ok := got[key]
		if !ok {
			return false
		}
		if gn, gok := toFloat(g); gok {
			if wn, wok := toFloat(w); wok && gn == wn {
				continue
			}
			return false
		}
		if g != w {
			return false
		}
	}
	return true
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}

// A contract with no exclusive bound is returned byte for byte, and a property that is named
// exclusiveMinimum is not a bound.
func TestExclusiveBoundsExactLeavesOtherContractsAlone(t *testing.T) {
	plain := []byte("components:\n  schemas:\n    Thing: {type: integer, minimum: 1}\n")
	if out, err := exclusiveBoundsExact(plain); err != nil || !bytes.Equal(out, plain) {
		t.Errorf("a contract with no exclusive bound changed: %q, %v", out, err)
	}
	named := []byte("components:\n  schemas:\n    Thing:\n      type: object\n      properties:\n        exclusiveMinimum: {type: integer}\n")
	if out, err := exclusiveBoundsExact(named); err != nil || !bytes.Equal(out, named) {
		t.Errorf("a property named exclusiveMinimum changed the contract: %q, %v", out, err)
	}
}

// The model refuses an exclusive bound and accepts the inclusive form that has the same values,
// and the Terraform schema gets plain AtLeast and AtMost validators.
func TestExclusiveBoundsGenerateInclusiveValidators(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "gauge.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := model.Load(data)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := model.Build(doc, "Gauge"); err == nil || !strings.Contains(err.Error(), "exclusiveMinimum") {
		t.Fatalf("the model reads the contract as written; want its exclusive bound error, got %v", err)
	}
	if data, err = exclusiveBoundsExact(data); err != nil {
		t.Fatal(err)
	}
	if doc, err = model.Load(data); err != nil {
		t.Fatal(err)
	}
	r, err := model.Build(doc, "Gauge")
	if err != nil {
		t.Fatalf("the exact form must build: %v", err)
	}
	out, err := buildTFResource(r, "gauge")
	if err != nil {
		t.Fatal(err)
	}
	validators := func(name string) []string {
		i := slices.IndexFunc(out.Attributes, func(a *tfAttr) bool { return a.Name == name })
		if i < 0 {
			t.Fatalf("no attribute %s", name)
		}
		return out.Attributes[i].Validators
	}
	for name, want := range map[string]string{
		"latency_ms": "float64validator.AtLeast(0.001)",
		"retries":    "int32validator.Between(1, 10)",
	} {
		if got := validators(name); !slices.Contains(got, want) {
			t.Errorf("%s validators = %v, want %s", name, got, want)
		}
	}
}

// A contract that the generator still refuses is reported as before, with the code of the iac module.
func TestExclusiveBoundWithoutAStepStaysAnEligibilityIssue(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "gauge.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.Replace(data, []byte("      multipleOf: 0.001\n"), nil, 1)
	_, err = validateOpenAPI(data, "Gauge", model.OperationIDs{}, "example.com/iac-test-sdk", providerModulePath)
	if err == nil || !strings.Contains(err.Error(), "NUMERIC_EXCLUSIVE_BOUND_UNSUPPORTED") {
		t.Fatalf("want NUMERIC_EXCLUSIVE_BOUND_UNSUPPORTED, got %v", err)
	}
}
