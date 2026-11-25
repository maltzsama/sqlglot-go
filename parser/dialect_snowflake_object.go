package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

func (p *Parser) parseSnowflakeObjectEntry() exp.Expression {
	if p.curr.TokenType == tokens.STRING && p.next.TokenType == tokens.COLON {
		key := p.parseString()
		p.match(tokens.COLON)
		value := p.parseDisjunction()
		return p.expression(exp.PropertyEQ(exp.Args{"this": exp.ToIdentifier(key.Name()), "expression": value}), nil, nil)
	}
	return p.parseBracketKeyValue()
}
