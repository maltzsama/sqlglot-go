package generator_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/dialects"
	"github.com/maltzsama/sqlglot-go/generator"
)

// A compound argument is parenthesized before IS NULL so Trino does not parse NOT (x IS NULL)
// (DEVIATIONS §1.20).
func TestConcatWsGuardParenthesizesCompoundArgument(t *testing.T) {
	out, err := sqlglot.Transpile("CONCAT_WS(',', NOT TRUE, 'x')", dialects.DialectType(""), dialects.DialectType("trino"), generator.Options{})
	if err != nil || len(out) != 1 {
		t.Fatalf("transpile: %v %v", out, err)
	}
	want := "CASE WHEN ',' IS NULL OR (NOT TRUE) IS NULL OR 'x' IS NULL THEN NULL ELSE CONCAT_WS(',', CAST(NOT TRUE AS VARCHAR), CAST('x' AS VARCHAR)) END"
	if out[0] != want {
		t.Fatalf("got %q\nwant %q", out[0], want)
	}
}
