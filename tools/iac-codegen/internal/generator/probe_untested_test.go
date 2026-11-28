package generator

import (
	"slices"
	"strings"
	"testing"
)

// A released resource that leaves an attribute out of every config reports it to OnUntested, which
// the probe reads, and does not print the warning.
func TestCheckAllSetGivesTheUntestedAttributesOfAReleasedResourceToTheProbe(t *testing.T) {
	s := testSynth(t, "")
	s.setPaths, s.excused = map[string]bool{"set": true}, map[string]bool{}
	s.settable = map[string]bool{"set": true, "ghost": true, "zz_probe0": true}
	var got []string
	s.file.OnUntested = func(paths []string) { got = paths }
	if err := s.checkAllSet(&tfResource{CRUD: &crudData{Existing: true}}); err != nil {
		t.Fatal(err)
	}
	if want := []string{"ghost", "zz_probe0"}; !slices.Equal(got, want) {
		t.Fatalf("untested = %v, want %v", got, want)
	}
}

// The probe fails a change only for the fields that it added: an attribute that was untested before
// the change is not a gap of the change.
func TestProbeUntestedFailsOnlyForTheFieldsThatTheProbeAdded(t *testing.T) {
	if err := probeUntested([]string{"layout.sections[].old_field"}); err != nil {
		t.Fatalf("an old untested attribute failed the change: %v", err)
	}
	err := probeUntested([]string{"old_field", "rules[].zz_probe3"})
	if err == nil || !strings.Contains(err.Error(), "rules[].zz_probe3") || strings.Contains(err.Error(), "old_field") {
		t.Fatalf("err = %v, want one that names only rules[].zz_probe3", err)
	}
}
