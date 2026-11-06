package optimizer

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/maltzsama/sqlglot-go/dialects"
	sqlerrors "github.com/maltzsama/sqlglot-go/errors"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
	"github.com/maltzsama/sqlglot-go/schema"
)

// selectAll ports SELECT_ALL (optimizer/pushdown_projections.py:19): the sentinel meaning an
// outer query selects all columns. It is not a valid identifier, so it can't collide with one.
const selectAll = "\x00SELECT_ALL"

// isSetReturningFunction ports SET_RETURNING_FUNCTIONS (optimizer/pushdown_projections.py:25):
// Explode (with its subclasses), Inline and Unnest multiply the rows of the whole query.
func isSetReturningFunction(e exp.Expression) bool {
	if e.Kind() == exp.KindExplode || e.Kind() == exp.KindUnnest {
		return true
	}
	for _, name := range []string{"Inline", "Posexplode", "ExplodeOuter", "PosexplodeOuter"} {
		if isClass(e, name) {
			return true
		}
	}
	return false
}

// nameSet is a Python set of selected names, shared by reference like upstream's: assigning one
// scope's set to another aliases it.
type nameSet struct{ names map[string]bool }

func newNameSet(names ...string) *nameSet {
	s := &nameSet{names: map[string]bool{}}
	for _, n := range names {
		s.names[n] = true
	}
	return s
}

func (s *nameSet) has(name string) bool { return s.names[name] }

// isSelfReferencingCTE ports _is_self_referencing_cte (optimizer/pushdown_projections.py:28-38).
func isSelfReferencingCTE(scope *Scope) bool {
	cte := scope.Expression.Parent()
	if cte == nil || cte.Kind() != exp.KindCTE || cte.Parent() == nil || cte.Parent().Kind() != exp.KindWith ||
		!truthyArg(cte.Parent().Arg("recursive")) {
		return false
	}
	for _, table := range scope.Expression.FindAll(exp.KindTable) {
		if table.SchemaName() == "" && table.Name() == cte.Alias() {
			return true
		}
	}
	return false
}

// defaultSelection ports default_selection (optimizer/pushdown_projections.py:42-43): the
// projection used when none remains.
func defaultSelection(isAgg bool) exp.Expression {
	if isAgg {
		return exp.AliasExpr(exp.Max(exp.Args{"this": literalNumber(pyNum{isInt: true, coef: bigOne()})}), "_", false)
	}
	return exp.AliasExpr(literalNumber(pyNum{isInt: true, coef: bigOne()}), "_", false)
}

// PushdownProjections ports pushdown_projections (optimizer/pushdown_projections.py:46-148): it
// removes the projections no outer query uses. When journal is non-nil, the mutations are
// recorded so Journal.Revert can undo them.
//
//	SELECT y.a AS a FROM (SELECT x.a AS a, x.b AS b FROM x) AS y
//	  ->  SELECT y.a AS a FROM (SELECT x.a AS a FROM x) AS y
func PushdownProjections(expression exp.Expression, schemaArg schema.SchemaType, removeUnusedSelections bool, dialect dialects.DialectType, journal *Journal) exp.Expression {
	s, err := schema.EnsureSchema(schemaArg, dialect, true)
	if err != nil {
		panic(err)
	}
	// Map of scope to the columns outer queries select from it.
	sourceColumnAliasCount := map[any]int{}
	referenced := map[*Scope]*nameSet{}
	referencedFor := func(scope *Scope) *nameSet {
		if set, ok := referenced[scope]; ok {
			return set
		}
		set := newNameSet()
		referenced[scope] = set
		return set
	}

	// The scope tree is traversed in DFS post-order and iterated in reverse, so a scope's
	// selected columns are complete by the time it is reached.
	scopes := traverseScopeForOptimizer(expression)
	for i := len(scopes) - 1; i >= 0; i-- {
		scope := scopes[i]
		parentSelections, ok := referenced[scope]
		if !ok {
			parentSelections = newNameSet(selectAll)
		}
		aliasCount := sourceColumnAliasCount[scope]

		// Columns can't be removed from SELECT DISTINCT or UNION DISTINCT, and INTERSECT and
		// EXCEPT match entire rows, so every column is used even in their ALL variants.
		if truthyArg(scope.Expression.Arg("distinct")) || scope.Expression.Kind() == exp.KindIntersect || scope.Expression.Kind() == exp.KindExcept {
			parentSelections = newNameSet(selectAll)
		}
		// A recursive CTE's body reads the CTE's own output, so its projections can't be pruned
		// based only on what the enclosing query selects.
		if isSelfReferencingCTE(scope) {
			parentSelections = newNameSet(selectAll)
		}

		if scope.Expression.Is(exp.TraitSetOperation) {
			setOp := scope.Expression
			if truthyArg(setOp.Arg("kind")) || truthyArg(setOp.Arg("side")) {
				// BigQuery's kind / side syntax (e.g. INNER UNION ALL BY NAME) changes the
				// operation's semantics; leave it alone.
				continue
			}
			if len(scope.UnionScopes) < 2 {
				continue
			}
			left, right := scope.UnionScopes[0], scope.UnionScopes[1]
			le, re := left.Expression, right.Expression
			if !le.Is(exp.TraitQuery) || !re.Is(exp.TraitQuery) {
				continue
			}
			if len(le.Selects()) != len(re.Selects()) {
				sql, _ := generator.New(dialectOrBase(dialect), generator.Options{}).Generate(scope.Expression)
				panic(&sqlerrors.OptimizeError{Msg: fmt.Sprintf("Invalid set operation due to column mismatch: %s.", sql)})
			}

			referenced[left] = parentSelections

			switch {
			case selectIsStar(re):
				referenced[right] = parentSelections
			case !selectIsStar(le):
				if truthyArg(scope.Expression.Arg("by_name")) {
					referenced[right] = referenced[left]
				} else {
					names := newNameSet()
					for i, sel := range le.Selects() {
						if parentSelections.has(selectAll) || parentSelections.has(sel.AliasOrName()) {
							names.names[re.Selects()[i].AliasOrName()] = true
						}
					}
					referenced[right] = names
				}
			}
		}

		if scope.Expression.Kind() == exp.KindSelect {
			if removeUnusedSelections {
				removeUnusedSelectionsFrom(scope, parentSelections, s, aliasCount, journal)
			}
			if scope.ScansAllSubscopeColumns() {
				continue
			}

			// Group columns by source name.
			selects := map[string][]string{}
			for _, col := range scope.Columns() {
				table := col.Text("table")
				selects[table] = append(selects[table], col.Name())
			}

			// Push the selected columns down to the next scope.
			selected := scope.SelectedSources()
			for _, name := range scope.SelectedSourceNames() {
				entry := selected[name]
				if source, ok := entry.Source.(*Scope); ok && source.Expression.Is(exp.TraitQuery) {
					var first exp.Expression
					if sels := source.Expression.Selects(); len(sels) > 0 {
						first = sels[0]
					}
					target := referencedFor(source)
					if len(scope.Pivots()) > 0 || (first != nil && first.Kind() == exp.KindQueryTransform) {
						target.names[selectAll] = true
					} else {
						for _, col := range selects[name] {
							target.names[col] = true
						}
					}
				}
				if aliases := entry.Node.AliasColumnNames(); len(aliases) > 0 {
					sourceColumnAliasCount[entry.Source] = len(aliases)
				}
			}
		}
	}
	return expression
}

func dialectOrBase(dialect dialects.DialectType) *dialects.Dialect {
	d, err := dialects.GetOrRaise(dialect)
	if err != nil {
		return dialects.Base()
	}
	return d
}

// removeUnusedSelectionsFrom ports _remove_unused_selections
// (optimizer/pushdown_projections.py:151-243).
func removeUnusedSelectionsFrom(scope *Scope, parentSelections *nameSet, s schema.Schema, aliasCount int, journal *Journal) {
	orderRefs := map[string]bool{}
	if order := asExpression(scope.Expression.Arg("order")); order != nil {
		// Columns without a qualified table are assumed to reference output columns.
		for _, c := range order.FindAll(exp.KindColumn) {
			if c.Text("table") == "" {
				orderRefs[c.Name()] = true
			}
		}
	}

	// Resolve bare GROUP BY ordinals before pruning.
	ordinalRefs := bareGroupByOrdinalRefs(scope.Expression)
	groupOrdinalSelections := map[exp.Expression]bool{}
	for _, ref := range ordinalRefs {
		groupOrdinalSelections[ref.selection] = true
	}

	// GROUP BY ALL without explicit keys groups by every non-aggregate projection, so those
	// projections are grouping keys and can't be pruned.
	implicitGroupByAll := isImplicitGroupByAll(scope.Expression)

	var newSelections []exp.Expression
	removed, star, isAgg := false, false, false
	all := parentSelections.has(selectAll)

	for _, selection := range scope.Expression.Selects() {
		name := selection.AliasOrName()
		isAggSelection := (implicitGroupByAll || !isAgg) && findInScope(selection, exp.TraitAggFunc) != nil

		switch {
		case all || parentSelections.has(name) || orderRefs[name] || aliasCount > 0 ||
			groupOrdinalSelections[selection] || (implicitGroupByAll && !isAggSelection):
			newSelections = append(newSelections, selection)
			aliasCount--
		case containsSetReturningFunction(selection):
			// A set-returning function multiplies the rows of the whole query, so this
			// projection affects every output column's cardinality and stays, though it is
			// unreferenced. It is not a positional alias slot, so aliasCount is untouched.
			newSelections = append(newSelections, selection)
		default:
			if selection.IsStar() {
				star = true
			}
			removed = true
		}

		if !isAgg && isAggSelection {
			isAgg = true
		}
	}

	if star {
		resolver := NewResolver(scope, s, true)
		names := map[string]bool{}
		for _, sel := range newSelections {
			names[sel.AliasOrName()] = true
		}
		var sorted []string
		for name := range parentSelections.names {
			sorted = append(sorted, name)
		}
		sort.Strings(sorted)
		for _, name := range sorted {
			if !names[name] {
				newSelections = append(newSelections,
					exp.AliasExpr(exp.Column_(name, resolver.GetTable(name), nil, nil, nil), name, false))
			}
		}
	}

	// If there are no remaining selections, select a single constant.
	if len(newSelections) == 0 {
		newSelections = append(newSelections, defaultSelection(isAgg))
	}

	if journal != nil && removed {
		journal.Record(scope.Expression, "expressions")
	}

	scope.Expression.SelectExprs(false, false, newSelections...)

	// Rewrite bare GROUP BY ordinals to their positions in the pruned SELECT list.
	if len(ordinalRefs) > 0 {
		newPos := map[exp.Expression]int{}
		for i, sel := range newSelections {
			newPos[sel] = i + 1
		}
		for _, ref := range ordinalRefs {
			pos, ok := newPos[ref.selection]
			if current, _ := strconv.Atoi(ref.node.Text("this")); ok && current != pos {
				if journal != nil {
					journal.Record(ref.node, "this")
				}
				ref.node.Set("this", strconv.Itoa(pos))
			}
		}
	}

	if removed {
		scope.clearCache()
	}
}

func containsSetReturningFunction(selection exp.Expression) bool {
	for _, node := range walkInScope(selection, nil) {
		if isSetReturningFunction(node) {
			return true
		}
	}
	return false
}

// isImplicitGroupByAll ports _is_implicit_group_by_all (optimizer/pushdown_projections.py:
// 246-257): a bare GROUP BY ALL infers its keys from the SELECT list, unlike ALL as a grouping
// sets modifier.
func isImplicitGroupByAll(sel exp.Expression) bool {
	group := asExpression(sel.Arg("group"))
	if group == nil || !truthyArg(group.Arg("all")) {
		return false
	}
	return len(group.Expressions()) == 0 && !truthyArg(group.Arg("cube")) &&
		!truthyArg(group.Arg("rollup")) && !truthyArg(group.Arg("grouping_sets"))
}

type ordinalRef struct {
	node, selection exp.Expression
}

// bareGroupByOrdinalRefs ports _bare_group_by_ordinal_refs (optimizer/pushdown_projections.py:
// 260-281): each bare GROUP BY integer ordinal with its pre-prune projection.
func bareGroupByOrdinalRefs(sel exp.Expression) []ordinalRef {
	group := asExpression(sel.Arg("group"))
	if group == nil {
		return nil
	}
	selects := sel.Selects()
	var refs []ordinalRef
	for _, node := range group.Expressions() {
		if node.Kind() == exp.KindLiteral && isPyInt(node) {
			pos, err := strconv.Atoi(node.Text("this"))
			if err == nil && pos >= 1 && pos <= len(selects) {
				refs = append(refs, ordinalRef{node, selects[pos-1]})
			}
		}
	}
	return refs
}
