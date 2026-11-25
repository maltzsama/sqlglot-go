package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestMySQLIntroducers ports test_introducers from tests/dialects/test_mysql.py (v30.17.0):
// a charset introducer parses into an Introducer node and re-renders canonically.
func TestMySQLIntroducers(t *testing.T) {
	cases := []struct {
		sql  string
		read string
		want string
	}{
		{"_utf8mb4 'hola'", "_utf8mb4'hola'", "_utf8mb4 'hola'"},
		{"N'some text'", "n'some text'", "N'some text'"},
		{"_latin1 x'4D7953514C'", "_latin1 X'4D7953514C'", "_latin1 x'4D7953514C'"},
	}
	for _, c := range cases {
		t.Run(c.sql, func(t *testing.T) {
			expr, err := sqlglot.ParseOne(c.read, "mysql")
			if err != nil {
				t.Fatalf("ParseOne(%q): %v", c.read, err)
			}
			got, err := sqlglot.Generate(expr, "mysql", generator.Options{})
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if got != c.want {
				t.Fatalf("want %q, got %q", c.want, got)
			}
		})
	}
}
