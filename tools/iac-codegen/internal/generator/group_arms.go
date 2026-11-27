package generator

import (
	"maps"
	"slices"

	"github.com/coralogix/coralogix-management-sdk/iac/model"
	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen/internal/overrides"
)

// extendGroupArms puts a new arm of a oneOf in the group that holds the other arms. A types.<T>.groups
// line lists the arms that the released resource has: frequency holds daily, weekly, and monthly.
// When the API adds an arm, the line cannot name it, so the arm joins the group that holds two or
// more arms of its oneOf. An arm that a line names (a field line or unsupportedArms), or that
// another group holds, stays where the lines put it. A group that holds one arm of a oneOf stands
// for that arm, so it takes no other.
func extendGroupArms(resource *model.Resource, file *overrides.File) {
	if file == nil || len(file.Types) == 0 {
		return
	}
	seen := map[*model.Type]bool{}
	var walk func(t *model.Type)
	walk = func(t *model.Type) {
		if t == nil || seen[t] {
			return
		}
		seen[t] = true
		switch t.Kind {
		case model.Object:
			extendTypeGroups(t, file)
			for _, f := range t.Fields {
				walk(f.Type)
			}
		case model.OneOf:
			for _, f := range t.Fields {
				walk(f.Type)
			}
		case model.List, model.Set, model.Map:
			walk(t.Elem)
		}
	}
	for _, f := range resource.Fields {
		walk(f.Type)
	}
}

func extendTypeGroups(t *model.Type, file *overrides.File) {
	line, ok := file.Types[t.Schema]
	if !ok || len(line.Groups) == 0 {
		return
	}
	for _, o := range t.Groups {
		holder := armHolder(line, o.Arms)
		if holder == "" {
			continue
		}
		for _, arm := range o.Arms {
			if _, named := line.Fields[arm]; named || line.GroupOf(arm) != "" || slices.Contains(line.UnsupportedArms, arm) {
				continue
			}
			g := line.Groups[holder]
			g.Fields = append(slices.Clone(g.Fields), arm)
			line.Groups[holder] = g
		}
	}
}

// armHolder returns the group that holds two or more of the arms, or "".
func armHolder(line overrides.Type, arms []string) string {
	for _, name := range slices.Sorted(maps.Keys(line.Groups)) {
		held := 0
		for _, arm := range arms {
			if slices.Contains(line.Groups[name].Fields, arm) {
				held++
			}
		}
		if held >= 2 {
			return name
		}
	}
	return ""
}
