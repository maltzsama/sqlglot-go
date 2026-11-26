package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestSparkTargetTail pins the Spark target mappings drained from the cross-dialect corpus.
func TestSparkTargetTail(t *testing.T) {
	cases := []struct{ read, sql, want string }{
		// SELECT preprocess: UNNEST becomes LATERAL VIEW EXPLODE / POSEXPLODE / INLINE.
		{"", "SELECT a FROM x CROSS JOIN UNNEST(y) AS t (a)", "SELECT a FROM x LATERAL VIEW EXPLODE(y) t AS a"},
		{"", "SELECT a FROM x CROSS JOIN UNNEST(y) WITH ORDINALITY AS t (a)", "SELECT a FROM x LATERAL VIEW POSEXPLODE(y) t AS pos, a"},
		{"", "SELECT a, b FROM x CROSS JOIN UNNEST(y, z) AS t (a, b)", "SELECT a, b FROM x LATERAL VIEW INLINE(ARRAYS_ZIP(y, z)) t AS a, b"},
		{"bigquery", "SELECT * FROM UNNEST(['7', '14']) AS x", "SELECT * FROM EXPLODE(ARRAY('7', '14')) AS _t0(x)"},
		{"bigquery", "SELECT * FROM tbl CROSS JOIN UNNEST(col) AS ref WITH OFFSET", "SELECT * FROM tbl LATERAL VIEW POSEXPLODE(col) AS offset, ref"},
		// ANY over an array becomes EXISTS with a lambda.
		{"postgres", "WITH t AS (SELECT ARRAY[1, 2, 3] AS col) SELECT * FROM t WHERE 1 <= ANY(col) AND 2 = ANY(col)", "WITH t AS (SELECT ARRAY(1, 2, 3) AS col) SELECT * FROM t WHERE EXISTS(col, x -> 1 <= x) AND EXISTS(col, x -> 2 = x)"},
		// Strings and functions.
		{"", "CONCAT_WS('-', 'a', 'b')", "CASE WHEN '-' IS NULL OR 'a' IS NULL OR 'b' IS NULL THEN NULL ELSE CONCAT_WS('-', 'a', 'b') END"},
		{"bigquery", "MD5(x)", "UNHEX(MD5(x))"},
		{"bigquery", "SELECT GENERATE_UUID()", "SELECT CAST(UUID() AS STRING)"},
		{"", "UUID()", "UUID()"},
		{"snowflake", "TO_ARRAY(x)", "IF(x IS NULL, NULL, ARRAY(x))"},
		{"snowflake", "ARRAY_CONSTRUCT_COMPACT(1, null, 2)", "ARRAY_COMPACT(ARRAY(1, NULL, 2))"},
		{"snowflake", "TRUNC(3.14159, 2)", "CAST(3.14159 AS BIGINT)"},
		{"sqlite", "UNICODE(x)", "ASCII(x)"},
		{"tsql", "SELECT N'test'", "SELECT 'test'"},
		{"oracle", "TO_NUMBER(x)", "CAST(x AS DOUBLE)"},
		{"presto", "TRY(x + 1)", "x + 1"},
		{"presto", `U&'Hello winter \2603 !'`, `'Hello winter \u2603 !'`},
		{"presto", `U&'Hello winter #2603 !' UESCAPE '#'`, `'Hello winter \u2603 !'`},
		// Dates and series.
		{"bigquery", "DATE_ADD(CURRENT_DATE(), INTERVAL -1 DAY)", "DATE_ADD(CURRENT_DATE, -1)"},
		{"bigquery", "SELECT LAST_DAY(CAST('2008-11-25' AS DATE), MONTH)", "SELECT LAST_DAY(CAST('2008-11-25' AS DATE))"},
		{"postgres", "SELECT * FROM GENERATE_SERIES(a, b)", "SELECT * FROM EXPLODE(SEQUENCE(a, b))"},
		{"postgres", "GENERATE_SERIES('2019-01-01'::TIMESTAMP, NOW(), '1day')", "EXPLODE(SEQUENCE(CAST('2019-01-01' AS TIMESTAMP), CAST(CURRENT_TIMESTAMP() AS TIMESTAMP), INTERVAL '1' DAY))"},
		{"snowflake", "SELECT DATE_PART('year', TIMESTAMP '2020-01-01')", "SELECT EXTRACT(year FROM CAST('2020-01-01' AS TIMESTAMP))"},
		{"postgres", "SELECT PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY amount)", "SELECT PERCENTILE_APPROX(amount, 0.5)"},
		// Types, casts and DDL.
		{"tsql", "CAST(x as MONEY)", "CAST(x AS DECIMAL(15, 4))"},
		{"tsql", "TRY_CONVERT(INT, x)", "TRY_CAST(x AS INT)"},
		{"tsql", "TRY_CONVERT(NVARCHAR, x, 121)", "TRY_CAST(DATE_FORMAT(x, 'yyyy-MM-dd HH:mm:ss.SSSSSS') AS VARCHAR(30))"},
		{"oracle", "SELECT CAST(NULL AS VARCHAR2(2328 BYTE)) AS COL1", "SELECT CAST(NULL AS VARCHAR(2328)) AS COL1"},
		{"hive", "ALTER TABLE x CHANGE COLUMN a a VARCHAR(10) COMMENT 'comment'", "ALTER TABLE x ALTER COLUMN a COMMENT 'comment'"},
		{"tsql", "IF OBJECT_ID('tempdb.dbo.#TempTableName') IS NOT NULL BEGIN DROP TABLE #TempTableName; END", "DROP TABLE IF EXISTS TempTableName"},
		// PIVOT IN values keep the source's identified-string output names.
		{"snowflake", "SELECT * FROM produce PIVOT (SUM(produce.sales) FOR produce.quarter IN ('Q1', 'Q2'))", "SELECT * FROM produce PIVOT(SUM(produce.sales) FOR quarter IN ('Q1' AS `'Q1'`, 'Q2' AS `'Q2'`))"},
	}
	for _, tc := range cases {
		out, err := sqlglot.Transpile(tc.sql, tc.read, "spark", generator.Options{})
		if err != nil || len(out) != 1 || out[0] != tc.want {
			t.Fatalf("%s -> spark\n got: %v %v\nwant: %s", tc.sql, out, err, tc.want)
		}
	}
}
