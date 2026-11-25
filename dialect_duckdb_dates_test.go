package sqlglot_test

import "testing"

// TestDuckDBDateCasts ports the date-string/date/int cast family from
// generators/duckdb.py:1674,1684,1764,1775.
func TestDuckDBDateCasts(t *testing.T) {
	cases := map[string]string{
		"DATE_STR_TO_DATE(x)":            "CAST(x AS DATE)",
		"TIME_STR_TO_DATE('2020-01-01')": "CAST('2020-01-01' AS DATE)",
		"DATE_TO_DATE_STR(x)":            "CAST(x AS TEXT)",
		"TS_OR_DS_TO_DATE_STR(x)":        "SUBSTRING(CAST(x AS TEXT), 1, 10)",
		"DI_TO_DATE(x)":                  "CAST(STRPTIME(CAST(x AS TEXT), '%Y%m%d') AS DATE)",
		"TS_OR_DI_TO_DI(x)":              "CAST(SUBSTR(REPLACE(CAST(x AS TEXT), '-', ''), 1, 8) AS INT)",
	}
	for sql, want := range cases {
		t.Run(sql, func(t *testing.T) { transpileCase(t, "", "duckdb", sql, want) })
	}
}

// TestDuckDBApproxQuantiles ports approxquantiles_sql (generators/duckdb.py:4686-4720):
// BigQuery's APPROX_QUANTILES(expr, n) becomes APPROX_QUANTILE(expr, [0, 1/n, ..., 1]).
func TestDuckDBApproxQuantiles(t *testing.T) {
	cases := map[string]string{
		"APPROX_QUANTILES(x, 1)":          "APPROX_QUANTILE(x, [0, 1])",
		"APPROX_QUANTILES(x, 2)":          "APPROX_QUANTILE(x, [0, 0.5, 1])",
		"APPROX_QUANTILES(x, 4)":          "APPROX_QUANTILE(x, [0, 0.25, 0.5, 0.75, 1])",
		"APPROX_QUANTILES(DISTINCT x, 2)": "APPROX_QUANTILE(DISTINCT x, [0, 0.5, 1])",
	}
	for sql, want := range cases {
		t.Run(sql, func(t *testing.T) { transpileCase(t, "bigquery", "duckdb", sql, want) })
	}
}
