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

// duckDBTimeStrToTimeSQL ports timestrtotime_sql (dialects/dialect.py:1769-1786): CAST(this
// AS TIMESTAMPTZ) when a zone is present, else CAST(this AS TIMESTAMP).
func duckDBTimeStrToTimeSQL(g *Generator, e expressions.Expression) string {
	dtype := expressions.DTypeTimestamp
	if truthy(e.Arg("zone")) {
		dtype = expressions.DTypeTimestampLtz
	}
	return g.gen(expressions.Cast(expressions.Args{
		"this": e.This(),
		"to":   expressions.DataType(expressions.Args{"this": dtype}),
	}))
}

// duckDBGenerateDateArraySQL ports _generate_datetime_array_sql (generators/duckdb.py:980-1000)
// for GenerateDateArray: BQ's GENERATE_DATE_ARRAY becomes a GENERATE_SERIES cast to DATE[].
func duckDBGenerateDateArraySQL(g *Generator, e expressions.Expression) string {
	start := implicitDatetimeCastDays(asExpression(e.Arg("start")))
	end := implicitDatetimeCastDays(asExpression(e.Arg("end")))
	series := expressions.GenerateSeries(expressions.Args{
		"start": start,
		"end":   end,
		"step":  e.Arg("step"),
	})
	arrayDate := expressions.DataType(expressions.Args{
		"this": expressions.DTypeArray,
		"expressions": []expressions.Expression{
			expressions.DataType(expressions.Args{"this": expressions.DTypeDate}),
		},
	})
	return g.gen(expressions.Cast(expressions.Args{"this": series, "to": arrayDate}))
}

// implicitDatetimeCastDays is _implicit_datetime_cast with the DATE default type.
func implicitDatetimeCastDays(arg expressions.Expression) expressions.Expression {
	return implicitDatetimeCast(arg)
}

// duckDBArrayInsertSQL ports _array_insert_sql (generators/duckdb.py:360-450): ARRAY_INSERT
// becomes a NULL-guarded LIST_CONCAT of slices around the inserted element.
func duckDBArrayInsertSQL(g *Generator, e expressions.Expression) string {
	this := e.This()
	position := asExpression(e.Arg("position"))
	element := asExpression(e.Arg("expression"))
	elementArray := expressions.Array(expressions.Args{"expressions": []expressions.Expression{element}})

	posValue, ok := duckDBIntLiteral(position)
	if !ok {
		g.unsupported("ARRAY_INSERT can only be transpiled with a literal position")
		return g.funcCall("ARRAY_INSERT", []any{this, position, element}, "(", ")", true)
	}
	offset, _ := duckDBIntLiteral(asExpression(e.Arg("offset")))
	if posValue > 0 {
		posValue -= offset
	} else if posValue < 0 {
		posValue += offset
	}

	slice := func(bounds ...int) expressions.Expression {
		return expressions.Bracket(expressions.Args{"this": this, "expressions": []expressions.Expression{
			expressions.Slice(expressions.Args{"this": expressions.LiteralNumber(strconv.Itoa(bounds[0])), "expression": expressions.LiteralNumber(strconv.Itoa(bounds[1]))}),
		}})
	}

	var concat []expressions.Expression
	switch {
	case posValue == 0:
		concat = []expressions.Expression{elementArray, this}
	case posValue > 0:
		right := expressions.Bracket(expressions.Args{"this": this, "expressions": []expressions.Expression{
			expressions.Slice(expressions.Args{"this": expressions.LiteralNumber(strconv.Itoa(posValue + 1))}),
		}})
		concat = []expressions.Expression{slice(1, posValue), elementArray, right}
	default:
		arrLen := expressions.Length(expressions.Args{"this": this})
		sliceEndPos := expressions.Add(expressions.Args{"this": arrLen, "expression": expressions.LiteralNumber(strconv.Itoa(posValue))})
		sliceStartPos := expressions.Add(expressions.Args{"this": sliceEndPos, "expression": expressions.LiteralNumber("1")})
		left := expressions.Bracket(expressions.Args{"this": this, "expressions": []expressions.Expression{
			expressions.Slice(expressions.Args{"this": expressions.LiteralNumber("1"), "expression": sliceEndPos}),
		}})
		right := expressions.Bracket(expressions.Args{"this": this, "expressions": []expressions.Expression{
			expressions.Slice(expressions.Args{"this": sliceStartPos}),
		}})
		concat = []expressions.Expression{left, elementArray, right}
	}

	listConcat := expressions.Anonymous(expressions.Args{"this": "LIST_CONCAT", "expressions": concat})
	return g.gen(expressions.If(expressions.Args{
		"this":  expressions.Is(expressions.Args{"this": this, "expression": expressions.Null()}),
		"true":  expressions.Null(),
		"false": listConcat,
	}))
}

// duckDBIntLiteral returns the integer value of a literal, accepting a leading negation.
func duckDBIntLiteral(e expressions.Expression) (int, bool) {
	if e == nil {
		return 0, false
	}
	if e.Kind() == expressions.KindNeg {
		v, ok := duckDBIntLiteral(asExpression(e.This()))
		return -v, ok
	}
	if e.Kind() == expressions.KindLiteral && e.IsInt() {
		v, err := strconv.Atoi(e.Text("this"))
		return v, err == nil
	}
	return 0, false
}
