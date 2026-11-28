package sqlglot_test

import "testing"

// Teradata DELETE and EXECUTE statements (parsers/teradata.py:193-212, 268-275; generators/
// teradata.py:116-119). DEL and EXEC are the Teradata keywords for DELETE and EXECUTE. Expected
// outputs are the pinned upstream v30.35.0 output.
func TestTeradataDeleteAndExecute(t *testing.T) {
	cases := []struct{ sql, want string }{
		{"DELETE t ALL", "DELETE t"},
		{"DEL FROM db.t ALL", "DELETE FROM db.t"},
		{`DELETE FROM t AS "ALL" ALL`, `DELETE FROM t AS "ALL"`},
		{"DELETE FROM t AS d ALL", "DELETE FROM t AS d"},
		{"EXEC example_macro", "EXECUTE example_macro"},
		{"EXEC example_db.example_macro(1)", "EXECUTE example_db.example_macro(1)"},
		{"EXEC example_macro()", "EXECUTE example_macro"},
		{"EXEC example_macro(1,, 3)", "EXEC example_macro(1,, 3)"},
		{"EXECUTE example_macro(DATE '2024-01-02', COALESCE(NULL, 3), 'a;b')", "EXECUTE example_macro(CAST('2024-01-02' AS DATE), COALESCE(NULL, 3), 'a;b')"},
		{"EXECUTE example_macro(arg_a=1, arg_b='value')", "EXECUTE example_macro(arg_a = 1, arg_b = 'value')"},
	}
	for _, tc := range cases {
		transpileCase(t, "teradata", "teradata", tc.sql, tc.want)
	}
}
