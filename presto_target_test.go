package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestPrestoTargetTail pins the BigQuery/Postgres to Presto mappings.
func TestPrestoTargetTail(t *testing.T) {
	cases := []struct{ read, sql, want string }{
		{"bigquery", "DELETE FROM db.example_table AS tb WHERE tb.x = 1", "DELETE FROM db.example_table WHERE x = 1"},
		{"bigquery", "TO_HEX(x)", "LOWER(TO_HEX(x))"},
		{"bigquery", "SELECT TO_HEX(MD5(some_string))", "SELECT LOWER(TO_HEX(MD5(some_string)))"},
		{"bigquery", "SHA1(x)", "SHA1(x)"},
		{"bigquery", "SELECT GENERATE_UUID()", "SELECT CAST(UUID() AS VARCHAR)"},
		{"bigquery", "TIMESTAMP(x)", "CAST(x AS TIMESTAMP WITH TIME ZONE)"},
		{"bigquery", "TIMESTAMP_DIFF(a, b, MONTH)", "DATE_DIFF('MONTH', b, a)"},
		{"bigquery", "SELECT DATETIME_DIFF(DATETIME '2021-02-01 00:00:00', DATETIME '2021-01-31 00:00:00', MONTH)", "SELECT DATE_DIFF('MONTH', DATE_TRUNC('MONTH', CAST('2021-01-31 00:00:00' AS TIMESTAMP)), DATE_TRUNC('MONTH', CAST('2021-02-01 00:00:00' AS TIMESTAMP)))"},
		{"bigquery", "TO_JSON_STRING(x)", "JSON_FORMAT(CAST(x AS JSON))"},
		{"bigquery", "SELECT a[1], b[OFFSET(1)], c[ORDINAL(1)], d[SAFE_OFFSET(1)], e[SAFE_ORDINAL(1)]", "SELECT a[2], b[2], c[1], ELEMENT_AT(d, 2), ELEMENT_AT(e, 1)"},

		{"postgres", "ARRAY_LENGTH(arr, 1)", "CARDINALITY(arr)"},
		{"postgres", "CONCAT(a, b)", "CONCAT(COALESCE(CAST(a AS VARCHAR), ''), COALESCE(CAST(b AS VARCHAR), ''))"},
		{"postgres", "a || b", "CONCAT(CAST(a AS VARCHAR), CAST(b AS VARCHAR))"},
		{"postgres", "SELECT * FROM GENERATE_SERIES(a, b)", "SELECT * FROM UNNEST(SEQUENCE(a, b))"},
		{"postgres", "SELECT 'epoch'::TIMESTAMP", "SELECT CAST('1970-01-01 00:00:00' AS TIMESTAMP)"},
		{"postgres", "SELECT PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY amount)", "SELECT APPROX_PERCENTILE(amount, 0.5)"},
		{"postgres", "WITH RECURSIVE t AS (SELECT 1 AS n UNION ALL SELECT n + 1 AS n FROM t WHERE n < 4) SELECT SUM(n) FROM t", "WITH RECURSIVE t(n) AS (SELECT 1 AS n UNION ALL SELECT n + 1 AS n FROM t WHERE n < 4) SELECT SUM(n) FROM t"},
	}
	for _, tc := range cases {
		out, err := sqlglot.Transpile(tc.sql, tc.read, "presto", generator.Options{})
		if err != nil || len(out) != 1 || out[0] != tc.want {
			t.Fatalf("%s -> presto\n got: %v %v\nwant: %s", tc.sql, out, err, tc.want)
		}
	}
}
