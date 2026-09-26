package main

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/model"
)

// surveyIssue is one schema shape that the generator cannot generate.
type surveyIssue struct {
	resource string
	path     string // field path inside the resource
	shape    string // the shape, without names, to group issues
}

// runSurvey measures the schema shapes of every Get resource in the spec
// that the model or the Terraform generator does not support. It ignores the
// operations: it assumes the contract (PATCH with a mask, flat bodies). It
// writes a summary by shape, then every issue.
func runSurvey(specPath string, out io.Writer) error {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return err
	}
	doc, err := model.Load(data)
	if err != nil {
		return err
	}
	resources := getResources(doc)
	var issues []surveyIssue
	for _, name := range resources {
		t, errs := model.Survey(doc, name)
		for _, e := range errs {
			issues = append(issues, modelIssue(name, e.Error()))
		}
		if t != nil {
			issues = append(issues, tfUnsupported(name, name, t)...)
		}
	}
	return writeSurvey(out, resources, issues)
}

var getOperation = regexp.MustCompile(`_Get([A-Z][A-Za-z0-9]*)$`)

// getResources returns the resource schema of each Get operation: the
// component with the name after "_Get", or the single component that the
// response wraps.
func getResources(doc *v3.Document) []string {
	seen := map[string]bool{}
	for pair := doc.Paths.PathItems.First(); pair != nil; pair = pair.Next() {
		op := pair.Value().Get
		if op == nil {
			continue
		}
		m := getOperation.FindStringSubmatch(op.OperationId)
		if m == nil {
			continue
		}
		if name := resourceOf(doc, m[1], op); name != "" {
			seen[name] = true
		}
	}
	return sortedKeys(seen)
}

func resourceOf(doc *v3.Document, name string, op *v3.Operation) string {
	if doc.Components.Schemas.GetOrZero(name) != nil {
		return name
	}
	resp := op.Responses.Codes.GetOrZero("200")
	if resp == nil {
		return ""
	}
	media := resp.Content.GetOrZero("application/json")
	if media == nil || media.Schema == nil {
		return ""
	}
	s := media.Schema.Schema()
	if s == nil || s.Properties == nil || s.Properties.Len() != 1 {
		return ""
	}
	p := s.Properties.First().Value()
	ref := p.GetReference()
	if ps := p.Schema(); ref == "" && ps != nil && len(ps.AllOf) == 1 {
		ref = ps.AllOf[0].GetReference()
	}
	return strings.TrimPrefix(ref, "#/components/schemas/")
}

// shapeCleanup turns an error message into a shape: it removes names,
// numbers, and value lists, so equal shapes group together.
var shapeCleanup = []struct {
	re   *regexp.Regexp
	with string
}{
	{regexp.MustCompile(`oneOf\[\d+\]`), "oneOf[n]"},
	{regexp.MustCompile(`"[^"]*"`), `"…"`},
	{regexp.MustCompile(`\[[^\]]*\]`), "[…]"},
	{regexp.MustCompile(`#/components/schemas/\S+`), "#/components/schemas/…"},
	{regexp.MustCompile(`\d+`), "n"},
	{regexp.MustCompile(`arm \S+ has`), "arm … has"},
}

var pathPrefix = regexp.MustCompile(`^([A-Za-z0-9_.\[\]{}]+): (.*)$`)

// modelIssue splits a model error into the deepest path and the shape.
func modelIssue(resource, msg string) surveyIssue {
	path := resource
	for {
		m := pathPrefix.FindStringSubmatch(msg)
		if m == nil || !strings.HasPrefix(m[1], resource) {
			break
		}
		path, msg = m[1], m[2]
	}
	for _, c := range shapeCleanup {
		msg = c.re.ReplaceAllString(msg, c.with)
	}
	for _, l := range shapeLabels {
		if strings.HasPrefix(msg, l.prefix) {
			msg = l.label
			break
		}
	}
	return surveyIssue{resource: resource, path: path, shape: "model: " + msg}
}

// shapeLabels name the model errors by their shape.
var shapeLabels = []struct{ prefix, label string }{
	{"oneOf[…]: not.anyOf lists", "oneOf with normal fields beside the arms"},
	{"oneOf arm … has attributes {Required:true", "oneOf arm listed in \"required\""},
	{"allOf with n entries", "several oneOf groups in one object (allOf of oneOf)"},
	{"enum has no values", "enum with only the *_UNSPECIFIED value"},
}

// tfUnsupported returns the shapes that the model accepts but that the
// Terraform generator rejects. It follows the rules of buildConv and
// buildTFResource.
func tfUnsupported(resource, path string, t *model.Type) []surveyIssue {
	issue := func(shape string) []surveyIssue {
		return []surveyIssue{{resource: resource, path: path, shape: "generator: " + shape}}
	}
	switch t.Kind {
	case model.Integer, model.Number:
		if !supportedNumber(t) {
			return issue(fmt.Sprintf("%s format %q", t.Kind, t.Format))
		}
	case model.Enum:
		if t.Schema == "" {
			return issue("inline enum schema")
		}
	case model.Object, model.OneOf:
		if t.Schema == "" && len(t.Fields) != 0 {
			return issue("inline object schema")
		}
		var out []surveyIssue
		for _, f := range t.Fields {
			out = append(out, tfUnsupported(resource, path+"."+f.Name, f.Type)...)
		}
		return out
	case model.List, model.Set:
		return collectionIssues(resource, path, t)
	case model.Map:
		return mapIssues(resource, path, t)
	}
	return nil
}

func collectionIssues(resource, path string, t *model.Type) []surveyIssue {
	issue := func(shape string) []surveyIssue {
		return []surveyIssue{{resource: resource, path: path, shape: "generator: " + shape}}
	}
	if _, _, ok := scalarElem(t.Elem); ok {
		return nil
	}
	switch e := t.Elem; e.Kind {
	case model.String, model.Enum:
		return tfUnsupported(resource, path+"[]", e)
	case model.Object, model.OneOf:
		if len(e.Fields) == 0 {
			return issue(fmt.Sprintf("%s of empty objects", t.Kind))
		}
		return tfUnsupported(resource, path+"[]", e)
	default:
		return issue(fmt.Sprintf("%s of %s", t.Kind, e.Kind))
	}
}

func mapIssues(resource, path string, t *model.Type) []surveyIssue {
	switch e := t.Elem; {
	case e.Kind == model.String && e.Format == "", e.Kind == model.Enum:
		return nil
	case e.Kind == model.Integer && e.WireString && e.Format == "uint64":
		return nil
	case scalarOK(e):
		return nil
	case (e.Kind == model.Object || e.Kind == model.OneOf) && len(e.Fields) != 0:
		return tfUnsupported(resource, path+"{}", e)
	}
	return []surveyIssue{{resource: resource, path: path, shape: "generator: map of " + typeName(t.Elem)}}
}

func writeSurvey(out io.Writer, resources []string, issues []surveyIssue) error {
	byShape := map[string][]surveyIssue{}
	blocked := map[string]bool{}
	for _, is := range issues {
		byShape[is.shape] = append(byShape[is.shape], is)
		blocked[is.resource] = true
	}
	shapes := sortedKeys(byShape)
	count := func(s string) int {
		rs := map[string]bool{}
		for _, is := range byShape[s] {
			rs[is.resource] = true
		}
		return len(rs)
	}
	sort.SliceStable(shapes, func(i, j int) bool { return count(shapes[i]) > count(shapes[j]) })

	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(tw, "%d resources, %d with no issue\n\n", len(resources), len(resources)-len(blocked))
	fmt.Fprintln(tw, "RESOURCES\tPLACES\tSHAPE\tEXAMPLE")
	for _, s := range shapes {
		fmt.Fprintf(tw, "%d\t%d\t%s\t%s\n", count(s), len(byShape[s]), s, byShape[s][0].path)
	}
	fmt.Fprintln(tw, "\nRESOURCE\tISSUES\tSHAPES")
	for _, r := range resources {
		n := 0
		for _, is := range issues {
			if is.resource == r {
				n++
			}
		}
		fmt.Fprintf(tw, "%s\t%d\t%s\n", r, n, strings.Join(resourceShapes(issues, r), "; "))
	}
	return tw.Flush()
}

// resourceShapes returns the sorted shapes of the issues of resource r.
func resourceShapes(issues []surveyIssue, r string) []string {
	seen := map[string]bool{}
	for _, is := range issues {
		if is.resource == r {
			seen[strings.TrimPrefix(strings.TrimPrefix(is.shape, "model: "), "generator: ")] = true
		}
	}
	return sortedKeys(seen)
}

// supportedNumber follows scalarConv: uint64 as a string, int32 and int64 as
// numbers, float and double.
func supportedNumber(t *model.Type) bool {
	if t.Kind == model.Integer && t.WireString {
		return t.Format == "uint64"
	}
	_, _, ok := scalarElem(t)
	return ok
}

func scalarOK(t *model.Type) bool {
	_, _, ok := scalarElem(t)
	return ok
}
