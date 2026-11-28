package parser

import (
	"strings"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// parsers/duckdb.py:446-493 DuckDBParser._parse_join: the `JOIN BY (TYPE x)` join-type hint,
// the NEAREST / APPROX NEAREST join modifier, and the table-alias guard that keeps NEAREST
// from being read as an alias (parsers/duckdb.py:437-444).

// duckDBNearestAhead is _parse_table_alias's lookahead (parsers/duckdb.py:438-441) without
// consuming tokens: `NEAREST <number>` or `APPROX NEAREST`.
func (p *Parser) duckDBNearestAhead() bool {
	// _match_text_seq skips quoted identifiers (TEXT_MATCH_EXCLUDED_TOKENS), so "APPROX" is an alias.
	plainWord := func(t tokens.Token, text string) bool {
		return t.IsValid() && !textMatchExcludedTokens[t.TokenType] && stringsUpper(t.Text) == text
	}
	if plainWord(p.curr, "NEAREST") && p.next.TokenType == tokens.NUMBER {
		return true
	}
	return plainWord(p.curr, "APPROX") && plainWord(p.next, "NEAREST")
}

// parseDuckDBJoin ports DuckDBParser._parse_join (parsers/duckdb.py:446-493).
func (p *Parser) parseDuckDBJoin(skipJoinToken bool, parseBracket bool, aliasTokens map[tokens.TokenType]bool) exp.Expression {
	joinType := ""
	if p.matchTextSeq("JOIN", "BY", "(", "TYPE") {
		typeVar := p.parseVar(true, nil, false)
		if typeVar == nil {
			p.raiseError("Expected a join type after JOIN BY (TYPE")
			return nil
		}
		joinType = typeVar.Text("this")
		p.matchRParen(nil)
		skipJoinToken = true
	}

	join := p.parseJoinBase(skipJoinToken, parseBracket, aliasTokens)
	if join == nil {
		return nil
	}

	if joinType != "" {
		parts := strings.Split(strings.TrimSuffix(stringsUpper(joinType), "_JOIN"), "_")
		if parts[0] == "LEFT" || parts[0] == "RIGHT" || parts[0] == "FULL" {
			join.Set("side", parts[0])
			parts = parts[1:]
		}
		if kind := strings.Join(parts, " "); kind != "" {
			join.Set("kind", kind)
		} else {
			join.Set("kind", nil)
		}
	}

	approximate := p.matchTextSeq("APPROX", "NEAREST")
	if approximate || p.matchTextSeq("NEAREST") {
		if join.Text("kind") == "CROSS" {
			join.Set("kind", nil)
		}
		count := p.parseNumber()
		if !p.matchTextSeq("BY") {
			p.raiseError("Expected BY after NEAREST count")
		}
		if !p.matchTexts(map[string]bool{"DISTANCE": true, "SIMILARITY": true}) {
			p.raiseError("Expected DISTANCE or SIMILARITY after BY")
		}
		similarity := stringsUpper(p.prev.Text) == "SIMILARITY"
		join.Set("nearest", p.expression(exp.Nearest(exp.Args{
			"this":        p.parseDisjunction(),
			"count":       count,
			"approximate": approximate,
			"similarity":  similarity,
		}), nil, nil))
	}
	return join
}
