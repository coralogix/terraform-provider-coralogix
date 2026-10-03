// Package acceptance reads the acceptance file of a generated resource. The generator writes an
// acceptance test from the schema of the resource. The file states what the schema cannot:
// the environment variables that the test needs, the resources that the test config depends on,
// and the values of the fields that must be real.
package acceptance

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"sort"
	"strings"

	"go.yaml.in/yaml/v4"
)

// FileName is the name of the acceptance file. It sits in the output directory of the resource.
const FileName = "acceptance.yaml"

// File is the acceptance file.
//
//	resource: GlobalRouter
//	env: [SLACK_INTEGRATION_ID]        # the test stops if one is missing
//	prerequisites: |                   # HCL that the config needs
//	  resource "coralogix_connector" "slack" { id = "@{run}-slack" ... }
//	values:                            # HCL for a field, instead of a made-up value
//	  rules[].targets[].connector_id: coralogix_connector.slack.id
//	upgradeFrom: "3.19.0"              # a released provider for the upgrade test
//
// In prerequisites and values, @{run} is a unique id of the test run, and @{env.NAME} is the value
// of an environment variable from env.
type File struct {
	// Resource is the OpenAPI component. The file must match --resource.
	Resource string `yaml:"resource"`
	// Env are the environment variables that the test needs besides the API key.
	Env []string `yaml:"env"`
	// Prerequisites is HCL that goes before the resource in every config.
	Prerequisites string `yaml:"prerequisites"`
	// Values maps the path of a field to an HCL expression. A path names the attributes from the
	// resource down, and [] marks a list or set element: rules[].targets[].connector_id.
	Values map[string]string `yaml:"values"`
	// Skip lists the fields that the test configs leave out, for example a deprecated field that
	// the API refuses next to its replacement. A path has the same form as in values.
	Skip []string `yaml:"skip"`
	// Minimal lists the optional fields that the minimal config sets, because the API refuses
	// the resource without them. Each is set with all of its own attributes.
	Minimal []string `yaml:"minimal"`
	// UpgradeMinimal lists the optional fields that the released provider needs to create the
	// resource, on top of minimal. The minimal upgrade test sets them.
	UpgradeMinimal []string `yaml:"upgradeMinimal"`
	// UpgradeFrom is a released provider version. The upgrade test creates the resource with it
	// and plans with this build. Empty: no upgrade test.
	UpgradeFrom string `yaml:"upgradeFrom"`
}

var (
	envName     = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)
	fieldPath   = regexp.MustCompile(`^[a-z][a-z0-9_]*(\[\])?(\.[a-z][a-z0-9_]*(\[\])?)*$`)
	version     = regexp.MustCompile(`^\d+\.\d+\.\d+$`)
	placeholder = regexp.MustCompile(`@\{[^}]*\}`)
)

// Parse reads the file. An unknown key or a wrong value is an error.
func Parse(data []byte) (*File, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var f File
	if err := dec.Decode(&f); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("%s is empty", FileName)
		}
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	if err := f.check(); err != nil {
		return nil, fmt.Errorf("%s: %w", FileName, err)
	}
	return &f, nil
}

func (f *File) check() error {
	if f.Resource == "" {
		return errors.New("resource is required")
	}
	steps := []func() error{f.checkEnv, f.checkValues, f.checkPaths, f.checkVersionAndText}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}
	return nil
}

func (f *File) checkEnv() error {
	seen := map[string]bool{}
	for _, name := range f.Env {
		if !envName.MatchString(name) {
			return fmt.Errorf("env %q is not an environment variable name", name)
		}
		if seen[name] {
			return fmt.Errorf("env %q is listed twice", name)
		}
		seen[name] = true
	}
	return f.checkPlaceholders("prerequisites", f.Prerequisites)
}

func (f *File) checkValues() error {
	for _, path := range f.valuePaths() {
		if !fieldPath.MatchString(path) {
			return fmt.Errorf("values: %q is not a field path such as rules[].targets[].connector_id", path)
		}
		if strings.TrimSpace(f.Values[path]) == "" {
			return fmt.Errorf("values.%s is empty", path)
		}
		if err := f.checkPlaceholders("values."+path, f.Values[path]); err != nil {
			return err
		}
	}
	return nil
}

func (f *File) checkPaths() error {
	for name, paths := range map[string][]string{"skip": f.Skip, "minimal": f.Minimal, "upgradeMinimal": f.UpgradeMinimal} {
		for _, path := range paths {
			if !fieldPath.MatchString(path) {
				return fmt.Errorf("%s: %q is not a field path such as rules[].targets[].preset_id", name, path)
			}
		}
	}
	return nil
}

func (f *File) checkVersionAndText() error {
	if f.UpgradeFrom != "" && !version.MatchString(f.UpgradeFrom) {
		return fmt.Errorf("upgradeFrom is %q, want a release such as 3.19.0", f.UpgradeFrom)
	}
	if f.UpgradeFrom == "" && len(f.UpgradeMinimal) != 0 {
		return errors.New("upgradeMinimal needs upgradeFrom: without it there is no upgrade test")
	}
	for _, text := range append([]string{f.Prerequisites}, f.valueTexts()...) {
		if strings.Contains(text, "`") {
			return errors.New("a backtick is not allowed in HCL: the generated test writes it in a Go raw string")
		}
	}
	return nil
}

// checkPlaceholders accepts only @{run} and @{env.NAME} with NAME in env.
func (f *File) checkPlaceholders(where, text string) error {
	for _, p := range placeholder.FindAllString(text, -1) {
		name := p[2 : len(p)-1]
		if name == "run" {
			continue
		}
		if env, ok := strings.CutPrefix(name, "env."); ok && slices.Contains(f.Env, env) {
			continue
		}
		return fmt.Errorf("%s: %s is not @{run} or @{env.NAME} with NAME in env", where, p)
	}
	return nil
}

// valuePaths returns the paths of values, sorted.
func (f *File) valuePaths() []string {
	paths := make([]string, 0, len(f.Values))
	for path := range f.Values {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func (f *File) valueTexts() []string {
	texts := make([]string, 0, len(f.Values))
	for _, path := range f.valuePaths() {
		texts = append(texts, f.Values[path])
	}
	return texts
}

// ValuePaths returns the paths in values, sorted. The generator checks each against the schema.
func (f *File) ValuePaths() []string { return f.valuePaths() }
