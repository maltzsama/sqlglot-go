package optimizer

import (
	"fmt"
	"strconv"
	"time"
)

// pyDate is a Python datetime.date or datetime.datetime value, as simplify's date folding
// (optimizer/simplify.py:367-535) manipulates them: a date has no time part, and str() and the
// arithmetic below follow the type. An aware datetime (one parsed with a UTC offset) keeps the
// offset in t's location.
type pyDate struct {
	t          time.Time
	isDatetime bool
	aware      bool
}

func newPyDate(year, month, day int) pyDate {
	return pyDate{t: time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)}
}

// asDatetime ports datetime(year=..., month=..., day=...) from a date, or the value itself.
func (d pyDate) asDatetime() pyDate {
	d.isDatetime = true
	return d
}

// asDate ports datetime.date(): the date part.
func (d pyDate) asDate() pyDate {
	y, m, day := d.t.Date()
	return newPyDate(y, int(m), day)
}

// String ports str(date) / str(datetime): YYYY-MM-DD, plus " HH:MM:SS[.ffffff][+HH:MM]".
func (d pyDate) String() string {
	s := d.t.Format("2006-01-02")
	if !d.isDatetime {
		return s
	}
	s += d.t.Format(" 15:04:05")
	if us := d.t.Nanosecond() / 1000; us != 0 {
		s += fmt.Sprintf(".%06d", us)
	}
	if d.aware {
		_, offset := d.t.Zone()
		sign := "+"
		if offset < 0 {
			sign, offset = "-", -offset
		}
		s += fmt.Sprintf("%s%02d:%02d", sign, offset/3600, offset%3600/60)
		if sec := offset % 60; sec != 0 {
			s += fmt.Sprintf(":%02d", sec)
		}
	}
	return s
}

// comparable reports whether Python can order the two values: an aware datetime and a naive
// one raise TypeError there, and simplify then gives up on the comparison.
func (d pyDate) comparable(o pyDate) bool { return d.aware == o.aware }

func (d pyDate) cmp(o pyDate) int {
	switch {
	case d.t.Before(o.t):
		return -1
	case d.t.After(o.t):
		return 1
	}
	return 0
}

// parseISODateParts ports date.fromisoformat into year, month, day.
func parseISODateParts(text string) (int, int, int, bool) {
	if m := isoCalendarRE.FindStringSubmatch(text); m != nil {
		y, mo, d := m[1], m[2], m[3]
		if y == "" {
			y, mo, d = m[4], m[5], m[6]
		}
		year, _ := strconv.Atoi(y)
		month, _ := strconv.Atoi(mo)
		day, _ := strconv.Atoi(d)
		if _, ok := parseISODate(text); !ok {
			return 0, 0, 0, false
		}
		return year, month, day, true
	}
	if m := isoWeekRE.FindStringSubmatch(text); m != nil {
		if _, ok := parseISODate(text); !ok {
			return 0, 0, 0, false
		}
		y, w, wd := m[1], m[2], m[3]
		if y == "" {
			y, w, wd = m[4], m[5], m[6]
		}
		year, _ := strconv.Atoi(y)
		week, _ := strconv.Atoi(w)
		weekday := 1
		if wd != "" {
			weekday, _ = strconv.Atoi(wd)
		}
		// date.fromisocalendar: the Monday of ISO week 1 is the Monday on or before Jan 4.
		jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
		monday := jan4.AddDate(0, 0, -((int(jan4.Weekday()) + 6) % 7))
		t := monday.AddDate(0, 0, (week-1)*7+weekday-1)
		return t.Year(), int(t.Month()), t.Day(), true
	}
	return 0, 0, 0, false
}

// pyFromISODate ports date.fromisoformat.
func pyFromISODate(text string) (pyDate, bool) {
	y, m, d, ok := parseISODateParts(text)
	if !ok {
		return pyDate{}, false
	}
	return newPyDate(y, m, d), true
}

// pyFromISODatetime ports datetime.fromisoformat (Python 3.11+).
func pyFromISODatetime(text string) (pyDate, bool) {
	runes := []rune(text)
	for _, n := range []int{10, 8, 7} {
		if len(runes) < n {
			continue
		}
		y, mo, d, ok := parseISODateParts(string(runes[:n]))
		if !ok {
			continue
		}
		if len(runes) == n {
			return newPyDate(y, mo, d).asDatetime(), true
		}
		if len(runes) <= n+1 {
			continue
		}
		timeText := string(runes[n+1:])
		parts := isoTimeRE.FindStringSubmatch(timeText)
		if parts == nil || !validISOTime(timeText) {
			continue
		}
		hour, _ := strconv.Atoi(parts[1])
		minute, second, micro := 0, 0, 0
		if parts[2] != "" {
			minute, _ = strconv.Atoi(parts[2])
		}
		if parts[3] != "" {
			second, _ = strconv.Atoi(parts[3])
		}
		if frac := parts[4]; frac != "" {
			// Digits beyond microseconds are truncated.
			for len(frac) < 6 {
				frac += "0"
			}
			micro, _ = strconv.Atoi(frac[:6])
		}
		loc := time.UTC
		aware := false
		if off := parts[5]; off != "" {
			aware = true
			if off != "Z" {
				sign := 1
				if off[0] == '-' {
					sign = -1
				}
				digits := ""
				for _, c := range off[1:] {
					if c >= '0' && c <= '9' {
						digits += string(c)
					}
				}
				h, _ := strconv.Atoi(digits[0:2])
				secs := h * 3600
				if len(digits) >= 4 {
					mm, _ := strconv.Atoi(digits[2:4])
					secs += mm * 60
				}
				if len(digits) >= 6 {
					ss, _ := strconv.Atoi(digits[4:6])
					secs += ss
				}
				loc = time.FixedZone("", sign*secs)
			}
		}
		t := time.Date(y, time.Month(mo), d, hour, minute, second, micro*1000, loc)
		return pyDate{t: t, isDatetime: true, aware: aware}, true
	}
	return pyDate{}, false
}

// relativedelta ports the dateutil.relativedelta values simplify builds through interval()
// (simplify.py:448-472): relative years, months and days, and a time delta.
type relativedelta struct {
	years, months, days                   int
	hours, minutes, seconds, microseconds int
}

// isZero ports relativedelta.__bool__ being false: no field set.
func (r relativedelta) isZero() bool {
	return r.years == 0 && r.months == 0 && r.days == 0 && !r.hasTime()
}

func (r relativedelta) hasTime() bool {
	return r.hours != 0 || r.minutes != 0 || r.seconds != 0 || r.microseconds != 0
}

func (r relativedelta) negate() relativedelta {
	return relativedelta{-r.years, -r.months, -r.days, -r.hours, -r.minutes, -r.seconds, -r.microseconds}
}

func daysIn(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// addTo ports relativedelta.__add__(date): years and months first, clamping the day to the
// target month's length, then the day and time delta. A date gains a time part only when the
// delta has one.
func (r relativedelta) addTo(d pyDate) pyDate {
	if r.hasTime() && !d.isDatetime {
		d = d.asDatetime()
	}
	year, month, day := d.t.Date()
	year += r.years
	if r.months != 0 {
		total := int(month) - 1 + r.months
		year += floorDiv(total, 12)
		month = time.Month(total - floorDiv(total, 12)*12 + 1)
	}
	if limit := daysIn(year, month); day > limit {
		day = limit
	}
	h, mi, s := d.t.Clock()
	t := time.Date(year, month, day, h, mi, s, d.t.Nanosecond(), d.t.Location())
	t = t.AddDate(0, 0, r.days)
	t = t.Add(time.Duration(r.hours)*time.Hour + time.Duration(r.minutes)*time.Minute +
		time.Duration(r.seconds)*time.Second + time.Duration(r.microseconds)*time.Microsecond)
	return pyDate{t: t, isDatetime: d.isDatetime, aware: d.aware}
}

func floorDiv(a, b int) int {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

// errUnsupportedUnit ports simplify's UnsupportedUnit: a rule that hits it leaves the
// expression unchanged (the @catch decorator).
type errUnsupportedUnit struct{ unit string }

func (e errUnsupportedUnit) Error() string { return "Unsupported unit: " + e.unit }

// interval ports interval (simplify.py:448-472).
func interval(unit string, n int) relativedelta {
	switch unit {
	case "year":
		return relativedelta{years: n}
	case "quarter":
		return relativedelta{months: 3 * n}
	case "month":
		return relativedelta{months: n}
	case "week":
		return relativedelta{days: 7 * n}
	case "day":
		return relativedelta{days: n}
	case "hour":
		return relativedelta{hours: n}
	case "minute":
		return relativedelta{minutes: n}
	case "second":
		return relativedelta{seconds: n}
	case "millisecond":
		return relativedelta{microseconds: 1000 * n}
	case "microsecond":
		return relativedelta{microseconds: n}
	}
	panic(errUnsupportedUnit{unit})
}

// datetimeFloor ports datetime_floor (simplify.py:475-514); weekOffset is the dialect's
// WEEK_OFFSET (0 = Monday, -1 = Sunday).
func datetimeFloor(d pyDate, unit string, weekOffset int) pyDate {
	if d.isDatetime {
		h, mi, s := d.t.Clock()
		ns := d.t.Nanosecond()
		y, mo, day := d.t.Date()
		switch unit {
		case "hour":
			return pyDate{t: time.Date(y, mo, day, h, 0, 0, 0, d.t.Location()), isDatetime: true, aware: d.aware}
		case "minute":
			return pyDate{t: time.Date(y, mo, day, h, mi, 0, 0, d.t.Location()), isDatetime: true, aware: d.aware}
		case "second":
			return pyDate{t: time.Date(y, mo, day, h, mi, s, 0, d.t.Location()), isDatetime: true, aware: d.aware}
		case "millisecond":
			return pyDate{t: time.Date(y, mo, day, h, mi, s, ns/1e6*1e6, d.t.Location()), isDatetime: true, aware: d.aware}
		case "microsecond":
			return d
		}
	}

	y, mo, day := d.t.Date()
	var result time.Time
	switch unit {
	case "year":
		result = time.Date(y, 1, 1, 0, 0, 0, 0, d.t.Location())
	case "quarter":
		result = time.Date(y, time.Month((int(mo)-1)/3*3+1), 1, 0, 0, 0, 0, d.t.Location())
	case "month":
		result = time.Date(y, mo, 1, 0, 0, 0, 0, d.t.Location())
	case "week":
		weekday := (int(d.t.Weekday()) + 6) % 7 // Python's weekday(): Monday is 0
		back := ((weekday-weekOffset)%7 + 7) % 7
		result = time.Date(y, mo, day, 0, 0, 0, 0, d.t.Location()).AddDate(0, 0, -back)
	case "day":
		result = time.Date(y, mo, day, 0, 0, 0, 0, d.t.Location())
	default:
		panic(errUnsupportedUnit{unit})
	}
	return pyDate{t: result, isDatetime: d.isDatetime, aware: d.aware}
}

// dateCeil ports date_ceil (simplify.py:528-534).
func dateCeil(d pyDate, unit string, weekOffset int) pyDate {
	floor := datetimeFloor(d, unit, weekOffset)
	if floor.cmp(d) == 0 && floor.isDatetime == d.isDatetime {
		return d
	}
	return interval(unit, 1).addTo(floor)
}
