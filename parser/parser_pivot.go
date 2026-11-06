package parser

import (
	"strings"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

func (p *Parser) parseLateral() exp.Expression {
	var crossApply any
	if p.matchPair(tokens.CROSS, tokens.APPLY, true) {
		crossApply = true
	} else if p.matchPair(tokens.OUTER, tokens.APPLY, true) {
		crossApply = false
	}

	var this exp.Expression
	var view any
	var outer any
	if crossApply != nil {
		this = p.parseSelect(false, true, true, true)
	} else if p.match(tokens.LATERAL) {
		this = p.parseSelect(false, true, true, true)
		view = p.match(tokens.VIEW)
		outer = p.match(tokens.OUTER)
	} else {
		return nil
	}

	if this == nil {
		// Mirror upstream `_parse_unnest() or _parse_function() or _parse_id_var(any_token=False)`:
		// short-circuit so a later alternative never runs (and never advances) once an
		// earlier one succeeds. Eager evaluation here would let parseIdVar swallow a
		// trailing bare table alias after parseFunction consumes a `func(...)` source
		// (e.g. `CROSS APPLY func(y) t`), silently dropping the alias.
		this = p.parseUnnest(true)
		if this == nil {
			this = p.parseFunction(nil, false, true, false)
		}
		if this == nil {
			this = p.parseIdVar(false, nil)
		}
		for p.match(tokens.DOT) {
			field := p.parseFunction(nil, false, true, false)
			if field == nil {
				field = p.parseIdVar(false, nil)
			}
			this = p.expression(exp.Dot(exp.Args{"this": this, "expression": field}), nil, nil)
		}
	}

	var ordinality any
	var tableAlias exp.Expression
	if viewValue, _ := view.(bool); viewValue {
		table := p.parseIdVar(false, nil)
		var columns []exp.Expression
		if p.match(tokens.ALIAS) {
			columns = p.parseCsv(func() exp.Expression { return p.parseIdVar(false, nil) })
		}
		tableAlias = p.expression(exp.TableAlias(exp.Args{"this": table, "columns": columns}), nil, nil)
	} else if this != nil && (this.Kind() == exp.KindSubquery || this.Kind() == exp.KindUnnest) && this.Arg("alias") != nil {
		tableAlias = asExpr(this.Arg("alias"))
		this.Set("alias", nil)
	} else {
		if p.matchPair(tokens.WITH, tokens.ORDINALITY, true) {
			ordinality = true
		}
		tableAlias = p.parseTableAlias(nil)
	}

	return p.expression(exp.Lateral(exp.Args{"this": this, "view": view, "outer": outer, "alias": tableAlias, "cross_apply": crossApply, "ordinality": ordinality}), nil, nil)
}

func (p *Parser) parseUnnest(withAlias bool) exp.Expression {
	if !p.matchPair(tokens.UNNEST, tokens.L_PAREN, false) {
		return nil
	}
	p.advance()
	expressions := p.parseWrappedCsv(p.parseEquality)
	var offset any
	if p.matchPair(tokens.WITH, tokens.ORDINALITY, true) {
		offset = true
	}
	var alias exp.Expression
	if withAlias {
		alias = p.parseTableAlias(nil)
	}
	// parser.py:5205-5207: with WITH ORDINALITY, a surplus alias column names the ordinal.
	if alias != nil && offset != nil {
		if columns, _ := alias.Arg("columns").([]exp.Expression); len(expressions) < len(columns) {
			offset = columns[len(columns)-1]
			alias.Set("columns", columns[:len(columns)-1])
		}
	}
	// divergence: PostgreSQL has no WITH OFFSET (BigQuery-only); upstream's shared grammar
	// accepts it in every dialect. See DEVIATIONS §1.21.
	if offset == nil && p.dialect.Name != "postgres" && p.matchPair(tokens.WITH, tokens.OFFSET, true) {
		p.match(tokens.ALIAS)
		offset = p.parseIdVar(false, nil)
		if offset == nil {
			offset = exp.ToIdentifier("offset", false)
		}
	}
	result := p.expression(exp.Unnest(exp.Args{"expressions": expressions, "alias": alias, "offset": offset}), nil, nil)
	// parsers/starrocks.py:78-95: UNNEST defaults its table alias/column to "unnest".
	if p.dialectIs("starrocks") {
		unnestID := func() exp.Expression { return exp.ToIdentifier("unnest") }
		aliasExpr := asExpr(result.Arg("alias"))
		if aliasExpr == nil {
			result.Set("alias", exp.TableAlias(exp.Args{
				"this":    unnestID(),
				"columns": []exp.Expression{unnestID()},
			}))
		} else if cols, _ := aliasExpr.Arg("columns").([]exp.Expression); len(cols) == 0 {
			aliasExpr.Set("columns", []exp.Expression{unnestID()})
		}
	}
	return result
}

func (p *Parser) parseDerivedTableValues() exp.Expression {
	isDerived := p.matchPair(tokens.L_PAREN, tokens.VALUES, true)
	if !isDerived && !p.matchTextSeq("VALUES") && !p.matchTextSeq("FORMAT", "VALUES") {
		return nil
	}
	expressions := p.parseCsv(func() exp.Expression { return p.parseValue(true) })
	alias := p.parseTableAlias(nil)
	if isDerived {
		p.matchRParen(nil)
	}
	return p.expression(exp.Values(exp.Args{"expressions": expressions, "alias": firstExpression(alias, p.parseTableAlias(nil))}), nil, nil)
}

func (p *Parser) parsePivots() []exp.Expression {
	if p.curr.TokenType != tokens.PIVOT && p.curr.TokenType != tokens.UNPIVOT {
		return nil
	}
	pivots := []exp.Expression{}
	for {
		pivot := p.parsePivot()
		if pivot == nil {
			break
		}
		pivots = append(pivots, pivot)
	}
	if len(pivots) == 0 {
		return nil
	}
	return pivots
}

func (p *Parser) parsePivot() exp.Expression {
	index := p.index
	var includeNulls any
	var unpivot bool
	if p.match(tokens.PIVOT) {
		unpivot = false
	} else if p.match(tokens.UNPIVOT) {
		unpivot = true
		if p.matchTextSeq("INCLUDE", "NULLS") {
			includeNulls = true
		} else if p.matchTextSeq("EXCLUDE", "NULLS") {
			includeNulls = false
		}
	} else {
		return nil
	}
	if !p.match(tokens.L_PAREN) {
		p.retreat(index)
		return nil
	}
	var expressions []exp.Expression
	if unpivot {
		expressions = p.parseCsv(p.parseColumn)
	} else {
		expressions = p.parseCsv(p.parsePivotAggregation)
	}
	if len(expressions) == 0 {
		p.raiseError("Failed to parse PIVOT's aggregation list")
	}
	if !p.match(tokens.FOR) {
		p.raiseError("Expecting FOR")
	}
	fields := []exp.Expression{}
	for {
		field := p.tryParse(p.parsePivotIn, false)
		if field == nil {
			break
		}
		fields = append(fields, field)
	}
	var defaultOnNull exp.Expression
	if p.matchTextSeq("DEFAULT", "ON", "NULL") {
		defaultOnNull = p.parseWrapped(p.parseBitwise, false)
	}
	group := p.parseGroup(false)
	p.matchRParen(nil)
	pivot := p.expression(exp.Pivot(exp.Args{"expressions": expressions, "fields": fields, "unpivot": unpivot, "include_nulls": includeNulls, "default_on_null": defaultOnNull, "group": group}), nil, nil)
	if unpivot {
		for i, expression := range expressions {
			expressions[i] = unpivotTarget(expression)
		}
		pivot.Set("expressions", expressions)
		for _, field := range fields {
			if field != nil && field.Kind() == exp.KindIn {
				field.Set("this", unpivotTarget(asExpr(field.Arg("this"))))
			}
		}
	}
	if unpivot {
		pivot.Set("value_columns_first", p.dialect.UnpivotValueColumnsFirst)
	}
	if !p.matchSet(map[tokens.TokenType]bool{tokens.PIVOT: true, tokens.UNPIVOT: true}, false) {
		pivot.Set("alias", p.parseTableAlias(nil))
	}
	if !unpivot {
		p.setPivotColumns(pivot, expressions)
	}
	return pivot
}

// setPivotColumns ports the PIVOT output-column inference of _parse_pivot (parser.py:5463-5505):
// the product of each field's IN values and the aggregation aliases, joined by "_". No registered
// dialect sets IDENTIFY_PIVOT_STRINGS or PREFIXED_PIVOT_COLUMNS, so a value is named by its
// alias_or_name and the aggregation alias goes last; the flags are recorded on the node as
// upstream does.
func (p *Parser) setPivotColumns(pivot exp.Expression, aggregations []exp.Expression) {
	// _pivot_column_names (parser.py:5509-5510).
	var names []string
	for _, agg := range aggregations {
		if alias := agg.Alias(); alias != "" {
			names = append(names, alias)
		}
	}

	var allFields [][]string
	for _, field := range pivot.(*exp.Node).ExpressionsFor("fields") {
		fieldExpressions := field.Expressions()
		// PivotAny is `ANY ORDER BY <column>`; its columns can't be inferred.
		if len(fieldExpressions) > 0 && fieldExpressions[0].Kind() == exp.KindPivotAny {
			continue
		}
		var parts []string
		for _, fld := range fieldExpressions {
			parts = append(parts, fld.AliasOrName())
		}
		allFields = append(allFields, parts)
	}

	columns := []exp.Expression{}
	if len(allFields) > 0 {
		if len(names) > 0 {
			allFields = append(allFields, names)
		}
		// Every combination of the pivot columns, e.g. PIVOT(sum(...) AS total FOR year IN
		// (2000, 2010) FOR country IN ('NL', 'US')) is the product of [[2000, 2010],
		// ['NL', 'US'], ['total']].
		combos := [][]string{{}}
		for _, parts := range allFields {
			var next [][]string
			for _, combo := range combos {
				for _, part := range parts {
					next = append(next, append(append([]string{}, combo...), part))
				}
			}
			combos = next
		}
		for _, combo := range combos {
			columns = append(columns, exp.ToIdentifier(strings.Join(combo, "_")))
		}
	}

	pivot.Set("columns", columns)
	pivot.Set("identify_pivot_strings", false)
	pivot.Set("prefixed_pivot_columns", false)
	pivot.Set("pivot_column_naming", "agg_name_if_aliased")
}

func (p *Parser) parsePivotAggregation() exp.Expression {
	f := p.parseFunction(nil, false, true, false)
	if f == nil {
		if p.prev.TokenType == tokens.COMMA {
			return nil
		}
		p.raiseError("Expecting an aggregation function in PIVOT")
	}
	return p.parseAlias(f, false)
}

func (p *Parser) parsePivotIn() exp.Expression {
	value := p.parseColumn()
	if !p.match(tokens.IN) {
		p.raiseError("Expecting IN")
	}
	if p.match(tokens.L_PAREN) {
		var exprs []exp.Expression
		if p.match(tokens.ANY) {
			exprs = []exp.Expression{p.expression(exp.PivotAny(exp.Args{"this": p.parseOrder(nil, false)}), nil, nil)}
		} else {
			exprs = p.parseCsv(p.parseAliasedPivotExpression)
		}
		p.matchRParen(nil)
		return p.expression(exp.In(exp.Args{"this": value, "expressions": exprs}), nil, nil)
	}
	return p.expression(exp.In(exp.Args{"this": value, "field": p.parseIdVar(false, nil)}), nil, nil)
}

func (p *Parser) parseAliasedPivotExpression() exp.Expression {
	this := p.parseSelectOrExpression(false)
	p.match(tokens.ALIAS)
	alias := p.parseBitwise()
	if alias != nil {
		if alias.Kind() == exp.KindColumn && alias.Arg("schema") == nil {
			alias = alias.This()
		}
		return p.expression(exp.PivotAlias(exp.Args{"this": this, "alias": alias}), nil, nil)
	}
	return this
}

func unpivotTarget(expression exp.Expression) exp.Expression {
	if expression == nil {
		return nil
	}
	if expression.Kind() == exp.KindColumn && expression.Arg("table") == nil {
		return expression.This()
	}
	if expression.Kind() == exp.KindTuple {
		expressions := expression.Expressions()
		for i, child := range expressions {
			expressions[i] = unpivotTarget(child)
		}
		expression.Set("expressions", expressions)
	}
	return expression
}
