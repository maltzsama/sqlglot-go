package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
)

func TestSnowflakeNthValueFromClause(t *testing.T) {
	for _, sql := range []string{
		"SELECT NTH_VALUE(a, 2) FROM t",
		"SELECT NTH_VALUE(a, 2) FROM FIRST OVER () FROM t",
		"SELECT NTH_VALUE(a, 2) FROM LAST OVER () FROM t",
	} {
		t.Run(sql, func(t *testing.T) {
			assertRoundTrip(t, "snowflake", sql, sql)
		})
	}
}

func TestSnowflakeShow(t *testing.T) {
	for _, tc := range []struct{ sql, want string }{
		{"SHOW VIEWS IN foo", "SHOW VIEWS IN SCHEMA foo"},
		{"SHOW TERSE PRIMARY KEYS IN db.schema.tbl", "SHOW PRIMARY KEYS IN TABLE db.schema.tbl"},
		{"SHOW ICEBERG TABLES IN db.schema", "SHOW ICEBERG TABLES IN SCHEMA db.schema"},
		{"show terse tables in db.schema starts with 'a' limit 10 from 'b'", "SHOW TERSE TABLES IN SCHEMA db.schema STARTS WITH 'a' LIMIT 10 FROM 'b'"},
		{"SHOW COLUMNS IN VIEW", "SHOW COLUMNS IN VIEW"},
	} {
		t.Run(tc.sql, func(t *testing.T) {
			node, err := sqlglot.ParseOne(tc.sql, "snowflake")
			if err != nil || node.Kind() != exp.KindShow {
				t.Fatalf("expected Show, got %v, %v", node, err)
			}
			got, err := sqlglot.Generate(node, "snowflake", generator.Options{})
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestSnowflakeHistoricalData(t *testing.T) {
	for _, sql := range []string{
		"SELECT * FROM t AT (OFFSET => -60 * 5)",
		"SELECT * FROM t BEFORE (STATEMENT => $query_id) AS x",
		"SELECT * FROM t CHANGES (INFORMATION => APPEND_ONLY) AT (STREAM => 's') END (TIMESTAMP => $ts)",
	} {
		t.Run(sql, func(t *testing.T) {
			node, err := sqlglot.ParseOne(sql, "snowflake")
			if err != nil {
				t.Fatal(err)
			}
			var historical bool
			for _, n := range node.Walk() {
				historical = historical || n.Kind() == exp.KindHistoricalData
			}
			if !historical {
				t.Fatal("expected structured historical data")
			}
			got, err := sqlglot.Generate(node, "snowflake", generator.Options{})
			if err != nil || got != sql {
				t.Fatalf("got %q, %v; want %q", got, err, sql)
			}
		})
	}
}

func TestSnowflakeSemanticView(t *testing.T) {
	sql := "SELECT * FROM SEMANTIC_VIEW(tbl DIMENSIONS a.b METRICS a.c WHERE a.c > 5)"
	want := "SELECT * FROM SEMANTIC_VIEW(tbl METRICS a.c DIMENSIONS a.b WHERE a.c > 5)"
	node, err := sqlglot.ParseOne(sql, "snowflake")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, n := range node.Walk() {
		if n.Kind() == exp.KindSemanticView {
			found = true
			if n.Arg("metrics") == nil || n.Arg("dimensions") == nil || n.Arg("where") == nil {
				t.Fatal("semantic view missing clauses")
			}
		}
	}
	if !found {
		t.Fatal("expected structured semantic view")
	}
	got, err := sqlglot.Generate(node, "snowflake", generator.Options{})
	if err != nil || got != want {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
}

func TestSnowflakeStageSources(t *testing.T) {
	for _, sql := range []string{
		"SELECT * FROM @foo/bar (FILE_FORMAT => ds.test.fmt, PATTERN => 'test') AS x",
		"SELECT * FROM @~/some/path/file.csv",
		"SELECT * FROM '@external/location' (FILE_FORMAT => 'path.to.csv')",
	} {
		t.Run(sql, func(t *testing.T) {
			node, err := sqlglot.ParseOne(sql, "snowflake")
			if err != nil {
				t.Fatal(err)
			}
			var stage exp.Expression
			for _, n := range node.Walk() {
				if n.Kind() == exp.KindTable {
					stage = n
					break
				}
			}
			if stage == nil || stage.This() == nil || (stage.This().Kind() != exp.KindVar && stage.This().Kind() != exp.KindLiteral) {
				t.Fatal("stage source is not structured as a table")
			}
			got, err := sqlglot.Generate(node, "snowflake", generator.Options{})
			if err != nil || got != sql {
				t.Fatalf("got %q, %v; want %q", got, err, sql)
			}
		})
	}
}

// TestSnowflakeCore ports the same-dialect round-trip core of test_snowflake
// (tests/dialects/test_snowflake.py).
func TestSnowflakeCore(t *testing.T) {
	for _, sql := range []string{
		"SELECT 1",
		`SELECT "a" FROM t`,
		`SELECT "a"."b" FROM "t" AS "a"`,
		"SELECT $1 FROM t",
		"SELECT * FROM foo WHERE x > 1",
		"SELECT CAST(x AS VARIANT)",
		"SELECT CAST(x AS VARCHAR)",
		"SELECT CAST(x AS DECIMAL(38, 0))",
		"SELECT TRY_CAST(x AS INT)",
		"SELECT IFF(x, 1, 0)",
		"SELECT BOOLAND(a, b)",
		"SELECT BOOLOR(a, b)",
		"SELECT BITAND(a, b)",
		"SELECT BITANDAGG(x)",
		"SELECT BITORAGG(x)",
		"SELECT BITXORAGG(x)",
		"SELECT BITSHIFTLEFT(a, 1)",
		"SELECT HEX_ENCODE(x)",
		"SELECT HEX_DECODE_BINARY(x)",
		"SELECT UUID_STRING()",
		"SELECT LISTAGG(x, ', ')",
		"SELECT OBJECT_CONSTRUCT('a', 1)",
		"SELECT ARRAY_CONTAINS(1, x)",
		"SELECT ARRAY_INTERSECTION(a, b)",
		"SELECT ARRAYS_OVERLAP(a, b)",
		"SELECT CHARINDEX(a, b)",
		"SELECT STARTSWITH(a, b)",
		"SELECT ENDSWITH(a, b)",
		"SELECT DAYOFWEEKISO(x)",
		"SELECT DAYOFYEAR(x)",
		"SELECT WEEKISO(x)",
		"SELECT YEAROFWEEK(x)",
		"SELECT DATEADD(DAY, 1, x)",
		"SELECT DATEDIFF(DAY, a, b)",
		"SELECT DATE_PART('year', x)",
		"SELECT DATE_PART(YEAR, x)",
		"SELECT TO_DATE(x)",
		"SELECT TO_TIMESTAMP(x)",
		"SELECT TRY_TO_DATE(x)",
		"SELECT MD5_BINARY(x)",
		"SELECT SHA1_BINARY(x)",
		"SELECT SHA2_BINARY(x, 256)",
	} {
		assertRoundTrip(t, "snowflake", sql, sql)
	}
}

// TestSnowflakeCanonicalizations pins the generator's same-dialect normalizations.
func TestSnowflakeCanonicalizations(t *testing.T) {
	cases := map[string]string{
		"SELECT ARRAY_CONSTRUCT(1, 2)":        "SELECT [1, 2]",
		"SELECT DATE(x)":                      "SELECT TO_DATE(x)",
		"SELECT TIMESTAMPADD(HOUR, 1, x)":     "SELECT DATEADD(HOUR, 1, x)",
		"SELECT TIMESTAMPDIFF(HOUR, a, b)":    "SELECT DATEDIFF(HOUR, a, b)",
		"SELECT EXTRACT(YEAR FROM x)":         "SELECT DATE_PART(YEAR, x)",
		"SELECT SQUARE(x)":                    "SELECT POWER(x, 2)",
		"SELECT LEN(x)":                       "SELECT LENGTH(x)",
		"SELECT ARRAY_CONSTRUCT()":            "SELECT []",
		"SELECT CAST(x AS OBJECT)":            "SELECT CAST(x AS OBJECT)",
		"SELECT CAST(x AS ARRAY(INT))":        "SELECT CAST(x AS ARRAY(INT))",
		"SELECT CAST(x AS MAP(VARCHAR, INT))": "SELECT CAST(x AS MAP(VARCHAR, INT))",
	}
	for sql, want := range cases {
		assertRoundTrip(t, "snowflake", sql, want)
	}
}

// TestSnowflakeUndrop ports the UNDROP statement.
func TestSnowflakeUndrop(t *testing.T) {
	for _, sql := range []string{
		"UNDROP TABLE my_table",
		"UNDROP TABLE db.schema.my_table",
		"UNDROP DATABASE my_db",
		"UNDROP SCHEMA my_schema",
		"UNDROP SCHEMA my_schema RENAME TO new_schema",
		"UNDROP TAG my_tag",
	} {
		assertRoundTrip(t, "snowflake", sql, sql)
	}
}

// TestSnowflakeDirectedJoin ports the DIRECTED join modifier.
func TestSnowflakeDirectedJoin(t *testing.T) {
	for _, sql := range []string{
		"SELECT * FROM a LEFT DIRECTED JOIN b USING (id)",
		"SELECT * FROM a RIGHT OUTER DIRECTED JOIN b USING (id)",
	} {
		assertRoundTrip(t, "snowflake", sql, sql)
	}
}

// TestSnowflakeReservedIdentifiers verifies the Snowflake-only ID_VAR keywords are usable as
// column names.
func TestSnowflakeReservedIdentifiers(t *testing.T) {
	for _, sql := range []string{
		"SELECT integration FROM t",
		"SELECT package FROM t",
		"SELECT policy FROM t",
		"SELECT pool FROM t",
		"SELECT role FROM t",
		"SELECT rule FROM t",
		"SELECT volume FROM t",
	} {
		assertRoundTrip(t, "snowflake", sql, sql)
	}
}

// TestSnowflakeDescribe verifies the describe kind is preserved for tables.
func TestSnowflakeDescribe(t *testing.T) {
	assertRoundTrip(t, "snowflake", "DESCRIBE TABLE db.table", "DESCRIBE TABLE db.table")
}

// ensure the generator import is referenced even if a helper changes.
var _ = generator.Options{}

// TestSnowflakeReviewRegressions pins the review-driven fixes: DIV0 null/grouping semantics,
// OBJECT_CONSTRUCT_KEEP_NULL, Boolean XOR/NOT, the bitwise padding argument, TIMEDIFF,
// CURRENT_TIMESTAMP precision and ARRAY_POSITION argument order.
func TestSnowflakeReviewRegressions(t *testing.T) {
	cases := map[string]string{
		"SELECT DIV0(NULL, 0)":                         "SELECT IFF(0 = 0 AND NOT NULL IS NULL, 0, NULL / 0)",
		"SELECT DIV0(a - b, c - d)":                    "SELECT IFF((c - d) = 0 AND NOT (a - b) IS NULL, 0, (a - b) / (c - d))",
		"SELECT OBJECT_CONSTRUCT_KEEP_NULL('k', NULL)": "SELECT OBJECT_CONSTRUCT_KEEP_NULL('k', NULL)",
		"SELECT OBJECT_CONSTRUCT('k', NULL)":           "SELECT OBJECT_CONSTRUCT('k', NULL)",
		"SELECT BOOLXOR(2, 0)":                         "SELECT BOOLXOR(2, 0)",
		"SELECT BOOLNOT(x)":                            "SELECT BOOLNOT(x)",
		"SELECT BITAND(a, b, 'pad')":                   "SELECT BITAND(a, b, 'pad')",
		"SELECT TIMEDIFF(HOUR, a, b)":                  "SELECT DATEDIFF(HOUR, a, b)",
		"SELECT CURRENT_TIMESTAMP(3)":                  "SELECT CURRENT_TIMESTAMP(3)",
		"SELECT ARRAY_CONTAINS('x', arr)":              "SELECT ARRAY_CONTAINS('x', arr)",
		"SELECT ARRAY_POSITION('x', arr)":              "SELECT ARRAY_POSITION('x', arr)",
	}
	for sql, want := range cases {
		assertRoundTrip(t, "snowflake", sql, want)
	}
}

// TestSnowflakeTimeDiffRenderer ensures a TimeDiff node produces Snowflake's DATEDIFF form.
func TestSnowflakeTimeDiffRenderer(t *testing.T) {
	expr := exp.TimeDiff(exp.Args{
		"this":       exp.Column(exp.Args{"this": exp.ToIdentifier("b")}),
		"expression": exp.Column(exp.Args{"this": exp.ToIdentifier("a")}),
		"unit":       exp.Var(exp.Args{"this": "HOUR"}),
	})
	got, err := sqlglot.Generate(expr, "snowflake", generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if want := "DATEDIFF(HOUR, a, b)"; got != want {
		t.Fatalf("TimeDiff = %q, want %q", got, want)
	}
}
