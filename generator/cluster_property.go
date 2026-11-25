package generator

import "github.com/maltzsama/sqlglot-go/expressions"

func init() {
	dispatch[expressions.KindClusterProperty] = (*Generator).clusterPropertySQL
}

func (g *Generator) clusterPropertySQL(e expressions.Expression) string {
	if e.This() != nil {
		g.unsupported("Unsupported CLUSTER BY " + g.sqlKey(e, "this"))
		return ""
	}
	return "CLUSTER BY (" + g.expressions(exprsOptions{expression: e, flat: true}) + ")"
}
