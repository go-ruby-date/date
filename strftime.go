// Copyright (c) the go-ruby-date/date authors
//
// SPDX-License-Identifier: BSD-3-Clause

package date

import (
	"strconv"
	"strings"
)

// This file implements Date#strftime — the directive engine. It walks the format
// string, and for each "%[flags][width]conv" sequence emits the field MRI would,
// honouring the - _ 0 ^ # flags, an explicit field width, the E/O locale
// modifiers (parsed and ignored, as MRI does), and the recursive compound
// directives (%c %D %F %T …). An unrecognised directive is emitted verbatim,
// matching MRI.

var monthNames = [...]string{
	"", "January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December",
}

var monthAbbr = [...]string{
	"", "Jan", "Feb", "Mar", "Apr", "May", "Jun",
	"Jul", "Aug", "Sep", "Oct", "Nov", "Dec",
}

var dayNames = [...]string{
	"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday",
}

var dayAbbr = [...]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

// Strftime formats d per the Ruby format string (Date#strftime / DateTime#strftime).
func (d *Date) Strftime(format string) string {
	var b strings.Builder
	d.strftimeInto(&b, format)
	return b.String()
}

// strftimeInto is the recursive worker so compound directives (%c expands to
// "%a %b %e %T %Y", etc.) reuse the same flag/width machinery.
func (d *Date) strftimeInto(b *strings.Builder, format string) {
	i := 0
	for i < len(format) {
		c := format[i]
		if c != '%' {
			b.WriteByte(c)
			i++
			continue
		}
		spec, conv, ok := parseDirective(format, i)
		if !ok {
			// A lone trailing '%' (or an empty directive) is emitted verbatim.
			b.WriteByte('%')
			i++
			continue
		}
		d.emitDirective(b, spec, conv)
		i = spec.end
	}
}

// directive captures the parsed pieces of one %-sequence.
type directive struct {
	flagMinus bool // '-': suppress padding
	flagSpace bool // '_': pad with spaces
	flagZero  bool // '0': pad with zeros
	flagUp    bool // '^': uppercase
	flagSwap  bool // '#': swap case
	hasWidth  bool
	width     int
	colons    int // count of ':' before z (for %z variants)
	end       int // index just past the directive in the source
}

// parseDirective parses "%[flags][width][:...][E|O]conv" starting at format[i]
// (which is '%'). It returns the parsed directive, the conversion byte, and
// whether a real directive (not a bare '%') was found.
func parseDirective(format string, i int) (directive, byte, bool) {
	j := i + 1
	var dir directive
	// Flags (any order, repeatable).
	for j < len(format) {
		switch format[j] {
		case '-':
			dir.flagMinus = true
		case '_':
			dir.flagSpace = true
		case '0':
			dir.flagZero = true
		case '^':
			dir.flagUp = true
		case '#':
			dir.flagSwap = true
		default:
			goto width
		}
		j++
	}
width:
	// Optional width.
	start := j
	for j < len(format) && format[j] >= '0' && format[j] <= '9' {
		j++
	}
	if j > start {
		dir.hasWidth = true
		dir.width, _ = strconv.Atoi(format[start:j])
	}
	// Leading colons (only meaningful before 'z').
	for j < len(format) && format[j] == ':' {
		dir.colons++
		j++
	}
	// E / O locale modifiers are accepted and ignored.
	if j < len(format) && (format[j] == 'E' || format[j] == 'O') {
		j++
	}
	if j >= len(format) {
		return dir, 0, false
	}
	conv := format[j]
	dir.end = j + 1
	return dir, conv, true
}

// emitDirective renders one conversion into b.
func (d *Date) emitDirective(b *strings.Builder, dir directive, conv byte) {
	switch conv {
	case '%':
		b.WriteString(dir.apply("%", padSpace, 0))
	case 'n':
		b.WriteString("\n")
	case 't':
		b.WriteString("\t")
	case 'Y':
		b.WriteString(dir.applyYear(d.Year(), 4))
	case 'C':
		b.WriteString(dir.applyNum(divFloor(d.Year(), 100), padZero, 2))
	case 'y':
		b.WriteString(dir.applyNum(floorMod(d.Year(), 100), padZero, 2))
	case 'm':
		b.WriteString(dir.applyNum(d.Month(), padZero, 2))
	case 'B':
		b.WriteString(dir.apply(monthNames[d.Month()], padSpace, 0))
	case 'b', 'h':
		b.WriteString(dir.apply(monthAbbr[d.Month()], padSpace, 0))
	case 'd':
		b.WriteString(dir.applyNum(d.Day(), padZero, 2))
	case 'e':
		b.WriteString(dir.applyNum(d.Day(), padSpaceNum, 2))
	case 'j':
		b.WriteString(dir.applyNum(d.Yday(), padZero, 3))
	case 'H':
		b.WriteString(dir.applyNum(d.Hour(), padZero, 2))
	case 'k':
		b.WriteString(dir.applyNum(d.Hour(), padSpaceNum, 2))
	case 'I':
		b.WriteString(dir.applyNum(hour12(d.Hour()), padZero, 2))
	case 'l':
		b.WriteString(dir.applyNum(hour12(d.Hour()), padSpaceNum, 2))
	case 'M':
		b.WriteString(dir.applyNum(d.Min(), padZero, 2))
	case 'S':
		b.WriteString(dir.applyNum(d.Sec(), padZero, 2))
	case 'L':
		b.WriteString(d.fracDigits(dir, 3))
	case 'N':
		b.WriteString(d.fracDigits(dir, 9))
	case 'p':
		b.WriteString(dir.apply(ampm(d.Hour(), true), padSpace, 0))
	case 'P':
		b.WriteString(dir.apply(ampm(d.Hour(), false), padSpace, 0))
	case 'A':
		b.WriteString(dir.apply(dayNames[d.Wday()], padSpace, 0))
	case 'a':
		b.WriteString(dir.apply(dayAbbr[d.Wday()], padSpace, 0))
	case 'u':
		b.WriteString(dir.applyNum(d.Cwday(), padZero, 1))
	case 'w':
		b.WriteString(dir.applyNum(d.Wday(), padZero, 1))
	case 'U':
		b.WriteString(dir.applyNum(d.weekNumber(0), padZero, 2))
	case 'W':
		b.WriteString(dir.applyNum(d.weekNumber(1), padZero, 2))
	case 'V':
		b.WriteString(dir.applyNum(d.Cweek(), padZero, 2))
	case 'G':
		b.WriteString(dir.applyYear(d.Cwyear(), 4))
	case 'g':
		b.WriteString(dir.applyNum(floorMod(d.Cwyear(), 100), padZero, 2))
	case 's':
		b.WriteString(dir.applyNum64(d.epochSeconds(), padZero, 1))
	case 'Q':
		b.WriteString(dir.applyNum64(d.epochMillis(), padZero, 1))
	case 'z':
		b.WriteString(dir.apply(d.formatOffset(dir.colons), padZero, 0))
	case 'Z':
		b.WriteString(dir.apply(d.zoneName(), padSpace, 0))
	case 'D':
		d.strftimeInto(b, "%m/%d/%y")
	case 'F':
		d.strftimeInto(b, "%Y-%m-%d")
	case 'T', 'X':
		d.strftimeInto(b, "%H:%M:%S")
	case 'R':
		d.strftimeInto(b, "%H:%M")
	case 'r':
		d.strftimeInto(b, "%I:%M:%S %p")
	case 'c':
		d.strftimeInto(b, "%a %b %e %H:%M:%S %Y")
	case 'x':
		d.strftimeInto(b, "%m/%d/%y")
	case 'v':
		d.strftimeInto(b, "%e-%^b-%Y")
	default:
		// Unknown directive: emit the original "%conv" verbatim (flags dropped,
		// matching MRI's behaviour for e.g. "%q").
		b.WriteByte('%')
		b.WriteByte(conv)
	}
}

// hour12 maps a 24-hour clock value to a 12-hour one (0 and 12 → 12).
func hour12(h int) int {
	h %= 12
	if h == 0 {
		return 12
	}
	return h
}

// ampm returns the AM/PM marker; upper selects "AM"/"PM" vs "am"/"pm".
func ampm(h int, upper bool) string {
	switch {
	case h < 12 && upper:
		return "AM"
	case h < 12:
		return "am"
	case upper:
		return "PM"
	default:
		return "pm"
	}
}

// divFloor is floored division used for the century (%C), so negative years give
// MRI's century.
func divFloor(a, b int) int { return floorDiv(a, b) }

// weekNumber returns the %U (firstWday 0 = Sunday) or %W (firstWday 1 = Monday)
// week-of-year, 00 .. 53 — the count of full weeks before d, with the partial
// leading week numbered 00.
func (d *Date) weekNumber(firstWday int) int {
	yday := d.Yday()
	wday := d.Wday()
	// Days from the week's start-day to this day.
	off := floorMod(wday-firstWday, 7)
	return (yday - off + 6) / 7
}

// fracDigits renders the sub-second field to n digits (%L → 3, %N → 9), honouring
// an explicit width that truncates (width < 9) or zero-pads (width > 9) the
// nanosecond fraction.
func (d *Date) fracDigits(dir directive, def int) string {
	n := def
	if dir.hasWidth {
		n = dir.width
	}
	// Full nine-digit nanosecond string.
	full := zeroPad(strconv.FormatInt(d.SecFractionNanos(), 10), 9)
	if n <= 9 {
		return full[:n]
	}
	return full + strings.Repeat("0", n-9)
}

// epochSeconds returns Unix seconds for the instant (%s). It works in seconds
// (not nanoseconds) so ancient dates do not overflow int64: whole days scale by
// 86_400, the time-of-day and offset add their seconds.
func (d *Date) epochSeconds() int64 {
	days := int64(d.jd - unixEpochJD)
	return days*86400 + floorDiv64(d.nsec, int64(1e9)) - int64(d.offset)
}

// epochMillis returns Unix milliseconds for the instant (%Q).
func (d *Date) epochMillis() int64 {
	days := int64(d.jd - unixEpochJD)
	return days*86400*1000 + floorDiv64(d.nsec, int64(1e6)) - int64(d.offset)*1000
}

// formatOffset renders the UTC offset (%z) in the variant selected by the number
// of colons: 0 → "+HHMM", 1 → "+HH:MM", 2 → "+HH:MM:SS", 3 → minimal "+HH" with
// :MM / :SS appended only when non-zero.
func (d *Date) formatOffset(colons int) string {
	off := d.offset
	sign := "+"
	if off < 0 {
		sign = "-"
		off = -off
	}
	h := off / 3600
	m := (off % 3600) / 60
	s := off % 60
	hh := zeroPad(strconv.Itoa(h), 2)
	mm := zeroPad(strconv.Itoa(m), 2)
	ss := zeroPad(strconv.Itoa(s), 2)
	switch colons {
	case 1:
		return sign + hh + ":" + mm
	case 2:
		return sign + hh + ":" + mm + ":" + ss
	case 3:
		out := sign + hh
		if s != 0 {
			out += ":" + mm + ":" + ss
		} else if m != 0 {
			out += ":" + mm
		}
		return out
	default:
		return sign + hh + mm
	}
}

// zoneName returns the %Z zone string. MRI renders it as the colon offset
// ("+05:30"); a plain Date with no offset is "+00:00".
func (d *Date) zoneName() string { return d.formatOffset(1) }

// padding strategy selectors for apply / applyNum.
type padMode int

const (
	padZero     padMode = iota // numeric, zero-filled by default
	padSpace                   // string, space-filled by default (no padding unless width)
	padSpaceNum                // numeric, space-filled by default (%e, %k, %l)
)

// applyNum formats an int with the conversion's natural width and pad mode, then
// applies the directive's flags.
func (dir directive) applyNum(n int, mode padMode, natural int) string {
	return dir.applyNumImpl(int64(n), mode, natural, false)
}

// applyYear formats a year-style field (%Y / %G): the absolute value is padded
// to the natural width and the sign, if any, is prepended outside it (so a
// negative year is one column wider than the natural width). An explicit width
// reverts to padding the whole signed value.
func (dir directive) applyYear(n int, natural int) string {
	return dir.applyNumImpl(int64(n), padZero, natural, true)
}

// applyNum64 is applyNum for 64-bit values (epoch seconds/millis).
func (dir directive) applyNum64(n int64, mode padMode, natural int) string {
	return dir.applyNumImpl(n, mode, natural, false)
}

// applyNumImpl is the shared numeric formatter. yearStyle selects the %Y/%G
// abs-pad behaviour for default widths.
func (dir directive) applyNumImpl(n int64, mode padMode, natural int, yearStyle bool) string {
	neg := n < 0
	digits := strconv.FormatInt(n, 10)
	if neg {
		digits = digits[1:]
	}
	if dir.flagMinus {
		if neg {
			return "-" + digits
		}
		return digits
	}
	width := natural
	if dir.hasWidth {
		width = dir.width
	}
	if yearStyle && !dir.hasWidth {
		// Pad the absolute value to the natural width; the sign is extra.
		digits = zeroPad(digits, width)
		if neg {
			return "-" + digits
		}
		return digits
	}
	// Pad the whole signed value to the width.
	signed := digits
	if neg {
		signed = "-" + digits
	}
	switch {
	case dir.flagSpace:
		return spacePad(signed, width)
	case mode == padSpaceNum && !dir.flagZero:
		return spacePad(signed, width)
	default:
		return zeroPadSigned(signed, width)
	}
}

// zeroPadSigned zero-pads a possibly-signed numeric string to width, inserting
// the zeros after the sign ("-12" → width 5 → "-0012").
func zeroPadSigned(s string, width int) string {
	if len(s) >= width || s == "" || (s[0] != '-' && s[0] != '+') {
		return zeroPad(s, width)
	}
	return string(s[0]) + zeroPad(s[1:], width-1)
}

// apply formats a string field, honouring the case (^ / #) and width flags.
func (dir directive) apply(s string, mode padMode, _ int) string {
	if dir.flagUp {
		s = strings.ToUpper(s)
	} else if dir.flagSwap {
		// MRI's '#' flag means "change case": a field with any lowercase letter
		// is upcased ("Monday" → "MONDAY", "pm" → "PM"), an all-uppercase field is
		// downcased ("PM" → "pm").
		if hasLower(s) {
			s = strings.ToUpper(s)
		} else {
			s = strings.ToLower(s)
		}
	}
	if dir.hasWidth && !dir.flagMinus {
		s = spacePad(s, dir.width)
	}
	return s
}

// zeroPad / spacePad left-pad s to width with '0' / ' '.
func zeroPad(s string, width int) string  { return pad(s, width, '0') }
func spacePad(s string, width int) string { return pad(s, width, ' ') }

func pad(s string, width int, ch byte) string {
	if len(s) >= width {
		return s
	}
	return strings.Repeat(string(ch), width-len(s)) + s
}

// hasLower reports whether s contains an ASCII lowercase letter — the test the
// '#' flag uses to decide which way to change case.
func hasLower(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 'a' && s[i] <= 'z' {
			return true
		}
	}
	return false
}
