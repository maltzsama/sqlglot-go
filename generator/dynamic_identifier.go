package generator

import "github.com/maltzsama/sqlglot-go/expressions"

func init() {
	dispatch[expressions.KindDynamicIdentifier] = (*Generator).dynamicIdentifierSQL
}

func (g *Generator) dynamicIdentifierSQL(e expressions.Expression) string {
	if !g.isDialect("snowflake") {
		if this := e.This(); this != nil && this.IsString() {
			if e.Arg("expressions") != nil {
				g.unsupported("Transpiling dynamically-invoked IDENTIFIER() functions is unsupported")
			}
			return this.Name()
		}
		g.unsupported("IDENTIFIER() with non-literal arguments is not supported")
	}
	sql := "IDENTIFIER(" + g.sqlKey(e, "this") + ")"
	if e.Arg("expressions") != nil {
		sql += "(" + g.expressions(exprsOptions{expression: e, flat: true}) + ")"
	}
	return sql
}
