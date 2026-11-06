package optimizer

import (
	"sort"

	"github.com/maltzsama/sqlglot-go/dialects"
	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// PushdownPredicates ports pushdown_predicates (optimizer/pushdown_predicates.py:19-87): it
// pushes WHERE and JOIN predicates down into the FROMs, JOINs and subqueries they filter.
//
//	SELECT y.a AS a FROM (SELECT x.a AS a FROM x AS x) AS y WHERE y.a = 1
//	  ->  SELECT y.a AS a FROM (SELECT x.a AS a FROM x AS x WHERE x.a = 1) AS y WHERE TRUE
func PushdownPredicates(expression exp.Expression, dialect dialects.DialectType) exp.Expression {
	root := buildScopeForOptimizer(expression)

	d, err := dialects.GetOrRaise(dialect)
	if err != nil {
		panic(err)
	}
	// Presto, Trino and Athena don't support inner joins whose right side is an UNNEST.
	unnestRequiresCrossJoin := d.Name == "presto" || d.Name == "trino" || d.Name == "athena"

	if root == nil {
		return expression
	}
	scopeRefCount := root.RefCount()

	scopes := root.Traverse()
	for i := len(scopes) - 1; i >= 0; i-- {
		scope := scopes[i]
		sel := scope.Expression
		where := asExpression(sel.Arg("where"))
		joins := expressionsFor(sel, "joins")
		if where != nil {
			selected := scope.SelectedSources()
			names := scope.SelectedSourceNames()
			joinIndex := map[string]int{}
			for i, join := range joins {
				joinIndex[join.AliasOrName()] = i
			}

			// A right join can only push down to itself, not to the source FROM table.
			pushdownAllowed := true
			for _, k := range names {
				entry := selected[k]
				parent := entry.Node.FindAncestor(exp.KindJoin, exp.KindFrom)
				if parent != nil && parent.Kind() == exp.KindJoin {
					if joinText(parent, "side") == "RIGHT" {
						selected = map[string]selectedSource{k: entry}
						names = []string{k}
						break
					}
					if entry.Node.Kind() == exp.KindUnnest && unnestRequiresCrossJoin {
						pushdownAllowed = false
						break
					}
				}
			}
			if pushdownAllowed {
				pushdown(where.This(), sourceSet{names, selected}, scopeRefCount, d, joinIndex)
			}
		}

		// Joins should only push down into themselves, not into other joins.
		for _, join := range joins {
			name := join.AliasOrName()
			if side := joinText(join, "side"); side == "RIGHT" || side == "FULL" {
				continue
			}
			if entry, ok := scope.SelectedSources()[name]; ok {
				pushdown(asExpression(join.Arg("on")),
					sourceSet{[]string{name}, map[string]selectedSource{name: entry}}, scopeRefCount, d, nil)
			}
		}
	}
	return expression
}

// sourceSet is upstream's Sources mapping: name -> (node, source), in insertion order.
type sourceSet struct {
	names   []string
	entries map[string]selectedSource
}

// pushdown ports pushdown (optimizer/pushdown_predicates.py:90-113).
func pushdown(condition exp.Expression, sources sourceSet, scopeRefCount map[any]int, dialect *dialects.Dialect, joinIndex map[string]int) {
	if condition == nil {
		return
	}
	condition = condition.Replace(Simplify(condition, false, false, dialect))
	cnfLike := normalized(condition, false) || !normalized(condition, true)

	var predicates []exp.Expression
	splitKind := exp.KindOr
	if cnfLike {
		splitKind = exp.KindAnd
	}
	if condition.Kind() == splitKind {
		predicates = flatten(condition, true)
	} else {
		predicates = []exp.Expression{condition}
	}

	if cnfLike {
		pushdownCNF(predicates, sources, scopeRefCount, joinIndex)
	} else {
		pushdownDNF(predicates, sources, scopeRefCount, joinIndex)
	}
}

// pushIntoSelect pushes predicate into node's HAVING when it aggregates, otherwise its WHERE.
func pushIntoSelect(node, predicate exp.Expression) {
	inner := replaceAliases(node, predicate)
	if findInScope(inner, exp.TraitAggFunc) != nil {
		node.Having(true, false, inner)
	} else {
		node.Where(true, false, inner)
	}
}

// pushdownCNF ports pushdown_cnf (optimizer/pushdown_predicates.py:116-144): with CNF-like
// predicates, each block is replaced in its parent.
func pushdownCNF(predicates []exp.Expression, sources sourceSet, scopeRefCount map[any]int, joinIndex map[string]int) {
	for _, predicate := range predicates {
		nodes := nodesForPredicate(predicate, sources, scopeRefCount)
		for _, name := range nodes.names {
			node := nodes.entries[name]
			if node.Kind() == exp.KindJoin {
				joinName := node.AliasOrName()
				predicateTables := columnTableNames(predicate, joinName)
				if len(joinIndex) > 0 {
					// Don't push the predicate if it references tables in later joins.
					thisIndex := joinIndex[joinName]
					allBefore := true
					for table := range predicateTables {
						idx, ok := joinIndex[table]
						if !ok {
							idx = -1
						}
						if idx >= thisIndex {
							allBefore = false
							break
						}
					}
					if allBefore {
						predicate.Replace(booleanLiteral(true))
						joinOnAppend(node, predicate)
						break
					}
				}
			}
			if node.Kind() == exp.KindSelect {
				predicate.Replace(booleanLiteral(true))
				pushIntoSelect(node, predicate)
			}
		}
	}
}

// pushdownDNF ports pushdown_dnf (optimizer/pushdown_predicates.py:147-207): with DNF
// predicates, only conditions present in every block can go down, and the original stays.
func pushdownDNF(predicates []exp.Expression, sources sourceSet, scopeRefCount map[any]int, joinIndex map[string]int) {
	// Tables referenced in every block of the DNF: in (a.x AND b.x) OR (a.y AND c.y) only a.
	pushdownTables := map[string]bool{}
	for _, a := range predicates {
		aTables := columnTableNames(a, "")
		for _, b := range predicates {
			bTables := columnTableNames(b, "")
			for t := range aTables {
				if !bTables[t] {
					delete(aTables, t)
				}
			}
		}
		for t := range aTables {
			pushdownTables[t] = true
		}
	}
	tables := make([]string, 0, len(pushdownTables))
	for t := range pushdownTables {
		tables = append(tables, t)
	}
	sort.Strings(tables)

	conditions := map[string]exp.Expression{}
	var nodes orderedNodes

	// Push all predicates down to their respective nodes.
	for _, table := range tables {
		for _, predicate := range predicates {
			nodes = nodesForPredicate(predicate, sources, scopeRefCount)
			if _, ok := nodes.entries[table]; !ok {
				continue
			}
			if prev, ok := conditions[table]; ok {
				// exp.or_ copies its operands (copy=True).
				conditions[table] = combine(exp.KindOr, prev.Copy(), predicate.Copy())
			} else {
				conditions[table] = predicate
			}
		}

		// Upstream iterates the `nodes` its inner loop left behind, i.e. those of the last
		// predicate.
		for _, name := range nodes.names {
			node := nodes.entries[name]
			predicate, ok := conditions[name]
			if !ok {
				continue
			}
			switch node.Kind() {
			case exp.KindJoin:
				if len(joinIndex) > 0 {
					thisIndex := joinIndex[name]
					allBefore := true
					for t := range columnTableNames(predicate, name) {
						idx, ok := joinIndex[t]
						if !ok {
							idx = -1
						}
						if idx >= thisIndex {
							allBefore = false
							break
						}
					}
					if !allBefore {
						continue
					}
				}
				joinOnAppend(node, predicate)
			case exp.KindSelect:
				pushIntoSelect(node, predicate)
			}
		}
	}
}

// orderedNodes is nodes_for_predicate's dict: table -> node, in insertion order.
type orderedNodes struct {
	names   []string
	entries map[string]exp.Expression
}

func (n *orderedNodes) set(name string, node exp.Expression) {
	if _, ok := n.entries[name]; !ok {
		n.names = append(n.names, name)
	}
	n.entries[name] = node
}

// nodesForPredicate ports nodes_for_predicate (optimizer/pushdown_predicates.py:210-271).
func nodesForPredicate(predicate exp.Expression, sources sourceSet, scopeRefCount map[any]int) orderedNodes {
	nodes := orderedNodes{entries: map[string]exp.Expression{}}
	tableSet := columnTableNames(predicate, "")
	ancestor := predicate.FindAncestor(exp.KindJoin, exp.KindWhere)
	whereCondition := ancestor != nil && ancestor.Kind() == exp.KindWhere

	tables := make([]string, 0, len(tableSet))
	for t := range tableSet {
		tables = append(tables, t)
	}
	sort.Strings(tables)

	for _, table := range tables {
		var node exp.Expression
		var source any
		if entry, ok := sources.entries[table]; ok {
			node, source = entry.Node, entry.Source
		}

		// In a WHERE statement, find the root join or from statement to push down to.
		if node != nil && whereCondition {
			node = node.FindAncestor(exp.KindJoin, exp.KindFrom)
		}

		// A node can reference a CTE, which should be pushed down.
		if node != nil && node.Kind() == exp.KindFrom {
			if sourceScope, ok := source.(*Scope); ok {
				// source.parent is the parent scope, whose query holds the CTE's WITH.
				parent := sourceScope.Parent
				if parent == nil {
					panic("Source node has no parent")
				}
				if with := asExpression(parent.Expression.Arg("with_")); with != nil && truthyArg(with.Arg("recursive")) {
					return orderedNodes{entries: map[string]exp.Expression{}}
				}
				node = sourceScope.Expression
			}
		}

		if node != nil && node.Kind() == exp.KindJoin {
			if joinText(node, "side") != "" {
				// A right join preserves its own source, so a WHERE predicate on it can only
				// go into that source, never into the match-only ON clause.
				sourceScope, isScope := source.(*Scope)
				if joinText(node, "side") != "RIGHT" || !isScope {
					return orderedNodes{entries: map[string]exp.Expression{}}
				}
				node = sourceScope.Expression
			} else {
				nodes.set(table, node)
			}
		}

		if node != nil && node.Kind() == exp.KindSelect && len(tables) == 1 {
			// Window expressions can't be pushed into.
			hasWindow := false
			for _, sel := range node.Selects() {
				if findInScope(sel, exp.KindWindow) != nil {
					hasWindow = true
					break
				}
			}
			// A select referenced in several places can't take a predicate, and LIMIT,
			// OFFSET and QUALIFY pick rows before the outer predicate runs.
			if node.Arg("group") == nil && scopeRefCount[source] < 2 && !hasWindow &&
				node.Arg("limit") == nil && node.Arg("offset") == nil && node.Arg("qualify") == nil {
				nodes.set(table, node)
			}
		}
	}
	return nodes
}

// replaceAliases ports replace_aliases (optimizer/pushdown_predicates.py:274-287): the
// predicate with each column naming a source projection replaced by that projection.
func replaceAliases(source, predicate exp.Expression) exp.Expression {
	aliases := map[string]exp.Expression{}
	for _, sel := range source.Selects() {
		if sel.Kind() == exp.KindAlias {
			aliases[sel.Alias()] = sel.This()
		} else {
			aliases[sel.Name()] = sel
		}
	}
	return predicate.Transform(func(column exp.Expression) exp.Expression {
		if column.Kind() == exp.KindColumn {
			if replacement, ok := aliases[column.Name()]; ok {
				return replacement.Copy()
			}
		}
		return column
	}, true)
}
