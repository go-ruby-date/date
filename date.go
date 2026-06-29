// Copyright (c) the go-ruby-date/date authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package date is a pure-Go (no cgo) reimplementation of Ruby's date standard
// library — MRI 4.0.5's Date and DateTime — the deterministic,
// interpreter-independent core of calendar arithmetic, parsing and formatting.
//
// The calendar core is built from scratch on the astronomical Julian Day Number
// (JDN): every Date is, fundamentally, an integer day count, so arbitrary BC/AD
// years, the proleptic Gregorian calendar and day/month arithmetic all reduce to
// integer math. No use is made of Go's time package for the calendar (only the
// Today seam defaults to it); the JDN conversions follow the same formulae MRI's
// date_core.c uses, so results match MRI byte-for-byte.
//
// A DateTime extends a Date with a time-of-day (hour/minute/second, a fractional
// nanosecond part) and a UTC offset in seconds, mirroring MRI's split between
// Date (a calendar value pinned to midnight) and DateTime (an instant).
package date

import (
	"errors"
	"time"
)

// Date is an MRI-compatible Ruby Date / DateTime value. It is an astronomical
// Julian Day Number (jd) plus, for a DateTime, a time-of-day in nanoseconds
// since local midnight (nsec, 0 for a plain Date) and a UTC offset in seconds
// (offset). The isDateTime flag distinguishes a Date (which formats and inspects
// as a bare calendar date) from a DateTime (which carries the wall clock).
type Date struct {
	jd         int   // astronomical Julian Day Number at local midnight
	nsec       int64 // nanoseconds since local midnight, 0 .. 86_400e9-1
	offset     int   // UTC offset in seconds, e.g. +7200 for +02:00
	start      int   // calendar-reform JDN: dates >= start are Gregorian, before are Julian
	isDateTime bool  // true for a DateTime, false for a plain Date
}

// Calendar-reform sentinels for the start parameter (Date::ITALY / ENGLAND /
// GREGORIAN / JULIAN in MRI). ITALY is the default — the date the Gregorian
// calendar was first adopted (1582-10-15 = JD 2_299_161).
const (
	ITALY     = 2299161  // 1582-10-15, MRI's default reform point
	ENGLAND   = 2361222  // 1752-09-14
	Gregorian = -1 << 62 // proleptic Gregorian everywhere (Date::GREGORIAN, -∞)
	Julian    = 1 << 62  // proleptic Julian everywhere (Date::JULIAN, +∞)
)

// ErrInvalidDate is returned by the constructors when the requested calendar /
// week-date / ordinal coordinates do not denote a real date.
var ErrInvalidDate = errors.New("invalid date")

const nsPerDay = 86_400 * int64(time.Second)

// gregorianToJD converts a proleptic-Gregorian (year, month, day) to its
// astronomical Julian Day Number. It does not validate.
func gregorianToJD(y, m, d int) int {
	a := floorDiv(14-m, 12)
	yy := y + 4800 - a
	mm := m + 12*a - 3
	return d + floorDiv(153*mm+2, 5) + 365*yy + floorDiv(yy, 4) - floorDiv(yy, 100) + floorDiv(yy, 400) - 32045
}

// julianToJD converts a proleptic-Julian (year, month, day) to its astronomical
// Julian Day Number — the calendar in force before the reform.
func julianToJD(y, m, d int) int {
	a := floorDiv(14-m, 12)
	yy := y + 4800 - a
	mm := m + 12*a - 3
	return d + floorDiv(153*mm+2, 5) + 365*yy + floorDiv(yy, 4) - 32083
}

// jdToGregorian inverts gregorianToJD.
func jdToGregorian(jd int) (y, m, d int) {
	a := jd + 32044
	b := floorDiv(4*a+3, 146097)
	c := a - floorDiv(146097*b, 4)
	dd := floorDiv(4*c+3, 1461)
	e := c - floorDiv(1461*dd, 4)
	mm := floorDiv(5*e+2, 153)
	d = e - floorDiv(153*mm+2, 5) + 1
	m = mm + 3 - 12*(mm/10)
	y = 100*b + dd - 4800 + mm/10
	return y, m, d
}

// jdToJulian inverts julianToJD.
func jdToJulian(jd int) (y, m, d int) {
	c := jd + 32082
	dd := floorDiv(4*c+3, 1461)
	e := c - floorDiv(1461*dd, 4)
	mm := floorDiv(5*e+2, 153)
	d = e - floorDiv(153*mm+2, 5) + 1
	m = mm + 3 - 12*(mm/10)
	y = dd - 4800 + mm/10
	return y, m, d
}

// civilToJDStart converts (y, m, d) to a JDN under the calendar selected by the
// reform point start: Gregorian if the resulting JDN is on or after start, else
// Julian. It returns the JDN and whether the (y,m,d) actually exists under that
// calendar (the reform gap days 1582-10-05..14 do not, for the ITALY start).
func civilToJDStart(y, m, d, start int) (int, bool) {
	g := gregorianToJD(y, m, d)
	if g >= start {
		// Confirm round-trip (guards the proleptic edges; always holds here).
		return g, true
	}
	j := julianToJD(y, m, d)
	if j >= start {
		// (y,m,d) lands in the reform gap: a Gregorian date too early to be
		// Gregorian, a Julian date too late to be Julian — non-existent.
		return 0, false
	}
	return j, true
}

// jdToCivilStart converts a JDN back to (y, m, d) under the reform: Gregorian on
// or after start, Julian before.
func jdToCivilStart(jd, start int) (y, m, d int) {
	if jd >= start {
		return jdToGregorian(jd)
	}
	return jdToJulian(jd)
}

// civilToJD is the ITALY-default shorthand used by the ordinal/commercial
// constructors and the era table, where the reform is the package default.
func civilToJD(y, m, d int) int { jd, _ := civilToJDStart(y, m, d, ITALY); return jd }

// floorDiv / floorMod give floored (Python-style) integer division and modulus,
// which the week-date and time-of-day math need to stay correct for negative
// operands (Go's / and % truncate toward zero).
func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

func floorMod(a, b int) int { return a - floorDiv(a, b)*b }

// NewDate constructs Date.new(y, m, d) / Date.civil(y, m, d) under the default
// ITALY reform. A negative month or day counts from the end (month -1 =
// December, day -1 = last of the month), as in MRI. An out-of-range coordinate,
// or one falling in the 1582 reform gap, yields ErrInvalidDate.
func NewDate(y, m, d int) (*Date, error) { return NewDateStart(y, m, d, ITALY) }

// NewDateStart is NewDate with an explicit calendar-reform point (Date.new's
// optional `start` — ITALY / ENGLAND / GREGORIAN / JULIAN, or any JDN).
func NewDateStart(y, m, d, start int) (*Date, error) {
	if m < 0 {
		m += 13
	}
	if m < 1 || m > 12 {
		return nil, ErrInvalidDate
	}
	last := lastDayOfMonthStart(y, m, start)
	if d < 0 {
		d += last + 1
	}
	if d < 1 || d > last {
		return nil, ErrInvalidDate
	}
	jd, ok := civilToJDStart(y, m, d, start)
	if !ok {
		return nil, ErrInvalidDate
	}
	return &Date{jd: jd, start: start}, nil
}

// DateJD constructs Date.jd(jd) — the date with the given astronomical Julian
// Day Number, under the default ITALY reform.
func DateJD(jd int) *Date { return &Date{jd: jd, start: ITALY} }

// Ordinal constructs Date.ordinal(y, yday) — the yday-th day of year y (negative
// yday counts from the end). An out-of-range yday yields ErrInvalidDate.
func Ordinal(y, yday int) (*Date, error) {
	jan1 := civilToJD(y, 1, 1)
	n := 365
	if leapYearStart(y, ITALY) {
		n = 366
	}
	if yday < 0 {
		yday += n + 1
	}
	if yday < 1 || yday > n {
		return nil, ErrInvalidDate
	}
	return &Date{jd: jan1 + yday - 1, start: ITALY}, nil
}

// Commercial constructs Date.commercial(cwyear, cweek, cwday) — an ISO week date
// (cwday 1 = Monday .. 7 = Sunday). Out-of-range coordinates yield
// ErrInvalidDate.
func Commercial(cwyear, cweek, cwday int) (*Date, error) {
	if cwday < 0 {
		cwday += 8
	}
	if cwday < 1 || cwday > 7 {
		return nil, ErrInvalidDate
	}
	weeks := weeksInCommercialYear(cwyear)
	if cweek < 0 {
		cweek += weeks + 1
	}
	if cweek < 1 || cweek > weeks {
		return nil, ErrInvalidDate
	}
	jd := commercialToJD(cwyear, cweek, cwday, ITALY)
	return &Date{jd: jd, start: ITALY}, nil
}

// commercialToJD converts an ISO week date to its astronomical JDN.
func commercialToJD(cwyear, cweek, cwday, start int) int {
	jan4, _ := civilToJDStart(cwyear, 1, 4, start)
	// Monday of ISO week 1 (the week containing Jan 4).
	week1Mon := jan4 - jdWday(jan4) + 1
	return week1Mon + (cweek-1)*7 + (cwday - 1)
}

// weeksInCommercialYear returns 52 or 53, the number of ISO weeks in cwyear.
func weeksInCommercialYear(cwyear int) int {
	p := func(y int) int { return floorMod(y+floorDiv(y, 4)-floorDiv(y, 100)+floorDiv(y, 400), 7) }
	if p(cwyear) == 4 || p(cwyear-1) == 3 {
		return 53
	}
	return 52
}

// jdWday returns the ISO weekday of a JDN: 1 = Monday .. 7 = Sunday.
func jdWday(jd int) int { return floorMod(jd, 7) + 1 }

// leapYear reports whether y is a proleptic-Gregorian leap year.
func leapYear(y int) bool { return y%4 == 0 && (y%100 != 0 || y%400 == 0) }

// julianLeap reports whether y is a Julian leap year (every fourth year).
func julianLeap(y int) bool { return floorMod(y, 4) == 0 }

// leapYearStart reports whether year y is a leap year under the reform: the
// calendar that governs that year's Feb/Mar boundary decides. (Year 1582 itself
// is governed by Gregorian under ITALY since Mar 1 1582 precedes the reform but
// Feb is Julian; MRI uses Julian for 1582's leap rule, which leapYearStart
// reproduces by testing Mar 1.)
func leapYearStart(y, start int) bool {
	if gregorianToJD(y, 3, 1) >= start {
		return leapYear(y)
	}
	return julianLeap(y)
}

// daysInMonth holds the (non-leap) length of each month, indexed 1..12.
var daysInMonth = [...]int{0, 31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}

// lastDayOfMonthStart returns the last day-of-month for (y, m) under the given
// reform, accounting for the calendar's leap February.
func lastDayOfMonthStart(y, m, start int) int {
	if m == 2 && leapYearStart(y, start) {
		return 29
	}
	return daysInMonth[m]
}

// Today returns the current local date as a plain Date. It is the only place the
// calendar core consults the wall clock; tests inject a deterministic clock via
// SetToday so they never depend on real time.
func Today() *Date {
	t := nowFunc()
	return &Date{jd: civilToJD(t.Year(), int(t.Month()), t.Day()), start: ITALY}
}

// nowFunc is the injectable clock behind Today; tests replace it via SetToday.
var nowFunc = func() time.Time { return time.Now() }

// SetToday overrides the clock Today consults and returns a function that
// restores the previous one — the test seam that keeps "today" deterministic.
func SetToday(y, m, d int) func() {
	prev := nowFunc
	nowFunc = func() time.Time { return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC) }
	return func() { nowFunc = prev }
}

// SetTodayInstant is SetToday with a full wall-clock instant, so tests that
// exercise Now's time-of-day are deterministic too. It returns the restore
// function.
func SetTodayInstant(y, m, d, h, min, s, ns int) func() {
	prev := nowFunc
	nowFunc = func() time.Time { return time.Date(y, time.Month(m), d, h, min, s, ns, time.UTC) }
	return func() { nowFunc = prev }
}

// Now returns the current local instant as a DateTime in UTC. Like Today it is
// the sole wall-clock consumer for DateTime and is driven by the same seam.
func Now() *Date {
	t := nowFunc().UTC()
	jd := civilToJD(t.Year(), int(t.Month()), t.Day())
	ns := int64(t.Hour())*3600e9 + int64(t.Minute())*60e9 + int64(t.Second())*1e9 + int64(t.Nanosecond())
	return &Date{jd: jd, nsec: ns, offset: 0, start: ITALY, isDateTime: true}
}

// NewDateTime constructs DateTime.new(y, m, d, h, min, s, offset). The offset is
// in seconds east of UTC. An invalid calendar date or out-of-range clock field
// yields ErrInvalidDate.
func NewDateTime(y, m, d, h, min, s, offsetSec int) (*Date, error) {
	base, err := NewDate(y, m, d)
	if err != nil {
		return nil, err
	}
	if h < 0 || h > 24 || min < 0 || min > 59 || s < 0 || s > 59 {
		return nil, ErrInvalidDate
	}
	ns := int64(h)*3600e9 + int64(min)*60e9 + int64(s)*1e9
	base.nsec = ns
	base.offset = offsetSec
	base.isDateTime = true
	return base, nil
}

// withTime returns a copy of d carrying the time-of-day fields, marked as a
// DateTime — the internal helper the parsers use when a time component is seen.
func (d *Date) withTime(ns int64, offsetSec int) *Date {
	return &Date{jd: d.jd, nsec: ns, offset: offsetSec, start: d.start, isDateTime: true}
}

// IsDateTime reports whether d carries a time-of-day (a DateTime) rather than
// being a bare calendar Date.
func (d *Date) IsDateTime() bool { return d.isDateTime }

// dateOnly returns the bare calendar Date for d, dropping any time-of-day and
// offset — the form Date.parse / Date.strptime yield from a timestamped string.
func (d *Date) dateOnly() *Date {
	return &Date{jd: d.jd, start: d.start}
}

// ToDate is the public form of dateOnly: the calendar Date underlying a DateTime
// (DateTime#to_date), with the clock dropped.
func (d *Date) ToDate() *Date { return d.dateOnly() }
