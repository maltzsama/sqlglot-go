package sqlglot_test

import (
	"strings"
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	sqlerrors "github.com/maltzsama/sqlglot-go/errors"
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
		"SELECT a ** b": "SELECT POWER(a, b)",
		// parsers/duckdb.py:106-110 EXPONENT lists both CARET and DSTAR (Teradata takes
		// only DSTAR), so `^` is exponentiation here, not xor.
		"SELECT 2 ^ 3":                   "SELECT POWER(2, 3)",
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

// TestDuckDBReviewRegressions pins the review-driven fixes: JSON extraction path arguments,
// typed JSON arrays, the null-safe array-overlap branch, MAKE_INTERVAL, DATE_DIFF units and
// ordered ARRAY_AGG within group.
func TestDuckDBReviewRegressions(t *testing.T) {
	assertRoundTrip(t, "duckdb", "SELECT DATE_DIFF('day', a, b)", "SELECT DATE_DIFF('DAY', a, b)")
	assertRoundTrip(t, "duckdb", "SELECT DATE_DIFF(unit_col, a, b)", "SELECT DATE_DIFF(unit_col, a, b)")
	assertRoundTrip(t, "duckdb",
		"SELECT ARRAY_AGG(x) WITHIN GROUP (ORDER BY y)",
		"SELECT ARRAY_AGG(x ORDER BY y)")

	// The extra JSON path arguments are kept on the node's expressions arg.
	expr, err := sqlglot.ParseOne("SELECT JSON_EXTRACT_PATH(x, 'a', 'b')", "duckdb")
	if err != nil {
		t.Fatal(err)
	}
	extract := expr.Expressions()[0]
	if extract == nil || len(extract.Expressions()) != 1 {
		t.Fatalf("expected one extra JSON path argument, got %s", expr.ToS())
	}

	// A prefix alias without an expression is a parse error, not a panic.
	if _, err := sqlglot.ParseOne("SELECT foo:", "duckdb"); err == nil {
		t.Fatal("expected a parse error for a prefix alias without an expression")
	}

	// MAKE_INTERVAL renders a real INTERVAL literal.
	intervalExpr, err := sqlglot.ParseOne("MAKE_INTERVAL(year => 1, month => 2)", "bigquery")
	if err != nil {
		t.Fatal(err)
	}
	got, err := sqlglot.Generate(intervalExpr, "duckdb", generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if want := "INTERVAL '1 year 2 month'"; got != want {
		t.Fatalf("MAKE_INTERVAL = %q, want %q", got, want)
	}
}

// TestDuckDBStructCastRendering pins the review-driven struct-cast fixes: an empty struct
// under a MAP cast renders MAP(), and a positional struct under a STRUCT cast renders ROW().
func TestDuckDBStructCastRendering(t *testing.T) {
	assertRoundTrip(t, "duckdb", "SELECT CAST({} AS MAP(VARCHAR, INT))", "SELECT CAST(MAP() AS MAP(TEXT, INT))")
	assertRoundTrip(t, "duckdb", "SELECT CAST({1, 2} AS STRUCT(a INT, b INT))", "SELECT CAST(ROW(1, 2) AS STRUCT(a INT, b INT))")
}

// TestDuckDBGroupingRegressions pins the review-driven grouping fixes for compound operands.
func TestDuckDBGroupingRegressions(t *testing.T) {
	a := exp.Column(exp.Args{"this": exp.ToIdentifier("a")})
	b := exp.Column(exp.Args{"this": exp.ToIdentifier("b")})
	c := exp.Column(exp.Args{"this": exp.ToIdentifier("c")})

	unix := exp.UnixToTime(exp.Args{
		"this":  exp.Add(exp.Args{"this": a, "expression": b}),
		"scale": exp.LiteralNumber("2"),
	})
	got, err := sqlglot.Generate(unix, "duckdb", generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if want := "TO_TIMESTAMP((a + b) / POWER(10, 2))"; got != want {
		t.Fatalf("UnixToTime = %q, want %q", got, want)
	}

	nanos := exp.New(exp.KindDateSub, exp.Args{
		"this":       a,
		"expression": exp.Add(exp.Args{"this": b, "expression": c}),
		"unit":       exp.Var(exp.Args{"this": "NANOSECOND"}),
	})
	got, err = sqlglot.Generate(nanos, "duckdb", generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if want := "MAKE_TIMESTAMP_NS(EPOCH_NS(CAST(a AS TIMESTAMP_NS)) - (b + c))"; got != want {
		t.Fatalf("nanosecond delta = %q, want %q", got, want)
	}
}

// TestDuckDBMacroOverloads covers DuckDB's multi-signature CREATE MACRO (issue #91). The
// table-first form previously degraded to a verbatim Command; it now parses into a
// MacroOverloads list carrying each signature and its is_table flag, and round-trips
// byte-for-byte. Single-body `AS TABLE` macros keep their existing round-trip.
func TestDuckDBMacroOverloads(t *testing.T) {
	const tableOverloads = "CREATE MACRO tbl (a) AS TABLE (SELECT a AS x), (a, b) AS TABLE (SELECT a AS x, b AS y)"
	assertRoundTrip(t, "duckdb", tableOverloads, tableOverloads)

	expr := mustParse(t, tableOverloads, "duckdb")
	if expr.Kind() != exp.KindCreate {
		t.Fatalf("root kind = %v, want Create", expr.Kind())
	}
	overloads, _ := expr.Arg("expression").(exp.Expression)
	if overloads == nil || overloads.Kind() != exp.KindMacroOverloads {
		t.Fatalf("expression = %v, want MacroOverloads", overloads)
	}
	items := overloads.Expressions()
	if len(items) != 2 {
		t.Fatalf("overload count = %d, want 2", len(items))
	}
	for i, want := range []struct {
		params  int
		isTable bool
	}{{1, true}, {2, true}} {
		item := items[i]
		if item.Kind() != exp.KindMacroOverload {
			t.Fatalf("overload %d kind = %v, want MacroOverload", i, item.Kind())
		}
		if got := item.Arg("is_table"); got != want.isTable {
			t.Fatalf("overload %d is_table = %v, want %v", i, got, want.isTable)
		}
		if got := len(item.Expressions()); got != want.params {
			t.Fatalf("overload %d param count = %d, want %d", i, got, want.params)
		}
		if body, _ := item.Arg("this").(exp.Expression); body == nil || body.Kind() != exp.KindSubquery {
			t.Fatalf("overload %d body = %v, want Subquery", i, body)
		}
	}

	// A mixed signature list keeps is_table per overload rather than on the list.
	const mixed = "CREATE MACRO tbl(a) AS TABLE (SELECT a AS x), (a, b) AS (SELECT a AS x, b AS y)"
	mixedWant := "CREATE MACRO tbl (a) AS TABLE (SELECT a AS x), (a, b) AS (SELECT a AS x, b AS y)"
	assertRoundTrip(t, "duckdb", mixed, mixedWant)
	mixedItems := mustParse(t, mixed, "duckdb").Arg("expression").(exp.Expression).Expressions()
	if len(mixedItems) != 2 {
		t.Fatalf("mixed overload count = %d, want 2", len(mixedItems))
	}
	if mixedItems[0].Arg("is_table") != true || mixedItems[1].Arg("is_table") != false {
		t.Fatalf("mixed is_table flags = %v/%v, want true/false",
			mixedItems[0].Arg("is_table"), mixedItems[1].Arg("is_table"))
	}

	// Overload lists with zero arguments and typed signatures round-trip too. The corpus
	// oracle canonicalizes the signature spacing: the function name and the first
	// `(params)` gain a space, matching upstream.
	for _, tc := range []struct{ sql, want string }{
		{
			"CREATE MACRO tbl() AS TABLE (SELECT 1), (a) AS TABLE (SELECT a)",
			"CREATE MACRO tbl () AS TABLE (SELECT 1), (a) AS TABLE (SELECT a)",
		},
		{
			"CREATE MACRO tbl(a TINYINT) AS TABLE (SELECT a), (a INT) AS TABLE (SELECT a + 1)",
			"CREATE MACRO tbl (a TINYINT) AS TABLE (SELECT a), (a INT) AS TABLE (SELECT a + 1)",
		},
		{
			"CREATE OR REPLACE MACRO tbl(a) AS TABLE (SELECT a AS x), (a, b) AS TABLE (SELECT a AS x, b AS y)",
			"CREATE OR REPLACE MACRO tbl (a) AS TABLE (SELECT a AS x), (a, b) AS TABLE (SELECT a AS x, b AS y)",
		},
	} {
		assertRoundTrip(t, "duckdb", tc.sql, tc.want)
	}

	// Single-body AS TABLE macros keep round-tripping and keep the is_table marker.
	for _, sql := range []string{
		"CREATE MACRO tbl(a) AS TABLE (SELECT a AS x)",
		"CREATE MACRO tbl(a) AS TABLE SELECT a",
		"CREATE MACRO my_table_function(a, b) AS TABLE SELECT x, y FROM (VALUES (a, b)) AS tbl(x, y)",
	} {
		assertRoundTrip(t, "duckdb", sql, sql)
		single := mustParse(t, sql, "duckdb")
		properties := single.Arg("properties")
		if properties == nil {
			t.Fatalf("%q: expected a ReturnsProperty, got none", sql)
		}
		found := false
		for _, property := range properties.(exp.Expression).Expressions() {
			if property.Kind() == exp.KindReturnsProperty && property.Arg("is_table") == true {
				found = true
			}
		}
		if !found {
			t.Fatalf("%q: expected an is_table ReturnsProperty", sql)
		}
	}

	// The supported TABLE marker must not escalate as an unsupported property.
	for _, level := range []sqlerrors.ErrorLevel{sqlerrors.RAISE, sqlerrors.IMMEDIATE} {
		if _, err := sqlglot.Generate(expr, "duckdb", generator.Options{UnsupportedLevel: &level}); err != nil {
			t.Fatalf("Generate at UnsupportedLevel=%v: %v", level, err)
		}
	}
}

// TestDuckDBMapIndexRoundTrip pins the map subscript key space: a CAST to MAP keeps its key
// verbatim instead of taking the array index offset (DEVIATIONS 1.34).
func TestDuckDBMapIndexRoundTrip(t *testing.T) {
	assertRoundTrip(t, "duckdb", "SELECT CAST(MAP() AS MAP(TEXT, INT))[0]", "SELECT CAST(MAP() AS MAP(TEXT, INT))[0]")
	assertRoundTrip(t, "duckdb", "SELECT TRY_CAST(MAP() AS MAP(TEXT, INT))[0]", "SELECT TRY_CAST(MAP() AS MAP(TEXT, INT))[0]")
	// A normalized negative index must still shift back to the user's written index.
	assertRoundTrip(t, "duckdb", "SELECT a[0]", "SELECT a[0]")
	assertRoundTrip(t, "duckdb", "SELECT a[-1]", "SELECT a[-1]")
}

// TestDuckDBTailTranspileFixes pins the tail drain: EXCLUDE, CREATE LIKE, TIMESERIES,
// INLINE/POSEXPLODE, STRTOK, UUID v5 and the sketch templates.
func TestDuckDBTailTranspileFixes(t *testing.T) {
	cases := []struct {
		read, write, sql, want string
	}{
		{"redshift", "duckdb", "SELECT *, 4 AS col4 EXCLUDE (col2, col3) FROM (SELECT 1 AS col1, 2 AS col2, 3 AS col3)", "SELECT * EXCLUDE (col2, col3) FROM (SELECT *, 4 AS col4 FROM (SELECT 1 AS col1, 2 AS col2, 3 AS col3))"},
		{"postgres", "duckdb", "CREATE TABLE a (LIKE b)", "CREATE TABLE a AS SELECT * FROM b LIMIT 0"},
		{"redshift", "duckdb", "SELECT DISTINCT a, b EXCLUDE (b) FROM t LIMIT 1", "SELECT DISTINCT * EXCLUDE (b) FROM (SELECT a, b FROM t) LIMIT 1"},
		{"redshift", "duckdb", "SELECT a, b EXCLUDE (b) INTO dst FROM src", "CREATE TABLE dst AS SELECT * EXCLUDE (b) FROM (SELECT a, b FROM src)"},
		{"spark", "duckdb", "SELECT * FROM x LATERAL VIEW OUTER inline(y) t AS a", "SELECT * FROM x LEFT JOIN LATERAL (SELECT UNNEST(y, max_depth => 2)) AS t(a) ON TRUE"},
		{"", "duckdb", "CREATE TABLE a LIKE b", "CREATE TABLE a AS SELECT * FROM b LIMIT 0"},
		{"databricks", "duckdb", "CREATE TABLE t (a INT, b TIMESTAMP, PRIMARY KEY (a, b TIMESERIES))", "CREATE TABLE t (a INT, b TIMESTAMPTZ, PRIMARY KEY (a, b))"},
		{"duckdb", "tsql", "WITH t1(c) AS (SELECT 1), t2 AS (SELECT CAST(c AS INTEGER) FROM t1) SELECT * FROM t2", "WITH t1(c) AS (SELECT 1), t2 AS (SELECT CAST(c AS INTEGER) AS c FROM t1) SELECT * FROM t2"},
		{"spark", "duckdb", "SELECT h.id, amount FROM hourlycostagg h LATERAL VIEW inline(h.costs) c", "SELECT h.id, amount FROM hourlycostagg AS h CROSS JOIN LATERAL (SELECT UNNEST(h.costs, max_depth => 2)) AS c"},
		{"spark", "duckdb", "SELECT POSEXPLODE(x) AS (a, b)", "SELECT GENERATE_SUBSCRIPTS(x, 1) - 1 AS a, UNNEST(x) AS b"},
		{"snowflake", "duckdb", "SELECT STRTOK('ab')", `SELECT CASE WHEN ' ' = '' AND 'ab' = '' THEN NULL WHEN ' ' = '' AND 1 = 1 THEN 'ab' WHEN ' ' = '' THEN NULL WHEN 1 < 0 THEN NULL WHEN 'ab' IS NULL OR ' ' IS NULL OR 1 IS NULL THEN NULL ELSE LIST_FILTER(REGEXP_SPLIT_TO_ARRAY('ab', CASE WHEN ' ' = '' THEN '' ELSE '[' || REGEXP_REPLACE(' ', '([\[\]^.\-*+?(){}|$\\])', '\\\1', 'g') || ']' END), x -> NOT x = '')[1] END`},
	}
	for _, tc := range cases {
		out, err := sqlglot.Transpile(tc.sql, tc.read, tc.write, generator.Options{})
		if err != nil {
			t.Fatalf("%s: %v", tc.sql, err)
		}
		if len(out) != 1 || out[0] != tc.want {
			t.Fatalf("%s\n got: %v\nwant: %s", tc.sql, out, tc.want)
		}
	}

	// Cross-dialect rewrites ported in the final tail.
	for _, tc := range []struct{ read, sql, want string }{
		{"bigquery", "SELECT AS STRUCT ARRAY(SELECT AS STRUCT 1 AS b FROM x) AS y FROM z", "SELECT {'y': ARRAY(SELECT {'b': 1} FROM x)} FROM z"},
		{"bigquery", "SELECT ts + MAKE_INTERVAL(1, 2, minute => 5, day => 3)", "SELECT ts + INTERVAL '1 year 2 month 5 minute 3 day'"},
		{"postgres", "WITH dates AS (SELECT GENERATE_SERIES('2020-01-01'::DATE, '2024-01-01'::DATE, '1 day'::INTERVAL) AS date) SELECT * FROM dates", "WITH dates AS (SELECT UNNEST(GENERATE_SERIES(CAST('2020-01-01' AS DATE), CAST('2024-01-01' AS DATE), CAST('1 day' AS INTERVAL))) AS date) SELECT * FROM dates"},
		{"snowflake", "SELECT * FROM produce PIVOT(SUM(produce.sales) FOR produce.quarter IN ('Q1', 'Q2'))", "SELECT * FROM produce PIVOT(SUM(sales) FOR quarter IN ('Q1' AS \"'Q1'\", 'Q2' AS \"'Q2'\"))"},
	} {
		out, err := sqlglot.Transpile(tc.sql, tc.read, "duckdb", generator.Options{})
		if err != nil || len(out) != 1 || out[0] != tc.want {
			t.Fatalf("%s: %v %v", tc.sql, out, err)
		}
	}
	// DuckDB UNNEST projections become BigQuery/Presto set-returning joins.
	for _, tc := range []struct{ write, sql, contains string }{
		{"bigquery", "SELECT UNNEST(x) + 1", "AS col FROM UNNEST(GENERATE_ARRAY(0, GREATEST(ARRAY_LENGTH(x)) - 1)) AS pos"},
		{"presto", "SELECT UNNEST(ARRAY[1, 2, 3])", "FROM UNNEST(SEQUENCE(1, GREATEST(CARDINALITY(ARRAY[1, 2, 3])))) AS _u(pos)"},
		{"snowflake", "SELECT UNNEST([1, 2, 3])", "IFF(_u.pos = _u_2.pos_2, _u_2.col, NULL) AS col"},
	} {
		out, err := sqlglot.Transpile(tc.sql, "duckdb", tc.write, generator.Options{})
		if err != nil || len(out) != 1 || !strings.Contains(out[0], tc.contains) {
			t.Fatalf("%s -> %s: %v %v", tc.sql, tc.write, out, err)
		}
	}

	// Longer sketch templates: assert the shape rather than repeat the whole literal.
	uuidOut, err := sqlglot.Transpile("SELECT UUID_STRING('fe971b24-9572-4005-b22f-351e9c09274d', 'foo')", "snowflake", "duckdb", generator.Options{})
	if err != nil || len(uuidOut) != 1 || !strings.Contains(uuidOut[0], "SUBSTRING(SHA1(UNHEX(REPLACE(") {
		t.Fatalf("uuid v5: %v %v", uuidOut, err)
	}
	for _, tc := range []struct{ sql, contains string }{
		{"MINHASH(4, col1)", "LIST_MIN(LIST_TRANSFORM(vals"},
		{"MINHASH_COMBINE(sig_col)", "WITH ORDINALITY AS t(val, pos)"},
		{"APPROXIMATE_SIMILARITY(sig_col)", "COUNT(DISTINCT h) AS num_distinct"},
		{"ZIPF(1, 10, 1234)", "ABS(HASH(1234)) % 1000000"},
		{"ZIPF(2, 100, RANDOM())", "SELECT RANDOM() AS r"},
		{"SELECT BITMAP_CONSTRUCT_AGG(v) FROM t", "UNHEX('08000000000000000000'"},
	} {
		out, err := sqlglot.Transpile(tc.sql, "snowflake", "duckdb", generator.Options{})
		if err != nil || len(out) != 1 || !strings.Contains(out[0], tc.contains) {
			t.Fatalf("%s: %v %v", tc.sql, out, err)
		}
	}
}
