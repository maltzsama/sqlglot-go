package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestGenerateDoesNotMutateAST guards the public contract that Generate is a pure rendering
// step: the input tree must render identically twice, and its serialized form must be
// unchanged. The generator used to rely on a defensive deep copy for this; these cases cover
// the dialect overrides that historically rewrote nodes in place.
func TestGenerateDoesNotMutateAST(t *testing.T) {
	cases := []struct {
		sql     string
		dialect string
	}{
		{"SELECT a FROM t LIMIT 10", ""},
		{"SELECT a FROM t SAMPLE (10)", "clickhouse"},
		{"UPDATE t SET a = 1 FROM s WHERE t.id = s.id", "tsql"},
		{"CREATE TABLE #tmp (a INT)", "tsql"},
		{"SELECT DATEADD(millisecond, 5, d) FROM t", "tsql"},
		{"SELECT TOP 10 a FROM t ORDER BY a", "tsql"},
		{"ALTER TABLE t SET TBLPROPERTIES ('a' = 'b')", "athena"},
		{"CREATE TABLE t (a INT) UNIQUE KEY (a) DISTRIBUTED BY HASH (a)", "starrocks"},
		{"SELECT DISTINCT ON (a) a, b FROM t", "starrocks"},
		{"SELECT * FROM t PIVOT (SUM(x) FOR y IN ('a'))", "oracle"},
		{"SELECT CAST(x AS INT) FROM t", "hive"},
		{"SELECT transform(a, x -> x + 1) FROM t", "clickhouse"},
		{"SELECT array_agg(a ORDER BY b) FROM t", "presto"},
		{"SELECT a FROM t WHERE a = 1", "postgres"},
	}
	for _, tc := range cases {
		expr, err := sqlglot.ParseOne(tc.sql, tc.dialect)
		if err != nil {
			t.Fatalf("ParseOne(%q, %q): %v", tc.sql, tc.dialect, err)
		}
		before := expr.ToS()
		first, err := sqlglot.Generate(expr, tc.dialect, generator.Options{})
		if err != nil {
			t.Fatalf("Generate(%q, %q): %v", tc.sql, tc.dialect, err)
		}
		if after := expr.ToS(); after != before {
			t.Errorf("Generate(%q, %q) mutated the AST\nbefore: %s\nafter:  %s", tc.sql, tc.dialect, before, after)
		}
		second, err := sqlglot.Generate(expr, tc.dialect, generator.Options{})
		if err != nil {
			t.Fatalf("second Generate(%q, %q): %v", tc.sql, tc.dialect, err)
		}
		if second != first {
			t.Errorf("Generate(%q, %q) is not repeatable\nfirst:  %s\nsecond: %s", tc.sql, tc.dialect, first, second)
		}
	}
}
