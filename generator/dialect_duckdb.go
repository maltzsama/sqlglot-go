package generator

import (
	"math"
	"regexp"
	"slices"
	"strconv"
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
	// generators/duckdb.py:1939-1949: DuckDB's PROPERTIES_LOCATION blanket marks every
	// base-registered property UNSUPPORTED (Like/Temporary/Returns/Sequence/Iceberg are
	// the exceptions), so the Teradata CREATE TABLE property set this port added to the
	// base table is explicitly unsupported here too.
	registerDialectPropertyLocations("duckdb", map[expressions.Kind]propertyLocation{
		expressions.KindBlockCompressionProperty: propertyLocationUnsupported,
		expressions.KindChecksumProperty:         propertyLocationUnsupported,
		expressions.KindDataBlocksizeProperty:    propertyLocationUnsupported,
		expressions.KindFileFormatProperty:       propertyLocationUnsupported,
		expressions.KindFallbackProperty:         propertyLocationUnsupported,
		expressions.KindFreespaceProperty:        propertyLocationUnsupported,
		expressions.KindGlobalProperty:           propertyLocationUnsupported,
		expressions.KindIsolatedLoadingProperty:  propertyLocationUnsupported,
		expressions.KindJournalProperty:          propertyLocationUnsupported,
		expressions.KindLogProperty:              propertyLocationUnsupported,
		expressions.KindMergeBlockRatioProperty:  propertyLocationUnsupported,
		expressions.KindPartitionedByProperty:    propertyLocationUnsupported,
		expressions.KindSetProperty:              propertyLocationUnsupported,
		expressions.KindVolatileProperty:         propertyLocationUnsupported,
		expressions.KindWithJournalTableProperty: propertyLocationUnsupported,
		// generators/duckdb.py PROPERTIES_LOCATION blanket-drops every other property.
		expressions.KindLocationProperty:      propertyLocationUnsupported,
		expressions.KindClusteredByProperty:   propertyLocationUnsupported,
		expressions.KindEngineProperty:        propertyLocationUnsupported,
		expressions.KindAutoIncrementProperty: propertyLocationUnsupported,
		expressions.KindCharacterSetProperty:  propertyLocationUnsupported,
		expressions.KindCollateProperty:       propertyLocationUnsupported,
		expressions.KindRowFormatProperty:     propertyLocationUnsupported,
		expressions.KindExternalProperty:      propertyLocationUnsupported,
		expressions.KindProperty:              propertyLocationUnsupported,
		expressions.KindSchemaCommentProperty: propertyLocationUnsupported,
	})

	handlers := map[expressions.Kind]func(*Generator, expressions.Expression) string{
		expressions.KindAnyValue: duckDBAnyValueSQL,
		// A TIMESERIES marker is metadata Databricks attaches to a primary-key column; DuckDB
		// keeps only the column reference.
		expressions.KindTimeseriesKey: func(g *Generator, e expressions.Expression) string {
			return g.sqlKey(e, "this")
		},
		// generators/duckdb.py:1748-1749 and 2020: CREATE FUNCTION drops a scalar RETURNS and
		// keeps only TABLE, and a RETURN body renders as its expression.
		expressions.KindReturnsProperty: func(g *Generator, e expressions.Expression) string {
			if this := asExpression(e.This()); this != nil && this.Kind() == expressions.KindSchema {
				return "TABLE"
			}
			return ""
		},
		expressions.KindReturn: func(g *Generator, e expressions.Expression) string {
			return g.sqlKey(e, "this")
		},
		expressions.KindAlter: func(g *Generator, e expressions.Expression) string {
			e.Set("iceberg", false)
			return g.alterSQL(e)
		},
		expressions.KindCommentColumnConstraint: func(g *Generator, _ expressions.Expression) string {
			return ""
		},
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
		expressions.KindWeekOfYear:        clickhouseRename("WEEKOFYEAR"),
		expressions.KindVariancePop:       clickhouseRename("VAR_POP"),
		expressions.KindJSONObjectAgg:     clickhouseRename("JSON_GROUP_OBJECT"),
		expressions.KindDateBin:           clickhouseRename("TIME_BUCKET"),
		expressions.KindArray:             duckDBArraySQL,
		expressions.KindStruct:            duckDBStructSQL,
		expressions.KindToMap:             duckDBToMapSQL,
		// generators/duckdb.py:3145-3160: DuckDB rejects `JOIN UNNEST(...)` without an ON
		// clause, so a synthetic UNNEST join gets a dummy ON TRUE and a plain comma join
		// loses its synthetic side/kind.
		expressions.KindJoin:             duckDBJoinSQL,
		expressions.KindRand:             duckDBRandSQL,
		expressions.KindJSONExtract:      duckDBArrowJSONExtract,
		expressions.KindJSONExtractArray: duckDBArrowJSONExtractArray,
		expressions.KindJSONValueArray:   duckDBArrowJSONExtractArray,
		expressions.KindArrayAppend:      duckDBArrayAppendSQL("LIST_APPEND", false),
		expressions.KindArrayPrepend:     duckDBArrayAppendSQL("LIST_PREPEND", true),
		expressions.KindArrayConcat:      duckDBArrayConcatSQL,
		expressions.KindArrayContains:    duckDBArrayContainsSQL,
		expressions.KindArrayOverlaps:    duckDBArrayOverlapsSQL,
		expressions.KindBitwiseAnd:       func(g *Generator, e expressions.Expression) string { return g.binary(e, "&") },
		expressions.KindBitwiseOr:        func(g *Generator, e expressions.Expression) string { return g.binary(e, "|") },
		expressions.KindIntDiv:           func(g *Generator, e expressions.Expression) string { return g.binary(e, "//") },
		expressions.KindCurrentTime:      func(*Generator, expressions.Expression) string { return "CURRENT_TIME" },
		expressions.KindCurrentTimestamp: func(g *Generator, e expressions.Expression) string {
			if truthy(e.Arg("sysdate")) {
				return "CURRENT_TIMESTAMP AT TIME ZONE 'UTC'"
			}
			return "CURRENT_TIMESTAMP"
		},
		expressions.KindLocaltime:             func(*Generator, expressions.Expression) string { return "LOCALTIME" },
		expressions.KindLocaltimestamp:        func(*Generator, expressions.Expression) string { return "LOCALTIMESTAMP" },
		expressions.KindMakeInterval:          duckDBMakeIntervalSQL,
		expressions.KindGroupConcat:           duckDBGroupConcatSQL,
		expressions.KindGenerateSeries:        duckDBGenerateSeriesSQL,
		expressions.KindLastDay:               duckDBLastDaySQL,
		expressions.KindGenerateDateArray:     duckDBGenerateDateArraySQL,
		expressions.KindArrayInsert:           duckDBArrayInsertSQL,
		expressions.KindTimeToStr:             duckDBTimeToStrSQL,
		expressions.KindStrToTime:             duckDBStrToTimeSQL,
		expressions.KindStrToDate:             duckDBStrToDateSQL,
		expressions.KindDecode:                duckDBEncodeDecodeSQL("DECODE"),
		expressions.KindEncode:                duckDBEncodeDecodeSQL("ENCODE"),
		expressions.KindUnixToTime:            duckDBUnixToTimeSQL,
		expressions.KindDateDiff:              duckDBDateDiffSQL,
		expressions.KindDatetimeDiff:          duckDBDateDiffSQL,
		expressions.KindTsOrDsDiff:            duckDBTsOrDsDiffSQL,
		expressions.KindSortArray:             duckDBSortArraySQL,
		expressions.KindTimestampFromParts:    duckDBTimestampFromPartsSQL,
		expressions.KindDateFromParts:         duckDBDateFromPartsSQL,
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
		expressions.KindIgnoreNulls:           duckDBIgnoreNullsSQL,
		expressions.KindTableSample:           duckDBTableSampleSQL,
		expressions.KindPivot:                 duckDBPivot,
		expressions.KindLambda:                duckDBLambdaSQL,
		expressions.KindGetbit:                clickhouseRename("GET_BIT"),
		expressions.KindToVariant:             duckDBToVariantSQL,
		expressions.KindIsInf:                 clickhouseRename("ISINF"),
		expressions.KindLogicalOr:             duckDBLogicalSQL("BOOL_OR"),
		expressions.KindLogicalAnd:            duckDBLogicalSQL("BOOL_AND"),
		expressions.KindDateAdd:               duckDBDateDeltaSQL("+"),
		expressions.KindInterval:              duckDBIntervalSQL,
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
		expressions.KindSHA2:        duckDBShA2SQL,
		expressions.KindStar:        duckDBStarSQL,
		expressions.KindRegexpFullMatch: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("REGEXP_FULL_MATCH", []any{e.This(), e.Arg("expression"), e.Arg("options")}, "(", ")", true)
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

// duckDBToMapSQL renders MAP literals from their key/value pairs directly.
// A ToMap wraps a Struct in the AST, but DuckDB map keys are expressions (and may
// be arrays); routing them through duckDBStructSQL incorrectly turns keys into
// struct field names.
func duckDBToMapSQL(g *Generator, e expressions.Expression) string {
	this := asExpression(e.Arg("this"))
	if this == nil {
		return "MAP {}"
	}
	if this.Kind() != expressions.KindStruct {
		return g.functionFallbackSQL(e)
	}
	pairs := this.Expressions()
	items := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		if pair.Kind() != expressions.KindPropertyEQ {
			items = append(items, g.gen(pair))
			continue
		}
		key := asExpression(pair.This())
		value := asExpression(pair.Arg("expression"))
		items = append(items, g.gen(key)+": "+g.gen(value))
	}
	return "MAP {" + strings.Join(items, ", ") + "}"
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
	if truthy(e.Arg("struct_name_inheritance")) {
		applyStructNameInheritance(exprs)
	}
	return inlineArrayUnlessQuery(g, e)
}

// inlineArrayUnlessQuery ports dialects/dialect.py:1513-1523: a single top-level query
// renders as ARRAY(query) unless value_constructor marks a bracketed scalar subquery.
func inlineArrayUnlessQuery(g *Generator, e expressions.Expression) string {
	exprs := e.Expressions()
	if len(exprs) == 1 && exprs[0] != nil && exprs[0].Is(expressions.TraitQuery) && !truthy(e.Arg("value_constructor")) {
		return g.funcCall("ARRAY", []any{exprs[0]}, "(", ")", true)
	}
	return g.inlineArraySQL(e)
}

// applyStructNameInheritance ports transforms.inherit_struct_field_names
// (transforms.py:1032-1090): later positional structs inherit the first struct's field names.
func applyStructNameInheritance(exprs []expressions.Expression) {
	if len(exprs) == 0 || exprs[0] == nil || exprs[0].Kind() != expressions.KindStruct {
		return
	}
	first := exprs[0].Expressions()
	names := make([]expressions.Expression, len(first))
	for i, field := range first {
		if field.Kind() != expressions.KindPropertyEQ {
			return
		}
		names[i] = asExpression(field.This())
	}
	for _, structExpr := range exprs[1:] {
		if structExpr == nil || structExpr.Kind() != expressions.KindStruct {
			continue
		}
		values := structExpr.Expressions()
		if len(values) != len(names) {
			continue
		}
		converted := make([]expressions.Expression, len(values))
		for i, value := range values {
			if value.Kind() == expressions.KindPropertyEQ {
				converted[i] = value
				continue
			}
			name := names[i]
			if name != nil && name.Kind() == expressions.KindIdentifier {
				converted[i] = expressions.PropertyEQ(expressions.Args{"this": name.Copy(), "expression": value})
			} else {
				converted[i] = value
			}
		}
		structExpr.Set("expressions", converted)
	}
}

// duckDBStructSQL ports _struct_sql (generators/duckdb.py:607-651).
func duckDBStructSQL(g *Generator, e expressions.Expression) string {
	ancestorCast := e.FindAncestor(expressions.KindCast, expressions.KindSelect)
	if ancestorCast != nil && ancestorCast.Kind() == expressions.KindSelect {
		ancestorCast = nil
	}

	// generators/duckdb.py:611-614: an empty struct cast works with MAP().
	if len(e.Expressions()) == 0 {
		if ancestorCast != nil {
			if to := asExpression(ancestorCast.Arg("to")); to != nil && expIsType(to, expressions.DTypeMap) {
				return "MAP()"
			}
		}
		return "{}"
	}

	// generators/duckdb.py:623-630: a positional struct under a STRUCT cast is rendered as
	// ROW(...), because DuckDB requires matching field names in a named-struct cast.
	isInlineStruct := e.Find(expressions.KindPropertyEQ) == nil && ancestorCast != nil
	if isInlineStruct {
		isInlineStruct = false
		for _, dataType := range ancestorCast.FindAll(expressions.KindDataType) {
			if expIsType(dataType, expressions.DTypeStruct) {
				isInlineStruct = true
				break
			}
		}
	}

	args := make([]string, 0, len(e.Expressions()))
	for i, expr := range e.Expressions() {
		isPropertyEq := expr.Kind() == expressions.KindPropertyEQ
		this := asExpression(expr.This())
		value := expr
		if isPropertyEq {
			value = asExpression(expr.Arg("expression"))
		}
		if isInlineStruct {
			args = append(args, g.gen(value))
			continue
		}
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
	if isInlineStruct {
		return "ROW(" + strings.Join(args, ", ") + ")"
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
	// generators/duckdb.py:878-886: wrap the arrow form under a binary, bracket, IN or NOT,
	// but only when the parent is a different class (Expression.same_parent, core.py:1178) -
	// a nested `a -> b -> c` keeps its left side unwrapped.
	if parent := e.Parent(); parent != nil && parent.Kind() != e.Kind() {
		switch {
		case parent.Is(expressions.TraitBinary), parent.Kind() == expressions.KindBracket,
			parent.Kind() == expressions.KindIn, parent.Kind() == expressions.KindNot:
			out = "(" + out + ")"
		}
	}
	return out
}

// duckDBRandSQL ports DuckDBGenerator.rand_sql (generators/duckdb.py:3731-3751).
func duckDBRandSQL(g *Generator, e expressions.Expression) string {
	if e.Arg("this") != nil {
		g.unsupported("RANDOM with seed is not supported in DuckDB")
	}
	lower := asExpression(e.Arg("lower"))
	upper := asExpression(e.Arg("upper"))
	if lower != nil && upper != nil {
		lowerOperand := lower
		if lower.Kind() == expressions.KindAdd || lower.Kind() == expressions.KindSub {
			lowerOperand = expressions.Paren(expressions.Args{"this": lower})
		}
		rangeSize := expressions.Paren(expressions.Args{
			"this": expressions.Sub(expressions.Args{"this": upper, "expression": lowerOperand}),
		})
		scaled := expressions.Add(expressions.Args{
			"this":       lower,
			"expression": expressions.Mul(expressions.Args{"this": expressions.Rand(expressions.Args{}), "expression": rangeSize}),
		})
		return g.gen(expressions.Cast(expressions.Args{
			"this": scaled,
			"to":   expressions.DataType(expressions.Args{"this": expressions.DTypeBigInt}),
		}))
	}
	return "RANDOM()"
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
	if e.Kind() == expressions.KindJSONValueArray {
		// BigQuery returns SQL NULL when any element is an object or array.
		extractSQL := g.gen(extract)
		jsonArray := "CAST(" + extractSQL + " AS JSON[])"
		nonScalar := "LIST_FILTER(" + jsonArray + ", LAMBDA x : JSON_TYPE(x) IN ('OBJECT', 'ARRAY'))"
		return "CASE WHEN " + extractSQL + " IS NULL THEN NULL WHEN LENGTH(" + nonScalar + ") = 0 THEN " +
			g.gen(expressions.Cast(expressions.Args{"this": extract, "to": to})) + " ELSE NULL END"
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

// duckDBArrayContainsSQL ports _array_contains_sql (generators/duckdb.py:549-562): with
// check_null, a NULL needle is handled by comparing the array's full and non-NULL counts.
func duckDBArrayContainsSQL(g *Generator, e expressions.Expression) string {
	this := asExpression(e.Arg("this"))
	expr := asExpression(e.Arg("expression"))
	funcSQL := "ARRAY_CONTAINS(" + g.gen(this) + ", " + g.gen(expr) + ")"
	if !truthy(e.Arg("check_null")) {
		return funcSQL
	}
	// generators/duckdb.py:547-558: exp.If renders through case_sql as CASE WHEN.
	checkNull := expressions.Nullif(expressions.Args{
		"this": expressions.NEQ(expressions.Args{
			"this":       expressions.ArraySize(expressions.Args{"this": this}),
			"expression": expressions.Anonymous(expressions.Args{"this": "LIST_COUNT", "expressions": []expressions.Expression{this}}),
		}),
		"expression": expressions.Boolean(expressions.Args{"this": false}),
	})
	return g.gen(expressions.If(expressions.Args{
		"this":  expressions.Is(expressions.Args{"this": expr, "expression": expressions.Null()}),
		"true":  checkNull,
		"false": expressions.Anonymous(expressions.Args{"this": "ARRAY_CONTAINS", "expressions": []expressions.Expression{this, expr}}),
	}))
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
	defaultOrder := []string{"year", "month", "week", "day", "hour", "minute", "second"}
	order := defaultOrder
	if raw, ok := e.MetaGet("make_interval_order").([]string); ok && len(raw) > 0 {
		order = append([]string{}, raw...)
		for _, unit := range defaultOrder {
			if !slices.Contains(order, unit) {
				order = append(order, unit)
			}
		}
	}
	args := make([]string, 0, len(order))
	for _, key := range order {
		if value := asExpression(e.Arg(key)); value != nil {
			if value.Kind() == expressions.KindKwarg {
				value = asExpression(value.Arg("expression"))
			}
			args = append(args, g.gen(value)+" "+key)
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
	series := duckDBGenerateSeriesBaseSQL(g, e)
	// generators/duckdb.py/generator.py:5585-5597: a Postgres set-returning generate_series in
	// a projection is UNNESTed; a table source keeps the bare call.
	if truthy(e.Arg("exploding")) {
		parent := e.Parent()
		if parent != nil && (parent.Kind() == expressions.KindAlias || parent.Kind() == expressions.KindTableAlias) {
			parent = parent.Parent()
		}
		if parent == nil || (parent.Kind() != expressions.KindTable && parent.Kind() != expressions.KindUnnest && parent.Kind() != expressions.KindLateral) {
			return "UNNEST(" + series + ")"
		}
	}
	return series
}

func duckDBGenerateSeriesBaseSQL(g *Generator, e expressions.Expression) string {
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

// duckDBLastDaySQL ports _last_day_sql (generators/duckdb.py:158-213): DuckDB's LAST_DAY only
// handles the month, so YEAR/QUARTER/WEEK are emulated.
func duckDBLastDaySQL(g *Generator, e expressions.Expression) string {
	date := e.This()
	unit := asExpression(e.Arg("unit"))
	name := ""
	if unit != nil {
		name = strings.ToUpper(unit.Name())
	}

	if weekStart := duckDBWeekStartDow(unit, name); weekStart > 0 {
		lastDow := weekStart - 1
		dow := expressions.Extract(expressions.Args{
			"this":       expressions.Var(expressions.Args{"this": "DAYOFWEEK"}),
			"expression": date,
		})
		daysToLast := expressions.Mod(expressions.Args{
			"this": expressions.Paren(expressions.Args{"this": expressions.Sub(expressions.Args{
				"this":       expressions.LiteralNumber(duckDBIntStr(lastDow + 7)),
				"expression": dow,
			})}),
			"expression": expressions.LiteralNumber("7"),
		})
		interval := expressions.Interval(expressions.Args{
			"this": daysToLast,
			"unit": expressions.Var(expressions.Args{"this": "DAY"}),
		})
		return g.gen(expressions.Cast(expressions.Args{
			"this": expressions.Add(expressions.Args{"this": date, "expression": interval}),
			"to":   expressions.DataType(expressions.Args{"this": expressions.DTypeDate}),
		}))
	}

	switch name {
	case "", "MONTH":
		return g.funcCall("LAST_DAY", []any{date}, "(", ")", true)
	case "YEAR":
		year := duckDBExtract("YEAR", date)
		return g.gen(expressions.Anonymous(expressions.Args{"this": "MAKE_DATE", "expressions": []expressions.Expression{
			year, expressions.LiteralNumber("12"), expressions.LiteralNumber("31"),
		}}))
	case "QUARTER":
		year := duckDBExtract("YEAR", date)
		quarter := duckDBExtract("QUARTER", date)
		lastMonth := expressions.Mul(expressions.Args{"this": quarter, "expression": expressions.LiteralNumber("3")})
		firstOfLast := expressions.Anonymous(expressions.Args{"this": "MAKE_DATE", "expressions": []expressions.Expression{
			year, lastMonth, expressions.LiteralNumber("1"),
		}})
		return g.gen(expressions.Anonymous(expressions.Args{"this": "LAST_DAY", "expressions": []expressions.Expression{firstOfLast}}))
	}
	g.unsupported("Unsupported date part '" + name + "' in LAST_DAY function")
	return g.functionFallbackSQL(e)
}

func duckDBExtract(part string, date expressions.Expression) expressions.Expression {
	return expressions.Extract(expressions.Args{
		"this":       expressions.Var(expressions.Args{"this": part}),
		"expression": date,
	})
}

// duckDBWeekStartDow maps a week-ish unit to its ISO 8601 day number (Monday=1..Sunday=7).
// BigQuery's bare WEEK defaults to Sunday; ISOWEEK and WEEK(MONDAY) are Monday.
func duckDBWeekStartDow(unit expressions.Expression, name string) int {
	if unit == nil {
		return 0
	}
	if unit.Kind() == expressions.KindWeekStart {
		switch strings.ToUpper(unit.Name()) {
		case "MONDAY":
			return 1
		case "TUESDAY":
			return 2
		case "WEDNESDAY":
			return 3
		case "THURSDAY":
			return 4
		case "FRIDAY":
			return 5
		case "SATURDAY":
			return 6
		case "SUNDAY":
			return 7
		}
		return 0
	}
	if unit.Kind() == expressions.KindColumn && name == "WEEK" {
		// BigQuery's bare WEEK starts on Sunday; the parser keeps it as a Column.
		return 7
	}
	switch name {
	case "WEEK", "ISOWEEK":
		return 1
	}
	return 0
}

func duckDBIntStr(i int) string {
	return strconv.Itoa(i)
}

// duckDBTimeToStrSQL ports the TimeToStr lambda (generators/duckdb.py:1745-1746).
func duckDBTimeToStrSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("STRFTIME", []any{e.This(), g.duckDBFormatTime(e)}, "(", ")", true)
}

// duckDBStrToTimeSQL ports the StrToTime/StrToDate StrPTIME form.
func duckDBStrToTimeSQL(g *Generator, e expressions.Expression) string {
	value, format := g.duckDBDefaultYear(e)
	if truthy(e.Arg("safe")) {
		castType := "TIMESTAMP"
		if target := asExpression(e.Arg("target_type")); target != nil {
			if dtype, ok := target.Arg("this").(expressions.DType); ok &&
				(dtype == expressions.DTypeTimestampLtz || dtype == expressions.DTypeTimestampTz) {
				castType = "TIMESTAMPTZ"
			}
		}
		return "CAST(" + g.funcCall("TRY_STRPTIME", []any{value, format}, "(", ")", true) + " AS " + castType + ")"
	}
	return g.funcCall("STRPTIME", []any{value, format}, "(", ")", true)
}

// duckDBStrToDateSQL ports strtodate_sql (generators/duckdb.py:2767-2776).
func duckDBStrToDateSQL(g *Generator, e expressions.Expression) string {
	value, format := g.duckDBDefaultYear(e)
	name := "STRPTIME"
	if truthy(e.Arg("safe")) {
		name = "TRY_STRPTIME"
	}
	return "CAST(" + g.funcCall(name, []any{value, format}, "(", ")", true) + " AS DATE)"
}

// duckDBParseDatetimeSQL ports parsedatetime_sql (generators/duckdb.py:2789-2791).
func duckDBParseDatetimeSQL(g *Generator, e expressions.Expression) string {
	value, format := g.duckDBDefaultYear(e)
	return g.funcCall("STRPTIME", []any{value, format}, "(", ")", true)
}

// duckDBDefaultYear ports _strptime_default_year (generators/duckdb.py:2777-2787).
func (g *Generator) duckDBDefaultYear(e expressions.Expression) (any, any) {
	value := any(e.This())
	var format any = g.duckDBFormatTimeExpr(e)
	if year := asExpression(e.Arg("default_year")); year != nil {
		value = expressions.DPipe(expressions.Args{
			"this": expressions.LiteralString(year.Name() + " "), "expression": e.This(),
		})
		if formatExpr := asExpression(format); formatExpr != nil {
			format = expressions.DPipe(expressions.Args{
				"this": expressions.LiteralString("%Y "), "expression": formatExpr,
			})
		}
	}
	return value, format
}

// duckDBFormatTimeExpr is duckDBFormatTime returning an expression node, so the default-year
// DPipe can wrap the converted format.
func (g *Generator) duckDBFormatTimeExpr(e expressions.Expression) expressions.Expression {
	format := asExpression(e.Arg("format"))
	if format == nil || !format.IsString() {
		return format
	}
	if converted, ok := dialects.FormatTime(format.Name(), g.dialect.InverseTimeMapping); ok {
		return expressions.LiteralString(stripStrictTimeFormat(converted))
	}
	return format
}

// duckDBFormatTime ports Generator.format_time for DuckDB's INVERSE_TIME_MAPPING.
func (g *Generator) duckDBFormatTime(e expressions.Expression) string {
	format := asExpression(e.Arg("format"))
	if format == nil || !format.IsString() {
		return g.gen(format)
	}
	if converted, ok := dialects.FormatTime(format.Name(), g.dialect.InverseTimeMapping); ok {
		return g.gen(expressions.LiteralString(stripStrictTimeFormat(converted)))
	}
	return g.gen(format)
}

// stripStrictTimeFormat removes the port's internal "strict" marker from a converted format
// (e.g. "%mstrict" -> "%m"), which only the Hive/Snowflake inverse maps understand.
func stripStrictTimeFormat(format string) string {
	for _, marker := range []string{"%mstrict", "%dstrict", "%Hstrict", "%Istrict", "%Mstrict", "%Sstrict"} {
		format = strings.ReplaceAll(format, marker, strings.TrimSuffix(marker, "strict"))
	}
	return format
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
	result := duckDBUnixToTimeBaseSQL(g, e)
	// generators/duckdb.py:848-877: only a naive timestamp target gets the UTC zone tag.
	if target := asExpression(e.Arg("target_type")); target != nil {
		if dtype, ok := target.Arg("this").(expressions.DType); ok && (dtype == expressions.DTypeTimestamp || dtype == expressions.DTypeTimestampNtz) {
			return result + " AT TIME ZONE 'UTC'"
		}
	}
	return result
}

func duckDBUnixToTimeBaseSQL(g *Generator, e expressions.Expression) string {
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
	// Other scales: divide by 10^scale and convert the seconds. A compound numerator must
	// stay grouped, otherwise only its last operand is divided.
	numerator := e.This()
	if numerator != nil && numerator.Is(expressions.TraitBinary) {
		numerator = expressions.Paren(expressions.Args{"this": numerator})
	}
	timestamp := expressions.Div(expressions.Args{
		"this":       numerator,
		"expression": expressions.Pow(expressions.Args{"this": expressions.LiteralNumber("10"), "expression": scale}),
	})
	return g.funcCall("TO_TIMESTAMP", []any{timestamp}, "(", ")", true)
}

// duckDBUnixSecondsSQL ports the UnixSeconds lambda (generators/duckdb.py:1768).
func duckDBUnixSecondsSQL(g *Generator, e expressions.Expression) string {
	// BigQuery truncates toward negative infinity; a plain CAST rounds half away from zero.
	return "CAST(FLOOR(" + g.funcCall("EPOCH", []any{implicitDatetimeCast(e.This())}, "(", ")", true) + ") AS BIGINT)"
}

// duckDBDateDiffSQL ports _date_diff_sql (generators/duckdb.py:953-980) for the non-boundary
// form.
func duckDBDateDiffSQL(g *Generator, e expressions.Expression) string {
	return duckDBDateDiffImpl(g, e)
}

// duckDBTsOrDsDiffSQL ports the TsOrDsDiff lambda (generators/duckdb.py:1779-1785): unlike
// DateDiff, it casts BOTH operands to TIMESTAMP and renders the raw unit (or DAY).
func duckDBTsOrDsDiffSQL(g *Generator, e expressions.Expression) string {
	unit := "DAY"
	if u := asExpression(e.Arg("unit")); u != nil && u.Name() != "" {
		unit = strings.ToUpper(u.Name())
	}
	ts := func(v expressions.Expression) expressions.Expression {
		return expressions.Cast(expressions.Args{
			"this": v,
			"to":   expressions.DataType(expressions.Args{"this": expressions.DTypeTimestamp}),
		})
	}
	return g.funcCall("DATE_DIFF", []any{
		expressions.LiteralString(unit),
		ts(asExpression(e.Arg("expression"))),
		ts(e.This()),
	}, "(", ")", true)
}

func duckDBDateDiffImpl(g *Generator, e expressions.Expression) string {
	unit := asExpression(e.Arg("unit"))

	// generators/duckdb.py:953-980: DATE_DIFF does not support NANOSECOND, so route it
	// through EPOCH_NS on both operands cast to TIMESTAMP_NS.
	if unit != nil && (unit.Kind() == expressions.KindVar || unit.Kind() == expressions.KindLiteral) &&
		strings.EqualFold(unit.Name(), "NANOSECOND") {
		toNS := func(v expressions.Expression) string {
			return g.gen(expressions.Cast(expressions.Args{"this": v, "to": expressions.DTypeTimestampNs.IntoExpr(nil)}))
		}
		return "EPOCH_NS(" + toNS(asExpression(e.This())) + ") - EPOCH_NS(" + toNS(asExpression(e.Arg("expression"))) + ")"
	}

	this := implicitDatetimeCast(asExpression(e.This()))
	expr := implicitDatetimeCast(asExpression(e.Arg("expression")))

	// A date-part-boundary WEEK diff must respect week boundaries, which DuckDB's
	// DATE_DIFF does not, so both operands are truncated to the requested week start.
	if truthy(e.Arg("date_part_boundary")) {
		if weekStart := duckDBWeekUnitStart(unit); weekStart > 0 && this != nil && expr != nil {
			this = duckDBWeekTruncExpr(this, weekStart, false, true)
			expr = duckDBWeekTruncExpr(expr, weekStart, false, true)
			return g.funcCall("DATE_DIFF", []any{expressions.LiteralString("WEEK"), expr, this}, "(", ")", true)
		}
	}
	return g.funcCall("DATE_DIFF", []any{duckDBDiffUnit(unit), expr, this}, "(", ")", true)
}

var duckDBTimezonePattern = regexp.MustCompile(`[+-]\d{2}(?::?\d{2})?$|Z$`)

// implicitDatetimeCast ports _implicit_datetime_cast (generators/duckdb.py:890-905): a string
// literal becomes a DATE, promoted to TIMESTAMP/TIMESTAMPTZ when it carries a time (and a
// timezone), so DATE_DIFF/DATE_TRUNC receive a typed operand.
func implicitDatetimeCast(arg expressions.Expression) expressions.Expression {
	if arg == nil || arg.Kind() != expressions.KindLiteral || !truthy(arg.Arg("is_string")) {
		return arg
	}
	name := arg.Name()
	dtype := expressions.DTypeDate
	if strings.Contains(name, ":") {
		dtype = expressions.DTypeTimestamp
		if duckDBTimezonePattern.MatchString(name) {
			dtype = expressions.DTypeTimestampLtz
		}
	}
	return expressions.Cast(expressions.Args{
		"this": arg,
		"to":   expressions.DataType(expressions.Args{"this": dtype}),
	})
}

// duckDBSortArraySQL ports _array_sort_sql (generators/duckdb.py:561-575). array_sort_order
// (dialects/dialect.py:3092-3100) reads literal asc and nulls_first flags; a non-literal flag
// leaves the call as LIST_SORT(this) and is reported as unsupported.
func duckDBSortArraySQL(g *Generator, e expressions.Expression) string {
	arr := e.This()
	asc, nullsFirst, ok := arraySortOrder(e)
	if !ok {
		g.unsupported("LIST_SORT only takes literal orders")
		return g.funcCall("LIST_SORT", []any{arr}, "(", ")", true)
	}
	if !nullsFirst {
		name := "LIST_SORT"
		if !asc {
			name = "ARRAY_REVERSE_SORT"
		}
		return g.funcCall(name, []any{arr}, "(", ")", true)
	}
	direction := "DESC"
	if asc {
		direction = "ASC"
	}
	return g.funcCall("LIST_SORT", []any{arr, expressions.LiteralString(direction), expressions.LiteralString("NULLS FIRST")}, "(", ")", true)
}

// arraySortOrder ports array_sort_order (dialects/dialect.py:3092-3100): asc defaults to true,
// and ok is false when a flag is not a literal boolean.
func arraySortOrder(e expressions.Expression) (asc bool, nullsFirst bool, ok bool) {
	ascExpr := asExpression(e.Arg("asc"))
	if ascExpr == nil {
		ascExpr = expressions.True_()
	}
	if ascExpr.Kind() != expressions.KindBoolean {
		return false, false, false
	}
	asc = truthy(ascExpr.Arg("this"))
	nf := asExpression(e.Arg("nulls_first"))
	if nf == nil {
		return asc, false, true
	}
	if nf.Kind() != expressions.KindBoolean {
		return false, false, false
	}
	return asc, truthy(nf.Arg("this")), true
}

// duckDBTimestampFromPartsSQL renders TimestampFromParts as MAKE_TIMESTAMP.
func duckDBTimestampFromPartsSQL(g *Generator, e expressions.Expression) string {
	if e.This() != nil && e.Arg("expression") != nil {
		return g.gen(expressions.Add(expressions.Args{"this": e.This(), "expression": e.Arg("expression")}))
	}
	sec := asExpression(e.Arg("sec"))
	if sec != nil {
		if milli := asExpression(e.Arg("milli")); milli != nil {
			sec = expressions.Add(expressions.Args{"this": sec, "expression": expressions.Paren(expressions.Args{
				"this": expressions.Div(expressions.Args{"this": milli, "expression": expressions.LiteralNumber("1000.0")}),
			})})
		}
		if nano := asExpression(e.Arg("nano")); nano != nil {
			sec = expressions.Add(expressions.Args{"this": sec, "expression": expressions.Paren(expressions.Args{
				"this": expressions.Div(expressions.Args{"this": nano, "expression": expressions.LiteralNumber("1000000000.0")}),
			})})
		}
		e.Set("sec", sec)
	}
	// Upstream pops milli/nano into sec, so they no longer appear as positional arguments.
	e.Set("milli", nil)
	e.Set("nano", nil)
	return g.funcCall("MAKE_TIMESTAMP", g.fallbackArgs(e), "(", ")", true)
}

// duckDBDateFromPartsSQL ports _date_from_parts_sql (generators/duckdb.py:1428-1456) for the
// non-overflow form.
func duckDBDateFromPartsSQL(g *Generator, e expressions.Expression) string {
	year := asExpression(e.Arg("year"))
	month := asExpression(e.Arg("month"))
	day := asExpression(e.Arg("day"))
	if !truthy(e.Arg("allow_overflow")) {
		return g.funcCall("MAKE_DATE", []any{year, month, day}, "(", ")", true)
	}
	// generators/duckdb.py:1428-1456: allow_overflow normalizes month/day through
	// MAKE_DATE(year, 1, 1) plus month/day-offset intervals.
	base := duckDBAnon("MAKE_DATE", year, expressions.LiteralNumber("1"), expressions.LiteralNumber("1"))
	wrapBinary := func(v expressions.Expression) expressions.Expression {
		if v != nil && v.Is(expressions.TraitBinary) {
			return expressions.Paren(expressions.Args{"this": v})
		}
		return v
	}
	monthInterval := expressions.Interval(expressions.Args{
		"this": expressions.Sub(expressions.Args{"this": wrapBinary(month), "expression": expressions.LiteralNumber("1")}),
		"unit": expressions.Var(expressions.Args{"this": "MONTH"}),
	})
	dayInterval := expressions.Interval(expressions.Args{
		"this": expressions.Sub(expressions.Args{"this": wrapBinary(day), "expression": expressions.LiteralNumber("1")}),
		"unit": expressions.Var(expressions.Args{"this": "DAY"}),
	})
	result := expressions.Add(expressions.Args{"this": base, "expression": monthInterval})
	result = expressions.Add(expressions.Args{"this": result, "expression": dayInterval})
	return g.gen(expressions.Cast(expressions.Args{"this": result, "to": expressions.DTypeDate.IntoExpr(nil)}))
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
// default. Text type parameters are preserved by the upstream base datatype_sql implementation.
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
	// generators/duckdb.py:658-660: modifiers are not supported for TIME,
	// [TIME | TIMESTAMP] WITH TIME ZONE.
	if dtype == expressions.DTypeTime || dtype == expressions.DTypeTimeTz || dtype == expressions.DTypeTimestampTz {
		if mapped, ok := g.lookupTypeMapping(dtype); ok {
			return mapped
		}
		return string(dtype)
	}
	params := e.Expressions()
	if dtype == expressions.DTypeBigDecimal || dtype == expressions.DTypeDecFloat {
		// generators/duckdb.py:1949-1952 TYPE_PARAM_SETTINGS: BIGDECIMAL/DECFLOAT
		// default to (38, 5) and cap their parameters at (38, 38).
		if len(params) == 0 {
			return "DECIMAL(38, 5)"
		}
		capped := make([]expressions.Expression, len(params))
		bounds := [2]string{"38", "38"}
		for i, param := range params {
			value := asExpression(param)
			if param.Kind() == expressions.KindDataTypeParam {
				value = asExpression(param.This())
			}
			if value != nil && value.IsNumber() && i < len(bounds) && duckDBNumberExceeds(value.Name(), bounds[i]) {
				g.unsupported(string(dtype) + " parameter " + value.Name() + " exceeds DuckDB's maximum of " + bounds[i] + "; capping")
				capped[i] = expressions.DataTypeParam(expressions.Args{"this": expressions.LiteralNumber(bounds[i])})
				continue
			}
			capped[i] = param
		}
		return g.dataTypeWithParams(e, dtype, capped)
	}
	if len(params) == 0 {
		switch dtype {
		case expressions.DTypeText:
			return "TEXT"
		}
	}
	return g.dataTypeSQL(e)
}

// duckDBNumberExceeds reports whether the decimal literal text exceeds bound.
func duckDBNumberExceeds(text, bound string) bool {
	return duckDBCompareDecimal(text, bound) > 0
}

func duckDBCompareDecimal(a, b string) int {
	ai, aerr := strconv.ParseFloat(a, 64)
	bi, berr := strconv.ParseFloat(b, 64)
	// A decimal literal too large for float64 (e.g. BIGDECIMAL(1e1000000)) parses to +Inf
	// with ErrRange; it still exceeds every bound DuckDB accepts.
	if math.IsInf(ai, 1) {
		return 1
	}
	if math.IsInf(ai, -1) {
		return -1
	}
	if aerr != nil || berr != nil {
		return 0
	}
	switch {
	case ai > bi:
		return 1
	case ai < bi:
		return -1
	}
	return 0
}

// dataTypeWithParams renders dtype with the given parameter nodes (the generic
// data_type_sql render without the duckDB defaults).
func (g *Generator) dataTypeWithParams(e expressions.Expression, dtype expressions.DType, params []expressions.Expression) string {
	clone := expressions.DataType(expressions.Args{
		"this":        dtype,
		"expressions": params,
		"nested":      e.Arg("nested"),
		"values":      e.Arg("values"),
	})
	return g.dataTypeSQL(clone)
}

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
		// One evaluation: TRY_CAST returns NULL for invalid JSON and JSON for valid input.
		return "TRY_CAST(" + g.sqlKey(e, "this") + " AS JSON)"
	}
	return g.funcCall("JSON", []any{e.This()}, "(", ")", true)
}

// duckDBIgnoreNullsSQL ports ignorenulls_sql (generators/duckdb.py:3987-4010): window
// functions keep the modifier (inline, since IGNORE_NULLS_IN_FUNC is set), ARRAY_AGG turns it
// into a non-NULL filter, and any other aggregate drops it.
func duckDBIgnoreNullsSQL(g *Generator, e expressions.Expression) string {
	this := asExpression(e.This())
	if this == nil {
		return ""
	}
	switch this.Kind() {
	case expressions.KindFirstValue, expressions.KindLag, expressions.KindLastValue,
		expressions.KindLead, expressions.KindNthValue:
		return g.ignoreNullsSQL(e)
	case expressions.KindArrayAgg:
		this.Set("nulls_excluded", true)
		return g.gen(this)
	case expressions.KindFirst:
		this = expressions.AnyValue(expressions.Args{"this": this.This()})
	}
	if this.Kind() != expressions.KindAnyValue && this.Kind() != expressions.KindApproxQuantiles {
		g.unsupported("IGNORE NULLS is not supported for non-window functions.")
	}
	return g.gen(this)
}

// duckDBPivot applies transforms.unqualify_columns to the whole operator
// (generators/duckdb.py:1739) before the base PIVOT rendering: DuckDB only accepts simple
// column names inside a (UN)PIVOT.
func duckDBPivot(g *Generator, e expressions.Expression) string {
	return g.pivotSQL(unqualifyColumnsDeep(e))
}

// duckDBTableSampleSQL ports tablesample_sql (generators/duckdb.py:3118-3133).
func duckDBTableSampleSQL(g *Generator, e expressions.Expression) string {
	keyword := g.dialect.TablesampleKeywords
	if parent := e.Parent(); parent == nil || parent.Kind() != expressions.KindSelect {
		keyword = "TABLESAMPLE"
	}
	if e.Arg("size") != nil {
		method := asExpression(e.Arg("method"))
		if method == nil || !strings.EqualFold(method.Name(), "RESERVOIR") {
			if method != nil {
				g.unsupported("Sampling method " + method.Name() + " is not supported with a discrete sample count, defaulting to reservoir sampling")
			}
			e.Set("method", expressions.Var(expressions.Args{"this": "RESERVOIR"}))
		}
	}
	return g.tableSampleSQLWithKeyword(e, keyword)
}

// duckDBLambdaSQL renders DuckDB's `LAMBDA x, y : body` form when the parser saw the keyword.
func duckDBLambdaSQL(g *Generator, e expressions.Expression) string {
	// generators/duckdb.py:2776-2785: from DuckDB 1.3 every lambda uses the LAMBDA form with a colon.
	if g.dialect.VersionAtLeast(1, 3) {
		params := g.expressions(exprsOptions{expression: e, key: "expressions", flat: true})
		return "LAMBDA " + params + " : " + g.sqlKey(e, "this")
	}
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
		unit := asExpression(e.Arg("unit"))
		// generators/duckdb.py:330-350: DuckDB has no INTERVAL ... NANOSECOND, so route the
		// delta through EPOCH_NS/MAKE_TIMESTAMP_NS.
		if unit != nil && (unit.Kind() == expressions.KindVar || unit.Kind() == expressions.KindLiteral) &&
			strings.EqualFold(unit.Name(), "NANOSECOND") {
			value := interval
			if interval != nil && interval.Kind() == expressions.KindInterval {
				value = asExpression(interval.Arg("this"))
			}
			timestampNS := g.gen(expressions.Cast(expressions.Args{
				"this": e.This(),
				"to":   expressions.DTypeTimestampNs.IntoExpr(nil),
			}))
			delta := g.gen(value)
			if value != nil && (value.Kind() == expressions.KindAdd || value.Kind() == expressions.KindSub) {
				delta = "(" + delta + ")"
			}
			return "MAKE_TIMESTAMP_NS(EPOCH_NS(" + timestampNS + ") " + op + " " + delta + ")"
		}
		if interval == nil || interval.Kind() != expressions.KindInterval {
			unitVar := unitToVarValue(unit)
			if unitVar != nil && unitVar.Kind() == expressions.KindVar {
				unitVar = expressions.Var(expressions.Args{"this": strings.ToUpper(unitVar.Name())})
			}
			interval = expressions.Interval(expressions.Args{
				"this": interval,
				"unit": unitVar,
			})
		}
		// dialect.py:2029-2051: cast TsOrDsAdd to its return_type, and a string-literal delta
		// operand to DATE (or DATETIME for the Datetime* variants), so the +/- interval works.
		thisSQL := g.sqlKey(e, "this")
		var toType expressions.Expression
		if e.Kind() == expressions.KindTsOrDsAdd {
			toType = asExpression(e.Arg("return_type"))
			if toType == nil {
				// temporal.py:150-152 TsOrDsAdd.return_type defaults to DATE.
				toType = expressions.DataType(expressions.Args{"this": expressions.DTypeDate})
			}
		} else if this := asExpression(e.This()); this != nil && this.Kind() == expressions.KindLiteral && truthy(this.Arg("is_string")) {
			dtype := expressions.DTypeDate
			switch e.Kind() {
			case expressions.KindDatetimeAdd, expressions.KindDatetimeSub:
				dtype = expressions.DTypeDatetime
			case expressions.KindTimestampAdd, expressions.KindTimestampSub:
				dtype = expressions.DTypeTimestamp
			}
			toType = expressions.DataType(expressions.Args{"this": dtype})
		}
		if toType != nil {
			thisSQL = g.gen(expressions.Cast(expressions.Args{"this": e.This(), "to": toType}))
		}
		return dateDeltaToBinarySQL(g, e, thisSQL+" "+op+" "+g.gen(interval))
	}
}

// dateDeltaToBinarySQL ports date_delta_to_binary_sql (dialects/dialect.py:2456-2464): the
// binary form is parenthesized when its parent is a binary, unary or AT TIME ZONE expression,
// except a parent Paren or an Add/Sub that has this node as its left operand.
func dateDeltaToBinarySQL(g *Generator, e expressions.Expression, binary string) string {
	parent := e.Parent()
	if parent == nil {
		return binary
	}
	if !(parent.Is(expressions.TraitBinary) || parent.Is(expressions.TraitUnary) || parent.Kind() == expressions.KindAtTimeZone) {
		return binary
	}
	leftOperand := (parent.Kind() == expressions.KindAdd || parent.Kind() == expressions.KindSub) && parent.This() == e
	if parent.Kind() == expressions.KindParen || leftOperand {
		return binary
	}
	return "(" + binary + ")"
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
		return g.arrayAggSQL(funcExpr)
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

// duckDBJoinSQL ports DuckDBGenerator.join_sql (generators/duckdb.py:3314-3336). MARK and
// SINGLE joins, and RIGHT SEMI/ANTI joins, render as `JOIN BY (TYPE <side_kind>)`. DuckDB has
// no ON-less `LEFT/INNER JOIN UNNEST(...)`, so such a join gets a dummy ON TRUE; any other
// plain comma-style join has its synthetic side and kind cleared first.
func duckDBJoinSQL(g *Generator, e expressions.Expression) string {
	side := strings.ToUpper(e.Text("side"))
	kind := strings.ToUpper(e.Text("kind"))
	if kind == "MARK" || kind == "SINGLE" || (side == "RIGHT" && (kind == "SEMI" || kind == "ANTI")) {
		typ := strings.Join(nonEmptyStrings(side, kind), "_")
		return g.joinSQLWithOp(e, "JOIN BY (TYPE "+typ+")")
	}
	if !truthy(e.Arg("using")) && !truthy(e.Arg("on")) && !truthy(e.Arg("nearest")) &&
		strings.ToUpper(e.Text("method")) == "" {
		switch kind {
		case "", "INNER", "OUTER":
			if this := asExpression(e.Arg("this")); this != nil && this.Kind() == expressions.KindUnnest {
				e.Set("on", expressions.True_())
				return g.joinSQL(e)
			}
			e.Set("side", nil)
			e.Set("kind", nil)
		}
	}
	return g.joinSQL(e)
}

// nonEmptyStrings returns the non-empty values in order (Python's `"_".join(x for x in ... if x)`).
func nonEmptyStrings(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// generators/duckdb.py:1729 exp.SHA renders as SHA1.
func init() {
	registerDialectDispatch("duckdb", map[expressions.Kind]func(*Generator, expressions.Expression) string{
		// generators/duckdb.py:2727 exp.StrPosition: strposition_sql (STRPOS).
		expressions.KindSHA: renameFunc("SHA1"),
		// generators/duckdb.py date-string <-> date <-> int family.
		expressions.KindDateStrToDate:   duckDBCastSQL(expressions.DTypeDate),
		expressions.KindTimeStrToDate:   duckDBCastSQL(expressions.DTypeDate),
		expressions.KindDateToDateStr:   duckDBCastSQL(expressions.DTypeText),
		expressions.KindTsOrDsToDateStr: duckDBTsOrDsToDateStr,
		expressions.KindDiToDate:        duckDBDiToDate,
		expressions.KindTsOrDiToDi:      duckDBTsOrDiToDi,
		expressions.KindApproxQuantiles: duckDBApproxQuantilesSQL,
		expressions.KindTimeStrToTime:   duckDBTimeStrToTimeSQL,
		expressions.KindArrayRemove:     duckDBArrayRemoveSQL,
	})
}

// duckDBIntervalSQL ports DuckDBGenerator.interval_sql (generators/duckdb.py:3307-3313):
// DuckDB truncates INTERVAL '1.5' DAY to one day, but keeps the fraction of INTERVAL '1.5 DAY'.
func duckDBIntervalSQL(g *Generator, e expressions.Expression) string {
	this := asExpression(e.This())
	unit := asExpression(e.Arg("unit"))
	if truthy(e.Arg("fraction_in_string")) && this != nil && unit != nil && unit.Kind() == expressions.KindVar {
		return "INTERVAL '" + this.Name() + " " + g.sqlKey(e, "unit") + "'"
	}
	return g.intervalSQL(e)
}
