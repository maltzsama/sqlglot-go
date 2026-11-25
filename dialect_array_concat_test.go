package sqlglot_test

import "testing"

// TestArrayConcat ports the ARRAY_CONCAT transpilation from dialect.py's array_concat_sql
// (v30.17.0): variable-length dialects concatenate directly, while PostgreSQL/Snowflake/
// Redshift nest binary calls, and DuckDB/ClickHouse use their own function names.
func TestArrayConcat(t *testing.T) {
	const sql = "ARRAY_CONCAT([1, 2], [3, 4], [5, 6])"
	cases := map[string]string{
		"bigquery":   "ARRAY_CONCAT([1, 2], [3, 4], [5, 6])",
		"presto":     "CONCAT(ARRAY[1, 2], ARRAY[3, 4], ARRAY[5, 6])",
		"trino":      "CONCAT(ARRAY[1, 2], ARRAY[3, 4], ARRAY[5, 6])",
		"hive":       "CONCAT(ARRAY(1, 2), ARRAY(3, 4), ARRAY(5, 6))",
		"spark":      "CONCAT(ARRAY(1, 2), ARRAY(3, 4), ARRAY(5, 6))",
		"spark2":     "CONCAT(ARRAY(1, 2), ARRAY(3, 4), ARRAY(5, 6))",
		"databricks": "CONCAT(ARRAY(1, 2), ARRAY(3, 4), ARRAY(5, 6))",
		"postgres":   "ARRAY_CAT(ARRAY[1, 2], ARRAY_CAT(ARRAY[3, 4], ARRAY[5, 6]))",
		"redshift":   "ARRAY_CONCAT(ARRAY(1, 2), ARRAY_CONCAT(ARRAY(3, 4), ARRAY(5, 6)))",
		"snowflake":  "ARRAY_CAT([1, 2], ARRAY_CAT([3, 4], [5, 6]))",
		"duckdb":     "LIST_CONCAT([1, 2], [3, 4], [5, 6])",
		"clickhouse": "arrayConcat([1, 2], [3, 4], [5, 6])",
	}
	for write, want := range cases {
		t.Run(write, func(t *testing.T) { transpileCase(t, "bigquery", write, sql, want) })
	}
}
