package sqlglot_test

import "testing"

// Teradata character hex literals: a quoted hex string followed by XC or XCV is a HexString with
// is_character (parsers/teradata.py:118-147, generators/teradata.py:105-111). Expected outputs
// are the pinned upstream v30.35.0 output.
func TestTeradataCharacterHexLiteral(t *testing.T) {
	cases := []struct{ sql, want string }{
		{"SELECT '0A'XC AS s", "SELECT '0A'XC AS s"},
		{"SELECT '0A'XCV AS s", "SELECT '0A'XC AS s"},
		{"COALESCE('0A'XC, NULL)", "COALESCE('0A'XC, NULL)"},
		{"CAST('0A'XC AS VARCHAR(10))", "CAST('0A'XC AS VARCHAR(10))"},
		{"SELECT CASE WHEN a = 1 THEN '41'XC ELSE '42'XC END FROM t", "SELECT CASE WHEN a = 1 THEN '41'XC ELSE '42'XC END FROM t"},
		{"SELECT ''XC AS s", "SELECT ''XC AS s"},
	}
	for _, tc := range cases {
		transpileCase(t, "teradata", "teradata", tc.sql, tc.want)
	}
}
