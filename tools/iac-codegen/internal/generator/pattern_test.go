package generator

import (
	"math"
	"regexp"
	"strings"
	"testing"
)

// Patterns of request fields in the Coralogix management APIs.
var apiPatterns = []string{
	`^[\s\S]+$`,
	`^[\x00-\xFF]*$`,
	`^[\s\S]*\S[\s\S]*$`,
	`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`,
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`,
	`^([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}|[A-Z]+-[0-9]+)$`,
	`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-7[0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`,
	`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(?:-((?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9][0-9]*|[0-9]*[A-Za-z-][0-9A-Za-z-]*))*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`,
	`^([1-9][0-9]{0,6}|[1-5][0-9]{7}|60000000)$`,
	`^\w+$`,
	`^[^@\s]+@[^@\s]+$`,
	`^https?://.*$`,
	`^-?[0-9]+$`,
	`^[0-9]+$`,
	`^[A-Za-z0-9+/]*={0,2}$`,
	`^\d+(\.\d+)?s$`,
	`^[1-9][0-9]*[smhdw]$`,
	`^[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?$`,
	`^csv$`,
}

func TestStringValueMatchesThePattern(t *testing.T) {
	for _, pattern := range apiPatterns {
		re := regexp.MustCompile(pattern)
		a := &tfAttr{Name: "name", Validators: []string{patternValidator(pattern)}}
		for _, updated := range []bool{false, true} {
			v := stringValue(a, updated)
			if !madeUpMatches(re, v) {
				t.Errorf("%s (updated %v): value %q does not match", pattern, updated, v)
			}
		}
	}
}

func TestStringValueKeepsTheMadeUpValueWhenItMatches(t *testing.T) {
	a := &tfAttr{Name: "name", Validators: []string{patternValidator(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)}}
	if got := stringValue(a, false); got != "@{run}-name" {
		t.Errorf("value = %q, want the unique made-up value", got)
	}
}

func TestStringValueFromPatternFitsLengthAndDiffersOnUpdate(t *testing.T) {
	uuid := `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`
	a := &tfAttr{Name: "team_id", Validators: []string{"stringvalidator.LengthBetween(36, 36)", patternValidator(uuid)}}
	created, updated := stringValue(a, false), stringValue(a, true)
	if created != "00000000-0000-0000-0000-000000000000" || updated != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("values = %q and %q, want a UUID of 0s and of 1s", created, updated)
	}
	if _, ok := patternValue(regexp.MustCompile(uuid), false, 0, 10); ok {
		t.Error("a UUID fits a maximum length of 10")
	}
	if _, ok := patternValue(regexp.MustCompile(uuid), false, 0, math.MaxInt); !ok {
		t.Error("no UUID value")
	}
}

func TestPatternValidatorRoundTrips(t *testing.T) {
	for _, pattern := range []string{`^[\s\S]+$`, "^a`b$", `^"[a-z]+"$`} {
		v := patternValidator(pattern)
		if strings.Contains(pattern, "`") == strings.Contains(v, "`"+pattern+"`") {
			t.Errorf("%s: validator %s, want a raw string only without a backquote", pattern, v)
		}
		re := patternOf(&tfAttr{Validators: []string{v}})
		if re == nil || re.String() != pattern {
			t.Errorf("%s: patternOf(%s) = %v", pattern, v, re)
		}
	}
}
