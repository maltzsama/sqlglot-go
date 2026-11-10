package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestRisingWaveIdentity is the same-dialect round-trip oracle for RisingWave,
// ported from .reference/sqlglot-v30.17.0/tests/dialects/test_risingwave.py. Cases are
// already in their canonical RisingWave spelling; MAP<...> generating MAP(...) is
// upstream's own expected rewrite, not a gap.
func TestRisingWaveIdentity(t *testing.T) {
	cases := []struct {
		sql      string
		want     string
		pretty   bool
		identify bool
	}{
		{sql: `CREATE SOURCE from_kafka (*, gen_i32_field INT AS int32_field + 2, gen_i64_field INT AS int64_field + 2, WATERMARK FOR time_col AS time_col - INTERVAL '5 SECOND') INCLUDE header foo VARCHAR AS myheader INCLUDE key AS mykey WITH (connector='kafka', topic='my_topic') FORMAT PLAIN ENCODE PROTOBUF (A=1, B=2) KEY ENCODE PROTOBUF (A=3, B=4)`, want: `CREATE SOURCE from_kafka (*, gen_i32_field INT AS int32_field + 2, gen_i64_field INT AS int64_field + 2, WATERMARK FOR time_col AS time_col - INTERVAL '5 SECOND') INCLUDE header foo VARCHAR AS myheader INCLUDE key AS mykey WITH (connector='kafka', topic='my_topic') FORMAT PLAIN ENCODE PROTOBUF (A=1, B=2) KEY ENCODE PROTOBUF (A=3, B=4)`},
		{sql: `CREATE SINK my_sink AS SELECT * FROM A WITH (connector='kafka', topic='my_topic') FORMAT PLAIN ENCODE PROTOBUF (A=1, B=2) KEY ENCODE PROTOBUF (A=3, B=4)`, want: `CREATE SINK my_sink AS SELECT * FROM A WITH (connector='kafka', topic='my_topic') FORMAT PLAIN ENCODE PROTOBUF (A=1, B=2) KEY ENCODE PROTOBUF (A=3, B=4)`},
		{sql: `WITH t1 AS MATERIALIZED (SELECT 1), t2 AS NOT MATERIALIZED (SELECT 2) SELECT * FROM t1, t2`, want: `WITH t1 AS MATERIALIZED (SELECT 1), t2 AS NOT MATERIALIZED (SELECT 2) SELECT * FROM t1, t2`},
		{sql: `SELECT CAST(NULL AS MAP(VARCHAR, INT)) AS map_column`, want: `SELECT CAST(NULL AS MAP(VARCHAR, INT)) AS map_column`},
		{sql: `SELECT NULL::MAP<VARCHAR, INT> AS map_column`, want: `SELECT CAST(NULL AS MAP(VARCHAR, INT)) AS map_column`},
		{sql: `CREATE TABLE t (map_col MAP(VARCHAR, INT))`, want: `CREATE TABLE t (map_col MAP(VARCHAR, INT))`},
		{sql: `CREATE TABLE t (map_col MAP<VARCHAR, INT>)`, want: `CREATE TABLE t (map_col MAP(VARCHAR, INT))`},
	}
	for _, c := range cases {
		t.Run(c.sql, func(t *testing.T) {
			expr, err := sqlglot.ParseOne(c.sql, "risingwave")
			if err != nil {
				t.Fatalf("ParseOne: %v", err)
			}
			got, err := sqlglot.Generate(expr, "risingwave", generator.Options{Pretty: c.pretty, Identify: c.identify})
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if got != c.want {
				t.Fatalf("round-trip mismatch\n want: %s\n  got: %s", c.want, got)
			}
		})
	}
}

// TestRisingWaveCrossDialect ports the validate_all arm of test_risingwave.py: the
// "read" row parses with the named dialect and regenerates with risingwave.
func TestRisingWaveCrossDialect(t *testing.T) {
	cases := []struct {
		kind     string // "read": parse dialect, generate risingwave; "write": parse risingwave, generate dialect
		sql      string
		dialect  string
		want     string
		pretty   bool
		identify bool
	}{
		{kind: "read", sql: `SELECT a FROM tbl FOR UPDATE`, dialect: "", want: `SELECT a FROM tbl`},
	}
	for _, c := range cases {
		read, write := "risingwave", "risingwave"
		if c.kind == "read" {
			read = c.dialect
		} else {
			write = c.dialect
		}
		t.Run(c.kind+" "+c.dialect+" "+c.sql, func(t *testing.T) {
			expr, err := sqlglot.ParseOne(c.sql, read)
			if err != nil {
				t.Fatalf("ParseOne(%q, read=%q): %v", c.sql, read, err)
			}
			got, err := sqlglot.Generate(expr, write, generator.Options{Pretty: c.pretty, Identify: c.identify})
			if err != nil {
				t.Fatalf("Generate(%q, write=%q): %v", c.sql, write, err)
			}
			if got != c.want {
				t.Fatalf("read=%q write=%q mismatch\n want: %s\n  got: %s", read, write, c.want, got)
			}
		})
	}
}
