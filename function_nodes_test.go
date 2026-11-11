package sqlglot_test

import "testing"

// TestLogicalAggregateRenames covers the LogicalAnd generator renames ported from
// generators/postgres.py and generators/mysql.py.
func TestLogicalAggregateRenames(t *testing.T) {
	assertRoundTrip(t, "postgres", "SELECT BOOL_AND(x) FROM t", "SELECT BOOL_AND(x) FROM t")
	assertRoundTrip(t, "mysql", "SELECT BOOL_AND(x) FROM t", "SELECT MIN(x) FROM t")
}

// TestApproxQuantilesNode ensures APPROX_QUANTILES parses to the structured node rather than
// Anonymous, so annotate_types can type it.
func TestApproxQuantilesNode(t *testing.T) {
	assertRoundTrip(t, "postgres", "SELECT APPROX_QUANTILES(x, 5) FROM t", "SELECT APPROX_QUANTILES(x, 5) FROM t")
}

// TestDuckDBArrayAggIgnoreNulls pins DuckDB's ARRAY_AGG IGNORE NULLS rewrite: the modifier
// becomes a FILTER over the non-NULL input instead of a trailing IGNORE NULLS.
func TestDuckDBArrayAggIgnoreNulls(t *testing.T) {
	assertRoundTrip(t, "duckdb", "SELECT ARRAY_AGG(x IGNORE NULLS) FROM t", "SELECT ARRAY_AGG(x) FILTER(WHERE x IS NOT NULL) FROM t")
}
