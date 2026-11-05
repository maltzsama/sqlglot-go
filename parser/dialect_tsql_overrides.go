package parser

import (
	exp "github.com/ridi-oss/sqlglot-go/expressions"
	"github.com/ridi-oss/sqlglot-go/tokens"
)

// TSQL parser overrides, ported from .reference/sqlglot-v30.17.0/sqlglot/parsers/tsql.py.
func init() {
	registerDialectParserOverrides("tsql", dialectParserOverrideSet{
		StatementParsers: map[tokens.TokenType]parserOverrideFunc{
			tokens.EXECUTE:  (*Parser).parseTSQLExecute,
			tokens.DECLARE:  (*Parser).parseDeclareStatement,
			tokens.BEGIN:    (*Parser).parseTSQLTransaction,
			tokens.COMMIT:   (*Parser).parseTSQLCommitOrRollback,
			tokens.ROLLBACK: (*Parser).parseTSQLCommitOrRollback,
		},
		FunctionParsers: map[string]parserOverrideFunc{
			"DATEPART":      (*Parser).parseTSQLDatepart,
			"JSON_ARRAYAGG": (*Parser).parseTSQLJSONArrayAgg,
			"OPENJSON":      (*Parser).parseOpenJSON,
			"CONVERT": func(p *Parser) exp.Expression {
				return p.parseTSQLConvert(p.strictCast, nil)
			},
			"TRY_CONVERT": func(p *Parser) exp.Expression {
				return p.parseTSQLConvert(false, true)
			},
		},
		QueryModifiers: map[tokens.TokenType]func(*Parser) (string, any){
			tokens.OPTION: func(p *Parser) (string, any) { return "options", p.parseTSQLOptions() },
			tokens.FOR:    func(p *Parser) (string, any) { return "for_", p.parseTSQLFor() },
		},
		NoParenFunctions: map[tokens.TokenType]func(exp.Args) exp.Expression{
			tokens.SESSION_USER: exp.SessionUser,
		},
		NoParenFunctionParsers: map[string]parserOverrideFunc{
			"NEXT": (*Parser).parseTSQLNextValueFor,
		},
		PropertyParsers: map[string]propertyParserFunc{
			"SYSTEM_VERSIONING": func(p *Parser, _ bool) exp.Expression {
				return p.parseSystemVersioningProperty(false)
			},
			"DATA_DELETION": func(p *Parser, _ bool) exp.Expression {
				return p.parseDataDeletionProperty()
			},
		},
	})
}

// parseTSQLExecute ports _parse_execute (parsers/tsql.py:468-489).
func (p *Parser) parseTSQLExecute() exp.Expression {
	var returnStatus exp.Expression
	index := p.index
	if p.match(tokens.PARAMETER) {
		param := p.parseParameter()
		if p.match(tokens.EQ) {
			returnStatus = param
		} else {
			p.retreat(index)
		}
	}

	execute := p.expression(exp.Execute(exp.Args{
		"this":          p.parseTable(true, false, nil, false, false, false, false),
		"expressions":   p.parseCsv(p.parseExpression),
		"return_status": returnStatus,
	}), nil, nil)

	if execute != nil && stringsUpper(execute.Name()) == "SP_EXECUTESQL" {
		execute = p.expression(exp.ExecuteSql(exp.Args{
			"this":          execute.Arg("this"),
			"expressions":   execute.Arg("expressions"),
			"return_status": execute.Arg("return_status"),
		}), nil, nil)
	}
	return execute
}

// parseTSQLTransaction ports _parse_transaction (parsers/tsql.py:630-644).
func (p *Parser) parseTSQLTransaction() exp.Expression {
	if p.matchTexts(map[string]bool{"TRAN": true, "TRANSACTION": true}) {
		transaction := p.expression(exp.Transaction(exp.Args{"this": p.parseIdVar(true, nil)}), nil, nil)
		if p.matchTextSeq("WITH", "MARK") {
			transaction.Set("mark", p.parseString())
		}
		return transaction
	}
	return p.parseAsCommand(p.prev)
}

// parseTSQLCommitOrRollback ports _parse_commit_or_rollback (parsers/tsql.py:597-628).
func (p *Parser) parseTSQLCommitOrRollback() exp.Expression {
	rollback := p.prev.TokenType == tokens.ROLLBACK
	p.matchTexts(map[string]bool{"TRAN": true, "TRANSACTION": true})
	this := p.parseIdVar(true, nil)
	if rollback {
		return p.expression(exp.Rollback(exp.Args{"this": this}), nil, nil)
	}
	var durability any
	if p.matchPair(tokens.WITH, tokens.L_PAREN, true) {
		p.matchTextSeq("DELAYED_DURABILITY")
		p.match(tokens.EQ)
		if p.matchTextSeq("OFF") {
			durability = false
		} else {
			p.match(tokens.ON)
			durability = true
		}
		p.matchRParen(nil)
	}
	return p.expression(exp.Commit(exp.Args{"this": this, "durability": durability}), nil, nil)
}

// parseTSQLNextValueFor ports _parse_next_value_for (parser.py:8074-8084).
func (p *Parser) parseTSQLNextValueFor() exp.Expression {
	if !p.matchTextSeq("VALUE", "FOR") {
		p.retreat(p.index - 1)
		return nil
	}
	var order exp.Expression
	if p.match(tokens.OVER) {
		order = p.parseWrapped(func() exp.Expression { return p.parseOrder(nil, false) }, false)
	}
	return p.expression(exp.NextValueFor(exp.Args{"this": p.parseColumn(), "order": order}), nil, nil)
}

// mapDatePart ports map_date_part (dialects/dialect.py:2149-2158).
func mapDatePart(part exp.Expression, mapping map[string]string) exp.Expression {
	if part == nil {
		return nil
	}
	if part.Kind() == exp.KindColumn && len(part.Parts()) != 1 {
		return part
	}
	mapped := mapping[stringsUpper(part.Name())]
	if mapped == "" {
		return part
	}
	if part.IsString() {
		return exp.LiteralString(mapped)
	}
	return exp.Var(exp.Args{"this": mapped})
}

// parseTSQLDatepart ports _parse_datepart (parsers/tsql.py:491-496).
func (p *Parser) parseTSQLDatepart() exp.Expression {
	this := p.parseVar(false, map[tokens.TokenType]bool{tokens.IDENTIFIER: true}, false)
	if this == nil {
		this = p.parseVar(true, nil, false)
	}
	var expression exp.Expression
	if p.match(tokens.COMMA) {
		expression = p.parseBitwise()
	}
	name := mapDatePart(this, p.dialect.DatePartMapping)
	return p.expression(exp.Extract(exp.Args{"this": name, "expression": expression}), nil, nil)
}

// parseOpenJSON ports _parse_open_json (parser.py:8485-8504).
func (p *Parser) parseOpenJSON() exp.Expression {
	this := p.parseBitwise()
	var path exp.Expression
	if p.match(tokens.COMMA) {
		path = p.parseString()
	}
	parseColumnDef := func() exp.Expression {
		column := p.parseField(true, nil, false)
		kind := p.parseTypes(false, false, true, false)
		path := p.parseString()
		asJSON := p.matchPair(tokens.ALIAS, tokens.JSON, true)
		return p.expression(exp.OpenJSONColumnDef(exp.Args{
			"this": column, "kind": kind, "path": path, "as_json": asJSON,
		}), nil, nil)
	}
	var expressions []exp.Expression
	if p.matchPair(tokens.R_PAREN, tokens.WITH, true) {
		p.matchLParen(nil)
		expressions = p.parseCsv(parseColumnDef)
	}
	return p.expression(exp.OpenJSON(exp.Args{"this": this, "path": path, "expressions": expressions}), nil, nil)
}

// parseTSQLJSONArrayAgg ports the TSQL FUNCTION_PARSERS["JSON_ARRAYAGG"] entry
// (parsers/tsql.py:440-446).
func (p *Parser) parseTSQLJSONArrayAgg() exp.Expression {
	return p.expression(exp.JSONArrayAgg(exp.Args{
		"this":          p.parseBitwise(),
		"order":         p.parseOrder(nil, false),
		"null_handling": p.parseOnHandling("NULL", "NULL", "ABSENT"),
	}), nil, nil)
}

// parseTSQLOptions ports _parse_options (parsers/tsql.py:518-532).
func (p *Parser) parseTSQLOptions() []exp.Expression {
	if !p.match(tokens.OPTION) {
		return nil
	}
	parseOption := func() exp.Expression {
		option := p.parseVarFromOptions(tsqlQueryOptions, false)
		if option == nil {
			return nil
		}
		p.match(tokens.EQ)
		return p.expression(exp.QueryOption(exp.Args{
			"this":       option,
			"expression": p.parsePrimaryOrVar(),
		}), nil, nil)
	}
	return p.parseWrappedCsv(parseOption)
}

// parseTSQLKeyValueOption ports _parse_key_value_option (parsers/tsql.py:534-541).
func (p *Parser) parseTSQLKeyValueOption() exp.Expression {
	this := p.parsePrimaryOrVar()
	var expression exp.Expression
	if p.curr.TokenType == tokens.L_PAREN {
		expression = p.parseWrapped(p.parseString, false)
	}
	return exp.XMLKeyValueOption(exp.Args{"this": this, "expression": expression})
}

// parseTSQLForClauseOption ports _parse_for_clause_option (parsers/tsql.py:543-549).
func (p *Parser) parseTSQLForClauseOption(options optionsType) exp.Expression {
	option := p.parseVarFromOptions(options, false)
	if option == nil {
		option = p.parseTSQLKeyValueOption()
	}
	return exp.QueryOption(exp.Args{"this": option})
}

// parseTSQLFor ports _parse_for (parsers/tsql.py:551-576).
func (p *Parser) parseTSQLFor() exp.Expression {
	if p.matchPair(tokens.FOR, tokens.XML, true) {
		return exp.ForClause(exp.Args{
			"kind": "XML",
			"expressions": p.parseCsv(func() exp.Expression {
				return p.parseTSQLForClauseOption(tsqlForXMLOptions)
			}),
		})
	}
	if p.matchPair(tokens.FOR, tokens.JSON, true) {
		return exp.ForClause(exp.Args{
			"kind": "JSON",
			"expressions": p.parseCsv(func() exp.Expression {
				return p.parseTSQLForClauseOption(tsqlForJSONOptions)
			}),
		})
	}
	if p.matchTextSeq("FOR", "BROWSE") {
		return exp.ForClause(exp.Args{"kind": "BROWSE"})
	}
	return nil
}

// parseTSQLConvert ports _parse_convert (parsers/tsql.py:652-658): CONVERT(type, expr[, style]).
func (p *Parser) parseTSQLConvert(strict bool, safe any) exp.Expression {
	this := p.parseTypes(false, false, true, false)
	p.match(tokens.COMMA)
	args := []exp.Expression{this}
	args = append(args, p.parseCsv(p.parseAssignment)...)
	convert := exp.ConvertExpr(exp.Args{
		"this":       args[0],
		"expression": seqGetExpr(args, 1),
		"style":      seqGetExpr(args, 2),
		"safe":       safe,
	})
	_ = strict
	return convert
}

func seqGetExpr(args []exp.Expression, i int) exp.Expression {
	if i < 0 || i >= len(args) {
		return nil
	}
	return args[i]
}

// tsqlQueryOptions ports OPTIONS (parsers/tsql.py:56-90).
var tsqlQueryOptions = optionsType{
	"DISABLE_OPTIMIZED_PLAN_FORCING":        {},
	"FAST":                                  {},
	"IGNORE_NONCLUSTERED_COLUMNSTORE_INDEX": {},
	"LABEL":                                 {},
	"MAXDOP":                                {},
	"MAXRECURSION":                          {},
	"MAX_GRANT_PERCENT":                     {},
	"MIN_GRANT_PERCENT":                     {},
	"NO_PERFORMANCE_SPOOL":                  {},
	"QUERYTRACEON":                          {},
	"RECOMPILE":                             {},
	"CONCAT":                                {{"UNION"}},
	"DISABLE":                               {{"EXTERNALPUSHDOWN"}, {"SCALEOUTEXECUTION"}},
	"EXPAND":                                {{"VIEWS"}},
	"FORCE":                                 {{"EXTERNALPUSHDOWN"}, {"ORDER"}, {"SCALEOUTEXECUTION"}},
	"HASH":                                  {{"GROUP"}, {"JOIN"}, {"UNION"}},
	"KEEP":                                  {{"PLAN"}},
	"KEEPFIXED":                             {{"PLAN"}},
	"LOOP":                                  {{"JOIN"}},
	"MERGE":                                 {{"JOIN"}, {"UNION"}},
	"OPTIMIZE":                              {{"FOR", "UNKNOWN"}},
	"ORDER":                                 {{"GROUP"}},
	"PARAMETERIZATION":                      {{"FORCED"}, {"SIMPLE"}},
	"ROBUST":                                {{"PLAN"}},
	"USE":                                   {{"PLAN"}},
}

// tsqlForXMLOptions ports FOR_XML_OPTIONS (parsers/tsql.py:93-107).
var tsqlForXMLOptions = optionsType{
	"AUTO":     {},
	"EXPLICIT": {},
	"TYPE":     {},
	"ELEMENTS": {{"XSINIL"}, {"ABSENT"}},
	"BINARY":   {{"BASE64"}},
}

// tsqlForJSONOptions ports FOR_JSON_OPTIONS (parsers/tsql.py:113-121).
var tsqlForJSONOptions = optionsType{
	"AUTO":                  {},
	"PATH":                  {},
	"INCLUDE_NULL_VALUES":   {},
	"WITHOUT_ARRAY_WRAPPER": {},
}
