package generator

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
)

// The probe adds one new field to every message of a group, and the new name is unique to the group.
func TestProbeMutateAddsAFieldToEveryMessageOfAGroup(t *testing.T) {
	data := []byte(`{"openapi":"3.1.0","info":{"title":"t","version":"1"},"paths":{},` +
		`"components":{"schemas":{` +
		`"Thing":{"type":"object","properties":{"name":{"type":"string"}}},` +
		`"ThingCreate":{"type":"object","properties":{"name":{"type":"string"}}},` +
		`"Other":{"type":"object","properties":{"id":{"type":"string"}}}}}}`)
	out, err := probeMutate(data, ProbeStringField, [][]string{{"Thing", "ThingCreate"}, {"Other"}})
	if err != nil {
		t.Fatal(err)
	}
	if got := bytes.Count(out, []byte("zzProbe0")); got != 2 {
		t.Errorf("group 0 got the field in %d messages, want 2:\n%s", got, out)
	}
	if got := bytes.Count(out, []byte("zzProbe1")); got != 1 {
		t.Errorf("group 1 got the field in %d messages, want 1:\n%s", got, out)
	}
	if _, err := model.Load(out); err != nil {
		t.Errorf("the changed contract does not load: %v\n%s", err, out)
	}
}

// A new enum value has the prefix of the others, so it reads as a new protobuf value.
func TestProbeMutateAddsAnEnumValueWithThePrefixOfTheOthers(t *testing.T) {
	data := []byte(`{"openapi":"3.1.0","info":{"title":"t","version":"1"},"paths":{},` +
		`"components":{"schemas":{"Level":{"type":"string","enum":["LEVEL_UNSPECIFIED","LEVEL_LOW","LEVEL_HIGH"]}}}}`)
	out, err := probeMutate(data, ProbeEnumValue, [][]string{{"Level"}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("LEVEL_ZZ_PROBE_0")) {
		t.Errorf("the new value is missing or has another prefix:\n%s", out)
	}
	if _, err := model.Load(out); err != nil {
		t.Errorf("the changed contract does not load: %v\n%s", err, out)
	}
}

// A resource whose contract takes every change passes, and one that cannot take a new optional field
// in the item of an unordered set is reported with the message that breaks it.
func TestProbeReportsTheChangesThatBreakAResource(t *testing.T) {
	cases := []struct {
		name, resource string
		wantField      bool   // a new field passes
		wantBroken     string // the message that a new field breaks, when it does not pass
	}{
		{name: "keything", resource: "KeyThing", wantField: true},
		{name: "groupsetthing", resource: "GroupSetThing", wantBroken: "InGroupThing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			options := ProbeOptions{
				Resource:      tc.resource,
				OpenAPIPath:   filepath.Join("testdata", tc.name+".yaml"),
				OverridesPath: filepath.Join("testdata", tc.name+"-overrides.yaml"),
			}
			if _, err := os.Stat(options.OverridesPath); err != nil {
				options.OverridesPath = ""
			}
			results, err := Probe(options)
			if err != nil {
				t.Fatal(err)
			}
			if len(results) != len(ProbeChanges) {
				t.Fatalf("got %d results, want one for each of %v", len(results), ProbeChanges)
			}
			for _, result := range results {
				if result.Change == ProbeEnumValue {
					if !result.Passed {
						t.Errorf("%s: %s %s", result.Change, result.Component, result.Problem)
					}
					continue
				}
				if tc.wantField != result.Passed {
					t.Errorf("%s: passed = %v, want %v (%s)", result.Change, result.Passed, tc.wantField, result.Problem)
				}
				if !result.Passed && !strings.Contains(result.Component, tc.wantBroken) {
					t.Errorf("%s: broken message %q, want one with %q", result.Change, result.Component, tc.wantBroken)
				}
				if !result.Passed && result.Problem == "" {
					t.Errorf("%s: a failed change has no problem text", result.Change)
				}
			}
		})
	}
}

// A resource that already fails the check has nothing to compare with.
func TestProbeRefusesAResourceThatFailsTheCheck(t *testing.T) {
	_, err := Probe(ProbeOptions{Resource: "NoSuchThing", OpenAPIPath: filepath.Join("testdata", "keything.yaml")})
	if err == nil || !strings.Contains(err.Error(), "before any change") {
		t.Fatalf("want an error about the check before the change, got %v", err)
	}
}
