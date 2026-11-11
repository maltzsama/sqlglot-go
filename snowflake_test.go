package sqlglot_test

import (
	"testing"

	"github.com/maltzsama/sqlglot-go/generator"
)

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
		"SELECT TIMESTAMPADD(HOUR, 1, x)",
		"SELECT TIMESTAMPDIFF(HOUR, a, b)",
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
