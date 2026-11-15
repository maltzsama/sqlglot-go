// Package jsonpath ports sqlglot/jsonpath.py (pinned v30.17.0): the JSONPath tokenizer and
// parser that turn a JSON path string into an exp.JSONPath AST.
package jsonpath

import (
	"fmt"
	"strings"
	"unicode"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// Config is a dialect's JSONPathTokenizer surface (jsonpath.py:15-45 plus the per-dialect
// VAR_TOKENS/IDENTIFIERS/SINGLE_TOKENS overrides).
type Config struct {
	// SingleTokens overrides the base single-character map when non-nil. Snowflake removes
	// "$" from it (dialects/snowflake.py:130-132).
	SingleTokens map[rune]tokens.TokenType
	// VarTokens holds the token types _parse_var_text merges; nil means {VAR}. BigQuery adds
	// DASH and NUMBER, hive adds DASH.
	VarTokens map[tokens.TokenType]bool
	// IdentifierDelims overrides the identifier delimiters; nil means {'"'}. Databricks adds
	// the backtick.
	IdentifierDelims map[rune]bool
	// SingleDotIsWildcard mirrors Dialect.JSON_PATH_SINGLE_DOT_IS_WILDCARD: when true a bare
	// `.` with no key is not an error.
	SingleDotIsWildcard bool
}

// baseSingleTokens ports JSONPathTokenizer.SINGLE_TOKENS (jsonpath.py:16-31). The quote
// characters are handled by the string/identifier scanner before this table is consulted.
var baseSingleTokens = map[rune]tokens.TokenType{
	'(': tokens.L_PAREN,
	')': tokens.R_PAREN,
	'[': tokens.L_BRACKET,
	']': tokens.R_BRACKET,
	':': tokens.COLON,
	',': tokens.COMMA,
	'-': tokens.DASH,
	'.': tokens.DOT,
	'?': tokens.PLACEHOLDER,
	'@': tokens.PARAMETER,
	'$': tokens.DOLLAR,
	'*': tokens.STAR,
}

var defaultVarTokens = map[tokens.TokenType]bool{tokens.VAR: true}

var defaultIdentifierDelims = map[rune]bool{'"': true}

// token mirrors sqlglot.tokens.Token for the fields the JSONPath parser reads: Start/End are
// inclusive rune offsets into the path.
type token struct {
	typ   tokens.TokenType
	text  string
	start int
	end   int
}

// tokenize ports JSONPathTokenizer.tokenize: the JSONPath character set is tiny, so the
// lexer is hand-written rather than routed through tokens.Tokenizer.
func tokenize(path string, cfg Config) ([]token, error) {
	single := baseSingleTokens
	if cfg.SingleTokens != nil {
		single = cfg.SingleTokens
	}
	identifiers := defaultIdentifierDelims
	if cfg.IdentifierDelims != nil {
		identifiers = cfg.IdentifierDelims
	}

	runes := []rune(path)
	out := make([]token, 0, len(runes))
	i := 0
	for i < len(runes) {
		r := runes[i]
		if unicode.IsSpace(r) {
			i++
			continue
		}

		// KEYWORDS {"..": DOT} (jsonpath.py:33-35): checked before the single-character dot.
		if r == '.' && i+1 < len(runes) && runes[i+1] == '.' {
			out = append(out, token{tokens.DOT, "..", i, i + 1})
			i += 2
			continue
		}

		// QUOTES ['"'] / STRING_ESCAPES ["\\"]: a single-quoted run is a STRING.
		if r == '\'' {
			text, end, err := scanDelimited(runes, i, '\'')
			if err != nil {
				return nil, err
			}
			out = append(out, token{tokens.STRING, text, i, end})
			i = end + 1
			continue
		}

		// IDENTIFIERS ['"'] (plus the backtick for databricks): an identifier run.
		if identifiers[r] {
			text, end, err := scanDelimited(runes, i, r)
			if err != nil {
				return nil, err
			}
			out = append(out, token{tokens.IDENTIFIER, text, i, end})
			i = end + 1
			continue
		}

		if typ, ok := single[r]; ok {
			out = append(out, token{typ, string(r), i, i})
			i++
			continue
		}

		if r >= '0' && r <= '9' {
			j := i
			for j < len(runes) && runes[j] >= '0' && runes[j] <= '9' {
				j++
			}
			out = append(out, token{tokens.NUMBER, string(runes[i:j]), i, j - 1})
			i = j
			continue
		}

		// VAR run: until whitespace, a single-token character, or a quote/identifier delimiter.
		j := i
		for j < len(runes) {
			c := runes[j]
			if unicode.IsSpace(c) || c == '\'' || identifiers[c] {
				break
			}
			if _, ok := single[c]; ok {
				break
			}
			j++
		}
		if j == i { // defensive: consume one rune rather than loop forever
			j = i + 1
		}
		out = append(out, token{tokens.VAR, string(runes[i:j]), i, j - 1})
		i = j
	}
	return out, nil
}

// scanDelimited scans a quote-delimited run with backslash escapes (IDENTIFIER_ESCAPES /
// STRING_ESCAPES are both ["\\"]) and returns its unescaped text plus the closing quote index.
func scanDelimited(runes []rune, start int, delim rune) (string, int, error) {
	var b strings.Builder
	i := start + 1
	for i < len(runes) {
		c := runes[i]
		if c == '\\' && i+1 < len(runes) {
			b.WriteRune(runes[i+1])
			i += 2
			continue
		}
		if c == delim {
			return b.String(), i, nil
		}
		b.WriteRune(c)
		i++
	}
	return "", 0, fmt.Errorf("unclosed %q at index %d", delim, start)
}

type parser struct {
	path   string
	runes  []rune
	tokens []token
	i      int
	cfg    Config
}

// Parse ports jsonpath.parse (jsonpath.py:48-201).
func Parse(path string, cfg Config) (exp.Expression, error) {
	toks, err := tokenize(path, cfg)
	if err != nil {
		return nil, err
	}
	p := &parser{path: path, runes: []rune(path), tokens: toks, cfg: cfg}

	// Canonicalize the AST so it always starts with a "root" element: "field" -> "$.field".
	p.match(tokens.DOLLAR)

	varTokens := cfg.VarTokens
	if varTokens == nil {
		varTokens = defaultVarTokens
	}

	expressions := []exp.Expression{exp.JSONPathRoot(exp.Args{})}
	for p.i < len(p.tokens) {
		dotOrColon := false
		if _, ok := p.match(tokens.DOT); ok {
			dotOrColon = true
		} else if _, ok := p.match(tokens.COLON); ok {
			dotOrColon = true
		}

		switch {
		case dotOrColon:
			recursive := p.prev().text == ".."

			var value any
			if _, ok := p.matchSet(varTokens); ok {
				value = p.parseVarText()
			} else if ident, ok := p.match(tokens.IDENTIFIER); ok {
				value = ident.text
			} else if _, ok := p.match(tokens.STAR); ok {
				value = exp.JSONPathWildcard(exp.Args{})
			}

			if recursive {
				expressions = append(expressions, exp.JSONPathRecursive(exp.Args{"this": value}))
			} else if value != nil {
				expressions = append(expressions, exp.JSONPathKey(exp.Args{"this": value}))
			} else if !cfg.SingleDotIsWildcard {
				return nil, p.errorf("Expected key name or * after DOT")
			}
		case p.matchOnly(tokens.L_BRACKET):
			bracket, err := p.parseBracket()
			if err != nil {
				return nil, err
			}
			expressions = append(expressions, bracket)
		case p.matchOnlySet(varTokens):
			expressions = append(expressions, exp.JSONPathKey(exp.Args{"this": p.parseVarText()}))
		case p.matchOnly(tokens.IDENTIFIER):
			expressions = append(expressions, exp.JSONPathKey(exp.Args{"this": p.prev().text}))
		case p.matchOnly(tokens.STAR):
			expressions = append(expressions, exp.JSONPathWildcard(exp.Args{}))
		default:
			return nil, p.errorf(fmt.Sprintf("Unexpected %s", p.currName()))
		}
	}

	return exp.JSONPath(exp.Args{"expressions": expressions}), nil
}

func (p *parser) currName() string {
	if p.i >= len(p.tokens) {
		return "EOF"
	}
	return p.tokens[p.i].typ.String()
}

func (p *parser) curr() (tokens.TokenType, bool) {
	if p.i >= len(p.tokens) {
		return 0, false
	}
	return p.tokens[p.i].typ, true
}

func (p *parser) prev() token { return p.tokens[p.i-1] }

func (p *parser) advance() token {
	p.i++
	return p.prev()
}

func (p *parser) errorf(msg string) error {
	return fmt.Errorf("%s at index %d: %s", msg, p.i, p.path)
}

// match is `_match(token_type)`: consume and return the token when it matches.
func (p *parser) match(typ tokens.TokenType) (token, bool) {
	if current, ok := p.curr(); ok && current == typ {
		return p.advance(), true
	}
	return token{}, false
}

// matchOnly reports a match without returning the token (for `case` conditions).
func (p *parser) matchOnly(typ tokens.TokenType) bool {
	_, ok := p.match(typ)
	return ok
}

// matchSet is `_match_set(types)`: advance and return the token when its type is in the set.
func (p *parser) matchSet(types map[tokens.TokenType]bool) (token, bool) {
	if current, ok := p.curr(); ok && types[current] {
		return p.advance(), true
	}
	return token{}, false
}

// matchOnlySet reports a set match without returning the token.
func (p *parser) matchOnlySet(types map[tokens.TokenType]bool) bool {
	_, ok := p.matchSet(types)
	return ok
}

// parseLiteral ports _parse_literal (jsonpath.py:104-135). It returns a string, an int, an
// exp.JSONPathPart, or the Python `False` marker as a Go bool false.
func (p *parser) parseLiteral() (any, error) {
	if tok, ok := p.match(tokens.STRING); ok {
		return tok.text, nil
	}
	if tok, ok := p.match(tokens.IDENTIFIER); ok {
		return tok.text, nil
	}
	if _, ok := p.match(tokens.STAR); ok {
		return exp.JSONPathWildcard(exp.Args{}), nil
	}
	if _, ok := p.match(tokens.PLACEHOLDER); ok || p.matchOnly(tokens.L_PAREN) {
		script := p.prev().text == "("
		start := p.i

		for {
			if _, ok := p.match(tokens.L_BRACKET); ok {
				if _, err := p.parseBracket(); err != nil { // nested call, discarded
					return nil, err
				}
			}
			if current, ok := p.curr(); !ok || current == tokens.R_BRACKET {
				break
			}
			p.advance()
		}

		var node exp.Expression
		text := p.slice(start, p.i)
		if script {
			node = exp.JSONPathScript(exp.Args{"this": text})
		} else {
			node = exp.JSONPathFilter(exp.Args{"this": text})
		}
		return node, nil
	}

	number := ""
	if _, ok := p.match(tokens.DASH); ok {
		number = "-"
	}
	if tok, ok := p.match(tokens.NUMBER); ok {
		number += tok.text
	}
	if number != "" {
		value := 0
		negative := strings.HasPrefix(number, "-")
		digits := strings.TrimPrefix(number, "-")
		for _, r := range digits {
			value = value*10 + int(r-'0')
		}
		if negative {
			value = -value
		}
		return value, nil
	}

	return false, nil
}

// slice ports path[tokens[start].start : end]: `start` is a token index, and `end` is the
// current token's inclusive End (Python's exclusive slice bound) or the last token's End.
func (p *parser) slice(start int, end int) string {
	startOffset := 0
	if start < len(p.tokens) {
		startOffset = p.tokens[start].start
	}
	if end < len(p.tokens) {
		stop := p.tokens[end].end
		if startOffset <= stop {
			return string(p.runes[startOffset:stop])
		}
		return ""
	}
	if len(p.tokens) > 0 {
		stop := p.tokens[len(p.tokens)-1].end
		if startOffset <= stop {
			return string(p.runes[startOffset:stop])
		}
	}
	return ""
}

// parseSlice ports _parse_slice (jsonpath.py:137-146).
func (p *parser) parseSlice() (any, error) {
	start, err := p.parseLiteral()
	if err != nil {
		return nil, err
	}
	var end any
	if p.matchOnly(tokens.COLON) {
		if end, err = p.parseLiteral(); err != nil {
			return nil, err
		}
	}
	var step any
	if p.matchOnly(tokens.COLON) {
		if step, err = p.parseLiteral(); err != nil {
			return nil, err
		}
	}
	if end == nil && step == nil {
		return start, nil
	}
	return exp.JSONPathSlice(exp.Args{"start": start, "end": end, "step": step}), nil
}

// parseBracket ports _parse_bracket (jsonpath.py:148-177).
func (p *parser) parseBracket() (exp.Expression, error) {
	literal, err := p.parseSlice()
	if err != nil {
		return nil, err
	}

	if _, isString := literal.(string); !isString && isFalseMarker(literal) {
		return nil, p.errorf("Cannot have empty segment")
	}

	indexes := []any{literal}
	for p.matchOnly(tokens.COMMA) {
		if literal, err = p.parseSlice(); err != nil {
			return nil, err
		}
		if _, isString := literal.(string); isString || !isFalseMarker(literal) {
			indexes = append(indexes, literal)
		}
	}

	var node exp.Expression
	if len(indexes) == 1 {
		switch value := indexes[0].(type) {
		case string:
			node = exp.JSONPathKey(exp.Args{"this": value})
		case exp.Expression:
			switch value.Kind() {
			case exp.KindJSONPathScript, exp.KindJSONPathFilter:
				node = exp.JSONPathSelector(exp.Args{"this": value})
			default:
				node = exp.JSONPathSubscript(exp.Args{"this": value})
			}
		default:
			node = exp.JSONPathSubscript(exp.Args{"this": value})
		}
	} else {
		node = exp.JSONPathUnion(exp.Args{"expressions": indexes})
	}

	if _, ok := p.match(tokens.R_BRACKET); !ok {
		return nil, p.errorf("Expected R_BRACKET")
	}

	return node, nil
}

// isFalseMarker reports Python's `False` sentinel returned by an empty _parse_literal.
func isFalseMarker(value any) bool {
	b, ok := value.(bool)
	return ok && !b
}

// parseVarText ports _parse_var_text (jsonpath.py:179-198): merge consecutive var tokens and
// return the path text they cover, which keeps a key with spaces intact for BigQuery.
func (p *parser) parseVarText() string {
	prevIndex := p.i - 2

	varTokens := p.cfg.VarTokens
	if varTokens == nil {
		varTokens = defaultVarTokens
	}
	for {
		if _, ok := p.matchSet(varTokens); !ok {
			break
		}
	}

	start := 0
	if prevIndex >= 0 && prevIndex < len(p.tokens) {
		start = p.tokens[prevIndex].end + 1
	}

	var text string
	if p.i >= len(p.tokens) {
		if start <= len(p.runes) {
			text = string(p.runes[start:])
		}
	} else if stop := p.tokens[p.i].start; start <= stop {
		text = string(p.runes[start:stop])
	}
	return text
}

// BaseSingleTokens returns a copy of the base single-character token table, so a dialect can
// subtract an entry (snowflake removes '$').
func BaseSingleTokens() map[rune]tokens.TokenType {
	out := make(map[rune]tokens.TokenType, len(baseSingleTokens))
	for k, v := range baseSingleTokens {
		out[k] = v
	}
	return out
}
