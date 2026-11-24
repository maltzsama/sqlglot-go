package generator

import (
	"strings"

	"github.com/maltzsama/sqlglot-go/dialects"
	"github.com/maltzsama/sqlglot-go/expressions"
)

// Snowflake recreates the SnowflakeGenerator TRANSFORMS/TYPE_MAPPING surface
// (generators/snowflake.py:651-1233) for the parts that change same-dialect output.
func init() {
	registerDialectTypeMapping("snowflake", map[expressions.DType]string{
		expressions.DTypeBigDecimal:   "DOUBLE",
		expressions.DTypeJSON:         "VARIANT",
		expressions.DTypeNested:       "OBJECT",
		expressions.DTypeStruct:       "OBJECT",
		expressions.DTypeText:         "VARCHAR",
		expressions.DTypeTimestampNtz: "TIMESTAMPNTZ",
		expressions.DTypeTimestampLtz: "TIMESTAMPLTZ",
		expressions.DTypeTimestampTz:  "TIMESTAMPTZ",
	})
	// generators/snowflake.py:670-671 exp.SetProperty / exp.VolatileProperty: UNSUPPORTED.
	registerDialectPropertyLocations("snowflake", map[expressions.Kind]propertyLocation{
		expressions.KindSetProperty:      propertyLocationUnsupported,
		expressions.KindVolatileProperty: propertyLocationUnsupported,
	})

	handlers := map[expressions.Kind]func(*Generator, expressions.Expression) string{
		expressions.KindApproxDistinct:        clickhouseRename("APPROX_COUNT_DISTINCT"),
		expressions.KindApproximateSimilarity: clickhouseRename("APPROXIMATE_SIMILARITY"),
		expressions.KindArgMax:                clickhouseRename("MAX_BY"),
		expressions.KindArgMin:                clickhouseRename("MIN_BY"),
		expressions.KindArray:                 func(g *Generator, e expressions.Expression) string { return g.inlineArraySQL(e) },
		expressions.KindArrayConcat:           func(g *Generator, e expressions.Expression) string { return g.snowflakeArrayConcatSQL(e) },
		expressions.KindArrayAppend:           snowflakeArrayAppendSQL("ARRAY_APPEND", false),
		expressions.KindArrayPrepend:          snowflakeArrayAppendSQL("ARRAY_PREPEND", true),
		expressions.KindArrayContains:         snowflakeArrayContainsSQL,
		expressions.KindArrayPosition:         snowflakeArrayPositionSQL,
		expressions.KindArrayIntersect:        clickhouseRename("ARRAY_INTERSECTION"),
		expressions.KindArrayOverlaps:         clickhouseRename("ARRAYS_OVERLAP"),
		expressions.KindBitwiseAnd:            clickhouseRename("BITAND"),
		expressions.KindBitwiseOr:             clickhouseRename("BITOR"),
		expressions.KindBitwiseXor:            clickhouseRename("BITXOR"),
		expressions.KindBitwiseNot:            clickhouseRename("BITNOT"),
		expressions.KindBitwiseAndAgg:         clickhouseRename("BITANDAGG"),
		expressions.KindBitwiseOrAgg:          clickhouseRename("BITORAGG"),
		expressions.KindBitwiseXorAgg:         clickhouseRename("BITXORAGG"),
		expressions.KindBitwiseLeftShift:      clickhouseRename("BITSHIFTLEFT"),
		expressions.KindBitwiseRightShift:     clickhouseRename("BITSHIFTRIGHT"),
		expressions.KindDayOfMonth:            clickhouseRename("DAYOFMONTH"),
		expressions.KindDayOfWeek:             clickhouseRename("DAYOFWEEK"),
		expressions.KindDayOfWeekIso:          clickhouseRename("DAYOFWEEKISO"),
		expressions.KindDayOfYear:             clickhouseRename("DAYOFYEAR"),
		expressions.KindWeekOfYear:            clickhouseRename("WEEKISO"),
		expressions.KindYearOfWeek:            clickhouseRename("YEAROFWEEK"),
		expressions.KindYearOfWeekIso:         clickhouseRename("YEAROFWEEKISO"),
		expressions.KindExplode:               clickhouseRename("FLATTEN"),
		expressions.KindCosineDistance:        clickhouseRename("VECTOR_COSINE_SIMILARITY"),
		expressions.KindEuclideanDistance:     clickhouseRename("VECTOR_L2_DISTANCE"),
		expressions.KindManhattanDistance:     clickhouseRename("VECTOR_L1_DISTANCE"),
		expressions.KindDotProduct:            clickhouseRename("VECTOR_INNER_PRODUCT"),
		// generators/snowflake.py:510 exp.Levenshtein, with unsupported cost args.
		expressions.KindLevenshtein:      clickhouseRename("EDITDISTANCE"),
		expressions.KindIf:               snowflakeIfSQL,
		expressions.KindStar:             snowflakeStarSQL,
		expressions.KindMax:              snowflakeMaxOrGreatest,
		expressions.KindMin:              snowflakeMinOrLeast,
		expressions.KindStarMap:          clickhouseRename("OBJECT_CONSTRUCT"),
		expressions.KindJSONKeys:         clickhouseRename("OBJECT_KEYS"),
		expressions.KindJSONFormat:       clickhouseRename("TO_JSON"),
		expressions.KindJSONExtract:      snowflakeJSONExtractSQL,
		expressions.KindUnixToTime:       snowflakeUnixToTimeSQL,
		expressions.KindTimeToStr:        snowflakeTimeToStrSQL,
		expressions.KindStrToTime:        snowflakeStrToTimeSQL,
		expressions.KindToNumber:         snowflakeToNumberSQL,
		expressions.KindRegexpExtract:    snowflakeRegexpExtractSQL,
		expressions.KindRegexpExtractAll: snowflakeRegexpExtractSQL,
		expressions.KindRegexpSubstr:     snowflakeRegexpExtractSQL,
		expressions.KindSHA:              clickhouseRename("SHA1"),
		expressions.KindSHA1Digest:       clickhouseRename("SHA1_BINARY"),
		expressions.KindMD5Digest:        clickhouseRename("MD5_BINARY"),
		expressions.KindMD5NumberLower64: clickhouseRename("MD5_NUMBER_LOWER64"),
		expressions.KindMD5NumberUpper64: clickhouseRename("MD5_NUMBER_UPPER64"),
		expressions.KindHex:              clickhouseRename("HEX_ENCODE"),
		expressions.KindSkewness:         clickhouseRename("SKEW"),
		expressions.KindStartsWith:       clickhouseRename("STARTSWITH"),
		expressions.KindEndsWith:         clickhouseRename("ENDSWITH"),
		expressions.KindSpace: func(g *Generator, e expressions.Expression) string {
			return g.gen(expressions.Repeat(expressions.Args{
				"this": expressions.LiteralString(" "), "times": e.This(),
			}))
		},
		expressions.KindStrtok: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("STRTOK", []any{e.This(), e.Arg("delimiter"), e.Arg("part_index")}, "(", ")", true)
		},
		expressions.KindStddevSamp:         clickhouseRename("STDDEV"),
		expressions.KindStuff:              clickhouseRename("INSERT"),
		expressions.KindStPoint:            clickhouseRename("ST_MAKEPOINT"),
		expressions.KindStringToArray:      clickhouseRename("STRTOK_TO_ARRAY"),
		expressions.KindStrtokToArray:      clickhouseRename("STRTOK_TO_ARRAY"),
		expressions.KindToArray:            clickhouseRename("TO_ARRAY"),
		expressions.KindBooland:            clickhouseRename("BOOLAND"),
		expressions.KindBoolor:             clickhouseRename("BOOLOR"),
		expressions.KindLogicalAnd:         clickhouseRename("BOOLAND_AGG"),
		expressions.KindLogicalOr:          clickhouseRename("BOOLOR_AGG"),
		expressions.KindUnhex:              clickhouseRename("HEX_DECODE_BINARY"),
		expressions.KindUuid:               clickhouseRename("UUID_STRING"),
		expressions.KindJSONValueArray:     snowflakeJSONValueArraySQL,
		expressions.KindJSONExtractArray:   snowflakeJSONValueArraySQL,
		expressions.KindDateAdd:            snowflakeDateDeltaSQL("DATEADD"),
		expressions.KindDateSub:            snowflakeDateDeltaSQL("DATEADD"),
		expressions.KindDateDiff:           snowflakeDateDeltaSQL("DATEDIFF"),
		expressions.KindDatetimeAdd:        snowflakeDateDeltaSQL("TIMESTAMPADD"),
		expressions.KindDatetimeSub:        snowflakeDateDeltaSQL("TIMESTAMPADD"),
		expressions.KindDatetimeDiff:       snowflakeDateDeltaSQL("TIMESTAMPDIFF"),
		expressions.KindTimeAdd:            snowflakeDateDeltaSQL("TIMEADD"),
		expressions.KindTimeSub:            snowflakeDateDeltaSQL("TIMEADD"),
		expressions.KindTimestampAdd:       snowflakeDateDeltaSQL("DATEADD"),
		expressions.KindTimestampSub:       snowflakeDateDeltaSQL("TIMESTAMPADD"),
		expressions.KindTimestampDiff:      snowflakeDateDeltaSQL("DATEDIFF"),
		expressions.KindTimeDiff:           snowflakeDateDeltaSQL("DATEDIFF"),
		expressions.KindTsOrDsAdd:          snowflakeDateDeltaSQL("DATEADD"),
		expressions.KindTsOrDsDiff:         snowflakeDateDeltaSQL("DATEDIFF"),
		expressions.KindDateTrunc:          snowflakeDateTruncSQL,
		expressions.KindTimestampTrunc:     snowflakeDateTruncSQL,
		expressions.KindDateFromParts:      clickhouseRename("DATE_FROM_PARTS"),
		expressions.KindTimeFromParts:      clickhouseRename("TIME_FROM_PARTS"),
		expressions.KindTimestampFromParts: clickhouseRename("TIMESTAMP_FROM_PARTS"),
		expressions.KindNthValue:           snowflakeNthValueSQL,
		expressions.KindGroupConcat:        snowflakeGroupConcatSQL,
		expressions.KindStrPosition:        snowflakeStrPositionSQL,
		expressions.KindLocaltime: func(g *Generator, e expressions.Expression) string {
			if e.Arg("this") != nil {
				return g.funcCall("CURRENT_TIME", []any{e.Arg("this")}, "(", ")", true)
			}
			return "CURRENT_TIME"
		},
		expressions.KindLocaltimestamp: func(*Generator, expressions.Expression) string { return "CURRENT_TIMESTAMP" },
		expressions.KindCurrentTimestamp: func(g *Generator, e expressions.Expression) string {
			if e.Arg("this") != nil {
				return g.funcCall("CURRENT_TIMESTAMP", []any{e.Arg("this")}, "(", ")", true)
			}
			return "CURRENT_TIMESTAMP()"
		},
		expressions.KindXor: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("BOOLXOR", []any{e.This(), e.Arg("expression")}, "(", ")", true)
		},
		expressions.KindBoolnot: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("BOOLNOT", []any{e.This()}, "(", ")", true)
		},
		expressions.KindJSONObject:   snowflakeJSONObjectSQL,
		expressions.KindTableSample:  snowflakeTableSampleSQL,
		expressions.KindPivot:        snowflakePivot,
		expressions.KindExtract:      snowflakeExtractSQL,
		expressions.KindDate:         snowflakeDateSQL,
		expressions.KindTsOrDsToDate: snowflakeTsOrDsToDateSQL,
		// generators/snowflake.py:754-760: spatial casts use constructor functions instead
		// of CAST syntax.
		expressions.KindCast: snowflakeCastSQL,
		// generators/snowflake.py:567 exp.Rand: lambda e: func("RANDOM", e.this) — only the
		// seed is rendered; the parser-injected lower/upper bounds are dropped.
		expressions.KindRand: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("RANDOM", []any{e.Arg("this")}, "(", ")", true)
		},
		expressions.KindUndrop:   snowflakeUndropSQL,
		expressions.KindDescribe: snowflakeDescribeSQL,
	}
	registerDialectDispatch("snowflake", handlers)
}

func snowflakeStrToTimeSQL(g *Generator, e expressions.Expression) string {
	name := "TO_TIMESTAMP"
	if target := asExpression(e.Arg("target_type")); target != nil &&
		target.Arg("this") == expressions.DTypeTime {
		name = "TO_TIME"
	}
	if boolValue(e.Arg("safe")) {
		name = "TRY_" + name
	}
	args := []any{e.Arg("this")}
	if format := asExpression(e.Arg("format")); format != nil {
		if format.IsString() {
			if converted, ok := dialects.FormatTime(format.Name(), g.dialect.InverseTimeMapping); ok {
				format = expressions.LiteralString(converted)
			}
		}
		args = append(args, format)
	}
	return g.funcCall(name, args, "(", ")", true)
}

func snowflakeToNumberSQL(g *Generator, e expressions.Expression) string {
	name := "TO_NUMBER"
	if boolValue(e.Arg("safe")) {
		name = "TRY_TO_NUMBER"
	}
	precision := asExpression(e.Arg("precision"))
	scale := asExpression(e.Arg("scale"))
	if precision != nil && precision.Name() == "38" && scale != nil && scale.Name() == "0" {
		precision, scale = nil, nil
	} else if scale != nil && scale.Name() == "0" {
		scale = nil
	}
	return g.funcCall(name, []any{e.Arg("this"), e.Arg("format"), precision, scale}, "(", ")", true)
}

func snowflakeNthValueSQL(g *Generator, e expressions.Expression) string {
	result := g.funcCall("NTH_VALUE", []any{e.This(), e.Arg("offset")}, "(", ")", true)
	if fromFirst, ok := e.Arg("from_first").(bool); ok {
		if fromFirst {
			return result + " FROM FIRST"
		}
		return result + " FROM LAST"
	}
	return result
}

func snowflakeTimeToStrSQL(g *Generator, e expressions.Expression) string {
	args := []any{e.Arg("this")}
	if format := asExpression(e.Arg("format")); format != nil {
		if format.IsString() {
			if converted, ok := dialects.FormatTime(format.Name(), g.dialect.InverseTimeMapping); ok {
				format = expressions.LiteralString(converted)
			}
		}
		args = append(args, format)
	}
	return g.funcCall("TO_CHAR", args, "(", ")", true)
}

func snowflakeRegexpExtractSQL(g *Generator, e expressions.Expression) string {
	name := "REGEXP_SUBSTR"
	if e.Kind() == expressions.KindRegexpExtractAll {
		name = "REGEXP_SUBSTR_ALL"
	}
	args := []any{e.Arg("this"), e.Arg("expression"), e.Arg("position"), e.Arg("occurrence"), e.Arg("parameters")}
	if group := asExpression(e.Arg("group")); group != nil && !(group.IsNumber() && group.Name() == "0") {
		args = append(args, group)
	}
	return g.funcCall(name, args, "(", ")", true)
}

func snowflakeUnixToTimeSQL(g *Generator, e expressions.Expression) string {
	args := []any{e.Arg("this")}
	if scale := e.Arg("scale"); scale != nil {
		args = append(args, scale)
	}
	return g.funcCall("TO_TIMESTAMP", args, "(", ")", true)
}

func snowflakeStarSQL(g *Generator, e expressions.Expression) string {
	except := g.expressions(exprsOptions{expression: e, key: "except_", flat: true})
	if except != "" {
		except = g.seg("EXCLUDE") + " (" + except + ")"
	}
	replace := g.expressions(exprsOptions{expression: e, key: "replace", flat: true})
	if replace != "" {
		replace = g.seg("REPLACE") + " (" + replace + ")"
	}
	rename := g.expressions(exprsOptions{expression: e, key: "rename", flat: true})
	if rename != "" {
		rename = g.seg("RENAME") + " (" + rename + ")"
	}
	ilike := g.sqlKey(e, "ilike")
	if ilike != "" {
		ilike = g.seg("ILIKE") + " " + ilike
	}
	return "*" + ilike + except + replace + rename
}

func snowflakeJSONExtractSQL(g *Generator, e expressions.Expression) string {
	this := asExpression(e.Arg("this"))
	if this != nil && this.Kind() != expressions.KindParseJSON &&
		this.Kind() != expressions.KindJSONExtract && !truthy(e.Arg("requires_json")) {
		this = expressions.ParseJSON(expressions.Args{"this": this})
	}
	path := asExpression(e.Arg("expression"))
	if path != nil && path.Kind() == expressions.KindJSONPath {
		parts := path.Expressions()
		if len(parts) > 0 && parts[0].Kind() == expressions.KindJSONPathRoot {
			parts = parts[1:]
		}
		path = expressions.JSONPath(expressions.Args{"expressions": parts})
	}
	pathSQL := g.gen(path)
	if pathSQL != "''" {
		pathSQL = strings.ReplaceAll(pathSQL, "''", "\\'")
	}
	return g.funcCall("GET_PATH", []any{this, pathSQL}, "(", ")", true)
}

// snowflakePivot applies _unqualify_pivot_columns (generators/snowflake.py:74-96) before the
// base PIVOT rendering: Snowflake doesn't allow columns referenced in UNPIVOT to be qualified,
// so the whole operator is unqualified; for PIVOT only the `ANY ORDER BY <column>` inside a
// PivotAny is unqualified, leaving the aggregate and FOR field qualified.
func snowflakePivot(g *Generator, e expressions.Expression) string {
	if boolValue(e.Arg("unpivot")) {
		e = unqualifyColumnsDeep(e)
	} else if fields, ok := e.Arg("fields").([]expressions.Expression); ok {
		for _, field := range fields {
			if field == nil {
				continue
			}
			fieldExprs := field.Expressions()
			if len(fieldExprs) > 0 && fieldExprs[0].Kind() == expressions.KindPivotAny {
				field.SetAt("expressions", unqualifyColumnsDeep(fieldExprs[0]), 0, true)
			}
		}
	}
	return g.pivotSQL(e)
}

// snowflakeArrayConcatSQL ports array_concat_sql("ARRAY_CAT").
func (g *Generator) snowflakeArrayConcatSQL(e expressions.Expression) string {
	this := e.This()
	rest := e.Expressions()
	return g.funcCall("ARRAY_CAT", append([]any{this}, listToAny(rest)...), "(", ")", true)
}

func listToAny(exprs []expressions.Expression) []any {
	out := make([]any, 0, len(exprs))
	for _, expr := range exprs {
		out = append(out, expr)
	}
	return out
}

// snowflakeArrayAppendSQL ports array_append_sql with Snowflake's argument order.
func snowflakeArrayAppendSQL(name string, swap bool) func(*Generator, expressions.Expression) string {
	return func(g *Generator, e expressions.Expression) string {
		this, expr := e.This(), asExpression(e.Arg("expression"))
		if swap {
			this, expr = expr, this
		}
		return g.funcCall(name, []any{this, expr}, "(", ")", true)
	}
}

// snowflakeArrayContainsSQL ports the ArrayContains lambda (generators/snowflake.py:662-667).
func snowflakeArrayContainsSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("ARRAY_CONTAINS", []any{e.Arg("expression"), e.This()}, "(", ")", true)
}

// snowflakeArrayPositionSQL ports the ArrayPosition lambda (generators/snowflake.py:669-673).
func snowflakeArrayPositionSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("ARRAY_POSITION", []any{e.Arg("expression"), e.This()}, "(", ")", true)
}

// snowflakeJSONObjectSQL ports the JSONObject lambda (generators/snowflake.py:507):
// OBJECT_CONSTRUCT_KEEP_NULL over the flattened key/value pairs.
func snowflakeJSONObjectSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("OBJECT_CONSTRUCT_KEEP_NULL", listToAny(e.Expressions()), "(", ")", true)
}

// snowflakeIfSQL ports if_sql for Snowflake's IFF.
func snowflakeIfSQL(g *Generator, e expressions.Expression) string {
	falseValue := e.Arg("false")
	if falseValue == nil {
		falseValue = expressions.Null()
	}
	return g.funcCall("IFF", []any{e.This(), e.Arg("true"), falseValue}, "(", ")", true)
}

// snowflakeMaxOrGreatest ports max_or_greatest (dialects/dialect.py:1807-1809).
func snowflakeMaxOrGreatest(g *Generator, e expressions.Expression) string {
	name := "MAX"
	if len(e.Expressions()) > 0 {
		name = "GREATEST"
	}
	return g.funcCall(name, g.fallbackArgs(e), "(", ")", true)
}

// snowflakeMinOrLeast ports min_or_least (dialects/dialect.py:1802-1804).
func snowflakeMinOrLeast(g *Generator, e expressions.Expression) string {
	name := "MIN"
	if len(e.Expressions()) > 0 {
		name = "LEAST"
	}
	return g.funcCall(name, g.fallbackArgs(e), "(", ")", true)
}

// snowflakeDateDeltaSQL ports date_delta_sql (dialects/dialect.py:2014-2026).
func snowflakeDateDeltaSQL(name string) func(*Generator, expressions.Expression) string {
	return func(g *Generator, e expressions.Expression) string {
		return g.funcCall(name, []any{
			snowflakeUnit(g, asExpression(e.Arg("unit"))),
			e.Arg("expression"),
			e.This(),
		}, "(", ")", true)
	}
}

// snowflakeDateTruncSQL ports timestamptrunc_sql (dialects/dialect.py:1700-1710) with the
// Snowflake quoted unit.
func snowflakeDateTruncSQL(g *Generator, e expressions.Expression) string {
	unit := asExpression(e.Arg("unit"))
	var unitSQL any
	if unit != nil {
		name := strings.ToUpper(unit.Name())
		if mapped := g.dialect.DatePartMapping[name]; mapped != "" {
			name = mapped
		}
		unitSQL = expressions.LiteralString(name)
	}
	return g.funcCall("DATE_TRUNC", []any{unitSQL, e.This()}, "(", ")", true)
}

// snowflakeDescribeSQL ports describe_sql keeping Snowflake's object kind keyword.
func snowflakeDescribeSQL(g *Generator, e expressions.Expression) string {
	kind := g.sqlKey(e, "kind")
	if kind != "" {
		kind += " "
	}
	target := g.sqlKey(e, "this")
	if kind == "" {
		if table := asExpression(e.Arg("this")); table != nil && table.Kind() == expressions.KindTable {
			kind = "TABLE "
		}
	}
	trailing := g.expressions(exprsOptions{expression: e, flat: true})
	if trailing != "" {
		trailing = " " + trailing
	}
	return "DESCRIBE " + kind + target + trailing
}

// snowflakeGroupConcatSQL ports the GroupConcat lambda (generators/snowflake.py:709-712)
// for the plain LISTAGG form.
func snowflakeGroupConcatSQL(g *Generator, e expressions.Expression) string {
	args := []any{e.This()}
	if sep := asExpression(e.Arg("separator")); sep != nil {
		args = append(args, sep)
	}
	return g.funcCall("LISTAGG", args, "(", ")", true)
}

// snowflakeStrPositionSQL ports the StrPosition lambda (generators/snowflake.py:743-746).
func snowflakeStrPositionSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("CHARINDEX", []any{e.Arg("substr"), e.This(), e.Arg("position")}, "(", ")", true)
}

// snowflakeJSONValueArraySQL renders JSON value arrays as ARRAY_AGG over GET_PATH.
func snowflakeJSONValueArraySQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("JSON_EXTRACT_PATH_TEXT", []any{e.This(), e.Arg("expression")}, "(", ")", true)
}

// snowflakeTableSampleSQL ports tablesample_sql for the BERNOULLI default.
func snowflakeTableSampleSQL(g *Generator, e expressions.Expression) string {
	if e.Arg("method") == nil {
		e.Set("method", expressions.Var(expressions.Args{"this": "BERNOULLI"}))
	}
	return g.tablesampleSQL(e)
}

// snowflakeDescribeKeyword is unused but kept for symmetry with future describe overrides.
var _ = strings.ToUpper

// snowflakeExtractSQL ports extract_sql (generators/snowflake.py:...): EXTRACT renders as
// DATE_PART(<unit>, <this>).
func snowflakeExtractSQL(g *Generator, e expressions.Expression) string {
	unit := asExpression(e.Arg("this"))
	return g.funcCall("DATE_PART", []any{unit, e.Arg("expression")}, "(", ")", true)
}

// snowflakeDateSQL renders DATE(<args>) as TO_DATE.
func snowflakeDateSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("TO_DATE", g.fallbackArgs(e), "(", ")", true)
}

// snowflakeTsOrDsToDateSQL ports the formatted DATE branch of _build_datetime and
// SnowflakeGenerator's time-format conversion (parsers/snowflake.py:82-128).
func snowflakeTsOrDsToDateSQL(g *Generator, e expressions.Expression) string {
	name := "TO_DATE"
	if boolValue(e.Arg("safe")) {
		name = "TRY_TO_DATE"
	}
	args := []any{e.Arg("this")}
	if format := asExpression(e.Arg("format")); format != nil {
		if format.IsString() {
			if converted, ok := dialects.FormatTime(format.Name(), g.dialect.InverseTimeMapping); ok {
				format = expressions.LiteralString(converted)
			}
		}
		args = append(args, format)
	}
	return g.funcCall(name, args, "(", ")", true)
}

// snowflakeCastSQL ports SnowflakeGenerator.cast_sql (generators/snowflake.py:754-760).
func snowflakeCastSQL(g *Generator, e expressions.Expression) string {
	to := asExpression(e.Arg("to"))
	if to != nil {
		if dtype, ok := to.Arg("this").(expressions.DType); ok {
			switch dtype {
			case expressions.DTypeGeography:
				return g.funcCall("TO_GEOGRAPHY", []any{e.Arg("this")}, "(", ")", true)
			case expressions.DTypeGeometry:
				return g.funcCall("TO_GEOMETRY", []any{e.Arg("this")}, "(", ")", true)
			}
		}
	}
	return g.castSQL(e)
}

// snowflakeUnit maps a date part through the dialect's DATE_PART_MAPPING and renders it as a
// bare uppercase Var (DATEADD(YEAR, ...) for an input unit of `y`).
func snowflakeUnit(g *Generator, unit expressions.Expression) any {
	if unit == nil {
		return nil
	}
	if unit.Kind() == expressions.KindVar || unit.Kind() == expressions.KindLiteral {
		name := strings.ToUpper(unit.Name())
		if mapped := g.dialect.DatePartMapping[name]; mapped != "" {
			name = mapped
		}
		return expressions.Var(expressions.Args{"this": name})
	}
	return unit
}

// snowflakeUndropSQL ports undrop_sql (generators/snowflake.py:846-850).
func snowflakeUndropSQL(g *Generator, e expressions.Expression) string {
	kind := e.Text("kind")
	if kind != "" {
		kind += " "
	}
	rename := g.sqlKey(e, "rename")
	if rename != "" {
		rename = " RENAME TO " + rename
	}
	return "UNDROP " + kind + g.sqlKey(e, "this") + rename
}
