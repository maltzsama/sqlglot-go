package optimizer

import (
	"fmt"

	exp "github.com/maltzsama/sqlglot-go/expressions"
)

type dmlSourceCandidate struct {
	expression exp.Expression
	location   string
	isTarget   bool
}

type dmlSourceValidationError struct {
	location string
	reason   string
}

func isDMLRootKind(kind exp.Kind) bool {
	return kind == exp.KindUpdate || kind == exp.KindDelete || kind == exp.KindMerge
}

func _traverseDML(scope *Scope, out *[]*Scope, mode scopeTraversalMode) {
	scope.dmlChildScopes = append(scope.dmlChildScopes, scope.CTEScopes...)
	candidates, validationErr := collectDMLSourceCandidates(scope.Expression)
	if validationErr != nil {
		logWarning(
			"Cannot build %s root scope: invalid %s (%s)",
			exp.ClassName(scope.Expression.Kind()),
			validationErr.location,
			validationErr.reason,
		)
		_traverseDMLQueries(scope, out, mode)
		return
	}

	sourceSubqueries := map[exp.Expression]bool{}
	for _, candidate := range candidates {
		if candidate.expression.Kind() == exp.KindSubquery {
			sourceSubqueries[candidate.expression] = true
		}
	}

	queryScopes := map[exp.Expression]*Scope{}
	processedSourceSubqueries := map[exp.Expression]bool{}
	for _, query := range findAllInScope(scope.Expression, exp.TraitQuery) {
		parent := query.Parent()
		if parent != nil && (parent.Kind() == exp.KindCTE || parent.Kind() == exp.KindSubquery) {
			continue
		}

		isSourceSubquery := sourceSubqueries[query]
		if isSourceSubquery && processedSourceSubqueries[query] {
			continue
		}
		childScope, wrapperScope := traverseDMLQuery(scope, query, isSourceSubquery, out, mode)
		if isSourceSubquery {
			processedSourceSubqueries[query] = true
		}
		if childScope == nil {
			continue
		}
		queryScopes[query] = childScope
		if isSourceSubquery {
			attachDMLSourceScope(scope, childScope, wrapperScope)
		} else {
			scope.SubqueryScopes = append(scope.SubqueryScopes, childScope)
			scope.dmlChildScopes = append(scope.dmlChildScopes, childScope)
		}
	}

	// A source can be attached through a defensive, undeclared argument key. ArgKeys does
	// not expose such keys to walkInScope, so make sure every validated source Subquery is
	// still traversed exactly once.
	for _, candidate := range candidates {
		subquery := candidate.expression
		if subquery.Kind() != exp.KindSubquery || processedSourceSubqueries[subquery] {
			continue
		}
		childScope, wrapperScope := traverseDMLQuery(scope, subquery, true, out, mode)
		processedSourceSubqueries[subquery] = true
		if childScope == nil {
			continue
		}
		queryScopes[subquery] = childScope
		attachDMLSourceScope(scope, childScope, wrapperScope)
	}

	if bindingErr := bindDMLSources(scope, candidates, queryScopes); bindingErr != nil {
		logWarning(
			"Cannot build %s root scope: invalid %s (%s)",
			exp.ClassName(scope.Expression.Kind()),
			bindingErr.location,
			bindingErr.reason,
		)
		return
	}

	*out = append(*out, scope)
}

func attachDMLSourceScope(scope *Scope, childScope *Scope, wrapperScope *Scope) {
	scope.DerivedTableScopes = append(scope.DerivedTableScopes, childScope)
	scope.TableScopes = append(scope.TableScopes, childScope)
	scope.dmlChildScopes = append(scope.dmlChildScopes, wrapperScope)
}

// _traverseDMLQueries mirrors the pinned v30.12.0 generic DML walk. Its query
// discovery is intentionally shape-dependent; do not replace it with a source walk.
func _traverseDMLQueries(scope *Scope, out *[]*Scope, mode scopeTraversalMode) {
	for _, query := range findAllInScope(scope.Expression, exp.TraitQuery) {
		parent := query.Parent()
		if parent == nil || (parent.Kind() != exp.KindCTE && parent.Kind() != exp.KindSubquery) {
			_traverseScope(newScope(query, scopeOptions{cteSources: scope.CTESources}), out, mode)
		}
	}
}

// _traverseDMLForOptimizer ports the exp.DML branch of _traverse_scope (scope.py:716-765)
// exactly, for the optimizer compatibility traversal; the caller already traversed the CTEs.
// No DML root scope is emitted, as upstream.
func _traverseDMLForOptimizer(scope *Scope, out *[]*Scope, mode scopeTraversalMode) {
	expression := scope.Expression

	// Bare tables in relation position (e.g. UPDATE ... FROM t, DELETE / MERGE ... USING t)
	// aren't part of any query, so they're scoped as standalone tables; _traverseTables also
	// picks up any joins hanging off of them.
	var relations []exp.Expression
	if from := asExpression(expression.Arg("from_")); from != nil && from.Kind() == exp.KindFrom {
		relations = append(relations, from.This())
	}
	switch using := expression.Arg("using").(type) {
	case []exp.Expression:
		relations = append(relations, using...)
	case exp.Expression:
		relations = append(relations, using)
	}
	for _, relation := range relations {
		if relation != nil && relation.Kind() == exp.KindTable {
			_traverseScope(newScope(relation, scopeOptions{cteSources: scope.CTESources}), out, mode)
		}
	}

	for _, query := range findAllInScope(expression, exp.TraitQuery) {
		// This check ensures we don't yield the CTE/nested queries twice.
		if parent := query.Parent(); parent != nil && (parent.Kind() == exp.KindCTE || parent.Kind() == exp.KindSubquery) {
			continue
		}

		if isFromOrJoin(query) {
			parent := query.Parent()
			if parent.Kind() == exp.KindJoin {
				if grand := parent.Parent(); grand != nil && (grand.Kind() == exp.KindSubquery || grand.Kind() == exp.KindTable) {
					// Scoped by the FROM-position relation (wrapper or table) it's joined to.
					continue
				}
			}
			// A query in FROM/JOIN position (e.g. UPDATE ... FROM (SELECT ...) AS s) acts like
			// a derived table, so its scope stays rooted at the Subquery wrapper to pick up the
			// wrapper's alias, column list and joins.
			_traverseScope(newScope(query, scopeOptions{cteSources: scope.CTESources}), out, mode)
		} else {
			// Queries in value position (SET, WHERE, USING, ...) are scoped as subqueries,
			// e.g. so their columns can be correlated to the DML's target table.
			_traverseScope(scope.branch(query, scopeOptions{scopeType: ScopeTypeSubquery}), out, mode)
		}
	}
}

func traverseDMLQuery(scope *Scope, query exp.Expression, isSourceSubquery bool, out *[]*Scope, mode scopeTraversalMode) (*Scope, *Scope) {
	before := len(*out)
	if isSourceSubquery {
		wrapperScope := newScope(query, scopeOptions{
			parent:       scope,
			scopeType:    ScopeTypeDerivedTable,
			cteSources:   scope.CTESources,
			outerColumns: query.AliasColumnNames(),
		})
		_traverseScope(wrapperScope, out, mode)
		if len(wrapperScope.SubqueryScopes) > 0 {
			childScope := wrapperScope.SubqueryScopes[len(wrapperScope.SubqueryScopes)-1]
			configureDMLSourceScope(childScope, scope, query)
			return childScope, wrapperScope
		}

		// A Subquery used as a DML source has a From/Join parent, so the regular
		// Subquery boundary logic intentionally leaves its inner query to the outer
		// table traversal. DML uses a dedicated source collector instead, therefore
		// traverse that inner query here while retaining the already-created wrapper.
		if len(*out) > before && (*out)[len(*out)-1] == wrapperScope {
			*out = (*out)[:len(*out)-1]
		}
		inner := query.This()
		if isNilDMLExpression(inner) || !inner.Is(exp.TraitQuery) {
			*out = append(*out, wrapperScope)
			return nil, wrapperScope
		}
		innerBefore := len(*out)
		_traverseScope(wrapperScope.branch(inner, scopeOptions{
			scopeType:    ScopeTypeDerivedTable,
			outerColumns: query.AliasColumnNames(),
		}), out, mode)
		if len(*out) == innerBefore {
			*out = append(*out, wrapperScope)
			return nil, wrapperScope
		}
		childScope := (*out)[len(*out)-1]
		configureDMLSourceScope(childScope, scope, query)
		wrapperScope.SubqueryScopes = append(wrapperScope.SubqueryScopes, childScope)
		*out = append(*out, wrapperScope)
		return childScope, wrapperScope
	}

	_traverseScope(newScope(query, scopeOptions{cteSources: scope.CTESources}), out, mode)
	if len(*out) == before {
		return nil, nil
	}
	return (*out)[len(*out)-1], nil
}

func configureDMLSourceScope(childScope *Scope, parent *Scope, subquery exp.Expression) {
	childScope.Parent = parent
	childScope.ScopeType = ScopeTypeDerivedTable
	childScope.OuterColumns = append([]string(nil), subquery.AliasColumnNames()...)
	childScope.canBeCorrelated = false
}

// dmlRejectNestedJoins fail-closes on a Join node that carries its own `joins` arg (e.g.
// u.joins=[joinV], joinV.joins=[joinW]) — a shape current parsers do not produce (joins are
// flattened onto the source's `joins` list). The source walk only recurses join.This(), so such
// nested joins would be silently missed, emitting a DML-root scope with an incomplete source set
// (a fail-open). Refuse it instead, so the root is omitted (complete-or-none) rather than partial.
func dmlRejectNestedJoins(join exp.Expression, location string) *dmlSourceValidationError {
	nested, err := strictDMLExpressionList(join.Arg("joins"), false, location+".joins")
	if err != nil {
		return err
	}
	if len(nested) > 0 {
		return &dmlSourceValidationError{
			location: location + ".joins",
			reason:   "joins nested on a Join node are not collected — omitting DML root fail-closed",
		}
	}
	return nil
}

func collectDMLSourceCandidates(root exp.Expression) ([]dmlSourceCandidate, *dmlSourceValidationError) {
	candidates := []dmlSourceCandidate{}
	visited := map[exp.Expression]bool{}

	var addCandidate func(exp.Expression, string, bool) *dmlSourceValidationError
	addCandidate = func(expression exp.Expression, location string, isTarget bool) *dmlSourceValidationError {
		if isNilDMLExpression(expression) {
			return &dmlSourceValidationError{location: location, reason: "missing Table or Subquery"}
		}
		if expression.Kind() != exp.KindTable && expression.Kind() != exp.KindSubquery {
			return &dmlSourceValidationError{
				location: location,
				reason:   fmt.Sprintf("expected Table or Subquery, got %s", exp.ClassName(expression.Kind())),
			}
		}
		if visited[expression] {
			return nil
		}
		visited[expression] = true
		candidates = append(candidates, dmlSourceCandidate{
			expression: expression,
			location:   location,
			isTarget:   isTarget,
		})

		joins, err := strictDMLExpressionList(expression.Arg("joins"), false, location+".joins")
		if err != nil {
			return err
		}
		for i, join := range joins {
			joinLocation := fmt.Sprintf("%s.joins[%d]", location, i)
			if join.Kind() != exp.KindJoin {
				return &dmlSourceValidationError{
					location: joinLocation,
					reason:   fmt.Sprintf("expected Join, got %s", exp.ClassName(join.Kind())),
				}
			}
			if err := dmlRejectNestedJoins(join, joinLocation); err != nil {
				return err
			}
			if err := addCandidate(join.This(), joinLocation+".this", false); err != nil {
				return err
			}
		}
		return nil
	}

	target := root.This()
	if !isNilDMLExpression(target) && target.Kind() == exp.KindSchema {
		target = target.This()
	}
	if err := addCandidate(target, "this", true); err != nil {
		return nil, err
	}

	fromUnderscore := root.Arg("from_")
	fromFallback := root.Arg("from")
	if fromUnderscore != nil && fromFallback != nil {
		return nil, &dmlSourceValidationError{
			location: "from_",
			reason:   "both from_ and defensive from fallback are present",
		}
	}
	fromValue := fromUnderscore
	fromLocation := "from_"
	if fromValue == nil {
		fromValue = fromFallback
		fromLocation = "from"
	}
	if fromValue != nil {
		fromExpression, ok := fromValue.(exp.Expression)
		if !ok || isNilDMLExpression(fromExpression) {
			return nil, &dmlSourceValidationError{location: fromLocation, reason: "expected From"}
		}
		if fromExpression.Kind() != exp.KindFrom {
			return nil, &dmlSourceValidationError{
				location: fromLocation,
				reason:   fmt.Sprintf("expected From, got %s", exp.ClassName(fromExpression.Kind())),
			}
		}
		if err := addCandidate(fromExpression.This(), fromLocation+".this", false); err != nil {
			return nil, err
		}
		fromExpressions, err := strictDMLExpressionList(fromExpression.Arg("expressions"), false, fromLocation+".expressions")
		if err != nil {
			return nil, err
		}
		for i, expression := range fromExpressions {
			if err := addCandidate(expression, fmt.Sprintf("%s.expressions[%d]", fromLocation, i), false); err != nil {
				return nil, err
			}
		}
	}

	rootJoins, err := strictDMLExpressionList(root.Arg("joins"), false, "joins")
	if err != nil {
		return nil, err
	}
	for i, join := range rootJoins {
		location := fmt.Sprintf("joins[%d]", i)
		if join.Kind() != exp.KindJoin {
			return nil, &dmlSourceValidationError{
				location: location,
				reason:   fmt.Sprintf("expected Join, got %s", exp.ClassName(join.Kind())),
			}
		}
		if err := dmlRejectNestedJoins(join, location); err != nil {
			return nil, err
		}
		if err := addCandidate(join.This(), location+".this", false); err != nil {
			return nil, err
		}
	}

	usingRequired := root.Kind() == exp.KindMerge
	usingExpressions, err := strictDMLExpressionList(root.Arg("using"), true, "using")
	if err != nil {
		return nil, err
	}
	if usingRequired && len(usingExpressions) == 0 {
		return nil, &dmlSourceValidationError{location: "using", reason: "missing required Table or Subquery"}
	}
	for i, expression := range usingExpressions {
		location := "using"
		if len(usingExpressions) > 1 {
			location = fmt.Sprintf("using[%d]", i)
		}
		if err := addCandidate(expression, location, false); err != nil {
			return nil, err
		}
	}

	return candidates, nil
}

func strictDMLExpressionList(value any, allowScalar bool, location string) ([]exp.Expression, *dmlSourceValidationError) {
	if value == nil {
		return nil, nil
	}

	var expressions []exp.Expression
	switch values := value.(type) {
	case exp.Expression:
		if !allowScalar {
			return nil, &dmlSourceValidationError{location: location, reason: "expected expression list"}
		}
		expressions = []exp.Expression{values}
	case []exp.Expression:
		expressions = values
	case []any:
		expressions = make([]exp.Expression, 0, len(values))
		for i, value := range values {
			expression, ok := value.(exp.Expression)
			if !ok || isNilDMLExpression(expression) {
				return nil, &dmlSourceValidationError{
					location: fmt.Sprintf("%s[%d]", location, i),
					reason:   "expected non-nil expression",
				}
			}
			expressions = append(expressions, expression)
		}
	default:
		return nil, &dmlSourceValidationError{location: location, reason: "expected expression list"}
	}

	for i, expression := range expressions {
		if isNilDMLExpression(expression) {
			return nil, &dmlSourceValidationError{
				location: fmt.Sprintf("%s[%d]", location, i),
				reason:   "expected non-nil expression",
			}
		}
	}
	return expressions, nil
}

func isNilDMLExpression(expression exp.Expression) bool {
	if expression == nil {
		return true
	}
	if node, ok := expression.(*exp.Node); ok {
		return node == nil
	}
	return false
}

func bindDMLSources(scope *Scope, candidates []dmlSourceCandidate, queryScopes map[exp.Expression]*Scope) *dmlSourceValidationError {
	taken := copySources(scope.Sources)
	bound := map[string]any{}
	boundOrder := []string{}

	for _, candidate := range candidates {
		expression := candidate.expression
		var source any
		var sourceName string

		if expression.Kind() == exp.KindSubquery {
			childScope := queryScopes[expression]
			if childScope == nil {
				return &dmlSourceValidationError{
					location: candidate.location,
					reason:   "Subquery did not produce a child scope",
				}
			}
			sourceName = getSourceAlias(expression)
			if candidate.isTarget {
				// The write target must remain the physical AST relation even when it is
				// subquery-shaped; only read-side Subqueries bind to child scopes.
				source = expression
			} else {
				source = childScope
			}
		} else {
			tableName := expression.Name()
			sourceName = expression.AliasOrName()
			source = expression

			if !candidate.isTarget && expression.SchemaName() == "" {
				if cteSource, ok := scope.Sources[tableName]; ok {
					pivots := expressionsFor(expression, "pivots")
					if len(pivots) > 0 {
						sourceName = pivots[len(pivots)-1].Alias()
					} else {
						source = cteSource
					}
				}
			}
		}

		key := sourceName
		if _, alreadyBound := bound[key]; alreadyBound || dmlSourceCollision(scope.Sources[key], source) {
			key = findNewName(taken, sourceName)
		}
		bound[key] = source
		boundOrder = append(boundOrder, key)
		taken[key] = source
	}

	for _, name := range boundOrder {
		scope.Sources[name] = bound[name]
		scope.sourceOrder = appendSourceOrder(scope.sourceOrder, name)
	}
	return nil
}

func dmlSourceCollision(existing any, source any) bool {
	if existing == nil {
		return false
	}
	existingScope, existingIsScope := existing.(*Scope)
	sourceScope, sourceIsScope := source.(*Scope)
	return !existingIsScope || !sourceIsScope || existingScope != sourceScope
}

func isDMLRootScope(scope *Scope) bool {
	return scope != nil && scope.IsRoot() && scope.Expression != nil && isDMLRootKind(scope.Expression.Kind())
}
