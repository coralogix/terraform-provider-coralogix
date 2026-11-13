package generator

import (
	"strings"
	"testing"
)

func TestAcceptanceSkipTestsMatchGeneratedSubtests(t *testing.T) {
	for name, test := range map[string]struct {
		text  string
		sweep bool
	}{
		"unknown":            {"skipTests: {full-lifecyle: API issue}\n", false},
		"upgrade absent":     {"skipTests: {upgrade-full: API issue}\n", false},
		"case absent":        {"upgradeFrom: '1.0.0'\nskipTests: {upgrade-missing: API issue}\n", false},
		"sweep absent":       {"skipTests: {values-and-choices: API issue}\n", false},
		"split sweep absent": {"stepCopies: 1\nskipTests: {values-and-choices-999: API issue}\n", true},
		"unsplit sweep":      {"stepCopies: 1\nskipTests: {values-and-choices: API issue}\n", true},
		"split name":         {"skipTests: {values-and-choices-1: API issue}\n", true},
		"offline test":       {"skipTests: {TestFailurePathsThingGenerated: API issue}\n", false},
	} {
		t.Run(name, func(t *testing.T) {
			res := extraUpgradeResource()
			if test.sweep {
				res = sweepResource()
			}
			_, err := buildAcceptance(res, "example.com/provider", testSynth(t, test.text).file, nil)
			if err == nil || !strings.Contains(err.Error(), "matches no generated live subtest") {
				t.Fatalf("err = %v, want a missing subtest error", err)
			}
		})
	}
}

func TestAcceptanceSkipTestsCoverMinimalAndSweepSubtests(t *testing.T) {
	for name, prefix := range map[string]string{
		"minimal-lifecycle":    "",
		"upgrade-minimal":      "upgradeFrom: '1.0.0'\n",
		"values-and-choices":   "",
		"values-and-choices-1": "stepCopies: 1\n",
	} {
		t.Run(name, func(t *testing.T) {
			file := testSynth(t, prefix+"skipTests: {"+name+": API issue}\n").file
			data, err := buildAcceptance(sweepResource(), "example.com/provider", file, nil)
			if err != nil {
				t.Fatal(err)
			}
			out, err := renderAcceptance(data)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(out), "t.Run(\""+name+"\", func(t *testing.T) {\n\t\tt.Skip(\"API issue\")") {
				t.Fatalf("%s does not start with its skip reason:\n%s", name, out)
			}
		})
	}
}

func TestAcceptanceSkipTestsKeepOtherSubtestsActive(t *testing.T) {
	file := extraUpgradeFile(t)
	const reason = "API says \"not ready\".\nRetry after `validation` is fixed."
	file.SkipTests = map[string]string{"full-lifecycle": reason, "upgrade-full": reason, "upgrade-write-only": reason}
	data, err := buildAcceptance(extraUpgradeResource(), "example.com/provider", file, nil)
	if err != nil {
		t.Fatal(err)
	}
	out, err := renderAcceptance(data)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	for _, name := range []string{"full-lifecycle", "upgrade-full", "upgrade-write-only"} {
		want := "t.Run(\"" + name + "\", func(t *testing.T) {\n\t\tt.Skip(\"API says \\\"not ready\\\".\\nRetry after `validation` is fixed.\")"
		if !strings.Contains(text, want) {
			t.Errorf("%s does not start with the quoted skip reason:\n%s", name, text)
		}
	}
	for _, name := range []string{"minimal-lifecycle", "upgrade-minimal"} {
		if !strings.Contains(text, "t.Run(\""+name+"\", func(t *testing.T) {\n\t\trun := newRun()") {
			t.Errorf("%s must remain active", name)
		}
	}
	if strings.Count(text, "t.Skip(\"API says") != len(file.SkipTests) {
		t.Fatal("a test outside skipTests was skipped")
	}
}
