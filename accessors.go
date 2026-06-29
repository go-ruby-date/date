// Copyright (c) the go-ruby-date/date authors
//
// SPDX-License-Identifier: BSD-3-Clause

package date

// This file holds the field accessors and arithmetic — the Date#year / #month /
// #wday family, the JD/MJD conversions, and the day/month shift operators — all
// derived from the stored Julian Day Number so they stay consistent.

// ymd returns d's calendar (year, month, day) under its own reform.
func (d *Date) ymd() (int, int, int) { return jdToCivilStart(d.jd, d.start) }

// Year returns the calendar year (negative for BC), under d's reform.
func (d *Date) Year() int { y, _, _ := d.ymd(); return y }

// Month returns the month, 1 = January .. 12 = December (Date#month / #mon).
func (d *Date) Month() int { _, m, _ := d.ymd(); return m }

// Day returns the day-of-month, 1 .. 31 (Date#day / #mday).
func (d *Date) Day() int { _, _, day := d.ymd(); return day }

// Wday returns the day-of-week, 0 = Sunday .. 6 = Saturday (Date#wday).
func (d *Date) Wday() int { return floorMod(d.jd+1, 7) }

// Yday returns the 1-based day-of-year, 1 .. 366 (Date#yday).
func (d *Date) Yday() int {
	y := d.Year()
	jan1, _ := civilToJDStart(y, 1, 1, d.start)
	return d.jd - jan1 + 1
}

// Jd returns the astronomical Julian Day Number (Date#jd).
func (d *Date) Jd() int { return d.jd }

// Mjd returns the Modified Julian Day Number (Date#mjd) — jd minus 2_400_001.
func (d *Date) Mjd() int { return d.jd - 2400001 }

// Cwday returns the ISO commercial day-of-week, 1 = Monday .. 7 = Sunday
// (Date#cwday).
func (d *Date) Cwday() int { return jdWday(d.jd) }

// Cweek returns the ISO commercial week number, 1 .. 53 (Date#cweek).
func (d *Date) Cweek() int { _, w := d.commercial(); return w }

// Cwyear returns the ISO commercial week-numbering year (Date#cwyear), which can
// differ from the calendar year at the year boundary.
func (d *Date) Cwyear() int { y, _ := d.commercial(); return y }

// commercial returns the ISO (cwyear, cweek) pair for d.
func (d *Date) commercial() (cwyear, cweek int) {
	// The Thursday of d's week determines the ISO year, read on d's own reform
	// calendar (so a pre-1582 date uses the Julian year, as MRI does).
	thursday := d.jd - jdWday(d.jd) + 4
	cwyear, _, _ = jdToCivilStart(thursday, d.start)
	week1Mon := commercialToJD(cwyear, 1, 1, d.start)
	cweek = floorDiv(d.jd-week1Mon, 7) + 1
	return cwyear, cweek
}

// Leap reports whether d's year is a leap year (Date#leap?), under its reform.
func (d *Date) Leap() bool { return leapYearStart(d.Year(), d.start) }

// Hour returns the hour-of-day, 0 .. 23 (DateTime#hour).
func (d *Date) Hour() int { return int(d.nsec / 3600e9) }

// Min returns the minute-of-hour, 0 .. 59 (DateTime#minute / #min).
func (d *Date) Min() int { return int(d.nsec/60e9) % 60 }

// Sec returns the second-of-minute, 0 .. 59 (DateTime#second / #sec).
func (d *Date) Sec() int { return int(d.nsec/1e9) % 60 }

// NsecOfDay returns the nanoseconds since local midnight (internal/whole-day).
func (d *Date) NsecOfDay() int64 { return d.nsec }

// SecFractionNanos returns the sub-second part in nanoseconds, 0 .. 999_999_999
// (the numerator MRI's DateTime#sec_fraction reports as a Rational over 1e9).
func (d *Date) SecFractionNanos() int64 { return d.nsec % 1e9 }

// Offset returns the UTC offset in seconds east of Greenwich (DateTime#offset is
// this over 86_400 as a day fraction; callers that need the fraction divide).
func (d *Date) Offset() int { return d.offset }

// Plus returns d shifted forward by n days (Date#+ / Date#next_day(n)).
func (d *Date) Plus(n int) *Date {
	c := *d
	c.jd += n
	return &c
}

// Minus returns d shifted back by n days (Date#- / Date#prev_day(n)).
func (d *Date) Minus(n int) *Date { return d.Plus(-n) }

// Diff returns the whole number of days from o to d (d - o), positive when d is
// later — MRI's Date#- between two dates yields a Rational that, for whole
// midnights, reduces to this integer.
func (d *Date) Diff(o *Date) int { return d.jd - o.jd }

// PlusMonths returns d shifted by n calendar months (Date#>> for n>0, Date#<< for
// n<0), with MRI's day-of-month clamping (Jan 31 >> 1 → Feb 28).
func (d *Date) PlusMonths(n int) *Date {
	y, m, day := d.ymd()
	total := (y*12 + (m - 1)) + n
	ny := floorDiv(total, 12)
	nm := floorMod(total, 12) + 1
	if last := lastDayOfMonthStart(ny, nm, d.start); day > last {
		day = last
	}
	c := *d
	c.jd, _ = civilToJDStart(ny, nm, day, d.start)
	return &c
}

// PlusYears returns d shifted by n years (Date#next_year(n) / #prev_year), a
// 12n-month shift with the same clamping (Feb 29 → Feb 28 off a leap year).
func (d *Date) PlusYears(n int) *Date { return d.PlusMonths(n * 12) }

// NextDay / PrevDay / NextMonth / PrevMonth / NextYear / PrevYear are the
// MRI-named one-step (defaulting) shifts. Callers wanting n steps pass it.

// NextDay returns d + n days (default n = 1).
func (d *Date) NextDay(n int) *Date { return d.Plus(n) }

// PrevDay returns d - n days (default n = 1).
func (d *Date) PrevDay(n int) *Date { return d.Plus(-n) }

// NextMonth returns d shifted forward n months (default n = 1).
func (d *Date) NextMonth(n int) *Date { return d.PlusMonths(n) }

// PrevMonth returns d shifted back n months (default n = 1).
func (d *Date) PrevMonth(n int) *Date { return d.PlusMonths(-n) }

// NextYear returns d shifted forward n years (default n = 1).
func (d *Date) NextYear(n int) *Date { return d.PlusYears(n) }

// PrevYear returns d shifted back n years (default n = 1).
func (d *Date) PrevYear(n int) *Date { return d.PlusYears(-n) }

// Succ / Next return the following day (Date#succ / #next).
func (d *Date) Succ() *Date { return d.Plus(1) }

// Cmp orders d against o: -1 if earlier, 0 if equal, +1 if later (Date#<=>),
// comparing by jd and, for DateTimes, by the instant (jd-day plus the
// offset-adjusted time-of-day).
func (d *Date) Cmp(o *Date) int {
	dj, dn := d.utcInstant()
	oj, on := o.utcInstant()
	switch {
	case dj != oj:
		if dj < oj {
			return -1
		}
		return 1
	case dn < on:
		return -1
	case dn > on:
		return 1
	default:
		return 0
	}
}

// utcInstant normalises d to UTC and returns (whole JDN, nanoseconds-within-day),
// the overflow-free key Cmp/Equal use so a Date (midnight, offset 0) and an
// equal-instant DateTime compare correctly across the whole supported range.
func (d *Date) utcInstant() (jd int, nsec int64) {
	ns := d.nsec - int64(d.offset)*int64(1e9)
	carry := floorDiv64(ns, nsPerDay)
	return d.jd + int(carry), ns - carry*nsPerDay
}

// unixEpochJD is the astronomical Julian Day Number of 1970-01-01.
const unixEpochJD = 2440588

// Equal reports whether d and o denote the same instant (Date#==).
func (d *Date) Equal(o *Date) bool { return d.Cmp(o) == 0 }

// Step calls fn for each date from d to limit inclusive, stepping by step days
// (which may be negative), mirroring Date#step. It returns the number of steps
// taken. A zero step would not terminate and so takes none.
func (d *Date) Step(limit *Date, step int, fn func(*Date)) int {
	if step == 0 {
		return 0
	}
	n := 0
	for cur := d; ; cur = cur.Plus(step) {
		if step > 0 && cur.jd > limit.jd {
			break
		}
		if step < 0 && cur.jd < limit.jd {
			break
		}
		fn(cur)
		n++
	}
	return n
}

// Upto calls fn for each date from d to limit inclusive, one day at a time
// (Date#upto); it is Step with a step of 1.
func (d *Date) Upto(limit *Date, fn func(*Date)) int { return d.Step(limit, 1, fn) }

// Downto calls fn for each date from d down to limit inclusive (Date#downto).
func (d *Date) Downto(limit *Date, fn func(*Date)) int { return d.Step(limit, -1, fn) }
