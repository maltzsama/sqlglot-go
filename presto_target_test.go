package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestPrestoTargetTail pins the Presto target mappings drained from the cross-dialect corpus.
func TestPrestoTargetTail(t *testing.T) {
	cases := []struct{ read, sql, want string }{
		// LATERAL VIEW becomes CROSS JOIN UNNEST.
		{"hive", "SELECT a FROM x LATERAL VIEW EXPLODE(y) t AS a", "SELECT a FROM x CROSS JOIN UNNEST(y) AS t(a)"},
		{"hive", "SELECT a FROM x LATERAL VIEW POSEXPLODE(y) t AS pos, col", "SELECT a FROM x CROSS JOIN LATERAL (SELECT pos - 1 AS pos, col FROM UNNEST(y) WITH ORDINALITY AS t(col, pos))"},
		{"spark", "SELECT id_column, name, age FROM test_table LATERAL VIEW INLINE(struc_column) explode_view AS name, age", "SELECT id_column, name, age FROM test_table CROSS JOIN UNNEST(struc_column) AS explode_view(name, age)"},
		{"hive", "SELECT a, b FROM x LATERAL VIEW EXPLODE(y) t AS a LATERAL VIEW EXPLODE(z) u AS b", "SELECT a, b FROM x CROSS JOIN UNNEST(y) AS t(a) CROSS JOIN UNNEST(z) AS u(b)"},
		// QUALIFY becomes a filtered derived table.
		{"snowflake", "SELECT i, p, o FROM qt QUALIFY ROW_NUMBER() OVER (PARTITION BY p ORDER BY o) = 1", "SELECT i, p, o FROM (SELECT i, p, o, ROW_NUMBER() OVER (PARTITION BY p ORDER BY o) AS _w FROM qt) AS _t WHERE _w = 1"},
		// Series and epochs.
		{"snowflake", "ARRAY_GENERATE_RANGE(0, 3)", "SEQUENCE(0, 2)"},
		{"snowflake", "SELECT DATE_PART(epoch_second, foo) as ddate from table_name", "SELECT TO_UNIXTIME(CAST(foo AS TIMESTAMP)) AS ddate FROM table_name"},
		{"snowflake", "SELECT DATE_PART(epoch_milliseconds, foo) as ddate from table_name", "SELECT TO_UNIXTIME(CAST(foo AS TIMESTAMP)) * 1000 AS ddate FROM table_name"},
		// Functions.
		{"snowflake", "UUID_STRING('fe971b24-9572-4005-b22f-351e9c09274d', 'foo')", "CAST(UUID() AS VARCHAR)"},
		{"spark", "DAYOFWEEK(CAST('2012-08-08 01:00:00' AS TIMESTAMP))", "((DAY_OF_WEEK(CAST(CAST(TRY_CAST('2012-08-08 01:00:00' AS TIMESTAMP WITH TIME ZONE) AS TIMESTAMP) AS DATE)) % 7) + 1)"},
		{"spark", "SELECT DATE_ADD(MONTH, 20, col)", "SELECT DATE_ADD('MONTH', 20, col)"},
		{"redshift", "x is false", "NOT x"},
		{"redshift", "x is not false", "NOT NOT x"},
		{"redshift", "x is true", "x"},
		{"redshift", "SELECT LEFT(a, 3), RIGHT(a, 3)", "SELECT SUBSTR(a, 1, 3), SUBSTR(a, LENGTH(a) - (3 - 1))"},
		{"redshift", "DATEDIFF('day', a, b)", "DATE_DIFF('DAY', CAST(a AS TIMESTAMP), CAST(b AS TIMESTAMP))"},
		{"spark", "TO_DATE(x, 'yyyy-MM-dd')", "CAST(CAST(x AS TIMESTAMP) AS DATE)"},
		{"hive", "DATE_FORMAT('2020-01-01', 'yyyy-MM-dd HH:mm:ss')", "DATE_FORMAT(CAST('2020-01-01' AS TIMESTAMP), '%Y-%m-%d %H:%i:%s')"},
		{"hive", "from_unixtime(x, \"yyyy-MM-dd'T'HH\")", "DATE_FORMAT(FROM_UNIXTIME(x), '%Y-%m-%d''T''%H')"},
	}
	for _, tc := range cases {
		out, err := sqlglot.Transpile(tc.sql, tc.read, "presto", generator.Options{})
		if err != nil || len(out) != 1 || out[0] != tc.want {
			t.Fatalf("%s -> presto\n got: %v %v\nwant: %s", tc.sql, out, err, tc.want)
		}
	}
}
