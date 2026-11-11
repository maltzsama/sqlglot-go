package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestSpark2Identity is the same-dialect round-trip oracle for Spark 2, ported from the
// spark2 write mappings in .reference/sqlglot-v30.17.0/tests/dialects/test_spark.py. The
// expected value mirrors upstream's own canonical rewrite, not the input spelling.
func TestSpark2Identity(t *testing.T) {
	cases := []struct {
		sql  string
		want string
	}{
		// TRIM_PATTERN_FIRST: the comma form puts the characters first.
		{sql: `SELECT TRIM('x', a)`, want: `SELECT TRIM('x' FROM a)`},
		{sql: `SELECT LTRIM(a, 'x')`, want: `SELECT TRIM(LEADING a FROM 'x')`},
		{sql: `SELECT RTRIM(a, 'x')`, want: `SELECT TRIM(TRAILING a FROM 'x')`},
		{sql: `SELECT LTRIM(a)`, want: `SELECT LTRIM(a)`},
		// FUNC_TOKENS += AND/OR: connector-function calls.
		{sql: `SELECT AND(a, b)`, want: `SELECT (a AND b)`},
		{sql: `SELECT OR(a, b, c)`, want: `SELECT (a OR b OR c)`},
		// Join hints.
		{sql: `SELECT /*+ BROADCAST(t1) */ * FROM t`, want: `SELECT /*+ BROADCAST(t1) */ * FROM t`},
		{sql: `SELECT /*+ MERGE(t1, t2) */ * FROM t`, want: `SELECT /*+ MERGE(t1, t2) */ * FROM t`},
		{sql: `SELECT /*+ SHUFFLE_REPLICATE_NL(t1) */ * FROM t`, want: `SELECT /*+ SHUFFLE_REPLICATE_NL(t1) */ * FROM t`},
		// APPROX_PERCENTILE shares hive's distinct-argument quantile grammar.
		{sql: `SELECT APPROX_PERCENTILE(DISTINCT x, 0.5)`, want: `SELECT PERCENTILE_APPROX(DISTINCT x, 0.5)`},
		// CHANGE_COLUMN_ALTER_SYNTAX: CHANGE COLUMN ... TYPE is ALTER COLUMN ... TYPE.
		{sql: `ALTER TABLE t CHANGE COLUMN a TYPE INT`, want: `ALTER TABLE t ALTER COLUMN a TYPE INT`},
		{sql: `ALTER TABLE t CHANGE a b INT COMMENT 'x'`, want: `ALTER TABLE t RENAME COLUMN a TO b`},
		// Spark's DROP COLUMNS form is structured; every other DROP action is a Command.
		{sql: `ALTER TABLE t DROP COLUMNS (a, b)`, want: `ALTER TABLE t DROP COLUMNS (a, b)`},
		{sql: `ALTER TABLE t DROP COLUMN a`, want: `ALTER TABLE t DROP COLUMN a`},
		// ELEMENT_AT is a safe=False bracket and renders back unchanged.
		{sql: `SELECT ELEMENT_AT(a, 2)`, want: `SELECT ELEMENT_AT(a, 2)`},
		// TIMESTAMP-typed arguments of the UTC converters.
		{sql: `SELECT FROM_UTC_TIMESTAMP('2016-08-31', 'Asia/Seoul')`, want: `SELECT FROM_UTC_TIMESTAMP(CAST('2016-08-31' AS TIMESTAMP), 'Asia/Seoul')`},
		{sql: `SELECT TO_UTC_TIMESTAMP(x, tz)`, want: `SELECT TO_UTC_TIMESTAMP(CAST(x AS TIMESTAMP), tz)`},
		// TO_TIMESTAMP: one argument casts, two build a formatted StrToTime.
		{sql: `SELECT TO_TIMESTAMP('2016-1-1')`, want: `SELECT CAST('2016-1-1' AS TIMESTAMP)`},
		{sql: `SELECT TO_TIMESTAMP('2016-1-1 3:4:5', 'yyyy-M-d H:m:s')`, want: `SELECT TO_TIMESTAMP('2016-1-1 3:4:5', 'yyyy-M-d H:m:s')`},
		// DATE_TRUNC / TRUNC.
		{sql: `SELECT DATE_TRUNC('MONTH', ts)`, want: `SELECT DATE_TRUNC('MONTH', ts)`},
		{sql: `SELECT TRUNC(d, 'MM')`, want: `SELECT TRUNC(d, 'MM')`},
		// UDF-style functions.
		{sql: `SELECT FORMAT_STRING('%s', a)`, want: `SELECT FORMAT_STRING('%s', a)`},
		{sql: `SELECT SHIFTLEFT(a, 1), SHIFTRIGHT(a, 1)`, want: `SELECT SHIFTLEFT(a, 1), SHIFTRIGHT(a, 1)`},
		{sql: `SELECT MAP_FROM_ARRAYS(a, b)`, want: `SELECT MAP_FROM_ARRAYS(a, b)`},
		{sql: `SELECT RLIKE(a, 'b')`, want: `SELECT a RLIKE 'b'`},
		{sql: `SELECT AGGREGATE(my_arr, 0, (acc, x) -> acc + x, s -> s * 2)`, want: `SELECT AGGREGATE(my_arr, 0, (acc, x) -> acc + x, s -> s * 2)`},
		// The lenient initcap delimiters keep the whole string as one word.
		{sql: `SELECT INITCAP(x)`, want: `SELECT INITCAP(x)`},
		// Cast builders.
		{sql: `SELECT BOOLEAN(1), INT('1'), STRING(1)`, want: `SELECT CAST(1 AS BOOLEAN), CAST('1' AS INT), CAST(1 AS STRING)`},
		// Hex strings and the TIMESTAMP keyword remap.
		{sql: `SELECT X'FF'`, want: `SELECT X'FF'`},
		{sql: `SELECT TIMESTAMP '2020-01-01'`, want: `SELECT CAST('2020-01-01' AS TIMESTAMP)`},
		// Datasource storage format.
		{sql: `CREATE TABLE t (a INT) USING delta`, want: `CREATE TABLE t (a INT) USING DELTA`},
		// PIVOT field unqualification.
		{sql: `SELECT * FROM t PIVOT(SUM(t.v) FOR t.c IN (1, 2))`, want: `SELECT * FROM t PIVOT(SUM(t.v) FOR c IN (1, 2))`},
		// A PIVOT alias moves onto a wrapping SELECT * subquery.
		{sql: `SELECT piv.x FROM tbl PIVOT(SUM(a) FOR b IN ('x')) piv`, want: `SELECT piv.x FROM (SELECT * FROM tbl PIVOT(SUM(a) FOR b IN ('x'))) AS piv`},
		// STRUCT positional fields are named col<N> by the Hive parser; Spark renders them as
		// aliases.
		{sql: `SELECT STRUCT(1, 2)`, want: `SELECT STRUCT(1 AS col1, 2 AS col2)`},
		{sql: `SELECT STRUCT(x, 1, y AS col3, STRUCT(5)) FROM t`, want: `SELECT STRUCT(x AS x, 1 AS col2, y AS col3, STRUCT(5 AS col1) AS col4) FROM t`},
		// TRANSFORM ... USING is a QueryTransform.
		{sql: `SELECT TRANSFORM(a, b) USING 'cat' FROM t`, want: `SELECT TRANSFORM(a, b) USING 'cat' FROM t`},
	}
	for _, c := range cases {
		t.Run(c.sql, func(t *testing.T) {
			expr, err := sqlglot.ParseOne(c.sql, "spark2")
			if err != nil {
				t.Fatalf("ParseOne: %v", err)
			}
			got, err := sqlglot.Generate(expr, "spark2", generator.Options{})
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if got != c.want {
				t.Fatalf("round-trip mismatch\n want: %s\n  got: %s", c.want, got)
			}
		})
	}
}

// TestSpark2AST pins the node shapes that the round-trip alone can't distinguish, e.g. a
// DROP COLUMN that degrades to Command on purpose.
func TestSpark2AST(t *testing.T) {
	cases := []struct {
		sql  string
		want exp.Kind
	}{
		{sql: `ALTER TABLE t DROP COLUMNS (a, b)`, want: exp.KindDrop},
		{sql: `ALTER TABLE t DROP COLUMN a`, want: exp.KindCommand},
		{sql: `ALTER TABLE t DROP CONSTRAINT x`, want: exp.KindCommand},
		{sql: `SELECT ELEMENT_AT(a, 2)`, want: exp.KindBracket},
		{sql: `SELECT AND(a, b)`, want: exp.KindParen},
	}
	for _, c := range cases {
		t.Run(c.sql, func(t *testing.T) {
			expr, err := sqlglot.ParseOne(c.sql, "spark2")
			if err != nil {
				t.Fatalf("ParseOne: %v", err)
			}
			node := expr
			if kind := expr.Kind(); kind == exp.KindAlter {
				actions, _ := expr.Arg("actions").([]exp.Expression)
				if len(actions) == 0 {
					t.Fatalf("%s: no actions", c.sql)
				}
				node = actions[0]
			} else if kind == exp.KindSelect {
				node = expr.Expressions()[0]
			}
			if node.Kind() != c.want {
				t.Fatalf("%s: got %s, want %s", c.sql, exp.ClassName(node.Kind()), exp.ClassName(c.want))
			}
		})
	}

	// The ELEMENT_AT bracket is 1-based and unsafe; the hint parses to a JoinHint.
	e, err := sqlglot.ParseOne(`SELECT ELEMENT_AT(a, 2)`, "spark2")
	if err != nil {
		t.Fatal(err)
	}
	bracket := e.Expressions()[0]
	if bracket.Arg("safe") != false || bracket.Arg("offset") != 1 {
		t.Fatalf("ELEMENT_AT bracket: safe=%v offset=%v", bracket.Arg("safe"), bracket.Arg("offset"))
	}

	h, err := sqlglot.ParseOne(`SELECT /*+ MERGE(t1) */ * FROM t`, "spark2")
	if err != nil {
		t.Fatal(err)
	}
	hint := h.Arg("hint")
	if hint == nil {
		t.Fatal("missing hint")
	}
	items := hint.(exp.Expression).Expressions()
	if len(items) != 1 || items[0].Kind() != exp.KindJoinHint || items[0].Text("this") != "MERGE" {
		t.Fatalf("hint items = %v", hint.(exp.Expression).ToS())
	}
}
