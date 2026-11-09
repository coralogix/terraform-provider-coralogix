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
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	dashboardschema "github.com/coralogix/terraform-provider-coralogix/internal/provider/dashboards/dashboard_schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// The base configs of the schema walk come from the schema, not from a hand list. A generator
// fills every configurable attribute with a valid value. A group of attributes of which exactly one
// may be set (a "one of" group) gets one member per variant, so the variants together reach every
// member. A path that no variant reaches fails the coverage test with its name.

var (
	exactlyOnePattern = regexp.MustCompile(`exactly one of \[([^\]]+)\] must be configured`)
	atMostOnePattern  = regexp.MustCompile(`at most one of these attributes may be configured: (.+)`)
	atLeastOnePattern = regexp.MustCompile("At least one of these attributes must be configured: (.+)\\.")
	conflictsPattern  = regexp.MustCompile(`these are not set: "\[([^\]]+)\]"`)
	oneOfPattern      = regexp.MustCompile(`value must be one of: \[([^\]]+)\]`)
	betweenPattern    = regexp.MustCompile(`value must be between (-?[0-9.e+]+) and (-?[0-9.e+]+)`)
	atLeastPattern    = regexp.MustCompile(`value must be at least (-?[0-9.]+)`)
	quotedPattern     = regexp.MustCompile(`"([^"]*)"`)
)

// dashboardGen builds one config. pin forces the arm of a one-of group at a path.
type dashboardGen struct {
	ctx     context.Context
	variant int
	pin     map[string]string
	// reached lists the paths that the config holds. groups lists the arms of each one-of group.
	reached map[string]bool
	groups  map[string][]string
	// arms lists the arm that each one-of group took.
	arms map[string]string
	// armsSeen lists every arm that a group took, over all the items of a list.
	armsSeen map[string]map[string]bool
	// visits counts how often each group was generated across the variants of a base. A group
	// takes its arms in turn by visit, so a nested group does not repeat with its parent.
	visits map[string]int
	// elements is the number of items in a list at the top of the schema. More items reach the
	// nested one-of groups of variables and annotations in fewer variants.
	elements int
}

func newDashboardGen(variant int, pin map[string]string, visits map[string]int) *dashboardGen {
	if visits == nil {
		visits = map[string]int{}
	}
	return &dashboardGen{
		ctx: context.Background(), variant: variant, pin: pin, visits: visits,
		reached: map[string]bool{}, groups: map[string][]string{}, arms: map[string]string{}, armsSeen: map[string]map[string]bool{},
	}
}

func joinPath(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}

func isConfigurable(a schema.Attribute) bool {
	return a.IsRequired() || a.IsOptional()
}

func stringValidatorDescriptions(ctx context.Context, vs []validator.String) []string {
	var out []string
	for _, v := range vs {
		out = append(out, v.Description(ctx))
	}
	return out
}

var undeclaredGroups = [][]string{
	{"preset", "color_range"},
	{"size", "color_range"},
	{"field", "regex", "field_type"},
}

// objectGroup is a set of attributes of which at most one (or exactly one) is set. A conflict
// group comes from a ConflictsWith validator: its winner is the member that a one-of group chose.
type objectGroup struct {
	members  []string
	conflict bool
}

func (g *dashboardGen) groupsOf(attrs map[string]schema.Attribute, path string, descriptions []string) []objectGroup {
	var out []objectGroup
	for _, d := range descriptions {
		switch {
		case exactlyOnePattern.MatchString(d):
			out = append(out, objectGroup{members: strings.Fields(exactlyOnePattern.FindStringSubmatch(d)[1])})
		case atMostOnePattern.MatchString(d):
			out = append(out, objectGroup{members: splitNames(atMostOnePattern.FindStringSubmatch(d)[1])})
		}
	}
	// These sets are one-of groups of the API that the schema does not validate. The provider sends
	// both arms when both are set, and the API rejects the request.
	for _, set := range undeclaredGroups {
		if slices.ContainsFunc(set, func(m string) bool { _, ok := attrs[m]; return !ok }) {
			continue
		}
		out = append(out, objectGroup{members: set})
	}
	names := make([]string, 0, len(attrs))
	for name := range attrs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if h, ok := dashboardHint(joinPath(path, name), name); ok && h == skipAttribute {
			continue
		}
		for _, d := range attributeDescriptions(g.ctx, attrs[name]) {
			if m := conflictsPattern.FindStringSubmatch(d); m != nil {
				members := []string{name}
				for _, other := range strings.Split(m[1], ",") {
					members = append(members, strings.TrimPrefix(strings.TrimSpace(other), "<."))
				}
				out = append(out, objectGroup{members: members, conflict: true})
			}
		}
	}
	return out
}

func splitNames(list string) []string {
	var out []string
	for _, n := range strings.Split(list, ",") {
		n = strings.Trim(strings.TrimSpace(n), "`.")
		if n != "" {
			out = append(out, n)
		}
	}
	return out
}

func objectDescriptions(ctx context.Context, a schema.Attribute) []string {
	var out []string
	switch x := a.(type) {
	case schema.SingleNestedAttribute:
		for _, v := range x.Validators {
			out = append(out, v.Description(ctx))
		}
	case schema.ListNestedAttribute:
		for _, v := range x.NestedObject.Validators {
			out = append(out, v.Description(ctx))
		}
	case schema.SetNestedAttribute:
		for _, v := range x.NestedObject.Validators {
			out = append(out, v.Description(ctx))
		}
	}
	return out
}

// attributeDescriptions returns the descriptions of every validator of the attribute itself.
func attributeDescriptions(ctx context.Context, a schema.Attribute) []string {
	switch x := a.(type) {
	case schema.StringAttribute:
		return stringValidatorDescriptions(ctx, x.Validators)
	case schema.Int64Attribute:
		var out []string
		for _, v := range x.Validators {
			out = append(out, v.Description(ctx))
		}
		return out
	case schema.Float64Attribute:
		var out []string
		for _, v := range x.Validators {
			out = append(out, v.Description(ctx))
		}
		return out
	case schema.BoolAttribute:
		var out []string
		for _, v := range x.Validators {
			out = append(out, v.Description(ctx))
		}
		return out
	case schema.ListAttribute:
		var out []string
		for _, v := range x.Validators {
			out = append(out, v.Description(ctx))
		}
		return out
	case schema.SingleNestedAttribute:
		var out []string
		for _, v := range x.Validators {
			out = append(out, v.Description(ctx))
		}
		return out
	}
	return nil
}

// object fills the attributes of one object.
func (g *dashboardGen) object(attrs map[string]schema.Attribute, path string, descriptions []string) map[string]any {
	excluded := g.resolveGroups(attrs, path, descriptions)
	out := map[string]any{}
	names := make([]string, 0, len(attrs))
	for name := range attrs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		a := attrs[name]
		if excluded[name] || !isConfigurable(a) {
			continue
		}
		child := joinPath(path, name)
		if v, ok := g.attribute(a, child, name); ok {
			out[name] = v
			g.reached[child] = true
		}
	}
	fixObject(out)
	g.fixLogsAggregation(out)
	return out
}

// resolveGroups decides which members of the groups are left out. A one-of group keeps one member,
// chosen by the variant or pinned. A conflict group keeps the member that a one-of group kept.
func (g *dashboardGen) resolveGroups(attrs map[string]schema.Attribute, path string, descriptions []string) map[string]bool {
	excluded, picked := map[string]bool{}, map[string]bool{}
	groups := g.groupsOf(attrs, path, descriptions)
	for _, conflictPass := range []bool{false, true} {
		for _, group := range groups {
			if group.conflict != conflictPass {
				continue
			}
			members := slices.DeleteFunc(slices.Clone(group.members), func(m string) bool { _, ok := attrs[m]; return !ok })
			if len(members) == 0 {
				continue
			}
			sort.Strings(members)
			key := joinPath(path, "{"+strings.Join(members, "|")+"}")
			g.groups[key] = members
			pick := g.pickMember(path, members, picked, excluded, conflictPass)
			g.arms[key] = pick
			if g.armsSeen[key] == nil {
				g.armsSeen[key] = map[string]bool{}
			}
			g.armsSeen[key][pick] = true
			picked[pick] = true
			for _, m := range members {
				if m != pick {
					excluded[m] = true
				}
			}
		}
	}
	return excluded
}

func (g *dashboardGen) pickMember(path string, members []string, picked, excluded map[string]bool, conflict bool) string {
	for _, m := range members {
		if picked[m] {
			return m
		}
	}
	if forced, ok := g.pin[path]; ok && slices.Contains(members, forced) {
		return forced
	}
	var open []string
	for _, m := range members {
		if !excluded[m] {
			open = append(open, m)
		}
	}
	if len(open) == 0 {
		open = members
	}
	// The arm that was taken least often so far, so no arm waits behind a cycle of the parent.
	key := path + "{" + strings.Join(members, "|") + "}"
	pick := open[0]
	for _, m := range open {
		if g.visits[key+"="+m] < g.visits[key+"="+pick] {
			pick = m
		}
	}
	g.visits[key+"="+pick]++
	return pick
}

// fixLogsAggregation repairs the aggregation rules: count takes no field, and every other type takes
// exactly one of field and observation_field.
func (g *dashboardGen) fixLogsAggregation(m map[string]any) {
	kind, _ := m["type"].(string)
	if !slices.Contains([]string{"count", "count_distinct", "sum", "avg", "min", "max", "percentile"}, kind) {
		return
	}
	_, hasField := m["field"]
	_, hasObservation := m["observation_field"]
	switch {
	case kind == "count":
		delete(m, "field")
		delete(m, "observation_field")
	case hasField && hasObservation:
		if g.variant%2 == 0 {
			delete(m, "observation_field")
		} else {
			delete(m, "field")
		}
	case !hasField && !hasObservation:
		m["field"] = "value-field"
	}
}

// fixObject repairs the combinations that a custom validator checks and that the descriptions do not say.
func fixObject(m map[string]any) {
	if _, isWidget := m["reference"]; isWidget {
		delete(m, "highlighted")
	}
	if source, ok := m["source"].(map[string]any); ok {
		if _, has := m["display_full_row"]; has {
			_, textbox := source["textbox"]
			m["display_full_row"] = textbox
		}
	}
	if m["type"] == "metadata" {
		if _, has := m["value"]; has {
			m["value"] = "service_name"
		}
	}
	fixSpansAggregation(m)
	fixFilterSelection(m)
	fixPercentile(m)
}

func fixSpansAggregation(m map[string]any) {
	aggregation, _ := m["aggregation_type"].(string)
	switch {
	case m["type"] == "dimension" && aggregation != "":
		m["aggregation_type"] = "error_count"
	case m["type"] == "metric" && (aggregation == "error_count" || aggregation == "unique_count"):
		m["aggregation_type"] = "max"
	}
}

// fixFilterSelection keeps selected_values empty for "all" and non-empty for "list".
func fixFilterSelection(m map[string]any) {
	switch m["selection_type"] {
	case "all":
		delete(m, "selected_values")
	case "list":
		if _, has := m["selected_values"]; !has {
			m["selected_values"] = []any{"value"}
		}
	}
}

// fixPercentile sets percent for the percentile type only.
func fixPercentile(m map[string]any) {
	kind, ok := m["type"].(string)
	if !ok {
		return
	}
	_, hasPercent := m["percent"]
	if kind != "percentile" && hasPercent {
		delete(m, "percent")
	}
	if kind == "percentile" && !hasPercent {
		m["percent"] = 50.0
	}
}

func dashboardV4Attrs() map[string]schema.Attribute { return dashboardschema.V4().Attributes }

func (g *dashboardGen) attribute(a schema.Attribute, path, name string) (any, bool) {
	if v, ok := dashboardHint(path, name); ok {
		return v, v != skipAttribute
	}
	ctx := g.ctx
	switch x := a.(type) {
	case schema.StringAttribute:
		return g.stringValue(stringValidatorDescriptions(ctx, x.Validators), name), true
	case schema.BoolAttribute:
		return true, true
	case schema.Int64Attribute:
		return float64(intInRange(attributeDescriptions(ctx, a))), true
	case schema.Float64Attribute:
		return floatInRange(attributeDescriptions(ctx, a)), true
	case schema.NumberAttribute:
		return 1.0, true
	case schema.ListAttribute:
		return g.listValue(x, path), true
	case schema.SingleNestedAttribute:
		return g.object(x.Attributes, path, objectDescriptions(ctx, a)), true
	case schema.ListNestedAttribute:
		items := []any{}
		for i := 0; i < max(1, g.elementsAt(path)); i++ {
			items = append(items, g.object(x.NestedObject.Attributes, path+"[]", objectDescriptions(ctx, a)))
		}
		return items, true
	case schema.SetNestedAttribute:
		return []any{g.object(x.NestedObject.Attributes, path+"[]", objectDescriptions(ctx, a))}, true
	}
	panic(fmt.Sprintf("unsupported attribute type %T at %s", a, path))
}

func (g *dashboardGen) elementsAt(path string) int {
	if strings.ContainsAny(path, ".[") {
		return 1
	}
	return g.elements
}

func (g *dashboardGen) stringValue(descriptions []string, name string) string {
	for _, d := range descriptions {
		if m := oneOfPattern.FindStringSubmatch(d); m != nil {
			values := quotedPattern.FindAllStringSubmatch(m[1], -1)
			return pickEnum(values, g.variant)
		}
	}
	for _, d := range descriptions {
		switch {
		case strings.Contains(d, "such as 900s"):
			return "900s"
		case strings.Contains(d, "such as 1s or 1m"):
			return "1m0s"
		}
	}
	if name == "interval" {
		return "1m0s"
	}
	return "value-" + name
}

// pickEnum picks a value that is not "unspecified" when there is one, and goes through the values by variant.
func pickEnum(quoted [][]string, variant int) string {
	var values []string
	for _, q := range quoted {
		if q[1] != "unspecified" {
			values = append(values, q[1])
		}
	}
	if len(values) == 0 {
		return "unspecified"
	}
	return values[variant%len(values)]
}

func intInRange(descriptions []string) int64 {
	low, high := int64(1), int64(1<<31-1)
	for _, d := range descriptions {
		if m := betweenPattern.FindStringSubmatch(d); m != nil {
			l, _ := strconv.ParseFloat(m[1], 64)
			h, _ := strconv.ParseFloat(m[2], 64)
			low, high = int64(l), int64(h)
		}
	}
	return min(max(1, low), high)
}

func floatInRange(descriptions []string) float64 {
	low, high := 1.0, 1e38
	for _, d := range descriptions {
		if m := betweenPattern.FindStringSubmatch(d); m != nil {
			low, _ = strconv.ParseFloat(m[1], 64)
			high, _ = strconv.ParseFloat(m[2], 64)
		}
	}
	return min(max(1, low), high)
}

func (g *dashboardGen) listValue(a schema.ListAttribute, path string) any {
	switch a.ElementType.(type) {
	case basetypes.StringType:
		for _, v := range a.Validators {
			if m := oneOfPattern.FindStringSubmatch(v.Description(g.ctx)); m != nil {
				return []any{pickEnum(quotedPattern.FindAllStringSubmatch(m[1], -1), g.variant)}
			}
		}
		return []any{"value-" + path[strings.LastIndex(path, ".")+1:]}
	case basetypes.Float64Type, basetypes.NumberType:
		return []any{1.0}
	case basetypes.Int64Type:
		return []any{1.0}
	}
	panic(fmt.Sprintf("unsupported list element type %T at %s", a.ElementType, path))
}

// dashboardSchemaPaths lists every attribute path of the schema, in the form the generator uses.
func dashboardSchemaPaths() []string {
	var out []string
	var walk func(attrs map[string]schema.Attribute, prefix string)
	walk = func(attrs map[string]schema.Attribute, prefix string) {
		for name, a := range attrs {
			path := joinPath(prefix, name)
			if !isConfigurable(a) {
				continue
			}
			out = append(out, path)
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
	sort.Strings(out)
	return out
}

// skipAttribute marks an attribute that the generator leaves out.
const skipAttribute = "\x00skip"

// dashboardHint gives a value for a path where the validators do not say enough, or skips it.
func dashboardHint(path, name string) (any, bool) {
	switch {
	case path == "content_json", path == "id":
		return skipAttribute, true
	case name == "id" && path != "folder.id":
		return "22222222-2222-4222-8222-222222222222", true
	case strings.HasSuffix(path, "stat_card.primary_value.mapped_values"):
		return skipAttribute, true
	case (name == "start" || name == "end") && strings.Contains(path, "time_frame.absolute"):
		return "2024-01-01T00:00:00Z", true
	case strings.HasSuffix(path, "time_frame.relative.duration"):
		return "seconds:900", true
	case name == "widget_ids":
		return []any{"11111111-1111-4111-8111-111111111111"}, true
	}
	return nil, false
}
