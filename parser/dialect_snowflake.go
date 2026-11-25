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
		// parsers/snowflake.py:772-778 PROPERTY_PARSERS additions. FILE_FORMAT and ROW are
		// handled inline in parseProperty (they need the wrapped-options/location-path
		// grammar); EXTERNAL is a plain marker.
		PropertyParsers: map[string]propertyParserFunc{
			"EXTERNAL": func(p *Parser, _ bool) exp.Expression {
				return p.expression(exp.ExternalProperty(nil), nil, nil)
			},
		},
		// parsers/snowflake.py:332 TYPED_LAMBDA_ARGS.
		TypedLambdaArgs: true,
		// parsers/snowflake.py:910-921 LAMBDAS: ARROW parses an assignment body and strips
		// the type annotation from the stored parameter list.
		Lambdas: map[tokens.TokenType]func(*Parser, []exp.Expression) exp.Expression{
			tokens.ARROW: snowflakeArrowLambda,
		},
		LambdaArgParser: snowflakeLambdaArg,
		FileLocation:    snowflakeFileLocation,
		TablePart:       snowflakeTablePart,
	})
}

// snowflakeFileLocation ports _parse_file_location (parsers/snowflake.py:1298-1304): COPY's
// file list accepts a subquery or a staged file, the latter parsed as table parts.
func snowflakeFileLocation(p *Parser) exp.Expression {
	if p.match(tokens.L_PAREN, false) {
		return p.parseSelect(true, false, false)
	}
	return p.snowflakeFileTableParts()
}

// snowflakeFileTableParts ports the Snowflake _parse_table_parts override
// (parsers/snowflake.py:1097-1137) for the COPY file-location position: a string or
// `@stage/path` location becomes a Table, optionally followed by the
// `(FILE_FORMAT => ..., PATTERN => ...)` suffix.
func (p *Parser) snowflakeFileTableParts() exp.Expression {
	var table exp.Expression
	switch {
	case p.curr.TokenType == tokens.STRING:
		table = p.parseString()
	case p.curr.TokenType == tokens.PARAMETER && p.curr.Text == "@":
		table = p.parseLocationPath()
	}
	if table == nil {
		return p.parseTableParts(false, false, false, false)
	}

	var fileFormat, pattern exp.Expression
	wrapped := p.match(tokens.L_PAREN)
	for p.curr.IsValid() && wrapped && !p.match(tokens.R_PAREN) {
		if p.matchTextSeq("FILE_FORMAT", "=>") {
			fileFormat = p.parseString()
			if fileFormat == nil {
				fileFormat = p.parseTableParts(false, false, false, false)
			}
		} else if p.matchTextSeq("PATTERN", "=>") {
			pattern = p.parseString()
		} else {
			break
		}
		p.match(tokens.COMMA)
	}

	return p.expression(exp.Table(exp.Args{"this": table, "format": fileFormat, "pattern": pattern}), nil, nil)
}

// parseLocationPath ports _parse_location_path (parsers/snowflake.py:1306-1318): consume
// every connected token until a comma or paren, keeping the raw source text as a Var.
func (p *Parser) parseLocationPath() exp.Expression {
	start := p.curr
	p.advanceAny(true)

	for p.isConnected() && p.curr.TokenType != tokens.COMMA &&
		p.curr.TokenType != tokens.L_PAREN && p.curr.TokenType != tokens.R_PAREN {
		p.advanceAny(true)
	}

	return p.expression(exp.Var(exp.Args{"this": p.findSQL(start, p.prev)}), nil, nil)
}

// snowflakeLambdaArg ports _parse_lambda_arg (parsers/snowflake.py:1320-1331): the base
// id-var followed by an optional type annotation, wrapped in a Cast.
func snowflakeLambdaArg(p *Parser) exp.Expression {
	this := p.parseIdVar(true, nil)
	if this == nil {
		return nil
	}
	if typ := p.parseTypes(false, false, false, false); typ != nil {
		return p.expression(exp.Cast(exp.Args{"this": this, "to": typ}), nil, nil)
	}
	return this
}

// snowflakeArrowLambda ports the LAMBDAS[ARROW] override (parsers/snowflake.py:912-919):
// the body is an assignment (not a disjunction) and the stored params drop the Cast wrapper
// introduced by snowflakeLambdaArg, keeping only the annotated name.
func snowflakeArrowLambda(p *Parser, params []exp.Expression) exp.Expression {
	body := replaceLambda(p.parseAssignment(), params)
	stripped := make([]exp.Expression, len(params))
	for i, param := range params {
		if param != nil && param.Kind() == exp.KindCast {
			stripped[i] = param.This()
		} else {
			stripped[i] = param
		}
	}
	return p.expression(exp.Lambda(exp.Args{"this": body, "expressions": stripped}), nil, nil)
}

func snowflakeTablePart(p *Parser, _ bool, this exp.Expression) exp.Expression {
	if this == nil {
		return nil
	}
	if this.Kind() == exp.KindParameter && p.index >= 2 && p.tokens[p.index-2].Text == "@" {
		name := this.This().Name()
		if identifier := this.This(); identifier != nil {
			quoted, _ := identifier.Arg("quoted").(bool)
			if quoted {
				name = `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
			}
		}
		for p.isConnected() {
			switch p.curr.TokenType {
			case tokens.SLASH, tokens.DOT, tokens.MOD, tokens.VAR, tokens.FILE, tokens.JSON,
				tokens.NUMBER, tokens.DASH:
				name += p.curr.Text
				p.advance()
			default:
				goto stagePathDone
			}
		}
	stagePathDone:
		return p.expression(exp.Var(exp.Args{"this": "@" + name}), nil, nil)
	}
	if p.prev.TokenType == tokens.STRING && this.Kind() == exp.KindIdentifier {
		return p.expression(exp.LiteralString(this.Name()), nil, nil)
	}
	return this
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

// snowflakeFlattenColumns ports SnowflakeParser.FLATTEN_COLUMNS
// (parsers/snowflake.py:866): the output columns FLATTEN produces when the query does not
// alias them itself.
func snowflakeFlattenColumns() []exp.Expression {
	names := []string{"SEQ", "KEY", "PATH", "INDEX", "VALUE", "THIS"}
	out := make([]exp.Expression, 0, len(names))
	for _, name := range names {
		out = append(out, exp.ToIdentifier(name))
	}
	return out
}
