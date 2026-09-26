package schemadump

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// Difference is one difference between a handwritten schema and the
// generated one that should replace it (D21).
type Difference struct {
	Path string
	Kind string // for example "only in handwritten", "flags", "validator"
	// Breaking is true when the switch would change a user configuration, a
	// plan, or the state. A difference that is not breaking is listed for a
	// review: for example a new validator from the spec.
	Breaking bool
	Detail   string
}

// Difference kinds.
const (
	KindOnlyHandwritten = "only in handwritten"
	KindOnlyGenerated   = "only in generated"
	KindType            = "type"
	KindFlags           = "flags"
	KindDefault         = "default"
	KindPlanModifier    = "plan modifier"
	KindValidator       = "validator"
	KindDeprecation     = "deprecation"
)

// Compare lists the differences between the handwritten schema hw and the
// generated schema gen, by path. Rules:
//   - An attribute only in hw breaks configurations that set it.
//   - An attribute only in gen breaks them when it is required; otherwise it
//     is an addition.
//   - Another type, element type, default, or Computed flag changes the plan
//     or the state. A required attribute that becomes optional does not.
//   - Another RequiresReplace changes the plan. Another UseStateForUnknown
//     only shows "known after apply" in more plans.
//   - A deprecation message only shows a warning: it is for a review.
//   - A "one of" validator that no longer accepts a value breaks the
//     configurations that use it. Other validators are for a review: the
//     generated ones come from the spec.
//
// The children of an attribute that is only on one side are not listed.
func Compare(hw, gen []Entry) []Difference {
	hm, gm := byPath(hw), byPath(gen)
	var out []Difference
	for _, p := range unionPaths(hm, gm) {
		h, inH := hm[p]
		g, inG := gm[p]
		switch {
		case !inG:
			if !parentMissing(p, gm) {
				out = append(out, Difference{Path: p, Kind: KindOnlyHandwritten, Breaking: true, Detail: h.Lines[0]})
			}
		case !inH:
			if !parentMissing(p, hm) {
				out = append(out, Difference{Path: p, Kind: KindOnlyGenerated, Breaking: hasFlag(g, "required"), Detail: g.Lines[0]})
			}
		default:
			out = append(out, compareEntry(p, h, g)...)
		}
	}
	return out
}

func compareEntry(p string, h, g Entry) []Difference {
	var out []Difference
	hk, hf := splitFirst(h.Lines[0])
	gk, gf := splitFirst(g.Lines[0])
	if hk != gk || elem(hf) != elem(gf) {
		out = append(out, Difference{Path: p, Kind: KindType, Breaking: true, Detail: h.Lines[0] + " → " + g.Lines[0]})
	}
	if flagsOf(hf) != flagsOf(gf) {
		// Only required → optional is safe: it accepts every old configuration.
		safe := hf["required"] && gf["optional"] && !gf["computed"] && !hf["computed"]
		out = append(out, Difference{Path: p, Kind: KindFlags, Breaking: !safe, Detail: flagsOf(hf) + " → " + flagsOf(gf)})
	}
	oneOfs, hv, gv := compareOneOf(p, h.Lines[1:], g.Lines[1:])
	out = append(out, oneOfs...)
	h.Lines = append(h.Lines[:1:1], hv...)
	g.Lines = append(g.Lines[:1:1], gv...)
	for _, d := range []struct {
		prefix, kind string
		breaking     func(line string) bool
	}{
		{"default: ", KindDefault, func(string) bool { return true }},
		{"plan modifier: ", KindPlanModifier, func(l string) bool { return !strings.Contains(l, "will not change") }},
		{"validator: ", KindValidator, func(string) bool { return false }},
		{"deprecated: ", KindDeprecation, func(string) bool { return false }},
	} {
		for _, l := range h.Lines[1:] {
			if strings.HasPrefix(l, d.prefix) && !slices.Contains(g.Lines, l) {
				out = append(out, Difference{Path: p, Kind: d.kind, Breaking: d.breaking(l), Detail: "- " + strings.TrimPrefix(l, d.prefix)})
			}
		}
		for _, l := range g.Lines[1:] {
			if strings.HasPrefix(l, d.prefix) && !slices.Contains(h.Lines, l) {
				out = append(out, Difference{Path: p, Kind: d.kind, Breaking: d.breaking(l), Detail: "+ " + strings.TrimPrefix(l, d.prefix)})
			}
		}
	}
	return out
}

// compareOneOf compares the "one of" validators of an attribute that have
// the same text around the values. A value that only hw accepts breaks the
// configurations that use it; a value that only gen accepts is new. It
// returns the other lines of both sides.
func compareOneOf(p string, h, g []string) ([]Difference, []string, []string) {
	var out []Difference
	gRest := slices.Clone(g)
	var hRest []string
	for _, hl := range h {
		hk, hvals, ok := oneOfValues(hl)
		i := slices.IndexFunc(gRest, func(gl string) bool { k, _, ok := oneOfValues(gl); return ok && k == hk })
		if !ok || i < 0 {
			hRest = append(hRest, hl)
			continue
		}
		_, gvals, _ := oneOfValues(gRest[i])
		gRest = slices.Delete(gRest, i, i+1)
		removed, added := minus(hvals, gvals), minus(gvals, hvals)
		if len(removed) != 0 {
			out = append(out, Difference{Path: p, Kind: KindValidator, Breaking: true,
				Detail: "values no longer accepted: " + strings.Join(removed, " ")})
		}
		if len(added) != 0 {
			out = append(out, Difference{Path: p, Kind: KindValidator, Detail: "new values: " + strings.Join(added, " ")})
		}
	}
	return out, hRest, gRest
}

// oneOfValues splits a validator line with "one of: [...]" into the text
// around the values, and the values.
func oneOfValues(line string) (string, []string, bool) {
	m := oneOfList.FindStringSubmatchIndex(line)
	if !strings.HasPrefix(line, "validator: ") || m == nil {
		return "", nil, false
	}
	return line[:m[0]] + line[m[1]:], strings.Fields(line[m[2]:m[3]]), true
}

func minus(a, b []string) []string {
	var out []string
	for _, v := range a {
		if !slices.Contains(b, v) {
			out = append(out, v)
		}
	}
	return out
}

// ReportText writes the differences, breaking ones first, and a summary.
func ReportText(diffs []Difference) string {
	sorted := slices.Clone(diffs)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Breaking && !sorted[j].Breaking })
	var b strings.Builder
	breaking := 0
	for _, d := range sorted {
		label := "review  "
		if d.Breaking {
			label = "BREAKING"
			breaking++
		}
		fmt.Fprintf(&b, "%s  %-20s %s  %s\n", label, d.Kind, d.Path, d.Detail)
	}
	fmt.Fprintf(&b, "%d breaking, %d for review\n", breaking, len(diffs)-breaking)
	return b.String()
}

func unionPaths(a, b map[string]Entry) []string {
	var paths []string
	for p := range a {
		paths = append(paths, p)
	}
	for p := range b {
		if _, ok := a[p]; !ok {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	return paths
}

// parentMissing reports whether an attribute above p is not in m.
func parentMissing(p string, m map[string]Entry) bool {
	for {
		i := strings.LastIndex(p, ".")
		if i < 0 {
			return false
		}
		p = strings.TrimSuffix(p[:i], "[]")
		if _, ok := m[p]; !ok {
			return true
		}
	}
}

// splitFirst splits the first line of an entry into the attribute kind and
// its flags and element type.
func splitFirst(line string) (string, map[string]bool) {
	parts := strings.Fields(line)
	flags := map[string]bool{}
	for _, f := range parts[1:] {
		flags[f] = true
	}
	return parts[0], flags
}

func elem(flags map[string]bool) string {
	for f := range flags {
		if strings.HasPrefix(f, "elem=") {
			return f
		}
	}
	return ""
}

func flagsOf(flags map[string]bool) string {
	var out []string
	for _, f := range []string{"required", "optional", "computed", "sensitive"} {
		if flags[f] {
			out = append(out, f)
		}
	}
	return strings.Join(out, " ")
}

func hasFlag(e Entry, flag string) bool {
	_, flags := splitFirst(e.Lines[0])
	return flags[flag]
}
