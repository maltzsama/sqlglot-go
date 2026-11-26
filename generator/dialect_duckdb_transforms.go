package generator

import (
	"strconv"
	"strings"

	"github.com/maltzsama/sqlglot-go/expressions"
)

// This file ports the remaining DuckDBGenerator overrides that the transpile corpus
// exercises (generators/duckdb.py). It is kept separate from dialect_duckdb.go so the
// date/array core port stays readable.

func init() {
	registerDialectDispatch("duckdb", map[expressions.Kind]func(*Generator, expressions.Expression) string{
		expressions.KindDayname:                duckDBDaynameSQL,
		expressions.KindMonthname:              duckDBMonthnameSQL,
		expressions.KindCurrentSchemas:         duckDBCurrentSchemasSQL,
		expressions.KindEqualNull:              duckDBEqualNullSQL,
		expressions.KindIsNullValue:            duckDBIsNullValueSQL,
		expressions.KindBitwiseLeftShift:       duckDBBitShiftSQL("<<"),
		expressions.KindBitwiseRightShift:      duckDBBitShiftSQL(">>"),
		expressions.KindBitwiseNot:             duckDBBitwiseNotSQL,
		expressions.KindBoolnot:                duckDBBoolnotSQL,
		expressions.KindBooland:                duckDBBoolSQL("AND"),
		expressions.KindBoolor:                 duckDBBoolSQL("OR"),
		expressions.KindBoolxorAgg:             duckDBBoolxorAggSQL,
		expressions.KindCorr:                   duckDBCorrSQL,
		expressions.KindCountIf:                duckDBCountIfSQL,
		expressions.KindMonthsBetween:          duckDBMonthsBetweenSQL,
		expressions.KindUnixMicros:             duckDBUnixUnitSQL("EPOCH_US"),
		expressions.KindUnixMillis:             duckDBUnixUnitSQL("EPOCH_MS"),
		expressions.KindUnixSeconds:            duckDBUnixSecondsSQL,
		expressions.KindStrToUnix:              duckDBStrToUnixSQL,
		expressions.KindTimeStrToUnix:          duckDBTimeStrToUnixSQL,
		expressions.KindTimeToUnix:             duckDBTimeToUnixSQL,
		expressions.KindUnixToStr:              duckDBUnixToStrSQL,
		expressions.KindUnixToTimeStr:          duckDBUnixToTimeStrSQL,
		expressions.KindDate:                   duckDBDateSQL,
		expressions.KindDatetime:               duckDBNoDatetimeSQL,
		expressions.KindTime:                   duckDBNoTimeSQL,
		expressions.KindTimestamp:              duckDBNoTimestampSQL,
		expressions.KindToBoolean:              duckDBToBooleanSQL,
		expressions.KindToNumber:               duckDBToNumberSQL,
		expressions.KindLevenshtein:            duckDBLevenshteinSQL,
		expressions.KindJarowinklerSimilarity:  duckDBJarowinklerSQL,
		expressions.KindArrayDistinct:          duckDBArrayDistinctSQL,
		expressions.KindArrayToString:          duckDBArrayToStringSQL,
		expressions.KindArraySlice:             duckDBArraySliceSQL,
		expressions.KindArrayPosition:          duckDBArrayPositionSQL,
		expressions.KindArrayExcept:            duckDBArrayBagSQL("EXCEPT"),
		expressions.KindArrayIntersect:         duckDBArrayBagSQL("INTERSECT"),
		expressions.KindFlatten:                renameFunc("FLATTEN"),
		expressions.KindAddMonths:              duckDBAddMonthsSQL,
		expressions.KindSpace:                  duckDBRepeatSQL,
		expressions.KindApproxQuantile:         duckDBApproxQuantileSQL,
		expressions.KindJSONFormat:             duckDBJSONFormatSQL,
		expressions.KindMapCat:                 duckDBMapCatSQL,
		expressions.KindMapContainsKey:         duckDBMapContainsKeySQL,
		expressions.KindMapDelete:              duckDBMapDeleteSQL,
		expressions.KindMapPick:                duckDBMapPickSQL,
		expressions.KindMapSize:                duckDBMapSizeSQL,
		expressions.KindMapInsert:              duckDBMapInsertSQL,
		expressions.KindArrayUniqueAgg:         duckDBArrayUniqueAggSQL,
		expressions.KindArrayConcatAgg:         duckDBArrayConcatAggSQL,
		expressions.KindArrayUnionAgg:          duckDBArrayUnionAggSQL,
		expressions.KindBase64Encode:           duckDBBase64EncodeSQL,
		expressions.KindBase64DecodeBinary:     duckDBBase64DecodeSQL(false),
		expressions.KindBase64DecodeString:     duckDBBase64DecodeSQL(true),
		expressions.KindConcatWs:               duckDBConcatWsSQL,
		expressions.KindSplitPart:              duckDBSplitPartSQL,
		expressions.KindSplit:                  duckDBSplitSQL,
		expressions.KindStrtokToArray:          duckDBStrtokToArraySQL,
		expressions.KindTrunc:                  duckDBTruncSQL,
		expressions.KindRound:                  duckDBRoundSQL,
		expressions.KindCeil:                   duckDBCeilFloorSQL("CEIL"),
		expressions.KindFloor:                  duckDBCeilFloorSQL("FLOOR"),
		expressions.KindNthValue:               duckDBNthValueSQL,
		expressions.KindTimeSlice:              duckDBTimeSliceSQL,
		expressions.KindArraysZip:              duckDBArraysZipSQL,
		expressions.KindBitmapBucketNumber:     duckDBBitmapBucketNumberSQL,
		expressions.KindBitmapBitPosition:      duckDBBitmapBitPositionSQL,
		expressions.KindRegexpReplace:          duckDBRegexpReplaceSQL,
		expressions.KindRegexpCount:            duckDBRegexpCountSQL,
		expressions.KindRegexpLike:             duckDBRegexpLikeSQL,
		expressions.KindDateTrunc:              duckDBDateTruncSQL,
		expressions.KindTimestampTrunc:         duckDBDateTruncSQL,
		expressions.KindDatetimeTrunc:          duckDBDateTruncSQL,
		expressions.KindGenerateTimestampArray: duckDBGenerateTimestampArraySQL,
		expressions.KindJSONExtractScalar:      duckDBJSONExtractScalarSQL,
		expressions.KindCurrentDate:            duckDBCurrentDateSQL,
		expressions.KindDateToDi:               duckDBDateToDiSQL,
		expressions.KindTimeToTimeStr:          duckDBTimeToTimeStrSQL,
		expressions.KindGenerator:              duckDBGeneratorSQL,
		expressions.KindTableFromRows:          duckDBTableFromRowsSQL,
		expressions.KindGreatest:               duckDBGreatestLeastSQL,
		expressions.KindLeast:                  duckDBGreatestLeastSQL,
		expressions.KindUnnest:                 duckDBUnnestSQL,
		expressions.KindExtract:                duckDBExtractSQL,
		expressions.KindIn:                     duckDBInSQL,
		expressions.KindRegexpExtract:          duckDBRegexpExtractSQL,
		expressions.KindRegexpExtractAll:       duckDBRegexpExtractSQL,
		expressions.KindRegexpInstr:            duckDBRegexpInstrSQL,
		expressions.KindArrayRemoveAt:          duckDBArrayRemoveAtSQL,
		expressions.KindPosexplode:             duckDBPosexplodeSQL,
		expressions.KindDecodeCase:             duckDBDecodeCaseSQL,
		expressions.KindSeq1:                   duckDBSeqSQL(1),
		expressions.KindSeq2:                   duckDBSeqSQL(2),
		expressions.KindSeq4:                   duckDBSeqSQL(4),
		expressions.KindSeq8:                   duckDBSeqSQL(8),
		expressions.KindToArray:                duckDBToArraySQL,
		expressions.KindTimeFromParts:          duckDBTimeFromPartsSQL,
		expressions.KindNumberToStr:            duckDBNumberToStrSQL,
		expressions.KindRegexpILike:            duckDBRegexpILikeSQL,
		expressions.KindNextDay:                duckDBDayNavigationSQL,
		expressions.KindPreviousDay:            duckDBDayNavigationSQL,
		expressions.KindLateral:                duckDBLateralSQL,
		expressions.KindTsOrDsToDatetime:       duckDBTsOrDsToDatetimeSQL,
		expressions.KindGetExtract:             duckDBGetExtractSQL,
		expressions.KindParseDatetime:          duckDBParseDatetimeSQL,
		expressions.KindFilter:                 duckDBFilterSQL,
		expressions.KindWindow:                 duckDBWindowSQL,
		expressions.KindNormal:                 duckDBNormalSQL,
		expressions.KindUniform:                duckDBUniformSQL,
		expressions.KindRandstr:                duckDBRandstrSQL,
		expressions.KindStripNullValue:         duckDBStripNullValueSQL,
		expressions.KindTsOrDsToTime:           duckDBTsOrDsToTimeSQL,
		expressions.KindTsOrDsToTimestamp:      duckDBTsOrDsToTimestampSQL,
		expressions.KindStarMap:                duckDBStarMapSQL,
		expressions.KindStrPosition:            duckDBStrPositionSQL,
		expressions.KindParseTime:              duckDBParseTimeSQL,
	})
}

func duckDBAnon(name string, args ...expressions.Expression) expressions.Expression {
	return expressions.Anonymous(expressions.Args{"this": name, "expressions": args})
}

func duckDBUpper(e expressions.Expression) expressions.Expression {
	return duckDBAnon("UPPER", e)
}

// duckDBDaynameSQL ports the Dayname lambda (generators/duckdb.py:1674-1682).
func duckDBDaynameSQL(g *Generator, e expressions.Expression) string {
	if truthy(e.Arg("abbreviated")) {
		return g.funcCall("STRFTIME", []any{e.This(), expressions.LiteralString("%a")}, "(", ")", true)
	}
	return g.funcCall("DAYNAME", []any{e.This()}, "(", ")", true)
}

// duckDBMonthnameSQL ports the Monthname lambda (generators/duckdb.py:1683-1691).
func duckDBMonthnameSQL(g *Generator, e expressions.Expression) string {
	if truthy(e.Arg("abbreviated")) {
		return g.funcCall("STRFTIME", []any{e.This(), expressions.LiteralString("%b")}, "(", ")", true)
	}
	return g.funcCall("MONTHNAME", []any{e.This()}, "(", ")", true)
}

// duckDBCurrentSchemasSQL ports the CurrentSchemas lambda (generators/duckdb.py:1630-1632).
func duckDBCurrentSchemasSQL(g *Generator, e expressions.Expression) string {
	arg := e.This()
	if arg == nil {
		arg = expressions.True_()
	}
	return g.funcCall("current_schemas", []any{arg}, "(", ")", true)
}

// duckDBEqualNullSQL ports the EqualNull lambda (generators/duckdb.py:1721-1723).
func duckDBEqualNullSQL(g *Generator, e expressions.Expression) string {
	return g.gen(expressions.NullSafeEQ(expressions.Args{"this": e.This(), "expression": e.Arg("expression")}))
}

// duckDBIsNullValueSQL ports the IsNullValue lambda (generators/duckdb.py:1652-1654).
func duckDBIsNullValueSQL(g *Generator, e expressions.Expression) string {
	return g.gen(expressions.EQ(expressions.Args{
		"this":       duckDBAnon("JSON_TYPE", e.This()),
		"expression": expressions.LiteralString("NULL"),
	}))
}

// duckDBBitShiftSQL ports _bitshift_sql (generators/duckdb.py:1276-1310) for integer operands.
func duckDBBitShiftSQL(op string) func(*Generator, expressions.Expression) string {
	return func(g *Generator, e expressions.Expression) string {
		this := e.This()
		expr := asExpression(e.Arg("expression"))
		if truthy(e.Arg("requires_int128")) && !duckDBIsBinaryArg(e) {
			this = expressions.Cast(expressions.Args{"this": this, "to": expressions.DTypeInt128.IntoExpr(nil)})
		}
		return g.gen(this) + " " + op + " " + g.gen(expr)
	}
}

// duckDBIsBinaryArg reports whether the expression's operand is binary/bit typed.
func duckDBIsBinaryArg(e expressions.Expression) bool {
	dtype, ok := castResultDType(e.This())
	if !ok {
		return false
	}
	switch dtype {
	case expressions.DTypeBinary, expressions.DTypeVarBinary, expressions.DTypeBit:
		return true
	}
	return false
}

// duckDBBitwiseNotSQL ports bitwisenot_sql (generators/duckdb.py:4728-4743) for the
// non-binary form: a negative operand is parenthesized after the tilde.
func duckDBBitwiseNotSQL(g *Generator, e expressions.Expression) string {
	this := e.This()
	if this != nil && this.Kind() == expressions.KindNeg {
		return "~(" + g.gen(this) + ")"
	}
	return "~" + g.gen(this)
}

// duckDBBoolnotSQL ports _boolnot_sql (generators/duckdb.py:1465-1468).
func duckDBBoolnotSQL(g *Generator, e expressions.Expression) string {
	return "NOT (" + g.gen(duckDBRoundInput(e.This(), e)) + ")"
}

// duckDBBoolSQL ports _booland_sql/_boolor_sql (generators/duckdb.py:1470-1482).
func duckDBBoolSQL(op string) func(*Generator, expressions.Expression) string {
	return func(g *Generator, e expressions.Expression) string {
		return "(" + g.gen(duckDBRoundInput(e.This(), e)) + " " + op + " " + g.gen(duckDBRoundInput(asExpression(e.Arg("expression")), e)) + ")"
	}
}

// duckDBRoundInput ports _round_arg (generators/duckdb.py:1459-1463).
func duckDBRoundInput(arg expressions.Expression, parent expressions.Expression) expressions.Expression {
	if arg == nil || !truthy(parent.Arg("round_input")) {
		return arg
	}
	return expressions.Round(expressions.Args{"this": arg, "decimals": expressions.LiteralNumber("0")})
}

// duckDBBoolxorAggSQL ports _boolxor_agg_sql (generators/duckdb.py:1261-1274).
func duckDBBoolxorAggSQL(g *Generator, e expressions.Expression) string {
	arg := e.This()
	if arg != nil {
		arg = expressions.Cast(expressions.Args{"this": arg, "to": expressions.DTypeBoolean.IntoExpr(nil)})
	}
	return g.funcCall("COUNT_IF", []any{arg}, "(", ")", true) + " = 1"
}

// duckDBCorrSQL ports _corr_sql (generators/duckdb.py:4760-4775) for the plain form.
func duckDBCorrSQL(g *Generator, e expressions.Expression) string {
	corr := g.funcCall("CORR", []any{e.This(), e.Arg("expression")}, "(", ")", true)
	if !truthy(e.Arg("null_on_zero_variance")) {
		return corr
	}
	// filter_sql/window_sql wrap the whole CORR(...) [FILTER] [OVER] expression.
	if parent := e.Parent(); parent != nil && (parent.Kind() == expressions.KindFilter || parent.Kind() == expressions.KindWindow) {
		return corr
	}
	return "CASE WHEN ISNAN(" + corr + ") THEN NULL ELSE " + corr + " END"
}

// duckDBCountIfSQL ports countif_sql (generators/duckdb.py:3162-3172).
func duckDBCountIfSQL(g *Generator, e expressions.Expression) string {
	cond := e.This()
	if truthy(e.Arg("zero_on_all_null")) && cond != nil {
		cond = expressions.Is(expressions.Args{"this": cond, "expression": expressions.True_()})
	}
	return g.funcCall("COUNT_IF", []any{cond}, "(", ")", true)
}

// duckDBMonthsBetweenSQL ports months_between_sql (dialects/dialect.py:1539-1588).
func duckDBMonthsBetweenSQL(g *Generator, e expressions.Expression) string {
	date1 := expressions.Cast(expressions.Args{"this": e.This(), "to": expressions.DTypeDate.IntoExpr(nil)})
	date2 := expressions.Cast(expressions.Args{"this": e.Arg("expression"), "to": expressions.DTypeDate.IntoExpr(nil)})
	whole := expressions.Anonymous(expressions.Args{"this": "DATE_DIFF", "expressions": []expressions.Expression{
		expressions.LiteralString("MONTH"), date2, date1,
	}})
	day := func(x expressions.Expression) expressions.Expression {
		return duckDBAnon("DAY", x)
	}
	lastDay := func(x expressions.Expression) expressions.Expression {
		return duckDBAnon("LAST_DAY", x)
	}
	day1 := day(date1.Copy())
	day2 := day(date2.Copy())
	bothLast := expressions.And(expressions.Args{
		"this":       expressions.EQ(expressions.Args{"this": day1.Copy(), "expression": day(lastDay(date1.Copy()))}),
		"expression": expressions.EQ(expressions.Args{"this": day2.Copy(), "expression": day(lastDay(date2.Copy()))}),
	})
	fractional := expressions.Div(expressions.Args{
		"this":       expressions.Paren(expressions.Args{"this": expressions.Sub(expressions.Args{"this": day1, "expression": day2})}),
		"expression": expressions.LiteralNumber("31.0"),
	})
	result := expressions.Add(expressions.Args{
		"this": whole,
		"expression": expressions.If(expressions.Args{
			"this": bothLast, "true": expressions.LiteralNumber("0"), "false": fractional,
		}),
	})
	return g.gen(result)
}

// duckDBUnixUnitSQL ports the UnixMicros/UnixMillis lambdas
// (generators/duckdb.py:1900-1906).
func duckDBUnixUnitSQL(name string) func(*Generator, expressions.Expression) string {
	return func(g *Generator, e expressions.Expression) string {
		return g.funcCall(name, []any{implicitDatetimeCast(e.This())}, "(", ")", true)
	}
}

// duckDBStrToUnixSQL ports the StrToUnix lambda (generators/duckdb.py:1845-1847).
func duckDBStrToUnixSQL(g *Generator, e expressions.Expression) string {
	strptime := g.funcCall("STRPTIME", []any{e.This(), g.duckDBFormatTime(e)}, "(", ")", true)
	return g.funcCall("EPOCH", []any{strptime}, "(", ")", true)
}

// duckDBTimeStrToUnixSQL ports the TimeStrToUnix lambda (generators/duckdb.py:1878-1880).
func duckDBTimeStrToUnixSQL(g *Generator, e expressions.Expression) string {
	cast := g.gen(expressions.Cast(expressions.Args{"this": e.This(), "to": expressions.DTypeTimestamp.IntoExpr(nil)}))
	return g.funcCall("EPOCH", []any{cast}, "(", ")", true)
}

// duckDBTimeToUnixSQL ports the TimeToUnix lambda (generators/duckdb.py:1881).
func duckDBTimeToUnixSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("EPOCH", []any{e.This()}, "(", ")", true)
}

// duckDBUnixToStrSQL ports the UnixToStr lambda (generators/duckdb.py:1903-1906).
func duckDBUnixToStrSQL(g *Generator, e expressions.Expression) string {
	ts := g.funcCall("TO_TIMESTAMP", []any{e.This()}, "(", ")", true)
	return g.funcCall("STRFTIME", []any{ts, g.duckDBFormatTime(e)}, "(", ")", true)
}

// duckDBUnixToTimeStrSQL ports the UnixToTimeStr lambda (generators/duckdb.py:1903).
func duckDBUnixToTimeStrSQL(g *Generator, e expressions.Expression) string {
	return "CAST(TO_TIMESTAMP(" + g.sqlKey(e, "this") + ") AS TEXT)"
}

// duckDBDateSQL ports _date_sql (generators/duckdb.py:288-304).
func duckDBDateSQL(g *Generator, e expressions.Expression) string {
	this := e.This()
	zone := asExpression(e.Arg("zone"))
	if zone != nil {
		this = expressions.Cast(expressions.Args{"this": this, "to": expressions.DTypeTimestamp.IntoExpr(nil)})
		atUTC := expressions.AtTimeZone(expressions.Args{"this": this, "zone": expressions.LiteralString("UTC")})
		this = expressions.AtTimeZone(expressions.Args{"this": atUTC, "zone": zone})
	}
	return g.gen(expressions.Cast(expressions.Args{"this": this, "to": expressions.DTypeDate.IntoExpr(nil)}))
}

// duckDBNoDatetimeSQL ports no_datetime_sql (dialects/dialect.py:1736-1749).
func duckDBNoDatetimeSQL(g *Generator, e expressions.Expression) string {
	this := e.This()
	expr := asExpression(e.Arg("expression"))
	if expr != nil && duckDBIsTimezoneName(expr.Name()) {
		this = expressions.Cast(expressions.Args{"this": this, "to": expressions.DTypeTimestampTz.IntoExpr(nil)})
		this = expressions.Cast(expressions.Args{
			"this": expressions.AtTimeZone(expressions.Args{"this": this, "zone": expr}),
			"to":   expressions.DTypeTimestamp.IntoExpr(nil),
		})
		return g.gen(this)
	}
	if expr != nil {
		this = expressions.Cast(expressions.Args{"this": this, "to": expressions.DTypeDate.IntoExpr(nil)})
		expr = expressions.Cast(expressions.Args{"this": expr, "to": expressions.DTypeTime.IntoExpr(nil)})
		return g.gen(expressions.Cast(expressions.Args{
			"this": expressions.Add(expressions.Args{"this": this, "expression": expr}),
			"to":   expressions.DTypeTimestamp.IntoExpr(nil),
		}))
	}
	return g.gen(expressions.Cast(expressions.Args{"this": this, "to": expressions.DTypeTimestamp.IntoExpr(nil)}))
}

// duckDBNoTimeSQL ports no_time_sql (dialects/dialect.py:1729-1733).
func duckDBNoTimeSQL(g *Generator, e expressions.Expression) string {
	this := expressions.Cast(expressions.Args{"this": e.This(), "to": expressions.DTypeTimestampTz.IntoExpr(nil)})
	zone := asExpression(e.Arg("zone"))
	if zone == nil {
		zone = expressions.Null()
	}
	expr := expressions.Cast(expressions.Args{
		"this": expressions.AtTimeZone(expressions.Args{"this": this, "zone": zone}),
		"to":   expressions.DTypeTime.IntoExpr(nil),
	})
	return g.gen(expr)
}

// duckDBNoTimestampSQL ports no_timestamp_sql (dialects/dialect.py:1712-1726).
func duckDBNoTimestampSQL(g *Generator, e expressions.Expression) string {
	zone := asExpression(e.Arg("zone"))
	if zone == nil {
		return g.gen(expressions.Cast(expressions.Args{"this": e.This(), "to": expressions.DTypeTimestamp.IntoExpr(nil)}))
	}
	if duckDBIsTimezoneName(zone.Name()) {
		return g.gen(expressions.AtTimeZone(expressions.Args{
			"this": expressions.Cast(expressions.Args{"this": e.This(), "to": expressions.DTypeTimestamp.IntoExpr(nil)}),
			"zone": zone,
		}))
	}
	return g.funcCall("TIMESTAMP", []any{e.This(), zone}, "(", ")", true)
}

// duckDBIsTimezoneName is a narrow stand-in for upstream's TIMEZONES lookup, covering the
// IANA region/city forms the corpus uses plus UTC.
func duckDBIsTimezoneName(name string) bool {
	if strings.EqualFold(name, "UTC") {
		return true
	}
	return strings.Contains(name, "/")
}

// duckDBToBooleanSQL ports _to_boolean_sql (generators/duckdb.py:234-286).
func duckDBToBooleanSQL(g *Generator, e expressions.Expression) string {
	arg := e.This()
	safe := truthy(e.Arg("safe"))
	upper := duckDBUpper(expressions.Cast(expressions.Args{
		"this": arg, "to": expressions.DTypeVarchar.IntoExpr(nil),
	}))
	ifs := []expressions.Expression{
		expressions.If(expressions.Args{
			"this": expressions.EQ(expressions.Args{"this": upper, "expression": expressions.LiteralString("ON")}),
			"true": expressions.True_(),
		}),
		expressions.If(expressions.Args{
			"this": expressions.EQ(expressions.Args{"this": upper, "expression": expressions.LiteralString("OFF")}),
			"true": expressions.Boolean(expressions.Args{"this": false}),
		}),
	}
	caseExpr := expressions.Case(expressions.Args{"ifs": ifs})
	if safe {
		caseExpr.Set("default", expressions.TryCast(expressions.Args{
			"this": arg, "to": expressions.DTypeBoolean.IntoExpr(nil),
		}))
	} else {
		castToReal := expressions.TryCast(expressions.Args{
			"this": arg, "to": expressions.DTypeFloat.IntoExpr(nil),
		})
		nanInf := expressions.Or(expressions.Args{
			"this":       duckDBAnon("ISNAN", castToReal),
			"expression": duckDBAnon("ISINF", castToReal),
		})
		caseExpr.Set("ifs", append(listFromValue(caseExpr.Arg("ifs")), expressions.If(expressions.Args{
			"this": nanInf,
			"true": duckDBAnon("ERROR", expressions.LiteralString("TO_BOOLEAN: Non-numeric values NaN and INF are not supported")),
		})))
		caseExpr.Set("default", expressions.Cast(expressions.Args{
			"this": arg, "to": expressions.DTypeBoolean.IntoExpr(nil),
		}))
	}
	return g.gen(caseExpr)
}

// duckDBToNumberSQL ports tonumber_sql (generators/duckdb.py:2581-2593).
func duckDBToNumberSQL(g *Generator, e expressions.Expression) string {
	format := asExpression(e.Arg("format"))
	precision := asExpression(e.Arg("precision"))
	scale := asExpression(e.Arg("scale"))
	if format == nil && precision != nil && scale != nil {
		return g.gen(expressions.Cast(expressions.Args{
			"this": e.This(),
			"to": expressions.DataType(expressions.Args{
				"this":        expressions.DTypeDecimal,
				"expressions": []expressions.Expression{precision, scale},
			}),
		}))
	}
	return g.functionFallbackSQL(e)
}

// duckDBLevenshteinSQL ports levenshtein_sql (generators/duckdb.py:3432-3442).
func duckDBLevenshteinSQL(g *Generator, e expressions.Expression) string {
	this := e.This()
	expr := asExpression(e.Arg("expression"))
	maxDist := asExpression(e.Arg("max_dist"))
	lev := g.funcCall("LEVENSHTEIN", []any{this, expr}, "(", ")", true)
	if maxDist == nil {
		return lev
	}
	return "CASE WHEN " + lev + " IS NULL OR " + g.gen(maxDist) + " IS NULL THEN NULL ELSE LEAST(" + lev + ", " + g.gen(maxDist) + ") END"
}

// duckDBJarowinklerSQL ports jarowinklersimilarity_sql (generators/duckdb.py:2457-2470).
func duckDBJarowinklerSQL(g *Generator, e expressions.Expression) string {
	this := e.This()
	expr := asExpression(e.Arg("expression"))
	if truthy(e.Arg("case_insensitive")) {
		this = duckDBUpper(this)
		expr = duckDBUpper(expr)
	}
	result := duckDBAnon("JARO_WINKLER_SIMILARITY", this, expr)
	if truthy(e.Arg("integer_scale")) {
		result = expressions.Cast(expressions.Args{
			"this": expressions.Mul(expressions.Args{"this": result, "expression": expressions.LiteralNumber("100")}),
			"to":   expressions.DTypeInt.IntoExpr(nil),
		})
	}
	return g.gen(result)
}

// duckDBArrayDistinctSQL ports arraydistinct_sql (generators/duckdb.py:3528-3546).
func duckDBArrayDistinctSQL(g *Generator, e expressions.Expression) string {
	this := e.This()
	distinct := g.funcCall("LIST_DISTINCT", []any{this}, "(", ")", true)
	if !truthy(e.Arg("check_null")) {
		return distinct
	}
	filtered := g.funcCall("LIST_FILTER", []any{this, expressions.Lambda(expressions.Args{
		"this": expressions.Not(expressions.Args{"this": expressions.Is(expressions.Args{
			"this":       expressions.Column(expressions.Args{"this": expressions.ToIdentifier("_u")}),
			"expression": expressions.Null(),
		})}),
		"expressions": []expressions.Expression{expressions.ToIdentifier("_u")},
	})}, "(", ")", true)
	filteredDistinct := g.funcCall("LIST_DISTINCT", []any{filtered}, "(", ")", true)
	appendNull := g.funcCall("LIST_APPEND", []any{filteredDistinct, expressions.Null()}, "(", ")", true)
	lenThis := g.funcCall("ARRAY_LENGTH", []any{this}, "(", ")", true)
	countThis := g.funcCall("LIST_COUNT", []any{this}, "(", ")", true)
	return "CASE WHEN " + lenThis + " <> " + countThis + " THEN " + appendNull + " ELSE " + distinct + " END"
}

// duckDBArrayToStringSQL ports arraytostring_sql (generators/duckdb.py:4100-4140).
func duckDBArrayToStringSQL(g *Generator, e expressions.Expression) string {
	delim := asExpression(e.Arg("expression"))
	this := e.This()
	if truthy(e.Arg("null_is_empty")) {
		transform := g.funcCall("LIST_TRANSFORM", []any{this, expressions.Lambda(expressions.Args{
			"this": expressions.Coalesce(expressions.Args{
				"this": expressions.Cast(expressions.Args{
					"this": expressions.ToIdentifier("x"), "to": expressions.DTypeText.IntoExpr(nil),
				}),
				"expressions": []expressions.Expression{expressions.LiteralString("")},
			}),
			"expressions": []expressions.Expression{expressions.ToIdentifier("x")},
		})}, "(", ")", true)
		base := g.funcCall("ARRAY_TO_STRING", []any{transform, delim}, "(", ")", true)
		if truthy(e.Arg("null_delim_is_null")) {
			return "CASE WHEN " + g.gen(delim) + " IS NULL THEN NULL ELSE " + base + " END"
		}
		return base
	}
	if nullText := asExpression(e.Arg("null")); nullText != nil {
		param := duckDBCollisionFreeName("x", nullText)
		transform := g.funcCall("LIST_TRANSFORM", []any{this, expressions.Lambda(expressions.Args{
			"this": expressions.Coalesce(expressions.Args{
				"this":        expressions.ToIdentifier(param),
				"expressions": []expressions.Expression{nullText},
			}),
			"expressions": []expressions.Expression{expressions.ToIdentifier(param)},
		})}, "(", ")", true)
		return g.funcCall("ARRAY_TO_STRING", []any{transform, delim}, "(", ")", true)
	}
	return g.funcCall("ARRAY_TO_STRING", []any{this, delim}, "(", ")", true)
}

// duckDBArraySliceSQL ports arrayslice_sql (generators/duckdb.py:3565-3595).
func duckDBArraySliceSQL(g *Generator, e expressions.Expression) string {
	start := asExpression(e.Arg("start"))
	end := asExpression(e.Arg("end"))
	if !truthy(e.Arg("zero_based")) {
		return g.funcCall("ARRAY_SLICE", []any{e.This(), start, end, e.Arg("step")}, "(", ")", true)
	}
	if start != nil {
		start = expressions.Case(expressions.Args{
			"ifs": []expressions.Expression{expressions.If(expressions.Args{
				"this": expressions.GTE(expressions.Args{"this": start, "expression": expressions.LiteralNumber("0")}),
				"true": expressions.Add(expressions.Args{"this": start, "expression": expressions.LiteralNumber("1")}),
			})},
			"default": start,
		})
	}
	if end != nil {
		end = expressions.Case(expressions.Args{
			"ifs": []expressions.Expression{expressions.If(expressions.Args{
				"this": expressions.LT(expressions.Args{"this": end, "expression": expressions.LiteralNumber("0")}),
				"true": expressions.Sub(expressions.Args{"this": end, "expression": expressions.LiteralNumber("1")}),
			})},
			"default": end,
		})
	}
	return g.funcCall("ARRAY_SLICE", []any{e.This(), start, end, e.Arg("step")}, "(", ")", true)
}

// duckDBArrayPositionSQL ports the ArrayPosition lambda (generators/duckdb.py:1616-1624).
func duckDBArrayPositionSQL(g *Generator, e expressions.Expression) string {
	base := g.funcCall("ARRAY_POSITION", []any{e.This(), e.Arg("expression")}, "(", ")", true)
	if truthy(e.Arg("zero_based")) {
		return base + " - 1"
	}
	return base
}

// duckDBArrayBagSQL ports the ARRAY_EXCEPT/ARRAY_INTERSECTION multiset emulation
// (generators/duckdb.py:3548-3563).
func duckDBArrayBagSQL(kind string) func(*Generator, expressions.Expression) string {
	return func(g *Generator, e expressions.Expression) string {
		var arr1, arr2 expressions.Expression
		if kind == "INTERSECT" {
			exprs := e.Expressions()
			if len(exprs) < 2 {
				return g.functionFallbackSQL(e)
			}
			arr1, arr2 = exprs[0], exprs[1]
		} else {
			arr1, arr2 = e.This(), asExpression(e.Arg("expression"))
		}
		if !truthy(e.Arg("is_multiset")) {
			if kind == "INTERSECT" {
				return g.functionFallbackSQL(e)
			}
			a1 := g.gen(arr1)
			a2 := g.gen(arr2)
			return "CASE WHEN " + a1 + " IS NULL OR " + a2 + " IS NULL THEN NULL ELSE LIST_FILTER(LIST_DISTINCT(" + a1 +
				"), e -> LENGTH(LIST_FILTER(" + a2 + ", x -> x IS NOT DISTINCT FROM e)) = 0) END"
		}
		a1 := g.gen(arr1)
		a2 := g.gen(arr2)
		cond := "LENGTH(LIST_FILTER(" + a1 + "[1:pair[2]], e -> e IS NOT DISTINCT FROM pair[1]))"
		op := ">"
		if kind == "INTERSECT" {
			op = "<="
		}
		pairs := "LIST_FILTER(LIST_ZIP(" + a1 + ", GENERATE_SERIES(1, LENGTH(" + a1 + "))), pair -> (" +
			cond + " " + op + " LENGTH(LIST_FILTER(" + a2 + ", e -> e IS NOT DISTINCT FROM pair[1]))))"
		body := "LIST_TRANSFORM(" + pairs + ", pair -> pair[1])"
		return "CASE WHEN " + a1 + " IS NULL OR " + a2 + " IS NULL THEN NULL ELSE " + body + " END"
	}
}

// duckDBAddMonthsSQL ports addmonths_sql (generators/duckdb.py:4356-4413).
func duckDBAddMonthsSQL(g *Generator, e expressions.Expression) string {
	this := e.This()
	months := asExpression(e.Arg("expression"))
	dtype, hasType := castResultDType(this)
	if this != nil && this.Kind() == expressions.KindLiteral && truthy(this.Arg("is_string")) {
		this = expressions.Cast(expressions.Args{"this": this, "to": expressions.DTypeTimestamp.IntoExpr(nil)})
	}
	isFloat := false
	if months != nil {
		switch months.Kind() {
		case expressions.KindLiteral:
			if months.IsNumber() && strings.Contains(months.Name(), ".") {
				isFloat = true
			}
		case expressions.KindNeg:
			if inner := asExpression(months.This()); inner != nil && inner.IsNumber() && strings.Contains(inner.Name(), ".") {
				isFloat = true
			}
		case expressions.KindCast:
			md, ok := castResultDType(months)
			isFloat = ok && (isFloatDataType(md) || md == expressions.DTypeDecimal || md == expressions.DTypeBigDecimal)
		}
	}
	var interval expressions.Expression
	if isFloat {
		interval = duckDBAnon("TO_MONTHS", expressions.Cast(expressions.Args{
			"this": expressions.Round(expressions.Args{"this": months}),
			"to":   expressions.DTypeInt.IntoExpr(nil),
		}))
	} else {
		intervalValue := months
		if months != nil && months.Kind() == expressions.KindNeg {
			intervalValue = expressions.Paren(expressions.Args{"this": months})
		}
		interval = expressions.Interval(expressions.Args{
			"this": intervalValue, "unit": expressions.Var(expressions.Args{"this": "MONTH"}),
		})
	}
	dateAdd := expressions.Add(expressions.Args{"this": this, "expression": interval})
	result := dateAdd
	if truthy(e.Arg("preserve_end_of_month")) {
		lastDay := duckDBAnon("LAST_DAY", this)
		result = expressions.Case(expressions.Args{
			"ifs": []expressions.Expression{expressions.If(expressions.Args{
				"this": expressions.EQ(expressions.Args{"this": lastDay, "expression": this}),
				"true": duckDBAnon("LAST_DAY", dateAdd),
			})},
			"default": dateAdd,
		})
	}
	if hasType && (dtype == expressions.DTypeDate || dtype == expressions.DTypeTimestampTz) {
		result = expressions.Cast(expressions.Args{"this": result, "to": expressions.DataType(expressions.Args{"this": dtype})})
	}
	return g.gen(result)
}

// duckDBRepeatSQL ports the Space lambda (generators/duckdb.py:3937-3944).
func duckDBRepeatSQL(g *Generator, e expressions.Expression) string {
	// generators/duckdb.py:3937-3944: REPEAT requires a BIGINT count.
	arg := expressions.Cast(expressions.Args{"this": e.This(), "to": expressions.DTypeBigInt.IntoExpr(nil)})
	return g.funcCall("REPEAT", []any{expressions.LiteralString(" "), arg}, "(", ")", true)
}

// duckDBApproxQuantileSQL ports approxquantile_sql (generators/duckdb.py:4677-4684).
func duckDBApproxQuantileSQL(g *Generator, e expressions.Expression) string {
	result := g.funcCall("APPROX_QUANTILE", []any{e.This(), e.Arg("quantile")}, "(", ")", true)
	dtype, ok := castResultDType(e)
	if ok && isFloatDataType(dtype) {
		return "CAST(" + result + " AS DOUBLE)"
	}
	return result
}

// duckDBJSONFormatSQL ports _json_format_sql (generators/duckdb.py:665-668).
func duckDBJSONFormatSQL(g *Generator, e expressions.Expression) string {
	sql := g.funcCall("TO_JSON", []any{e.This(), e.Arg("options")}, "(", ")", true)
	return "CAST(" + sql + " AS TEXT)"
}

// duckDBMapCatSQL ports mapcat_sql (generators/duckdb.py:3849-3855).
func duckDBMapCatSQL(g *Generator, e expressions.Expression) string {
	base := g.funcCall("MAP_CONCAT", []any{e.This(), e.Arg("expression")}, "(", ")", true)
	a := g.sqlKey(e, "this")
	b := g.sqlKey(e, "expression")
	return "CASE WHEN " + a + " IS NULL OR " + b + " IS NULL THEN NULL ELSE " + base + " END"
}

// duckDBMapContainsKeySQL ports mapcontainskey_sql (generators/duckdb.py:3857-3860).
func duckDBMapContainsKeySQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("ARRAY_CONTAINS", []any{
		duckDBAnon("MAP_KEYS", e.This()),
		e.Arg("expression"),
	}, "(", ")", true)
}

// duckDBMapDeleteSQL ports mapdelete_sql (generators/duckdb.py:3862-3876).
func duckDBMapDeleteSQL(g *Generator, e expressions.Expression) string {
	keys := e.Expressions()
	x := expressions.ToIdentifier("x")
	xDotKey := expressions.Dot(expressions.Args{
		"this": x.Copy(), "expression": expressions.ToIdentifier("key"),
	})
	lambda := expressions.Lambda(expressions.Args{
		"this":        expressions.Not(expressions.Args{"this": expressions.In(expressions.Args{"this": xDotKey, "expressions": keys})}),
		"expressions": []expressions.Expression{x},
	})
	filtered := g.funcCall("LIST_FILTER", []any{duckDBAnon("MAP_ENTRIES", e.This()), lambda}, "(", ")", true)
	return g.funcCall("MAP_FROM_ENTRIES", []any{filtered}, "(", ")", true)
}

// duckDBMapPickSQL ports mappick_sql (generators/duckdb.py:3878-3899).
func duckDBMapPickSQL(g *Generator, e expressions.Expression) string {
	keys := e.Expressions()
	x := expressions.ToIdentifier("x")
	xDotKey := expressions.Dot(expressions.Args{
		"this": x.Copy(), "expression": expressions.ToIdentifier("key"),
	})
	lambda := expressions.Lambda(expressions.Args{
		"this":        expressions.In(expressions.Args{"this": xDotKey, "expressions": keys}),
		"expressions": []expressions.Expression{x},
	})
	filtered := g.funcCall("LIST_FILTER", []any{duckDBAnon("MAP_ENTRIES", e.This()), lambda}, "(", ")", true)
	return g.funcCall("MAP_FROM_ENTRIES", []any{filtered}, "(", ")", true)
}

// duckDBMapSizeSQL ports mapsize_sql (generators/duckdb.py:3901-3903).
func duckDBMapSizeSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("CARDINALITY", []any{e.This()}, "(", ")", true)
}

// duckDBMapInsertSQL ports mapinsert_sql (generators/duckdb.py:3905-3928).
func duckDBMapInsertSQL(g *Generator, e expressions.Expression) string {
	mapArg := e.This()
	key := asExpression(e.Arg("key"))
	value := asExpression(e.Arg("value"))
	// generators/duckdb.py:3905-3928: cast the value to the MAP's declared value type.
	if value != nil && mapArg != nil && mapArg.Kind() == expressions.KindCast {
		if to := asExpression(mapArg.Arg("to")); to != nil {
			dtype, _ := to.Arg("this").(expressions.DType)
			params := to.Expressions()
			if dtype == expressions.DTypeMap && len(params) > 1 {
				value = expressions.Cast(expressions.Args{"this": value, "to": params[1]})
			}
		}
	}
	entry := expressions.ToMap(expressions.Args{
		"this": expressions.Struct(expressions.Args{"expressions": []expressions.Expression{
			expressions.PropertyEQ(expressions.Args{"this": key, "expression": value}),
		}}),
	})
	return g.funcCall("MAP_CONCAT", []any{mapArg, entry}, "(", ")", true)
}

// duckDBArrayUniqueAggSQL ports arrayuniqueagg_sql (generators/duckdb.py:3499-3505).
func duckDBArrayUniqueAggSQL(g *Generator, e expressions.Expression) string {
	this := e.This()
	list := g.funcCall("LIST", []any{expressions.Distinct(expressions.Args{
		"expressions": []expressions.Expression{this},
	})}, "(", ")", true)
	return list + " FILTER(WHERE NOT " + g.gen(this) + " IS NULL)"
}

// duckDBArrayConcatAggSQL ports arrayconcatagg_sql (generators/duckdb.py:3507-3522).
func duckDBArrayConcatAggSQL(g *Generator, e expressions.Expression) string {
	this := e.This()
	inner := this
	if this != nil && this.Kind() == expressions.KindOrder {
		inner = this.This()
	}
	agg := expressions.New(expressions.KindArrayAgg, expressions.Args{"this": this})
	filterSQL := g.gen(expressions.Filter(expressions.Args{
		"this": agg,
		"expression": expressions.Where(expressions.Args{"this": expressions.Not(expressions.Args{
			"this": expressions.Is(expressions.Args{"this": inner.Copy(), "expression": expressions.Null()}),
		})}),
	}))
	return g.funcCall("FLATTEN", []any{filterSQL}, "(", ")", true)
}

// duckDBArrayUnionAggSQL ports arrayunionagg_sql (generators/duckdb.py:3524-3526).
func duckDBArrayUnionAggSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("LIST_UNION_AGG", []any{e.This()}, "(", ")", true)
}

// duckDBBase64EncodeSQL ports base64encode_sql (generators/duckdb.py:3763-3799).
func duckDBBase64EncodeSQL(g *Generator, e expressions.Expression) string {
	base := g.funcCall("TO_BASE64", []any{e.This()}, "(", ")", true)
	if chars := asExpression(e.Arg("alphabet")); chars != nil && chars.IsString() {
		base = duckDBReplaceAlphabet(g, base, chars.Name(), false)
	}
	if maxLine := asExpression(e.Arg("max_line_length")); maxLine != nil && maxLine.IsInt() && maxLine.Name() != "0" {
		pattern := expressions.LiteralString("(.{" + maxLine.Name() + "})")
		replacement := expressions.DPipe(expressions.Args{
			"this":       expressions.LiteralString(`\1`),
			"expression": duckDBAnon("CHR", expressions.LiteralNumber("10")),
		})
		base = g.funcCall("RTRIM", []any{
			g.funcCall("REGEXP_REPLACE", []any{base, pattern, replacement, expressions.LiteralString("g")}, "(", ")", true),
			duckDBAnon("CHR", expressions.LiteralNumber("10")),
		}, "(", ")", true)
	}
	return base
}

// duckDBReplaceAlphabet applies the upstream base64 alphabet replacement chain.
func duckDBReplaceAlphabet(g *Generator, base, alphabet string, reverse bool) string {
	defaults := []string{"+", "/", "="}
	for i, r := range alphabet {
		if i >= len(defaults) {
			break
		}
		def := defaults[i]
		if string(r) == def {
			continue
		}
		src, dst := def, string(r)
		if reverse {
			src, dst = string(r), def
		}
		base = g.funcCall("REPLACE", []any{base, expressions.LiteralString(src), expressions.LiteralString(dst)}, "(", ")", true)
	}
	return base
}

// duckDBBase64DecodeSQL ports _base64_decode_sql (generators/duckdb.py:136-156).
func duckDBBase64DecodeSQL(toText bool) func(*Generator, expressions.Expression) string {
	return func(g *Generator, e expressions.Expression) string {
		thisSQL := g.gen(e.This())
		if chars := asExpression(e.Arg("alphabet")); chars != nil && chars.IsString() {
			thisSQL = duckDBReplaceAlphabet(g, thisSQL, chars.Name(), true)
		}
		fromBase64 := g.funcCall("FROM_BASE64", []any{thisSQL}, "(", ")", true)
		if toText {
			return "DECODE(" + fromBase64 + ")"
		}
		return fromBase64
	}
}

// duckDBConcatWsSQL ports concatws_sql (generators/duckdb.py:4142-4155).
func duckDBConcatWsSQL(g *Generator, e expressions.Expression) string {
	args := e.Expressions()
	var condParts []string
	for _, arg := range args {
		condParts = append(condParts, g.gen(expressions.Is(expressions.Args{"this": arg, "expression": expressions.Null()})))
	}
	callArgs := make([]any, 0, len(args))
	for _, expr := range args {
		callArgs = append(callArgs, expr)
	}
	base := g.funcCall("CONCAT_WS", callArgs, "(", ")", true)
	if truthy(e.Arg("coalesce")) {
		return base
	}
	return "CASE WHEN " + strings.Join(condParts, " OR ") + " THEN NULL ELSE " + base + " END"
}

// duckDBSplitPartSQL ports splitpart_sql (generators/duckdb.py:4031-4089) for the
// 1-based default form.
func duckDBSplitPartSQL(g *Generator, e expressions.Expression) string {
	delimiter := asExpression(e.Arg("delimiter"))
	partIndex := asExpression(e.Arg("part_index"))
	if partIndex != nil && partIndex.IsInt() && partIndex.Name() == "0" && truthy(e.Arg("part_index_zero_as_one")) {
		partIndex = expressions.LiteralNumber("1")
	}
	return g.funcCall("SPLIT_PART", []any{e.This(), delimiter, partIndex}, "(", ")", true)
}

// duckDBSplitSQL ports split_sql (generators/duckdb.py:4011-4029).
func duckDBSplitSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("STR_SPLIT", []any{e.This(), e.Arg("expression")}, "(", ")", true)
}

// duckDBStrtokToArraySQL ports strtoktoarray_sql (generators/duckdb.py:4658-4675).
func duckDBStrtokToArraySQL(g *Generator, e expressions.Expression) string {
	delim := asExpression(e.Arg("expression"))
	if delim == nil {
		delim = expressions.LiteralString(" ")
	}
	escaped := duckDBAnon("REGEXP_REPLACE",
		delim,
		expressions.LiteralString(`([\[\]^.\-*+?(){}|$\\])`),
		expressions.LiteralString(`\\\1`),
		expressions.LiteralString("g"),
	)
	pattern := expressions.Case(expressions.Args{
		"ifs": []expressions.Expression{expressions.If(expressions.Args{
			"this": expressions.EQ(expressions.Args{"this": delim.Copy(), "expression": expressions.LiteralString("")}),
			"true": expressions.LiteralString(".^"),
		})},
		"default": duckDBAnon("CONCAT", expressions.LiteralString("["), escaped, expressions.LiteralString("]")),
	})
	split := duckDBAnon("REGEXP_SPLIT_TO_ARRAY", e.This(), pattern)
	x := expressions.ToIdentifier("x")
	filtered := duckDBAnon("LIST_FILTER", split, expressions.Lambda(expressions.Args{
		"this": expressions.Not(expressions.Args{"this": expressions.EQ(expressions.Args{
			"this": x.Copy(), "expression": expressions.LiteralString(""),
		})}),
		"expressions": []expressions.Expression{x.Copy()},
	}))
	return g.gen(expressions.Case(expressions.Args{
		"ifs": []expressions.Expression{expressions.If(expressions.Args{
			"this": expressions.Is(expressions.Args{"this": delim.Copy(), "expression": expressions.Null()}),
			"true": expressions.Null(),
		})},
		"default": filtered,
	}))
}

// duckDBTruncSQL ports trunc_sql (generators/duckdb.py:2867-2876).
func duckDBTruncSQL(g *Generator, e expressions.Expression) string {
	decimals := asExpression(e.Arg("decimals"))
	return g.funcCall("TRUNC", []any{e.This(), decimals}, "(", ")", true)
}

// duckDBRoundSQL ports round_sql (generators/duckdb.py:4516-4537).
func duckDBRoundSQL(g *Generator, e expressions.Expression) string {
	this := e.This()
	decimals := asExpression(e.Arg("decimals"))
	truncate := asExpression(e.Arg("truncate"))
	if decimals != nil && truthy(e.Arg("casts_non_integer_decimals")) && !decimals.IsInt() {
		decimals = expressions.Cast(expressions.Args{"this": decimals, "to": expressions.DTypeInt.IntoExpr(nil)})
	}
	name := "ROUND"
	if truncate != nil {
		switch truncate.Name() {
		case "ROUND_HALF_EVEN", "HALF_TO_EVEN":
			name = "ROUND_EVEN"
			truncate = nil
		case "ROUND_HALF_AWAY_FROM_ZERO", "HALF_AWAY_FROM_ZERO":
			truncate = nil
		}
	}
	var args []any
	args = append(args, this)
	if decimals != nil {
		args = append(args, decimals)
	}
	if truncate != nil {
		args = append(args, truncate)
	}
	return g.funcCall(name, args, "(", ")", true)
}

// duckDBCeilFloorSQL ports _ceil_floor (generators/duckdb.py:1353-1358).
func duckDBCeilFloorSQL(fn string) func(*Generator, expressions.Expression) string {
	return func(g *Generator, e expressions.Expression) string {
		this := e.This()
		decimals := asExpression(e.Arg("decimals"))
		if decimals == nil || e.Arg("to") != nil {
			return g.funcCall(fn, []any{this}, "(", ")", true)
		}
		if this != nil && this.Is(expressions.TraitBinary) {
			this = expressions.Paren(expressions.Args{"this": this})
		}
		nInt := decimals
		if !decimals.IsInt() {
			nInt = expressions.Cast(expressions.Args{"this": decimals, "to": expressions.DTypeInt.IntoExpr(nil)})
		}
		power := expressions.Pow(expressions.Args{"this": expressions.LiteralNumber("10"), "expression": nInt})
		inner := expressions.Round(expressions.Args{
			"this": expressions.Div(expressions.Args{
				"this":       duckDBAnon(fn, expressions.Mul(expressions.Args{"this": this, "expression": power})),
				"expression": power,
			}),
			"decimals":                   decimals,
			"casts_non_integer_decimals": true,
		})
		return g.gen(inner)
	}
}

// duckDBNthValueSQL ports nthvalue_sql (generators/duckdb.py:2472-2477).
func duckDBNthValueSQL(g *Generator, e expressions.Expression) string {
	if v := e.Arg("from_first"); v != nil {
		if b, ok := v.(bool); ok && !b {
			g.unsupported("DuckDB's NTH_VALUE doesn't support starting from the end ")
		}
	}
	return g.functionFallbackSQL(e)
}

// duckDBTimeSliceSQL ports timeslice_sql (generators/duckdb.py:2317-2351).
func duckDBTimeSliceSQL(g *Generator, e expressions.Expression) string {
	length := e.Arg("expression")
	unit := asExpression(e.Arg("unit"))
	if unit != nil && (unit.Kind() == expressions.KindLiteral || unit.Kind() == expressions.KindVar) {
		unit = expressions.Var(expressions.Args{"this": strings.ToUpper(unit.Name())})
	}
	interval := expressions.Interval(expressions.Args{"this": length, "unit": unit})
	bucket := duckDBAnon("time_bucket", interval, e.This())
	if !strings.EqualFold(e.Text("kind"), "END") {
		return g.gen(bucket)
	}
	add := expressions.Add(expressions.Args{"this": bucket, "expression": interval})
	if dtype, ok := castResultDType(e.This()); ok && dtype == expressions.DTypeDate {
		return g.gen(expressions.Cast(expressions.Args{"this": add, "to": expressions.DTypeDate.IntoExpr(nil)}))
	}
	return g.gen(add)
}

// duckDBArraysZipSQL ports arrayszip_sql (generators/duckdb.py:3597-3648).
func duckDBArraysZipSQL(g *Generator, e expressions.Expression) string {
	args := e.Expressions()
	if len(args) == 0 {
		return "[" + g.gen(expressions.Map(expressions.Args{
			"keys": expressions.Array(expressions.Args{}), "values": expressions.Array(expressions.Args{}),
		})) + "]"
	}
	lengths := make([]expressions.Expression, len(args))
	for i, arg := range args {
		lengths[i] = expressions.Length(expressions.Args{"this": arg})
	}
	maxLen := lengths[0]
	if len(lengths) > 1 {
		maxLen = expressions.Greatest(expressions.Args{"this": lengths[0], "expressions": lengths[1:]})
	}
	var nullChecks, emptyChecks []expressions.Expression
	emptyStructArgs := make([]expressions.Expression, len(args))
	transformStructArgs := make([]expressions.Expression, len(args))
	index := expressions.Add(expressions.Args{
		"this":       expressions.Column(expressions.Args{"this": expressions.ToIdentifier("__i")}),
		"expression": expressions.LiteralNumber("1"),
	})
	for i, arg := range args {
		nullChecks = append(nullChecks, expressions.Is(expressions.Args{"this": arg, "expression": expressions.Null()}))
		emptyChecks = append(emptyChecks, expressions.EQ(expressions.Args{
			"this": expressions.Length(expressions.Args{"this": arg}), "expression": expressions.LiteralNumber("0"),
		}))
		emptyStructArgs[i] = expressions.PropertyEQ(expressions.Args{
			"this": expressions.LiteralString("$" + duckDBIntStr(i+1)), "expression": expressions.Null(),
		})
		coalesced := duckDBAnon("COALESCE", arg, expressions.Array(expressions.Args{}))
		transformStructArgs[i] = expressions.PropertyEQ(expressions.Args{
			"this": expressions.LiteralString("$" + duckDBIntStr(i+1)),
			"expression": expressions.Bracket(expressions.Args{
				"this": coalesced, "expressions": []expressions.Expression{index},
			}),
		})
	}
	emptyStruct := expressions.Struct(expressions.Args{"expressions": emptyStructArgs})
	transformStruct := expressions.Struct(expressions.Args{"expressions": transformStructArgs})
	listTransform := duckDBAnon("LIST_TRANSFORM",
		duckDBAnon("RANGE", expressions.LiteralNumber("0"), maxLen),
		expressions.Lambda(expressions.Args{
			"this":        transformStruct,
			"expressions": []expressions.Expression{expressions.ToIdentifier("__i")},
		}),
	)
	caseExpr := expressions.Case(expressions.Args{
		"ifs": []expressions.Expression{
			expressions.If(expressions.Args{
				"this": duckDBConnectorExpr(expressions.KindOr, nullChecks),
				"true": expressions.Null(),
			}),
			expressions.If(expressions.Args{
				"this": duckDBConnectorExpr(expressions.KindAnd, emptyChecks),
				"true": expressions.Array(expressions.Args{"expressions": []expressions.Expression{emptyStruct}}),
			}),
		},
		"default": listTransform,
	})
	return g.gen(caseExpr)
}

// duckDBBitmapBucketNumberSQL ports bitmapbucketnumber_sql (generators/duckdb.py:2353-2377).
func duckDBBitmapBucketNumberSQL(g *Generator, e expressions.Expression) string {
	value := e.This()
	positive := expressions.Add(expressions.Args{
		"this": expressions.Paren(expressions.Args{"this": expressions.IntDiv(expressions.Args{
			"this":       expressions.Paren(expressions.Args{"this": expressions.Sub(expressions.Args{"this": value, "expression": expressions.LiteralNumber("1")})}),
			"expression": expressions.LiteralNumber("32768"),
		})}),
		"expression": expressions.LiteralNumber("1"),
	})
	nonPositive := expressions.IntDiv(expressions.Args{
		"this": value, "expression": expressions.LiteralNumber("32768"),
	})
	return g.gen(expressions.Case(expressions.Args{
		"ifs": []expressions.Expression{expressions.If(expressions.Args{
			"this": expressions.GT(expressions.Args{"this": value, "expression": expressions.LiteralNumber("0")}),
			"true": positive,
		})},
		"default": nonPositive,
	}))
}

// duckDBBitmapBitPositionSQL ports bitmapbitposition_sql (generators/duckdb.py:2378-2400).
func duckDBBitmapBitPositionSQL(g *Generator, e expressions.Expression) string {
	value := e.This()
	inner := expressions.If(expressions.Args{
		"this":  expressions.GT(expressions.Args{"this": value, "expression": expressions.LiteralNumber("0")}),
		"true":  expressions.Sub(expressions.Args{"this": value, "expression": expressions.LiteralNumber("1")}),
		"false": expressions.Abs(expressions.Args{"this": value}),
	})
	return g.gen(expressions.Mod(expressions.Args{
		"this":       expressions.Paren(expressions.Args{"this": inner}),
		"expression": expressions.LiteralNumber("32768"),
	}))
}

// duckDBValidateRegexpFlags ports _validate_regexp_flags (generators/duckdb.py:3306-3335).
func duckDBValidateRegexpFlags(g *Generator, flags expressions.Expression, supported string) string {
	if flags == nil || !flags.IsString() {
		if flags != nil {
			g.unsupported("Non-literal regexp flags are not fully supported in DuckDB")
		}
		return ""
	}
	raw := flags.Name()
	var out strings.Builder
	for _, r := range raw {
		if strings.ContainsRune(supported, r) {
			out.WriteRune(r)
		}
	}
	return out.String()
}

// duckDBRegexpLikeSQL ports regexplike_sql (generators/duckdb.py:3419-3430).
func duckDBRegexpLikeSQL(g *Generator, e expressions.Expression) string {
	flag := duckDBValidateRegexpFlags(g, asExpression(e.Arg("flag")), "cims")
	name := "REGEXP_MATCHES"
	if truthy(e.Arg("full_match")) {
		name = "REGEXP_FULL_MATCH"
	}
	args := []any{e.This(), e.Arg("expression")}
	if flag != "" {
		args = append(args, expressions.LiteralString(flag))
	}
	return g.funcCall(name, args, "(", ")", true)
}

// duckDBRegexpReplaceSQL ports regexpreplace_sql (generators/duckdb.py:3368-3417).
func duckDBRegexpReplaceSQL(g *Generator, e expressions.Expression) string {
	subject := e.This()
	pattern := e.Arg("expression")
	replacement := asExpression(e.Arg("replacement"))
	if replacement == nil {
		replacement = expressions.LiteralString("")
	}
	position := asExpression(e.Arg("position"))
	occurrence := asExpression(e.Arg("occurrence"))
	flags := duckDBValidateRegexpFlags(g, asExpression(e.Arg("modifiers")), "cimsg")

	occ := 0
	if occurrence != nil && occurrence.IsInt() {
		if v, err := strconv.Atoi(occurrence.Name()); err == nil {
			occ = v
		}
	}
	if occurrence != nil && !occurrence.IsInt() {
		g.unsupported("REGEXP_REPLACE with non-literal occurrence")
	} else if occ > 1 {
		g.unsupported("REGEXP_REPLACE occurrence > 1 is not supported in DuckDB")
	} else if occ == 0 && !strings.Contains(flags, "g") && !truthy(e.Arg("single_replace")) {
		flags += "g"
	}

	var prefix string
	if position != nil && !position.IsInt() {
		g.unsupported("REGEXP_REPLACE with non-literal position")
	} else if position != nil && position.IsInt() {
		if pos, err := strconv.Atoi(position.Name()); err == nil && pos > 1 {
			prefix = g.funcCall("SUBSTRING", []any{subject, expressions.LiteralNumber("1"), expressions.LiteralNumber(duckDBIntStr(pos - 1))}, "(", ")", true)
			subject = duckDBAnon("SUBSTRING", subject, expressions.LiteralNumber(duckDBIntStr(pos)))
		}
	}
	args := []any{subject, pattern, replacement}
	if flags != "" {
		args = append(args, expressions.LiteralString(flags))
	}
	result := g.funcCall("REGEXP_REPLACE", args, "(", ")", true)
	if prefix != "" {
		return prefix + " || " + result
	}
	return result
}

// duckDBRegexpCountSQL ports regexpcount_sql (generators/duckdb.py:3336-3366).
func duckDBRegexpCountSQL(g *Generator, e expressions.Expression) string {
	this := e.This()
	pattern := asExpression(e.Arg("expression"))
	position := asExpression(e.Arg("position"))
	flags := duckDBValidateRegexpFlags(g, asExpression(e.Arg("parameters")), "ims")
	if position != nil {
		this = expressions.Substring(expressions.Args{"this": this, "start": position})
	}
	if flags != "" {
		pattern = expressions.Concat(expressions.Args{"expressions": []expressions.Expression{
			expressions.LiteralString("(?" + flags + ")"), pattern,
		}})
	}
	return g.gen(expressions.Case(expressions.Args{
		"ifs": []expressions.Expression{expressions.If(expressions.Args{
			"this": expressions.EQ(expressions.Args{"this": pattern, "expression": expressions.LiteralString("")}),
			"true": expressions.LiteralNumber("0"),
		})},
		"default": expressions.Length(expressions.Args{
			"this": duckDBAnon("REGEXP_EXTRACT_ALL", this, pattern),
		}),
	}))
}

// duckDBDateTruncSQL ports datetrunc_sql/datetimetrunc_sql/timestamptrunc_sql
// (generators/duckdb.py:4427-4506).
func duckDBDateTruncSQL(g *Generator, e expressions.Expression) string {
	unitExpr := asExpression(e.Arg("unit"))
	date := e.This()
	weekStart := duckDBWeekUnitStart(unitExpr)
	unit := duckDBUnitToStr(unitExpr)
	var result string
	if e.Kind() == expressions.KindDatetimeTrunc {
		date = expressions.Cast(expressions.Args{"this": date, "to": expressions.DTypeDatetime.IntoExpr(nil)})
	}
	// BigQuery TIMESTAMP_TRUNC with a zone truncates in that timezone and returns UTC.
	if zone := asExpression(e.Arg("zone")); zone != nil && duckDBIsDateUnit(unit) {
		shifted := expressions.AtTimeZone(expressions.Args{"this": date, "zone": zone})
		var truncated expressions.Expression
		if weekStart > 0 {
			truncated = duckDBWeekTruncExpr(shifted, weekStart, true, false)
		} else {
			truncated = expressions.Anonymous(expressions.Args{"this": "DATE_TRUNC", "expressions": []expressions.Expression{
				asExpression(duckDBUnitToStr(asExpression(e.Arg("unit")))), shifted,
			}})
		}
		if truncated.Kind() == expressions.KindAdd {
			truncated = expressions.Paren(expressions.Args{"this": truncated})
		}
		return g.gen(expressions.AtTimeZone(expressions.Args{"this": truncated, "zone": zone}))
	}
	if weekStart > 0 {
		result = g.gen(duckDBWeekTruncExpr(date, weekStart, true, e.Kind() == expressions.KindDateTrunc))
	} else {
		result = g.funcCall("DATE_TRUNC", []any{unit, date}, "(", ")", true)
	}
	if truthy(e.Arg("input_type_preserved")) {
		if dtype, ok := castResultDType(date); ok {
			if dtype == expressions.DTypeTime || dtype == expressions.DTypeTimeTz {
				dummyDate := expressions.Cast(expressions.Args{
					"this": expressions.LiteralString("1970-01-01"),
					"to":   expressions.DTypeDate.IntoExpr(nil),
				})
				dateTime := expressions.Add(expressions.Args{"this": dummyDate, "expression": date})
				truncated := g.funcCall("DATE_TRUNC", []any{unit, dateTime}, "(", ")", true)
				return "CAST(" + truncated + " AS " + g.gen(expressions.DataType(expressions.Args{"this": dtype})) + ")"
			}
			if e.Kind() == expressions.KindDateTrunc && dtype == expressions.DTypeDate && duckDBIsDateUnit(unit) {
				return result
			}
			return "CAST(" + result + " AS " + g.gen(expressions.DataType(expressions.Args{"this": dtype})) + ")"
		}
	}
	return result
}

// duckDBUnitToStr ports unit_to_str for a literal/var unit.
func duckDBUnitToStr(unit expressions.Expression) any {
	if unit == nil {
		return nil
	}
	switch unit.Kind() {
	case expressions.KindLiteral, expressions.KindVar, expressions.KindColumn,
		expressions.KindIdentifier, expressions.KindWeekStart:
		if name := unit.Name(); name != "" {
			return expressions.LiteralString(strings.ToUpper(name))
		}
	}
	return unit
}

func duckDBIsDateUnit(unit any) bool {
	s, ok := unit.(expressions.Expression)
	if !ok || s == nil || (s.Kind() != expressions.KindLiteral && s.Kind() != expressions.KindVar) {
		return false
	}
	switch strings.ToUpper(s.Name()) {
	case "YEAR", "QUARTER", "MONTH", "WEEK", "DAY":
		return true
	}
	return false
}

// duckDBWeekUnitStart ports _week_trunc_start_dow for literal/var ISOWEEK and WeekStart.
func duckDBWeekUnitStart(unit expressions.Expression) int {
	if unit == nil {
		return 0
	}
	if unit.Kind() == expressions.KindLiteral || unit.Kind() == expressions.KindVar {
		switch strings.ToUpper(unit.Name()) {
		case "WEEK", "ISOWEEK":
			return 1
		}
		return 0
	}
	if unit.Kind() == expressions.KindWeekStart {
		return duckDBWeekStartDow(unit, "")
	}
	return 0
}

// duckDBWeekTruncExpr ports _build_week_trunc_expression (generators/duckdb.py:910-951).
func duckDBWeekTruncExpr(date expressions.Expression, startDow int, preserveStartDay, castToDate bool) expressions.Expression {
	shift := 1 - startDow
	if startDow == 7 {
		shift = 1
	}
	truncated := expressions.Anonymous(expressions.Args{
		"this": "DATE_TRUNC",
		"expressions": []expressions.Expression{
			expressions.LiteralString("WEEK"), date,
		},
	})
	if shift == 0 {
		return truncated
	}
	interval := expressions.Interval(expressions.Args{
		"this": expressions.LiteralString(duckDBIntStr(shift)),
		"unit": expressions.Var(expressions.Args{"this": "DAY"}),
	})
	shifted := expressions.Add(expressions.Args{"this": date, "expression": interval})
	truncated.Set("expressions", []expressions.Expression{expressions.LiteralString("WEEK"), shifted})
	if !preserveStartDay {
		return truncated
	}
	back := expressions.Interval(expressions.Args{
		"this": expressions.LiteralString(duckDBIntStr(-shift)),
		"unit": expressions.Var(expressions.Args{"this": "DAY"}),
	})
	shiftedBack := expressions.Add(expressions.Args{"this": truncated, "expression": back})
	if castToDate {
		return expressions.Cast(expressions.Args{"this": shiftedBack, "to": expressions.DTypeDate.IntoExpr(nil)})
	}
	return shiftedBack
}

// duckDBGenerateTimestampArraySQL ports _generate_datetime_array_sql for
// GenerateTimestampArray (generators/duckdb.py:981-1001).
func duckDBGenerateTimestampArraySQL(g *Generator, e expressions.Expression) string {
	start := implicitDatetimeCast(asExpression(e.Arg("start")))
	end := implicitDatetimeCast(asExpression(e.Arg("end")))
	return g.gen(expressions.GenerateSeries(expressions.Args{
		"start": start, "end": end, "step": e.Arg("step"),
	}))
}

// duckDBJSONExtractScalarSQL ports jsonextractscalar_sql (generators/duckdb.py:4721-4726).
func duckDBJSONExtractScalarSQL(g *Generator, e expressions.Expression) string {
	if truthy(e.Arg("scalar_only")) {
		inner := duckDBAnon("JSON_VALUE", e.This(), asExpression(e.Arg("expression")))
		node := expressions.New(expressions.KindJSONExtractScalar, expressions.Args{
			"this": inner, "expression": expressions.LiteralString("$"),
		})
		return duckDBArrowJSONExtract(g, node)
	}
	return duckDBArrowJSONExtract(g, e)
}

// duckDBCurrentDateSQL ports currentdate_sql (generators/duckdb.py:2819-2827).
func duckDBCurrentDateSQL(g *Generator, e expressions.Expression) string {
	if e.This() == nil {
		return "CURRENT_DATE"
	}
	return g.gen(expressions.Cast(expressions.Args{
		"this": expressions.AtTimeZone(expressions.Args{
			"this": expressions.CurrentTimestamp(expressions.Args{}), "zone": e.This(),
		}),
		"to": expressions.DTypeDate.IntoExpr(nil),
	}))
}

// duckDBDateToDiSQL ports the DateToDi lambda (generators/duckdb.py:1700-1702).
func duckDBDateToDiSQL(g *Generator, e expressions.Expression) string {
	return "CAST(STRFTIME(" + g.sqlKey(e, "this") + ", '%Y%m%d') AS INT)"
}

// duckDBTimeToTimeStrSQL ports the TimeToTimeStr lambda (generators/duckdb.py:1750-1752).
func duckDBTimeToTimeStrSQL(g *Generator, e expressions.Expression) string {
	return "CAST(" + g.sqlKey(e, "this") + " AS TEXT)"
}

// duckDBCollisionFreeName returns base unless a column in exprs already binds it.
func duckDBCollisionFreeName(base string, exprs ...expressions.Expression) string {
	used := map[string]bool{}
	for _, expr := range exprs {
		if expr == nil {
			continue
		}
		for _, col := range expr.FindAll(expressions.KindColumn) {
			used[col.Name()] = true
		}
	}
	name := base
	for used[name] {
		name = "_" + name
	}
	return name
}

// duckDBConnectorExpr folds a slice into And/Or nodes, returning the single node
// unchanged when there is only one (so no dangling operator is rendered).
func duckDBConnectorExpr(kind expressions.Kind, exprs []expressions.Expression) expressions.Expression {
	if len(exprs) == 0 {
		return nil
	}
	result := exprs[0]
	for _, expr := range exprs[1:] {
		result = expressions.New(kind, expressions.Args{"this": result, "expression": expr})
	}
	return result
}

// duckDBGeneratorSQL ports generator_sql (generators/duckdb.py:2619-2631).
func duckDBGeneratorSQL(g *Generator, e expressions.Expression) string {
	if e.Arg("timelimit") != nil {
		g.unsupported("GENERATOR TIMELIMIT parameter is not supported in DuckDB")
	}
	rowcount := asExpression(e.Arg("rowcount"))
	if rowcount == nil {
		g.unsupported("GENERATOR without ROWCOUNT is not supported in DuckDB")
		return g.funcCall("range", []any{expressions.LiteralNumber("0")}, "(", ")", true)
	}
	return g.funcCall("range", []any{rowcount}, "(", ")", true)
}

// duckDBTableFromRowsSQL ports tablefromrows_sql (generators/duckdb.py:3946-3957): a
// Generator is unwrapped to RANGE, preserving the alias and joins.
func duckDBTableFromRowsSQL(g *Generator, e expressions.Expression) string {
	this := asExpression(e.This())
	var base string
	if this != nil && this.Kind() == expressions.KindGenerator {
		base = g.gen(this)
	} else {
		base = "TABLE(" + g.gen(this) + ")"
	}
	if alias := g.sqlKey(e, "alias"); alias != "" {
		base += " AS " + alias
	}
	base += g.expressions(exprsOptions{expression: e, key: "pivots", sep: "", flat: true})
	base += g.sqlKey(e, "sample")
	base += g.expressions(exprsOptions{expression: e, key: "joins", sep: "", flat: true})
	return base
}

// duckDBGreatestLeastSQL ports _greatest_least_sql (generators/duckdb.py:2595-2617).
func duckDBGreatestLeastSQL(g *Generator, e expressions.Expression) string {
	fallback := g.functionFallbackSQL(e)
	if truthy(e.Arg("ignore_nulls")) {
		return fallback
	}
	all := append([]expressions.Expression{e.This()}, e.Expressions()...)
	var conds []expressions.Expression
	for _, arg := range all {
		if arg == nil {
			continue
		}
		conds = append(conds, expressions.Is(expressions.Args{"this": arg, "expression": expressions.Null()}))
	}
	cond := duckDBConnectorExpr(expressions.KindOr, conds)
	if cond == nil {
		return fallback
	}
	return "CASE WHEN " + g.gen(cond) + " THEN NULL ELSE " + fallback + " END"
}

// duckDBUnnestSQL ports unnest_sql (generators/duckdb.py:3959-3979). BigQuery's
// explode_array UNNEST gains max_depth => 2 and is wrapped in a subquery so the struct
// fields are exploded rather than the array flattened.
func duckDBUnnestSQL(g *Generator, e expressions.Expression) string {
	if !truthy(e.Arg("explode_array")) {
		return g.unnestSQL(e)
	}
	exprs := e.Expressions()
	exprs = append(exprs, expressions.Kwarg(expressions.Args{
		"this":       expressions.Var(expressions.Args{"this": "max_depth"}),
		"expression": expressions.LiteralNumber("2"),
	}))
	e.Set("expressions", exprs)
	alias := asExpression(e.Arg("alias"))
	if alias != nil && alias.Kind() == expressions.KindTableAlias {
		e.Set("alias", nil)
		if cols, _ := alias.Arg("columns").([]expressions.Expression); len(cols) > 0 {
			alias = expressions.TableAlias(expressions.Args{"this": cols[0]})
		}
	}
	inner := g.unnestSQL(e)
	out := "(SELECT " + inner + ")"
	if alias != nil {
		out += " AS " + g.gen(alias)
	}
	return out
}

// duckDBExtractSQL ports extract_sql (generators/duckdb.py:2994-3057).
func duckDBExtractSQL(g *Generator, e expressions.Expression) string {
	part := ""
	if unit := asExpression(e.Arg("this")); unit != nil {
		part = strings.ToUpper(unit.Name())
	}
	datetimeExpr := asExpression(e.Arg("expression"))
	strftimeMappings := map[string][2]string{
		"WEEKISO":       {"%V", "INT"},
		"YEAROFWEEK":    {"%G", "INT"},
		"YEAROFWEEKISO": {"%G", "INT"},
		"NANOSECOND":    {"%n", "BIGINT"},
	}
	if mapping, ok := strftimeMappings[part]; ok {
		fmt, castType := mapping[0], mapping[1]
		isNanoTime := part == "NANOSECOND"
		if isNanoTime {
			if dtype, ok := castResultDType(datetimeExpr); ok && (dtype == expressions.DTypeTime || dtype == expressions.DTypeTimeTz) {
				g.unsupported("Parameter NANOSECOND is not supported with TIME type in DuckDB")
				inner := duckDBExtract("MICROSECOND", datetimeExpr)
				mul := expressions.Mul(expressions.Args{"this": inner, "expression": expressions.LiteralNumber("1000")})
				return "CAST(" + g.gen(mul) + " AS " + castType + ")"
			}
			datetimeExpr = expressions.Cast(expressions.Args{"this": datetimeExpr, "to": expressions.DTypeTimestampNs.IntoExpr(nil)})
		}
		strftime := g.funcCall("STRFTIME", []any{datetimeExpr, expressions.LiteralString(fmt)}, "(", ")", true)
		return "CAST(" + strftime + " AS " + castType + ")"
	}
	epochMappings := map[string]string{
		"EPOCH_SECOND": "EPOCH", "EPOCH_MILLISECOND": "EPOCH_MS",
		"EPOCH_MICROSECOND": "EPOCH_US", "EPOCH_NANOSECOND": "EPOCH_NS",
	}
	if fn, ok := epochMappings[part]; ok {
		result := g.funcCall(fn, []any{datetimeExpr}, "(", ")", true)
		if part == "EPOCH_SECOND" {
			return "CAST(" + result + " AS BIGINT)"
		}
		return result
	}
	return g.extractSQL(e)
}

// duckDBInSQL ports in_sql (generators/duckdb.py:3135-3143) for `x IN UNNEST(arr)`.
func duckDBInSQL(g *Generator, e expressions.Expression) string {
	unnest := asExpression(e.Arg("unnest"))
	if unnest == nil {
		return g.inSQL(e)
	}
	var arr expressions.Expression
	if exprs := unnest.Expressions(); len(exprs) > 0 {
		arr = exprs[0]
	}
	value := e.This()
	a := g.gen(arr)
	v := g.gen(value)
	return "CASE WHEN " + a + " IS NULL OR ARRAY_LENGTH(" + a + ") = 0 THEN FALSE WHEN ARRAY_CONTAINS(" + a + ", " + v + ") THEN TRUE WHEN " +
		v + " IS NULL OR ARRAY_LENGTH(" + a + ") <> LIST_COUNT(" + a + ") THEN NULL ELSE FALSE END"
}

// duckDBRegexpExtractSQL ports _regexp_extract_sql (generators/duckdb.py:4157-4208).
func duckDBRegexpExtractSQL(g *Generator, e expressions.Expression) string {
	this := e.This()
	group := asExpression(e.Arg("group"))
	params := asExpression(e.Arg("parameters"))
	position := asExpression(e.Arg("position"))
	occurrence := asExpression(e.Arg("occurrence"))
	nullIfOverflow := truthy(e.Arg("null_if_pos_overflow"))

	if params != nil && params.IsString() && strings.Contains(params.Name(), "e") {
		params = expressions.LiteralString(strings.ReplaceAll(params.Name(), "e", ""))
	}
	flags := ""
	if params != nil && params.IsString() {
		flags = duckDBValidateRegexpFlags(g, params, "cims")
	}
	if group != nil && group.IsInt() && group.Name() == "0" && flags == "" {
		group = nil
	}
	if position != nil && (!position.IsInt() || duckDBIntLiteralGT1(position)) {
		this = expressions.Substring(expressions.Args{"this": this, "start": position})
		if nullIfOverflow {
			this = expressions.Nullif(expressions.Args{"this": this, "expression": expressions.LiteralString("")})
		}
	}
	isExtractAll := e.Kind() == expressions.KindRegexpExtractAll
	nonSingleOccurrence := occurrence != nil && (!occurrence.IsInt() || duckDBIntLiteralGT1(occurrence))
	name := "REGEXP_EXTRACT"
	if isExtractAll || nonSingleOccurrence {
		name = "REGEXP_EXTRACT_ALL"
	}
	argNodes := []expressions.Expression{this, asExpression(e.Arg("expression"))}
	if group != nil {
		argNodes = append(argNodes, group)
	}
	if flags != "" {
		argNodes = append(argNodes, expressions.LiteralString(flags))
	}
	callNode := expressions.Anonymous(expressions.Args{"this": name, "expressions": argNodes})
	if isExtractAll && nonSingleOccurrence {
		return g.gen(expressions.Bracket(expressions.Args{
			"this":        callNode,
			"expressions": []expressions.Expression{expressions.Slice(expressions.Args{"this": occurrence})},
		}))
	}
	if nonSingleOccurrence {
		return g.funcCall("ARRAY_EXTRACT", []any{callNode, occurrence}, "(", ")", true)
	}
	return g.gen(callNode)
}

func duckDBIntLiteralGT1(e expressions.Expression) bool {
	if e == nil || !e.IsInt() {
		return false
	}
	v, err := strconv.Atoi(e.Name())
	return err == nil && v > 1
}

// duckDBArrayRemoveAtSQL ports _array_remove_at_sql (generators/duckdb.py:454-542).
func duckDBArrayRemoveAtSQL(g *Generator, e expressions.Expression) string {
	this := e.This()
	position := asExpression(e.Arg("position"))
	pos, ok := duckDBIntLiteral(position)
	if !ok {
		g.unsupported("ARRAY_REMOVE_AT can only be transpiled with a literal position")
		return g.funcCall("ARRAY_REMOVE_AT", []any{this, position}, "(", ")", true)
	}
	var result string
	switch {
	case pos == 0:
		result = g.gen(expressions.Bracket(expressions.Args{
			"this":        this,
			"expressions": []expressions.Expression{expressions.Slice(expressions.Args{"this": expressions.LiteralNumber("2")})},
		}))
	case pos > 0:
		left := g.gen(expressions.Bracket(expressions.Args{
			"this": this,
			"expressions": []expressions.Expression{expressions.Slice(expressions.Args{
				"this": expressions.LiteralNumber("1"), "expression": expressions.LiteralNumber(duckDBIntStr(pos)),
			})},
		}))
		right := g.gen(expressions.Bracket(expressions.Args{
			"this":        this,
			"expressions": []expressions.Expression{expressions.Slice(expressions.Args{"this": expressions.LiteralNumber(duckDBIntStr(pos + 2))})},
		}))
		result = g.funcCall("LIST_CONCAT", []any{left, right}, "(", ")", true)
	case pos == -1:
		arrLen := expressions.Length(expressions.Args{"this": this})
		result = g.gen(expressions.Bracket(expressions.Args{
			"this": this,
			"expressions": []expressions.Expression{expressions.Slice(expressions.Args{
				"this": expressions.LiteralNumber("1"),
				"expression": expressions.Add(expressions.Args{
					"this": arrLen, "expression": expressions.LiteralNumber("-1"),
				}),
			})},
		}))
	default:
		arrLen := expressions.Length(expressions.Args{"this": this})
		sliceEnd := expressions.Add(expressions.Args{"this": arrLen.Copy(), "expression": expressions.LiteralNumber(duckDBIntStr(pos))})
		sliceStart := expressions.Add(expressions.Args{"this": arrLen.Copy(), "expression": expressions.LiteralNumber(duckDBIntStr(pos + 2))})
		left := expressions.Bracket(expressions.Args{
			"this": this,
			"expressions": []expressions.Expression{expressions.Slice(expressions.Args{
				"this": expressions.LiteralNumber("1"), "expression": sliceEnd,
			})},
		})
		right := expressions.Bracket(expressions.Args{
			"this":        this,
			"expressions": []expressions.Expression{expressions.Slice(expressions.Args{"this": sliceStart})},
		})
		result = g.funcCall("LIST_CONCAT", []any{left, right}, "(", ")", true)
	}
	if pos < 0 {
		// A negative position past the start keeps the source array unchanged instead of
		// producing two overlapping slices. A literal array's length is known here, so
		// in-range literals keep the plain slice output.
		abs := -pos
		needsGuard := true
		if this != nil && this.Kind() == expressions.KindArray {
			needsGuard = len(this.Expressions()) < abs
		}
		if needsGuard {
			result = "CASE WHEN " + g.gen(expressions.Length(expressions.Args{"this": this})) + " < " +
				duckDBIntStr(abs) + " THEN " + g.gen(this) + " ELSE " + result + " END"
		}
	}
	return "CASE WHEN " + g.gen(expressions.Is(expressions.Args{"this": this, "expression": expressions.Null()})) + " THEN NULL ELSE " + result + " END"
}

// duckDBPosexplodeSQL ports posexplode_sql (generators/duckdb.py:4318-4354).
func duckDBPosexplodeSQL(g *Generator, e expressions.Expression) string {
	this := e.This()
	pos := expressions.ToIdentifier("pos")
	col := expressions.ToIdentifier("col")
	parent := e.Parent()
	if parent != nil && parent.Kind() == expressions.KindAliases {
		exprs := parent.Expressions()
		if len(exprs) >= 2 {
			pos, col = exprs[0], exprs[1]
		}
	} else if parent != nil && parent.Kind() == expressions.KindTable {
		if alias := asExpression(parent.Arg("alias")); alias != nil {
			if cols, _ := alias.Arg("columns").([]expressions.Expression); len(cols) > 0 {
				if len(cols) >= 2 {
					pos, col = cols[0], cols[1]
				} else {
					col = cols[0]
				}
			}
			parent.Set("alias", nil)
		}
	}
	unnestSQL := g.gen(expressions.Unnest(expressions.Args{
		"expressions": []expressions.Expression{this}, "alias": col,
	}))
	genSub := expressions.Sub(expressions.Args{
		"this":       duckDBAnon("GENERATE_SUBSCRIPTS", this, expressions.LiteralNumber("1")),
		"expression": expressions.LiteralNumber("1"),
	})
	genAlias := g.gen(expressions.New(expressions.KindAlias, expressions.Args{"this": genSub, "alias": pos}))
	body := genAlias + ", " + unnestSQL
	if parent != nil && (parent.Kind() == expressions.KindFrom ||
		(parent.Parent() != nil && parent.Parent().Kind() == expressions.KindFrom)) {
		return "(SELECT " + body + ")"
	}
	return body
}

// duckDBDecodeCaseSQL ports decodecase_sql (generator.py:6143-6167) with
// SUPPORTS_DECODE_CASE=false (generators/duckdb.py:1576).
func duckDBDecodeCaseSQL(g *Generator, e expressions.Expression) string {
	exprs := e.Expressions()
	if len(exprs) == 0 {
		return ""
	}
	decodeExpr := exprs[0]
	var ifs []expressions.Expression
	for i := 1; i+1 < len(exprs); i += 2 {
		search, result := exprs[i], exprs[i+1]
		if search.Kind() == expressions.KindLiteral {
			ifs = append(ifs, expressions.If(expressions.Args{
				"this": expressions.EQ(expressions.Args{"this": decodeExpr, "expression": search}),
				"true": result,
			}))
			continue
		}
		if search.Kind() == expressions.KindNull {
			ifs = append(ifs, expressions.If(expressions.Args{
				"this": expressions.Is(expressions.Args{"this": decodeExpr, "expression": expressions.Null()}),
				"true": result,
			}))
			continue
		}
		if search.Is(expressions.TraitBinary) {
			search = expressions.Paren(expressions.Args{"this": search})
		}
		cond := expressions.Or(expressions.Args{
			"this": expressions.EQ(expressions.Args{"this": decodeExpr, "expression": search}),
			"expression": expressions.Paren(expressions.Args{"this": expressions.And(expressions.Args{
				"this":       expressions.Is(expressions.Args{"this": decodeExpr, "expression": expressions.Null()}),
				"expression": expressions.Is(expressions.Args{"this": search, "expression": expressions.Null()}),
			})}),
		})
		ifs = append(ifs, expressions.If(expressions.Args{"this": cond, "true": result}))
	}
	args := expressions.Args{"ifs": ifs}
	if len(exprs)%2 == 0 {
		args["default"] = exprs[len(exprs)-1]
	}
	return g.gen(expressions.Case(args))
}

// duckDBSeqSQL ports _seq_sql (generators/duckdb.py:827-846).
func duckDBSeqSQL(byteWidth int) func(*Generator, expressions.Expression) string {
	return func(g *Generator, e expressions.Expression) string {
		maxVals := map[int]string{1: "256", 2: "65536", 4: "4294967296", 8: "18446744073709551616"}
		maxVal := expressions.LiteralNumber(maxVals[byteWidth])

		// _seq_to_range_in_generator: inside TABLE(GENERATOR(...)) the range column is the
		// same sequence, so emit `range % max` instead of a nested window function.
		if duckDBInGeneratorSelect(e) {
			return g.gen(expressions.Mod(expressions.Args{
				"this":       expressions.Column(expressions.Args{"this": expressions.ToIdentifier("range")}),
				"expression": maxVal,
			}))
		}
		base := expressions.Paren(expressions.Args{"this": expressions.Sub(expressions.Args{
			"this": expressions.Window(expressions.Args{
				"this": expressions.RowNumber(expressions.Args{}),
				"order": expressions.Order(expressions.Args{
					"expressions": []expressions.Expression{expressions.Ordered(expressions.Args{
						"this": expressions.LiteralNumber("1"), "nulls_first": true,
					})},
				}),
			}),
			"expression": expressions.LiteralNumber("1"),
		})})
		return g.gen(expressions.Mod(expressions.Args{"this": base, "expression": maxVal}))
	}
}

// duckDBInGeneratorSelect reports whether e is projected by a SELECT whose FROM is a
// TABLE(GENERATOR(...)) rows source.
func duckDBInGeneratorSelect(e expressions.Expression) bool {
	parent := e.Parent()
	for parent != nil {
		if parent.Kind() == expressions.KindSelect {
			from := asExpression(parent.Arg("from_"))
			if from == nil {
				return false
			}
			tfr := asExpression(from.This())
			if tfr != nil && tfr.Kind() == expressions.KindTable {
				tfr = asExpression(tfr.This())
			}
			if tfr == nil || tfr.Kind() != expressions.KindTableFromRows {
				return false
			}
			gen := asExpression(tfr.This())
			return gen != nil && gen.Kind() == expressions.KindGenerator
		}
		parent = parent.Parent()
	}
	return false
}

// duckDBToArraySQL ports the ToArray lambda (generators/duckdb.py:1739).
func duckDBToArraySQL(g *Generator, e expressions.Expression) string {
	this := e.This()
	return g.gen(expressions.Case(expressions.Args{
		"ifs": []expressions.Expression{expressions.If(expressions.Args{
			"this": expressions.Is(expressions.Args{"this": this, "expression": expressions.Null()}),
			"true": expressions.Null(),
		})},
		"default": expressions.Array(expressions.Args{"expressions": []expressions.Expression{this}}),
	}))
}

// duckDBTimeFromPartsSQL ports timefromparts_sql (generators/duckdb.py:2950-2992).
func duckDBTimeFromPartsSQL(g *Generator, e expressions.Expression) string {
	hour := asExpression(e.Arg("hour"))
	minute := asExpression(e.Arg("min"))
	sec := asExpression(e.Arg("sec"))
	nano := asExpression(e.Arg("nano"))
	if !truthy(e.Arg("overflow")) {
		if nano != nil {
			sec = expressions.Add(expressions.Args{
				"this":       sec,
				"expression": expressions.Div(expressions.Args{"this": nano, "expression": expressions.LiteralNumber("1000000000.0")}),
			})
		}
		return g.funcCall("MAKE_TIME", []any{hour, minute, sec}, "(", ")", true)
	}
	isInt := func(x expressions.Expression) bool { return x != nil && x.IsInt() }
	if nano == nil && isInt(hour) && isInt(minute) && isInt(sec) {
		h, herr := strconv.Atoi(hour.Name())
		m, merr := strconv.Atoi(minute.Name())
		secs, serr := strconv.Atoi(sec.Name())
		if herr == nil && merr == nil && serr == nil &&
			h >= 0 && h <= 23 && m >= 0 && m <= 59 && secs >= 0 && secs <= 59 {
			return g.funcCall("MAKE_TIME", []any{hour, minute, sec}, "(", ")", true)
		}
	}
	if nano != nil {
		sec = expressions.Add(expressions.Args{
			"this":       sec,
			"expression": expressions.Div(expressions.Args{"this": nano, "expression": expressions.LiteralNumber("1000000000.0")}),
		})
	}
	hoursMinutes := expressions.Add(expressions.Args{
		"this":       expressions.Paren(expressions.Args{"this": expressions.Mul(expressions.Args{"this": hour, "expression": expressions.LiteralNumber("3600")})}),
		"expression": expressions.Paren(expressions.Args{"this": expressions.Mul(expressions.Args{"this": minute, "expression": expressions.LiteralNumber("60")})}),
	})
	total := expressions.Add(expressions.Args{"this": hoursMinutes, "expression": sec})
	return g.gen(expressions.Add(expressions.Args{
		"this": expressions.Cast(expressions.Args{
			"this": expressions.LiteralString("00:00:00"), "to": expressions.DTypeTime.IntoExpr(nil),
		}),
		"expression": expressions.Interval(expressions.Args{
			"this": total, "unit": expressions.Var(expressions.Args{"this": "SECOND"}),
		}),
	}))
}

// duckDBNumberToStrSQL ports numbertostr_sql (generators/duckdb.py:4299-4305).
func duckDBNumberToStrSQL(g *Generator, e expressions.Expression) string {
	fmt := asExpression(e.Arg("format"))
	if fmt != nil && fmt.IsInt() {
		template := expressions.LiteralString("{:,." + fmt.Name() + "f}")
		return g.funcCall("FORMAT", []any{template, e.This()}, "(", ")", true)
	}
	g.unsupported("Only integer formats are supported by NumberToStr")
	return g.functionFallbackSQL(e)
}

// duckDBRegexpILikeSQL ports the RegexpILike lambda (generators/duckdb.py:1757-1759).
func duckDBRegexpILikeSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("REGEXP_MATCHES", []any{
		e.This(), e.Arg("expression"), expressions.LiteralString("i"),
	}, "(", ")", true)
}

// duckDBDayNavigationSQL ports _day_navigation_sql (generators/duckdb.py:1055-1120).
func duckDBDayNavigationSQL(g *Generator, e expressions.Expression) string {
	dateExpr := e.This()
	dayName := asExpression(e.Arg("expression"))
	isodow := duckDBAnon("ISODOW", dateExpr)
	var target expressions.Expression
	weekDays := map[string]int{"MONDAY": 1, "TUESDAY": 2, "WEDNESDAY": 3, "THURSDAY": 4, "FRIDAY": 5, "SATURDAY": 6, "SUNDAY": 7}
	if dayName != nil && dayName.Kind() == expressions.KindLiteral {
		name := strings.ToUpper(dayName.Name())
		found := 0
		for day, dow := range weekDays {
			if strings.HasPrefix(day, name) {
				found = dow
				break
			}
		}
		if found == 0 {
			return g.functionFallbackSQL(e)
		}
		target = expressions.LiteralNumber(strconv.Itoa(found))
	} else if dayName != nil {
		upper := duckDBUpper(dayName)
		var ifs []expressions.Expression
		for day, dow := range weekDays {
			ifs = append(ifs, expressions.If(expressions.Args{
				"this": duckDBAnon("STARTS_WITH", upper.Copy(), expressions.LiteralString(day[:2])),
				"true": expressions.LiteralNumber(strconv.Itoa(dow)),
			}))
		}
		target = expressions.Case(expressions.Args{"ifs": ifs})
	} else {
		return g.functionFallbackSQL(e)
	}
	var offset expressions.Expression
	if e.Kind() == expressions.KindNextDay {
		offset = expressions.Add(expressions.Args{
			"this": expressions.Paren(expressions.Args{"this": expressions.Mod(expressions.Args{
				"this": expressions.Paren(expressions.Args{"this": expressions.Add(expressions.Args{
					"this":       expressions.Paren(expressions.Args{"this": expressions.Sub(expressions.Args{"this": target, "expression": isodow})}),
					"expression": expressions.LiteralNumber("6"),
				})}),
				"expression": expressions.LiteralNumber("7"),
			})}),
			"expression": expressions.LiteralNumber("1"),
		})
		dateExpr = expressions.Add(expressions.Args{"this": dateExpr, "expression": expressions.Interval(expressions.Args{
			"this": offset, "unit": expressions.Var(expressions.Args{"this": "DAY"}),
		})})
	} else {
		offset = expressions.Add(expressions.Args{
			"this": expressions.Paren(expressions.Args{"this": expressions.Mod(expressions.Args{
				"this": expressions.Paren(expressions.Args{"this": expressions.Add(expressions.Args{
					"this":       expressions.Paren(expressions.Args{"this": expressions.Sub(expressions.Args{"this": isodow, "expression": target})}),
					"expression": expressions.LiteralNumber("6"),
				})}),
				"expression": expressions.LiteralNumber("7"),
			})}),
			"expression": expressions.LiteralNumber("1"),
		})
		dateExpr = expressions.Sub(expressions.Args{"this": dateExpr, "expression": expressions.Interval(expressions.Args{
			"this": offset, "unit": expressions.Var(expressions.Args{"this": "DAY"}),
		})})
	}
	return g.gen(expressions.Cast(expressions.Args{"this": dateExpr, "to": expressions.DTypeDate.IntoExpr(nil)}))
}

// duckDBLateralSQL ports _explode_to_unnest_sql (generators/duckdb.py:1497-1524).
func duckDBLateralSQL(g *Generator, e expressions.Expression) string {
	this := asExpression(e.This())
	if this == nil || this.Kind() != expressions.KindExplode {
		return g.lateralSQL(e)
	}
	alias := asExpression(e.Arg("alias"))
	var exprs []expressions.Expression
	if x := this.This(); x != nil {
		exprs = append(exprs, x)
	}
	unnest := expressions.Unnest(expressions.Args{"expressions": exprs})
	if alias != nil {
		unnest.Set("alias", alias)
	}
	return " CROSS JOIN " + g.gen(unnest)
}

// duckDBTsOrDsToDatetimeSQL ports tsordstodatetime_sql (generator.py:5195-5200).
func duckDBTsOrDsToDatetimeSQL(g *Generator, e expressions.Expression) string {
	this := asExpression(e.This())
	if this != nil && this.Kind() == expressions.KindCast {
		if dtype, ok := castResultDType(this); ok {
			switch dtype {
			case expressions.DTypeDatetime, expressions.DTypeTimestamp, expressions.DTypeTimestampTz, expressions.DTypeTimestampLtz:
				return g.gen(this)
			}
		}
	}
	return g.gen(expressions.Cast(expressions.Args{"this": this, "to": expressions.DTypeDatetime.IntoExpr(nil)}))
}

// duckDBGetExtractSQL ports getextract_sql (generator.py:6187-6200): arrays index directly
// (1-based in DuckDB), anything else falls back to a JSON extraction.
func duckDBGetExtractSQL(g *Generator, e expressions.Expression) string {
	this := asExpression(e.This())
	expr := asExpression(e.Arg("expression"))
	if this != nil && this.Kind() == expressions.KindCast {
		if to := asExpression(this.Arg("to")); to != nil {
			if dtype, _ := to.Arg("this").(expressions.DType); dtype == expressions.DTypeMap {
				return g.gen(expressions.Bracket(expressions.Args{
					"this": this, "expressions": []expressions.Expression{expr},
				}))
			}
		}
	}
	if this != nil && this.Kind() == expressions.KindArray {
		index := expr
		if expr != nil && expr.IsInt() {
			if v, err := strconv.Atoi(expr.Name()); err == nil {
				index = expressions.LiteralNumber(strconv.Itoa(v + 1))
			}
		}
		return g.gen(expressions.Bracket(expressions.Args{
			"this":        this,
			"expressions": []expressions.Expression{index},
		}))
	}
	return g.gen(expressions.JSONExtract(expressions.Args{"this": this, "expression": expr}))
}

// duckDBFilterSQL ports filter_sql (generators/duckdb.py:4754-4758) for CORR.
func duckDBFilterSQL(g *Generator, e expressions.Expression) string {
	base := g.filterSQL(e)
	if corr := asExpression(e.This()); corr == nil || corr.Kind() != expressions.KindCorr || !truthy(corr.Arg("null_on_zero_variance")) {
		return base
	}
	if parent := e.Parent(); parent != nil && parent.Kind() == expressions.KindWindow {
		return base
	}
	return "CASE WHEN ISNAN(" + base + ") THEN NULL ELSE " + base + " END"
}

// duckDBWindowSQL ports window_sql (generators/duckdb.py:4745-4752) for CORR.
func duckDBWindowSQL(g *Generator, e expressions.Expression) string {
	base := g.windowSQL(e)
	this := asExpression(e.This())
	if this != nil && this.Kind() == expressions.KindFilter {
		this = asExpression(this.This())
	}
	if this == nil || this.Kind() != expressions.KindCorr || !truthy(this.Arg("null_on_zero_variance")) {
		return base
	}
	return "CASE WHEN ISNAN(" + base + ") THEN NULL ELSE " + base + " END"
}

// duckDBNormalSQL ports normal_sql (generators/duckdb.py:2878-2901).
func duckDBNormalSQL(g *Generator, e expressions.Expression) string {
	mean := e.This()
	stddev := asExpression(e.Arg("stddev"))
	gen := asExpression(e.Arg("gen"))
	var u1, u2 string
	if gen != nil && gen.Kind() == expressions.KindRand && gen.This() == nil {
		u1, u2 = "RANDOM()", "RANDOM()"
	} else {
		seed := gen
		if gen != nil && gen.Kind() == expressions.KindRand {
			seed = asExpression(gen.This())
		}
		u1 = duckDBSeededRandom(g, seed)
		u2 = duckDBSeededRandom(g, expressions.Add(expressions.Args{"this": seed, "expression": expressions.LiteralNumber("1")}))
	}
	return g.gen(mean) + " + (" + g.gen(stddev) + " * SQRT(-2 * LN(GREATEST(" + u1 + ", 1e-10))) * COS(2 * PI() * " + u2 + "))"
}

// duckDBSeededRandom renders (ABS(HASH(seed)) % 1000000) / 1000000.0.
func duckDBSeededRandom(g *Generator, seed expressions.Expression) string {
	return "(ABS(HASH(" + g.gen(seed) + ")) % 1000000) / 1000000.0"
}

// duckDBUniformSQL ports uniform_sql (generators/duckdb.py:2904-2948).
func duckDBUniformSQL(g *Generator, e expressions.Expression) string {
	minVal := asExpression(e.This())
	maxVal := asExpression(e.Arg("expression"))
	gen := asExpression(e.Arg("gen"))
	isInt := minVal != nil && minVal.IsInt() && maxVal != nil && maxVal.IsInt()
	var random string
	if gen == nil || gen.Kind() != expressions.KindRand {
		random = duckDBSeededRandom(g, gen)
	} else {
		random = "RANDOM()"
	}
	rangeSQL := g.gen(maxVal) + " - " + g.gen(minVal)
	if isInt {
		rangeSQL += " + 1"
	}
	result := g.gen(minVal) + " + " + random + " * (" + rangeSQL + ")"
	if isInt {
		return "CAST(FLOOR(" + result + ") AS BIGINT)"
	}
	return result
}

// duckDBRandstrSQL ports randstr_sql (generators/duckdb.py:2479-2504).
func duckDBRandstrSQL(g *Generator, e expressions.Expression) string {
	length := e.This()
	gen := asExpression(e.Arg("generator"))
	var seed expressions.Expression
	if gen != nil && gen.Kind() == expressions.KindRand {
		seed = asExpression(gen.This())
		if seed == nil {
			seed = gen
		}
	} else if gen != nil {
		seed = gen
	} else {
		seed = expressions.LiteralNumber("123456")
	}
	pool := expressions.LiteralString("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz")
	inner := duckDBAnon("SUBSTRING", pool,
		expressions.Add(expressions.Args{"this": expressions.LiteralNumber("1"), "expression": expressions.Cast(expressions.Args{
			"this": duckDBAnon("FLOOR", expressions.Mul(expressions.Args{"this": expressions.Column(expressions.Args{"this": expressions.ToIdentifier("random_value")}), "expression": expressions.LiteralNumber("62")})),
			"to":   expressions.DTypeInt.IntoExpr(nil),
		})}),
		expressions.LiteralNumber("1"),
	)
	listagg := g.funcCall("LISTAGG", []any{inner, expressions.LiteralString("")}, "(", ")", true)
	seedSQL := g.gen(seed)
	randomValue := "(ABS(HASH(i + " + seedSQL + ")) % 1000) / 1000.0"
	from := "SELECT " + randomValue + " AS random_value FROM RANGE(" + g.gen(length) + ") AS t(i)"
	return "(SELECT " + listagg + " FROM (" + from + "))"
}

// duckDBStripNullValueSQL ports stripnullvalue_sql (generators/duckdb.py:2860-2866).
func duckDBStripNullValueSQL(g *Generator, e expressions.Expression) string {
	this := g.sqlKey(e, "this")
	return "CASE WHEN JSON_TYPE(" + this + ") = 'NULL' THEN NULL ELSE " + this + " END"
}

// duckDBTsOrDsToTimeSQL ports tsordstotime_sql (generators/duckdb.py:2802-2817) for the
// simple cast form.
func duckDBTsOrDsToTimeSQL(g *Generator, e expressions.Expression) string {
	safe := truthy(e.Arg("safe"))
	cast := "CAST"
	if safe {
		cast = "TRY_CAST"
	}
	if asExpression(e.Arg("format")) != nil {
		name := "STRPTIME"
		if safe {
			name = "TRY_STRPTIME"
		}
		parsed := g.funcCall(name, []any{e.This(), g.duckDBFormatTime(e)}, "(", ")", true)
		return cast + "(" + parsed + " AS TIME)"
	}
	return g.gen(expressions.New(map[bool]expressions.Kind{true: expressions.KindTryCast, false: expressions.KindCast}[safe],
		expressions.Args{"this": e.This(), "to": expressions.DTypeTime.IntoExpr(nil)}))
}

// duckDBTsOrDsToTimestampSQL ports tsordstotimestamp_sql (generator.py:5195 equivalent):
// a plain cast to TIMESTAMP.
func duckDBTsOrDsToTimestampSQL(g *Generator, e expressions.Expression) string {
	return g.gen(expressions.Cast(expressions.Args{"this": e.This(), "to": expressions.DTypeTimestamp.IntoExpr(nil)}))
}

// duckDBStarMapSQL renders OBJECT_CONSTRUCT(*) forms: the empty/whole-row map becomes MAP(),
// and a flat key/value argument list becomes a DuckDB struct literal.
func duckDBStarMapSQL(g *Generator, e expressions.Expression) string {
	exprs := e.Expressions()
	if len(exprs) == 0 {
		return "MAP()"
	}
	var items []string
	for i := 0; i+1 < len(exprs); i += 2 {
		key := expKeySQL(g, exprs[i])
		items = append(items, key+": "+g.gen(exprs[i+1]))
	}
	if len(items) == 0 {
		return "MAP()"
	}
	return "{" + strings.Join(items, ", ") + "}"
}

// expKeySQL renders an OBJECT_CONSTRUCT key: string literals as-is, identifiers quoted as
// string keys.
func expKeySQL(g *Generator, key expressions.Expression) string {
	if key == nil {
		return ""
	}
	if key.Kind() == expressions.KindLiteral && key.IsString() {
		return g.gen(key)
	}
	return g.gen(expressions.LiteralString(key.Name()))
}

// duckDBStrPositionSQL ports strposition_sql (generators/duckdb.py:2699-2727) for the
// clamp_position form, delegating the rest to the shared transpiler.
func duckDBStrPositionSQL(g *Generator, e expressions.Expression) string {
	if truthy(e.Arg("clamp_position")) {
		if position := asExpression(e.Arg("position")); position != nil {
			clamped := expressions.If(expressions.Args{
				"this":  expressions.LTE(expressions.Args{"this": position, "expression": expressions.LiteralNumber("0")}),
				"true":  expressions.LiteralNumber("1"),
				"false": position.Copy(),
			})
			clone := expressions.New(expressions.KindStrPosition, expressions.Args{
				"this":     e.This(),
				"substr":   e.Arg("substr"),
				"position": clamped,
			})
			return strPositionTranspileSQL("STRPOS", false, false)(g, clone)
		}
	}
	return strPositionTranspileSQL("STRPOS", false, false)(g, e)
}

// duckDBParseTimeSQL ports parsetime_sql (generators/duckdb.py:2793-2800).
func duckDBParseTimeSQL(g *Generator, e expressions.Expression) string {
	return g.gen(expressions.Cast(expressions.Args{
		"this": duckDBAnon("STRPTIME", e.This(), g.duckDBFormatTimeExpr(e)),
		"to":   expressions.DTypeTime.IntoExpr(nil),
	}))
}

// duckDBRegexpInstrSQL ports regexpinstr_sql (generators/duckdb.py:4216-4297).
func duckDBRegexpInstrSQL(g *Generator, e expressions.Expression) string {
	this := e.This()
	pattern := asExpression(e.Arg("expression"))
	position := asExpression(e.Arg("position"))
	origOcc := asExpression(e.Arg("occurrence"))
	occurrence := origOcc
	if occurrence == nil {
		occurrence = expressions.LiteralNumber("1")
	}
	option := asExpression(e.Arg("option"))
	params := asExpression(e.Arg("parameters"))

	if flags := duckDBValidateRegexpFlags(g, params, "ims"); flags != "" {
		pattern = expressions.Concat(expressions.Args{"expressions": []expressions.Expression{
			expressions.LiteralString("(?" + flags + ")"), pattern,
		}})
	}
	posOffset := expressions.LiteralNumber("0")
	if position != nil && (!position.IsInt() || duckDBIntLiteralGT1(position)) {
		this = expressions.Substring(expressions.Args{"this": this, "start": position})
		posOffset = expressions.Sub(expressions.Args{"this": position, "expression": expressions.LiteralNumber("1")})
	}
	sumLengths := func(funcName string, end expressions.Expression) expressions.Expression {
		lst := expressions.Bracket(expressions.Args{
			"this": duckDBAnon(funcName, this.Copy(), pattern.Copy()),
			"expressions": []expressions.Expression{expressions.Slice(expressions.Args{
				"this": expressions.LiteralNumber("1"), "expression": end,
			})},
		})
		transform := duckDBAnon("LIST_TRANSFORM", lst, expressions.Lambda(expressions.Args{
			"this":        expressions.Length(expressions.Args{"this": expressions.ToIdentifier("x")}),
			"expressions": []expressions.Expression{expressions.ToIdentifier("x")},
		}))
		return expressions.Coalesce(expressions.Args{
			"this":        duckDBAnon("LIST_SUM", transform),
			"expressions": []expressions.Expression{expressions.LiteralNumber("0")},
		})
	}
	basePos := expressions.Add(expressions.Args{
		"this": expressions.Add(expressions.Args{
			"this": expressions.Add(expressions.Args{
				"this":       expressions.LiteralNumber("1"),
				"expression": sumLengths("STRING_SPLIT_REGEX", occurrence),
			}),
			"expression": sumLengths("REGEXP_EXTRACT_ALL", expressions.Sub(expressions.Args{
				"this": occurrence, "expression": expressions.LiteralNumber("1"),
			})),
		}),
		"expression": posOffset,
	})
	if option != nil && option.IsInt() && option.Name() == "1" {
		matchAtOcc := expressions.Bracket(expressions.Args{
			"this":        duckDBAnon("REGEXP_EXTRACT_ALL", this.Copy(), pattern.Copy()),
			"expressions": []expressions.Expression{occurrence},
		})
		basePos = expressions.Add(expressions.Args{
			"this": basePos,
			"expression": expressions.Coalesce(expressions.Args{
				"this":        expressions.Length(expressions.Args{"this": matchAtOcc}),
				"expressions": []expressions.Expression{expressions.LiteralNumber("0")},
			}),
		})
	}
	var nullChecks []expressions.Expression
	for _, arg := range []expressions.Expression{e.This(), pattern, position, origOcc, option, params} {
		if arg != nil {
			nullChecks = append(nullChecks, expressions.Is(expressions.Args{"this": arg.Copy(), "expression": expressions.Null()}))
		}
	}
	matches := duckDBAnon("REGEXP_EXTRACT_ALL", this.Copy(), pattern.Copy())
	lengthMatches := expressions.Length(expressions.Args{"this": matches})
	caseExpr := expressions.Case(expressions.Args{
		"ifs": []expressions.Expression{
			expressions.If(expressions.Args{
				"this": duckDBConnectorExpr(expressions.KindOr, nullChecks),
				"true": expressions.Null(),
			}),
			expressions.If(expressions.Args{
				"this": expressions.EQ(expressions.Args{"this": pattern.Copy(), "expression": expressions.LiteralString("")}),
				"true": expressions.LiteralNumber("0"),
			}),
			expressions.If(expressions.Args{
				"this": expressions.LT(expressions.Args{"this": lengthMatches, "expression": occurrence}),
				"true": expressions.LiteralNumber("0"),
			}),
		},
		"default": basePos,
	})
	return g.gen(caseExpr)
}
