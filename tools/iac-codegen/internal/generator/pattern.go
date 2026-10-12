package generator

import (
	"regexp"
	"regexp/syntax"
	"strconv"
	"strings"
	"unicode/utf8"
)

// patternCall matches a pattern validator of a string, as patternValidator writes it.
var patternCall = regexp.MustCompile("^stringvalidator\\.RegexMatches\\(regexp\\.MustCompile\\((`[^`]*`|\"(?:[^\"\\\\]|\\\\.)*\")\\), \"\"\\)$")

// patternOf returns the pattern of the validators of a string attribute, or nil.
func patternOf(a *tfAttr) *regexp.Regexp {
	for _, v := range a.Validators {
		m := patternCall.FindStringSubmatch(v)
		if m == nil {
			continue
		}
		pattern, err := strconv.Unquote(m[1])
		if err != nil {
			return nil
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil
		}
		return re
	}
	return nil
}

// sampleRuns are values of @{run} in a test run: "acc-" and 8 hex characters. A made-up value
// passes a pattern only when it passes with each of them.
var sampleRuns = []string{"acc-00000000", "acc-ffffffff", "acc-0a1b2c3d"}

// madeUpMatches reports whether the made-up value v, with @{run}, matches re in every run.
func madeUpMatches(re *regexp.Regexp, v string) bool {
	for _, run := range sampleRuns {
		if !re.MatchString(strings.ReplaceAll(v, "@{run}", run)) {
			return false
		}
	}
	return true
}

// patternValue builds a short value that matches re, for a pattern that the made-up value does
// not match: a UUID pattern gives "00000000-0000-0000-0000-000000000000". The update config takes
// the second character of each character class, so the two values differ when a class has two. It
// returns false when the value does not match re or the length range.
func patternValue(re *regexp.Regexp, updated bool, low, high int) (string, bool) {
	tree, err := syntax.Parse(re.String(), syntax.Perl)
	if err != nil {
		return "", false
	}
	pick := 0
	if updated {
		pick = 1
	}
	var b strings.Builder
	writeSample(&b, tree.Simplify(), pick)
	v := b.String()
	n := utf8.RuneCountInString(v)
	if !re.MatchString(v) || n < low || n > high {
		return "", false
	}
	return v, true
}

// writeSample writes a short value of re: the first alternative, the fewest repeats, and one
// repeat for a star, so that a value such as "https?://.*" is not cut to "http://".
func writeSample(b *strings.Builder, re *syntax.Regexp, pick int) {
	switch re.Op {
	case syntax.OpLiteral:
		b.WriteString(string(re.Rune))
	case syntax.OpCharClass:
		b.WriteRune(classRune(re.Rune, pick))
	case syntax.OpAnyChar, syntax.OpAnyCharNotNL:
		b.WriteRune(rune('a' + pick))
	case syntax.OpCapture, syntax.OpPlus, syntax.OpStar, syntax.OpAlternate:
		writeSample(b, re.Sub[0], pick)
	case syntax.OpRepeat:
		for range re.Min {
			writeSample(b, re.Sub[0], pick)
		}
	case syntax.OpConcat:
		for _, sub := range re.Sub {
			writeSample(b, sub, pick)
		}
	}
	// An optional part, an empty match, and an anchor add nothing.
}

// sampleRunes are the characters that a sample prefers, in order: digits, letters, then the
// other printable ASCII characters.
const sampleRunes = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-_.:/@+=,;!#$%&'()*<>?[]^`{|}~\"\\"

// classRune returns the pick-th preferred character of a character class, given as rune ranges.
// A class with fewer preferred characters gives its last one. A class with none gives its first
// rune.
func classRune(ranges []rune, pick int) rune {
	var found []rune
	for _, r := range sampleRunes {
		if inRanges(ranges, r) {
			found = append(found, r)
			if len(found) > pick {
				return r
			}
		}
	}
	if len(found) != 0 {
		return found[len(found)-1]
	}
	return ranges[0]
}

func inRanges(ranges []rune, r rune) bool {
	for i := 0; i+1 < len(ranges); i += 2 {
		if ranges[i] <= r && r <= ranges[i+1] {
			return true
		}
	}
	return false
}
