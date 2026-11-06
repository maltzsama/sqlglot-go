package optimizer

import (
	"regexp"
	"strings"

	"github.com/maltzsama/sqlglot-go/dialects"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/schema"
)

// Ports optimizer/annotate_types.py (sqlglot v30.17.0): type inference over the AST.
//
// A "type value" below is what upstream passes around as `exp.DataType | exp.DType`: either an
// exp.DType or a DataType expression (nil standing for None).
//
// Deferred: PIVOT / UNPIVOT output column types (_get_pivot_column_types and
// _get_unpivot_column_types), which need Pivot.output_columns, not ported yet. A column read
// through a pivot is annotated UNKNOWN.

// bigintExtractDateParts ports BIGINT_EXTRACT_DATE_PARTS (annotate_types.py:37-43): the
// EXTRACT/DATE_PART specifiers that return BIGINT instead of INT.
var bigintExtractDateParts = map[string]bool{
	"EPOCH_SECOND":      true,
	"EPOCH_MILLISECOND": true,
	"EPOCH_MICROSECOND": true,
	"EPOCH_NANOSECOND":  true,
	"NANOSECOND":        true,
}

// AnnotateTypes ports annotate_types (annotate_types.py:46-83): it infers the types of
// expression and annotates its AST accordingly. schemaArg is a schema.SchemaType.
//
//	schema {"y": {"cola": "SMALLINT"}}
//	SELECT x.cola + 2.5 AS cola FROM (SELECT y.cola AS cola FROM y AS y) AS x
//	  -> the projection is annotated DOUBLE
func AnnotateTypes(expression exp.Expression, schemaArg schema.SchemaType, dialect dialects.DialectType) exp.Expression {
	s, err := schema.EnsureSchema(schemaArg, dialect, true)
	if err != nil {
		panic(err)
	}
	return NewTypeAnnotator(s).Annotate(expression, true)
}

// isDateUnit ports helper.is_date_unit (helper.py:448-453).
func isDateUnit(expression exp.Expression) bool {
	if expression == nil {
		return false
	}
	switch strings.ToLower(expression.Name()) {
	case "day", "week", "month", "quarter", "year", "year_month":
		return true
	}
	return false
}

// isoDateRE and isoDatetimeRE approximate datetime.date.fromisoformat and
// datetime.datetime.fromisoformat (helper.py:432-445) for the ISO 8601 shapes they accept:
// YYYY-MM-DD / YYYYMMDD, optionally followed by T or a space, HH[:MM[:SS[.ffffff]]] and a
// Z / ±HH[:MM] offset.
var (
	isoDateRE     = regexp.MustCompile(`^\d{4}-?\d{2}-?\d{2}$`)
	isoDatetimeRE = regexp.MustCompile(`^\d{4}-?\d{2}-?\d{2}([T ]\d{2}(:?\d{2}(:?\d{2}([.,]\d{1,6})?)?)?(Z|[+-]\d{2}(:?\d{2})?)?)?$`)
)

func validISODate(text string) bool {
	digits := strings.ReplaceAll(text, "-", "")
	if len(digits) < 8 {
		return false
	}
	month := (digits[4]-'0')*10 + digits[5] - '0'
	day := (digits[6]-'0')*10 + digits[7] - '0'
	return month >= 1 && month <= 12 && day >= 1 && day <= 31
}

func isISODate(text string) bool { return isoDateRE.MatchString(text) && validISODate(text) }

func isISODatetime(text string) bool {
	if !isoDatetimeRE.MatchString(text) {
		return false
	}
	date := text
	if i := strings.IndexAny(text, "T "); i >= 0 {
		date = text[:i]
	}
	return validISODate(date)
}

// coerceDateLiteral ports _coerce_date_literal (annotate_types.py:86-97).
func coerceDateLiteral(l exp.Expression, unit exp.Expression) exp.DType {
	dateText := l.Name()
	isISODate := isISODate(dateText)

	if isISODate && isDateUnit(unit) {
		return exp.DTypeDate
	}
	// An ISO date is also an ISO datetime, but not vice versa.
	if isISODate || isISODatetime(dateText) {
		return exp.DTypeDatetime
	}
	return exp.DTypeUnknown
}

// coerceDate ports _coerce_date (annotate_types.py:100-103).
func coerceDate(l exp.Expression, unit exp.Expression) any {
	if !isDateUnit(unit) {
		return exp.DTypeDatetime
	}
	if typ := l.Type(); typ != nil {
		return typeThis(typ)
	}
	return exp.DTypeUnknown
}

// binaryCoercion ports BinaryCoercionFunc: given both sides of a binary operation, the
// resulting type value.
type binaryCoercion func(l, r exp.Expression) any

type dtypePair [2]exp.DType

// swapAll ports swap_all (annotate_types.py:114-115): every coercion also applies with its
// operands swapped.
func swapAll(coercions map[dtypePair]binaryCoercion) map[dtypePair]binaryCoercion {
	out := make(map[dtypePair]binaryCoercion, 2*len(coercions))
	for pair, fn := range coercions {
		out[pair] = fn
	}
	for pair, fn := range coercions {
		fn := fn
		out[dtypePair{pair[1], pair[0]}] = func(l, r exp.Expression) any { return fn(r, l) }
	}
	return out
}

// buildCoercesTo ports _build_coerces_to (annotate_types.py:118-155): for each type, the types
// it can be coerced into, by Spark's ANSI precedence.
func buildCoercesTo() map[exp.DType]map[exp.DType]bool {
	textPrecedence := []exp.DType{exp.DTypeText, exp.DTypeNVarchar, exp.DTypeVarchar, exp.DTypeNChar, exp.DTypeChar}
	numericPrecedence := []exp.DType{
		exp.DTypeDecFloat, exp.DTypeDouble, exp.DTypeFloat, exp.DTypeBigDecimal, exp.DTypeDecimal,
		exp.DTypeBigInt, exp.DTypeInt, exp.DTypeSmallInt, exp.DTypeTinyInt,
	}
	timelikePrecedence := []exp.DType{exp.DTypeTimestampLtz, exp.DTypeTimestampTz, exp.DTypeTimestamp, exp.DTypeDatetime, exp.DTypeDate}

	result := map[exp.DType]map[exp.DType]bool{}
	for _, precedence := range [][]exp.DType{textPrecedence, numericPrecedence, timelikePrecedence} {
		coercesTo := map[exp.DType]bool{}
		for _, dataType := range precedence {
			copied := make(map[exp.DType]bool, len(coercesTo))
			for k := range coercesTo {
				copied[k] = true
			}
			result[dataType] = copied
			coercesTo[dataType] = true
		}
	}
	return result
}

// defaultCoercesTo ports TypeAnnotator.COERCES_TO / _COERCES_TO (annotate_types.py:158-164).
var defaultCoercesTo = buildCoercesTo()

// defaultBinaryCoercions ports TypeAnnotator.BINARY_COERCIONS (annotate_types.py:166-195).
var defaultBinaryCoercions = func() map[dtypePair]binaryCoercion {
	out := map[dtypePair]binaryCoercion{}
	textInterval := map[dtypePair]binaryCoercion{}
	for t := range exp.TextTypes {
		textInterval[dtypePair{t, exp.DTypeInterval}] = func(l, r exp.Expression) any {
			return coerceDateLiteral(l, asExpression(r.Arg("unit")))
		}
	}
	textNumeric := map[dtypePair]binaryCoercion{}
	for text := range exp.TextTypes {
		for numeric := range exp.NumericTypes {
			// Text + numeric yields the numeric type, to match most dialects' semantics.
			textNumeric[dtypePair{text, numeric}] = func(l, r exp.Expression) any {
				if lt := l.Type(); lt != nil && exp.NumericTypes[typeThis(lt)] {
					return lt
				}
				return r.Type()
			}
		}
	}
	dateInterval := map[dtypePair]binaryCoercion{
		{exp.DTypeDate, exp.DTypeInterval}: func(l, r exp.Expression) any {
			return coerceDate(l, asExpression(r.Arg("unit")))
		},
	}
	for _, group := range []map[dtypePair]binaryCoercion{textInterval, textNumeric, dateInterval} {
		for pair, fn := range swapAll(group) {
			out[pair] = fn
		}
	}
	return out
}()

// typeThis is `t.this` for a DataType, or the DType itself.
func typeThis(t any) exp.DType {
	switch v := t.(type) {
	case exp.DType:
		return v
	case exp.Expression:
		if v != nil {
			if d, ok := v.Arg("this").(exp.DType); ok {
				return d
			}
		}
	}
	return ""
}

// isNilType reports whether a type value is None.
func isNilType(t any) bool {
	if t == nil {
		return true
	}
	if e, ok := t.(exp.Expression); ok && e == nil {
		return true
	}
	return false
}

// typeIsType is `t.is_type(*dtypes)` for a type value.
func typeIsType(t any, dtypes ...any) bool {
	switch v := t.(type) {
	case exp.DType:
		return exp.DataTypeIsType(exp.DataType(exp.Args{"this": v}), false, dtypes...)
	case exp.Expression:
		return v != nil && exp.DataTypeIsType(v, false, dtypes...)
	}
	return false
}

// isClass reports whether expression is the upstream class name, false while that class has
// no Kind in this port.
func isClass(expression exp.Expression, name string) bool {
	k, ok := exp.KindByClassName(name)
	return ok && expression.Kind() == k
}

// typedMap is an insertion-ordered name -> type value map (upstream's dict).
type typedMap struct {
	keys   []string
	values map[string]any
}

func newTypedMap() *typedMap { return &typedMap{values: map[string]any{}} }

func (m *typedMap) set(name string, t any) {
	if _, ok := m.values[name]; !ok {
		m.keys = append(m.keys, name)
	}
	m.values[name] = t
}

func (m *typedMap) get(name string) any {
	if m == nil {
		return nil
	}
	return m.values[name]
}

type scopeSourceKey struct {
	scope *Scope
	name  string
}

// TypeAnnotator ports TypeAnnotator (annotate_types.py:158-1098).
type TypeAnnotator struct {
	schema          schema.Schema
	dialect         *dialects.Dialect
	metadata        map[exp.Kind]typeSpec
	coercesTo       map[exp.DType]map[exp.DType]bool
	binaryCoercions map[dtypePair]binaryCoercion

	// visited caches the annotated sub-expressions, so each is only visited once.
	visited map[exp.Expression]bool
	// nullExpressions caches NULL-annotated expressions, set to the dialect's default NULL type
	// once inference completes.
	nullExpressions     map[exp.Expression]bool
	nullExpressionOrder []exp.Expression
	supportsNullType    bool
	// setopColumnTypes maps a set operation to its projection types, computed when it is the
	// expression of a scope source.
	setopColumnTypes map[exp.Expression]*typedMap
	// overwriteTypes false enables partial annotation by skipping already-annotated nodes.
	overwriteTypes     bool
	scopeSourceSelects map[scopeSourceKey]*typedMap
}

// NewTypeAnnotator ports TypeAnnotator.__init__ (annotate_types.py:197-229) with the default
// metadata, coercions and overwrite_types=True.
func NewTypeAnnotator(s schema.Schema) *TypeAnnotator {
	var d *dialects.Dialect
	if s != nil {
		d = s.Dialect()
	}
	if d == nil {
		d = dialects.Base()
	}
	return &TypeAnnotator{
		schema:             s,
		dialect:            d,
		metadata:           expressionMetadataFor(d.Name),
		coercesTo:          coercesToFor(d.Name),
		binaryCoercions:    defaultBinaryCoercions,
		visited:            map[exp.Expression]bool{},
		nullExpressions:    map[exp.Expression]bool{},
		supportsNullType:   false,
		setopColumnTypes:   map[exp.Expression]*typedMap{},
		overwriteTypes:     true,
		scopeSourceSelects: map[scopeSourceKey]*typedMap{},
	}
}

// Clear ports TypeAnnotator.clear (annotate_types.py:231-235).
func (a *TypeAnnotator) Clear() {
	a.visited = map[exp.Expression]bool{}
	a.nullExpressions = map[exp.Expression]bool{}
	a.nullExpressionOrder = nil
	a.setopColumnTypes = map[exp.Expression]*typedMap{}
	a.scopeSourceSelects = map[scopeSourceKey]*typedMap{}
}

// Uncache ports TypeAnnotator.uncache (annotate_types.py:237-250): it evicts expression (or
// its subtree, if deep) from the annotation caches, before an annotated tree is mutated.
func (a *TypeAnnotator) Uncache(expression exp.Expression, deep bool) {
	nodes := []exp.Expression{expression}
	if deep {
		nodes = expression.Walk()
	}
	for _, node := range nodes {
		delete(a.visited, node)
		delete(a.nullExpressions, node)
		delete(a.setopColumnTypes, node)
	}
}

// setType ports _set_type (annotate_types.py:254-276).
func (a *TypeAnnotator) setType(expression exp.Expression, targetType any) exp.Expression {
	prevType := expression.Type()
	if isNilType(targetType) {
		targetType = exp.DTypeUnknown
	}
	switch v := targetType.(type) {
	case exp.DType:
		expression.SetType(exp.DataType(exp.Args{"this": v}))
	case exp.Expression:
		expression.SetType(v)
	}
	a.visited[expression] = true

	if !a.supportsNullType && typeThis(expression.Type()) == exp.DTypeNull {
		if !a.nullExpressions[expression] {
			a.nullExpressions[expression] = true
			a.nullExpressionOrder = append(a.nullExpressionOrder, expression)
		}
	} else if prevType != nil && typeThis(prevType) == exp.DTypeNull {
		delete(a.nullExpressions, expression)
	}
	return expression
}

// Annotate ports TypeAnnotator.annotate (annotate_types.py:278-295). annotateScope false
// skips the scope traversal, annotating non-column expressions only.
func (a *TypeAnnotator) Annotate(expression exp.Expression, annotateScope bool) exp.Expression {
	if annotateScope {
		for _, scope := range traverseScopeForOptimizer(expression) {
			a.AnnotateScope(scope)
		}
	}

	// This takes care of non-traversable expressions.
	a.annotateExpression(expression, nil)

	// Replace the NULL type with the default type of the targeted dialect, since the former is
	// not an actual type; it mostly aids type coercion, e.g. in set operations.
	for _, expr := range a.nullExpressionOrder {
		if a.nullExpressions[expr] {
			a.setType(expr, exp.DTypeUnknown)
		}
	}
	return expression
}

// getScopeSourceSelects ports _get_scope_source_selects (annotate_types.py:297-347). Pivot
// sources are deferred (see the file comment) and yield no types.
func (a *TypeAnnotator) getScopeSourceSelects(scope *Scope, sourceName string) *typedMap {
	key := scopeSourceKey{scope, sourceName}
	if selects, ok := a.scopeSourceSelects[key]; ok {
		return selects
	}
	selects := newTypedMap()
	if source, ok := scope.Sources[sourceName].(*Scope); ok {
		selects = a.getSourceScopeSelects(source)
	}
	a.scopeSourceSelects[key] = selects
	return selects
}

// getSourceScopeSelects ports _get_source_scope_selects (annotate_types.py:349-394).
func (a *TypeAnnotator) getSourceScopeSelects(source *Scope) *typedMap {
	expression := source.Expression
	out := newTypedMap()

	if expression.Is(exp.TraitUDTF) {
		var values []exp.Expression
		switch expression.Kind() {
		case exp.KindLateral:
			if this := expression.This(); this != nil && this.Kind() == exp.KindExplode {
				values = []exp.Expression{this.This()}
			}
		case exp.KindUnnest:
			values = []exp.Expression{expression}
		default:
			if isClass(expression, "TableFromRows") {
				break
			}
			if exprs := expression.Expressions(); len(exprs) > 0 {
				values = exprs[0].Expressions()
			}
		}
		if len(values) == 0 {
			return out
		}

		var expType exp.Expression
		switch {
		case expression.Kind() == exp.KindUnnest:
			expType = expression.Type()
		case expression.Kind() == exp.KindLateral && expression.This() != nil && expression.This().Kind() == exp.KindExplode:
			expType = expression.This().Type()
		}

		if expType != nil && expType.IsType(exp.DTypeStruct) {
			for _, colDef := range expType.Expressions() {
				if colDef.Kind() == exp.KindColumnDef {
					if kind := asExpression(colDef.Arg("kind")); kind != nil {
						out.set(colDef.Name(), kind)
					}
				}
			}
			return out
		}

		aliasColumnNames := expression.AliasColumnNames()
		for i, alias := range aliasColumnNames {
			if i >= len(values) {
				break
			}
			out.set(alias, values[i].Type())
		}
		return out
	}

	if expression.Is(exp.TraitSetOperation) && (truthyArg(expression.Arg("by_name")) ||
		len(expression.This().Selects()) == len(expression.Expr().Selects())) {
		return a.getSetopColumnTypes(expression)
	}

	if expression.Is(exp.TraitQuery) {
		for _, s := range expression.Selects() {
			if typ := s.Type(); typ != nil {
				out.set(s.AliasOrName(), typ)
			}
		}
	}
	return out
}

func structType(columns []exp.Expression) exp.Expression {
	return exp.DataType(exp.Args{"this": exp.DTypeStruct, "expressions": columns, "nested": true})
}

// AnnotateScope ports annotate_scope (annotate_types.py:396-446).
func (a *TypeAnnotator) AnnotateScope(scope *Scope) {
	if mapping, ok := a.schema.(*schema.MappingSchema); ok && mapping != nil {
		for _, tableColumn := range scope.TableColumns() {
			switch source := scope.Sources[tableColumn.Name()].(type) {
			case exp.Expression:
				if source == nil || source.Kind() != exp.KindTable {
					continue
				}
				columns, err := a.schema.Find(source, false, true)
				if err != nil || columns == nil {
					continue
				}
				var defs []exp.Expression
				for _, name := range columns.Keys() {
					kind, _ := columns.Get(name)
					defs = append(defs, exp.ColumnDef(exp.Args{"this": exp.ToIdentifier(name), "kind": kind}))
				}
				a.setType(tableColumn, structType(defs))
			case *Scope:
				if source.Expression.Is(exp.TraitQuery) {
					if queryType, ok := source.Expression.MetaGet("query_type").(exp.Expression); ok && queryType != nil && queryType.IsType(exp.DTypeStruct) {
						a.setType(tableColumn, queryType)
					}
				}
			}
		}
	}

	// Iterate through all the expressions of the current scope in post-order, and annotate.
	a.annotateExpression(scope.Expression, scope)
	a.fixupOrderByAliases(scope)

	if a.dialect.QueryResultsAreStructs && scope.Expression.Is(exp.TraitQuery) {
		var defs []exp.Expression
		unknown := false
		for _, sel := range scope.Expression.Selects() {
			var kind exp.Expression
			if typ := sel.Type(); typ != nil {
				kind = typ.Copy()
				if kind.IsType(exp.DTypeUnknown) {
					unknown = true
				}
			}
			defs = append(defs, exp.ColumnDef(exp.Args{"this": exp.ToIdentifier(sel.OutputName()), "kind": kind}))
		}
		if !unknown {
			// Not set through setType on purpose: annotating the query directly would make
			// other contexts (e.g., ARRAY(<query>)) read it as a STRUCT value.
			scope.Expression.Meta()["query_type"] = structType(defs)
		}
	}
}

type annotateFrame struct {
	expr              exp.Expression
	childrenAnnotated bool
}

// annotateExpression ports _annotate_expression (annotate_types.py:448-546).
func (a *TypeAnnotator) annotateExpression(expression exp.Expression, scope *Scope) {
	stack := []annotateFrame{{expression, false}}

	for len(stack) > 0 {
		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		expr := frame.expr

		if a.visited[expr] || (!a.overwriteTypes && expr.Type() != nil && !expr.IsType(exp.DTypeUnknown)) {
			continue // We've already inferred the expression's type.
		}

		if !frame.childrenAnnotated {
			stack = append(stack, annotateFrame{expr, true})
			for _, child := range expr.IterExpressions(false) {
				stack = append(stack, annotateFrame{child, false})
			}
			continue
		}

		if scope != nil && expr.Kind() == exp.KindColumn && expr.Text("table") != "" {
			a.annotateQualifiedColumn(expr, scope)
			continue
		}

		spec, ok := a.metadata[expr.Kind()]
		switch {
		case ok && spec.annotator != nil:
			spec.annotator(a, expr)
		case ok && spec.returns != nil:
			a.setType(expr, spec.returns)
		default:
			a.setType(expr, exp.DTypeUnknown)
		}
	}
}

// annotateQualifiedColumn is the qualified-column branch of _annotate_expression
// (annotate_types.py:470-537).
func (a *TypeAnnotator) annotateQualifiedColumn(expr exp.Expression, scope *Scope) {
	table := expr.Text("table")
	var source any
	sourceScope := scope
	for sourceScope != nil && source == nil {
		if s, ok := sourceScope.Sources[table]; ok && s != nil {
			source = s
		} else {
			sourceScope = sourceScope.Parent
		}
	}

	switch src := source.(type) {
	case exp.Expression:
		if src.Kind() == exp.KindTable {
			colType, err := a.schema.GetColumnType(src, expr, nil, nil)
			if err != nil {
				colType = nil
			}
			a.setType(expr, colType)
		} else if unnest := src.Expr(); unnest != nil && unnest.Kind() == exp.KindUnnest {
			// A non-Table, non-Scope source has no projection types; upstream then reads
			// source.expression.
			a.setType(expr, unnest.Type())
		} else {
			a.setType(expr, exp.DTypeUnknown)
		}
	case *Scope:
		if colType := a.getScopeSourceSelects(sourceScope, table).get(expr.Name()); !isNilType(colType) {
			a.setType(expr, colType)
		} else if src.Expression.Kind() == exp.KindUnnest {
			a.setType(expr, src.Expression.Type())
		} else {
			a.setType(expr, exp.DTypeUnknown)
		}
	default:
		a.setType(expr, exp.DTypeUnknown)
	}

	if expr.IsType(exp.DTypeJSON) {
		if dotParts, ok := expr.MetaGet("dot_parts").([]string); ok && len(dotParts) > 0 {
			// JSON dot access is case sensitive across all dialects, so undo the normalization.
			i := 0
			for parent := expr.Parent(); parent != nil && parent.Kind() == exp.KindDot && i < len(dotParts); parent = parent.Parent() {
				identifier := parent.Expr()
				if identifier != nil && identifier.Kind() == exp.KindIdentifier {
					// Rename in place to preserve the identifier's meta, e.g. token positions.
					identifier.Set("this", dotParts[i])
					identifier.Set("quoted", true)
				} else if identifier != nil {
					identifier.Replace(exp.ToIdentifier(dotParts[i], true))
				}
				i++
			}
			delete(expr.Meta(), "dot_parts")
		}
	}

	if typ := expr.Type(); typ != nil {
		if nullable, ok := typ.Arg("nullable").(bool); ok && !nullable {
			expr.Meta()["nonnull"] = true
		}
	}
}

// fixupOrderByAliases ports _fixup_order_by_aliases (annotate_types.py:548-583).
func (a *TypeAnnotator) fixupOrderByAliases(scope *Scope) {
	query := scope.Expression
	if !query.Is(exp.TraitQuery) {
		return
	}
	order := asExpression(query.Arg("order"))
	if order == nil {
		return
	}

	// Alias -> type from fully-annotated projections (last match wins, consistent with how
	// expandAliasRefs handles duplicate aliases).
	aliasTypes := map[string]exp.Expression{}
	for _, sel := range query.Selects() {
		if sel.Kind() == exp.KindAlias && sel.This() != nil && sel.This().Type() != nil && !sel.This().IsType(exp.DTypeUnknown) {
			aliasTypes[sel.Alias()] = sel.This().Type()
		}
	}
	if len(aliasTypes) == 0 {
		return
	}

	for _, ordered := range order.Expressions() {
		var aliasCols []exp.Expression
		for _, c := range ordered.FindAll(exp.KindColumn) {
			if c.Text("table") == "" {
				if _, ok := aliasTypes[c.Name()]; ok {
					aliasCols = append(aliasCols, c)
				}
			}
		}
		for _, col := range aliasCols {
			a.setType(col, aliasTypes[col.Name()])
		}
		if len(aliasCols) > 0 {
			for _, node := range ordered.WalkWithPrune(true, func(n exp.Expression) bool { return n.Kind() == exp.KindSubquery }) {
				if node.Kind() != exp.KindColumn && node.Kind() != exp.KindLiteral {
					delete(a.visited, node)
				}
			}
			a.annotateExpression(ordered, scope)
		}
	}
}

// maybeCoerce ports _maybe_coerce (annotate_types.py:585-620): type2 if type1 can be coerced
// into it, otherwise type1. A parameterized type (e.g. DECIMAL(18, 2)) is assumed not to coerce.
func (a *TypeAnnotator) maybeCoerce(type1, type2 any) any {
	var type1Value, type2Value exp.DType
	if dt, ok := type1.(exp.Expression); ok && dt != nil {
		d, isDType := dt.Arg("this").(exp.DType)
		if len(dt.Expressions()) > 0 || !isDType {
			return type1
		}
		type1Value = d
	} else {
		type1Value = typeThis(type1)
	}
	if dt, ok := type2.(exp.Expression); ok && dt != nil {
		d, isDType := dt.Arg("this").(exp.DType)
		if len(dt.Expressions()) > 0 || !isDType {
			return type2
		}
		type2Value = d
	} else {
		type2Value = typeThis(type2)
	}

	// We propagate the UNKNOWN type upwards if found.
	if type1Value == exp.DTypeUnknown || type2Value == exp.DTypeUnknown {
		return exp.DTypeUnknown
	}
	if type1Value == exp.DTypeNull {
		return type2Value
	}
	if type2Value == exp.DTypeNull {
		return type1Value
	}
	if a.coercesTo[type1Value][type2Value] {
		return type2Value
	}
	return type1Value
}

// getSetopColumnTypes ports _get_setop_column_types (annotate_types.py:622-688): the coerced
// column types of a set operation, across the operands of its whole chain.
func (a *TypeAnnotator) getSetopColumnTypes(setop exp.Expression) *typedMap {
	if cached, ok := a.setopColumnTypes[setop]; ok {
		return cached
	}
	colTypes := newTypedMap()

	// Validate that left and right have the same number of projections (BY NAME operations
	// match columns by name, so their counts are allowed to differ).
	if !setop.Is(exp.TraitSetOperation) || len(setop.This().Selects()) == 0 || len(setop.Expr().Selects()) == 0 ||
		!(truthyArg(setop.Arg("by_name")) || len(setop.This().Selects()) == len(setop.Expr().Selects())) {
		return colTypes
	}

	// Process a chain / sub-tree of set operations.
	for _, setOp := range setop.WalkWithPrune(true, func(n exp.Expression) bool {
		return !n.Is(exp.TraitSetOperation) && n.Kind() != exp.KindSubquery
	}) {
		if !setOp.Is(exp.TraitSetOperation) {
			continue
		}

		setopCols := newTypedMap()
		if truthyArg(setOp.Arg("by_name")) {
			// Columns missing from one side are filled with NULLs, so the other side's type
			// is preserved (NULL is the identity for maybeCoerce).
			rTypeBySelect := newTypedMap()
			for _, s := range setOp.Expr().Selects() {
				rTypeBySelect.set(s.AliasOrName(), s.Type())
			}
			for _, s := range setOp.This().Selects() {
				name := s.AliasOrName()
				var rType any = exp.DTypeNull
				if _, ok := rTypeBySelect.values[name]; ok {
					rType = rTypeBySelect.values[name]
					if isNilType(rType) {
						rType = exp.DTypeUnknown
					}
					delete(rTypeBySelect.values, name)
				}
				setopCols.set(name, a.maybeCoerce(s.Type(), rType))
			}
			for _, name := range rTypeBySelect.keys {
				rType, ok := rTypeBySelect.values[name]
				if !ok {
					continue
				}
				if isNilType(rType) {
					rType = exp.DTypeUnknown
				}
				setopCols.set(name, rType)
			}
		} else {
			left, right := setOp.This().Selects(), setOp.Expr().Selects()
			for i := 0; i < len(left) && i < len(right); i++ {
				setopCols.set(left[i].AliasOrName(), a.maybeCoerce(left[i].Type(), right[i].Type()))
			}
		}

		// Coerce intermediate results with the previously registered types, if they exist.
		for _, name := range setopCols.keys {
			previous := colTypes.get(name)
			if isNilType(previous) {
				previous = exp.DTypeNull
			}
			colTypes.set(name, a.maybeCoerce(setopCols.values[name], previous))
		}
	}

	a.setopColumnTypes[setop] = colTypes
	return colTypes
}

// annotateBinary ports _annotate_binary (annotate_types.py:752-779).
func (a *TypeAnnotator) annotateBinary(expression exp.Expression) exp.Expression {
	left, right := expression.Left(), expression.Right()
	if left == nil || right == nil {
		a.setType(expression, nil)
		return expression
	}

	leftType, rightType := exp.DTypeUnknown, exp.DTypeUnknown
	if t := left.Type(); t != nil {
		leftType = typeThis(t)
	}
	if t := right.Type(); t != nil {
		rightType = typeThis(t)
	}

	if expression.Is(exp.TraitConnector) || expression.Is(exp.TraitPredicate) {
		a.setType(expression, exp.DTypeBoolean)
	} else if fn, ok := a.binaryCoercions[dtypePair{leftType, rightType}]; ok {
		a.setType(expression, fn(left, right))
	} else {
		a.annotateByArgs(expression, annotateArgs{exprs: []exp.Expression{left, right}})
	}

	if expression.Kind() == exp.KindIs || (left.MetaGet("nonnull") == true && right.MetaGet("nonnull") == true) {
		expression.Meta()["nonnull"] = true
	}
	return expression
}

// annotateUnary ports _annotate_unary (annotate_types.py:781-790).
func (a *TypeAnnotator) annotateUnary(expression exp.Expression) exp.Expression {
	this := expression.This()
	if expression.Kind() == exp.KindNot {
		a.setType(expression, exp.DTypeBoolean)
	} else if this != nil {
		a.setType(expression, this.Type())
	} else {
		a.setType(expression, nil)
	}
	if this != nil && this.MetaGet("nonnull") == true {
		expression.Meta()["nonnull"] = true
	}
	return expression
}

// annotateLiteral ports _annotate_literal (annotate_types.py:792-802).
func (a *TypeAnnotator) annotateLiteral(expression exp.Expression) exp.Expression {
	switch {
	case expression.IsString():
		a.setType(expression, exp.DTypeVarchar)
	case expression.IsInt():
		a.setType(expression, exp.DTypeInt)
	default:
		a.setType(expression, exp.DTypeDouble)
	}
	expression.Meta()["nonnull"] = true
	return expression
}

// annotateArgs carries _annotate_by_args' arguments: arg keys of the expression and/or
// explicit expressions, in order, plus the promote and array flags.
type annotateArgs struct {
	keys    []string
	exprs   []exp.Expression
	promote bool
	array   bool
	// order interleaves keys and exprs as given: true takes the next key, false the next expr.
	order []bool
}

func byKeys(keys ...string) annotateArgs { return annotateArgs{keys: keys} }

// argValues yields the expressions _annotate_by_args iterates, in argument order.
func (args annotateArgs) values(expression exp.Expression) []exp.Expression {
	var out []exp.Expression
	appendKey := func(key string) {
		switch v := expression.Arg(key).(type) {
		case exp.Expression:
			if v != nil {
				out = append(out, v)
			}
		case []exp.Expression:
			out = append(out, v...)
		}
	}
	if len(args.order) == 0 {
		for _, key := range args.keys {
			appendKey(key)
		}
		out = append(out, args.exprs...)
		return out
	}
	ki, ei := 0, 0
	for _, isKey := range args.order {
		if isKey {
			appendKey(args.keys[ki])
			ki++
		} else {
			if args.exprs[ei] != nil {
				out = append(out, args.exprs[ei])
			}
			ei++
		}
	}
	return out
}

// annotateByArgs ports _annotate_by_args (annotate_types.py:804-885).
func (a *TypeAnnotator) annotateByArgs(expression exp.Expression, args annotateArgs) exp.Expression {
	var literalType, nonLiteralType, nestedType any

	for _, expr := range args.values(expression) {
		exprType := expr.Type()
		if exprType == nil || exprType.IsType(exp.DTypeUnknown) {
			a.setType(expression, exp.DTypeUnknown)
			return expression
		}
		if nestedType != nil {
			continue
		}
		// Stop coercing at the first nested data type found.
		if truthyArg(exprType.Arg("nested")) {
			nestedType = exprType
		} else if expr.Kind() == exp.KindLiteral {
			if literalType == nil {
				literalType = exprType
			}
			literalType = a.maybeCoerce(literalType, exprType)
		} else {
			if nonLiteralType == nil {
				nonLiteralType = exprType
			}
			nonLiteralType = a.maybeCoerce(nonLiteralType, exprType)
		}
	}

	var resultType any
	switch {
	case nestedType != nil:
		resultType = nestedType
	case literalType != nil && nonLiteralType != nil:
		if a.dialect.PrioritizeNonLiteralTypes {
			l, n := typeThis(literalType), typeThis(nonLiteralType)
			if (exp.IntegerTypes[l] && exp.IntegerTypes[n]) || (exp.RealTypes[l] && exp.RealTypes[n]) {
				resultType = nonLiteralType
			}
		}
		if resultType == nil {
			resultType = a.maybeCoerce(nonLiteralType, literalType)
		}
	case literalType != nil:
		resultType = literalType
	case nonLiteralType != nil:
		resultType = nonLiteralType
	default:
		resultType = exp.DTypeUnknown
	}

	a.setType(expression, resultType)

	if args.promote {
		thisType := typeThis(resultType)
		if exp.IntegerTypes[thisType] {
			a.setType(expression, exp.DTypeBigInt)
		} else if exp.FloatTypes[thisType] {
			a.setType(expression, exp.DTypeDouble)
		}
	}

	if args.array {
		a.setType(expression, exp.DataType(exp.Args{
			"this":        exp.DTypeArray,
			"expressions": []exp.Expression{expression.Type()},
			"nested":      true,
		}))
	}
	return expression
}

// annotateTimeunit ports _annotate_timeunit (annotate_types.py:887-899).
func (a *TypeAnnotator) annotateTimeunit(expression exp.Expression) exp.Expression {
	this := expression.This()
	unit := asExpression(expression.Arg("unit"))
	var datatype any = exp.DTypeUnknown
	if this != nil && this.Type() != nil {
		switch thisType := typeThis(this.Type()); {
		case exp.TextTypes[thisType]:
			datatype = coerceDateLiteral(this, unit)
		case exp.TemporalTypes[thisType]:
			datatype = coerceDate(this, unit)
		}
	}
	a.setType(expression, datatype)
	return expression
}

// annotateBracket ports _annotate_bracket (annotate_types.py:901-916).
func (a *TypeAnnotator) annotateBracket(expression exp.Expression) exp.Expression {
	exprs := expression.Expressions()
	this := expression.This()
	if len(exprs) == 0 || this == nil {
		a.setType(expression, exp.DTypeUnknown)
		return expression
	}
	bracketArg := exprs[0]

	switch {
	case bracketArg.Kind() == exp.KindSlice:
		a.setType(expression, this.Type())
	case this.Type() != nil && this.Type().IsType(exp.DTypeArray):
		var elem exp.Expression
		if e := this.Type().Expressions(); len(e) > 0 {
			elem = e[0]
		}
		a.setType(expression, elem)
	case this.Kind() == exp.KindMap || this.Kind() == exp.KindVarMap:
		keys := mapKeys(this)
		index := indexOfExpr(keys, bracketArg)
		if index < 0 {
			a.setType(expression, exp.DTypeUnknown)
			break
		}
		values := mapValues(this)
		if index < len(values) && values[index] != nil {
			a.setType(expression, values[index].Type())
		} else {
			a.setType(expression, nil)
		}
	default:
		a.setType(expression, exp.DTypeUnknown)
	}
	return expression
}

// mapKeys / mapValues port Map.keys / Map.values (expressions/array.py): the elements of the
// keys / values Array.
func mapKeys(m exp.Expression) []exp.Expression {
	if keys := asExpression(m.Arg("keys")); keys != nil && keys.Kind() == exp.KindArray {
		return keys.Expressions()
	}
	return nil
}

func mapValues(m exp.Expression) []exp.Expression {
	if values := asExpression(m.Arg("values")); values != nil && values.Kind() == exp.KindArray {
		return values.Expressions()
	}
	return nil
}

// annotateDiv ports _annotate_div (annotate_types.py:918-934).
func (a *TypeAnnotator) annotateDiv(expression exp.Expression) exp.Expression {
	leftType, rightType := exp.DTypeUnknown, exp.DTypeUnknown
	if left := expression.Left(); left != nil && left.Type() != nil {
		leftType = typeThis(left.Type())
	}
	if right := expression.Right(); right != nil && right.Type() != nil {
		rightType = typeThis(right.Type())
	}

	if truthyArg(expression.Arg("typed")) && exp.IntegerTypes[leftType] && exp.IntegerTypes[rightType] {
		a.setType(expression, exp.DTypeBigInt)
	} else {
		a.setType(expression, a.maybeCoerce(leftType, rightType))
		if typ := expression.Type(); typ != nil && !exp.RealTypes[typeThis(typ)] {
			a.setType(expression, a.maybeCoerce(typ, exp.DTypeDouble))
		}
	}
	return expression
}

// annotateDot ports _annotate_dot (annotate_types.py:936-952).
func (a *TypeAnnotator) annotateDot(expression exp.Expression) exp.Expression {
	a.setType(expression, nil)

	// Propagate the type from qualified UDF calls (e.g., db.my_udf(...)).
	if e := expression.Expr(); e != nil && e.Kind() == exp.KindAnonymous {
		a.setType(expression, e.Type())
		return expression
	}

	if this := expression.This(); this != nil {
		if thisType := this.Type(); thisType != nil && thisType.IsType(exp.DTypeStruct) {
			for _, e := range thisType.Expressions() {
				if expression.Expr() != nil && e.Name() == expression.Expr().Name() {
					a.setType(expression, asExpression(e.Arg("kind")))
					break
				}
			}
		}
	}
	return expression
}

// annotateExplode ports _annotate_explode (annotate_types.py:954-960).
func (a *TypeAnnotator) annotateExplode(expression exp.Expression) exp.Expression {
	if this := expression.This(); this != nil {
		if inputType := this.Type(); inputType != nil && inputType.IsType(exp.DTypeArray) {
			var elem exp.Expression
			if e := inputType.Expressions(); len(e) > 0 {
				elem = e[0]
			}
			a.setType(expression, elem)
			return expression
		}
	}
	a.setType(expression, nil)
	return expression
}

// annotateUnnest ports _annotate_unnest (annotate_types.py:962-971).
func (a *TypeAnnotator) annotateUnnest(expression exp.Expression) exp.Expression {
	var exprType exp.Expression
	if exprs := expression.Expressions(); len(exprs) > 0 && exprs[0].IsType(exp.DTypeArray) {
		if e := exprs[0].Type().Expressions(); len(e) > 0 {
			exprType = e[0]
		}
	}
	a.setType(expression, exprType)
	return expression
}

// annotateSubquery ports _annotate_subquery (annotate_types.py:973-987): a scalar subquery (a
// single projection) has that projection's type.
func (a *TypeAnnotator) annotateSubquery(expression exp.Expression) exp.Expression {
	if query := expression.Unnest(); query != nil && query.Is(exp.TraitQuery) {
		if selects := query.Selects(); len(selects) == 1 {
			a.setType(expression, selects[0].Type())
			return expression
		}
	}
	a.setType(expression, exp.DTypeUnknown)
	return expression
}

// annotateStructValue ports _annotate_struct_value (annotate_types.py:989-1011). It returns a
// ColumnDef, a DataType, or nil.
func (a *TypeAnnotator) annotateStructValue(expression exp.Expression) exp.Expression {
	var this exp.Expression
	kind := expression.Type()

	if alias := asExpression(expression.Arg("alias")); alias != nil {
		// Case: STRUCT(key AS value).
		this = alias.Copy()
	} else if expression.Expr() != nil {
		// Case: STRUCT(key = value) or STRUCT(key := value).
		this = expression.This().Copy()
		kind = expression.Expr().Type()
	} else if expression.Kind() == exp.KindColumn {
		// Case: STRUCT(c).
		this = expression.This().Copy()
	}

	if kind != nil && kind.IsType(exp.DTypeUnknown) {
		return nil
	}
	if this != nil {
		return exp.ColumnDef(exp.Args{"this": this, "kind": kind})
	}
	return kind
}

// annotateStruct ports _annotate_struct (annotate_types.py:1013-1027).
func (a *TypeAnnotator) annotateStruct(expression exp.Expression) exp.Expression {
	var fields []exp.Expression
	for _, expr := range expression.Expressions() {
		fieldType := a.annotateStructValue(expr)
		if fieldType == nil {
			a.setType(expression, nil)
			return expression
		}
		fields = append(fields, fieldType)
	}
	a.setType(expression, structType(fields))
	return expression
}

// annotateMap ports _annotate_map (annotate_types.py:1035-1049).
func (a *TypeAnnotator) annotateMap(expression exp.Expression) exp.Expression {
	keys := asExpression(expression.Arg("keys"))
	values := asExpression(expression.Arg("values"))

	mapType := exp.DataType(exp.Args{"this": exp.DTypeMap})
	if keys != nil && values != nil && keys.Kind() == exp.KindArray && values.Kind() == exp.KindArray {
		keyType := firstElementType(keys)
		valueType := firstElementType(values)
		if keyType != nil && valueType != nil {
			mapType.Set("expressions", []exp.Expression{keyType, valueType})
			mapType.Set("nested", true)
		}
	}
	a.setType(expression, mapType)
	return expression
}

// firstElementType is `seq_get(arr.type.expressions, 0)` when that is not UNKNOWN, else nil.
func firstElementType(arr exp.Expression) exp.Expression {
	typ := arr.Type()
	if typ == nil {
		return nil
	}
	exprs := typ.Expressions()
	if len(exprs) == 0 || exprs[0].IsType(exp.DTypeUnknown) {
		return nil
	}
	return exprs[0]
}

// annotateToMap ports _annotate_to_map (annotate_types.py:1051-1063).
func (a *TypeAnnotator) annotateToMap(expression exp.Expression) exp.Expression {
	mapType := exp.DataType(exp.Args{"this": exp.DTypeMap})
	if arg := expression.This(); arg != nil && arg.IsType(exp.DTypeStruct) {
		for _, colDef := range arg.Type().Expressions() {
			kind := asExpression(colDef.Arg("kind"))
			if kind != nil && !kind.IsType(exp.DTypeUnknown) {
				mapType.Set("expressions", []exp.Expression{exp.DataType(exp.Args{"this": exp.DTypeVarchar}), kind})
				mapType.Set("nested", true)
				break
			}
		}
	}
	a.setType(expression, mapType)
	return expression
}

// annotateExtract ports _annotate_extract (annotate_types.py:1065-1075).
func (a *TypeAnnotator) annotateExtract(expression exp.Expression) exp.Expression {
	switch part := expression.Name(); {
	case part == "TIME":
		a.setType(expression, exp.DTypeTime)
	case part == "DATE":
		a.setType(expression, exp.DTypeDate)
	case bigintExtractDateParts[part]:
		a.setType(expression, exp.DTypeBigInt)
	default:
		a.setType(expression, exp.DTypeInt)
	}
	return expression
}

// annotateWithinGroup ports _annotate_within_group (annotate_types.py:1077-1085).
func (a *TypeAnnotator) annotateWithinGroup(expression exp.Expression) exp.Expression {
	if this := expression.This(); this != nil && isClass(this, "PercentileDisc") {
		var sortType any = exp.DTypeUnknown
		if order := asExpression(expression.Arg("expression")); order != nil {
			if exprs := order.Expressions(); len(exprs) > 0 && exprs[0].This() != nil {
				sortType = exprs[0].This().Type()
			}
		}
		a.setType(expression, sortType)
		return expression
	}
	return a.annotateByArgs(expression, byKeys("this"))
}

// annotateByArrayElement ports _annotate_by_array_element (annotate_types.py:1087-1098).
func (a *TypeAnnotator) annotateByArrayElement(expression exp.Expression) exp.Expression {
	if arrayArg := expression.This(); arrayArg != nil && arrayArg.Type() != nil && arrayArg.Type().IsType(exp.DTypeArray) {
		var elem any = exp.DTypeUnknown
		if e := arrayArg.Type().Expressions(); len(e) > 0 {
			elem = e[0]
		}
		a.setType(expression, elem)
		return expression
	}
	a.setType(expression, exp.DTypeUnknown)
	return expression
}
