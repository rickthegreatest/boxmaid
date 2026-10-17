package boxmaid

import "testing"

func TestDates(t *testing.T) {
	for _, c := range []struct{ y, m, d int64 }{{1970, 1, 1}, {2000, 2, 29}, {2024, 12, 31}, {1969, 7, 20}, {2100, 3, 1}} {
		y, m, d := civilFromDays(daysFromCivil(c.y, c.m, c.d))
		if y != c.y || m != c.m || d != c.d {
			t.Errorf("%v round trip = %d-%d-%d", c, y, m, d)
		}
	}
	if weekday(dateMS(2024, 1, 6, 0, 0, 0, 0)) != 6 || weekday(dateMS(1970, 1, 1, 0, 0, 0, 0)) != 4 {
		t.Error("weekday")
	}
	if got := addMonths(dateMS(2024, 1, 31, 0, 0, 0, 0), 1); got != dateMS(2024, 3, 2, 0, 0, 0, 0) {
		t.Errorf("Jan 31 + 1 month = %s", formatAxisDate(got, "day"))
	}
	if v, ok := parseMomentDate("17/10/2026 14:05", "DD/MM/YYYY HH:mm"); !ok || v != dateMS(2026, 10, 17, 14, 5, 0, 0) {
		t.Errorf("parse: %d %v", v, ok)
	}
	if _, ok := parseMomentDate("2026-13-01", "YYYY-MM-DD"); ok {
		t.Error("month 13 parsed")
	}
	if formatAxisDate(dateMS(2026, 10, 7, 9, 3, 0, 0), "day") != "Oct 07" || formatAxisDate(dateMS(2026, 10, 7, 9, 3, 0, 0), "time") != "09:03" {
		t.Error("format")
	}
}
