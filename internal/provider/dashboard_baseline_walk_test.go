// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"

	dashboardschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// The schema walk applies a base config for each widget kind and query arm, then changes one thing
// at a time: it removes an optional attribute, empties a list, or sets another enum value. Each
// path is changed once, in the first variant that holds it. A walk golden saves the base request
// and state in full, and every later step saves only what changed against its base.

const (
	walkLayoutKey  = "layout.sections[].rows[].widgets[]"
	walkDefinition = walkLayoutKey + ".definition"
)

var walkWidgetKinds = []string{"line_chart", "data_table", "gauge", "pie_chart", "bar_chart", "horizontal_bar_chart", "hexagon"}

// walkBase is one base config: a widget kind with a query arm, a dynamic visualization, or the top level.
type walkBase struct {
	name string
	pin  map[string]string
	// top makes the config hold the top-level attributes next to a small layout.
	top bool
}

func (b walkBase) config(variant int, visits map[string]int) (j, *dashboardGen) {
	g := newDashboardGen(variant, b.pin, visits)
	if b.top {
		g.elements = 6
	}
	attrs := dashboardV4Attrs()
	cfg := j{}
	names := []string{"layout"}
	if b.top {
		names = []string{"name", "description", "variables", "variables_v2", "filters", "time_frame", "folder", "annotations", "auto_refresh", "access_policy"}
	}
	for _, name := range names {
		if v, ok := g.attribute(attrs[name], name, name); ok {
			cfg[name] = v
			g.reached[name] = true
		}
	}
	if b.top {
		cfg["layout"] = dashboardLayout(markdownWidget("note", "hello"))
	} else {
		cfg["name"] = "walk " + b.name
	}
	return cfg, g
}

// walkBases lists the base configs. A probe generation finds the one-of groups that pick the query
// arm and the dynamic visualization.
func walkBases() []walkBase {
	var bases []walkBase
	widgetPin := func(kind string) map[string]string {
		return map[string]string{walkLayoutKey: "definition", walkDefinition: kind}
	}
	probe := func(pin map[string]string) *dashboardGen {
		g := newDashboardGen(0, pin, nil)
		g.attribute(dashboardV4Attrs()["layout"], "layout", "layout")
		return g
	}
	for _, kind := range walkWidgetKinds {
		for _, group := range groupPathsWith(probe(widgetPin(kind)), kind, "logs", "metrics", "spans")[:1] {
			for _, arm := range group.members {
				pin := widgetPin(kind)
				pin[group.path] = arm
				bases = append(bases, walkBase{name: kind + "_" + arm, pin: pin})
			}
		}
	}
	bases = append(bases, walkBase{name: "markdown", pin: widgetPin("markdown")})
	dynamicPin := widgetPin("dynamic")
	for _, group := range groupPathsWith(probe(dynamicPin), "dynamic", "stat", "table", "gauge")[:1] {
		for _, arm := range group.members {
			pin := widgetPin("dynamic")
			pin[group.path] = arm
			bases = append(bases, walkBase{name: "dynamic_" + arm, pin: pin})
		}
	}
	reference := map[string]string{walkLayoutKey: "reference"}
	bases = append(bases, walkBase{name: "reference", pin: reference})
	bases = append(bases, walkBase{name: "top_level", top: true, pin: map[string]string{
		"variables[].definition": "multi_select",
	}})
	return bases
}

type walkGroup struct {
	path    string
	members []string
}

// groupPathsWith returns the one-of groups under a path part that have all the members.
func groupPathsWith(g *dashboardGen, part string, members ...string) []walkGroup {
	var out []walkGroup
	for key, got := range g.groups {
		path, _, _ := strings.Cut(key, ".{")
		if !strings.Contains(path, "."+part) || !strings.Contains(key, "{") {
			continue
		}
		if !slices.ContainsFunc(members, func(m string) bool { return slices.Contains(got, m) }) {
			continue
		}
		out = append(out, walkGroup{path: path, members: got})
	}
	sort.Slice(out, func(a, b int) bool {
		if len(out[a].members) != len(out[b].members) {
			return len(out[a].members) > len(out[b].members)
		}
		return out[a].path < out[b].path
	})
	return out
}

// walkVariants builds variants until every one-of group that the base does not pin has had every member.
func (b walkBase) variants() []walkVariant {
	var out []walkVariant
	seen := map[string]map[string]bool{}
	visits := map[string]int{}
	reached, quiet := map[string]bool{}, 0
	for variant := 0; variant < 96; variant++ {
		cfg, g := b.config(variant, visits)
		grew := false
		for path := range g.reached {
			if !reached[path] {
				reached[path], grew = true, true
			}
		}
		if grew {
			quiet = 0
		} else {
			quiet++
		}
		out = append(out, walkVariant{cfg: cfg, gen: g})
		for key, arms := range g.armsSeen {
			if seen[key] == nil {
				seen[key] = map[string]bool{}
			}
			for arm := range arms {
				seen[key][arm] = true
			}
		}
		complete := true
		for key, members := range g.groups {
			path, _, _ := strings.Cut(key, ".{")
			if _, pinned := b.pin[path]; pinned {
				continue
			}
			for _, m := range members {
				if !seen[key][m] {
					complete = false
				}
			}
		}
		// A nested group shows only when its parent takes the arm that holds it, so the
		// variants go on until a few of them reach nothing new.
		if complete && quiet >= 12 {
			out = out[:len(out)-quiet]
			break
		}
	}
	return out
}

type walkVariant struct {
	cfg j
	gen *dashboardGen
}

// walkMutation is one change to one path of a config.
type walkMutation struct {
	path  string
	label string
	apply func(parent map[string]any, name string)
}

// walkBatch is a group of changes of one kind that are applied together.
type walkBatch struct {
	kind      string
	mutations []walkMutation
}

var walkIndex = func() map[string]schema.Attribute {
	index := map[string]schema.Attribute{}
	var walk func(attrs map[string]schema.Attribute, prefix string)
	walk = func(attrs map[string]schema.Attribute, prefix string) {
		for name, a := range attrs {
			path := joinPath(prefix, name)
			index[path] = a
			switch x := a.(type) {
			case schema.SingleNestedAttribute:
				walk(x.Attributes, path)
			case schema.ListNestedAttribute:
				walk(x.NestedObject.Attributes, path+"[]")
			case schema.SetNestedAttribute:
				walk(x.NestedObject.Attributes, path+"[]")
			}
		}
	}
	walk(dashboardschema.V4().Attributes, "")
	return index
}()

func deepCopyConfig(cfg j) j {
	raw, err := json.Marshal(cfg)
	if err != nil {
		panic(err)
	}
	var out j
	if err := json.Unmarshal(raw, &out); err != nil {
		panic(err)
	}
	return out
}

// numbersToFloat keeps the numbers of a copied config the type that the harness reads.
func parentOf(cfg any, path string) (map[string]any, string) {
	segments := strings.Split(path, ".")
	node := cfg
	for _, segment := range segments[:len(segments)-1] {
		name := strings.TrimSuffix(segment, "[]")
		m, ok := node.(map[string]any)
		if !ok {
			return nil, ""
		}
		node = m[name]
		if strings.HasSuffix(segment, "[]") {
			list, ok := node.([]any)
			if !ok || len(list) == 0 {
				return nil, ""
			}
			node = list[0]
		}
	}
	m, ok := node.(map[string]any)
	if !ok {
		return nil, ""
	}
	return m, strings.TrimSuffix(segments[len(segments)-1], "[]")
}

// walkBatches lists the changes that a variant adds, as one batch for each kind. A path that an
// earlier variant changed is left out. When a batch fails, runBatch splits it, so a failure names
// the change that caused it.
func walkBatches(v walkVariant, done map[string]bool) []walkBatch {
	var paths []string
	for path := range v.gen.reached {
		if !done[path] {
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	arms := map[string]bool{}
	for key, members := range v.gen.groups {
		for _, m := range members {
			arms[strings.SplitN(key, ".{", 2)[0]+"."+m] = true
		}
	}
	// A change inside a removed or emptied object has no effect. So a batch never holds a path next to
	// its ancestor or descendant. Such a change goes to a later round of the same kind.
	batches := map[string]*walkBatch{}
	add := func(kind string, m walkMutation) {
		for round := 1; ; round++ {
			key := walkBatchKey(kind, round)
			if batches[key] == nil {
				batches[key] = &walkBatch{kind: key}
			}
			if !slices.ContainsFunc(batches[key].mutations, func(other walkMutation) bool { return pathsNest(other.path, m.path) }) {
				batches[key].mutations = append(batches[key].mutations, m)
				return
			}
		}
	}
	for _, path := range paths {
		a := walkIndex[path]
		parent, name := parentOf(v.cfg, path)
		if a == nil || parent == nil {
			continue
		}
		// Only the first item of a list is changed, so a path that this item lacks waits for a later variant.
		used, present := parent[name]
		if !present {
			continue
		}
		done[path] = true
		for _, m := range walkMutationsFor(path, a, parent, name, used, arms[path]) {
			add(m.kind, m.walkMutation)
		}
	}
	var out []walkBatch
	for _, kind := range []string{"remove optional values", "remove optional objects", "empty optional lists", "set enums to unspecified", "set enums to their last value"} {
		for round := 1; batches[walkBatchKey(kind, round)] != nil; round++ {
			out = append(out, *batches[walkBatchKey(kind, round)])
		}
	}
	return out
}

func walkBatchKey(kind string, round int) string {
	if round == 1 {
		return kind
	}
	return fmt.Sprintf("%s, round %d", kind, round)
}

// pathsNest reports whether one path is the other, or lies inside it.
func pathsNest(a, b string) bool {
	inside := func(outer, inner string) bool {
		return strings.HasPrefix(inner, outer+".") || strings.HasPrefix(inner, outer+"[].")
	}
	return a == b || inside(a, b) || inside(b, a)
}

func isNestedAttribute(a schema.Attribute) bool {
	switch a.(type) {
	case schema.SingleNestedAttribute, schema.ListNestedAttribute, schema.SetNestedAttribute:
		return true
	}
	return false
}

func isListAttribute(a schema.Attribute) bool {
	switch a.(type) {
	case schema.ListAttribute, schema.ListNestedAttribute:
		return true
	}
	return false
}

// otherEnumValues returns "unspecified" and the last value when the attribute is an enum and the config uses neither.
func otherEnumValues(a schema.Attribute, used any) []string {
	s, ok := a.(schema.StringAttribute)
	if !ok {
		return nil
	}
	for _, v := range s.Validators {
		m := oneOfPattern.FindStringSubmatch(v.Description(context.Background()))
		if m == nil {
			continue
		}
		var values []string
		for _, q := range quotedPattern.FindAllStringSubmatch(m[1], -1) {
			values = append(values, q[1])
		}
		var out []string
		for _, want := range []string{"unspecified", values[len(values)-1]} {
			if want != used && slices.Contains(values, want) && !slices.Contains(out, want) {
				out = append(out, want)
			}
		}
		return out
	}
	return nil
}

// --- the diff format ---

type walkStep struct {
	Step         string   `json:"step"`
	Changes      []string `json:"changes,omitempty"`
	Diagnostics  []string `json:"diagnostics,omitempty"`
	RequestDiff  []string `json:"requestDiff,omitempty"`
	StateDiff    []string `json:"stateDiff,omitempty"`
	Inconsistent []string `json:"inconsistentResult,omitempty"`
	ReplanEmpty  *bool    `json:"replanEmpty,omitempty"`
	ReplanDiff   []string `json:"replanDiff,omitempty"`
}

type walkFull struct {
	Step    string `json:"step"`
	Request any    `json:"request"`
	State   any    `json:"state"`
}

func normalizeJSON(v any) any {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		panic(err)
	}
	return out
}

func jsonDiff(a, b any, path string, out *[]string) {
	switch x := a.(type) {
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok {
			break
		}
		keys := map[string]bool{}
		for k := range x {
			keys[k] = true
		}
		for k := range y {
			keys[k] = true
		}
		sorted := make([]string, 0, len(keys))
		for k := range keys {
			sorted = append(sorted, k)
		}
		sort.Strings(sorted)
		for _, k := range sorted {
			av, aok := x[k]
			bv, bok := y[k]
			switch {
			case !aok:
				*out = append(*out, "+ "+path+"."+k+": "+short(bv))
			case !bok:
				*out = append(*out, "- "+path+"."+k)
			default:
				jsonDiff(av, bv, path+"."+k, out)
			}
		}
		return
	case []any:
		y, ok := b.([]any)
		if !ok {
			break
		}
		for i := 0; i < max(len(x), len(y)); i++ {
			switch {
			case i >= len(x):
				*out = append(*out, fmt.Sprintf("+ %s[%d]: %s", path, i, short(y[i])))
			case i >= len(y):
				*out = append(*out, fmt.Sprintf("- %s[%d]", path, i))
			default:
				jsonDiff(x[i], y[i], fmt.Sprintf("%s[%d]", path, i), out)
			}
		}
		return
	}
	if fmt.Sprint(a) != fmt.Sprint(b) {
		*out = append(*out, "~ "+path+": "+short(a)+" → "+short(b))
	}
}

func short(v any) string {
	raw, _ := json.Marshal(v)
	if len(raw) > 160 {
		// The digest covers the whole value, so a change after the cut still changes the golden.
		// Random ids differ on every run, so each one is replaced by its order of appearance first.
		seen := map[string]int{}
		stable := dashboardUUIDPattern.ReplaceAllFunc(raw, func(id []byte) []byte {
			if _, ok := seen[string(id)]; !ok {
				seen[string(id)] = len(seen) + 1
			}
			return []byte(fmt.Sprintf("<id-%d>", seen[string(id)]))
		})
		sum := sha256.Sum256(stable)
		return fmt.Sprintf("%s…[sha256:%x]", raw[:160], sum[:4])
	}
	return string(raw)
}

func walkRequestBody(steps []grRequest) any {
	for _, r := range steps {
		if r.Method == "POST" || r.Method == "PUT" {
			if raw, ok := r.Body.(json.RawMessage); ok {
				var body any
				if err := json.Unmarshal(raw, &body); err != nil {
					panic(err)
				}
				return body
			}
		}
	}
	return nil
}

func walkDiags(s grStep) []string {
	var out []string
	for _, d := range [][]string{s.ValidateDiags, s.PlanDiags, s.ApplyDiags} {
		for _, line := range d {
			out = append(out, line[:min(len(line), 400)])
		}
	}
	// The framework runs validators in map order.
	sort.Strings(out)
	return out
}

// walkBaseline is the request and state that later steps are compared with.
type walkBaseline struct {
	request any
	state   any
}

func (h *grHarness) walkRecord(baseline walkBaseline, name string) walkStep {
	s := h.steps[len(h.steps)-1]
	step := walkStep{Step: name, Diagnostics: walkDiags(s), Inconsistent: s.Inconsistent, ReplanEmpty: s.ReplanEmpty, ReplanDiff: s.ReplanDiff}
	if body := walkRequestBody(s.Requests); body != nil {
		jsonDiff(baseline.request, normalizeJSON(body), "", &step.RequestDiff)
	}
	if s.State != nil {
		jsonDiff(baseline.state, normalizeJSON(s.State), "", &step.StateDiff)
	}
	return step
}

func (h *grHarness) walkFull(name string) walkFull {
	s := h.steps[len(h.steps)-1]
	return walkFull{Step: name, Request: normalizeJSON(walkRequestBody(s.Requests)), State: normalizeJSON(s.State)}
}

// runWalk applies one base and its changes, and returns the golden content.
func runWalk(h *grHarness, b walkBase) []any {
	var out []any
	variants := b.variants()
	cfg0 := variants[0].cfg
	renamed := func(cfg j) j { return dashboardWith(cfg, j{"name": fmt.Sprint(cfg["name"]) + " updated"}) }

	h.dashboards().addFolder(walkFolderID, "walk folder")
	state := h.Apply("create variant 0", h.null(), cfg0)
	if state.IsNull() {
		return append(out, walkStep{Step: "create variant 0", Diagnostics: walkDiags(h.steps[len(h.steps)-1])})
	}
	out = append(out, h.walkFull("create variant 0"))
	baseState := h.Apply("update variant 0 with another name", state, renamed(cfg0))
	out = append(out, h.walkFull("update variant 0 with another name"))
	baseline0 := walkBaseline{request: out[len(out)-1].(walkFull).Request, state: out[len(out)-1].(walkFull).State}
	baseState = h.RefreshPlan("refresh and plan variant 0", baseState)
	out = append(out, h.walkRecord(baseline0, "refresh and plan variant 0"))

	done := map[string]bool{}
	for k, v := range variants {
		prior, baseline := baseState, baseline0
		if k > 0 {
			name := fmt.Sprintf("apply variant %d", k)
			prior = h.Apply(name, baseState, renamed(v.cfg))
			out = append(out, h.walkRecord(baseline0, name))
			if prior.IsNull() || h.steps[len(h.steps)-1].State == nil {
				continue
			}
			baseline = walkBaseline{request: normalizeJSON(walkRequestBody(h.steps[len(h.steps)-1].Requests)), state: normalizeJSON(h.steps[len(h.steps)-1].State)}
		}
		for _, batch := range walkBatches(v, done) {
			out = h.runBatch(out, v.cfg, prior, baseline, renamed, batch.kind, batch.mutations)
		}
	}
	h.Destroy("destroy", baseState)
	return out
}

// runBatch applies the mutations together. When the apply fails, it splits the batch in two, until
// a failure names one change.
func (h *grHarness) runBatch(out []any, base j, prior tftypes.Value, baseline walkBaseline, renamed func(j) j, kind string, mutations []walkMutation) []any {
	cfg := deepCopyConfig(base)
	labels := make([]string, len(mutations))
	for i, m := range mutations {
		parent, name := parentOf(cfg, m.path)
		if parent != nil {
			m.apply(parent, name)
		}
		labels[i] = m.label
	}
	name := fmt.Sprintf("%s (%d)", kind, len(mutations))
	if len(mutations) == 1 {
		name = mutations[0].label
	}
	h.Apply(name, prior, renamed(cfg))
	step := h.walkRecord(baseline, name)
	failed := slices.ContainsFunc(step.Diagnostics, func(d string) bool { return strings.HasPrefix(d, "error") })
	if failed && len(mutations) > 1 {
		step.Changes = labels
		out = append(out, step)
		half := len(mutations) / 2
		out = h.runBatch(out, base, prior, baseline, renamed, kind, mutations[:half])
		return h.runBatch(out, base, prior, baseline, renamed, kind, mutations[half:])
	}
	if len(mutations) > 1 {
		step.Changes = labels
	}
	return append(out, step)
}

// The walk takes about ten seconds for each base, because the framework plans the large schema in
// about a second. A plain run walks only the small bases. DASHBOARD_WALK=all walks every base.
//
//	DASHBOARD_WALK=all UPDATE_GOLDEN=1 go test ./internal/provider -run TestDashboardBaselineWalk
var walkSmokeBases = []string{"markdown", "reference"}

// The bases run in parallel. Each has its own fake and API key (see newParallelDashboardHarness).
func TestDashboardBaselineWalk(t *testing.T) {
	t.Setenv("CORALOGIX_API_KEY", "baseline-test-key")
	t.Setenv("CORALOGIX_DOMAIN", "eu2.coralogix.com")
	t.Setenv("CORALOGIX_ENV", "")
	t.Setenv("CORALOGIX_DASHBOARD_VALIDATION", "false")
	router := installDashboardRouter(t)
	for _, b := range walkBases() {
		t.Run(b.name, func(t *testing.T) {
			if os.Getenv("DASHBOARD_WALK") != "all" && !slices.Contains(walkSmokeBases, b.name) {
				t.Skip("set DASHBOARD_WALK=all to walk every base")
			}
			t.Parallel()
			h := newParallelDashboardHarness(t, router, "walk-"+b.name)
			steps := runWalk(h, b)
			checkGoldenAt(t, dashboardGoldenDir, "walk_"+b.name+".json", marshalDashboardGolden(t, steps))
		})
	}
}

var _ = tftypes.NewValue

// walkUncovered are the schema paths that no base config reaches, each with the reason.
var walkUncovered = map[string]string{
	"layout.sections[].rows[].widgets[].definition.dynamic.visualization.stat_card.primary_value.mapped_values": "the config validator rejects it on the primary value",
	"content_json":                          "not a widget attribute; the content-json-* scenarios cover it",
	"variables[].definition.constant_value": "the API rejects it; the variables-legacy-constant-value scenario records the failure",
}

func walkIsUncovered(path string) bool {
	for prefix := range walkUncovered {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// TestDashboardWalkReachesEverySchemaPath is decision 1: a path that no base config reaches fails
// with its name. Then extend the generator hints, or list the path in walkUncovered with a reason.
func TestDashboardWalkReachesEverySchemaPath(t *testing.T) {
	reached := map[string]bool{}
	for _, b := range walkBases() {
		for _, v := range b.variants() {
			for path := range v.gen.reached {
				reached[path] = true
			}
		}
	}
	var missing []string
	for _, path := range dashboardSchemaPaths() {
		if !reached[path] && !walkIsUncovered(path) {
			missing = append(missing, path)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("%d schema paths are not reached by any base config:\n%s", len(missing), strings.Join(missing, "\n"))
	}
}

// A change inside a removed or emptied object has no effect, and the walk would still list it as
// tested. So no batch may hold a path together with its ancestor or descendant.
func TestDashboardWalkBatchesNeverNestPaths(t *testing.T) {
	batches, nested := 0, 0
	for _, b := range walkBases() {
		done := map[string]bool{}
		for _, v := range b.variants() {
			for _, batch := range walkBatches(v, done) {
				batches++
				for i, m := range batch.mutations {
					for _, other := range batch.mutations[:i] {
						if m.path != other.path && pathsNest(m.path, other.path) {
							nested++
							t.Errorf("%s: batch %q holds %s and %s", b.name, batch.kind, other.path, m.path)
						}
					}
				}
			}
		}
	}
	if batches == 0 {
		t.Fatal("no batches were built")
	}
	t.Logf("%d batches, %d nested pairs", batches, nested)
}

// Every variant of every base has to pass the config validators, or the walk records the failure
// of the base and nothing else.
func TestDashboardWalkBasesAreValid(t *testing.T) {
	if os.Getenv("DASHBOARD_WALK") != "all" {
		t.Skip("set DASHBOARD_WALK=all to validate every base")
	}
	h := newDashboardHarness(t)
	for _, b := range walkBases() {
		for k, v := range b.variants() {
			var errs []string
			for _, d := range diagStrings(h.validate(h.config(v.cfg))) {
				if strings.HasPrefix(d, "error") {
					errs = append(errs, d[:min(len(d), 300)])
				}
			}
			if len(errs) > 0 {
				sort.Strings(errs)
				t.Errorf("%s variant %d: %s", b.name, k, strings.Join(errs[:min(len(errs), 3)], "\n   "))
			}
		}
	}
}

// pairedEnum is an enum that a custom validator checks together with its siblings, such as the
// operator type with its selection type. Changing it alone only makes a combination that the
// validator rejects. The variants already take every value of such an enum.
func pairedEnum(parent map[string]any, name string) bool {
	switch name {
	case "selection_type", "aggregation_type":
		return true
	case "type":
		for _, sibling := range []string{"selection_type", "percent", "field", "observation_field", "aggregation_type", "selected_values"} {
			if _, ok := parent[sibling]; ok {
				return true
			}
		}
	}
	return false
}

type kindMutation struct {
	kind string
	walkMutation
}

// walkMutationsFor lists the changes of one path: remove it, empty it, or set another enum value.
func walkMutationsFor(path string, a schema.Attribute, parent map[string]any, name string, used any, isArm bool) []kindMutation {
	var out []kindMutation
	if a.IsOptional() && !isArm && path != "layout" && !strings.HasSuffix(path, ".title") {
		kind := "remove optional values"
		if isNestedAttribute(a) {
			kind = "remove optional objects"
		}
		out = append(out, kindMutation{kind, walkMutation{path, "remove " + path, func(p map[string]any, n string) { delete(p, n) }}})
	}
	if a.IsOptional() && isListAttribute(a) {
		out = append(out, kindMutation{"empty optional lists", walkMutation{path, "empty " + path, func(p map[string]any, n string) { p[n] = []any{} }}})
	}
	if pairedEnum(parent, name) {
		return out
	}
	for _, value := range otherEnumValues(a, used) {
		kind := "set enums to their last value"
		if value == "unspecified" {
			kind = "set enums to unspecified"
		}
		out = append(out, kindMutation{kind, walkMutation{path, "set " + path + " = " + value, func(p map[string]any, n string) { p[n] = value }}})
	}
	return out
}
