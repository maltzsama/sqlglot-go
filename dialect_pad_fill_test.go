package sqlglot_test

import "testing"

// TestPadFillPattern ports the PAD_FILL_PATTERN_IS_REQUIRED behavior (generator.py:5572-5579):
// a two-argument LPAD/RPAD renders an explicit space fill for dialects that require it
// (Hive, MySQL, DuckDB, Presto families) and stays two-argument elsewhere (Spark, Postgres).
func TestPadFillPattern(t *testing.T) {
	withFill := map[string]string{
		"hive":      "SELECT LPAD('bar', 5, ' ')",
		"mysql":     "SELECT LPAD('bar', 5, ' ')",
		"doris":     "SELECT LPAD('bar', 5, ' ')",
		"starrocks": "SELECT LPAD('bar', 5, ' ')",
		"duckdb":    "SELECT LPAD('bar', 5, ' ')",
		"presto":    "SELECT LPAD('bar', 5, ' ')",
		"trino":     "SELECT LPAD('bar', 5, ' ')",
		"spark2":    "SELECT LPAD('bar', 5, ' ')",
	}
	for write, want := range withFill {
		t.Run(write, func(t *testing.T) { transpileCase(t, "", write, "SELECT LPAD('bar', 5)", want) })
	}
	noFill := map[string]string{
		"spark":    "SELECT LPAD('bar', 5)",
		"postgres": "SELECT LPAD('bar', 5)",
		"bigquery": "SELECT LPAD('bar', 5)",
	}
	for write, want := range noFill {
		t.Run(write, func(t *testing.T) { transpileCase(t, "", write, "SELECT LPAD('bar', 5)", want) })
	}
}
