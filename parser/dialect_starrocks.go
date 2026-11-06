package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// StarRocks parser overrides, ported from
// .reference/sqlglot-v30.17.0/sqlglot/parsers/starrocks.py.
func init() {
	registerDialectParserOverrides("starrocks", dialectParserOverrideSet{
		// parsers/starrocks.py:13-14: StarRocks supports LEFT SEMI/ANTI JOIN natively, so
		// SEMI/ANTI are no longer alias tokens.
		RemovedTableAliasTokens: map[tokens.TokenType]bool{
			tokens.ANTI: true,
			tokens.SEMI: true,
		},
		StatementParsers: map[tokens.TokenType]parserOverrideFunc{
			tokens.REFRESH: (*Parser).parseStarRocksRefresh,
		},
		PropertyParsers: map[string]propertyParserFunc{
			"PROPERTIES": func(p *Parser, _ bool) exp.Expression {
				return p.parsePropertiesList()
			},
			"UNIQUE": func(p *Parser, _ bool) exp.Expression {
				return p.parseCompositeKeyProperty(exp.KindUniqueKeyProperty)
			},
			"ROLLUP":       (*Parser).parseRollupProperty,
			"REFRESH":      (*Parser).parseRefreshTriggerProperty,
			"PARTITION BY": (*Parser).parseStarRocksPartitionProperty,
		},
	})
}

// parsePropertiesList ports _parse_wrapped_properties used as a property parser
// (parsers/starrocks.py:38). The shared registry is singleton-valued, so the list is wrapped
// in a Properties node and flattened by parseProperties.
func (p *Parser) parsePropertiesList() exp.Expression {
	return p.expression(exp.Properties(exp.Args{"expressions": p.parseWrappedProperties()}), nil, nil)
}

// parseCompositeKeyProperty ports _parse_composite_key_property (parser.py:3092-3094).
func (p *Parser) parseCompositeKeyProperty(kind exp.Kind) exp.Expression {
	p.matchTextSeq("KEY")
	expressions := p.parseWrappedIdVars()
	return p.expression(exp.New(kind, exp.Args{"expressions": expressions}), nil, nil)
}

// parseRollupProperty ports _parse_rollup_property (parsers/starrocks.py:44-62).
func (p *Parser) parseRollupProperty(isDefault bool) exp.Expression {
	parseRollupIndex := func() exp.Expression {
		this := p.parseIdVar(false, nil)
		expressions := p.parseWrappedIdVars()
		var fromIndex exp.Expression
		if p.matchTextSeq("FROM") {
			fromIndex = p.parseIdVar(false, nil)
		}
		var properties exp.Expression
		if p.matchTextSeq("PROPERTIES") {
			properties = p.expression(exp.Properties(exp.Args{
				"expressions": p.parseWrappedProperties(),
			}), nil, nil)
		}
		return p.expression(exp.RollupIndex(exp.Args{
			"this":        this,
			"expressions": expressions,
			"from_index":  fromIndex,
			"properties":  properties,
		}), nil, nil)
	}
	return p.expression(exp.RollupProperty(exp.Args{
		"expressions": p.parseWrappedCsv(parseRollupIndex),
	}), nil, nil)
}

// parseRefreshTriggerProperty ports _parse_refresh_property
// (parsers/starrocks.py:143-166).
func (p *Parser) parseRefreshTriggerProperty(isDefault bool) exp.Expression {
	var method, kind any
	if p.matchTexts(map[string]bool{"DEFERRED": true, "IMMEDIATE": true}) {
		method = stringsUpper(p.prev.Text)
	}
	if p.matchTexts(map[string]bool{"ASYNC": true, "MANUAL": true}) {
		kind = stringsUpper(p.prev.Text)
	}
	var start exp.Expression
	if p.matchTextSeq("START") {
		start = p.parseWrapped(p.parseString, false)
	}

	var every, unit exp.Expression
	if p.matchTextSeq("EVERY") {
		p.match(tokens.L_PAREN)
		p.matchTextSeq("INTERVAL")
		every = p.parseNumber()
		unit = p.parseVar(true, nil, false)
		p.match(tokens.R_PAREN)
	}
	return p.expression(exp.RefreshTriggerProperty(exp.Args{
		"method": method,
		"kind":   kind,
		"starts": start,
		"every":  every,
		"unit":   unit,
	}), nil, nil)
}

// parseStarRocksPartitionProperty ports _parse_partition_property
// (parsers/starrocks.py:106-130) on top of MySQL's RANGE/LIST grammar.
func (p *Parser) parseStarRocksPartitionProperty(isDefault bool) exp.Expression {
	var partitionKind exp.Kind
	var valueParser func() exp.Expression
	switch {
	case p.matchTextSeq("RANGE"):
		partitionKind = exp.KindPartitionByRangeProperty
		valueParser = p.parseMySQLPartitionRangeValue
	case p.matchTextSeq("LIST"):
		partitionKind = exp.KindPartitionByListProperty
		valueParser = p.parseMySQLPartitionListValue
	default:
		return p.parseStarRocksPartitionedBy()
	}

	partitionExpressions := p.parseWrappedCsv(p.parseAssignment)

	// parsers/mysql.py:605-612: a following `(PARTITION ...)` list makes the full property.
	if p.peekTextSeq("(", "PARTITION") {
		createExpressions := p.parseWrappedCsv(valueParser)
		return p.expression(exp.New(partitionKind, exp.Args{
			"partition_expressions": partitionExpressions,
			"create_expressions":    createExpressions,
		}), nil, nil)
	}

	// parsers/starrocks.py:114-130: wrap the partition expressions, optionally with a
	// dynamic-partition START/END/EVERY list.
	p.match(tokens.L_PAREN)
	var createExpressions []exp.Expression
	if p.peekTextSeq("START") {
		createExpressions = p.parseCsv(p.parseStarRocksPartitioningGranularityDynamic)
	}
	p.match(tokens.R_PAREN)
	var partExprArg any = partitionExpressions
	var createArg any
	if createExpressions != nil {
		createArg = createExpressions
	}
	return p.expression(exp.PartitionByRangeProperty(exp.Args{
		"partition_expressions": partExprArg,
		"create_expressions":    createArg,
	}), nil, nil)
}

// parseStarRocksPartitioningGranularityDynamic ports
// _parse_partitioning_granularity_dynamic (parsers/starrocks.py:132-141).
func (p *Parser) parseStarRocksPartitioningGranularityDynamic() exp.Expression {
	p.matchTextSeq("START")
	start := p.parseWrapped(p.parseString, false)
	p.matchTextSeq("END")
	end := p.parseWrapped(p.parseString, false)
	p.matchTextSeq("EVERY")
	every := p.parseWrapped(func() exp.Expression {
		if interval := p.parseInterval(true); interval != nil {
			return interval
		}
		return p.parseNumber()
	}, false)
	return p.expression(exp.PartitionByRangePropertyDynamic(exp.Args{
		"start": start,
		"end":   end,
		"every": every,
	}), nil, nil)
}

// parseStarRocksPartitionedBy ports _parse_partitioned_by
// (parsers/starrocks.py:97-104): the partition key is a Schema of assignments.
func (p *Parser) parseStarRocksPartitionedBy() exp.Expression {
	return p.expression(exp.PartitionedByProperty(exp.Args{
		"this": exp.Schema(exp.Args{
			"expressions": p.parseWrappedCsv(p.parseAssignment, true),
		}),
	}), nil, nil)
}

// moveStarRocksPrimaryKey ports the create postprocess in _parse_create
// (parsers/starrocks.py:64-76): a PrimaryKey property moves into the schema.
func (p *Parser) moveStarRocksPrimaryKey(create exp.Expression) exp.Expression {
	if create == nil || create.Kind() != exp.KindCreate {
		return create
	}
	schema := asExpr(create.Arg("this"))
	if schema == nil || schema.Kind() != exp.KindSchema {
		return create
	}
	props := asExpr(create.Arg("properties"))
	if props == nil {
		return create
	}
	if primaryKey := props.Find(exp.KindPrimaryKey); primaryKey != nil {
		schema.Append("expressions", primaryKey.Pop())
	}
	return create
}

// parseStarRocksRefresh ports _parse_refresh (parser.py:8975-8989).
func (p *Parser) parseStarRocksRefresh() exp.Expression {
	var kind string
	switch {
	case p.matchTextSeq("EXTERNAL", "TABLE"):
		kind = "EXTERNAL TABLE"
	case p.match(tokens.TABLE):
		kind = "TABLE"
	case p.matchTextSeq("MATERIALIZED", "VIEW"):
		kind = "MATERIALIZED VIEW"
	}
	var this exp.Expression
	if s := p.parseString(); s != nil {
		this = s
	} else {
		this = p.parseTable(false, false, nil, false, false, false, false)
	}
	if kind == "" && (this == nil || this.Kind() != exp.KindLiteral) {
		return p.parseAsCommand(p.prev)
	}
	return p.expression(exp.Refresh(exp.Args{"this": this, "kind": kind}), nil, nil)
}
