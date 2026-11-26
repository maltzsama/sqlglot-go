package sqlglot_test

import (
	"strings"
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// duckDBEngineCorrection transpiles sql and asserts the generated DuckDB SQL. These cases pin
// the real-engine fixes recorded in DEVIATIONS.md §1.32; the matching corpus expectations live
// in testdata/transpile_corrections.jsonl.
func duckDBEngineCorrection(t *testing.T, read, sql, want string) {
	t.Helper()
	out, err := sqlglot.Transpile(sql, read, "duckdb", generator.Options{})
	if err != nil {
		t.Fatalf("transpile %q: %v", sql, err)
	}
	if len(out) != 1 || out[0] != want {
		t.Fatalf("%s -> duckdb\n got: %v\nwant: %s", sql, out, want)
	}
}

func TestDuckDBEngineCorrections(t *testing.T) {
	t.Run("array_remove_keeps_nulls", func(t *testing.T) {
		duckDBEngineCorrection(t, "snowflake",
			"ARRAY_REMOVE([1, NULL, 2], 1)",
			"LIST_FILTER([1, NULL, 2], _u -> _u IS DISTINCT FROM 1)")
		duckDBEngineCorrection(t, "snowflake",
			"ARRAY_REMOVE([1, 2, 3], NULL)",
			"CASE WHEN NULL IS NULL THEN NULL ELSE LIST_FILTER([1, 2, 3], _u -> _u IS DISTINCT FROM NULL) END")
	})

	t.Run("base64_alphabet_translate", func(t *testing.T) {
		duckDBEngineCorrection(t, "snowflake",
			"BASE64_DECODE_BINARY(x, '-_+')",
			"FROM_BASE64(TRANSLATE(x, '-_+', '+/='))")
		duckDBEngineCorrection(t, "snowflake",
			"BASE64_DECODE_STRING('U25vd2ZsYWtl', '-_+')",
			"DECODE(FROM_BASE64(TRANSLATE('U25vd2ZsYWtl', '-_+', '+/=')))")
	})

	t.Run("months_between_includes_time", func(t *testing.T) {
		duckDBEngineCorrection(t, "spark",
			"SELECT MONTHS_BETWEEN('1997-02-28 10:30:00', '1996-10-30')",
			"SELECT DATE_DIFF('MONTH', CAST('1996-10-30' AS DATE), CAST('1997-02-28 10:30:00' AS DATE)) + CASE WHEN DAY('1997-02-28 10:30:00') = DAY(LAST_DAY('1997-02-28 10:30:00')) AND DAY('1996-10-30') = DAY(LAST_DAY('1996-10-30')) THEN 0 ELSE (DAY('1997-02-28 10:30:00') - DAY('1996-10-30') + DATE_DIFF('MICROSECOND', DATE_TRUNC('DAY', '1997-02-28 10:30:00'), '1997-02-28 10:30:00') / 86400000000 - DATE_DIFF('MICROSECOND', DATE_TRUNC('DAY', '1996-10-30'), '1996-10-30') / 86400000000) / 31.0 END")
	})

	t.Run("timestamp_trunc_subday_zone", func(t *testing.T) {
		duckDBEngineCorrection(t, "bigquery",
			"TIMESTAMP_TRUNC(TIMESTAMP '2024-01-01 12:40:00+00', HOUR, 'Asia/Kolkata')",
			"DATE_TRUNC('HOUR', CAST('2024-01-01 12:40:00+00' AS TIMESTAMPTZ) AT TIME ZONE 'Asia/Kolkata') AT TIME ZONE 'Asia/Kolkata'")
	})

	t.Run("date_zone_uses_instant", func(t *testing.T) {
		duckDBEngineCorrection(t, "bigquery",
			"DATE(CAST('2024-01-01 00:30:00+00' AS TIMESTAMPTZ), 'America/New_York')",
			"CAST(CAST('2024-01-01 00:30:00+00' AS TIMESTAMPTZ) AT TIME ZONE 'America/New_York' AS DATE)")
	})

	t.Run("json_value_array_scalar_only", func(t *testing.T) {
		duckDBEngineCorrection(t, "bigquery",
			"JSON_VALUE_ARRAY('{\"arr\": [{\"a\":1}]}', '$.arr')",
			"CASE WHEN '{\"arr\": [{\"a\":1}]}' -> '$.arr' IS NULL THEN NULL WHEN LENGTH(LIST_FILTER(CAST('{\"arr\": [{\"a\":1}]}' -> '$.arr' AS JSON[]), x -> JSON_TYPE(x) IN ('OBJECT', 'ARRAY'))) = 0 THEN CAST('{\"arr\": [{\"a\":1}]}' -> '$.arr' AS TEXT[]) ELSE NULL END")
	})

	t.Run("safe_parse_json_try_cast", func(t *testing.T) {
		duckDBEngineCorrection(t, "snowflake", "TRY_PARSE_JSON(x)", "TRY_CAST(x AS JSON)")
	})

	t.Run("right_semi_anti_typed_join", func(t *testing.T) {
		out, err := sqlglot.Transpile("SELECT * FROM l RIGHT ANTI JOIN r ON l.id = r.id", "spark", "duckdb", generator.Options{})
		if err != nil {
			t.Fatalf("transpile: %v", err)
		}
		if len(out) != 1 || !strings.Contains(out[0], "JOIN BY (TYPE RIGHT_ANTI)") {
			t.Fatalf("RIGHT ANTI JOIN -> %v, want a JOIN BY (TYPE RIGHT_ANTI) join", out)
		}
	})
}
