package generator

import "github.com/maltzsama/sqlglot-go/expressions"

// parseJSONSQL ports parsejson_sql (generator.py:5548-5552) with the per-dialect
// PARSE_JSON_NAME: base/hive PARSE_JSON, presto JSON_PARSE, mysql none (bare value), and
// postgres's CAST(x AS JSON) transform (generators/postgres.py:353).
func (g *Generator) parseJSONSQL(e expressions.Expression) string {
	// Postgres renders ParseJSON as a JSON cast (generators/postgres.py:353). Redshift
	// inherits Postgres but drops that transform (generators/redshift.py:64) and sets
	// PARSE_JSON_NAME = "JSON_PARSE", so an explicit name wins over the cast.
	if g.dialect.ParseJSONName == nil && g.isDialect("postgres") {
		return "CAST(" + g.sqlKey(e, "this") + " AS JSON)"
	}
	name := "PARSE_JSON"
	if g.dialect.ParseJSONName != nil {
		name = *g.dialect.ParseJSONName
	}
	if name == "" {
		// PARSE_JSON_NAME = None: the node degrades to its argument (mysql, tsql, spark2,
		// sqlite, doris).
		return g.sqlKey(e, "this")
	}
	return g.funcCall(name, []any{e.Arg("this"), e.Arg("expression")}, "(", ")", true)
}

// regexpReplaceSQL ports regexpreplace_sql (generator.py) / regexp_replace_sql (dialect.py:1868-1871).
func (g *Generator) regexpReplaceSQL(e expressions.Expression) string {
	return g.funcCall("REGEXP_REPLACE", []any{e.Arg("this"), e.Arg("expression"), e.Arg("replacement"), e.Arg("position"), e.Arg("occurrence"), e.Arg("modifiers")}, "(", ")", true)
}

func init() {
	dispatch[expressions.KindParseJSON] = (*Generator).parseJSONSQL
	dispatch[expressions.KindRegexpReplace] = (*Generator).regexpReplaceSQL
}
