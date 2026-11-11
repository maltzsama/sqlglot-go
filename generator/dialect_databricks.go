package generator

import (
	"github.com/maltzsama/sqlglot-go/expressions"
)

// Databricks registers DatabricksGenerator's TYPE_MAPPING and TRANSFORMS
// (generators/databricks.py:23-70) plus its method overrides (generators/databricks.py:72-127).
func init() {
	registerDialectTypeMapping("databricks", map[expressions.DType]string{
		expressions.DTypeNull: "VOID",
	})

	registerDialectDispatch("databricks", map[expressions.Kind]func(*Generator, expressions.Expression) string{
		expressions.KindCurrentVersion: func(*Generator, expressions.Expression) string { return "CURRENT_VERSION()" },
		expressions.KindCurrentCatalog: func(*Generator, expressions.Expression) string { return "CURRENT_CATALOG()" },
		expressions.KindDateAdd:        dateDeltaSQL("DATEADD", false),
		// generators/databricks.py:58-59 removes the inherited RegexpLike and TryCast
		// mappings, so the base rendering applies.
		expressions.KindRegexpLike:  func(g *Generator, e expressions.Expression) string { return g.functionFallbackSQL(e) },
		expressions.KindTryCast:     (*Generator).tryCastSQL,
		expressions.KindDateDiff:    dateDeltaSQL("DATEDIFF", false),
		expressions.KindGroupConcat: sparkGroupConcat,
		// generators/databricks.py:48-56.
		expressions.KindJSONExtract:  databricksJSONExtractSQL,
		expressions.KindJSONPath:     databricksJSONPathSQL,
		expressions.KindJSONPathRoot: func(_ *Generator, e expressions.Expression) string { return databricksJSONPathRoot(e) },
		expressions.KindToChar:       databricksToCharSQL,
		// generators/databricks.py:60-62.
		expressions.KindRegrAvgx: databricksRegrSQL,
		expressions.KindRegrSxx:  databricksRegrSQL,
		expressions.KindRegrSyy:  databricksRegrSQL,
		// generators/databricks.py:72-127 method overrides.
		expressions.KindCreate:          databricksCreate,
		expressions.KindColumnDef:       databricksColumnDefSQL,
		expressions.KindTimeseriesKey:   databricksTimeseriesKeySQL,
		expressions.KindUniform:         databricksUniformSQL,
		expressions.KindClusterProperty: databricksClusterPropertySQL,
	})
}

// databricksJSONExtractSQL ports the JSONExtract transform (generators/databricks.py:48): the
// variant accessor `this:path`.
func databricksJSONExtractSQL(g *Generator, e expressions.Expression) string {
	return g.sqlKey(e, "this") + ":" + g.sqlKey(e, "expression")
}

// databricksJSONPathSQL ports jsonpath_sql (generators/databricks.py:99-106).
func databricksJSONPathSQL(g *Generator, e expressions.Expression) string {
	e.Set("escape", nil)
	path := g.jsonPathSQL(e)
	if p := e.Parent(); p != nil && p.Kind() == expressions.KindJSONExtractScalar {
		return g.quoteStart + path + g.quoteEnd
	}
	return path
}

// databricksJSONPathRoot ports the JSONPathRoot transform (generators/databricks.py:49-52):
// the `$` root only appears inside a quoted scalar-extraction path.
func databricksJSONPathRoot(e expressions.Expression) string {
	if p := e.Parent(); p != nil {
		if gp := p.Parent(); gp != nil && gp.Kind() == expressions.KindJSONExtractScalar {
			return "$"
		}
	}
	return ""
}

// databricksToCharSQL ports the ToChar transform (generators/databricks.py:52-56).
func databricksToCharSQL(g *Generator, e expressions.Expression) string {
	if !truthy(e.Arg("is_numeric")) {
		return g.functionFallbackSQL(e)
	}
	to, err := expressions.DataTypeBuild("STRING", "", false, false, nil)
	if err != nil {
		return g.functionFallbackSQL(e)
	}
	return g.castSQLWithPrefix(expressions.Cast(expressions.Args{"this": e.This(), "to": to}), "")
}

// databricksRegrSQL ports _regr_sql (generators/databricks.py:118-123).
func databricksRegrSQL(g *Generator, e expressions.Expression) string {
	name := g.sqlName(e.Kind())
	x := asExpression(e.Arg("expression"))
	if x != nil && x.Kind() == expressions.KindDistinct {
		args := []any{expressions.Distinct(expressions.Args{
			"expressions": []expressions.Expression{e.This()},
		})}
		for _, x := range x.Expressions() {
			args = append(args, x)
		}
		return g.funcCall(name, args, "(", ")", true)
	}
	return g.funcCall(name, []any{e.This(), x}, "(", ")", true)
}

// databricksCreate ports create_sql (generators/databricks.py:72-81): a RETURNS TABLE function
// body is wrapped in exp.Return before the Spark CREATE transforms run.
func databricksCreate(g *Generator, e expressions.Expression) string {
	body := asExpression(e.Arg("expression"))
	if body != nil && body.Kind() != expressions.KindReturn && e.Text("kind") == "FUNCTION" {
		for _, property := range e.FindAll(expressions.KindReturnsProperty) {
			if truthy(property.Arg("is_table")) {
				e.Set("expression", expressions.Return(expressions.Args{"this": body}))
				break
			}
		}
	}
	return sparkCreate(g, e)
}

// databricksColumnDefSQL ports columndef_sql (generators/databricks.py:83-94): a generated
// identity column is always BIGINT.
func databricksColumnDefSQL(g *Generator, e expressions.Expression) string {
	if e.Find(expressions.KindGeneratedAsIdentityColumnConstraint) != nil {
		if kind := asExpression(e.Arg("kind")); kind != nil && kind.Kind() == expressions.KindDataType {
			if dtype, ok := kind.Arg("this").(expressions.DType); ok && expressions.IntegerTypes[dtype] {
				e.Set("kind", expressions.DTypeBigInt.IntoExpr(nil))
			}
		}
	}
	return hiveColumnDefSQL(g, e)
}

// databricksTimeseriesKeySQL ports timeserieskey_sql (generators/databricks.py:96-97).
func databricksTimeseriesKeySQL(g *Generator, e expressions.Expression) string {
	return g.sqlKey(e, "this") + " TIMESERIES"
}

// databricksUniformSQL ports uniform_sql (generators/databricks.py:108-116): a `gen` argument
// from Snowflake's UNIFORM contributes the seed.
func databricksUniformSQL(g *Generator, e expressions.Expression) string {
	seed := asExpression(e.Arg("seed"))
	if gen := asExpression(e.Arg("gen")); gen != nil {
		seed = gen.This()
	}
	return g.funcCall("UNIFORM", []any{e.This(), e.Arg("expression"), seed}, "(", ")", true)
}

// databricksClusterPropertySQL ports clusterproperty_sql (generators/databricks.py:125-127).
func databricksClusterPropertySQL(g *Generator, e expressions.Expression) string {
	this := g.sqlKey(e, "this")
	if this == "" {
		this = "(" + g.expressions(exprsOptions{expression: e, flat: true}) + ")"
	}
	return "CLUSTER BY " + this
}
