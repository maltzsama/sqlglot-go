package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestSnowflakeMinus ports test_minus from tests/dialects/test_snowflake.py (v30.17.0):
// Snowflake and Oracle read the MINUS set operator as EXCEPT.
func TestSnowflakeMinus(t *testing.T) {
	const want = "SELECT 1 EXCEPT SELECT 1"
	for _, read := range []string{"oracle", "snowflake"} {
		expr, err := sqlglot.ParseOne("SELECT 1 MINUS SELECT 1", read)
		if err != nil {
			t.Fatalf("ParseOne(%q): %v", read, err)
		}
		got, err := sqlglot.Generate(expr, "", generator.Options{})
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		if got != want {
			t.Fatalf("read=%q: want %q, got %q", read, want, got)
		}
	}
}
