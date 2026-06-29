// Copyright (c) the go-ruby-date/date authors
//
// SPDX-License-Identifier: BSD-3-Clause

package date

import (
	"errors"
	"testing"
)

// These deterministic, Ruby-free golden tests carry MRI-4.0.5-verified
// expectations inline, so they hold to 100% on the no-Ruby / qemu / Windows CI
// lanes. "Today" is always injected (SetToday) — never real time.

func mustDate(t *testing.T, y, m, d int) *Date {
	t.Helper()
	dd, err := NewDate(y, m, d)
	if err != nil {
		t.Fatalf("NewDate(%d,%d,%d): %v", y, m, d, err)
	}
	return dd
}

func mustDT(t *testing.T, y, mo, d, h, mi, s, off int) *Date {
	t.Helper()
	dd, err := NewDateTime(y, mo, d, h, mi, s, off)
	if err != nil {
		t.Fatalf("NewDateTime: %v", err)
	}
	return dd
}

func TestConstructorsAndAccessors(t *testing.T) {
	d := mustDate(t, 2026, 6, 29)
	if d.Jd() != 2461221 {
		t.Errorf("Jd=%d want 2461221", d.Jd())
	}
	if d.Mjd() != 61220 {
		t.Errorf("Mjd=%d", d.Mjd())
	}
	if d.Year() != 2026 || d.Month() != 6 || d.Day() != 29 {
		t.Errorf("ymd=%d-%d-%d", d.Year(), d.Month(), d.Day())
	}
	if d.Wday() != 1 || d.Cwday() != 1 || d.Yday() != 180 {
		t.Errorf("wday=%d cwday=%d yday=%d", d.Wday(), d.Cwday(), d.Yday())
	}
	if d.Cwyear() != 2026 || d.Cweek() != 27 {
		t.Errorf("cwyear=%d cweek=%d", d.Cwyear(), d.Cweek())
	}
	if d.Leap() {
		t.Error("2026 not leap")
	}
	if !mustDate(t, 2024, 2, 29).Leap() {
		t.Error("2024 leap")
	}

	// Negative month/day count from the end.
	if got := mustDateStr(t, -1, -1); got != "2026-12-31" {
		t.Errorf("neg m/d = %s", got)
	}

	// Date.jd / ordinal / commercial.
	if DateJD(2451545).String() != "2000-01-01" {
		t.Errorf("DateJD")
	}
	o, _ := Ordinal(2026, 180)
	if o.String() != "2026-06-29" {
		t.Errorf("Ordinal=%s", o.String())
	}
	if _, err := Ordinal(2026, 400); !errors.Is(err, ErrInvalidDate) {
		t.Error("Ordinal overflow")
	}
	if oo, _ := Ordinal(2024, -1); oo.String() != "2024-12-31" {
		t.Errorf("Ordinal neg=%s", oo.String())
	}
	c, _ := Commercial(2026, 1, 1)
	if c.String() != "2025-12-29" {
		t.Errorf("Commercial=%s", c.String())
	}
	if cc, _ := Commercial(2026, 1, -1); cc.String() != "2026-01-04" {
		t.Errorf("Commercial neg cwday=%s", cc.String())
	}
	if cw, _ := Commercial(2020, -1, 1); cw.String() != "2020-12-28" {
		t.Errorf("Commercial neg cweek=%s", cw.String())
	}
}

func mustDateStr(t *testing.T, m, d int) string {
	dd, err := NewDate(2026, m, d)
	if err != nil {
		t.Fatalf("NewDate: %v", err)
	}
	return dd.String()
}

func TestConstructorErrors(t *testing.T) {
	bad := [][3]int{{2026, 13, 1}, {2026, 0, 1}, {2026, 2, 30}, {2026, 1, 32}, {2026, 1, 0}, {1582, 10, 10}}
	for _, b := range bad {
		if _, err := NewDate(b[0], b[1], b[2]); !errors.Is(err, ErrInvalidDate) {
			t.Errorf("NewDate%v should be invalid", b)
		}
	}
	if _, err := Commercial(2026, 0, 1); err == nil {
		t.Error("commercial week 0")
	}
	if _, err := Commercial(2026, 1, 8); err == nil {
		t.Error("commercial cwday 8")
	}
	if _, err := Commercial(2026, 54, 1); err == nil {
		t.Error("commercial week 54")
	}
	if _, err := NewDateTime(2026, 1, 1, 25, 0, 0, 0); err == nil {
		t.Error("hour 25")
	}
	if _, err := NewDateTime(2026, 1, 1, 0, 60, 0, 0); err == nil {
		t.Error("min 60")
	}
	if _, err := NewDateTime(2026, 1, 1, 0, 0, 61, 0); err == nil {
		t.Error("sec 61")
	}
	if _, err := NewDateTime(2026, 13, 1, 0, 0, 0, 0); err == nil {
		t.Error("bad date in dt")
	}
}

// TestReform exercises the Julian/Gregorian reform: pre-1582 dates use Julian,
// the gap days are invalid, and an explicit start switches calendars. All values
// are from MRI's default-ITALY Date.
func TestReform(t *testing.T) {
	if mustDate(t, 1, 1, 1).Jd() != 1721424 {
		t.Errorf("0001-01-01 jd=%d want 1721424 (Julian)", mustDate(t, 1, 1, 1).Jd())
	}
	if mustDate(t, 1, 1, 1).Wday() != 6 {
		t.Errorf("0001-01-01 wday=%d want 6 (Sat)", mustDate(t, 1, 1, 1).Wday())
	}
	if mustDate(t, 1582, 10, 4).Jd() != 2299160 {
		t.Error("last Julian day")
	}
	if mustDate(t, 1582, 10, 15).Jd() != 2299161 {
		t.Error("first Gregorian day")
	}
	// GREGORIAN-forced gives the proleptic value.
	g, _ := NewDateStart(1, 1, 1, Gregorian)
	if g.Jd() != 1721426 {
		t.Errorf("proleptic Gregorian 0001 jd=%d", g.Jd())
	}
	// JULIAN-forced everywhere.
	j, _ := NewDateStart(2000, 1, 1, Julian)
	if j.Jd() != 2451558 {
		t.Errorf("Julian 2000 jd=%d", j.Jd())
	}
	if mustDate(t, 1, 1, 1).Cweek() != 53 || mustDate(t, 1, 1, 1).Cwyear() != 0 {
		t.Errorf("0001 ISO = %d-W%d", mustDate(t, 1, 1, 1).Cwyear(), mustDate(t, 1, 1, 1).Cweek())
	}
	if !mustDate(t, 4, 2, 29).Leap() { // Julian leap
		t.Error("year 4 Julian leap")
	}
}

func TestStrftimeDirectives(t *testing.T) {
	dt := mustDT(t, 2026, 6, 29, 14, 3, 5, 19800) // +05:30
	cases := map[string]string{
		"%Y": "2026", "%y": "26", "%C": "20", "%m": "06", "%B": "June",
		"%b": "Jun", "%h": "Jun", "%d": "29", "%e": "29", "%j": "180",
		"%H": "14", "%k": "14", "%I": "02", "%l": " 2", "%M": "03",
		"%S": "05", "%L": "000", "%N": "000000000", "%p": "PM", "%P": "pm",
		"%A": "Monday", "%a": "Mon", "%u": "1", "%w": "1", "%U": "26",
		"%W": "26", "%V": "27", "%G": "2026", "%g": "26", "%s": "1782721985",
		"%Q": "1782721985000", "%z": "+0530", "%:z": "+05:30", "%::z": "+05:30:00",
		"%:::z": "+05:30", "%Z": "+05:30", "%D": "06/29/26", "%F": "2026-06-29",
		"%T": "14:03:05", "%R": "14:03", "%r": "02:03:05 PM", "%v": "29-JUN-2026",
		"%c": "Mon Jun 29 14:03:05 2026", "%x": "06/29/26", "%X": "14:03:05",
		"%%": "%", "%n": "\n", "%t": "\t",
	}
	for f, want := range cases {
		if got := dt.Strftime(f); got != want {
			t.Errorf("Strftime(%q)=%q want %q", f, got, want)
		}
	}
}

func TestStrftimeFlagsAndWidth(t *testing.T) {
	d := mustDate(t, 2026, 6, 5)
	cases := map[string]string{
		"%-d":     "5",
		"%_d":     " 5",
		"%03d":    "005",
		"%e":      " 5",
		"%-m":     "6",
		"%_m":     " 6",
		"%0m":     "06",
		"%^A":     "FRIDAY",
		"%#A":     "FRIDAY",
		"%#a":     "FRI",
		"%5Y":     "02026",
		"%EY":     "2026", // E modifier ignored
		"%Od":     "05",   // O modifier ignored
		"%q":      "%q",   // unknown directive passes through
		"literal": "literal",
		"%":       "%", // trailing percent
		"100%%":   "100%",
	}
	for f, want := range cases {
		if got := d.Strftime(f); got != want {
			t.Errorf("Strftime(%q)=%q want %q", f, got, want)
		}
	}
}

func TestStrftimeNegativeYear(t *testing.T) {
	dt := mustDT(t, -196, 1, 1, 0, 0, 0, 0)
	cases := map[string]string{
		"%Y": "-0196", "%C": "-2", "%5Y": "-0196", "%G": "-0196", "%-Y": "-196",
	}
	for f, want := range cases {
		if got := dt.Strftime(f); got != want {
			t.Errorf("neg year Strftime(%q)=%q want %q", f, got, want)
		}
	}
}

func TestStrftimeMeridiemAndFraction(t *testing.T) {
	mid := mustDT(t, 2026, 1, 1, 0, 0, 0, 0)
	if got := mid.Strftime("%I %l %p %P"); got != "12 12 AM am" {
		t.Errorf("midnight = %q", got)
	}
	noon := mustDT(t, 2026, 1, 1, 12, 0, 0, 0)
	if got := noon.Strftime("%I %p"); got != "12 PM" {
		t.Errorf("noon = %q", got)
	}
	// Fractional seconds: an injected nsec.
	frac := &Date{jd: 2461221, nsec: 3600e9*0 + 123456789, isDateTime: true, start: ITALY}
	if got := frac.Strftime("%N %3N %6N %9N %12N %L"); got != "123456789 123 123456 123456789 123456789000 123" {
		t.Errorf("frac = %q", got)
	}
}

func TestStrftimeOffsetVariants(t *testing.T) {
	cases := []struct {
		off  int
		spec string
		want string
	}{
		{0, "%z", "+0000"}, {0, "%:z", "+00:00"}, {0, "%::z", "+00:00:00"}, {0, "%:::z", "+00"},
		{-28800, "%z", "-0800"}, {-28800, "%:z", "-08:00"}, {-28800, "%:::z", "-08"},
		{60, "%::z", "+00:01:00"}, {60, "%:::z", "+00:01"},
		{19800, "%:::z", "+05:30"},
	}
	for _, c := range cases {
		dt := mustDT(t, 2026, 1, 1, 0, 0, 0, c.off)
		if got := dt.Strftime(c.spec); got != c.want {
			t.Errorf("off %d %s = %q want %q", c.off, c.spec, got, c.want)
		}
	}
}

func TestNamedFormats(t *testing.T) {
	d := mustDate(t, 2026, 6, 29)
	if d.Iso8601() != "2026-06-29" {
		t.Errorf("iso8601=%s", d.Iso8601())
	}
	if d.Rfc3339() != "2026-06-29T00:00:00+00:00" {
		t.Errorf("rfc3339=%s", d.Rfc3339())
	}
	if d.Rfc2822() != "Mon, 29 Jun 2026 00:00:00 +0000" {
		t.Errorf("rfc2822=%s", d.Rfc2822())
	}
	if d.Httpdate() != "Mon, 29 Jun 2026 00:00:00 GMT" {
		t.Errorf("httpdate=%s", d.Httpdate())
	}
	if d.Ctime() != "Mon Jun 29 00:00:00 2026" || d.Asctime() != d.Ctime() {
		t.Errorf("ctime=%s", d.Ctime())
	}
	if d.Jisx0301() != "R08.06.29" {
		t.Errorf("jisx=%s", d.Jisx0301())
	}
	// DateTime forms.
	dt := mustDT(t, 2026, 6, 29, 14, 3, 5, 18000) // +05:00
	if dt.String() != "2026-06-29T14:03:05+05:00" {
		t.Errorf("dt to_s=%s", dt.String())
	}
	if dt.Iso8601() != "2026-06-29T14:03:05+05:00" {
		t.Errorf("dt iso=%s", dt.Iso8601())
	}
	if dt.Rfc2822() != "Mon, 29 Jun 2026 14:03:05 +0500" {
		t.Errorf("dt rfc2822=%s", dt.Rfc2822())
	}
	// httpdate shifts to GMT, crossing the day boundary here.
	early := mustDT(t, 2026, 6, 29, 2, 0, 0, 18000)
	if early.Httpdate() != "Sun, 28 Jun 2026 21:00:00 GMT" {
		t.Errorf("dt http=%s", early.Httpdate())
	}
	if dt.Jisx0301() != "R08.06.29T14:03:05+05:00" {
		t.Errorf("dt jisx=%s", dt.Jisx0301())
	}
}

func TestJisx0301Eras(t *testing.T) {
	cases := map[string][3]int{
		"1872-12-31": {1872, 12, 31}, // pre-Gregorian-adoption → ISO
		"M06.01.01":  {1873, 1, 1},
		"M45.07.29":  {1912, 7, 29},
		"T01.07.30":  {1912, 7, 30},
		"S01.12.25":  {1926, 12, 25},
		"S64.01.07":  {1989, 1, 7},
		"H01.01.08":  {1989, 1, 8},
		"H31.04.30":  {2019, 4, 30},
		"R01.05.01":  {2019, 5, 1},
	}
	for want, ymd := range cases {
		if got := mustDate(t, ymd[0], ymd[1], ymd[2]).Jisx0301(); got != want {
			t.Errorf("jisx %v = %q want %q", ymd, got, want)
		}
	}
}

func TestArithmetic(t *testing.T) {
	d := mustDate(t, 2026, 6, 29)
	if d.Plus(5).String() != "2026-07-04" {
		t.Errorf("plus")
	}
	if d.Minus(5).String() != "2026-06-24" {
		t.Errorf("minus")
	}
	if d.Diff(mustDate(t, 2026, 6, 24)) != 5 {
		t.Errorf("diff")
	}
	if d.PlusMonths(8).String() != "2027-02-28" { // Jun 29 → Feb 28 clamp
		t.Errorf("plusmonths=%s", d.PlusMonths(8).String())
	}
	if d.PlusMonths(-13).String() != "2025-05-29" {
		t.Errorf("minusmonths")
	}
	if d.PlusYears(1).String() != "2027-06-29" {
		t.Errorf("plusyears")
	}
	// Feb 29 → Feb 28 on a non-leap next year.
	if mustDate(t, 2024, 2, 29).PlusYears(1).String() != "2025-02-28" {
		t.Errorf("leap clamp")
	}
	if d.NextDay(1).String() != "2026-06-30" || d.PrevDay(1).String() != "2026-06-28" {
		t.Errorf("next/prev day")
	}
	if d.NextMonth(1).String() != "2026-07-29" || d.PrevMonth(1).String() != "2026-05-29" {
		t.Errorf("next/prev month")
	}
	if d.NextYear(1).String() != "2027-06-29" || d.PrevYear(1).String() != "2025-06-29" {
		t.Errorf("next/prev year")
	}
	if d.Succ().String() != "2026-06-30" {
		t.Errorf("succ")
	}
}

func TestComparison(t *testing.T) {
	a := mustDate(t, 2026, 6, 29)
	b := mustDate(t, 2026, 6, 30)
	if a.Cmp(b) != -1 || b.Cmp(a) != 1 || a.Cmp(a) != 0 {
		t.Errorf("cmp")
	}
	if !a.Equal(mustDate(t, 2026, 6, 29)) || a.Equal(b) {
		t.Errorf("equal")
	}
	// A Date at midnight equals the equivalent DateTime instant (UTC).
	dtUTC := mustDT(t, 2026, 6, 29, 0, 0, 0, 0)
	if !a.Equal(dtUTC) {
		t.Errorf("date == midnight datetime")
	}
	// Offsets fold into the instant comparison.
	x := mustDT(t, 2026, 6, 29, 12, 0, 0, 0)
	y := mustDT(t, 2026, 6, 29, 14, 0, 0, 7200) // same instant
	if x.Cmp(y) != 0 || !x.Equal(y) {
		t.Errorf("offset instant equality")
	}
}

func TestStepAndUpto(t *testing.T) {
	d := mustDate(t, 2026, 1, 1)
	limit := mustDate(t, 2026, 1, 5)
	var got []string
	n := d.Step(limit, 1, func(c *Date) { got = append(got, c.String()) })
	if n != 5 || len(got) != 5 || got[0] != "2026-01-01" || got[4] != "2026-01-05" {
		t.Errorf("step = %v (%d)", got, n)
	}
	if d.Upto(limit, func(*Date) {}) != 5 {
		t.Errorf("upto count")
	}
	if limit.Downto(d, func(*Date) {}) != 5 {
		t.Errorf("downto count")
	}
	if d.Step(limit, 2, func(*Date) {}) != 3 {
		t.Errorf("step by 2")
	}
	if d.Step(limit, 0, func(*Date) {}) != 0 {
		t.Errorf("step 0 should take none")
	}
	// Negative step that never reaches a higher limit.
	if d.Step(limit, -1, func(*Date) {}) != 0 {
		t.Errorf("negative step past limit")
	}
}

func TestTodaySeam(t *testing.T) {
	restore := SetToday(2026, 6, 29)
	defer restore()
	if Today().String() != "2026-06-29" {
		t.Errorf("today=%s", Today().String())
	}
	if Now().Iso8601() != "2026-06-29T00:00:00+00:00" {
		t.Errorf("now=%s", Now().Iso8601())
	}
	if Today().IsDateTime() {
		t.Error("Today is a Date")
	}
	if !Now().IsDateTime() {
		t.Error("Now is a DateTime")
	}
}

func TestNowWithTime(t *testing.T) {
	restore := SetTodayInstant(2026, 6, 29, 14, 30, 45, 123456789)
	defer restore()
	if got := Now().Strftime("%H:%M:%S %N"); got != "14:30:45 123456789" {
		t.Errorf("now instant = %q", got)
	}
}

func TestDateTimeAccessors(t *testing.T) {
	dt := mustDT(t, 2026, 6, 29, 14, 3, 5, 19800)
	if dt.Hour() != 14 || dt.Min() != 3 || dt.Sec() != 5 {
		t.Errorf("hms = %d:%d:%d", dt.Hour(), dt.Min(), dt.Sec())
	}
	if dt.Offset() != 19800 {
		t.Errorf("offset=%d", dt.Offset())
	}
	if dt.NsecOfDay() != int64(14)*3600e9+int64(3)*60e9+int64(5)*1e9 {
		t.Errorf("nsecOfDay=%d", dt.NsecOfDay())
	}
	frac := &Date{jd: 2461221, nsec: 500000000, isDateTime: true, start: ITALY}
	if frac.SecFractionNanos() != 500000000 {
		t.Errorf("sec_fraction=%d", frac.SecFractionNanos())
	}
	if !dt.IsDateTime() {
		t.Error("IsDateTime")
	}
	if dt.ToDate().IsDateTime() || dt.ToDate().String() != "2026-06-29" {
		t.Errorf("ToDate")
	}
}
