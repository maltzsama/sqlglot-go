package generator

import "github.com/maltzsama/sqlglot-go/expressions"

// duckDBCastSQL renders CAST(this AS <dtype>), the duckdb transform for the *ToDate /
// *ToDateStr / *DateToStr family (generators/duckdb.py:1674,1764).
func duckDBCastSQL(dtype expressions.DType) func(*Generator, expressions.Expression) string {
	return func(g *Generator, e expressions.Expression) string {
		return g.gen(expressions.Cast(expressions.Args{
			"this": e.This(),
			"to":   expressions.DataType(expressions.Args{"this": dtype}),
		}))
	}
}

// duckDBTsOrDsToDateStr ports the duckdb behavior for TsOrDsToDateStr:
// SUBSTRING(CAST(x AS TEXT), 1, 10).
func duckDBTsOrDsToDateStr(g *Generator, e expressions.Expression) string {
	castText := expressions.Cast(expressions.Args{
		"this": e.This(),
		"to":   expressions.DataType(expressions.Args{"this": expressions.DTypeText}),
	})
	return g.gen(expressions.Substring(expressions.Args{
		"this":   castText,
		"start":  expressions.LiteralNumber("1"),
		"length": expressions.LiteralNumber("10"),
	}))
}

// duckDBDiToDate ports generators/duckdb.py DiToDate:
// CAST(STRPTIME(CAST(x AS TEXT), '%Y%m%d') AS DATE).
func duckDBDiToDate(g *Generator, e expressions.Expression) string {
	castText := expressions.Cast(expressions.Args{
		"this": e.This(),
		"to":   expressions.DataType(expressions.Args{"this": expressions.DTypeText}),
	})
	strptime := expressions.Anonymous(expressions.Args{
		"this":        "STRPTIME",
		"expressions": []expressions.Expression{castText, expressions.LiteralString("%Y%m%d")},
	})
	return g.gen(expressions.Cast(expressions.Args{
		"this": strptime,
		"to":   expressions.DataType(expressions.Args{"this": expressions.DTypeDate}),
	}))
}

// duckDBTsOrDiToDi ports generators/duckdb.py TsOrDiToDi:
// CAST(SUBSTR(REPLACE(CAST(x AS TEXT), '-', ”), 1, 8) AS INT).
func duckDBTsOrDiToDi(g *Generator, e expressions.Expression) string {
	castText := expressions.Cast(expressions.Args{
		"this": e.This(),
		"to":   expressions.DataType(expressions.Args{"this": expressions.DTypeText}),
	})
	replaced := expressions.Anonymous(expressions.Args{
		"this":        "REPLACE",
		"expressions": []expressions.Expression{castText, expressions.LiteralString("-"), expressions.LiteralString("")},
	})
	substr := expressions.Anonymous(expressions.Args{
		"this":        "SUBSTR",
		"expressions": []expressions.Expression{replaced, expressions.LiteralNumber("1"), expressions.LiteralNumber("8")},
	})
	return g.gen(expressions.Cast(expressions.Args{
		"this": substr,
		"to":   expressions.DataType(expressions.Args{"this": expressions.DTypeInt}),
	}))
}
