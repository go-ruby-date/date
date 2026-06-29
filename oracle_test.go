// Copyright (c) the go-ruby-date/date authors
//
// SPDX-License-Identifier: BSD-3-Clause

package date

import (
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// The oracle tests run the local `ruby` binary as a differential reference for
// MRI's date library. They skip themselves when ruby is absent (the qemu
// cross-arch lanes and the Windows lane), so the deterministic suite alone drives
// the 100% gate there. Every oracle script $stdout.binmode's itself so Windows
// text-mode never pollutes the bytes (the go-ruby-erb lesson), and the oracle is
// gated on MRI >= 4.0 so an older system ruby does not produce spurious diffs.

func rubyBin(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby not on PATH; skipping MRI oracle")
	}
	ver := strings.TrimSpace(rubyEval(t, path, "print RUBY_VERSION"))
	if !rubyAtLeast(ver, 4, 0) {
		t.Skipf("ruby %s < 4.0; skipping MRI oracle", ver)
	}
	return path
}

// rubyAtLeast reports whether a "MAJOR.MINOR.PATCH" version string is >= the
// given major/minor floor.
func rubyAtLeast(ver string, major, minor int) bool {
	parts := strings.Split(ver, ".")
	if len(parts) < 2 {
		return false
	}
	maj, err1 := strconv.Atoi(parts[0])
	min, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return false
	}
	return maj > major || (maj == major && min >= minor)
}

// rubyEval runs a Ruby script with the date library required and returns stdout.
// The script binmodes stdin and stdout so cross-platform byte output is exact.
func rubyEval(t *testing.T, bin, script string) string {
	t.Helper()
	full := "$stdout.binmode\n$stdin.binmode\n" + script
	cmd := exec.Command(bin, "-rdate", "-e", full)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ruby error: %v\nscript:\n%s\noutput:\n%s", err, script, out)
	}
	return string(out)
}

// TestOracleStrftime checks every supported directive against MRI on a corpus of
// dates and datetimes spanning the Julian/Gregorian reform, leap years, the
// year boundary and assorted offsets.
func TestOracleStrftime(t *testing.T) {
	bin := rubyBin(t)
	dirs := []string{
		"%Y", "%y", "%C", "%m", "%B", "%b", "%h", "%d", "%e", "%j",
		"%H", "%k", "%I", "%l", "%M", "%S", "%L", "%N", "%p", "%P",
		"%A", "%a", "%u", "%w", "%U", "%W", "%V", "%G", "%g", "%s",
		"%Q", "%z", "%:z", "%::z", "%:::z", "%Z", "%D", "%F", "%T", "%R",
		"%r", "%v", "%c", "%x", "%X", "%-d", "%_d", "%03d", "%^A", "%#A",
		"%5Y", "%-m",
	}
	type tc struct {
		y, mo, d, h, mi, s, off int
	}
	corpus := []tc{
		{2026, 6, 29, 14, 3, 5, 19800},
		{2000, 1, 1, 0, 0, 0, 0},
		{2024, 2, 29, 23, 59, 59, -28800},
		{1, 1, 1, 6, 30, 0, 3600},
		{1582, 10, 15, 12, 0, 0, 0},
		{1872, 12, 31, 0, 0, 0, 0},
		{1873, 1, 1, 0, 0, 0, 0},
		{1989, 1, 8, 1, 2, 3, 60},
		{-44, 3, 15, 9, 0, 0, 0},
		{2027, 1, 1, 0, 0, 0, 0},
	}
	for _, c := range corpus {
		dt, err := NewDateTime(c.y, c.mo, c.d, c.h, c.mi, c.s, c.off)
		if err != nil {
			t.Fatalf("ctor %v: %v", c, err)
		}
		for _, dir := range dirs {
			want := rubyEval(t, bin, oracleStrftimeScript(c, dir))
			if got := dt.Strftime(dir); got != want {
				t.Errorf("Strftime(%q) for %v = %q want %q", dir, c, got, want)
			}
		}
	}
}

func oracleStrftimeScript(c struct{ y, mo, d, h, mi, s, off int }, dir string) string {
	off := offsetString(c.off)
	return "dt = DateTime.new(" +
		itoac(c.y) + "," + itoac(c.mo) + "," + itoac(c.d) + "," +
		itoac(c.h) + "," + itoac(c.mi) + "," + itoac(c.s) + ",\"" + off + "\")\n" +
		"print dt.strftime(" + strconv.Quote(dir) + ")"
}

// offsetString renders an offset in seconds as the "+HH:MM" zone string MRI's
// DateTime.new expects.
func offsetString(sec int) string {
	sign := "+"
	if sec < 0 {
		sign = "-"
		sec = -sec
	}
	h := sec / 3600
	m := (sec % 3600) / 60
	return sign + twoDigit(h) + ":" + twoDigit(m)
}

func twoDigit(n int) string {
	s := strconv.Itoa(n)
	if len(s) < 2 {
		return "0" + s
	}
	return s
}

func itoac(n int) string { return strconv.Itoa(n) }

// TestOracleNamedFormats checks iso8601 / rfc3339 / rfc2822 / httpdate / ctime /
// jisx0301 against MRI for both Date and DateTime.
func TestOracleNamedFormats(t *testing.T) {
	bin := rubyBin(t)
	dates := [][3]int{{2026, 6, 29}, {1, 1, 1}, {2000, 2, 29}, {1873, 1, 1}, {1872, 12, 31}, {2019, 5, 1}}
	for _, ymd := range dates {
		d, err := NewDate(ymd[0], ymd[1], ymd[2])
		if err != nil {
			t.Fatalf("NewDate %v: %v", ymd, err)
		}
		script := "d = Date.new(" + itoac(ymd[0]) + "," + itoac(ymd[1]) + "," + itoac(ymd[2]) + ")\n" +
			"print [d.iso8601, d.rfc3339, d.rfc2822, d.httpdate, d.ctime, d.jisx0301].join(\"\\x1f\")"
		parts := strings.Split(rubyEval(t, bin, script), "\x1f")
		got := []string{d.Iso8601(), d.Rfc3339(), d.Rfc2822(), d.Httpdate(), d.Ctime(), d.Jisx0301()}
		for i, w := range parts {
			if got[i] != w {
				t.Errorf("named[%d] for %v = %q want %q", i, ymd, got[i], w)
			}
		}
	}
}

// TestOracleArithmetic checks day and month shifts and the week-date / ordinal
// accessors against MRI.
func TestOracleArithmetic(t *testing.T) {
	bin := rubyBin(t)
	dates := [][3]int{{2026, 6, 29}, {2024, 2, 29}, {2020, 1, 1}, {1, 1, 1}, {2000, 12, 31}}
	for _, ymd := range dates {
		d, _ := NewDate(ymd[0], ymd[1], ymd[2])
		script := "d = Date.new(" + itoac(ymd[0]) + "," + itoac(ymd[1]) + "," + itoac(ymd[2]) + ")\n" +
			"vals = [(d+10).iso8601, (d-10).iso8601, (d >> 5).iso8601, (d << 5).iso8601, " +
			"d.next_year.iso8601, d.prev_month.iso8601, " +
			"d.cwyear.to_s, d.cweek.to_s, d.cwday.to_s, d.yday.to_s, d.wday.to_s, d.jd.to_s, d.leap?.to_s]\n" +
			"print vals.join(\"\\x1f\")"
		parts := strings.Split(rubyEval(t, bin, script), "\x1f")
		got := []string{
			d.Plus(10).Iso8601(), d.Minus(10).Iso8601(), d.PlusMonths(5).Iso8601(),
			d.PlusMonths(-5).Iso8601(), d.NextYear(1).Iso8601(), d.PrevMonth(1).Iso8601(),
			strconv.Itoa(d.Cwyear()), strconv.Itoa(d.Cweek()), strconv.Itoa(d.Cwday()),
			strconv.Itoa(d.Yday()), strconv.Itoa(d.Wday()), strconv.Itoa(d.Jd()),
			strconv.FormatBool(d.Leap()),
		}
		for i, w := range parts {
			if got[i] != w {
				t.Errorf("arith[%d] for %v = %q want %q", i, ymd, got[i], w)
			}
		}
	}
}

// TestOracleParse checks Date.parse / DateTime.parse / Date.strptime against MRI
// over a representative corpus.
func TestOracleParse(t *testing.T) {
	bin := rubyBin(t)
	parseCases := []string{
		"2026-06-29", "2026/06/29", "June 29, 2026", "29 June 2026",
		"Jun 29 2026", "20260629", "29-06-2026", "2026-180", "2026-W27-1",
		"June 2026", "July 4, 1776",
	}
	for _, in := range parseCases {
		want := strings.TrimSpace(rubyEval(t, bin,
			"print Date.parse("+strconv.Quote(in)+").iso8601"))
		d, err := Parse(in, false)
		if err != nil {
			t.Errorf("Parse(%q): %v (MRI %q)", in, err, want)
			continue
		}
		if d.Iso8601() != want {
			t.Errorf("Parse(%q) = %q want %q", in, d.Iso8601(), want)
		}
	}

	dtCases := []string{
		"2026-06-29T14:03:05+05:00", "2026-06-29 14:03:05",
		"Mon, 29 Jun 2026 14:03:05 +0500",
	}
	for _, in := range dtCases {
		want := strings.TrimSpace(rubyEval(t, bin,
			"print DateTime.parse("+strconv.Quote(in)+").strftime(\"%Y-%m-%dT%H:%M:%S%:z\")"))
		d, err := ParseDateTime(in, false)
		if err != nil {
			t.Errorf("ParseDateTime(%q): %v", in, err)
			continue
		}
		if got := d.Strftime("%Y-%m-%dT%H:%M:%S%:z"); got != want {
			t.Errorf("ParseDateTime(%q) = %q want %q", in, got, want)
		}
	}

	strpCases := []struct{ in, fmt string }{
		{"2026/06/29", "%Y/%m/%d"}, {"06/29/2026", "%m/%d/%Y"},
		{"29 6 2026", "%d %m %Y"}, {"Jun 29, 2026", "%b %d, %Y"},
		{"2026-180", "%Y-%j"}, {"2026 W27 1", "%G W%V %u"},
	}
	for _, c := range strpCases {
		want := strings.TrimSpace(rubyEval(t, bin,
			"print Date.strptime("+strconv.Quote(c.in)+","+strconv.Quote(c.fmt)+").iso8601"))
		d, err := Strptime(c.in, c.fmt)
		if err != nil {
			t.Errorf("Strptime(%q,%q): %v", c.in, c.fmt, err)
			continue
		}
		if d.Iso8601() != want {
			t.Errorf("Strptime(%q,%q) = %q want %q", c.in, c.fmt, d.Iso8601(), want)
		}
	}
}
