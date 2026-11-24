package generator

import "github.com/maltzsama/sqlglot-go/expressions"

// comprehensionSQL ports Generator.comprehension_sql (generator.py:5038-5046).
func (g *Generator) comprehensionSQL(e expressions.Expression) string {
	value := g.sqlKey(e, "this")
	item := g.sqlKey(e, "expression")
	position := g.sqlKey(e, "position")
	if position != "" {
		position = ", " + position
	}
	iterator := g.sqlKey(e, "iterator")
	condition := g.sqlKey(e, "condition")
	if condition != "" {
		condition = " IF " + condition
	}
	return value + " FOR " + item + position + " IN " + iterator + condition
}

func (g *Generator) columnsSQL(e expressions.Expression) string {
	call := g.funcCall("COLUMNS", g.fallbackArgs(e), "(", ")", true)
	if boolValue(e.Arg("unpack")) {
		return "*" + call
	}
	return call
}

func init() {
	dispatch[expressions.KindComprehension] = (*Generator).comprehensionSQL
	dispatch[expressions.KindColumns] = (*Generator).columnsSQL
}
