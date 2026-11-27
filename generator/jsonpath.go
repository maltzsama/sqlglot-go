package generator

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/maltzsama/sqlglot-go/dialects"
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
	// JSON_PATH_PART_TRANSFORMS entries this port needs (jsonpath.py:203-238).
	dispatch[expressions.KindJSONPathFilter] = func(_ *Generator, e expressions.Expression) string { return "?" + e.Text("this") }
	dispatch[expressions.KindJSONPathRecursive] = (*Generator).jsonPathRecursiveSQL
	dispatch[expressions.KindJSONPathScript] = func(_ *Generator, e expressions.Expression) string { return "(" + e.Text("this") }
	dispatch[expressions.KindJSONPathSelector] = func(g *Generator, e expressions.Expression) string {
		return "[" + g.jsonPathPartValue(e.Arg("this")) + "]"
	}
	dispatch[expressions.KindJSONPathSlice] = (*Generator).jsonPathSliceSQL
	dispatch[expressions.KindJSONPathUnion] = (*Generator).jsonPathUnionSQL
}

// jsonPathRecursiveSQL ports the JSONPathRecursive transform (jsonpath.py:206):
// `..{e.this or ”}`. `this` is a string name or a JSONPathWildcard node.
func (g *Generator) jsonPathRecursiveSQL(e expressions.Expression) string {
	value := e.Arg("this")
	if value == nil {
		return ".."
	}
	if part := g.jsonPathPartValue(value); part != "" {
		return ".." + part
	}
	return ".."
}

// jsonPathSliceSQL ports the JSONPathSlice transform (jsonpath.py:214-219):
// ":".join("" if p is False else json_path_part(p) for p in [start, end, step] if p is not None).
func (g *Generator) jsonPathSliceSQL(e expressions.Expression) string {
	parts := make([]string, 0, 3)
	for _, key := range []string{"start", "end", "step"} {
		value := e.Arg(key)
		if value == nil {
			continue
		}
		if isFalseValue(value) {
			parts = append(parts, "")
			continue
		}
		parts = append(parts, g.jsonPathPartValue(value))
	}
	return strings.Join(parts, ":")
}

// jsonPathUnionSQL ports the JSONPathUnion transform (jsonpath.py:220-222).
func (g *Generator) jsonPathUnionSQL(e expressions.Expression) string {
	values, _ := e.Arg("expressions").([]any)
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, g.jsonPathPartValue(value))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

// isFalseValue reports whether a JSONPath slice arg is upstream's Python `False` marker,
// which renders as the empty string in a slice.
func isFalseValue(value any) bool {
	b, ok := value.(bool)
	return ok && !b
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
	if (!quoted || !g.dialect.JSONPathKeyQuotedForcesBrackets) && isSafeJSONPathKey(name, g.dialect.JSONPathKeySafety) {
		return "." + name
	}

	part := g.jsonPathPartValue(name)
	if quoted && g.dialect.QuoteJSONPath {
		// The whole path is a quoted string literal, so a bracketed key must be escaped again.
		part = g.escapeStr(part)
	}
	if g.quoteJSONPathKeyUsingBrackets && g.dialect.JSONPathBracketedKeySupported {
		return "[" + part + "]"
	}
	return "." + part
}

// jsonPathSubscriptSQL ports _jsonpathsubscript_sql (generator.py:5376-5378).
func (g *Generator) jsonPathSubscriptSQL(e expressions.Expression) string {
	if g.isDialect("bigquery") || g.isDialect("sqlite") || g.isDialect("tsql") {
		if inner := asExpression(e.Arg("this")); inner != nil && inner.Kind() == expressions.KindJSONPathWildcard {
			// These generators omit JSONPathWildcard from their supported parts.
			if !g.isDialect("bigquery") {
				g.unsupported("Unsupported JSONPathPart type JSONPathWildcard")
			}
			return ""
		}
	}
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
		if g.quoteJSONPathKeyUsingBrackets && g.dialect.JSONPathSingleQuoteEscape {
			// Upstream formats the unescaped expression here (generator.py:3817), a quirk kept
			// for parity.
			return "\\'" + v + "\\'"
		}
		return `"` + strings.ReplaceAll(v, `"`, `\"`) + `"`
	}
	return ""
}

// isSafeJSONPathKey ports SAFE_JSON_PATH_KEY_RE (generator.py:853, base SAFE_IDENTIFIER_RE;
// generators/hive.py:237 and generators/bigquery.py override it): a key that may appear after
// a dot without brackets.
func isSafeJSONPathKey(name string, safety dialects.JSONPathKeySafety) bool {
	switch safety {
	case dialects.JSONPathKeySafetyBigQuery:
		// ^[\-\w]*$ — a dash and word characters, including the empty key.
		for _, r := range name {
			if r != '-' && r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
				return false
			}
		}
		return true
	case dialects.JSONPathKeySafetyDash:
		// ^[_\-a-zA-Z][\-\w]*$ — the first rune may also be a dash.
		if name == "" {
			return false
		}
		for i, r := range name {
			if r == '-' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
				if i == 0 && unicode.IsDigit(r) {
					return false
				}
				continue
			}
			return false
		}
		return true
	}

	// Base: ^[_a-zA-Z][\w]*$.
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
