// Package issue defines deterministic eligibility reports.
package issue

import (
	"fmt"
	"slices"
	"strings"
)

// Issue describes one fact that prevents deterministic generation.
type Issue struct {
	Code        string
	Location    string
	Message     string
	Remediation string
}

// Report is a set of eligibility issues.
type Report []Issue

// Normalize removes identical issues and sorts the report by location and code.
func (r Report) Normalize() Report {
	seen := make(map[Issue]bool, len(r))
	out := make(Report, 0, len(r))
	for _, item := range r {
		if seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	slices.SortFunc(out, func(a, b Issue) int {
		if n := strings.Compare(a.Location, b.Location); n != 0 {
			return n
		}
		if n := strings.Compare(a.Code, b.Code); n != 0 {
			return n
		}
		if n := strings.Compare(a.Message, b.Message); n != 0 {
			return n
		}
		return strings.Compare(a.Remediation, b.Remediation)
	})
	return out
}

// Error formats all issues as one stable eligibility report.
func (r Report) Error() string {
	r = r.Normalize()
	if len(r) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Resource is not eligible for Terraform generation: %d problems\n", len(r))
	for _, item := range r {
		fmt.Fprintf(&b, "\n%s %s\n  %s\n  %s\n", item.Code, item.Location, item.Message, item.Remediation)
	}
	return strings.TrimSuffix(b.String(), "\n")
}
