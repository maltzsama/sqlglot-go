package generator

import "github.com/maltzsama/sqlglot-go/expressions"

func init() {
	dispatch[expressions.KindHistoricalData] = (*Generator).historicalDataSQL
	dispatch[expressions.KindChanges] = (*Generator).changesSQL
}

func (g *Generator) historicalDataSQL(e expressions.Expression) string {
	return e.Text("this") + " (" + e.Text("kind") + " => " + g.sqlKey(e, "expression") + ")"
}

func (g *Generator) changesSQL(e expressions.Expression) string {
	out := "CHANGES (INFORMATION => " + g.sqlKey(e, "information") + ")"
	for _, key := range []string{"at_before", "end"} {
		if part := g.sqlKey(e, key); part != "" {
			out += g.sep() + part
		}
	}
	return out
}
