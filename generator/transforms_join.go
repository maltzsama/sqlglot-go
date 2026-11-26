package generator

import (
	"strings"

	"github.com/maltzsama/sqlglot-go/expressions"
)

// eliminateFullOuterJoin ports transforms.eliminate_full_outer_join
// (transforms.py:632-669): a single FULL OUTER join becomes a UNION ALL of a LEFT join and
// an anti-joined RIGHT join.
func eliminateFullOuterJoin(e expressions.Expression) expressions.Expression {
	if e.Kind() != expressions.KindSelect {
		return e
	}
	joins := listFromValue(e.Arg("joins"))
	fullIndex := -1
	for i, item := range joins {
		join := asExpression(item)
		if join != nil && strings.EqualFold(join.Text("side"), "FULL") {
			if fullIndex >= 0 {
				return e
			}
			fullIndex = i
		}
	}
	if fullIndex < 0 {
		return e
	}

	from := asExpression(e.Arg("from_"))
	full := asExpression(joins[fullIndex])
	if from == nil || full == nil || from.This() == nil || full.This() == nil {
		return e
	}

	copyExpr := e.Copy()
	e.Set("limit", nil)
	copyJoins := listFromValue(copyExpr.Arg("joins"))
	copyJoin := asExpression(copyJoins[fullIndex])
	if copyJoin == nil {
		return e
	}

	tables := []string{from.This().AliasOrName(), full.This().AliasOrName()}
	condition := asExpression(full.Arg("on"))
	if condition == nil {
		parts := make([]expressions.Expression, 0)
		for _, item := range listFromValue(full.Arg("using")) {
			column := asExpression(item)
			if column == nil {
				continue
			}
			parts = append(parts, expressions.EQ(expressions.Args{
				"this": expressions.Column(expressions.Args{
					"this":  expressions.ToIdentifier(column.Name(), false),
					"table": expressions.ToIdentifier(tables[0], false),
				}),
				"expression": expressions.Column(expressions.Args{
					"this":  expressions.ToIdentifier(column.Name(), false),
					"table": expressions.ToIdentifier(tables[1], false),
				}),
			}))
		}
		condition = andAll(parts)
	}
	if condition == nil {
		return e
	}

	full.Set("side", "LEFT")
	copyJoin.Set("side", "RIGHT")

	antiJoin := expressions.Select(expressions.Args{
		"expressions": []expressions.Expression{expressions.LiteralNumber("1")},
		"from_":       expressions.From(expressions.Args{"this": from.This().Copy()}),
		"where":       expressions.Where(expressions.Args{"this": condition.Copy()}),
	})
	copyExpr.Set("where", expressions.Where(expressions.Args{
		"this": expressions.Not(expressions.Args{
			"this": expressions.Exists(expressions.Args{"this": antiJoin}),
		}),
	}))
	copyExpr.Set("with_", nil)
	e.Set("order", nil)

	return expressions.Union(expressions.Args{
		"this":       e,
		"expression": copyExpr,
		"distinct":   false,
	})
}

// andAll combines expressions with AND, returning nil for an empty list.
func andAll(parts []expressions.Expression) expressions.Expression {
	if len(parts) == 0 {
		return nil
	}
	result := parts[0]
	for _, part := range parts[1:] {
		result = expressions.And(expressions.Args{"this": result, "expression": part})
	}
	return result
}
