package generator

import (
	"strings"

	"github.com/maltzsama/sqlglot-go/dialects"
	"github.com/maltzsama/sqlglot-go/expressions"
)

// DuckDB recreates the DuckDBGenerator TRANSFORMS/TYPE_MAPPING surface
// (generators/duckdb.py:1590-1940) for the parts that change same-dialect output.
func init() {
	registerDialectTypeMapping("duckdb", map[expressions.DType]string{
		expressions.DTypeBinary:       "BLOB",
		expressions.DTypeBpchar:       "TEXT",
		expressions.DTypeChar:         "TEXT",
		expressions.DTypeDatetime:     "TIMESTAMP",
		expressions.DTypeDecFloat:     "DECIMAL",
		expressions.DTypeFloat:        "REAL",
		expressions.DTypeJSONB:        "JSON",
		expressions.DTypeNChar:        "TEXT",
		expressions.DTypeNVarchar:     "TEXT",
		expressions.DTypeUInt:         "UINTEGER",
		expressions.DTypeVarBinary:    "BLOB",
		expressions.DTypeRowVersion:   "BLOB",
		expressions.DTypeVarchar:      "TEXT",
		expressions.DTypeTimestampLtz: "TIMESTAMPTZ",
		expressions.DTypeTimestampNtz: "TIMESTAMP",
		expressions.DTypeTimestampS:   "TIMESTAMP_S",
		expressions.DTypeTimestampMs:  "TIMESTAMP_MS",
		expressions.DTypeTimestampNs:  "TIMESTAMP_NS",
		expressions.DTypeBigDecimal:   "DECIMAL",
	})

	handlers := map[expressions.Kind]func(*Generator, expressions.Expression) string{
		expressions.KindAnyValue:          duckDBAnyValueSQL,
		expressions.KindApproxDistinct:    clickhouseRename("APPROX_COUNT_DISTINCT"),
		expressions.KindBitwiseCount:      clickhouseRename("BIT_COUNT"),
		expressions.KindCosineDistance:    clickhouseRename("LIST_COSINE_DISTANCE"),
		expressions.KindCurrentVersion:    clickhouseRename("version"),
		expressions.KindDayOfMonth:        clickhouseRename("DAYOFMONTH"),
		expressions.KindDayOfWeek:         clickhouseRename("DAYOFWEEK"),
		expressions.KindDayOfWeekIso:      clickhouseRename("ISODOW"),
		expressions.KindDayOfYear:         clickhouseRename("DAYOFYEAR"),
		expressions.KindEuclideanDistance: clickhouseRename("LIST_DISTANCE"),
		expressions.KindExplode:           clickhouseRename("UNNEST"),
		expressions.KindIsNan:             clickhouseRename("ISNAN"),
		expressions.KindPercentileCont:    clickhouseRename("QUANTILE_CONT"),
		expressions.KindPercentileDisc:    clickhouseRename("QUANTILE_DISC"),
		expressions.KindRegexpSplit:       clickhouseRename("STR_SPLIT_REGEX"),
		expressions.KindTransform:         clickhouseRename("LIST_TRANSFORM"),
		expressions.KindArraySum:          clickhouseRename("LIST_SUM"),
		expressions.KindArrayMax:          clickhouseRename("LIST_MAX"),
		expressions.KindArrayMin:          clickhouseRename("LIST_MIN"),
		expressions.KindArrayFilter:       clickhouseRename("LIST_FILTER"),
		expressions.KindTimeToUnix:        clickhouseRename("EPOCH"),
		expressions.KindWeekOfYear:        clickhouseRename("WEEKOFYEAR"),
		expressions.KindVariancePop:       clickhouseRename("VAR_POP"),
		expressions.KindJSONObjectAgg:     clickhouseRename("JSON_GROUP_OBJECT"),
		expressions.KindDateBin:           clickhouseRename("TIME_BUCKET"),
		expressions.KindArray:             duckDBArraySQL,
		expressions.KindStruct:            duckDBStructSQL,
		expressions.KindJSONExtract:       duckDBArrowJSONExtract,
		expressions.KindJSONExtractScalar: duckDBArrowJSONExtract,
		expressions.KindJSONExtractArray:  duckDBArrowJSONExtractArray,
		expressions.KindJSONValueArray:    duckDBArrowJSONExtractArray,
		expressions.KindArrayAppend:       duckDBArrayAppendSQL("LIST_APPEND", false),
		expressions.KindArrayPrepend:      duckDBArrayAppendSQL("LIST_PREPEND", true),
		expressions.KindArrayConcat:       duckDBArrayConcatSQL,
		expressions.KindArrayContains:     duckDBArrayContainsSQL,
		expressions.KindArrayOverlaps:     duckDBArrayOverlapsSQL,
		expressions.KindBitwiseAnd:        func(g *Generator, e expressions.Expression) string { return g.binary(e, "&") },
		expressions.KindBitwiseOr:         func(g *Generator, e expressions.Expression) string { return g.binary(e, "|") },
		expressions.KindIntDiv:            func(g *Generator, e expressions.Expression) string { return g.binary(e, "//") },
		expressions.KindCurrentTime:       func(*Generator, expressions.Expression) string { return "CURRENT_TIME" },
		expressions.KindCurrentTimestamp: func(g *Generator, e expressions.Expression) string {
			return "CURRENT_TIMESTAMP"
		},
		expressions.KindLocaltime:             func(*Generator, expressions.Expression) string { return "LOCALTIME" },
		expressions.KindLocaltimestamp:        func(*Generator, expressions.Expression) string { return "LOCALTIMESTAMP" },
		expressions.KindMakeInterval:          duckDBMakeIntervalSQL,
		expressions.KindGroupConcat:           duckDBGroupConcatSQL,
		expressions.KindGenerateSeries:        duckDBGenerateSeriesSQL,
		expressions.KindLastDay:               duckDBLastDaySQL,
		expressions.KindTimeToStr:             duckDBTimeToStrSQL,
		expressions.KindStrToTime:             duckDBStrToTimeSQL,
		expressions.KindStrToDate:             duckDBStrToTimeSQL,
		expressions.KindDecode:                duckDBEncodeDecodeSQL("DECODE"),
		expressions.KindEncode:                duckDBEncodeDecodeSQL("ENCODE"),
		expressions.KindUnixToTime:            duckDBUnixToTimeSQL,
		expressions.KindUnixSeconds:           duckDBUnixSecondsSQL,
		expressions.KindDateDiff:              duckDBDateDiffSQL,
		expressions.KindDatetimeDiff:          duckDBDateDiffSQL,
		expressions.KindTsOrDsDiff:            duckDBTsOrDsDiffSQL,
		expressions.KindTimestampTrunc:        duckDBTimestampTruncSQL,
		expressions.KindDateTrunc:             duckDBTimestampTruncSQL,
		expressions.KindSortArray:             duckDBSortArraySQL,
		expressions.KindTimestampFromParts:    duckDBTimestampFromPartsSQL,
		expressions.KindDateFromParts:         duckDBDateFromPartsSQL,
		expressions.KindJarowinklerSimilarity: clickhouseRename("JARO_WINKLER_SIMILARITY"),
		expressions.KindRegexpLike:            clickhouseRename("REGEXP_MATCHES"),
		expressions.KindArrayCompact:          duckDBArrayCompactSQL,
		expressions.KindArrayConstructCompact: duckDBArrayConstructCompactSQL,
		expressions.KindAttach:                duckDBAttachSQL,
		expressions.KindDetach:                duckDBDetachSQL,
		expressions.KindAttachOption:          duckDBAttachOptionSQL,
		expressions.KindInstall:               duckDBInstallSQL,
		expressions.KindDataType:              duckDBDataTypeSQL,
		expressions.KindBitwiseAndAgg:         duckDBBitwiseAggSQL("BIT_AND"),
		expressions.KindBitwiseOrAgg:          duckDBBitwiseAggSQL("BIT_OR"),
		expressions.KindBitwiseXorAgg:         duckDBBitwiseAggSQL("BIT_XOR"),
		expressions.KindParseJSON:             duckDBParseJSONSQL,
		expressions.KindArrayDistinct:         clickhouseRename("LIST_DISTINCT"),
		expressions.KindIgnoreNulls:           duckDBIgnoreNullsSQL,
		expressions.KindTableSample:           duckDBTableSampleSQL,
		expressions.KindLambda:                duckDBLambdaSQL,
		expressions.KindGetbit:                clickhouseRename("GET_BIT"),
		expressions.KindToVariant:             duckDBToVariantSQL,
		expressions.KindLogicalOr:             duckDBLogicalSQL("BOOL_OR"),
		expressions.KindLogicalAnd:            duckDBLogicalSQL("BOOL_AND"),
		expressions.KindDateAdd:               duckDBDateDeltaSQL("+"),
		expressions.KindDateSub:               duckDBDateDeltaSQL("-"),
		expressions.KindDatetimeAdd:           duckDBDateDeltaSQL("+"),
		expressions.KindDatetimeSub:           duckDBDateDeltaSQL("-"),
		expressions.KindTimeAdd:               duckDBDateDeltaSQL("+"),
		expressions.KindTimeSub:               duckDBDateDeltaSQL("-"),
		expressions.KindTimestampAdd:          duckDBDateDeltaSQL("+"),
		expressions.KindTimestampSub:          duckDBDateDeltaSQL("-"),
		expressions.KindTsOrDsAdd:             duckDBDateDeltaSQL("+"),
		expressions.KindPositionalColumn:      func(g *Generator, e expressions.Expression) string { return "#" + g.sqlKey(e, "this") },
		expressions.KindUnpivotColumns: func(g *Generator, e expressions.Expression) string {
			return "NAME " + g.sqlKey(e, "this") + " VALUE " + g.expressions(exprsOptions{expression: e, flat: true})
		},
		expressions.KindSummarize: func(g *Generator, e expressions.Expression) string {
			table := ""
			if truthy(e.Arg("table")) {
				table = " TABLE"
			}
			return "SUMMARIZE" + table + " " + g.sqlKey(e, "this")
		},
		expressions.KindShow: func(g *Generator, e expressions.Expression) string {
			from := g.sqlKey(e, "from_")
			if from != "" {
				from = " FROM " + from
			}
			return "SHOW " + g.sqlKey(e, "this") + from
		},
		expressions.KindWithinGroup: duckDBWithinGroupSQL,
		expressions.KindLength: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("LENGTH", []any{e.This()}, "(", ")", true)
		},
		expressions.KindSHA2:             duckDBShA2SQL,
		expressions.KindSplit:            clickhouseRename("STR_SPLIT"),
		expressions.KindRegexpExtract:    duckDBRegexpExtractSQL,
		expressions.KindRegexpExtractAll: duckDBRegexpExtractSQL,
		expressions.KindStar:             duckDBStarSQL,
		expressions.KindRegexpFullMatch: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("REGEXP_FULL_MATCH", []any{e.This(), e.Arg("expression")}, "(", ")", true)
		},
		expressions.KindStartsWith: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("STARTS_WITH", []any{e.This(), e.Arg("expression")}, "(", ")", true)
		},
		expressions.KindPow: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("POWER", []any{e.This(), e.Arg("expression")}, "(", ")", true)
		},
		expressions.KindBitwiseXor: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("XOR", []any{e.This(), e.Arg("expression")}, "(", ")", true)
		},
	}
	registerDialectDispatch("duckdb", handlers)
}

// duckDBAnyValueSQL ports _anyvalue_sql (generators/duckdb.py:1123-1130).
func duckDBAnyValueSQL(g *Generator, e expressions.Expression) string {
	if having := asExpression(e.This()); having != nil && having.Kind() == expressions.KindHavingMax {
		name := "ARG_MIN_NULL"
		if truthy(having.Arg("max")) {
			name = "ARG_MAX_NULL"
		}
		return g.funcCall(name, []any{having.This(), having.Arg("expression")}, "(", ")", true)
	}
	return g.functionFallbackSQL(e)
}

// duckDBItoa renders a non-negative int.
func duckDBItoa(i int) string {
	if i == 0 {
		return "0"
	}
	digits := ""
	for i > 0 {
		digits = string(rune('0'+i%10)) + digits
		i /= 10
	}
	return digits
}

// duckDBArraySQL ports the Array transform (generators/duckdb.py:1598) with
// inline_array_unless_query.
func duckDBArraySQL(g *Generator, e expressions.Expression) string {
	exprs := e.Expressions()
	if len(exprs) > 0 && exprs[0].Find(expressions.TraitQuery) != nil {
		return g.funcCall("ARRAY", []any{exprs[0]}, "(", ")", true)
	}
	return g.inlineArraySQL(e)
}

// duckDBStructSQL ports _struct_sql (generators/duckdb.py:607-651).
func duckDBStructSQL(g *Generator, e expressions.Expression) string {
	if len(e.Expressions()) == 0 {
		return "{}"
	}
	args := make([]string, 0, len(e.Expressions()))
	for i, expr := range e.Expressions() {
		isPropertyEq := expr.Kind() == expressions.KindPropertyEQ
		this := asExpression(expr.This())
		value := expr
		if isPropertyEq {
			value = asExpression(expr.Arg("expression"))
		}
		_ = value
		var key string
		switch {
		case this != nil && this.Kind() == expressions.KindIdentifier:
			key = g.gen(expressions.LiteralString(expr.Name()))
		case isPropertyEq:
			key = g.gen(this)
		default:
			key = g.gen(expressions.LiteralString("_" + duckDBItoa(i)))
		}
		args = append(args, key+": "+g.gen(value))
	}
	return "{" + strings.Join(args, ", ") + "}"
}

// duckDBArrowJSONExtract ports _arrow_json_extract_sql (generators/duckdb.py:881-886).
func duckDBArrowJSONExtract(g *Generator, e expressions.Expression) string {
	op := "->>"
	if e.Kind() == expressions.KindJSONExtract {
		op = "->"
	}
	out := g.arrowJSONExtractBinary(e, op)
	// generators/duckdb.py:878-886: wrap the arrow form under a binary, bracket, IN or NOT.
	if parent := e.Parent(); parent != nil {
		switch {
		case parent.Is(expressions.TraitBinary), parent.Kind() == expressions.KindBracket,
			parent.Kind() == expressions.KindIn, parent.Kind() == expressions.KindNot:
			out = "(" + out + ")"
		}
	}
	return out
}

// arrowJSONExtractBinary renders `this -> path`, wrapping the path when needed.
func (g *Generator) arrowJSONExtractBinary(e expressions.Expression, op string) string {
	return g.sqlKey(e, "this") + " " + op + " " + g.sqlKey(e, "expression")
}

// duckDBArrowJSONExtractArray ports _json_extract_value_array_sql
// (generators/duckdb.py:1003-1008): the JSON extraction is cast to ARRAY<STRING> for a
// JSON_VALUE_ARRAY and ARRAY<JSON> for a JSON_EXTRACT_ARRAY.
func duckDBArrowJSONExtractArray(g *Generator, e expressions.Expression) string {
	extract := expressions.JSONExtract(expressions.Args{"this": e.This(), "expression": e.Arg("expression")})
	typeName := "ARRAY<JSON>"
	if e.Kind() == expressions.KindJSONValueArray {
		typeName = "ARRAY<STRING>"
	}
	to, err := expressions.DataTypeBuild(typeName, "duckdb", false, false, nil)
	if err != nil {
		return g.gen(extract)
	}
	return g.gen(expressions.Cast(expressions.Args{"this": extract, "to": to}))
}

// duckDBArrayAppendSQL ports array_append_sql (dialects/dialect.py array_append_sql).
func duckDBArrayAppendSQL(name string, swap bool) func(*Generator, expressions.Expression) string {
	return func(g *Generator, e expressions.Expression) string {
		this, expr := e.This(), asExpression(e.Arg("expression"))
		if swap {
			this, expr = expr, this
		}
		return g.funcCall(name, []any{this, expr}, "(", ")", true)
	}
}

// duckDBArrayConcatSQL ports array_concat_sql("LIST_CONCAT").
func duckDBArrayConcatSQL(g *Generator, e expressions.Expression) string {
	args := []any{e.This()}
	for _, expr := range e.Expressions() {
		args = append(args, expr)
	}
	return g.funcCall("LIST_CONCAT", args, "(", ")", true)
}

// duckDBArrayContainsSQL ports _array_contains_sql (generators/duckdb.py:549-562) for the
// non-null-check form.
func duckDBArrayContainsSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("ARRAY_CONTAINS", []any{e.This(), e.Arg("expression")}, "(", ")", true)
}

// duckDBArrayOverlapsSQL ports _array_overlaps_sql (generators/duckdb.py:565-606). The
// null-safe form also treats a shared NULL as an overlap.
func duckDBArrayOverlapsSQL(g *Generator, e expressions.Expression) string {
	if !truthy(e.Arg("null_safe")) {
		return g.binary(e, "&&")
	}
	arr1 := g.sqlKey(e, "this")
	arr2 := g.sqlKey(e, "expression")
	return "(" + arr1 + " && " + arr2 + ") OR (ARRAY_LENGTH(" + arr1 + ") <> LIST_COUNT(" + arr1 +
		") AND ARRAY_LENGTH(" + arr2 + ") <> LIST_COUNT(" + arr2 + "))"
}

// duckDBMakeIntervalSQL ports no_make_interval_sql (dialects/dialect.py:2523-2531) with
// sep=" ": each value is paired with its unit inside an INTERVAL literal.
func duckDBMakeIntervalSQL(g *Generator, e expressions.Expression) string {
	args := make([]string, 0, 7)
	for _, key := range []string{"year", "month", "week", "day", "hour", "minute", "second"} {
		if value := asExpression(e.Arg(key)); value != nil {
			if value.Kind() == expressions.KindKwarg {
				value = asExpression(value.Arg("expression"))
			}
			args = append(args, g.gen(value)+" "+strings.ToUpper(key))
		}
	}
	return "INTERVAL '" + strings.Join(args, " ") + "'"
}

// duckDBGroupConcatSQL ports the GroupConcat lambda (generators/duckdb.py:1657) with
// within_group=False and the default separator.
func duckDBGroupConcatSQL(g *Generator, e expressions.Expression) string {
	this := asExpression(e.This())
	separator := asExpression(e.Arg("separator"))
	if separator == nil {
		separator = expressions.LiteralString(",")
	}
	var limit, order expressions.Expression
	if this != nil && this.Kind() == expressions.KindLimit && this.This() != nil {
		limit = this
		this = this.This().Pop()
	}
	if this != nil {
		if found := this.Find(expressions.KindOrder); found != nil && found.This() != nil {
			order = found
			this = found.This().Pop()
		}
	}
	argsSQL := ""
	if this != nil {
		argsSQL = g.gen(this)
	}
	if sep := g.gen(separator); sep != "" {
		if argsSQL != "" {
			argsSQL += ", "
		}
		argsSQL += sep
	}
	modifiers := ""
	if limit != nil {
		modifiers = g.gen(limit)
	}
	if order != nil {
		modifiers = g.gen(order) + modifiers
	}
	return "LISTAGG(" + argsSQL + modifiers + ")"
}

// duckDBGenerateSeriesSQL ports generate_series_sql("GENERATE_SERIES", "RANGE")
// (dialects/dialect.py:1420-1436).
func duckDBGenerateSeriesSQL(g *Generator, e expressions.Expression) string {
	name := "GENERATE_SERIES"
	if truthy(e.Arg("is_end_exclusive")) {
		name = "RANGE"
	}
	start := e.Arg("start")
	end := e.Arg("end")
	step := e.Arg("step")
	// dialects/dialect.py:1420-1436: an exclusive series with a dedicated function name
	// (DuckDB's RANGE) keeps its end verbatim.
	return g.funcCall(name, []any{start, end, step}, "(", ")", true)
}

// duckDBLastDaySQL ports _last_day_sql (generators/duckdb.py:158-213) for the native
// LAST_DAY forms.
func duckDBLastDaySQL(g *Generator, e expressions.Expression) string {
	unit := strings.ToUpper(e.Text("unit"))
	if unit == "" || unit == "MONTH" {
		return g.funcCall("LAST_DAY", []any{e.This()}, "(", ")", true)
	}
	return g.functionFallbackSQL(e)
}

// duckDBTimeToStrSQL ports the TimeToStr lambda (generators/duckdb.py:1745-1746).
func duckDBTimeToStrSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("STRFTIME", []any{e.This(), g.duckDBFormatTime(e)}, "(", ")", true)
}

// duckDBStrToTimeSQL ports the StrToTime/StrToDate StrPTIME form.
func duckDBStrToTimeSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("STRPTIME", []any{e.This(), g.duckDBFormatTime(e)}, "(", ")", true)
}

// duckDBFormatTime ports Generator.format_time for DuckDB's INVERSE_TIME_MAPPING.
func (g *Generator) duckDBFormatTime(e expressions.Expression) string {
	format := asExpression(e.Arg("format"))
	if format == nil || !format.IsString() {
		return g.gen(format)
	}
	if converted, ok := dialects.FormatTime(format.Name(), g.dialect.InverseTimeMapping); ok {
		return g.gen(expressions.LiteralString(converted))
	}
	return g.gen(format)
}

// duckDBEncodeDecodeSQL ports encode_decode_sql with replace=False.
func duckDBEncodeDecodeSQL(name string) func(*Generator, expressions.Expression) string {
	return func(g *Generator, e expressions.Expression) string {
		return g.funcCall(name, []any{e.This()}, "(", ")", true)
	}
}

// duckDBUnixToTimeSQL ports _unix_to_time_sql (generators/duckdb.py:848-879) for the common
// epoch forms.
func duckDBUnixToTimeSQL(g *Generator, e expressions.Expression) string {
	scale := asExpression(e.Arg("scale"))
	if scale == nil || (scale.IsInt() && scale.Name() == "0") {
		return g.funcCall("TO_TIMESTAMP", []any{e.This()}, "(", ")", true)
	}
	if scale.IsInt() {
		switch scale.Name() {
		case "3":
			return g.funcCall("EPOCH_MS", []any{e.This()}, "(", ")", true)
		case "6":
			return g.funcCall("MAKE_TIMESTAMP", []any{e.This()}, "(", ")", true)
		}
	}
	return g.functionFallbackSQL(e)
}

// duckDBUnixSecondsSQL ports the UnixSeconds lambda (generators/duckdb.py:1768).
func duckDBUnixSecondsSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("EPOCH", []any{e.This()}, "(", ")", true)
}

// duckDBDateDiffSQL ports _date_diff_sql (generators/duckdb.py:953-980) for the non-boundary
// form.
func duckDBDateDiffSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("DATE_DIFF", []any{
		duckDBDiffUnit(asExpression(e.Arg("unit"))),
		e.Arg("expression"),
		e.This(),
	}, "(", ")", true)
}

// duckDBTsOrDsDiffSQL ports the TsOrDsDiff lambda (generators/duckdb.py:1748-1752).
func duckDBTsOrDsDiffSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("DATE_DIFF", []any{
		duckDBDiffUnit(asExpression(e.Arg("unit"))),
		e.Arg("expression"),
		e.This(),
	}, "(", ")", true)
}

// duckDBTimestampTruncSQL ports timestamptrunc_sql (generators/duckdb.py:4462-...): DuckDB
// renders DATE_TRUNC(<unit string>, <this>).
func duckDBTimestampTruncSQL(g *Generator, e expressions.Expression) string {
	unit := duckDBDiffUnit(asExpression(e.Arg("unit")))
	return g.funcCall("DATE_TRUNC", []any{unit, e.This()}, "(", ")", true)
}

// duckDBSortArraySQL ports _array_sort_sql (generators/duckdb.py:545-548): a descending sort
// is ARRAY_REVERSE_SORT, everything else LIST_SORT.
func duckDBSortArraySQL(g *Generator, e expressions.Expression) string {
	arr := e.This()
	ascExpr := asExpression(e.Arg("asc"))
	nullsFirst := asExpression(e.Arg("nulls_first"))

	isBool := func(node expressions.Expression) bool {
		return node != nil && node.Kind() == expressions.KindBoolean
	}
	isTrue := func(node expressions.Expression) bool { return isBool(node) && truthy(node.Arg("this")) }

	if !isBool(ascExpr) && !isBool(nullsFirst) {
		return g.funcCall("LIST_SORT", []any{arr, ascExpr, nullsFirst}, "(", ")", true)
	}

	nullsAreFirst := isTrue(nullsFirst)
	var nullsFirstSQL expressions.Expression
	if nullsAreFirst {
		nullsFirstSQL = expressions.LiteralString("NULLS FIRST")
	}

	if !isBool(ascExpr) {
		return g.funcCall("LIST_SORT", []any{arr, ascExpr, nullsFirstSQL}, "(", ")", true)
	}

	descending := ascExpr != nil && !truthy(ascExpr.Arg("this"))
	if !descending && !nullsAreFirst {
		return g.funcCall("LIST_SORT", []any{arr}, "(", ")", true)
	}
	if !nullsAreFirst {
		return g.funcCall("ARRAY_REVERSE_SORT", []any{arr}, "(", ")", true)
	}
	direction := "ASC"
	if descending {
		direction = "DESC"
	}
	return g.funcCall("LIST_SORT", []any{
		arr,
		expressions.LiteralString(direction),
		expressions.LiteralString("NULLS FIRST"),
	}, "(", ")", true)
}

// duckDBTimestampFromPartsSQL renders TimestampFromParts as MAKE_TIMESTAMP.
func duckDBTimestampFromPartsSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("MAKE_TIMESTAMP", g.fallbackArgs(e), "(", ")", true)
}

// duckDBDateFromPartsSQL ports _date_from_parts_sql (generators/duckdb.py:1428-1456) for the
// non-overflow form.
func duckDBDateFromPartsSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("MAKE_DATE", []any{e.Arg("year"), e.Arg("month"), e.Arg("day")}, "(", ")", true)
}

// duckDBArrayCompactSQL ports array_compact_sql (dialects/dialect.py): LIST_FILTER over a
// non-NULL predicate.
func duckDBArrayCompactSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("LIST_FILTER", []any{
		e.This(),
		expressions.Lambda(expressions.Args{
			"this": expressions.Not(expressions.Args{"this": expressions.Is(expressions.Args{
				"this":       expressions.Column(expressions.Args{"this": expressions.ToIdentifier("_u")}),
				"expression": expressions.Null(),
			})}),
			"expressions": []expressions.Expression{expressions.ToIdentifier("_u")},
		}),
	}, "(", ")", true)
}

// duckDBArrayConstructCompactSQL ports the ArrayConstructCompact lambda: the constructor
// arguments become an inline array, filtered like ARRAY_COMPACT.
func duckDBArrayConstructCompactSQL(g *Generator, e expressions.Expression) string {
	array := expressions.Array(expressions.Args{"expressions": e.Expressions()})
	return g.funcCall("LIST_FILTER", []any{
		array,
		expressions.Lambda(expressions.Args{
			"this": expressions.Not(expressions.Args{"this": expressions.Is(expressions.Args{
				"this":       expressions.Column(expressions.Args{"this": expressions.ToIdentifier("_u")}),
				"expression": expressions.Null(),
			})}),
			"expressions": []expressions.Expression{expressions.ToIdentifier("_u")},
		}),
	}, "(", ")", true)
}

// duckDBAttachSQL ports attach_sql (generator.py:5901-5907).
func duckDBAttachSQL(g *Generator, e expressions.Expression) string {
	exists := ""
	if truthy(e.Arg("exists")) {
		exists = " IF NOT EXISTS"
	}
	options := g.expressions(exprsOptions{expression: e, flat: true})
	if options != "" {
		options = " (" + options + ")"
	}
	return "ATTACH" + exists + " " + g.sqlKey(e, "this") + options
}

// duckDBDetachSQL ports detach_sql (generator.py:5909-5925).
func duckDBDetachSQL(g *Generator, e expressions.Expression) string {
	kind := g.sqlKey(e, "kind")
	if kind != "" {
		kind = " " + kind
	}
	exists := ""
	if truthy(e.Arg("exists")) {
		exists = " IF EXISTS"
		if kind == "" {
			kind = " DATABASE"
		}
	}
	this := g.sqlKey(e, "this")
	if this != "" {
		this = " " + this
	}
	return "DETACH" + kind + exists + this
}

// duckDBAttachOptionSQL ports attachoption_sql (generator.py:5927-5931).
func duckDBAttachOptionSQL(g *Generator, e expressions.Expression) string {
	value := g.sqlKey(e, "expression")
	if value != "" {
		value = " " + value
	}
	return g.sqlKey(e, "this") + value
}

// duckDBInstallSQL ports install_sql (generators/duckdb.py:2686-2691).
func duckDBInstallSQL(g *Generator, e expressions.Expression) string {
	force := ""
	if truthy(e.Arg("force")) {
		force = "FORCE "
	}
	from := g.sqlKey(e, "from_")
	if from != "" {
		from = " FROM " + from
	}
	return force + "INSTALL " + g.sqlKey(e, "this") + from
}

// duckDBDataTypeSQL ports _datatype_sql (generators/duckdb.py:654-663) with the DECIMAL
// default and the TEXT precision strip from TYPE_CONVERTERS.
func duckDBDataTypeSQL(g *Generator, e expressions.Expression) string {
	dtype, _ := e.Arg("this").(expressions.DType)
	if dtype == expressions.DTypeArray {
		inner := ""
		if nested := e.Expressions(); len(nested) > 0 {
			inner = g.gen(nested[0])
		}
		values := g.expressions(exprsOptions{expression: e, key: "values", flat: true})
		return inner + "[" + values + "]"
	}
	params := e.Expressions()
	if len(params) == 0 {
		switch dtype {
		case expressions.DTypeDecimal:
			return "DECIMAL(18, 3)"
		case expressions.DTypeText:
			return "TEXT"
		}
	}
	if isTextDataType(dtype) {
		copy := e.Copy()
		copy.Set("expressions", nil)
		return g.dataTypeSQL(copy)
	}
	return g.dataTypeSQL(e)
}

// isTextDataType reports whether dtype is one of the text-family types.
func isTextDataType(dtype expressions.DType) bool {
	switch dtype {
	case expressions.DTypeText, expressions.DTypeVarchar, expressions.DTypeChar,
		expressions.DTypeNChar, expressions.DTypeNVarchar, expressions.DTypeBpchar:
		return true
	}
	return false
}

// duckDBBitwiseAggSQL ports _bitwise_agg_sql (generators/duckdb.py:1132-1161).
func duckDBBitwiseAggSQL(name string) func(*Generator, expressions.Expression) string {
	return func(g *Generator, e expressions.Expression) string {
		arg := asExpression(e.This())
		if arg != nil {
			dtype, ok := castResultDType(arg)
			if ok {
				if isFloatDataType(dtype) {
					arg = expressions.Round(expressions.Args{"this": arg})
					arg = expressions.Cast(expressions.Args{"this": arg, "to": expressions.DTypeInt.IntoExpr(nil)})
				} else if isRealOrTextDataType(dtype) {
					arg = expressions.Cast(expressions.Args{"this": arg, "to": expressions.DTypeInt.IntoExpr(nil)})
				}
			}
		}
		return g.funcCall(name, []any{arg}, "(", ")", true)
	}
}

// castResultDType returns the type of a CAST argument.
func castResultDType(arg expressions.Expression) (expressions.DType, bool) {
	if arg == nil || arg.Kind() != expressions.KindCast {
		return expressions.DTypeUnknown, false
	}
	to := asExpression(arg.Arg("to"))
	if to == nil {
		return expressions.DTypeUnknown, false
	}
	dtype, ok := to.Arg("this").(expressions.DType)
	return dtype, ok
}

// isFloatDataType reports whether dtype is a floating-point type.
func isFloatDataType(dtype expressions.DType) bool {
	switch dtype {
	case expressions.DTypeFloat, expressions.DTypeDouble, expressions.DTypeDecFloat:
		return true
	}
	return false
}

// isRealOrTextDataType reports whether dtype needs a direct INT cast for bit aggregates.
func isRealOrTextDataType(dtype expressions.DType) bool {
	if isFloatDataType(dtype) {
		return true
	}
	switch dtype {
	case expressions.DTypeDecimal, expressions.DTypeBigDecimal:
		return true
	}
	return isTextDataType(dtype)
}

// duckDBParseJSONSQL ports parsejson_sql (generators/duckdb.py:2840-2847) for the plain form.
func duckDBParseJSONSQL(g *Generator, e expressions.Expression) string {
	if truthy(e.Arg("safe")) {
		return g.functionFallbackSQL(e)
	}
	return g.funcCall("JSON", []any{e.This()}, "(", ")", true)
}

// duckDBIgnoreNullsSQL ports ignorenulls_sql (generators/duckdb.py:3987-4010): IGNORE NULLS
// on a generic aggregate is dropped, and on ARRAY_AGG becomes a filter over the non-NULLs.
func duckDBIgnoreNullsSQL(g *Generator, e expressions.Expression) string {
	this := asExpression(e.This())
	if this == nil {
		return ""
	}
	switch this.Kind() {
	case expressions.KindArrayAgg:
		arg := this.This()
		return g.funcCall("LIST", []any{arg}, "(", ")", true) + " FILTER(WHERE " + g.gen(arg) + " IS NOT NULL)"
	case expressions.KindFirst, expressions.KindAnyValue:
		return g.gen(this)
	}
	return g.gen(this) + " IGNORE NULLS"
}

// duckDBTableSampleSQL ports tablesample_sql (generators/duckdb.py:3118-3133).
func duckDBTableSampleSQL(g *Generator, e expressions.Expression) string {
	keyword := g.dialect.TablesampleKeywords
	if parent := e.Parent(); parent == nil || parent.Kind() != expressions.KindSelect {
		keyword = "TABLESAMPLE"
	}
	if e.Arg("size") != nil {
		method := asExpression(e.Arg("method"))
		if method != nil && !strings.EqualFold(method.Name(), "RESERVOIR") {
			e.Set("method", expressions.Var(expressions.Args{"this": "RESERVOIR"}))
		}
	}
	return g.tableSampleSQLWithKeyword(e, keyword)
}

// duckDBLambdaSQL renders DuckDB's `LAMBDA x, y : body` form when the parser saw the keyword.
func duckDBLambdaSQL(g *Generator, e expressions.Expression) string {
	if truthy(e.Arg("colon")) {
		params := g.expressions(exprsOptions{expression: e, key: "expressions", flat: true})
		return "LAMBDA " + params + " : " + g.sqlKey(e, "this")
	}
	return g.lambdaSQL(e)
}

// duckDBToVariantSQL ports the ToVariant lambda (generators/duckdb.py:1737).
func duckDBToVariantSQL(g *Generator, e expressions.Expression) string {
	return g.gen(expressions.Cast(expressions.Args{"this": e.This(), "to": expressions.DTypeVariant.IntoExpr(nil)}))
}

// duckDBLogicalSQL ports the LogicalOr/LogicalAnd lambdas (generators/duckdb.py:1727-1732).
func duckDBLogicalSQL(name string) func(*Generator, expressions.Expression) string {
	return func(g *Generator, e expressions.Expression) string {
		arg := e.This()
		if arg != nil {
			arg = expressions.Cast(expressions.Args{"this": arg, "to": expressions.DTypeBoolean.IntoExpr(nil)})
		}
		return g.funcCall(name, []any{arg}, "(", ")", true)
	}
}

// duckDBDateDeltaSQL ports _date_delta_to_binary_interval_op
// (generators/duckdb.py:320-358) for the plain interval forms.
func duckDBDateDeltaSQL(op string) func(*Generator, expressions.Expression) string {
	return func(g *Generator, e expressions.Expression) string {
		interval := asExpression(e.Arg("expression"))
		if interval == nil || interval.Kind() != expressions.KindInterval {
			interval = expressions.Interval(expressions.Args{
				"this": interval,
				"unit": unitToVarValue(asExpression(e.Arg("unit"))),
			})
		}
		return g.sqlKey(e, "this") + " " + op + " " + g.gen(interval)
	}
}

// duckDBWithinGroupSQL ports withingroup_sql (generators/duckdb.py:3196-3236) for the
// PERCENTILES form.
func duckDBWithinGroupSQL(g *Generator, e expressions.Expression) string {
	funcExpr := asExpression(e.This())
	if funcExpr != nil && funcExpr.Kind() == expressions.KindArrayAgg {
		// generators/duckdb.py:3200-3226: DuckDB requires ORDER BY inside the aggregate, so
		// move the WITHIN GROUP order into the ArrayAgg argument.
		order := asExpression(e.Arg("expression"))
		if order == nil || order.Kind() != expressions.KindOrder || funcExpr.This() == nil {
			return g.gen(funcExpr)
		}
		funcExpr.Set("this", expressions.Order(expressions.Args{
			"this":        funcExpr.This().Copy(),
			"expressions": order.Expressions(),
		}))
		return g.functionFallbackSQL(funcExpr)
	}
	order := asExpression(e.Arg("expression"))
	if funcExpr != nil && (funcExpr.Kind() == expressions.KindPercentileCont || funcExpr.Kind() == expressions.KindPercentileDisc) {
		if ordered := order.Find(expressions.KindOrdered); ordered != nil {
			funcExpr.Set("expression", funcExpr.This())
			funcExpr.Set("this", ordered.This())
		}
	}
	this := strings.TrimSuffix(g.gen(funcExpr), ")")
	return this + g.gen(order) + ")"
}

// duckDBShA2SQL ports the SHA2 lambda (generators/duckdb.py:1734-1736): 256-bit digests render
// as the plain SHA256 function.
func duckDBShA2SQL(g *Generator, e expressions.Expression) string {
	if e.Text("length") == "256" || e.Arg("length") == nil {
		return g.funcCall("SHA256", []any{e.This()}, "(", ")", true)
	}
	return g.functionFallbackSQL(e)
}

// duckDBRegexpExtractSQL renders REGEXP_EXTRACT/REGEXP_EXTRACT_ALL in DuckDB's argument order.
func duckDBRegexpExtractSQL(g *Generator, e expressions.Expression) string {
	name := "REGEXP_EXTRACT"
	if e.Kind() == expressions.KindRegexpExtractAll {
		name = "REGEXP_EXTRACT_ALL"
	}
	group := asExpression(e.Arg("group"))
	parameters := asExpression(e.Arg("parameters"))
	args := []any{e.This(), e.Arg("expression")}
	isZeroGroup := group != nil && group.IsInt() && group.Name() == "0"
	if parameters != nil {
		if group != nil {
			args = append(args, group)
		}
		args = append(args, parameters)
	} else if group != nil && !isZeroGroup {
		args = append(args, group)
	}
	return g.funcCall(name, args, "(", ")", true)
}

// duckDBStarSQL ports the Star rendering with DuckDB's EXCLUDE keyword.
func duckDBStarSQL(g *Generator, e expressions.Expression) string {
	except := g.expressions(exprsOptions{expression: e, key: "except_", flat: true})
	if except != "" {
		except = g.seg("EXCLUDE") + " (" + except + ")"
	}
	replace := g.expressions(exprsOptions{expression: e, key: "replace", flat: true})
	if replace != "" {
		replace = g.seg("REPLACE") + " (" + replace + ")"
	}
	return "*" + except + replace
}

// duckDBDiffUnit renders a DATE_DIFF unit: a Var/Literal unit is uppercased and quoted, any
// other expression is kept, and an absent unit defaults to DAY.
func duckDBDiffUnit(unit expressions.Expression) any {
	if unit == nil {
		return expressions.LiteralString("DAY")
	}
	switch unit.Kind() {
	case expressions.KindVar, expressions.KindLiteral:
		return expressions.LiteralString(strings.ToUpper(unit.Name()))
	}
	return unit
}
