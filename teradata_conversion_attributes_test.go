package sqlglot_test

import "testing"

// Teradata conversion attributes (parsers/teradata.py:118-527): TYPE, FORMAT, TITLE and NAMED
// lists after an expression, the table-function form TABLE(db.f(...)), and the nested conversions
// they produce. Expected outputs are the pinned upstream v30.35.0 output.
func TestTeradataConversionAttributes(t *testing.T) {
	cases := []struct{ sql, want string }{
		{"SELECT x (NAMED result) AS a FROM t", "SELECT x AS a FROM t"},
		{"SELECT x (NAMED result) FROM t", "SELECT x AS result FROM t"},
		{"SELECT x (TITLE 'Heading') FROM t", "SELECT x (TITLE 'Heading') FROM t"},
		{"SELECT x (FORMAT '999') (NAMED result) (TITLE 'Heading') AS a FROM t", "SELECT x (FORMAT '999') (TITLE 'Heading') AS a FROM t"},
		{"SELECT amount (NAMED r, DECIMAL(12, 2), TITLE 'Revenue') FROM t", "SELECT CAST(amount AS DECIMAL(12, 2)) (TITLE 'Revenue') AS r FROM t"},
		{"SUM(amount) (DECIMAL(12, 2)) (BIGINT)", "CAST(CAST(SUM(amount) AS DECIMAL(12, 2)) AS BIGINT)"},
		{"date_text (FORMAT 'YYYYMMDD', DATE)", "date_text (DATE, FORMAT 'YYYYMMDD')"},
		{"SELECT t.d (FORMAT 'YYYY-MM-DD') (CHAR(10)) FROM t", "SELECT t.d (FORMAT 'YYYY-MM-DD') (CHAR(10)) FROM t"},
		{"SELECT UPPER(TITLE(x (TITLE 'Heading'))) FROM t", "SELECT UPPER(TITLE(x (TITLE 'Heading'))) FROM t"},
		{"SELECT x FROM t WHERE x (NAMED result) > 1", "SELECT x FROM t WHERE x (NAMED result) > 1"},
		{"SELECT * FROM TABLE(db.my_udf(DATE)) AS t", "SELECT * FROM TABLE(db.my_udf(DATE)) AS t"},
		{"SELECT * FROM TABLE(my_udf(DATE)) AS t", "SELECT * FROM TABLE(MY_UDF(DATE)) AS t"},
		{"SELECT * FROM TABLE(db.CURDATE()) AS t", "SELECT * FROM TABLE(db.CURDATE()) AS t"},
	}
	for _, tc := range cases {
		transpileCase(t, "teradata", "teradata", tc.sql, tc.want)
	}
}
