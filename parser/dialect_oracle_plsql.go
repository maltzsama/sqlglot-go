package parser

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// Oracle PL/SQL unit boundaries (DEVIATIONS §1.24).
//
// Beyond upstream: pinned sqlglot splits every input on `;`, so a PL/SQL unit — a stored
// `CREATE [OR REPLACE] {PROCEDURE|FUNCTION|PACKAGE [BODY]|TRIGGER|TYPE BODY}` or an anonymous
// `[DECLARE ...] BEGIN ... END;` block — is cut at the first `;` inside its body. The result is a
// truncated statement plus a stray `END`, or a parse error for anonymous blocks. A consumer that
// executes or classifies statements one by one would then send a truncated CREATE to the
// database, and the body's statements would surface as top-level statements.
//
// Under oracle, splitOracleChunks keeps a unit in one chunk: it tracks the unit's block nesting
// and only ends the unit at a `;` once that nesting is closed again (or at a `/` terminator line,
// §1.23). The body is not modeled; parseOraclePLSQLUnit keeps it verbatim, the same way the MySQL
// account-object statements do (stmt_account_object.go):
//
//   - a stored unit becomes a Create root whose `kind` names the unit ("PROCEDURE", "PACKAGE
//     BODY", ...) and whose `this` is a Command holding the verbatim statement text;
//   - an anonymous block becomes a Command led by its BEGIN/DECLARE keyword.
//
// A unit whose nesting is still open at the end of input, or at a `/` line, fails closed with a
// parse error instead of becoming a truncated statement.

// plsqlUnit describes a chunk that holds one PL/SQL unit.
type plsqlUnit struct {
	// kind is the Create kind of a stored unit ("PROCEDURE", "PACKAGE BODY", ...), or "" for
	// an anonymous block.
	kind string
	// terminated reports whether the unit's block nesting was closed before the unit ended.
	terminated bool
}

// plsqlWord is the uppercased keyword text of tok, or "" for a quoted identifier or a string
// literal, which never act as PL/SQL keywords.
func plsqlWord(tok tokens.Token) string {
	if tok.TokenType == tokens.IDENTIFIER || tok.TokenType == tokens.STRING {
		return ""
	}
	return stringsUpper(tok.Text)
}

// plsqlUnitStart reports whether toks (a statement's tokens, from its first one) open a PL/SQL
// unit, and if so the Create kind of a stored unit ("" for an anonymous block).
func plsqlUnitStart(toks []tokens.Token) (kind string, ok bool) {
	word := func(i int) string {
		if i < len(toks) {
			return plsqlWord(toks[i])
		}
		return ""
	}
	switch word(0) {
	case "BEGIN", "DECLARE":
		return "", true
	case "CREATE":
	default:
		return "", false
	}
	i := 1
	if word(i) == "OR" && word(i+1) == "REPLACE" {
		i += 2
	}
	if word(i) == "EDITIONABLE" || word(i) == "NONEDITIONABLE" {
		i++
	}
	switch word(i) {
	case "PROCEDURE", "FUNCTION", "TRIGGER":
		return word(i), true
	case "PACKAGE":
		if word(i+1) == "BODY" {
			return "PACKAGE BODY", true
		}
		return "PACKAGE", true
	case "TYPE":
		if word(i+1) == "BODY" {
			return "TYPE BODY", true
		}
	}
	return "", false
}

// plsqlFrame is one open level of PL/SQL block nesting.
type plsqlFrame uint8

const (
	// plsqlDeclare is a declaration section (DECLARE, or a subprogram/package `IS|AS`): its
	// BEGIN continues the same level instead of opening a new one.
	plsqlDeclare plsqlFrame = iota
	// plsqlCompound is a compound trigger body: each timing-point BEGIN opens its own level.
	plsqlCompound
	// plsqlBlock is a BEGIN block or a CASE/IF/LOOP construct, closed by its END.
	plsqlBlock
)

// plsqlScanner tracks the block nesting of one PL/SQL unit, token by token.
type plsqlScanner struct {
	stack []plsqlFrame
	// parens is the parenthesis depth; a subprogram's IS|AS only counts outside parameters.
	parens int
	// subprogram is set between a PROCEDURE/FUNCTION/PACKAGE/TYPE BODY heading and its IS|AS
	// (or the `;` of a forward declaration).
	subprogram bool
	// afterEnd is set on the token right after an END, so `END IF`/`END LOOP`/`END CASE` do
	// not reopen what they close.
	afterEnd bool
	// broken records a stray END with nothing open.
	broken   bool
	prevWord string
}

// open reports whether the unit still has unclosed nesting.
func (s *plsqlScanner) open() bool { return len(s.stack) > 0 }

// feed advances the scanner over toks[i] (toks[i+1] is used as lookahead).
func (s *plsqlScanner) feed(toks []tokens.Token, i int) {
	tok := toks[i]
	word := plsqlWord(tok)
	afterEnd := s.afterEnd
	s.afterEnd = false
	defer func() { s.prevWord = word }()

	switch tok.TokenType {
	case tokens.L_PAREN:
		s.parens++
		return
	case tokens.R_PAREN:
		if s.parens > 0 {
			s.parens--
		}
		return
	case tokens.SEMICOLON:
		// A forward declaration (`PROCEDURE p;`) has no IS|AS body.
		s.subprogram = false
		return
	}
	if word == "" || s.parens > 0 {
		return
	}
	if afterEnd && (word == "IF" || word == "LOOP" || word == "CASE") {
		return
	}
	switch word {
	case "PROCEDURE", "FUNCTION", "PACKAGE":
		s.subprogram = true
	case "BODY":
		if s.prevWord == "TYPE" {
			s.subprogram = true
		}
	case "IS", "AS":
		if !s.subprogram {
			return
		}
		s.subprogram = false
		// A call specification (`AS LANGUAGE ...` / `AS EXTERNAL ...`) has no PL/SQL body.
		if i+1 < len(toks) {
			if next := plsqlWord(toks[i+1]); next == "LANGUAGE" || next == "EXTERNAL" {
				return
			}
		}
		s.stack = append(s.stack, plsqlDeclare)
	case "COMPOUND":
		if i+1 < len(toks) && plsqlWord(toks[i+1]) == "TRIGGER" {
			s.stack = append(s.stack, plsqlCompound)
		}
	case "DECLARE":
		s.stack = append(s.stack, plsqlDeclare)
	case "BEGIN":
		if top := len(s.stack) - 1; top >= 0 && s.stack[top] == plsqlDeclare {
			s.stack[top] = plsqlBlock
			return
		}
		s.stack = append(s.stack, plsqlBlock)
	case "CASE", "IF", "LOOP":
		s.stack = append(s.stack, plsqlBlock)
	case "END":
		if len(s.stack) == 0 {
			s.broken = true
			return
		}
		s.stack = s.stack[:len(s.stack)-1]
		s.afterEnd = true
	}
}

// splitOracleChunks is parse's `;` chunking with PL/SQL units kept whole: inside a unit, a `;`
// only ends the chunk once the unit's nesting is closed, while a `/` terminator line (§1.23)
// always ends it. Chunk-level behavior otherwise matches the generic loop in parse. It returns
// the chunks plus the units, keyed by chunk index.
func splitOracleChunks(rawTokens []tokens.Token) ([][]tokens.Token, map[int]plsqlUnit) {
	total := len(rawTokens)
	chunks := [][]tokens.Token{{}}
	var units map[int]plsqlUnit
	var scanner *plsqlScanner
	finishUnit := func(chunk int) {
		unit := units[chunk]
		unit.terminated = !scanner.open() && !scanner.broken
		units[chunk] = unit
		scanner = nil
	}
	for i, token := range rawTokens {
		current := len(chunks) - 1
		if scanner == nil && len(chunks[current]) == 0 {
			if kind, ok := plsqlUnitStart(rawTokens[i:]); ok {
				if units == nil {
					units = map[int]plsqlUnit{}
				}
				units[current] = plsqlUnit{kind: kind}
				scanner = &plsqlScanner{}
			}
		}
		if scanner != nil {
			terminator := token.TokenType == tokens.SEMICOLON && token.Text != ";"
			if token.TokenType == tokens.SEMICOLON && (terminator || !scanner.open()) {
				finishUnit(current)
			} else {
				scanner.feed(rawTokens, i)
				chunks[current] = append(chunks[current], token)
				continue
			}
		}
		if token.TokenType == tokens.SEMICOLON {
			if len(token.Comments) > 0 {
				chunks = append(chunks, []tokens.Token{token})
			}
			if i < total-1 {
				chunks = append(chunks, []tokens.Token{})
			}
		} else {
			chunks[current] = append(chunks[current], token)
		}
	}
	if scanner != nil {
		finishUnit(len(chunks) - 1)
	}
	return chunks, units
}

// parseOraclePLSQLUnit consumes the current chunk, which splitOracleChunks marked as a PL/SQL
// unit, and builds its verbatim node (see the file comment).
func (p *Parser) parseOraclePLSQLUnit(unit plsqlUnit) exp.Expression {
	start := p.curr
	if !unit.terminated {
		p.raiseError("Unterminated PL/SQL block", start)
		p.checkErrors()
		return nil
	}
	for p.curr.IsValid() {
		p.advance()
	}
	text := p.findSQL(start, p.prev)
	var node exp.Expression
	if unit.kind != "" {
		// Built bare like the account-object body: the verbatim text already carries any inline
		// comments; the statement's leading comments attach to the root below.
		body := exp.Command(exp.Args{"this": text})
		node = p.expression(exp.New(exp.KindCreate, exp.Args{"kind": unit.kind, "this": body}), nil, nil)
	} else {
		runes := []rune(text)
		size := len([]rune(start.Text))
		node = p.expression(exp.Command(exp.Args{"this": string(runes[:size]), "expression": string(runes[size:])}), nil, nil)
	}
	node.AddComments(start.Comments, true)
	return node
}
