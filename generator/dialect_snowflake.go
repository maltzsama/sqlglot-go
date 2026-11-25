package generator

import (
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
		expressions.KindArray:                 func(g *Generator, e expressions.Expression) string { return g.inlineArraySQL(e) },
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
	snowflakeExplodeProjectionToUnnest(e)
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

// snowflakeExplodeProjectionToUnnest ports explode_projection_to_unnest (transforms.py:402-
// 552) with the default index_offset=0. A projection whose subtree contains an Explode (even
// inside a nested subquery, which is why the outer SELECT is rewritten for the correlated
// FLATTEN case) becomes a positional IFF over two FLATTEN sources plus a generated series.
// The Posexplode/Aliases branches are not ported: this port has no Posexplode node, so they
// are unreachable here.
func snowflakeExplodeProjectionToUnnest(selectExpr expressions.Expression) {
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

	const indexOffset = 0
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
			renamed := expressions.AliasExpr(selectItem, "", false)
			selectItem.Replace(renamed)
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
		sizeBound := expressions.Sub(expressions.Args{
			"this":       size.Copy(),
			"expression": expressions.LiteralNumber("1"),
		})
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
		end = expressions.Sub(expressions.Args{"this": end, "expression": expressions.LiteralNumber("1")})
		if elements := series.Expressions(); len(elements) > 0 {
			elements[0].Set("end", end)
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
