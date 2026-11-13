package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestDremioTypeMappings ports test_type_mappings (tests/dialects/test_dremio.py:10-37).
func TestDremioTypeMappings(t *testing.T) {
	cases := map[string]string{
		"CAST(x AS SMALLINT)":     "CAST(x AS INT)",
		"CAST(x AS TINYINT)":      "CAST(x AS INT)",
		"CAST(x AS BINARY)":       "CAST(x AS VARBINARY)",
		"CAST(x AS TEXT)":         "CAST(x AS VARCHAR)",
		"CAST(x AS NCHAR)":        "CAST(x AS VARCHAR)",
		"CAST(x AS CHAR)":         "CAST(x AS VARCHAR)",
		"CAST(x AS TIMESTAMPNTZ)": "CAST(x AS TIMESTAMP)",
		"CAST(x AS DATETIME)":     "CAST(x AS TIMESTAMP)",
		"CAST(x AS ARRAY)":        "CAST(x AS LIST)",
		"CAST(x AS BIT)":          "CAST(x AS BOOLEAN)",
	}
	for sql, want := range cases {
		assertRoundTrip(t, "dremio", sql, want)
	}
}

// TestDremioNullOrdering ports test_null_ordering (tests/dialects/test_dremio.py:60-75):
// NULLS LAST is the default and is dropped.
func TestDremioNullOrdering(t *testing.T) {
	assertRoundTrip(t, "dremio", "SELECT * FROM t ORDER BY a NULLS LAST", "SELECT * FROM t ORDER BY a")
	assertRoundTrip(t, "dremio", "SELECT * FROM t ORDER BY a DESC NULLS LAST", "SELECT * FROM t ORDER BY a DESC")
	assertRoundTrip(t, "dremio", "SELECT * FROM t ORDER BY a NULLS FIRST", "SELECT * FROM t ORDER BY a NULLS FIRST")
	assertRoundTrip(t, "dremio", "SELECT * FROM t ORDER BY a DESC NULLS FIRST", "SELECT * FROM t ORDER BY a DESC NULLS FIRST")
}

// TestDremioTimeTravel ports test_time_travel (tests/dialects/test_dremio.py:77-108).
func TestDremioTimeTravel(t *testing.T) {
	assertRoundTrip(t, "dremio", "SELECT * FROM t AT SNAPSHOT '4132119532727284872'", "SELECT * FROM t AT SNAPSHOT '4132119532727284872'")
	assertRoundTrip(t, "dremio", "SELECT * FROM t AT TIMESTAMP '2024-01-01 12:00:00.000'", "SELECT * FROM t AT TIMESTAMP '2024-01-01 12:00:00.000'")
	assertRoundTrip(t, "dremio", "SELECT * FROM t AT SNAPSHOT '4132119532727284872' AS a", "SELECT * FROM t AT SNAPSHOT '4132119532727284872' AS a")
	// Other dialects' time-travel spellings transpile into the AT form.
	got, err := sqlglot.Transpile("SELECT * FROM t VERSION AS OF '123'", "spark", "dremio", generator.Options{})
	if err != nil || got[0] != "SELECT * FROM t AT SNAPSHOT '123'" {
		t.Fatalf("spark VERSION AS OF -> dremio = %q, err %v", got, err)
	}
	got, err = sqlglot.Transpile("SELECT * FROM t TIMESTAMP AS OF '2024-01-01'", "spark", "dremio", generator.Options{})
	if err != nil || got[0] != "SELECT * FROM t AT TIMESTAMP '2024-01-01'" {
		t.Fatalf("spark TIMESTAMP AS OF -> dremio = %q, err %v", got, err)
	}
}

// TestDremioConcatAndTimezone ports test_concat_coalesce and test_convert_timezone
// (tests/dialects/test_dremio.py:38-45, 110-117).
func TestDremioConcatAndTimezone(t *testing.T) {
	assertRoundTrip(t, "dremio", "SELECT CONCAT('a', NULL)", "SELECT CONCAT('a', NULL)")
	// Base coalesces NULL to '' unless the dialect sets CONCAT_COALESCE.
	got, err := sqlglot.Transpile("SELECT CONCAT('a', NULL)", "dremio", "", generator.Options{})
	if err != nil || got[0] != "SELECT CONCAT('a', COALESCE(NULL, ''))" {
		t.Fatalf("dremio CONCAT -> base = %q, err %v", got, err)
	}
	got, err = sqlglot.Transpile("SELECT CONVERT_TIMEZONE('America/Chicago', DateColumn)", "dremio", "dremio", generator.Options{})
	if err != nil || got[0] != "SELECT CONVERT_TIMEZONE('America/Chicago', DateColumn)" {
		t.Fatalf("dremio CONVERT_TIMEZONE = %q, err %v", got, err)
	}
	got, err = sqlglot.Transpile("SELECT CONVERT_TIMEZONE('America/Chicago', DateColumn)", "dremio", "", generator.Options{})
	if err != nil || got[0] != "SELECT DateColumn AT TIME ZONE 'America/Chicago'" {
		t.Fatalf("dremio CONVERT_TIMEZONE -> base = %q, err %v", got, err)
	}
}

// TestDremioFlags ports test_interval_plural, test_limit_only_literals and
// test_multi_arg_distinct_unsupported (tests/dialects/test_dremio.py:119-129).
func TestDremioFlags(t *testing.T) {
	assertRoundTrip(t, "dremio", "INTERVAL '7' DAYS", "INTERVAL '7' DAY")
	assertRoundTrip(t, "dremio", "SELECT * FROM t LIMIT 1 + 1", "SELECT * FROM t LIMIT 2")
	assertRoundTrip(t, "dremio", "SELECT COUNT(DISTINCT a, b) FROM t",
		"SELECT COUNT(DISTINCT CASE WHEN a IS NULL THEN NULL WHEN b IS NULL THEN NULL ELSE (a, b) END) FROM t")
}

// TestDremioTimeMapping ports the TIME_MAPPING directions from test_time_mapping
// (tests/dialects/test_dremio.py:131-164).
func TestDremioTimeMapping(t *testing.T) {
	ts := "CAST('2025-06-24 12:34:56' AS TIMESTAMP)"
	assertRoundTrip(t, "dremio", "SELECT TO_CHAR("+ts+", 'yyyy-mm-dd hh24:mi:ss')",
		"SELECT TO_CHAR("+ts+", 'yyyy-mm-dd hh24:mi:ss')")
	assertRoundTrip(t, "dremio", "SELECT TO_CHAR("+ts+", 'yy-ddd hh24:mi:ss.fff tzd')",
		"SELECT TO_CHAR("+ts+", 'yy-ddd hh24:mi:ss.fff tzd')")
	// Postgres patterns convert through the shared mapping.
	got, err := sqlglot.Transpile("SELECT TO_CHAR("+ts+", 'YYYY-MM-DD HH24:MI:SS')", "postgres", "dremio", generator.Options{})
	if err != nil || got[0] != "SELECT TO_CHAR("+ts+", 'yyyy-mm-dd hh24:mi:ss')" {
		t.Fatalf("postgres TO_CHAR -> dremio = %q, err %v", got, err)
	}
}

// TestDremioToCharSpecial ports test_to_char_special (tests/dialects/test_dremio.py:166-188):
// only a literal format containing '#' marks the ToChar as numeric.
func TestDremioToCharSpecial(t *testing.T) {
	numeric := []string{"TO_CHAR(5555, '#')", "TO_CHAR(3.14, '#.#')", "TO_CHAR(columnname, '#.##')"}
	for _, sql := range numeric {
		expr, err := sqlglot.ParseOne(sql, "dremio")
		if err != nil {
			t.Fatalf("parse %q: %v", sql, err)
		}
		node := expr
		if len(expr.Expressions()) > 0 {
			node = expr.Expressions()[0]
		}
		if node.Kind() != expressions.KindToChar {
			t.Fatalf("%q parsed to %s, want ToChar", sql, node.ToS())
		}
		if isNumeric, _ := node.Arg("is_numeric").(bool); !isNumeric {
			t.Fatalf("%q: expected is_numeric, got %s", sql, node.ToS())
		}
	}
	for _, sql := range []string{"TO_CHAR(5555)", "TO_CHAR(3.14, columnname)", "TO_CHAR(123, 'abcd')", "TO_CHAR(3.14, UPPER('abcd'))"} {
		expr, err := sqlglot.ParseOne(sql, "dremio")
		if err != nil {
			t.Fatalf("parse %q: %v", sql, err)
		}
		node := expr
		if len(expr.Expressions()) > 0 {
			node = expr.Expressions()[0]
		}
		if isNumeric, _ := node.Arg("is_numeric").(bool); isNumeric {
			t.Fatalf("%q: expected a non-numeric ToChar", sql)
		}
	}
}

// TestDremioFunctions ports test_date_add/sub, test_datetime_parsing,
// test_array_generate_range, test_current_date_utc, test_repeatstr, test_regexp_like,
// test_date_part, test_datetype and test_try_cast (tests/dialects/test_dremio.py:190-261).
func TestDremioFunctions(t *testing.T) {
	assertRoundTrip(t, "dremio", "SELECT DATE_ADD(col, 1)", "SELECT DATE_ADD(col, 1)")
	assertRoundTrip(t, "dremio", "SELECT DATE_ADD(col, CAST(1 AS INTERVAL HOUR))", "SELECT DATE_ADD(col, CAST(1 AS INTERVAL HOUR))")
	assertRoundTrip(t, "dremio", "SELECT DATE_ADD(TIMESTAMP '2022-01-01 12:00:00', CAST(-1 AS INTERVAL HOUR))",
		"SELECT DATE_ADD(CAST('2022-01-01 12:00:00' AS TIMESTAMP), CAST(-1 AS INTERVAL HOUR))")
	assertRoundTrip(t, "dremio", "SELECT DATE_SUB(col, 1)", "SELECT DATE_SUB(col, 1)")
	assertRoundTrip(t, "dremio", "SELECT DATE_SUB(col, CAST(1 AS INTERVAL HOUR))", "SELECT DATE_SUB(col, CAST(1 AS INTERVAL HOUR))")
	assertRoundTrip(t, "dremio", "SELECT DATE_FORMAT(CAST('2025-08-18 15:30:00' AS TIMESTAMP), 'yyyy-mm-dd')",
		"SELECT TO_CHAR(CAST('2025-08-18 15:30:00' AS TIMESTAMP), 'yyyy-mm-dd')")
	assertRoundTrip(t, "dremio", "ARRAY_GENERATE_RANGE(1, 4)", "ARRAY_GENERATE_RANGE(1, 4)")
	got, err := sqlglot.Transpile("ARRAY_GENERATE_RANGE(1, 4)", "dremio", "duckdb", generator.Options{})
	if err != nil || got[0] != "GENERATE_SERIES(1, 4)" {
		t.Fatalf("ARRAY_GENERATE_RANGE -> duckdb = %q, err %v", got, err)
	}
	assertRoundTrip(t, "dremio", "SELECT CURRENT_DATE_UTC", "SELECT CURRENT_DATE_UTC")
	assertRoundTrip(t, "dremio", "SELECT CURRENT_DATE_UTC()", "SELECT CURRENT_DATE_UTC")
	assertRoundTrip(t, "dremio", "SELECT REPEAT(x, 5)", "SELECT REPEAT(x, 5)")
	assertRoundTrip(t, "dremio", "SELECT REPEATSTR(x, 5)", "SELECT REPEAT(x, 5)")
	assertRoundTrip(t, "dremio", "REGEXP_MATCHES(x, y)", "REGEXP_LIKE(x, y)")
	assertRoundTrip(t, "dremio", "SELECT DATE_PART('YEAR', date '2021-04-01')", "SELECT EXTRACT('YEAR' FROM CAST('2021-04-01' AS DATE))")
	assertRoundTrip(t, "dremio", "DATETYPE(2024,2,2)", "DATE('2024-02-02')")
	assertRoundTrip(t, "dremio", "DATETYPE(x,y,z)", "CAST(CONCAT(x, '-', y, '-', z) AS DATE)")
	// Dremio has no TRY_CAST: a base TRY_CAST renders as CAST.
	got, err = sqlglot.Transpile("TRY_CAST(a AS FLOAT)", "", "dremio", generator.Options{})
	if err != nil || got[0] != "CAST(a AS FLOAT)" {
		t.Fatalf("TRY_CAST -> dremio = %q, err %v", got, err)
	}
}
