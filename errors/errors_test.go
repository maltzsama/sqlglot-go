package errors_test

import (
	stderrors "errors"
	"testing"

	sqlerrors "github.com/maltzsama/sqlglot-go/errors"
)

func TestConcatMessagesBounds(t *testing.T) {
	errs := []error{stderrors.New("first"), stderrors.New("second"), stderrors.New("third")}
	cases := []struct {
		name string
		errs []error
		max  int
		want string
	}{
		{"below length", errs, 2, "first\n\nsecond\n\n... and 1 more"},
		{"above length", errs, 10, "first\n\nsecond\n\nthird"},
		{"zero", errs, 0, "... and 3 more"},
		{"negative clamps to zero", errs, -1, "... and 3 more"},
		{"empty", nil, 3, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sqlerrors.ConcatMessages(tc.errs, tc.max); got != tc.want {
				t.Fatalf("ConcatMessages = %q, want %q", got, tc.want)
			}
		})
	}
}
