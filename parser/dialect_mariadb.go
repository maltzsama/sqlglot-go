package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// MariaDB parser overrides. Upstream sqlglot has no MariaDB dialect, so these are
// beyond-upstream grammar extensions gated on the dialect name; the override chain treats
// "mariadb" as a child of "mysql", so every MySQL override still applies.
func init() {
	registerDialectParserOverrides("mariadb", dialectParserOverrideSet{
		// MariaDB sequence access: NEXT VALUE FOR seq / PREVIOUS VALUE FOR seq. NEXT reuses the
		// upstream `_parse_next_value_for` builder (parser.py:8074) that TSQL/Oracle register;
		// PREVIOUS has no upstream node, so it builds the port's PrevValueFor (see DEVIATIONS
		// "Opt-in behavioral extensions").
		NoParenFunctionParsers: map[string]parserOverrideFunc{
			"NEXT":     (*Parser).parseTSQLNextValueFor,
			"PREVIOUS": (*Parser).parseMariaDBPreviousValueFor,
		},
	})
}

// parseMariaDBPreviousValueFor ports the MariaDB `PREVIOUS VALUE FOR <seq>` sequence form,
// mirroring the upstream `_parse_next_value_for` grammar (parser.py:8074-8084) with the
// PrevValueFor node.
func (p *Parser) parseMariaDBPreviousValueFor() exp.Expression {
	if !p.matchTextSeq("VALUE", "FOR") {
		p.retreat(p.index - 1)
		return nil
	}
	return p.expression(exp.PrevValueFor(exp.Args{"this": p.parseColumn()}), nil, nil)
}
