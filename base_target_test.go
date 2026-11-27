package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestBaseTargetTail pins mappings that render generic SQL in the base dialect.
func TestBaseTargetTail(t *testing.T) {
	cases := []struct{ read, sql, want string }{
		{"snowflake", "DATE_TRUNC('day', x)", "TIMESTAMP_TRUNC(x, DAY)"},
		{"snowflake", "DATE_TRUNC(year, x)", "TIMESTAMP_TRUNC(x, YEAR)"},
		{"bigquery", "TIMESTAMP_TRUNC(x, DAY, 'UTC')", "TIMESTAMP_TRUNC(x, DAY, 'UTC')"},
		{"clickhouse", "SELECT LEFTPAD('bar', 5)", "SELECT LPAD('bar', 5)"},
		{"clickhouse", "SELECT RIGHTPAD('bar', 5, ' ')", "SELECT RPAD('bar', 5, ' ')"},
		{"clickhouse", "JSONExtractString(x, 'y')", "JSON_EXTRACT_SCALAR(x, '$.y')"},
		{"clickhouse", "JSONExtractString(x, 'y', 1, 'z')", "JSON_EXTRACT_SCALAR(x, '$.y[0].z')"},
		{"clickhouse", "farmFingerprint64(x)", "FARM_FINGERPRINT(x)"},
		{"redshift", "FARMFINGERPRINT64(x)", "FARM_FINGERPRINT(x)"},
		{"postgres", "ARRAY_PREPEND(x, arr)", "ARRAY_PREPEND(arr, x)"},
		{"duckdb", "LIST_PREPEND(x, arr)", "ARRAY_PREPEND(arr, x)"},
		{"postgres", "GEN_RANDOM_UUID()", "UUID()"},
		{"bigquery", "JSON_STRIP_NULLS(PARSE_JSON('[{\"f1\":1}]'))", "JSON_STRIP_NULLS(PARSE_JSON('[{\"f1\":1}]'))"},
		{"snowflake", "OBJECT_CONSTRUCT('a', b, 'c', d)", "STRUCT(b AS a, d AS c)"},
	}
	for _, tc := range cases {
		out, err := sqlglot.Transpile(tc.sql, tc.read, "", generator.Options{})
		if err != nil || len(out) != 1 || out[0] != tc.want {
			t.Fatalf("%s -> base\n got: %v %v\nwant: %s", tc.sql, out, err, tc.want)
		}
	}
}
