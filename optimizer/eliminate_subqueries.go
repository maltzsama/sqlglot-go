package optimizer

import exp "github.com/maltzsama/sqlglot-go/expressions"

// existingCTEs ports eliminate_subqueries' ExistingCTEsMapping (dict[exp.Expr, str]): CTE
// bodies keyed by structural equality, the way a Python dict keys an Expr by its hash and
// __eq__. Entries are bucketed by the hash at insertion and matched with Equal, which compares
// current hashes — the same outcome as a Python lookup after the key was mutated.
type existingCTEs map[uint64][]existingCTE

type existingCTE struct {
	body exp.Expression
	name string
}

func (m existingCTEs) get(body exp.Expression) string {
	for _, entry := range m[body.HashKey()] {
		if entry.body.Equal(body) {
			return entry.name
		}
	}
	return ""
}

func (m existingCTEs) put(body exp.Expression, name string) {
	key := body.HashKey()
	m[key] = append(m[key], existingCTE{body: body, name: name})
}

// EliminateSubqueries ports eliminate_subqueries (optimizer/eliminate_subqueries.py:16-115):
// it rewrites derived tables as CTEs, deduplicating identical ones.
//
//	SELECT a FROM (SELECT * FROM x) AS y  ->  WITH y AS (SELECT * FROM x) SELECT a FROM y AS y
func EliminateSubqueries(expression exp.Expression) exp.Expression {
	if expression.Kind() == exp.KindSubquery {
		// It's possible to have subqueries at the root, e.g. (SELECT * FROM x) LIMIT 1.
		EliminateSubqueries(expression.This())
		return expression
	}

	root := buildScopeForOptimizer(expression)
	if root == nil {
		return expression
	}

	// Map of alias->Scope|Table: all aliases already used in the expression. We don't want
	// to create new CTEs that conflict with these names.
	taken := map[string]any{}

	// All CTE aliases in the root scope are taken.
	for _, scope := range root.CTEScopes {
		if parent := scope.Expression.Parent(); parent != nil {
			taken[parent.Alias()] = scope
		}
	}

	// All table names are taken.
	for _, scope := range root.Traverse() {
		for _, source := range scope.Sources {
			if table, ok := source.(exp.Expression); ok && table != nil && table.Kind() == exp.KindTable {
				taken[table.Name()] = table
			}
		}
	}

	// Existing CTEs in the root expression, used for deduplication.
	existing := existingCTEs{}

	var recursive any = false
	if with := asExpression(root.Expression.Arg("with_")); with != nil {
		recursive = with.Arg("recursive")
		for _, cte := range with.Expressions() {
			existing.put(cte.This(), cte.Alias())
		}
	}
	var newCTEs []exp.Expression

	// We're adding more CTEs, but we want to maintain the DAG order. Derived tables within
	// an existing CTE need to come before the existing CTE.
	for _, cteScope := range root.CTEScopes {
		// Append all the new CTEs from this existing CTE.
		for _, scope := range cteScope.Traverse() {
			if scope == cteScope {
				// Don't try to eliminate this CTE itself.
				continue
			}
			if newCTE := eliminateSubquery(scope, existing, taken); newCTE != nil {
				newCTEs = append(newCTEs, newCTE)
			}
		}

		// Append the existing CTE itself.
		if cteParent := cteScope.Expression.Parent(); cteParent != nil {
			newCTEs = append(newCTEs, cteParent)
		}
	}

	// Now append the rest.
	var rest []*Scope
	rest = append(rest, root.UnionScopes...)
	rest = append(rest, root.SubqueryScopes...)
	rest = append(rest, root.TableScopes...)
	for _, scope := range rest {
		for _, childScope := range scope.Traverse() {
			if newCTE := eliminateSubquery(childScope, existing, taken); newCTE != nil {
				newCTEs = append(newCTEs, newCTE)
			}
		}
	}

	if len(newCTEs) > 0 {
		query := expression
		if expression.Is(exp.TraitDDL) {
			query = asExpression(expression.Arg("expression"))
		}
		if query == nil || !query.Is(exp.TraitQuery) {
			// This can be reached for DMLs, which shouldn't hold the WITH clause; attach it
			// to the root query, which is also where any pre-existing CTEs in newCTEs came from.
			query = root.Expression.Unnest()
		}
		query.Set("with_", exp.With(exp.Args{"expressions": newCTEs, "recursive": recursive}))
	}
	return expression
}

// eliminateSubquery ports _eliminate (optimizer/eliminate_subqueries.py:118-127).
func eliminateSubquery(scope *Scope, existing existingCTEs, taken map[string]any) exp.Expression {
	if scope.IsDerivedTable() {
		return eliminateDerivedTable(scope, existing, taken)
	}
	if scope.IsCTE() {
		return eliminateCTE(scope, existing, taken)
	}
	return nil
}

// eliminateDerivedTable ports _eliminate_derived_table (optimizer/eliminate_subqueries.py:130-156).
func eliminateDerivedTable(scope *Scope, existing existingCTEs, taken map[string]any) exp.Expression {
	// This makes sure that we don't:
	// - drop the "pivot" arg from a pivoted subquery
	// - eliminate a lateral correlated subquery
	parentScope := scope.Parent
	if parentScope == nil || len(parentScope.Pivots()) > 0 || parentScope.Expression.Kind() == exp.KindLateral {
		return nil
	}

	exprParent := scope.Expression.Parent()
	if exprParent == nil || exprParent.Kind() != exp.KindSubquery || exprParent == parentScope.Expression {
		// In the latter case the wrapper is the parent scope's root, e.g., the FROM clause
		// of a DML statement or a parenthesized DDL source, not one of its derived tables.
		return nil
	}

	// Get rid of redundant exp.Subquery expressions, i.e. those that are just used as wrappers.
	toReplace := exprParent.Unwrap()
	name, cte := newCTE(scope, existing, taken)
	alias := toReplace.Alias()
	if alias == "" {
		alias = name
	}
	table := exp.AliasExpr(exp.Table_(name, nil, nil, nil, nil), alias, false)
	table.Set("joins", toReplace.Arg("joins"))

	toReplace.Replace(table)

	return cte
}

// eliminateCTE ports _eliminate_cte (optimizer/eliminate_subqueries.py:159-181).
func eliminateCTE(scope *Scope, existing existingCTEs, taken map[string]any) exp.Expression {
	parent := scope.Expression.Parent()
	if parent == nil {
		return nil
	}
	name, cte := newCTE(scope, existing, taken)

	with := parent.Parent()
	parent.Pop()
	if with != nil && len(with.Expressions()) == 0 {
		with.Pop()
	}

	// Rename references to this CTE.
	if scope.Parent == nil {
		return cte
	}
	for _, childScope := range scope.Parent.Traverse() {
		selected := childScope.SelectedSources()
		for _, sourceName := range childScope.SelectedSourceNames() {
			entry := selected[sourceName]
			if source, ok := entry.Source.(*Scope); ok && source == scope {
				newTable := exp.AliasExpr(exp.Table_(name, nil, nil, nil, nil), entry.Node.AliasOrName(), false)
				entry.Node.Replace(newTable)
			}
		}
	}

	return cte
}

// newCTE ports _new_cte (optimizer/eliminate_subqueries.py:184-215). It returns a new name for
// scope's CTE in the root scope, and a new CTE for it — or nil when scope duplicates an
// existing CTE, whose name is returned instead.
func newCTE(scope *Scope, existing existingCTEs, taken map[string]any) (string, exp.Expression) {
	duplicateAlias := existing.get(scope.Expression)
	name := ""
	if parent := scope.Expression.Parent(); parent != nil {
		name = parent.Alias()
	}

	if name == "" {
		name = findNewName(taken, "cte")
	}

	if duplicateAlias != "" {
		name = duplicateAlias
	} else if _, ok := taken[name]; ok {
		name = findNewName(taken, name)
	}

	taken[name] = scope

	if duplicateAlias != "" {
		return name, nil
	}
	existing.put(scope.Expression, name)
	return name, exp.CTE(exp.Args{
		"this":  scope.Expression,
		"alias": exp.TableAlias(exp.Args{"this": exp.ToIdentifier(name)}),
	})
}
