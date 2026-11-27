package generator

import (
	"slices"
	"testing"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

func groupArmsFile(types map[string]overrides.Type) *overrides.File {
	return &overrides.File{Types: types}
}

// A new arm of a oneOf joins the group that holds the other arms, so a new arm needs no line.
func TestNewOneOfArmJoinsTheGroupThatHoldsTheOtherArms(t *testing.T) {
	typ := &model.Type{Kind: model.Object, Schema: "Dynamic", Groups: []model.OneOfGroup{{Arms: []string{"daily", "weekly", "monthly", "yearly"}}}}
	file := groupArmsFile(map[string]overrides.Type{"Dynamic": {Groups: map[string]overrides.Group{"frequency": {Fields: []string{"daily", "weekly", "monthly"}}}}})
	extendTypeGroups(typ, file)
	if got, want := file.Types["Dynamic"].Groups["frequency"].Fields, []string{"daily", "weekly", "monthly", "yearly"}; !slices.Equal(got, want) {
		t.Fatalf("frequency = %v, want %v", got, want)
	}
}

// A group that holds one arm stands for that arm. An arm that a line names, or that another group
// holds, stays where the lines put it.
func TestNewOneOfArmStaysOutOfAGroupThatTheLinesDoNotMean(t *testing.T) {
	typ := &model.Type{Kind: model.Object, Schema: "Window", Groups: []model.OneOfGroup{{Arms: []string{"absolute", "relative", "other"}}}}
	file := groupArmsFile(map[string]overrides.Type{"Window": {Groups: map[string]overrides.Group{"relative": {Fields: []string{"relative"}, Optional: true}}}})
	extendTypeGroups(typ, file)
	if got := file.Types["Window"].Groups["relative"].Fields; !slices.Equal(got, []string{"relative"}) {
		t.Fatalf("a group of one arm took another arm: %v", got)
	}

	typ = &model.Type{Kind: model.Object, Schema: "Dynamic", Groups: []model.OneOfGroup{{Arms: []string{"a", "b", "c", "d", "e"}}}}
	file = groupArmsFile(map[string]overrides.Type{"Dynamic": {
		Fields: map[string]overrides.Field{"c": {}},
		Groups: map[string]overrides.Group{"both": {Fields: []string{"a", "b"}}, "alone": {Fields: []string{"d"}}},
	}})
	extendTypeGroups(typ, file)
	if got, want := file.Types["Dynamic"].Groups["both"].Fields, []string{"a", "b", "e"}; !slices.Equal(got, want) {
		t.Fatalf("both = %v, want %v: c has a line and d has a group", got, want)
	}
}
