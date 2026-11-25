package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestPostgresBitwiseXor ports the `x # y` case from test_postgres.py (v30.17.0):
// postgres reads `#` as BitwiseXor, rendering `^` in the base dialect and `#` back in
// postgres.
func TestPostgresBitwiseXor(t *testing.T) {
	expr, err := sqlglot.ParseOne("x # y", "postgres")
	if err != nil {
		t.Fatalf("ParseOne: %v", err)
	}
	for _, c := range []struct{ write, want string }{
		{"", "x ^ y"},
		{"postgres", "x # y"},
	} {
		got, err := sqlglot.Generate(expr, c.write, generator.Options{})
		if err != nil {
			t.Fatalf("Generate(%q): %v", c.write, err)
		}
		if got != c.want {
			t.Fatalf("write=%q: want %q, got %q", c.write, c.want, got)
		}
	}
}
