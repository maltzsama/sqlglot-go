package optimizer

import (
	"sort"

	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// MergeSubqueries ports merge_subqueries (optimizer/merge_subqueries.py:17-52): it merges
// derived tables into the outer query, and CTEs too when they are selected from only once.
// With leaveTablesIsolated, it does not merge when that would put several table selects in
// a single query.
//
//	SELECT a FROM (SELECT x.a FROM x) CROSS JOIN y  ->  SELECT x.a FROM x CROSS JOIN y
func MergeSubqueries(expression exp.Expression, leaveTablesIsolated bool) exp.Expression {
	// Shared across both passes so the scope tree is only built once, as long as mergeCTEs
	// doesn't mutate the AST; if it does, the scopes it was given are no longer valid, so the
	// scope tree needs to be rebuilt before mergeDerivedTables runs.
	scopes := traverseScopeForOptimizer(expression)
	merged := mergeCTEs(expression, leaveTablesIsolated, scopes)
	if merged {
		scopes = traverseScopeForOptimizer(expression)
	}
	return mergeDerivedTables(expression, leaveTablesIsolated, scopes)
}

// unmergableArgs ports UNMERGABLE_ARGS (optimizer/merge_subqueries.py:55-63): if a derived
// table has any of these Select args, it can't be merged.
var unmergableArgs = func() map[string]bool {
	mergeable := map[string]bool{"expressions": true, "from_": true, "joins": true, "where": true, "order": true, "hint": true}
	out := map[string]bool{}
	for _, key := range exp.ArgKeys(exp.KindSelect) {
		if !mergeable[key] {
			out[key] = true
		}
	}
	return out
}()

// safeToReplaceUnwrapped ports SAFE_TO_REPLACE_UNWRAPPED (optimizer/merge_subqueries.py:66-74):
// outer projections of these types can be replaced without parentheses, because the
// precedence won't be altered.
func safeToReplaceUnwrapped(expression exp.Expression) bool {
	switch expression.Kind() {
	case exp.KindColumn, exp.KindEQ, exp.KindNEQ, exp.KindParen:
		return true
	}
	return expression.Is(exp.TraitFunc)
}

type cteSelection struct {
	outer, inner *Scope
	table        exp.Expression
}

// mergeCTEs ports merge_ctes (optimizer/merge_subqueries.py:77-113).
func mergeCTEs(expression exp.Expression, leaveTablesIsolated bool, scopes []*Scope) bool {
	// All places where we select from CTEs, keyed on the CTE scope so we can detect CTEs that
	// are selected from multiple times (in first-seen order, as upstream's dict).
	var order []*Scope
	selections := map[*Scope][]cteSelection{}
	for _, outerScope := range scopes {
		selected := outerScope.SelectedSources()
		for _, name := range outerScope.SelectedSourceNames() {
			entry := selected[name]
			innerScope, ok := entry.Source.(*Scope)
			if !ok || !innerScope.IsCTE() {
				continue
			}
			if _, seen := selections[innerScope]; !seen {
				order = append(order, innerScope)
			}
			selections[innerScope] = append(selections[innerScope], cteSelection{outerScope, innerScope, entry.Node})
		}
	}

	merged := false
	for _, key := range order {
		if len(selections[key]) != 1 {
			continue
		}
		selection := selections[key][0]
		outerScope, innerScope, table := selection.outer, selection.inner, selection.table
		fromOrJoin := table.FindAncestor(exp.KindFrom, exp.KindJoin)
		if fromOrJoin == nil {
			continue
		}
		if mergeable(outerScope, innerScope, leaveTablesIsolated, fromOrJoin) {
			alias := table.AliasOrName()
			renameInnerSources(outerScope, innerScope, alias)
			mergeFrom(outerScope, innerScope, table, alias)
			mergeExpressions(outerScope, innerScope, alias)
			mergeOrder(outerScope, innerScope)
			mergeJoins(outerScope, innerScope, fromOrJoin)
			mergeWhere(outerScope, innerScope, fromOrJoin)
			mergeHints(outerScope, innerScope)
			popCTE(innerScope)
			outerScope.clearCache()
			merged = true
		}
	}
	return merged
}

// mergeDerivedTables ports merge_derived_tables (optimizer/merge_subqueries.py:116-141).
func mergeDerivedTables(expression exp.Expression, leaveTablesIsolated bool, scopes []*Scope) exp.Expression {
	for _, outerScope := range scopes {
		for _, subquery := range outerScope.DerivedTables() {
			fromOrJoin := subquery.FindAncestor(exp.KindFrom, exp.KindJoin)
			if fromOrJoin == nil {
				continue
			}
			alias := subquery.AliasOrName()
			innerScope, ok := outerScope.Sources[alias].(*Scope)
			if !ok {
				continue
			}
			if mergeable(outerScope, innerScope, leaveTablesIsolated, fromOrJoin) {
				renameInnerSources(outerScope, innerScope, alias)
				mergeFrom(outerScope, innerScope, subquery, alias)
				mergeExpressions(outerScope, innerScope, alias)
				mergeOrder(outerScope, innerScope)
				mergeJoins(outerScope, innerScope, fromOrJoin)
				mergeWhere(outerScope, innerScope, fromOrJoin)
				mergeHints(outerScope, innerScope)
				outerScope.clearCache()
			}
		}
	}
	return expression
}

// selectIsStar ports Select.is_star: any projection is a star.
func selectIsStar(sel exp.Expression) bool {
	for _, projection := range sel.Expressions() {
		if projection.IsStar() {
			return true
		}
	}
	return false
}

// mergeable ports _mergeable (optimizer/merge_subqueries.py:144-335): whether the inner
// select can be merged into the outer query.
func mergeable(outerScope, innerScope *Scope, leaveTablesIsolated bool, fromOrJoin exp.Expression) bool {
	innerSelect := innerScope.Expression.Unnest()
	outer := outerScope.Expression
	innerName := fromOrJoin.AliasOrName()
	if fromOrJoin.Kind() == exp.KindFrom {
		// From.alias_or_name is its source's alias_or_name.
		innerName = fromOrJoin.This().AliasOrName()
	}

	// A window function's result depends on the full row set it sees, so merging the subquery
	// into the outer query is unsafe when the outer query filters or joins (WHERE/JOIN), which
	// changes that row set, or a window column is referenced in an operation that isn't
	// pushed down (GROUP BY, ORDER BY, HAVING, aggregate).
	windowProjectionBlocksMerge := func(windowAliases map[string]bool) bool {
		if len(windowAliases) == 0 {
			return false
		}
		if truthyArg(outer.Arg("where")) || truthyArg(outer.Arg("joins")) {
			return true
		}
		for _, col := range outerScope.Columns() {
			if col.Text("table") == innerName && windowAliases[col.Name()] &&
				findAncestorOf(col, exp.KindGroup, exp.KindOrder, exp.KindHaving, exp.TraitAggFunc) != nil {
				return true
			}
		}
		return false
	}

	// A numeric-literal projection referenced in GROUP BY can't be inlined, because a bare
	// integer literal is positional. A reference that is itself a top-level GROUP BY item can
	// merge as the ordinal of the outer projection that selects it; any other reference, e.g.,
	// ROLLUP / CUBE / GROUPING SETS, tuples, expressions, etc, blocks the merge, since
	// ordinals aren't universally supported there, e.g., Presto / Trino only accept columns.
	literalGroupUnmergeable := func(numberLiteralAliases map[string]bool) bool {
		group := asExpression(outer.Arg("group"))
		if group == nil || len(numberLiteralAliases) == 0 {
			return false
		}
		grouped := map[string]bool{}
		topLevel := map[exp.Expression]bool{}
		for _, e := range group.Expressions() {
			topLevel[e.Unnest()] = true
		}
		for _, col := range group.FindAll(exp.KindColumn) {
			if col.Text("table") != innerName || !numberLiteralAliases[col.Name()] {
				continue
			}
			if !topLevel[col] {
				return true
			}
			grouped[col.Name()] = true
		}
		if len(grouped) == 0 {
			return false
		}
		projected := map[string]bool{}
		for _, s := range outer.Selects() {
			if unaliased := s.Unalias(); unaliased.Kind() == exp.KindColumn && unaliased.Text("table") == innerName {
				projected[unaliased.Name()] = true
			}
		}
		for name := range grouped {
			if !projected[name] {
				return true
			}
		}
		return false
	}

	// All columns from the inner select in the ON clause must be from the first FROM table.
	//
	// That is, this can be merged:
	//     SELECT * FROM x JOIN (SELECT y.a AS a FROM y JOIN z) AS q ON x.a = q.a
	// But this can't:
	//     SELECT * FROM x JOIN (SELECT z.a AS a FROM y JOIN z) AS q ON x.a = q.a
	outerSelectJoinsOnInnerSelectJoin := func(projections map[string]exp.Expression) bool {
		if fromOrJoin.Kind() != exp.KindJoin {
			return false
		}
		on := asExpression(fromOrJoin.Arg("on"))
		if on == nil {
			return false
		}
		innerFrom := asExpression(innerScope.Expression.Arg("from_"))
		if innerFrom == nil {
			return false
		}
		innerFromTable := innerFrom.This().AliasOrName()
		for _, c := range on.FindAll(exp.KindColumn) {
			if c.Text("table") != innerName {
				continue
			}
			for _, col := range projections[c.Name()].FindAll(exp.KindColumn) {
				if col.Text("table") != innerFromTable {
					return true
				}
			}
		}
		return false
	}

	// Recursive CTEs look like this:
	//     WITH RECURSIVE cte AS (
	//       SELECT * FROM x  <-- inner scope
	//       UNION ALL
	//       SELECT * FROM cte  <-- outer scope
	//     )
	isRecursive := func() bool {
		cte := innerScope.Expression.Parent()
		for node := outer.Parent(); node != nil; node = node.Parent() {
			if node == cte {
				return true
			}
		}
		return false
	}

	// A numeric-literal projection under a bare ORDER BY key can't merge (it would become
	// positional).
	literalInOrderBy := func(numberLiteralAliases map[string]bool) bool {
		orderNode := asExpression(outer.Arg("order"))
		if orderNode == nil {
			return false
		}
		for _, o := range orderNode.Expressions() {
			if key := o.This().Unnest(); key.Kind() == exp.KindColumn && key.Text("table") == innerName && numberLiteralAliases[key.Name()] {
				return true
			}
		}
		return false
	}

	if outer.Kind() != exp.KindSelect || selectIsStar(outer) || innerSelect.Kind() != exp.KindSelect {
		return false
	}
	for key := range unmergableArgs {
		if truthyArg(innerSelect.Arg(key)) {
			return false
		}
	}
	isJoin := fromOrJoin.Kind() == exp.KindJoin
	if innerSelect.Arg("from_") == nil ||
		len(outerScope.Pivots()) > 0 ||
		(leaveTablesIsolated && len(outerScope.SelectedSources()) > 1) ||
		(isJoin && truthyArg(innerSelect.Arg("joins"))) {
		return false
	}
	if isJoin && truthyArg(innerSelect.Arg("where")) {
		switch joinText(fromOrJoin, "side") {
		case "FULL", "LEFT", "RIGHT":
			return false
		}
	}
	if fromOrJoin.Kind() == exp.KindFrom && truthyArg(innerSelect.Arg("where")) {
		for _, j := range expressionsFor(outer, "joins") {
			if side := joinText(j, "side"); side == "FULL" || side == "RIGHT" {
				return false
			}
		}
	}
	if (truthyArg(innerSelect.Arg("order")) && outerScope.IsUnion()) ||
		(len(innerSelect.Expressions()) > 0 && innerSelect.Expressions()[0].Kind() == exp.KindQueryTransform) {
		return false
	}

	// Single pass over the projections, precomputing what the checks below need.
	windowAliases := map[string]bool{}
	numberLiteralAliases := map[string]bool{}
	projections := map[string]exp.Expression{}

	for _, s := range innerSelect.Selects() {
		name := s.AliasOrName()
		projections[name] = s
		if s.Unalias().IsNumber() {
			numberLiteralAliases[name] = true
		}
		for _, node := range s.Walk() {
			if node.Is(exp.TraitAggFunc) || node.Kind() == exp.KindSelect || node.Kind() == exp.KindExplode {
				return false
			}
			if node.Kind() == exp.KindWindow {
				windowAliases[name] = true
			}
		}
	}

	return !outerSelectJoinsOnInnerSelectJoin(projections) &&
		!windowProjectionBlocksMerge(windowAliases) &&
		!literalGroupUnmergeable(numberLiteralAliases) &&
		!literalInOrderBy(numberLiteralAliases) &&
		!(innerScope.IsCTE() && isRecursive())
}

// renameInnerSources ports _rename_inner_sources (optimizer/merge_subqueries.py:338-366): it
// renames any sources in the inner query that conflict with names in the outer query.
func renameInnerSources(outerScope, innerScope *Scope, alias string) {
	innerTaken := innerScope.SelectedSources()
	outerTaken := outerScope.SelectedSources()

	taken := map[string]any{}
	var conflicts []string
	for name := range outerTaken {
		taken[name] = true
		if _, ok := innerTaken[name]; ok && name != alias {
			conflicts = append(conflicts, name)
		}
	}
	for name := range innerTaken {
		taken[name] = true
	}
	// Upstream iterates a set; the new names don't depend on the order, so sort for a stable
	// result.
	sort.Strings(conflicts)

	for _, conflict := range conflicts {
		newName := findNewName(taken, conflict)

		source := innerTaken[conflict].Node
		newAlias := exp.ToIdentifier(newName)

		switch {
		case source.Kind() == exp.KindTable && source.Alias() != "":
			source.Set("alias", exp.TableAlias(exp.Args{"this": newAlias}))
		case source.Kind() == exp.KindTable:
			source.Replace(exp.AliasExpr(source.Copy(), newAlias, false))
		case source.Parent() != nil && source.Parent().Kind() == exp.KindSubquery:
			source.Parent().Set("alias", exp.TableAlias(exp.Args{"this": newAlias}))
		}

		for _, col := range innerScope.SourceColumns(conflict) {
			col.Set("table", exp.ToIdentifier(newName))
		}

		old := conflict
		innerScope.RenameSource(&old, newName)
	}
}

// mergeFrom ports _merge_from (optimizer/merge_subqueries.py:369-389): it merges the FROM
// clause of the inner query into the outer query.
func mergeFrom(outerScope, innerScope *Scope, nodeToReplace exp.Expression, alias string) {
	newSubquery := asExpression(innerScope.Expression.Arg("from_")).This()
	newSubquery.Set("joins", nodeToReplace.Arg("joins"))
	nodeToReplace.Replace(newSubquery)
	for _, joinHint := range outerScope.JoinHints() {
		for _, table := range joinHint.FindAll(exp.KindTable) {
			if table.AliasOrName() == nodeToReplace.AliasOrName() {
				table.Set("this", exp.ToIdentifier(newSubquery.AliasOrName()))
			}
		}
	}
	outerScope.RemoveSource(alias)
	outerScope.AddSource(newSubquery.AliasOrName(), innerScope.Sources[newSubquery.AliasOrName()])
}

// mergeJoins ports _merge_joins (optimizer/merge_subqueries.py:392-415): it merges the JOIN
// clauses of the inner query into the outer query, keeping the join order.
func mergeJoins(outerScope, innerScope *Scope, fromOrJoin exp.Expression) {
	var newJoins []exp.Expression
	for _, join := range expressionsFor(innerScope.Expression, "joins") {
		newJoins = append(newJoins, join)
		outerScope.AddSource(join.AliasOrName(), innerScope.Sources[join.AliasOrName()])
	}
	if len(newJoins) == 0 {
		return
	}

	outerJoins := expressionsFor(outerScope.Expression, "joins")
	position := 0
	if fromOrJoin.Kind() != exp.KindFrom {
		for i, join := range outerJoins {
			if join == fromOrJoin {
				position = i + 1
				break
			}
		}
	}
	merged := make([]exp.Expression, 0, len(outerJoins)+len(newJoins))
	merged = append(merged, outerJoins[:position]...)
	merged = append(merged, newJoins...)
	merged = append(merged, outerJoins[position:]...)
	outerScope.Expression.Set("joins", merged)
}

// mergeExpressions ports _merge_expressions (optimizer/merge_subqueries.py:418-485): it
// replaces outer columns that reference the inner query with the inner projections.
func mergeExpressions(outerScope, innerScope *Scope, alias string) {
	// Collect all columns that reference the alias of the inner query.
	outerColumns := map[string][]exp.Expression{}
	for _, col := range outerScope.Columns() {
		if col.Text("table") == alias {
			outerColumns[col.Name()] = append(outerColumns[col.Name()], col)
		}
	}

	group := asExpression(outerScope.Expression.Arg("group"))

	// Replace columns with the projection expression in the inner query.
	for _, expression := range innerScope.Expression.Expressions() {
		projectionName := expression.AliasOrName()
		if projectionName == "" {
			continue
		}
		columnsToReplace := outerColumns[projectionName]
		if len(columnsToReplace) == 0 {
			continue
		}

		expression = expression.Unalias()
		mustWrapExpression := !safeToReplaceUnwrapped(expression)

		isNumber := expression.IsNumber()
		last := len(columnsToReplace) - 1

		groupOrdinal := 0
		if isNumber && outerScope.Expression.Kind() == exp.KindSelect {
			// Find the ordinal of the outer SELECT that references this inner projection.
			for j, s := range outerScope.Expression.Selects() {
				if unaliased := s.Unalias(); unaliased.Kind() == exp.KindColumn &&
					unaliased.Text("table") == alias && unaliased.Name() == projectionName {
					groupOrdinal = j + 1
					break
				}
			}
		}

		for i, col := range columnsToReplace {
			parent := col.Parent()

			// A numeric-literal projection can't be inlined into a top-level GROUP BY item
			// (positional context): canonicalize to the projection's ordinal to match qualify.
			// _mergeable guarantees the ordinal exists.
			if isNumber && group != nil {
				replaced := false
				for _, item := range group.Expressions() {
					if item.Unnest() == col {
						item.Replace(exp.LiteralNumber(groupOrdinal))
						replaced = true
						break
					}
				}
				if replaced {
					continue
				}
			}

			// Ensures we don't alter the intended operator precedence if there's additional
			// context surrounding the outer expression (i.e. it's not a simple projection).
			if parent != nil && (parent.Is(exp.TraitUnary) || parent.Is(exp.TraitBinary)) && mustWrapExpression {
				expression = exp.Paren(exp.Args{"this": expression})
			}

			// Make sure we do not accidentally change the name of the column.
			if parent != nil && parent.Kind() == exp.KindSelect && col.Name() != expression.Name() {
				// exp.alias_ copies its expression (copy=True).
				expression = exp.AliasExpr(expression.Copy(), col.Name(), false)
			}

			// Skip the expensive deep copy for the last reference since the inner query is
			// about to be removed, so we can move the expression directly.
			if i < last {
				col.Replace(expression.Copy())
			} else {
				col.Replace(expression)
			}
		}
	}
}

// joinOnAppend ports Join.on(*expressions, copy=False) with append=True
// (expressions/query.py:708-750): it ANDs conditions into the join's ON, and a CROSS join
// stops being one.
func joinOnAppend(join exp.Expression, conditions ...exp.Expression) {
	all := []exp.Expression{}
	if existing := asExpression(join.Arg("on")); existing != nil {
		all = append(all, existing)
	}
	all = append(all, conditions...)
	join.Set("on", exp.And_(all...))
	if joinText(join, "kind") == "CROSS" {
		join.Set("kind", nil)
	}
}

// mergeWhere ports _merge_where (optimizer/merge_subqueries.py:488-520): it merges the WHERE
// clause of the inner query into the outer query, or into the ON of an outer join when it only
// references sources joined up to that point.
func mergeWhere(outerScope, innerScope *Scope, fromOrJoin exp.Expression) {
	where := asExpression(innerScope.Expression.Arg("where"))
	if where == nil || where.This() == nil {
		return
	}

	expression := outerScope.Expression

	if fromOrJoin.Kind() == exp.KindJoin {
		// Merge predicates from an outer join to the ON clause if it only has columns that
		// are already joined.
		sources := map[string]bool{}
		if from := asExpression(expression.Arg("from_")); from != nil {
			sources[from.This().AliasOrName()] = true
		}
		for _, join := range expressionsFor(expression, "joins") {
			source := join.AliasOrName()
			sources[source] = true
			if source == fromOrJoin.AliasOrName() {
				break
			}
		}

		subset := true
		for name := range columnTableNames(where.This(), "") {
			if !sources[name] {
				subset = false
				break
			}
		}
		if subset {
			joinOnAppend(fromOrJoin, where.This())
			return
		}
	}

	expression.Where(true, false, where.This())
}

// mergeOrder ports _merge_order (optimizer/merge_subqueries.py:523-545): the inner ORDER BY
// moves to the outer query when that query has nothing that would reorder its rows.
func mergeOrder(outerScope, innerScope *Scope) {
	innerOrder := asExpression(innerScope.Expression.Arg("order"))
	if innerOrder == nil {
		return
	}
	for _, arg := range []string{"group", "distinct", "having", "order"} {
		if truthyArg(outerScope.Expression.Arg(arg)) {
			return
		}
	}
	if len(outerScope.SelectedSources()) != 1 {
		return
	}
	for _, expression := range outerScope.Expression.Expressions() {
		if expression.Find(exp.TraitAggFunc) != nil {
			return
		}
	}
	outerScope.Expression.Set("order", innerOrder)
}

// mergeHints ports _merge_hints (optimizer/merge_subqueries.py:548-556).
func mergeHints(outerScope, innerScope *Scope) {
	innerHint := asExpression(innerScope.Expression.Arg("hint"))
	if innerHint == nil {
		return
	}
	if outerHint := asExpression(outerScope.Expression.Arg("hint")); outerHint != nil {
		for _, hintExpression := range innerHint.Expressions() {
			outerHint.Append("expressions", hintExpression)
		}
		return
	}
	outerScope.Expression.Set("hint", innerHint)
}

// popCTE ports _pop_cte (optimizer/merge_subqueries.py:559-575): it removes the CTE from the AST.
func popCTE(innerScope *Scope) {
	cte := innerScope.Expression.Parent()
	if cte == nil {
		return
	}
	with := cte.Parent()
	if with == nil {
		return
	}
	if len(with.Expressions()) == 1 {
		with.Pop()
	} else {
		cte.Pop()
	}
}
