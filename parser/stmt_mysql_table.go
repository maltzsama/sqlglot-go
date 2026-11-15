package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// parseTableStatement models `TABLE tbl` — the table value constructor that is exactly `SELECT *
// FROM tbl` — as a real Select so a consumer gets the same column lineage it would for the SELECT
// form. The generic expression path mis-parses the leading TABLE keyword as an Alias. Dispatched by
// the leading TABLE token at statement start, so CREATE/DROP/ALTER TABLE (their own statement
// parsers) and a TABLE keyword inside a query block are untouched.
//
// Scope: only the bare `TABLE tbl_name` (optionally schema/catalog-qualified) is modeled. The
// operand must be a plain table identifier — a table function (`TABLE f()`), placeholder (`TABLE ?`)
// or other non-identifier is rejected here (retreat + fall through) so it fails closed under the
// default IMMEDIATE parser rather than faking a Select. Real MySQL also permits `TABLE t ORDER BY`,
// `LIMIT`, set operations (`TABLE a UNION TABLE b`), a parenthesized/subquery `TABLE`, and
// `INSERT … TABLE t`; none of those query-block positions is modeled — the top-level trailer forms
// are left unconsumed and fail closed, while a parenthesized `(TABLE t)` inside another query keeps
// upstream's wrong Alias. The schema-qualified spelling `TABLE db.users` is grammar beyond pinned
// upstream (it parse-errors at the dot), tracked in testdata/upstream_extensions.jsonl; the
// unqualified `TABLE users` is the DEVIATIONS §1.15 correctness fix. See DEVIATIONS §1.15.
func (p *Parser) parseTableStatement() exp.Expression {
	// The dialects whose grammar defines the TABLE value constructor: MySQL 8.0.19+ (and
	// MariaDB through the mysql chain) and Trino's `TABLE table_name` query. Elsewhere the
	// leading TABLE keyword keeps upstream's Alias mis-parse so the port stays faithful.
	if !p.dialectIs("mysql") && !p.dialectIs("trino") {
		return nil
	}
	if p.curr.TokenType != tokens.TABLE {
		return nil
	}
	comments := p.curr.Comments
	start := p.index
	p.advance() // consume TABLE
	table := p.parseTableParts(false, false, false, false)
	// parseTableParts always returns an exp.Table wrapper (it raises rather than returning nil on a
	// missing name under the default IMMEDIATE parser). Require its `this` to be a plain Identifier:
	// this rejects table functions/placeholders, and also the lenient-parser case where a missing
	// name leaves `this` nil — retreat so the statement fails closed instead of yielding a Select.
	if this := asExpr(table.Arg("this")); this == nil || this.Kind() != exp.KindIdentifier || table.Arg("pivots") != nil {
		p.retreat(start)
		return nil
	}
	from := p.expression(exp.From(exp.Args{"this": table}), nil, nil)
	return p.expression(exp.Select(exp.Args{
		"expressions": []exp.Expression{exp.Star(nil)},
		"from_":       from,
	}), nil, comments)
}
