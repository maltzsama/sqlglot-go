package generator

import (
	"strings"

	"github.com/maltzsama/sqlglot-go/expressions"
)

func (g *Generator) setOperationSQL(e expressions.Expression) string {
	opNames := map[expressions.Kind]string{expressions.KindUnion: "UNION", expressions.KindExcept: "EXCEPT", expressions.KindIntersect: "INTERSECT"}
	opName := opNames[e.Kind()]
	distinctArg := e.Arg("distinct")
	distinct, hasDistinct := distinctArg.(bool)
	if !hasDistinct && distinctArg == nil {
		distinct = true
	}
	distinctOrAll := ""
	if g.isDialect("clickhouse") {
		// dialects/clickhouse.py:38-42: EXCEPT and INTERSECT default to ALL,
		// while UNION needs its mode spelled out.
		if e.Kind() == expressions.KindUnion {
			switch {
			case !hasDistinct:
				g.unsupported("UNION requires DISTINCT or ALL to be specified")
			case distinct:
				distinctOrAll = " DISTINCT"
			default:
				distinctOrAll = " ALL"
			}
		} else if hasDistinct && distinct {
			distinctOrAll = " DISTINCT"
		}
	} else if g.dialect.SetOpExplicitDistinct {
		// BigQuery's SET_OP_DISTINCT_BY_DEFAULT is None for every operation, so DISTINCT/ALL is
		// always spelled out (generator.py:1865-1875).
		if distinct {
			distinctOrAll = " DISTINCT"
		} else {
			distinctOrAll = " ALL"
		}
	} else if hasDistinct && !distinct {
		distinctOrAll = " ALL"
	}
	sideKind := joinNonEmpty(" ", e.Text("side"), e.Text("kind"))
	if sideKind != "" {
		sideKind += " "
	}
	byName := ""
	if boolValue(e.Arg("by_name")) {
		byName = " BY NAME"
	}
	on := g.expressions(exprsOptions{expression: e, key: "on", flat: true})
	if on != "" {
		on = " ON (" + on + ")"
	}
	return sideKind + opName + distinctOrAll + byName + on
}

// setOperationsWithWrappedModifiers ports generator.py:1886-1900 for dialects whose
// set operations cannot carry their own ORDER BY or LIMIT clauses.
func (g *Generator) setOperationsWithWrappedModifiers(e expressions.Expression) string {
	limit := asExpression(e.Arg("limit"))
	order := asExpression(e.Arg("order"))
	if limit == nil && order == nil {
		return g.setOperationsSQL(e)
	}
	if limit != nil {
		limit.Pop()
	}
	if order != nil {
		order.Pop()
	}
	source := expressions.Subquery(expressions.Args{
		"this":  e,
		"alias": expressions.TableAlias(expressions.Args{"this": expressions.ToIdentifier("_l_0")}),
	})
	selectExpr := expressions.Select(expressions.Args{
		"expressions": []expressions.Expression{expressions.Star(expressions.Args{})},
		"from_":       expressions.From(expressions.Args{"this": source}),
		"limit":       limit,
		"order":       order,
	})
	return g.gen(g.preprocess(selectExpr))
}

func (g *Generator) setOperationsSQL(e expressions.Expression) string {
	sqls := []string{}
	stack := []any{e}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if expr, ok := node.(expressions.Expression); ok && !isNilExpression(expr) && (expr.Kind() == expressions.KindUnion || expr.Kind() == expressions.KindExcept || expr.Kind() == expressions.KindIntersect) {
			stack = append(stack, expr.Arg("expression"))
			stack = append(stack, g.maybeComment(g.setOperationSQL(expr), nil, expr.Comments(), true))
			stack = append(stack, expr.Arg("this"))
		} else {
			sqls = append(sqls, g.gen(node))
		}
	}
	this := strings.Join(sqls, g.sep())
	this = g.queryModifiers(e, this)
	// A MySQL file-write INTO hoisted to the set-operation node renders at the trailing position,
	// after the query modifiers (ORDER BY/LIMIT) — mirroring the plain-Select path in selectSQL.
	if into := asExpression(e.Arg("into")); into != nil && isOutfileKeyword(into.Text("kind")) {
		this += g.sqlKey(e, "into")
	}
	return g.prependCtes(e, this)
}

func (g *Generator) withSQL(e expressions.Expression) string {
	// generator.py:1538-1553: inline UDFs render in their own leading `WITH FUNCTION` clause,
	// before the ordinary CTEs and each later WITH clause.
	udfs := g.expressions(exprsOptions{expression: e, key: "udfs", flat: true})
	if udfs != "" {
		udfs = "WITH " + udfs
	}

	sql := g.expressions(exprsOptions{expression: e, flat: true})
	recursive := ""
	if boolValue(e.Arg("recursive")) && g.dialect.CteRecursiveKeywordRequired {
		recursive = "RECURSIVE "
	}
	search := g.sqlKey(e, "search")
	if search != "" {
		search = " " + search
	}
	if sql != "" {
		sql = "WITH " + recursive + sql + search
	}
	if udfs != "" && sql != "" {
		return udfs + " " + sql
	}
	return udfs + sql
}

func (g *Generator) cteSQL(e expressions.Expression) string {
	alias := asExpression(e.Arg("alias"))
	if alias != nil {
		alias.AddComments(e.PopComments(), false)
	}
	aliasSQL := g.sqlKey(e, "alias")
	materialized := ""
	if v, ok := e.Arg("materialized").(bool); ok {
		if v {
			materialized = "MATERIALIZED "
		} else {
			materialized = "NOT MATERIALIZED "
		}
	}
	keyExpressions := g.expressions(exprsOptions{expression: e, key: "key_expressions", flat: true})
	if keyExpressions != "" {
		keyExpressions = " USING KEY (" + keyExpressions + ")"
	}
	return aliasSQL + keyExpressions + " AS " + materialized + g.wrap(e)
}

func (g *Generator) recursiveWithSearchSQL(e expressions.Expression) string {
	kind := g.sqlKey(e, "kind")
	this := g.sqlKey(e, "this")
	set := g.sqlKey(e, "expression")
	using := g.sqlKey(e, "using")
	if using != "" {
		using = " USING " + using
	}
	kindSQL := kind
	if kind != "CYCLE" {
		kindSQL = "SEARCH " + kind + " FIRST BY"
	}
	return kindSQL + " " + this + " SET " + set + using
}
