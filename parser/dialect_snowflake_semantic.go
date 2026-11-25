package parser

import (
	"strings"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

func (p *Parser) parseSemanticView() exp.Expression {
	args := exp.Args{"this": p.parseTableParts(false, false, false, false)}
	for p.curr.IsValid() && p.curr.TokenType != tokens.R_PAREN {
		if p.matchTexts(map[string]bool{"DIMENSIONS": true, "METRICS": true, "FACTS": true}) {
			key := strings.ToLower(p.prev.Text)
			args[key] = p.parseCsv(func() exp.Expression {
				return p.parseAlias(p.parseDisjunction(), true)
			})
		} else if p.match(tokens.WHERE) {
			args["where"] = p.parseExpression()
		} else {
			p.raiseError("Expecting ) or encountered unexpected keyword")
			break
		}
	}
	return p.expression(exp.New(exp.KindSemanticView, args), nil, nil)
}
