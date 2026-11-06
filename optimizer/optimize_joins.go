package optimizer

import (
	"sort"
	"strings"

	sqlerrors "github.com/maltzsama/sqlglot-go/errors"
	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// joinAttrs ports JOIN_ATTRS (optimizer/optimize_joins.py:10).
var joinAttrs = []string{"on", "side", "kind", "using", "method"}

// OptimizeJoins ports optimize_joins (optimizer/optimize_joins.py:13-55): it removes cross
// joins where possible and reorders joins based on predicate dependencies.
//
//	SELECT * FROM x CROSS JOIN y JOIN z ON x.a = z.a AND y.a = z.a
//	  ->  SELECT * FROM x JOIN z ON x.a = z.a AND TRUE JOIN y ON y.a = z.a
func OptimizeJoins(expression exp.Expression) exp.Expression {
	for _, sel := range expression.FindAll(exp.KindSelect) {
		joins := expressionsFor(sel, "joins")
		if !isReorderable(joins) {
			continue
		}

		references := map[string][]exp.Expression{}
		type crossJoin struct {
			name string
			join exp.Expression
		}
		var crossJoins []crossJoin

		for _, join := range joins {
			tables := otherTableNames(join)
			if len(tables) > 0 {
				for table := range tables {
					references[table] = append(references[table], join)
				}
			} else {
				crossJoins = append(crossJoins, crossJoin{join.AliasOrName(), join})
			}
		}

		for _, cross := range crossJoins {
			for _, dep := range references[cross.name] {
				// An ANTI join's ON clause is negated, i.e. NOT EXISTS(a AND b) is not
				// equivalent to b AND NOT EXISTS(a), so its conjuncts can't be extracted.
				if joinText(dep, "kind") == "ANTI" {
					continue
				}

				on := asExpression(dep.Arg("on"))

				// Only conjuncts can be extracted, i.e. `a OR b` is not `b AND (a OR TRUE)`.
				if on == nil || on.Kind() != exp.KindAnd {
					continue
				}
				if len(otherTableNames(dep)) < 2 {
					continue
				}
				for _, predicate := range flatten(on, true) {
					if columnTableNames(predicate, "")[cross.name] {
						predicate.Replace(exp.Boolean(exp.Args{"this": true}))
						// join.on(predicate, append=False, copy=False) (expressions/query.py:
						// 708-750): the combined condition replaces the join's ON, and a CROSS
						// join stops being one.
						cross.join.Set("on", exp.And_(asExpression(cross.join.Arg("on")), predicate))
						if joinText(cross.join, "kind") == "CROSS" {
							cross.join.Set("kind", nil)
						}
					}
				}
			}
		}
	}

	expression = reorderJoins(expression)
	expression = normalizeJoins(expression)
	return expression
}

// reorderJoins ports reorder_joins (optimizer/optimize_joins.py:58-80): it reorders joins by
// topological sort order based on predicate references.
func reorderJoins(expression exp.Expression) exp.Expression {
	for _, from := range expression.FindAll(exp.KindFrom) {
		parent := from.Parent()
		if parent == nil {
			panic(&sqlerrors.OptimizeError{Msg: "FROM clause without parent expression"})
		}
		joins := expressionsFor(parent, "joins")
		if !isReorderable(joins) {
			continue
		}

		joinsByName := map[string]exp.Expression{}
		dag := map[string]map[string]bool{}
		for _, join := range joins {
			name := join.AliasOrName()
			joinsByName[name] = join
			dag[name] = otherTableNames(join)
		}

		fromName := ""
		if this := from.This(); this != nil {
			// From.alias_or_name is its source's alias_or_name.
			fromName = this.AliasOrName()
		}
		reordered := []exp.Expression{}
		for _, name := range tsort(dag) {
			if join, ok := joinsByName[name]; ok && name != fromName {
				reordered = append(reordered, join)
			}
		}
		parent.Set("joins", reordered)
	}
	return expression
}

// normalizeJoins ports normalize (optimizer/optimize_joins.py:83-100): it removes INNER and
// OUTER from joins, as they are optional.
func normalizeJoins(expression exp.Expression) exp.Expression {
	for _, join := range expression.FindAll(exp.KindJoin) {
		hasAttr := false
		for _, key := range joinAttrs {
			if truthyArg(join.Arg(key)) {
				hasAttr = true
				break
			}
		}
		if !hasAttr {
			join.Set("kind", "CROSS")
		}

		if joinText(join, "kind") == "CROSS" {
			join.Set("on", nil)
		} else {
			if kind := joinText(join, "kind"); kind == "INNER" || kind == "OUTER" {
				join.Set("kind", nil)
			}
			if !truthyArg(join.Arg("on")) && !truthyArg(join.Arg("using")) {
				join.Set("on", exp.Boolean(exp.Args{"this": true}))
			}
		}
	}
	return expression
}

// otherTableNames ports other_table_names (optimizer/optimize_joins.py:103-105).
func otherTableNames(join exp.Expression) map[string]bool {
	on := asExpression(join.Arg("on"))
	if on == nil {
		return map[string]bool{}
	}
	return columnTableNames(on, join.AliasOrName())
}

// isReorderable ports _is_reorderable (optimizer/optimize_joins.py:108-124): joins with a side
// (LEFT, RIGHT, FULL) cannot be reordered without changing which rows are included.
func isReorderable(joins []exp.Expression) bool {
	for _, join := range joins {
		if joinText(join, "side") != "" {
			return false
		}
	}
	return true
}

// joinText is Join.kind / Join.side / Join.method (expressions/query.py:684-694): the arg's
// text, uppercased.
func joinText(join exp.Expression, key string) string {
	return strings.ToUpper(join.Text(key))
}

// truthyArg mirrors Python truthiness for an arg value: unset, nil, false, "" and empty lists
// are falsy.
func truthyArg(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case string:
		return v != ""
	case []exp.Expression:
		return len(v) > 0
	case []any:
		return len(v) > 0
	case exp.Expression:
		return v != nil
	}
	return true
}

// flatten ports Expr.flatten (expressions/core.py:1232-1235): the operands of a chain of nodes
// of expression's own kind (A AND B AND C -> A, B, C), in order, unnested unless unnest is
// false or the operand is a Subquery.
func flatten(expression exp.Expression, unnest bool) []exp.Expression {
	kind := expression.Kind()
	var out []exp.Expression
	for _, node := range expression.WalkWithPrune(false, func(n exp.Expression) bool {
		return n.Parent() != nil && n.Kind() != kind
	}) {
		if node.Kind() == kind {
			continue
		}
		if unnest && node.Kind() != exp.KindSubquery {
			node = node.Unnest()
		}
		out = append(out, node)
	}
	return out
}

// columnTableNames ports exp.column_table_names (expressions/builders.py:728-748): the table
// qualifiers of every column in expression, except exclude.
func columnTableNames(expression exp.Expression, exclude string) map[string]bool {
	names := map[string]bool{}
	for _, column := range expression.FindAll(exp.KindColumn) {
		if table := column.Text("table"); table != "" && table != exclude {
			names[table] = true
		}
	}
	return names
}

// tsort ports helper.tsort (helper.py:187-219): a topological sort of dag that emits each
// dependency layer in sorted order. It panics on a cycle, as upstream raises.
func tsort(dag map[string]map[string]bool) []string {
	graph := map[string]map[string]bool{}
	for node, deps := range dag {
		copied := map[string]bool{}
		for dep := range deps {
			copied[dep] = true
			if _, ok := dag[dep]; !ok {
				if _, ok := graph[dep]; !ok {
					graph[dep] = map[string]bool{}
				}
			}
		}
		graph[node] = copied
	}

	var result []string
	for len(graph) > 0 {
		var current []string
		for node, deps := range graph {
			if len(deps) == 0 {
				current = append(current, node)
			}
		}
		if len(current) == 0 {
			panic("Cycle error")
		}
		for _, node := range current {
			delete(graph, node)
		}
		for _, deps := range graph {
			for _, node := range current {
				delete(deps, node)
			}
		}
		sort.Strings(current)
		result = append(result, current...)
	}
	return result
}
