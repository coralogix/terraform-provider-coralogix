package generator

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
)

const probeShapesContract = `{"openapi":"3.1.0","info":{"title":"t","version":"1"},"paths":{},` +
	`"components":{"schemas":{` +
	`"Thing":{"type":"object","oneOf":[{"required":["a"],"type":"object"},{"required":["b"],"type":"object"},` +
	`{"type":"object","not":{"anyOf":[{"required":["a"],"type":"object"},{"required":["b"],"type":"object"}]}}],` +
	`"properties":{"a":{"type":"string"},"b":{"type":"string"}}},` +
	`"ThingCreate":{"type":"object","properties":{"name":{"type":"string"}}},` +
	`"Other":{"type":"object","properties":{"id":{"type":"string"}}}}}}`

// A new nested object is one new message that every message of the group refers to.
func TestProbeMutateAddsOneNewMessageToAGroup(t *testing.T) {
	out, err := probeMutate([]byte(probeShapesContract), ProbeObjectField, [][]string{{"Thing", "ThingCreate"}, {"Other"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := bytes.Count(out, []byte("zzProbeObject0:")); got != 2 {
		t.Errorf("group 0 got the field in %d messages, want 2:\n%s", got, out)
	}
	if got := bytes.Count(out, []byte("ZzProbeMessage0:")); got != 1 {
		t.Errorf("group 0 has %d new messages, want 1:\n%s", got, out)
	}
	if got := bytes.Count(out, []byte("ZzProbeMessage1:")); got != 1 {
		t.Errorf("group 1 has %d new messages, want 1:\n%s", got, out)
	}
	if _, err := model.Load(out); err != nil {
		t.Errorf("the changed contract does not load: %v\n%s", err, out)
	}
}

// A new oneOf arm is required by one new entry, is allowed by the entry that allows no arm, and is
// a property. A message without a oneOf has no arm to add.
func TestProbeMutateAddsAnArmToEveryOneOf(t *testing.T) {
	out, err := probeMutate([]byte(probeShapesContract), ProbeOneOfArm, [][]string{{"Thing", "ThingCreate"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := bytes.Count(out, []byte("zzProbeArm0")); got != 3 {
		t.Errorf("the arm is named %d times, want 3 (the entry, the entry that allows none, the property):\n%s", got, out)
	}
	if bytes.Contains(out, []byte("ThingCreate:\n        type: object\n        oneOf")) || bytes.Count(out, []byte("zzProbeObject")) != 0 {
		t.Errorf("a message without a oneOf got an arm:\n%s", out)
	}
	if _, err := model.Load(out); err != nil {
		t.Errorf("the changed contract does not load: %v\n%s", err, out)
	}
}

// ProbeAll tries the new messages after the other changes, and a resource that takes both passes.
func TestProbeAllTriesTheNewMessageChanges(t *testing.T) {
	results, err := ProbeAll(ProbeOptions{
		Resource:      "KeyThing",
		OpenAPIPath:   filepath.Join("testdata", "keything.yaml"),
		OverridesPath: filepath.Join("testdata", "keything-overrides.yaml"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != len(ProbeChanges)+len(ProbeShapeChanges) {
		t.Fatalf("got %d results, want %d", len(results), len(ProbeChanges)+len(ProbeShapeChanges))
	}
	for _, result := range results {
		if !result.Passed {
			t.Errorf("%s: %s %s", result.Change, result.Component, result.Problem)
		}
	}
	if got := results[len(results)-1].Change; got != ProbeOneOfArm {
		t.Errorf("last change = %s, want %s", got, ProbeOneOfArm)
	}
}
