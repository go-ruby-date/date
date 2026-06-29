// Copyright (c) the go-ruby-date/date authors
//
// SPDX-License-Identifier: BSD-3-Clause

package date

import "strconv"

// This file holds the named string formats — to_s / inspect, iso8601, rfc3339,
// rfc2822, httpdate, ctime and jisx0301 — each expressed in terms of the
// strftime engine so they stay byte-for-byte with MRI.

// String renders Date#to_s: a plain Date as "YYYY-MM-DD" and a DateTime as the
// ISO-8601 instant "YYYY-MM-DDTHH:MM:SS±HH:MM".
func (d *Date) String() string {
	if d.isDateTime {
		return d.Strftime("%Y-%m-%dT%H:%M:%S%:z")
	}
	return d.Strftime("%Y-%m-%d")
}

// Iso8601 renders Date#iso8601: identical to to_s for both Date and DateTime.
func (d *Date) Iso8601() string { return d.String() }

// Rfc3339 renders Date#rfc3339 / DateTime#rfc3339 — always the full timestamp
// form, so a plain Date appears at midnight UTC.
func (d *Date) Rfc3339() string {
	return d.Strftime("%Y-%m-%dT%H:%M:%S%:z")
}

// Rfc2822 renders Date#rfc2822 / #rfc822 — "Day, D Mon YYYY HH:MM:SS ±HHMM"
// (the day-of-month is not zero-padded, matching MRI).
func (d *Date) Rfc2822() string {
	return d.Strftime("%a, %-d %b %Y %H:%M:%S %z")
}

// Httpdate renders Date#httpdate — the RFC 1123 / HTTP form in GMT, so the
// instant is shifted to UTC first (which can move it to an adjacent day).
func (d *Date) Httpdate() string {
	return d.toUTC().Strftime("%a, %d %b %Y %H:%M:%S GMT")
}

// Ctime renders Date#ctime / #asctime — "Day Mon DD HH:MM:SS YYYY".
func (d *Date) Ctime() string {
	return d.Strftime("%a %b %e %H:%M:%S %Y")
}

// Asctime is the MRI alias for Ctime.
func (d *Date) Asctime() string { return d.Ctime() }

// toUTC returns the same instant expressed at offset 0 (UTC), used by Httpdate.
func (d *Date) toUTC() *Date {
	// Shift the local time-of-day to UTC and carry the borrow into the day count,
	// keeping every value small (no Unix-epoch-scaled nanosecond intermediate, so
	// no int64 overflow even for ancient dates).
	ns := d.nsec - int64(d.offset)*int64(1e9)
	carry := floorDiv64(ns, nsPerDay)
	ns -= carry * nsPerDay
	return &Date{jd: d.jd + int(carry), nsec: ns, offset: 0, start: d.start, isDateTime: d.isDateTime}
}

// floorDiv64 is the 64-bit floored division toUTC needs for negative instants.
func floorDiv64(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

// jisX0301Eras lists the Japanese era transitions newest-first: the first JDN of
// each era and its one-letter code. A date before Meiji (1868-09-08) has no era
// and falls back to plain ISO, matching MRI.
var jisX0301Eras = []struct {
	startJD  int // first day this era code appears in jisx0301
	baseYear int // calendar year of the era's year 1
	code     string
}{
	{gregorianToJD(2019, 5, 1), 2019, "R"},   // Reiwa
	{gregorianToJD(1989, 1, 8), 1989, "H"},   // Heisei
	{gregorianToJD(1926, 12, 25), 1926, "S"}, // Showa
	{gregorianToJD(1912, 7, 30), 1912, "T"},  // Taisho
	// Japan adopted the Gregorian calendar on 1873-01-01; jisx0301 only uses the
	// Meiji era from that date, though Meiji year 1 is 1868.
	{gregorianToJD(1873, 1, 1), 1868, "M"},
}

// Jisx0301 renders Date#jisx0301 — the Japanese-era calendar form
// "E YY.MM.DD" (e.g. "R08.06.29"), with the time appended for a DateTime. A
// pre-Meiji date falls back to ISO-8601.
func (d *Date) Jisx0301() string {
	for _, era := range jisX0301Eras {
		if d.jd >= era.startJD {
			eraYear := d.Year() - era.baseYear + 1
			head := era.code + zeroPad(strconv.Itoa(eraYear), 2) +
				"." + d.Strftime("%m") + "." + d.Strftime("%d")
			if d.isDateTime {
				return head + d.Strftime("T%H:%M:%S%:z")
			}
			return head
		}
	}
	// Pre-Meiji: ISO fallback.
	return d.Iso8601()
}
