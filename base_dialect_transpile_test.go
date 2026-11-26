package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestBaseToDialectTail pins the base-source mappings ported for Presto, MySQL and Hive.
func TestBaseToDialectTail(t *testing.T) {
	cases := []struct{ sql, write, want string }{
		{"DATE_STR_TO_DATE(x)", "presto", "CAST(x AS DATE)"},
		{"DATE_TO_DI(x)", "presto", "CAST(DATE_FORMAT(x, '%Y%m%d') AS INT)"},
		{"TIME_STR_TO_TIME('2020-01-01', 'America/Los_Angeles')", "presto", "CAST('2020-01-01' AS TIMESTAMP WITH TIME ZONE)"},
		{"TS_OR_DS_ADD(x, 1, 'DAY')", "presto", "DATE_ADD('DAY', 1, CAST(CAST(x AS TIMESTAMP) AS DATE))"},
		{"UNIX_TO_STR(x, y)", "presto", "DATE_FORMAT(FROM_UNIXTIME(x), y)"},
		{"ARRAY_FILTER(the_array, x -> x > 0)", "presto", "FILTER(the_array, x -> x > 0)"},
		{"CONCAT(a)", "presto", "CAST(a AS VARCHAR)"},
		{"TS_OR_DS_TO_DATE_STR(x)", "presto", "SUBSTR(CAST(x AS VARCHAR), 1, 10)"},

		{"DATE_TO_DI(x)", "hive", "CAST(DATE_FORMAT(x, 'yyyyMMdd') AS INT)"},
		{"DI_TO_DATE(x)", "hive", "TO_DATE(CAST(x AS STRING), 'yyyyMMdd')"},
		{"TS_OR_DI_TO_DI(x)", "hive", "CAST(SUBSTR(REPLACE(CAST(x AS STRING), '-', ''), 1, 8) AS INT)"},
		{"UNIX_TO_TIME_STR(x)", "hive", "FROM_UNIXTIME(x)"},
		{"DATE_SUB('2020-01-01', 1)", "hive", "DATE_ADD('2020-01-01', -1)"},
		{"DATE_SUB('2020-01-01', 2, month)", "hive", "ADD_MONTHS('2020-01-01', -2)"},
		{"CONCAT_WS('-', 'a', 'b')", "hive", "CASE WHEN '-' IS NULL OR 'a' IS NULL OR 'b' IS NULL THEN NULL ELSE CONCAT_WS('-', 'a', 'b') END"},
		{"x ILIKE '%y'", "hive", "LOWER(x) LIKE LOWER('%y')"},
		{"x NOT ILIKE '%y'", "hive", "LOWER(x) NOT LIKE LOWER('%y')"},
		{"VAR_MAP(a, b, c, d)", "hive", "MAP(a, b, c, d)"},
		{"CREATE INDEX my_idx ON tbl(a, b)", "hive", "CREATE INDEX my_idx ON TABLE tbl(a, b)"},

		{"DATE_TRUNC('month', x)", "mysql", "STR_TO_DATE(CONCAT(YEAR(x), ' ', MONTH(x), ' 1'), '%Y %c %e')"},
		{"TS_OR_DS_TO_DATE(x)", "mysql", "DATE(x)"},
		{"TS_OR_DS_TO_DATE(x, '%B')", "mysql", "STR_TO_DATE(x, '%M')"},
		{"DAY(TS_OR_DS_TO_DATE(x))", "mysql", "DAY(x)"},
		{"STR_TO_DATE(x, '%Y-%m-%dT%H:%M:%S')", "mysql", "STR_TO_DATE(x, '%Y-%m-%dT%T')"},
		{"SELECT a IS DISTINCT FROM b", "mysql", "SELECT NOT a <=> b"},
		{"SELECT a IS NOT DISTINCT FROM b", "mysql", "SELECT a <=> b"},
		{"x ILIKE '%y'", "mysql", "LOWER(x) LIKE LOWER('%y')"},
		{"SELECT IS_ASCII(x)", "mysql", "SELECT REGEXP_LIKE(x, '^[[:ascii:]]*$')"},
	}
	for _, tc := range cases {
		out, err := sqlglot.Transpile(tc.sql, "", tc.write, generator.Options{})
		if err != nil || len(out) != 1 || out[0] != tc.want {
			t.Fatalf("%s -> %s\n got: %v %v\nwant: %s", tc.sql, tc.write, out, err, tc.want)
		}
	}
}
