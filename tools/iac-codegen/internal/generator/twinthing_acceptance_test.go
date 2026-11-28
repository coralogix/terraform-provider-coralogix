package generator

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// updateTwinGolden rewrites the recorded acceptance test of twins that have the same fields. The file is
// not under testdata/golden: it records the output that the generator made before folds had sides.
var updateTwinGolden = flag.Bool("update-twin-golden", false, "rewrite testdata/twinthing-same-acceptance.go.txt")

// The fields that only the shown twin (keepParams) has, and those that only the folded twin
// (dropParams) has, in the twin thing of testdata/twinthing.yaml.
var (
	twinShownOnly  = []string{"label", "strict", "limits"}
	twinFoldedOnly = []string{"reason", "hard", "threshold", "extra"}
)

func generateTwinThingAcceptance(t *testing.T, edit func(string) string) string {
	t.Helper()
	input, loadDir := twinThingInput(t, edit)
	out := filepath.Join(t.TempDir(), "parsething")
	options := Options{
		Resource:       "ParseThing",
		OutputDir:      out,
		OverridesPath:  filepath.Join("testdata", "twinthing-overrides.yaml"),
		AcceptancePath: filepath.Join("testdata", "twinthing-acceptance.yaml"),
	}
	if err := generateFromInput(options, input, loadDir); err != nil {
		t.Fatal(err)
	}
	return readGenerated(t, out, "acceptance_test.go")
}

var accConfigConst = regexp.MustCompile("(?s)const (\\w+Config) = `(.*?)`")

// accConfigs returns the HCL of each config constant of a generated acceptance test, by name.
func accConfigs(t *testing.T, text string) map[string]string {
	t.Helper()
	configs := map[string]string{}
	for _, m := range accConfigConst.FindAllStringSubmatch(text, -1) {
		configs[m[1]] = m[2]
	}
	if configs["fullConfig"] == "" {
		t.Fatal("the generated acceptance test has no full config")
	}
	return configs
}

// setsField reports whether the config sets the attribute (a line "name = ...").
func setsField(config, name string) bool {
	return regexp.MustCompile(`(?m)^\s*` + name + ` = `).MatchString(config)
}

func setsAny(config string, names []string) bool {
	return slices.ContainsFunc(names, func(name string) bool { return setsField(config, name) })
}

// An object of a fold that has fields of one twin only is set with the fields of one twin: the
// flag selects the twin, and the generated expand refuses the fields of the other.
func TestTwinThingAcceptanceSetsTheFieldsOfOneTwin(t *testing.T) {
	configs := accConfigs(t, generateTwinThingAcceptance(t, keep))
	for name, config := range configs {
		if setsAny(config, twinShownOnly) && setsAny(config, twinFoldedOnly) {
			t.Errorf("%s sets fields of both twins:\n%s", name, config)
		}
		if setsAny(config, twinShownOnly) && strings.Contains(config, "keep_matching = false") {
			t.Errorf("%s sets a field of the shown twin with a false flag:\n%s", name, config)
		}
		if setsAny(config, twinFoldedOnly) && strings.Contains(config, "keep_matching = true") {
			t.Errorf("%s sets a field of the folded twin with a true flag:\n%s", name, config)
		}
	}
}

// The full config shows the shown twin, and a step of the sweep shows the folded twin.
func TestTwinThingAcceptanceCoversBothTwins(t *testing.T) {
	configs := accConfigs(t, generateTwinThingAcceptance(t, keep))
	for _, name := range []string{"fullConfig", "updatedConfig"} {
		config := configs[name]
		if !setsField(config, "label") || strings.Contains(config, "keep_matching = false") {
			t.Errorf("%s does not show the shown twin:\n%s", name, config)
		}
	}
	covered := false
	for name, config := range configs {
		if strings.HasPrefix(name, "variant") && strings.Contains(config, "keep_matching = false") && setsField(config, "reason") && setsField(config, "threshold") {
			covered = true
		}
	}
	if !covered {
		t.Errorf("no step of the sweep sets the folded twin: %v", configs)
	}
}

// Twins that have the same fields have no side to choose: the acceptance test is the one that the
// generator made before sides existed (the updated config flips the flag).
func TestTwinThingAcceptanceOfTwinsWithTheSameFieldsIsUnchanged(t *testing.T) {
	same := func(spec string) string {
		for _, drop := range []string{
			"        label: {type: string, x-coralogix-presence: true}\n",
			"        strict: {type: string, minLength: 1}\n",
			"        limits: {$ref: '#/components/schemas/ParseLimits'}\n",
			"        reason: {type: string, x-coralogix-presence: true}\n",
			"        hard: {type: string, minLength: 1}\n",
			"        threshold: {type: integer, format: int64, x-coralogix-presence: true}\n",
			"        extra: {$ref: '#/components/schemas/ParseExtra'}\n",
			"      required: [strict]\n",
			"      required: [hard]\n",
		} {
			spec = strings.Replace(spec, drop, "", 1)
		}
		return spec
	}
	got := generateTwinThingAcceptance(t, same)
	configs := accConfigs(t, got)
	if !strings.Contains(configs["fullConfig"], "keep_matching = true") || !strings.Contains(configs["updatedConfig"], "keep_matching = false") {
		t.Errorf("the flag is not true in the full config and false in the updated one:\n%s\n%s", configs["fullConfig"], configs["updatedConfig"])
	}
	golden := filepath.Join("testdata", "twinthing-same-acceptance.go.txt")
	if *updateTwinGolden {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != got {
		t.Fatal("the acceptance test of twins with the same fields differs from testdata/twinthing-same-acceptance.go.txt")
	}
}
