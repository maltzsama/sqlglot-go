package tokens

import "testing"

// TestRuneViewASCIIAndUnicode pins the two representations: ASCII input is indexed out of the
// original string with no []rune copy, non-ASCII input falls back to []rune, and both expose
// the same rune-indexed len/at/slice behavior.
func TestRuneViewASCIIAndUnicode(t *testing.T) {
	cases := []struct {
		sql   string
		runes []rune
	}{
		{"SELECT 1", []rune("SELECT 1")},
		{"SELECT '한;글'", []rune("SELECT '한;글'")},
		{"", nil},
	}
	for _, tc := range cases {
		view := newRuneView(tc.sql)
		if got := view.len(); got != len(tc.runes) {
			t.Fatalf("newRuneView(%q).len() = %d, want %d", tc.sql, got, len(tc.runes))
		}
		for i, want := range tc.runes {
			if got := view.at(i); got != want {
				t.Fatalf("newRuneView(%q).at(%d) = %q, want %q", tc.sql, i, got, want)
			}
		}
		if got := view.slice(0, view.len()); got != tc.sql {
			t.Fatalf("newRuneView(%q).slice(0, len) = %q, want the input", tc.sql, got)
		}
	}
	if view := newRuneView("SELECT 1"); view.runes != nil {
		t.Fatal("ASCII input should stay string-backed")
	}
	if view := newRuneView("SELECT '한'"); view.runes == nil {
		t.Fatal("non-ASCII input should fall back to []rune")
	}
}
