package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestLimit ports test_limit from tests/dialects/test_dialect.py (v30.17.0).
func TestLimit(t *testing.T) {
	transpile := func(t *testing.T, read, write, sql, want string) {
		t.Helper()
		expr, err := sqlglot.ParseOne(sql, read)
		if err != nil {
			t.Fatalf("ParseOne(%q, %q): %v", sql, read, err)
		}
		got, err := sqlglot.Generate(expr, write, generator.Options{})
		if err != nil {
			t.Fatalf("Generate(%q): %v", write, err)
		}
		if got != want {
			t.Fatalf("read=%q write=%q: want %q, got %q", read, write, want, got)
		}
	}

	// Identity: the base dialect keeps `LIMIT all`, where `all` is the projection alias.
	transpile(t, "", "", "WITH t AS (SELECT 1 AS all) SELECT 1 FROM t LIMIT all", "WITH t AS (SELECT 1 AS all) SELECT 1 FROM t LIMIT all")

	// LIMIT ALL is a no-op limit clause in the dialects that support it: it is dropped and
	// `all` is never treated as a table alias.
	const want = "WITH t AS (SELECT 1 AS all) SELECT 1 FROM t"
	for _, read := range []string{"databricks", "duckdb", "presto", "postgres", "spark"} {
		transpile(t, read, "", "WITH t AS (SELECT 1 AS all) SELECT 1 FROM t LIMIT all", want)
	}

	// MySQL's `LIMIT offset, count` becomes SQLite's `LIMIT count OFFSET offset`.
	transpile(t, "", "sqlite", "SELECT * FROM data LIMIT 10, 20", "SELECT * FROM data LIMIT 20 OFFSET 10")
}
