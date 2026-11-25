package sqlglot_test

import "testing"

// TestHexCaseCollapse ports build_upper/build_lower (parser.py:98-107): UPPER over a
// Hex/LowerHex collapses to Hex (and the bigquery Hex override renders UPPER(TO_HEX(...))),
// so UPPER(TO_HEX(x)) transpiles to the plain hex form.
func TestHexCaseCollapse(t *testing.T) {
	// UPPER(TO_HEX(x)) under bigquery is exactly Hex(x).
	cases := map[string]string{
		"":           "HEX(x)",
		"duckdb":     "HEX(x)",
		"clickhouse": "HEX(x)",
		"hive":       "HEX(x)",
		"mysql":      "HEX(x)",
		"presto":     "TO_HEX(x)",
		"trino":      "TO_HEX(x)",
		"spark":      "HEX(x)",
		"sqlite":     "HEX(x)",
		"bigquery":   "UPPER(TO_HEX(x))",
	}
	for write, want := range cases {
		t.Run(write, func(t *testing.T) {
			transpileCase(t, "bigquery", write, "UPPER(TO_HEX(x))", want)
		})
	}
}
