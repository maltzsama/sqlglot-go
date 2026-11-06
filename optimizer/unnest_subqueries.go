package optimizer

import exp "github.com/maltzsama/sqlglot-go/expressions"

// UnnestSubqueries ports unnest_subqueries (optimizer/unnest_subqueries.py:8-39): it rewrites
// predicates with subqueries into joins. Scalar subqueries become cross joins; correlated or
// vectorized subqueries become a grouped LEFT JOIN, so the join is not many to many.
//
//	SELECT * FROM x AS x WHERE (SELECT y.a AS a FROM y AS y WHERE x.a = y.a) = 1
//	  ->  SELECT * FROM x AS x LEFT JOIN (SELECT y.a AS a FROM y AS y WHERE TRUE GROUP BY y.a)
//	      AS _u_0 ON x.a = _u_0.a WHERE _u_0.a = 1
//
// divergence (Go-forced): where upstream builds a condition from a SQL string (an f-string fed
// to exp.condition or Select.select), this builds the node that string parses to, because the
// optimizer does not depend on the parser. The generated SQL is the same.
func UnnestSubqueries(expression exp.Expression) exp.Expression {
	nextAliasName := nameSequence("_u_")

	for _, scope := range traverseScopeForOptimizer(expression) {
		sel := scope.Expression
		parent := sel.ParentSelect()
		if parent == nil {
			continue
		}
		if external := scope.ExternalColumns(); len(external) > 0 {
			decorrelate(sel, parent, external, nextAliasName)
		} else if scope.ScopeType == ScopeTypeSubquery {
			unnestSubquery(sel, parent, nextAliasName)
		}
	}
	return expression
}

// findAncestorOf ports Expr.find_ancestor(*types) for a mix of kinds and traits.
func findAncestorOf(expression exp.Expression, targets ...any) exp.Expression {
	for ancestor := expression.Parent(); ancestor != nil; ancestor = ancestor.Parent() {
		for _, target := range targets {
			if matchesTarget(ancestor, target) {
				return ancestor
			}
		}
	}
	return nil
}

// replaceCondition ports _replace (optimizer/unnest_subqueries.py:327-328): it replaces
// expression with a copy of condition (exp.condition copies an Expr).
func replaceCondition(expression, condition exp.Expression) exp.Expression {
	return expression.Replace(condition.Copy())
}

// otherOperand ports _other_operand (optimizer/unnest_subqueries.py:331-345).
func otherOperand(expression exp.Expression) exp.Expression {
	if expression == nil {
		return nil
	}
	switch expression.Kind() {
	case exp.KindIn:
		return expression.This()
	case exp.KindAny, exp.KindAll:
		return otherOperand(expression.Parent())
	}
	if expression.Is(exp.TraitBinary) {
		switch expression.Left().Kind() {
		case exp.KindSubquery, exp.KindAny, exp.KindExists, exp.KindAll:
			return expression.Right()
		}
		return expression.Left()
	}
	return nil
}

func column(name, table string) exp.Expression {
	return exp.Column_(name, table, nil, nil, nil)
}

// lambdaX builds `_x -> body`; the parser turns a reference to the lambda parameter into an
// Identifier, so the body's `_x` columns become identifiers the same way.
func lambdaX(body exp.Expression) exp.Expression {
	body = body.Transform(func(node exp.Expression) exp.Expression {
		if node.Kind() == exp.KindColumn && node.Text("table") == "" && node.Name() == "_x" {
			return exp.ToIdentifier("_x")
		}
		return node
	}, false)
	return exp.Lambda(exp.Args{"this": body, "expressions": []exp.Expression{exp.ToIdentifier("_x")}})
}

// unnestSubquery ports unnest (optimizer/unnest_subqueries.py:42-160).
func unnestSubquery(sel, parentSelect exp.Expression, nextAliasName func() string) {
	if len(sel.Selects()) > 1 {
		return
	}

	predicate := findAncestorOf(sel, exp.TraitCondition)
	if predicate == nil ||
		// Do not unnest subqueries inside table-valued functions such as FROM GENERATE_SERIES(...),
		// FROM UNNEST(...) etc in order to preserve join order.
		(predicate.Is(exp.TraitFunc) && predicate.Parent() != nil &&
			(predicate.Parent().Kind() == exp.KindTable || predicate.Parent().Kind() == exp.KindFrom || predicate.Parent().Kind() == exp.KindJoin)) ||
		parentSelect != predicate.ParentSelect() ||
		parentSelect.Arg("from_") == nil ||
		// NOT IN has three-valued semantics that the LEFT-JOIN-anti rewrite doesn't preserve: a
		// NULL in the subquery makes NOT IN evaluate to NULL for every outer row.
		(predicate.Kind() == exp.KindIn && predicate.Parent() != nil && predicate.Parent().Kind() == exp.KindNot) {
		return
	}

	if sel.Is(exp.TraitSetOperation) {
		innerAlias := nextAliasName()
		var projections []exp.Expression
		for _, s := range sel.Selects() {
			projections = append(projections, exp.AliasExpr(column(s.AliasOrName(), innerAlias), s.AliasOrName(), false))
		}
		sel = exp.SelectQuery(projections...).From_(sel.Subquery(innerAlias, true), true)
	}

	alias := nextAliasName()
	clause := findAncestorOf(predicate, exp.KindHaving, exp.KindWhere, exp.KindJoin)

	// This subquery returns a scalar and can just be converted to a cross join.
	if predicate.Kind() != exp.KindIn && predicate.Kind() != exp.KindAny {
		col := column(sel.Selects()[0].AliasOrName(), alias)

		var clauseParentSelect exp.Expression
		if clause != nil {
			clauseParentSelect = clause.ParentSelect()
		}

		hasAgg := false
		for _, projection := range parentSelect.Selects() {
			if findInScope(projection, exp.TraitAggFunc) != nil {
				hasAgg = true
				break
			}
		}
		if (clause != nil && clause.Kind() == exp.KindHaving && clauseParentSelect == parentSelect) ||
			((clause == nil || clauseParentSelect != parentSelect) && (truthyArg(parentSelect.Arg("group")) || hasAgg)) {
			col = exp.Max(exp.Args{"this": col})
		} else if sel.Parent() == nil || sel.Parent().Kind() != exp.KindSubquery {
			return
		}

		joinType := "CROSS"
		var on []exp.Expression
		if predicate.Kind() == exp.KindExists {
			// If a subquery returns no rows, cross-joining against it incorrectly eliminates all
			// rows from the parent query. Therefore, we use a LEFT JOIN that always matches (ON
			// TRUE), then check for non-NULL column values to determine whether the subquery
			// contained rows.
			col = col.Is_(exp.Null()).Not_(true)
			joinType = "LEFT"
			on = []exp.Expression{exp.True_()}
		}

		replaceCondition(sel.Parent(), col)
		parentSelect.Join(sel, exp.JoinOptions{On: on, JoinType: joinType, JoinAlias: alias})
		return
	}

	if findInScope(sel, exp.KindLimit) != nil || findInScope(sel, exp.KindOffset) != nil {
		return
	}

	if predicate.Kind() == exp.KindAny {
		predicate = findAncestorOf(predicate, exp.KindEQ)
		if predicate == nil || parentSelect != predicate.ParentSelect() {
			return
		}
	}

	col := otherOperand(predicate)
	value := sel.Selects()[0]

	joinKey := column(value.Alias(), alias)
	joinKeyNotNull := joinKey.Is_(exp.Null()).Not_(true)

	if clause != nil && clause.Kind() == exp.KindJoin {
		replaceCondition(predicate, exp.True_())
		parentSelect.Where(true, false, joinKeyNotNull)
	} else {
		replaceCondition(predicate, joinKeyNotNull)
	}

	if group := asExpression(sel.Arg("group")); group != nil {
		if !sameExpressionSet([]exp.Expression{value.This()}, group.Expressions()) {
			sel = exp.SelectQuery(exp.AliasExpr(column(value.Alias(), "_q"), value.Alias(), false)).
				From_(sel.Subquery("_q", false), false).
				GroupBy(true, false, column(value.Alias(), "_q"))
		}
	} else if findInScope(value.This(), exp.TraitAggFunc) == nil {
		sel = sel.GroupBy(true, false, value.This())
	}

	parentSelect.Join(sel, exp.JoinOptions{
		On:        []exp.Expression{col.Eq(joinKey)},
		JoinType:  "LEFT",
		JoinAlias: alias,
	})
}

// keyAlias is one entry of decorrelate's key_aliases dict, keyed by expression equality.
type keyAlias struct {
	key   exp.Expression
	alias string
}

func indexOfExpr(list []exp.Expression, e exp.Expression) int {
	for i, item := range list {
		if item.Equal(e) {
			return i
		}
	}
	return -1
}

// sameExpressionSet reports whether a and b hold the same expressions, compared structurally,
// as Python compares two sets of Expr.
func sameExpressionSet(a, b []exp.Expression) bool {
	for _, item := range a {
		if indexOfExpr(b, item) < 0 {
			return false
		}
	}
	for _, item := range b {
		if indexOfExpr(a, item) < 0 {
			return false
		}
	}
	return true
}

type decorrelateKey struct {
	key, column, predicate exp.Expression
}

// decorrelate ports decorrelate (optimizer/unnest_subqueries.py:163-324).
func decorrelate(sel, parentSelect exp.Expression, externalColumns []exp.Expression, nextAliasName func() string) {
	where := asExpression(sel.Arg("where"))

	if where == nil || where.Find(exp.KindOr) != nil || sel.Find(exp.KindLimit) != nil || sel.Find(exp.KindOffset) != nil {
		return
	}

	tableAlias := nextAliasName()
	var keys []decorrelateKey

	// For all external columns in the where statement, find the relevant predicate keys to
	// convert it into a join.
	for _, col := range externalColumns {
		if col.FindAncestor(exp.KindWhere) != where {
			return
		}

		// The predicate is replaced with TRUE below, which is only sound if its result flows
		// into the WHERE through conjunctions; wrappers like NOT would invert that TRUE.
		predicate := findAncestorOf(col, exp.TraitPredicate)
		var ancestor exp.Expression
		if predicate != nil {
			ancestor = predicate.Parent()
		}
		for ancestor != nil && (ancestor.Kind() == exp.KindAnd || ancestor.Kind() == exp.KindParen) {
			ancestor = ancestor.Parent()
		}

		if ancestor != where {
			return
		}

		if !predicate.Is(exp.TraitBinary) {
			return
		}
		key := predicate.Left()
		for _, node := range predicate.Left().Walk() {
			if node == col {
				key = predicate.Right()
				break
			}
		}

		keys = append(keys, decorrelateKey{key, col, predicate})
	}

	hasEQ := false
	for _, k := range keys {
		if k.predicate.Kind() == exp.KindEQ {
			hasEQ = true
			break
		}
	}
	if !hasEQ {
		return
	}

	isSubqueryProjection := false
	for _, projection := range parentSelect.Selects() {
		if node := projection.Unalias(); node.Kind() == exp.KindSubquery && node == sel.Parent() {
			isSubqueryProjection = true
			break
		}
	}

	value := sel.Selects()[0]
	var keyAliases []keyAlias
	var groupBy []exp.Expression
	aliasOf := func(key exp.Expression) (string, bool) {
		for _, entry := range keyAliases {
			if entry.key.Equal(key) {
				return entry.alias, true
			}
		}
		return "", false
	}

	for _, k := range keys {
		// If we filter on the value of the subquery, it needs to be unique.
		if k.key.Equal(value.This()) {
			if _, ok := aliasOf(k.key); ok {
				for i := range keyAliases {
					if keyAliases[i].key.Equal(k.key) {
						keyAliases[i].alias = value.Alias()
					}
				}
			} else {
				keyAliases = append(keyAliases, keyAlias{k.key, value.Alias()})
			}
			groupBy = append(groupBy, k.key)
		} else {
			if _, ok := aliasOf(k.key); !ok {
				keyAliases = append(keyAliases, keyAlias{k.key, nextAliasName()})
			}
			// All predicates that are equalities must also be in the unique so that we don't
			// do a many to many join.
			if k.predicate.Kind() == exp.KindEQ && indexOfExpr(groupBy, k.key) < 0 {
				groupBy = append(groupBy, k.key)
			}
		}
	}

	parentPredicate := findAncestorOf(sel, exp.TraitPredicate)

	// When the subquery is embedded inside a function (e.g. COALESCE, TRIM) in the SELECT
	// list, the ancestor chain contains no Predicate node AND the subquery is not a direct
	// projection.
	if parentPredicate == nil && !isSubqueryProjection {
		return
	}

	// If the value of the subquery is not an agg or a key, we need to collect it into an
	// array so that it can be grouped. For subquery projections, we use a MAX aggregation.
	aggFunc := exp.ArrayAgg
	if isSubqueryProjection {
		aggFunc = exp.Max
	}
	if value.Kind() != exp.KindSubquery && findInScope(value, exp.TraitAggFunc) == nil && indexOfExpr(groupBy, value.This()) < 0 {
		sel.SelectExprs(false, false, exp.AliasExpr(aggFunc(exp.Args{"this": value.This()}), value.Alias(), false, false))
	}

	// Exists queries should not have any selects as it only checks if there are any rows; all
	// selects will be added by the optimizer and only used for join keys.
	isExists := parentPredicate != nil && parentPredicate.Kind() == exp.KindExists
	if isExists {
		sel.Set("expressions", []exp.Expression{})
	}

	for _, entry := range keyAliases {
		if indexOfExpr(groupBy, entry.key) >= 0 {
			// Add all keys to the projections of the subquery so that we can use it as a join key.
			if isExists || !entry.key.Equal(value.This()) {
				// Upstream: select.select(f"{key} AS {alias}"), a parsed copy of the key.
				sel.SelectExprs(true, false, exp.AliasExpr(entry.key.Copy(), entry.alias, false))
			}
		} else {
			sel.SelectExprs(true, false, exp.AliasExpr(aggFunc(exp.Args{"this": entry.key.Copy()}), entry.alias, false, false))
		}
	}

	alias := column(value.Alias(), tableAlias)
	other := otherOperand(parentPredicate)
	var opType exp.Kind
	if parentPredicate != nil && parentPredicate.Parent() != nil {
		opType = parentPredicate.Parent().Kind()
	}

	switch {
	case isExists:
		alias = column(keyAliases[0].alias, tableAlias)
		// Upstream: f"NOT {alias} IS NULL".
		parentPredicate = replaceCondition(parentPredicate, alias.Is_(exp.Null()).Not_(true))
	case parentPredicate != nil && parentPredicate.Kind() == exp.KindAll:
		predicate := exp.New(opType, exp.Args{"this": other, "expression": column("_x", "")})
		// Upstream: f"ARRAY_ALL({alias}, _x -> {predicate})".
		parentPredicate = replaceCondition(parentPredicate.Parent(),
			exp.ArrayAll(exp.Args{"this": alias, "expression": lambdaX(predicate.Copy())}))
	case parentPredicate != nil && parentPredicate.Kind() == exp.KindAny:
		if indexOfExpr(groupBy, value.This()) >= 0 {
			predicate := exp.New(opType, exp.Args{"this": other, "expression": alias})
			parentPredicate = replaceCondition(parentPredicate.Parent(), predicate)
		} else {
			predicate := exp.New(opType, exp.Args{"this": other, "expression": column("_x", "")})
			// Upstream: f"ARRAY_ANY({alias}, _x -> {predicate})".
			parentPredicate = replaceCondition(parentPredicate,
				exp.ArrayAny(exp.Args{"this": alias, "expression": lambdaX(predicate.Copy())}))
		}
	case parentPredicate != nil && parentPredicate.Kind() == exp.KindIn:
		if indexOfExpr(groupBy, value.This()) >= 0 {
			// Upstream: f"{other} = {alias}".
			parentPredicate = replaceCondition(parentPredicate, exp.New(exp.KindEQ, exp.Args{"this": other.Copy(), "expression": alias}))
		} else {
			// Upstream: f"ARRAY_ANY({alias}, _x -> _x = {parent_predicate.this})".
			body := exp.New(exp.KindEQ, exp.Args{"this": column("_x", ""), "expression": parentPredicate.This().Copy()})
			parentPredicate = replaceCondition(parentPredicate,
				exp.ArrayAny(exp.Args{"this": alias, "expression": lambdaX(body)}))
		}
	default:
		var replacement exp.Expression = alias
		if isSubqueryProjection && sel.Parent().Alias() != "" {
			replacement = exp.AliasExpr(alias, sel.Parent().Alias(), false)
		}

		// COUNT always returns 0 on empty datasets, so we need take that into consideration
		// here by transforming all counts into 0 and using that as the coalesced value.
		if findInScope(value, exp.KindCount) != nil {
			removeAggs := func(node exp.Expression) exp.Expression {
				if node.Kind() == exp.KindCount {
					return exp.LiteralNumber(0)
				}
				if node.Is(exp.TraitAggFunc) {
					return exp.Null()
				}
				return node
			}
			replacement = exp.Coalesce(exp.Args{
				"this":        replacement,
				"expressions": []exp.Expression{value.This().Transform(removeAggs, true)},
			})
		}

		sel.Parent().Replace(replacement)
	}

	for _, k := range keys {
		k.predicate.Replace(exp.True_())
		keyAliasName, _ := aliasOf(k.key)
		nested := column(keyAliasName, tableAlias)

		if isSubqueryProjection {
			k.key.Replace(nested)
			if k.predicate.Kind() != exp.KindEQ {
				parentSelect.Where(true, false, k.predicate)
			}
			continue
		}

		if indexOfExpr(groupBy, k.key) >= 0 {
			k.key.Replace(nested)
		} else {
			// Built as AST rather than a SQL string, because dialect-specific operators such as
			// Postgres' `@>` can't be round-tripped through the default dialect's parser.
			k.key.Replace(exp.ToIdentifier("_x"))
			right := exp.ArrayAny(exp.Args{
				"this":       nested,
				"expression": exp.Lambda(exp.Args{"this": k.predicate.Copy(), "expressions": []exp.Expression{exp.ToIdentifier("_x")}}),
			})
			parentPredicate = replaceCondition(parentPredicate,
				exp.Paren(exp.Args{"this": exp.And_(parentPredicate.Copy(), right)}))
		}
	}

	var on []exp.Expression
	for _, k := range keys {
		if k.predicate.Kind() == exp.KindEQ {
			on = append(on, k.predicate)
		}
	}
	parentSelect.Join(sel.GroupBy(true, false, groupBy...), exp.JoinOptions{
		On:        on,
		JoinType:  "LEFT",
		JoinAlias: tableAlias,
	})
}
