package optimizer

import exp "github.com/maltzsama/sqlglot-go/expressions"

// EliminateCTEs ports eliminate_ctes (optimizer/eliminate_ctes.py:13-58): it removes unused
// CTEs from expression, popping the whole WITH clause once its last CTE goes. When journal is
// non-nil, every mutation is recorded first so Journal.Revert can undo the pass.
//
//	WITH y AS (SELECT a FROM x) SELECT a FROM z  ->  SELECT a FROM z
func EliminateCTEs(expression exp.Expression, journal *Journal) exp.Expression {
	root := BuildScope(expression)
	if root == nil {
		return expression
	}
	refCount := root.RefCount()

	// Traverse the scope tree in reverse so we can remove chains of unused CTEs.
	scopes := root.Traverse()
	for i := len(scopes) - 1; i >= 0; i-- {
		scope := scopes[i]
		if !scope.IsCTE() || refCount[scope] > 0 {
			continue
		}
		cteNode := scope.Expression.Parent()
		if cteNode == nil {
			continue
		}
		withNode := cteNode.Parent()
		if journal != nil && withNode != nil {
			journal.Record(withNode, "expressions")
		}
		cteNode.Pop()

		// Pop the entire WITH clause if this is the last CTE.
		if withNode != nil && len(withNode.Expressions()) == 0 {
			if journal != nil && withNode.Parent() != nil {
				journal.Record(withNode.Parent(), "with_")
			}
			withNode.Pop()
		}

		// Decrement the ref count for all sources this CTE selects from.
		selected := scope.SelectedSources()
		for _, name := range scope.SelectedSourceNames() {
			if source, ok := selected[name].Source.(*Scope); ok {
				refCount[source]--
			}
		}
	}
	return expression
}
