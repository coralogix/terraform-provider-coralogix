package main

import (
	"fmt"
	"sort"
	"strings"
)

const maxExamples = 3

// lint checks that the area table is complete and that no area uses code of another area
// without saying so. Both make the file filter safe: a change inside an area then runs every
// test that can see that change.
func (m *model) lint() []string {
	var errs []string
	errs = append(errs, m.lintFiles()...)
	errs = append(errs, m.lintTable()...)
	errs = append(errs, m.lintEdges()...)
	errs = append(errs, m.lintTests()...)
	return errs
}

// lintFiles finds files that no area owns, inside the directories where every file must
// have an owner.
func (m *model) lintFiles() []string {
	var unowned []string
	var errs []string
	for _, p := range m.files {
		_, k, err := m.cfg.classify(p)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		if k == kindImplicit && m.cfg.isStrict(p) {
			unowned = append(unowned, p)
		}
	}
	if len(unowned) > 0 {
		errs = append(errs, fmt.Sprintf(
			"%d files are in no area and not in the shared list. Add each one to an area, or to `shared`:\n%s",
			len(unowned), examples(unowned)))
	}
	return errs
}

// lintTable finds entries of the table that point to nothing.
func (m *model) lintTable() []string {
	var errs []string
	names := make([]string, 0, len(m.cfg.areas))
	for n := range m.cfg.areas {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		errs = append(errs, m.lintArea(name, m.cfg.areas[name])...)
	}
	for _, p := range m.cfg.prefixes {
		if p.owner == "" && !m.hasFile(p.path) {
			errs = append(errs, fmt.Sprintf("shared path %s matches no file", p.path))
		}
	}
	return errs
}

func (m *model) lintArea(name string, a rawArea) []string {
	var errs []string
	for _, d := range a.DependsOn {
		if _, ok := m.cfg.areas[d]; !ok || d == name {
			errs = append(errs, fmt.Sprintf("area %s: depends_on %q is not another area", name, d))
		}
	}
	for _, p := range a.Paths {
		if !m.hasFile(p) {
			errs = append(errs, fmt.Sprintf("area %s: path %s matches no file", name, p))
		}
	}
	for _, r := range a.Resources {
		if !m.hasFile(rawArea{Resources: []string{r}}.expand()...) {
			errs = append(errs, fmt.Sprintf("area %s: resource %s matches no file", name, r))
		}
	}
	for _, g := range a.Generated {
		if !m.hasFile("internal/provider/generated/" + g + "/") {
			errs = append(errs, fmt.Sprintf("area %s: generated %s matches no file", name, g))
		}
	}
	return errs
}

// hasFile reports whether any file starts with one of the prefixes.
func (m *model) hasFile(prefixes ...string) bool {
	for _, f := range m.files {
		for _, p := range prefixes {
			if strings.HasPrefix(f, p) {
				return true
			}
		}
	}
	return false
}

// lintEdges finds code of one area that uses code of another area, without depends_on.
func (m *model) lintEdges() []string {
	type pair struct{ from, to string }
	found := map[pair][]string{}
	for _, e := range refEdges(m.gofiles) {
		from, to := m.ownerOf(e.from), m.ownerOf(e.to)
		if from == "" || to == "" || from == to || m.cfg.dependsOn(from, to) {
			continue
		}
		k := pair{from, to}
		found[k] = append(found[k], fmt.Sprintf("%s uses %s from %s", e.from, e.via, e.to))
	}
	var pairs []pair
	for k := range found {
		pairs = append(pairs, k)
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].from != pairs[j].from {
			return pairs[i].from < pairs[j].from
		}
		return pairs[i].to < pairs[j].to
	})
	var errs []string
	for _, k := range pairs {
		errs = append(errs, fmt.Sprintf(
			"area %s uses code of area %s. Add `depends_on: [%s]` to area %s, or remove the use:\n%s",
			k.from, k.to, k.to, k.from, examples(found[k])))
	}
	return errs
}

// lintTests finds tests that no job starts. The coverage check of the nightly run finds the
// same tests. This check finds them before the merge.
func (m *model) lintTests() []string {
	var missing []string
	for _, f := range m.gofiles {
		for _, t := range f.tests {
			if len(jobsFor(m.jobs, t)) == 0 {
				missing = append(missing, fmt.Sprintf("%s (%s)", t, f.path))
			}
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return []string{fmt.Sprintf("%d tests are started by no job. Check the shard patterns in the workflow:\n%s",
		len(missing), examples(missing))}
}

func examples(items []string) string {
	sort.Strings(items)
	var b strings.Builder
	for i, it := range items {
		if i == maxExamples {
			fmt.Fprintf(&b, "  ... and %d more\n", len(items)-maxExamples)
			break
		}
		fmt.Fprintf(&b, "  %s\n", it)
	}
	return strings.TrimRight(b.String(), "\n")
}
