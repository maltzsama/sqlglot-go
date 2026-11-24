package generator

import "github.com/maltzsama/sqlglot-go/expressions"

func init() {
	dispatch[expressions.KindSemanticView] = (*Generator).semanticViewSQL
	dispatch[expressions.KindDirectoryStage] = (*Generator).directoryStageSQL
}

func (g *Generator) directoryStageSQL(e expressions.Expression) string {
	return "DIRECTORY(" + g.sqlKey(e, "this") + ")"
}

func (g *Generator) semanticViewSQL(e expressions.Expression) string {
	body := g.seg(g.sqlKey(e, "this"), "")
	for _, clause := range []struct{ key, keyword string }{
		{"metrics", "METRICS"},
		{"dimensions", "DIMENSIONS"},
		{"facts", "FACTS"},
	} {
		if values := g.expressions(exprsOptions{expression: e, key: clause.key, flat: true}); values != "" {
			body += g.seg(clause.keyword + " " + values)
		}
	}
	if where := g.sqlKey(e, "where"); where != "" {
		body += g.seg("WHERE " + where)
	}
	return "SEMANTIC_VIEW(" + g.indent(body, 0, nil, true, false) + g.seg(")", "")
}
