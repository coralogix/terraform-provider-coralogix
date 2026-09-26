package specload_test

import (
	"slices"
	"testing"
	"time"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
	"github.com/pb33f/libopenapi/datamodel/high/base"
	v3 "github.com/pb33f/libopenapi/datamodel/high/v3"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/sdkspec"
)

func loadLibopenapi(t *testing.T, data []byte) *v3.Document {
	t.Helper()
	// FilterPathAndValues -> Filters.pathAndValues (array) -> FilterPathAndValues is
	// a loop, but an empty array ends it. By default libopenapi reports it as infinite.
	config := datamodel.NewDocumentConfiguration()
	config.IgnoreArrayCircularReferences = true
	doc, err := libopenapi.NewDocumentWithConfiguration(data, config)
	if err != nil {
		t.Fatalf("new document: %v", err)
	}
	model, err := doc.BuildV3Model()
	if err != nil {
		t.Fatalf("build v3 model: %v", err)
	}
	return &model.Model
}

func libopenapiKeys(s *base.Schema) []string {
	var keys []string
	for k := range s.Properties.KeysFromOldest() {
		keys = append(keys, k)
	}
	return keys
}

func libopenapiBody(t *testing.T, op *v3.Operation) *base.Schema {
	t.Helper()
	if op == nil || op.RequestBody == nil {
		t.Fatal("operation has no request body")
	}
	media := op.RequestBody.Content.GetOrZero("application/json")
	if media == nil || media.Schema == nil {
		t.Fatal("request body has no application/json schema")
	}
	if media.Schema.IsReference() {
		t.Fatalf("request body is a $ref (%s), want inline", media.Schema.GetReference())
	}
	return media.Schema.Schema()
}

// TestLibopenapi checks the parts of the source spec that the generator needs.
func TestLibopenapi(t *testing.T) {
	data, err := sdkspec.Read()
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	doc := loadLibopenapi(t, data)
	t.Logf("load: %s", time.Since(start))

	t.Run("load", func(t *testing.T) {
		if doc.Version != "3.1.0" {
			t.Errorf("version = %q, want 3.1.0", doc.Version)
		}
	})
	collection := doc.Paths.PathItems.GetOrZero(pathCollection)
	item := doc.Paths.PathItems.GetOrZero(pathItem)
	t.Run("operations", func(t *testing.T) { checkOperations(t, collection, item) })
	t.Run("request bodies", func(t *testing.T) { checkRequestBodies(t, collection, item) })
	t.Run("ref resolution", func(t *testing.T) { checkRefResolution(t, collection) })
	t.Run("oneOf", func(t *testing.T) { checkOneOf(t, doc) })
}

// TestLibopenapiSyntax checks the overlay keywords and the 3.1-only syntax on
// a small sample.
func TestLibopenapiSyntax(t *testing.T) {
	sample := loadLibopenapi(t, []byte(overlaySample)).Components.Schemas.GetOrZero("Sample").Schema()
	t.Run("overlay keywords", func(t *testing.T) { checkOverlayKeywords(t, sample) })
	t.Run("3.1 syntax", func(t *testing.T) {
		if got := sampleProp(sample, "nickname").Type; !slices.Equal(got, []string{"string", "null"}) {
			t.Errorf("type = %v, want [string null]", got)
		}
		if got := len(sampleProp(sample, "label").Examples); got != 2 {
			t.Errorf("examples = %d, want 2", got)
		}
	})
	t.Run("property order", func(t *testing.T) {
		if got := libopenapiKeys(sample); !slices.Equal(got, propertyOrder) {
			t.Errorf("order = %v, want %v", got, propertyOrder)
		}
	})
}

func checkOperations(t *testing.T, collection, item *v3.PathItem) {
	if collection == nil || item == nil {
		t.Fatal("paths not found")
	}
	for name, op := range map[string]*v3.Operation{
		"POST": collection.Post, "GET": item.Get, "PATCH": item.Patch, "DELETE": item.Delete,
	} {
		if op == nil {
			t.Errorf("%s: operation not found", name)
		}
	}
}

func checkRequestBodies(t *testing.T, collection, item *v3.PathItem) {
	create := libopenapiBody(t, collection.Post)
	update := libopenapiBody(t, item.Patch)
	if got := libopenapiKeys(create); !sameSet(got, wantCreateProps) {
		t.Errorf("create properties = %v, want %v", got, wantCreateProps)
	}
	if got := libopenapiKeys(update); !sameSet(got, wantUpdateProps) {
		t.Errorf("update properties = %v, want %v", got, wantUpdateProps)
	}
	if got := update.Properties.GetOrZero("updateMask").Schema().Pattern; got == "" {
		t.Error("updateMask pattern is empty")
	}
}

func checkRefResolution(t *testing.T, collection *v3.PathItem) {
	config := libopenapiBody(t, collection.Post).Properties.GetOrZero("config").Schema()
	if len(config.AllOf) != 1 || config.AllOf[0].GetReference() != refEvalConfig {
		t.Fatalf("config allOf = %v, want one $ref to %s", config.AllOf, refEvalConfig)
	}
	evalConfig := config.AllOf[0].Schema()
	sqlLoadProxy := evalConfig.Properties.GetOrZero("sqlLoad")
	if sqlLoadProxy.GetReference() != refSQLLoad {
		t.Fatalf("sqlLoad ref = %q, want %s", sqlLoadProxy.GetReference(), refSQLLoad)
	}
	sqlLoad := sqlLoadProxy.Schema()
	if got := libopenapiKeys(sqlLoad); !sameSet(got, wantSQLLoad) {
		t.Errorf("SqlLoadConfig properties = %v, want %v", got, wantSQLLoad)
	}
	if got := sqlLoad.Properties.GetOrZero("joinLimit").Schema().Type; !slices.Equal(got, []string{"string"}) {
		t.Errorf("joinLimit type = %v, want [string]", got)
	}
}

func checkOneOf(t *testing.T, doc *v3.Document) {
	evalConfig := doc.Components.Schemas.GetOrZero("EvaluationConfig").Schema()
	if len(evalConfig.OneOf) != wantArms+1 {
		t.Fatalf("oneOf entries = %d, want %d", len(evalConfig.OneOf), wantArms+1)
	}
	var arms []string
	for _, entry := range evalConfig.OneOf[:wantArms] {
		s := entry.Schema()
		if len(s.Required) != 1 {
			t.Fatalf("oneOf entry required = %v, want one name", s.Required)
		}
		arms = append(arms, s.Required[0])
	}
	if props := libopenapiKeys(evalConfig); !sameSet(arms, props) {
		t.Errorf("oneOf arms %v do not match properties %v", arms, props)
	}
	none := evalConfig.OneOf[wantArms].Schema()
	if none.Not == nil || len(none.Not.Schema().AnyOf) != wantArms {
		t.Errorf("last oneOf entry is not `not: anyOf` with %d entries", wantArms)
	}
}

func checkOverlayKeywords(t *testing.T, sample *base.Schema) {
	if got := sampleProp(sample, "joinLimit").Format; got != "uint64" {
		t.Errorf("format = %q, want uint64", got)
	}
	if d := sampleProp(sample, "isEnabled").Default; d == nil || d.Value != "false" {
		t.Errorf("default = %v, want false", d)
	}
	topics := sampleProp(sample, "topics")
	if u := topics.UniqueItems; u == nil || !*u {
		t.Errorf("uniqueItems = %v, want true", u)
	}
	if x := topics.Extensions.GetOrZero("x-coralogix-collection"); x == nil || x.Value != "set" {
		t.Errorf("x-coralogix-collection = %v, want set", x)
	}
	if x := sampleProp(sample, "threshold").Extensions.GetOrZero("x-coralogix-presence"); x == nil || x.Value != "true" {
		t.Errorf("x-coralogix-presence = %v, want true", x)
	}
}

func sampleProp(sample *base.Schema, name string) *base.Schema {
	return sample.Properties.GetOrZero(name).Schema()
}

func sameSet(a, b []string) bool {
	a, b = slices.Clone(a), slices.Clone(b)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(a, b)
}
