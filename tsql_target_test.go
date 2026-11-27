package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestTSQLTargetTail pins the T-SQL target mappings added for the corpus.
func TestTSQLTargetTail(t *testing.T) {
	cases := []struct{ read, sql, want string }{
		{"snowflake", "SELECT * FROM t QUALIFY COUNT(*) OVER () > 1", "SELECT * FROM (SELECT *, COUNT_BIG(*) OVER () AS _w FROM t) AS _t WHERE _w > 1"},
		{"spark", "SELECT IF(cond, 'True', 'False')", "SELECT IIF(cond <> 0, 'True', 'False')"},
		{"mysql", "CHAR(10)", "CHAR(10)"},
		{"mysql", "GROUP_CONCAT(DISTINCT x ORDER BY y DESC)", "STRING_AGG(x, ',') WITHIN GROUP (ORDER BY y DESC)"},
		{"mysql", "GROUP_CONCAT(DISTINCT a, b, c ORDER BY d SEPARATOR '')", "STRING_AGG(a + b + c, '') WITHIN GROUP (ORDER BY d)"},
		{"postgres", "SELECT MAKE_TIME(23, 59, 59)", "SELECT TIMEFROMPARTS(23, 59, 59, 0, 0)"},
		{"oracle", "TRUNC(3.14159)", "ROUND(3.14159, 0, 1)"},
		{"hive", "DATE_ADD('2020-01-01', 1)", "DATEADD(DAY, 1, CAST(CAST('2020-01-01' AS DATETIME2) AS DATE))"},
		{"bigquery", "SELECT DATE_DIFF(d1, d2, WEEK(SUNDAY))", "SELECT DATEDIFF(WEEK, d2, d1)"},
		{"", "SELECT * FROM UNNEST(GENERATE_DATE_ARRAY(DATE '2020-01-01', DATE '2020-02-01', INTERVAL 1 WEEK))", "WITH _generated_dates(date_value) AS (SELECT CAST('2020-01-01' AS DATE) AS date_value UNION ALL SELECT CAST(DATEADD(WEEK, 1, date_value) AS DATE) FROM _generated_dates WHERE CAST(DATEADD(WEEK, 1, date_value) AS DATE) <= CAST('2020-02-01' AS DATE)) SELECT * FROM (SELECT date_value AS date_value FROM _generated_dates) AS _generated_dates"},
	}
	for _, tc := range cases {
		out, err := sqlglot.Transpile(tc.sql, tc.read, "tsql", generator.Options{})
		if err != nil || len(out) != 1 || out[0] != tc.want {
			t.Fatalf("%s -> tsql\n got: %v %v\nwant: %s", tc.sql, out, err, tc.want)
		}
	}
}
