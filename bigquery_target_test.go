package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestBigQueryTargetTail pins the BigQuery target mappings drained from the cross-dialect corpus.
func TestBigQueryTargetTail(t *testing.T) {
	cases := []struct{ read, sql, want string }{
		// Optimizer hints are not supported and disappear.
		{"spark", "SELECT /*+ BROADCAST(table) */ cola FROM table", "SELECT cola FROM table"},
		{"spark", "SELECT /*+ COALESCE(3), REPARTITION(1) */ * FROM x", "SELECT * FROM x"},
		// CTE column lists are pushed into the projections.
		{"spark", "WITH cte(foo) AS (SELECT 1) SELECT foo FROM cte", "WITH cte AS (SELECT 1 AS foo) SELECT foo FROM cte"},
		{"postgres", "WITH cte(foo) AS (SELECT 1 UNION ALL SELECT 2) SELECT foo FROM cte", "WITH cte AS (SELECT 1 AS foo UNION ALL SELECT 2) SELECT foo FROM cte"},
		// Exploding GENERATE_SERIES becomes UNNEST(GENERATE_ARRAY(...)).
		{"postgres", "SELECT GENERATE_SERIES(1, 5)", "SELECT _gen_series_value FROM UNNEST(GENERATE_ARRAY(1, 5)) AS _gen_series_value"},
		{"postgres", "SELECT y, GENERATE_SERIES(1, 3) AS g FROM t", "SELECT y, g FROM t CROSS JOIN UNNEST(GENERATE_ARRAY(1, 3)) AS g"},
		{"postgres", "SELECT y, GENERATE_SERIES(1, 2) AS a, GENERATE_SERIES(11, 13) AS b FROM t", "SELECT y, a, b FROM t CROSS JOIN UNNEST(GENERATE_ARRAY(1, 2)) AS a CROSS JOIN UNNEST(GENERATE_ARRAY(11, 13)) AS b"},
		// Series and intervals.
		{"postgres", "SELECT MAKE_TIME(15, 30, 00)", "SELECT TIME(15, 30, 00)"},
		{"presto", "SELECT INTERVAL '1' days", "SELECT INTERVAL '1' DAY"},
		{"presto", "SELECT INTERVAL '1' SeCoNds", "SELECT INTERVAL '1' SECOND"},
		{"tsql", "DATEADD(year, 50, '2006-07-31')", "DATE_ADD('2006-07-31', INTERVAL 50 YEAR)"},
		// Functions and literals.
		{"spark", "SELECT SPACE(2)", "SELECT REPEAT(' ', 2)"},
		{"clickhouse", "SELECT any(y) AS x", "SELECT ANY_VALUE(y) AS x"},
		{"oracle", "TO_NUMBER(x)", "CAST(x AS FLOAT64)"},
		{"presto", "TO_HEX(x)", "UPPER(TO_HEX(x))"},
		{"presto", "LOWER(TO_HEX(x))", "TO_HEX(x)"},
		{"presto", "SELECT FROM_ISO8601_DATE('2020-05-11')", "SELECT CAST('2020-05-11' AS DATE)"},
		{"presto", "SELECT FROM_ISO8601_TIMESTAMP('2020-05-11T11:15:05')", "SELECT CAST('2020-05-11T11:15:05' AS TIMESTAMP)"},
		{"mysql", "SELECT 0xCC", "SELECT FROM_HEX('CC')"},
		{"mysql", "SELECT x'CC'", "SELECT FROM_HEX('CC')"},
		{"bigquery", "SELECT 0xA", "SELECT 0xA"},
		// Diffs, limits and formats.
		{"mysql", "TIMESTAMPDIFF(month, b, a)", "TIMESTAMP_DIFF(a, b, MONTH)"},
		{"mysql", "SELECT * FROM test LIMIT 0 + 1, 0 + 1", "SELECT * FROM test LIMIT 1 OFFSET 1"},
		{"hive", "DATE_FORMAT('2020-01-01', 'yyyy-MM-dd HH:mm:ss')", "FORMAT_DATE('%Y-%m-%d %H:%M:%S', CAST('2020-01-01' AS DATETIME))"},
		{"spark", "SELECT TO_DATE(x, 'MM/dd/yyyy')", "SELECT CAST(SAFE_CAST(x AS TIMESTAMP FORMAT 'MM/DD/YYYY') AS DATE)"},
		{"tsql", "EOMONTH(GETDATE(), -1)", "LAST_DAY(DATE_ADD(CAST(CURRENT_TIMESTAMP() AS DATE), INTERVAL -1 MONTH))"},
		// JSON and arrays.
		{"bigquery", "JSON_EXTRACT(x, '$.y[*]')", "JSON_EXTRACT(x, '$.y')"},
		{"", "ARRAY_ANY(arr, x -> pred)", "(ARRAY_LENGTH(arr) = 0 OR ARRAY_LENGTH(ARRAY(SELECT x FROM UNNEST(arr) AS x WHERE pred)) <> 0)"},
		// Snowflake specifics.
		{"snowflake", "REGEXP_REPLACE(subject, pattern, replacement, position)", "REGEXP_REPLACE(subject, pattern, replacement)"},
		{"snowflake", "REGEXP_REPLACE(subject, pattern, replacement, position, occurrence, 'c')", "REGEXP_REPLACE(subject, pattern, replacement)"},
		{"snowflake", "CREATE FUNCTION a() RETURNS TABLE (b INT) AS 'SELECT 1'", "CREATE TABLE FUNCTION a() RETURNS TABLE <b INT64> AS SELECT 1"},
	}
	for _, tc := range cases {
		out, err := sqlglot.Transpile(tc.sql, tc.read, "bigquery", generator.Options{})
		if err != nil || len(out) != 1 || out[0] != tc.want {
			t.Fatalf("%s -> bigquery\n got: %v %v\nwant: %s", tc.sql, out, err, tc.want)
		}
	}
}
