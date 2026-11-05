package parser

import (
	exp "github.com/ridi-oss/sqlglot-go/expressions"
	"github.com/ridi-oss/sqlglot-go/tokens"
)

// Oracle parser overrides, ported from
// .reference/sqlglot-v30.17.0/sqlglot/parsers/oracle.py.
func init() {
	registerDialectParserOverrides("oracle", dialectParserOverrideSet{
		// parsers/oracle.py:63-65 removes CONVERT from FUNCTION_PARSERS so the FUNCTIONS
		// overlay's ConvertToCharset builder handles it.
		DisabledFunctionParsers: map[string]bool{"CONVERT": true},
		// parsers/oracle.py:63-75 FUNCTION_PARSERS.
		FunctionParsers: map[string]parserOverrideFunc{
			"JSON_ARRAY":    (*Parser).parseOracleJSONArray,
			"JSON_ARRAYAGG": (*Parser).parseOracleJSONArrayAgg,
			"JSON_EXISTS":   (*Parser).parseOracleJSONExists,
			"LISTAGG":       (*Parser).parseStringAgg,
			"TO_NUMBER":     (*Parser).parseOracleToNumber,
		},
		// parsers/oracle.py:57-62 NO_PAREN_FUNCTION_PARSERS.
		NoParenFunctionParsers: map[string]parserOverrideFunc{
			"NEXT":        (*Parser).parseOracleNextValueFor,
			"PRIOR":       (*Parser).parseOraclePrior,
			"SYSDATE":     (*Parser).parseOracleSysdate,
			"DBMS_RANDOM": (*Parser).parseOracleDBMSRandom,
		},
		// parsers/oracle.py:64-67 NO_PAREN_FUNCTIONS.
		NoParenFunctions: map[tokens.TokenType]func(exp.Args) exp.Expression{
			tokens.LOCALTIMESTAMP: exp.Localtimestamp,
			tokens.SYSTIMESTAMP:   exp.Systimestamp,
		},
		// parsers/oracle.py:77-89 PROPERTY_PARSERS.
		PropertyParsers: map[string]propertyParserFunc{
			"GLOBAL":  (*Parser).parseOracleGlobalTemporary,
			"PRIVATE": (*Parser).parseOraclePrivateTemporary,
			"FORCE":   (*Parser).parseOracleForceProperty,
		},
		// parsers/oracle.py:91-99 QUERY_MODIFIER_PARSERS.
		QueryModifiers: map[tokens.TokenType]func(*Parser) (string, any){
			tokens.ORDER_SIBLINGS_BY: func(p *Parser) (string, any) { return "order", p.parseOrder(nil, false) },
			tokens.WITH:              func(p *Parser) (string, any) { return "options", p.parseOracleQueryRestrictions() },
		},
	})
}

// oracleQueryRestrictions ports QUERY_RESTRICTIONS (parsers/oracle.py:113-118).
var oracleQueryRestrictions = optionsType{
	"WITH": {{"READ", "ONLY"}, {"CHECK", "OPTION"}},
}

// parseOracleQueryRestrictions ports _parse_query_restrictions
// (parsers/oracle.py:190-199).
func (p *Parser) parseOracleQueryRestrictions() exp.Expression {
	kind := p.parseVarFromOptions(oracleQueryRestrictions, false)
	if kind == nil {
		return nil
	}
	var expression exp.Expression
	if p.match(tokens.CONSTRAINT) {
		expression = p.parseField(false, nil, false)
	}
	return p.expression(exp.QueryOption(exp.Args{"this": kind, "expression": expression}), nil, nil)
}

// parseOracleGlobalTemporary ports PROPERTY_PARSERS["GLOBAL"] (parsers/oracle.py:79-82).
func (p *Parser) parseOracleGlobalTemporary(isDefault bool) exp.Expression {
	if p.matchTextSeq("TEMPORARY") {
		return p.expression(exp.TemporaryProperty(exp.Args{"this": "GLOBAL"}), nil, nil)
	}
	return nil
}

// parseOraclePrivateTemporary ports PROPERTY_PARSERS["PRIVATE"] (parsers/oracle.py:83-86).
func (p *Parser) parseOraclePrivateTemporary(isDefault bool) exp.Expression {
	if p.matchTextSeq("TEMPORARY") {
		return p.expression(exp.TemporaryProperty(exp.Args{"this": "PRIVATE"}), nil, nil)
	}
	return nil
}

// parseOracleForceProperty ports PROPERTY_PARSERS["FORCE"] (parsers/oracle.py:87).
func (p *Parser) parseOracleForceProperty(isDefault bool) exp.Expression {
	return p.expression(exp.New(exp.KindForceProperty, nil), nil, nil)
}

// parseOracleToNumber ports _parse_to_number (parsers/oracle.py:120-138).
func (p *Parser) parseOracleToNumber() exp.Expression {
	this := p.parseBitwise()

	var defaultExpr exp.Expression
	if p.match(tokens.DEFAULT) {
		defaultExpr = p.parseBitwise()
		p.matchTextSeq("ON", "CONVERSION", "ERROR")
	}
	var format, nlsparam exp.Expression
	if p.match(tokens.COMMA) {
		format = p.parseBitwise()
		if p.match(tokens.COMMA) {
			nlsparam = p.parseBitwise()
		}
	}
	return p.expression(exp.ToNumber(exp.Args{
		"this":     this,
		"format":   format,
		"nlsparam": nlsparam,
		"default":  defaultExpr,
	}), nil, nil)
}

// parseOracleDBMSRandom ports _parse_dbms_random (parsers/oracle.py:140-150).
func (p *Parser) parseOracleDBMSRandom() exp.Expression {
	if p.matchTextSeq(".", "VALUE") {
		var lower, upper exp.Expression
		if p.match(tokens.L_PAREN, false) {
			lowerUpper := p.parseWrappedCsv(p.parseBitwise)
			if len(lowerUpper) == 2 {
				lower, upper = lowerUpper[0], lowerUpper[1]
			}
		}
		return p.expression(exp.Rand(exp.Args{"lower": lower, "upper": upper}), nil, nil)
	}
	p.retreat(p.index - 1)
	return nil
}

// parseOraclePrior ports NO_PAREN_FUNCTION_PARSERS["PRIOR"] (parsers/oracle.py:60).
func (p *Parser) parseOraclePrior() exp.Expression {
	return p.expression(exp.Prior(exp.Args{"this": p.parseBitwise()}), nil, nil)
}

// parseOracleSysdate ports NO_PAREN_FUNCTION_PARSERS["SYSDATE"] (parsers/oracle.py:61).
func (p *Parser) parseOracleSysdate() exp.Expression {
	return p.expression(exp.CurrentTimestamp(exp.Args{"sysdate": true}), nil, nil)
}

// parseOracleNextValueFor ports NO_PAREN_FUNCTION_PARSERS["NEXT"] (parsers/oracle.py:59).
func (p *Parser) parseOracleNextValueFor() exp.Expression {
	return p.parseTSQLNextValueFor()
}

// parseOracleJSONArray ports _parse_oracle_json_array (parsers/oracle.py:152-156).
func (p *Parser) parseOracleJSONArray() exp.Expression {
	return p.parseOracleJSONArrayLike(exp.KindJSONArray, exp.Args{
		"expressions": p.parseCsv(func() exp.Expression { return p.parseFormatJson(p.parseBitwise()) }),
	})
}

// parseOracleJSONArrayAgg ports _parse_oracle_json_arrayagg (parsers/oracle.py:158-163).
func (p *Parser) parseOracleJSONArrayAgg() exp.Expression {
	return p.parseOracleJSONArrayLike(exp.KindJSONArrayAgg, exp.Args{
		"this":  p.parseFormatJson(p.parseBitwise()),
		"order": p.parseOrder(nil, false),
	})
}

// parseOracleJSONArrayLike ports _parse_json_array (parsers/oracle.py:165-174).
func (p *Parser) parseOracleJSONArrayLike(kind exp.Kind, args exp.Args) exp.Expression {
	args["null_handling"] = p.parseOnHandling("NULL", "NULL", "ABSENT")
	if p.matchTextSeq("RETURNING") {
		args["return_type"] = p.parseTypes(false, false, true, false)
	}
	args["strict"] = p.matchTextSeq("STRICT")
	return p.expression(exp.New(kind, args), nil, nil)
}

// parseOracleJSONExists ports _parse_json_exists (parsers/oracle.py:201-213).
func (p *Parser) parseOracleJSONExists() exp.Expression {
	this := p.parseFormatJson(p.parseBitwise())
	p.match(tokens.COMMA)
	path := p.parseBitwise()
	var passing []exp.Expression
	if p.matchTextSeq("PASSING") {
		passing = p.parseCsv(func() exp.Expression { return p.parseAlias(p.parseBitwise(), false) })
	}
	return p.expression(exp.JSONExists(exp.Args{
		"this":         this,
		"path":         path,
		"passing":      passing,
		"on_condition": p.parseOnCondition(),
	}), nil, nil)
}

// parseOracleInto ports _parse_into (parsers/oracle.py:215-232).
func (p *Parser) parseOracleInto() exp.Expression {
	bulkCollect := p.match(tokens.BULK_COLLECT_INTO)
	if !bulkCollect && !p.match(tokens.INTO) {
		return nil
	}
	index := p.index
	expressions := p.parseExpressions()
	if len(expressions) == 1 {
		p.retreat(index)
		p.match(tokens.TABLE)
		return p.expression(exp.Into(exp.Args{
			"this":         p.parseTable(true, false, nil, false, false, false, false),
			"bulk_collect": bulkCollect,
		}), nil, nil)
	}
	return p.expression(exp.Into(exp.Args{
		"bulk_collect": bulkCollect,
		"expressions":  expressions,
	}), nil, nil)
}

// parseOracleConnectWithPrior ports _parse_connect_with_prior (parsers/oracle.py:234-235).
func (p *Parser) parseOracleConnectWithPrior() exp.Expression {
	return p.parseAssignment()
}

// parseOracleInsertTable ports _parse_insert_table (parsers/oracle.py:250-261): Oracle
// insert targets take an alias without AS, an optional PARTITION, then the column list.
func (p *Parser) parseOracleInsertTable() exp.Expression {
	this := p.parseTableParts(true, false, false, false)
	if this != nil && this.Kind() == exp.KindTable {
		if alias := p.parseIdVar(false, nil); alias != nil {
			this.Set("alias", exp.TableAlias(exp.Args{"this": alias}))
		}
		if partition := p.parsePartition(); partition != nil {
			this.Set("partition", partition)
		}
		return p.parseSchema(this)
	}
	return this
}

// parseIntoStatement dispatches INTO parsing to the Oracle override when needed.
func (p *Parser) parseIntoStatement() exp.Expression {
	if p.dialect.Name == "oracle" {
		return p.parseOracleInto()
	}
	return p.parseInto()
}

// parseOracleHintFunctionCall ports _parse_hint_function_call
// (parsers/oracle.py:176-188) with _parse_hint_args (parsers/oracle.py:190-197): a bare
// NAME( followed by whitespace-separated vars. A comma or a dotted arg makes this fail, so
// the caller falls back to the raw hint text, matching upstream.
func (p *Parser) parseOracleHintFunctionCall() exp.Expression {
	if !p.curr.IsValid() || !p.next.IsValid() || p.next.TokenType != tokens.L_PAREN {
		return nil
	}
	name := p.curr.Text
	p.advance(2)
	var args []exp.Expression
	for {
		arg := p.parseVar(false, nil, false)
		if arg == nil {
			break
		}
		args = append(args, arg)
	}
	this := p.expression(exp.Anonymous(exp.Args{"this": name, "expressions": args}), nil, nil)
	p.matchRParen(this)
	return this
}
