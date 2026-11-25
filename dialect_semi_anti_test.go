package sqlglot_test

import (
	"testing"
)

// TestEliminateSemiAndAntiJoins ports the eliminate_semi_and_anti_joins cases from
// tests/test_transforms.py (v30.17.0), driven through transpile: the write dialect rewrites
// SEMI/ANTI joins into correlated [NOT] EXISTS predicates, including the parenthesization of
// a chained AND. StarRocks overrides the Select preprocess without this transform, so it
// preserves LEFT SEMI/ANTI joins (tests/dialects/test_starrocks.py:30-31).
func TestEliminateSemiAndAntiJoins(t *testing.T) {
	cases := []struct{ read, write, sql, want string }{
		{"duckdb", "bigquery",
			"SELECT t1.id FROM t1 ANTI JOIN t2 ON t1.id = t2.id",
			"SELECT t1.id FROM t1 WHERE NOT EXISTS(SELECT 1 FROM t2 WHERE t1.id = t2.id)"},
		{"duckdb", "bigquery",
			"SELECT t1.id FROM t1 SEMI JOIN t2 ON t1.id = t2.id",
			"SELECT t1.id FROM t1 WHERE EXISTS(SELECT 1 FROM t2 WHERE t1.id = t2.id)"},
		{"duckdb", "bigquery",
			"SELECT t1.id FROM t1 ANTI JOIN t2 ON t1.id = t2.id ANTI JOIN t3 ON t1.id = t3.id",
			"SELECT t1.id FROM t1 WHERE NOT EXISTS(SELECT 1 FROM t2 WHERE t1.id = t2.id) AND NOT EXISTS(SELECT 1 FROM t3 WHERE t1.id = t3.id)"},
		{"duckdb", "bigquery",
			"SELECT t1.id FROM t1 ANTI JOIN t2 ON t1.id = t2.id ANTI JOIN t3 ON t1.id = t3.id ANTI JOIN t4 ON t1.id = t4.id",
			"SELECT t1.id FROM t1 WHERE (NOT EXISTS(SELECT 1 FROM t2 WHERE t1.id = t2.id) AND NOT EXISTS(SELECT 1 FROM t3 WHERE t1.id = t3.id)) AND NOT EXISTS(SELECT 1 FROM t4 WHERE t1.id = t4.id)"},
		{"duckdb", "bigquery",
			"SELECT t1.id FROM t1 SEMI JOIN t2 ON t1.id = t2.id SEMI JOIN t3 ON t1.id = t3.id",
			"SELECT t1.id FROM t1 WHERE EXISTS(SELECT 1 FROM t2 WHERE t1.id = t2.id) AND EXISTS(SELECT 1 FROM t3 WHERE t1.id = t3.id)"},
		{"starrocks", "starrocks",
			"SELECT t1.id FROM t1 LEFT ANTI JOIN t2 ON t1.id = t2.id",
			"SELECT t1.id FROM t1 LEFT ANTI JOIN t2 ON t1.id = t2.id"},
		{"starrocks", "starrocks",
			"SELECT t1.id FROM t1 LEFT SEMI JOIN t2 ON t1.id = t2.id",
			"SELECT t1.id FROM t1 LEFT SEMI JOIN t2 ON t1.id = t2.id"},
	}
	for _, c := range cases {
		t.Run(c.read+"/"+c.sql, func(t *testing.T) { transpileCase(t, c.read, c.write, c.sql, c.want) })
	}
}
