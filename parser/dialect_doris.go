package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// Doris parser overrides, ported from
// .reference/sqlglot-v30.17.0/sqlglot/parsers/doris.py.
func init() {
	registerDialectParserOverrides("doris", dialectParserOverrideSet{
		// parsers/doris.py:55-57: MySQL FUNCTION_PARSERS minus GROUP_CONCAT.
		DisabledFunctionParsers: map[string]bool{"GROUP_CONCAT": true},
		// parsers/doris.py:59-61: MySQL NO_PAREN_FUNCTIONS minus CURRENT_DATE.
		DisabledNoParenFunctions: map[tokens.TokenType]bool{tokens.CURRENT_DATE: true},
		PropertyParsers: map[string]propertyParserFunc{
			"PROPERTIES": func(p *Parser, _ bool) exp.Expression {
				return p.parsePropertiesList()
			},
			"UNIQUE": func(p *Parser, _ bool) exp.Expression {
				return p.parseCompositeKeyProperty(exp.KindUniqueKeyProperty)
			},
			"KEY": func(p *Parser, _ bool) exp.Expression {
				return p.parseCompositeKeyProperty(exp.KindUniqueKeyProperty)
			},
			"BUILD":        (*Parser).parseDorisBuildProperty,
			"REFRESH":      (*Parser).parseDorisRefreshProperty,
			"PARTITION BY": (*Parser).parseDorisPartitionProperty,
		},
	})
}

// parseDorisBuildProperty ports _parse_build_property (parsers/doris.py:123-124).
func (p *Parser) parseDorisBuildProperty(isDefault bool) exp.Expression {
	return p.expression(exp.BuildProperty(exp.Args{"this": p.parseVar(true, nil, true)}), nil, nil)
}

// parseDorisRefreshProperty ports _parse_refresh_property (parsers/doris.py:126-143).
func (p *Parser) parseDorisRefreshProperty(isDefault bool) exp.Expression {
	method := p.parseVar(true, nil, true)
	p.match(tokens.ON)

	var kind any
	if p.matchTexts(map[string]bool{"MANUAL": true, "COMMIT": true, "SCHEDULE": true}) {
		kind = stringsUpper(p.prev.Text)
	}
	var every, unit exp.Expression
	if p.matchTextSeq("EVERY") {
		every = p.parseNumber()
		unit = p.parseVar(true, nil, false)
	}
	var starts exp.Expression
	if p.matchTextSeq("STARTS") {
		starts = p.parseString()
	}
	return p.expression(exp.RefreshTriggerProperty(exp.Args{
		"method": method,
		"kind":   kind,
		"every":  every,
		"unit":   unit,
		"starts": starts,
	}), nil, nil)
}

// parseDorisPartitionProperty ports _parse_partition_property (parsers/doris.py:73-97)
// on top of MySQL's RANGE/LIST grammar.
func (p *Parser) parseDorisPartitionProperty(isDefault bool) exp.Expression {
	var partitionKind exp.Kind
	var valueParser func() exp.Expression
	switch {
	case p.matchTextSeq("RANGE"):
		partitionKind = exp.KindPartitionByRangeProperty
		valueParser = p.parseDorisPartitionRangeValue
	case p.matchTextSeq("LIST"):
		partitionKind = exp.KindPartitionByListProperty
		valueParser = p.parseMySQLPartitionListValue
	default:
		// parsers/doris.py:78-79 falls back to the base _parse_partitioned_by.
		return p.parsePartitionedBy()
	}

	partitionExpressions := p.parseWrappedCsv(p.parseAssignment)

	if p.peekTextSeq("(", "PARTITION") {
		createExpressions := p.parseWrappedCsv(valueParser)
		return p.expression(exp.New(partitionKind, exp.Args{
			"partition_expressions": partitionExpressions,
			"create_expressions":    createExpressions,
		}), nil, nil)
	}

	// parsers/doris.py:86-97: dynamic partition range with FROM/TO/INTERVAL.
	p.match(tokens.L_PAREN)
	var createExpressions []exp.Expression
	if p.peekTextSeq("FROM") {
		createExpressions = p.parseCsv(p.parseDorisPartitioningGranularityDynamic)
	}
	p.match(tokens.R_PAREN)
	var createArg any
	if createExpressions != nil {
		createArg = createExpressions
	}
	return p.expression(exp.PartitionByRangeProperty(exp.Args{
		"partition_expressions": partitionExpressions,
		"create_expressions":    createArg,
	}), nil, nil)
}

// parseDorisPartitioningGranularityDynamic ports _parse_partitioning_granularity_dynamic
// (parsers/doris.py:99-111).
func (p *Parser) parseDorisPartitioningGranularityDynamic() exp.Expression {
	p.matchTextSeq("FROM")
	start := p.parseWrapped(p.parseString, false)
	p.matchTextSeq("TO")
	end := p.parseWrapped(p.parseString, false)
	p.matchTextSeq("INTERVAL")
	number := p.parseNumber()
	unit := p.parseVar(true, nil, false)
	every := p.expression(exp.Interval(exp.Args{"this": number, "unit": unit}), nil, nil)
	return p.expression(exp.PartitionByRangePropertyDynamic(exp.Args{
		"start": start,
		"end":   end,
		"every": every,
	}), nil, nil)
}

// parseDorisPartitionRangeValue ports _parse_partition_range_value
// (parsers/doris.py:113-121) including Doris's `VALUES [(...), (...))` bracket syntax.
func (p *Parser) parseDorisPartitionRangeValue() exp.Expression {
	p.matchTextSeq("PARTITION")
	name := p.parseIdVar(false, nil)

	if p.matchTextSeq("VALUES", "LESS", "THAN") {
		values := p.parseWrappedCsv(p.parseExpression)
		if len(values) == 1 && values[0] != nil && values[0].Kind() == exp.KindColumn &&
			stringsUpper(values[0].Name()) == "MAXVALUE" {
			values = []exp.Expression{exp.Var(exp.Args{"this": "MAXVALUE"})}
		}
		return p.expression(exp.Partition(exp.Args{"expressions": []exp.Expression{
			p.expression(exp.PartitionRange(exp.Args{"this": name, "expressions": values}), nil, nil),
		}}), nil, nil)
	}

	p.matchTextSeq("VALUES")
	p.match(tokens.L_BRACKET)
	// Doris's VALUES [(...), (...)) groups are represented as Tuples so the generator can
	// tell a group apart from a scalar.
	values := p.parseCsv(func() exp.Expression {
		return p.expression(exp.Tuple(exp.Args{
			"expressions": p.parseWrappedCsv(p.parseExpression),
		}), nil, nil)
	})
	p.match(tokens.R_BRACKET)
	p.match(tokens.R_PAREN)

	return p.expression(exp.Partition(exp.Args{"expressions": []exp.Expression{
		p.expression(exp.PartitionRange(exp.Args{"this": name, "expressions": values}), nil, nil),
	}}), nil, nil)
}
