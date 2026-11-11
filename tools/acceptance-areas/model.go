package main

import (
	"fmt"
	"sort"
	"strings"
)

// model joins the area table, the Go source, and the jobs of the workflow.
type model struct {
	cfg     *config
	files   []string
	gofiles []*goFile
	jobs    []job
}

func loadModel() (*model, error) {
	cfg, err := loadConfig(configPath)
	if err != nil {
		return nil, err
	}
	jobs, err := loadJobs(workflowPath)
	if err != nil {
		return nil, err
	}
	files, err := gitFiles()
	if err != nil {
		return nil, err
	}
	module, err := modulePath()
	if err != nil {
		return nil, err
	}
	gofiles, err := scanGo(files, module)
	if err != nil {
		return nil, err
	}
	return &model{cfg: cfg, files: files, gofiles: gofiles, jobs: jobs}, nil
}

// ownerOf returns the area of a file, or "" for a shared file. An ambiguous or unknown
// file counts as shared. Shared is the safe side: a change to it runs every job.
func (m *model) ownerOf(p string) string {
	owner, k, err := m.cfg.classify(p)
	if err != nil || k != kindArea {
		return ""
	}
	return owner
}

// testJobs returns, for each area, the jobs that run its tests. The key "" holds the jobs
// that run the tests of shared files. A change to any area also runs those jobs, because a
// test of shared code may depend on any area.
func (m *model) testJobs() map[string][]string {
	sets := map[string]map[string]bool{}
	for _, f := range m.gofiles {
		if _, k, _ := m.cfg.classify(f.path); k == kindIgnored {
			continue
		}
		area := m.ownerOf(f.path)
		for _, t := range f.tests {
			if sets[area] == nil {
				sets[area] = map[string]bool{}
			}
			for _, j := range jobsFor(m.jobs, t) {
				sets[area][j] = true
			}
		}
	}
	out := map[string][]string{}
	for area, set := range sets {
		for j := range set {
			out[area] = append(out[area], j)
		}
		sort.Strings(out[area])
	}
	return out
}

// detect chooses the jobs for a list of changed files. all is true when every job must run.
func (m *model) detect(changed []string) (code, all bool, jobs []string, why string) {
	areas := map[string]bool{}
	for _, p := range changed {
		owner, k, err := m.cfg.classify(p)
		switch {
		case err == nil && k == kindIgnored:
			continue
		case err == nil && k == kindArea:
			areas[owner] = true
			code = true
		default:
			return true, true, nil, fmt.Sprintf("%s is not in an area", p)
		}
	}
	if !code {
		return false, false, nil, "only ignored files changed"
	}
	byArea := m.testJobs()
	set := map[string]bool{}
	for _, j := range byArea[""] {
		set[j] = true
	}
	affected := m.cfg.dependents(areas)
	for a := range affected {
		for _, j := range byArea[a] {
			set[j] = true
		}
	}
	for j := range set {
		jobs = append(jobs, j)
	}
	sort.Strings(jobs)
	names := make([]string, 0, len(affected))
	for a := range affected {
		names = append(names, a)
	}
	sort.Strings(names)
	return true, false, jobs, "areas: " + strings.Join(names, ", ")
}
