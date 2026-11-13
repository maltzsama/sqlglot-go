package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// Dremio parser overrides, ported from
// .reference/sqlglot-v30.17.0/sqlglot/parsers/dremio.py.
func init() {
	registerDialectParserOverrides("dremio", dialectParserOverrideSet{
		// parsers/dremio.py:89-92 TABLE_ALIAS_TOKENS adds ANTI and SEMI.
		AliasTokens: map[tokens.TokenType]bool{tokens.ANTI: true, tokens.SEMI: true},
		// parsers/dremio.py:94-97 NO_PAREN_FUNCTION_PARSERS adds CURRENT_DATE_UTC.
		NoParenFunctionParsers: map[string]parserOverrideFunc{
			"CURRENT_DATE_UTC": (*Parser).parseDremioCurrentDateUTC,
		},
	})
}

// parseDremioCurrentDateUTC ports _parse_current_date_utc (parsers/dremio.py:111-120):
// CURRENT_DATE_UTC[(...)] builds CAST(CURRENT_TIMESTAMP AT TIME ZONE 'UTC' AS DATE).
func (p *Parser) parseDremioCurrentDateUTC() exp.Expression {
	if p.match(tokens.L_PAREN) {
		p.matchRParen(nil)
	}
	return p.expression(exp.Cast(exp.Args{
		"this": exp.AtTimeZone(exp.Args{
			"this": exp.CurrentTimestamp(nil),
			"zone": exp.LiteralString("UTC"),
		}),
		"to": exp.DataType(exp.Args{"this": exp.DTypeDate}),
	}), nil, nil)
}
