package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// SQLite parser overrides, ported from
// .reference/sqlglot-v30.17.0/sqlglot/parsers/sqlite.py. The grammar that does not fit the
// table-driven hooks lives as sqlite-gated branches in the shared parser methods: the
// ||/->/->> concat tier in parseFactorOperand, the PRAGMA form in parsePragma, the
// standalone-UNIQUE check in parseUnique, computed columns in parseGeneratedAsIdentity, MATCH
// in parseRange, INDEXED BY in parseTableCore, and the ARROW/DARROW removal in
// parseColumnOpsBase.
func init() {
	registerDialectParserOverrides("sqlite", dialectParserOverrideSet{
		// parsers/sqlite.py:43-46 TABLE_ALIAS_TOKENS adds ANTI and SEMI.
		AliasTokens: map[tokens.TokenType]bool{tokens.ANTI: true, tokens.SEMI: true},
		// parsers/sqlite.py:87-92 STATEMENT_PARSERS adds ATTACH and DETACH.
		StatementParsers: map[tokens.TokenType]parserOverrideFunc{
			tokens.ATTACH: (*Parser).parseSQLiteAttach,
			tokens.DETACH: (*Parser).parseSQLiteDetach,
		},
		// parsers/sqlite.py:81-85 PROPERTY_PARSERS adds USING and VIRTUAL.
		PropertyParsers: map[string]propertyParserFunc{
			"USING": func(p *Parser, _ bool) exp.Expression {
				return p.parseSQLiteModuleProperty()
			},
			"VIRTUAL": func(p *Parser, _ bool) exp.Expression {
				return p.expression(exp.VirtualProperty(nil), nil, nil)
			},
		},
	})
}

// parseSQLiteAttach ports the ATTACH arm of STATEMENT_PARSERS (parsers/sqlite.py:89).
func (p *Parser) parseSQLiteAttach() exp.Expression {
	return p.parseSQLiteAttachDetach(true)
}

// parseSQLiteDetach ports the DETACH arm of STATEMENT_PARSERS (parsers/sqlite.py:90).
func (p *Parser) parseSQLiteDetach() exp.Expression {
	return p.parseSQLiteAttachDetach(false)
}

// parseSQLiteAttachDetach ports _parse_attach_detach (parsers/sqlite.py:170-178).
func (p *Parser) parseSQLiteAttachDetach(isAttach bool) exp.Expression {
	p.match(tokens.DATABASE)
	this := p.parseExpression()
	if isAttach {
		return p.expression(exp.Attach(exp.Args{"this": this}), nil, nil)
	}
	return p.expression(exp.Detach(exp.Args{"this": this}), nil, nil)
}

// parseSQLiteModuleProperty ports _parse_module_property (parsers/sqlite.py:144-151):
// `USING <module>([col, ...])`.
func (p *Parser) parseSQLiteModuleProperty() exp.Expression {
	name := p.parseIdVar(true, nil)
	var expressions []exp.Expression
	if p.match(tokens.L_PAREN, false) {
		expressions = p.parseWrappedCsv(func() exp.Expression { return p.parseIdVar(true, nil) })
	}
	return p.expression(exp.ModuleProperty(exp.Args{"this": name, "expressions": expressions}), nil, nil)
}

// parseSQLitePragma ports _parse_pragma (parsers/sqlite.py:153-168): the base form is
// `PRAGMA <expression>`, but SQLite additionally accepts a dotted name and both `= value`
// and `(value)` value forms, with the value parsed as an expression before a var.
func (p *Parser) parseSQLitePragma() exp.Expression {
	name := p.parseVar(true, nil, false)
	if p.match(tokens.DOT) {
		name = p.expression(exp.Dot(exp.Args{"this": name, "expression": p.parseVar(true, nil, false)}), nil, nil)
	}
	p.match(tokens.EQ)
	value := p.parseWrapped(func() exp.Expression {
		if v := p.parseUnary(); v != nil {
			return v
		}
		return p.parseVar(true, nil, false)
	}, true)
	if value != nil {
		return p.expression(exp.Pragma(exp.Args{"this": exp.EQ(exp.Args{"this": name, "expression": value})}), nil, nil)
	}
	return p.expression(exp.Pragma(exp.Args{"this": name}), nil, nil)
}

// sqliteArithmeticTokens ports SQLiteParser.ARITHMETIC_TOKENS (parsers/sqlite.py:63-67):
// Parser.BITWISE ∪ Parser.TERM ∪ Parser.FACTOR, minus COLLATE. A concat-tier expression is
// parenthesized when it appears next to one of these, so its tighter SQLite precedence
// survives regeneration in dialects where the tier binds looser than arithmetic.
var sqliteArithmeticTokens = map[tokens.TokenType]bool{
	tokens.AMP: true, tokens.CARET: true, tokens.PIPE: true,
	tokens.DASH: true, tokens.PLUS: true, tokens.MOD: true,
	tokens.DIV: true, tokens.LR_ARROW: true, tokens.LLRR_ARROW: true,
	tokens.SLASH: true, tokens.STAR: true,
}

// parseSQLiteFactorOperand ports _parse_factor_operand (parsers/sqlite.py:100-123): ||, ->
// and ->> form a single left-associative tier that binds tighter than multiplication.
func (p *Parser) parseSQLiteFactorOperand() exp.Expression {
	inArithmeticOperand := sqliteArithmeticTokens[p.prev.TokenType]
	this := p.parseSQLiteConcatOperand()
	parsedOp := false

	for p.curr.IsValid() {
		op := p.curr.TokenType
		if op != tokens.DPIPE && op != tokens.ARROW && op != tokens.DARROW {
			break
		}
		if op != tokens.DPIPE && this.Kind() == exp.KindDPipe {
			// Parenthesize so that dialects where the arrows bind tighter than || still
			// produce the same left-associative grouping (parsers/sqlite.py:106-109).
			this = p.expression(exp.Paren(exp.Args{"this": this}), nil, nil)
		}
		p.advance()
		operand := p.parseSQLiteConcatOperand()
		switch op {
		case tokens.DPIPE:
			this = p.expression(exp.DPipe(exp.Args{
				"this":       this,
				"expression": operand,
				"safe":       !p.dialect.StrictStringConcat,
			}), nil, nil)
		case tokens.ARROW:
			this = p.expression(exp.JSONExtract(arrowJSONExtractArgs(p, this, operand)), nil, nil)
		case tokens.DARROW:
			this = p.expression(exp.JSONExtractScalar(arrowJSONExtractArgs(p, this, operand)), nil, nil)
		}
		parsedOp = true
	}

	if parsedOp && (inArithmeticOperand || (p.curr.IsValid() && sqliteArithmeticTokens[p.curr.TokenType])) {
		// This tier binds tighter than arithmetic in SQLite but looser elsewhere, so
		// parenthesize it to preserve the grouping in other dialects
		// (parsers/sqlite.py:115-122).
		this = p.expression(exp.Paren(exp.Args{"this": this}), nil, nil)
	}
	return this
}

// parseSQLiteConcatOperand ports _parse_concat_operand (parsers/sqlite.py:125-134): COLLATE
// binds tighter than any binary operator, including the concat tier.
func (p *Parser) parseSQLiteConcatOperand() exp.Expression {
	this := p.parseUnary()
	for p.match(tokens.COLLATE) {
		collate := p.expression(exp.Collate(exp.Args{"this": this, "expression": p.parseUnary()}), nil, nil)
		normalizeCollate(collate)
		this = collate
	}
	return this
}
