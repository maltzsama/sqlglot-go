package generator

import "github.com/maltzsama/sqlglot-go/expressions"

func init() {
	dispatch[expressions.KindModelAttribute] = (*Generator).modelAttributeSQL
}

func (g *Generator) modelAttributeSQL(e expressions.Expression) string {
	if !g.isDialect("snowflake") {
		g.unsupported("The model!attribute syntax is not supported")
		return ""
	}
	return g.sqlKey(e, "this") + "!" + g.sqlKey(e, "expression")
}
