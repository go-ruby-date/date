// Copyright (c) the go-ruby-date/date authors
//
// SPDX-License-Identifier: BSD-3-Clause

package date

import (
	"regexp"
	"strconv"
	"strings"
)

// This file implements Date.parse — the heuristic, multi-format reader. MRI's
// Date._parse tries a battery of patterns; this is a faithful subset covering the
// forms the differential corpus exercises (ISO, slashed, dotted, month-name,
// RFC-2822, week-date and ordinal), folding any time-of-day and zone it finds
// into a DateTime.

var (
	reISO       = regexp.MustCompile(`^(\d{4})-(\d{1,2})-(\d{1,2})$`)
	reSlash     = regexp.MustCompile(`^(\d{1,4})[/.](\d{1,2})[/.](\d{1,4})$`)
	reDash      = regexp.MustCompile(`^(\d{1,4})-(\d{1,2})-(\d{1,4})$`)
	reBasic     = regexp.MustCompile(`^(\d{4})(\d{2})(\d{2})$`)
	reWeek      = regexp.MustCompile(`^(\d{4})-?W(\d{2})-?(\d)?$`)
	reOrdinal   = regexp.MustCompile(`^(\d{4})-(\d{3})$`)
	reYearMon   = regexp.MustCompile(`^(\d{4})-(\d{1,2})$`)
	reTime      = regexp.MustCompile(`(\d{1,2}):(\d{2})(?::(\d{2})(?:\.(\d+))?)?\s*([+-]\d{2}:?\d{2}|Z)?`)
	reMonthName = regexp.MustCompile(`(?i)^([A-Za-z]+)\.?\s+(\d{1,2})(?:st|nd|rd|th)?,?\s+(\d{4})$`)
	reDayMonth  = regexp.MustCompile(`(?i)^(\d{1,2})(?:st|nd|rd|th)?\.?\s+([A-Za-z]+)\.?\s+(\d{4})$`)
	reMonYear   = regexp.MustCompile(`(?i)^([A-Za-z]+)\.?\s+(\d{4})$`)
	reMonDayY   = regexp.MustCompile(`(?i)^([A-Za-z]+)\.?\s+(\d{1,2})\s+(\d{4})$`)
)

// Parse reads s heuristically (Date.parse) and returns a plain Date — any
// time-of-day in s is recognised (so the string still parses) but dropped, as
// MRI's Date.parse yields a Date. comp controls two-digit-year expansion for the
// slashed/dashed numeric forms. Use ParseDateTime to retain the clock.
func Parse(s string, comp bool) (*Date, error) {
	d, err := ParseDateTime(s, comp)
	if err != nil {
		return nil, err
	}
	return d.dateOnly(), nil
}

// ParseDateTime reads s heuristically (DateTime.parse), retaining any
// time-of-day and zone it finds.
func ParseDateTime(s string, comp bool) (*Date, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, ErrInvalidDate
	}

	var f fields
	body := s

	// Peel a trailing / embedded time-of-day off first; the remainder is the
	// date part. The "T" separator and a leading weekday word are tolerated.
	if loc := reTime.FindStringSubmatchIndex(s); loc != nil && timeLooksReal(s, loc) {
		applyTime(&f, s[loc[0]:loc[1]])
		body = strings.TrimSpace(s[:loc[0]] + s[loc[1]:])
		body = strings.TrimSuffix(body, "T")
		body = strings.TrimSpace(body)
	}

	// Strip a leading weekday word and its comma (RFC-2822 / "Mon, ").
	body = stripLeadingWeekday(body)

	if !parseDateBody(&f, body, comp) {
		return nil, ErrInvalidDate
	}
	return f.resolve(comp, true)
}

// timeLooksReal guards reTime against matching a bare date like "2026-06-29"
// (which has no colon): the match must contain a ':' to be a real clock.
func timeLooksReal(s string, loc []int) bool {
	return strings.Contains(s[loc[0]:loc[1]], ":")
}

// applyTime fills the clock fields from a "HH:MM[:SS[.fff]][ ±zone]" match.
func applyTime(f *fields, t string) {
	// t is a slice that already matched reTime in the caller, so the submatch
	// always succeeds.
	m := reTime.FindStringSubmatch(t)
	f.hour, _ = strconv.Atoi(m[1])
	f.min, _ = strconv.Atoi(m[2])
	if m[3] != "" {
		f.sec, _ = strconv.Atoi(m[3])
	}
	if m[4] != "" {
		frac := m[4]
		if len(frac) > 9 {
			frac = frac[:9]
		}
		v, _ := strconv.Atoi(frac)
		f.nsec = scaleFraction(int64(v), len(frac), 9)
	}
	f.hourSet = true
	if m[5] != "" {
		si := 0
		if off, ok := scanOffset(m[5], &si); ok {
			f.offset, f.offSet = off, true
		}
	}
}

// stripLeadingWeekday removes a leading English weekday name and any trailing
// comma/space, so "Mon, 29 Jun 2026" reduces to "29 Jun 2026".
func stripLeadingWeekday(s string) string {
	low := strings.ToLower(s)
	for i := 0; i < 7; i++ {
		for _, name := range []string{strings.ToLower(dayNames[i]), strings.ToLower(dayAbbr[i])} {
			if strings.HasPrefix(low, name) {
				rest := s[len(name):]
				rest = strings.TrimLeft(rest, ",. ")
				return rest
			}
		}
	}
	return s
}

// parseDateBody matches the date portion against the known forms in priority
// order, filling f. It reports whether any form matched.
func parseDateBody(f *fields, body string, comp bool) bool {
	if body == "" {
		// A pure time string ("14:03:05") has no date and is invalid in MRI.
		return false
	}
	switch {
	case match3(reISO, body, func(a, b, c int) { setYMD(f, a, b, c) }):
	case match3(reBasic, body, func(a, b, c int) { setYMD(f, a, b, c) }):
	case match2(reOrdinal, body, func(a, b int) { f.year, f.yearSet, f.yearDigits = a, true, 4; f.yday, f.ydaySet = b, true }):
	case matchWeek(f, body):
	case match2(reYearMon, body, func(a, b int) { f.year, f.yearSet, f.yearDigits = a, true, 4; f.mon, f.monSet = b, true }):
	case reMonthName.MatchString(body) && setMonthNameForm(f, reMonthName, body, 1, 2, 3):
	case reMonDayY.MatchString(body) && setMonthNameForm(f, reMonDayY, body, 1, 2, 3):
	case reDayMonth.MatchString(body) && setDayMonthForm(f, body):
	case reMonYear.MatchString(body) && setMonYearForm(f, body):
	case matchNumericTriple(f, reSlash, body, comp):
	case matchNumericTriple(f, reDash, body, comp):
	default:
		return false
	}
	return true
}

// setYMD records an unambiguous year-month-day triple.
func setYMD(f *fields, y, m, d int) {
	f.year, f.yearSet, f.yearDigits = y, true, 4
	f.mon, f.monSet = m, true
	f.mday, f.mdaySet = d, true
}

// matchWeek handles the ISO week-date form "YYYY-Www[-d]".
func matchWeek(f *fields, body string) bool {
	m := reWeek.FindStringSubmatch(body)
	if m == nil {
		return false
	}
	f.cwyear, f.cwyearSet = atoi(m[1]), true
	f.cweek, f.cweekSet = atoi(m[2]), true
	if m[3] != "" {
		f.cwday, f.cwdaySet = atoi(m[3]), true
	}
	return true
}

// setMonthNameForm handles "Month D, YYYY" / "Month D YYYY" (month name first).
func setMonthNameForm(f *fields, re *regexp.Regexp, body string, mi, di, yi int) bool {
	m := re.FindStringSubmatch(body)
	mon, ok := monthFromName(m[mi])
	if !ok {
		return false
	}
	f.mon, f.monSet = mon, true
	f.mday, f.mdaySet = atoi(m[di]), true
	f.year, f.yearSet, f.yearDigits = atoi(m[yi]), true, 4
	return true
}

// setDayMonthForm handles "D Month YYYY" (day first).
func setDayMonthForm(f *fields, body string) bool {
	m := reDayMonth.FindStringSubmatch(body)
	mon, ok := monthFromName(m[2])
	if !ok {
		return false
	}
	f.mday, f.mdaySet = atoi(m[1]), true
	f.mon, f.monSet = mon, true
	f.year, f.yearSet, f.yearDigits = atoi(m[3]), true, 4
	return true
}

// setMonYearForm handles "Month YYYY" (day defaults to 1).
func setMonYearForm(f *fields, body string) bool {
	m := reMonYear.FindStringSubmatch(body)
	mon, ok := monthFromName(m[1])
	if !ok {
		return false
	}
	f.mon, f.monSet = mon, true
	f.year, f.yearSet, f.yearDigits = atoi(m[2]), true, 4
	return true
}

// matchNumericTriple handles slashed/dashed numeric triples, applying MRI's
// heuristic: a 4-digit leading group is the year (Y/M/D); otherwise the trailing
// group is the year and the order is day/month/year.
func matchNumericTriple(f *fields, re *regexp.Regexp, body string, comp bool) bool {
	m := re.FindStringSubmatch(body)
	if m == nil {
		return false
	}
	a, b, c := atoi(m[1]), atoi(m[2]), atoi(m[3])
	ad, cd := len(m[1]), len(m[3])
	switch {
	case ad == 4:
		// YYYY/M/D — an unambiguous leading 4-digit year.
		f.year, f.yearSet, f.yearDigits = a, true, ad
		f.mon, f.monSet = b, true
		f.mday, f.mdaySet = c, true
	case cd == 4:
		// D/M/YYYY — a trailing 4-digit year means day comes first.
		f.mday, f.mdaySet = a, true
		f.mon, f.monSet = b, true
		f.year, f.yearSet, f.yearDigits = c, true, cd
	default:
		// All short: MRI reads Y/M/D, so "29-6-26" → 2029-06-26.
		f.year, f.yearSet, f.yearDigits = a, true, ad
		f.mon, f.monSet = b, true
		f.mday, f.mdaySet = c, true
	}
	return validMon(f.mon)
}

// helper predicates and small converters.

func validMon(m int) bool { return m >= 1 && m <= 12 }

func atoi(s string) int { v, _ := strconv.Atoi(s); return v }

// monthFromName resolves a full or abbreviated English month name.
func monthFromName(name string) (int, bool) {
	low := strings.ToLower(strings.TrimSuffix(name, "."))
	for i := 1; i <= 12; i++ {
		if low == strings.ToLower(monthNames[i]) || low == strings.ToLower(monthAbbr[i]) {
			return i, true
		}
	}
	return 0, false
}

// match2 / match3 run a regexp and, on a match, invoke fn with the captured
// integers, reporting whether it matched.
func match2(re *regexp.Regexp, s string, fn func(a, b int)) bool {
	m := re.FindStringSubmatch(s)
	if m == nil {
		return false
	}
	fn(atoi(m[1]), atoi(m[2]))
	return true
}

func match3(re *regexp.Regexp, s string, fn func(a, b, c int)) bool {
	m := re.FindStringSubmatch(s)
	if m == nil {
		return false
	}
	fn(atoi(m[1]), atoi(m[2]), atoi(m[3]))
	return true
}
