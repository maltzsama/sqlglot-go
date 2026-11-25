package sqlglot_test

import "testing"

// TestStrPosition ports the STR_POSITION transpilation from tests/dialects/test_dialect.py
// (v30.17.0, test_operators): dialects keep STRPOS/POSITION and transpile the position
// argument into a SUBSTRING-wrapped call plus an offset, guarded by a zero check.
func TestStrPosition(t *testing.T) {
	twoArg := map[string]string{
		"athena":      "STRPOS(haystack, needle)",
		"presto":      "STRPOS(haystack, needle)",
		"trino":       "STRPOS(haystack, needle)",
		"duckdb":      "STRPOS(haystack, needle)",
		"postgres":    "POSITION(needle IN haystack)",
		"redshift":    "POSITION(needle IN haystack)",
		"materialize": "POSITION(needle IN haystack)",
		"risingwave":  "POSITION(needle IN haystack)",
	}
	for write, want := range twoArg {
		t.Run("2arg/"+write, func(t *testing.T) {
			transpileCase(t, "", write, "STR_POSITION(haystack, needle)", want)
		})
	}

	withPosition := map[string]string{
		"athena":   "IF(STRPOS(SUBSTR(haystack, position), needle) = 0, 0, STRPOS(SUBSTR(haystack, position), needle) + position - 1)",
		"presto":   "IF(STRPOS(SUBSTR(haystack, position), needle) = 0, 0, STRPOS(SUBSTR(haystack, position), needle) + position - 1)",
		"trino":    "IF(STRPOS(SUBSTR(haystack, position), needle) = 0, 0, STRPOS(SUBSTR(haystack, position), needle) + position - 1)",
		"duckdb":   "CASE WHEN STRPOS(SUBSTRING(haystack, position), needle) = 0 THEN 0 ELSE STRPOS(SUBSTRING(haystack, position), needle) + position - 1 END",
		"postgres": "CASE WHEN POSITION(needle IN SUBSTRING(haystack FROM position)) = 0 THEN 0 ELSE POSITION(needle IN SUBSTRING(haystack FROM position)) + position - 1 END",
	}
	for write, want := range withPosition {
		t.Run("pos/"+write, func(t *testing.T) {
			transpileCase(t, "", write, "STR_POSITION(haystack, needle, position)", want)
		})
	}
}
