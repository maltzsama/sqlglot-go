package optimizer_test

import (
	"strings"
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
	"github.com/maltzsama/sqlglot-go/optimizer"
	"github.com/maltzsama/sqlglot-go/schema"
)

// Ports of the core annotation tests in tests/test_optimizer.py (sqlglot v30.17.0).

func annotate(t *testing.T, sql string, s schema.SchemaType, dialect string) exp.Expression {
	t.Helper()
	e, err := sqlglot.ParseOne(sql, dialect)
	if err != nil {
		t.Fatalf("ParseOne(%q): %v", sql, err)
	}
	return optimizer.AnnotateTypes(e, s, dialect)
}

// dtypeOf is `expression.type.this`.
func dtypeOf(e exp.Expression) exp.DType {
	if e == nil || e.Type() == nil {
		return ""
	}
	d, _ := e.Type().Arg("this").(exp.DType)
	return d
}

func wantDType(t *testing.T, what string, e exp.Expression, want exp.DType) {
	t.Helper()
	if got := dtypeOf(e); got != want {
		t.Errorf("%s: type %q, want %q", what, got, want)
	}
}

// TestCastTypeAnnotation ports test_cast_type_annotation (lines 2190-2203).
func TestCastTypeAnnotation(t *testing.T) {
	e := annotate(t, "CAST('2020-01-01' AS TIMESTAMPTZ(9))", nil, "")
	wantDType(t, "cast", e, exp.DTypeTimestampTz)
	wantDType(t, "cast operand", e.This(), exp.DTypeVarchar)
	to := asExprArg(e, "to")
	wantDType(t, "cast target", to, exp.DTypeTimestampTz)
	wantDType(t, "cast target precision", to.Expressions()[0].This(), exp.DTypeInt)

	e = annotate(t, "CAST(x AS INTERVAL)", nil, "")
	wantDType(t, "interval cast", e, exp.DTypeInterval)
	wantDType(t, "interval cast operand", e.This(), exp.DTypeUnknown)
}

func asExprArg(e exp.Expression, key string) exp.Expression {
	v, _ := e.Arg(key).(exp.Expression)
	return v
}

// TestBinaryAnnotation ports test_binary_annotation (lines 2211-2224).
func TestBinaryAnnotation(t *testing.T) {
	e := annotate(t, "SELECT 0.0 + (2 + 3)", nil, "").Expressions()[0]
	wantDType(t, "sum", e, exp.DTypeDouble)
	wantDType(t, "left", e.Left(), exp.DTypeDouble)
	wantDType(t, "right paren", e.Right(), exp.DTypeInt)
	wantDType(t, "inner sum", e.Right().This(), exp.DTypeInt)
	wantDType(t, "inner left", e.Right().This().Left(), exp.DTypeInt)
	wantDType(t, "inner right", e.Right().This().Right(), exp.DTypeInt)

	for _, numeric := range []exp.DType{exp.DTypeBigInt, exp.DTypeDouble, exp.DTypeInt} {
		e := annotate(t, "SELECT '1' + CAST(x AS "+string(numeric)+")", nil, "").Expressions()[0]
		wantDType(t, "text + "+string(numeric), e, numeric)
	}
}

// TestTypedDivAnnotation ports test_typeddiv_annotation (lines 2226-2240).
func TestTypedDivAnnotation(t *testing.T) {
	exprs := annotate(t, "SELECT 2 / 3, 2 / 3.0", nil, "presto").Expressions()
	wantDType(t, "presto int / int", exprs[0], exp.DTypeBigInt)
	wantDType(t, "presto int / double", exprs[1], exp.DTypeDouble)

	exprs = annotate(t, "SELECT SUM(2 / 3), CAST(2 AS DECIMAL) / 3", nil, "mysql").Expressions()
	wantDType(t, "mysql SUM(2 / 3)", exprs[0], exp.DTypeDouble)
	wantDType(t, "mysql 2 / 3", exprs[0].This(), exp.DTypeDouble)
	wantDType(t, "mysql decimal / int", exprs[1], exp.DTypeDecimal)
}

// TestDerivedTablesColumnAnnotation ports test_derived_tables_column_annotation (2351-2381).
func TestDerivedTablesColumnAnnotation(t *testing.T) {
	s := schema.M("x", schema.M("cola", "INT"), "y", schema.M("cola", "FLOAT"))
	e := annotate(t, `SELECT a.cola AS cola FROM (
		SELECT x.cola + y.cola AS cola FROM (SELECT x.cola AS cola FROM x AS x) AS x
		JOIN (SELECT y.cola AS cola FROM y AS y) AS y) AS a`, s, "")
	wantDType(t, "a.cola AS cola", e.Expressions()[0], exp.DTypeFloat)

	additionAlias := asExprArg(e, "from_").This().This().Expressions()[0]
	wantDType(t, "x.cola + y.cola AS cola", additionAlias, exp.DTypeFloat)
	addition := additionAlias.This()
	wantDType(t, "addition", addition, exp.DTypeFloat)
	wantDType(t, "x.cola", addition.This(), exp.DTypeInt)
	wantDType(t, "y.cola", addition.Expr(), exp.DTypeFloat)
}

// TestCTEColumnAnnotation ports test_cte_column_annotation (lines 2383-2440).
func TestCTEColumnAnnotation(t *testing.T) {
	s := schema.M("x", schema.M("cola", "CHAR"), "y", schema.M("colb", "TEXT", "colc", "BOOLEAN"))
	e := annotate(t, `WITH tbl AS (
		SELECT x.cola + 'bla' AS cola, y.colb AS colb, y.colc AS colc
		FROM (SELECT x.cola AS cola FROM x AS x) AS x
		JOIN (SELECT y.colb AS colb, y.colc AS colc FROM y AS y) AS y)
		SELECT tbl.cola + tbl.colb + 'foo' AS col FROM tbl AS tbl WHERE tbl.colc = True`, s, "")
	wantDType(t, "col", e.Expressions()[0], exp.DTypeText)

	outer := e.Expressions()[0].This()
	wantDType(t, "outer addition", outer, exp.DTypeText)
	wantDType(t, "outer left", outer.Left(), exp.DTypeText)
	wantDType(t, "outer right", outer.Right(), exp.DTypeVarchar)
	inner := outer.Left()
	wantDType(t, "tbl.cola", inner.Left(), exp.DTypeVarchar)
	wantDType(t, "tbl.colb", inner.Right(), exp.DTypeText)
	wantDType(t, "where", asExprArg(e, "where").This(), exp.DTypeBoolean)

	cteSelect := asExprArg(e, "with_").Expressions()[0].This()
	wantDType(t, "cte cola", cteSelect.Expressions()[0], exp.DTypeVarchar)
	wantDType(t, "cte colb", cteSelect.Expressions()[1], exp.DTypeText)
	wantDType(t, "cte colc", cteSelect.Expressions()[2], exp.DTypeBoolean)
	cteAddition := cteSelect.Expressions()[0].This()
	wantDType(t, "x.cola + 'bla'", cteAddition, exp.DTypeVarchar)
	wantDType(t, "x.cola", cteAddition.Left(), exp.DTypeChar)
	wantDType(t, "'bla'", cteAddition.Right(), exp.DTypeVarchar)
	for i, want := range []exp.DType{exp.DTypeChar, exp.DTypeText} {
		subquery := cteSelect.FindAll(exp.KindSubquery)[i]
		wantDType(t, "derived projection", subquery.This().Expressions()[0].This(), want)
	}
}

// TestUnknownAnnotation ports test_unknown_annotation (lines 2483-2507).
func TestUnknownAnnotation(t *testing.T) {
	s := schema.M("x", schema.M("cola", "VARCHAR"))
	alias := annotate(t, "SELECT x.cola + SOME_ANONYMOUS_FUNC(x.cola) AS col FROM x AS x", s, "").Expressions()[0]
	wantDType(t, "col", alias, exp.DTypeUnknown)
	concat := alias.This()
	wantDType(t, "concat", concat, exp.DTypeUnknown)
	wantDType(t, "x.cola", concat.Left(), exp.DTypeVarchar)
	wantDType(t, "anonymous call", concat.Right(), exp.DTypeUnknown)
	wantDType(t, "anonymous call arg", concat.Right().Expressions()[0], exp.DTypeVarchar)

	// Unqualified columns must not raise.
	annotate(t, "select x from y lateral view explode(y) as x", nil, "")

	// NULL <op> UNKNOWN yields UNKNOWN.
	wantDType(t, "NULL + anonymous", annotate(t, "SELECT NULL + ANONYMOUS_FUNC()", nil, "").Expressions()[0], exp.DTypeUnknown)
}

// TestPredicateAnnotation ports test_predicate_annotation (lines 2562-2567).
func TestPredicateAnnotation(t *testing.T) {
	wantDType(t, "BETWEEN", annotate(t, "x BETWEEN a AND b", nil, ""), exp.DTypeBoolean)
	wantDType(t, "IN", annotate(t, "x IN (a, b, c, d)", nil, ""), exp.DTypeBoolean)
}

// TestAggFuncAnnotation ports test_aggfunc_annotation (lines 2569-2591).
func TestAggFuncAnnotation(t *testing.T) {
	s := schema.M("x", schema.M("cola", "SMALLINT", "colb", "FLOAT", "colc", "TEXT", "cold", "DATE"))
	for _, tc := range []struct {
		fn, col string
		want    exp.DType
	}{
		{"AVG", "cola", exp.DTypeDouble},
		{"SUM", "cola", exp.DTypeBigInt},
		{"SUM", "colb", exp.DTypeDouble},
		{"MIN", "cola", exp.DTypeSmallInt},
		{"MIN", "colb", exp.DTypeFloat},
		{"MAX", "colc", exp.DTypeText},
		{"MAX", "cold", exp.DTypeDate},
		{"COUNT", "colb", exp.DTypeBigInt},
		{"STDDEV", "cola", exp.DTypeDouble},
		{"ABS", "cola", exp.DTypeSmallInt},
		{"ABS", "colb", exp.DTypeFloat},
	} {
		e := annotate(t, "SELECT "+tc.fn+"(x."+tc.col+") AS _col_0 FROM x AS x", s, "")
		wantDType(t, tc.fn+"("+tc.col+")", e.Expressions()[0], tc.want)
	}
}

// TestConcatAnnotation ports test_concat_annotation (lines 2593-2595).
func TestConcatAnnotation(t *testing.T) {
	wantDType(t, "CONCAT", annotate(t, "CONCAT('A', 'B')", nil, ""), exp.DTypeVarchar)
}

// TestRootSubqueryAnnotation ports test_root_subquery_annotation (lines 2597-2601).
func TestRootSubqueryAnnotation(t *testing.T) {
	e := annotate(t, "(SELECT 1, 2 FROM x) LIMIT 0", nil, "")
	if e.Kind() != exp.KindSubquery {
		t.Fatalf("root is %s, want Subquery", exp.ClassName(e.Kind()))
	}
	wantDType(t, "first", e.Selects()[0], exp.DTypeInt)
	wantDType(t, "second", e.Selects()[1], exp.DTypeInt)
}

// TestUnionAnnotation ports the optimize()-free part of test_union_annotation (lines
// 2801-2834); the chained-UNION cases go through optimize() and land with #32.
func TestUnionAnnotation(t *testing.T) {
	for _, tc := range []struct {
		left, right string
		want        exp.DType
	}{
		{"SELECT 1::INT AS c", "SELECT 2::BIGINT AS c", exp.DTypeBigInt},
		{"SELECT 1::INT AS c", "SELECT 2::BIGDECIMAL AS c", exp.DTypeBigDecimal},
		{"SELECT 1 AS c", "SELECT NULL AS c", exp.DTypeInt},
		{"SELECT FOO() AS c", "SELECT 1 AS c", exp.DTypeUnknown},
		{"SELECT FOO() AS c", "SELECT BAR() AS c", exp.DTypeUnknown},
	} {
		lr := annotate(t, "SELECT t.c FROM ("+tc.left+" UNION ALL "+tc.right+") t(c)", nil, "")
		rl := annotate(t, "SELECT t.c FROM ("+tc.right+" UNION ALL "+tc.left+") t(c)", nil, "")
		wantDType(t, tc.left+" UNION ALL "+tc.right, lr.Selects()[0], tc.want)
		wantDType(t, tc.right+" UNION ALL "+tc.left, rl.Selects()[0], tc.want)
	}

	byName := annotate(t, "SELECT t.a, t.d FROM (SELECT 1 a, 3 d, UNION ALL BY NAME SELECT 7.0 d, 8::BIGINT a) AS t(a, d)", nil, "")
	wantDType(t, "by name a", byName.Selects()[0], exp.DTypeBigInt)
	wantDType(t, "by name d", byName.Selects()[1], exp.DTypeDouble)
}

// TestPivotAnnotation ports test_multiple_pivots_annotate_types and test_unpivot_cte_annotate_types
// (tests/test_optimizer.py:869-955): raw SELECT * over a chained (UN)PIVOT runs through qualify,
// which expands the star to the operators' output columns, then annotate_types types each one.
// Types flow through every chained operator — the name columns are text and each value column
// takes the type of the columns it was folded from — and an unaliased operator shadowing a CTE
// resolves its source types through the CTE's scope.
func TestPivotAnnotation(t *testing.T) {
	for _, tc := range []struct {
		dialect, sql string
		schema       *schema.Mapping
		want         []string
	}{
		{"snowflake", `SELECT * FROM t UNPIVOT(revenue FOR month IN (jan, feb)) UNPIVOT(headcount FOR region IN (north, south))`,
			schema.M("t", schema.M("id", "int", "jan", "int", "feb", "int", "north", "double", "south", "double")),
			[]string{"ID INT", "MONTH VARCHAR", "REVENUE INT", "REGION VARCHAR", "HEADCOUNT DOUBLE"}},
		{"snowflake", `SELECT * FROM t PIVOT(SUM(val) FOR cat IN ('a' AS a, 'b' AS b)) PIVOT(SUM(amt) FOR kind IN ('x' AS x, 'y' AS y))`,
			schema.M("t", schema.M("id", "int", "cat", "text", "val", "int", "kind", "text", "amt", "double")),
			[]string{"ID INT", "A BIGINT", "B BIGINT", "X DOUBLE", "Y DOUBLE"}},
		{"duckdb", `WITH c AS (SELECT id, jan, feb FROM t) SELECT * FROM c UNPIVOT(v FOR m IN (jan, feb))`,
			schema.M("t", schema.M("id", "int", "jan", "int", "feb", "int")),
			[]string{"id INT", "m VARCHAR", "v INT"}},
	} {
		e, err := sqlglot.ParseOne(tc.sql, tc.dialect)
		if err != nil {
			t.Fatalf("ParseOne(%q): %v", tc.sql, err)
		}
		opts := optimizer.DefaultQualifyOpts()
		opts.Schema = tc.schema
		opts.Dialect = tc.dialect
		e = optimizer.Qualify(e, opts)
		e = optimizer.AnnotateTypes(e, tc.schema, tc.dialect)

		var got []string
		for _, s := range e.Selects() {
			typ := "<nil>"
			if s.Type() != nil {
				typ, _ = sqlglot.Generate(s.Type(), "", generator.Options{})
			}
			got = append(got, s.AliasOrName()+" "+typ)
		}
		if strings.Join(got, ", ") != strings.Join(tc.want, ", ") {
			t.Errorf("[%s] %s\n got: %v\nwant: %v", tc.dialect, tc.sql, got, tc.want)
		}
	}
}

// TestPortedFunctionNodeAnnotation guards the function nodes ported for #44: they now parse
// to their structured Kind, so EXPRESSION_METADATA types them instead of UNKNOWN.
func TestPortedFunctionNodeAnnotation(t *testing.T) {
	cases := []struct {
		sql  string
		want exp.DType
	}{
		{"ACOS(x)", exp.DTypeDouble},
		{"PI()", exp.DTypeDouble},
		{"RANK()", exp.DTypeBigInt},
		{"COVAR_POP(x, y)", exp.DTypeDouble},
		{"WEEK(x)", exp.DTypeTinyInt},
		{"MONTHNAME(x)", exp.DTypeVarchar},
	}
	for _, tc := range cases {
		e := annotate(t, tc.sql, nil, "")
		wantDType(t, tc.sql, e, tc.want)
	}
}

// TestNullTypeAnnotation covers the DEFAULT_NULL_TYPE / SUPPORTS_NULL_TYPE split
// (annotate_types.py:288-293): a bare NULL folds to the dialect's default null type, except
// under dialects that keep NULL (VOID) as a real type.
func TestNullTypeAnnotation(t *testing.T) {
	wantDType(t, "base NULL", annotate(t, "NULL", nil, ""), exp.DTypeUnknown)
	wantDType(t, "bigquery NULL", annotate(t, "NULL", nil, "bigquery"), exp.DTypeBigInt)
	wantDType(t, "spark NULL", annotate(t, "NULL", nil, "spark"), exp.DTypeNull)
	wantDType(t, "databricks NULL", annotate(t, "NULL", nil, "databricks"), exp.DTypeNull)
}

// TestBigQueryArrayAnnotation covers typing/bigquery.py:127-193: ARRAY(<set operation>) takes
// the coerced first projection as its element type instead of the base by-args rule.
func TestBigQueryArrayAnnotation(t *testing.T) {
	for _, tc := range []struct {
		sql  string
		want exp.DType
	}{
		{"ARRAY(SELECT 1 UNION ALL SELECT 2)", exp.DTypeInt},
		{"ARRAY(SELECT 1 UNION ALL SELECT 2.5)", exp.DTypeDouble},
		{"ARRAY(SELECT 'a' UNION ALL SELECT 'b')", exp.DTypeVarchar},
	} {
		e := annotate(t, tc.sql, nil, "bigquery")
		if e.Type() == nil || !e.Type().IsType(exp.DTypeArray) {
			t.Fatalf("%s: type %v, want ARRAY", tc.sql, e.Type())
		}
		elems := e.Type().Expressions()
		if len(elems) != 1 {
			t.Fatalf("%s: element count %d, want 1", tc.sql, len(elems))
		}
		wantDType(t, tc.sql+" element", elems[0], tc.want)
	}
}

// TestAliasFunctionArgShapes pins the argument mapping of the aliases ported for #94 that do
// not follow their class's positional arg_types.
func TestAliasFunctionArgShapes(t *testing.T) {
	// MySQL CONVERT_TZ(timestamp, source_zone, target_zone).
	ct, err := sqlglot.ParseOne("CONVERT_TZ('2026-01-01 12:00:00', '+00:00', '+02:00')", "mysql")
	if err != nil {
		t.Fatal(err)
	}
	if ct.Kind() != exp.KindConvertTimezone {
		t.Fatalf("CONVERT_TZ kind = %v", ct.Kind())
	}
	if got := ct.Text("timestamp"); got != "2026-01-01 12:00:00" {
		t.Errorf("timestamp = %q", got)
	}
	if got := ct.Text("source_tz"); got != "+00:00" {
		t.Errorf("source_tz = %q", got)
	}
	if got := ct.Text("target_tz"); got != "+02:00" {
		t.Errorf("target_tz = %q", got)
	}

	// Snowflake EDITDISTANCE(a, b, max_distance) maps to Levenshtein.max_dist.
	ed, err := sqlglot.ParseOne("EDITDISTANCE('a', 'abcdefgh', 2)", "snowflake")
	if err != nil {
		t.Fatal(err)
	}
	if ed.Kind() != exp.KindLevenshtein {
		t.Fatalf("EDITDISTANCE kind = %v", ed.Kind())
	}
	if got := ed.Text("max_dist"); got != "2" {
		t.Errorf("max_dist = %q", got)
	}

	// TRY_TO_TIMESTAMP normalizes the format through the Snowflake TIME_MAPPING and keeps safe.
	ts, err := sqlglot.ParseOne("TRY_TO_TIMESTAMP('2024-01-02', 'YYYY-MM-DD')", "snowflake")
	if err != nil {
		t.Fatal(err)
	}
	if ts.Kind() != exp.KindStrToTime || ts.Arg("safe") != true {
		t.Fatalf("TRY_TO_TIMESTAMP kind/safe = %v/%v", ts.Kind(), ts.Arg("safe"))
	}
	if got := ts.Text("format"); got != "%Y-%m-%d" {
		t.Errorf("format = %q, want %%Y-%%m-%%d", got)
	}

	// DIV generates the CAST(... AS INT) form in base and MySQL.
	for _, tc := range []struct{ dialect, want string }{
		{"", "CAST(7 / 2 AS INT)"},
		{"mysql", "CAST(7 / 2 AS SIGNED)"}, // MySQL maps INT to SIGNED.
	} {
		e, err := sqlglot.ParseOne("7 DIV 2", tc.dialect)
		if err != nil {
			t.Fatal(err)
		}
		if e.Kind() != exp.KindIntDiv {
			t.Fatalf("[%s] DIV kind = %v", tc.dialect, e.Kind())
		}
		got, err := sqlglot.Generate(e, tc.dialect, generator.Options{})
		if err != nil {
			t.Fatalf("[%s] Generate: %v", tc.dialect, err)
		}
		if got != tc.want {
			t.Errorf("[%s] DIV = %q, want %q", tc.dialect, got, tc.want)
		}
	}
}
