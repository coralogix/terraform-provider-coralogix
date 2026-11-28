package generator

import (
	"slices"
	"strconv"
)

// A carrySiblings fold shows two twin arms as one object and a flag. A field that only one twin
// has applies only while the flag selects that twin: the generated expand refuses it otherwise
// ("Attribute does not fit the flag"). So a generated config sets, in such an object, the fields
// of one twin: the shown twin (flag true) in every config, and the folded twin (flag false) in the
// steps of the values-and-choices sweep, which plans the side by what no config has set yet.

// foldSide is the flag value that allows a field that only one twin of a fold has.
type foldSide struct {
	Flag string // Terraform name of the flag attribute
	When bool   // the flag value that allows the field: true for the shown twin, false for the folded one
}

// foldChoice is the side of a fold that the config sets in one object.
type foldChoice struct{ value bool }

// foldKey is the key under which the sweep records that a config set the folded side at a path.
func foldKey(key string) string { return "fold\x00" + key }

// foldFlagOf returns the flag of a fold that the attributes hold, when a field of only one twin sits
// beside it. An object whose twins have the same fields has no side to choose, and its flag is an
// ordinary attribute.
func foldFlagOf(attrs []*tfAttr) *tfAttr {
	if !slices.ContainsFunc(attrs, func(a *tfAttr) bool { return a.FoldSide != nil }) {
		return nil
	}
	i := slices.IndexFunc(attrs, func(a *tfAttr) bool { return a.FoldFlag })
	if i < 0 {
		return nil
	}
	return attrs[i]
}

// chooseFold picks the side of the fold that the object at key sets, and remembers it for the
// attributes of the object (s.folds). It returns nil for an object without a side to choose. Every
// config takes the shown side, except a step of the sweep that the folded side gains more in. A
// flag that the acceptance file fixes to a literal fixes the side; a flag that it fixes to an
// expression leaves the object as it is.
func (s *accSynth) chooseFold(attrs []*tfAttr, key string, mode accMode) *foldChoice {
	flag := foldFlagOf(attrs)
	if flag == nil {
		return nil
	}
	var choice *foldChoice
	if v, _, fixed := s.fileValue(joinPath(key, flag.Name)); fixed {
		if state, ok := literalState(flag, v); ok {
			choice = &foldChoice{value: state == "true"}
		}
	} else {
		choice = &foldChoice{value: !(s.sweeping && mode != accMinimal && s.sideGain(attrs, key, false) > s.sideGain(attrs, key, true))}
	}
	if choice != nil && !choice.value && s.cov != nil {
		s.cov[foldKey(key)] = true
	}
	if s.folds == nil {
		s.folds = map[string]*foldChoice{}
	}
	s.folds[key] = choice
	return choice
}

// sideGain counts what the sweep sets new on one side of the fold at key: the enum values and the
// arms below the fields of that side, and, for the folded side, the side itself.
func (s *accSynth) sideGain(attrs []*tfAttr, key string, side bool) int {
	g := 0
	for _, a := range attrs {
		if a.FoldSide != nil && a.FoldSide.When == side {
			g += s.gain(a, joinPath(key, a.Name))
		}
	}
	if !side && !s.cov[foldKey(key)] {
		g++
	}
	return g
}

// foldGain is what the sweep sets new in the fold that the attributes hold, as an attribute below
// the one that gain counts: the folded side, when no config has set it.
func (s *accSynth) foldGain(attrs []*tfAttr, key string) int {
	if foldFlagOf(attrs) == nil || s.cov[foldKey(key)] {
		return 0
	}
	return 1
}

// sideOff reports whether the config leaves the attribute out because it belongs to the twin that
// the flag of its object does not select.
func (s *accSynth) sideOff(a *tfAttr, key string) bool {
	if a.FoldSide == nil {
		return false
	}
	c := s.folds[parentKey(a, key)]
	return c != nil && c.value != a.FoldSide.When
}

// attrValue makes the value of an attribute of an object. The flag of a fold takes the side that
// the object sets.
func (s *accSynth) attrValue(a *tfAttr, tfPath, key string, mode accMode, need requiredValue, forced bool, choice *foldChoice) (string, []accCheck, error) {
	if _, _, fixed := s.fileValue(key); choice != nil && a.FoldFlag && !fixed {
		v := strconv.FormatBool(choice.value)
		return v, []accCheck{{Path: tfPath, Value: v}}, nil
	}
	return s.valueOrRequired(a, tfPath, key, mode, need, forced)
}

// forcedIn reports whether a requires knob of an included field puts the attribute in the config:
// not when the file skips it, the released provider lacks it, it is extra, or it belongs to the twin
// that the flag does not select.
func (s *accSynth) forcedIn(a *tfAttr, key string, forced bool) bool {
	return forced && !slices.Contains(s.file.Skip, key) && s.inRelease(key) && !a.Extra && !s.sideOff(a, key)
}
