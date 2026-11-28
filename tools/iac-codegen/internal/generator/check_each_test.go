package generator

import (
	"os"
	"path/filepath"
	"testing"
)

// CheckEach reads the document once. It must give every component the result that Check gives it.
func TestCheckEachGivesEveryComponentTheResultOfCheck(t *testing.T) {
	spec, err := os.ReadFile(filepath.Join("testdata", "always-thing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	path := candidate(t, t.TempDir(), spec)
	got := map[string]string{}
	err = CheckEach(CheckOptions{OpenAPIPath: path}, func(component string, err error) {
		if _, seen := got[component]; seen {
			t.Fatalf("component %s reported twice", component)
		}
		got[component] = errorText(err)
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["AlwaysThing"]; !ok || len(got) < 2 {
		t.Fatalf("components = %v, want every component of the document", got)
	}
	for component, text := range got {
		want := errorText(Check(CheckOptions{Resource: component, OpenAPIPath: path}))
		if text != want {
			t.Errorf("%s: CheckEach = %q, Check = %q", component, text, want)
		}
	}
}

func TestCheckEachTakesNoResource(t *testing.T) {
	if err := CheckEach(CheckOptions{Resource: "AlwaysThing", OpenAPIPath: "x.yaml"}, func(string, error) {}); err == nil {
		t.Fatal("CheckEach with a resource name passed")
	}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
