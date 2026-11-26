package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestMySQLTargetTail pins the MySQL target mappings drained from the cross-dialect corpus.
func TestMySQLTargetTail(t *testing.T) {
	cases := []struct{ read, sql, want string }{
		// Timestamp truncation through a difference from year zero.
		{"postgres", "DATE_TRUNC(DAY, TIMESTAMP '2001-02-16 20:38:40')", "DATE_ADD('0000-01-01 00:00:00', INTERVAL (TIMESTAMPDIFF(DAY, '0000-01-01 00:00:00', CAST('2001-02-16 20:38:40' AS DATETIME))) DAY)"},
		{"spark", "DATE_TRUNC(MONTH, TIMESTAMP '2001-02-16 20:38:40')", "DATE_ADD('0000-01-01 00:00:00', INTERVAL (TIMESTAMPDIFF(MONTH, '0000-01-01 00:00:00', TIMESTAMP('2001-02-16 20:38:40'))) MONTH)"},
		// GENERATE_DATE_ARRAY becomes a recursive CTE.
		{"", "SELECT * FROM UNNEST(GENERATE_DATE_ARRAY(DATE '2020-01-01', DATE '2020-02-01', INTERVAL 1 WEEK))", "WITH RECURSIVE _generated_dates(date_value) AS (SELECT CAST('2020-01-01' AS DATE) AS date_value UNION ALL SELECT CAST(DATE_ADD(date_value, INTERVAL 1 WEEK) AS DATE) FROM _generated_dates WHERE CAST(DATE_ADD(date_value, INTERVAL 1 WEEK) AS DATE) <= CAST('2020-02-01' AS DATE)) SELECT * FROM (SELECT date_value FROM _generated_dates) AS _generated_dates"},
		// FULL OUTER joins are rewritten as a union of LEFT and anti-joined RIGHT.
		{"postgres", "SELECT * FROM t1 FULL OUTER JOIN t2 USING (x, y) ", "SELECT * FROM t1 LEFT OUTER JOIN t2 USING (x, y) UNION ALL SELECT * FROM t1 RIGHT OUTER JOIN t2 USING (x, y) WHERE NOT EXISTS(SELECT 1 FROM t1 WHERE t1.x = t2.x AND t1.y = t2.y)"},
		{"postgres", "SELECT * FROM x FULL JOIN y ON x.id = y.id ORDER BY 1 LIMIT 0", "SELECT * FROM x LEFT JOIN y ON x.id = y.id UNION ALL SELECT * FROM x RIGHT JOIN y ON x.id = y.id WHERE NOT EXISTS(SELECT 1 FROM x WHERE x.id = y.id) ORDER BY 1 LIMIT 0"},
		// Intervals and windows.
		{"postgres", "SELECT NOW() + MAKE_INTERVAL(years => 1, months => 2, days => 3)", "SELECT CURRENT_TIMESTAMP() + INTERVAL 1 YEAR + INTERVAL 2 MONTH + INTERVAL 3 DAY"},
		{"postgres", "SELECT NOW() - MAKE_INTERVAL(years => 1)", "SELECT CURRENT_TIMESTAMP() - INTERVAL 1 YEAR"},
		{"snowflake", "SELECT LAG(col1) IGNORE NULLS OVER (ORDER BY col2) FROM table1", "SELECT LAG(col1) OVER (ORDER BY CASE WHEN col2 IS NULL THEN 1 ELSE 0 END, col2) FROM table1"},
		// Functions.
		{"bigquery", "BIT_COUNT(x)", "BIT_COUNT(x)"},
		{"mysql", "MONTHNAME(x)", "DATE_FORMAT(x, '%M')"},
		{"mysql", "SELECT ISNULL(x)", "SELECT (x IS NULL)"},
		{"mysql", "SELECT TO_DAYS(x)", "SELECT (DATEDIFF(x, '0000-01-01') + 1)"},
		{"mysql", "SELECT DATE_FORMAT('2007-10-04 22:23:00', '%H:%i:%s')", "SELECT DATE_FORMAT('2007-10-04 22:23:00', '%T')"},
		{"clickhouse", "SELECT formatDateTime(NOW(), '%Y-%m-%d')", "SELECT DATE_FORMAT(NOW(), '%Y-%m-%d')"},
		{"bigquery", "DATE_DIFF(DATE '2010-07-07', DATE '2008-12-25', DAY)", "DATEDIFF(CAST('2010-07-07' AS DATE), CAST('2008-12-25' AS DATE))"},
		{"bigquery", "SELECT LAST_DAY(CAST('2008-11-25' AS DATE), MONTH)", "SELECT LAST_DAY(CAST('2008-11-25' AS DATE))"},
		{"bigquery", "SELECT TIMESTAMP_ADD(TIMESTAMP \"2008-12-25 15:30:00+00\", INTERVAL 10 MINUTE)", "SELECT DATE_ADD(TIMESTAMP('2008-12-25 15:30:00+00'), INTERVAL '10' MINUTE)"},
		{"oracle", "TRUNC(3.14159)", "TRUNCATE(3.14159)"},
		{"oracle", "DATE '2022-01-01'", "CAST('2022-01-01' AS DATE)"},
		{"postgres", "SELECT '11' || '22'", "SELECT CONCAT('11', '22')"},
		{"postgres", "SELECT EXTRACT(epoch FROM TIMESTAMP '2024-04-29 12:00:00')", "SELECT UNIX_TIMESTAMP(CAST('2024-04-29 12:00:00' AS DATETIME))"},
		{"postgres", "SELECT department, array_agg(name) AS employee_names FROM data GROUP BY department", "SELECT department, GROUP_CONCAT(name) AS employee_names FROM data GROUP BY department"},
		{"snowflake", "SELECT CONVERT_TIMEZONE('America/Los_Angeles', 'America/New_York', '2024-08-06 09:10:00.000')", "SELECT CONVERT_TZ('2024-08-06 09:10:00.000', 'America/Los_Angeles', 'America/New_York')"},
		{"sqlite", "UNICODE(x)", "ORD(CONVERT(x USING utf32))"},
		// Misc.
		{"mysql", "SELECT * FROM test LIMIT 0 + 1, 0 + 1", "SELECT * FROM test LIMIT 1 OFFSET 1"},
		{"bigquery", "CURRENT_DATE('UTC')", "CURRENT_DATE AT TIME ZONE 'UTC'"},
		{"tsql", "EOMONTH(GETDATE(), -1)", "LAST_DAY(DATE_ADD(CURRENT_TIMESTAMP(), INTERVAL -1 MONTH))"},
	}
	for _, tc := range cases {
		out, err := sqlglot.Transpile(tc.sql, tc.read, "mysql", generator.Options{})
		if err != nil || len(out) != 1 || out[0] != tc.want {
			t.Fatalf("%s -> mysql\n got: %v %v\nwant: %s", tc.sql, out, err, tc.want)
		}
	}
}
