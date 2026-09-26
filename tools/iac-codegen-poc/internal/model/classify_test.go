package model_test

import (
	"testing"

	"github.com/coralogix/terraform-provider-coralogix/tools/iac-codegen-poc/internal/model"
)

func TestClassify(t *testing.T) {
	for _, c := range []struct {
		create, update, get bool
		want                model.Behavior // "" means rejected
	}{
		{true, true, true, model.Normal},
		{true, false, true, model.Immutable},
		{false, false, true, model.Computed},
		{false, true, true, ""},
		{true, true, false, ""},
		{true, false, false, ""},
		{false, true, false, ""},
		{false, false, false, ""},
	} {
		got, err := model.Classify(c.create, c.update, c.get)
		if c.want == "" {
			if err == nil {
				t.Errorf("Classify(%t, %t, %t) = %q, want an error", c.create, c.update, c.get, got)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("Classify(%t, %t, %t) = %q, %v, want %q", c.create, c.update, c.get, got, err, c.want)
		}
	}
}
