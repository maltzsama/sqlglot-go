package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

var bitwiseTokens = map[tokens.TokenType]func(exp.Args) exp.Expression{
	tokens.AMP:   exp.BitwiseAnd,
	tokens.PIPE:  exp.BitwiseOr,
	tokens.CARET: exp.BitwiseXor,
}

// parseBitwise ports _parse_bitwise (parser.py:6064-6095): AMP/PIPE/CARET plus, when
// dialect-enabled, DPIPE-as-concat, DQMARK-as-Coalesce, and the two-token `<<`/`>>` shift
// operators (matchPair, since LT LT / GT GT are two adjacent single-char tokens, not one).
func (p *Parser) parseBitwise() exp.Expression {
	this := p.parseTerm()
	for {
		if constructor, ok := bitwiseTokens[p.curr.TokenType]; ok {
			p.advance()
			this = p.expression(constructor(exp.Args{"this": this, "expression": p.parseTerm()}), nil, nil)
		} else if p.dialect.DPipeIsStringConcat && p.match(tokens.DPIPE) {
			this = p.expression(exp.DPipe(exp.Args{"this": this, "expression": p.parseTerm(), "safe": !p.dialect.StrictStringConcat}), nil, nil)
		} else if p.match(tokens.DQMARK) {
			this = p.expression(exp.Coalesce(exp.Args{"this": this, "expressions": []exp.Expression{p.parseTerm()}}), nil, nil)
		} else if p.matchPair(tokens.LT, tokens.LT, true) {
			this = p.expression(exp.BitwiseLeftShift(exp.Args{"this": this, "expression": p.parseTerm()}), nil, nil)
		} else if p.matchPair(tokens.GT, tokens.GT, true) {
			this = p.expression(exp.BitwiseRightShift(exp.Args{"this": this, "expression": p.parseTerm()}), nil, nil)
		} else if p.dialect.JSONOperatorsAreBinary && jsonOperatorTokens[p.curr.TokenType] {
			// JSON_OPERATORS at the bitwise tier (parser.py:6312-6313, v30.17.0).
			op := columnOperators[p.curr.TokenType]
			p.advance()
			this = op(p, this, p.parseTerm())
		} else {
			break
		}
	}
	return this
}

var termTokens = map[tokens.TokenType]func(exp.Args) exp.Expression{
	tokens.DASH:    exp.Sub,
	tokens.PLUS:    exp.Add,
	tokens.MOD:     exp.Mod,
	tokens.COLLATE: exp.Collate,
}

// parseTerm ports _parse_term (parser.py:6097-6117). The COLLATE post-build step mirrors
// upstream's normalization (parser.py:6107-6115): a single-part column collation operand
// (e.g. `x COLLATE utf8_bin`) becomes a bare Var (or the Identifier itself, if quoted) instead
// of staying a Column, while a qualified one (e.g. `x COLLATE pg_catalog."default"`) is left
// as-is so it still round-trips.
func (p *Parser) parseTerm() exp.Expression {
	this := p.parseFactor()
	for constructor, ok := termTokens[p.curr.TokenType]; ok; constructor, ok = termTokens[p.curr.TokenType] {
		p.advance()
		comments := p.prevComments
		this = p.expression(constructor(exp.Args{"this": this, "expression": p.parseFactor()}), nil, comments)

		if this != nil && this.Kind() == exp.KindCollate {
			normalizeCollate(this)
		}
	}
	return this
}

// normalizeCollate ports the COLLATE post-build step in _parse_term (parser.py:6107-6115):
// a single-part column collation operand (e.g. `x COLLATE utf8_bin`) becomes a bare Var (or
// the Identifier itself, if quoted) instead of staying a Column, while a qualified one
// (e.g. `x COLLATE pg_catalog."default"`) is left as-is so it still round-trips. SQLite's
// concat tier applies the same normalization (parsers/sqlite.py:125-134).
func normalizeCollate(collate exp.Expression) {
	if collate == nil || collate.Kind() != exp.KindCollate {
		return
	}
	expr := collate.Expr()
	if expr == nil || expr.Kind() != exp.KindColumn || len(expr.Parts()) != 1 {
		return
	}
	ident := expr.This()
	if ident == nil || ident.Kind() != exp.KindIdentifier {
		return
	}
	if quoted, _ := ident.Arg("quoted").(bool); quoted {
		collate.Set("expression", ident)
	} else {
		collate.Set("expression", exp.Var(exp.Args{"this": ident.Name()}))
	}
}

var factorTokens = map[tokens.TokenType]func(exp.Args) exp.Expression{
	tokens.SLASH:      exp.Div,
	tokens.STAR:       exp.Mul,
	tokens.DIV:        exp.Div,
	tokens.LR_ARROW:   exp.Distance,
	tokens.LLRR_ARROW: exp.DistanceNd,
}

func (p *Parser) parseFactor() exp.Expression {
	// EXPONENT is empty for base/mysql/postgres, so parse_method is always _parse_unary
	// (parser.py:6119-6121); only the first parse_method() call is wrapped in
	// _parse_at_time_zone, not the ones inside the loop below.
	this := p.parseAtTimeZone(p.parseFactorOperand())
	for constructor, ok := factorTokens[p.curr.TokenType]; ok; constructor, ok = factorTokens[p.curr.TokenType] {
		tokenType := p.curr.TokenType
		p.advance()
		comments := p.prevComments
		args := exp.Args{"this": this, "expression": p.parseFactorOperand()}
		if tokenType == tokens.SLASH || tokenType == tokens.DIV {
			args["typed"] = p.dialect.TypedDivision
			args["safe"] = p.dialect.SafeDivision
		}
		this = p.expression(constructor(args), nil, comments)
	}
	return this
}

// parseFactorOperand ports _parse_factor_operand (parser.py:6365-6376): with a non-empty
// dialect EXPONENT table, `**`/`^` bind tighter than `*`/`/`.
func (p *Parser) parseFactorOperand() exp.Expression {
	if len(p.dialect.ExponentOperators) > 0 {
		return p.parseExponent()
	}
	// parsers/sqlite.py:100-123: sqlite's ||/->/->> tier binds tighter than multiplication.
	if p.dialect.Name == "sqlite" {
		return p.parseSQLiteFactorOperand()
	}
	return p.parseUnary()
}

// parseExponent ports _parse_exponent (parser.py:6368-6376): `while self._match_set(self.
// EXPONENT)` — only the dialect's own table's token types are exponent operators, so
// Teradata's CARET never reaches the Pow constructor here.
func (p *Parser) parseExponent() exp.Expression {
	this := p.parseUnary()
	for p.dialect.ExponentOperators[p.curr.TokenType] {
		comments := p.prevComments
		p.advance()
		this = p.expression(exp.Pow(exp.Args{"this": this, "expression": p.parseUnary()}), nil, comments)
	}
	return this
}

func (p *Parser) parseUnary() exp.Expression {
	if p.match(tokens.DASH) {
		return p.expression(exp.Neg(exp.Args{"this": p.parseUnary()}), nil, p.prevComments)
	}
	if p.match(tokens.PLUS) {
		return p.parseUnary()
	}
	if p.match(tokens.NOT) {
		// Upstream UNARY_PARSERS binds NOT to _parse_equality, not _parse_unary
		// (parser.py:1115), so NOT is lower-precedence than comparison/range ops:
		// `NOT a = b` -> Not(EQ(a, b)), `NOT a IN (..)` -> Not(In(a, ..)).
		return p.expression(exp.Not(exp.Args{"this": p.parseEquality()}), nil, p.prevComments)
	}
	if p.match(tokens.TILDE) {
		return p.expression(exp.BitwiseNot(exp.Args{"this": p.parseUnary()}), nil, p.prevComments)
	}
	if p.dialectFamily("postgres") && p.match(tokens.RLIKE) {
		// Postgres remaps the single-char `~` lexeme from TILDE to RLIKE, since `~` also
		// doubles as the binary REGEXP-LIKE operator there (parsers/postgres.py:185-188);
		// UNARY_PARSERS still builds BitwiseNot when RLIKE appears in prefix position.
		return p.expression(exp.BitwiseNot(exp.Args{"this": p.parseUnary()}), nil, p.prevComments)
	}
	if p.match(tokens.PIPE_SLASH) {
		return p.expression(exp.Sqrt(exp.Args{"this": p.parseUnary()}), nil, p.prevComments)
	}
	if p.match(tokens.DPIPE_SLASH) {
		return p.expression(exp.Cbrt(exp.Args{"this": p.parseUnary()}), nil, p.prevComments)
	}
	return p.parseType(true, false)
}
