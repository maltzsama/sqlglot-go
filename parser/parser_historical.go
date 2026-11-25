package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

var historicalPrefixes = map[string]bool{"AT": true, "BEFORE": true, "END": true}
var historicalKinds = map[string]bool{"OFFSET": true, "STATEMENT": true, "STREAM": true, "TIMESTAMP": true, "VERSION": true}

func (p *Parser) parseHistoricalData() exp.Expression {
	index := p.index
	if !p.matchTexts(historicalPrefixes) {
		return nil
	}
	prefix := stringsUpper(p.prev.Text)
	if !p.match(tokens.L_PAREN) || !p.matchTexts(historicalKinds) {
		p.retreat(index)
		return nil
	}
	kind := stringsUpper(p.prev.Text)
	if !p.match(tokens.FARROW) {
		p.retreat(index)
		return nil
	}
	value := p.parseBitwise()
	if value == nil {
		p.retreat(index)
		return nil
	}
	p.matchRParen(value)
	return p.expression(exp.New(exp.KindHistoricalData, exp.Args{"this": prefix, "kind": kind, "expression": value}), nil, nil)
}

func (p *Parser) parseChanges() exp.Expression {
	if !p.matchTextSeq("CHANGES", "(", "INFORMATION", "=>") {
		return nil
	}
	information := p.parseVar(true, nil, false)
	p.matchRParen(information)
	return p.expression(exp.New(exp.KindChanges, exp.Args{
		"information": information,
		"at_before":   p.parseHistoricalData(),
		"end":         p.parseHistoricalData(),
	}), nil, nil)
}
