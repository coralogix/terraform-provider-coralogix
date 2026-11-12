package generator

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/acceptance"
)

// The values-and-choices subtest sends every value of every enum and every arm of every oneOf group
// to the API. The full config sets the first value and the first arm. The steps after it set what
// no earlier config has set. A step takes, in each oneOf group, the arm with the most that is not
// set yet (the arm itself, and the enum values below it), and in each enum the first value that is
// not set yet. The sweep stops when a step sets nothing new. Because the steps are planned by what
// is left, a value below an arm that is not the first one is set too, which a step number per level
// would miss.

// sweepArmWeight makes an arm that no config has set outweigh any number of enum values below it.
const sweepArmWeight = 1000

// maxSweepSteps stops a sweep that does not end. The values and the arms of a schema are finite and
// each step sets one more, so a longer sweep is a bug.
const maxSweepSteps = 500

// choices returns the values that a user can set in an enum attribute, in the order of the
// contract. An enum of a new resource has no mapping: its validator states the values, spelled as
// the API does.
func choices(a *tfAttr) []enumOption {
	if len(a.EnumChoices) != 0 {
		return a.EnumChoices
	}
	var out []enumOption
	for _, v := range enumValues(a) {
		out = append(out, enumOption{API: v, TF: v})
	}
	return out
}

// enumID identifies an enum for the sweep: its component, or the path of an attribute that has a
// list of values without one.
func enumID(a *tfAttr, key string) string {
	if a.EnumSchema != "" {
		return a.EnumSchema
	}
	return key
}

func enumCoverKey(id, tf string) string { return "enum\x00" + id + "\x00" + tf }

func armCoverKey(a *tfAttr) string { return "arm\x00" + a.Component + "\x00" + a.Property }

func (s *accSynth) skippedValue(a *tfAttr, o enumOption) bool {
	return a.EnumSchema != "" && slices.Contains(s.file.SkipValues[a.EnumSchema].Values, o.API)
}

func (s *accSynth) skippedArm(a *tfAttr) bool {
	return slices.Contains(s.file.SkipChoices[a.Component].Choices, a.Property)
}

// uncovered returns the first value of the enum that no config sets yet and that the file does not skip.
func (s *accSynth) uncovered(a *tfAttr, key string) (enumOption, bool) {
	id := enumID(a, key)
	for _, o := range choices(a) {
		if !s.cov[enumCoverKey(id, o.TF)] && !s.skippedValue(a, o) {
			return o, true
		}
	}
	return enumOption{}, false
}

// sweepValue returns the value that a step of the sweep sets in an enum attribute, and marks it as
// set. It reports false when the attribute is no enum, or has no value left to set: the step then
// keeps the value of the full config.
func (s *accSynth) sweepValue(a *tfAttr, key string) (string, bool) {
	if !s.sweeping {
		return "", false
	}
	o, ok := s.uncovered(a, key)
	if !ok {
		return "", false
	}
	s.cov[enumCoverKey(enumID(a, key), o.TF)] = true
	return o.TF, true
}

// noteValue records the value that a config set in an enum attribute.
func (s *accSynth) noteValue(a *tfAttr, key, tf string) {
	if s.cov != nil && len(choices(a)) != 0 {
		s.cov[enumCoverKey(enumID(a, key), tf)] = true
	}
}

// noteArm records the arm that a config set.
func (s *accSynth) noteArm(a *tfAttr) {
	if s.cov != nil {
		s.cov[armCoverKey(a)] = true
	}
}

// gain counts what the config sets new when it sets the attribute: the enum values below it that no
// config has set, one per enum attribute. A fixed value of the acceptance file and a left out
// attribute set nothing.
func (s *accSynth) gain(a *tfAttr, key string) int {
	if !s.include(a, key, accFull, false) {
		return 0
	}
	if _, fixed := s.file.Values[key]; fixed {
		return 0
	}
	g := 0
	if len(choices(a)) != 0 && (a.Kind == "String" || a.Kind == "List" || a.Kind == "Set") {
		if _, ok := s.uncovered(a, key); ok {
			g++
		}
	}
	below := key
	if a.Kind == "ListNested" || a.Kind == "SetNested" {
		below = key + "[]"
	}
	for _, c := range a.Attributes {
		g += s.gain(c, joinPath(below, c.Name))
	}
	return g
}

// armPicks chooses the arm of each oneOf group of the attributes for a step of the sweep. A group
// takes the arm with the most gain. Without a gain, or without a sweep, it is left out of the map:
// the first arm that the config includes is taken, as in the full config.
func (s *accSynth) armPicks(attrs []*tfAttr, key string, mode accMode, top bool) map[string]*tfAttr {
	if !s.sweeping {
		return nil
	}
	picks := map[string]*tfAttr{}
	best := map[string]int{}
	for _, a := range attrs {
		attrKey := joinPath(key, a.Name)
		if a.OneOfGroup == "" || a.Required || !s.include(a, attrKey, mode, top) || s.skippedArm(a) || refused(a) || !s.canMake(a, attrKey, mode) {
			continue
		}
		g := s.gain(a, attrKey)
		if !s.cov[armCoverKey(a)] {
			g += sweepArmWeight
		}
		if g > best[a.OneOfGroup] {
			best[a.OneOfGroup], picks[a.OneOfGroup] = g, a
		}
	}
	return picks
}

// canMake reports whether the test can make a value for the arm: a made-up value needs a kind of
// attribute that the generator supports. An arm that it cannot make stays out of the sweep, and
// sweepSteps reports it, so that a person lists it in skipChoices or sets its values in the file.
func (s *accSynth) canMake(a *tfAttr, key string, mode accMode) bool {
	id := armCoverKey(a)
	if err, ok := s.armBuilds[id]; ok {
		return err == nil
	}
	cov, extra := s.cov, s.extra
	s.cov = maps.Clone(cov) // the trial sets nothing
	_, _, err := s.makeValue(a, "trial", key, mode)
	s.cov, s.extra = cov, extra
	if s.armBuilds == nil {
		s.armBuilds = map[string]error{}
	}
	s.armBuilds[id] = err
	return err == nil
}

// maxStepCopies is the most copies of the resource that a step holds. A resource with a oneOf group
// at its top level (an alert has one arm for each type) sets one arm in a copy, so a step with
// copies sets many arms at once, and Terraform applies the copies together.
const maxStepCopies = 10

// sweepCopies makes the config of one step: a copy of the resource, and another while the next one
// sets a value or an arm that no config has set. A singleton has one copy, because the API holds
// one resource of it. The copies differ in their top-level strings: the name ends with the number
// of the copy.
func (s *accSynth) sweepCopies(res *tfResource) (string, error) {
	limit := maxStepCopies
	if res.CRUD.Singleton {
		limit = 1
	}
	var blocks []string
	for n := 0; n < limit; n++ {
		before := len(s.cov)
		s.extra, s.element = 0, n
		body, _, err := s.attrs(res.Attributes, "", "", accFull, true)
		s.element = 0
		if err != nil {
			return "", fmt.Errorf("acceptance test: %w", err)
		}
		if len(s.cov) == before {
			break // this copy sets nothing new
		}
		name := "test"
		if n > 0 {
			name = "test_" + strconv.Itoa(n+1)
		}
		blocks = append(blocks, namedResourceBlock("coralogix_"+res.CRUD.TypeName, name, body))
	}
	return strings.Join(blocks, "\n\n"), nil
}

// sweepSteps makes the configs that follow the full config in the values-and-choices subtest. The
// full config has set its values and arms already (s.cov).
func (s *accSynth) sweepSteps(res *tfResource) ([]*accStep, error) {
	if err := s.checkSkipLists(res.Attributes); err != nil {
		return nil, err
	}
	s.sweeping = true
	defer func() { s.sweeping = false }()
	var steps []*accStep
	for len(steps) < maxSweepSteps {
		before := len(s.cov)
		config, err := s.sweepCopies(res)
		if err != nil {
			return nil, err
		}
		if len(s.cov) == before {
			return steps, s.unmadeArms()
		}
		steps = append(steps, &accStep{N: len(steps) + 1, Config: config})
	}
	return nil, fmt.Errorf("acceptance test: the values-and-choices sweep did not end after %d steps", maxSweepSteps)
}

// checkSkipLists reports a skipValues or skipChoices entry that names nothing in the schema.
func (s *accSynth) checkSkipLists(attrs []*tfAttr) error {
	enums := map[string]map[string]bool{}
	arms := map[string]map[string]bool{}
	var walk func(attrs []*tfAttr)
	walk = func(attrs []*tfAttr) {
		for _, a := range attrs {
			if a.EnumSchema != "" {
				if enums[a.EnumSchema] == nil {
					enums[a.EnumSchema] = map[string]bool{}
				}
				for _, o := range choices(a) {
					enums[a.EnumSchema][o.API] = true
				}
			}
			if a.OneOfGroup != "" && !a.Required {
				if arms[a.Component] == nil {
					arms[a.Component] = map[string]bool{}
				}
				arms[a.Component][a.Property] = true
			}
			walk(a.Attributes)
		}
	}
	walk(attrs)
	for _, name := range sortedStringKeys(s.file.SkipValues) {
		values, ok := enums[name]
		if !ok {
			return fmt.Errorf("%s: skipValues.%s: the schema has no enum %q. The enums: %s", acceptance.FileName, name, name, strings.Join(sortedStringKeys(enums), ", "))
		}
		for _, v := range s.file.SkipValues[name].Values {
			if !values[v] {
				return fmt.Errorf("%s: skipValues.%s: the enum has no accepted value %s", acceptance.FileName, name, strconv.Quote(v))
			}
		}
	}
	for _, name := range sortedStringKeys(s.file.SkipChoices) {
		props, ok := arms[name]
		if !ok {
			return fmt.Errorf("%s: skipChoices.%s: the schema has no oneOf group in the component %q. The components with one: %s", acceptance.FileName, name, name, strings.Join(sortedStringKeys(arms), ", "))
		}
		for _, c := range s.file.SkipChoices[name].Choices {
			if !props[c] {
				return fmt.Errorf("%s: skipChoices.%s: the component has no oneOf arm %s", acceptance.FileName, name, strconv.Quote(c))
			}
		}
	}
	return nil
}

func sortedStringKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// unmadeArms reports the arms that the test cannot make a value for and that no skipChoices entry
// names. Each message says what the generator could not make.
func (s *accSynth) unmadeArms() error {
	var lines []string
	for _, id := range sortedStringKeys(s.armBuilds) {
		if err := s.armBuilds[id]; err != nil {
			_, component, property := splitArmKey(id)
			lines = append(lines, fmt.Sprintf("  %s: %s (%v)", component, property, err))
		}
	}
	if len(lines) == 0 {
		return nil
	}
	return fmt.Errorf("acceptance test: the values-and-choices subtest cannot make a value for %d oneOf arms. For each, add the arm to skipChoices in %s with a reason, or set the values that it needs in values:\n%s", len(lines), acceptance.FileName, strings.Join(lines, "\n"))
}

func splitArmKey(id string) (kind, component, property string) {
	parts := strings.SplitN(id, "\x00", 3)
	for len(parts) < 3 {
		parts = append(parts, "")
	}
	return parts[0], parts[1], parts[2]
}

// maxSweepElements is the most elements that a step puts in one list or set. A step with many
// elements covers many arms at once, and a longer list needs more from the API than the test can
// know.
const maxSweepElements = 12

// maxStepElements is the most elements after the first that a whole step adds to its lists. The
// lists of a resource can hold lists (sections, rows, widgets), so a limit for each list alone would
// multiply.
const maxStepElements = 20

// sweepElements makes the list or set of objects of a sweep step. It makes an element, and another
// while the next one sets a value or an arm that no config has set. A list of widgets then holds a
// widget of each kind in one step. The list has at most as many elements as its size validators allow.
func (s *accSynth) sweepElements(a *tfAttr, tfPath, key string, mode accMode) (string, []accCheck, error) {
	if err := checkOneElement(a); err != nil {
		return "", nil, err
	}
	limit := min(maxSweepElements, sizeCap(a))
	if n, ok := s.file.MaxElements[key]; ok {
		limit = min(limit, n)
	}
	outer := s.element
	defer func() { s.element = outer }()
	var elements []string
	var checks []accCheck
	for n := 0; n < limit; n++ {
		s.element = n
		before := len(s.cov)
		path := tfPath + "." + strconv.Itoa(n)
		if a.Kind == "SetNested" {
			path = tfPath + ".*" // the position of a set element is not known: no element checks
		}
		body, c, err := s.attrs(a.Attributes, path, key+"[]", mode, false)
		if err != nil {
			return "", nil, err
		}
		if n > 0 && (len(s.cov) == before || s.extra >= maxStepElements) {
			break // this element sets nothing new, or the step has enough elements
		}
		if n > 0 {
			s.extra++
		}
		elements = append(elements, indentLines(objectHCL(body)+","))
		if a.Kind != "SetNested" {
			checks = append(checks, c...)
		}
	}
	checks = append([]accCheck{{Path: tfPath + ".#", Value: strconv.Itoa(len(elements))}}, checks...)
	return "[\n" + strings.Join(elements, "\n") + "\n]", checks, nil
}

// sizeCap returns the most elements that the size validators of a collection allow.
func sizeCap(a *tfAttr) int {
	limit := math.MaxInt
	for _, v := range a.Validators {
		m := sizeCall.FindStringSubmatch(v)
		if m == nil {
			continue
		}
		var bounds []int
		for _, arg := range strings.Split(m[2], ",") {
			n, err := strconv.Atoi(strings.TrimSpace(arg))
			if err != nil {
				return 1
			}
			bounds = append(bounds, n)
		}
		switch {
		case m[1] == "SizeAtMost" && len(bounds) == 1:
			limit = min(limit, bounds[0])
		case m[1] == "SizeBetween" && len(bounds) == 2:
			limit = min(limit, bounds[1])
		}
	}
	return limit
}

// numbered puts the number of the element at the end of a made-up string of a sweep step, when the
// string stays inside its length limits and has no pattern to match.
func (s *accSynth) numbered(a *tfAttr, hcl, state string) (string, string) {
	if !s.sweeping || s.element == 0 || len(choices(a)) != 0 || patternOf(a) != nil || a.CustomType != "" {
		return hcl, state
	}
	_, high := lengthRange(a)
	next := state + "-" + strconv.Itoa(s.element+1)
	if high > 0 && len(next) > high {
		return hcl, state
	}
	return strconv.Quote(next), next
}
