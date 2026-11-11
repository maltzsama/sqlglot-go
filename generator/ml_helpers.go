package generator

import "github.com/maltzsama/sqlglot-go/expressions"

// Base-generator handlers for the BigQuery AI/ML helper nodes (generator.py:5054-5152) and the
// GAP_FILL table function (generator.py:5528-5531). They are dialect-agnostic upstream, so they
// live in the shared dispatch.
func init() {
	dispatch[expressions.KindGapFill] = (*Generator).gapFillSQL
	dispatch[expressions.KindExport] = func(g *Generator, e expressions.Expression) string {
		connection := ""
		if conn := g.sqlKey(e, "connection"); conn != "" {
			connection = "WITH CONNECTION " + conn + " "
		}
		return "EXPORT DATA " + connection + g.sqlKey(e, "options") + " AS " + g.sqlKey(e, "this")
	}
	dispatch[expressions.KindHavingMax] = func(g *Generator, e expressions.Expression) string {
		kind := "MIN"
		if truthy(e.Arg("max")) {
			kind = "MAX"
		}
		return g.sqlKey(e, "this") + " HAVING " + kind + " " + g.sqlKey(e, "expression")
	}
	dispatch[expressions.KindSafeFunc] = func(g *Generator, e expressions.Expression) string {
		return "SAFE." + g.sqlKey(e, "this")
	}
	dispatch[expressions.KindNetFunc] = func(g *Generator, e expressions.Expression) string {
		return "NET." + g.sqlKey(e, "this")
	}
	dispatch[expressions.KindForIn] = (*Generator).forInSQL
	dispatch[expressions.KindPredict] = func(g *Generator, e expressions.Expression) string {
		return g.mlSQL(e, "PREDICT")
	}
	dispatch[expressions.KindMLTranslate] = func(g *Generator, e expressions.Expression) string {
		return g.mlSQL(e, "TRANSLATE")
	}
	dispatch[expressions.KindMLForecast] = func(g *Generator, e expressions.Expression) string {
		return g.mlSQL(e, "FORECAST")
	}
	dispatch[expressions.KindGenerateEmbedding] = func(g *Generator, e expressions.Expression) string {
		name := "GENERATE_EMBEDDING"
		if truthy(e.Arg("is_text")) {
			name = "GENERATE_TEXT_EMBEDDING"
		}
		return g.mlSQL(e, name)
	}
	dispatch[expressions.KindGenerateText] = func(g *Generator, e expressions.Expression) string {
		return g.mlSQL(e, "GENERATE_TEXT")
	}
	dispatch[expressions.KindGenerateTable] = func(g *Generator, e expressions.Expression) string {
		return g.mlSQL(e, "GENERATE_TABLE")
	}
	dispatch[expressions.KindGenerateBool] = func(g *Generator, e expressions.Expression) string {
		return g.mlSQL(e, "GENERATE_BOOL")
	}
	dispatch[expressions.KindGenerateInt] = func(g *Generator, e expressions.Expression) string {
		return g.mlSQL(e, "GENERATE_INT")
	}
	dispatch[expressions.KindGenerateDouble] = func(g *Generator, e expressions.Expression) string {
		return g.mlSQL(e, "GENERATE_DOUBLE")
	}
	dispatch[expressions.KindAIForecast] = bigQueryAIForecastSQL
	dispatch[expressions.KindFeaturesAtTime] = bigQueryFeaturesAtTimeSQL
	dispatch[expressions.KindVectorSearch] = bigQueryVectorSearchSQL
}

// mlSQL ports _ml_sql (generator.py:5054-5066): MODEL <model> [, TABLE <source>] [, <params>].
func (g *Generator) mlSQL(e expressions.Expression, name string) string {
	model := "MODEL " + g.sqlKey(e, "this")
	expr := asExpression(e.Arg("expression"))
	var exprSQL any
	if expr != nil {
		if expr.Kind() == expressions.KindTable {
			exprSQL = "TABLE " + g.gen(expr)
		} else {
			exprSQL = g.gen(expr)
		}
	}
	var parameters any
	if params := g.sqlKey(e, "params_struct"); params != "" {
		parameters = params
	}
	return g.funcCall(name, []any{model, exprSQL, parameters}, "(", ")", true)
}

// bigQueryAIForecastSQL ports aiforecast_sql (generator.py:5096-5112).
func bigQueryAIForecastSQL(g *Generator, e expressions.Expression) string {
	thisSQL := g.sqlKey(e, "this")
	if this := asExpression(e.This()); this != nil && this.Kind() == expressions.KindTable {
		thisSQL = "TABLE " + thisSQL
	}
	return g.funcCall("FORECAST", []any{
		thisSQL,
		e.Arg("data_col"),
		e.Arg("timestamp_col"),
		e.Arg("model"),
		e.Arg("id_cols"),
		e.Arg("horizon"),
		e.Arg("forecast_end_timestamp"),
		e.Arg("confidence_level"),
		e.Arg("output_historical_time_series"),
		e.Arg("context_window"),
	}, "(", ")", true)
}

// bigQueryFeaturesAtTimeSQL ports featuresattime_sql (generator.py:5115-5126).
func bigQueryFeaturesAtTimeSQL(g *Generator, e expressions.Expression) string {
	thisSQL := g.sqlKey(e, "this")
	if this := asExpression(e.This()); this != nil && this.Kind() == expressions.KindTable {
		thisSQL = "TABLE " + thisSQL
	}
	return g.funcCall("FEATURES_AT_TIME", []any{
		thisSQL,
		e.Arg("time"),
		e.Arg("num_rows"),
		e.Arg("ignore_feature_nulls"),
	}, "(", ")", true)
}

// bigQueryVectorSearchSQL ports vectorsearch_sql (generator.py:5128-5146).
func bigQueryVectorSearchSQL(g *Generator, e expressions.Expression) string {
	thisSQL := g.sqlKey(e, "this")
	if this := asExpression(e.This()); this != nil && this.Kind() == expressions.KindTable {
		thisSQL = "TABLE " + thisSQL
	}
	queryTable := g.sqlKey(e, "query_table")
	if qt := asExpression(e.Arg("query_table")); qt != nil && qt.Kind() == expressions.KindTable {
		queryTable = "TABLE " + queryTable
	}
	return g.funcCall("VECTOR_SEARCH", []any{
		thisSQL,
		e.Arg("column_to_search"),
		queryTable,
		e.Arg("query_column_to_search"),
		e.Arg("top_k"),
		e.Arg("distance_type"),
		e.Arg("options"),
	}, "(", ")", true)
}

// forInSQL ports forin_sql (generator.py:5148-5151).
func (g *Generator) forInSQL(e expressions.Expression) string {
	return "FOR " + g.sqlKey(e, "this") + " DO " + g.sqlKey(e, "expression")
}

// gapFillSQL ports gapfill_sql (generator.py:5528-5531).
func (g *Generator) gapFillSQL(e expressions.Expression) string {
	args := []any{"TABLE " + g.sqlKey(e, "this")}
	for _, key := range expressions.ArgKeys(e.Kind()) {
		if key == "this" {
			continue
		}
		if value := e.Arg(key); value != nil {
			args = append(args, value)
		}
	}
	return g.funcCall("GAP_FILL", args, "(", ")", true)
}
