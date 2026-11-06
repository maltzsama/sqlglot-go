package optimizer

import exp "github.com/maltzsama/sqlglot-go/expressions"

// EliminateJoins ports eliminate_joins (optimizer/eliminate_joins.py:13-50): it removes joins
// whose source is unused, when the join condition can't produce duplicate rows.
//
//	SELECT x.a FROM x LEFT JOIN (SELECT DISTINCT y.b FROM y) AS y ON x.b = y.b  ->  SELECT x.a FROM x
func EliminateJoins(expression exp.Expression) exp.Expression {
	for _, scope := range traverseScopeForOptimizer(expression) {
		joins := expressionsFor(scope.Expression, "joins")
		if len(joins) == 0 {
			continue
		}

		// If any columns in this scope aren't qualified, it's hard to determine whether a join is
		// unused, so skip the rule.
		if len(scope.UnqualifiedColumns()) > 0 {
			continue
		}

		// Reverse the joins so we can remove chains of unused joins.
		for i := len(joins) - 1; i >= 0; i-- {
			join := joins[i]
			if kind := joinText(join, "kind"); kind == "SEMI" || kind == "ANTI" {
				continue
			}
			alias := join.AliasOrName()
			if shouldEliminateJoin(scope, join, alias) {
				join.Pop()
				scope.RemoveSource(alias)
			}
		}
	}
	return expression
}

// shouldEliminateJoin ports _should_eliminate_join (optimizer/eliminate_joins.py:53-62).
func shouldEliminateJoin(scope *Scope, join exp.Expression, alias string) bool {
	innerSource, ok := scope.Sources[alias].(*Scope)
	if !ok || joinIsUsed(scope, join, alias) {
		return false
	}
	return (joinText(join, "side") == "LEFT" && isJoinedOnAllUniqueOutputs(innerSource, join)) ||
		(join.Arg("on") == nil && hasSingleOutputRow(innerSource))
}

// joinIsUsed ports _join_is_used (optimizer/eliminate_joins.py:65-75): a column outside the
// ON clause references the join.
func joinIsUsed(scope *Scope, join exp.Expression, alias string) bool {
	onColumns := map[exp.Expression]bool{}
	if on := asExpression(join.Arg("on")); on != nil {
		for _, column := range on.FindAll(exp.KindColumn) {
			onColumns[column] = true
		}
	}
	for _, column := range scope.SourceColumns(alias) {
		if !onColumns[column] {
			return true
		}
	}
	return false
}

// isJoinedOnAllUniqueOutputs ports _is_joined_on_all_unique_outputs
// (optimizer/eliminate_joins.py:78-85).
func isJoinedOnAllUniqueOutputs(scope *Scope, join exp.Expression) bool {
	uniqueOutputs := uniqueOutputs(scope)
	if len(uniqueOutputs) == 0 {
		return false
	}
	_, joinKeys, _ := joinCondition(join)
	for _, key := range joinKeys {
		delete(uniqueOutputs, key.Name())
	}
	return len(uniqueOutputs) == 0
}

// uniqueOutputs ports _unique_outputs (optimizer/eliminate_joins.py:88-114): the output columns
// of scope that must have a unique combination per row.
func uniqueOutputs(scope *Scope) map[string]bool {
	expr := scope.Expression
	namedSelects := func() map[string]bool {
		out := map[string]bool{}
		for _, name := range expr.NamedSelects() {
			out[name] = true
		}
		return out
	}
	if expr.Arg("distinct") != nil {
		return namedSelects()
	}

	if group := asExpression(expr.Arg("group")); group != nil {
		grouped := exprSetOf(group.Expressions())
		var groupedOutputs []exp.Expression
		unique := map[string]bool{}
		for _, sel := range expr.Selects() {
			output := sel.Unalias()
			if indexOfExpr(grouped, output) >= 0 {
				if indexOfExpr(groupedOutputs, output) < 0 {
					groupedOutputs = append(groupedOutputs, output)
				}
				unique[sel.AliasOrName()] = true
			}
		}
		// All the grouped expressions must be in the output.
		for _, g := range grouped {
			if indexOfExpr(groupedOutputs, g) < 0 {
				return map[string]bool{}
			}
		}
		return unique
	}

	if hasSingleOutputRow(scope) {
		return namedSelects()
	}
	return map[string]bool{}
}

// hasSingleOutputRow ports _has_single_output_row (optimizer/eliminate_joins.py:117-123).
func hasSingleOutputRow(scope *Scope) bool {
	expr := scope.Expression
	if expr.Kind() != exp.KindSelect {
		return false
	}
	allAgg := true
	for _, e := range expr.Selects() {
		if !e.Unalias().Is(exp.TraitAggFunc) {
			allAgg = false
			break
		}
	}
	return allAgg || isLimit1(scope) || expr.Arg("from_") == nil
}

// isLimit1 ports _is_limit_1 (optimizer/eliminate_joins.py:126-128).
func isLimit1(scope *Scope) bool {
	limit := asExpression(scope.Expression.Arg("limit"))
	if limit == nil || limit.Expr() == nil {
		return false
	}
	return limit.Expr().Text("this") == "1"
}

// joinCondition ports join_condition (optimizer/eliminate_joins.py:131-196): the join's source
// keys, join keys and remaining predicate.
func joinCondition(join exp.Expression) (sourceKey, joinKey []exp.Expression, on exp.Expression) {
	name := join.AliasOrName()
	on = asExpression(join.Arg("on"))
	if on == nil {
		on = booleanLiteral(true)
	}
	on = on.Copy()

	extractCondition := func(condition exp.Expression) {
		left, right := condition.Left().Unnest(), condition.Right().Unnest()
		leftTables, rightTables := columnTableNames(left, ""), columnTableNames(right, "")
		switch {
		case leftTables[name] && !rightTables[name]:
			joinKey = append(joinKey, left)
			sourceKey = append(sourceKey, right)
			condition.Replace(booleanLiteral(true))
		case rightTables[name] && !leftTables[name]:
			joinKey = append(joinKey, right)
			sourceKey = append(sourceKey, left)
			condition.Replace(booleanLiteral(true))
		}
	}

	// Find the join keys: in SELECT ... FROM x JOIN y ON x.a = y.b AND y.b > 1, y.b is the join
	// key and x.a the source key.
	switch {
	case normalized(on, false):
		if on.Kind() != exp.KindAnd {
			on = combine(exp.KindAnd, on, booleanLiteral(true))
		}
		for _, condition := range flatten(on, true) {
			if condition.Kind() == exp.KindEQ {
				extractCondition(condition)
			}
		}
	case normalized(on, true):
		var conditions []exp.Expression
		for _, condition := range flatten(on, true) {
			var parts []exp.Expression
			for _, part := range flatten(condition, true) {
				if part.Kind() == exp.KindEQ {
					parts = append(parts, part)
				}
			}
			if len(conditions) == 0 {
				conditions = parts
				continue
			}
			var temp []exp.Expression
			for _, p := range parts {
				var cs []exp.Expression
				for _, c := range conditions {
					if p.Equal(c) {
						cs = append(cs, c)
					}
				}
				if len(cs) > 0 {
					temp = append(temp, p)
					temp = append(temp, cs...)
				}
			}
			conditions = temp
		}
		for _, condition := range conditions {
			extractCondition(condition)
		}
	}
	return sourceKey, joinKey, on
}
