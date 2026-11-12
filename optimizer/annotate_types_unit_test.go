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
// (tests/test_optimizer.py:869-955). Upstream runs them under snowflake and duckdb, which this port
// doesn't have yet, so they run here under oracle and tsql. The input is the pinned upstream's
// qualify output for each case, so the test covers annotate_types alone; this port's qualify still
// expands a star through a PIVOT to the pre-pivot columns (tracked separately). Types flow through
// every chained operator, and an unaliased operator shadowing a CTE resolves its source types
// through the CTE's scope.
func TestPivotAnnotation(t *testing.T) {
	unpivotSchema := schema.M("t", schema.M("id", "int", "jan", "int", "feb", "int", "north", "double", "south", "double"))
	pivotSchema := schema.M("t", schema.M("id", "int", "cat", "text", "val", "int", "kind", "text", "amt", "double"))
	cteSchema := schema.M("t", schema.M("id", "int", "jan", "int", "feb", "int"))

	for _, tc := range []struct {
		dialect, sql string
		schema       *schema.Mapping
		want         []string
	}{
		{"oracle", `SELECT "T"."ID" AS "ID", "T"."MONTH" AS "MONTH", "T"."REVENUE" AS "REVENUE", "T"."REGION" AS "REGION", "T"."HEADCOUNT" AS "HEADCOUNT" FROM "T" "T" UNPIVOT("REVENUE" FOR "MONTH" IN ("JAN", "FEB")) UNPIVOT("HEADCOUNT" FOR "REGION" IN ("NORTH", "SOUTH")) "T"`,
			unpivotSchema, []string{"ID INT", "MONTH VARCHAR", "REVENUE INT", "REGION VARCHAR", "HEADCOUNT DOUBLE"}},
		{"oracle", `SELECT "_0"."ID" AS "ID", "_0"."A" AS "A", "_0"."B" AS "B", "_0"."X" AS "X", "_0"."Y" AS "Y" FROM "T" "T" PIVOT(SUM("T"."VAL") FOR "CAT" IN ('a' AS "A", 'b' AS "B")) PIVOT(SUM("T"."AMT") FOR "KIND" IN ('x' AS "X", 'y' AS "Y")) "_0"`,
			pivotSchema, []string{"ID INT", "A BIGINT", "B BIGINT", "X DOUBLE", "Y DOUBLE"}},
		{"oracle", `WITH "C" AS (SELECT "T"."ID" AS "ID", "T"."JAN" AS "JAN", "T"."FEB" AS "FEB" FROM "T" "T") SELECT "C"."ID" AS "ID", "C"."M" AS "M", "C"."V" AS "V" FROM "C" "C" UNPIVOT("V" FOR "M" IN ("JAN", "FEB")) "C"`,
			cteSchema, []string{"ID INT", "M VARCHAR", "V INT"}},
		{"tsql", "SELECT [t].[id] AS [id], [t].[revenue] AS [revenue], [t].[month] AS [month], [t].[headcount] AS [headcount], [t].[region] AS [region] FROM [t] AS [t] UNPIVOT([revenue] FOR [month] IN ([t].[jan], [t].[feb])) UNPIVOT([headcount] FOR [region] IN ([t].[north], [t].[south])) AS [t]",
			unpivotSchema, []string{"id INT", "revenue INT", "month VARCHAR", "headcount DOUBLE", "region VARCHAR"}},
		{"tsql", "SELECT [_0].[id] AS [id], [_0].[a] AS [a], [_0].[b] AS [b], [_0].[x] AS [x], [_0].[y] AS [y] FROM [t] AS [t] PIVOT(SUM([t].[val]) FOR [t].[cat] IN ('a' AS [a], 'b' AS [b])) PIVOT(SUM([t].[amt]) FOR [t].[kind] IN ('x' AS [x], 'y' AS [y])) AS [_0]",
			pivotSchema, []string{"id INT", "a BIGINT", "b BIGINT", "x DOUBLE", "y DOUBLE"}},
		{"tsql", "WITH [c] AS (SELECT [t].[id] AS [id], [t].[jan] AS [jan], [t].[feb] AS [feb] FROM [t] AS [t]) SELECT [c].[id] AS [id], [c].[v] AS [v], [c].[m] AS [m] FROM [c] AS [c] UNPIVOT([v] FOR [m] IN ([c].[jan], [c].[feb])) AS [c]",
			cteSchema, []string{"id INT", "v INT", "m VARCHAR"}},
	} {
		e := annotate(t, tc.sql, tc.schema, tc.dialect)
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
