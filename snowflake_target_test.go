package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestSnowflakeTargetTail pins the Snowflake target mappings added for the corpus:
// TIME_STR_TO_TIME casts, UNIFORM seeds, MAP flattening into OBJECT_CONSTRUCT,
// VARIANT probing in ARRAY_CONTAINS, the DATE_FORMAT path, TABLESAMPLE's default
// method, ALTER MODIFY, and the millisecond-to-nanosecond TIMESTAMP_FROM_PARTS.
func TestSnowflakeTargetTail(t *testing.T) {
	cases := []struct{ read, sql, want string }{
		{"", "TIME_STR_TO_TIME('2020-01-01 12:13:14-08:00', 'America/Los_Angeles')", "CAST('2020-01-01 12:13:14-08:00' AS TIMESTAMPTZ)"},
		{"", "TIME_STR_TO_TIME('2020-01-01')", "CAST('2020-01-01' AS TIMESTAMP)"},
		{"databricks", "UNIFORM(1, 10, 5)", "UNIFORM(1, 10, RANDOM(5))"},
		{"hive", "ARRAY_CONTAINS(x, 1)", "ARRAY_CONTAINS(CAST(1 AS VARIANT), x)"},
		{"hive", "MAP(a, b, c, d)", "OBJECT_CONSTRUCT(a, b, c, d)"},
		{"hive", "SELECT * FROM x TABLESAMPLE (1 PERCENT) AS foo", "SELECT * FROM x AS foo TABLESAMPLE (1)"},
		{"mysql", "SELECT DATE_FORMAT('2017-06-15 22:23:34', '%H')", "SELECT TO_CHAR(CAST('2017-06-15 22:23:34' AS TIMESTAMP), 'hh24')"},
		{"mysql", "SELECT DATE_FORMAT('2017-06-15', '%Y-%m-%d')", "SELECT TO_CHAR(CAST('2017-06-15' AS TIMESTAMP), 'yyyy-mm-DD')"},
		{"presto", "MAP(ARRAY(a, b), ARRAY(c, d))", "OBJECT_CONSTRUCT(a, c, b, d)"},
		{"presto", "SELECT CONTAINS(ARRAY['1'], '1')", "SELECT ARRAY_CONTAINS(CAST('1' AS VARIANT), ['1'])"},
		{"snowflake", "ALTER TABLE a MODIFY COLUMN my_column DROP NOT NULL", "ALTER TABLE a ALTER COLUMN my_column DROP NOT NULL"},
		{"spark", "MAP_FROM_ARRAYS(ARRAY(1), c)", "OBJECT_CONSTRUCT([1], c)"},
		{"spark", "TO_DATE(x, 'yyyy-MM-dd')", "TRY_TO_DATE(x, 'yyyy-mm-DD')"},
		{"trino", "SELECT FORMAT('%s', 123)", "SELECT TO_CHAR(123)"},
		{"tsql", "SELECT DATETIMEFROMPARTS(2013, 4, 5, 12, 00, 00, 0)", "SELECT TIMESTAMP_FROM_PARTS(2013, 4, 5, 12, 00, 00, 0 * 1000000)"},
	}
	for _, tc := range cases {
		out, err := sqlglot.Transpile(tc.sql, tc.read, "snowflake", generator.Options{})
		if err != nil || len(out) != 1 || out[0] != tc.want {
			t.Fatalf("%s -> snowflake\n got: %v %v\nwant: %s", tc.sql, out, err, tc.want)
		}
	}
}
