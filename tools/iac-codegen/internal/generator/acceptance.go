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

// accCheck is one attribute check of an acceptance step. Absent: the attribute must not be in state
// (a write-only value).
type accCheck struct {
	Path, Value string
	Absent      bool
}

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
	// UpgradeCases are extra upgrade subtests from the acceptance file.
	UpgradeCases []upgradeCaseData
	// DataSource is the data source step of full-lifecycle. nil when the package has no data source.
	DataSource *accDataSource
}

// accDataSource is the step that reads the data source of the resource that the full config created.
type accDataSource struct {
	Address string // address of the data source in the config
	// ByNameAddress is the address of the data source that reads the resource by its name, or "".
	ByNameAddress string
	Config        string // the data blocks
	// Pairs are the attribute paths that the data source and the resource must have with the same
	// value: the id, and every value that the full config checks.
	Pairs []string
}

// upgradeCaseData is one extra upgrade subtest.
type upgradeCaseData struct {
	Name, Ident string
	Config      string
	Checks      []accCheck
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
	// immutables records the HCL of each immutable value that a walk sets, by its state path. nil
	// records nothing.
	immutables map[string]string
}

var quoted = regexp.MustCompile(`"([^"]*)"`)

// buildAcceptance makes the data of the acceptance test of a resource.
// prior is the upgrade attributes file of the last run, or nil.
func buildAcceptance(res *tfResource, providerModule string, file *acceptance.File, prior *acceptance.UpgradeAttributes) (*acceptanceData, error) {
	typeName := res.CRUD.TypeName
	s := &accSynth{file: file, minimal: file.Minimal, idAttr: res.CRUD.IDAttr, known: map[string]bool{}, settable: map[string]bool{}, required: map[string]bool{}, used: map[string]bool{}}
	out := &acceptanceData{
		Package:        res.Package,
		VersionHeader:  version.Header,
		ProviderImport: providerModule + "/internal/provider",
		Name:           camelize(typeName), // the Terraform type, which can differ from the SDK type
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
	if res.DataSource != nil {
		out.DataSource = dataSourceStep(res.CRUD, res.DataSource.Name, out.Full.Checks)
	}
	return out, nil
}

// dataSourceStep reads the data source by the id of the resource, or with no input for a singleton,
// which depends on the resource instead. With name lookup, a second data source reads the resource
// by its name.
func dataSourceStep(crud *crudData, name *dsNameLookup, checks []accCheck) *accDataSource {
	resourceType := "coralogix_" + crud.TypeName
	body := fmt.Sprintf("%s = %s.test.%s", crud.IDAttr, resourceType, crud.IDAttr)
	if crud.Singleton {
		body = fmt.Sprintf("depends_on = [%s.test]", resourceType)
	}
	out := &accDataSource{
		Address: "data." + resourceType + ".test",
		Config:  fmt.Sprintf("data %q \"test\" {\n  %s\n}", resourceType, body),
		Pairs:   []string{crud.IDAttr},
	}
	if name != nil {
		out.ByNameAddress = "data." + resourceType + ".by_name"
		out.Config += fmt.Sprintf("\n\ndata %q \"by_name\" {\n  %s = %s.test.%s\n}", resourceType, name.Attr, resourceType, name.Attr)
	}
	for _, check := range checks {
		if !check.Absent && !slices.Contains(out.Pairs, check.Path) {
			out.Pairs = append(out.Pairs, check.Path)
		}
	}
	return out
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
	if err != nil {
		return err
	}
	return s.addUpgradeCases(out, attrs, resourceType)
}

// addUpgradeCases makes the extra upgrade subtests. Each is the full upgrade config plus the case
// values. extraAttributes are included when the merged values name them, so a write-only map can
// use a real field name.
func (s *accSynth) addUpgradeCases(out *acceptanceData, attrs []*tfAttr, resourceType string) error {
	base := out.Full
	if out.UpgradeFull != nil {
		base = out.UpgradeFull
	}
	orig := s.file.Values
	defer func() { s.file.Values = orig }()
	for _, c := range s.file.UpgradeCases {
		for _, path := range c.ValuePaths() {
			if !s.known[path] {
				return fmt.Errorf("upgradeCases.%s.values: %q is not an attribute of the resource", c.Name, path)
			}
			if !s.settable[path] {
				return fmt.Errorf("upgradeCases.%s.values: %q is set by the server: a config cannot set it", c.Name, path)
			}
			if slices.Contains(s.file.Skip, path) {
				return fmt.Errorf("upgradeCases.%s.values: %q is also in skip: skip leaves it out of every config", c.Name, path)
			}
			if s.released != nil && !s.released[path] {
				return fmt.Errorf("upgradeCases.%s.values: %q is not in %s: the released provider %s would reject it; add it if that release has the attribute",
					c.Name, path, acceptance.UpgradeFileName, s.file.UpgradeFrom)
			}
		}
		s.file.Values = mergeValues(orig, c.Values)
		body, checks, err := s.attrs(attrs, "", "", accFull, true)
		if err != nil {
			return err
		}
		for _, path := range c.ValuePaths() {
			if !s.used[path] {
				return fmt.Errorf("upgradeCases.%s.values: %q is not set by the config: it names no attribute, or the attribute is skipped, computed, or in a oneOf arm that the test leaves out", c.Name, path)
			}
		}
		config := resourceBlock(resourceType, body)
		if config == base.Config {
			return fmt.Errorf("upgradeCases.%s does not change the upgrade config: give values that the full upgrade config does not already set", c.Name)
		}
		out.UpgradeCases = append(out.UpgradeCases, upgradeCaseData{
			Name: c.Name, Ident: upgradeCaseIdent(c.Name), Config: config, Checks: checks,
		})
	}
	return nil
}

func mergeValues(base, extra map[string]string) map[string]string {
	out := maps.Clone(base)
	if out == nil {
		out = map[string]string{}
	}
	maps.Copy(out, extra)
	return out
}

func upgradeCaseIdent(name string) string {
	parts := strings.Split(name, "-")
	for i, p := range parts {
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, "")
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
// action is sure: no-op for the same config, replace when an immutable value surely
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
		if !immutable(a) && !holdsImmutable(a) {
			continue
		}
		s.released = released
		before, beforeValues, err := s.immutableValues(a, fromMode)
		if err != nil {
			return "", err
		}
		s.released = nil
		after, afterValues, err := s.immutableValues(a, toMode)
		if err != nil {
			return "", err
		}
		change := attrChange(a, before, after)
		if !immutable(a) {
			change = nestedChange(beforeValues, afterValues)
		}
		switch change {
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
	if slices.Contains(s.file.Skip, key) || !s.inRelease(key) {
		return false // skipped, or the released provider does not have it
	}
	if a.Extra {
		_, ok := s.file.Values[key]
		return ok // extraAttributes are not API fields; a made-up map key is not valid
	}
	switch {
	case !a.Required && !a.Optional:
		return false // the server sets it
	case s.released != nil && s.releasedRequired[key]:
		return true // the released provider requires it, even when this build does not
	case top && a.Name == s.idAttr && !a.Required:
		return false // the server makes the id; a changed id would replace the resource
	case a.Required, a.OneOfRequired:
		return true // a OneOfRequired group needs one arm; the walk takes the first that is included
	}
	return mode != accMinimal || slices.Contains(s.minimal, key) || s.minimalBelow(key)
}

// inRelease reports whether the released provider of the upgrade test has the attribute. Without
// an upgrade config, every attribute is in.
func (s *accSynth) inRelease(key string) bool {
	return s.released == nil || s.released[key]
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
			defaults = append(defaults, accCheck{Path: a.Name, Value: value})
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

// immutableValues renders the attribute as the config of the mode does, and returns its lines and
// the immutable values in it, by state path. The values come from the real config, so a oneOf
// group keeps the arm that the config takes.
func (s *accSynth) immutableValues(a *tfAttr, mode accMode) (string, map[string]string, error) {
	s.immutables = map[string]string{}
	defer func() { s.immutables = nil }()
	lines, _, err := s.attrs([]*tfAttr{a}, "", "", mode, true)
	return lines, s.immutables, err
}

// holdsImmutable reports whether an attribute below a replaces the resource when it changes.
func holdsImmutable(a *tfAttr) bool {
	return slices.ContainsFunc(a.Attributes, func(child *tfAttr) bool { return immutable(child) || holdsImmutable(child) })
}

// nestedChange compares the immutable values below a top-level attribute in two configs. A value
// that both set and that differs replaces the resource. A value that only one sets may: that
// depends on the plan modifiers of its parents, and on what the state holds.
func nestedChange(before, after map[string]string) attrChangeKind {
	change := changeNone
	for path, value := range after {
		prior, ok := before[path]
		switch {
		case ok && prior != value:
			return changeSure
		case !ok:
			change = changeMaybe
		}
	}
	for path := range before {
		if _, ok := after[path]; !ok {
			change = changeMaybe
		}
	}
	return change
}

// immutable reports whether a change of a replaces the resource, with the
// stock RequiresReplace or with requestReplaceModifier.
func immutable(a *tfAttr) bool { return replaces(a) }

// value makes the HCL value of an attribute and the checks on it.
func (s *accSynth) value(a *tfAttr, tfPath, key string, mode accMode) (string, []accCheck, error) {
	hcl, checks, err := s.makeValue(a, tfPath, key, mode)
	if err == nil && s.immutables != nil && immutable(a) {
		s.immutables[tfPath] = hcl
	}
	return hcl, checks, err
}

func (s *accSynth) makeValue(a *tfAttr, tfPath, key string, mode accMode) (string, []accCheck, error) {
	if mode == accMinimal && slices.Contains(s.minimal, key) {
		mode = accFull
	}
	if immutable(a) && mode == accUpdated {
		mode = accFull // a changed immutable attribute replaces the resource
	}
	if v, ok := s.file.Values[key]; ok {
		s.used[key] = true
		var checks []accCheck
		if a.WriteOnly {
			checks = []accCheck{{Path: tfPath, Absent: true}}
		}
		return strings.TrimSpace(v), checks, nil
	}
	switch a.Kind {
	case "String", "Bool", "Int64", "Int32", "Float64", "Float32":
		v, check, err := scalarValue(a, mode)
		if err != nil {
			return "", nil, err
		}
		return v, []accCheck{{Path: tfPath, Value: check}}, nil
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
	checks = append([]accCheck{{Path: tfPath + ".#", Value: "1"}}, checks...)
	return "[\n" + indentLines(objectHCL(body)+",") + "\n]", checks, nil
}

// scalarValue makes a plain value of a scalar attribute and the string that the state holds.
func scalarValue(a *tfAttr, mode accMode) (hcl, state string, err error) {
	updated := mode == accUpdated
	switch a.Kind {
	case "Bool":
		v := !updated || a.TrueOnly
		return strconv.FormatBool(v), strconv.FormatBool(v), nil
	case "Int64", "Int32":
		full, next := numberValues(a, 1)
		if updated {
			full = next
		}
		v := strconv.FormatInt(int64(full), 10)
		return v, v, nil
	case "Float64", "Float32":
		full, next := numberValues(a, 1.5)
		if updated {
			full = next
		}
		v := strconv.FormatFloat(full, 'g', -1, 64)
		return v, v, nil
	}
	v, err := stringValue(a, updated)
	if err != nil {
		return "", "", err
	}
	return strconv.Quote(v), v, nil
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
// depends on the entity type), and the test cannot know how. When the plain string does not match
// the pattern of the attribute, the value is built from the pattern, and is not unique per run. When
// the built value does not fit the length limits either, the test needs the value in the acceptance
// file.
func stringValue(a *tfAttr, updated bool) (string, error) {
	if values := enumValues(a); len(values) != 0 {
		return values[0], nil
	}
	low, high := lengthRange(a)
	if a.CustomType == rfc3339Type {
		return timeValue(a, updated, low, high)
	}
	v := madeUpString(a.Name, updated, low, high)
	re := patternOf(a)
	if re == nil || madeUpMatches(re, v) {
		return v, nil
	}
	if sample, ok := patternValue(re, updated, low, high); ok {
		return sample, nil
	}
	return "", fmt.Errorf("no made-up value of %s matches the pattern %q and the length limits: set the value in %s", a.Name, re, acceptance.FileName)
}

// timeValue is a time in UTC, a day apart in the update config. A value built from a pattern could
// be no valid time, so a pattern or a length that rejects it needs the value in the acceptance file.
func timeValue(a *tfAttr, updated bool, low, high int) (string, error) {
	v := "2030-01-01T00:00:00Z"
	if updated {
		v = "2030-01-02T00:00:00Z"
	}
	if re := patternOf(a); re != nil && !re.MatchString(v) || len(v) < low || len(v) > high {
		return "", fmt.Errorf("the time %s of %s does not pass its validators: set the value in %s", v, a.Name, acceptance.FileName)
	}
	return v, nil
}

// timeType returns rfc3339Type when elemType is the time type, so an element of a list of times
// gets a time value.
func timeType(elemType string) string {
	if elemType == rfc3339Type {
		return rfc3339Type
	}
	return ""
}

// runLength is the length of @{run} in a test run: "acc-" and 8 characters.
const runLength = 12

// madeUpString makes "@{run}-<name>", or "@{run}-<name>-updated" in the update config, fit the
// length range. It keeps @{run}, so the value stays unique per run: a long value is cut after it,
// and the update starts with "u" instead of "-" so that the two values still differ. A short value
// is padded with "x". A maximum below the length of @{run} gets a fixed value.
func madeUpString(name string, updated bool, low, high int) string {
	room := high - runLength       // the longest tail that fits with a test run
	short := low - len(exampleRun) // the shortest tail that fits in the docs example
	if room < 0 || short > room {
		n := min(max(low, 1), high)
		if updated {
			return strings.Repeat("b", n)
		}
		return strings.Repeat("a", n)
	}
	tail := "-" + name
	if updated {
		tail += "-updated"
		if len(tail) > room {
			tail = "u" + name
		}
	}
	tail = tail[:min(len(tail), room)]
	if pad := short - len(tail); pad > 0 {
		tail += strings.Repeat("x", pad)
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
		"types.StringType": "String", rfc3339Type: "String", "types.BoolType": "Bool", "types.Int64Type": "Int64",
		"types.Int32Type": "Int32", "types.Float64Type": "Float64", "types.Float32Type": "Float32",
	}[a.ElementType]
	if !ok {
		return "", nil, fmt.Errorf("elements of type %s are not supported: set the value in %s", a.ElementType, acceptance.FileName)
	}
	if err := checkOneElement(a); err != nil {
		return "", nil, err
	}
	item, state, err := scalarValue(&tfAttr{Name: a.Name, Kind: elem, Validators: a.ElemValidators, CustomType: timeType(a.ElementType)}, mode)
	if err != nil {
		return "", nil, err
	}
	switch a.Kind {
	case "Map":
		return "{ key = " + item + " }", []accCheck{{Path: tfPath + ".key", Value: state}}, nil
	case "List":
		return "[" + item + "]", []accCheck{{Path: tfPath + ".#", Value: "1"}, {Path: tfPath + ".0", Value: state}}, nil
	}
	return "[" + item + "]", []accCheck{{Path: tfPath + ".#", Value: "1"}}, nil
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
