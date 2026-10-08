package generator

import (
	"errors"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/acceptance"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/version"
)

// accCheck is one attribute check of an acceptance step.
type accCheck struct{ Path, Value string }

// Expr is the Go expression of the expected value. A value with a placeholder is rendered at run time.
func (c accCheck) Expr() string {
	if strings.Contains(c.Value, "@{") {
		return "render(run, " + strconv.Quote(c.Value) + ")"
	}
	return strconv.Quote(c.Value)
}

// accStep is one config of the acceptance test and what the test checks after the apply.
type accStep struct {
	Config string     // the resource block, HCL with @{run} and @{env.NAME}
	Checks []accCheck // attributes that must have the value
	Absent []string   // top-level attributes that must not be in the state
	// Kept are the checks of the step that removes attributes from the full config: an attribute
	// that keeps its state value still has the value of the full config.
	Kept []accCheck
}

// acceptanceData is the template data of the generated acceptance test.
type acceptanceData struct {
	Package, VersionHeader string
	ProviderImport         string // import path of the provider package
	Name                   string // resource name, for the names of the tests
	Address                string // address of the resource in the config
	IDAttr                 string
	IDCustom               bool // the id attribute is not "id": the import step names it
	Env                    []string
	Prerequisites          string
	Full, Updated, Minimal *accStep
	// UpgradeMinimal is the minimal config plus the fields that the released provider needs.
	UpgradeMinimal *accStep
	// UpgradeFull is the full config without the attributes that the released provider does not
	// have. nil when it is the full config.
	UpgradeFull *accStep
	UpgradeFrom string
	// UpgradeAttributes is the upgrade attributes file to write. nil without an upgrade test.
	UpgradeAttributes *acceptance.UpgradeAttributes
	// UpgradeFullAction and UpgradeMinimalAction are the plan checks of the step after the upgrade,
	// to the updated config and to the full config. Go expressions of type []plancheck.PlanCheck.
	UpgradeFullAction, UpgradeMinimalAction string
}

type accMode int

const (
	accFull accMode = iota
	accUpdated
	accMinimal
)

// accSynth makes the configs from the schema of the resource. A value comes from the acceptance
// file, or the synthesizer makes a plain one from the kind of the attribute.
type accSynth struct {
	file *acceptance.File
	// minimal are the optional fields that the minimal config sets.
	minimal []string
	idAttr  string
	known   map[string]bool // every attribute path that the walk saw
	// settable are the attribute paths that the user can set: Required or Optional.
	settable map[string]bool
	// required are the attribute paths that the walk saw as Required.
	required map[string]bool
	// released are the attributes that the released provider of the upgrade test has. When it is
	// not nil, the walk leaves out every other attribute, and sets every one in releasedRequired.
	released map[string]bool
	// releasedRequired are the attributes that the released provider requires.
	releasedRequired map[string]bool
	used             map[string]bool // every value path that a config used
}

var quoted = regexp.MustCompile(`"([^"]*)"`)

// buildAcceptance makes the data of the acceptance test of a resource.
// prior is the upgrade attributes file of the last run, or nil.
func buildAcceptance(res *tfResource, providerModule string, file *acceptance.File, prior *acceptance.UpgradeAttributes) (*acceptanceData, error) {
	if len(res.ConfigValidators) != 0 {
		return nil, errors.New("acceptance test: a oneOf group among the top-level fields is not supported")
	}
	if path := nestedImmutable("", res.Attributes, true); path != "" {
		return nil, fmt.Errorf("acceptance test: %q has RequiresReplace below the top level, which nextAction does not compare", path)
	}
	typeName := res.CRUD.TypeName
	s := &accSynth{file: file, minimal: file.Minimal, idAttr: res.CRUD.IDAttr, known: map[string]bool{}, settable: map[string]bool{}, required: map[string]bool{}, used: map[string]bool{}}
	out := &acceptanceData{
		Package:        res.Package,
		VersionHeader:  version.Header,
		ProviderImport: providerModule + "/internal/provider",
		Name:           res.CRUD.Resource,
		Address:        "coralogix_" + typeName + ".test",
		IDAttr:         res.CRUD.IDAttr,
		IDCustom:       res.CRUD.IDAttr != "id",
		Env:            file.Env,
		Prerequisites:  strings.TrimSpace(file.Prerequisites),
		UpgradeFrom:    file.UpgradeFrom,
	}
	for _, step := range []struct {
		mode accMode
		into **accStep
	}{{accFull, &out.Full}, {accUpdated, &out.Updated}, {accMinimal, &out.Minimal}} {
		body, checks, err := s.attrs(res.Attributes, "", "", step.mode, true)
		if err != nil {
			return nil, fmt.Errorf("acceptance test: %w", err)
		}
		*step.into = &accStep{Config: resourceBlock("coralogix_"+typeName, body), Checks: checks}
	}
	out.Minimal.Absent = s.absent(res.Attributes)
	defaults, kept := s.removed(res.Attributes, out.Full.Checks)
	out.Minimal.Checks = append(out.Minimal.Checks, defaults...)
	out.Minimal.Kept = kept
	if file.UpgradeFrom != "" {
		if err := s.upgradeSteps(out, res.Attributes, "coralogix_"+typeName, prior); err != nil {
			return nil, fmt.Errorf("acceptance test: %w", err)
		}
	}
	if err := s.checkFile(); err != nil {
		return nil, err
	}
	return out, nil
}

// upgradeSteps makes the configs of the upgrade test. The released provider gets only the
// attributes that it has, so the first config of each upgrade subtest leaves out the newer ones.
// The next config is applied with this build, so it keeps them.
func (s *accSynth) upgradeSteps(out *acceptanceData, attrs []*tfAttr, resourceType string, prior *acceptance.UpgradeAttributes) error {
	schema := map[string]*tfAttr{}
	schemaAttrs(schema, "", attrs)
	if prior == nil || prior.From != s.file.UpgradeFrom {
		// The list is new, or upgradeFrom changed: the schema of now is the schema of the release.
		prior = &acceptance.UpgradeAttributes{From: s.file.UpgradeFrom, Attributes: map[string]acceptance.UpgradeAttribute{}}
		for path, a := range schema {
			prior.Attributes[path] = upgradeAttribute(a)
		}
	}
	out.UpgradeAttributes = prior
	released, releasedRequired := releasedAttrs(prior, schema)
	if err := lostRequired(prior, released); err != nil {
		return err
	}
	if err := newRequired(schema, released, prior.From); err != nil {
		return err
	}
	defer func() { s.released, s.releasedRequired, s.minimal = nil, nil, s.file.Minimal }()
	s.released, s.releasedRequired = released, releasedRequired
	body, checks, err := s.attrs(attrs, "", "", accFull, true)
	if err != nil {
		return err
	}
	from := out.Full
	if config := resourceBlock(resourceType, body); config != out.Full.Config {
		out.UpgradeFull = &accStep{Config: config, Checks: checks}
		from = out.UpgradeFull
	}
	if out.UpgradeFullAction, err = s.nextAction(attrs, from, out.Updated, accFull, accUpdated); err != nil {
		return err
	}
	s.minimal = append(slices.Clone(s.file.Minimal), s.file.UpgradeMinimal...)
	body, checks, err = s.attrs(attrs, "", "", accMinimal, true)
	if err != nil {
		return err
	}
	out.UpgradeMinimal = &accStep{Config: resourceBlock(resourceType, body), Checks: checks}
	out.UpgradeMinimalAction, err = s.nextAction(attrs, out.UpgradeMinimal, out.Full, accMinimal, accFull)
	return err
}

// schemaAttrs adds every attribute path of the schema, nested ones and every oneOf arm included,
// with the path form of the acceptance file: rules[].targets[].connector_id.
func schemaAttrs(out map[string]*tfAttr, key string, attrs []*tfAttr) {
	for _, a := range attrs {
		path := joinPath(key, a.Name)
		out[path] = a
		child := path
		if a.Kind == "ListNested" || a.Kind == "SetNested" {
			child += "[]"
		}
		schemaAttrs(out, child, a.Attributes)
	}
}

// releasedAttrs returns the attributes that the release has: the paths that it has in the same
// type. A newer attribute, or one whose type changed, is left out of the first config. required are
// those that the release requires.
func releasedAttrs(prior *acceptance.UpgradeAttributes, schema map[string]*tfAttr) (released, required map[string]bool) {
	released, required = map[string]bool{}, map[string]bool{}
	for path, old := range prior.Attributes {
		if a, ok := schema[path]; ok && upgradeAttribute(a).Type == old.Type {
			released[path] = true
			required[path] = old.Required
		}
	}
	return released, required
}

// upgradeAttribute is the shape of an attribute in the upgrade attributes file.
func upgradeAttribute(a *tfAttr) acceptance.UpgradeAttribute {
	kind := a.Kind
	if a.ElementType != "" {
		kind += "(" + strings.TrimSuffix(strings.TrimPrefix(a.ElementType, "types."), "Type") + ")"
	}
	return acceptance.UpgradeAttribute{Type: kind, Required: a.Required}
}

// parentPath is the path of the object that holds the attribute, or "" at the top level.
func parentPath(path string) string {
	if i := strings.LastIndex(path, "."); i >= 0 {
		return strings.TrimSuffix(path[:i], "[]")
	}
	return ""
}

// lostRequired fails on an attribute that the released provider requires inside an object that
// the first config sets, when this build no longer has it in the same type: no config works with
// both providers.
func lostRequired(prior *acceptance.UpgradeAttributes, released map[string]bool) error {
	for _, path := range slices.Sorted(maps.Keys(prior.Attributes)) {
		parent := parentPath(path)
		if prior.Attributes[path].Required && !released[path] && (parent == "" || released[parent]) {
			return fmt.Errorf("%q is required by the released provider %s, but this build removed it or changed its type: "+
				"no config works with both; set upgradeFrom to a newer release", path, prior.From)
		}
	}
	return nil
}

// newRequired fails on a required attribute that the released provider does not have, inside an
// object that it has. The first config would leave it out, and this build would reject that config.
func newRequired(schema map[string]*tfAttr, released map[string]bool, from string) error {
	for _, path := range slices.Sorted(maps.Keys(schema)) {
		parent := parentPath(path)
		if schema[path].Required && !released[path] && (parent == "" || released[parent]) {
			return fmt.Errorf("%q is required but the released provider %s does not have it: no config works with both; "+
				"set upgradeFrom to a release that has it", path, from)
		}
	}
	return nil
}

// The plan checks of the step after an upgrade that nextAction returns, as Go expressions of type
// []plancheck.PlanCheck. accPlanUnknown checks no action.
const (
	accPlanNoop      = "[]plancheck.PlanCheck{plancheck.ExpectResourceAction(resourceAddress, plancheck.ResourceActionNoop)}"
	accPlanReplace   = "[]plancheck.PlanCheck{plancheck.ExpectResourceAction(resourceAddress, plancheck.ResourceActionReplace)}"
	accPlanNoReplace = "[]plancheck.PlanCheck{expectNoReplace{}}"
	accPlanUnknown   = "nil"
)

// nextAction is the plan check of a step from one config to the next. It is exact only where the
// action is sure: no-op for the same config, replace when a top-level immutable attribute surely
// differs. Otherwise the step must not replace the resource; it may update it or not, because a
// default or an API value in state can equal what the next config sets. The value checks after the
// apply catch a missing update. When an immutable attribute may differ (a left-out
// Optional+Computed one without a known default), the step checks no action.
// s.minimal and s.released must be those of the first config. The next config has every attribute.
func (s *accSynth) nextAction(attrs []*tfAttr, from, to *accStep, fromMode, toMode accMode) (string, error) {
	if from.Config == to.Config {
		return accPlanNoop, nil
	}
	released := s.released
	defer func() { s.released = released }()
	maybe := false
	for _, a := range attrs {
		if !immutable(a) {
			continue
		}
		s.released = released
		before, _, err := s.attrs([]*tfAttr{a}, "", "", fromMode, true)
		if err != nil {
			return "", err
		}
		s.released = nil
		after, _, err := s.attrs([]*tfAttr{a}, "", "", toMode, true)
		if err != nil {
			return "", err
		}
		switch attrChange(a, before, after) {
		case changeSure:
			return accPlanReplace, nil
		case changeMaybe:
			maybe = true // the state may already hold the next value, or not
		}
	}
	if maybe {
		return accPlanUnknown, nil
	}
	return accPlanNoReplace, nil
}

type attrChangeKind int

const (
	changeNone attrChangeKind = iota
	changeSure
	changeMaybe
)

// attrChange says whether one top-level attribute differs between two configs, from its config
// lines ("" when a config leaves it out). A left-out Computed attribute holds its known default, or
// what the API returned when no default is known.
func attrChange(a *tfAttr, before, after string) attrChangeKind {
	if before == after {
		return changeNone
	}
	if (before != "" && after != "") || !a.Computed {
		return changeSure
	}
	def, ok := defaultState(a)
	if !ok {
		return changeMaybe
	}
	if line := a.Name + " = " + hclLiteral(a, def); before == line || after == line {
		return changeNone
	}
	return changeSure
}

// hclLiteral is the HCL of a state value of a scalar attribute.
func hclLiteral(a *tfAttr, value string) string {
	if a.Kind == "String" {
		return strconv.Quote(value)
	}
	return value
}

// checkFile fails when the acceptance file names a field that the walk did not see, a minimal field
// that the user cannot set, or a value that no config used. A stale line cannot stay.
func (s *accSynth) checkFile() error {
	for name, paths := range map[string][]string{"skip": s.file.Skip, "minimal": s.file.Minimal, "upgradeMinimal": s.file.UpgradeMinimal} {
		for _, path := range paths {
			if !s.known[path] {
				return fmt.Errorf("%s: %q is not an attribute of the resource", name, path)
			}
			if name != "skip" && slices.Contains(s.file.Skip, path) {
				return fmt.Errorf("%s: %q is also in skip: skip leaves it out of every config", name, path)
			}
			if name != "skip" && !s.settable[path] {
				return fmt.Errorf("%s: %q is set by the server: a config cannot set it", name, path)
			}
			if name == "skip" && s.required[path] {
				return fmt.Errorf("skip: %q is required: a config cannot leave it out", path)
			}
		}
	}
	for _, path := range s.file.ValuePaths() {
		if !s.used[path] {
			return fmt.Errorf("values: %q is not set by the full config: it names no attribute, or the attribute is skipped, computed, or in a oneOf arm that the test leaves out", path)
		}
	}
	return nil
}

func resourceBlock(resourceType, body string) string {
	if body == "" {
		return fmt.Sprintf("resource %q \"test\" {\n}", resourceType)
	}
	return fmt.Sprintf("resource %q \"test\" {\n%s\n}", resourceType, indentLines(body))
}

func indentLines(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = "  " + line
		}
	}
	return strings.Join(lines, "\n")
}

func joinPath(parent, name string) string {
	if parent == "" {
		return name
	}
	return parent + "." + name
}

// include says whether the config of the mode sets the attribute.
func (s *accSynth) include(a *tfAttr, key string, mode accMode, top bool) bool {
	switch {
	case slices.Contains(s.file.Skip, key):
		return false
	case a.Extra:
		return false // extraAttributes are not API fields; a made-up map key is not valid
	case s.released != nil && !s.released[key]:
		return false // the released provider does not have it
	case !a.Required && !a.Optional:
		return false // the server sets it
	case s.released != nil && s.releasedRequired[key]:
		return true // the released provider requires it, even when this build does not
	case top && a.Name == s.idAttr && !a.Required:
		return false // the server makes the id; a changed id would replace the resource
	case a.Required:
		return true
	case a.OneOfRequired:
		return true // the group needs one arm; the walk takes the first that is included
	}
	return mode != accMinimal || slices.Contains(s.minimal, key) || s.minimalBelow(key)
}

// minimalBelow reports whether the minimal config sets a field below the attribute. The attribute
// is then set too, with its required fields and the listed ones.
func (s *accSynth) minimalBelow(key string) bool {
	return slices.ContainsFunc(s.minimal, func(p string) bool {
		return strings.HasPrefix(p, key+".") || strings.HasPrefix(p, key+"[].")
	})
}

// attrs makes the "name = value" lines of the attributes of one object, and the checks.
func (s *accSynth) attrs(attrs []*tfAttr, tfPath, key string, mode accMode, top bool) (string, []accCheck, error) {
	var lines []string
	var checks []accCheck
	armTaken := map[string]bool{} // the oneOf groups that already have an arm
	for _, a := range attrs {
		attrKey := joinPath(key, a.Name)
		s.known[attrKey] = true
		s.settable[attrKey] = a.Required || a.Optional
		s.required[attrKey] = a.Required
		if !s.include(a, attrKey, mode, top) {
			continue
		}
		if a.OneOfGroup != "" && !a.Required {
			// A oneOf group takes one arm. The test sets the first arm of each group.
			if armTaken[a.OneOfGroup] {
				continue
			}
			armTaken[a.OneOfGroup] = true
		}
		value, c, err := s.value(a, joinPath(tfPath, a.Name), attrKey, mode)
		if err != nil {
			return "", nil, fmt.Errorf("%s: %w", attrKey, err)
		}
		lines = append(lines, a.Name+" = "+value)
		checks = append(checks, c...)
	}
	return strings.Join(lines, "\n"), checks, nil
}

// absent lists the top-level attributes that the minimal config leaves out and that the server
// does not set: the state must not have them.
func (s *accSynth) absent(attrs []*tfAttr) []string {
	var out []string
	for _, a := range attrs {
		scalar := slices.Contains([]string{"String", "Bool", "Int64", "Int32", "Float64", "Float32"}, a.Kind)
		if scalar && a.Optional && !a.Computed && !s.include(a, a.Name, accMinimal, true) {
			out = append(out, a.Name)
		}
	}
	return out
}

// removed returns what the state holds for the top-level Optional+Computed scalars that the minimal
// config leaves out, when the schema states it. defaults hold for every minimal step: a static
// default or a declared server default. kept hold after the full config: an attribute with
// UseStateForUnknown keeps its value. An attribute with none of these takes what the API returns,
// which the schema does not state; the empty plan after the apply and the import still check it.
func (s *accSynth) removed(attrs []*tfAttr, full []accCheck) (defaults, kept []accCheck) {
	for _, a := range attrs {
		scalar := slices.Contains([]string{"String", "Bool", "Int64", "Int32", "Float64", "Float32"}, a.Kind)
		if !scalar || !a.Optional || !a.Computed || s.include(a, a.Name, accMinimal, true) {
			continue
		}
		if value, ok := defaultState(a); ok {
			defaults = append(defaults, accCheck{a.Name, value})
			continue
		}
		if !slices.ContainsFunc(a.Modifiers, func(m string) bool { return strings.Contains(m, "UseStateForUnknown") }) {
			continue
		}
		if i := slices.IndexFunc(full, func(c accCheck) bool { return c.Path == a.Name }); i >= 0 {
			kept = append(kept, full[i])
		}
	}
	return defaults, kept
}

var (
	// staticDefault matches a static default, such as booldefault.StaticBool(false).
	staticDefault = regexp.MustCompile(`^\w+default\.Static\w+\((.*)\)$`)
	// serverDefaultCall matches the plan modifier of a declared server default.
	serverDefaultCall = regexp.MustCompile(`^serverDefaultModifier\{value: types\.\w+Value\((.*)\)\}$`)
)

// defaultState returns the state value of an attribute that its config leaves out, when the
// attribute has a static default or a declared server default.
func defaultState(a *tfAttr) (string, bool) {
	exprs := append([]string{a.Default}, a.Modifiers...)
	for _, expr := range exprs {
		m := staticDefault.FindStringSubmatch(expr)
		if m == nil {
			m = serverDefaultCall.FindStringSubmatch(expr)
		}
		if m == nil {
			continue
		}
		if v, err := strconv.Unquote(m[1]); err == nil {
			return v, true
		}
		return m[1], true
	}
	return "", false
}

// nestedImmutable returns the path of an attribute below the top level that has RequiresReplace, or
// "". The schema builder adds it only to top-level fields, so nextAction compares only those.
func nestedImmutable(key string, attrs []*tfAttr, top bool) string {
	for _, a := range attrs {
		path := joinPath(key, a.Name)
		if !top && immutable(a) {
			return path
		}
		child := path
		if a.Kind == "ListNested" || a.Kind == "SetNested" {
			child += "[]"
		}
		if found := nestedImmutable(child, a.Attributes, false); found != "" {
			return found
		}
	}
	return ""
}

// immutable reports whether a change of a replaces the resource, with the
// stock RequiresReplace or with requestReplaceModifier.
func immutable(a *tfAttr) bool { return replaces(a) }

// value makes the HCL value of an attribute and the checks on it.
func (s *accSynth) value(a *tfAttr, tfPath, key string, mode accMode) (string, []accCheck, error) {
	if mode == accMinimal && slices.Contains(s.minimal, key) {
		mode = accFull
	}
	if immutable(a) && mode == accUpdated {
		mode = accFull // a changed immutable attribute replaces the resource
	}
	if v, ok := s.file.Values[key]; ok {
		s.used[key] = true
		return strings.TrimSpace(v), nil, nil
	}
	switch a.Kind {
	case "String", "Bool", "Int64", "Int32", "Float64", "Float32":
		v, check := scalarValue(a, mode)
		return v, []accCheck{{tfPath, check}}, nil
	case "Map", "List", "Set":
		return s.collectionValue(a, tfPath, mode)
	case "SingleNested":
		body, checks, err := s.attrs(a.Attributes, tfPath, key, mode, false)
		if err != nil {
			return "", nil, err
		}
		return objectHCL(body), checks, nil
	case "ListNested", "SetNested":
		return s.nestedCollection(a, tfPath, key, mode)
	}
	return "", nil, fmt.Errorf("the %s attribute kind is not supported: set the value in %s", a.Kind, acceptance.FileName)
}

func objectHCL(body string) string {
	if body == "" {
		return "{}"
	}
	return "{\n" + indentLines(body) + "\n}"
}

func (s *accSynth) nestedCollection(a *tfAttr, tfPath, key string, mode accMode) (string, []accCheck, error) {
	if err := checkOneElement(a); err != nil {
		return "", nil, err
	}
	element := tfPath + ".0"
	if a.Kind == "SetNested" {
		element = tfPath + ".*" // the position of a set element is not known: no element checks
	}
	body, checks, err := s.attrs(a.Attributes, element, key+"[]", mode, false)
	if err != nil {
		return "", nil, err
	}
	if a.Kind == "SetNested" {
		checks = nil
	}
	checks = append([]accCheck{{tfPath + ".#", "1"}}, checks...)
	return "[\n" + indentLines(objectHCL(body)+",") + "\n]", checks, nil
}

// scalarValue makes a plain value of a scalar attribute and the string that the state holds.
func scalarValue(a *tfAttr, mode accMode) (hcl, state string) {
	updated := mode == accUpdated
	switch a.Kind {
	case "Bool":
		v := !updated
		return strconv.FormatBool(v), strconv.FormatBool(v)
	case "Int64", "Int32":
		full, next := numberValues(a, 1)
		if updated {
			full = next
		}
		v := strconv.FormatInt(int64(full), 10)
		return v, v
	case "Float64", "Float32":
		full, next := numberValues(a, 1.5)
		if updated {
			full = next
		}
		v := strconv.FormatFloat(full, 'g', -1, 64)
		return v, v
	}
	v := stringValue(a, updated)
	return strconv.Quote(v), v
}

// sizeCall matches a size validator of a collection, such as listvalidator.SizeAtLeast(2).
var sizeCall = regexp.MustCompile(`^(?:list|set|map)validator\.(SizeAtLeast|SizeAtMost|SizeBetween)\((.*)\)$`)

// checkOneElement returns an error when a size validator of the collection rejects one element,
// the size of a made-up collection. The test then needs the value in the acceptance file.
func checkOneElement(a *tfAttr) error {
	for _, v := range a.Validators {
		m := sizeCall.FindStringSubmatch(v)
		if m == nil {
			continue
		}
		var bounds []int
		for _, arg := range strings.Split(m[2], ",") {
			n, err := strconv.Atoi(strings.TrimSpace(arg))
			if err != nil {
				return fmt.Errorf("cannot read %s: set the value in %s", v, acceptance.FileName)
			}
			bounds = append(bounds, n)
		}
		fits := m[1] == "SizeAtLeast" && bounds[0] <= 1 ||
			m[1] == "SizeAtMost" && bounds[0] >= 1 ||
			m[1] == "SizeBetween" && len(bounds) == 2 && bounds[0] <= 1 && bounds[1] >= 1
		if !fits {
			return fmt.Errorf("one made-up element does not pass %s: set the value in %s", v, acceptance.FileName)
		}
	}
	return nil
}

// rangeCall matches a range validator of a number, such as int64validator.Between(1, 5).
var rangeCall = regexp.MustCompile(`^(?:int32|int64|float32|float64)validator\.(AtLeast|AtMost|Between)\((.*)\)$`)

// numberValues returns the made-up numbers of the full and the updated config: base and base+1
// when they fit the range validators of the attribute. Otherwise the start of the range and the
// next number, or the middle when the next number is outside. A range of one number keeps it.
func numberValues(a *tfAttr, base float64) (full, updated float64) {
	low, high := numberRange(a)
	full, updated = base, base+1
	switch {
	case full >= low && updated <= high:
		return full, updated
	case !math.IsInf(low, -1):
		full, updated = low, low+1
	default:
		full, updated = high, high-1
	}
	if updated > high {
		updated = (full + high) / 2
	}
	if updated < low {
		updated = (full + low) / 2
	}
	return full, updated
}

// numberRange returns the bounds of the range validators of a number attribute. A missing bound
// is infinite.
func numberRange(a *tfAttr) (low, high float64) {
	low, high = math.Inf(-1), math.Inf(1)
	for _, v := range a.Validators {
		m := rangeCall.FindStringSubmatch(v)
		if m == nil {
			continue
		}
		var bounds []float64
		for _, arg := range strings.Split(m[2], ",") {
			n, err := strconv.ParseFloat(strings.TrimSpace(arg), 64)
			if err != nil {
				return math.Inf(-1), math.Inf(1)
			}
			bounds = append(bounds, n)
		}
		switch {
		case m[1] == "AtLeast" && len(bounds) == 1:
			low = max(low, bounds[0])
		case m[1] == "AtMost" && len(bounds) == 1:
			high = min(high, bounds[0])
		case m[1] == "Between" && len(bounds) == 2:
			low, high = max(low, bounds[0]), min(high, bounds[1])
		}
	}
	return low, high
}

// stringValue is the first accepted value of an enum, or a plain unique string. An enum keeps its
// value in the update config: the valid values of other fields can depend on it (a rule condition
// depends on the entity type), and the test cannot know how.
func stringValue(a *tfAttr, updated bool) string {
	if values := enumValues(a); len(values) != 0 {
		return values[0]
	}
	low, high := lengthRange(a)
	return madeUpString(a.Name, updated, low, high)
}

// runLength is the length of @{run} in a test run: "acc-" and 8 characters.
const runLength = 12

// madeUpString makes "@{run}-<name>", or "@{run}-<name>-updated" in the update config, fit the
// length range. It keeps @{run}, so the value stays unique per run: a long value is cut after it,
// and the update starts with "u" instead of "-" so that the two values still differ. A short value
// is padded with "x". A maximum below the length of @{run} gets a fixed value.
func madeUpString(name string, updated bool, low, high int) string {
	if high < runLength {
		n := min(max(low, 1), high)
		if updated {
			return strings.Repeat("b", n)
		}
		return strings.Repeat("a", n)
	}
	room := high - runLength
	tail := "-" + name
	if updated {
		tail += "-updated"
		if len(tail) > room {
			tail = "u" + name
		}
	}
	tail = tail[:min(len(tail), room)]
	if short := low - runLength - len(tail); short > 0 {
		tail += strings.Repeat("x", short)
	}
	return "@{run}" + tail
}

// lengthCall matches a length validator of a string, such as stringvalidator.LengthAtMost(32).
var lengthCall = regexp.MustCompile(`^stringvalidator\.(LengthAtLeast|LengthAtMost|LengthBetween)\((.*)\)$`)

// lengthRange returns the bounds of the length validators of a string attribute. A missing
// maximum is math.MaxInt.
func lengthRange(a *tfAttr) (low, high int) {
	low, high = 0, math.MaxInt
	for _, v := range a.Validators {
		m := lengthCall.FindStringSubmatch(v)
		if m == nil {
			continue
		}
		var bounds []int
		for _, arg := range strings.Split(m[2], ",") {
			n, err := strconv.Atoi(strings.TrimSpace(arg))
			if err != nil {
				return 0, math.MaxInt
			}
			bounds = append(bounds, n)
		}
		switch {
		case m[1] == "LengthAtLeast" && len(bounds) == 1:
			low = max(low, bounds[0])
		case m[1] == "LengthAtMost" && len(bounds) == 1:
			high = min(high, bounds[0])
		case m[1] == "LengthBetween" && len(bounds) == 2:
			low, high = max(low, bounds[0]), min(high, bounds[1])
		}
	}
	return low, high
}

// enumValues returns the values of the OneOf validator of a string attribute, without the zero
// value "unspecified" when others exist.
func enumValues(a *tfAttr) []string {
	var values []string
	for _, v := range a.Validators {
		if !strings.HasPrefix(v, "stringvalidator.OneOf(") {
			continue
		}
		for _, m := range quoted.FindAllStringSubmatch(v, -1) {
			values = append(values, m[1])
		}
	}
	if len(values) > 1 {
		values = slices.DeleteFunc(values, func(v string) bool { return v == "unspecified" })
	}
	return values
}

// collectionValue makes a map, list, or set of plain values. Only string, number, and bool
// elements are made up.
func (s *accSynth) collectionValue(a *tfAttr, tfPath string, mode accMode) (string, []accCheck, error) {
	elem, ok := map[string]string{
		"types.StringType": "String", "types.BoolType": "Bool", "types.Int64Type": "Int64",
		"types.Int32Type": "Int32", "types.Float64Type": "Float64", "types.Float32Type": "Float32",
	}[a.ElementType]
	if !ok {
		return "", nil, fmt.Errorf("elements of type %s are not supported: set the value in %s", a.ElementType, acceptance.FileName)
	}
	if err := checkOneElement(a); err != nil {
		return "", nil, err
	}
	item, state := scalarValue(&tfAttr{Name: a.Name, Kind: elem, Validators: a.ElemValidators}, mode)
	switch a.Kind {
	case "Map":
		return "{ key = " + item + " }", []accCheck{{tfPath + ".key", state}}, nil
	case "List":
		return "[" + item + "]", []accCheck{{tfPath + ".#", "1"}, {tfPath + ".0", state}}, nil
	}
	return "[" + item + "]", []accCheck{{tfPath + ".#", "1"}}, nil
}

// UsesNoReplace reports whether a step after an upgrade checks only that the plan does not replace.
func (d *acceptanceData) UsesNoReplace() bool {
	return d.UpgradeFullAction == accPlanNoReplace || d.UpgradeMinimalAction == accPlanNoReplace
}

// renderAcceptance returns the generated acceptance test.
func renderAcceptance(data *acceptanceData) ([]byte, error) {
	var buf strings.Builder
	if err := templates.ExecuteTemplate(&buf, "acceptance_test.go.tmpl", data); err != nil {
		return nil, fmt.Errorf("acceptance_test.go: %w", err)
	}
	return formatGenerated("acceptance_test.go", []byte(buf.String()))
}

// checkUpgradeLocation requires the acceptance file of an upgrade test in the output directory. The
// upgrade attributes file sits next to it, so check and generate read the same list.
func checkUpgradeLocation(options Options, file *acceptance.File) error {
	if file == nil || file.UpgradeFrom == "" {
		return nil
	}
	same, err := sameDirectory(filepath.Dir(acceptancePath(options)), options.OutputDir)
	if err != nil {
		return err
	}
	if !same {
		return fmt.Errorf("the acceptance file has upgradeFrom, so it must be in the output directory %s: %s sits next to it",
			options.OutputDir, acceptance.UpgradeFileName)
	}
	return nil
}

// readUpgrade reads the upgrade attributes file in dir. It returns nil when there is none.
func readUpgrade(dir string) (*acceptance.UpgradeAttributes, error) {
	if dir == "" {
		return nil, nil
	}
	data, err := os.ReadFile(filepath.Join(dir, acceptance.UpgradeFileName))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", acceptance.UpgradeFileName, err)
	}
	return acceptance.ParseUpgrade(data)
}

// readAcceptance reads the acceptance file. It returns nil when there is none.
func readAcceptance(options Options) (*acceptance.File, error) {
	path := acceptancePath(options)
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read acceptance file %s: %w", path, err)
	}
	file, err := acceptance.Parse(data)
	if err != nil {
		return nil, err
	}
	if file.Resource != options.Resource {
		return nil, fmt.Errorf("%s is for resource %q, but the selected resource is %q", acceptance.FileName, file.Resource, options.Resource)
	}
	return file, nil
}

func acceptancePath(options Options) string {
	if options.AcceptancePath != "" {
		return options.AcceptancePath
	}
	if options.OutputDir == "" {
		return ""
	}
	candidate := filepath.Join(options.OutputDir, acceptance.FileName)
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate
	}
	return ""
}

// keepAcceptance puts the acceptance file of the output directory back, like keepOverrides.
func keepAcceptance(options Options, files map[string][]byte) error {
	path := acceptancePath(options)
	if path == "" || filepath.Base(path) != acceptance.FileName {
		return nil
	}
	inOutput, err := sameDirectory(filepath.Dir(path), options.OutputDir)
	if err != nil || !inOutput {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read acceptance file %s: %w", path, err)
	}
	files[acceptance.FileName] = data
	return nil
}
