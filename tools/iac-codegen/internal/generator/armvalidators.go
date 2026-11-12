package generator

import (
	"fmt"
	"slices"
	"strings"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

// siblingRule is an exactlyOneOf or a conflictsWith line that names only the siblings of the
// attribute in its object.
type siblingRule struct {
	Exactly bool     // exactlyOneOf: exactly one of Names is set; else the attribute conflicts with Names
	Names   []string // the Terraform names: for exactlyOneOf the attribute and its siblings
}

// siblingRuleOf returns the rule of a validator line when it names only siblings of an attribute that
// is inside an object. A name with "../" reaches another object, and an attribute at the top level has
// no object validator, so those keep the path expression.
func siblingRuleOf(a *tfAttr, v overrides.Validator) (siblingRule, bool) {
	if len(a.UpSteps) == 0 {
		return siblingRule{}, false
	}
	targets := v.ExactlyOneOf
	exactly := true
	if len(targets) == 0 {
		targets, exactly = v.ConflictsWith, false
	}
	if len(targets) == 0 || len(v.ExactlyOneOf) != 0 && len(v.ConflictsWith) != 0 {
		return siblingRule{}, false
	}
	for _, t := range targets {
		if ups, _ := splitTarget(t); ups != 0 {
			return siblingRule{}, false
		}
	}
	if exactly {
		return siblingRule{Exactly: true, Names: append([]string{a.Name}, targets...)}, true
	}
	return siblingRule{Names: slices.Clone(targets)}, true
}

// oneOfArmsValidatorName is the name of the validator type that the generated schema file defines for
// the oneOf groups of an object.
const oneOfArmsValidatorName = "oneOfArmsValidator"

// addArmValidators puts the checks of the oneOf groups that the attributes in attrs hold in one validator
// on each object attribute: the object of a single nested attribute, or of the items of a list, set, or
// map. The validator reads the attributes of the object it is on. A validator on each arm that names
// its siblings by a path expression walks the whole configuration for every sibling, so its cost grows
// with the number of objects in the configuration. A computed-only object has no validators.
func addArmValidators(attrs []*tfAttr) {
	for _, a := range attrs {
		if len(a.Attributes) == 0 {
			continue
		}
		addArmValidators(a.Attributes)
		if computedOnly(a) {
			continue
		}
		expr := armsValidatorExpr(a.Attributes)
		switch {
		case expr == "":
		case a.Kind == "SingleNested":
			a.Validators = append(a.Validators, expr)
		default:
			a.ItemValidators = append(a.ItemValidators, expr)
		}
	}
}

// armsValidatorExpr is the Go expression of the oneOfArmsValidator of an object with the attributes
// children, or "" when no attribute is the arm of a group and none has a conflict. An arm of a
// top-level oneOf (RootArm) is checked by a resource validator, not here.
func armsValidatorExpr(children []*tfAttr) string {
	var order []string
	arms := map[string][]string{}
	required := map[string]bool{}
	var conflictOrder []string
	conflicts := map[string][]string{}
	conflictWith := map[string][]string{}
	for _, c := range children {
		if c.OneOfGroup != "" && !c.RootArm {
			if _, ok := arms[c.OneOfGroup]; !ok {
				order = append(order, c.OneOfGroup)
			}
			arms[c.OneOfGroup] = append(arms[c.OneOfGroup], c.Name)
			required[c.OneOfGroup] = required[c.OneOfGroup] || c.OneOfRequired
		}
		if len(c.ArmConflicts) > 0 {
			key := strings.Join(c.ArmConflicts, ",")
			if _, ok := conflicts[key]; !ok {
				conflictOrder = append(conflictOrder, key)
				conflictWith[key] = c.ArmConflicts
			}
			conflicts[key] = append(conflicts[key], c.Name)
		}
	}
	for _, c := range children {
		for _, rule := range c.SiblingRules {
			if rule.Exactly {
				names := slices.Sorted(slices.Values(rule.Names))
				key := "rule:" + strings.Join(names, ",")
				if _, ok := arms[key]; !ok {
					order = append(order, key)
					arms[key] = names
					required[key] = true
				}
				continue
			}
			key := c.Name + "!" + strings.Join(rule.Names, ",")
			conflictOrder = append(conflictOrder, key)
			conflicts[key] = []string{c.Name}
			conflictWith[key] = rule.Names
		}
	}
	if len(order) == 0 && len(conflictOrder) == 0 {
		return ""
	}
	out := oneOfArmsValidatorName + "{"
	var parts []string
	if len(order) > 0 {
		var groups []string
		for _, key := range order {
			groups = append(groups, fmt.Sprintf("{Arms: %s, Required: %t}", goStrings(arms[key]), required[key]))
		}
		parts = append(parts, "Groups: []oneOfArmGroup{"+strings.Join(groups, ", ")+"}")
	}
	if len(conflictOrder) > 0 {
		var list []string
		for _, key := range conflictOrder {
			list = append(list, fmt.Sprintf("{Left: %s, Right: %s}", goStrings(conflicts[key]), goStrings(conflictWith[key])))
		}
		parts = append(parts, "Conflicts: []oneOfArmConflict{"+strings.Join(list, ", ")+"}")
	}
	return out + strings.Join(parts, ", ") + "}"
}

// goStrings is the Go literal of a list of strings.
func goStrings(names []string) string {
	quoted := make([]string, 0, len(names))
	for _, n := range names {
		quoted = append(quoted, fmt.Sprintf("%q", n))
	}
	return "[]string{" + strings.Join(quoted, ", ") + "}"
}

// HasArmValidators reports whether an attribute has the oneOfArmsValidator, which the schema file then
// defines.
func (r *tfResource) HasArmValidators() bool {
	var uses func(attrs []*tfAttr) bool
	uses = func(attrs []*tfAttr) bool {
		for _, a := range attrs {
			for _, list := range [][]string{a.Validators, a.ItemValidators} {
				for _, v := range list {
					if strings.HasPrefix(v, oneOfArmsValidatorName+"{") {
						return true
					}
				}
			}
			if uses(a.Attributes) {
				return true
			}
		}
		return false
	}
	return uses(r.Attributes)
}

// siblingDefaultModifierName is the name of the plan modifier type that the generated schema file
// defines for the attributes with a defaultFromSibling line.
const siblingDefaultModifierName = "siblingDefaultModifier"

// HasSiblingDefaults reports whether an attribute has the siblingDefaultModifier, which the schema file
// then defines.
func (r *tfResource) HasSiblingDefaults() bool {
	var uses func(attrs []*tfAttr) bool
	uses = func(attrs []*tfAttr) bool {
		for _, a := range attrs {
			for _, m := range a.Modifiers {
				if strings.HasPrefix(m, siblingDefaultModifierName+"{") {
					return true
				}
			}
			if uses(a.Attributes) {
				return true
			}
		}
		return false
	}
	return uses(r.Attributes)
}

// unsupportedBoolValidatorName is the name of the validator type that the generated schema file defines
// for a bool that the API never accepts at its place (the unsupported validator).
const unsupportedBoolValidatorName = "unsupportedBoolValidator"

// HasUnsupportedValidators reports whether an attribute has the unsupportedBoolValidator, which the
// schema file then defines.
func (r *tfResource) HasUnsupportedValidators() bool {
	var uses func(attrs []*tfAttr) bool
	uses = func(attrs []*tfAttr) bool {
		for _, a := range attrs {
			for _, v := range a.Validators {
				if strings.HasPrefix(v, unsupportedBoolValidatorName+"{") {
					return true
				}
			}
			if uses(a.Attributes) {
				return true
			}
		}
		return false
	}
	return uses(r.Attributes)
}
