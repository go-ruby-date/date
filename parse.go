// Copyright (c) the go-ruby-date/date authors
//
// SPDX-License-Identifier: BSD-3-Clause

package date

import "strings"

// This file implements Date.parse (the heuristic, multi-format reader) and
// Date.strptime (the explicit-format reader). Both build a fields struct then
// resolve it to a Date / DateTime, so the field-completion logic (defaulting a
// missing day to 1, promoting to a DateTime when a clock field is seen, applying
// a two-digit-year expansion) is shared.

// fields accumulates the components a parse recognises. The *Set booleans record
// which were actually seen, so resolution can default or reject as MRI does.
type fields struct {
	year, mon, mday      int
	hour, min, sec       int
	nsec                 int64
	offset               int
	cwyear, cweek, cwday int
	yday                 int
	epochSec             int64
	epochMilli           int64

	yearSet, monSet, mdaySet      bool
	hourSet, offSet               bool
	cwyearSet, cweekSet, cwdaySet bool
	ydaySet, epochSecSet          bool
	epochMilliSet                 bool
	yearDigits                    int // digits seen for the year, for comp expansion
}

// resolve turns accumulated fields into a Date / DateTime, applying MRI's
// completion rules. comp controls two-digit-year expansion (1969..2068 window);
// requireDate rejects a time-only result (strptime "14:03" with no date is an
// error in MRI, whereas a parsed value always needs a date anyway).
func (f *fields) resolve(comp, requireDate bool) (*Date, error) {
	// Epoch forms win outright.
	if f.epochSecSet {
		return fromEpochNanos(f.epochSec*int64(1e9) + f.nsec), nil
	}
	if f.epochMilliSet {
		return fromEpochNanos(f.epochMilli * int64(1e6)), nil
	}

	if requireDate && !f.yearSet && !f.monSet && !f.mdaySet &&
		!f.cwyearSet && !f.cweekSet && !f.cwdaySet && !f.ydaySet {
		return nil, ErrInvalidDate
	}

	var base *Date
	switch {
	case f.cwyearSet || f.cweekSet || f.cwdaySet:
		cwy, cw, cwd := f.cwyear, f.cweek, f.cwday
		if !f.cweekSet {
			cw = 1
		}
		if !f.cwdaySet {
			cwd = 1
		}
		d, err := Commercial(cwy, cw, cwd)
		if err != nil {
			return nil, err
		}
		base = d
	case f.ydaySet:
		y := f.year
		if !f.yearSet {
			return nil, ErrInvalidDate
		}
		d, err := Ordinal(y, f.yday)
		if err != nil {
			return nil, err
		}
		base = d
	default:
		y := f.year
		if comp && f.yearSet && f.yearDigits <= 2 {
			y = expandTwoDigitYear(y)
		}
		mon := f.mon
		if !f.monSet {
			mon = 1
		}
		mday := f.mday
		if !f.mdaySet {
			mday = 1
		}
		d, err := NewDate(y, mon, mday)
		if err != nil {
			return nil, err
		}
		base = d
	}

	if f.hourSet || f.offSet || f.nsec != 0 {
		ns := int64(f.hour)*3600e9 + int64(f.min)*60e9 + int64(f.sec)*1e9 + f.nsec
		return base.withTime(ns, f.offset), nil
	}
	return base, nil
}

// expandTwoDigitYear maps a two-digit year into 1969..2068, the window MRI's
// Date._parse comp mode uses.
func expandTwoDigitYear(y int) int {
	if y >= 69 {
		return 1900 + y
	}
	return 2000 + y
}

// fromEpochNanos builds a UTC DateTime from nanoseconds since the Unix epoch.
func fromEpochNanos(ns int64) *Date {
	days := floorDiv64(ns, nsPerDay)
	rem := ns - days*nsPerDay
	return &Date{jd: unixEpochJD + int(days), nsec: rem, offset: 0, start: ITALY, isDateTime: true}
}

// Strptime parses s against the explicit Ruby strptime format (Date.strptime /
// DateTime.strptime). An unmatched format or out-of-range value is an error.
func Strptime(s, format string) (*Date, error) {
	var f fields
	si, fi := 0, 0
	for fi < len(format) {
		if format[fi] != '%' {
			if format[fi] == ' ' {
				// A space in the format skips run of whitespace in the input.
				for si < len(s) && s[si] == ' ' {
					si++
				}
				fi++
				continue
			}
			if si >= len(s) || s[si] != format[fi] {
				return nil, ErrInvalidDate
			}
			si++
			fi++
			continue
		}
		fi++
		if fi >= len(format) {
			return nil, ErrInvalidDate
		}
		conv := format[fi]
		fi++
		n, err := f.scan(s, &si, conv)
		if err != nil {
			return nil, err
		}
		_ = n
	}
	return f.resolve(false, true)
}

// scan consumes one %conv directive from s at *si, updating fields.
func (f *fields) scan(s string, si *int, conv byte) (bool, error) {
	switch conv {
	case '%':
		if *si >= len(s) || s[*si] != '%' {
			return false, ErrInvalidDate
		}
		*si++
	case 'Y':
		v, d, ok := scanInt(s, si, 10)
		if !ok {
			return false, ErrInvalidDate
		}
		f.year, f.yearSet, f.yearDigits = v, true, d
	case 'y':
		v, _, ok := scanInt(s, si, 2)
		if !ok {
			return false, ErrInvalidDate
		}
		f.year, f.yearSet, f.yearDigits = expandTwoDigitYear(v), true, 2
	case 'C':
		v, _, ok := scanInt(s, si, 2)
		if !ok {
			return false, ErrInvalidDate
		}
		f.year = v*100 + f.year%100
		f.yearSet, f.yearDigits = true, 4
	case 'm':
		v, _, ok := scanInt(s, si, 2)
		if !ok || v < 1 || v > 12 {
			return false, ErrInvalidDate
		}
		f.mon, f.monSet = v, true
	case 'd', 'e':
		skipSpaces(s, si)
		v, _, ok := scanInt(s, si, 2)
		if !ok || v < 1 || v > 31 {
			return false, ErrInvalidDate
		}
		f.mday, f.mdaySet = v, true
	case 'H', 'k':
		skipSpaces(s, si)
		v, _, ok := scanInt(s, si, 2)
		if !ok || v > 24 {
			return false, ErrInvalidDate
		}
		f.hour, f.hourSet = v, true
	case 'I', 'l':
		skipSpaces(s, si)
		v, _, ok := scanInt(s, si, 2)
		if !ok || v < 1 || v > 12 {
			return false, ErrInvalidDate
		}
		f.hour, f.hourSet = v%12, true
	case 'M':
		v, _, ok := scanInt(s, si, 2)
		if !ok || v > 59 {
			return false, ErrInvalidDate
		}
		f.min = v
	case 'S':
		v, _, ok := scanInt(s, si, 2)
		if !ok || v > 60 {
			return false, ErrInvalidDate
		}
		f.sec = v
	case 'L':
		v, d, ok := scanInt(s, si, 3)
		if !ok {
			return false, ErrInvalidDate
		}
		f.nsec = scaleFraction(int64(v), d, 3)
	case 'N':
		v, d, ok := scanInt(s, si, 9)
		if !ok {
			return false, ErrInvalidDate
		}
		f.nsec = scaleFraction(int64(v), d, 9)
	case 'j':
		v, _, ok := scanInt(s, si, 3)
		if !ok || v < 1 || v > 366 {
			return false, ErrInvalidDate
		}
		f.yday, f.ydaySet = v, true
	case 'p', 'P':
		ok := f.scanMeridiem(s, si)
		if !ok {
			return false, ErrInvalidDate
		}
	case 'b', 'B', 'h':
		m, ok := scanMonthName(s, si)
		if !ok {
			return false, ErrInvalidDate
		}
		f.mon, f.monSet = m, true
	case 'a', 'A':
		if !scanDayName(s, si) {
			return false, ErrInvalidDate
		}
	case 'z', 'Z':
		off, ok := scanOffset(s, si)
		if !ok {
			return false, ErrInvalidDate
		}
		f.offset, f.offSet = off, true
	case 'G':
		v, d, ok := scanInt(s, si, 10)
		if !ok {
			return false, ErrInvalidDate
		}
		f.cwyear, f.cwyearSet, f.yearDigits = v, true, d
	case 'V':
		v, _, ok := scanInt(s, si, 2)
		if !ok {
			return false, ErrInvalidDate
		}
		f.cweek, f.cweekSet = v, true
	case 'u':
		v, _, ok := scanInt(s, si, 1)
		if !ok {
			return false, ErrInvalidDate
		}
		f.cwday, f.cwdaySet = v, true
	case 's':
		v, _, ok := scanInt(s, si, 19)
		if !ok {
			return false, ErrInvalidDate
		}
		f.epochSec, f.epochSecSet = int64(v), true
	case 'Q':
		v, _, ok := scanInt(s, si, 19)
		if !ok {
			return false, ErrInvalidDate
		}
		f.epochMilli, f.epochMilliSet = int64(v), true
	default:
		return false, ErrInvalidDate
	}
	return true, nil
}

// scanMeridiem reads an AM/PM marker and folds it into the hour, supporting both
// 12-hour (from %I) and bare cases.
func (f *fields) scanMeridiem(s string, si *int) bool {
	if *si+2 > len(s) {
		return false
	}
	m := strings.ToUpper(s[*si : *si+2])
	switch m {
	case "AM":
		f.hourSet = true
	case "PM":
		f.hour += 12
		f.hourSet = true
	default:
		return false
	}
	*si += 2
	return true
}

// scaleFraction converts a fractional value with `digits` digits (at most 9, as
// every caller caps the field at nine digits) to a nanosecond count, scaling it
// up by 10^(9-digits).
func scaleFraction(v int64, digits, _ int) int64 {
	for d := digits; d < 9; d++ {
		v *= 10
	}
	return v
}

// scanInt reads up to maxDigits decimal digits at *si, returning the value, the
// digit count, and whether at least one digit was read.
func scanInt(s string, si *int, maxDigits int) (int, int, bool) {
	start := *si
	n := 0
	count := 0
	for *si < len(s) && count < maxDigits && s[*si] >= '0' && s[*si] <= '9' {
		n = n*10 + int(s[*si]-'0')
		*si++
		count++
	}
	if *si == start {
		return 0, 0, false
	}
	return n, count, true
}

// skipSpaces advances *si past leading ASCII spaces (for space-padded fields).
func skipSpaces(s string, si *int) {
	for *si < len(s) && s[*si] == ' ' {
		*si++
	}
}

// scanMonthName matches a full or abbreviated English month name at *si.
func scanMonthName(s string, si *int) (int, bool) {
	rest := strings.ToLower(s[*si:])
	for i := 1; i <= 12; i++ {
		full := strings.ToLower(monthNames[i])
		if strings.HasPrefix(rest, full) {
			*si += len(full)
			return i, true
		}
	}
	for i := 1; i <= 12; i++ {
		ab := strings.ToLower(monthAbbr[i])
		if strings.HasPrefix(rest, ab) {
			*si += len(ab)
			return i, true
		}
	}
	return 0, false
}

// scanDayName matches a full or abbreviated English day name at *si (its value
// is not retained — it only advances the cursor).
func scanDayName(s string, si *int) bool {
	rest := strings.ToLower(s[*si:])
	for i := 0; i < 7; i++ {
		if strings.HasPrefix(rest, strings.ToLower(dayNames[i])) {
			*si += len(dayNames[i])
			return true
		}
	}
	for i := 0; i < 7; i++ {
		if strings.HasPrefix(rest, strings.ToLower(dayAbbr[i])) {
			*si += len(dayAbbr[i])
			return true
		}
	}
	return false
}

// scanOffset matches a zone offset ("Z", "+HHMM", "+HH:MM", "+HH") at *si,
// returning the offset in seconds.
func scanOffset(s string, si *int) (int, bool) {
	if *si < len(s) && (s[*si] == 'Z' || s[*si] == 'z') {
		*si++
		return 0, true
	}
	if *si >= len(s) || (s[*si] != '+' && s[*si] != '-') {
		return 0, false
	}
	sign := 1
	if s[*si] == '-' {
		sign = -1
	}
	*si++
	hh, _, ok := scanInt(s, si, 2)
	if !ok {
		return 0, false
	}
	mm := 0
	if *si < len(s) && s[*si] == ':' {
		*si++
	}
	if *si < len(s) && s[*si] >= '0' && s[*si] <= '9' {
		mm, _, _ = scanInt(s, si, 2)
	}
	return sign * (hh*3600 + mm*60), true
}
