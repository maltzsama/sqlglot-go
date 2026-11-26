package generator

import (
	"slices"
	"strconv"
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
		expressions.KindSetProperty:           propertyLocationUnsupported,
		expressions.KindVolatileProperty:      propertyLocationUnsupported,
		expressions.KindRowAccessProperty:     propertyLocationPostSchema,
		expressions.KindExternalProperty:      propertyLocationPostCreate,
		expressions.KindPartitionedByProperty: propertyLocationPostSchema,
		expressions.KindLocationProperty:      propertyLocationPostWith,
	})

	handlers := map[expressions.Kind]func(*Generator, expressions.Expression) string{
		expressions.KindAlterSet: func(g *Generator, e expressions.Expression) string {
			out := "SET"
			if values := g.expressions(exprsOptions{expression: e, flat: true}); values != "" {
				out += " " + values
			}
			for _, option := range []struct{ key, name, sep string }{
				{"file_format", "STAGE_FILE_FORMAT", " "},
				{"copy_options", "STAGE_COPY_OPTIONS", " "},
				{"tag", "TAG", ", "},
			} {
				if values := g.expressions(exprsOptions{expression: e, key: option.key, flat: true, sep: option.sep}); values != "" {
					if option.key == "tag" {
						out += " TAG " + values
					} else {
						out += " " + option.name + " = (" + values + ")"
					}
				}
			}
			return out
		},
		expressions.KindFileFormatProperty: func(g *Generator, e expressions.Expression) string {
			return "FILE_FORMAT=(" + g.expressions(exprsOptions{expression: e, flat: true, sep: " "}) + ")"
		},
		// generators/snowflake.py:483: `HANDLER = <expr>`.
		expressions.KindHandlerProperty: func(g *Generator, e expressions.Expression) string {
			return "HANDLER = " + g.sqlKey(e, "this")
		},
		// generators/snowflake.py:513/537: LOCATION=<expr> and PARTITION BY <expr>, plus
		// the bare EXTERNAL marker of CREATE EXTERNAL TABLE.
		expressions.KindLocationProperty: func(g *Generator, e expressions.Expression) string {
			return "LOCATION=" + g.sqlKey(e, "this")
		},
		expressions.KindPartitionedByProperty: func(g *Generator, e expressions.Expression) string {
			return "PARTITION BY " + g.sqlKey(e, "this")
		},
		expressions.KindExternalProperty: func(*Generator, expressions.Expression) string {
			return "EXTERNAL"
		},
		expressions.KindApproxDistinct:        clickhouseRename("APPROX_COUNT_DISTINCT"),
		expressions.KindApproximateSimilarity: clickhouseRename("APPROXIMATE_SIMILARITY"),
		expressions.KindArgMax:                clickhouseRename("MAX_BY"),
		expressions.KindArgMin:                clickhouseRename("MIN_BY"),
		expressions.KindArray:                 snowflakeArraySQL,
		// generators/snowflake.py:413 exp.ArrayConcat: array_concat_sql("ARRAY_CAT").
		expressions.KindArrayConcat:       arrayConcatSQL("ARRAY_CAT"),
		expressions.KindArrayAppend:       snowflakeArrayAppendSQL("ARRAY_APPEND", false),
		expressions.KindArrayPrepend:      snowflakeArrayAppendSQL("ARRAY_PREPEND", false),
		expressions.KindArrayContains:     snowflakeArrayContainsSQL,
		expressions.KindArrayPosition:     snowflakeArrayPositionSQL,
		expressions.KindArrayIntersect:    clickhouseRename("ARRAY_INTERSECTION"),
		expressions.KindArrayOverlaps:     clickhouseRename("ARRAYS_OVERLAP"),
		expressions.KindBitwiseAnd:        clickhouseRename("BITAND"),
		expressions.KindBitwiseOr:         clickhouseRename("BITOR"),
		expressions.KindBitwiseXor:        clickhouseRename("BITXOR"),
		expressions.KindBitwiseNot:        clickhouseRename("BITNOT"),
		expressions.KindBitwiseAndAgg:     clickhouseRename("BITANDAGG"),
		expressions.KindBitwiseOrAgg:      clickhouseRename("BITORAGG"),
		expressions.KindBitwiseXorAgg:     clickhouseRename("BITXORAGG"),
		expressions.KindBitwiseLeftShift:  clickhouseRename("BITSHIFTLEFT"),
		expressions.KindBitwiseRightShift: clickhouseRename("BITSHIFTRIGHT"),
		expressions.KindDayOfMonth:        clickhouseRename("DAYOFMONTH"),
		expressions.KindDayOfWeek:         clickhouseRename("DAYOFWEEK"),
		expressions.KindDayOfWeekIso:      clickhouseRename("DAYOFWEEKISO"),
		expressions.KindDayOfYear:         clickhouseRename("DAYOFYEAR"),
		expressions.KindWeekOfYear:        clickhouseRename("WEEKISO"),
		expressions.KindYearOfWeek:        clickhouseRename("YEAROFWEEK"),
		expressions.KindYearOfWeekIso:     clickhouseRename("YEAROFWEEKISO"),
		expressions.KindExplode:           clickhouseRename("FLATTEN"),
		expressions.KindCosineDistance:    clickhouseRename("VECTOR_COSINE_SIMILARITY"),
		expressions.KindEuclideanDistance: clickhouseRename("VECTOR_L2_DISTANCE"),
		expressions.KindManhattanDistance: clickhouseRename("VECTOR_L1_DISTANCE"),
		expressions.KindDotProduct:        clickhouseRename("VECTOR_INNER_PRODUCT"),
		// generators/snowflake.py:510 exp.Levenshtein, with unsupported cost args.
		expressions.KindLevenshtein: clickhouseRename("EDITDISTANCE"),
		// generators/snowflake.py:778-782 log_sql: a one-arg Log is Snowflake's natural log.
		expressions.KindLog: func(g *Generator, e expressions.Expression) string {
			if e.Arg("expression") == nil {
				return g.funcCall("LN", []any{e.Arg("this")}, "(", ")", true)
			}
			return g.logSQL(e)
		},
		expressions.KindUnnest:           snowflakeUnnestSQL,
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
		expressions.KindStddevSamp:       clickhouseRename("STDDEV"),
		expressions.KindStuff:            clickhouseRename("INSERT"),
		expressions.KindStPoint:          clickhouseRename("ST_MAKEPOINT"),
		expressions.KindStringToArray:    clickhouseRename("STRTOK_TO_ARRAY"),
		expressions.KindStrtokToArray:    clickhouseRename("STRTOK_TO_ARRAY"),
		expressions.KindToArray:          clickhouseRename("TO_ARRAY"),
		expressions.KindBooland:          clickhouseRename("BOOLAND"),
		expressions.KindBoolor:           clickhouseRename("BOOLOR"),
		expressions.KindLogicalAnd:       clickhouseRename("BOOLAND_AGG"),
		expressions.KindLogicalOr:        clickhouseRename("BOOLOR_AGG"),
		expressions.KindUnhex:            clickhouseRename("HEX_DECODE_BINARY"),
		expressions.KindUuid:             clickhouseRename("UUID_STRING"),
		expressions.KindJSONValueArray:   snowflakeJSONValueArraySQL,
		expressions.KindJSONExtractArray: snowflakeJSONQueryArraySQL,
		expressions.KindDateAdd:          snowflakeDateDeltaSQL("DATEADD"),
		expressions.KindDateSub:          snowflakeDateDeltaSubSQL("DATEADD"),
		expressions.KindDateDiff:         snowflakeDateDeltaSQL("DATEDIFF"),
		expressions.KindDatetimeAdd:      snowflakeDateDeltaSQL("TIMESTAMPADD"),
		expressions.KindDatetimeSub:      snowflakeDateDeltaSubSQL("TIMESTAMPADD"),
		expressions.KindDatetimeDiff:     snowflakeDateDeltaSQL("TIMESTAMPDIFF"),
		expressions.KindTimeAdd:          snowflakeDateDeltaSQL("TIMEADD"),
		expressions.KindTimeSub:          snowflakeDateDeltaSubSQL("TIMEADD"),
		expressions.KindTimestampAdd:     snowflakeDateDeltaSQL("TIMESTAMPADD"),
		expressions.KindTimestampSub:     snowflakeDateDeltaSubSQL("TIMESTAMPADD"),
		expressions.KindTimestampDiff: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("TIMESTAMPDIFF", []any{e.Arg("unit"), e.Arg("expression"), e.This()}, "(", ")", true)
		},
		expressions.KindTryCast:            snowflakeTryCastSQL,
		expressions.KindWhen:               snowflakeWhenSQL,
		expressions.KindDot:                snowflakeDotSQL,
		expressions.KindStrToDate:          snowflakeStrToDateSQL,
		expressions.KindParseDatetime:      snowflakeParseDatetimeSQL,
		expressions.KindTimestamp:          snowflakeTimestampSQL,
		expressions.KindJSONExtractScalar:  snowflakeJSONExtractScalarSQL,
		expressions.KindInt64:              snowflakeInt64SQL,
		expressions.KindLowerHex:           snowflakeLowerHexSQL,
		expressions.KindMakeInterval:       snowflakeMakeIntervalSQL,
		expressions.KindIgnoreNulls:        snowflakeNullsModifierSQL(true),
		expressions.KindRespectNulls:       snowflakeNullsModifierSQL(false),
		expressions.KindArrayConcatAgg:     snowflakeArrayConcatAggSQL,
		expressions.KindCurrentDate:        snowflakeCurrentDateSQL,
		expressions.KindTimeDiff:           snowflakeDateDeltaSQL("DATEDIFF"),
		expressions.KindTsOrDsAdd:          snowflakeDateDeltaSQL("DATEADD"),
		expressions.KindTsOrDsDiff:         snowflakeDateDeltaSQL("DATEDIFF"),
		expressions.KindDateTrunc:          snowflakeDateTruncSQL,
		expressions.KindTimestampTrunc:     snowflakeDateTruncSQL,
		expressions.KindDateFromParts:      clickhouseRename("DATE_FROM_PARTS"),
		expressions.KindTimeFromParts:      clickhouseRename("TIME_FROM_PARTS"),
		expressions.KindTimestampFromParts: clickhouseRename("TIMESTAMP_FROM_PARTS"),
		expressions.KindNthValue:           snowflakeNthValueSQL,
		expressions.KindWindow:             snowflakeWindowSQL,
		expressions.KindNot:                snowflakeNotSQL,
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
			if truthy(e.Arg("sysdate")) {
				return g.funcCall("SYSDATE", nil, "(", ")", true)
			}
			if e.Arg("this") != nil {
				return g.funcCall("CURRENT_TIMESTAMP", []any{e.Arg("this")}, "(", ")", true)
			}
			return "CURRENT_TIMESTAMP()"
		},
		expressions.KindStripNullValue: renameFunc("STRIP_NULL_VALUE"),
		expressions.KindFilter:         snowflakeFilterSQL,
		expressions.KindPercentileCont: percentileOrderedSQL("PERCENTILE_CONT"),
		expressions.KindPercentileDisc: percentileOrderedSQL("PERCENTILE_DISC"),
		expressions.KindWithinGroup:    snowflakeWithinGroupSQL,
		expressions.KindAtTimeZone: func(g *Generator, e expressions.Expression) string {
			naive := false
			if this := asExpression(e.This()); this != nil {
				if this.IsString() {
					naive = true
				} else if dtype, ok := castResultDType(this); ok && dtype == expressions.DTypeTimestamp {
					naive = true
				}
			}
			if naive {
				// A naive timestamp must be read in the supplied zone; Snowflake's two-argument
				// CONVERT_TIMEZONE would read it in the session zone instead.
				return g.funcCall("CONVERT_TIMEZONE", []any{e.Arg("zone"), expressions.LiteralString("UTC"), e.This()}, "(", ")", true)
			}
			return g.funcCall("CONVERT_TIMEZONE", []any{e.Arg("zone"), e.This()}, "(", ")", true)
		},
		expressions.KindCurrentSchemas: func(g *Generator, e expressions.Expression) string {
			return "CURRENT_SCHEMAS()"
		},
		expressions.KindArrayDistinct: func(g *Generator, e expressions.Expression) string {
			if truthy(e.Arg("check_null")) {
				return g.funcCall("ARRAY_DISTINCT", []any{e.This()}, "(", ")", true)
			}
			return g.funcCall("ARRAY_DISTINCT", []any{g.funcCall("ARRAY_COMPACT", []any{e.This()}, "(", ")", true)}, "(", ")", true)
		},
		expressions.KindArrayAgg: snowflakeArrayAggSQL,
		expressions.KindValues: func(g *Generator, e expressions.Expression) string {
			if e.Find(expressions.KindStruct) != nil || e.Find(expressions.KindStarMap) != nil || e.Find(expressions.KindVarMap) != nil || e.Find(expressions.KindMap) != nil {
				return g.valuesSQLAsTable(e, false)
			}
			return g.valuesSQL(e)
		},
		expressions.KindJSONArray: func(g *Generator, e expressions.Expression) string {
			args := make([]any, 0, len(e.Expressions()))
			for _, arg := range e.Expressions() {
				if arg != nil && arg.Kind() == expressions.KindNull {
					arg = expressions.ParseJSON(expressions.Args{"this": expressions.LiteralString("null")})
				}
				args = append(args, arg)
			}
			return g.funcCall("TO_VARIANT", []any{g.funcCall("ARRAY_CONSTRUCT", args, "(", ")", true)}, "(", ")", true)
		},
		expressions.KindGetExtract: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("GET", []any{e.This(), e.Arg("expression")}, "(", ")", true)
		},
		expressions.KindParseJSON: func(g *Generator, e expressions.Expression) string {
			name := "PARSE_JSON"
			if truthy(e.Arg("safe")) {
				name = "TRY_" + name
			}
			args := []any{e.This()}
			if mode := asExpression(e.Arg("expression")); mode != nil {
				args = append(args, mode)
			}
			return g.funcCall(name, args, "(", ")", true)
		},
		expressions.KindFlatten:       renameFunc("ARRAY_FLATTEN"),
		expressions.KindGenerator:     snowflakeGeneratorSQL,
		expressions.KindTableFromRows: snowflakeTableFromRowsSQL,
		expressions.KindSortArray:     snowflakeSortArraySQL,
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
		expressions.KindToChar:       snowflakeToCharSQL,
		expressions.KindTsOrDsToTime: snowflakeTsOrDsToTimeSQL,
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
		// generators/snowflake.py:490-495: ARRAY_GENERATE_RANGE with an inclusive end.
		expressions.KindGenerateSeries: snowflakeGenerateSeriesSQL,
		// generators/snowflake.py:545-555: the Select preprocess chain.
		expressions.KindSelect: snowflakeSelectSQL,
		// generators/snowflake.py function renames (same-dialect spelling).
		expressions.KindByteLength: renameFunc("OCTET_LENGTH"),
		expressions.KindSHA2Digest: renameFunc("SHA2_BINARY"),
		expressions.KindTimeSlice:  renameFunc("TIME_SLICE"),
		expressions.KindDecodeCase: renameFunc("DECODE"),
	}
	registerDialectDispatch("snowflake", handlers)
}

// snowflakeGenerateSeriesSQL ports the base Snowflake exp.GenerateSeries transform
// (generators/snowflake.py:490-495).
func snowflakeGenerateSeriesSQL(g *Generator, e expressions.Expression) string {
	end := e.Arg("end")
	if !boolValue(e.Arg("is_end_exclusive")) {
		// `e.args["end"] + 1` goes through Expression._binop, which wraps a Binary operand in
		// Paren (expressions/core.py:1399-1407); the transformed end is a Sub.
		end = expressions.Add(expressions.Args{
			"this":       wrapBinaryExpr(asExpression(e.Arg("end"))),
			"expression": expressions.LiteralNumber("1"),
		})
	}
	return g.funcCall("ARRAY_GENERATE_RANGE", []any{e.Arg("start"), end, e.Arg("step")}, "(", ")", true)
}

// wrapBinaryExpr ports expressions._wrap(expression, Binary): a Binary operand of a builder
// comparison/arith node is parenthesized (expressions/core.py:2792-2793).
func wrapBinaryExpr(expression expressions.Expression) expressions.Expression {
	if expression != nil && expression.Is(expressions.TraitBinary) {
		return expressions.Paren(expressions.Args{"this": expression})
	}
	return expression
}

// snowflakeSelectSQL ports the exp.Select preprocess entry (generators/snowflake.py:545-555).
// Only explode_projection_to_unnest is ported here; the other upstream transforms either
// have no equivalent construct in this port yet (eliminate_window_clause,
// eliminate_distinct_on, eliminate_semi_and_anti_joins, _transform_generate_date_array) or
// are folded into the dedicated generators (unnest_sql's alias handling covers part of
// _qualify_unnested_columns, jsonextract_sql covers _eliminate_dot_variant_lookup).
func snowflakeSelectSQL(g *Generator, e expressions.Expression) string {
	snowflakeTransformGenerateDateArray(e)
	snowflakeInlineNamedWindows(e)
	snowflakeQualifyUnnestedColumns(e)
	snowflakeEliminateDotVariantLookup(e)
	explodeProjectionToUnnest(e, 0)
	return g.selectSQL(e)
}

// findNewName ports find_new_name (helper used by transforms.py:414-417).
func findNewName(taken map[string]bool, base string) string {
	if !taken[base] {
		return base
	}
	for i := 2; ; i++ {
		name := base + "_" + strconv.Itoa(i)
		if !taken[name] {
			return name
		}
	}
}

// snowflakeSelectSourceNames returns the aliases of the SELECT's own sources, the naming set
// `Scope(expression).references` provides to explode_projection_to_unnest
// (transforms.py:412). Only FROM/JOIN sources belong to this scope: projections and nested
// selects live in child scopes and must not contribute names.
func snowflakeSelectSourceNames(selectExpr expressions.Expression) map[string]bool {
	names := map[string]bool{}
	add := func(node expressions.Expression) {
		if node == nil {
			return
		}
		switch node.Kind() {
		case expressions.KindTable, expressions.KindSubquery, expressions.KindUnnest,
			expressions.KindLateral, expressions.KindValues:
			if name := node.AliasOrName(); name != "" {
				names[name] = true
			}
		}
	}
	if from := asExpression(selectExpr.Arg("from_")); from != nil {
		add(from.This())
		for _, source := range from.Expressions() {
			add(source)
		}
	}
	for _, joinValue := range listFromValue(selectExpr.Arg("joins")) {
		if join := asExpression(joinValue); join != nil {
			add(join.This())
		}
	}
	return names
}

// explodeProjectionToUnnest ports explode_projection_to_unnest (transforms.py:402-552). A projection whose subtree contains an Explode (even
// inside a nested subquery, which is why the outer SELECT is rewritten for the correlated
// FLATTEN case) becomes a positional IFF over two FLATTEN sources plus a generated series.
// The Posexplode/Aliases branches are not ported: this port has no Posexplode node, so they
// are unreachable here.
func explodeProjectionToUnnest(selectExpr expressions.Expression, indexOffset int) {
	takenSelectNames := map[string]bool{}
	for _, name := range selectExpr.NamedSelects() {
		takenSelectNames[name] = true
	}
	takenSourceNames := snowflakeSelectSourceNames(selectExpr)

	newName := func(names map[string]bool, base string) string {
		name := findNewName(names, base)
		names[name] = true
		return name
	}

	var arrays []expressions.Expression
	seriesAlias := newName(takenSelectNames, "pos")
	series := expressions.Unnest(expressions.Args{
		"expressions": []expressions.Expression{expressions.GenerateSeries(expressions.Args{
			"start": expressions.LiteralNumber(strconv.Itoa(indexOffset)),
		})},
		"alias": expressions.TableAlias(expressions.Args{
			"this":    expressions.ToIdentifier(newName(takenSourceNames, "_u"), false),
			"columns": []expressions.Expression{expressions.ToIdentifier(seriesAlias, false)},
		}),
	})
	seriesTableAlias := asExpression(series.Arg("alias")).This()

	for _, selectItem := range selectExpr.Selects() {
		explode := selectItem.Find(expressions.KindExplode)
		if explode == nil {
			continue
		}

		posAlias := ""
		explodeAlias := ""
		var explodeAliasIdent expressions.Expression
		alias := selectItem
		if selectItem.Kind() == expressions.KindAlias {
			// Keep the alias Identifier node: upstream stores it (not just its name) so a
			// quoted alias such as "MyCol" stays quoted in every rebuilt reference
			// (transforms.py:437-480).
			explodeAliasIdent = asExpression(selectItem.Arg("alias"))
			if explodeAliasIdent != nil {
				explodeAlias = explodeAliasIdent.Name()
			}
		} else {
			// exp.alias_(select, ""): keep the projection but give it an (empty) alias slot.
			// Wrapping reparents the item, so Replace cannot reach the SELECT list; swap the
			// slot by identity instead.
			renamed := expressions.AliasExpr(selectItem, "", false)
			replaceSelectItem(selectExpr, selectItem, renamed)
			alias = renamed
			explode = renamed.Find(expressions.KindExplode)
		}

		explodeArg := explode.This()
		// This ensures that we won't use EXPLODE's argument as a new selection
		if explodeArg != nil && explodeArg.Kind() == expressions.KindColumn {
			takenSelectNames[explodeArg.OutputName()] = true
		}

		unnestSourceAlias := newName(takenSourceNames, "_u")

		if explodeAlias == "" {
			explodeAlias = newName(takenSelectNames, "col")
		}
		if explodeAliasIdent == nil {
			explodeAliasIdent = expressions.ToIdentifier(explodeAlias, false)
		}
		if posAlias == "" {
			posAlias = newName(takenSelectNames, "pos")
		}

		alias.Set("alias", explodeAliasIdent.Copy())

		seriesColumn := func() expressions.Expression {
			return expressions.Column(expressions.Args{
				"this":  expressions.ToIdentifier(seriesAlias, false),
				"table": seriesTableAlias.Copy(),
			})
		}
		unnestColumn := func(name string) expressions.Expression {
			return expressions.Column(expressions.Args{
				"this":  expressions.ToIdentifier(name, false),
				"table": expressions.ToIdentifier(unnestSourceAlias, false),
			})
		}
		explodeColumn := func() expressions.Expression {
			return expressions.Column(expressions.Args{
				"this":  explodeAliasIdent.Copy(),
				"table": expressions.ToIdentifier(unnestSourceAlias, false),
			})
		}
		explode.Replace(expressions.If(expressions.Args{
			"this": expressions.EQ(expressions.Args{
				"this":       seriesColumn(),
				"expression": unnestColumn(posAlias),
			}),
			"true": explodeColumn(),
		}))

		if len(arrays) == 0 {
			if selectExpr.Arg("from_") != nil {
				selectExpr.Join(series, expressions.JoinOptions{JoinType: "CROSS"})
			} else {
				selectExpr.From_(series, false)
			}
		}

		size := expressions.ArraySize(expressions.Args{"this": explodeArg.Copy()})
		arrays = append(arrays, size)

		selectExpr.Join(expressions.Unnest(expressions.Args{
			"expressions": []expressions.Expression{explodeArg.Copy()},
			"offset":      expressions.ToIdentifier(posAlias, false),
			"alias": expressions.TableAlias(expressions.Args{
				"this":    expressions.ToIdentifier(unnestSourceAlias, false),
				"columns": []expressions.Expression{explodeAliasIdent.Copy()},
			}),
		}), expressions.JoinOptions{JoinType: "CROSS"})

		// index_offset != 1, so the positional bound is the (zero-based) last array index.
		sizeBound := size.Copy()
		if indexOffset != 1 {
			sizeBound = expressions.Sub(expressions.Args{
				"this":       sizeBound,
				"expression": expressions.LiteralNumber("1"),
			})
		}
		selectExpr.Where(true, false, expressions.Or_(
			expressions.EQ(expressions.Args{
				"this":       seriesColumn(),
				"expression": unnestColumn(posAlias),
			}),
			expressions.And_(
				expressions.GT(expressions.Args{
					"this":       seriesColumn(),
					"expression": wrapBinaryExpr(sizeBound.Copy()),
				}),
				expressions.EQ(expressions.Args{
					"this":       unnestColumn(posAlias),
					"expression": wrapBinaryExpr(sizeBound.Copy()),
				}),
			),
		))
	}

	if len(arrays) > 0 {
		end := expressions.Greatest(expressions.Args{"this": arrays[0], "expressions": arrays[1:]})
		// index_offset != 1: end = end - (1 - index_offset).
		if indexOffset != 1 {
			end = expressions.Sub(expressions.Args{"this": end, "expression": expressions.LiteralNumber("1")})
		}
		if elements := series.Expressions(); len(elements) > 0 {
			elements[0].Set("end", end)
		}
	}
}

// replaceSelectItem swaps one SELECT projection for another by node identity.
func replaceSelectItem(selectExpr, old, replacement expressions.Expression) {
	items := selectExpr.Expressions()
	for i, item := range items {
		if item == old {
			items[i] = replacement
			selectExpr.Set("expressions", items)
			return
		}
	}
}

func snowflakeStrToTimeSQL(g *Generator, e expressions.Expression) string {
	// generators/snowflake.py:957-973: the function name comes from TIMESTAMP_TYPES
	// (parsers/snowflake.py:74-79), and TO_TIME stays for the port's TIME target.
	name := "TO_TIMESTAMP"
	if target := asExpression(e.Arg("target_type")); target != nil {
		switch target.Arg("this") {
		case expressions.DTypeTime:
			name = "TO_TIME"
		case expressions.DTypeTimestampLtz:
			name = "TO_TIMESTAMP_LTZ"
		case expressions.DTypeTimestampNtz:
			name = "TO_TIMESTAMP_NTZ"
		case expressions.DTypeTimestampTz:
			name = "TO_TIMESTAMP_TZ"
		}
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

func snowflakeWindowSQL(g *Generator, e expressions.Expression) string {
	spec := asExpression(e.Arg("spec"))
	function := asExpression(e.Arg("this"))
	if function != nil && (function.Kind() == expressions.KindIgnoreNulls ||
		function.Kind() == expressions.KindRespectNulls) {
		function = asExpression(function.This())
	}
	ranking := function != nil && (function.Kind() == expressions.KindFirstValue ||
		function.Kind() == expressions.KindLastValue || function.Kind() == expressions.KindNthValue)
	if ranking && spec != nil && strings.EqualFold(spec.Text("kind"), "ROWS") &&
		strings.EqualFold(spec.Text("start"), "UNBOUNDED") &&
		strings.EqualFold(spec.Text("start_side"), "PRECEDING") &&
		strings.EqualFold(spec.Text("end"), "UNBOUNDED") &&
		strings.EqualFold(spec.Text("end_side"), "FOLLOWING") {
		copy := e.Copy()
		copy.Set("spec", nil)
		return g.windowSQL(copy)
	}
	return g.windowSQL(e)
}

func snowflakeNotSQL(g *Generator, e expressions.Expression) string {
	in := asExpression(e.This())
	if in != nil && in.Kind() == expressions.KindIn && in.Arg("query") != nil {
		return g.sqlKey(in, "this") + " <> ALL " + g.sqlKey(in, "query")
	}
	return g.notSQL(e)
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
	if group := asExpression(e.Arg("group")); group != nil && (!group.IsNumber() || group.Name() != "0") {
		if args[2] == nil {
			args[2] = expressions.LiteralNumber("1")
		}
		if args[3] == nil {
			args[3] = expressions.LiteralNumber("1")
		}
		if args[4] == nil {
			args[4] = expressions.LiteralString("c")
		}
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

func snowflakeJSONPathSQL(g *Generator, path expressions.Expression) string {
	if path != nil && path.Kind() == expressions.KindLiteral && path.IsString() {
		text := path.Name()
		text = strings.TrimPrefix(text, "$.")
		text = strings.TrimPrefix(text, "$")
		return "'" + strings.ReplaceAll(text, "'", "''") + "'"
	}
	if path != nil && path.Kind() == expressions.KindJSONPath {
		parts := path.Expressions()
		if len(parts) > 0 && parts[0].Kind() == expressions.KindJSONPathRoot {
			parts = parts[1:]
		}
		path = expressions.JSONPath(expressions.Args{"expressions": parts})
	}
	pathSQL := g.gen(path)
	if path != nil && (path.Kind() == expressions.KindJSONPath ||
		(path.Kind() == expressions.KindLiteral && path.IsString())) && pathSQL != "''" {
		pathSQL = strings.ReplaceAll(pathSQL, "''", "\\'")
	}
	return pathSQL
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
	if path != nil && (path.Kind() == expressions.KindJSONPath ||
		(path.Kind() == expressions.KindLiteral && path.IsString())) && pathSQL != "''" {
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

// snowflakeUnnestSQL ports unnest_sql (generators/snowflake.py:804-844): Snowflake
// rewrites UNNEST to TABLE(FLATTEN(...)) with the flatten column list.
func snowflakeUnnestSQL(g *Generator, e expressions.Expression) string {
	aliasExpr := asExpression(e.Arg("alias"))
	offset := e.Arg("offset")
	parent := e.Parent()

	value := expressions.Expression(nil)
	if aliasExpr != nil {
		if columns := listFromValue(aliasExpr.Arg("columns")); len(columns) > 0 {
			value = asExpression(columns[0])
		}
	}
	if value == nil {
		value = expressions.ToIdentifier("value", false)
	}
	ordinal := expressions.Expression(expressions.ToIdentifier("index", false))
	if offsetExpr := asExpression(offset); offsetExpr != nil {
		ordinal = offsetExpr
	}
	columns := []expressions.Expression{
		expressions.ToIdentifier("seq", false),
		expressions.ToIdentifier("key", false),
		expressions.ToIdentifier("path", false),
		ordinal,
		value,
		expressions.ToIdentifier("this", false),
	}
	if aliasExpr != nil {
		aliasExpr.Set("columns", columns)
	} else if parent != nil && (parent.Kind() == expressions.KindFrom || parent.Kind() == expressions.KindJoin) {
		// _qualify_unnested_columns (generators/snowflake.py:293-303) gives a From/Join
		// unnest a columns-only alias; tablealias_sql then names it via nextNameSQL.
		aliasExpr = expressions.TableAlias(expressions.Args{"columns": columns})
	} else {
		aliasExpr = expressions.TableAlias(expressions.Args{
			"this": expressions.ToIdentifier("_u", false), "columns": columns,
		})
	}

	inputs := e.Expressions()
	tableInput := ""
	if len(inputs) > 0 {
		tableInput = g.gen(inputs[0])
	}
	if !strings.HasPrefix(tableInput, "INPUT =>") {
		tableInput = "INPUT => " + tableInput
	}
	explode := "FLATTEN(" + tableInput + ")"
	if parent == nil || parent.Kind() != expressions.KindLateral {
		explode = "TABLE(" + explode + ")"
	}
	alias := g.gen(aliasExpr)
	if alias != "" {
		alias = " AS " + alias
	}
	prefix := ""
	if parent == nil || (parent.Kind() != expressions.KindFrom && parent.Kind() != expressions.KindJoin && parent.Kind() != expressions.KindLateral) {
		prefix = g.gen(value) + " FROM "
	}
	return prefix + explode + alias
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
// snowflakeDateDeltaSubSQL negates the count before DATEADD/TIMESTAMPADD (generators/
// snowflake.py:977-1014): snowflake has no SUB form.
func snowflakeDateDeltaSubSQL(name string) func(*Generator, expressions.Expression) string {
	return func(g *Generator, e expressions.Expression) string {
		count := asExpression(e.Arg("expression"))
		if count != nil {
			count = expressions.Mul(expressions.Args{
				"this":       count,
				"expression": expressions.LiteralNumber("-1"),
			})
		}
		return g.funcCall(name, []any{
			snowflakeUnit(g, asExpression(e.Arg("unit"))),
			count,
			e.This(),
		}, "(", ")", true)
	}
}

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
	this := asExpression(e.This())
	var order expressions.Expression
	if this != nil && this.Kind() == expressions.KindOrder {
		order = this
		this = asExpression(this.This())
	}
	sep := asExpression(e.Arg("separator"))
	if sep != nil && sep.Kind() == expressions.KindOrder {
		if order == nil {
			order = sep
		}
		sep = asExpression(sep.This())
	}
	args := []any{this}
	if sep != nil {
		args = append(args, sep)
	}
	sql := g.funcCall("LISTAGG", args, "(", ")", true)
	if order != nil {
		orderOnly := expressions.Order(expressions.Args{"expressions": order.Expressions()})
		sql += " WITHIN GROUP (" + strings.TrimSpace(g.gen(orderOnly)) + ")"
	}
	return sql
}

// snowflakeStrPositionSQL ports the StrPosition lambda (generators/snowflake.py:743-746).
func snowflakeStrPositionSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("CHARINDEX", []any{e.Arg("substr"), e.This(), e.Arg("position")}, "(", ")", true)
}

// snowflakeJSONValueArraySQL renders JSON value arrays as ARRAY_AGG over GET_PATH.
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
	return g.funcCall("DATE_PART", []any{snowflakeUnit(g, unit), e.Arg("expression")}, "(", ")", true)
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

func snowflakeToCharSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("TO_CHAR", g.fallbackArgs(e), "(", ")", true)
}

func snowflakeTsOrDsToTimeSQL(g *Generator, e expressions.Expression) string {
	name := "TO_TIME"
	if boolValue(e.Arg("safe")) {
		name = "TRY_TO_TIME"
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
	if unit.Kind() == expressions.KindLiteral && unit.IsString() {
		return unit
	}
	if unit.Kind() == expressions.KindVar || unit.Kind() == expressions.KindLiteral {
		name := strings.ToUpper(unit.Name())
		if mapped := g.dialect.DatePartMapping[name]; mapped != "" {
			if mapped != name {
				return expressions.Var(expressions.Args{"this": mapped})
			}
		}
		return unit
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

// Snowflake's TO_/TRY_TO_/APPROX_PERCENTILE/GREATEST_IGNORE_NULLS spellings.
func init() {
	registerDialectDispatch("snowflake", map[expressions.Kind]func(*Generator, expressions.Expression) string{
		expressions.KindStructExtract: structExtractSQL,
		expressions.KindToBinary: func(g *Generator, e expressions.Expression) string {
			return g.funcCall(tryPrefix(e)+"TO_BINARY", g.fallbackArgs(e), "(", ")", true)
		},
		expressions.KindToDouble: func(g *Generator, e expressions.Expression) string {
			return g.funcCall(tryPrefix(e)+"TO_DOUBLE", g.fallbackArgs(e), "(", ")", true)
		},
		expressions.KindToFile: func(g *Generator, e expressions.Expression) string {
			return g.funcCall(tryPrefix(e)+"TO_FILE", g.fallbackArgs(e), "(", ")", true)
		},
		expressions.KindApproxQuantile: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("APPROX_PERCENTILE", []any{e.This(), e.Arg("quantile")}, "(", ")", true)
		},
		expressions.KindGreatest: func(g *Generator, e expressions.Expression) string {
			name := "GREATEST"
			if truthy(e.Arg("ignore_nulls")) {
				name = "GREATEST_IGNORE_NULLS"
			}
			return g.funcCall(name, g.fallbackArgs(e), "(", ")", true)
		},
		expressions.KindLeast: func(g *Generator, e expressions.Expression) string {
			name := "LEAST"
			if truthy(e.Arg("ignore_nulls")) {
				name = "LEAST_IGNORE_NULLS"
			}
			return g.funcCall(name, g.fallbackArgs(e), "(", ")", true)
		},
	})
}

func tryPrefix(e expressions.Expression) string {
	if truthy(e.Arg("safe")) {
		return "TRY_"
	}
	return ""
}

func init() {
	registerDialectDispatch("snowflake", map[expressions.Kind]func(*Generator, expressions.Expression) string{
		expressions.KindDecrypt: func(g *Generator, e expressions.Expression) string {
			return g.funcCall(tryPrefix(e)+"DECRYPT", g.fallbackArgs(e), "(", ")", true)
		},
		expressions.KindDecryptRaw: func(g *Generator, e expressions.Expression) string {
			return g.funcCall(tryPrefix(e)+"DECRYPT_RAW", g.fallbackArgs(e), "(", ")", true)
		},
		expressions.KindToBoolean: func(g *Generator, e expressions.Expression) string {
			return g.funcCall(tryPrefix(e)+"TO_BOOLEAN", g.fallbackArgs(e), "(", ")", true)
		},
	})
}

// Snowflake's COLLATE_IS_FUNC=True renders COLLATE as a function (generators/snowflake.py:384).
func init() {
	registerDialectDispatch("snowflake", map[expressions.Kind]func(*Generator, expressions.Expression) string{
		expressions.KindCollate: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("COLLATE", []any{e.This(), e.Arg("expression")}, "(", ")", true)
		},
	})
}

// snowflakeSortArraySQL ports sortarray_sql (generators/snowflake.py:625-631).
func snowflakeSortArraySQL(g *Generator, e expressions.Expression) string {
	asc := asExpression(e.Arg("asc"))
	nullsFirst := asExpression(e.Arg("nulls_first"))
	if isFalseExpr(asc) && isTrueExpr(nullsFirst) {
		nullsFirst = nil
	}
	// The third positional argument is null placement, so a nulls_first without asc
	// needs an explicit ascending TRUE to occupy the direction slot.
	if asc == nil && nullsFirst != nil {
		asc = expressions.Boolean(expressions.Args{"this": true})
	}
	return g.funcCall("ARRAY_SORT", []any{e.This(), asc, nullsFirst}, "(", ")", true)
}

func isTrueExpr(e expressions.Expression) bool {
	if e == nil || e.Kind() != expressions.KindBoolean {
		return false
	}
	return truthy(e.Arg("this"))
}

func isFalseExpr(e expressions.Expression) bool {
	if e == nil || e.Kind() != expressions.KindBoolean {
		return false
	}
	return !truthy(e.Arg("this"))
}

// snowflakeGeneratorSQL ports generator_sql (generators/snowflake.py:792-802).
func snowflakeGeneratorSQL(g *Generator, e expressions.Expression) string {
	var args []any
	if rowcount := e.Arg("rowcount"); rowcount != nil {
		args = append(args, expressions.Kwarg(expressions.Args{
			"this": expressions.Var(expressions.Args{"this": "ROWCOUNT"}), "expression": rowcount,
		}))
	}
	if timelimit := e.Arg("timelimit"); timelimit != nil {
		args = append(args, expressions.Kwarg(expressions.Args{
			"this": expressions.Var(expressions.Args{"this": "TIMELIMIT"}), "expression": timelimit,
		}))
	}
	return g.funcCall("GENERATOR", args, "(", ")", true)
}

// snowflakeTableFromRowsSQL ports the inherited base tablefromrows_sql output for Snowflake:
// TABLE(<this>) with an optional alias.
func snowflakeTableFromRowsSQL(g *Generator, e expressions.Expression) string {
	sql := "TABLE(" + g.sqlKey(e, "this") + ")"
	if alias := g.sqlKey(e, "alias"); alias != "" {
		sql += " AS " + alias
	}
	return sql
}

// snowflakeFilterSQL ports filter_sql (generators/snowflake.py:1178-1248): rewrite
// FILTER into IFF inside the aggregate's argument, or COUNT_IF for star counts.
func snowflakeFilterSQL(g *Generator, e expressions.Expression) string {
	agg := asExpression(e.This())
	if agg == nil {
		return g.functionFallbackSQL(e)
	}
	cond := asExpression(e.Arg("expression"))
	if cond != nil && cond.Kind() == expressions.KindWhere {
		cond = asExpression(cond.This())
	}
	if cond == nil {
		return g.functionFallbackSQL(e)
	}
	aggArg := asExpression(agg.This())
	if agg.Kind() == expressions.KindWithinGroup {
		inner := asExpression(agg.This())
		if inner == nil {
			return g.functionFallbackSQL(e)
		}
		if inner.Kind() == expressions.KindMode || inner.Kind() == expressions.KindPercentileCont || inner.Kind() == expressions.KindPercentileDisc {
			if order := asExpression(agg.Arg("expression")); order != nil {
				for _, ordered := range order.Expressions() {
					key := asExpression(ordered.This())
					ordered.Set("this", expressions.If(expressions.Args{"this": cond.Copy(), "true": key.Copy()}))
				}
			}
			return g.gen(agg)
		}
		if inner.Kind() == expressions.KindArrayAgg || inner.Kind() == expressions.KindGroupConcat {
			aggArg = asExpression(inner.This())
		} else {
			g.unsupported("Unable to rewrite FILTER into the aggregate's arguments")
			return g.gen(agg)
		}
	}
	if agg.Kind() == expressions.KindCount && aggArg != nil && aggArg.IsStar() {
		// COUNT_IF returns NULL on an empty input; COUNT(IFF(cond, 1, NULL)) returns 0.
		iff := g.funcCall("IFF", []any{cond, expressions.LiteralNumber("1"), expressions.Null()}, "(", ")", true)
		return g.funcCall("COUNT", []any{iff}, "(", ")", true)
	}
	var orderNode expressions.Expression
	if aggArg != nil && aggArg.Kind() == expressions.KindOrder {
		orderNode = aggArg
		aggArg = asExpression(aggArg.This())
	}
	var targets []expressions.Expression
	if aggArg != nil && aggArg.Kind() == expressions.KindDistinct {
		targets = aggArg.Expressions()
		if orderNode != nil {
			// Snowflake requires the DISTINCT expression and the order key to match.
			for _, ordered := range orderNode.Expressions() {
				key := asExpression(ordered.This())
				for _, target := range targets {
					if key != nil && target != nil && key.Kind() == target.Kind() && key.Name() == target.Name() {
						ordered.Set("this", expressions.If(expressions.Args{"this": cond.Copy(), "true": key.Copy()}))
						break
					}
				}
			}
		}
	} else {
		targets = []expressions.Expression{aggArg}
	}
	for _, target := range targets {
		if target == nil {
			continue
		}
		target.Replace(expressions.If(expressions.Args{"this": cond.Copy(), "true": target.Copy()}))
	}
	return g.gen(agg)
}

// snowflakeWithinGroupSQL ports withingroup_sql (generators/snowflake.py:1250-1259).
func snowflakeWithinGroupSQL(g *Generator, e expressions.Expression) string {
	this := asExpression(e.This())
	if this != nil && this.Kind() == expressions.KindMode && this.This() == nil {
		if order := asExpression(e.Arg("expression")); order != nil && order.Kind() == expressions.KindOrder && len(order.Expressions()) == 1 {
			return g.gen(expressions.New(expressions.KindMode, expressions.Args{"this": asExpression(order.Expressions()[0].This())}))
		}
	}
	return g.withinGroupSQL(e)
}

// snowflakeArrayAggSQL moves an inline ORDER BY into WITHIN GROUP, which is the only
// form Snowflake accepts for an ordered ARRAY_AGG.
func snowflakeArrayAggSQL(g *Generator, e expressions.Expression) string {
	this := asExpression(e.This())
	if this != nil && this.Kind() == expressions.KindOrder {
		order := expressions.Order(expressions.Args{"expressions": this.Expressions()})
		sql := g.funcCall("ARRAY_AGG", []any{this.This()}, "(", ")", true)
		return sql + " WITHIN GROUP (" + strings.TrimSpace(g.gen(order)) + ")"
	}
	return g.arrayAggSQL(e)
}

// snowflakeFormatTime converts a format literal through the inverse time mapping.
func snowflakeFormatTime(g *Generator, e expressions.Expression) any {
	format := asExpression(e.Arg("format"))
	if format != nil && format.IsString() {
		if converted, ok := dialects.FormatTime(format.Name(), g.dialect.InverseTimeMapping); ok {
			return expressions.LiteralString(converted)
		}
	}
	return format
}

func snowflakeStrToDateSQL(g *Generator, e expressions.Expression) string {
	// generators/snowflake.py:571: StrToDate renders as DATE(value, format).
	return g.funcCall("DATE", []any{e.This(), snowflakeFormatTime(g, e)}, "(", ")", true)
}

func snowflakeParseDatetimeSQL(g *Generator, e expressions.Expression) string {
	// generator.py:4151-4156: PARSE_DATETIME keeps the raw format and drops default_year.
	return g.funcCall("PARSE_DATETIME", []any{e.This(), e.Arg("format")}, "(", ")", true)
}

func snowflakeTimestampSQL(g *Generator, e expressions.Expression) string {
	// generators/snowflake.py: exp.Timestamp: no_timestamp_sql. A zone reads the value there
	// through CONVERT_TIMEZONE.
	if zone := asExpression(e.Arg("zone")); zone != nil {
		// A naive timestamp is read in the supplied zone: Snowflake's two-argument form would
		// read it in the session zone.
		naive := false
		if this := asExpression(e.This()); this != nil {
			if this.IsString() {
				naive = true
			} else if dtype, ok := castResultDType(this); ok && dtype == expressions.DTypeTimestamp {
				naive = true
			}
		}
		if naive {
			value := e.This()
			if dtype, ok := castResultDType(asExpression(value)); !ok || dtype != expressions.DTypeTimestamp {
				value = expressions.Cast(expressions.Args{"this": value, "to": expressions.DTypeTimestamp.IntoExpr(nil)})
			}
			return g.gen(expressions.Anonymous(expressions.Args{
				"this":        "CONVERT_TIMEZONE",
				"expressions": []expressions.Expression{zone, expressions.LiteralString("UTC"), value},
			}))
		}
		return g.gen(expressions.Anonymous(expressions.Args{
			"this": "CONVERT_TIMEZONE",
			"expressions": []expressions.Expression{
				zone,
				expressions.Cast(expressions.Args{"this": e.This(), "to": expressions.DTypeTimestamp.IntoExpr(nil)}),
			},
		}))
	}
	return g.gen(expressions.Cast(expressions.Args{
		"this": e.This(), "to": expressions.DTypeTimestampTz.IntoExpr(nil),
	}))
}

func snowflakeJSONExtractScalarSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("JSON_EXTRACT_PATH_TEXT", []any{e.This(), snowflakeJSONPathSQL(g, asExpression(e.Arg("expression")))}, "(", ")", true)
}

func snowflakeJSONArrayInput(g *Generator, e expressions.Expression) string {
	this := asExpression(e.This())
	if this != nil && (this.Kind() == expressions.KindParseJSON || this.Kind() == expressions.KindJSONExtract) {
		return g.gen(this)
	}
	return "PARSE_JSON(" + g.gen(e.This()) + ")"
}

func snowflakeJSONValueArraySQL(g *Generator, e expressions.Expression) string {
	path := snowflakeJSONPathSQL(g, asExpression(e.Arg("expression")))
	return "TRANSFORM(GET_PATH(" + snowflakeJSONArrayInput(g, e) + ", " + path + "), x -> CAST(x AS VARCHAR))"
}

func snowflakeJSONQueryArraySQL(g *Generator, e expressions.Expression) string {
	path := snowflakeJSONPathSQL(g, asExpression(e.Arg("expression")))
	return "TRANSFORM(GET_PATH(" + snowflakeJSONArrayInput(g, e) + ", " + path + "), x -> PARSE_JSON(TO_JSON(x)))"
}

func snowflakeInt64SQL(g *Generator, e expressions.Expression) string {
	return g.gen(expressions.Cast(expressions.Args{"this": e.This(), "to": expressions.DTypeBigInt.IntoExpr(nil)}))
}

func snowflakeLowerHexSQL(g *Generator, e expressions.Expression) string {
	// TO_HEX(SHA1(x)) becomes TO_CHAR(SHA1_BINARY(x)); other hex values keep HEX_ENCODE.
	if this := asExpression(e.This()); this != nil && this.Kind() == expressions.KindSHA1Digest {
		return g.funcCall("TO_CHAR", []any{expressions.Anonymous(expressions.Args{
			"this": "SHA1_BINARY", "expressions": []expressions.Expression{this.This()},
		})}, "(", ")", true)
	}
	return g.functionFallbackSQL(e)
}

func snowflakeMakeIntervalSQL(g *Generator, e expressions.Expression) string {
	// no_make_interval_sql with sep=", " on Snowflake: lowercase units in source order.
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
	return "INTERVAL '" + strings.Join(args, ", ") + "'"
}

func snowflakeNullsModifierSQL(ignore bool) func(*Generator, expressions.Expression) string {
	return func(g *Generator, e expressions.Expression) string {
		// Snowflake ARRAY_AGG already applies its own NULL semantics; the modifier is dropped.
		if this := asExpression(e.This()); this != nil && this.Kind() == expressions.KindArrayAgg {
			return g.gen(this)
		}
		if ignore {
			return g.ignoreNullsSQL(e)
		}
		return g.respectNullsSQL(e)
	}
}

func snowflakeArrayConcatAggSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("ARRAY_FLATTEN", []any{expressions.Anonymous(expressions.Args{
		"this": "ARRAY_AGG", "expressions": []expressions.Expression{e.This()},
	})}, "(", ")", true)
}

func snowflakeCurrentDateSQL(g *Generator, e expressions.Expression) string {
	zone := asExpression(e.This())
	if zone == nil {
		return "CURRENT_DATE"
	}
	// generators/snowflake.py:1095-1100: CURRENT_DATE(zone) reads the current instant there.
	return g.gen(expressions.Cast(expressions.Args{
		"this": expressions.Anonymous(expressions.Args{
			"this":        "CONVERT_TIMEZONE",
			"expressions": []expressions.Expression{zone, expressions.Anonymous(expressions.Args{"this": "CURRENT_TIMESTAMP"})},
		}),
		"to": expressions.DTypeDate.IntoExpr(nil),
	}))
}

// snowflakeTryCastSQL ports trycast_sql (generators/snowflake.py:762-776): Snowflake requires
// TRY_CAST's value to be a string; anything else falls back to CAST.
func snowflakeTryCastSQL(g *Generator, e expressions.Expression) string {
	this := asExpression(e.This())
	if truthy(e.Arg("requires_string")) || snowflakeIsText(this) {
		return g.tryCastSQL(e)
	}
	return g.castSQL(e)
}

func snowflakeIsText(e expressions.Expression) bool {
	if e == nil {
		return false
	}
	if e.IsString() || e.Kind() == expressions.KindNational || e.Kind() == expressions.KindRawString {
		return true
	}
	if dtype, ok := castResultDType(e); ok {
		switch dtype {
		case expressions.DTypeText, expressions.DTypeVarchar, expressions.DTypeChar,
			expressions.DTypeNVarchar, expressions.DTypeNChar:
			return true
		}
	}
	return false
}

// snowflakeWhenSQL drops BY SOURCE, which Snowflake does not support.
func snowflakeWhenSQL(g *Generator, e expressions.Expression) string {
	if boolValue(e.Arg("source")) {
		e.Set("source", false)
	}
	return g.whenSQL(e)
}

// snowflakeInlineNamedWindows ports eliminate_window_clause (transforms.py:1001-1029) for the
// simple inheritance form: every OVER <name> copies the named window's parts.
func snowflakeInlineNamedWindows(selectExpr expressions.Expression) {
	defs, _ := selectExpr.Arg("windows").([]expressions.Expression)
	if len(defs) == 0 {
		return
	}
	byName := map[string]expressions.Expression{}
	for _, def := range defs {
		if def == nil {
			continue
		}
		name := strings.ToLower(def.Name())
		if base := asExpression(def.Arg("alias")); base != nil {
			if inherited := byName[strings.ToLower(base.Name())]; inherited != nil {
				def.Set("alias", nil)
				for _, key := range []string{"partition_by", "order", "spec"} {
					if value := inherited.Arg(key); value != nil {
						def.Set(key, value)
					}
				}
			}
		}
		byName[name] = def
	}
	for _, window := range selectExpr.FindAll(expressions.KindWindow) {
		if window == nil {
			continue
		}
		isDef := false
		for _, def := range defs {
			if window == def {
				isDef = true
				break
			}
		}
		if isDef {
			continue
		}
		if ancestor := window.FindAncestor(expressions.KindSelect); ancestor != selectExpr {
			continue
		}
		alias := asExpression(window.Arg("alias"))
		if alias == nil {
			continue
		}
		def := byName[strings.ToLower(alias.Name())]
		if def == nil {
			continue
		}
		window.Set("alias", nil)
		for _, key := range []string{"partition_by", "order", "spec"} {
			if value := def.Arg(key); value != nil {
				window.Set(key, value)
			}
		}
	}
	selectExpr.Set("windows", nil)
}

// snowflakeArraySQL renders ARRAY(SELECT AS STRUCT ...) as an ARRAY_AGG(OBJECT_CONSTRUCT(...))
// subquery (generators/snowflake.py array/STRUCT handling).
func snowflakeArraySQL(g *Generator, e expressions.Expression) string {
	if truthy(e.Arg("struct_name_inheritance")) {
		items := e.Expressions()
		if len(items) > 0 && items[0] != nil && items[0].Kind() == expressions.KindStruct {
			first := items[0].Expressions()
			named := len(first) > 0
			for _, field := range first {
				if field == nil || field.Kind() != expressions.KindPropertyEQ {
					named = false
					break
				}
			}
			if named {
				for _, item := range items[1:] {
					if item == nil || item.Kind() != expressions.KindStruct || len(item.Expressions()) != len(first) {
						continue
					}
					fields := make([]expressions.Expression, len(first))
					for i, field := range item.Expressions() {
						if field != nil && field.Kind() == expressions.KindPropertyEQ {
							fields[i] = field
							continue
						}
						fields[i] = expressions.PropertyEQ(expressions.Args{
							"this":       asExpression(first[i].Arg("this")).Copy(),
							"expression": field,
						})
					}
					item.Set("expressions", fields)
				}
			}
		}
	}
	items := e.Expressions()
	if len(items) == 1 && items[0] != nil && items[0].Kind() == expressions.KindSelect &&
		strings.EqualFold(items[0].Text("kind"), "STRUCT") {
		query := items[0]
		args := make([]expressions.Expression, 0, len(query.Expressions())*2)
		for _, item := range query.Expressions() {
			if item == nil {
				continue
			}
			if item.Kind() == expressions.KindAlias {
				args = append(args, expressions.LiteralString(asExpression(item.Arg("alias")).Name()), item.This())
				continue
			}
			if name := item.OutputName(); name != "" {
				args = append(args, expressions.LiteralString(name), item)
				continue
			}
			args = append(args, item)
		}
		agg := expressions.Anonymous(expressions.Args{
			"this": "ARRAY_AGG",
			"expressions": []expressions.Expression{expressions.Anonymous(expressions.Args{
				"this": "OBJECT_CONSTRUCT", "expressions": args,
			})},
		})
		clone := query.Copy()
		clone.Set("kind", nil)
		clone.Set("expressions", []expressions.Expression{agg})
		return "(" + g.gen(clone) + ")"
	}
	return g.inlineArraySQL(e)
}

// snowflakeDotSQL keeps the Snowflake colon form for a field of an OBJECT cast.
func snowflakeDotSQL(g *Generator, e expressions.Expression) string {
	if left := asExpression(e.This()); left != nil && left.Kind() == expressions.KindCast {
		if to := asExpression(left.Arg("to")); to != nil {
			dtype, _ := to.Arg("this").(expressions.DType)
			if dtype == expressions.DTypeStruct || dtype == expressions.DTypeObject {
				return g.gen(left) + ":" + g.gen(e.Arg("expression"))
			}
		}
	}
	return g.dotSQL(e)
}

// snowflakeUnnestStructColumns returns the struct field names an UNNEST can produce.
func snowflakeUnnestStructColumns(unnest expressions.Expression) []string {
	for _, value := range unnest.Expressions() {
		array := asExpression(value)
		if array == nil || array.Kind() != expressions.KindArray {
			continue
		}
		for _, item := range array.Expressions() {
			st := asExpression(item)
			if st == nil || st.Kind() != expressions.KindStruct || len(st.Expressions()) == 0 {
				continue
			}
			names := make([]string, 0, len(st.Expressions()))
			ok := true
			for _, field := range st.Expressions() {
				if field == nil || field.Kind() != expressions.KindPropertyEQ {
					ok = false
					break
				}
				names = append(names, field.Name())
			}
			if ok && len(names) > 0 {
				return names
			}
		}
	}
	return nil
}

// snowflakeQualifyUnnestedColumns ports _qualify_unnested_columns (generators/snowflake.py:
// 245-333): a struct field projected from UNNEST is qualified with the flattened value name.
func snowflakeQualifyUnnestedColumns(selectExpr expressions.Expression) {
	var unnests []expressions.Expression
	for _, unnest := range selectExpr.FindAll(expressions.KindUnnest) {
		if unnest == nil {
			continue
		}
		parent := unnest.Parent()
		if parent == nil || (parent.Kind() != expressions.KindFrom && parent.Kind() != expressions.KindJoin) {
			continue
		}
		if ancestor := unnest.FindAncestor(expressions.KindSelect); ancestor != selectExpr {
			continue
		}
		unnests = append(unnests, unnest)
	}
	if len(unnests) == 0 {
		return
	}
	columnSource := map[string]expressions.Expression{}
	var unnestIdentifier expressions.Expression
	for _, unnest := range unnests {
		alias := asExpression(unnest.Arg("alias"))
		var identifier expressions.Expression
		if alias != nil {
			identifier = alias.This()
			if identifier == nil {
				if cols := listFromValue(alias.Arg("columns")); len(cols) > 0 {
					identifier = asExpression(cols[0])
				}
			}
		}
		if identifier == nil {
			identifier = expressions.ToIdentifier("value", false)
			unnest.Set("alias", expressions.TableAlias(expressions.Args{
				"columns": []expressions.Expression{identifier.Copy()},
			}))
		}
		for _, name := range snowflakeUnnestStructColumns(unnest) {
			columnSource[strings.ToLower(name)] = identifier.Copy()
		}
		unnestIdentifier = identifier
	}
	sourceCount := 0
	if from := asExpression(selectExpr.Arg("from_")); from != nil && from.This() != nil {
		sourceCount++
	}
	for _, join := range listFromValue(selectExpr.Arg("joins")) {
		if asExpression(join) != nil {
			sourceCount++
		}
	}
	for _, column := range selectExpr.FindAll(expressions.KindColumn) {
		if column == nil || asExpression(column.Arg("table")) != nil {
			continue
		}
		if !snowflakeColumnInSelect(column, selectExpr) {
			continue
		}
		// A column inside the UNNEST argument belongs to the outer scope, not the flattened
		// row; only an exact struct-field match (a sibling UNNEST argument) is qualified.
		if ancestor := column.FindAncestor(expressions.KindUnnest, expressions.KindSelect); ancestor != nil &&
			ancestor.Kind() == expressions.KindUnnest {
			if table := columnSource[strings.ToLower(column.Name())]; table != nil {
				column.Set("table", table.Copy())
			}
			continue
		}
		table := columnSource[strings.ToLower(column.Name())]
		if table == nil && unnestIdentifier != nil && sourceCount == 1 && !strings.EqualFold(column.Name(), unnestIdentifier.Name()) {
			table = unnestIdentifier
		}
		if table != nil {
			column.Set("table", table.Copy())
		}
	}
}

// snowflakeColumnInSelect reports whether the column's nearest SELECT ancestor is the one
// being transformed (upstream iterates only the current scope's columns).
func snowflakeColumnInSelect(column, selectExpr expressions.Expression) bool {
	ancestor := column.FindAncestor(expressions.KindSelect)
	return ancestor == selectExpr
}

// snowflakeEliminateDotVariantLookup ports _eliminate_dot_variant_lookup
// (generators/snowflake.py:336-369): a column read off a columns-only UNNEST alias becomes a
// bracketed object lookup, aliased when it is a top-level projection.
func snowflakeEliminateDotVariantLookup(selectExpr expressions.Expression) {
	aliases := map[string]bool{}
	for _, unnest := range selectExpr.FindAll(expressions.KindUnnest) {
		if unnest == nil {
			continue
		}
		if ancestor := unnest.FindAncestor(expressions.KindSelect); ancestor != selectExpr {
			continue
		}
		alias := asExpression(unnest.Arg("alias"))
		if alias == nil || alias.This() != nil {
			continue
		}
		cols := listFromValue(alias.Arg("columns"))
		if len(cols) == 1 {
			aliases[strings.ToLower(asExpression(cols[0]).Name())] = true
		}
	}
	if len(aliases) == 0 {
		return
	}
	for _, column := range selectExpr.FindAll(expressions.KindColumn) {
		tableExpr := asExpression(column.Arg("table"))
		if tableExpr == nil || !aliases[strings.ToLower(tableExpr.Name())] {
			continue
		}
		if !snowflakeColumnInSelect(column, selectExpr) {
			continue
		}
		bracket := expressions.Bracket(expressions.Args{
			"this":        tableExpr.Copy(),
			"expressions": []expressions.Expression{expressions.LiteralString(column.Name())},
		})
		if column.Parent() == selectExpr {
			column.Replace(expressions.AliasExpr(bracket, asExpression(column.Arg("this")).Copy(), false))
		} else {
			column.Replace(bracket)
		}
	}
}

// snowflakeTransformGenerateDateArray ports _transform_generate_date_array and
// _unnest_generate_date_array (generators/snowflake.py:118-199) for a step of one unit.
func snowflakeTransformGenerateDateArray(selectExpr expressions.Expression) {
	for _, series := range selectExpr.FindAll(expressions.KindGenerateDateArray) {
		unnest := series.Parent()
		if unnest == nil || unnest.Kind() != expressions.KindUnnest || len(unnest.Expressions()) != 1 {
			continue
		}
		join := unnest.Parent()
		if join == nil || join.Kind() != expressions.KindJoin {
			continue
		}
		start := asExpression(series.Arg("start"))
		end := asExpression(series.Arg("end"))
		step := asExpression(series.Arg("step"))
		if start == nil || end == nil || step == nil || step.Kind() != expressions.KindInterval || step.Name() != "1" {
			continue
		}
		unit := asExpression(step.Arg("unit"))
		if unit == nil {
			continue
		}
		alias := asExpression(unnest.Arg("alias"))
		var valueName expressions.Expression
		if alias != nil {
			if cols := listFromValue(alias.Arg("columns")); len(cols) > 0 {
				valueName = asExpression(cols[0])
			}
		}
		if valueName == nil {
			valueName = expressions.ToIdentifier("value", false)
		}
		dateAdd := expressions.DateAdd(expressions.Args{
			"this":       expressions.Cast(expressions.Args{"this": start.Copy(), "to": expressions.DTypeDate.IntoExpr(nil)}),
			"expression": expressions.Cast(expressions.Args{"this": valueName.Copy(), "to": expressions.DTypeInt.IntoExpr(nil)}),
			"unit":       unit.Copy(),
		})
		diff := expressions.DateDiff(expressions.Args{
			"this":       end.Copy(),
			"expression": start.Copy(),
			"unit":       unit.Copy(),
		})
		count := expressions.Add(expressions.Args{"this": diff, "expression": expressions.LiteralNumber("1")})
		unnest.Set("expressions", []expressions.Expression{expressions.GenerateSeries(expressions.Args{
			"start":            expressions.LiteralNumber("0"),
			"end":              count,
			"is_end_exclusive": true,
		})})
		replaceName := valueName.Name()
		for _, column := range selectExpr.FindAll(expressions.KindColumn) {
			if column == nil || !strings.EqualFold(column.Name(), replaceName) || !snowflakeColumnInSelect(column, selectExpr) {
				continue
			}
			if column.Parent() == selectExpr {
				column.Replace(expressions.AliasExpr(dateAdd.Copy(), expressions.ToIdentifier(replaceName, false), false))
			} else {
				column.Replace(dateAdd.Copy())
			}
		}
		join.Set("this", expressions.Lateral(expressions.Args{"this": unnest}))
		join.Set("kind", nil)
	}
}
