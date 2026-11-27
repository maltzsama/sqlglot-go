package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestPostgresTargetTail pins the Postgres target mappings.
func TestPostgresTargetTail(t *testing.T) {
	cases := []struct{ read, sql, want string }{
		{"databricks", "SELECT DATEDIFF(day, 'start', 'end')", "SELECT CAST(EXTRACT(epoch FROM CAST('end' AS TIMESTAMP) - CAST('start' AS TIMESTAMP)) / 86400 AS BIGINT)"},
		{"databricks", "SELECT DATEDIFF(month, 'start', 'end')", "SELECT CAST(EXTRACT(year FROM AGE(CAST('end' AS TIMESTAMP), CAST('start' AS TIMESTAMP))) * 12 + EXTRACT(month FROM AGE(CAST('end' AS TIMESTAMP), CAST('start' AS TIMESTAMP))) AS BIGINT)"},
		{"mysql", "SELECT DATEDIFF(x, y)", "SELECT (CAST(x AS DATE) - CAST(y AS DATE))"},
		{"tsql", "SELECT t.x, y.z FROM x CROSS APPLY tvfTest(t.x) y(z)", "SELECT t.x, y.z FROM x INNER JOIN LATERAL TVFTEST(t.x) AS y(z) ON TRUE"},
		{"", "SELECT * FROM t QUALIFY COUNT(*) OVER () > 1", "SELECT * FROM (SELECT *, COUNT(*) OVER () AS _w FROM t) AS _t WHERE _w > 1"},
		{"", "SELECT COUNT_IF(col % 2 = 0) FROM foo", "SELECT SUM(CASE WHEN col % 2 = 0 THEN 1 ELSE 0 END) FROM foo"},
		{"", "MEDIAN(x)", "PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY x)"},
		{"", "SELECT IS_ASCII(x)", "SELECT (x ~ '^[[:ascii:]]*$')"},
		{"", "SELECT TIME_TO_UNIX(TIMESTAMP '2023-01-04 04:05:06.789')", "SELECT DATE_PART('epoch', CAST('2023-01-04 04:05:06.789' AS TIMESTAMP))"},
		{"", "UUID()", "GEN_RANDOM_UUID()"},
		{"", "JSONB_EXTRACT(x, 'y')", "x #> 'y'"},
		{"", "JSONB_EXTRACT_SCALAR(x, 'y')", "x #>> 'y'"},
		{"tsql", "EOMONTH(GETDATE(), -1)", "CAST(DATE_TRUNC('MONTH', CAST(CURRENT_TIMESTAMP AS DATE) + INTERVAL '-1 MONTH') + INTERVAL '1 MONTH' - INTERVAL '1 DAY' AS DATE)"},
		{"bigquery", "DATE_SUB(CURRENT_DATE(), INTERVAL 1 DAY)", "CURRENT_DATE - INTERVAL '1 DAY'"},
		{"", "ARRAY_PREPEND(a, b)", "ARRAY_PREPEND(b, a)"},
		{"postgres", "SELECT REGEXP_REPLACE('aaa', 'a', 'b')", "SELECT REGEXP_REPLACE('aaa', 'a', 'b')"},
		{"snowflake", "REGEXP_REPLACE('aaa', 'a', 'b', 1)", "REGEXP_REPLACE('aaa', 'a', 'b', 1, 'g')"},
		{"postgres", "SELECT GENERATE_SERIES(a, b, '1.5 days')", "SELECT GENERATE_SERIES(a, b, INTERVAL '1.5 DAY')"},
		{"snowflake", "ARRAY_GENERATE_RANGE(3, 0, -1)", "GENERATE_SERIES(3, 0 + 1, -1)"},
		{"", "SELECT 'ok' FROM t QUALIFY COUNT(*) OVER () > 1", "SELECT _c AS ok FROM (SELECT 'ok' AS _c, COUNT(*) OVER () AS _w FROM t) AS _t WHERE _w > 1"},
		{"", "SELECT l.id, r.id FROM l, r QUALIFY ROW_NUMBER() OVER () = 1", "SELECT id, _c AS id FROM (SELECT l.id, r.id AS _c, ROW_NUMBER() OVER () AS _w FROM l, r) AS _t WHERE _w = 1"},
		{"", "SELECT a FROM t QUALIFY ROW_NUMBER() OVER () = 1 AND b > 0", "SELECT a FROM (SELECT a, b, ROW_NUMBER() OVER () AS _w FROM t) AS _t WHERE _w = 1 AND b > 0"},
		{"", "SELECT x, ROW_NUMBER() OVER (ORDER BY x) AS rn FROM t QUALIFY rn = 2 LIMIT 1", "SELECT x, rn FROM (SELECT x, ROW_NUMBER() OVER (ORDER BY x NULLS FIRST) AS rn FROM t) AS _t WHERE rn = 2 LIMIT 1"},
		// Expectations checked against the pinned Python oracle (generators/postgres.py:72):
		// a non-literal amount renders as INTERVAL '1 <unit>' * <amount>.
		{"tsql", "EOMONTH(GETDATE(), n)", "CAST(DATE_TRUNC('MONTH', CAST(CURRENT_TIMESTAMP AS DATE) + INTERVAL '1 MONTH' * n) + INTERVAL '1 MONTH' - INTERVAL '1 DAY' AS DATE)"},
		{"", "DATE_ADD(x, n, 'DAY')", "x + INTERVAL '1 DAY' * n"},
		{"snowflake", "REGEXP_REPLACE(subject, pattern, replacement)", "REGEXP_REPLACE(subject, pattern, replacement, 'g')"},
		{"snowflake", "ARRAY_GENERATE_RANGE(0, 3)", "GENERATE_SERIES(0, 3 - 1)"},
	}
	for _, tc := range cases {
		out, err := sqlglot.Transpile(tc.sql, tc.read, "postgres", generator.Options{})
		if err != nil || len(out) != 1 || out[0] != tc.want {
			t.Fatalf("%s -> postgres\n got: %v %v\nwant: %s", tc.sql, out, err, tc.want)
		}
	}
}
