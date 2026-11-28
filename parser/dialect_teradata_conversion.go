package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// Teradata conversion attributes: `expr (TYPE [, FORMAT 'f'] [, TITLE 't'] [, NAMED n])` and the
// standalone attribute forms. Ports parsers/teradata.py:43-65 (CONVERSION_TYPES, CONVERSION_ATTRIBUTES)
// and parsers/teradata.py:118-125, 429-520 (_conversion_follows, _parse_conversion_type,
// _parse_conversion_items, _parse_type).

// teradataConversionTypes ports CONVERSION_TYPES: each type name maps to the number of size
// parameters it may take in a conversion (parsers/teradata.py:43-64).
var teradataConversionTypes = map[string]int{
	"BYTEINT": 0, "SMALLINT": 0, "INT": 0, "INTEGER": 0, "BIGINT": 0,
	"DEC": 2, "DECIMAL": 2, "NUMERIC": 2, "FLOAT": 1, "REAL": 0, "DOUBLE PRECISION": 0,
	"CHAR": 1, "CHARACTER": 1, "VARCHAR": 1, "CHAR VARYING": 1, "CHARACTER VARYING": 1,
	"DATE": 0, "TIME": 1, "TIMESTAMP": 1, "INTERVAL": 0,
}

// teradataConversionAttributes ports CONVERSION_ATTRIBUTES (parsers/teradata.py:65).
var teradataConversionAttributes = map[string]bool{"FORMAT": true, "TITLE": true, "NAMED": true}

// conversionTypeKeys and conversionAttributeKeys are the keyword sets as token texts.
var (
	conversionTypeKeys      = conversionTypeKeySet()
	conversionAttributeKeys = teradataConversionAttributes
)

func conversionTypeKeySet() map[string]bool {
	out := make(map[string]bool, len(teradataConversionTypes))
	for k := range teradataConversionTypes {
		out[k] = true
	}
	return out
}

// teradataConversionFollows ports _conversion_follows (parsers/teradata.py:429-443): a
// parenthesized conversion or attribute list follows the current word.
func (p *Parser) teradataConversionFollows() bool {
	if p.next.TokenType != tokens.L_PAREN {
		return false
	}
	index := p.index
	p.advance(2)
	var follows bool
	if p.matchTexts(conversionAttributeKeys, false) {
		// Attribute keywords can also be function names, e.g. f(TITLE(x)).
		follows = p.next.TokenType != tokens.L_PAREN
	} else {
		follows = p.matchTexts(conversionTypeKeys, false) && p.teradataConversionItemsParse()
	}
	p.retreat(index)
	return follows
}

// teradataConversionItemsParse runs the item parser speculatively (Python's _try_parse): it
// reports whether the items parsed, and leaves the position unchanged on failure.
func (p *Parser) teradataConversionItemsParse() bool {
	index := p.index
	if _, ok := p.parseTeradataConversionItems(); ok {
		return true
	}
	p.retreat(index)
	return false
}

// parseTeradataConversionType ports _parse_conversion_type (parsers/teradata.py:445-464): in
// this ambiguous position only a literal precision, scale or length counts.
func (p *Parser) parseTeradataConversionType() exp.Expression {
	index := p.index
	if p.next.TokenType == tokens.L_PAREN {
		maxParams := teradataConversionTypes[stringsUpper(p.curr.Text)]
		p.advance(2)
		matched := false
		for i := 0; i < maxParams; i++ {
			if !p.match(tokens.NUMBER) || !isDecimalText(p.prev.Text) {
				return nil
			}
			if p.match(tokens.R_PAREN) {
				matched = true
				break
			}
			if !p.match(tokens.COMMA) {
				return nil
			}
		}
		if !matched {
			return nil
		}
		p.retreat(index)
	}
	return p.parseTypes(false, false, false, false)
}

// isDecimalText reports whether text is all ASCII digits (Python's str.isdecimal).
func isDecimalText(text string) bool {
	if text == "" {
		return false
	}
	for _, c := range text {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// parseTeradataConversionItems ports _parse_conversion_items (parsers/teradata.py:466-496). It
// returns the keyed items and whether the parenthesized list parsed completely.
func (p *Parser) parseTeradataConversionItems() (map[string]exp.Expression, bool) {
	items := map[string]exp.Expression{}
	for {
		var key string
		var value exp.Expression
		if p.matchTexts(conversionAttributeKeys) {
			key = stringsUpper(p.prev.Text)
			if key == "NAMED" {
				value = p.parseIdVar(false, map[tokens.TokenType]bool{tokens.VAR: true, tokens.STRING: true})
				if value == nil || value.Kind() != exp.KindIdentifier {
					return nil, false
				}
			} else {
				value = p.parseString()
				if value == nil || (key == "TITLE" && !value.IsString()) {
					return nil, false
				}
			}
		} else if p.matchTexts(conversionTypeKeys, false) {
			key = "TYPE"
			value = p.parseTeradataConversionType()
			if value == nil || value.Kind() != exp.KindDataType {
				return nil, false
			}
		} else {
			return nil, false
		}
		if _, dup := items[key]; dup {
			return nil, false
		}
		items[key] = value
		if !p.match(tokens.COMMA) {
			break
		}
	}
	if !p.match(tokens.R_PAREN) {
		return nil, false
	}
	return items, true
}

// isNumericDataType reports whether a data type is one of the NUMERIC_TYPES (datatypes.py).
func isNumericDataType(dt exp.Expression) bool {
	for dtype := range exp.NumericTypes {
		if dt.IsType(dtype) {
			return true
		}
	}
	return false
}

// parseTeradataTypeConversions ports TeradataParser._parse_type (parsers/teradata.py:498-527):
// each parenthesized conversion or attribute list after a type wraps it.
func (p *Parser) parseTeradataTypeConversions(this exp.Expression, parseInterval, fallbackToIdentifier bool) exp.Expression {
	for this != nil && p.match(tokens.L_PAREN) {
		index := p.index - 1
		items, ok := p.parseTeradataConversionItems()
		if !ok {
			if p.matchTexts(conversionAttributeKeys, false) {
				p.raiseError("Invalid Teradata conversion attribute")
			}
			p.retreat(index)
			break
		}
		to, format := items["TYPE"], items["FORMAT"]
		if to != nil {
			if format != nil || !isNumericDataType(to) {
				this = p.expression(exp.TeradataConversion(exp.Args{"this": this, "to": to, "format": format}), nil, nil)
			} else {
				this = p.expression(exp.Cast(exp.Args{"this": this, "to": to}), nil, nil)
			}
		} else if format != nil {
			this = p.expression(exp.FormatPhrase(exp.Args{"this": this, "format": format}), nil, nil)
		}
		if title, ok := items["TITLE"]; ok {
			this = p.expression(exp.TeradataTitle(exp.Args{"this": this, "title": title}), nil, nil)
		}
		if named, ok := items["NAMED"]; ok {
			this = p.expression(exp.TeradataNamed(exp.Args{"this": this, "alias": named}), nil, nil)
		}
		this = p.parseColumnOps(this)
	}
	return this
}

// teradataProjections ports TeradataParser._parse_projections (parsers/teradata.py:529-545):
// NAMED and TITLE wrappers and FORMAT phrases on a projection move out of the expression, and a
// NAMED wrapper becomes the projection's alias.
func (p *Parser) teradataProjections(projections []exp.Expression) []exp.Expression {
	for i, projection := range projections {
		var alias exp.Expression
		if projection != nil && projection.Kind() == exp.KindAlias && projection.Arg("this") != nil {
			alias = projection
		}
		this := projection
		if alias != nil {
			this = asExpr(alias.Arg("this"))
		}
		node := this
		var namedAlias exp.Expression
		for node != nil && (node.Kind() == exp.KindFormatPhrase || node.Kind() == exp.KindTeradataTitle || node.Kind() == exp.KindTeradataNamed) {
			child := asExpr(node.Arg("this"))
			if node.Kind() == exp.KindTeradataNamed {
				if namedAlias == nil {
					namedAlias = asExpr(node.Arg("alias"))
				}
				if node == this {
					this = child
				}
				node.Replace(child)
			}
			node = child
		}
		if namedAlias != nil {
			if alias != nil {
				alias.Set("this", this)
			} else {
				projections[i] = p.expression(exp.AliasNode(exp.Args{"this": this, "alias": namedAlias}), nil, nil)
			}
		}
	}
	return projections
}

// teradataNoParenConversion ports the conversion branch of TeradataParser._parse_function
// (parsers/teradata.py:403-427). It reports ok=true when the branch decides the result: build is
// then the no-paren builder to use (nil means no builder, so the call is not a function).
func (p *Parser) teradataNoParenConversion(functions map[string]func([]exp.Expression) exp.Expression) (func() exp.Expression, bool) {
	name := stringsUpper(p.curr.Text)
	if p.curr.TokenType == tokens.IDENTIFIER {
		return nil, true
	}
	if p.prev.TokenType != tokens.DOT {
		build := p.noParenFunctionFor(p.curr.TokenType)
		switch name {
		case "CURDATE":
			build = func(exp.Args) exp.Expression { return exp.CurrentDate(exp.Args{}) }
		case "CURTIME":
			build = func(exp.Args) exp.Expression { return exp.CurrentTime(exp.Args{}) }
		}
		if build != nil {
			return func() exp.Expression { return build(exp.Args{}) }, true
		}
		if name == "DATE" {
			return nil, true
		}
	}
	if p.index+2 < len(p.tokens) && teradataConversionAttributes[stringsUpper(p.tokens[p.index+2].Text)] {
		return nil, true
	}
	// name not in FUNCTIONS (the caller's table, or the dialect's) and not in FUNCTION_PARSERS.
	known := false
	if functions != nil {
		_, known = functions[name]
	} else {
		known = dialectFunction(p.dialect, name) != nil
	}
	if !known && p.functionParser(name) == nil {
		return nil, true
	}
	return nil, false
}

// parseTeradataTableFunction ports TeradataParser._parse_table_function (parsers/teradata.py:377-398):
// TABLE(db.f(DATE)) needs a function call even when DATE could be a target type. The qualified
// name builds a dotted chain, and the call is never parsed as a no-paren form.
func (p *Parser) parseTeradataTableFunction() exp.Expression {
	p.advance(2)
	var parts []exp.Expression
	for p.next.TokenType == tokens.DOT {
		qualifier := p.parseIdVar(false, nil)
		if qualifier == nil || !p.match(tokens.DOT) {
			return nil
		}
		parts = append(parts, qualifier)
	}
	anonymous := len(parts) > 0 || p.curr.TokenType == tokens.IDENTIFIER
	call := p.parseFunction(nil, anonymous, false, len(parts) > 0)
	if call == nil || !p.match(tokens.R_PAREN) {
		return nil
	}
	this := call
	if len(parts) > 0 {
		this = exp.DotBuild(append(parts, call))
	}
	return p.expression(exp.New(exp.KindTableFromRows, exp.Args{"this": this}), nil, nil)
}

// teradataDeleteAliasTokens ports TABLE_ALIAS_TOKENS - {ALL} (parsers/teradata.py:268-271).
func teradataDeleteAliasTokens() map[tokens.TokenType]bool {
	out := make(map[tokens.TokenType]bool, len(tableAliasTokens))
	for tt := range tableAliasTokens {
		if tt != tokens.ALL {
			out[tt] = true
		}
	}
	return out
}

// parseTeradataExecute ports TeradataParser._parse_execute (parsers/teradata.py:193-212): a macro
// call `EXECUTE db.macro(args)` builds an Execute. Other EXECUTE forms, such as EXECUTE IMMEDIATE,
// are kept as a Command.
func (p *Parser) parseTeradataExecute() exp.Expression {
	start := p.prev
	this := p.parseTableParts(true, false, false, false)
	var expressions []exp.Expression
	if p.match(tokens.L_PAREN) && !p.match(tokens.R_PAREN) {
		for {
			argument := p.parseAssignment()
			if argument == nil {
				// Do not discard omitted arguments or replace them with NULL.
				return p.parseAsCommand(start)
			}
			expressions = append(expressions, argument)
			if !p.match(tokens.COMMA) {
				break
			}
		}
		p.matchRParen(nil)
	}
	if p.curr.IsValid() {
		// Preserve other EXECUTE forms, such as EXECUTE IMMEDIATE.
		return p.parseAsCommand(start)
	}
	return p.expression(exp.Execute(exp.Args{"this": this, "expressions": expressions}), nil, nil)
}
