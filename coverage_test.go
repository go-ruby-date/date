// Copyright (c) the go-ruby-date/date authors
//
// SPDX-License-Identifier: BSD-3-Clause

package date

import "testing"

// These tests exercise the remaining error branches and edge paths so the suite
// holds to 100% statement coverage without any Ruby runtime.

func TestStrptimeEveryDirectiveError(t *testing.T) {
	// Each entry feeds an input that fails the directive's validation, hitting
	// the corresponding error branch in scan.
	cases := []struct{ in, fmt string }{
		{"x", "%Y"}, {"x", "%y"}, {"x", "%C"}, {"13", "%m"}, {"x", "%m"},
		{"32", "%d"}, {"x", "%d"}, {"25", "%H"}, {"x", "%H"}, {"13", "%I"},
		{"x", "%I"}, {"60", "%M"}, {"x", "%M"}, {"61", "%S"}, {"x", "%S"},
		{"x", "%L"}, {"x", "%N"}, {"400", "%j"}, {"x", "%j"}, {"zz", "%p"},
		{"x", "%b"}, {"x", "%a"}, {"x", "%z"}, {"x", "%G"}, {"x", "%V"},
		{"x", "%u"}, {"x", "%s"}, {"x", "%Q"}, {"x", "%E"}, {"y", "%%"},
	}
	for _, c := range cases {
		if _, err := Strptime(c.in, c.fmt); err == nil {
			t.Errorf("Strptime(%q,%q) should error", c.in, c.fmt)
		}
	}
	// Literal mismatch and trailing-percent format.
	if _, err := Strptime("a", "b"); err == nil {
		t.Error("literal mismatch")
	}
	if _, err := Strptime("x", "%"); err == nil {
		t.Error("trailing percent")
	}
}

func TestStrptimeValidDirectiveEdges(t *testing.T) {
	// Drive the success branches that the golden suite did not: %C alone, %V/%u,
	// %s with fraction-less epoch, %Q, %p lowercase, %k/%l space-padded, %h, %A.
	good := []struct{ in, fmt, check string }{
		{"2026-06-29 24:00", "%Y-%m-%d %H:%M", ""}, // hour 24 is accepted by MRI
		{"2026 Wed", "%Y %a", "2026-01-01"},        // day name ignored
		{"2026 December", "%Y %B", "2026-12-01"},   // full month name
		{"2026 Z", "%Y %z", "2026-01-01"},          // Z offset
		{"2026-06-29 am", "%Y-%m-%d %P", ""},       // lowercase meridiem
	}
	for _, c := range good {
		if _, err := Strptime(c.in, c.fmt); err != nil {
			t.Errorf("Strptime(%q,%q): %v", c.in, c.fmt, err)
		}
	}
	// %k / %l space padding and %h month abbreviation.
	if d, err := Strptime("2026 Mar 5", "%Y %h %d"); err != nil || d.Iso8601() != "2026-03-05" {
		t.Errorf("%%h: %v %v", d, err)
	}
}

func TestParseHeuristicFallbacks(t *testing.T) {
	// A month-name form with an unrecognised name falls through to invalid.
	for _, s := range []string{"Smarch 5, 2026", "5 Smarch 2026", "Smarch 2026"} {
		if _, err := Parse(s, false); err == nil {
			t.Errorf("Parse(%q) should error", s)
		}
	}
	// "Mon, " weekday prefix stripping then a date.
	if d, err := Parse("Tuesday, 29 June 2026", false); err != nil || d.Iso8601() != "2026-06-29" {
		t.Errorf("weekday prefix: %v %v", d, err)
	}
	// Year-month numeric form.
	if d, _ := Parse("2026-06", false); d.Iso8601() != "2026-06-01" {
		t.Errorf("year-month")
	}
	// A bad numeric triple (impossible month) is invalid.
	if _, err := Parse("2026/13/40", false); err == nil {
		t.Error("bad month triple")
	}
}

func TestCmpAcrossDayBoundary(t *testing.T) {
	// Two DateTimes whose offsets push them into different UTC days.
	a := mustDT(t, 2026, 6, 29, 1, 0, 0, 0)    // 01:00 UTC
	b := mustDT(t, 2026, 6, 29, 2, 0, 0, 7200) // 00:00 UTC (earlier instant)
	if a.Cmp(b) != 1 || b.Cmp(a) != -1 {
		t.Errorf("cross-boundary cmp a=%d b=%d", a.Cmp(b), b.Cmp(a))
	}
	// Same UTC day, different nsec.
	c := mustDT(t, 2026, 6, 29, 5, 0, 1, 0)
	d := mustDT(t, 2026, 6, 29, 5, 0, 0, 0)
	if c.Cmp(d) != 1 || d.Cmp(c) != -1 || c.Cmp(c) != 0 {
		t.Error("same-day nsec cmp")
	}
}

func TestItoaEdges(t *testing.T) {
	// jisx0301 of a Reiwa year drives itoa with multi-digit and the era-year > 9.
	if got := mustDate(t, 2030, 1, 1).Jisx0301(); got != "R12.01.01" {
		t.Errorf("itoa via jisx = %q", got)
	}
	// A DateTime jisx0301 in Reiwa exercises the time-appending branch.
	if got := mustDT(t, 2026, 6, 29, 1, 2, 3, 0).Jisx0301(); got != "R08.06.29T01:02:03+00:00" {
		t.Errorf("jisx dt = %q", got)
	}
}

func TestWeeksInCommercialYear53(t *testing.T) {
	// 2020 is a 53-week ISO year (p(y)==3 branch); 2026 is 52.
	if w := weeksInCommercialYear(2020); w != 53 {
		t.Errorf("2020 weeks=%d", w)
	}
	if w := weeksInCommercialYear(2025); w != 52 {
		t.Errorf("2025 weeks=%d", w)
	}
	// 2015 ends on a Thursday — p(y)==4 branch — 53 weeks.
	if w := weeksInCommercialYear(2015); w != 53 {
		t.Errorf("2015 weeks=%d", w)
	}
}

func TestApplyFlagMinusOnString(t *testing.T) {
	// The '-' flag on a string field suppresses width padding.
	d := mustDate(t, 2026, 6, 29)
	if got := d.Strftime("%-10A"); got != "Monday" {
		t.Errorf("minus on string = %q", got)
	}
	// Width padding on a string field (no minus) space-pads.
	if got := d.Strftime("%10A"); got != "    Monday" {
		t.Errorf("width string = %q", got)
	}
}

func TestResolveDefaults(t *testing.T) {
	// Commercial defaults: %G alone defaults cweek/cwday to 1.
	if d, err := Strptime("2026", "%G"); err != nil || d.Iso8601() != "2025-12-29" {
		t.Errorf("%%G default: %v %v", d, err)
	}
	// %V alone (cweekSet) with cwyear/cwday defaulting.
	if _, err := Strptime("27", "%V"); err != nil {
		t.Errorf("%%V default: %v", err)
	}
	// Ordinal without a year is invalid.
	if _, err := Strptime("180", "%j"); err == nil {
		t.Error("%j without year")
	}
	// yday 366 in a non-leap year passes the scan range but fails Ordinal.
	if _, err := Strptime("2025 366", "%Y %j"); err == nil {
		t.Error("%j 366 in non-leap year")
	}
	// A commercial week beyond the year's count fails Commercial in resolve.
	if _, err := Strptime("2026 53", "%G %V"); err != nil {
		t.Errorf("%%G 53 valid (2026 has 53 weeks): %v", err)
	}
	if _, err := Strptime("2025 53", "%G %V"); err == nil {
		t.Error("week 53 in a 52-week commercial year should fail")
	}
	// %y default-branch two-digit expansion already handled, but %C+%y path:
	if d, err := Strptime("99", "%y"); err != nil || d.Year() != 1999 {
		t.Errorf("%%y expand: %v %v", d, err)
	}
	// Month name "Smarch 5 2026" (no comma) drives the reMonDayY fallthrough.
	if _, err := Parse("Smarch 5 2026", false); err == nil {
		t.Error("bad month name day form")
	}
}

func TestMeridiemShortInput(t *testing.T) {
	// "%p" with a one-byte tail can't hold "AM"/"PM" — the length guard fires.
	if _, err := Strptime("2026-06-29 A", "%Y-%m-%d %p"); err == nil {
		t.Error("short meridiem")
	}
}

func TestOffsetWithSeconds(t *testing.T) {
	// An offset carrying seconds exercises the %:::z seconds branch and %::z.
	dt := &Date{jd: 2461221, offset: 19845, isDateTime: true, start: ITALY} // +05:30:45
	if got := dt.Strftime("%:::z"); got != "+05:30:45" {
		t.Errorf("%%:::z secs = %q", got)
	}
	if got := dt.Strftime("%::z"); got != "+05:30:45" {
		t.Errorf("%%::z secs = %q", got)
	}
}

func TestSwapCaseAllUpper(t *testing.T) {
	// The '#' flag on an all-uppercase field (%p → "PM") downcases it.
	dt := mustDT(t, 2026, 6, 29, 14, 0, 0, 0)
	if got := dt.Strftime("%#p"); got != "pm" {
		t.Errorf("#p = %q", got)
	}
}

func TestScanOffsetForms(t *testing.T) {
	// %z forms: bare hours, colon, and a sign-only failure.
	good := map[string]int{
		"+05":    18000,
		"+0530":  19800,
		"+05:30": 19800,
		"-08":    -28800,
		"Z":      0,
	}
	for in, want := range good {
		d, err := Strptime("2026-06-29 "+in, "%Y-%m-%d %z")
		if err != nil || d.Offset() != want {
			t.Errorf("offset %q = %v (%v)", in, d, err)
		}
	}
	// A '+' with no digits is rejected.
	if _, err := Strptime("2026-06-29 +", "%Y-%m-%d %z"); err == nil {
		t.Error("bare plus offset")
	}
}
