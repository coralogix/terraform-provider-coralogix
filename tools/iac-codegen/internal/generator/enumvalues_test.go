package generator

import (
	"slices"
	"testing"
)

// The values of the description come from the data that was recorded with each validator, not from
// the text of the validator: a validator spelled any way lists its values, one without a record
// lists none, and a validator that was removed from the attribute lists none either.
func TestEnumValidatorValuesComeFromRecordedData(t *testing.T) {
	a := &tfAttr{EnumSchema: "Kind", ValueKind: "String", Validators: []string{"checkKind()"}}
	a.recordEnumValues("checkKind()", []string{"a", "b", "c"})
	values, ok := enumValidatorValues(a)
	if !ok || !slices.Equal(values, []string{"a", "b", "c"}) {
		t.Fatalf("values = %v, %v, want a, b, c from the record", values, ok)
	}
	a.Validators = append(a.Validators, "stringvalidator.OneOf(\"x\")")
	if values, _ := enumValidatorValues(a); !slices.Equal(values, []string{"a", "b", "c"}) {
		t.Fatalf("values = %v, want a validator without a record to add nothing", values)
	}
	a.recordEnumValues("second()", []string{"b", "c", "d"})
	a.Validators = append(a.Validators, "second()")
	if values, _ := enumValidatorValues(a); !slices.Equal(values, []string{"b", "c"}) {
		t.Fatalf("values = %v, want the values that both validators accept", values)
	}
	a.Validators = nil
	if values, ok := enumValidatorValues(a); ok || values != nil {
		t.Fatalf("values = %v, %v, want none for an attribute without validators", values, ok)
	}
}
