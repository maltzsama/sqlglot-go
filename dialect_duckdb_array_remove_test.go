package sqlglot_test

import "testing"

// TestDuckDBArrayRemove ports remove_from_array_using_filter (dialects/dialect.py:2319):
// ARRAY_REMOVE(a, x) becomes LIST_FILTER(a, _u -> _u <> x). The base ARRAY_REMOVE function
// name also maps to the ArrayRemove node now.
func TestDuckDBArrayRemove(t *testing.T) {
	transpileCase(t, "", "duckdb",
		"ARRAY_REMOVE(the_array, target)",
		"LIST_FILTER(the_array, _u -> _u <> target)")
}
