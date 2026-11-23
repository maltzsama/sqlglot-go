package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

func (p *Parser) parseExpression() exp.Expression {
	// parsers/duckdb.py:341-355: a prefix alias, e.g. `foo: 1`.
	if p.dialect.PrefixAlias && p.next.TokenType == tokens.COLON {
		alias := p.parseIdVar(false, aliasTokens)
		p.match(tokens.COLON)
		comments := p.prevComments
		this := p.parseAssignment()
		if this == nil {
			p.raiseError("Expected expression after prefix alias")
			return nil
		}
		comments = append(comments, this.PopComments()...)
		return p.expression(exp.AliasExpr(this, alias, false), nil, comments)
	}
	this := p.parseAlias(p.parseAssignment(), false)
	// parsers/clickhouse.py:952-960: `SELECT <expr> APPLY(func)` query modifier chain.
	if p.dialect.Name == "clickhouse" {
		this = p.applyClickhouseApplyChain(this)
	}
	return this
}

var assignmentTokens = map[tokens.TokenType]func(exp.Args) exp.Expression{
	tokens.COLON_EQ: exp.PropertyEQ,
}

// parseAssignment ports _parse_assignment (parser.py:5790-5810): the ASSIGNMENT `:=`
// operator, e.g. mysql `SELECT @var1 := 1, @var2 := COUNT(*)`.
func (p *Parser) parseAssignment() exp.Expression {
	this := p.parseDisjunction()
	if this == nil && assignmentTokens[p.next.TokenType] != nil {
		// `<non-identifier token> := <expr>` (parser.py:5793-5796): lets a token that
		// otherwise fails to parse as a standalone expression still act as the LHS.
		if tok := p.advanceAny(true); tok != nil {
			this = exp.Column(exp.Args{"this": exp.ToIdentifier(tok.Text)})
		}
	}
	for constructor, ok := assignmentTokens[p.curr.TokenType]; ok; constructor, ok = assignmentTokens[p.curr.TokenType] {
		if this != nil && this.Kind() == exp.KindColumn && len(this.Parts()) == 1 {
			this = this.This()
		}
		p.advance()
		comments := p.prevComments
		this = p.expression(constructor(exp.Args{"this": this, "expression": p.parseAssignment()}), nil, comments)
	}
	// parsers/clickhouse.py:546-558 _parse_assignment: `x ? y : z` -> If.
	if p.dialect.Name == "clickhouse" && p.match(tokens.PLACEHOLDER) {
		trueExpr := p.parseAssignment()
		var falseExpr any
		if p.match(tokens.COLON) {
			falseExpr = p.parseAssignment()
		} else {
			falseExpr = false
		}
		this = p.expression(exp.If(exp.Args{"this": this, "true": trueExpr, "false": falseExpr}), nil, nil)
	}
	return this
}

// parseDisjunction ports _parse_disjunction (parser.py:5812-5822), matching the DISJUNCTION
// token set. Upstream's DISJUNCTION is per-dialect: base has only OR (parser.py:885-887);
// mysql additionally maps DPIPE -> exp.Or (parsers/mysql.py:78-80), since mysql's
// DPIPE_IS_STRING_CONCAT=False frees `||` up to mean logical OR instead of concatenation.
func (p *Parser) parseDisjunction() exp.Expression {
	this := p.parseXorLevel()
	for p.match(tokens.OR) || (p.dialectIs("mysql") && p.match(tokens.DPIPE)) {
		comments := p.prevComments
		this = p.expression(exp.Or(exp.Args{"this": this, "expression": p.parseXorLevel()}), nil, comments)
	}
	return this
}

// parseXorLevel places MariaDB's XOR between OR and AND (MariaDB operator precedence: OR is
// lower than XOR, which is lower than AND); MySQL keeps upstream's grouping where XOR shares
// the AND level.
func (p *Parser) parseXorLevel() exp.Expression {
	this := p.parseConjunction()
	for p.dialectIs("mariadb") && p.match(tokens.XOR) {
		comments := p.prevComments
		this = p.expression(exp.Xor(exp.Args{"this": this, "expression": p.parseConjunction()}), nil, comments)
	}
	return this
}

// parseConjunction ports _parse_conjunction (parser.py:5824-5834), matching the CONJUNCTION
// token set. Upstream's CONJUNCTION is per-dialect: base has only AND (parser.py:877-879);
// mysql additionally maps DAMP (`&&`) -> exp.And and XOR -> exp.Xor (parsers/mysql.py:72-76).
func (p *Parser) parseConjunction() exp.Expression {
	this := p.parseEquality()
	for {
		var constructor func(exp.Args) exp.Expression
		switch {
		case p.match(tokens.AND):
			constructor = exp.And
		case p.dialectIs("mysql") && p.match(tokens.DAMP):
			constructor = exp.And
		case p.dialectIs("mysql") && !p.dialectIs("mariadb") && p.match(tokens.XOR):
			constructor = exp.Xor
		default:
			return this
		}
		comments := p.prevComments
		this = p.expression(constructor(exp.Args{"this": this, "expression": p.parseEquality()}), nil, comments)
	}
}

var equalityTokens = map[tokens.TokenType]func(exp.Args) exp.Expression{
	tokens.EQ:          exp.EQ,
	tokens.NEQ:         exp.NEQ,
	tokens.NULLSAFE_EQ: exp.NullSafeEQ,
}

func (p *Parser) parseEquality() exp.Expression {
	this := p.parseComparison()
	for constructor, ok := equalityTokens[p.curr.TokenType]; ok; constructor, ok = equalityTokens[p.curr.TokenType] {
		p.advance()
		comments := p.prevComments
		this = p.expression(constructor(exp.Args{"this": this, "expression": p.parseComparison()}), nil, comments)
	}
	return this
}

var comparisonTokens = map[tokens.TokenType]func(exp.Args) exp.Expression{
	tokens.GT:  exp.GT,
	tokens.GTE: exp.GTE,
	tokens.LT:  exp.LT,
	tokens.LTE: exp.LTE,
}

func (p *Parser) parseComparison() exp.Expression {
	this := p.parseRange(nil)
	for constructor, ok := comparisonTokens[p.curr.TokenType]; ok; constructor, ok = comparisonTokens[p.curr.TokenType] {
		p.advance()
		comments := p.prevComments
		this = p.expression(constructor(exp.Args{"this": this, "expression": p.parseRange(nil)}), nil, comments)
	}
	return this
}
