package issue

import (
	"strings"
	"testing"
)

func TestReportIsDeduplicatedAndSorted(t *testing.T) {
	a := Issue{Code: "B", Location: "z", Message: "second", Remediation: "fix"}
	b := Issue{Code: "A", Location: "a", Message: "first", Remediation: "fix"}
	report := Report{a, b, a}.Normalize()
	if len(report) != 2 || report[0] != b || report[1] != a {
		t.Fatalf("normalized report = %#v", report)
	}
	text := report.Error()
	if !strings.Contains(text, "2 problems") || strings.Index(text, "A a") > strings.Index(text, "B z") {
		t.Fatalf("unexpected report:\n%s", text)
	}
}
