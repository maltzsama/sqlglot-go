package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestPrestoSourceTail pins the Presto to Hive/Spark mappings.
func TestPrestoSourceTail(t *testing.T) {
	cases := []struct{ write, sql, want string }{
		{"hive", "MAP(ARRAY[a, b], ARRAY[c, d])", "MAP(a, c, b, d)"},
		{"hive", "DATE_DIFF('hour', x, y)", "(UNIX_TIMESTAMP(y) - UNIX_TIMESTAMP(x)) / 3600"},
		{"hive", "DATE_DIFF('millisecond', x, y)", "(UNIX_TIMESTAMP(y) - UNIX_TIMESTAMP(x)) * 1000"},
		{"hive", "ARRAY_JOIN(x, '-', 'a')", "CONCAT_WS('-', x)"},
		{"hive", "SELECT ARRAY_SORT(x, (left, right) -> -1)", "SELECT SORT_ARRAY(x)"},

		{"spark", "MAP(ARRAY(a, b), ARRAY(c, d))", "MAP_FROM_ARRAYS(ARRAY(a, b), ARRAY(c, d))"},
		{"spark", "MAP(ARRAY('a'), ARRAY('b'))", "MAP_FROM_ARRAYS(ARRAY('a'), ARRAY('b'))"},
		{"spark", "TO_HEX(x)", "HEX(x)"},
		{"spark", "IS_NAN(x)", "ISNAN(x)"},
		{"spark", "SELECT FROM_ISO8601_DATE('2020-05-11')", "SELECT CAST('2020-05-11' AS DATE)"},
		{"spark", "SELECT FROM_ISO8601_TIMESTAMP('2020-05-11T11:15:05')", "SELECT CAST('2020-05-11T11:15:05' AS TIMESTAMP)"},
		{"spark", "SELECT MIN_BY(a.id, a.timestamp, 3) FROM a", "SELECT MIN_BY(a.id, a.timestamp) FROM a"},
	}
	for _, tc := range cases {
		out, err := sqlglot.Transpile(tc.sql, "presto", tc.write, generator.Options{})
		if err != nil || len(out) != 1 || out[0] != tc.want {
			t.Fatalf("%s -> %s\n got: %v %v\nwant: %s", tc.sql, tc.write, out, err, tc.want)
		}
	}
}
