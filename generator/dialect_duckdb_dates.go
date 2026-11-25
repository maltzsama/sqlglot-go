package generator

import (
	"strconv"
	"strings"

	"github.com/maltzsama/sqlglot-go/expressions"
)

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

// duckDBApproxQuantilesSQL ports generators/duckdb.py:4686-4720 approxquantiles_sql:
// BigQuery's APPROX_QUANTILES(expr, n) becomes DuckDB's APPROX_QUANTILE(expr, [0, 1/n, ..., 1]).
func duckDBApproxQuantilesSQL(g *Generator, e expressions.Expression) string {
	this := e.This()
	var numExpr expressions.Expression
	if this != nil && this.Kind() == expressions.KindDistinct {
		exprs := this.Expressions()
		if len(exprs) < 2 {
			g.unsupported("APPROX_QUANTILES requires a bucket count argument")
			return g.functionFallbackSQL(e)
		}
		numExpr = exprs[1]
		numExpr.Pop()
	} else {
		numExpr = asExpression(e.Arg("expression"))
	}

	text := ""
	if numExpr != nil && numExpr.Kind() == expressions.KindLiteral {
		text = numExpr.Text("this")
	}
	n := 0
	if text != "" && !strings.Contains(text, ".") {
		n, _ = strconv.Atoi(text)
	}
	if n <= 0 {
		g.unsupported("APPROX_QUANTILES bucket count must be a positive integer")
		return g.functionFallbackSQL(e)
	}

	quantiles := make([]expressions.Expression, 0, n+1)
	for i := 0; i <= n; i++ {
		quantiles = append(quantiles, expressions.LiteralNumber(quantileLiteral(i, n)))
	}
	return g.gen(expressions.ApproxQuantile(expressions.Args{
		"this":     this,
		"quantile": expressions.Array(expressions.Args{"expressions": quantiles}),
	}))
}

// quantileLiteral renders i/n the way Decimal division would: a bare integer at the ends,
// otherwise the shortest exact decimal for the common small bucket counts.
func quantileLiteral(i, n int) string {
	if i == 0 {
		return "0"
	}
	if i == n {
		return "1"
	}
	return strconv.FormatFloat(float64(i)/float64(n), 'f', -1, 64)
}

// duckDBArrayRemoveSQL ports remove_from_array_using_filter (dialects/dialect.py:2319) for a
// non-propagating source: ARRAY_REMOVE(a, x) becomes LIST_FILTER(a, _u -> _u <> x).
func duckDBArrayRemoveSQL(g *Generator, e expressions.Expression) string {
	lambdaID := expressions.ToIdentifier("_u")
	cond := expressions.NEQ(expressions.Args{"this": lambdaID, "expression": e.Arg("expression")})
	filter := expressions.ArrayFilter(expressions.Args{
		"this":       e.This(),
		"expression": expressions.Lambda(expressions.Args{"this": cond, "expressions": []expressions.Expression{lambdaID}}),
	})
	return g.gen(filter)
}
