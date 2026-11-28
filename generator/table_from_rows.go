package generator

import "github.com/maltzsama/sqlglot-go/expressions"

// tableFromRowsSQL ports Generator.tablefromrows_sql (generator.py:2672-2681): `TABLE(<this>)`
// with the optional alias, pivots, sample and joins. Dialects that render TABLE(...) differently
// register their own override.
func (g *Generator) tableFromRowsSQL(e expressions.Expression) string {
	table := g.funcCall("TABLE", []any{e.This()}, "(", ")", true)
	alias := g.sqlKey(e, "alias")
	if alias != "" {
		alias = " AS " + alias
	}
	sample := g.sqlKey(e, "sample")
	pivots := g.expressions(exprsOptions{expression: e, key: "pivots", emptySep: true, flat: true})
	joins := g.indent(g.expressions(exprsOptions{expression: e, key: "joins", emptySep: true, flat: true}), 0, nil, true, false)
	return table + alias + pivots + sample + joins
}

func init() {
	dispatch[expressions.KindTableFromRows] = (*Generator).tableFromRowsSQL
}
