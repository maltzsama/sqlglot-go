package generator

import "github.com/maltzsama/sqlglot-go/expressions"

// parseJSONSQL ports parsejson_sql (generator.py:5548-5552) with the per-dialect
// PARSE_JSON_NAME: base/hive PARSE_JSON, presto JSON_PARSE, mysql none (bare value), and
// postgres's CAST(x AS JSON) transform (generators/postgres.py:353).
func (g *Generator) parseJSONSQL(e expressions.Expression) string {
	switch {
	case g.dialect.Name == "mysql":
		return g.sqlKey(e, "this")
	case g.isDialect("postgres"):
		return "CAST(" + g.sqlKey(e, "this") + " AS JSON)"
	case g.isDialect("presto"):
		return g.funcCall("JSON_PARSE", []any{e.Arg("this"), e.Arg("expression")}, "(", ")", true)
	case g.isDialect("spark2") && !g.isDialect("databricks"):
		// Spark2Generator.PARSE_JSON_NAME is None (generators/spark2.py:115), so PARSE_JSON
		// degrades to its argument - Spark parses it inline. Spark inherits the None; Databricks
		// restores "PARSE_JSON" (generators/databricks.py:21), hence the exclusion.
		return g.sqlKey(e, "this")
	}
	return g.funcCall("PARSE_JSON", []any{e.Arg("this"), e.Arg("expression")}, "(", ")", true)
}

// regexpReplaceSQL ports regexpreplace_sql (generator.py) / regexp_replace_sql (dialect.py:1868-1871).
func (g *Generator) regexpReplaceSQL(e expressions.Expression) string {
	return g.funcCall("REGEXP_REPLACE", []any{e.Arg("this"), e.Arg("expression"), e.Arg("replacement"), e.Arg("position"), e.Arg("occurrence"), e.Arg("modifiers")}, "(", ")", true)
}

func init() {
	dispatch[expressions.KindParseJSON] = (*Generator).parseJSONSQL
	dispatch[expressions.KindRegexpReplace] = (*Generator).regexpReplaceSQL
}
