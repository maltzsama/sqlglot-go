package parser

import (
	"strings"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// parsers/snowflake.py supplies the SnowflakeParser overlays.
func init() {
	registerDialectParserOverrides("snowflake", dialectParserOverrideSet{
		StatementParsers: map[tokens.TokenType]parserOverrideFunc{
			// parsers/snowflake.py:761-767 STATEMENT_PARSERS.
			tokens.UNDROP: (*Parser).parseSnowflakeUndrop,
		},
		FunctionParsers: map[string]parserOverrideFunc{
			// parsers/snowflake.py:736-742 FUNCTION_PARSERS.
			"DATE_PART": (*Parser).parseSnowflakeDatePart,
		},
		// parsers/snowflake.py:382-387 NO_PAREN_FUNCTIONS.
		NoParenFunctions: map[tokens.TokenType]func(exp.Args) exp.Expression{
			tokens.LOCALTIME:      func(exp.Args) exp.Expression { return exp.Localtime(exp.Args{}) },
			tokens.LOCALTIMESTAMP: func(exp.Args) exp.Expression { return exp.Localtimestamp(exp.Args{}) },
			tokens.CURRENT_TIME:   func(exp.Args) exp.Expression { return exp.Localtime(exp.Args{}) },
		},
		// parsers/snowflake.py:344-373 ALIAS_TOKENS / TABLE_ALIAS_TOKENS additions.
		AliasTokens: map[tokens.TokenType]bool{
			tokens.ANTI: true, tokens.INTEGRATION: true, tokens.PACKAGE: true, tokens.POLICY: true,
			tokens.POOL: true, tokens.ROLE: true, tokens.RULE: true, tokens.SEMI: true,
			tokens.VOLUME: true, tokens.WINDOW: true,
		},
	})
}

// parseSnowflakeUndrop ports _parse_undrop (parsers/snowflake.py:1252-1262).
func (p *Parser) parseSnowflakeUndrop() exp.Expression {
	start := p.prev
	kind := p.parseVarFromOptions(snowflakeUndropObjects, false)
	if kind == nil {
		return p.parseAsCommand(start)
	}
	kindName := strings.ToUpper(kind.Name())
	isDBReference := kindName == "ACCOUNT" || kindName == "DATABASE" || kindName == "SCHEMA"
	this := p.parseTableParts(isDBReference, isDBReference, false, false)
	var rename exp.Expression
	if p.matchTextSeq("RENAME", "TO") {
		rename = p.parseTableParts(false, false, false, false)
	}
	return p.expression(exp.Undrop(exp.Args{"this": this, "kind": kindName, "rename": rename}), nil, nil)
}

// snowflakeUndropObjects ports UNDROP_OBJECTS (parsers/snowflake.py:879-897).
var snowflakeUndropObjects = optionsType{
	"ACCOUNT": {}, "DATABASE": {}, "NOTEBOOK": {}, "SCHEMA": {}, "SNAPSHOT": {},
	"STREAMLIT": {}, "TABLE": {}, "TAG": {}, "TYPE": {},
	"DYNAMIC":  {{"TABLE"}},
	"EXTERNAL": {{"VOLUME"}},
	"ICEBERG":  {{"TABLE"}},
}

// parseSnowflakeDatePart ports _parse_date_part (parsers/snowflake.py:1061-1071).
func (p *Parser) parseSnowflakeDatePart() exp.Expression {
	this := p.parseVar(false, nil, false)
	if this == nil {
		this = p.parseType(false, false)
	}
	if this == nil {
		return nil
	}
	var expression exp.Expression
	if p.matchSet(map[tokens.TokenType]bool{tokens.FROM: true, tokens.COMMA: true}) {
		expression = p.parseBitwise()
	}
	if name := strings.ToUpper(this.Name()); name != "" {
		if mapped := p.dialect.DatePartMapping[name]; mapped != "" {
			this = exp.Var(exp.Args{"this": mapped})
		}
	}
	return p.expression(exp.Extract(exp.Args{"this": this, "expression": expression}), nil, nil)
}
