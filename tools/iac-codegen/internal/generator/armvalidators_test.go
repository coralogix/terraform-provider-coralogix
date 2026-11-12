package generator

import (
	"strings"
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

// A sibling rule inside an object is checked by the validator of the parent object, so its cost does
// not grow with the number of objects. A rule at the top level, or one that reaches another object,
// keeps its path expression.
func TestSiblingRulesMoveToTheParentObject(t *testing.T) {
	nested := &tfAttr{Name: "definition", UpSteps: []int{2}}
	rule, ok := siblingRuleOf(nested, overrides.Validator{ExactlyOneOf: []string{"reference"}})
	if !ok || !rule.Exactly || strings.Join(rule.Names, ",") != "definition,reference" {
		t.Fatalf("exactlyOneOf in an item = %+v, %v; want a rule of definition and reference", rule, ok)
	}
	rule, ok = siblingRuleOf(&tfAttr{Name: "color", UpSteps: []int{1}}, overrides.Validator{ConflictsWith: []string{"internal"}})
	if !ok || rule.Exactly || strings.Join(rule.Names, ",") != "internal" {
		t.Fatalf("conflictsWith in an object = %+v, %v; want a conflict with internal", rule, ok)
	}
	if _, ok := siblingRuleOf(&tfAttr{Name: "layout"}, overrides.Validator{ExactlyOneOf: []string{"content_json"}}); ok {
		t.Error("a top-level attribute has no parent object validator, so it keeps the path expression")
	}
	if _, ok := siblingRuleOf(nested, overrides.Validator{ExactlyOneOf: []string{"../other"}}); ok {
		t.Error("a target in another object keeps the path expression")
	}
	if _, ok := siblingRuleOf(nested, overrides.Validator{AlsoRequires: []string{"reference"}}); ok {
		t.Error("alsoRequires keeps the path expression")
	}

	children := []*tfAttr{{Name: "definition", SiblingRules: []siblingRule{rule2("definition", "reference")}}, {Name: "reference", SiblingRules: []siblingRule{rule2("reference", "definition")}}}
	got := armsValidatorExpr(children)
	if strings.Count(got, `Arms: []string{"definition", "reference"}`) != 1 || !strings.Contains(got, "Required: true") {
		t.Errorf("validator = %s; want one required group of both arms", got)
	}
}

func rule2(names ...string) siblingRule { return siblingRule{Exactly: true, Names: names} }
