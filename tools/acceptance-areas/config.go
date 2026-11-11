package main

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	configPath   = ".github/acceptance-areas.yaml"
	workflowPath = ".github/workflows/acceptance-tests.yml"
)

type rawArea struct {
	// Paths are file or directory prefixes. A prefix that ends in "/" is a directory.
	Paths []string `yaml:"paths"`
	// Resources are Terraform resource names, without the coralogix_ prefix. Each one
	// expands to the test, test data, and example paths of that resource.
	Resources []string `yaml:"resources"`
	// Generated are directory names under internal/provider/generated/.
	Generated []string `yaml:"generated"`
	// DependsOn lists the areas whose code this area uses.
	DependsOn []string `yaml:"depends_on"`
}

type rawConfig struct {
	Ignore []string           `yaml:"ignore"`
	Strict []string           `yaml:"strict"`
	Shared []string           `yaml:"shared"`
	Areas  map[string]rawArea `yaml:"areas"`
}

type prefix struct {
	path  string
	owner string // an area name, or "" for a shared prefix
}

type config struct {
	ignore   []*regexp.Regexp
	strict   []string
	prefixes []prefix
	areas    map[string]rawArea
}

func (a rawArea) expand() []string {
	out := append([]string{}, a.Paths...)
	for _, r := range a.Resources {
		out = append(out,
			"internal/provider/resource_coralogix_"+r,
			"internal/provider/data_source_coralogix_"+r,
			"internal/provider/"+r+"_",
			"internal/provider/testdata/"+r,
			"examples/resources/coralogix_"+r,
			"examples/data-sources/coralogix_"+r,
		)
	}
	for _, g := range a.Generated {
		out = append(out, "internal/provider/generated/"+g+"/")
	}
	return out
}

func loadConfig(path string) (*config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var raw rawConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return buildConfig(raw)
}

func buildConfig(raw rawConfig) (*config, error) {
	c := &config{strict: raw.Strict, areas: raw.Areas}
	for _, expr := range raw.Ignore {
		re, err := regexp.Compile(expr)
		if err != nil {
			return nil, fmt.Errorf("ignore %q: %w", expr, err)
		}
		c.ignore = append(c.ignore, re)
	}
	for _, p := range raw.Shared {
		c.prefixes = append(c.prefixes, prefix{path: p})
	}
	for name, area := range raw.Areas {
		for _, p := range area.expand() {
			c.prefixes = append(c.prefixes, prefix{path: p, owner: name})
		}
	}
	return c, nil
}

type kind int

const (
	kindIgnored kind = iota
	kindArea
	kindShared   // listed under shared
	kindImplicit // matches nothing
)

// classify finds the owner of a file. The longest matching prefix wins. Two different
// owners with the same longest prefix length are ambiguous.
func (c *config) classify(path string) (owner string, k kind, err error) {
	for _, re := range c.ignore {
		if re.MatchString(path) {
			return "", kindIgnored, nil
		}
	}
	best := -1
	var owners []string
	for _, p := range c.prefixes {
		if !strings.HasPrefix(path, p.path) {
			continue
		}
		switch {
		case len(p.path) > best:
			best, owners = len(p.path), []string{p.owner}
		case len(p.path) == best:
			owners = append(owners, p.owner)
		}
	}
	if best < 0 {
		return "", kindImplicit, nil
	}
	owners = unique(owners)
	if len(owners) > 1 {
		return "", kindImplicit, fmt.Errorf("%s matches areas %v with equally long prefixes", path, owners)
	}
	if owners[0] == "" {
		return "", kindShared, nil
	}
	return owners[0], kindArea, nil
}

func (c *config) isStrict(path string) bool {
	for _, s := range c.strict {
		if strings.HasPrefix(path, s) {
			return true
		}
	}
	return false
}

// dependents returns the given areas plus every area that depends on them, directly or not.
func (c *config) dependents(areas map[string]bool) map[string]bool {
	out := map[string]bool{}
	for a := range areas {
		out[a] = true
	}
	for changed := true; changed; {
		changed = false
		for name, area := range c.areas {
			if out[name] {
				continue
			}
			for _, d := range area.DependsOn {
				if out[d] {
					out[name], changed = true, true
					break
				}
			}
		}
	}
	return out
}

func (c *config) dependsOn(from, to string) bool {
	for _, d := range c.areas[from].DependsOn {
		if d == to {
			return true
		}
	}
	return false
}

func unique(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}
