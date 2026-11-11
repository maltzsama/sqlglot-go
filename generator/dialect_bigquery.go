package generator

import (
	"strconv"
	"strings"

	"github.com/maltzsama/sqlglot-go/dialects"
	"github.com/maltzsama/sqlglot-go/expressions"
)

// BigQuery registers the BigQueryGenerator TRANSFORMS, TYPE_MAPPING and PROPERTIES_LOCATION
// (generators/bigquery.py:302-481) plus its method overrides (generators/bigquery.py:583-733).
func init() {
	registerDialectTypeMapping("bigquery", map[expressions.DType]string{
		expressions.DTypeBigDecimal:   "BIGNUMERIC",
		expressions.DTypeBigInt:       "INT64",
		expressions.DTypeBinary:       "BYTES",
		expressions.DTypeBlob:         "BYTES",
		expressions.DTypeBoolean:      "BOOL",
		expressions.DTypeChar:         "STRING",
		expressions.DTypeDecimal:      "NUMERIC",
		expressions.DTypeDouble:       "FLOAT64",
		expressions.DTypeFloat:        "FLOAT64",
		expressions.DTypeInt:          "INT64",
		expressions.DTypeNChar:        "STRING",
		expressions.DTypeNVarchar:     "STRING",
		expressions.DTypeSmallInt:     "INT64",
		expressions.DTypeText:         "STRING",
		expressions.DTypeTimestamp:    "DATETIME",
		expressions.DTypeTimestampNtz: "DATETIME",
		expressions.DTypeTimestampTz:  "TIMESTAMP",
		expressions.DTypeTimestampLtz: "TIMESTAMP",
		expressions.DTypeTinyInt:      "INT64",
		expressions.DTypeRowVersion:   "BYTES",
		expressions.DTypeUUID:         "STRING",
		expressions.DTypeVarBinary:    "BYTES",
		expressions.DTypeVarchar:      "STRING",
		expressions.DTypeVariant:      "ANY TYPE",
	})

	registerDialectPropertyLocations("bigquery", map[expressions.Kind]propertyLocation{
		expressions.KindPartitionedByProperty: propertyLocationPostSchema,
		expressions.KindVolatileProperty:      propertyLocationUnsupported,
	})

	registerDialectDispatch("bigquery", map[expressions.Kind]func(*Generator, expressions.Expression) string{
		expressions.KindAIEmbed:               clickhouseRename("EMBED"),
		expressions.KindAIGenerate:            clickhouseRename("GENERATE"),
		expressions.KindAISimilarity:          clickhouseRename("SIMILARITY"),
		expressions.KindApproxTopK:            clickhouseRename("APPROX_TOP_COUNT"),
		expressions.KindApproxDistinct:        clickhouseRename("APPROX_COUNT_DISTINCT"),
		expressions.KindArgMax:                bigQueryArgMaxMin("MAX_BY"),
		expressions.KindArgMin:                bigQueryArgMaxMin("MIN_BY"),
		expressions.KindArray:                 bigQueryArraySQL,
		expressions.KindArrayContains:         bigQueryArrayContainsSQL,
		expressions.KindArrayFilter:           bigQueryFilterArrayUsingUnnest,
		expressions.KindArrayRemove:           bigQueryFilterArrayUsingUnnest,
		expressions.KindBitwiseAndAgg:         clickhouseRename("BIT_AND"),
		expressions.KindBitwiseOrAgg:          clickhouseRename("BIT_OR"),
		expressions.KindBitwiseXorAgg:         clickhouseRename("BIT_XOR"),
		expressions.KindBitwiseCount:          clickhouseRename("BIT_COUNT"),
		expressions.KindByteLength:            clickhouseRename("BYTE_LENGTH"),
		expressions.KindCast:                  bigQueryCastSQL,
		expressions.KindCollateProperty:       bigQueryCollatePropertySQL,
		expressions.KindCommit:                func(*Generator, expressions.Expression) string { return "COMMIT TRANSACTION" },
		expressions.KindCountIf:               clickhouseRename("COUNTIF"),
		expressions.KindCreate:                bigQueryCreateSQL,
		expressions.KindCTE:                   bigQueryCTESQL,
		expressions.KindDateAdd:               bigQueryDateAddIntervalSQL("DATE", "ADD"),
		expressions.KindDateDiff:              bigQueryDateDiffSQL,
		expressions.KindDateFromParts:         clickhouseRename("DATE"),
		expressions.KindDateStrToDate:         bigQueryDateStrToDateSQL,
		expressions.KindDateSub:               bigQueryDateAddIntervalSQL("DATE", "SUB"),
		expressions.KindDatetimeAdd:           bigQueryDateAddIntervalSQL("DATETIME", "ADD"),
		expressions.KindDatetimeSub:           bigQueryDateAddIntervalSQL("DATETIME", "SUB"),
		expressions.KindDateFromUnixDate:      clickhouseRename("DATE_FROM_UNIX_DATE"),
		expressions.KindFromTimeZone:          bigQueryFromTimeZoneSQL,
		expressions.KindGenerateSeries:        bigQueryGenerateSeriesSQL("GENERATE_ARRAY"),
		expressions.KindGroupConcat:           bigQueryGroupConcatSQL,
		expressions.KindHex:                   bigQueryHexSQL,
		expressions.KindIf:                    bigQueryIfSQL,
		expressions.KindILike:                 bigQueryNoILikeSQL,
		expressions.KindIntDiv:                clickhouseRename("DIV"),
		expressions.KindInt64:                 clickhouseRename("INT64"),
		expressions.KindJSONBool:              clickhouseRename("BOOL"),
		expressions.KindJSONExtract:           bigQueryJSONExtractSQL,
		expressions.KindJSONExtractArray:      bigQueryJSONExtractSQL,
		expressions.KindJSONExtractScalar:     bigQueryJSONExtractSQL,
		expressions.KindJSONFormat:            bigQueryJSONFormatSQL,
		expressions.KindJSONKeysAtDepth:       clickhouseRename("JSON_KEYS"),
		expressions.KindJSONValueArray:        clickhouseRename("JSON_VALUE_ARRAY"),
		expressions.KindLevenshtein:           bigQueryLevenshteinSQL,
		expressions.KindMax:                   bigQueryMaxOrGreatestSQL,
		expressions.KindMD5:                   bigQueryMD5SQL,
		expressions.KindMD5Digest:             clickhouseRename("MD5"),
		expressions.KindMin:                   bigQueryMinOrLeastSQL,
		expressions.KindNormalize:             bigQueryNormalizeSQL,
		expressions.KindPartitionedByProperty: bigQueryPartitionedBySQL,
		expressions.KindParseTime:             bigQueryParseTimeSQL,
		expressions.KindParseDatetime:         bigQueryParseDatetimeSQL,
		expressions.KindRegexpExtract:         bigQueryRegexpExtractSQL,
		expressions.KindRegexpExtractAll:      bigQueryRegexpExtractAllSQL,
		expressions.KindRegexpReplace:         (*Generator).regexpReplaceSQL,
		expressions.KindRegexpLike:            clickhouseRename("REGEXP_CONTAINS"),
		expressions.KindReturnsProperty:       bigQueryReturnsPropertySQL,
		expressions.KindRollback:              func(*Generator, expressions.Expression) string { return "ROLLBACK TRANSACTION" },
		expressions.KindSHA:                   clickhouseRename("SHA1"),
		expressions.KindSHA2:                  bigQuerySHA2SQL,
		expressions.KindSHA1Digest:            clickhouseRename("SHA1"),
		expressions.KindSHA2Digest:            bigQuerySHA2SQL,
		expressions.KindStabilityProperty:     bigQueryStabilityPropertySQL,
		expressions.KindString:                clickhouseRename("STRING"),
		expressions.KindStrPosition:           (*Generator).strPositionSQL,
		expressions.KindStrToDate:             bigQueryStrToDatetimeSQL,
		expressions.KindStrToTime:             bigQueryStrToDatetimeSQL,
		expressions.KindSessionUser:           func(*Generator, expressions.Expression) string { return "SESSION_USER()" },
		expressions.KindTimeAdd:               bigQueryDateAddIntervalSQL("TIME", "ADD"),
		expressions.KindTimeFromParts:         clickhouseRename("TIME"),
		expressions.KindTimestampFromParts:    clickhouseRename("DATETIME"),
		expressions.KindTimeSub:               bigQueryDateAddIntervalSQL("TIME", "SUB"),
		expressions.KindTimestampAdd:          bigQueryDateAddIntervalSQL("TIMESTAMP", "ADD"),
		expressions.KindTimestampDiff:         clickhouseRename("TIMESTAMP_DIFF"),
		expressions.KindTimestampSub:          bigQueryDateAddIntervalSQL("TIMESTAMP", "SUB"),
		expressions.KindTimeStrToTime:         bigQueryTimeStrToTimeSQL,
		expressions.KindTransaction:           func(*Generator, expressions.Expression) string { return "BEGIN TRANSACTION" },
		expressions.KindTsOrDsAdd:             bigQueryTsOrDsAddSQL,
		expressions.KindTsOrDsDiff:            bigQueryTsOrDsDiffSQL,
		expressions.KindTsOrDsToTime:          clickhouseRename("TIME"),
		expressions.KindTsOrDsToDatetime:      clickhouseRename("DATETIME"),
		expressions.KindTsOrDsToTimestamp:     clickhouseRename("TIMESTAMP"),
		expressions.KindUnhex:                 clickhouseRename("FROM_HEX"),
		expressions.KindUnixDate:              clickhouseRename("UNIX_DATE"),
		expressions.KindUnixToTime:            bigQueryUnixToTimeSQL,
		expressions.KindUuid:                  func(*Generator, expressions.Expression) string { return "GENERATE_UUID()" },
		expressions.KindValues:                bigQueryValuesSQL,
		expressions.KindVariancePop:           clickhouseRename("VAR_POP"),
		expressions.KindSafeDivide:            clickhouseRename("SAFE_DIVIDE"),
		// Method overrides (generators/bigquery.py:583-733).
		expressions.KindSelect:     bigQuerySelectSQL,
		expressions.KindWeekStart:  bigQueryWeekStartSQL,
		expressions.KindDateTrunc:  bigQueryDateTruncSQL,
		expressions.KindMod:        (*Generator).bigQueryModSQL,
		expressions.KindTimeToStr:  bigQueryTimeToStrSQL,
		expressions.KindEQ:         (*Generator).bigQueryEQSQL,
		expressions.KindAtTimeZone: (*Generator).bigQueryAtTimeZoneSQL,
		expressions.KindTryCast:    bigQueryTryCastSQL,
		expressions.KindBracket:    (*Generator).bigQueryBracketSQL,

		expressions.KindVersion:         (*Generator).bigQueryVersionSQL,
		expressions.KindContains:        bigQueryContainsSQL,
		expressions.KindClusterProperty: bigQueryClusterPropertySQL,
	})
}

// bigQueryArgMaxMin ports arg_max_or_min_no_count (dialects/dialect.py:1976-1981): the count
// argument is unsupported, so only this/expression render.
func bigQueryArgMaxMin(name string) func(*Generator, expressions.Expression) string {
	return func(g *Generator, e expressions.Expression) string {
		if e.Arg("count") != nil {
			g.unsupported(name + ": count is not supported")
		}
		return g.funcCall(name, []any{e.This(), e.Arg("expression")}, "(", ")", true)
	}
}

// bigQueryArraySQL ports inline_array_unless_query (dialects/dialect.py:1262-1266).
func bigQueryArraySQL(g *Generator, e expressions.Expression) string {
	exprs := e.Expressions()
	if len(exprs) > 0 && exprs[0].Find(expressions.TraitQuery) != nil {
		return g.funcCall("ARRAY", []any{exprs[0]}, "(", ")", true)
	}
	return g.inlineArraySQL(e)
}

// bigQueryArrayContainsSQL ports _array_contains_sql (generators/bigquery.py:168-175).
func bigQueryArrayContainsSQL(g *Generator, e expressions.Expression) string {
	unnest := expressions.Unnest(expressions.Args{
		"expressions": []expressions.Expression{asExpression(e.This())},
		"alias": expressions.TableAlias(expressions.Args{
			"this":    expressions.ToIdentifier("_unnest"),
			"columns": []expressions.Expression{expressions.ToIdentifier("_col")},
		}),
	})
	cond := expressions.EQ(expressions.Args{
		"this":       expressions.Column(expressions.Args{"this": expressions.ToIdentifier("_col")}),
		"expression": e.Arg("expression"),
	})
	query := expressions.Select(expressions.Args{
		"expressions": []expressions.Expression{expressions.LiteralNumber("1")},
		"from_":       expressions.From(expressions.Args{"this": unnest}),
		"where":       expressions.Where(expressions.Args{"this": cond}),
	})
	return g.gen(expressions.Exists(expressions.Args{"this": query}))
}

// bigQueryFilterArrayUsingUnnest ports filter_array_using_unnest (dialects/dialect.py:
// 2287-2305).
func bigQueryFilterArrayUsingUnnest(g *Generator, e expressions.Expression) string {
	cond := asExpression(e.Arg("expression"))
	var alias expressions.Expression
	if cond != nil && cond.Kind() == expressions.KindLambda && len(cond.Expressions()) == 1 {
		alias = cond.Expressions()[0]
		cond = cond.This()
	} else if cond != nil && cond.Is(expressions.TraitPredicate) {
		alias = expressions.ToIdentifier("_u")
	} else if e.Kind() == expressions.KindArrayRemove {
		alias = expressions.ToIdentifier("_u")
		cond = expressions.NEQ(expressions.Args{
			"this":       alias,
			"expression": e.Arg("expression"),
		})
	} else {
		g.unsupported("Unsupported filter condition")
		return ""
	}

	unnest := expressions.Unnest(expressions.Args{
		"expressions": []expressions.Expression{e.This()},
		"alias": expressions.TableAlias(expressions.Args{
			"columns": []expressions.Expression{alias},
		}),
	})
	filtered := expressions.Select(expressions.Args{
		"expressions": []expressions.Expression{alias},
		"from_":       expressions.From(expressions.Args{"this": unnest}),
		"where":       expressions.Where(expressions.Args{"this": cond}),
	})
	return g.gen(expressions.Array(expressions.Args{"expressions": []expressions.Expression{filtered}}))
}

// bigQueryCastSQL ports the Cast transform (generators/bigquery.py:320) and cast_sql
// (generators/bigquery.py:716-727): precision parameterized text types are dropped, and an
// inline ARRAY literal renders its type before the brackets.
func bigQueryCastSQL(g *Generator, e expressions.Expression) string {
	// generators/bigquery.py:320 remove_precision_parameterized_types: precision is only
	// allowed in DDL, so it is stripped from every DataType under the Cast.
	for _, dataType := range e.FindAll(expressions.KindDataType) {
		params := dataType.Expressions()
		filtered := make([]expressions.Expression, 0, len(params))
		for _, param := range params {
			if param.Kind() != expressions.KindDataTypeParam {
				filtered = append(filtered, param)
			}
		}
		dataType.Set("expressions", filtered)
	}

	if to := asExpression(e.Arg("to")); to != nil && to.Kind() == expressions.KindDataType &&
		expIsTextType(to) && len(to.Expressions()) > 0 {
		e.Set("to", expressions.DataType(expressions.Args{"this": to.Arg("this")}))
	}

	if this := asExpression(e.Arg("this")); this != nil && this.Kind() == expressions.KindArray {
		exprs := this.Expressions()
		if len(exprs) == 0 || exprs[0].Find(expressions.TraitQuery) == nil {
			return g.sqlKey(e, "to") + g.gen(this)
		}
	}
	return g.castSQLWithPrefix(e, "")
}

// bigQueryTryCastSQL ports trycast_sql (generators/bigquery.py:665-666).
func bigQueryTryCastSQL(g *Generator, e expressions.Expression) string {
	return g.castSQLWithPrefix(e, "SAFE_")
}

func expIsTextType(to expressions.Expression) bool {
	if dt, ok := to.Arg("this").(expressions.DType); ok {
		return dt == expressions.DTypeText || dt == expressions.DTypeVarchar || dt == expressions.DTypeChar
	}
	return false
}

// bigQueryCollatePropertySQL ports the CollateProperty transform (generators/bigquery.py:
// 321-325).
func bigQueryCollatePropertySQL(g *Generator, e expressions.Expression) string {
	if boolValue(e.Arg("default")) {
		return "DEFAULT COLLATE " + g.sqlKey(e, "this")
	}
	return "COLLATE " + g.sqlKey(e, "this")
}

// bigQueryCreateSQL ports _create_sql (generators/bigquery.py:73-81).
func bigQueryCreateSQL(g *Generator, e expressions.Expression) string {
	if e.Text("kind") == "FUNCTION" {
		for _, returns := range e.FindAll(expressions.KindReturnsProperty) {
			if truthy(returns.Arg("is_table")) {
				e.Set("kind", "TABLE FUNCTION")
				if body := asExpression(e.Arg("expression")); body != nil &&
					(body.Kind() == expressions.KindSubquery || body.Kind() == expressions.KindLiteral) {
					e.Set("expression", body.This())
				}
				break
			}
		}
	}
	return g.createSQL(e)
}

// bigQueryCTESQL ports the CTE preprocess (generators/bigquery.py:329) and
// _pushdown_cte_column_names (generators/bigquery.py:113-137): BigQuery forbids column names
// on a CTE, so they are pushed into the inner projection aliases.
func bigQueryCTESQL(g *Generator, e expressions.Expression) string {
	alias := asExpression(e.Arg("alias"))
	query := asExpression(e.Arg("this"))
	if alias != nil && query != nil {
		names := exprList(alias.Arg("columns"))
		selects := query.Selects()
		if len(names) > 0 && len(selects) > 0 {
			alias.Set("columns", nil)
			for i, proj := range selects {
				if i >= len(names) {
					break
				}
				inner := proj
				if proj.Kind() == expressions.KindAlias {
					inner = asExpression(proj.This())
				}
				proj.Replace(expressions.AliasExpr(inner, names[i].Name(), false))
			}
		}
	}
	return g.cteSQL(e)
}

// bigQueryDateAddIntervalSQL ports date_add_interval_sql (dialects/dialect.py:1691-1697).
func bigQueryDateAddIntervalSQL(dataType, kind string) func(*Generator, expressions.Expression) string {
	return func(g *Generator, e expressions.Expression) string {
		interval := expressions.Interval(expressions.Args{
			"this": e.Arg("expression"),
			"unit": unitToVarValue(asExpression(e.Arg("unit"))),
		})
		return dataType + "_" + kind + "(" + g.sqlKey(e, "this") + ", " + g.gen(interval) + ")"
	}
}

// bigQueryDateDiffSQL ports the DateDiff transform (generators/bigquery.py:331).
func bigQueryDateDiffSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("DATE_DIFF", []any{e.This(), e.Arg("expression"), unitToVarValue(asExpression(e.Arg("unit")))}, "(", ")", true)
}

// bigQueryDateStrToDateSQL ports datestrtodate_sql (dialects/dialect.py:1787-1788).
func bigQueryDateStrToDateSQL(g *Generator, e expressions.Expression) string {
	return g.gen(expressions.Cast(expressions.Args{
		"this": e.This(),
		"to":   expressions.DTypeDate.IntoExpr(nil),
	}))
}

// bigQueryFromTimeZoneSQL ports the FromTimeZone transform (generators/bigquery.py:338-340).
func bigQueryFromTimeZoneSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("DATETIME", []any{
		g.funcCall("TIMESTAMP", []any{e.This(), e.Arg("zone")}, "(", ")", true),
		"'UTC'",
	}, "(", ")", true)
}

// bigQueryGenerateSeriesSQL ports generate_series_sql (dialects/dialect.py:1420-1436).
func bigQueryGenerateSeriesSQL(name string) func(*Generator, expressions.Expression) string {
	return func(g *Generator, e expressions.Expression) string {
		start := e.Arg("start")
		end := e.Arg("end")
		step := e.Arg("step")
		if truthy(e.Arg("is_end_exclusive")) {
			end = expressions.Sub(expressions.Args{"this": end, "expression": expressions.LiteralNumber("1")})
		}
		return g.funcCall(name, []any{start, end, step}, "(", ")", true)
	}
}

// bigQueryGroupConcatSQL ports the GroupConcat transform (generators/bigquery.py:342-344):
// STRING_AGG(this, separator) with no default separator and no WITHIN GROUP.
// bigQueryGroupConcatSQL ports groupconcat_sql (dialects/dialect.py:2539-2583) with BigQuery's
// lambda arguments: func_name="STRING_AGG", within_group=False, sep=None.
func bigQueryGroupConcatSQL(g *Generator, e expressions.Expression) string {
	this := asExpression(e.This())
	separator := asExpression(e.Arg("separator"))

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
	if separator != nil {
		if argsSQL != "" {
			argsSQL += ", "
		}
		argsSQL += g.gen(separator)
	}

	modifiers := ""
	if limit != nil {
		modifiers = g.gen(limit)
	}
	if order != nil {
		modifiers = g.gen(order) + modifiers
	}
	return "STRING_AGG(" + argsSQL + modifiers + ")"
}

// bigQueryHexSQL ports hex_sql (generator.py:2020-2025).
func bigQueryHexSQL(g *Generator, e expressions.Expression) string {
	text := g.funcCall(hexFuncName(g), []any{e.Arg("this")}, "(", ")", true)
	if g.dialect.HexLowercase {
		text = g.funcCall("LOWER", []any{text}, "(", ")", true)
	}
	return text
}

func hexFuncName(g *Generator) string {
	if g.dialect.HexFunc != "" {
		return g.dialect.HexFunc
	}
	return "HEX"
}

// bigQueryIfSQL ports the If transform (generators/bigquery.py:347): IF(cond, true, NULL).
func bigQueryIfSQL(g *Generator, e expressions.Expression) string {
	falseValue := e.Arg("false")
	if falseValue == nil {
		falseValue = expressions.Null()
	}
	return g.funcCall("IF", []any{e.This(), e.Arg("true"), falseValue}, "(", ")", true)
}

// bigQueryNoILikeSQL ports no_ilike_sql (dialects/dialect.py:1269-1276).
func bigQueryNoILikeSQL(g *Generator, e expressions.Expression) string {
	like := expressions.Like(expressions.Args{
		"this":       expressions.Lower(expressions.Args{"this": e.This()}),
		"expression": expressions.Lower(expressions.Args{"this": e.Arg("expression")}),
		"negate":     e.Arg("negate"),
	})
	return g.likeSQL(like)
}

// bigQueryJSONExtractSQL ports _json_extract_sql (generators/bigquery.py:231-245): the parsed
// spelling is used when known, else the class name.
func bigQueryJSONExtractSQL(g *Generator, e expressions.Expression) string {
	name, _ := e.MetaGet("name").(string)
	if name == "" {
		name = g.sqlName(e.Kind())
	}
	return g.funcCall(strings.ToUpper(name), g.fallbackArgs(e), "(", ")", true)
}

// bigQueryJSONFormatSQL ports the JSONFormat transform (generators/bigquery.py:355-359).
func bigQueryJSONFormatSQL(g *Generator, e expressions.Expression) string {
	name := "TO_JSON_STRING"
	if truthy(e.Arg("to_json")) {
		name = "TO_JSON"
	}
	return g.funcCall(name, []any{e.This(), e.Arg("options")}, "(", ")", true)
}

// bigQueryLevenshteinSQL ports _levenshtein_sql (generators/bigquery.py:222-228).
func bigQueryLevenshteinSQL(g *Generator, e expressions.Expression) string {
	var maxDist any
	if md := asExpression(e.Arg("max_dist")); md != nil {
		maxDist = expressions.Kwarg(expressions.Args{
			"this":       expressions.Var(expressions.Args{"this": "max_distance"}),
			"expression": md,
		})
	}
	return g.funcCall("EDIT_DISTANCE", []any{e.This(), e.Arg("expression"), maxDist}, "(", ")", true)
}

// bigQueryMaxOrGreatestSQL ports max_or_greatest (dialects/dialect.py:1807-1809).
func bigQueryMaxOrGreatestSQL(g *Generator, e expressions.Expression) string {
	name := "MAX"
	if len(e.Expressions()) > 0 {
		name = "GREATEST"
	}
	return g.funcCall(name, g.fallbackArgs(e), "(", ")", true)
}

// bigQueryMinOrLeastSQL ports min_or_least (dialects/dialect.py:1802-1804).
func bigQueryMinOrLeastSQL(g *Generator, e expressions.Expression) string {
	name := "MIN"
	if len(e.Expressions()) > 0 {
		name = "LEAST"
	}
	return g.funcCall(name, g.fallbackArgs(e), "(", ")", true)
}

// bigQueryMD5SQL ports the MD5 transform (generators/bigquery.py:364): TO_HEX(MD5(x)).
func bigQueryMD5SQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("TO_HEX", []any{g.funcCall("MD5", []any{e.This()}, "(", ")", true)}, "(", ")", true)
}

// bigQueryNormalizeSQL ports the Normalize transform (generators/bigquery.py:367-371).
func bigQueryNormalizeSQL(g *Generator, e expressions.Expression) string {
	name := "NORMALIZE"
	if truthy(e.Arg("is_casefold")) {
		name = "NORMALIZE_AND_CASEFOLD"
	}
	return g.funcCall(name, []any{e.This(), e.Arg("form")}, "(", ")", true)
}

// bigQueryPartitionedBySQL ports the PartitionedByProperty transform (generators/bigquery.py:
// 372).
func bigQueryPartitionedBySQL(g *Generator, e expressions.Expression) string {
	return "PARTITION BY " + g.sqlKey(e, "this")
}

// bigQueryParseTimeSQL ports the ParseTime transform (generators/bigquery.py:385).
func bigQueryParseTimeSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("PARSE_TIME", []any{g.bigQueryFormatTime(e, false), e.This()}, "(", ")", true)
}

// bigQueryParseDatetimeSQL ports the ParseDatetime transform (generators/bigquery.py:386).
func bigQueryParseDatetimeSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("PARSE_DATETIME", []any{g.bigQueryFormatTime(e, false), e.This()}, "(", ")", true)
}

// bigQueryRegexpExtractSQL ports the RegexpExtract transform (generators/bigquery.py:373-379).
func bigQueryRegexpExtractSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("REGEXP_EXTRACT", []any{
		e.This(), e.Arg("expression"), e.Arg("position"), e.Arg("occurrence"),
	}, "(", ")", true)
}

func bigQueryRegexpExtractAllSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("REGEXP_EXTRACT_ALL", []any{e.This(), e.Arg("expression")}, "(", ")", true)
}

// bigQueryReturnsPropertySQL ports _returnsproperty_sql (generators/bigquery.py:64-70).
func bigQueryReturnsPropertySQL(g *Generator, e expressions.Expression) string {
	this := asExpression(e.Arg("this"))
	if this != nil && this.Kind() == expressions.KindSchema {
		return "RETURNS " + g.sqlKey(this, "this") + " <" + g.expressions(exprsOptions{expression: this}) + ">"
	}
	return "RETURNS " + g.gen(this)
}

// bigQuerySHA2SQL ports sha256_sql / sha2_digest_sql (dialects/dialect.py:2388-2393).
func bigQuerySHA2SQL(g *Generator, e expressions.Expression) string {
	length := e.Text("length")
	if length == "" {
		length = "256"
	}
	return g.funcCall("SHA"+length, []any{e.This()}, "(", ")", true)
}

// bigQueryStabilityPropertySQL ports the StabilityProperty transform (generators/bigquery.py:
// 401-403).
func bigQueryStabilityPropertySQL(_ *Generator, e expressions.Expression) string {
	if e.Name() == "IMMUTABLE" {
		return "DETERMINISTIC"
	}
	return "NOT DETERMINISTIC"
}

// bigQueryStrToDatetimeSQL ports _str_to_datetime_sql (generators/bigquery.py:206-219).
func bigQueryStrToDatetimeSQL(g *Generator, e expressions.Expression) string {
	dtype := "TIMESTAMP"
	if e.Kind() == expressions.KindStrToDate {
		dtype = "DATE"
	}
	if truthy(e.Arg("safe")) {
		return "SAFE_CAST(" + g.sqlKey(e, "this") + " AS " + dtype + " FORMAT " + g.bigQueryFormatTime(e, true) + ")"
	}
	return g.funcCall("PARSE_"+dtype, []any{g.bigQueryFormatTime(e, false), e.This(), e.Arg("zone")}, "(", ")", true)
}

// bigQueryTimeStrToTimeSQL ports timestrtotime_sql (dialects/dialect.py:1769-1784).
func bigQueryTimeStrToTimeSQL(g *Generator, e expressions.Expression) string {
	dtype := expressions.DTypeTimestamp
	if truthy(e.Arg("zone")) {
		dtype = expressions.DTypeTimestampTz
	}
	return g.gen(expressions.Cast(expressions.Args{"this": e.This(), "to": dtype.IntoExpr(nil)}))
}

// bigQueryTsOrDsAddSQL ports _ts_or_ds_add_sql (generators/bigquery.py:178-179) with
// ts_or_ds_add_cast (dialects/dialect.py:1984-1994).
func bigQueryTsOrDsAddSQL(g *Generator, e expressions.Expression) string {
	returnType := asExpression(e.Arg("return_type"))
	if returnType == nil {
		returnType = expressions.DTypeDate.IntoExpr(nil)
	}
	if this := asExpression(e.This()); this != nil {
		source := this.Copy()
		if expIsType(returnType, expressions.DTypeDate) {
			source = expressions.Cast(expressions.Args{"this": source, "to": expressions.DTypeTimestamp.IntoExpr(nil)})
		}
		this.Replace(expressions.Cast(expressions.Args{"this": source, "to": returnType}))
	}
	interval := expressions.Interval(expressions.Args{
		"this": e.Arg("expression"),
		"unit": unitToVarValue(asExpression(e.Arg("unit"))),
	})
	return "DATE_ADD(" + g.sqlKey(e, "this") + ", " + g.gen(interval) + ")"
}

// bigQueryTsOrDsDiffSQL ports _ts_or_ds_diff_sql (generators/bigquery.py:182-186).
func bigQueryTsOrDsDiffSQL(g *Generator, e expressions.Expression) string {
	if this := asExpression(e.This()); this != nil {
		this.Replace(expressions.Cast(expressions.Args{"this": this.Copy(), "to": expressions.DTypeTimestamp.IntoExpr(nil)}))
	}
	if expr := asExpression(e.Arg("expression")); expr != nil {
		expr.Replace(expressions.Cast(expressions.Args{"this": expr.Copy(), "to": expressions.DTypeTimestamp.IntoExpr(nil)}))
	}
	return g.funcCall("DATE_DIFF", []any{e.This(), e.Arg("expression"), unitToVarValue(asExpression(e.Arg("unit")))}, "(", ")", true)
}

func expIsType(dtype expressions.Expression, want expressions.DType) bool {
	if dtype == nil {
		return false
	}
	if dt, ok := dtype.Arg("this").(expressions.DType); ok {
		return dt == want
	}
	return false
}

// bigQueryUnixToTimeSQL ports _unix_to_time_sql (generators/bigquery.py:189-203).
func bigQueryUnixToTimeSQL(g *Generator, e expressions.Expression) string {
	scale := asExpression(e.Arg("scale"))
	timestamp := e.This()
	if scale == nil || (scale.IsInt() && scale.Name() == "0") {
		return g.funcCall("TIMESTAMP_SECONDS", []any{timestamp}, "(", ")", true)
	}
	if scale.IsInt() {
		switch scale.Name() {
		case "3":
			return g.funcCall("TIMESTAMP_MILLIS", []any{timestamp}, "(", ")", true)
		case "6":
			return g.funcCall("TIMESTAMP_MICROS", []any{timestamp}, "(", ")", true)
		}
	}
	divided := expressions.Cast(expressions.Args{
		"this": expressions.Div(expressions.Args{
			"this":       timestamp,
			"expression": expressions.Pow(expressions.Args{"this": expressions.LiteralNumber("10"), "expression": scale}),
		}),
		"to": expressions.DTypeBigInt.IntoExpr(nil),
	})
	return g.funcCall("TIMESTAMP_SECONDS", []any{divided}, "(", ")", true)
}

// bigQueryValuesSQL ports _derived_table_values_to_unnest (generators/bigquery.py:39-61).
func bigQueryValuesSQL(g *Generator, e expressions.Expression) string {
	if e.FindAncestor(expressions.KindFrom, expressions.KindJoin) == nil {
		return g.valuesSQL(e)
	}

	alias := asExpression(e.Arg("alias"))
	aliasColumns := []expressions.Expression{}
	if alias != nil {
		aliasColumns = exprList(alias.Arg("columns"))
	}

	structs := []expressions.Expression{}
	for _, tuple := range e.FindAll(expressions.KindTuple) {
		fieldExpressions := tuple.Expressions()
		properties := make([]expressions.Expression, 0, len(fieldExpressions))
		for i, field := range fieldExpressions {
			name := "_c" + strconv.Itoa(i)
			if i < len(aliasColumns) {
				name = aliasColumns[i].Name()
			}
			properties = append(properties, expressions.PropertyEQ(expressions.Args{
				"this":       expressions.ToIdentifier(name),
				"expression": field,
			}))
		}
		structs = append(structs, expressions.Struct(expressions.Args{"expressions": properties}))
	}

	var aliasNameOnly expressions.Expression
	if alias != nil {
		aliasNameOnly = expressions.TableAlias(expressions.Args{
			"columns": []expressions.Expression{asExpression(alias.Arg("this"))},
		})
	}
	unnest := expressions.Unnest(expressions.Args{
		"expressions": []expressions.Expression{expressions.Array(expressions.Args{"expressions": structs})},
		"alias":       aliasNameOnly,
	})
	return g.unnestSQL(unnest)
}

// bigQueryWeekStartSQL ports weekstart_sql (generators/bigquery.py:583-588).
func bigQueryWeekStartSQL(g *Generator, e expressions.Expression) string {
	this := asExpression(e.This())
	if this != nil && strings.ToUpper(this.Name()) == "SUNDAY" {
		return "WEEK"
	}
	return g.funcCall("WEEK", []any{e.This()}, "(", ")", true)
}

// bigQueryDateTruncSQL ports datetrunc_sql (generators/bigquery.py:590-593).
func bigQueryDateTruncSQL(g *Generator, e expressions.Expression) string {
	unit := asExpression(e.Arg("unit"))
	unitSQL := ""
	if unit != nil {
		if unit.IsString() {
			unitSQL = unit.Name()
		} else {
			unitSQL = g.gen(unit)
		}
	}
	return g.funcCall("DATE_TRUNC", []any{e.This(), unitSQL, e.Arg("zone")}, "(", ")", true)
}

// bigQueryModSQL ports mod_sql (generators/bigquery.py:595-602): MOD with unparenthesized args.
func (g *Generator) bigQueryModSQL(e expressions.Expression) string {
	unnest := func(node expressions.Expression) any {
		if node != nil && node.Kind() == expressions.KindParen {
			return node.This()
		}
		return node
	}
	return g.funcCall("MOD", []any{unnest(asExpression(e.This())), unnest(asExpression(e.Arg("expression")))}, "(", ")", true)
}

// bigQueryTimeToStrSQL ports timetostr_sql (generators/bigquery.py:629-643).
func bigQueryTimeToStrSQL(g *Generator, e expressions.Expression) string {
	this := asExpression(e.This())
	funcName := "FORMAT_DATE"
	if this != nil {
		switch this.Kind() {
		case expressions.KindTsOrDsToDatetime:
			funcName = "FORMAT_DATETIME"
		case expressions.KindTsOrDsToTimestamp:
			funcName = "FORMAT_TIMESTAMP"
		case expressions.KindTsOrDsToTime:
			funcName = "FORMAT_TIME"
		}
	}
	timeExpr := e.This()
	switch {
	case this != nil && (this.Kind() == expressions.KindTsOrDsToDatetime ||
		this.Kind() == expressions.KindTsOrDsToTimestamp ||
		this.Kind() == expressions.KindTsOrDsToTime ||
		this.Kind() == expressions.KindTsOrDsToDate):
		timeExpr = this.This()
	}
	return g.funcCall(funcName, []any{g.bigQueryFormatTime(e, false), timeExpr, e.Arg("zone")}, "(", ")", true)
}

// bigQueryEQSQL ports eq_sql (generators/bigquery.py:645-651): comparing with NULL yields NULL.
func (g *Generator) bigQueryEQSQL(e expressions.Expression) string {
	isNull := func(node expressions.Expression) bool {
		return node != nil && node.Kind() == expressions.KindNull
	}
	if (isNull(asExpression(e.This())) || isNull(asExpression(e.Arg("expression")))) &&
		(e.Parent() == nil || e.Parent().Kind() != expressions.KindUpdate) {
		return "NULL"
	}
	return g.eqSQL(e)
}

// bigQueryAtTimeZoneSQL ports attimezone_sql (generators/bigquery.py:653-663).
func (g *Generator) bigQueryAtTimeZoneSQL(e expressions.Expression) string {
	parent := e.Parent()
	if parent == nil || parent.Kind() != expressions.KindCast || !expIsTextType(asExpression(parent.Arg("to"))) {
		return g.funcCall("TIMESTAMP", []any{
			g.funcCall("DATETIME", []any{e.This(), e.Arg("zone")}, "(", ")", true),
		}, "(", ")", true)
	}
	return g.atTimeZoneSQL(e)
}

// bigQueryBracketSQL ports bracket_sql (generators/bigquery.py:668-696).
func (g *Generator) bigQueryBracketSQL(e expressions.Expression) string {
	this := asExpression(e.This())
	exprs := e.Expressions()

	if len(exprs) == 1 && this != nil && this.IsType(expressions.DTypeStruct) {
		arg := exprs[0]
		if argType := arg.Type(); argType != nil &&
			argType.IsType(expressions.DTypeText, expressions.DTypeVarchar, expressions.DTypeChar) {
			return g.gen(this) + "." + arg.Name()
		}
	}

	exprsSQL := g.expressions(exprsOptions{expression: e, flat: true})
	if offset, ok := e.Arg("offset").(int); ok {
		switch offset {
		case 0:
			exprsSQL = "OFFSET(" + exprsSQL + ")"
		case 1:
			exprsSQL = "ORDINAL(" + exprsSQL + ")"
		default:
			g.unsupported("Unsupported array offset")
		}
	}
	if truthy(e.Arg("safe")) {
		exprsSQL = "SAFE_" + exprsSQL
	}
	return g.gen(this) + "[" + exprsSQL + "]"
}

// bigQueryVersionSQL ports version_sql (generators/bigquery.py:701-704).
func (g *Generator) bigQueryVersionSQL(e expressions.Expression) string {
	if e.Name() == "TIMESTAMP" {
		e.Set("this", "SYSTEM_TIME")
	}
	return g.versionSQL(e)
}

// bigQueryContainsSQL ports contains_sql (generators/bigquery.py:706-714).
func bigQueryContainsSQL(g *Generator, e expressions.Expression) string {
	this := asExpression(e.This())
	expr := asExpression(e.Arg("expression"))
	if this != nil && this.Kind() == expressions.KindLower && expr != nil && expr.Kind() == expressions.KindLower {
		this = this.This()
		expr = expr.This()
	}
	return g.funcCall("CONTAINS_SUBSTR", []any{this, expr, e.Arg("json_scope")}, "(", ")", true)
}

// bigQueryClusterPropertySQL ports clusterproperty_sql (generators/bigquery.py:729-733).
func bigQueryClusterPropertySQL(g *Generator, e expressions.Expression) string {
	if truthy(e.Arg("this")) {
		g.unsupported("Unsupported CLUSTER BY")
		return ""
	}
	return g.opExpressions("CLUSTER BY", e)
}

// bigQueryFormatTime ports Generator.format_time for BigQuery: the forward TIME_MAPPING, or
// the INVERSE_FORMAT_MAPPING for the SAFE_CAST FORMAT branch.
func (g *Generator) bigQueryFormatTime(e expressions.Expression, inverse bool) string {
	format := asExpression(e.Arg("format"))
	if format == nil || !format.IsString() {
		return g.gen(format)
	}
	mapping := g.dialect.InverseTimeMapping
	if inverse {
		mapping = g.dialect.InverseFormatMapping
	}
	if converted, ok := dialects.FormatTime(format.Name(), mapping); ok {
		return g.gen(expressions.LiteralString(converted))
	}
	return g.gen(format)
}

// exprList narrows an arg value (slice or single) to its non-nil Expressions.
func exprList(value any) []expressions.Expression {
	items := listFromValue(value)
	out := make([]expressions.Expression, 0, len(items))
	for _, item := range items {
		if expr := asExpression(item); expr != nil {
			out = append(out, expr)
		}
	}
	return out
}

// bigQuerySelectSQL ports the Select preprocess pipeline (generators/bigquery.py:387-396)
// for the transforms this port has: _alias_ordered_group rewrites a GROUP BY expression that
// exactly matches a projection alias into a reference to that alias.
func bigQuerySelectSQL(g *Generator, e expressions.Expression) string {
	group := asExpression(e.Arg("group"))
	order := asExpression(e.Arg("order"))
	if group != nil && order != nil {
		aliases := map[string]expressions.Expression{}
		for _, selectExpr := range e.Selects() {
			if selectExpr.Kind() != expressions.KindAlias {
				continue
			}
			if alias := asExpression(selectExpr.Arg("alias")); alias != nil {
				aliases[selectExpr.This().ToS()] = alias
			}
		}
		for _, grouped := range group.Expressions() {
			if grouped.IsInt() {
				continue
			}
			if alias, ok := aliases[grouped.ToS()]; ok {
				grouped.Replace(expressions.Column(expressions.Args{"this": alias.Copy()}))
			}
		}
	}
	return g.selectSQL(e)
}
