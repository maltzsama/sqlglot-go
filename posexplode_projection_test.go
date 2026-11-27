package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestPosexplodeProjectionUnnest pins the explode_projection_to_unnest Posexplode branch for
// the Presto and BigQuery targets.
func TestPosexplodeProjectionUnnest(t *testing.T) {
	cases := []struct{ dialect, sql, want string }{
		{"presto", "SELECT POSEXPLODE(ARRAY(2, 3)) AS x", "SELECT IF(_u.pos = _u_2.pos_2, _u_2.x) AS x, IF(_u.pos = _u_2.pos_2, _u_2.pos_2) AS pos_2 FROM UNNEST(SEQUENCE(1, GREATEST(CARDINALITY(ARRAY[2, 3])))) AS _u(pos) CROSS JOIN UNNEST(ARRAY[2, 3]) WITH ORDINALITY AS _u_2(x, pos_2) WHERE _u.pos = _u_2.pos_2 OR (_u.pos > CARDINALITY(ARRAY[2, 3]) AND _u_2.pos_2 = CARDINALITY(ARRAY[2, 3]))"},
		{"presto", "SELECT POSEXPLODE(x) AS (a, b)", "SELECT IF(_u.pos = _u_2.a, _u_2.b) AS b, IF(_u.pos = _u_2.a, _u_2.a) AS a FROM UNNEST(SEQUENCE(1, GREATEST(CARDINALITY(x)))) AS _u(pos) CROSS JOIN UNNEST(x) WITH ORDINALITY AS _u_2(b, a) WHERE _u.pos = _u_2.a OR (_u.pos > CARDINALITY(x) AND _u_2.a = CARDINALITY(x))"},
		{"bigquery", "SELECT POSEXPLODE(ARRAY(2, 3)) AS x", "SELECT IF(pos = pos_2, x, NULL) AS x, IF(pos = pos_2, pos_2, NULL) AS pos_2 FROM UNNEST(GENERATE_ARRAY(0, GREATEST(ARRAY_LENGTH([2, 3])) - 1)) AS pos CROSS JOIN UNNEST([2, 3]) AS x WITH OFFSET AS pos_2 WHERE pos = pos_2 OR (pos > (ARRAY_LENGTH([2, 3]) - 1) AND pos_2 = (ARRAY_LENGTH([2, 3]) - 1))"},
	}
	for _, tc := range cases {
		out, err := sqlglot.Transpile(tc.sql, "spark", tc.dialect, generator.Options{})
		if err != nil || len(out) != 1 || out[0] != tc.want {
			t.Fatalf("%s -> %s\n got: %v %v\nwant: %s", tc.sql, tc.dialect, out, err, tc.want)
		}
	}
}
