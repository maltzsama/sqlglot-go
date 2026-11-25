package generator

import "github.com/maltzsama/sqlglot-go/expressions"

func init() {
	dispatch[expressions.KindGet] = (*Generator).fileTransferSQL
	dispatch[expressions.KindPut] = (*Generator).fileTransferSQL
}

func (g *Generator) fileTransferSQL(e expressions.Expression) string {
	file, target := g.sqlKey(e, "this"), g.sqlKey(e, "target")
	verb := "GET"
	if e.Kind() == expressions.KindPut {
		verb = "PUT"
		file, target = target, file
	}
	out := verb + " " + target + " " + file
	if props := asExpression(e.Arg("properties")); props != nil {
		if sql := g.renderProperties(props, "", " ", "", false); sql != "" {
			out += " " + sql
		}
	}
	return out
}
