package generator

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
)

// The tfgen commands read the overrides file from a path, and a value that the API adds to an enum
// then needs no names entry: it takes the plain name. The same file, parsed by a test without the
// commands, still asks for the entry (TestNewEnumValueWithoutAStyleAsksForANamesEntry).
func TestOverridesReadFromAPathNameANewEnumValueByThePlainRule(t *testing.T) {
	const lines = "resource: LegacyThing\nmode: existing\nvalidators:\n  inferred: false\nenums:\n  legacy.Kind:\n    zero: unspecified\n    names: {ALPHA: 'A.', BETA: 'B.'}\n"
	path := filepath.Join(t.TempDir(), "behavior-overrides.yaml")
	if err := os.WriteFile(path, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := readOverrides(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := validateOpenAPIWith(legacySpec(t), "LegacyThing", model.OperationIDs{}, "sdk", "provider", file); slices.Contains(eligibilityCodes(t, err), "ENUM_NAME_UNDECIDED") {
		t.Fatalf("err = %v, want no ENUM_NAME_UNDECIDED for a file that tfgen reads", err)
	}
	if got := file.Enums["legacy.Kind"].TFValue("GAMMA"); got != "gamma" {
		t.Fatalf("TFValue(GAMMA) = %q, want the plain name gamma", got)
	}
	if got := file.Enums["legacy.Kind"].TFValue("ALPHA"); got != "A." {
		t.Fatalf("TFValue(ALPHA) = %q, a released name must not change", got)
	}
}
