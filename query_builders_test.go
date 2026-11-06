package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestQueryBuilders ports the expression-argument cases of tests/test_build.py (sqlglot
// v30.17.0) for the builders in expressions/query_builders.go. Upstream's string arguments
// are parsed here first, since the Go builders take expressions only.
func TestQueryBuilders(t *testing.T) {
	p := func(sql string) exp.Expression {
		t.Helper()
		e, err := sqlglot.ParseOne(sql, "")
		if err != nil {
			t.Fatalf("ParseOne(%q): %v", sql, err)
		}
		return e
	}
	sel := func(projections ...string) exp.Expression {
		var exprs []exp.Expression
		for _, projection := range projections {
			exprs = append(exprs, p(projection))
		}
		return exp.SelectQuery(exprs...)
	}
	from := func(e exp.Expression, table string) exp.Expression {
		return e.From_(exp.Table_(table, nil, nil, nil, nil), false)
	}
	x := p("x")

	cases := []struct {
		build func() exp.Expression
		want  string
	}{
		{func() exp.Expression { return x.Eq(exp.LiteralNumber(1)) }, "x = 1"},
		{func() exp.Expression { return x.Is_(exp.Null()) }, "x IS NULL"},
		{func() exp.Expression { return from(sel("x"), "tbl").Where(true, false, p("x > 0")) }, "SELECT x FROM tbl WHERE x > 0"},
		{func() exp.Expression { return from(sel("x"), "tbl").Where(true, false, p("x < 4 OR x > 5")) }, "SELECT x FROM tbl WHERE x < 4 OR x > 5"},
		{func() exp.Expression {
			return from(sel("x"), "tbl").Where(true, false, p("x > 0")).Where(true, false, p("x < 9"))
		}, "SELECT x FROM tbl WHERE x > 0 AND x < 9"},
		{func() exp.Expression { return from(sel("x"), "tbl").Where(true, false, p("x > 0"), p("x < 9")) }, "SELECT x FROM tbl WHERE x > 0 AND x < 9"},
		{func() exp.Expression {
			return from(sel("x"), "tbl").Where(true, false, p("x > 0")).Where(false, false, p("x < 9"))
		}, "SELECT x FROM tbl WHERE x < 9"},
		{func() exp.Expression { return from(sel("x", "y"), "tbl").GroupBy(true, false, p("x")) }, "SELECT x, y FROM tbl GROUP BY x"},
		{func() exp.Expression {
			return from(sel("x", "y", "z", "a"), "tbl").GroupBy(true, false, p("x"), p("y"), p("z")).GroupBy(true, false, p("a"))
		}, "SELECT x, y, z, a FROM tbl GROUP BY x, y, z, a"},
		{func() exp.Expression {
			return from(sel("x"), "tbl").Join(exp.Table_("tbl2", nil, nil, nil, nil), exp.JoinOptions{JoinType: "left outer"})
		}, "SELECT x FROM tbl LEFT OUTER JOIN tbl2"},
		{func() exp.Expression {
			return from(sel("x"), "tbl").Join(exp.Table_("tbl2", nil, nil, nil, nil), exp.JoinOptions{JoinType: "left outer", JoinAlias: "foo"})
		}, "SELECT x FROM tbl LEFT OUTER JOIN tbl2 AS foo"},
		{func() exp.Expression {
			return from(sel("x"), "tbl").Join(from(sel("y"), "tbl2"), exp.JoinOptions{JoinType: "left outer"})
		}, "SELECT x FROM tbl LEFT OUTER JOIN (SELECT y FROM tbl2)"},
		{func() exp.Expression {
			return from(sel("x"), "tbl").Join(from(sel("y"), "tbl2").Subquery("aliased", true), exp.JoinOptions{JoinType: "left outer"})
		}, "SELECT x FROM tbl LEFT OUTER JOIN (SELECT y FROM tbl2) AS aliased"},
		{func() exp.Expression {
			return from(sel("x"), "tbl").Join(from(sel("y"), "tbl2"), exp.JoinOptions{JoinType: "left outer", JoinAlias: "aliased"})
		}, "SELECT x FROM tbl LEFT OUTER JOIN (SELECT y FROM tbl2) AS aliased"},
		{func() exp.Expression {
			return from(sel("x"), "tbl").Join(from(sel("b"), "tbl2"), exp.JoinOptions{On: []exp.Expression{p("a=b")}, JoinType: "left", JoinAlias: "aliased"})
		}, "SELECT x FROM tbl LEFT JOIN (SELECT b FROM tbl2) AS aliased ON a = b"},
		{func() exp.Expression { return from(sel("x"), "tbl").Subquery("", true) }, "(SELECT x FROM tbl)"},
		{func() exp.Expression { return from(sel("x"), "tbl").Subquery("y", true) }, "(SELECT x FROM tbl) AS y"},
		{func() exp.Expression {
			return sel("x").From_(from(sel("x"), "tbl").Subquery("", true), false)
		}, "SELECT x FROM (SELECT x FROM tbl)"},
	}
	for _, tc := range cases {
		got, err := sqlglot.Generate(tc.build(), "", generator.Options{})
		if err != nil || got != tc.want {
			t.Errorf("got %q (err %v), want %q", got, err, tc.want)
		}
	}
}

// TestTransform ports test_transform_simple and test_transform_no_infinite_recursion
// (tests/test_expressions.py:595-619).
func TestTransform(t *testing.T) {
	gen := func(e exp.Expression) string {
		sql, err := sqlglot.Generate(e, "presto", generator.Options{})
		if err != nil {
			t.Fatal(err)
		}
		return sql
	}
	expression, _ := sqlglot.ParseOne("IF(a > 0, a, b)", "")
	fun := func(node exp.Expression) exp.Expression {
		if node.Kind() == exp.KindColumn && node.Name() == "a" {
			e, _ := sqlglot.ParseOne("c - 2", "")
			return e
		}
		return node
	}
	copied := expression.Transform(fun, true)
	if got := gen(copied); got != "IF(c - 2 > 0, c - 2, b)" || copied == expression {
		t.Errorf("copying transform = %q (same instance %v)", got, copied == expression)
	}
	inPlace := expression.Transform(fun, false)
	if got := gen(inPlace); got != "IF(c - 2 > 0, c - 2, b)" || inPlace != expression {
		t.Errorf("in-place transform = %q (same instance %v)", got, inPlace == expression)
	}

	a, _ := sqlglot.ParseOne("a", "")
	wrapped := a.Transform(func(node exp.Expression) exp.Expression {
		if node.Kind() == exp.KindColumn && node.Name() == "a" {
			e, _ := sqlglot.ParseOne("FUN(a)", "")
			return e
		}
		return node
	}, true)
	if got, _ := sqlglot.Generate(wrapped, "", generator.Options{}); got != "FUN(a)" {
		t.Errorf("transform recursion = %q, want FUN(a)", got)
	}
}
