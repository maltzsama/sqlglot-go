package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

func transpileCase(t *testing.T, read, write, sql, want string) {
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
		t.Fatalf("read=%q write=%q sql=%q: want %q, got %q", read, write, sql, want, got)
	}
}

// TestNVL2 ports test_nvl2 from tests/dialects/test_dialect.py (v30.17.0): dialects with
// NVL2_SUPPORTED=False render NVL2 as a CASE expression (and the 2-arg form has no ELSE).
func TestNVL2(t *testing.T) {
	nvl2 := map[string][2]string{
		"":           {"SELECT NVL2(a, b, c)", "SELECT NVL2(a, b)"},
		"bigquery":   {"SELECT CASE WHEN NOT a IS NULL THEN b ELSE c END", "SELECT CASE WHEN NOT a IS NULL THEN b END"},
		"clickhouse": {"SELECT CASE WHEN NOT (a IS NULL) THEN b ELSE c END", "SELECT CASE WHEN NOT (a IS NULL) THEN b END"},
		"databricks": {"SELECT NVL2(a, b, c)", "SELECT NVL2(a, b)"},
		"doris":      {"SELECT CASE WHEN NOT a IS NULL THEN b ELSE c END", "SELECT CASE WHEN NOT a IS NULL THEN b END"},
		"dremio":     {"SELECT CASE WHEN NOT a IS NULL THEN b ELSE c END", "SELECT CASE WHEN NOT a IS NULL THEN b END"},
		"duckdb":     {"SELECT CASE WHEN NOT a IS NULL THEN b ELSE c END", "SELECT CASE WHEN NOT a IS NULL THEN b END"},
		"hive":       {"SELECT CASE WHEN NOT a IS NULL THEN b ELSE c END", "SELECT CASE WHEN NOT a IS NULL THEN b END"},
		"mysql":      {"SELECT CASE WHEN NOT a IS NULL THEN b ELSE c END", "SELECT CASE WHEN NOT a IS NULL THEN b END"},
		"oracle":     {"SELECT NVL2(a, b, c)", "SELECT NVL2(a, b)"},
		"postgres":   {"SELECT CASE WHEN NOT a IS NULL THEN b ELSE c END", "SELECT CASE WHEN NOT a IS NULL THEN b END"},
		"presto":     {"SELECT CASE WHEN NOT a IS NULL THEN b ELSE c END", "SELECT CASE WHEN NOT a IS NULL THEN b END"},
		"redshift":   {"SELECT NVL2(a, b, c)", "SELECT NVL2(a, b)"},
		"snowflake":  {"SELECT NVL2(a, b, c)", "SELECT NVL2(a, b)"},
		"spark":      {"SELECT NVL2(a, b, c)", "SELECT NVL2(a, b)"},
		"spark2":     {"SELECT NVL2(a, b, c)", "SELECT NVL2(a, b)"},
		"sqlite":     {"SELECT CASE WHEN NOT a IS NULL THEN b ELSE c END", "SELECT CASE WHEN NOT a IS NULL THEN b END"},
		"starrocks":  {"SELECT CASE WHEN NOT a IS NULL THEN b ELSE c END", "SELECT CASE WHEN NOT a IS NULL THEN b END"},
		"teradata":   {"SELECT NVL2(a, b, c)", "SELECT NVL2(a, b)"},
		"trino":      {"SELECT CASE WHEN NOT a IS NULL THEN b ELSE c END", "SELECT CASE WHEN NOT a IS NULL THEN b END"},
		"tsql":       {"SELECT CASE WHEN NOT a IS NULL THEN b ELSE c END", "SELECT CASE WHEN NOT a IS NULL THEN b END"},
	}
	for write, want := range nvl2 {
		t.Run("3arg/"+write, func(t *testing.T) { transpileCase(t, "", write, "SELECT NVL2(a, b, c)", want[0]) })
		t.Run("2arg/"+write, func(t *testing.T) { transpileCase(t, "", write, "SELECT NVL2(a, b)", want[1]) })
	}
}

// TestJSONKeys ports test_json_keys from tests/dialects/test_dialect.py (v30.17.0). Spark and
// Databricks spell it JSON_OBJECT_KEYS and Snowflake OBJECT_KEYS; the base/MySQL family keep
// JSON_KEYS.
func TestJSONKeys(t *testing.T) {
	cases := []struct{ read, write, sql, want string }{
		{"", "", "JSON_KEYS(foo)", "JSON_KEYS(foo)"},
		{"mysql", "mysql", "JSON_KEYS(foo)", "JSON_KEYS(foo)"},
		{"starrocks", "starrocks", "JSON_KEYS(foo)", "JSON_KEYS(foo)"},
		{"duckdb", "duckdb", "JSON_KEYS(foo)", "JSON_KEYS(foo)"},
		{"doris", "doris", "JSON_KEYS(foo)", "JSON_KEYS(foo)"},
		{"spark", "spark", "JSON_OBJECT_KEYS(foo)", "JSON_OBJECT_KEYS(foo)"},
		{"databricks", "databricks", "JSON_OBJECT_KEYS(foo)", "JSON_OBJECT_KEYS(foo)"},
		{"snowflake", "snowflake", "OBJECT_KEYS(foo)", "OBJECT_KEYS(foo)"},
		{"", "", "JSON_KEYS(foo, '$.a')", "JSON_KEYS(foo, '$.a')"},
		{"mysql", "mysql", "JSON_KEYS(foo, '$.a')", "JSON_KEYS(foo, '$.a')"},
		{"duckdb", "duckdb", "JSON_KEYS(foo, '$.a')", "JSON_KEYS(foo, '$.a')"},
	}
	for _, c := range cases {
		t.Run(c.write+"/"+c.sql, func(t *testing.T) { transpileCase(t, c.read, c.write, c.sql, c.want) })
	}
}

// TestFetchFirstImplicitCount ports test_fetch_first_implicit_count from
// tests/dialects/test_dialect.py (v30.17.0): a FETCH FIRST without a count is one row, so
// LIMIT-based dialects emit LIMIT 1, never a bare LIMIT.
func TestFetchFirstImplicitCount(t *testing.T) {
	one := map[string]string{
		"postgres":  "SELECT * FROM t FETCH FIRST ROWS ONLY",
		"presto":    "SELECT * FROM t FETCH FIRST ROWS ONLY",
		"oracle":    "SELECT * FROM t FETCH FIRST ROWS ONLY",
		"mysql":     "SELECT * FROM t LIMIT 1",
		"sqlite":    "SELECT * FROM t LIMIT 1",
		"duckdb":    "SELECT * FROM t LIMIT 1",
		"bigquery":  "SELECT * FROM t LIMIT 1",
		"spark":     "SELECT * FROM t LIMIT 1",
		"hive":      "SELECT * FROM t LIMIT 1",
		"redshift":  "SELECT * FROM t LIMIT 1",
		"starrocks": "SELECT * FROM t LIMIT 1",
		"doris":     "SELECT * FROM t LIMIT 1",
	}
	for write, want := range one {
		t.Run(write, func(t *testing.T) { transpileCase(t, "", write, "SELECT * FROM t FETCH FIRST ROWS ONLY", want) })
	}
	transpileCase(t, "", "mysql", "SELECT * FROM t FETCH FIRST 10 ROWS ONLY", "SELECT * FROM t LIMIT 10")
	transpileCase(t, "", "oracle", "SELECT * FROM t FETCH FIRST 10 ROWS ONLY", "SELECT * FROM t FETCH FIRST 10 ROWS ONLY")
}
