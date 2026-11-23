package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

func (p *Parser) parseRange(this exp.Expression) exp.Expression {
	if this == nil {
		this = p.parseBitwise()
	}
	// mysql _parse_range override (parsers/mysql.py:303-318): `x SOUNDS LIKE y` desugars
	// to EQ(Soundex(x), Soundex(y)) ONCE, before the base range loop — it is not a
	// RANGE_PARSERS entry, so it cannot chain (`a LIKE b SOUNDS LIKE c` errors upstream).
	// SOUNDS_LIKE is mysql-only tokenized. NOT SOUNDS LIKE falls through and errors, as
	// does real MySQL 8.0 (1064).
	if p.curr.TokenType == tokens.SOUNDS_LIKE {
		p.advance()
		this = p.expression(exp.EQ(exp.Args{
			"this":       p.expression(exp.Soundex(exp.Args{"this": this}), nil, nil),
			"expression": p.expression(exp.Soundex(exp.Args{"this": p.parseBitwise()}), nil, nil),
		}), nil, nil)
		// MySQL evaluates = and IS left to right (same precedence): parenthesize before IS.
		if p.curr.TokenType == tokens.IS {
			this = p.expression(exp.Paren(exp.Args{"this": this}), nil, nil)
		}
	}

	// v30.17.0 (parser.py:6052-6082): range operators chain in a loop, each NOT bound to
	// the operator that follows it; a negated link gets parenthesized when another range
	// operator (or NOT) follows so precedence survives regeneration.
	for {
		negate := p.match(tokens.NOT)
		matched := true
		before := this
		switch p.curr.TokenType {
		case tokens.GLOBAL:
			// RANGE_PARSERS[TokenType.GLOBAL] (parsers/clickhouse.py:407-410): ClickHouse
			// `x [NOT] GLOBAL IN (...)` -> In(is_global=True), negated as needed.
			if p.dialect.Name == "clickhouse" {
				p.advance()
				this = p.parseClickhouseGlobalIn(this)
			} else {
				matched = false
			}
		case tokens.BETWEEN:
			p.advance()
			this = p.parseBetween(this)
		case tokens.IN:
			p.advance()
			this = p.parseIn(this, false)
		case tokens.LIKE:
			// negateRange folds a preceding NOT into the Like node's negate arg.
			p.advance()
			this = p.parseEscape(p.expression(exp.Like(exp.Args{"this": this, "expression": p.parseBitwise()}), nil, nil))
		case tokens.ILIKE:
			p.advance()
			this = p.parseEscape(p.expression(exp.ILike(exp.Args{"this": this, "expression": p.parseBitwise()}), nil, nil))
		case tokens.SIMILAR_TO:
			// binary_range_parser(exp.SimilarTo) (parser.py:62-71): unlike LIKE/ILIKE above,
			// SimilarTo has no "negate" arg, so `negate` stays set and the generic
			// `Not`-wrapping below (_negate_range's fallback) applies to `NOT ... SIMILAR TO`.
			p.advance()
			this = p.parseEscape(p.expression(exp.SimilarTo(exp.Args{"this": this, "expression": p.parseBitwise()}), nil, nil))
		case tokens.ISNULL:
			p.advance()
			this = p.expression(exp.Is(exp.Args{"this": this, "expression": exp.Null()}), nil, nil)
		case tokens.GLOB:
			// binary_range_parser(exp.Glob) (parser.py:1188).
			p.advance()
			this = p.parseEscape(p.expression(exp.Glob(exp.Args{"this": this, "expression": p.parseBitwise()}), nil, nil))
		case tokens.MATCH:
			// parsers/sqlite.py:94-98 RANGE_PARSERS[MATCH] = binary_range_parser(exp.Match).
			if p.dialect.Name == "sqlite" {
				p.advance()
				this = p.expression(exp.Match(exp.Args{"this": this, "expression": p.parseBitwise()}), nil, nil)
			} else {
				matched = false
			}
		case tokens.TILDE:
			// parsers/duckdb.py:222: RANGE_PARSERS[TILDE] = binary_range_parser(RegexpFullMatch).
			if p.dialect.RegexpRangeOperators {
				p.advance()
				this = p.expression(exp.RegexpFullMatch(exp.Args{"this": this, "expression": p.parseBitwise()}), nil, nil)
			} else {
				matched = false
			}
		case tokens.CARET_AT:
			// parsers/duckdb.py:224: RANGE_PARSERS[CARET_AT] = binary_range_parser(StartsWith).
			if p.dialect.CaretAtStartsWith {
				p.advance()
				this = p.expression(exp.StartsWith(exp.Args{"this": this, "expression": p.parseBitwise()}), nil, nil)
			} else {
				matched = false
			}
		case tokens.OVERLAPS:
			// binary_range_parser(exp.Overlaps) (parser.py:1195). BigQuery removes OVERLAPS
			// from RANGE_PARSERS (parsers/bigquery.py:368-370), so it stays an alias.
			if p.dialect.Name == "bigquery" {
				matched = false
				break
			}
			p.advance()
			this = p.parseEscape(p.expression(exp.Overlaps(exp.Args{"this": this, "expression": p.parseBitwise()}), nil, nil))
		case tokens.RLIKE:
			// binary_range_parser(exp.RegexpLike) (parser.py:1196). RLIKE is only ever
			// tokenized by dialects that remap it (e.g. postgres' bare `~`), so this case is
			// effectively dialect-gated at the tokenizer without needing a check here.
			p.advance()
			args := exp.Args{"this": this, "expression": p.parseBitwise()}
			if p.dialect.RlikeFullMatch {
				args["full_match"] = true
			}
			this = p.parseEscape(p.expression(exp.RegexpLike(args), nil, nil))
		case tokens.IRLIKE:
			// binary_range_parser(exp.RegexpILike) (parser.py:1191): base `~*` keyword.
			p.advance()
			this = p.parseEscape(p.expression(exp.RegexpILike(exp.Args{"this": this, "expression": p.parseBitwise()}), nil, nil))
		case tokens.AT_GT:
			// binary_range_parser(exp.ArrayContainsAll) (parser.py:1186): postgres `@>`.
			p.advance()
			this = p.parseEscape(p.expression(exp.ArrayContainsAll(exp.Args{"this": this, "expression": p.parseBitwise()}), nil, nil))
		case tokens.LT_AT:
			// binary_range_parser(exp.ArrayContainedBy) (parser.py:1194): postgres `<@`.
			p.advance()
			this = p.parseEscape(p.expression(exp.ArrayContainedBy(exp.Args{"this": this, "expression": p.parseBitwise()}), nil, nil))
		case tokens.QMARK_AMP:
			// binary_range_parser(exp.JSONBContainsAllTopKeys) (parser.py:1199): postgres `?&`.
			p.advance()
			this = p.parseEscape(p.expression(exp.JSONBContainsAllTopKeys(exp.Args{"this": this, "expression": p.parseBitwise()}), nil, nil))
		case tokens.QMARK_PIPE:
			// binary_range_parser(exp.JSONBContainsAnyTopKeys) (parser.py:1200): postgres `?|`.
			p.advance()
			this = p.parseEscape(p.expression(exp.JSONBContainsAnyTopKeys(exp.Args{"this": this, "expression": p.parseBitwise()}), nil, nil))
		case tokens.HASH_DASH:
			// binary_range_parser(exp.JSONBDeleteAtPath) (parser.py:1201): postgres `#-`.
			p.advance()
			this = p.parseEscape(p.expression(exp.JSONBDeleteAtPath(exp.Args{"this": this, "expression": p.parseBitwise()}), nil, nil))
		case tokens.AT_QMARK:
			// binary_range_parser(exp.JSONBPathExists) (parser.py:1202): postgres `@?`.
			p.advance()
			this = p.parseEscape(p.expression(exp.JSONBPathExists(exp.Args{"this": this, "expression": p.parseBitwise()}), nil, nil))
		case tokens.AMP_LT:
			// binary_range_parser(exp.ExtendsLeft) (parser.py:1205, v30.17.0): postgres `&<`.
			p.advance()
			this = p.parseEscape(p.expression(exp.ExtendsLeft(exp.Args{"this": this, "expression": p.parseBitwise()}), nil, nil))
		case tokens.AMP_GT:
			// binary_range_parser(exp.ExtendsRight) (parser.py:1206, v30.17.0): postgres `&>`.
			p.advance()
			this = p.parseEscape(p.expression(exp.ExtendsRight(exp.Args{"this": this, "expression": p.parseBitwise()}), nil, nil))
		case tokens.ADJACENT:
			// binary_range_parser(exp.Adjacent) (parser.py:1203): postgres range `-|-`.
			p.advance()
			this = p.parseEscape(p.expression(exp.Adjacent(exp.Args{"this": this, "expression": p.parseBitwise()}), nil, nil))
		case tokens.DAMP:
			// postgres-only: parsers/postgres.py RANGE_PARSERS adds
			// TokenType.DAMP: binary_range_parser(exp.ArrayOverlaps) on top of the base
			// Parser.RANGE_PARSERS. mysql's `&&` (TokenType.DAMP -> exp.And) is a
			// CONJUNCTION-level entry instead, owned by parseConjunction; base has no `&&`
			// support upstream either, so this case does nothing (leaves DAMP unconsumed)
			// for every other dialect.
			if p.dialectFamily("postgres") || p.dialect.ArrayOverlapsDAMP {
				p.advance()
				this = p.parseEscape(p.expression(exp.ArrayOverlaps(exp.Args{"this": this, "expression": p.parseBitwise()}), nil, nil))
			} else {
				matched = false
			}
		case tokens.DCOLON:
			// parsers/tsql.py:417-423: T-SQL RANGE_PARSERS[DCOLON] wraps the result of
			// _parse_function() or _parse_var(any_token=True) in ScopeResolution; `this` may
			// be nil for a statement-leading `::FOO(...)`, matching upstream reaching the
			// range loop with this=None after _parse_bitwise returns nothing. Postfix `x::y`
			// is consumed earlier by parseColumnOps, so this arm only sees the leading form.
			// Every other dialect leaves DCOLON to the column-operator/cast path.
			if p.dialect.Name == "tsql" {
				p.advance()
				expr := p.parseFunction(nil, false, false, false)
				if expr == nil {
					expr = p.parseVar(true, nil, false)
				}
				this = p.expression(exp.ScopeResolution(exp.Args{"this": this, "expression": expr}), nil, nil)
			} else {
				matched = false
			}
		case tokens.DAT:
			// parsers/postgres.py RANGE_PARSERS[TokenType.DAT]: postgres `x @@ y` full-text
			// match, built as exp.MatchAgainst(this=<rhs>, expressions=[<lhs>]) - not a
			// dedicated JSONB-path-match node (v30.12.0 has no exp.JSONBPathMatch). DAT is
			// only ever tokenized by postgres' `"@@": DAT` keyword remap, so (like DAMP's
			// non-postgres branches) this case is unreachable for other dialects.
			p.advance()
			this = p.expression(exp.MatchAgainst(exp.Args{"this": p.parseBitwise(), "expressions": []exp.Expression{this}}), nil, nil)
		case tokens.OPERATOR:
			// _parse_operator (parser.py:10122-10139): postgres `x OPERATOR(schema.op) y`,
			// chainable (`x OPERATOR(op1) y OPERATOR(op2) z`).
			p.advance()
			for p.match(tokens.L_PAREN) {
				op := ""
				for p.curr.IsValid() && !p.match(tokens.R_PAREN) {
					op += p.curr.Text
					p.advance()
				}
				comments := p.prevComments
				this = p.expression(exp.Operator(exp.Args{"this": this, "operator": op, "expression": p.parseBitwise()}), nil, comments)
				if !p.match(tokens.OPERATOR) {
					break
				}
			}
		case tokens.MEMBER_OF:
			// mysql RANGE_PARSERS[TokenType.MEMBER_OF] (parsers/mysql.py:97-99): `x MEMBER
			// OF(y)` -> exp.JSONArrayContains(this=x, expression=y) (no dedicated exp.MemberOf
			// class upstream). MEMBER_OF is mysql-only tokenized, so this case is unreachable
			// elsewhere.
			p.advance()
			this = p.expression(exp.JSONArrayContains(exp.Args{"this": this, "expression": p.parseWrapped(p.parseExpression, false)}), nil, nil)
		case tokens.IS:
			// RANGE_PARSERS[TokenType.IS] = lambda self, this: self._parse_is(this)
			// (parser.py:1192): matched here in addition to the unconditional `if
			// self._match(TokenType.IS)` check below (parser.py:5880-5881) so a chained
			// `x IS TRUE IS TRUE` gets two chances to consume an IS - parseIs's own trailing
			// parseColumnOps call doesn't re-enter IS itself.
			p.advance()
			this = p.parseIs(this)
		default:
			if negate && p.curr.TokenType == tokens.NULL {
				p.advance()
				this = p.expression(exp.Is(exp.Args{"this": this, "expression": exp.Null()}), nil, nil)
			} else if p.curr.TokenType == tokens.NOTNULL {
				// Postgres ISNULL/NOTNULL postfix predicates. NORMALIZE_NOT_NULL
				// (dialect.py:757; postgres.py:16 overrides False): postgres keeps
				// Is(negate=True) so `deleted NOTNULL` renders `deleted IS NOT NULL`.
				p.advance()
				if p.dialect.NormalizeNotNull {
					this = p.expression(exp.Is(exp.Args{"this": this, "expression": exp.Null()}), nil, nil)
					this = p.expression(exp.Not(exp.Args{"this": this}), nil, nil)
				} else {
					this = p.expression(exp.Is(exp.Args{"this": this, "expression": exp.Null(), "negate": true}), nil, nil)
				}
			} else {
				matched = false
			}
		}
		if !matched {
			if negate {
				p.retreat(p.index - 1)
			}
			break
		}
		// A matched arm whose sub-parse failed (parseIs on a bare `x IS`) returns nil;
		// upstream's `if not expression: return this` exits with the pre-arm value —
		// looping again would spin forever on the unconsumed token.
		if this == nil {
			return before
		}
		if negate {
			this = p.negateRange(this)
			if p.curr.IsValid() && (p.curr.TokenType == tokens.NOT || p.isRangeChainToken(p.curr.TokenType)) {
				this = p.expression(exp.Paren(exp.Args{"this": this}), nil, nil)
			}
		}
	}
	return this
}

// rangeChainTokens mirrors RANGE_PARSERS' key set for the chain-parenthesization check
// (parser.py:6076-6080); the Go port dispatches ranges via the switch in parseRange.
var rangeChainTokens = map[tokens.TokenType]bool{
	tokens.AT_GT: true, tokens.BETWEEN: true, tokens.GLOB: true, tokens.ILIKE: true,
	tokens.IN: true, tokens.IRLIKE: true, tokens.IS: true, tokens.LIKE: true,
	tokens.LT_AT: true, tokens.OVERLAPS: true, tokens.RLIKE: true, tokens.SIMILAR_TO: true,
	tokens.QMARK_AMP: true, tokens.QMARK_PIPE: true, tokens.HASH_DASH: true,
	tokens.AT_QMARK: true, tokens.ADJACENT: true, tokens.OPERATOR: true,
	tokens.AMP_LT: true, tokens.AMP_GT: true,
}

// dialectRangeChainTokens are range tokens only in one dialect's RANGE_PARSERS: postgres
// `&&`/`@@`, mysql MEMBER OF. Elsewhere they are not range operators (mysql `&&` is a
// conjunction), so the chain-parenthesization check must ignore them.
func (p *Parser) isRangeChainToken(tt tokens.TokenType) bool {
	if rangeChainTokens[tt] {
		return true
	}
	switch tt {
	case tokens.DAMP, tokens.DAT:
		return p.dialectFamily("postgres")
	case tokens.MEMBER_OF:
		return p.dialectIs("mysql")
	case tokens.MATCH:
		return p.dialect.Name == "sqlite"
	}
	return false
}

// negateRange ports _negate_range (parser.py:6019-6029): LIKE/ILIKE absorb the NOT as
// their negate arg (through an Escape wrapper); everything else wraps in Not.
func (p *Parser) negateRange(this exp.Expression) exp.Expression {
	if this == nil {
		return this
	}
	expression := this
	if this.Kind() == exp.KindEscape {
		expression = this.This()
	}
	if expression != nil && (expression.Kind() == exp.KindLike || expression.Kind() == exp.KindILike) {
		expression.Set("negate", true)
		return this
	}
	return p.expression(exp.Not(exp.Args{"this": this}), nil, nil)
}

// parseEscape ports _parse_escape (parser.py:5966-5971): `<this> ESCAPE <string|NULL>`.
func (p *Parser) parseEscape(this exp.Expression) exp.Expression {
	if !p.match(tokens.ESCAPE) {
		return this
	}
	expression := p.parseString()
	if expression == nil {
		expression = p.parseNull()
	}
	return p.expression(exp.Escape(exp.Args{"this": this, "expression": expression}), nil, nil)
}

// isJSONPredicateKind is IS_JSON_PREDICATE_KIND (parser.py:1707): the optional kind word
// following `IS JSON` - `x IS JSON [VALUE|SCALAR|ARRAY|OBJECT] ...`.
var isJSONPredicateKind = map[string]bool{"VALUE": true, "SCALAR": true, "ARRAY": true, "OBJECT": true}

func (p *Parser) parseIs(this exp.Expression) exp.Expression {
	index := p.index - 1
	negate := p.match(tokens.NOT)
	if p.matchTextSeq("DISTINCT", "FROM") {
		constructor := exp.NullSafeNEQ
		if negate {
			constructor = exp.NullSafeEQ
		}
		return p.expression(constructor(exp.Args{"this": this, "expression": p.parseBitwise()}), nil, nil)
	}
	var expression exp.Expression
	if p.match(tokens.JSON) {
		// `x IS JSON [VALUE|SCALAR|ARRAY|OBJECT] [WITH|WITHOUT] [UNIQUE [KEYS]]`
		// (parser.py:5904-5916).
		var kind any
		if p.matchTexts(isJSONPredicateKind) {
			kind = stringsUpper(p.prev.Text)
		}
		var with_ any
		if p.matchTextSeq("WITH") {
			with_ = true
		} else if p.matchTextSeq("WITHOUT") {
			with_ = false
		}
		unique := p.match(tokens.UNIQUE)
		p.matchTextSeq("KEYS")
		expression = p.expression(exp.JSON(exp.Args{"this": kind, "with_": with_, "unique": unique}), nil, nil)
	} else {
		expression = p.parseNull()
		if expression == nil {
			expression = p.parseBoolean()
		}
		if expression == nil {
			expression = p.parseBitwise()
		}
		if expression == nil {
			p.retreat(index)
			return nil
		}
	}
	// v30.17.0 (parser.py:6125-6130): under !NORMALIZE_NOT_NULL, `x IS NOT NULL` keeps
	// Is(negate=True) rather than normalizing to NOT(Is(...)).
	if negate && expression != nil && expression.Kind() == exp.KindNull && !p.dialect.NormalizeNotNull {
		this = p.expression(exp.Is(exp.Args{"this": this, "expression": expression, "negate": true}), nil, nil)
	} else {
		this = p.expression(exp.Is(exp.Args{"this": this, "expression": expression}), nil, nil)
		if negate {
			this = p.expression(exp.Not(exp.Args{"this": this}), nil, nil)
		}
	}
	return p.parseColumnOps(this)
}

func (p *Parser) parseIn(this exp.Expression, alias bool) exp.Expression {
	if unnest := p.parseUnnest(false); unnest != nil {
		return p.expression(exp.In(exp.Args{"this": this, "unnest": unnest}), nil, nil)
	}
	// parser.py:6137-6151: IN_PARTS accepts either parens or brackets.
	if p.match(tokens.L_PAREN) || p.match(tokens.L_BRACKET) {
		matchedLParen := p.prev.TokenType == tokens.L_PAREN
		expressions := p.parseCsv(func() exp.Expression { return p.parseSelectOrExpression(alias) })
		if len(expressions) == 1 && expressions[0] != nil && expressions[0].Is(exp.TraitQuery) {
			query := p.parseQueryModifiers(expressions[0])
			this = p.expression(exp.In(exp.Args{"this": this, "query": exp.Subquery(exp.Args{"this": query})}), nil, nil)
		} else {
			this = p.expression(exp.In(exp.Args{"this": this, "expressions": expressions}), nil, nil)
		}
		if matchedLParen {
			p.matchRParen(this)
		} else if !p.match(tokens.R_BRACKET) {
			p.raiseError("Expecting ]")
		}
	} else {
		this = p.expression(exp.In(exp.Args{"this": this, "field": p.parseColumn()}), nil, nil)
	}
	return this
}

func (p *Parser) parseBetween(this exp.Expression) exp.Expression {
	var symmetric any
	if p.matchTextSeq("SYMMETRIC") {
		symmetric = true
	} else if p.matchTextSeq("ASYMMETRIC") {
		symmetric = false
	}
	low := p.parseBitwise()
	p.match(tokens.AND)
	high := p.parseBitwise()
	return p.expression(exp.Between(exp.Args{"this": this, "low": low, "high": high, "symmetric": symmetric}), nil, nil)
}
