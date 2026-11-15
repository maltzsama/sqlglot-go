package optimizer

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// CanonicalizeInternalNames ports canonicalize_internal_names
// (optimizer/canonicalize_internal_names.py:13-359).
//
// Rewrite a query to a canonical structural form. Preserves data-contract names (base-table
// names and columns, top-level output aliases) and canonicalizes internal names (table
// aliases, CTE/subquery names, internal column aliases) to sequential `_tN` / `_cN`. For set
// operations the top-level output is the leftmost leaf SELECT.
//
// Assumes the expression has already been processed by qualify (or equivalent), so that
// columns are bound to sources and aliases exist on every projection / subquery / CTE.
func CanonicalizeInternalNames(expression exp.Expression) exp.Expression {
	// Skip non-queries for now (e.g., UPDATE ... SET x = s.x FROM (SELECT ...) AS s).
	if expression == nil || !expression.Is(exp.TraitQuery) {
		return expression
	}

	// Top-level output scopes: their aliases are the query's data contract. Regular UNION
	// takes names from the left branch; UNION BY NAME takes names from the union of all
	// branches, so both sides of a by_name SetOperation contribute.
	outputScopeExprs := map[exp.Expression]bool{}
	stack := []exp.Expression{expression}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if isSetOperation(node) {
			// Access the args directly instead of the left/right properties, because set
			// operation operands aren't guaranteed to be Query nodes, e.g. in
			// VALUES (1) UNION ALL SELECT 1.
			if this := node.This(); this != nil {
				stack = append(stack, this.Unnest())
			}
			if truthy(node.Arg("by_name")) {
				if right := node.Expr(); right != nil {
					stack = append(stack, right.Unnest())
				}
			}
		} else {
			outputScopeExprs[node] = true
		}
	}

	nextTable := nameSequence("_t")
	nextColumn := nameSequence("_c")

	canon := func(ident exp.Expression, name string) {
		if ident == nil {
			return
		}
		ident.Set("this", name)
		ident.Set("quoted", true)
	}

	scopeTable := map[exp.Expression]string{}
	scopeOutputs := map[exp.Expression]map[string]string{}

	// Shared across scopes so a Table referenced from multiple scopes (LATERAL/UNNEST) gets
	// consistent column names.
	tableColumns := map[exp.Expression]map[string]string{}

	// Shared across UDTF sources so chained correlated UNNESTs get consistent names for
	// their alias columns.
	udtfColumns := map[exp.Expression]map[string]string{}

	for _, scope := range traverseScopeForOptimizer(expression) {
		scopeExpr := scope.Expression
		isOutputScope := outputScopeExprs[scopeExpr]

		columnsBySource := map[string][]exp.Expression{}
		for _, col := range scope.Columns() {
			columnsBySource[col.TableName()] = append(columnsBySource[col.TableName()], col)
		}
		for _, tableCol := range scope.TableColumns() {
			columnsBySource[tableCol.Name()] = append(columnsBySource[tableCol.Name()], tableCol)
		}

		// Include qualified stars (e.g. `t.*`).
		for _, star := range scope.Stars() {
			if star.Kind() == exp.KindColumn && star.TableName() != "" {
				columnsBySource[star.TableName()] = append(columnsBySource[star.TableName()], star)
			}
		}

		// source_canon: source's canonical name (used in WITH "_tN" AS, and as the physical
		// table name in Table.this). Shared across every reference to the same source within
		// this scope. ref_alias: per-reference canonical alias (used as the alias on each
		// Table node and as the qualifier on its column refs). The first reference reuses
		// source_canon for compactness; subsequent refs get fresh `_tN` names so a self-join
		// like `cte AS x JOIN cte AS y` produces unique aliases.
		tableMap := map[string][2]string{}
		refCanonTaken := map[string]bool{}

		for _, sourceName := range scope.sourceOrder {
			source, ok := scope.Sources[sourceName]
			if !ok {
				continue
			}
			sourceCols := columnsBySource[sourceName]

			var aliasHolder exp.Expression
			var canonT string
			var nameMap map[string]string
			var childOutput map[string]string

			sourceScope, isScope := source.(*Scope)
			isBaseSource := false
			if !isScope {
				base, isExpr := source.(exp.Expression)
				if !isExpr || base == nil || base.Kind() != exp.KindTable {
					continue
				}
				isBaseSource = true
			}

			if isBaseSource {
				base := source.(exp.Expression)
				canonT = scopeTable[base]
				if canonT == "" {
					canonT = nextTable()
					scopeTable[base] = canonT
				}
				nameMap = tableColumns[base]
				if nameMap == nil {
					nameMap = map[string]string{}
					tableColumns[base] = nameMap
				}
			} else {
				// Key by the expression so a recursive CTE's self-reference, i.e. a separate
				// Scope whose Expression is the same object as the CTE's base branch, shares
				// the base branch's output mapping.
				srcExpr := sourceScope.Expression
				childOutput = scopeOutputs[srcExpr]
				parent := srcExpr.Parent()
				if parent != nil && (parent.Kind() == exp.KindCTE || parent.Kind() == exp.KindSubquery) {
					aliasHolder = parent
				} else if cte := parent; isSetOperation(parent) && cte.Parent() != nil && cte.Parent().Kind() == exp.KindCTE {
					if with_ := cte.Parent().Parent(); with_ != nil && with_.Kind() == exp.KindWith && truthy(with_.Arg("recursive")) {
						aliasHolder = cte.Parent()
					}
				} else if sourceScope.IsUDTF() {
					aliasHolder = srcExpr
				}

				// A UDTF (unlike a CTE/subquery) can be a source in multiple scopes, so keep
				// its alias_holder and share its column map across them.
				isUdtfSource := aliasHolder != nil && aliasHolder == srcExpr

				tableKey := srcExpr
				if aliasHolder != nil {
					tableKey = aliasHolder
				}
				canonT = scopeTable[tableKey]
				if canonT == "" {
					canonT = nextTable()
					scopeTable[tableKey] = canonT
				} else if !isUdtfSource {
					aliasHolder = nil // already renamed on a previous encounter
				}

				if isUdtfSource {
					nameMap = udtfColumns[srcExpr]
					if nameMap == nil {
						nameMap = map[string]string{}
						udtfColumns[srcExpr] = nameMap
					}
				} else {
					nameMap = map[string]string{}
				}
			}

			var refAlias string
			if refCanonTaken[canonT] {
				refAlias = nextTable()
			} else {
				refAlias = canonT
				refCanonTaken[canonT] = true
			}

			tableMap[sourceName] = [2]string{canonT, refAlias}

			// UNNEST struct fields are physical columns (preserve); the element alias is not.
			structFieldNames := map[string]bool{}
			var src exp.Expression
			if isScope {
				src = sourceScope.Expression
			}
			if src != nil && src.Kind() == exp.KindUnnest {
				if exprs := expressionsFor(src, "expressions"); len(exprs) > 0 {
					if typ := exprs[0].Type(); typ != nil {
						if elementType, ok := seqGet(expressionsFor(typ, "expressions"), 0); ok && elementType.IsType(exp.DTypeStruct) {
							for _, columnDef := range expressionsFor(elementType, "expressions") {
								structFieldNames[columnDef.Name()] = true
							}
						}
					}
				}
			}

			for _, srcCol := range sourceCols {
				// BigQuery whole-row struct ref (`SELECT t FROM t`): the identifier IS the
				// table alias, so rename it to this reference's alias.
				if srcCol.Kind() == exp.KindTableColumn {
					canon(asExpression(srcCol.Arg("this")), refAlias)
					continue
				}

				oldName := srcCol.Name()
				preserveCol := isBaseSource || structFieldNames[oldName]
				canonCol, seen := nameMap[oldName]

				if !seen {
					if preserveCol {
						canonCol = oldName
					} else if childCanon := childOutput[oldName]; childCanon != "" {
						canonCol = childCanon
					} else {
						canonCol = nextColumn()
					}
					nameMap[oldName] = canonCol
				}

				// Base-table and UNNEST struct-field column refs name physical columns =>
				// preserve verbatim (including quote state). Scope-sourced refs are internal
				// handles pointing at CTE/subquery aliases (injected unquoted via
				// to_identifier), so canon them.
				if !preserveCol {
					canon(asExpression(srcCol.Arg("this")), canonCol)
				}

				if tableID := asExpression(srcCol.Arg("table")); tableID != nil {
					canon(tableID, refAlias)
				}
			}

			if aliasHolder != nil {
				if alias := asExpression(aliasHolder.Arg("alias")); alias != nil {
					if aliasThis := asExpression(alias.Arg("this")); aliasThis != nil && aliasThis.Kind() == exp.KindIdentifier {
						// The CTE/Subquery name itself uses source_canon (canon_t), not
						// ref_alias — it's the source's identity, not a reference.
						canon(aliasThis, canonT)
					}
					if columns := expressionsFor(alias, "columns"); len(columns) > 0 {
						newColumns := make([]exp.Expression, 0, len(columns))
						for _, column := range columns {
							name := column.Name()
							if mapped, ok := nameMap[name]; ok {
								name = mapped
							}
							newColumns = append(newColumns, exp.Identifier(exp.Args{"this": name, "quoted": truthy(column.Arg("quoted"))}))
						}
						alias.Set("columns", newColumns)
					}
				}

				// BigQuery UNNEST ... WITH OFFSET AS <id> declares a pseudo-column via the
				// offset arg (not the alias).
				if aliasHolder.Kind() == exp.KindUnnest {
					if offsetID := asExpression(aliasHolder.Arg("offset")); offsetID != nil && offsetID.Kind() == exp.KindIdentifier {
						if mapped, ok := nameMap[offsetID.Name()]; ok {
							canon(offsetID, mapped)
						}
					}
				}
			}
		}

		// PIVOT output column names are bound to the IN-clause literals, so only the alias
		// (which qualifies column refs like `my_pivot.x`) is canonicalized.
		for _, pivot := range scope.Pivots() {
			pivotAlias := asExpression(pivot.Arg("alias"))
			if pivotAlias == nil {
				continue
			}
			pivotAliasThis := asExpression(pivotAlias.Arg("this"))
			if pivotAliasThis == nil || pivotAliasThis.Kind() != exp.KindIdentifier {
				continue
			}
			pivotCols := columnsBySource[pivotAliasThis.Name()]
			if len(pivotCols) == 0 {
				continue
			}

			pivotCanonT := nextTable()
			canon(pivotAliasThis, pivotCanonT)

			for _, pivotCol := range pivotCols {
				if tableID := asExpression(pivotCol.Arg("table")); tableID != nil {
					canon(tableID, pivotCanonT)
				}
			}
		}

		for _, table := range scope.Tables() {
			entry, ok := tableMap[table.AliasOrName()]
			if !ok {
				continue
			}
			sourceCanon, refAlias := entry[0], entry[1]

			// Real tables (qualified with db/catalog) keep their physical name; CTE/subquery
			// references get the source's canonical `_tN`.
			if tableThis := asExpression(table.Arg("this")); tableThis != nil && tableThis.Kind() == exp.KindIdentifier && !truthy(table.Arg("schema")) {
				canon(tableThis, sourceCanon)
			}

			if alias := asExpression(table.Arg("alias")); alias != nil {
				// The reference alias uses ref_alias, which is unique per Table node so
				// multiple references to the same source can coexist.
				if aliasThis := asExpression(alias.Arg("this")); aliasThis != nil && aliasThis.Kind() == exp.KindIdentifier {
					canon(aliasThis, refAlias)
				}
				if columns := expressionsFor(alias, "columns"); len(columns) > 0 {
					tc := tableColumns[table]
					newColumns := make([]exp.Expression, 0, len(columns))
					for _, column := range columns {
						name := column.Name()
						if mapped, ok := tc[name]; ok {
							name = mapped
						}
						newColumns = append(newColumns, exp.Identifier(exp.Args{"this": name, "quoted": truthy(column.Arg("quoted"))}))
					}
					alias.Set("columns", newColumns)
				}
			}
		}

		outputMap := map[string]string{}
		if scopeExpr.Kind() == exp.KindSelect {
			for _, sel := range scopeExpr.Selects() {
				// Sets default name for subquery projections, both aliased and unaliased.
				if sel.Kind() != exp.KindAlias && sel.Kind() != exp.KindSubquery {
					continue
				}
				aliasThis := asExpression(sel.Arg("alias"))
				if aliasThis == nil || aliasThis.Name() == "" {
					continue
				}
				oldAlias := aliasThis.Name()
				if isOutputScope {
					outputMap[oldAlias] = oldAlias
				} else {
					newName := nextColumn()
					sel.Set("alias", exp.Identifier(exp.Args{"this": newName, "quoted": true}))
					outputMap[oldAlias] = newName
				}
			}
		} else if isSetOperation(scopeExpr) && len(scope.UnionScopes) > 0 {
			// Regular UNION names come from the left branch. UNION BY NAME folds in
			// right-branch names too (any column unique to the right still appears in the
			// output).
			if left, ok := scopeOutputs[scope.UnionScopes[0].Expression]; ok {
				for k, v := range left {
					outputMap[k] = v
				}
			}
			if truthy(scopeExpr.Arg("by_name")) && len(scope.UnionScopes) > 1 {
				for k, v := range scopeOutputs[scope.UnionScopes[1].Expression] {
					if _, exists := outputMap[k]; !exists {
						outputMap[k] = v
					}
				}
			}
		} else if scope.IsUDTF() && len(scope.SubqueryScopes) > 0 {
			for k, v := range scopeOutputs[scope.SubqueryScopes[0].Expression] {
				outputMap[k] = v
			}
		}

		scopeOutputs[scopeExpr] = outputMap

		// Unqualified alias references (ORDER BY a, HAVING a > 5, ...).
		for _, col := range FindAllInScope(scopeExpr, exp.KindColumn) {
			if col.TableName() != "" {
				continue
			}
			if mapped, ok := outputMap[col.Name()]; ok {
				canon(asExpression(col.Arg("this")), mapped)
			}
		}

		// UBN matches branches by original alias. When both branches are internal and
		// aliased to distinct _cN, matching originals land on different slots and UBN splits
		// them into disjoint output columns, dropping data. Align the right branch's
		// canonicals to the left's. No-op when both branches preserved their aliases
		// (top-level UBN).
		if isSetOperation(scopeExpr) && truthy(scopeExpr.Arg("by_name")) && len(scope.UnionScopes) > 1 {
			leftScope, rightScope := scope.UnionScopes[0], scope.UnionScopes[1]
			leftOut := scopeOutputs[leftScope.Expression]
			rightOut := scopeOutputs[rightScope.Expression]

			rename := map[string]string{}
			for origName, leftCanon := range leftOut {
				if rightCanon, ok := rightOut[origName]; ok && rightCanon != leftCanon {
					rename[rightCanon] = leftCanon
				}
			}

			if len(rename) > 0 {
				renameStack := []exp.Expression{rightScope.Expression}
				for len(renameStack) > 0 {
					node := renameStack[len(renameStack)-1]
					renameStack = renameStack[:len(renameStack)-1]
					if isSetOperation(node) {
						if this := node.This(); this != nil {
							renameStack = append(renameStack, this)
						}
						if right := node.Expr(); right != nil {
							renameStack = append(renameStack, right)
						}
						continue
					}
					if node.Kind() != exp.KindSelect {
						continue
					}
					for _, sel := range node.Selects() {
						if sel.Kind() != exp.KindAlias {
							continue
						}
						if aid := asExpression(sel.Arg("alias")); aid != nil && aid.Kind() == exp.KindIdentifier {
							if to, ok := rename[aid.Name()]; ok {
								canon(aid, to)
							}
						}
					}
					for _, col := range FindAllInScope(node, exp.KindColumn) {
						if col.TableName() != "" {
							continue
						}
						if to, ok := rename[col.Name()]; ok {
							canon(asExpression(col.Arg("this")), to)
						}
					}
				}

				renamedRight := map[string]string{}
				for k, v := range rightOut {
					if to, ok := rename[v]; ok {
						renamedRight[k] = to
					} else {
						renamedRight[k] = v
					}
				}
				scopeOutputs[rightScope.Expression] = renamedRight
			}
		}
	}

	return expression
}
