package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestSnowflakeTargetTail pins the Snowflake target mappings added for the corpus.
func TestSnowflakeTargetTail(t *testing.T) {
	cases := []struct{ read, sql, want string }{
		{"redshift", "x ~* 'pat'", "REGEXP_LIKE(x, 'pat', 'i')"},
	}
	for _, tc := range cases {
		out, err := sqlglot.Transpile(tc.sql, tc.read, "snowflake", generator.Options{})
		if err != nil || len(out) != 1 || out[0] != tc.want {
			t.Fatalf("%s -> snowflake\n got: %v %v\nwant: %s", tc.sql, out, err, tc.want)
		}
	}
}
