package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// RisingWave parser overrides, ported from
// .reference/sqlglot-v30.17.0/sqlglot/parsers/risingwave.py. RisingWaveParser extends
// PostgresParser, so lookups fall back to the postgres overlay.
func init() {
	registerDialectParserOverrides("risingwave", dialectParserOverrideSet{
		// parsers/risingwave.py:18-22 PROPERTY_PARSERS: ENCODE / INCLUDE / KEY on top of
		// the Postgres (base) table.
		PropertyParsers: map[string]propertyParserFunc{
			"ENCODE":  func(p *Parser, _ bool) exp.Expression { return p.parseEncodeProperty(false) },
			"INCLUDE": func(p *Parser, _ bool) exp.Expression { return p.parseIncludeProperty() },
			"KEY":     func(p *Parser, _ bool) exp.Expression { return p.parseEncodeProperty(true) },
		},
		// parsers/risingwave.py:34-37 _parse_table_hints always returns None.
		DisableTableHints: true,
	})
}

// parseIncludeProperty ports _parse_include_property (parsers/risingwave.py:40-54):
// `INCLUDE <name> [<coldef> [AS <alias>]]` — the CREATE SOURCE option that maps an upstream
// header column onto a RisingWave column.
func (p *Parser) parseIncludeProperty() exp.Expression {
	this := p.parseVarOrString(false)

	var header exp.Expression
	var columnDef exp.Expression
	// The first `_match(TokenType.ALIAS)` decides whether a column definition follows: a
	// header-less `INCLUDE key AS mykey` goes straight to the alias.
	if !p.match(tokens.ALIAS) {
		header = p.parseField(false, nil, false)
		if header != nil {
			columnDef = p.expression(exp.ColumnDef(exp.Args{
				"this": header,
				"kind": p.parseTypes(false, false, true, false),
			}), nil, nil)
		}
	}
	p.match(tokens.ALIAS)
	alias := p.parseIdVar(false, p.aliasTokensFor())

	return p.expression(exp.IncludeProperty(exp.Args{
		"this":       this,
		"alias":      alias,
		"column_def": columnDef,
	}), nil, nil)
}

// parseEncodeProperty ports _parse_encode_property (parsers/risingwave.py:56-70):
// `ENCODE <fmt> [(k = v, ...)]`, where the KEY property parser calls it with key=true so the
// option renders as `KEY ENCODE <fmt>`.
//
// The ENCODE text match is optional (as upstream's `_match_text_seq("ENCODE")` is): the
// plain ENCODE entry reaches here with ENCODE already consumed by the PROPERTY_PARSERS
// dispatch, while the KEY entry (`KEY ENCODE ...`) still has it ahead.
func (p *Parser) parseEncodeProperty(key bool) exp.Expression {
	p.matchTextSeq("ENCODE")
	this := p.parseVarOrString(false)

	var properties exp.Expression
	if p.match(tokens.L_PAREN, false) {
		properties = p.expression(exp.Properties(exp.Args{
			"expressions": p.parseWrappedProperties(),
		}), nil, nil)
	}

	args := exp.Args{"this": this, "properties": properties}
	// parsers/risingwave.py:66-70 passes key=None for a plain ENCODE; None args are
	// omitted by repr()/ToS, so the arg is only recorded when it is actually True.
	if key {
		args["key"] = true
	}
	return p.expression(exp.EncodeProperty(args), nil, nil)
}

// parseWatermarkConstraint ports the "WATERMARK" entry of RisingWaveParser.CONSTRAINT_PARSERS
// (parsers/risingwave.py:25-28): `WATERMARK FOR <col> AS <expr>`, the schema-level watermark
// declaration of a CREATE SOURCE. Both halves are guarded matches upstream too
// (`self._match(...) and ...`), so a missing FOR/AS leaves that arg unset rather than
// consuming anything.
func (p *Parser) parseWatermarkConstraint() exp.Expression {
	var this exp.Expression
	if p.match(tokens.FOR) {
		this = p.parseColumn()
	}
	var expression exp.Expression
	if p.match(tokens.ALIAS) {
		expression = p.parseDisjunction()
	}
	return p.expression(exp.WatermarkColumnConstraint(exp.Args{
		"this":       this,
		"expression": expression,
	}), nil, nil)
}
