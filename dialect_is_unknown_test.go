package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestIsUnknown ports test_is_unknown from tests/dialects/test_dialect.py (v30.17.0):
// in many dialects `<x> IS [NOT] UNKNOWN` is equivalent to `<x> IS [NOT] NULL`, so the
// construct folds to the NULL predicate on parse.
func TestIsUnknown(t *testing.T) {
	cases := []struct {
		sql  string
		read []string
		want string
	}{
		{
			sql:  "x IS UNKNOWN",
			read: []string{"", "bigquery", "mysql", "postgres", "redshift", "duckdb", "spark", "databricks"},
			want: "x IS NULL",
		},
		{
			sql:  "x IS NOT UNKNOWN",
			read: []string{"", "bigquery", "mysql", "redshift", "duckdb", "spark", "databricks"},
			want: "NOT x IS NULL",
		},
		{
			// postgres keeps the negated form (NORMALIZE_NOT_NULL=False).
			sql:  "x IS NOT UNKNOWN",
			read: []string{"postgres"},
			want: "x IS NOT NULL",
		},
	}
	for _, c := range cases {
		for _, d := range c.read {
			t.Run(d+"/"+c.sql, func(t *testing.T) {
				expr, err := sqlglot.ParseOne(c.sql, d)
				if err != nil {
					t.Fatalf("ParseOne(%q, %q): %v", c.sql, d, err)
				}
				got, err := sqlglot.Generate(expr, "", generator.Options{})
				if err != nil {
					t.Fatalf("Generate: %v", err)
				}
				if got != c.want {
					t.Fatalf("read %q: want %q, got %q", d, c.want, got)
				}
			})
		}
	}
}
