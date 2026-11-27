package generator

import "testing"

// The scalar arm of the inner object of an inline arm is read from an empty inner value when no
// earlier arm has its merged field. The read writes the merged field, whatever its Go name.
func TestInlineArmReadsAFreshInnerScalarArm(t *testing.T) {
	inner := &convObject{SDK: "InnerSelection"}
	arm := &typedArm{Inner: inner}
	scalar := &convField{TFName: "value", Model: "Value"}
	ia := &typedArm{Scalar: scalar, InitScalar: true}
	arm.addInnerScalar(ia, []typedPair{{Arm: "Selected", Merged: "Selected"}})
	if len(arm.InnerScalars) != 0 {
		t.Fatal("a scalar arm whose field is not fresh must not be read")
	}
	arm.addInnerScalar(ia, []typedPair{{Arm: "Value", Merged: "Merged"}})
	if len(arm.InnerScalars) != 1 || arm.InnerScalars[0].Scalar.Model != "Merged" || arm.InnerScalars[0].SDK != "InnerSelection" {
		t.Fatalf("inner scalars = %+v", arm.InnerScalars)
	}
	if scalar.Model != "Value" {
		t.Error("the arm of the inner object must keep its own Go name")
	}
	arm.addInnerScalar(&typedArm{Scalar: scalar}, []typedPair{{Arm: "Value", Merged: "Merged"}})
	if len(arm.InnerScalars) != 1 {
		t.Error("only the first scalar arm reads the field")
	}
}
