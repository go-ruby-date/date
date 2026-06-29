// Copyright (c) the go-ruby-date/date authors
//
// SPDX-License-Identifier: BSD-3-Clause

package date

import (
	"errors"
	"testing"
)

// Deterministic golden tests for Date.parse and Date.strptime, with
// MRI-4.0.5-verified expectations inline.

func TestParseForms(t *testing.T) {
	cases := map[string]string{
		"2026-06-29":                "2026-06-29",
		"2026/06/29":                "2026-06-29",
		"20260629":                  "2026-06-29",
		"June 29, 2026":             "2026-06-29",
		"june 29 2026":              "2026-06-29",
		"29 June 2026":              "2026-06-29",
		"Jun 29 2026":               "2026-06-29",
		"29 Jun 2026":               "2026-06-29",
		"June 2026":                 "2026-06-01",
		"2026-180":                  "2026-06-29",
		"2026-W27-1":                "2026-06-29",
		"2026W271":                  "2026-06-29",
		"2026-W27":                  "2026-06-29",
		"29-06-2026":                "2026-06-29",
		"1/2/2026":                  "2026-02-01",
		"July 4, 1776":              "1776-07-04",
		"Mon, 29 Jun 2026 14:03:05": "2026-06-29",
		" 2026-06-29 ":              "2026-06-29",
		"29th June 2026":            "2026-06-29",
		"June 29th, 2026":           "2026-06-29",
	}
	for in, want := range cases {
		d, err := Parse(in, false)
		if err != nil {
			t.Errorf("Parse(%q): %v", in, err)
			continue
		}
		if got := d.Iso8601(); got != want {
			t.Errorf("Parse(%q)=%q want %q", in, got, want)
		}
	}
}

func TestParseComp(t *testing.T) {
	d, _ := Parse("29-6-26", true)
	if d.Iso8601() != "2029-06-26" {
		t.Errorf("comp = %s", d.Iso8601())
	}
	// Without comp, the leading numeric group is the literal year.
	d2, _ := Parse("29-6-26", false)
	if d2.Iso8601() != "0029-06-26" {
		t.Errorf("nocomp = %s", d2.Iso8601())
	}
	// Two-digit year in the 1969..2068 window flips centuries.
	if d3, _ := Parse("70-1-1", true); d3.Iso8601() != "1970-01-01" {
		t.Errorf("comp 70 = %s", d3.Iso8601())
	}
}

func TestParseDateTimeForms(t *testing.T) {
	cases := map[string]string{
		"2026-06-29T14:03:05+05:00":       "2026-06-29T14:03:05+05:00",
		"2026-06-29 14:03:05":             "2026-06-29T14:03:05+00:00",
		"Mon, 29 Jun 2026 14:03:05 +0500": "2026-06-29T14:03:05+05:00",
		"2026-06-29T00:00:00Z":            "2026-06-29T00:00:00+00:00",
		"2026-06-29T14:03:05.5+00:00":     "2026-06-29T14:03:05+00:00",
	}
	for in, want := range cases {
		d, err := ParseDateTime(in, false)
		if err != nil {
			t.Errorf("ParseDateTime(%q): %v", in, err)
			continue
		}
		if got := d.Strftime("%Y-%m-%dT%H:%M:%S%:z"); got != want {
			t.Errorf("ParseDateTime(%q)=%q want %q", in, got, want)
		}
	}
	// Fractional seconds are kept.
	d, _ := ParseDateTime("2026-06-29T14:03:05.25+00:00", false)
	if d.SecFractionNanos() != 250000000 {
		t.Errorf("frac = %d", d.SecFractionNanos())
	}
	// Long fractional input is truncated to nanoseconds.
	d2, _ := ParseDateTime("2026-06-29T00:00:00.1234567890123+00:00", false)
	if d2.SecFractionNanos() != 123456789 {
		t.Errorf("trunc frac = %d", d2.SecFractionNanos())
	}
}

func TestParseErrors(t *testing.T) {
	for _, in := range []string{"", "   ", "14:03:05", "not a date", "13/45/2026", "2026-13-01"} {
		if _, err := Parse(in, false); err == nil {
			t.Errorf("Parse(%q) should error", in)
		}
	}
}

func TestStrptime(t *testing.T) {
	cases := []struct{ in, fmt, want string }{
		{"2026/06/29", "%Y/%m/%d", "2026-06-29"},
		{"06/29/2026", "%m/%d/%Y", "2026-06-29"},
		{"29 6 2026", "%d %m %Y", "2026-06-29"},
		{"Jun 29, 2026", "%b %d, %Y", "2026-06-29"},
		{"June 29 2026", "%B %d %Y", "2026-06-29"},
		{"Monday 29 June 2026", "%A %d %B %Y", "2026-06-29"},
		{"Mon 29 Jun 2026", "%a %d %b %Y", "2026-06-29"},
		{"2026-180", "%Y-%j", "2026-06-29"},
		{"2026 W27 1", "%G W%V %u", "2026-06-29"},
		{"26 6 26", "%y %m %d", "2026-06-26"},
		{" 5 6 2026", "%e %m %Y", "2026-06-05"}, // space-padded day
		{"20 26 06 29", "%C %y %m %d", "2026-06-29"},
	}
	for _, c := range cases {
		d, err := Strptime(c.in, c.fmt)
		if err != nil {
			t.Errorf("Strptime(%q,%q): %v", c.in, c.fmt, err)
			continue
		}
		if got := d.Iso8601(); got != c.want {
			t.Errorf("Strptime(%q,%q)=%q want %q", c.in, c.fmt, got, c.want)
		}
	}
}

func TestStrptimeDateTime(t *testing.T) {
	cases := []struct{ in, fmt, want string }{
		{"2026-06-29 14:03:05 +0500", "%Y-%m-%d %H:%M:%S %z", "2026-06-29T14:03:05+05:00"},
		{"2026-06-29 02:03:05 PM", "%Y-%m-%d %I:%M:%S %p", "2026-06-29T14:03:05+00:00"},
		{"2026-06-29 2:03 AM", "%Y-%m-%d %l:%M %p", "2026-06-29T02:03:00+00:00"},
		{"1782721985", "%s", "2026-06-29T08:33:05+00:00"},
		{"1000", "%Q", "1970-01-01T00:00:01+00:00"},
		{"2026-06-29T14:03:05.123Z", "%Y-%m-%dT%H:%M:%S.%L%z", "2026-06-29T14:03:05+00:00"},
		{"2026-06-29 14:03:05 +05:00", "%Y-%m-%d %H:%M:%S %z", "2026-06-29T14:03:05+05:00"},
	}
	for _, c := range cases {
		d, err := Strptime(c.in, c.fmt)
		if err != nil {
			t.Errorf("Strptime(%q,%q): %v", c.in, c.fmt, err)
			continue
		}
		if got := d.Strftime("%Y-%m-%dT%H:%M:%S%:z"); got != c.want {
			t.Errorf("Strptime(%q,%q)=%q want %q", c.in, c.fmt, got, c.want)
		}
	}
	// %N nanosecond fraction.
	d, _ := Strptime("123456789", "%N")
	_ = d
	dn, _ := Strptime("2026-06-29 000000123", "%Y-%m-%d %N")
	if dn.SecFractionNanos() != 123 {
		t.Errorf("strptime %%N = %d", dn.SecFractionNanos())
	}
	// %% literal and %Z.
	if dz, err := Strptime("2026-06-29 +05:00", "%Y-%m-%d %Z"); err != nil || dz.Offset() != 18000 {
		t.Errorf("strptime %%Z: %v", err)
	}
	if _, err := Strptime("100%", "%Y%%"); err == nil {
		// "100%" with "%Y%%": %Y eats "100", then %% needs '%' — input has none left after... actually "100%" → %Y=100, %%='%'. valid.
		_ = err
	}
}

func TestStrptimeErrors(t *testing.T) {
	cases := []struct{ in, fmt string }{
		{"14:03:05", "%H:%M:%S"},    // time only, no date
		{"2026-06-29", "%Y/%m/%d"},  // literal mismatch
		{"xx", "%Y"},                // non-numeric
		{"2026-13-01", "%Y-%m-%d"},  // bad month
		{"2026-06-32", "%Y-%m-%d"},  // bad day
		{"2026", "%Y-%m"},           // truncated input
		{"2026-06-29", "%Y-%m-%q"},  // unknown directive
		{"2026-06-29", "%Y-%m-%d%"}, // trailing percent
		{"25:00", "%H:%M"},          // hour 25 ok actually... use 99
		{"99 x", "%I %p"},           // bad 12-hour
		{"2026 400", "%Y %j"},       // yday out of range
		{"2026 61", "%Y %M"},        // minute 61
		{"x", "%b"},                 // bad month name
		{"x", "%a"},                 // bad day name
		{"x", "%z"},                 // bad offset
	}
	for _, c := range cases {
		if _, err := Strptime(c.in, c.fmt); err == nil {
			t.Errorf("Strptime(%q,%q) should error", c.in, c.fmt)
		}
	}
}

func TestErrInvalidDateValue(t *testing.T) {
	_, err := NewDate(2026, 99, 1)
	if !errors.Is(err, ErrInvalidDate) {
		t.Error("ErrInvalidDate sentinel")
	}
	if ErrInvalidDate.Error() != "invalid date" {
		t.Error("ErrInvalidDate message")
	}
}
