package sqlglot_test

import (
	"sync"
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestConcurrentParseGenerate exercises the shared dialect templates from many goroutines.
// Run with -race; it guards the cache contract (dialects are immutable after construction,
// and Generate works on a copy of the input AST).
func TestConcurrentParseGenerate(t *testing.T) {
	cases := []struct {
		sql     string
		dialect string
	}{
		{"SELECT a, b FROM t WHERE a > 1", ""},
		{"SELECT a FROM t FINAL", "clickhouse"},
		{"SELECT a FROM t SETTINGS x = 1", "clickhouse"},
		{"CREATE TABLE t (c1 INT) UNIQUE KEY (c1) DISTRIBUTED BY HASH (c1)", "starrocks"},
		{"CREATE MATERIALIZED VIEW mv REFRESH ASYNC AS SELECT 1", "doris"},
		{"SELECT NVL(a, 1) FROM dual", "oracle"},
		{"SELECT TOP 10 * FROM t", "tsql"},
		{"SELECT x::int FROM t", "postgres"},
	}
	// Pre-parse once so the same AST is generated concurrently (Generate must copy it).
	asts := make([]exp.Expression, len(cases))
	for i, c := range cases {
		expr, err := sqlglot.ParseOne(c.sql, c.dialect)
		if err != nil {
			t.Fatalf("ParseOne(%q): %v", c.sql, err)
		}
		asts[i] = expr
	}

	var wg sync.WaitGroup
	errs := make(chan error, len(cases)*8)
	for round := 0; round < 8; round++ {
		for i, c := range cases {
			wg.Add(1)
			go func(i int, c struct {
				sql     string
				dialect string
			}) {
				defer wg.Done()
				expr, err := sqlglot.ParseOne(c.sql, c.dialect)
				if err != nil {
					errs <- err
					return
				}
				if _, err := sqlglot.Generate(expr, c.dialect, generator.Options{}); err != nil {
					errs <- err
					return
				}
				// The same AST instance is generated concurrently: Generate must deep-copy.
				if _, err := sqlglot.Generate(asts[i], c.dialect, generator.Options{}); err != nil {
					errs <- err
				}
			}(i, c)
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent parse/generate: %v", err)
	}
}
