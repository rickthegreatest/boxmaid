package renderer

import "strconv"

// Dates for gantt charts, as milliseconds since 1970 in UTC, with the
// calendar done by hand (days from civil and back, after Howard Hinnant) so
// Go and Rust compute exactly the same dates.

const (
	msSecond = int64(1000)
	msMinute = 60 * msSecond
	msHour   = 60 * msMinute
	msDay    = 24 * msHour
)

func floorDiv(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

// daysFromCivil is the day number of a date (month and day may overflow;
// they roll over like a calendar would).
func daysFromCivil(y, m, d int64) int64 {
	// Fold the month into 1..12 first.
	y += floorDiv(m-1, 12)
	m = (m-1)%12 + 1
	if m <= 0 {
		m += 12
	}
	if m <= 2 {
		y--
	}
	era := floorDiv(y, 400)
	yoe := y - era*400
	mp := (m + 9) % 12
	doy := (153*mp+2)/5 + d - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe - 719468
}

func civilFromDays(z int64) (y, m, d int64) {
	z += 719468
	era := floorDiv(z, 146097)
	doe := z - era*146097
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365
	y = yoe + era*400
	doy := doe - (365*yoe + yoe/4 - yoe/100)
	mp := (5*doy + 2) / 153
	d = doy - (153*mp+2)/5 + 1
	m = mp + 3
	if m > 12 {
		m -= 12
	}
	if m <= 2 {
		y++
	}
	return
}

// weekday is 0 for Sunday … 6 for Saturday.
func weekday(ms int64) int64 {
	return ((floorDiv(ms, msDay) % 7) + 7 + 4) % 7 // 1970-01-01 was a Thursday
}

func dateMS(y, mo, d, h, mi, s, milli int64) int64 {
	return daysFromCivil(y, mo, d)*msDay + h*msHour + mi*msMinute + s*msSecond + milli
}

// addMonths moves a date by whole months, rolling an overflowing day over
// like Go's AddDate (Jan 31 + 1 month is Mar 2 or 3).
func addMonths(ms int64, months int64) int64 {
	days := floorDiv(ms, msDay)
	rest := ms - days*msDay
	y, m, d := civilFromDays(days)
	return daysFromCivil(y, m+months, d)*msDay + rest
}

// parseMomentDate reads s in a Moment.js-style format ("YYYY-MM-DD",
// "DD/MM/YYYY HH:mm", "X" for unix seconds, "x" for milliseconds).
func parseMomentDate(s, format string) (int64, bool) {
	switch format {
	case "X", "x":
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, false
		}
		if format == "X" {
			n *= msSecond
		}
		return n, true
	}
	y, mo, d := int64(1970), int64(1), int64(1)
	var h, mi, sec, milli int64
	i := 0
	num := func(maxLen int) (int64, bool) {
		j := i
		for j < len(s) && j-i < maxLen && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j == i {
			return 0, false
		}
		n, _ := strconv.ParseInt(s[i:j], 10, 64)
		i = j
		return n, true
	}
	for f := 0; f < len(format); {
		tok := ""
		for _, t := range []string{"YYYY", "SSS", "YY", "MM", "DD", "Do", "HH", "mm", "ss", "M", "D", "H"} {
			if len(format)-f >= len(t) && format[f:f+len(t)] == t {
				tok = t
				break
			}
		}
		if tok == "" {
			if i >= len(s) || s[i] != format[f] {
				return 0, false
			}
			i++
			f++
			continue
		}
		f += len(tok)
		var v int64
		var ok bool
		switch tok {
		case "YYYY":
			v, ok = num(4)
			y = v
		case "YY":
			v, ok = num(2)
			y = 2000 + v
		case "MM", "M":
			v, ok = num(2)
			mo = v
		case "DD", "D", "Do":
			v, ok = num(2)
			d = v
			if tok == "Do" && i+2 <= len(s) {
				i += 2 // "st", "nd", "rd", "th"
			}
		case "HH", "H":
			v, ok = num(2)
			h = v
		case "mm":
			v, ok = num(2)
			mi = v
		case "ss":
			v, ok = num(2)
			sec = v
		case "SSS":
			v, ok = num(3)
			milli = v
		}
		if !ok {
			return 0, false
		}
	}
	if i != len(s) || mo < 1 || mo > 12 || d < 1 || d > 31 {
		return 0, false
	}
	return dateMS(y, mo, d, h, mi, sec, milli), true
}

var monthNames = []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

func pad2(n int64) string {
	if n < 10 {
		return "0" + strconv.FormatInt(n, 10)
	}
	return strconv.FormatInt(n, 10)
}

// formatAxisDate writes an axis label: "Jan 02", "15:04" or "Jan 2006".
func formatAxisDate(ms int64, style string) string {
	days := floorDiv(ms, msDay)
	y, m, d := civilFromDays(days)
	rest := ms - days*msDay
	switch style {
	case "time":
		return pad2(rest/msHour) + ":" + pad2(rest%msHour/msMinute)
	case "month":
		return monthNames[m-1] + " " + strconv.FormatInt(y, 10)
	}
	return monthNames[m-1] + " " + pad2(d)
}
