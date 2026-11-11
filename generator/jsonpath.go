package generator

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/maltzsama/sqlglot-go/expressions"
)

// JSON path rendering ports generator.py:3795-3822 (jsonpath_sql / json_path_part) and the
// generator.py:5350-5379 JSONPathKey/JSONPathSubscript methods.
func init() {
	dispatch[expressions.KindJSONPath] = (*Generator).jsonPathSQL
	dispatch[expressions.KindJSONPathKey] = (*Generator).jsonPathKeySQL
	dispatch[expressions.KindJSONPathRoot] = func(*Generator, expressions.Expression) string { return "$" }
	dispatch[expressions.KindJSONPathSubscript] = (*Generator).jsonPathSubscriptSQL
	dispatch[expressions.KindJSONPathWildcard] = func(*Generator, expressions.Expression) string { return "*" }
}

// jsonPathSQL ports jsonpath_sql (generator.py:3795-3801).
func (g *Generator) jsonPathSQL(e expressions.Expression) string {
	sqls := make([]string, 0, len(e.Expressions()))
	for _, part := range e.Expressions() {
		sqls = append(sqls, g.gen(part))
	}
	path := strings.TrimLeft(strings.Join(sqls, ""), ".")
	if g.dialect.QuoteJSONPath {
		path = g.quoteStart + path + g.quoteEnd
	}
	return path
}

// jsonPathKeySQL ports _jsonpathkey_sql (generator.py:5350-5374).
func (g *Generator) jsonPathKeySQL(e expressions.Expression) string {
	this := asExpression(e.Arg("this"))
	if this != nil && this.Kind() == expressions.KindJSONPathWildcard {
		if part := g.gen(this); part != "" {
			return "." + part
		}
		return ""
	}

	name := e.Name()
	quoted := boolValue(e.Arg("quoted"))
	if (!quoted || !g.dialect.JSONPathKeyQuotedForcesBrackets) && isSafeJSONPathKey(name) {
		return "." + name
	}

	part := g.jsonPathPartValue(name)
	if quoted && g.dialect.QuoteJSONPath {
		// The whole path is a quoted string literal, so a bracketed key must be escaped again.
		part = g.escapeStr(part)
	}
	if g.dialect.JSONPathBracketedKeySupported {
		return "[" + part + "]"
	}
	return "." + part
}

// jsonPathSubscriptSQL ports _jsonpathsubscript_sql (generator.py:5376-5378).
func (g *Generator) jsonPathSubscriptSQL(e expressions.Expression) string {
	part := g.jsonPathPartValue(e.Arg("this"))
	if part == "" {
		return ""
	}
	return "[" + part + "]"
}

// jsonPathPartValue ports json_path_part (generator.py:3803-3822): a part node renders through
// its own handler, an integer is stringified, and a key is quoted.
func (g *Generator) jsonPathPartValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case expressions.Expression:
		if v == nil {
			return ""
		}
		return g.gen(v)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case string:
		if g.dialect.JSONPathSingleQuoteEscape {
			// Upstream formats the unescaped expression here (generator.py:3817), a quirk kept
			// for parity.
			return "\\'" + v + "\\'"
		}
		return `"` + strings.ReplaceAll(v, `"`, `\"`) + `"`
	}
	return ""
}

// isSafeJSONPathKey ports SAFE_IDENTIFIER_RE (expressions/core.py:2810): a word that may appear
// after a dot without brackets.
func isSafeJSONPathKey(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		if i == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}
