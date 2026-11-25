package sqlglot_test

import "testing"

// TestEliminateDistinctOn ports the DISTINCT ON case from test_redshift.py (v30.17.0):
// writing a DISTINCT ON query to a dialect whose generator runs eliminate_distinct_on
// rewrites it into a ROW_NUMBER() window filter. The NULLS FIRST/LAS emulation that MySQL,
// StarRocks, and T-SQL still need is tracked separately.
func TestEliminateDistinctOn(t *testing.T) {
	const nullsFirst = "SELECT a, b FROM (SELECT a AS a, b AS b, ROW_NUMBER() OVER (PARTITION BY a ORDER BY c DESC NULLS FIRST) AS _row_number FROM x) AS _t WHERE _row_number = 1"
	const noNulls = "SELECT a, b FROM (SELECT a AS a, b AS b, ROW_NUMBER() OVER (PARTITION BY a ORDER BY c DESC) AS _row_number FROM x) AS _t WHERE _row_number = 1"
	const oracle = "SELECT a, b FROM (SELECT a AS a, b AS b, ROW_NUMBER() OVER (PARTITION BY a ORDER BY c DESC) AS _row_number FROM x) _t WHERE _row_number = 1"

	cases := map[string]string{
		"bigquery":   nullsFirst,
		"databricks": nullsFirst,
		"hive":       nullsFirst,
		"presto":     nullsFirst,
		"spark":      nullsFirst,
		"sqlite":     nullsFirst,
		"trino":      nullsFirst,
		"oracle":     oracle,
		"redshift":   noNulls,
		"snowflake":  noNulls,
	}
	for write, want := range cases {
		t.Run(write, func(t *testing.T) {
			transpileCase(t, "redshift", write, "SELECT DISTINCT ON (a) a, b FROM x ORDER BY c DESC", want)
		})
	}
}
