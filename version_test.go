package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// Version clauses (`Table.version`) are ported 1:1 from VERSION_PHRASES
// (parser.py:1738-1745) + _parse_version (parser.py:5120-5146). These cases mirror the
// upstream dialect tests so the clauses stay round-trip-stable and transpile correctly.
//
// Sources: tests/dialects/test_bigquery.py:216-220, test_hive.py:524,
// test_presto.py:795,798,1207, test_spark.py:1162-1168, test_dremio.py:78-100.

// TestVersionIdentity ports the upstream identity cases: FOR SYSTEM_TIME / SYSTEM TIME,
// hive TIMESTAMP AS OF, presto FOR VERSION/TIMESTAMP AS OF, spark/databricks
// TIMESTAMP AS OF and dremio AT SNAPSHOT / AT TIMESTAMP.
func TestVersionIdentity(t *testing.T) {
	cases := []struct {
		dialect, sql, want string
	}{
		{"bigquery", "SELECT * FROM tbl FOR SYSTEM_TIME AS OF z", ""},
		// The compound keyword fell away in v30.17.0; _parse_version normalizes it.
		{"bigquery", "SELECT * FROM tbl FOR SYSTEM TIME AS OF z", "SELECT * FROM tbl FOR SYSTEM_TIME AS OF z"},
		{"hive", "SELECT * FROM my_table TIMESTAMP AS OF DATE_ADD(CURRENT_DATE, -1)", ""},
		{"presto", "SELECT * FROM example.testdb.customer_orders FOR VERSION AS OF 8954597067493422955", ""},
		{"presto", "SELECT * FROM example.testdb.customer_orders FOR TIMESTAMP AS OF CAST('2022-03-23 09:59:29.803 Europe/Vienna' AS TIMESTAMP)", ""},
		{"presto", "SELECT * FROM foo FOR TIMESTAMP AS OF CAST('2020-01-01 00:00:00' AS TIMESTAMP) AS bar", ""},
		{"spark", "SELECT * FROM foo TIMESTAMP AS OF '2020-01-01 00:00:00' AS bar", ""},
		{"databricks", "SELECT * FROM foo TIMESTAMP AS OF '2020-01-01 00:00:00' AS bar", ""},
		{"dremio", "SELECT * FROM t AT SNAPSHOT '4132119532727284872'", ""},
		{"dremio", "SELECT * FROM t AT TIMESTAMP '2024-01-01 12:00:00.000'", ""},
		{"dremio", "SELECT * FROM t AT SNAPSHOT '4132119532727284872' AS a", ""},
		{"dremio", "SELECT * FROM t AT TIMESTAMP '2024-01-01'", ""},
	}
	for _, c := range cases {
		want := c.want
		if want == "" {
			want = c.sql
		}
		assertRoundTrip(t, c.dialect, c.sql, want)
	}
}

// TestVersionTranspile ports the upstream validate_all read/write pairs: dremio's
// AT SNAPSHOT / AT TIMESTAMP against spark's VERSION/TIMESTAMP AS OF and trino's
// FOR VERSION/TIMESTAMP AS OF.
func TestVersionTranspile(t *testing.T) {
	cases := []struct {
		sql, read, write, want string
	}{
		{"SELECT * FROM t VERSION AS OF '123'", "spark", "dremio", "SELECT * FROM t AT SNAPSHOT '123'"},
		{"SELECT * FROM t FOR VERSION AS OF '123'", "trino", "dremio", "SELECT * FROM t AT SNAPSHOT '123'"},
		{"SELECT * FROM t AT SNAPSHOT '123'", "dremio", "spark", "SELECT * FROM t VERSION AS OF '123'"},
		{"SELECT * FROM t AT SNAPSHOT '123'", "dremio", "trino", "SELECT * FROM t FOR VERSION AS OF '123'"},
		{"SELECT * FROM t TIMESTAMP AS OF '2024-01-01'", "spark", "dremio", "SELECT * FROM t AT TIMESTAMP '2024-01-01'"},
		{"SELECT * FROM t AT TIMESTAMP '2024-01-01'", "dremio", "spark", "SELECT * FROM t TIMESTAMP AS OF '2024-01-01'"},
		{"SELECT * FROM foo TIMESTAMP AS OF '2020-01-01 00:00:00' AS bar", "databricks", "spark", "SELECT * FROM foo TIMESTAMP AS OF '2020-01-01 00:00:00' AS bar"},
	}
	for _, c := range cases {
		got, err := sqlglot.Transpile(c.sql, c.read, c.write, generator.Options{})
		if err != nil {
			t.Fatalf("Transpile(%q, %q, %q): %v", c.sql, c.read, c.write, err)
		}
		if got[0] != c.want {
			t.Errorf("%s -> %s %q = %q, want %q", c.read, c.write, c.sql, got[0], c.want)
		}
	}
}
