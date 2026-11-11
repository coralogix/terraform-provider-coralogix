package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// filter copies how `go test` reads -run and -skip (see testing/match.go): a top-level `|`
// separates alternatives, and a `/` outside brackets and parentheses separates the pattern
// of one test level from the next.
type filter [][]*regexp.Regexp

func newFilter(pattern string) (filter, error) {
	var f filter
	for _, alt := range splitAlternatives(pattern) {
		var elems []*regexp.Regexp
		for _, e := range alt {
			re, err := regexp.Compile(e)
			if err != nil {
				return nil, fmt.Errorf("pattern %q: %w", pattern, err)
			}
			elems = append(elems, re)
		}
		f = append(f, elems)
	}
	return f, nil
}

func splitAlternatives(s string) [][]string {
	var alts [][]string
	var cur []string
	brackets, parens := 0, 0
	for i := 0; i < len(s); {
		switch s[i] {
		case '[':
			brackets++
		case ']':
			if brackets--; brackets < 0 {
				brackets = 0
			}
		case '(':
			if brackets == 0 {
				parens++
			}
		case ')':
			if brackets == 0 {
				parens--
			}
		case '\\':
			i++
		case '/', '|':
			if brackets == 0 && parens == 0 {
				cur = append(cur, s[:i])
				if s[i] == '|' {
					alts = append(alts, cur)
					cur = nil
				}
				s = s[i+1:]
				i = 0
				continue
			}
		}
		i++
	}
	return append(alts, append(cur, s))
}

// matches reports whether the name path matches. partial is true when the pattern has more
// levels than the name, so a subtest could still match.
func (f filter) matches(name []string) (ok, partial bool) {
	for _, alt := range f {
		matched := true
		for i, s := range name {
			if i >= len(alt) {
				break
			}
			if !alt[i].MatchString(s) {
				matched = false
				break
			}
		}
		if matched {
			return true, len(alt) > len(name)
		}
	}
	return false, false
}

// selector says whether one job starts a top-level test. A job starts a test when -run
// matches it, also in part, and -skip does not match it in full. A skip of a subtest
// leaves the parent test running.
type selector struct {
	run  filter // nil runs everything
	skip filter // nil skips nothing
}

func (s selector) selects(test string) bool {
	if s.run != nil {
		if ok, _ := s.run.matches([]string{test}); !ok {
			return false
		}
	}
	if s.skip != nil {
		if ok, partial := s.skip.matches([]string{test}); ok && !partial {
			return false
		}
	}
	return true
}

type job struct {
	id  string
	sel selector
}

type workflowFile struct {
	Env  map[string]string `yaml:"env"`
	Jobs map[string]struct {
		Strategy struct {
			Matrix struct {
				Include []map[string]any `yaml:"include"`
			} `yaml:"matrix"`
		} `yaml:"strategy"`
		Steps []struct {
			Run string `yaml:"run"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

var (
	matrixJobs    = []string{"test", "dashboard", "provider-migration"}
	catchAllJobs  = []string{"test-unsharded", "dashboard-unsharded"}
	varRef        = regexp.MustCompile(`\$([A-Z][A-Z0-9_]*)`)
	skipInit      = regexp.MustCompile(`(?m)^\s*skip_pattern="([^"]*)"`)
	runVar        = regexp.MustCompile(`-run "\$([A-Z][A-Z0-9_]*)"`)
	shardListBody = regexp.MustCompile(`(?s)shard_patterns=\((.*?)\)`)
)

func loadJobs(path string) ([]job, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var wf workflowFile
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var jobs []job
	for _, name := range matrixJobs {
		j, ok := wf.Jobs[name]
		if !ok {
			return nil, fmt.Errorf("%s: job %q not found", path, name)
		}
		for _, inc := range j.Strategy.Matrix.Include {
			shard, pattern, skip := str(inc["shard"]), str(inc["pattern"]), str(inc["skip"])
			sel, err := newSelector(pattern, skip)
			if err != nil {
				return nil, fmt.Errorf("job %s shard %s: %w", name, shard, err)
			}
			jobs = append(jobs, job{id: name + "/" + shard, sel: sel})
		}
	}
	for _, name := range catchAllJobs {
		j, ok := wf.Jobs[name]
		if !ok {
			return nil, fmt.Errorf("%s: job %q not found", path, name)
		}
		var script string
		for _, s := range j.Steps {
			if strings.Contains(s.Run, "skip_pattern=") {
				script = s.Run
			}
		}
		pattern, skip, err := catchAllPatterns(script, wf.Env)
		if err != nil {
			return nil, fmt.Errorf("job %s: %w", name, err)
		}
		sel, err := newSelector(pattern, skip)
		if err != nil {
			return nil, fmt.Errorf("job %s: %w", name, err)
		}
		jobs = append(jobs, job{id: name, sel: sel})
	}
	return jobs, nil
}

func newSelector(run, skip string) (selector, error) {
	var s selector
	var err error
	if run != "" {
		if s.run, err = newFilter(run); err != nil {
			return s, err
		}
	}
	if skip != "" {
		if s.skip, err = newFilter(skip); err != nil {
			return s, err
		}
	}
	return s, nil
}

// catchAllPatterns reads the -run and -skip patterns of a catch-all job from its shell
// script. The script builds the skip pattern from the patterns of every other shard.
func catchAllPatterns(script string, env map[string]string) (run, skip string, err error) {
	m := skipInit.FindStringSubmatch(script)
	if m == nil {
		return "", "", fmt.Errorf("no skip_pattern=\"...\" line in the script")
	}
	if skip, err = expandVars(m[1], env); err != nil {
		return "", "", err
	}
	wrap := strings.Contains(script, `skip_pattern+="|($pattern)"`)
	if body := shardListBody.FindStringSubmatch(script); body != nil {
		for _, ref := range varRef.FindAllStringSubmatch(body[1], -1) {
			p, err := expandVars(ref[0], env)
			if err != nil {
				return "", "", err
			}
			if wrap {
				p = "(" + p + ")"
			}
			skip += "|" + p
		}
	}
	if r := runVar.FindStringSubmatch(script); r != nil {
		if run, err = expandVars("$"+r[1], env); err != nil {
			return "", "", err
		}
	}
	return run, skip, nil
}

func expandVars(s string, env map[string]string) (string, error) {
	var missing string
	out := varRef.ReplaceAllStringFunc(s, func(ref string) string {
		v, ok := env[ref[1:]]
		if !ok {
			missing = ref
		}
		return v
	})
	if missing != "" {
		return "", fmt.Errorf("unknown variable %s", missing)
	}
	return out, nil
}

func str(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// jobsFor lists the jobs that start the top-level test.
func jobsFor(jobs []job, test string) []string {
	var out []string
	for _, j := range jobs {
		if j.sel.selects(test) {
			out = append(out, j.id)
		}
	}
	return out
}

// jobOfRanFile maps the name of a ran-tests file to a job id.
func jobOfRanFile(base string) (string, bool) {
	base = strings.TrimSuffix(base, ".txt")
	switch {
	case base == "non-dashboard-unsharded":
		return "test-unsharded", true
	case base == "dashboard-unsharded":
		return "dashboard-unsharded", true
	case strings.HasPrefix(base, "non-dashboard-"):
		return "test/" + strings.TrimPrefix(base, "non-dashboard-"), true
	case strings.HasPrefix(base, "provider-migration-"):
		return "provider-migration/" + strings.TrimPrefix(base, "provider-migration-"), true
	case strings.HasPrefix(base, "dashboard-"):
		return "dashboard/" + strings.TrimPrefix(base, "dashboard-"), true
	}
	return "", false
}
