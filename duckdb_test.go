package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestDuckDBCore ports the same-dialect round-trip core of test_duckdb
// (tests/dialects/test_duckdb.py:12).
func TestDuckDBCore(t *testing.T) {
	for _, sql := range []string{
		"SELECT 1",
		"SELECT * FROM foo",
		"SELECT * FROM foo WHERE x > 1",
		"SELECT CAST(x AS BIGINT)",
		"SELECT CAST(x AS DECIMAL(15, 4))",
		"SELECT CAST(x AS BLOB)",
		"SELECT CAST(x AS TEXT)",
		"SELECT [1, 2, 3]",
		"SELECT {'a': 1, 'b': 'x'}",
		"SELECT [1, 2] FROM t",
		"SELECT a & b, a | b FROM t",
		"SELECT a / b FROM t",
		"SELECT a && b FROM t",
		"SELECT x -> '$.a' FROM t",
		"SELECT x ->> '$.a' FROM t",
		"SELECT REGEXP_FULL_MATCH(x, 'pattern') FROM t",
		"SELECT STARTS_WITH(x, 'prefix') FROM t",
		"SELECT POWER(a, b) FROM t",
		"SELECT POWER(a, b) FROM t",
		"SELECT COLUMNS(*) FROM t",
		"SELECT * EXCLUDE (a) FROM t",
		"SELECT * REPLACE (a + 1 AS a) FROM t",
	} {
		assertRoundTrip(t, "duckdb", sql, sql)
	}
}

// TestDuckDBFunctions ports the function-call round-trips.
func TestDuckDBFunctions(t *testing.T) {
	for _, sql := range []string{
		"SELECT DATE_TRUNC('DAY', x)",
		"SELECT DATE_TRUNC('MONTH', CAST(x AS DATE))",
		"SELECT DATE_DIFF('DAY', a, b)",
		"SELECT DATE_DIFF('MONTH', a, b)",
		"SELECT EPOCH(x)",
		"SELECT EPOCH_MS(x)",
		"SELECT MAKE_DATE(2020, 1, 1)",
		"SELECT MAKE_TIMESTAMP(2020, 1, 1, 0, 0, 0)",
		"SELECT STRFTIME(x, '%Y-%m-%d')",
		"SELECT STRPTIME('2020-01-01', '%Y-%m-%d')",
		"SELECT LIST_SORT(x)",
		"SELECT ARRAY_REVERSE_SORT(x)",
		"SELECT LIST_SORT(x, 'DESC')",
		"SELECT LIST_FILTER(x, _u -> _u > 1)",
		"SELECT LIST_TRANSFORM(x, _u -> _u + 1)",
		"SELECT LIST_DISTINCT(x)",
		"SELECT LIST_CONCAT(a, b)",
		"SELECT LIST_APPEND(a, 1)",
		"SELECT LIST_PREPEND(a, 1)",
		"SELECT ARRAY_CONTAINS(a, 1)",
		"SELECT STR_SPLIT(x, ',')",
		"SELECT STR_SPLIT_REGEX(x, ',')",
		"SELECT REGEXP_MATCHES(x, 'a')",
		"SELECT REGEXP_EXTRACT(x, 'a')",
		"SELECT REGEXP_REPLACE(x, 'a', 'b')",
		"SELECT SHA256(x)",
		"SELECT MD5(x)",
		"SELECT GET_BIT(x, 2)",
		"SELECT CARDINALITY(x)",
		"SELECT JARO_WINKLER_SIMILARITY(a, b)",
		"SELECT QUANTILE_CONT(x, 0.5)",
		"SELECT QUANTILE_DISC(x, 0.5)",
		"SELECT XOR(a, b)",
		"SELECT REGEXP_FULL_MATCH(a, b)",
		"SELECT STARTS_WITH(a, b)",
		"SELECT ABS(x)",
		"SELECT LENGTH('abc')",
	} {
		assertRoundTrip(t, "duckdb", sql, sql)
	}
}

// TestDuckDBTemporalOps verifies the interval binary-op canonicalization.
func TestDuckDBTemporalOps(t *testing.T) {
	cases := map[string]string{
		"SELECT DATE_ADD(CAST('2020-01-01' AS DATE), INTERVAL 1 DAY)": "SELECT CAST('2020-01-01' AS DATE) + INTERVAL '1' DAY",
	}
	for sql, want := range cases {
		assertRoundTrip(t, "duckdb", sql, want)
	}
}

// TestDuckDBTypes ports the DuckDB type conversion rules: DECIMAL defaults and the
// TEXT precision strip.
func TestDuckDBTypes(t *testing.T) {
	cases := map[string]string{
		"CAST(x AS DECIMAL)":          "CAST(x AS DECIMAL(18, 3))",
		"CAST(x AS NUMERIC)":          "CAST(x AS DECIMAL(18, 3))",
		"CAST(x AS NUMERIC(1, 2))":    "CAST(x AS DECIMAL(1, 2))",
		"CAST(x AS VARCHAR(5))":       "CAST(x AS TEXT)",
		"CAST(x AS STRUCT(a BIGINT))": "CAST(x AS STRUCT(a BIGINT))",
		"COL::BIGINT[]":               "CAST(COL AS BIGINT[])",
	}
	for sql, want := range cases {
		assertRoundTrip(t, "duckdb", sql, want)
	}
}

// TestDuckDBPrefixAlias ports the DuckDB prefix alias grammar
// (parsers/duckdb.py:341-364).
func TestDuckDBPrefixAlias(t *testing.T) {
	assertRoundTrip(t, "duckdb", "SELECT foo: 1", "SELECT 1 AS foo")
	assertRoundTrip(t, "duckdb", "SELECT foo: 1, bar: 2", "SELECT 1 AS foo, 2 AS bar")
	assertRoundTrip(t, "duckdb", "SELECT * FROM foo: t", "SELECT * FROM t AS foo")
}

// TestDuckDBLambda ports the LAMBDA keyword form (parsers/duckdb.py:327-338).
func TestDuckDBLambda(t *testing.T) {
	assertRoundTrip(t, "duckdb",
		"SELECT LIST_TRANSFORM([5, NULL, 6], LAMBDA x : COALESCE(x, 0) + 1)",
		"SELECT LIST_TRANSFORM([5, NULL, 6], LAMBDA x : COALESCE(x, 0) + 1)")
	assertRoundTrip(t, "duckdb",
		"SELECT LIST_TRANSFORM([5, NULL, 6], LAMBDA x, y : COALESCE(x, y, 0) + 1)",
		"SELECT LIST_TRANSFORM([5, NULL, 6], LAMBDA x, y : COALESCE(x, y, 0) + 1)")
}

// TestDuckDBAttachDetach ports the ATTACH/DETACH round-trips.
func TestDuckDBAttachDetach(t *testing.T) {
	cases := map[string]string{
		"ATTACH 'file.db' AS db_alias":               "ATTACH 'file.db' AS db_alias",
		"ATTACH 'file.db' (READ_ONLY FALSE)":         "ATTACH 'file.db' (READ_ONLY FALSE)",
		"ATTACH IF NOT EXISTS 'file.db' AS db_alias": "ATTACH IF NOT EXISTS 'file.db' AS db_alias",
		"ATTACH DATABASE 'file.db'":                  "ATTACH 'file.db'",
		"DETACH new_database":                        "DETACH new_database",
		"DETACH DATABASE db":                         "DETACH db",
		"DETACH IF EXISTS file":                      "DETACH DATABASE IF EXISTS file",
		"INSTALL httpfs":                             "INSTALL httpfs",
		"FORCE INSTALL httpfs":                       "FORCE INSTALL httpfs",
	}
	for sql, want := range cases {
		assertRoundTrip(t, "duckdb", sql, want)
	}
}

// TestDuckDBSummarize ports the SUMMARIZE statement.
func TestDuckDBSummarize(t *testing.T) {
	for _, sql := range []string{
		"SUMMARIZE tbl",
		"SUMMARIZE SELECT * FROM tbl",
	} {
		assertRoundTrip(t, "duckdb", sql, sql)
	}
}

// TestDuckDBSimplifiedPivot ports _parse_simplified_pivot (parser.py:5323-5353).
func TestDuckDBSimplifiedPivot(t *testing.T) {
	for _, sql := range []string{
		"PIVOT Cities ON Year USING SUM(Population)",
		"PIVOT Cities ON Year USING SUM(Population) GROUP BY Country",
		"UNPIVOT monthly_sales ON jan, feb INTO NAME month VALUE sales",
	} {
		assertRoundTrip(t, "duckdb", sql, sql)
	}
}

// TestDuckDBTablesample ports tablesample_sql (generators/duckdb.py:3118-3133).
func TestDuckDBTablesample(t *testing.T) {
	cases := map[string]string{
		"SELECT * FROM tbl USING SAMPLE 10%":               "SELECT * FROM tbl USING SAMPLE SYSTEM (10 PERCENT)",
		"SELECT * FROM tbl USING SAMPLE 5":                 "SELECT * FROM tbl USING SAMPLE RESERVOIR (5 ROWS)",
		"SELECT * FROM tbl USING SAMPLE 10% (system, 377)": "SELECT * FROM tbl USING SAMPLE SYSTEM (10 PERCENT) REPEATABLE (377)",
		"SELECT * FROM tbl TABLESAMPLE RESERVOIR(20%)":     "SELECT * FROM tbl TABLESAMPLE RESERVOIR (20 PERCENT)",
	}
	for sql, want := range cases {
		assertRoundTrip(t, "duckdb", sql, want)
	}
}

// TestDuckDBSetVariable ports the SET VARIABLE form (parsers/duckdb.py:272-275).
func TestDuckDBSetVariable(t *testing.T) {
	assertRoundTrip(t, "duckdb", "SET VARIABLE my_var TO 30", "SET VARIABLE my_var = 30")
}

// TestDuckDBShow ports the SHOW TABLES forms.
func TestDuckDBShow(t *testing.T) {
	for _, sql := range []string{
		"SHOW TABLES",
		"SHOW ALL TABLES",
		"SHOW TABLES FROM my_database",
		"SHOW TABLES FROM my_schema",
	} {
		assertRoundTrip(t, "duckdb", sql, sql)
	}
}

// TestDuckDBCopy ports the COPY statement (COPY_HAS_INTO_KEYWORD = False).
func TestDuckDBCopy(t *testing.T) {
	for _, sql := range []string{
		"COPY lineitem (l_orderkey) TO 'orderkey.tbl' WITH (DELIMITER '|')",
		"COPY lineitem FROM 'lineitem.ndjson' WITH (FORMAT JSON)",
		"COPY (SELECT 42 AS a) TO 'query.json' WITH (FORMAT JSON, ARRAY TRUE)",
	} {
		assertRoundTrip(t, "duckdb", sql, sql)
	}
}

// TestDuckDBFromFirst ports the leading FROM syntax.
func TestDuckDBFromFirst(t *testing.T) {
	assertRoundTrip(t, "duckdb", "FROM tbl", "SELECT * FROM tbl")
	assertRoundTrip(t, "duckdb", "FROM tbl SELECT *", "SELECT * FROM tbl")
}

// TestDuckDBNodeShapes verifies a few structured nodes the dialect builds.
func TestDuckDBNodeShapes(t *testing.T) {
	expr, err := sqlglot.ParseOne("DATE_TRUNC('DAY', x)", "duckdb")
	if err != nil {
		t.Fatal(err)
	}
	if expr.Kind() != exp.KindTimestampTrunc {
		t.Fatalf("expected TimestampTrunc, got %s", expr.ToS())
	}

	expr, err = sqlglot.ParseOne("LIST_SORT(x, 'DESC')", "duckdb")
	if err != nil {
		t.Fatal(err)
	}
	if expr.Kind() != exp.KindSortArray {
		t.Fatalf("expected SortArray, got %s", expr.ToS())
	}

	expr, err = sqlglot.ParseOne("PIVOT Cities ON Year USING SUM(Population)", "duckdb")
	if err != nil {
		t.Fatal(err)
	}
	if expr.Kind() != exp.KindPivot {
		t.Fatalf("expected Pivot, got %s", expr.ToS())
	}
}

// TestDuckDBCanonicalizations pins a few generator normalizations.
func TestDuckDBCanonicalizations(t *testing.T) {
	cases := map[string]string{
		"SELECT a ** b":                  "SELECT POWER(a, b)",
		"SELECT x ~ y":                   "SELECT REGEXP_FULL_MATCH(x, y)",
		"SELECT x !~ y":                  "SELECT NOT REGEXP_FULL_MATCH(x, y)",
		"SELECT a ^@ b":                  "SELECT STARTS_WITH(a, b)",
		"SELECT e'Hello world'":          "SELECT e'Hello world'",
		"SELECT STRING_AGG(x, ', ')":     "SELECT LISTAGG(x, ', ')",
		"SELECT STRING_TO_ARRAY(x, 'a')": "SELECT STR_SPLIT(x, 'a')",
		"SELECT len('asdf')":             "SELECT LENGTH('asdf')",
		"SELECT BOOL_OR(x)":              "SELECT BOOL_OR(CAST(x AS BOOLEAN))",
	}
	for sql, want := range cases {
		assertRoundTrip(t, "duckdb", sql, want)
	}
}

// ensure the generator import is referenced even if a helper changes.
var _ = generator.Options{}
