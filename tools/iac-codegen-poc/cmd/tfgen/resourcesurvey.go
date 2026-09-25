package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/model"
)

// resourceOps are the operations of one resource name, by verb.
type resourceOps map[string]foundOperation

type foundOperation struct {
	method, path string
	item         *v3.PathItem
	op           *v3.Operation
}

var crudOperation = regexp.MustCompile(`_(Create|Get|Update|Replace|Delete)([A-Z][A-Za-z0-9]*)$`)

// runResourceSurvey measures the resource shapes (operations, ids, bodies,
// responses) of every resource name that has a Get and a Create or Update. It ignores the
// update verb (PUT or PATCH) and the update mask: it assumes the contract for
// them. It writes a summary by shape, then the shapes of each resource.
func runResourceSurvey(specPath string, out io.Writer) error {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return err
	}
	doc, err := model.Load(data)
	if err != nil {
		return err
	}
	byName := collectOperations(doc)
	var names []string
	readOnly := 0
	for n, ops := range byName {
		if _, ok := ops["Get"]; !ok {
			continue
		}
		if !writable(ops) {
			readOnly++ // a data source, not a managed resource
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names)
	fmt.Fprintf(out, "%d read-only names (Get, maybe Delete, no Create or Update) are not counted.\n", readOnly)
	shapes := map[string][]string{}
	for _, n := range names {
		shapes[n] = operationShapes(doc, n, byName[n])
	}
	return writeResourceSurvey(out, names, shapes)
}

// writable reports whether the resource has a Create or an Update.
func writable(ops resourceOps) bool {
	for _, v := range []string{"Create", "Update", "Replace"} {
		if _, ok := ops[v]; ok {
			return true
		}
	}
	return false
}

func collectOperations(doc *v3.Document) map[string]resourceOps {
	byName := map[string]resourceOps{}
	for pair := doc.Paths.PathItems.First(); pair != nil; pair = pair.Next() {
		item := pair.Value()
		for method, op := range map[string]*v3.Operation{
			"POST": item.Post, "GET": item.Get, "PUT": item.Put, "PATCH": item.Patch, "DELETE": item.Delete,
		} {
			if op == nil {
				continue
			}
			m := crudOperation.FindStringSubmatch(op.OperationId)
			if m == nil {
				continue
			}
			if byName[m[2]] == nil {
				byName[m[2]] = resourceOps{}
			}
			byName[m[2]][m[1]] = foundOperation{method: method, path: pair.Key(), item: item, op: op}
		}
	}
	return byName
}

// operationShapes returns the shapes of one resource that the generator
// needs to know about. "ok" shapes are the ones it supports today.
func operationShapes(doc *v3.Document, name string, ops resourceOps) []string {
	get := ops["Get"]
	resource := resourceOf(doc, name, get.op)
	var shapes []string
	shapes = append(shapes, "operations: "+operationSet(ops))
	shapes = append(shapes, idShapes(doc, resource, ops)...)
	for _, verb := range []string{"Create", "Update", "Replace"} {
		if o, ok := ops[verb]; ok {
			shapes = append(shapes, bodyShapes(verb, o.op, resource)...)
		}
	}
	shapes = append(shapes, responseShape("Get", get.op, resource))
	if c, ok := ops["Create"]; ok {
		if s := responseShape("Create", c.op, resource); !strings.HasSuffix(s, "wraps the resource") {
			shapes = append(shapes, s)
		}
	}
	shapes = append(shapes, typeMismatches(doc, resource, ops)...)
	return shapes
}

func operationSet(ops resourceOps) string {
	_, c := ops["Create"]
	_, d := ops["Delete"]
	_, u := ops["Update"]
	_, r := ops["Replace"]
	update := u || r
	switch {
	case c && d && update:
		return "Create, Get, Update, Delete"
	case c && d:
		return "Create, Get, Delete (no Update: a change replaces the resource)"
	case update && !c && !d:
		return "Get, Update only (a singleton or settings)"
	case !c && !d && !update:
		return "Get only (a data source)"
	}
	var have []string
	for _, v := range []string{"Create", "Get", "Update", "Replace", "Delete"} {
		if _, ok := ops[v]; ok {
			have = append(have, v)
		}
	}
	return "other: " + strings.Join(have, ", ")
}

// idShapes checks the path parameters of Get: one id, or parents too, and
// whether the id parameter has the name of a resource field.
func idShapes(doc *v3.Document, resource string, ops resourceOps) []string {
	params := surveyPathParams(ops["Get"])
	var shapes []string
	switch len(params) {
	case 0:
		shapes = append(shapes, "id: Get has no path parameter")
	case 1:
	default:
		shapes = append(shapes, fmt.Sprintf("id: parent ids in the path (%d path parameters)", len(params)))
	}
	if c, ok := ops["Create"]; ok && len(surveyPathParams(c)) > 0 {
		shapes = append(shapes, "id: Create has path parameters (a sub-resource)")
	}
	if len(params) > 0 && resource != "" {
		id := params[len(params)-1]
		if s := componentSchema(doc, resource); s != nil && s.Properties.GetOrZero(id) == nil {
			shapes = append(shapes, "id: the id path parameter is not a resource field ({"+id+"})")
		}
	}
	return shapes
}

func surveyPathParams(f foundOperation) []string {
	var out []string
	for _, p := range append(append([]*v3.Parameter{}, f.item.Parameters...), f.op.Parameters...) {
		if p.In == "path" {
			out = append(out, p.Name)
		}
	}
	return out
}

// bodyShapes checks a request body: inline, or a $ref; flat, or a wrapper of
// the resource.
func bodyShapes(verb string, op *v3.Operation, resource string) []string {
	if op.RequestBody == nil {
		return []string{verb + " body: none"}
	}
	media := op.RequestBody.Content.GetOrZero("application/json")
	if media == nil || media.Schema == nil {
		return []string{verb + " body: not JSON"}
	}
	var shapes []string
	if media.Schema.IsReference() {
		shapes = append(shapes, verb+" body: a $ref component, not inline")
	}
	if s := media.Schema.Schema(); s != nil && s.Properties != nil {
		for pair := s.Properties.First(); pair != nil; pair = pair.Next() {
			if refName(pair.Value()) == resource {
				shapes = append(shapes, verb+" body: wraps the resource in a field ("+pair.Key()+")")
			}
		}
	}
	return shapes
}

// responseShape describes the 200 response of op compared with the resource.
func responseShape(verb string, op *v3.Operation, resource string) string {
	resp := op.Responses.Codes.GetOrZero("200")
	if resp == nil {
		return verb + " response: no 200 response"
	}
	media := resp.Content.GetOrZero("application/json")
	if media == nil || media.Schema == nil {
		return verb + " response: not JSON"
	}
	if refName(media.Schema) == resource && resource != "" {
		return verb + " response: is the resource itself"
	}
	s := media.Schema.Schema()
	if s == nil || s.Properties == nil {
		return verb + " response: no resource"
	}
	wraps := false
	for pair := s.Properties.First(); pair != nil; pair = pair.Next() {
		if refName(pair.Value()) == resource {
			wraps = true
		}
	}
	switch {
	case !wraps:
		return verb + " response: no resource"
	case s.Properties.Len() > 1:
		return verb + " response: the resource and other fields beside it"
	}
	return verb + " response: wraps the resource"
}

// typeMismatches returns a shape when a field of a flat Create body has a
// different $ref or type than the same field of the resource.
func typeMismatches(doc *v3.Document, resource string, ops resourceOps) []string {
	c, ok := ops["Create"]
	res := componentSchema(doc, resource)
	if !ok || res == nil || c.op.RequestBody == nil {
		return nil
	}
	media := c.op.RequestBody.Content.GetOrZero("application/json")
	if media == nil || media.Schema == nil || media.Schema.Schema() == nil || media.Schema.Schema().Properties == nil {
		return nil
	}
	body := media.Schema.Schema()
	for pair := body.Properties.First(); pair != nil; pair = pair.Next() {
		r := res.Properties.GetOrZero(pair.Key())
		if r != nil && typeSig(r) != typeSig(pair.Value()) {
			return []string{"fields: a field has another type in Create than in Get (" + pair.Key() + ")"}
		}
	}
	return nil
}

func typeSig(p *base.SchemaProxy) string {
	if n := refName(p); n != "" {
		return "$ref " + n
	}
	s := p.Schema()
	if s == nil {
		return ""
	}
	sig := strings.Join(s.Type, "|") + " " + s.Format
	if s.Items != nil && s.Items.IsA() {
		sig += " of " + typeSig(s.Items.A)
	}
	return sig
}

// refName returns the component name of a $ref, also inside a single allOf.
func refName(p *base.SchemaProxy) string {
	ref := p.GetReference()
	if s := p.Schema(); ref == "" && s != nil && len(s.AllOf) == 1 {
		ref = s.AllOf[0].GetReference()
	}
	return strings.TrimPrefix(ref, "#/components/schemas/")
}

func componentSchema(doc *v3.Document, name string) *base.Schema {
	if p := doc.Components.Schemas.GetOrZero(name); p != nil {
		return p.Schema()
	}
	return nil
}

func writeResourceSurvey(out io.Writer, names []string, shapes map[string][]string) error {
	count := map[string][]string{}
	for _, n := range names {
		for _, s := range shapes[n] {
			key := regexp.MustCompile(` \(([^)]*)\)$`).ReplaceAllString(s, "")
			if strings.HasPrefix(s, "operations:") {
				key = s
			}
			count[key] = append(count[key], n)
		}
	}
	keys := sortedKeys(count)
	sort.SliceStable(keys, func(i, j int) bool { return len(count[keys[i]]) > len(count[keys[j]]) })
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "%d writable resource names with a Get operation\n\n", len(names))
	fmt.Fprintln(tw, "RESOURCES\tSHAPE\tEXAMPLES")
	for _, k := range keys {
		ex := count[k]
		if len(ex) > 3 {
			ex = append(ex[:3:3], "…")
		}
		fmt.Fprintf(tw, "%d\t%s\t%s\n", len(count[k]), k, strings.Join(ex, ", "))
	}
	fmt.Fprintln(tw, "\nRESOURCE\tSHAPES")
	for _, n := range names {
		fmt.Fprintf(tw, "%s\t%s\n", n, strings.Join(shapes[n], "; "))
	}
	return tw.Flush()
}
