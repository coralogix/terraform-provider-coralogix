package model

import (
	"fmt"
	"strconv"
	"strings"
)

// Dump returns a readable, deterministic text form of the resource.
func Dump(r *Resource) string {
	var b strings.Builder
	fmt.Fprintf(&b, "resource %s\n", r.Name)
	fmt.Fprintf(&b, "  id param:     %s\n", r.IDParam)
	fmt.Fprintf(&b, "  update mask:  %s\n", r.UpdateMask)

	b.WriteString("\noperations\n")
	var ops [][]string
	for _, o := range []struct {
		name string
		op   Operation
	}{{"create", r.Create}, {"get", r.Get}, {"update", r.Update}, {"delete", r.Delete}} {
		ops = append(ops, []string{o.name, o.op.Method, o.op.Path, o.op.OperationID,
			"body " + orDash(o.op.Body), "response " + responseString(o.op.Response)})
	}
	for _, line := range table(ops) {
		fmt.Fprintf(&b, "  %s\n", line)
	}

	b.WriteString("\nfields\n")
	rows := [][]string{{"NAME", "BEHAVIOR", "CREATE", "UPDATE", "GET", "TYPE"}}
	for _, f := range r.Fields {
		get := "-"
		if f.InGet {
			get = "yes"
		}
		rows = append(rows, []string{f.Name, string(f.Behavior), locationString(f.Create),
			locationString(f.Update), get, typeString(f.Type)})
	}
	for i, line := range table(rows) {
		fmt.Fprintf(&b, "  %s\n", line)
		if i > 0 {
			dumpChildren(&b, r.Fields[i-1].Type, 6)
		}
	}
	return b.String()
}

// dumpChildren writes the fields of an object, the arms of a oneOf, or the
// fields of the objects in a list, set, or map.
func dumpChildren(b *strings.Builder, t *Type, indent int) {
	if t.Elem != nil {
		t = t.Elem
	}
	if len(t.Fields) == 0 {
		return
	}
	var rows [][]string
	for _, f := range t.Fields {
		attrs := attrsString(f.Attrs)
		if t.Kind == OneOf {
			attrs = "arm"
		}
		rows = append(rows, []string{f.Name, attrs, typeString(f.Type)})
	}
	for i, line := range table(rows) {
		fmt.Fprintf(b, "%s%s\n", strings.Repeat(" ", indent), line)
		dumpChildren(b, t.Fields[i].Type, indent+2)
	}
}

// table pads each column but the last to the widest cell in the column.
func table(rows [][]string) []string {
	var widths []int
	for _, row := range rows {
		for i, cell := range row {
			if i >= len(widths) {
				widths = append(widths, 0)
			}
			widths[i] = max(widths[i], len(cell))
		}
	}
	lines := make([]string, len(rows))
	for r, row := range rows {
		var line strings.Builder
		for i, cell := range row {
			if i == len(row)-1 {
				line.WriteString(cell)
				break
			}
			fmt.Fprintf(&line, "%-*s  ", widths[i], cell)
		}
		lines[r] = strings.TrimRight(line.String(), " ")
	}
	return lines
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func responseString(r Response) string {
	if r.Field == "" {
		return r.Schema
	}
	return r.Schema + "." + r.Field
}

func locationString(a *Attrs) string {
	if a == nil {
		return "-"
	}
	return attrsString(*a)
}

func attrsString(a Attrs) string {
	parts := []string{"optional"}
	if a.Required {
		parts[0] = "required"
	}
	if a.Presence {
		parts = append(parts, "presence")
	}
	if a.Default != nil {
		parts = append(parts, "default="+*a.Default)
	}
	return strings.Join(parts, " ")
}

func typeString(t *Type) string {
	parts := []string{string(t.Kind)}
	switch t.Kind {
	case List, Set, Map:
		parts[0] = fmt.Sprintf("%s<%s>", t.Kind, typeString(t.Elem))
	case Enum:
		parts = append(parts, t.Schema, "["+strings.Join(t.Values, " ")+"]")
	case Object:
		parts = append(parts, t.Schema)
		if len(t.Fields) == 0 {
			parts = append(parts, "{}")
		}
	case OneOf:
		arms := fmt.Sprintf("(%d arms", len(t.Fields))
		if t.AllowNone {
			arms += ", no arm allowed"
		}
		parts = append(parts, t.Schema, arms+")")
	default:
		parts = append(parts, t.Format)
		if t.WireString {
			parts = append(parts, "(wire: string)")
		}
	}
	parts = append(parts,
		rangeString("len ", intString(t.MinLength), intString(t.MaxLength)),
		rangeString("", floatString(t.Minimum), floatString(t.Maximum)),
		rangeString("items ", intString(t.MinItems), intString(t.MaxItems)))
	var out []string
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " ")
}

func rangeString(label, lo, hi string) string {
	if lo == "" && hi == "" {
		return ""
	}
	return label + lo + ".." + hi
}

func intString(v *int64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatInt(*v, 10)
}

func floatString(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'g', -1, 64)
}
