package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// parseColonAsVariantExtract ports _parse_colon_as_variant_extract (parser.py:6936-6987): the
// `variant:key[key2].key3` accessor chain, built as JSONExtract over a JSONPath. Called from
// parseColumn when the dialect sets COLON_IS_VARIANT_EXTRACT (databricks).
func (p *Parser) parseColonAsVariantExtract(this exp.Expression) exp.Expression {
	pathParts := []exp.Expression{exp.JSONPathRoot(nil)}

	for p.match(tokens.COLON) {
		// COLON_CHAIN_IS_SINGLE_EXTRACT False (databricks) closes the previous extraction
		// before starting the next one, so x:y:z nests JSONExtract nodes.
		if !p.dialect.ColonChainIsSingleExtract {
			this, pathParts = p.buildJSONExtract(this, pathParts)
		}

		if key := p.parseIdVar(true, map[tokens.TokenType]bool{tokens.SELECT: true}); key != nil {
			pathParts = append(pathParts, exp.JSONPathKey(exp.Args{
				"this":   key.Name(),
				"quoted": key.Kind() == exp.KindIdentifier && argBool(key.Arg("quoted")),
			}))
		}

		for {
			if p.match(tokens.DOT) {
				if next := p.parseIdVar(true, map[tokens.TokenType]bool{tokens.SELECT: true}); next != nil {
					pathParts = append(pathParts, exp.JSONPathKey(exp.Args{
						"this":   next.Name(),
						"quoted": next.Kind() == exp.KindIdentifier && argBool(next.Arg("quoted")),
					}))
				}
				continue
			}
			if p.match(tokens.L_BRACKET) {
				bracket := p.parseBracketKeyValue()
				if !p.match(tokens.R_BRACKET) {
					p.raiseError("Expected ]")
				}
				if bracket != nil {
					switch {
					case bracket.IsString():
						pathParts = append(pathParts, exp.JSONPathKey(exp.Args{"this": bracket.Name(), "quoted": true}))
					case bracket.IsStar():
						pathParts = append(pathParts, exp.JSONPathSubscript(exp.Args{"this": exp.JSONPathWildcard(nil)}))
					case bracket.IsNumber():
						pathParts = append(pathParts, exp.JSONPathSubscript(exp.Args{"this": bracket.ToPy()}))
					default:
						this, pathParts = p.buildJSONExtract(this, pathParts)
						this = p.expression(exp.Bracket(exp.Args{
							"this":        this,
							"expressions": []exp.Expression{bracket},
							"json_access": true,
						}), nil, nil)
					}
				}
				continue
			}
			if p.match(tokens.DCOLON) {
				this, pathParts = p.buildJSONExtract(this, pathParts)
				castType := p.parseTypes(false, false, true, false)
				if castType == nil {
					p.raiseError("Expected type after '::'")
				}
				this = p.expression(exp.Cast(exp.Args{"this": this, "to": castType}), nil, nil)
				continue
			}
			break
		}
	}

	this, _ = p.buildJSONExtract(this, pathParts)
	return this
}

// buildJSONExtract ports _build_json_extract (parser.py:6918-6932).
func (p *Parser) buildJSONExtract(this exp.Expression, pathParts []exp.Expression) (exp.Expression, []exp.Expression) {
	if len(pathParts) > 1 {
		this = p.expression(exp.JSONExtract(exp.Args{
			"this":            this,
			"expression":      exp.JSONPath(exp.Args{"expressions": pathParts}),
			"variant_extract": true,
			"requires_json":   false,
		}), nil, nil)
		pathParts = []exp.Expression{exp.JSONPathRoot(nil)}
	}
	return this, pathParts
}

// argBool reports whether value is the boolean true.
func argBool(value any) bool {
	b, _ := value.(bool)
	return b
}

// parseStreamTable ports _parse_stream (parser.py:4613-4620): a `STREAM <table>` source,
// tokenized only by dialects that add the STREAM keyword (databricks).
func (p *Parser) parseStreamTable() exp.Expression {
	index := p.index
	if !p.match(tokens.STREAM) {
		return nil
	}
	var this exp.Expression
	p.tryParse(func() exp.Expression {
		this = p.parseTable(false, false, nil, false, false, false, false)
		return this
	}, false)
	if this != nil {
		return p.expression(exp.Stream(exp.Args{"this": this}), nil, nil)
	}
	p.retreat(index)
	return nil
}
