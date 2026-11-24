package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

func (p *Parser) parseSnowflakeLocation() exp.Expression {
	start := p.curr
	p.advance()
	for p.curr.IsValid() && p.isConnected() && p.curr.TokenType != tokens.COMMA &&
		p.curr.TokenType != tokens.L_PAREN && p.curr.TokenType != tokens.R_PAREN {
		p.advance()
	}
	return exp.Var(exp.Args{"this": p.findSQL(start, p.prev)})
}

func (p *Parser) parseSnowflakeStageTable(isDBReference bool) exp.Expression {
	var location exp.Expression
	if p.curr.TokenType == tokens.STRING {
		location = p.parseString()
	} else {
		location = p.parseSnowflakeLocation()
	}
	var format, pattern exp.Expression
	if p.match(tokens.L_PAREN) {
		for p.curr.IsValid() && !p.match(tokens.R_PAREN) {
			switch {
			case p.matchTextSeq("FILE_FORMAT", "=>"):
				format = p.parseString()
				if format == nil {
					format = p.parseTableParts(false, isDBReference, false, false)
				}
			case p.matchTextSeq("PATTERN", "=>"):
				pattern = p.parseString()
			default:
				p.raiseError("Expected FILE_FORMAT or PATTERN")
				return nil
			}
			p.match(tokens.COMMA)
		}
	}
	return p.expression(exp.Table(exp.Args{"this": location, "format": format, "pattern": pattern}), nil, nil)
}
