// Package schemadump writes a Terraform Plugin Framework resource schema as
// text, and compares two dumps. The POC uses it to compare the generated
// schema with the handwritten one.
//
// This package is not internal: tools/hwschema, a separate module, imports it.
package schemadump

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// Entry is one attribute, or the resource validators.
type Entry struct {
	Path  string   // for example "config.sql_load.join_limit"; "(resource)" for the resource validators
	Lines []string // first line: kind and flags; then defaults, validators, and plan modifiers, sorted
}

const resourcePath = "(resource)"

// Dump returns one entry per attribute, sorted by path. Descriptions are not
// part of the dump.
func Dump(s schema.Schema, validators []resource.ConfigValidator) []Entry {
	var out []Entry
	dumpAttributes(&out, "", s.Attributes)
	if len(validators) > 0 {
		e := Entry{Path: resourcePath}
		for _, v := range validators {
			e.Lines = append(e.Lines, "validator: "+normalize(v.Description(context.Background())))
		}
		sort.Strings(e.Lines)
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func dumpAttributes(out *[]Entry, prefix string, attrs map[string]schema.Attribute) {
	for name, a := range attrs {
		p := prefix + name
		*out = append(*out, entry(p, a))
		v := reflect.ValueOf(a)
		if f := v.FieldByName("Attributes"); f.IsValid() {
			dumpAttributes(out, p+".", f.Interface().(map[string]schema.Attribute))
		}
		if f := v.FieldByName("NestedObject"); f.IsValid() {
			dumpAttributes(out, p+"[].", f.FieldByName("Attributes").Interface().(map[string]schema.Attribute))
		}
	}
}

type describer interface {
	Description(context.Context) string
}

func entry(p string, a schema.Attribute) Entry {
	first := reflect.TypeOf(a).Name()
	for _, flag := range []struct {
		on   bool
		name string
	}{{a.IsRequired(), "required"}, {a.IsOptional(), "optional"}, {a.IsComputed(), "computed"}, {a.IsSensitive(), "sensitive"}} {
		if flag.on {
			first += " " + flag.name
		}
	}
	v := reflect.ValueOf(a)
	if f := v.FieldByName("ElementType"); f.IsValid() && !f.IsNil() {
		first += " elem=" + f.Interface().(attr.Type).String()
	}
	var details []string
	if f := v.FieldByName("Default"); f.IsValid() && !f.IsNil() {
		details = append(details, "default: "+describe(f.Interface()))
	}
	for _, field := range []struct{ name, label string }{{"Validators", "validator"}, {"PlanModifiers", "plan modifier"}} {
		f := v.FieldByName(field.name)
		if !f.IsValid() {
			continue
		}
		for i := range f.Len() {
			details = append(details, field.label+": "+describe(f.Index(i).Interface()))
		}
	}
	sort.Strings(details)
	return Entry{Path: p, Lines: append([]string{first}, details...)}
}

func describe(v any) string {
	d, ok := v.(describer)
	if !ok {
		return fmt.Sprintf("%T", v)
	}
	return normalize(d.Description(context.Background()))
}

var oneOfList = regexp.MustCompile(`one of: \[([^\]]*)\]`)

// normalize sorts the values in a "one of: [...]" description. A handwritten
// list can come from map keys, which have no fixed order.
func normalize(s string) string {
	return oneOfList.ReplaceAllStringFunc(s, func(m string) string {
		items := strings.Fields(oneOfList.FindStringSubmatch(m)[1])
		slices.Sort(items)
		return "one of: [" + strings.Join(items, " ") + "]"
	})
}

// Text writes the entries, one block per entry.
func Text(entries []Entry) string {
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "%s  %s\n", e.Path, e.Lines[0])
		for _, l := range e.Lines[1:] {
			fmt.Fprintf(&b, "    %s\n", l)
		}
	}
	return b.String()
}

// Parse reads the output of Text.
func Parse(text string) ([]Entry, error) {
	var out []Entry
	for i, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		if l, ok := strings.CutPrefix(line, "    "); ok {
			if len(out) == 0 {
				return nil, fmt.Errorf("line %d: detail before the first entry", i+1)
			}
			out[len(out)-1].Lines = append(out[len(out)-1].Lines, l)
			continue
		}
		p, first, ok := strings.Cut(line, "  ")
		if !ok {
			return nil, fmt.Errorf("line %d: want \"path  kind\"", i+1)
		}
		out = append(out, Entry{Path: p, Lines: []string{first}})
	}
	return out, nil
}

// Diff compares two dumps. For each path it writes:
//
//   - path  kind   only in a
//   - path  kind   only in b
//     ~ path         in both, with the lines that differ ("- line" in a, "+ line" in b)
func Diff(a, b []Entry) string {
	am, bm := byPath(a), byPath(b)
	var paths []string
	for p := range am {
		paths = append(paths, p)
	}
	for p := range bm {
		if _, ok := am[p]; !ok {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	var out strings.Builder
	for _, p := range paths {
		ea, inA := am[p]
		eb, inB := bm[p]
		switch {
		case !inB:
			fmt.Fprintf(&out, "- %s  %s\n", p, ea.Lines[0])
		case !inA:
			fmt.Fprintf(&out, "+ %s  %s\n", p, eb.Lines[0])
		default:
			var lines []string
			for _, l := range ea.Lines {
				if !slices.Contains(eb.Lines, l) {
					lines = append(lines, "    - "+l)
				}
			}
			for _, l := range eb.Lines {
				if !slices.Contains(ea.Lines, l) {
					lines = append(lines, "    + "+l)
				}
			}
			if len(lines) > 0 {
				fmt.Fprintf(&out, "~ %s\n%s\n", p, strings.Join(lines, "\n"))
			}
		}
	}
	return out.String()
}

func byPath(entries []Entry) map[string]Entry {
	m := map[string]Entry{}
	for _, e := range entries {
		m[e.Path] = e
	}
	return m
}
