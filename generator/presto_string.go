package generator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/maltzsama/sqlglot-go/expressions"
)

// escapedUnicodePattern ports ESCAPED_UNICODE_RE (generator.py:28).
var escapedUnicodePattern = regexp.MustCompile(`\\(\d+)`)

// unicodeStringSQL ports unicodestring_sql (generator.py:1653-1680).
func (g *Generator) unicodeStringSQL(e expressions.Expression) string {
	escape := asExpression(e.Arg("escape"))
	supportsUEscape := g.supportsUEscape()
	left, right := g.dialect.QuoteStart, g.dialect.QuoteEnd
	substitute := `\u$1`
	escapeSQL := ""
	if g.dialect.UnicodeStart != "" {
		left, right = g.dialect.UnicodeStart, g.dialect.UnicodeEnd
		substitute = `\$1`
		if escape != nil && supportsUEscape {
			escapeSQL = " UESCAPE " + g.gen(escape)
		}
	}

	this := e.Name()
	if g.dialect.UnicodeStart == "" || (escape != nil && !supportsUEscape) {
		if escape != nil {
			pattern := regexp.MustCompile(regexp.QuoteMeta(escape.Name()) + `(\d+)`)
			this = pattern.ReplaceAllString(this, substitute)
		} else {
			this = escapedUnicodePattern.ReplaceAllString(this, substitute)
		}
	}
	this = strings.ReplaceAll(g.replaceLineBreaks(this), "'", "''")
	return left + this + right + escapeSQL
}

// supportsUEscape ports SUPPORTS_UESCAPE for dialects without a custom escape clause.
func (g *Generator) supportsUEscape() bool {
	return !g.isDialect("bigquery") && !g.isDialect("clickhouse") && !g.isDialect("dremio") && !g.isDialect("oracle")
}

// structSQL ports struct_sql (generator.py:5267-5278): PropertyEQ fields render as aliases.
func (g *Generator) structSQL(e expressions.Expression) string {
	if g.isDialect("snowflake") {
		fields := e.Expressions()
		if len(fields) == 1 && (fields[0].IsStar() || (fields[0].Kind() == expressions.KindILike && fields[0].This().IsStar())) {
			return "{" + g.gen(fields[0]) + "}"
		}
		args := make([]any, 0, len(fields)*2)
		for i, field := range fields {
			if field.Kind() == expressions.KindPropertyEQ {
				key := field.This()
				if key.Kind() == expressions.KindIdentifier {
					key = expressions.LiteralString(key.Name())
				}
				args = append(args, key, field.Expr())
			} else {
				args = append(args, expressions.LiteralString(fmt.Sprintf("_%d", i)), field)
			}
		}
		return g.funcCall("OBJECT_CONSTRUCT", args, "(", ")", true)
	}
	args := []any{}
	for _, field := range e.Expressions() {
		if field.Kind() == expressions.KindPropertyEQ {
			var alias any = field.This()
			if field.This().IsString() {
				alias = field.Name()
			}
			field = expressions.AliasExpr(field.Arg("expression"), alias, false)
		}
		args = append(args, field)
	}
	return g.funcCall(g.sqlName(e.Kind()), args, "(", ")", true)
}

// versionSQL ports version_sql (generator.py:2649-2653).
func (g *Generator) versionSQL(e expressions.Expression) string {
	sql := "FOR " + e.Name() + " " + e.Text("kind")
	if expression := g.sqlKey(e, "expression"); expression != "" {
		sql += " " + expression
	}
	return sql
}

func init() {
	dispatch[expressions.KindUnicodeString] = (*Generator).unicodeStringSQL
	dispatch[expressions.KindVersion] = (*Generator).versionSQL
	dispatch[expressions.KindStruct] = (*Generator).structSQL
}
