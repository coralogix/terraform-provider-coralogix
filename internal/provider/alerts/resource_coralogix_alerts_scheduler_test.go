// Copyright 2024 Coralogix Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package alerts

import (
	"testing"
	"time"
)

func TestNormalizeStartTimeFromAPI(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"canonical", "2021-01-04T00:00:00.000", "2021-01-04T00:00:00.000"},
		{"with Z suffix", "2021-01-04T00:00:00.000Z", "2021-01-04T00:00:00.000"},
		{"no millis with Z", "2021-01-04T00:00:00Z", "2021-01-04T00:00:00.000"},
		{"no millis", "2021-01-04T00:00:00", "2021-01-04T00:00:00.000"},
		{"RFC3339", "2021-01-04T00:00:00Z", "2021-01-04T00:00:00.000"},
		{"RFC3339 with offset", "2021-01-04T02:00:00+02:00", "2021-01-04T00:00:00.000"},
		{"RFC3339Nano", "2021-01-04T00:00:00.123456789Z", "2021-01-04T00:00:00.123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeStartTimeFromAPI(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeStartTimeFromAPI(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNormalizeStartTimeFromAPI_InvalidReturnsUnchanged(t *testing.T) {
	invalid := "not-a-time"
	got := normalizeStartTimeFromAPI(invalid)
	if got != invalid {
		t.Errorf("normalizeStartTimeFromAPI(%q) = %q, want unchanged %q", invalid, got, invalid)
	}
}

func TestParseStartTime(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantOK    bool
		wantYear  int
		wantMonth int
		wantDay   int
	}{
		{"canonical", "2021-01-04T00:00:00.000", true, 2021, 1, 4},
		{"with Z", "2021-01-04T00:00:00.000Z", true, 2021, 1, 4},
		{"no millis", "2021-01-04T00:00:00", true, 2021, 1, 4},
		{"no millis Z", "2021-01-04T00:00:00Z", true, 2021, 1, 4},
		{"RFC3339 offset", "2021-01-04T02:00:00+02:00", true, 2021, 1, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseStartTime(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("parseStartTime(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if !tt.wantOK {
				return
			}
			y, m, d := got.Date()
			if y != tt.wantYear || m != time.Month(tt.wantMonth) || d != tt.wantDay {
				t.Errorf("parseStartTime(%q) date = %d-%02d-%02d, want %d-%02d-%02d",
					tt.input, y, m, d, tt.wantYear, tt.wantMonth, tt.wantDay)
			}
		})
	}
}

func TestParseStartTime_Invalid(t *testing.T) {
	_, ok := parseStartTime("not-a-time")
	if ok {
		t.Error("parseStartTime(\"not-a-time\") wanted ok=false")
	}
}

func TestStartTimeSemanticallyEqual(t *testing.T) {
	tests := []struct {
		name string
		a    string
		b    string
		want bool
	}{
		{"same string", "2021-01-04T00:00:00.000", "2021-01-04T00:00:00.000", true},
		{"canonical vs Z", "2021-01-04T00:00:00.000", "2021-01-04T00:00:00.000Z", true},
		{"Z vs no millis", "2021-01-04T00:00:00Z", "2021-01-04T00:00:00", true},
		{"different instant", "2021-01-04T00:00:00.000", "2021-01-04T01:00:00.000", false},
		{"invalid a", "invalid", "2021-01-04T00:00:00.000", false},
		{"invalid b", "2021-01-04T00:00:00.000", "invalid", false},
		{"both invalid same string", "invalid", "invalid", true}, // a == b shortcut
		{"both invalid different", "invalid1", "invalid2", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := startTimeSemanticallyEqual(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("startTimeSemanticallyEqual(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}
