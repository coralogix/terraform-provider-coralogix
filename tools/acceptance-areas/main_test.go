package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestFilterMatchesLikeGoTest(t *testing.T) {
	cases := []struct {
		pattern     string
		name        []string
		ok, partial bool
	}{
		{"^TestA$", []string{"TestA"}, true, false},
		{"^TestA$", []string{"TestAB"}, false, false},
		{"^TestA$/^sub", []string{"TestA"}, true, true},
		{"^TestA$/^sub", []string{"TestA", "other"}, false, false},
		{"^TestA$/^sub", []string{"TestA", "subcase"}, true, false},
		// A slash inside parentheses does not split the levels.
		{"^TestA(x/y)$", []string{"TestAx/y"}, true, false},
		// A top-level bar starts a new alternative.
		{"^TestA$|^TestB$", []string{"TestB"}, true, false},
		// The first alternative that matches decides `partial`.
		{"^TestA$/^sub|^TestA$", []string{"TestA"}, true, true},
	}
	for _, c := range cases {
		f, err := newFilter(c.pattern)
		if err != nil {
			t.Fatal(err)
		}
		ok, partial := f.matches(c.name)
		if ok != c.ok || partial != c.partial {
			t.Errorf("%q on %v = (%t, %t), want (%t, %t)", c.pattern, c.name, ok, partial, c.ok, c.partial)
		}
	}
}

func TestSelector(t *testing.T) {
	cases := []struct {
		run, skip, test string
		want            bool
	}{
		{"", "", "TestA", true},
		{"^TestA", "", "TestB", false},
		{"^TestA", "^TestAB$", "TestAB", false},
		// A skip of a subtest leaves the parent test running.
		{"", "^TestA$/^sub", "TestA", true},
		// The run pattern of a shard that takes some subtests selects the parent test.
		{"^TestA$/^sub", "", "TestA", true},
	}
	for _, c := range cases {
		sel, err := newSelector(c.run, c.skip)
		if err != nil {
			t.Fatal(err)
		}
		if got := sel.selects(c.test); got != c.want {
			t.Errorf("run=%q skip=%q selects(%s) = %t, want %t", c.run, c.skip, c.test, got, c.want)
		}
	}
}

func TestCatchAllPatterns(t *testing.T) {
	script := `
          shard_patterns=(
            "$SHARD_ONE"
            "$SHARD_TWO"
          )
          skip_pattern="$MIGRATION|$QUOTA"
          for pattern in "${shard_patterns[@]}"; do
            skip_pattern+="|($pattern)"
          done
          go test -run "$DASH" -skip "$skip_pattern"`
	env := map[string]string{"SHARD_ONE": "^A$", "SHARD_TWO": "^B$", "MIGRATION": "^M", "QUOTA": "^Q", "DASH": "^Dash"}
	run, skip, err := catchAllPatterns(script, env)
	if err != nil {
		t.Fatal(err)
	}
	if run != "^Dash" || skip != "^M|^Q|(^A$)|(^B$)" {
		t.Errorf("got run=%q skip=%q", run, skip)
	}
	if _, _, err := catchAllPatterns(`skip_pattern="$MISSING"`, env); err == nil {
		t.Error("an unknown variable must fail")
	}
}

func testConfig(t *testing.T) *config {
	t.Helper()
	c, err := buildConfig(rawConfig{
		Ignore: []string{`\.md$`},
		Strict: []string{"internal/provider/"},
		Shared: []string{"internal/provider/provider.go"},
		Areas: map[string]rawArea{
			"alerts":     {Paths: []string{"internal/provider/alerts/"}, Resources: []string{"alert"}, DependsOn: []string{"dashboards"}},
			"dashboards": {Paths: []string{"internal/provider/dashboards/"}},
			"slo":        {Resources: []string{"slo"}},
			"slo_v2":     {Resources: []string{"slo_v2"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestClassify(t *testing.T) {
	c := testConfig(t)
	cases := []struct {
		path  string
		owner string
		kind  kind
	}{
		{"README.md", "", kindIgnored},
		{"internal/provider/alerts/x.go", "alerts", kindArea},
		{"internal/provider/resource_coralogix_alert_test.go", "alerts", kindArea},
		{"examples/resources/coralogix_alert/resource.tf", "alerts", kindArea},
		{"internal/provider/provider.go", "", kindShared},
		{"internal/utils/x.go", "", kindImplicit},
		// The longest prefix wins: slo_v2 is not part of slo.
		{"internal/provider/resource_coralogix_slo_v2_test.go", "slo_v2", kindArea},
		{"internal/provider/resource_coralogix_slo_test.go", "slo", kindArea},
	}
	for _, tc := range cases {
		owner, k, err := c.classify(tc.path)
		if err != nil || owner != tc.owner || k != tc.kind {
			t.Errorf("classify(%s) = (%q, %v, %v), want (%q, %v)", tc.path, owner, k, err, tc.owner, tc.kind)
		}
	}
}

func TestDependents(t *testing.T) {
	c := testConfig(t)
	got := c.dependents(map[string]bool{"dashboards": true})
	if !got["alerts"] || !got["dashboards"] || got["slo"] {
		t.Errorf("dependents of dashboards = %v", got)
	}
}

func testModel(t *testing.T) *model {
	t.Helper()
	sel := func(run string) selector {
		s, err := newSelector(run, "")
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	return &model{
		cfg: testConfig(t),
		gofiles: []*goFile{
			{path: "internal/provider/resource_coralogix_alert_test.go", tests: []string{"TestAccAlert"}},
			{path: "internal/provider/dashboards/d_test.go", tests: []string{"TestAccDashboard"}},
			{path: "internal/provider/resource_coralogix_slo_test.go", tests: []string{"TestAccSlo"}},
			{path: "internal/utils/utils_test.go", tests: []string{"TestUtils"}},
		},
		jobs: []job{
			{id: "test/alerts", sel: sel("^TestAccAlert")},
			{id: "dashboard/core", sel: sel("^TestAccDashboard")},
			{id: "test/slo", sel: sel("^TestAccSlo")},
			{id: "test-unsharded", sel: sel("^TestUtils")},
		},
	}
}

func TestDetect(t *testing.T) {
	m := testModel(t)
	cases := []struct {
		name    string
		changed []string
		code    bool
		all     bool
		jobs    []string
	}{
		{"docs only", []string{"CHANGELOG.md"}, false, false, nil},
		{"one area", []string{"internal/provider/resource_coralogix_slo_test.go", "CHANGELOG.md"},
			true, false, []string{"test-unsharded", "test/slo"}},
		// alerts uses dashboards, so a dashboard change also runs the alert jobs.
		{"area with a dependent", []string{"internal/provider/dashboards/x.go"},
			true, false, []string{"dashboard/core", "test-unsharded", "test/alerts"}},
		{"a file in no area", []string{"internal/provider/slo_x.go", "internal/utils/x.go"}, true, true, nil},
		{"workflow", []string{".github/workflows/acceptance-tests.yml"}, true, true, nil},
		{"shared file", []string{"internal/provider/provider.go"}, true, true, nil},
	}
	for _, c := range cases {
		code, all, jobs, _ := m.detect(c.changed)
		if code != c.code || all != c.all || !reflect.DeepEqual(jobs, c.jobs) {
			t.Errorf("%s: got (%t, %t, %v), want (%t, %t, %v)", c.name, code, all, jobs, c.code, c.all, c.jobs)
		}
	}
}

func TestLintFindsUndeclaredUse(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The slo file uses a helper from the alert file. Another file imports a dashboards package.
	write("internal/provider/resource_coralogix_alert_test.go", "package provider\nfunc helper() int { return 1 }\n")
	write("internal/provider/resource_coralogix_slo_test.go", "package provider\nfunc TestSlo() { _ = helper() }\n")
	write("internal/provider/alerts/a.go", "package alerts\nimport _ \"example.com/m/internal/provider/dashboards/schema\"\n")
	write("internal/provider/dashboards/schema/s.go", "package schema\n")
	t.Chdir(dir)

	files := []string{
		"internal/provider/resource_coralogix_alert_test.go", "internal/provider/resource_coralogix_slo_test.go",
		"internal/provider/alerts/a.go", "internal/provider/dashboards/schema/s.go",
	}
	gofiles, err := scanGo(files, "example.com/m")
	if err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(t)
	cfg.areas["alerts"] = rawArea{DependsOn: nil} // drop the declared dependency
	m := &model{cfg: cfg, files: files, gofiles: gofiles}
	joined := strings.Join(m.lintEdges(), "\n")
	for _, want := range []string{
		"area slo uses code of area alerts",
		"area alerts uses code of area dashboards",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("lint output lacks %q:\n%s", want, joined)
		}
	}
}

func TestJobOfRanFile(t *testing.T) {
	cases := map[string]string{
		"non-dashboard-unsharded.txt":      "test-unsharded",
		"non-dashboard-access-control.txt": "test/access-control",
		"dashboard-unsharded.txt":          "dashboard-unsharded",
		"dashboard-core.txt":               "dashboard/core",
		"provider-migration-team.txt":      "provider-migration/team",
	}
	for file, want := range cases {
		if got, ok := jobOfRanFile(file); !ok || got != want {
			t.Errorf("jobOfRanFile(%s) = %q, %t, want %q", file, got, ok, want)
		}
	}
}
