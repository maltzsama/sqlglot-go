package generator_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestTypedDivisionCastOperands pins div_sql's operand typing (generator.py:4451-4470):
// a cast operand counts as its target type through Cast.is_type (functions.py:57-70), so
// the TYPED_DIVISION wrap must not double-wrap an operand that is already a DOUBLE cast,
// and the untyped->typed BIGINT cast must fire for integer casts. The BIGINT branch reads
// the pre-NULLIF divisor local on purpose — upstream builds `l / r` from the same stale
// locals (generator.py:4465), so the guard stays outside the cast.
func TestTypedDivisionCastOperands(t *testing.T) {
	cases := []struct {
		sql, read, write, want string
	}{
		// teradata TYPED_DIVISION wraps both operands once — never around an existing cast.
		{"SELECT CAST(a AS DOUBLE) / CAST(b AS DOUBLE)", "", "teradata",
			"SELECT CAST(a AS DOUBLE PRECISION) / CAST(b AS DOUBLE PRECISION)"},
		// sqlite typed division -> base: both operands integer casts, so the result is cast
		// to BIGINT (with the raw divisor, matching upstream).
		{"SELECT CAST(a AS INT) / CAST(b AS INT)", "sqlite", "",
			"SELECT CAST(CAST(a AS INT) / CAST(b AS INT) AS BIGINT)"},
		// Bare literals are unannotated, so no BIGINT cast — but sqlite's safe division
		// still guards the divisor with NULLIF.
		{"SELECT 5 / 0", "sqlite", "", "SELECT 5 / NULLIF(0, 0)"},
	}
	for _, tc := range cases {
		got, err := sqlglot.Transpile(tc.sql, tc.read, tc.write, generator.Options{})
		if err != nil {
			t.Fatalf("Transpile(%q, %q, %q): %v", tc.sql, tc.read, tc.write, err)
		}
		if got[0] != tc.want {
			t.Errorf("%q (%s -> %s) = %q, want %q", tc.sql, tc.read, tc.write, got[0], tc.want)
		}
	}
}
