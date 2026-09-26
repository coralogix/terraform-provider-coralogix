package main

import (
	"bytes"
	"flag"
	"os"
	"strings"
	"sync"
	"testing"

	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"
	"golang.org/x/tools/go/packages"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/model"
)

var update = flag.Bool("update", false, "rewrite the golden files")

const (
	patchedSpec = "../../spec/openapi.patched.yaml"
	namesGolden = "testdata/sdk_names.golden"
	// pinnedSDK is the SDK version in README.md, "Pinned versions".
	pinnedSDK = "v1.9.4-0.20260908121026-582cbc8f62f3"
)

// TestSDKNames checks the names against the pinned SDK and compares the list
// with the golden file. To rewrite the file, run:
// go test ./cmd/tfgen -run TestSDKNames -update
func TestSDKNames(t *testing.T) {
	_, refs, err := checkedSDKNames(patchedSpec, "AiEvaluation", realSDK)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := writeSDKNames(&buf, refs); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if *update {
		if err := os.WriteFile(namesGolden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(namesGolden)
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("SDK names differ from %s. Run with -update and check the diff.\ngot:\n%s", namesGolden, got)
	}
}

func TestPinnedSDKVersion(t *testing.T) {
	_, pkgs := testSDK(t)
	for path, p := range pkgs {
		if p.Module == nil || p.Module.Version != pinnedSDK {
			t.Errorf("%s: module %+v, want version %s", path, p.Module, pinnedSDK)
		}
	}
}

// TestSDKNamesMissing changes the model so that one SDK name does not match.
// The error must name the model path and the expected SDK name.
func TestSDKNamesMissing(t *testing.T) {
	cases := []struct {
		name   string
		change func(r *model.Resource)
		want   string
	}{
		{
			name:   "field",
			change: func(r *model.Resource) { nestedField(t, r, "config", "sqlLoad", "joinLimit").Name = "joinLimits" },
			want:   "fields.config.sqlLoad.joinLimits: SDK field SqlLoadConfig.JoinLimits: not found",
		},
		{
			name: "field type",
			change: func(r *model.Resource) {
				f := nestedField(t, r, "config", "sqlLoad", "cteLimit")
				f.Type = &model.Type{Kind: model.Integer, Format: "int64"}
			},
			want: "fields.config.sqlLoad.cteLimit: SDK field SqlLoadConfig.CteLimit: type is *string, want *int64",
		},
		{
			name: "enum value",
			change: func(r *model.Resource) {
				f := topField(t, r, "target")
				f.Type = &model.Type{Kind: model.Enum, Schema: f.Type.Schema, Values: []string{"PROMPT", "FOO"}}
			},
			want: "fields.target.FOO: SDK const EVALUATIONTARGET_FOO: not found",
		},
		{
			name:   "operation",
			change: func(r *model.Resource) { r.Get.OperationID = "AiEvaluationsService_FetchAiEvaluation" },
			want:   "get: SDK method AIEvaluationsServiceAPIService.AiEvaluationsServiceFetchAiEvaluation: not found",
		},
	}
	doc, pkgs := testSDK(t)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := model.Build(doc, "AiEvaluation")
			if err != nil {
				t.Fatal(err)
			}
			c.change(r)
			refs, err := resolveSDKNames(r, "AI Evaluations Service", realSDK)
			if err != nil {
				t.Fatal(err)
			}
			err = checkSDKNames(refs, pkgs)
			if err == nil {
				t.Fatal("no error")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error:\n%v\nwant it to contain:\n%s", err, c.want)
			}
		})
	}
}

func TestCamelize(t *testing.T) {
	cases := map[string]string{
		"AI Evaluations Service":                  "AIEvaluationsService",
		"AiEvaluationsService_CreateAiEvaluation": "AiEvaluationsServiceCreateAiEvaluation",
		"v3.FilterOperator":                       "V3FilterOperator",
		"joinLimit":                               "JoinLimit",
	}
	for in, want := range cases {
		if got := camelize(in); got != want {
			t.Errorf("camelize(%q) = %q, want %q", in, got, want)
		}
	}
}

var specOnce = sync.OnceValues(func() (*v3.Document, error) {
	data, err := os.ReadFile(patchedSpec)
	if err != nil {
		return nil, err
	}
	return model.Load(data)
})

var pkgsOnce = sync.OnceValues(func() (map[string]*packages.Package, error) {
	doc, err := specOnce()
	if err != nil {
		return nil, err
	}
	r, err := model.Build(doc, "AiEvaluation")
	if err != nil {
		return nil, err
	}
	refs, err := resolveSDKNames(r, "AI Evaluations Service", realSDK)
	if err != nil {
		return nil, err
	}
	return loadSDK(refs)
})

// testSDK loads the patched spec and the pinned SDK packages once per test run.
func testSDK(t *testing.T) (*v3.Document, map[string]*packages.Package) {
	t.Helper()
	doc, err := specOnce()
	if err != nil {
		t.Fatal(err)
	}
	pkgs, err := pkgsOnce()
	if err != nil {
		t.Fatal(err)
	}
	return doc, pkgs
}

func topField(t *testing.T, r *model.Resource, name string) *model.ResourceField {
	t.Helper()
	for _, f := range r.Fields {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("no field %s", name)
	return nil
}

// nestedField finds top.path[0].path[1]...
func nestedField(t *testing.T, r *model.Resource, top string, path ...string) *model.Field {
	t.Helper()
	typ := topField(t, r, top).Type
	var found *model.Field
	for _, name := range path {
		found = nil
		for _, f := range typ.Fields {
			if f.Name == name {
				found = f
			}
		}
		if found == nil {
			t.Fatalf("no field %s", name)
		}
		typ = found.Type
	}
	return found
}

// TestRealReplaceViewFolder checks a real full-replace resource (E11)
// against the pinned SDK: ViewFolder is a PUT on the collection path with the
// id in the body. It also checks two SDK naming rules that the fakes do not
// have: an inline body with a title (F45), and a cxsdk accessor that is not
// the tag without " Service" (F17).
func TestRealReplaceViewFolder(t *testing.T) {
	r, refs, err := checkedSDKNames(patchedSpec, "ViewFolder", realSDK)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Replace || !r.IDInBody {
		t.Errorf("replace %t, id in body %t; want both", r.Replace, r.IDInBody)
	}
	want := map[string]string{
		"create.body": "CreateViewFolderRequest", // the title
		"update.body": "ViewFolder1",             // the title is a component name
	}
	for _, ref := range refs {
		if w, ok := want[ref.Path]; ok && ref.Kind == kindType && ref.Name != w {
			t.Errorf("%s: type %s, want %s", ref.Path, ref.Name, w)
		}
		if ref.Path == "resource" && ref.Kind == kindMethod && ref.Owner == "ClientSet" && ref.Name != "ViewsFolders" {
			t.Errorf("accessor %s, want ViewsFolders (found by type)", ref.Name)
		}
	}
}

func TestInlineBodyName(t *testing.T) {
	for _, c := range []struct {
		op   model.Operation
		want string
	}{
		{model.Operation{}, "SCreateThingRequest"},
		{model.Operation{BodyTitle: "CreateThingRequest"}, "CreateThingRequest"},
		{model.Operation{BodyTitle: "Thing", BodyTitleIsComponent: true}, "Thing1"},
	} {
		if got := inlineBodyName("SCreateThing", c.op); got != c.want {
			t.Errorf("%+v: %s, want %s", c.op, got, c.want)
		}
	}
}
