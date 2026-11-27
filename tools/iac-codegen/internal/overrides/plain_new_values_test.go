package overrides

import (
	"errors"
	"testing"
)

// With PlainNewValues, a value that no single style names keeps the plain rule, and a value that a
// style names still takes the style. Released names do not change.
func TestPlainNewValuesNameWhatNoStyleNames(t *testing.T) {
	const head = "resource: R\nmode: existing\nvalidators:\n  inferred: false\nenums:\n"
	tests := map[string]struct {
		line     string
		contract []string
		value    string
		want     string
	}{
		"styles that disagree": {"  AlertDefPriority:\n    zero: P5\n    names: {ALERT_DEF_PRIORITY_P4: P4, ALERT_DEF_PRIORITY_P3: P3}\n",
			[]string{"ALERT_DEF_PRIORITY_UNSPECIFIED", "ALERT_DEF_PRIORITY_P4", "ALERT_DEF_PRIORITY_P3", "ALERT_DEF_PRIORITY_URGENT"}, "ALERT_DEF_PRIORITY_URGENT", "alert_def_priority_urgent"},
		"no style": {"  NotifyOn:\n    zero: Triggered Only\n    names: {NOTIFY_ON_TRIGGERED_AND_RESOLVED: Triggered and Resolved}\n",
			[]string{"NOTIFY_ON_UNSPECIFIED", "NOTIFY_ON_TRIGGERED_AND_RESOLVED", "NOTIFY_ON_NEVER"}, "NOTIFY_ON_NEVER", "notify_on_never"},
		"a style that fits": {"  DayOfWeek:\n    zero: Monday\n    names: {DAY_OF_WEEK_TUESDAY: Tuesday, DAY_OF_WEEK_SUNDAY: Sunday}\n",
			[]string{"DAY_OF_WEEK_UNSPECIFIED", "DAY_OF_WEEK_TUESDAY", "DAY_OF_WEEK_SUNDAY", "DAY_OF_WEEK_HOLIDAY"}, "DAY_OF_WEEK_HOLIDAY", "Holiday"},
		"a released name": {"  AlertDefPriority:\n    zero: P5\n    names: {ALERT_DEF_PRIORITY_P4: P4, ALERT_DEF_PRIORITY_P3: P3}\n",
			[]string{"ALERT_DEF_PRIORITY_UNSPECIFIED", "ALERT_DEF_PRIORITY_P4", "ALERT_DEF_PRIORITY_P3", "ALERT_DEF_PRIORITY_URGENT"}, "ALERT_DEF_PRIORITY_P4", "P4"},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			f, err := Parse([]byte(head + test.line))
			if err != nil {
				t.Fatal(err)
			}
			enum := ""
			for k := range f.Enums {
				enum = k
			}
			f.PlainNewValues = true
			if err := f.FillEnum(enum, test.contract); err != nil {
				var undecided *UndecidedNames
				t.Fatalf("err = %v (undecided: %v), want none", err, errors.As(err, &undecided))
			}
			if got := f.Enums[enum].TFValue(test.value); got != test.want {
				t.Fatalf("TFValue(%s) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}
