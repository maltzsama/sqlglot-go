package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// parsers/duckdb.py supplies the DuckDBParser overlays. FUNCTIONS entries live in
// dialects/duckdb.go; the grammar callbacks and helpers live here.
func init() {
	registerDialectParserOverrides("duckdb", dialectParserOverrideSet{
		StatementParsers: map[tokens.TokenType]parserOverrideFunc{
			// parsers/duckdb.py:263-270 STATEMENT_PARSERS.
			tokens.ATTACH:    func(p *Parser) exp.Expression { return p.parseDuckDBAttachDetach(true) },
			tokens.DETACH:    func(p *Parser) exp.Expression { return p.parseDuckDBAttachDetach(false) },
			tokens.FORCE:     (*Parser).parseDuckDBForce,
			tokens.INSTALL:   func(p *Parser) exp.Expression { return p.parseDuckDBInstall(false) },
			tokens.SHOW:      (*Parser).parseDuckDBShow,
			tokens.SUMMARIZE: (*Parser).parseDuckDBSummarize,
			// parser.py:1174,1181 STATEMENT_PARSERS PIVOT/UNPIVOT.
			tokens.PIVOT:   func(p *Parser) exp.Expression { return p.parseSimplifiedPivot(false) },
			tokens.UNPIVOT: func(p *Parser) exp.Expression { return p.parseSimplifiedPivot(true) },
		},
		// parsers/duckdb.py:188-191 NO_PAREN_FUNCTIONS.
		NoParenFunctions: map[tokens.TokenType]func(exp.Args) exp.Expression{
			tokens.LOCALTIME:       func(exp.Args) exp.Expression { return exp.Localtime(exp.Args{}) },
			tokens.LOCALTIMESTAMP:  func(exp.Args) exp.Expression { return exp.Localtimestamp(exp.Args{}) },
			tokens.CURRENT_CATALOG: func(exp.Args) exp.Expression { return exp.CurrentCatalog(exp.Args{}) },
			tokens.SESSION_USER:    func(exp.Args) exp.Expression { return exp.SessionUser(exp.Args{}) },
		},
		// parsers/duckdb.py:241-250 NO_PAREN_FUNCTION_PARSERS.
		NoParenFunctionParsers: map[string]parserOverrideFunc{
			"MAP": (*Parser).parseDuckDBMap,
			"@": func(p *Parser) exp.Expression {
				return exp.Abs(exp.Args{"this": p.parseBitwise()})
			},
		},
	})
}

// parseDuckDBAttachDetach ports _parse_attach_detach (parsers/duckdb.py:343-368).
func (p *Parser) parseDuckDBAttachDetach(isAttach bool) exp.Expression {
	p.match(tokens.DATABASE)
	exists := p.parseExists(isAttach)
	this := p.parseAlias(p.parsePrimaryOrVar(), true)

	var expressions []exp.Expression
	if p.match(tokens.L_PAREN, false) {
		expressions = p.parseWrappedCsv(func() exp.Expression {
			return p.expression(exp.AttachOption(exp.Args{
				"this":       p.parseVar(true, nil, false),
				"expression": p.parseField(true, nil, false),
			}), nil, nil)
		})
	}
	if isAttach {
		return p.expression(exp.Attach(exp.Args{"this": this, "exists": exists, "expressions": expressions}), nil, nil)
	}
	return p.expression(exp.Detach(exp.Args{"this": this, "exists": exists}), nil, nil)
}

// parseDuckDBShow ports _parse_show_duckdb (parsers/duckdb.py:370-372).
func (p *Parser) parseDuckDBShow() exp.Expression {
	var this string
	switch {
	case p.matchTextSeq("ALL", "TABLES"):
		this = "ALL TABLES"
	case p.matchTextSeq("TABLES"):
		this = "TABLES"
	default:
		return p.parseAsCommand(p.prev)
	}
	var from exp.Expression
	if p.match(tokens.FROM) {
		from = p.parseTable(true, false, nil, false, false, false, false)
	}
	return p.expression(exp.Show(exp.Args{"this": this, "from_": from}), nil, nil)
}

// parseDuckDBForce ports _parse_force (parsers/duckdb.py:374-380).
func (p *Parser) parseDuckDBForce() exp.Expression {
	if !p.match(tokens.INSTALL) {
		return p.parseAsCommand(p.prev)
	}
	return p.parseDuckDBInstall(true)
}

// parseDuckDBInstall ports _parse_install (parsers/duckdb.py:382-388).
func (p *Parser) parseDuckDBInstall(force bool) exp.Expression {
	this := p.parseIdVar(false, nil)
	var from exp.Expression
	if p.match(tokens.FROM) {
		from = p.parseVarOrString(false)
	}
	return p.expression(exp.Install(exp.Args{"this": this, "from_": from, "force": force}), nil, nil)
}

// parseDuckDBMap ports _parse_map (parsers/duckdb.py:301-307).
func (p *Parser) parseDuckDBMap() exp.Expression {
	if p.match(tokens.L_BRACE, false) {
		return p.expression(exp.ToMap(exp.Args{"this": p.parseBracket(nil)}), nil, nil)
	}
	args := p.parseWrappedCsv(func() exp.Expression { return p.parseAssignment() })
	return p.expression(exp.Map(exp.Args{
		"keys":   seqGetExpressions(args, 0),
		"values": seqGetExpressions(args, 1),
	}), nil, nil)
}

// seqGetExpressions returns args[i] or nil.
func seqGetExpressions(args []exp.Expression, i int) exp.Expression {
	if i < 0 || i >= len(args) {
		return nil
	}
	return args[i]
}

// parseDuckDBPositionalColumn ports the `#N` positional-column primary
// (parsers/duckdb.py:390-394).
func (p *Parser) parseDuckDBPositionalColumn() (exp.Expression, bool) {
	if p.curr.TokenType != tokens.HASH || p.next.TokenType != tokens.NUMBER {
		return nil, false
	}
	p.advance()
	number := p.curr.Text
	p.advance()
	return p.expression(exp.PositionalColumn(exp.Args{"this": exp.LiteralNumber(number)}), nil, nil), true
}

// replaceLambdaExpressions ports _replace_lambda (parser.py:9729-9760) for the untyped
// lambda-argument form: a body column naming an argument becomes that bare identifier.
func (p *Parser) replaceLambdaExpressions(node exp.Expression, expressions []exp.Expression) exp.Expression {
	if node == nil {
		return node
	}
	names := map[string]bool{}
	for _, arg := range expressions {
		if arg != nil {
			names[arg.Name()] = true
		}
	}
	for _, column := range node.FindAll(exp.KindColumn) {
		parts := column.Parts()
		if len(parts) == 0 || !names[parts[0].Name()] {
			continue
		}
		if column.Arg("table") != nil {
			column.Replace(exp.ColumnsToDot(column))
			continue
		}
		column.Replace(column.This())
	}
	return node
}

// parseDuckDBSummarize ports the SUMMARIZE branch of _parse_select
// (parser.py:4138-4141).
func (p *Parser) parseDuckDBSummarize() exp.Expression {
	table := p.match(tokens.TABLE)
	this := p.parseSelect(true, false, true, false)
	if this == nil {
		this = p.parseString()
	}
	if this == nil {
		this = p.parseTable(false, false, nil, false, false, false, false)
	}
	return p.expression(exp.Summarize(exp.Args{"this": this, "table": table}), nil, nil)
}

// parseSimplifiedPivot ports _parse_simplified_pivot (parser.py:5323-5353).
func (p *Parser) parseSimplifiedPivot(isUnpivot any) exp.Expression {
	parseOn := func() exp.Expression {
		this := p.parseBitwise()
		if p.match(tokens.IN) {
			return p.parseIn(this, false)
		}
		if p.match(tokens.ALIAS, false) {
			return p.parseAlias(this, false)
		}
		return this
	}

	this := p.parseTable(false, false, nil, false, false, false, false)
	var expressions []exp.Expression
	if p.match(tokens.ON) {
		expressions = p.parseCsv(parseOn)
	}
	into := p.parseUnpivotColumns()
	var using []exp.Expression
	if p.match(tokens.USING) {
		using = p.parseCsv(func() exp.Expression {
			return p.parseAlias(p.parseColumn(), false)
		})
	}
	group := p.parseGroup(false)
	return p.expression(exp.Pivot(exp.Args{
		"this":        this,
		"expressions": expressions,
		"using":       using,
		"group":       group,
		"unpivot":     isUnpivot,
		"into":        into,
	}), nil, nil)
}

// parseUnpivotColumns ports _parse_unpivot_columns (parser.py:5311-5320).
func (p *Parser) parseUnpivotColumns() exp.Expression {
	if !p.match(tokens.INTO) {
		return nil
	}
	var name exp.Expression
	if p.matchTextSeq("NAME") {
		name = p.parseColumn()
	}
	var values []exp.Expression
	if p.matchTextSeq("VALUE") {
		values = p.parseCsv(func() exp.Expression { return p.parseColumn() })
	}
	return p.expression(exp.UnpivotColumns(exp.Args{"this": name, "expressions": values}), nil, nil)
}
