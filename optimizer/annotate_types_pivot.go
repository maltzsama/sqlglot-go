package optimizer

import (
	"fmt"

	sqlerrors "github.com/maltzsama/sqlglot-go/errors"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/schema"
)

// orderedRenames is Pivot.output_columns' ordered post-rename -> pre-rename map.
type orderedRenames struct {
	keys []string
	pre  map[string]string
}

// pivotOutputColumns ports Pivot.output_columns (expressions/query.py:1780-1848): the
// (UN)PIVOT's output columns, in the order it produces them, mapped to their pre-rename
// source-side names.
func pivotOutputColumns(pivot exp.Expression, prePivotColumns []string) orderedRenames {
	out := orderedRenames{pre: map[string]string{}}
	excluded := map[string]bool{}
	var outputs []string

	if truthyArg(pivot.Arg("unpivot")) {
		var nameColumns, valueColumns []exp.Expression
		for _, field := range expressionsFor(pivot, "fields") {
			if field.Kind() != exp.KindIn {
				continue
			}
			if this := field.This(); this != nil && this.Kind() == exp.KindIdentifier {
				nameColumns = append(nameColumns, this)
			}
			for _, e := range field.Expressions() {
				for _, c := range e.FindAll(exp.KindColumn) {
					excluded[c.OutputName()] = true
				}
			}
		}
		for _, e := range pivot.Expressions() {
			idents := []exp.Expression{e}
			if e.Kind() == exp.KindTuple {
				idents = e.Expressions()
			}
			for _, ident := range idents {
				if ident.Kind() == exp.KindIdentifier {
					valueColumns = append(valueColumns, ident)
				}
			}
		}
		// T-SQL emits the value column(s) ahead of the name column, everyone else after it.
		ordered := append(append([]exp.Expression{}, nameColumns...), valueColumns...)
		if truthyArg(pivot.Arg("value_columns_first")) {
			ordered = append(append([]exp.Expression{}, valueColumns...), nameColumns...)
		}
		for _, i := range ordered {
			outputs = append(outputs, i.Name())
		}
	} else {
		for _, c := range pivot.FindAll(exp.KindColumn) {
			excluded[c.OutputName()] = true
		}
		for _, c := range expressionsFor(pivot, "columns") {
			outputs = append(outputs, c.OutputName())
		}
		if len(outputs) == 0 {
			for _, c := range pivot.Expressions() {
				outputs = append(outputs, c.AliasOrName())
			}
		}
	}

	if len(excluded) == 0 || len(outputs) == 0 {
		return out
	}

	var preRename []string
	for _, c := range prePivotColumns {
		if !excluded[c] {
			preRename = append(preRename, c)
		}
	}
	preRename = append(preRename, outputs...)

	// `PIVOT(...) AS alias(c1, c2, ...)` renames the output columns positionally from the
	// front (DuckDB, Snowflake); the remaining columns keep their auto names.
	postRename := preRename
	if alias := asExpression(pivot.Arg("alias")); alias != nil {
		if renames := expressionsFor(alias, "columns"); len(renames) > 0 {
			postRename = nil
			for _, r := range renames {
				postRename = append(postRename, r.Name())
			}
			if len(renames) < len(preRename) {
				postRename = append(postRename, preRename[len(renames):]...)
			}
		}
	}

	// dict(zip(post_rename, pre_rename)): a repeated output name keeps its first position and
	// its last source name.
	for i := 0; i < len(postRename) && i < len(preRename); i++ {
		if _, ok := out.pre[postRename[i]]; !ok {
			out.keys = append(out.keys, postRename[i])
		}
		out.pre[postRename[i]] = preRename[i]
	}
	return out
}

// pivotSourceSelects is the pivot branch of _get_scope_source_selects
// (annotate_types.py:309-345): the column types a (chain of) PIVOT / UNPIVOT produces for the
// source named sourceName, or nil when sourceName is not a pivot's output.
func (a *TypeAnnotator) pivotSourceSelects(scope *Scope, source any, sourceName string) *typedMap {
	var pivots []exp.Expression
	if table, ok := source.(exp.Expression); ok && table != nil && table.Kind() == exp.KindTable {
		pivots = expressionsFor(table, "pivots")
	} else {
		pivots = scope.Pivots()
	}
	// Only the last operator in a chain carries the alias that names the resulting source, so
	// match on it and fold the whole chain in order.
	if len(pivots) == 0 || pivots[len(pivots)-1].AliasOrName() != sourceName {
		return nil
	}

	parent := pivots[len(pivots)-1].Parent()
	var parentSource any
	if parent != nil {
		parentSource = scope.Sources[parent.AliasOrName()]
	}
	if _, isScope := parentSource.(*Scope); !isScope && parent != nil && parent.Kind() == exp.KindTable && parent.SchemaName() == "" {
		// A chain aliased like the CTE it reads from shadows it in scope.Sources, so reach for
		// the CTE's scope directly.
		parentSource = scope.CTESources[parent.Name()]
	}

	srcTypes := newTypedMap()
	if parentScope, ok := parentSource.(*Scope); ok {
		srcTypes = a.getSourceScopeSelects(parentScope)
	} else if _, isMapping := a.schema.(*schema.MappingSchema); isMapping && parent != nil && parent.Kind() == exp.KindTable {
		if columns, err := a.schema.Find(parent, false, true); err == nil && columns != nil {
			for _, name := range columns.Keys() {
				value, _ := columns.Get(name)
				srcTypes.set(name, value)
			}
		}
	}

	for _, pivot := range pivots {
		if truthyArg(pivot.Arg("unpivot")) {
			srcTypes = a.unpivotColumnTypes(pivot, srcTypes)
		} else {
			srcTypes = a.pivotColumnTypes(pivot, srcTypes)
		}
	}
	return srcTypes
}

// unpivotColumnTypes ports _get_unpivot_column_types (annotate_types.py:690-722).
func (a *TypeAnnotator) unpivotColumnTypes(pivot exp.Expression, srcTypes *typedMap) *typedMap {
	newTypes := newTypedMap()

	for _, field := range expressionsFor(pivot, "fields") {
		fieldCol := field.This()
		var first exp.Expression
		if exprs := field.Expressions(); len(exprs) > 0 {
			first = exprs[0]
		}

		var inSrc exp.Expression
		if first != nil && first.Kind() == exp.KindPivotAlias && asExpression(first.Arg("alias")) != nil {
			newTypes.set(fieldCol.Name(), asExpression(first.Arg("alias")).Type())
			inSrc = first.This()
		} else {
			newTypes.set(fieldCol.Name(), exp.DataType(exp.Args{"this": exp.DTypeVarchar}))
			inSrc = first
		}
		if inSrc == nil {
			continue
		}

		inCols := []exp.Expression{inSrc}
		if inSrc.Kind() == exp.KindTuple {
			inCols = inSrc.Expressions()
		}
		var valCols []exp.Expression
		if exprs := pivot.Expressions(); len(exprs) > 0 {
			valCols = []exp.Expression{exprs[0]}
			if exprs[0].Kind() == exp.KindTuple {
				valCols = exprs[0].Expressions()
			}
		}
		for i := 0; i < len(valCols) && i < len(inCols); i++ {
			// A chained operator's IN-list may name columns an earlier operator produced,
			// which carry no annotation of their own.
			var inType any = inCols[i].Type()
			if t := inCols[i].Type(); t == nil || t.IsType(exp.DTypeUnknown) {
				if srcType := srcTypes.get(inCols[i].OutputName()); !isNilType(srcType) {
					inType = srcType
				}
			}
			newTypes.set(valCols[i].OutputName(), inType)
		}
	}

	out := newTypedMap()
	for _, name := range pivotOutputColumns(pivot, srcTypes.keys).keys {
		typ := newTypes.get(name)
		if isNilType(typ) {
			typ = srcTypes.get(name)
		}
		if !isNilType(typ) {
			out.set(name, typ)
		}
	}
	return out
}

// pivotColumnTypes ports _get_pivot_column_types (annotate_types.py:724-750).
func (a *TypeAnnotator) pivotColumnTypes(pivot exp.Expression, srcTypes *typedMap) *typedMap {
	fields := expressionsFor(pivot, "fields")
	if len(fields) == 0 || fields[0].Kind() != exp.KindIn {
		got := "<nil>"
		if len(fields) > 0 {
			got = exp.ClassName(fields[0].Kind())
		}
		panic(&sqlerrors.OptimizeError{Msg: "Expected In expression for pivot field, got " + got})
	}
	pivotConstants := fields[0].Expressions()

	// The first aggColsOffset entries are source columns that pass through the PIVOT
	// unchanged; the rest are the aggregated columns, one per IN value and aggregate.
	outputToSrc := pivotOutputColumns(pivot, srcTypes.keys)

	var aggTypes []exp.Expression
	for _, agg := range pivot.Expressions() {
		if agg.Kind() == exp.KindAlias {
			aggTypes = append(aggTypes, agg.This().Type())
		} else {
			aggTypes = append(aggTypes, agg.Type())
		}
	}
	aggColsOffset := len(outputToSrc.keys) - len(pivotConstants)*len(aggTypes)
	if aggColsOffset < 0 {
		panic(&sqlerrors.OptimizeError{Msg: fmt.Sprintf("Negative pivot column offset: %d", aggColsOffset)})
	}

	newTypes := newTypedMap()
	for _, name := range outputToSrc.keys[:aggColsOffset] {
		if typ := srcTypes.get(outputToSrc.pre[name]); !isNilType(typ) {
			newTypes.set(name, typ)
		}
	}
	i := aggColsOffset
	for range pivotConstants {
		for _, aggType := range aggTypes {
			if i >= len(outputToSrc.keys) {
				break
			}
			if aggType != nil {
				newTypes.set(outputToSrc.keys[i], aggType)
			}
			i++
		}
	}
	return newTypes
}
