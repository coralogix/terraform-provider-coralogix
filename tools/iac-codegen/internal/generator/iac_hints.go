package generator

import (
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

// The knobs of openapiv3_field.iac (x-coralogix-iac) change only the generated acceptance test, never
// the Terraform schema. The iac module reads them into model.IaCHints. A line of the acceptance file
// for the same path always wins over a knob.
//
//	examples        the made-up value of the field; [1] is the update value (exampleOfHints)
//	required_by_api the minimal config sets the field (include)
//	conflicts_with  no config sets both fields; the sweep sets each in a config of its own (conflictLosers)
//	requires        a config that sets the field also sets the required sibling (forcedByRequires)
//	arm_fixed       an update cannot switch to or from the arm: a sweep step uses a new resource (sweepCopies)

// findIaCHints returns the knobs of the fields of the resource, keyed "<component>.<property>" like the
// examples (findExamples): the component of the field in the resource response. A field has the hints
// of the response, the Create request, or the Update request (Field.IaC), the first that has any.
func findIaCHints(r *model.Resource) map[string]*model.IaCHints {
	out := map[string]*model.IaCHints{}
	seen := map[*model.Type]bool{}
	var walk func(t *model.Type)
	walk = func(t *model.Type) {
		if t == nil || seen[t] {
			return
		}
		seen[t] = true
		walk(t.Elem)
		for _, f := range t.Fields {
			if h := f.IaC(); h != nil && t.Schema != "" {
				out[t.Schema+"."+f.Name] = h
			}
			walk(f.Type)
		}
	}
	for _, f := range r.Fields {
		if h := f.IaC(); h != nil {
			out[r.Name+"."+f.Name] = h
		}
		walk(f.Type)
	}
	return out
}

// markIaC gives each attribute the knobs of its API field. A knob example replaces the example of the
// contract. It runs after markExamples, so it wins.
func markIaC(attrs []*tfAttr, hints map[string]*model.IaCHints, file *overrides.File) {
	if len(hints) == 0 {
		return
	}
	for _, a := range attrs {
		if h, ok := hints[contractField(a, file)]; ok && !a.Extra {
			a.IaC = h
			if e := exampleOfHints(h.Examples); e != nil {
				a.Example = e
			}
		}
		markIaC(a.Attributes, hints, file)
	}
}

// exampleOfHints turns the `examples` knob into the example that the acceptance test uses. A scalar
// or an array of scalars is an example. An object (a map example) is not: mapExampleKey reads it.
// nil when the knob has no example, or one that is not a scalar or an array of scalars.
func exampleOfHints(values []any) *contractExample {
	var items []exampleItem
	for _, v := range values {
		switch v := v.(type) {
		case []any:
			item := exampleItem{Array: true}
			for _, e := range v {
				ev, ok := exampleScalar(e)
				if !ok {
					return nil
				}
				item.Values = append(item.Values, ev)
			}
			items = append(items, item)
		default:
			ev, ok := exampleScalar(v)
			if !ok {
				return nil
			}
			items = append(items, exampleItem{Values: []exampleValue{ev}})
		}
	}
	if len(items) == 0 {
		return nil
	}
	return &contractExample{Items: items}
}

// exampleScalar writes a JSON scalar as the text and the YAML tag that the examples of the contract use.
func exampleScalar(v any) (exampleValue, bool) {
	switch v := v.(type) {
	case string:
		return exampleValue{Text: v, Tag: "!!str"}, true
	case bool:
		return exampleValue{Text: strconv.FormatBool(v), Tag: "!!bool"}, true
	case int:
		return exampleValue{Text: strconv.Itoa(v), Tag: "!!int"}, true
	case uint64:
		return exampleValue{Text: strconv.FormatUint(v, 10), Tag: "!!int"}, true
	case float64:
		return exampleValue{Text: strconv.FormatFloat(v, 'g', -1, 64), Tag: "!!float"}, true
	}
	return exampleValue{}, false
}

var hclIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

// mapExampleKey returns the key that the config of the mode gives the one entry of a map, and its HCL
// form. The key is the first key of the object in `examples` ([1] for the updated config when it
// exists). Without such an example, or with a key that cannot be a plain path step, it is "key".
func mapExampleKey(a *tfAttr, mode accMode) (key, hcl string) {
	key, hcl = "key", "key"
	if a.IaC == nil || len(a.IaC.Examples) == 0 {
		return
	}
	example := a.IaC.Examples[0]
	if mode == accUpdated && len(a.IaC.Examples) > 1 {
		example = a.IaC.Examples[1]
	}
	object, ok := example.(map[string]any)
	if !ok || len(object) == 0 {
		return
	}
	first := slices.Sorted(maps.Keys(object))[0]
	if first == "" || strings.ContainsAny(first, ".*[]\"\\$%{}") {
		return
	}
	if hclIdentifier.MatchString(first) {
		return first, first
	}
	return first, strconv.Quote(first)
}

// requiredBy returns the sibling attribute of the hints' target: the attribute of the object whose
// API field (Property) is the name.
func sibling(attrs []*tfAttr, property string) *tfAttr {
	for _, a := range attrs {
		if a.Property == property {
			return a
		}
	}
	return nil
}

// conflictGroups returns, for each attribute of the object that has a conflicts_with knob, the
// attributes that it conflicts with. A knob names the other field, so the pair is symmetric.
func conflictPeers(attrs []*tfAttr) map[*tfAttr][]*tfAttr {
	peers := map[*tfAttr][]*tfAttr{}
	for _, a := range attrs {
		if a.IaC == nil {
			continue
		}
		for _, name := range a.IaC.ConflictsWith {
			b := sibling(attrs, name)
			if b == nil || b == a {
				continue
			}
			peers[a] = append(peers[a], b)
			peers[b] = append(peers[b], a)
		}
	}
	return peers
}

const conflictWeight = 1000

func conflictCoverKey(key string) string { return "conflict\x00" + key }

// conflictLosers returns the attributes that the config leaves out because a field that they conflict
// with is set. Of a group of fields that conflict, one is set. The full config takes the first that
// it includes. A step of the sweep takes the field that gives the most gain, and the field that no
// config has set yet comes first, so each field is in a config of its own.
func (s *accSynth) conflictLosers(attrs []*tfAttr, key string, mode accMode, top bool, selected map[*tfAttr]bool) map[*tfAttr]bool {
	peers := conflictPeers(attrs)
	if len(peers) == 0 {
		return nil
	}
	losers := map[*tfAttr]bool{}
	for _, group := range conflictComponents(attrs, peers) {
		// Only a field that the config sets can win or lose. An arm that the group of its oneOf does not
		// take is not set, so it must not hold back a field that it conflicts with.
		var candidates []*tfAttr
		requiredIn := false
		for _, a := range group {
			switch {
			case !selected[a]:
			case a.Required:
				requiredIn = true // a required field is always set: the fields that conflict with it never are
			default:
				candidates = append(candidates, a)
			}
		}
		if requiredIn {
			for _, a := range candidates {
				losers[a] = true
				s.excuse(joinPath(key, a.Name)) // no config can set it: it always conflicts with a required field
			}
			continue
		}
		if len(candidates) < 2 {
			continue
		}
		winner := candidates[0]
		if s.sweeping {
			// A field with nothing new to set leaves the step: a config may set none of the group,
			// and a field that is always set would hold back the enum values that its requires fix.
			winner = nil
			best := 0
			for _, a := range candidates {
				g := s.gain(a, joinPath(key, a.Name))
				if !s.cov[conflictCoverKey(joinPath(key, a.Name))] {
					g += conflictWeight
				}
				if g > best {
					best, winner = g, a
				}
			}
		}
		for _, a := range candidates {
			if a != winner {
				losers[a] = true
			}
		}
		if winner != nil && s.cov != nil && s.trial == 0 {
			s.cov[conflictCoverKey(joinPath(key, winner.Name))] = true
		}
	}
	return losers
}

// excuse says that no config can set the attribute, so checkAllSet does not ask for one.
func (s *accSynth) excuse(path string) {
	if s.excused != nil {
		s.excused[path] = true
	}
}

// forcedConflicts adds to losers the fields that conflict with a field that a requires knob forces:
// the requirement wins, because the API needs the forced field. It returns the fields that it added.
// The caller then collects the requirements again, since a loser no longer requires anything.
func forcedConflicts(attrs []*tfAttr, forced map[*tfAttr]requiredValue, losers map[*tfAttr]bool) []*tfAttr {
	var added []*tfAttr
	peers := conflictPeers(attrs)
	for _, a := range attrs { // in the order of the object, so the result does not depend on the map
		if _, isForced := forced[a]; !isForced {
			continue
		}
		for _, p := range peers[a] {
			if _, alsoForced := forced[p]; !p.Required && !losers[p] && !alsoForced {
				losers[p] = true
				added = append(added, p)
			}
		}
	}
	return added
}

// conflictComponents groups the attributes that conflict, directly or through another, in the order
// of the object.
func conflictComponents(attrs []*tfAttr, peers map[*tfAttr][]*tfAttr) [][]*tfAttr {
	seen := map[*tfAttr]bool{}
	var groups [][]*tfAttr
	for _, a := range attrs {
		if seen[a] || len(peers[a]) == 0 {
			continue
		}
		member := map[*tfAttr]bool{}
		var walk func(x *tfAttr)
		walk = func(x *tfAttr) {
			if member[x] {
				return
			}
			member[x], seen[x] = true, true
			for _, p := range peers[x] {
				walk(p)
			}
		}
		walk(a)
		var group []*tfAttr
		for _, x := range attrs {
			if member[x] {
				group = append(group, x)
			}
		}
		groups = append(groups, group)
	}
	return groups
}

// requiredValue is the attribute that a requires knob forces into the config, with the value it
// must have ("" Equals: any value).
type requiredValue struct {
	Equals    any
	HasEquals bool
}

// forcedByRequires returns the attributes of the object that a config sets because an included
// attribute requires them. Only one step is followed: a requirement of a forced attribute does not add
// another.
func (s *accSynth) forcedByRequires(attrs []*tfAttr, key string, mode accMode, top bool, losers map[*tfAttr]bool, picks map[string]*tfAttr) map[*tfAttr]requiredValue {
	var forced map[*tfAttr]requiredValue
	emitted := s.emittedAttrs(attrs, key, mode, top, losers, picks)
	for _, a := range attrs {
		if a.IaC == nil || len(a.IaC.Requires) == 0 || !emitted[a] {
			continue
		}
		for _, r := range a.IaC.Requires {
			b := sibling(attrs, r.Field)
			if b == nil || b == a || (!b.Required && !b.Optional) || refused(b) {
				continue
			}
			if b.OneOfGroup != "" && !b.Required && !emitted[b] {
				continue // an arm that the group does not take cannot be forced: the group has its arm already
			}
			if forced == nil {
				forced = map[*tfAttr]requiredValue{}
			}
			forced[b] = requiredValue{Equals: r.Equals, HasEquals: r.HasEquals}
		}
	}
	return forced
}

// emittedAttrs returns the attributes that the config of the object sets by its own rules, before the
// attributes that a requires knob forces: an included attribute that no conflict removes, and, of a
// oneOf group, only the arm that the config takes (the same choice as takeArm, without its effects). A
// requires knob of an arm that the config leaves out must not force anything.
func (s *accSynth) emittedAttrs(attrs []*tfAttr, key string, mode accMode, top bool, losers map[*tfAttr]bool, picks map[string]*tfAttr) map[*tfAttr]bool {
	armTaken := s.requiredArms(attrs, key, mode, top)
	out := map[*tfAttr]bool{}
	for _, a := range typedFirst(attrs) {
		if losers[a] || !s.include(a, joinPath(key, a.Name), mode, top) || s.sideOff(a, joinPath(key, a.Name)) {
			continue
		}
		if a.OneOfGroup != "" && !a.Required {
			if refused(a) || armTaken[a.OneOfGroup] {
				continue
			}
			if pick, ok := picks[a.OneOfGroup]; ok && pick != a {
				continue
			}
			armTaken[a.OneOfGroup] = true
		}
		out[a] = true
	}
	return out
}

// forcedLiteral writes the value of a requires knob for the attribute: the HCL, and the state text.
// An enum takes the Terraform value of the API value. ok is false when the attribute has no plain
// literal for the value; the config then makes up a value as for any other attribute.
func forcedLiteral(a *tfAttr, r requiredValue) (hcl, state string, ok bool) {
	if !r.HasEquals {
		return "", "", false
	}
	if len(choices(a)) != 0 {
		if api, isString := r.Equals.(string); isString {
			for _, o := range choices(a) {
				if o.API == api {
					return strconv.Quote(o.TF), o.TF, true
				}
			}
		}
		return "", "", false
	}
	v, ok := exampleScalar(r.Equals)
	if !ok {
		return "", "", false
	}
	return exampleLiteral(a.Kind, v)
}

// sweepsFixedArm says that the attribute is an arm that an update cannot switch to or from.
func fixedArm(a *tfAttr) bool { return a.IaC != nil && a.IaC.ArmFixed }

func (s *accSynth) noteFixedArm(a *tfAttr) {
	if fixedArm(a) && s.trial == 0 {
		s.usedFixedArm = true
	}
}

// copyNumber returns the number of the copy n (from 0) of a sweep step. A copy that sets a fixed arm,
// or that followed one, takes a number that no other step uses, so Terraform creates a new resource
// instead of an update that switches the arm.
func (s *accSynth) copyNumber(n int, fixedNow bool) int {
	if fixedNow || s.prevFixed[n] {
		return n + 1 + 10*s.stepNo
	}
	return n + 1
}
