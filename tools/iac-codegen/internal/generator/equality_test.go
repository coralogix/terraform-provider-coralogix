package generator

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/issue"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/model"
)

const configThingHead = "resource: ConfigThing\nmode: existing\nvalidators:\n  inferred: false\ntypes:\n"

func configThingSpec(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "configthing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// extraFields adds fields of other kinds to the resource component of the contract.
const extraFields = `        version: {type: string, readOnly: true}
        mode: {type: string, enum: [MODE_UNSPECIFIED, MODE_FAST], x-coralogix-presence: true}
        createTime: {type: string, format: date-time, readOnly: true}
        count: {type: integer, format: int32, x-coralogix-presence: true}
`

func TestEqualityRejectsFieldsThatAreNoDocument(t *testing.T) {
	spec := strings.Replace(configThingSpec(t), "        version: {type: string, readOnly: true}\n", extraFields, 1)
	if spec == configThingSpec(t) {
		t.Fatal("the test contract did not change: update the replaced text")
	}
	tests := map[string]struct {
		component, field, line string
		code                   string
	}{
		"list of objects":   {"ConfigThing", "remotes", "{equality: yaml}", "OVERRIDE_EQUALITY_NOT_STRING"},
		"enum":              {"ConfigThing", "mode", "{equality: json}", "OVERRIDE_EQUALITY_NOT_STRING"},
		"date-time":         {"ConfigThing", "createTime", "{equality: yaml}", "OVERRIDE_EQUALITY_NOT_STRING"},
		"integer":           {"ConfigThing", "count", "{equality: json}", "OVERRIDE_EQUALITY_NOT_STRING"},
		"read-only field":   {"ConfigThing", "version", "{equality: yaml}", "OVERRIDE_EQUALITY_COMPUTED"},
		"readOnly override": {"ConfigThing", "name", "{readOnly: true, equality: yaml}", "OVERRIDE_EQUALITY_COMPUTED"},
		"nested string":     {"ConfigRemote", "name", "{equality: yaml}", ""},
		"missing field":     {"ConfigRemote", "rawConfig", "{equality: yaml}", "OVERRIDE_UNUSED"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			text := configThingHead + "  " + test.component + ":\n    fields:\n      " + test.field + ": " + test.line + "\n"
			_, err := validateOpenAPIWith([]byte(spec), "ConfigThing", model.OperationIDs{}, "sdk", "provider", mustParse(t, text))
			location := "behavior-overrides.yaml:types." + test.component + ".fields." + test.field
			if test.code == "" {
				// Any plain string field accepts the key, nested ones too.
				for _, item := range reportOf(t, err) {
					if strings.HasPrefix(item.Code, "OVERRIDE_EQUALITY") {
						t.Fatalf("report = %v, want no equality issue", err)
					}
				}
				return
			}
			if report := requireReport(t, err); !reportHas(report, test.code, location) {
				t.Fatalf("report:\n%s\nwant %s at %s", report, test.code, location)
			}
		})
	}
}

// The flatten of a map of objects and of a computed object passes no prior model, so equality in the
// objects they hold is rejected instead of generating code that does not compile.
func TestEqualityRejectsContainersWithoutAPrior(t *testing.T) {
	remotes := "                remotes:\n                  type: array\n                  items: {$ref: '#/components/schemas/ConfigRemoteCreate'}\n"
	byName := "                byName:\n                  type: object\n                  additionalProperties: {$ref: '#/components/schemas/ConfigRemoteCreate'}\n"
	mapSpec := strings.ReplaceAll(configThingSpec(t), remotes, remotes+byName)
	mapSpec = strings.Replace(mapSpec, "    ConfigRemote:\n", "        byName:\n          type: object\n          additionalProperties: {$ref: '#/components/schemas/ConfigRemote'}\n    ConfigRemote:\n", 1)
	computedSpec := strings.Replace(configThingSpec(t), "    ConfigRemote:\n",
		"        status:\n          readOnly: true\n          allOf:\n            - $ref: '#/components/schemas/ConfigStatus'\n"+
			"    ConfigStatus:\n      type: object\n      required: []\n      properties:\n        document: {type: string, x-coralogix-presence: true}\n"+
			"    ConfigRemote:\n", 1)
	if strings.Count(mapSpec, "byName:") != 3 || !strings.Contains(computedSpec, "ConfigStatus:") {
		t.Fatal("the test contracts did not change as intended: update the replaced text")
	}
	tests := map[string]struct{ spec, overrides string }{
		"map of objects":  {mapSpec, "  ConfigRemote:\n    fields:\n      rawConfiguration: {equality: yaml}\n"},
		"computed object": {computedSpec, "  ConfigStatus:\n    fields:\n      document: {equality: yaml}\n"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := validateOpenAPIWith([]byte(test.spec), "ConfigThing", model.OperationIDs{}, "sdk", "provider", mustParse(t, configThingHead+test.overrides))
			report := requireReport(t, err)
			if !reportHas(report, "OVERRIDE_PRIOR_CONTAINER", "renderer.conversion") || len(report) != 1 {
				t.Fatalf("report:\n%s\nwant only OVERRIDE_PRIOR_CONTAINER", report)
			}
		})
	}
}

// A set has no order to pair prior items by. The generator does not convert a set of objects yet,
// so the check is tested on its own: it must keep rejecting equality there once sets are supported.
func TestPriorContainersRejectSetsOfObjects(t *testing.T) {
	item := &convObject{Model: "ItemModel", NeedsPrior: true}
	for collection, want := range map[string]bool{"List": false, "Set": true} {
		d := &convData{Objects: []*convObject{{Model: "RootModel", Fields: []*convField{
			{TFName: "items", Conv: convObjects, Collection: collection, Object: item},
		}}}}
		if err := checkPriorContainers(d); errors.Is(err, errPriorContainer) != want {
			t.Errorf("%s: err = %v, want rejected = %t", collection, err, want)
		}
	}
}

// check --overrides and generate share one eligibility path, so both report the same issues.
func TestCheckAndGenerateAgreeOnEquality(t *testing.T) {
	dir := t.TempDir()
	specPath, overridesPath := filepath.Join(dir, "openapi.yaml"), filepath.Join(dir, "overrides.yaml")
	if err := os.WriteFile(specPath, []byte(configThingSpec(t)), 0o644); err != nil {
		t.Fatal(err)
	}
	text := configThingHead + "  ConfigThing:\n    fields:\n      version: {equality: json}\n      remotes: {equality: yaml}\n"
	if err := os.WriteFile(overridesPath, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	checkErr := Check(CheckOptions{Resource: "ConfigThing", OpenAPIPath: specPath, OverridesPath: overridesPath})
	input, loadDir := syntheticInput(t)
	input.OpenAPI = []byte(configThingSpec(t))
	generateErr := generateFromInput(Options{Resource: "ConfigThing", OutputDir: filepath.Join(dir, "configthing"), OverridesPath: overridesPath}, input, loadDir)
	checkReport, generateReport := requireReport(t, checkErr), requireReport(t, generateErr)
	if checkReport.Error() != generateReport.Error() {
		t.Fatalf("check:\n%s\ngenerate:\n%s", checkReport, generateReport)
	}
	if !reportHas(checkReport, "OVERRIDE_EQUALITY_COMPUTED", "behavior-overrides.yaml:types.ConfigThing.fields.version") ||
		!reportHas(checkReport, "OVERRIDE_EQUALITY_NOT_STRING", "behavior-overrides.yaml:types.ConfigThing.fields.remotes") {
		t.Fatalf("report = %v, want both equality issues", checkReport)
	}
}

// A resource without an equality line has no equality file, and keeps its output.
func TestEqualityFileOnlyWithAnEqualityLine(t *testing.T) {
	if _, err := os.Stat(filepath.Join("testdata", "golden", "legacything", "equality.go")); !os.IsNotExist(err) {
		t.Fatalf("legacything has equality.go: %v", err)
	}
	if _, err := os.Stat(filepath.Join("testdata", "golden", "configthing", "equality.go")); err != nil {
		t.Fatal(err)
	}
}

// reportOf returns the eligibility report of err, or nil when err is nil.
func reportOf(t *testing.T, err error) issue.Report {
	t.Helper()
	if err == nil {
		return nil
	}
	var eligibility *EligibilityError
	if !errors.As(err, &eligibility) {
		t.Fatalf("err = %v, want an EligibilityError", err)
	}
	return eligibility.Report
}

// requireReport returns the eligibility report of err, which must not be nil.
func requireReport(t *testing.T, err error) issue.Report {
	t.Helper()
	if err == nil {
		t.Fatal("err = nil, want an EligibilityError")
	}
	return reportOf(t, err)
}

func reportHas(report issue.Report, code, location string) bool {
	for _, item := range report {
		if item.Code == code && item.Location == location {
			return true
		}
	}
	return false
}
