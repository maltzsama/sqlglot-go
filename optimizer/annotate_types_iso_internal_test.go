package optimizer

import "testing"

// TestISODateParsing pins isISODate / isISODatetime to the behavior of upstream's is_iso_date
// and is_iso_datetime (helper.py:432-445), which accept what date.fromisoformat and
// datetime.fromisoformat accept.
func TestISODateParsing(t *testing.T) {
	for _, tc := range []struct {
		text           string
		date, datetime bool
	}{
		{"2020-01-01", true, true},
		{"20200101", true, true},
		{"2020-02-30", false, false},
		{"2020-13-01", false, false},
		{"2021-W53", false, false},
		{"2020-W53", true, true},
		{"2020-W01-1", true, true},
		{"2020W011", true, true},
		{"2020-W01-8", false, false},
		{"2020-1-01", false, false},
		{"2020-01-01T10", false, true},
		{"2020-01-01 10:30", false, true},
		{"2020-01-01T10:30:45.123456", false, true},
		{"2020-01-01T10:30:45.1234567", false, true},
		{"2020-01-01T25:00", false, false},
		{"2020-01-01x10:30", false, true},
		{"2020-01-01T10:30Z", false, true},
		{"2020-01-01T10:30+05:30", false, true},
		{"2020-01-01T103045", false, true},
		{"2020-01-01T10:30:45+0530", false, true},
		{"2020-01-01T10:61", false, false},
		{"foo", false, false},
		{"2020-01", false, false},
		{"", false, false},
		{"2020-01-01T", false, false},
		{"2020-01-01  10:30", false, false},
		{"2020-02-29", true, true},
		{"2019-02-29", false, false},
		{"0000-01-01", false, false},
	} {
		if got := isISODate(tc.text); got != tc.date {
			t.Errorf("isISODate(%q) = %v, want %v", tc.text, got, tc.date)
		}
		if got := isISODatetime(tc.text); got != tc.datetime {
			t.Errorf("isISODatetime(%q) = %v, want %v", tc.text, got, tc.datetime)
		}
	}
}
