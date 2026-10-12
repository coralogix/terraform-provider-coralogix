package generator

import (
	"testing"
)

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
