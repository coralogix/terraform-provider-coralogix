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

// Package schemadump writes a Terraform Plugin Framework resource schema as
// text. Two schemas that behave the same give the same text, so a test can
// keep the text as a golden file and compare it after a refactor.
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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

type describer interface {
	Description(context.Context) string
}

type entry struct {
	path  string
	lines []string // first line: kind and flags; then details, sorted
}

// Text returns the schema as text: the version, then one block per
// attribute, sorted by path. Each block lists the attribute kind and flags,
// the descriptions, the deprecation message, the default, the validators, and
// the plan modifiers.
func Text(s schema.Schema) string {
	var entries []entry
	dumpAttributes(&entries, "", s.Attributes)
	sort.Slice(entries, func(i, j int) bool { return entries[i].path < entries[j].path })

	var b strings.Builder
	fmt.Fprintf(&b, "(schema)  version=%d\n", s.Version)
	for _, l := range detailLines(s.Description, s.MarkdownDescription, s.DeprecationMessage) {
		fmt.Fprintf(&b, "    %s\n", l)
	}
	for _, e := range entries {
		fmt.Fprintf(&b, "%s  %s\n", e.path, e.lines[0])
		for _, l := range e.lines[1:] {
			fmt.Fprintf(&b, "    %s\n", l)
		}
	}
	return b.String()
}

func dumpAttributes(out *[]entry, prefix string, attrs map[string]schema.Attribute) {
	for name, a := range attrs {
		p := prefix + name
		*out = append(*out, attributeEntry(p, a))
		v := reflect.ValueOf(a)
		if f := v.FieldByName("Attributes"); f.IsValid() {
			dumpAttributes(out, p+".", f.Interface().(map[string]schema.Attribute))
		}
		if f := v.FieldByName("NestedObject"); f.IsValid() {
			*out = append(*out, nestedObjectEntry(p+"[]", f))
			dumpAttributes(out, p+"[].", f.FieldByName("Attributes").Interface().(map[string]schema.Attribute))
		}
	}
}

func attributeEntry(p string, a schema.Attribute) entry {
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
	if f := v.FieldByName("AttributeTypes"); f.IsValid() && f.Len() > 0 {
		first += " attrs=" + attributeTypes(f.Interface().(map[string]attr.Type))
	}
	if f := v.FieldByName("CustomType"); f.IsValid() && !f.IsNil() {
		first += " custom=" + fmt.Sprintf("%T", f.Interface())
	}

	details := detailLines(
		stringField(v, "Description"),
		stringField(v, "MarkdownDescription"),
		stringField(v, "DeprecationMessage"),
	)
	if f := v.FieldByName("Default"); f.IsValid() && !f.IsNil() {
		details = append(details, "default: "+describe(f.Interface()))
	}
	details = append(details, listDetails(v, "Validators", "validator")...)
	details = append(details, listDetails(v, "PlanModifiers", "plan modifier")...)
	sort.Strings(details)
	return entry{path: p, lines: append([]string{first}, details...)}
}

func nestedObjectEntry(p string, nested reflect.Value) entry {
	details := listDetails(nested, "Validators", "validator")
	details = append(details, listDetails(nested, "PlanModifiers", "plan modifier")...)
	if f := nested.FieldByName("CustomType"); f.IsValid() && !f.IsNil() {
		details = append(details, "custom: "+fmt.Sprintf("%T", f.Interface()))
	}
	sort.Strings(details)
	return entry{path: p, lines: append([]string{"NestedObject"}, details...)}
}

func stringField(v reflect.Value, name string) string {
	if f := v.FieldByName(name); f.IsValid() && f.Kind() == reflect.String {
		return f.String()
	}
	return ""
}

func detailLines(description, markdown, deprecation string) []string {
	var out []string
	if description != "" {
		out = append(out, "description: "+fmt.Sprintf("%q", description))
	}
	if markdown != "" {
		out = append(out, "markdown: "+fmt.Sprintf("%q", markdown))
	}
	if deprecation != "" {
		out = append(out, "deprecated: "+fmt.Sprintf("%q", deprecation))
	}
	return out
}

func listDetails(v reflect.Value, field, label string) []string {
	f := v.FieldByName(field)
	if !f.IsValid() {
		return nil
	}
	var out []string
	for i := range f.Len() {
		out = append(out, label+": "+describe(f.Index(i).Interface()))
	}
	return out
}

func attributeTypes(m map[string]attr.Type) string {
	names := make([]string, 0, len(m))
	for k := range m {
		names = append(names, k)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, k := range names {
		parts = append(parts, k+":"+m[k].String())
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func describe(v any) string {
	d, ok := v.(describer)
	if !ok {
		return fmt.Sprintf("%T", v)
	}
	return fmt.Sprintf("%T %s", v, normalize(d.Description(context.Background())))
}

var oneOfList = regexp.MustCompile(`one of: \[([^\]]*)\]`)

// normalize sorts the values in a "one of: [...]" description. A list built
// from map keys has no fixed order.
func normalize(s string) string {
	return oneOfList.ReplaceAllStringFunc(s, func(m string) string {
		items := strings.Fields(oneOfList.FindStringSubmatch(m)[1])
		slices.Sort(items)
		return "one of: [" + strings.Join(items, " ") + "]"
	})
}
