<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-date/brand/main/social/go-ruby-date-date.png" alt="go-ruby-date/date" width="720"></p>

# date — go-ruby-date

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-date.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of Ruby's [`date`](https://docs.ruby-lang.org/en/master/Date.html)
standard library** — MRI 4.0.5's `Date` and `DateTime`. It is the deterministic,
interpreter-independent core of calendar arithmetic, parsing and formatting:
construct a date from civil / ordinal / week-date / Julian-Day coordinates, shift
it by days / months / years, render it with the full `strftime` directive set,
and read it back with `Date.parse` / `Date.strptime` — **without any Ruby
runtime**, matching MRI byte-for-byte across a broad differential corpus.

It is the `date` backend for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), but is a
**standalone, reusable** module with no dependency on the Ruby runtime — a sibling
of [go-ruby-regexp](https://github.com/go-ruby-regexp/regexp) (the Onigmo engine),
[go-ruby-erb](https://github.com/go-ruby-erb/erb) (the ERB compiler) and
[go-ruby-yaml](https://github.com/go-ruby-yaml/yaml) (the Psych port).

> **What it is — and isn't.** Calendar math, the `strftime` directive engine and
> the `parse` / `strptime` readers are fully deterministic and need **no
> interpreter**, so they live here as pure Go built on the astronomical Julian Day
> Number — exactly as MRI's `date_core.c` does. Binding these values to live Ruby
> `Date` / `DateTime` objects is the host's job; this library hands back a small,
> idiomatic Go `*Date` the host maps to and from its own objects.

## Features

A faithful port of MRI's `date`, validated against the `ruby` binary on every
supported platform:

- **The Julian Day Number model** — arbitrary BC/AD years on the proleptic
  calendar, with the **Julian → Gregorian reform** (`Date::ITALY` default,
  `ENGLAND` / `GREGORIAN` / `JULIAN`, or any JDN): pre-1582 dates use the Julian
  calendar, the 1582-10-05…14 gap is invalid, and leap-year rules switch with the
  calendar.
- **Constructors** — `NewDate` (`Date.new` / `civil`), `DateJD` (`jd`),
  `Ordinal`, `Commercial` (ISO week date), `NewDateTime`, plus the deterministic
  `Today` / `Now` seam (`SetToday` / `SetTodayInstant` for tests — never real
  time in tests).
- **The full `strftime` directive set** —
  `%Y %y %C %m %B %b %h %d %e %j %H %k %I %l %M %S %L %N %p %P %A %a %u %w %U %W
  %V %G %g %s %Q %z %Z %D %F %T %R %r %v %c %x %X %n %t %%` — with the `- _ 0 ^ #`
  flags, an explicit field width, the `:` / `::` / `:::` `%z` variants, the
  ignored `E` / `O` locale modifiers, and the MRI year-padding quirk (`%Y` of a
  negative year is sign-plus-four-digits).
- **`Parse`** (heuristic, multi-format — ISO, slashed/dashed, month-name,
  RFC-2822, week-date, ordinal; `comp` two-digit-year expansion) and
  **`Strptime`** (explicit format, including `%s` / `%Q` epoch input).
- **Arithmetic** — `+`/`-` days (`Plus` / `Minus` / `Diff`), `>>`/`<<` months
  (`PlusMonths`, day clamping), `PlusYears`, `Next*` / `Prev*`, and
  `Step` / `Upto` / `Downto`.
- **Accessors** — `Year Month Day Wday Yday Cwyear Cweek Cwday Jd Mjd Leap`, and
  for `DateTime` `Hour Min Sec SecFractionNanos Offset`.
- **Named formats** — `Iso8601`, `Rfc3339`, `Rfc2822`, `Httpdate`, `Ctime`,
  `Jisx0301` (Japanese eras), plus `Cmp` / `Equal` (instant-aware).

## Usage

```go
package main

import (
	"fmt"

	"github.com/go-ruby-date/date"
)

func main() {
	d, _ := date.NewDate(2026, 6, 29)
	fmt.Println(d.Strftime("%A, %-d %B %Y"))     // Monday, 29 June 2026
	fmt.Println(d.Iso8601(), d.Jd(), d.Cweek())  // 2026-06-29 2461221 27
	fmt.Println(d.PlusMonths(8).String())        // 2027-02-28  (Jun 29 → Feb clamp)

	dt, _ := date.NewDateTime(2026, 6, 29, 14, 3, 5, 19800) // +05:30
	fmt.Println(dt.Strftime("%Y-%m-%dT%H:%M:%S%:z"))        // 2026-06-29T14:03:05+05:30
	fmt.Println(dt.Strftime("%s"))                          // 1782721985

	p, _ := date.Parse("June 29, 2026", false)
	fmt.Println(p.Iso8601())                                // 2026-06-29

	s, _ := date.Strptime("2026-180", "%Y-%j")
	fmt.Println(s.Iso8601())                                // 2026-06-29
}
```

## Tests & coverage

```sh
go test -race -coverprofile=cover.out ./...
go tool cover -func=cover.out | tail -1   # total: (statements) 100.0%
```

The suite has two layers: deterministic, Ruby-free **golden tests** (with
MRI-4.0.5-verified expectations inline) that hold to **100% coverage** on the
no-Ruby, qemu cross-arch and Windows lanes, and a differential **MRI oracle**
(gated on Ruby ≥ 4.0, `$stdout.binmode` for exact bytes on Windows) that runs
where `ruby` is present. CI builds and tests on three OSes and all six supported
64-bit architectures (amd64, arm64, riscv64, loong64, ppc64le, s390x).

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-date/date authors.

## WebAssembly

Being pure Go (CGO=0), this library also compiles to **WebAssembly** — both
`GOOS=js GOARCH=wasm` (browser / Node.js) and `GOOS=wasip1 GOARCH=wasm` (WASI).
CI builds both targets on every push, alongside the six 64-bit native/qemu arches.

```sh
GOOS=js     GOARCH=wasm go build ./...   # browser / Node
GOOS=wasip1 GOARCH=wasm go build ./...   # WASI (wasmtime, wasmer, wasmedge, …)
```
