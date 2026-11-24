package generator

import "github.com/maltzsama/sqlglot-go/expressions"

func (g *Generator) snowflakeShowSQL(e expressions.Expression) string {
	out := "SHOW "
	if truthy(e.Arg("terse")) {
		out += "TERSE "
	}
	if truthy(e.Arg("iceberg")) {
		out += "ICEBERG "
	}
	out += e.Name()
	if truthy(e.Arg("history")) {
		out += " HISTORY"
	}
	out += g.prefixedSQL("LIKE", e, "like")
	if kind := e.Text("scope_kind"); kind != "" {
		out += " IN " + kind
	}
	if scope := g.sqlKey(e, "scope"); scope != "" {
		out += " " + scope
	}
	out += g.prefixedSQL("STARTS WITH", e, "starts_with")
	out += g.sqlKey(e, "limit")
	out += g.prefixedSQL("FROM", e, "from_")
	if privileges := g.expressions(exprsOptions{expression: e, key: "privileges", flat: true}); privileges != "" {
		out += " WITH PRIVILEGES " + privileges
	}
	return out
}
