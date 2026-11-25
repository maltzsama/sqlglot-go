package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

func (p *Parser) parseSnowflakeTags() exp.Expression {
	return p.expression(exp.New(exp.KindTags, exp.Args{"expressions": p.parseWrappedCsv(p.parseProperty)}), nil, nil)
}

func (p *Parser) parseSnowflakePolicyConstraint() exp.Expression {
	if p.prev.TokenType != tokens.WITH {
		p.retreat(p.index - 1)
	}
	var kind exp.Kind
	if p.matchTextSeq("MASKING", "POLICY") {
		kind = exp.KindMaskingPolicyColumnConstraint
	} else if p.matchTextSeq("PROJECTION", "POLICY") {
		kind = exp.KindProjectionPolicyColumnConstraint
	} else if p.match(tokens.TAG) {
		return p.parseSnowflakeTags()
	} else {
		return nil
	}
	policy := p.parseColumn()
	if policy != nil && policy.Kind() == exp.KindColumn {
		policy = exp.ColumnsToDot(policy)
	}
	args := exp.Args{"this": policy}
	if kind == exp.KindMaskingPolicyColumnConstraint && p.match(tokens.USING) {
		args["expressions"] = p.parseWrappedCsv(func() exp.Expression { return p.parseIdVar(false, nil) })
	}
	return p.expression(exp.New(kind, args), nil, nil)
}

func (p *Parser) parseSnowflakeRowAccess() exp.Expression {
	policy := p.parseColumn()
	if policy != nil && policy.Kind() == exp.KindColumn {
		policy = exp.ColumnsToDot(policy)
	}
	if !p.match(tokens.ON) {
		p.raiseError("Expected ON after ROW ACCESS POLICY name")
	}
	columns := p.parseWrappedCsv(func() exp.Expression { return p.parseIdVar(false, nil) })
	return p.expression(exp.New(exp.KindRowAccessProperty, exp.Args{"this": policy, "expressions": columns}), nil, nil)
}
