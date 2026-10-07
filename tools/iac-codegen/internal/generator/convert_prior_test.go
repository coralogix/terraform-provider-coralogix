package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetFlattenTemplateMatchesPriorByIdentity(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("templates", "convert.go.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.Contains(text, "matchPriorSetItem") {
		t.Fatal("set flatten with NeedsPrior must match prior items by identity, not slice index")
	}
	if !strings.Contains(text, "sameUnordered") {
		t.Fatal("same* must compare nested keepPriorOrder lists without regard to order")
	}
}

func TestSameCheckComparesKeepPriorOrderListsUnordered(t *testing.T) {
	got, err := sameCheck(&convField{
		Conv: convObjects, SDK: "Targets", KeepPriorOrder: true,
		Object: &convObject{Func: "LegacyTarget"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "sameUnordered(a.Targets, b.Targets, sameLegacyTarget)"
	if got != want {
		t.Fatalf("sameCheck = %q, want %q", got, want)
	}
}
