package generator

import (
	"strconv"
	"strings"

	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// ClickHouse generator overrides, ported from
// .reference/sqlglot-v30.17.0/sqlglot/generators/clickhouse.py.
func init() {
	registerDialectDispatch("clickhouse", clickhouseDispatch())
	registerDialectTypeMapping("clickhouse", clickhouseTypeMapping())
	registerDialectPropertyLocations("clickhouse", map[exp.Kind]propertyLocation{
		exp.KindAutoRefreshProperty:   propertyLocationPostName,
		exp.KindDefinerProperty:       propertyLocationPostSchema,
		exp.KindOnCluster:             propertyLocationPostName,
		exp.KindPartitionedByProperty: propertyLocationPostSchema,
		exp.KindUuidProperty:          propertyLocationPostName,
		exp.KindVolatileProperty:      propertyLocationUnsupported,
	})
}

// clickhouseRename ports rename_func with the full flattened argument list (generator's
// rename passes only `this`, which is wrong for the multi-argument ClickHouse functions).
func clickhouseRename(name string) func(*Generator, exp.Expression) string {
	return func(g *Generator, e exp.Expression) string {
		return g.funcCall(name, g.fallbackArgs(e), "(", ")", true)
	}
}

// clickhouseCall renders `name(<args...>)` for the given expressions, skipping nil args.
func clickhouseCall(g *Generator, name string, args ...exp.Expression) string {
	anyArgs := make([]any, 0, len(args))
	for _, arg := range args {
		if arg != nil {
			anyArgs = append(anyArgs, arg)
		}
	}
	return g.funcCall(name, anyArgs, "(", ")", true)
}

// clickhouseDateDelta ports _datetime_delta_sql (generators/clickhouse.py:68-81).
func clickhouseDateDelta(name string) func(*Generator, exp.Expression) string {
	return func(g *Generator, e exp.Expression) string {
		unit := asExpression(e.Arg("unit"))
		if unit == nil {
			return g.funcCall(g.sqlName(e.Kind()), g.fallbackArgs(e), "(", ")", true)
		}
		return clickhouseCall(g, name,
			exp.Var(exp.Args{"this": strings.ToUpper(unit.Name())}),
			asExpression(e.Arg("expression")),
			e.This(),
			asExpression(e.Arg("zone")),
		)
	}
}

// clickhouseAnyToHas ports _any_to_has (generators/clickhouse.py:496-511).
func clickhouseAnyToHas(prefix string, fallback func(*Generator, exp.Expression) string) func(*Generator, exp.Expression) string {
	return func(g *Generator, e exp.Expression) string {
		left, right := asExpression(e.Arg("this")), asExpression(e.Arg("expression"))
		var arr, this exp.Expression
		if left != nil && left.Kind() == exp.KindAny {
			arr, this = left, right
		} else if right != nil && right.Kind() == exp.KindAny {
			arr, this = right, left
		} else {
			return fallback(g, e)
		}
		inner := asExpression(arr.Arg("this"))
		if inner != nil && inner.Kind() == exp.KindParen {
			inner = asExpression(inner.Arg("this"))
		}
		return prefix + clickhouseCall(g, "has", inner, this)
	}
}

// clickhouseJSONExtract ports json_extract_segments("JSONExtractString", quoted_index=False)
// (dialects/dialect.py:2250-2277) for the no-JSONPath case this port produces.
func clickhouseJSONExtract(g *Generator, e exp.Expression) string {
	return g.funcCall("JSONExtractString", g.fallbackArgs(e), "(", ")", true)
}

// clickhouseMapSQL ports _map_sql (generators/clickhouse.py:135-152).
func clickhouseMapSQL(g *Generator, e exp.Expression) string {
	parent := e.Parent()
	if parent == nil || parent.ArgKey() != "settings" {
		keys := asExpression(e.Arg("keys"))
		values := asExpression(e.Arg("values"))
		args := []any{}
		if keys != nil && values != nil && keys.Kind() == exp.KindArray && values.Kind() == exp.KindArray {
			ks, vs := keys.Expressions(), values.Expressions()
			for i := 0; i < len(ks) && i < len(vs); i++ {
				args = append(args, ks[i], vs[i])
			}
		} else {
			g.unsupported("Cannot convert array columns into map.")
			return g.funcCall("map", g.fallbackArgs(e), "(", ")", true)
		}
		return "map(" + g.formatArgs(args, ", ") + ")"
	}
	keys := asExpression(e.Arg("keys"))
	values := asExpression(e.Arg("values"))
	if keys == nil || values == nil || keys.Kind() != exp.KindArray || values.Kind() != exp.KindArray {
		g.unsupported("Cannot convert array columns into map.")
		return ""
	}
	ks, vs := keys.Expressions(), values.Expressions()
	parts := make([]string, 0, len(ks))
	for i := 0; i < len(ks) && i < len(vs); i++ {
		parts = append(parts, g.gen(ks[i])+": "+g.gen(vs[i]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// clickhouseUnixToTime ports _unix_to_time_sql (generators/clickhouse.py:32-48).
func clickhouseUnixToTime(g *Generator, e exp.Expression) string {
	scale := e.Arg("scale")
	if scale == nil {
		return clickhouseCall(g, "fromUnixTimestamp", exp.Cast(exp.Args{"this": e.This(), "to": exp.DTypeBigInt.IntoExpr(nil)}))
	}
	name := "fromUnixTimestamp"
	switch strings.ToUpper(toString(scale)) {
	case "MILLIS":
		name = "fromUnixTimestamp64Milli"
	case "MICROS":
		name = "fromUnixTimestamp64Micro"
	case "NANOS":
		name = "fromUnixTimestamp64Nano"
	}
	return clickhouseCall(g, name, exp.Cast(exp.Args{"this": e.This(), "to": exp.DTypeBigInt.IntoExpr(nil)}))
}

// toString renders a scalar arg value for comparison.
func toString(v any) string {
	if e, ok := v.(exp.Expression); ok {
		return e.Name()
	}
	return ""
}

// clickhouseQuantileSQL ports _quantile_sql (generators/clickhouse.py:56-65).
func clickhouseQuantileSQL(g *Generator, e exp.Expression) string {
	quantile := asExpression(e.Arg("quantile"))
	args := "(" + g.sqlKey(e, "this") + ")"
	if quantile != nil && quantile.Kind() == exp.KindArray {
		return g.funcCall("quantiles", []any{quantile.Expressions()}, "(", ")", true) + args
	}
	return g.funcCall("quantile", []any{quantile}, "(", ")", true) + args
}

// clickhouseTimeStrToTime ports _timestrtotime_sql (generators/clickhouse.py:84-132).
func clickhouseTimeStrToTime(g *Generator, e exp.Expression) string {
	ts := e.This()
	tz := asExpression(e.Arg("zone"))
	if tz != nil && ts != nil && ts.Kind() == exp.KindLiteral {
		// ClickHouse rejects a UTC offset inside the literal; drop it and pad fractional
		// seconds to the 6 digits its DateTime64 literal form expects.
		tsString := strings.TrimSpace(ts.Name())
		parts := strings.Split(tsString, ".")
		if len(parts) == 2 {
			offsetSep := "+"
			if !strings.Contains(parts[1], "+") {
				offsetSep = "-"
			}
			fracParts := strings.Split(parts[1], offsetSep)
			fracParts[0] += strings.Repeat("0", maxInt(0, 6-len(fracParts[0])))
			tsString = parts[0] + "." + fracParts[0]
			if len(fracParts) > 1 {
				tsString += offsetSep + fracParts[1]
			}
		}
		ts = exp.LiteralString(tsString)
	}
	dtypeArgs := []exp.Expression{exp.DataTypeParam(exp.Args{"this": exp.LiteralNumber("6")})}
	if tz != nil {
		dtypeArgs = append(dtypeArgs, exp.DataTypeParam(exp.Args{"this": tz}))
	}
	dtype := exp.DTypeDatetime64.IntoExpr(exp.Args{"expressions": dtypeArgs, "nullable": false})
	return g.gen(exp.Cast(exp.Args{"this": ts, "to": dtype}))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// clickhouseJSONCastSQL ports _json_cast_sql (generators/clickhouse.py:155-163).
func clickhouseJSONCastSQL(g *Generator, e exp.Expression) string {
	this := g.sqlKey(e, "this")
	to := asExpression(e.Arg("to"))
	toSQL := g.gen(to)
	if to != nil && len(to.Expressions()) > 0 {
		toSQL = g.gen(exp.ToIdentifier(toSQL))
	}
	return this + ".:" + toSQL
}

// clickhouseDispatch mirrors ClickHouseGenerator.TRANSFORMS
// (generators/clickhouse.py:278-387).
func clickhouseDispatch() map[exp.Kind]func(*Generator, exp.Expression) string {
	d := map[exp.Kind]func(*Generator, exp.Expression) string{
		exp.KindAnyValue:                 clickhouseRename("any"),
		exp.KindApproxDistinct:           clickhouseRename("uniq"),
		exp.KindArrayDistinct:            clickhouseRename("arrayDistinct"),
		exp.KindArrayConcat:              clickhouseRename("arrayConcat"),
		exp.KindArrayContains:            clickhouseRename("has"),
		exp.KindArrayRemove:              clickhouseArrayRemove,
		exp.KindArrayReverse:             clickhouseRename("arrayReverse"),
		exp.KindArraySlice:               clickhouseRename("arraySlice"),
		exp.KindArraySum:                 clickhouseRename("arraySum"),
		exp.KindArrayMax:                 clickhouseRename("arrayMax"),
		exp.KindArrayMin:                 clickhouseRename("arrayMin"),
		exp.KindArgMax:                   clickhouseArgMaxMin("argMax"),
		exp.KindArgMin:                   clickhouseArgMaxMin("argMin"),
		exp.KindArray:                    clickhouseArray,
		exp.KindCityHash64:               clickhouseRename("cityHash64"),
		exp.KindCastToStrType:            clickhouseRename("CAST"),
		exp.KindCurrentDatabase:          clickhouseRename("CURRENT_DATABASE"),
		exp.KindCurrentSchemas:           clickhouseRename("CURRENT_SCHEMAS"),
		exp.KindCountIf:                  clickhouseRename("countIf"),
		exp.KindCosineDistance:           clickhouseRename("cosineDistance"),
		exp.KindCompressColumnConstraint: clickhouseCompressConstraint,
		exp.KindComputedColumnConstraint: clickhouseComputedConstraint,
		exp.KindCurrentDate:              func(g *Generator, e exp.Expression) string { return "CURRENT_DATE()" },
		exp.KindCurrentVersion:           clickhouseRename("VERSION"),
		exp.KindDateAdd:                  clickhouseDateDelta("DATE_ADD"),
		exp.KindDateDiff:                 clickhouseDateDelta("DATE_DIFF"),
		exp.KindDateStrToDate:            clickhouseRename("toDate"),
		exp.KindDateSub:                  clickhouseDateDelta("DATE_SUB"),
		exp.KindExplode:                  clickhouseRename("arrayJoin"),
		exp.KindFarmFingerprint:          clickhouseRename("farmFingerprint64"),
		exp.KindFinal:                    clickhouseFinal,
		exp.KindIsNan:                    clickhouseRename("isNaN"),
		exp.KindJarowinklerSimilarity:    clickhouseRename("jaroWinklerSimilarity"),
		exp.KindJSONCast:                 clickhouseJSONCastSQL,
		exp.KindJSONExtract:              clickhouseJSONExtract,
		exp.KindJSONExtractScalar:        clickhouseJSONExtract,
		exp.KindMedian:                   clickhouseRename("median"),
		exp.KindNullif:                   clickhouseRename("nullIf"),
		exp.KindPartitionedByProperty:    func(g *Generator, e exp.Expression) string { return "PARTITION BY " + g.sqlKey(e, "this") },
		exp.KindPivot:                    clickhouseNoPivot,
		exp.KindQuantile:                 clickhouseQuantileSQL,
		exp.KindRegexpLike: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "match", e.This(), asExpression(e.Arg("expression")))
		},
		exp.KindRand:              clickhouseRename("randCanonical"),
		exp.KindStartsWith:        clickhouseRename("startsWith"),
		exp.KindStruct:            clickhouseRename("tuple"),
		exp.KindTrunc:             clickhouseRename("trunc"),
		exp.KindEndsWith:          clickhouseRename("endsWith"),
		exp.KindEuclideanDistance: clickhouseRename("L2Distance"),
		exp.KindStrPosition: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "POSITION", e.This(), asExpression(e.Arg("substr")), asExpression(e.Arg("position")))
		},
		exp.KindTimeToStr:     clickhouseTimeToStr,
		exp.KindTimeStrToTime: clickhouseTimeStrToTime,
		exp.KindTimestampAdd:  clickhouseDateDelta("TIMESTAMP_ADD"),
		exp.KindTimestampSub:  clickhouseDateDelta("TIMESTAMP_SUB"),
		exp.KindTypeof:        clickhouseRename("toTypeName"),
		exp.KindVarMap:        clickhouseMapSQL,
		exp.KindMap:           clickhouseMapSQL,
		exp.KindXor: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "xor", e.This(), asExpression(e.Arg("expression")))
		},
		exp.KindMD5Digest: clickhouseRename("MD5"),
		exp.KindMD5: func(g *Generator, e exp.Expression) string {
			return "LOWER(HEX(" + clickhouseCall(g, "MD5", e.This()) + "))"
		},
		exp.KindSHA:        clickhouseRename("SHA1"),
		exp.KindSHA1Digest: clickhouseRename("SHA1"),
		exp.KindSHA2: func(g *Generator, e exp.Expression) string {
			length := "256"
			if l := g.sqlKey(e, "length"); l != "" {
				length = l
			}
			return clickhouseCall(g, "SHA"+length, e.This())
		},
		exp.KindSHA2Digest: func(g *Generator, e exp.Expression) string {
			length := "256"
			if l := g.sqlKey(e, "length"); l != "" {
				length = l
			}
			return clickhouseCall(g, "SHA"+length, e.This())
		},
		exp.KindSplit: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "splitByString", asExpression(e.Arg("expression")), e.This(), asExpression(e.Arg("limit")))
		},
		exp.KindRegexpSplit: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "splitByRegexp", asExpression(e.Arg("expression")), e.This(), asExpression(e.Arg("limit")))
		},
		exp.KindUnixToTime:            clickhouseUnixToTime,
		exp.KindTrim:                  (*Generator).trimSQLStandard,
		exp.KindVariance:              clickhouseRename("varSamp"),
		exp.KindSchemaCommentProperty: func(g *Generator, e exp.Expression) string { return "COMMENT " + g.sqlKey(e, "this") },
		exp.KindStddev:                clickhouseRename("stddevSamp"),
		exp.KindChr:                   clickhouseRename("CHAR"),
		exp.KindLag: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "lagInFrame", e.This(), asExpression(e.Arg("offset")), asExpression(e.Arg("default")))
		},
		exp.KindLead: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "leadInFrame", e.This(), asExpression(e.Arg("offset")), asExpression(e.Arg("default")))
		},
		exp.KindLevenshtein: clickhouseRename("editDistance"),
		exp.KindParseDatetime: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "parseDateTime", e.This(), asExpression(e.Arg("format")), asExpression(e.Arg("zone")))
		},
		exp.KindArrayFilter: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "arrayFilter", asExpression(e.Arg("expression")), e.This())
		},
		exp.KindTransform: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "arrayMap", asExpression(e.Arg("expression")), e.This())
		},

		// Statement/property nodes.
		exp.KindDetach:           clickhouseDetach,
		exp.KindOnCluster:        func(g *Generator, e exp.Expression) string { return "ON CLUSTER " + g.sqlKey(e, "this") },
		exp.KindPartitionId:      func(g *Generator, e exp.Expression) string { return "ID " + g.sqlKey(e, "this") },
		exp.KindReplacePartition: clickhouseReplacePartition,
		exp.KindProjectionDef:    clickhouseProjectionDef,
		exp.KindNestedJSONSelect: func(g *Generator, e exp.Expression) string {
			return g.sqlKey(e, "this") + ".^" + g.sqlKey(e, "expression")
		},
		exp.KindAutoRefreshProperty: clickhouseAutoRefresh,
		exp.KindUuidProperty:        func(g *Generator, e exp.Expression) string { return "UUID " + g.sqlKey(e, "this") },
		exp.KindApply: func(g *Generator, e exp.Expression) string {
			return g.sqlKey(e, "this") + " APPLY(" + g.sqlKey(e, "expression") + ")"
		},
		exp.KindColumns: clickhouseColumns,
		exp.KindAssumeColumnConstraint: func(g *Generator, e exp.Expression) string {
			return "ASSUME (" + g.sqlKey(e, "this") + ")"
		},
		exp.KindAlterModifySqlSecurity: func(g *Generator, e exp.Expression) string {
			return "MODIFY " + g.expressions(exprsOptions{expression: e, sep: " "})
		},
		exp.KindCombinedAggFunc:          clickhouseAnonymousAgg,
		exp.KindAnonymousAggFunc:         clickhouseAnonymousAgg,
		exp.KindCombinedParameterizedAgg: clickhouseParameterizedAgg,
		exp.KindParameterizedAgg:         clickhouseParameterizedAgg,
		exp.KindIndexColumnConstraint:    clickhouseIndexConstraint,
		exp.KindPartition: func(g *Generator, e exp.Expression) string {
			return "PARTITION " + g.expressions(exprsOptions{expression: e, flat: true})
		},
		exp.KindPreWhere: clickhousePreWhere,
		exp.KindSettingsProperty: func(g *Generator, e exp.Expression) string {
			return "SETTINGS" + g.seg("") + g.expressions(exprsOptions{expression: e})
		},
		exp.KindWithFill:       clickhouseWithFill,
		exp.KindSkipJSONColumn: clickhouseSkipJSONColumn,
		exp.KindUtcTimestamp: func(g *Generator, e exp.Expression) string {
			return g.gen(exp.CurrentTimestamp(exp.Args{"this": exp.LiteralString("UTC")}))
		},
		exp.KindMergeTreeTTL:       clickhouseMergeTreeTTL,
		exp.KindMergeTreeTTLAction: clickhouseMergeTreeTTLAction,
		exp.KindDictProperty:       clickhouseDictProperty,
		exp.KindDictSubProperty:    func(g *Generator, e exp.Expression) string { return g.sqlKey(e, "this") + " " + g.sqlKey(e, "value") },
		exp.KindDictRange:          clickhouseDictRange,
		exp.KindSampleProperty:     func(g *Generator, e exp.Expression) string { return "SAMPLE BY " + g.sqlKey(e, "this") },

		// Methods reached through dispatch that ClickHouse overrides.
		exp.KindOffset:            clickhouseOffset,
		exp.KindCTE:               clickhouseCTE,
		exp.KindValues:            clickhouseValues,
		exp.KindIs:                clickhouseIs,
		exp.KindIn:                clickhouseIn,
		exp.KindNot:               clickhouseNot,
		exp.KindDataType:          clickhouseDataType,
		exp.KindCreate:            clickhouseCreate,
		exp.KindPlaceholder:       clickhousePlaceholder,
		exp.KindEQ:                clickhouseAnyToHas("", func(g *Generator, e exp.Expression) string { return g.eqSQL(e) }),
		exp.KindNEQ:               clickhouseAnyToHas("NOT ", func(g *Generator, e exp.Expression) string { return g.neqSQL(e) }),
		exp.KindRegexpILike:       clickhouseRegexpILike,
		exp.KindLikeProperty:      func(g *Generator, e exp.Expression) string { return "AS " + g.sqlKey(e, "this") },
		exp.KindJSONPathSubscript: clickhouseJSONPathSubscript,
		exp.KindGroupConcat:       clickhouseGroupConcat,
		exp.KindStrToDate:         clickhouseStrToDate,
		exp.KindCast:              clickhouseCast,
		exp.KindTryCast:           clickhouseTryCast,
		exp.KindTimestampTrunc:    clickhouseTimestampTrunc,
		exp.KindDateTrunc:         clickhouseTimestampTrunc,
	}
	return d
}

// clickhouseTypeMapping ports ClickHouseGenerator.TYPE_MAPPING
// (generators/clickhouse.py:213-276).
func clickhouseTypeMapping() map[exp.DType]string {
	return map[exp.DType]string{
		exp.DTypeBlob: "String", exp.DTypeChar: "String", exp.DTypeLongBlob: "String",
		exp.DTypeLongText: "String", exp.DTypeMediumBlob: "String", exp.DTypeMediumText: "String",
		exp.DTypeTinyBlob: "String", exp.DTypeTinyText: "String", exp.DTypeText: "String",
		exp.DTypeVarBinary: "String", exp.DTypeVarchar: "String",
		exp.DTypeArray: "Array", exp.DTypeBoolean: "Bool", exp.DTypeBigInt: "Int64",
		exp.DTypeDate32: "Date32", exp.DTypeDatetime: "DateTime", exp.DTypeDatetime2: "DateTime",
		exp.DTypeSmallDatetime: "DateTime", exp.DTypeDatetime64: "DateTime64",
		exp.DTypeDecimal: "Decimal", exp.DTypeDecimal32: "Decimal32", exp.DTypeDecimal64: "Decimal64",
		exp.DTypeDecimal128: "Decimal128", exp.DTypeDecimal256: "Decimal256",
		exp.DTypeTimestamp: "DateTime", exp.DTypeTimestampNtz: "DateTime", exp.DTypeTimestampTz: "DateTime",
		exp.DTypeDouble: "Float64", exp.DTypeEnum: "Enum", exp.DTypeEnum8: "Enum8", exp.DTypeEnum16: "Enum16",
		exp.DTypeFixedString: "FixedString", exp.DTypeFloat: "Float32", exp.DTypeInt: "Int32",
		exp.DTypeMediumInt: "Int32", exp.DTypeInt128: "Int128", exp.DTypeInt256: "Int256",
		exp.DTypeLowCardinality: "LowCardinality", exp.DTypeMap: "Map", exp.DTypeNested: "Nested",
		exp.DTypeNothing: "Nothing", exp.DTypeSmallInt: "Int16", exp.DTypeStruct: "Tuple",
		exp.DTypeTinyInt: "Int8", exp.DTypeUBigInt: "UInt64", exp.DTypeUInt: "UInt32",
		exp.DTypeUInt128: "UInt128", exp.DTypeUInt256: "UInt256", exp.DTypeUSmallInt: "UInt16",
		exp.DTypeUTinyInt: "UInt8", exp.DTypeIpv4: "IPv4", exp.DTypeIpv6: "IPv6",
		exp.DTypePoint: "Point", exp.DTypeRing: "Ring", exp.DTypeLineString: "LineString",
		exp.DTypeMultiLineString: "MultiLineString", exp.DTypePolygon: "Polygon",
		exp.DTypeMultiPolygon: "MultiPolygon", exp.DTypeAggregateFunction: "AggregateFunction",
		exp.DTypeSimpleAggregateFunction: "SimpleAggregateFunction", exp.DTypeDynamic: "Dynamic",
	}
}

var clickhouseNonNullableTypes = map[exp.DType]bool{
	exp.DTypeArray: true, exp.DTypeMap: true, exp.DTypeStruct: true, exp.DTypePoint: true,
	exp.DTypeRing: true, exp.DTypeLineString: true, exp.DTypeMultiLineString: true,
	exp.DTypePolygon: true, exp.DTypeMultiPolygon: true,
}

var clickhouseStringTypes = map[exp.DType]bool{
	exp.DTypeBlob: true, exp.DTypeChar: true, exp.DTypeLongBlob: true, exp.DTypeLongText: true,
	exp.DTypeMediumBlob: true, exp.DTypeMediumText: true, exp.DTypeTinyBlob: true,
	exp.DTypeTinyText: true, exp.DTypeText: true, exp.DTypeVarBinary: true, exp.DTypeVarchar: true,
}

// clickhouseDataType ports datatype_sql (generators/clickhouse.py:524-554).
func clickhouseDataType(g *Generator, e exp.Expression) string {
	dtype := ""
	if dt, ok := e.Arg("this").(exp.DType); ok && clickhouseStringTypes[dt] {
		dtype = "String"
	} else {
		dtype = g.dataTypeSQL(e)
	}

	nullable, hasNullable := e.Arg("nullable").(bool)
	if (hasNullable && nullable) || (!hasNullable && !clickhouseNullableExempt(e)) {
		dtype = "Nullable(" + dtype + ")"
	}
	return dtype
}

// clickhouseNullableExempt mirrors the Map-key and composite-type exclusions in
// datatype_sql (generators/clickhouse.py:543-551).
func clickhouseNullableExempt(e exp.Expression) bool {
	if dt, ok := e.Arg("this").(exp.DType); ok && clickhouseNonNullableTypes[dt] {
		return true
	}
	parent := e.Parent()
	if parent != nil && parent.Kind() == exp.KindDataType {
		if dt, ok := parent.Arg("this").(exp.DType); ok && dt == exp.DTypeMap {
			if e.Index() <= 0 {
				return true
			}
		}
	}
	return false
}

// clickhouseAutoRefresh ports autorefreshproperty_sql (generators/clickhouse.py:584-605).
func clickhouseAutoRefresh(g *Generator, e exp.Expression) string {
	cadence := g.sqlKey(e, "cadence")
	interval := e.This()
	schedule := ""
	if cadence != "" && interval != nil {
		schedule = " " + cadence + " " + clickhouseRefreshInterval(g, interval)
	}
	offset := ""
	if o := asExpression(e.Arg("offset")); o != nil {
		offset = " OFFSET " + clickhouseRefreshInterval(g, o)
	}
	randomize := ""
	if r := asExpression(e.Arg("randomize")); r != nil {
		randomize = " RANDOMIZE FOR " + clickhouseRefreshInterval(g, r)
	}
	dependencies := g.expressions(exprsOptions{expression: e, flat: true})
	if dependencies != "" {
		dependencies = " DEPENDS ON " + dependencies
	}
	settings := ""
	if s := asExpression(e.Arg("settings")); s != nil {
		settings = " " + g.gen(s)
	}
	appendStr := ""
	if boolValue(e.Arg("append")) {
		appendStr = " APPEND"
	}
	return "REFRESH" + schedule + offset + randomize + dependencies + settings + appendStr
}

// clickhouseRefreshInterval ports _refresh_interval_sql (generators/clickhouse.py:584-587).
func clickhouseRefreshInterval(g *Generator, e exp.Expression) string {
	if e.Kind() == exp.KindAdd {
		return clickhouseRefreshInterval(g, asExpression(e.Arg("this"))) + " " + clickhouseRefreshInterval(g, asExpression(e.Arg("expression")))
	}
	return g.gen(e)
}

// clickhouseDetach ports the Detach rendering (no explicit upstream method; the generic
// ddl tail renders it through the arg order).
func clickhouseDetach(g *Generator, e exp.Expression) string {
	sql := "DETACH "
	if kind := g.sqlKey(e, "kind"); kind != "" {
		sql += kind + " "
	}
	if boolValue(e.Arg("exists")) {
		sql += "IF EXISTS "
	}
	sql += g.sqlKey(e, "this")
	if cluster := asExpression(e.Arg("cluster")); cluster != nil {
		sql += " " + g.gen(cluster)
	}
	if boolValue(e.Arg("permanent")) {
		sql += " PERMANENTLY"
	}
	if boolValue(e.Arg("sync")) {
		sql += " SYNC"
	}
	return sql
}

// clickhouseReplacePartition ports replacepartition_sql (generators/clickhouse.py:665-666).
func clickhouseReplacePartition(g *Generator, e exp.Expression) string {
	return "REPLACE " + g.sqlKey(e, "expression") + " FROM " + g.sqlKey(e, "source")
}

// clickhouseProjectionDef ports projectiondef_sql (generators/clickhouse.py:668-669).
func clickhouseProjectionDef(g *Generator, e exp.Expression) string {
	return "PROJECTION " + g.sqlKey(e, "this") + " " + g.wrap(g.sqlKey(e, "expression"))
}

// clickhouseColumns ports the Columns node rendering (parsers/clickhouse.py:962-968).
func clickhouseColumns(g *Generator, e exp.Expression) string {
	return "COLUMNS(" + g.sqlKey(e, "this") + ")"
}

// clickhouseAnonymousAgg ports anonymousaggfunc_sql/combinedaggfunc_sql
// (generator.py:6106-6110).
func clickhouseAnonymousAgg(g *Generator, e exp.Expression) string {
	name := g.sqlKey(e, "this")
	args := make([]any, 0)
	for _, arg := range e.Expressions() {
		args = append(args, arg)
	}
	return g.funcCall(name, args, "(", ")", false)
}

// clickhouseParameterizedAgg ports parameterizedagg_sql/combinedparameterizedagg_sql
// (generator.py:6102-6113): `<name>(<expressions>)(<params>)`.
func clickhouseParameterizedAgg(g *Generator, e exp.Expression) string {
	args := make([]any, 0)
	for _, arg := range e.Expressions() {
		args = append(args, arg)
	}
	params := g.expressions(exprsOptions{expression: e, key: "params", flat: true})
	return g.funcCall(g.sqlKey(e, "this"), args, "(", ")", false) + "(" + params + ")"
}

// clickhouseIndexConstraint ports indexcolumnconstraint_sql (generators/clickhouse.py:647-657).
func clickhouseIndexConstraint(g *Generator, e exp.Expression) string {
	this := g.sqlKey(e, "this")
	if this != "" {
		this = " " + this
	}
	expr := g.sqlKey(e, "expression")
	if expr != "" {
		expr = " " + expr
	}
	indexType := g.sqlKey(e, "index_type")
	if indexType != "" {
		indexType = " TYPE " + indexType
	}
	granularity := g.sqlKey(e, "granularity")
	if granularity != "" {
		granularity = " GRANULARITY " + granularity
	}
	return "INDEX" + this + expr + indexType + granularity
}

// clickhousePreWhere ports prewhere_sql (generators/clickhouse.py:643-645).
func clickhousePreWhere(g *Generator, e exp.Expression) string {
	return g.seg("PREWHERE") + g.sep() + g.indent(g.sqlKey(e, "this"), 0, nil, false, false)
}

// clickhouseOffset ports offset_sql (generators/clickhouse.py:452-461).
func clickhouseOffset(g *Generator, e exp.Expression) string {
	offset := g.offsetSQL(e)
	parent := e.Parent()
	if parent != nil && parent.Kind() == exp.KindSelect {
		if limit := asExpression(parent.Arg("limit")); limit != nil && limit.Kind() == exp.KindFetch {
			offset += " ROWS"
		}
	}
	return offset
}

// clickhouseCTE ports cte_sql (generators/clickhouse.py:556-562).
func clickhouseCTE(g *Generator, e exp.Expression) string {
	if boolValue(e.Arg("scalar")) {
		this := g.sqlKey(e, "this")
		alias := g.sqlKey(e, "alias")
		return this + " AS " + alias
	}
	return g.cteSQL(e)
}

// clickhouseValues ports values_sql (generators/clickhouse.py:701-712).
func clickhouseValues(g *Generator, e exp.Expression) string {
	alias := asExpression(e.Arg("alias"))
	valuesAsTable := true
	if alias != nil && len(listFromValue(alias.Arg("columns"))) > 0 && len(e.Expressions()) > 0 {
		values := e.Expressions()[0].Expressions()
		valuesAsTable = false
		for _, value := range values {
			if value.Kind() == exp.KindTuple {
				valuesAsTable = true
				break
			}
		}
	}
	return g.valuesSQLAsTable(e, valuesAsTable)
}

// clickhouseIs ports is_sql (generators/clickhouse.py:674-681).
func clickhouseIs(g *Generator, e exp.Expression) string {
	sql := g.isSQL(e)
	if parent := e.Parent(); parent != nil && parent.Kind() == exp.KindNot {
		sql = g.wrap(sql)
	}
	return sql
}

// clickhouseIn ports in_sql (generators/clickhouse.py:683-689).
func clickhouseIn(g *Generator, e exp.Expression) string {
	sql := g.inSQL(e)
	if parent := e.Parent(); parent != nil && parent.Kind() == exp.KindNot && boolValue(e.Arg("is_global")) {
		sql = strings.Replace(sql, "GLOBAL IN", "GLOBAL NOT IN", 1)
	}
	return sql
}

// clickhouseNot ports not_sql (generators/clickhouse.py:691-699).
func clickhouseNot(g *Generator, e exp.Expression) string {
	this := asExpression(e.Arg("this"))
	if this != nil && this.Kind() == exp.KindIn {
		if boolValue(this.Arg("is_global")) {
			return g.gen(this)
		}
		e.Set("this", exp.Paren(exp.Args{"this": this}))
	}
	return g.notSQL(e)
}

// clickhousePlaceholder ports placeholder_sql (generators/clickhouse.py:578-579).
func clickhousePlaceholder(g *Generator, e exp.Expression) string {
	return "{" + e.Name() + ": " + g.sqlKey(e, "kind") + "}"
}

// clickhouseRegexpILike ports regexpilike_sql (generators/clickhouse.py:519-522).
func clickhouseRegexpILike(g *Generator, e exp.Expression) string {
	regex := clickhouseCall(g, "CONCAT", exp.LiteralString("(?i)"), asExpression(e.Arg("expression")))
	return "match(" + g.gen(e.This()) + ", " + regex + ")"
}

// clickhouseJSONPathSubscript ports _jsonpathsubscript_sql (generators/clickhouse.py:489-491).
func clickhouseJSONPathSubscript(g *Generator, e exp.Expression) string {
	this := g.sqlKey(e, "this")
	if n, err := strconv.Atoi(this); err == nil {
		return strconv.Itoa(n + 1)
	}
	return this
}

// clickhouseStrToDate ports strtodate_sql (generators/clickhouse.py:463-471).
func clickhouseStrToDate(g *Generator, e exp.Expression) string {
	sql := g.functionFallbackSQL(e)
	parent := e.Parent()
	if parent == nil || parent.Kind() != exp.KindCast {
		return g.castSQLWithPrefix(exp.Cast(exp.Args{"this": e, "to": exp.DTypeDate.IntoExpr(nil)}), "")
	}
	return sql
}

// clickhouseCast ports cast_sql's StrToDate special case (generators/clickhouse.py:473-479).
func clickhouseCast(g *Generator, e exp.Expression) string {
	this := e.This()
	if this != nil && this.Kind() == exp.KindStrToDate {
		if to := asExpression(e.Arg("to")); to != nil && to.Arg("this") == exp.DTypeDatetime {
			return g.gen(this)
		}
	}
	return g.castSQL(e)
}

// clickhouseTryCast ports trycast_sql (generators/clickhouse.py:481-487).
func clickhouseTryCast(g *Generator, e exp.Expression) string {
	dtype := asExpression(e.Arg("to"))
	if dtype != nil && dtype.Kind() == exp.KindDataType {
		dt, _ := dtype.Arg("this").(exp.DType)
		if !clickhouseNonNullableTypes[dt] {
			dtype.Set("nullable", true)
		}
	}
	return g.castSQL(e)
}

// clickhouseTimestampTrunc ports timestamptrunc_sql/datetrunc_sql
// (generators/clickhouse.py:714-722).
func clickhouseTimestampTrunc(g *Generator, e exp.Expression) string {
	unit := asExpression(e.Arg("unit"))
	if unit != nil {
		unit = exp.LiteralString(strings.ToUpper(unit.Name()))
	}
	return clickhouseCall(g, "dateTrunc", unit, e.This(), asExpression(e.Arg("zone")))
}

// clickhouseGroupConcat ports groupconcat_sql (generators/clickhouse.py:426-450).
func clickhouseGroupConcat(g *Generator, e exp.Expression) string {
	this := e.This()
	separator := asExpression(e.Arg("separator"))
	if this != nil && this.Kind() == exp.KindLimit && this.This() != nil {
		limit := this
		this = asExpression(limit.This())
		if this != nil {
			this = this.Pop()
		}
		return g.gen(exp.ParameterizedAgg(exp.Args{
			"this":        "groupConcat",
			"params":      []exp.Expression{this},
			"expressions": []exp.Expression{separator, asExpression(limit.Arg("expression"))},
		}))
	}
	if separator != nil {
		return g.gen(exp.ParameterizedAgg(exp.Args{
			"this":        "groupConcat",
			"params":      []exp.Expression{this},
			"expressions": []exp.Expression{separator},
		}))
	}
	return clickhouseCall(g, "groupConcat", this)
}

// clickhouseCreate ports create_sql (generators/clickhouse.py:625-641): a CTAS comment
// property moves after the query, which is parenthesized.
func clickhouseCreate(g *Generator, e exp.Expression) string {
	query := asExpression(e.Arg("expression"))
	var comment exp.Expression
	if query != nil && query.Is(exp.TraitQuery) {
		if prop := e.Find(exp.KindSchemaCommentProperty); prop != nil {
			comment = prop
			prop.Pop()
			e.Set("expression", exp.Paren(exp.Args{"this": query}))
		}
	}
	sql := g.createSQL(e)
	if comment != nil {
		if commentSQL := g.gen(comment); commentSQL != "" {
			sql += " " + commentSQL
		}
	}
	return sql
}

// clickhouseMergeTreeTTL ports mergetreettl_sql (generator.py:4190-4199).
func clickhouseMergeTreeTTL(g *Generator, e exp.Expression) string {
	where := g.sqlKey(e, "where")
	group := g.sqlKey(e, "group")
	aggregates := g.expressions(exprsOptions{expression: e, key: "aggregates"})
	aggregatesSQL := ""
	if aggregates != "" {
		aggregatesSQL = g.seg("SET") + g.seg(aggregates)
	}
	if where == "" && group == "" && aggregatesSQL == "" && len(e.Expressions()) == 1 {
		return "TTL " + g.expressions(exprsOptions{expression: e, flat: true})
	}
	return "TTL" + g.seg(g.expressions(exprsOptions{expression: e})) + where + group + aggregatesSQL
}

// clickhouseMergeTreeTTLAction ports mergetreettlaction_sql (generator.py:4179-4188).
func clickhouseMergeTreeTTLAction(g *Generator, e exp.Expression) string {
	sql := g.sqlKey(e, "this")
	if boolValue(e.Arg("delete")) {
		sql += " DELETE"
	}
	if recompress := g.sqlKey(e, "recompress"); recompress != "" {
		sql += " RECOMPRESS " + recompress
	}
	if toDisk := g.sqlKey(e, "to_disk"); toDisk != "" {
		sql += " TO DISK " + toDisk
	}
	if toVolume := g.sqlKey(e, "to_volume"); toVolume != "" {
		sql += " TO VOLUME " + toVolume
	}
	return sql
}

// clickhouseDictProperty ports dictproperty_sql (generator.py:4905-4910).
func clickhouseDictProperty(g *Generator, e exp.Expression) string {
	this := g.sqlKey(e, "this")
	kind := g.sqlKey(e, "kind")
	settings := g.expressions(exprsOptions{expression: e, key: "settings", sep: " "})
	args := "()"
	if settings != "" {
		args = "(" + g.sep("") + settings + g.seg(")", "")
	}
	return this + "(" + kind + args + ")"
}

// clickhouseDictRange ports dictrange_sql (generator.py:4912-4916).
func clickhouseDictRange(g *Generator, e exp.Expression) string {
	return g.sqlKey(e, "this") + "(MIN " + g.sqlKey(e, "min") + " MAX " + g.sqlKey(e, "max") + ")"
}

// clickhouseArgMaxMin ports arg_max_or_min_no_count (dialects/dialect.py).
func clickhouseArgMaxMin(name string) func(*Generator, exp.Expression) string {
	return func(g *Generator, e exp.Expression) string {
		return clickhouseCall(g, name, e.This(), asExpression(e.Arg("expression")))
	}
}

// clickhouseArrayRemove ports remove_from_array_using_filter (dialects/dialect.py:2319-2328).
func clickhouseArrayRemove(g *Generator, e exp.Expression) string {
	lambdaID := exp.ToIdentifier("_u")
	cond := exp.NEQ(exp.Args{"this": lambdaID, "expression": asExpression(e.Arg("expression"))})
	filter := exp.ArrayFilter(exp.Args{
		"this":       e.This(),
		"expression": exp.Lambda(exp.Args{"this": cond, "expressions": []exp.Expression{lambdaID}}),
	})
	return g.gen(filter)
}

// clickhouseCompressConstraint ports CompressColumnConstraint rendering
// (generators/clickhouse.py:302-304).
func clickhouseCompressConstraint(g *Generator, e exp.Expression) string {
	return "CODEC(" + g.expressions(exprsOptions{expression: e, key: "this", flat: true}) + ")"
}

// clickhouseComputedConstraint ports ComputedColumnConstraint rendering
// (generators/clickhouse.py:305-307).
func clickhouseComputedConstraint(g *Generator, e exp.Expression) string {
	prefix := "ALIAS"
	if boolValue(e.Arg("persisted")) {
		prefix = "MATERIALIZED"
	}
	return prefix + " " + g.sqlKey(e, "this")
}

// clickhouseFinal ports Final rendering (generators/clickhouse.py:316).
func clickhouseFinal(g *Generator, e exp.Expression) string {
	return g.sqlKey(e, "this") + " FINAL"
}

// clickhouseNoPivot ports no_pivot_sql (dialects/dialect.py): PIVOT is unsupported.
func clickhouseNoPivot(g *Generator, e exp.Expression) string {
	g.unsupported("PIVOT is not supported in ClickHouse")
	return ""
}

// clickhouseTimeToStr ports the TimeToStr transform (generators/clickhouse.py:345-350).
func clickhouseTimeToStr(g *Generator, e exp.Expression) string {
	return clickhouseCall(g, "formatDateTime", e.This(), asExpression(e.Arg("format")), asExpression(e.Arg("zone")))
}

// clickhouseOnClusterTargets ports ON_CLUSTER_TARGETS (generators/clickhouse.py:402-411).
var clickhouseOnClusterTargets = map[string]bool{
	"SCHEMA": true, "DATABASE": true, "TABLE": true, "VIEW": true, "DICTIONARY": true,
	"INDEX": true, "FUNCTION": true, "NAMED COLLECTION": true,
}

// clickhouseCreateable ports the ClickHouse createable_sql override
// (generators/clickhouse.py:607-623): name, POST_NAME properties, then the schema columns.
func (g *Generator) clickhouseCreateable(e exp.Expression, postName []exp.Expression) string {
	thisNode := asExpression(e.Arg("this"))
	name := g.sqlKey(e, "this")
	if thisNode != nil && thisNode.Kind() == exp.KindSchema {
		name = g.sqlKey(thisNode, "this")
	}
	props := make([]string, 0, len(postName))
	for _, prop := range postName {
		props = append(props, g.gen(prop))
	}
	schema := ""
	if thisNode != nil && thisNode.Kind() == exp.KindSchema {
		schema = g.schemaColumnsSQL(thisNode)
	}
	if schema != "" {
		schema = g.sep() + schema
	}
	return name + g.sep() + strings.Join(props, " ") + schema
}

// clickhouseArray ports the inline array rendering (dialects/dialect.py inline_array_sql):
// ClickHouse array literals are bracket-delimited.
func clickhouseArray(g *Generator, e exp.Expression) string {
	return "[" + g.expressions(exprsOptions{expression: e, flat: true}) + "]"
}

// clickhouseWithFill ports withfill_sql (generator.py:3087-3101).
func clickhouseWithFill(g *Generator, e exp.Expression) string {
	fromSQL := ""
	if from := asExpression(e.Arg("from_")); from != nil {
		fromSQL = " FROM " + g.gen(from)
	}
	toSQL := ""
	if to := asExpression(e.Arg("to")); to != nil {
		toSQL = " TO " + g.gen(to)
	}
	stepSQL := ""
	if step := asExpression(e.Arg("step")); step != nil {
		stepSQL = " STEP " + g.gen(step)
	}
	interpolate := ""
	var values []string
	for _, item := range listFromValue(e.Arg("interpolate")) {
		expr := asExpression(item)
		if expr == nil {
			continue
		}
		if expr.Kind() == exp.KindAlias {
			values = append(values, g.sqlKey(expr, "alias")+" AS "+g.sqlKey(expr, "this"))
		} else {
			values = append(values, g.gen(expr))
		}
	}
	if len(values) > 0 {
		interpolate = " INTERPOLATE (" + strings.Join(values, ", ") + ")"
	}
	return "WITH FILL" + fromSQL + toSQL + stepSQL + interpolate
}

// clickhouseSkipJSONColumn ports the SkipJSONColumn rendering (parser.py:6725-6731).
func clickhouseSkipJSONColumn(g *Generator, e exp.Expression) string {
	sql := "SKIP "
	if boolValue(e.Arg("regexp")) {
		sql += "REGEXP "
	}
	return sql + g.sqlKey(e, "expression")
}
