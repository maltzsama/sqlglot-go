package generator

import (
	"github.com/maltzsama/sqlglot-go/dialects"
	"strconv"
	"strings"

	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// generators/presto.py:303; cross-dialect Select preprocessing is intentionally omitted.
func init() {
	table := map[exp.Kind]func(*Generator, exp.Expression) string{
		exp.KindStructExtract: structExtractSQL,
		exp.KindSelect: func(g *Generator, e exp.Expression) string {
			if g.dialect.Name == "presto" {
				snowflakeInlineNamedWindows(e)
			}
			if asExpression(e.Arg("qualify")) != nil {
				e = eliminateQualify(e)
			}
			explodeProjectionToUnnest(e, 1)
			return g.selectSQL(e)
		},
		exp.KindLateral: prestoLateralSQL,
		exp.KindTable:   prestoTableSQL,
		exp.KindFromISO8601Date: func(g *Generator, e exp.Expression) string {
			return g.funcCall("FROM_ISO8601_DATE", []any{e.This()}, "(", ")", true)
		},
		exp.KindFromISO8601Timestamp: func(g *Generator, e exp.Expression) string {
			return g.funcCall("FROM_ISO8601_TIMESTAMP", []any{e.This()}, "(", ")", true)
		},
		exp.KindFromISO8601TimestampNanos: func(g *Generator, e exp.Expression) string {
			return g.funcCall("FROM_ISO8601_TIMESTAMP_NANOS", []any{e.This()}, "(", ")", true)
		},
		exp.KindTimeToStr: (*Generator).prestoTimeToStrSQL,
		exp.KindStrToTime: (*Generator).prestoStrToTimeSQL,
		exp.KindStrToDate: func(g *Generator, e exp.Expression) string { return "CAST(" + g.prestoStrToTimeSQL(e) + " AS DATE)" },
		exp.KindTimestampTrunc: func(g *Generator, e exp.Expression) string {
			return g.funcCall("DATE_TRUNC", []any{prestoUnit(e), e.Arg("this")}, "(", ")", true)
		},
		exp.KindRegexpExtract:    (*Generator).prestoRegexpExtractSQL,
		exp.KindRegexpExtractAll: (*Generator).prestoRegexpExtractSQL,
		exp.KindApproxQuantile:   prestoCall("APPROX_PERCENTILE", "this", "weight", "quantile", "accuracy"),
		exp.KindDateAdd:          (*Generator).prestoDateAddSQL,
		exp.KindDateDiff: func(g *Generator, e exp.Expression) string {
			return g.funcCall("DATE_DIFF", []any{prestoUnit(e), e.Arg("expression"), e.Arg("this")}, "(", ")", true)
		},
		exp.KindTsOrDsDiff: func(g *Generator, e exp.Expression) string {
			return g.funcCall("DATE_DIFF", []any{prestoUnit(e),
				prestoCastToTimestamp(asExpression(e.Arg("expression"))),
				prestoCastToTimestamp(e.This())}, "(", ")", true)
		},
		exp.KindTimestampAdd: func(g *Generator, e exp.Expression) string {
			return "DATE_ADD('" + strings.ToUpper(asExpression(e.Arg("unit")).Name()) + "', " +
				g.sqlKey(e, "expression") + ", " + g.sqlKey(e, "this") + ")"
		},
		exp.KindDayOfWeek: func(g *Generator, e exp.Expression) string {
			return "((DAY_OF_WEEK(" + g.sqlKey(e, "this") + ") % 7) + 1)"
		},
		exp.KindIs: prestoIsSQL,
		exp.KindLeft: func(g *Generator, e exp.Expression) string {
			return g.funcCall("SUBSTR", []any{e.This(), "1", e.Arg("expression")}, "(", ")", true)
		},
		exp.KindRight: func(g *Generator, e exp.Expression) string {
			return "SUBSTR(" + g.sqlKey(e, "this") + ", LENGTH(" + g.sqlKey(e, "this") + ") - (" +
				g.sqlKey(e, "expression") + " - 1))"
		},
		exp.KindExtract:           prestoExtractSQL,
		exp.KindDecode:            prestoCall("FROM_UTF8", "this", "replace"),
		exp.KindEncode:            prestoCall("TO_UTF8", "this", "replace"),
		exp.KindUnixToTime:        prestoUnixToTimeSQL,
		exp.KindSortArray:         prestoSortArraySQL,
		exp.KindArrayToString:     prestoArrayToStringSQL,
		exp.KindQuantile:          prestoCall("APPROX_PERCENTILE", "this", "quantile"),
		exp.KindLastDay:           prestoCall("LAST_DAY_OF_MONTH", "this"),
		exp.KindGenerateSeries:    prestoGenerateSeriesSQL,
		exp.KindGenerateDateArray: sequenceSQL,
		// generators/presto.py:393 exp.StrPosition: strposition_sql(supports_occurrence=True).
		exp.KindStrPosition: strPositionTranspileSQL("STRPOS", false, true),
		// generators/presto.py:321 exp.ArrayConcat: rename_func("CONCAT").
		exp.KindArrayConcat: renameFunc("CONCAT"),
		exp.KindIf:          prestoCall("IF", "this", "true", "false"),
		exp.KindAtTimeZone:  prestoCall("AT_TIMEZONE", "this", "zone"),
		exp.KindDataType:    (*Generator).prestoDataTypeSQL,
		exp.KindStruct:      (*Generator).prestoStructSQL,
		exp.KindBracket:     (*Generator).prestoBracketSQL,
		exp.KindInterval:    (*Generator).prestoIntervalSQL,
		exp.KindTransaction: (*Generator).prestoTransactionSQL,
		exp.KindGroupConcat: func(g *Generator, e exp.Expression) string {
			return g.funcCall("ARRAY_JOIN", []any{g.funcCall("ARRAY_AGG", []any{e.Arg("this")}, "(", ")", true), e.Arg("separator")}, "(", ")", true)
		},
		exp.KindSqlSecurityProperty:   func(g *Generator, e exp.Expression) string { return "SECURITY " + g.sqlKey(e, "this") },
		exp.KindSchemaCommentProperty: func(g *Generator, e exp.Expression) string { return "COMMENT " + g.sqlKey(e, "this") },
		exp.KindHex: func(g *Generator, e exp.Expression) string {
			return g.funcCall("TO_HEX", []any{e.Arg("this")}, "(", ")", true)
		},
		exp.KindSchema:             (*Generator).prestoSchemaSQL,
		exp.KindFileFormatProperty: func(g *Generator, e exp.Expression) string { return "format=" + g.gen(exp.LiteralString(e.Name())) },
		exp.KindSHA2Digest:         (*Generator).prestoSHA2DigestSQL,
		exp.KindSHA2:               (*Generator).prestoSHA2SQL,
		exp.KindFirst:              (*Generator).prestoFirstLastSQL,
		exp.KindLast:               (*Generator).prestoFirstLastSQL,
		exp.KindDateStrToDate:      prestoCast("DATE"),
		exp.KindDateToDateStr:      prestoCast("VARCHAR"),
		exp.KindDateToDi: func(g *Generator, e exp.Expression) string {
			return "CAST(DATE_FORMAT(" + g.sqlKey(e, "this") + ", '%Y%m%d') AS INT)"
		},
		exp.KindDiToDate: func(g *Generator, e exp.Expression) string {
			return "CAST(DATE_PARSE(CAST(" + g.sqlKey(e, "this") + " AS VARCHAR), '%Y%m%d') AS DATE)"
		},
		exp.KindTimeStrToTime: prestoTimeStrToTimeSQL,
		exp.KindTimeToTimeStr: prestoCast("VARCHAR"),
		exp.KindStrToUnix:     prestoStrToUnixSQL,
		exp.KindTimeStrToUnix: func(g *Generator, e exp.Expression) string {
			return "TO_UNIXTIME(DATE_PARSE(" + g.sqlKey(e, "this") + ", '%Y-%m-%d %T'))"
		},
		exp.KindTsOrDsToDate: func(g *Generator, e exp.Expression) string {
			if format := asExpression(e.Arg("format")); format != nil {
				if stripStrictTimeFormat(format.Name()) != "%Y-%m-%d" {
					return g.tsOrDsToDateSQL(e)
				}
			}
			return "CAST(CAST(" + g.sqlKey(e, "this") + " AS TIMESTAMP) AS DATE)"
		},
		exp.KindTsOrDiToDi: func(g *Generator, e exp.Expression) string {
			return "CAST(SUBSTR(REPLACE(CAST(" + g.sqlKey(e, "this") + " AS VARCHAR), '-', ''), 1, 8) AS INT)"
		},
		exp.KindTsOrDsAdd: func(g *Generator, e exp.Expression) string {
			unit := asExpression(e.Arg("unit"))
			unitSQL := ""
			if unit != nil {
				unitSQL = "'" + strings.ToUpper(unit.Name()) + "'"
			}
			// _ts_or_ds_add_sql + ts_or_ds_add_cast (generators/presto.py:111-114,
			// dialects/dialect.py:1984-1994): the operand is cast to the node's return
			// type (DATE by default), going through TIMESTAMP only for a DATE result.
			returnType := asExpression(e.Arg("return_type"))
			if returnType == nil {
				returnType = exp.DTypeDate.IntoExpr(nil)
			}
			source := asExpression(e.This())
			if expIsType(returnType, exp.DTypeDate) {
				source = exp.Cast(exp.Args{"this": source, "to": exp.DTypeTimestamp.IntoExpr(nil)})
			}
			value := exp.Cast(exp.Args{"this": source, "to": returnType})
			return g.funcCall("DATE_ADD", []any{unitSQL, e.Arg("expression"), value}, "(", ")", true)
		},
		exp.KindUnixToStr: func(g *Generator, e exp.Expression) string {
			return "DATE_FORMAT(FROM_UNIXTIME(" + g.sqlKey(e, "this") + "), " + g.gen(prestoFormatTime(e)) + ")"
		},
		exp.KindUnixToTimeStr: func(g *Generator, e exp.Expression) string {
			return "CAST(FROM_UNIXTIME(" + g.sqlKey(e, "this") + ") AS VARCHAR)"
		},
		exp.KindTsOrDsToDateStr: func(g *Generator, e exp.Expression) string {
			return "SUBSTR(CAST(" + g.sqlKey(e, "this") + " AS VARCHAR), 1, 10)"
		},
		exp.KindTimeStrToDate: func(g *Generator, e exp.Expression) string {
			return "CAST(" + g.sqlKey(e, "this") + " AS TIMESTAMP)"
		},
		exp.KindArrayFilter: renameFunc("FILTER"),
		exp.KindLowerHex: func(g *Generator, e exp.Expression) string {
			return "LOWER(TO_HEX(" + g.sqlKey(e, "this") + "))"
		},
		exp.KindMD5: func(g *Generator, e exp.Expression) string {
			return "LOWER(TO_HEX(MD5(" + g.sqlKey(e, "this") + ")))"
		},
		exp.KindSHA1Digest: func(g *Generator, e exp.Expression) string {
			return "SHA1(" + g.sqlKey(e, "this") + ")"
		},
		exp.KindSHA: func(g *Generator, e exp.Expression) string {
			return "SHA1(" + g.sqlKey(e, "this") + ")"
		},
		exp.KindUuid: func(g *Generator, e exp.Expression) string {
			if truthy(e.Arg("is_string")) || e.Arg("name") != nil {
				return "CAST(UUID() AS VARCHAR)"
			}
			return "UUID()"
		},
		exp.KindTimestamp: prestoTimestampSQL,
		exp.KindTimestampDiff: func(g *Generator, e exp.Expression) string {
			return "DATE_DIFF('" + prestoUnitName(e) + "', " + g.sqlKey(e, "expression") + ", " + g.sqlKey(e, "this") + ")"
		},
		exp.KindDatetimeDiff: prestoDatetimeDiffSQL,
		exp.KindArraySize: func(g *Generator, e exp.Expression) string {
			return "CARDINALITY(" + g.sqlKey(e, "this") + ")"
		},
		exp.KindWithinGroup: prestoWithinGroupSQL,
		exp.KindDelete:      prestoDeleteSQL,
		exp.KindCast:        prestoCastSQL,
		exp.KindConcat:      prestoConcatFunctionSQL,
		exp.KindDPipe: func(g *Generator, e exp.Expression) string {
			if !truthy(e.Arg("safe")) {
				return g.binary(e, "||")
			}
			return "CONCAT(CAST(" + g.sqlKey(e, "this") + " AS VARCHAR), CAST(" + g.sqlKey(e, "expression") + " AS VARCHAR))"
		},
		exp.KindJSONFormat: func(g *Generator, e exp.Expression) string {
			this := asExpression(e.This())
			if this == nil {
				return g.funcCall("JSON_FORMAT", []any{this, e.Arg("options")}, "(", ")", true)
			}
			if !truthy(e.Arg("is_json")) {
				dtype, known := castResultDType(this)
				if this.Kind() != exp.KindCast || !known || dtype != exp.DTypeJSON {
					this = exp.Cast(exp.Args{"this": this, "to": exp.DTypeJSON.IntoExpr(nil)})
				}
			}
			return g.funcCall("JSON_FORMAT", []any{this, e.Arg("options")}, "(", ")", true)
		},
		exp.KindArrayAny: renameFunc("ANY_MATCH"),
		exp.KindCreate:   (*Generator).prestoCreateSQL,
		exp.KindWith:     prestoWithSQL,
		exp.KindJSONExtract: func(g *Generator, e exp.Expression) string {
			args := []any{e.Arg("this"), e.Arg("expression")}
			for _, x := range e.Expressions() {
				args = append(args, x)
			}
			return g.funcCall("JSON_EXTRACT", args, "(", ")", true)
		},
		exp.KindILike: func(g *Generator, e exp.Expression) string {
			op := "LIKE"
			if boolValue(e.Arg("negate")) {
				op = "NOT LIKE"
			}
			return "LOWER(" + g.sqlKey(e, "this") + ") " + op + " LOWER(" + g.sqlKey(e, "expression") + ")"
		},
		exp.KindInitcap: func(g *Generator, e exp.Expression) string {
			// presto.py:50-58.
			if d := asExpression(e.Arg("expression")); d != nil && (!d.IsString() || d.Name() != g.dialect.InitcapDefaultDelimiterChars) {
				g.unsupported("INITCAP does not support custom delimiters")
			}
			return "REGEXP_REPLACE(" + g.sqlKey(e, "this") + ", '(\\w)(\\w*)', x -> UPPER(x[1]) || LOWER(x[2]))"
		},
		exp.KindXor: func(g *Generator, e exp.Expression) string {
			a, b := g.sqlKey(e, "this"), g.sqlKey(e, "expression")
			return "(" + a + " AND (NOT " + b + ")) OR ((NOT " + a + ") AND " + b + ")"
		},
	}
	for kind, name := range map[exp.Kind]string{
		exp.KindAnyValue: "ARBITRARY", exp.KindArrayContains: "CONTAINS", exp.KindArrayUniqueAgg: "SET_AGG", exp.KindArraySlice: "SLICE",
		exp.KindBitwiseAnd: "BITWISE_AND", exp.KindBitwiseOr: "BITWISE_OR", exp.KindBitwiseXor: "BITWISE_XOR", exp.KindBitwiseNot: "BITWISE_NOT", exp.KindBitwiseLeftShift: "BITWISE_LEFT_SHIFT", exp.KindBitwiseRightShift: "BITWISE_RIGHT_SHIFT",
		exp.KindDayOfWeekIso: "DAY_OF_WEEK", exp.KindLogicalOr: "BOOL_OR", exp.KindStrToMap: "SPLIT_TO_MAP", exp.KindTrunc: "TRUNCATE", exp.KindVariancePop: "VAR_POP", exp.KindLevenshtein: "LEVENSHTEIN_DISTANCE", exp.KindUnhex: "FROM_HEX", exp.KindTimeToUnix: "TO_UNIXTIME", exp.KindMD5Digest: "MD5", exp.KindSubstring: "SUBSTR",
	} {
		table[kind] = prestoRename(name)
	}
	// generators/presto.py:334-336; Localtime/Localtimestamp keep the base handlers, which
	// preserve a precision argument (generator.py:6262-6268).
	for kind, name := range map[exp.Kind]string{exp.KindCurrentTime: "CURRENT_TIME", exp.KindCurrentTimestamp: "CURRENT_TIMESTAMP", exp.KindCurrentUser: "CURRENT_USER"} {
		table[kind] = func(_ *Generator, _ exp.Expression) string { return name }
	}
	registerDialectDispatch("presto", table)
	registerDialectTypeMapping("presto", map[exp.DType]string{exp.DTypeBinary: "VARBINARY", exp.DTypeBit: "BOOLEAN", exp.DTypeDatetime: "TIMESTAMP", exp.DTypeDatetime64: "TIMESTAMP", exp.DTypeFloat: "REAL", exp.DTypeHllSketch: "HYPERLOGLOG", exp.DTypeInt: "INTEGER", exp.DTypeStruct: "ROW", exp.DTypeText: "VARCHAR", exp.DTypeTimestampTz: "TIMESTAMP", exp.DTypeTimestampNtz: "TIMESTAMP", exp.DTypeTimeTz: "TIME"})
	// generators/presto.py:285 exp.VolatileProperty: UNSUPPORTED.
	registerDialectPropertyLocations("presto", map[exp.Kind]propertyLocation{
		exp.KindLocationProperty: propertyLocationUnsupported,
		exp.KindVolatileProperty: propertyLocationUnsupported,
	})
}

func prestoRename(name string) func(*Generator, exp.Expression) string {
	return func(g *Generator, e exp.Expression) string {
		return g.funcCall(name, g.fallbackArgs(e), "(", ")", true)
	}
}
func prestoCall(name string, keys ...string) func(*Generator, exp.Expression) string {
	return func(g *Generator, e exp.Expression) string {
		args := make([]any, 0, len(keys))
		for _, key := range keys {
			args = append(args, e.Arg(key))
		}
		return g.funcCall(name, args, "(", ")", true)
	}
}

// generators/presto.py:288-301 and generator.py datatype_sql.
func (g *Generator) prestoDataTypeSQL(e exp.Expression) string {
	if exp.IsType(e, exp.DTypeTimestampTz) || exp.IsType(e, exp.DTypeTimeTz) {
		return g.dataTypeSQL(e) + " WITH TIME ZONE"
	}
	if exp.IsType(e, exp.DTypeArray) || exp.IsType(e, exp.DTypeMap) || exp.IsType(e, exp.DTypeStruct) {
		c := e.Copy()
		c.Set("nested", false)
		return g.dataTypeSQL(c)
	}
	return g.dataTypeSQL(e)
}

// generators/presto.py:588-620. Named fields need their types (upstream annotate_types); the
// port infers them for literal values and reports anything else unsupported.
func (g *Generator) prestoStructSQL(e exp.Expression) string {
	values := []string{}
	schema := []string{}
	unknown := false
	for _, x := range e.Expressions() {
		if x.Kind() == exp.KindPropertyEQ {
			value := asExpression(x.Arg("expression"))
			if dtype := prestoLiteralType(value); dtype != "" {
				schema = append(schema, g.sqlKey(x, "this")+" "+dtype)
			} else {
				unknown = true
			}
			values = append(values, g.gen(value))
		} else {
			values = append(values, g.gen(x))
		}
	}
	if len(values) == 0 || len(schema) != len(values) {
		if unknown {
			g.unsupported("Cannot convert untyped key-value definitions (try annotate_types).")
		}
		return "ROW(" + strings.Join(values, ", ") + ")"
	}
	return "CAST(ROW(" + strings.Join(values, ", ") + ") AS ROW(" + strings.Join(schema, ", ") + "))"
}

func prestoLiteralType(e exp.Expression) string {
	if e == nil {
		return ""
	}
	switch e.Kind() {
	case exp.KindLiteral:
		if e.IsString() {
			return "VARCHAR"
		}
		if strings.ContainsAny(e.Name(), ".eE") {
			return "DOUBLE"
		}
		return "INTEGER"
	case exp.KindBoolean:
		return "BOOLEAN"
	case exp.KindNeg:
		return prestoLiteralType(e.This())
	}
	return ""
}

// generator.py:3678-3680 with SUPPORTS_SINGLE_ARG_CONCAT = False (presto.py:271).

func prestoUnitName(e exp.Expression) string {
	if unit := asExpression(e.Arg("unit")); unit != nil {
		return strings.ToUpper(unit.Name())
	}
	return "DAY"
}

// prestoDatetimeDiffSQL ports the Presto DATETIME_DIFF form: a truncated date diff, with a
// one-day shift so WEEK boundaries match BigQuery.
func prestoDatetimeDiffSQL(g *Generator, e exp.Expression) string {
	unitExpr := asExpression(e.Arg("unit"))
	unit := prestoUnitName(e)
	shift := unit == "WEEK"
	if unitExpr != nil && unitExpr.Kind() == exp.KindWeekStart {
		unit = "WEEK"
		shift = true
	}
	makeArg := func(v exp.Expression) string {
		sql := g.gen(v)
		if dtype, ok := castResultDType(v); !ok || dtype != exp.DTypeTimestamp {
			sql = "CAST(" + sql + " AS TIMESTAMP)"
		}
		if shift {
			sql = sql + " + INTERVAL '1' DAY"
		}
		return "DATE_TRUNC('" + unit + "', " + sql + ")"
	}
	return "DATE_DIFF('" + unit + "', " + makeArg(asExpression(e.Arg("expression"))) + ", " + makeArg(asExpression(e.This())) + ")"
}

func prestoWithinGroupSQL(g *Generator, e exp.Expression) string {
	funcExpr := asExpression(e.This())
	order := asExpression(e.Arg("expression"))
	if funcExpr != nil && funcExpr.Kind() == exp.KindPercentileCont && order != nil {
		if ordered := order.Find(exp.KindOrdered); ordered != nil {
			return g.funcCall("APPROX_PERCENTILE", []any{ordered.This(), funcExpr.This()}, "(", ")", true)
		}
	}
	return g.withinGroupSQL(e)
}

// prestoDeleteSQL drops the target alias and column qualifiers, which Presto rejects.
func prestoDeleteSQL(g *Generator, e exp.Expression) string {
	e = e.Copy()
	if asExpression(e.This()) == nil {
		if tables, _ := e.Arg("tables").([]exp.Expression); len(tables) > 0 {
			e.Set("this", tables[0])
			e.Set("tables", nil)
		}
	}
	hadAlias := false
	if table := asExpression(e.This()); table != nil {
		if table.Arg("alias") != nil {
			hadAlias = true
			table.Set("alias", nil)
		}
	}
	// Presto only needs the qualifiers removed when the alias they referred to was dropped.
	if hadAlias {
		if where := asExpression(e.Arg("where")); where != nil {
			for _, column := range where.FindAll(exp.KindColumn) {
				if column != nil {
					column.Set("table", nil)
					column.Set("schema", nil)
					column.Set("catalog", nil)
				}
			}
		}
	}
	return g.deleteSQL(e)
}

// prestoCastSQL ports epoch_cast_to_ts (transforms.py:603-611) for a literal 'epoch'.
func prestoCastSQL(g *Generator, e exp.Expression) string {
	if this := asExpression(e.This()); this != nil && this.IsString() && strings.EqualFold(this.Name(), "epoch") {
		if to := asExpression(e.Arg("to")); to != nil {
			if dtype, ok := to.Arg("this").(exp.DType); ok {
				switch dtype {
				case exp.DTypeDate, exp.DTypeDatetime, exp.DTypeTimestamp, exp.DTypeTimestampTz, exp.DTypeTimestampNtz:
					e = e.Copy()
					e.Set("this", exp.LiteralString("1970-01-01 00:00:00"))
				}
			}
		}
	}
	return g.castSQL(e)
}

func prestoConcatFunctionSQL(g *Generator, e exp.Expression) string {
	if !truthy(e.Arg("safe")) && !truthy(e.Arg("coalesce")) {
		return g.concatSQL(e)
	}
	args := e.Expressions()
	if len(args) == 1 {
		return "CAST(" + g.gen(args[0]) + " AS VARCHAR)"
	}
	parts := make([]string, 0, len(args))
	for _, arg := range args {
		parts = append(parts, "COALESCE(CAST("+g.gen(arg)+" AS VARCHAR), '')")
	}
	return "CONCAT(" + strings.Join(parts, ", ") + ")"
}

// prestoGenerateSeriesSQL renders SEQUENCE wrapped in UNNEST, with a normalized interval step.
func prestoGenerateSeriesSQL(g *Generator, e exp.Expression) string {
	start := asExpression(e.Arg("start"))
	end := asExpression(e.Arg("end"))
	step := asExpression(e.Arg("step"))
	castTimestamp := false
	for _, v := range []exp.Expression{start, end} {
		if v == nil {
			continue
		}
		if v.Kind() == exp.KindCurrentTimestamp {
			castTimestamp = true
		}
		if dtype, ok := castResultDType(v); ok && (dtype == exp.DTypeTimestamp || dtype == exp.DTypeDate) {
			castTimestamp = true
		}
	}
	argSQL := func(v exp.Expression) string {
		if v == nil {
			return ""
		}
		if dtype, ok := castResultDType(v); ok && dtype == exp.DTypeTimestamp {
			return g.gen(v)
		}
		if dtype, ok := castResultDType(v); ok && dtype == exp.DTypeDate {
			return "CAST(" + g.gen(v) + " AS TIMESTAMP)"
		}
		if castTimestamp {
			return "CAST(" + g.gen(v) + " AS TIMESTAMP)"
		}
		return g.gen(v)
	}
	if truthy(e.Arg("is_end_exclusive")) {
		stepValue := asExpression(e.Arg("step"))
		if stepValue == nil {
			stepValue = exp.LiteralNumber("1")
		}
		endInt, endOK := hiveIntLiteral(end)
		stepInt, stepOK := hiveIntLiteral(stepValue)
		if endOK && stepOK {
			end = exp.LiteralNumber(strconv.Itoa(endInt - stepInt))
		} else {
			end = exp.Paren(exp.Args{"this": exp.Sub(exp.Args{"this": end, "expression": stepValue})})
		}
	}
	args := []string{argSQL(start), argSQL(end)}
	if step != nil {
		if step.Kind() == exp.KindLiteral && step.IsString() {
			args = append(args, prestoIntervalFromString(step.Name()))
		} else {
			args = append(args, g.gen(step))
		}
	}
	series := "SEQUENCE(" + strings.Join(args, ", ") + ")"
	if !truthy(e.Arg("exploding")) {
		return series
	}
	if parent := e.Parent(); parent != nil && parent.Kind() == exp.KindUnnest {
		return series
	}
	return "UNNEST(" + series + ")"
}

func prestoTimestampSQL(g *Generator, e exp.Expression) string {
	zone := asExpression(e.Arg("zone"))
	if zone == nil {
		return "CAST(" + g.sqlKey(e, "this") + " AS TIMESTAMP WITH TIME ZONE)"
	}
	if duckDBIsTimezoneName(zone.Name()) {
		return "CAST(CAST(" + g.sqlKey(e, "this") + " AS TIMESTAMP) AS TIMESTAMP) AT TIME ZONE " + g.sqlKey(e, "zone")
	}
	return g.funcCall("TIMESTAMP", []any{e.This(), zone}, "(", ")", true)
}

// prestoIntervalFromString converts a Postgres step such as '  2   days  ' to INTERVAL '2' DAY.
func prestoIntervalFromString(text string) string {
	trimmed := strings.TrimSpace(text)
	digits := ""
	for len(trimmed) > 0 && (trimmed[0] == '-' || trimmed[0] == '+' || (trimmed[0] >= '0' && trimmed[0] <= '9')) {
		digits += string(trimmed[0])
		trimmed = trimmed[1:]
	}
	unit := strings.ToUpper(strings.TrimSpace(trimmed))
	unit = strings.TrimSuffix(unit, "S")
	if digits == "" {
		return "'" + text + "'"
	}
	return "INTERVAL '" + digits + "' " + unit
}

// prestoWithSQL ports the With preprocess for recursive CTE column aliases.
func prestoWithSQL(g *Generator, e exp.Expression) string {
	if !truthy(e.Arg("recursive")) {
		return g.withSQL(e)
	}
	e = e.Copy()
	next := 0
	for _, cte := range e.Expressions() {
		if cte == nil || cte.Kind() != exp.KindCTE {
			continue
		}
		alias := asExpression(cte.Arg("alias"))
		if alias == nil {
			continue
		}
		if cols := listFromValue(alias.Arg("columns")); len(cols) > 0 {
			continue
		}
		query := asExpression(cte.This())
		for query != nil && query.Is(exp.TraitSetOperation) {
			query = query.This()
			if query != nil {
				query = query.Unnest()
			}
		}
		if query == nil || query.Kind() != exp.KindSelect {
			continue
		}
		columns := make([]exp.Expression, 0, len(query.Expressions()))
		for _, projection := range query.Expressions() {
			name := ""
			if projection != nil {
				name = projection.OutputName()
			}
			switch projection.Kind() {
			case exp.KindAlias, exp.KindColumn, exp.KindIdentifier, exp.KindLiteral:
			default:
				name = ""
			}
			if name == "" {
				name = "_c_" + strconv.Itoa(next)
				next++
			}
			columns = append(columns, exp.ToIdentifier(name, false))
		}
		alias.Set("columns", columns)
	}
	return g.withSQL(e)
}

// presto.py:639-648: Presto has no CREATE VIEW column list.
func (g *Generator) prestoCreateSQL(e exp.Expression) string {
	if e.Text("kind") == "VIEW" {
		if schema := e.This(); schema != nil && schema.Kind() == exp.KindSchema && len(schema.Expressions()) > 0 {
			e = e.Copy()
			e.This().Set("expressions", nil)
		}
	}
	return g.createSQL(e)
}

// generators/presto.py:583-586; parser Bracket offsets already match Presto's one-based indexing.
func (g *Generator) prestoBracketSQL(e exp.Expression) string {
	if boolValue(e.Arg("safe")) {
		args := []any{e.Arg("this")}
		offset := g.dialect.IndexOffset
		if v, ok := e.Arg("offset").(int); ok {
			offset -= v
		}
		if xs := e.Expressions(); len(xs) > 0 {
			index := xs[0]
			if value, ok := bracketIndexValue(index); ok {
				index = exp.LiteralNumber(strconv.FormatInt(value+int64(offset), 10))
			}
			args = append(args, index)
		}
		return g.funcCall("ELEMENT_AT", args, "(", ")", true)
	}
	return g.bracketSQL(e)
}

// generators/presto.py:622-625.
func (g *Generator) prestoIntervalSQL(e exp.Expression) string {
	if e.This() != nil && strings.HasPrefix(strings.ToUpper(e.Text("unit")), "WEEK") {
		return "(" + e.This().Name() + " * INTERVAL '7' DAY)"
	}
	return g.intervalSQL(e)
}

// generators/presto.py:627-630.
func (g *Generator) prestoTransactionSQL(e exp.Expression) string {
	modes := []string{}
	switch v := e.Arg("modes").(type) {
	case []string:
		modes = v
	case []any:
		for _, x := range v {
			modes = append(modes, g.gen(x))
		}
	case []exp.Expression:
		for _, x := range v {
			modes = append(modes, g.gen(x))
		}
	}
	if len(modes) > 0 {
		return "START TRANSACTION " + strings.Join(modes, ", ")
	}
	return "START TRANSACTION"
}

// generators/presto.py:67-89.
func (g *Generator) prestoSchemaSQL(e exp.Expression) string {
	if parent := e.Parent(); parent != nil && parent.Kind() == exp.KindPartitionedByProperty {
		// Partition column names go into ARRAY[] string literals unquoted.
		args := []string{}
		for _, x := range e.Expressions() {
			var name string
			switch {
			case x.Is(exp.TraitFunc) || x.Kind() == exp.KindProperty:
				name = g.gen(x)
			case x.Kind() == exp.KindIdentifier:
				name = x.Name()
			default:
				this := asExpression(x.Arg("this"))
				if this != nil && this.Kind() == exp.KindIdentifier {
					name = this.Name()
				} else {
					name = g.sqlKey(x, "this")
				}
			}
			args = append(args, g.gen(exp.LiteralString(name)))
		}
		return "ARRAY[" + strings.Join(args, ", ") + "]"
	}
	if parent := e.Parent(); parent != nil {
		// The partition columns must also exist on the table: fold PARTITIONED_BY (col TYPE, ...)
		// ColumnDefs into the table schema.
		for _, schema := range parent.FindAll(exp.KindSchema) {
			if schema == e {
				continue
			}
			if sp := schema.Parent(); sp != nil && sp.Kind() == exp.KindPartitionedByProperty {
				for _, def := range schema.FindAll(exp.KindColumnDef) {
					e.Append("expressions", def.Copy())
				}
			}
		}
	}
	return g.schemaSQL(e)
}

// generators/presto.py:147-169 _to_int: the interval is cast to BIGINT unless annotate_types
// proves it integer. Without the full annotator this mirrors its literal/arithmetic/known-
// function rules and casts everything else (columns, unknown calls), like upstream.
func (g *Generator) prestoDateAddSQL(e exp.Expression) string {
	interval := e.Arg("expression")
	if iv := asExpression(interval); iv != nil && !prestoIntegerTyped(iv) {
		interval = "CAST(" + g.gen(interval) + " AS BIGINT)"
	}
	return g.funcCall("DATE_ADD", []any{prestoUnit(e), interval, e.Arg("this")}, "(", ")", true)
}

func prestoIntegerTyped(e exp.Expression) bool {
	switch e.Kind() {
	case exp.KindLiteral:
		return !e.IsString() && !strings.ContainsAny(e.Name(), ".eE")
	case exp.KindNeg, exp.KindParen, exp.KindFloor:
		// FLOOR(x) types as BIGINT for any numeric x (annotate_types FLOOR rule).
		return e.Kind() == exp.KindFloor || prestoIntegerTyped(e.This())
	case exp.KindAdd, exp.KindSub, exp.KindMul, exp.KindMod:
		return prestoIntegerTyped(e.This()) && prestoIntegerTyped(asExpression(e.Arg("expression")))
	case exp.KindCoalesce:
		if !prestoIntegerTyped(e.This()) {
			return false
		}
		for _, x := range e.Expressions() {
			if !prestoIntegerTyped(x) {
				return false
			}
		}
		return true
	case exp.KindCast, exp.KindTryCast:
		to := asExpression(e.Arg("to"))
		if to == nil {
			return false
		}
		dt, ok := to.Arg("this").(exp.DType)
		return ok && exp.IntegerTypes[dt]
	case exp.KindLength, exp.KindCount, exp.KindDayOfWeekIso, exp.KindDayOfYear, exp.KindWeekOfYear, exp.KindDayOfMonth, exp.KindDayOfWeek, exp.KindTimeToUnix:
		return true
	}
	return false
}

// prestoSHA2DigestSQL ports _sha2_digest_sql (generators/presto.py:37-46).
func (g *Generator) prestoSHA2DigestSQL(e exp.Expression) string {
	length, this := g.prestoSHA2Args(e)
	return g.funcCall("SHA"+length, []any{this}, "(", ")", true)
}

// prestoSHA2SQL ports sha2_sql (generators/presto.py:553-563): the hex-string form.
func (g *Generator) prestoSHA2SQL(e exp.Expression) string {
	length, this := g.prestoSHA2Args(e)
	return g.funcCall("LOWER", []any{g.funcCall("TO_HEX", []any{g.funcCall("SHA"+length, []any{this}, "(", ")", true)}, "(", ")", true)}, "(", ")", true)
}

func (g *Generator) prestoSHA2Args(e exp.Expression) (string, any) {
	length := e.Text("length")
	if length == "" {
		length = "256"
	}
	if length != "256" && length != "512" {
		g.unsupported("SHA" + length + " is not supported in Presto")
	}
	this := e.Arg("this")
	// The digest takes VARBINARY: a text-typed argument (a CAST to a text type, the only case
	// where upstream sees a type without annotate_types) is encoded first.
	if cast := asExpression(this); cast != nil && (cast.Kind() == exp.KindCast || cast.Kind() == exp.KindTryCast) {
		if to := asExpression(cast.Arg("to")); to != nil {
			if dt, ok := to.Arg("this").(exp.DType); ok && exp.TextTypes[dt] {
				this = g.funcCall("TO_UTF8", []any{cast}, "(", ")", true)
			}
		}
	}
	return length, this
}

// prestoFirstLastSQL ports _first_last_sql (generators/presto.py:124-135): FIRST/LAST survive
// only under MATCH_RECOGNIZE, elsewhere they become ARBITRARY.
func (g *Generator) prestoFirstLastSQL(e exp.Expression) string {
	if ancestor := e.FindAncestor(exp.KindMatchRecognize, exp.KindSelect); ancestor != nil && ancestor.Kind() == exp.KindMatchRecognize {
		return g.functionFallbackSQL(e)
	}
	return g.funcCall("ARBITRARY", g.fallbackArgs(e), "(", ")", true)
}

// dialects/dialect.py:2059-2066.
func prestoUnit(e exp.Expression) exp.Expression {
	unit := asExpression(e.Arg("unit"))
	if unit == nil {
		return exp.LiteralString("DAY")
	}
	if unit.Kind() == exp.KindLiteral || unit.Kind() == exp.KindVar {
		return exp.LiteralString(strings.ToUpper(unit.Name()))
	}
	return unit
}

func prestoFormatTime(e exp.Expression) any {
	format := asExpression(e.Arg("format"))
	if format != nil && format.IsString() {
		name := format.Name()
		if strings.Contains(name, "strict") {
			name = prestoStrictTimeFormat.Replace(name)
		}
		return exp.LiteralString(dialects.PrestoFormatTime(name))
	}
	return format
}

func (g *Generator) prestoTimeToStrSQL(e exp.Expression) string {
	return g.funcCall("DATE_FORMAT", []any{e.Arg("this"), prestoFormatTime(e)}, "(", ")", true)
}

func (g *Generator) prestoStrToTimeSQL(e exp.Expression) string {
	return g.funcCall("DATE_PARSE", []any{e.Arg("this"), prestoFormatTime(e)}, "(", ")", true)
}

func (g *Generator) prestoRegexpExtractSQL(e exp.Expression) string {
	group := asExpression(e.Arg("group"))
	if group != nil && group.Name() == "0" {
		group = nil
	}
	name := "REGEXP_EXTRACT"
	if e.Kind() == exp.KindRegexpExtractAll {
		name = "REGEXP_EXTRACT_ALL"
	}
	return g.funcCall(name, []any{e.Arg("this"), e.Arg("expression"), group}, "(", ")", true)
}

// generators/presto.py:314-315 exp.ArgMax/exp.ArgMin: MAX_BY/MIN_BY.
func init() {
	registerDialectDispatch("presto", map[exp.Kind]func(*Generator, exp.Expression) string{
		exp.KindArgMax: renameFunc("MAX_BY"),
		exp.KindArgMin: renameFunc("MIN_BY"),
	})
}

// prestoUnixToTimeSQL ports the Presto FROM_UNIXTIME transform for a scaled epoch.
func prestoUnixToTimeSQL(g *Generator, e exp.Expression) string {
	if scale := asExpression(e.Arg("scale")); scale != nil {
		ts := exp.Div(exp.Args{
			"this":       exp.Cast(exp.Args{"this": e.This(), "to": exp.DTypeDouble.IntoExpr(nil)}),
			"expression": exp.Anonymous(exp.Args{"this": "POW", "expressions": []exp.Expression{exp.LiteralNumber("10"), scale}}),
		})
		return g.funcCall("FROM_UNIXTIME", []any{ts}, "(", ")", true)
	}
	return prestoCall("FROM_UNIXTIME", "this", "zone", "hours", "minutes")(g, e)
}

// prestoSortArraySQL ports the Presto ARRAY_SORT transform for SortArray.
func prestoSortArraySQL(g *Generator, e exp.Expression) string {
	asc := asExpression(e.Arg("asc"))
	nullsFirst := asExpression(e.Arg("nulls_first"))
	descending := asc != nil && !truthy(asc.Arg("this"))
	if asc == nil && nullsFirst == nil {
		return g.funcCall("ARRAY_SORT", []any{e.This()}, "(", ")", true)
	}
	if asc != nil && truthy(asc.Arg("this")) && nullsFirst == nil {
		return g.funcCall("ARRAY_SORT", []any{e.This()}, "(", ")", true)
	}
	a, b := exp.ToIdentifier("a"), exp.ToIdentifier("b")
	nullRank := func(whenLeftNull, whenRightNull int) []exp.Expression {
		return []exp.Expression{
			exp.If(exp.Args{
				"this": exp.And(exp.Args{
					"this":       exp.Is(exp.Args{"this": a.Copy(), "expression": exp.Null()}),
					"expression": exp.Is(exp.Args{"this": b.Copy(), "expression": exp.Null()}),
				}),
				"true": exp.LiteralNumber("0"),
			}),
			exp.If(exp.Args{"this": exp.Is(exp.Args{"this": a.Copy(), "expression": exp.Null()}), "true": exp.LiteralNumber(strconv.Itoa(whenLeftNull))}),
			exp.If(exp.Args{"this": exp.Is(exp.Args{"this": b.Copy(), "expression": exp.Null()}), "true": exp.LiteralNumber(strconv.Itoa(whenRightNull))}),
		}
	}
	leftNull, rightNull := 1, -1 // nulls last
	if nullsFirst != nil && truthy(nullsFirst.Arg("this")) {
		leftNull, rightNull = -1, 1 // nulls first
	}
	ifs := nullRank(leftNull, rightNull)
	less, greater := -1, 1
	if descending {
		less, greater = 1, -1
	}
	ifs = append(ifs,
		exp.If(exp.Args{"this": exp.LT(exp.Args{"this": a.Copy(), "expression": b.Copy()}), "true": exp.LiteralNumber(strconv.Itoa(less))}),
		exp.If(exp.Args{"this": exp.GT(exp.Args{"this": a.Copy(), "expression": b.Copy()}), "true": exp.LiteralNumber(strconv.Itoa(greater))}),
	)
	cond := exp.Case(exp.Args{"ifs": ifs, "default": exp.LiteralNumber("0")})
	lambda := exp.Lambda(exp.Args{"this": cond, "expressions": []exp.Expression{a, b}})
	return g.funcCall("ARRAY_SORT", []any{e.This(), lambda}, "(", ")", true)
}

// prestoArrayToStringSQL ports the Presto ARRAY_JOIN transform, forwarding the optional
// null-replacement argument.
func prestoArrayToStringSQL(g *Generator, e exp.Expression) string {
	args := []any{e.This(), e.Arg("expression")}
	if nullArg := asExpression(e.Arg("null")); nullArg != nil {
		args = append(args, nullArg)
	}
	return g.funcCall("ARRAY_JOIN", args, "(", ")", true)
}

func prestoCast(name string) func(*Generator, exp.Expression) string {
	return func(g *Generator, e exp.Expression) string {
		return "CAST(" + g.sqlKey(e, "this") + " AS " + name + ")"
	}
}

func prestoTimeStrToTimeSQL(g *Generator, e exp.Expression) string {
	target := "TIMESTAMP"
	if asExpression(e.Arg("zone")) != nil {
		target = "TIMESTAMP WITH TIME ZONE"
	}
	return "CAST(" + g.sqlKey(e, "this") + " AS " + target + ")"
}

// prestoTableSQL names a table-source GENERATE_SERIES through the UNNEST column alias.
func prestoTableSQL(g *Generator, e exp.Expression) string {
	series := asExpression(e.This())
	if series == nil || series.Kind() != exp.KindGenerateSeries {
		return g.tableSQL(e)
	}
	rendered := g.gen(series)
	alias := asExpression(e.Arg("alias"))
	if alias == nil {
		return rendered
	}
	if cols := listFromValue(alias.Arg("columns")); len(cols) == 0 {
		if name := asExpression(alias.This()); name != nil {
			return rendered + " AS _u(" + g.gen(name) + ")"
		}
	}
	return rendered + " " + g.sqlKey(e, "alias")
}

// prestoLateralSQL ports _explode_to_unnest_sql (generators/presto.py:206-225): a LATERAL
// VIEW EXPLODE/INLINE becomes CROSS JOIN UNNEST, and POSEXPLODE becomes a lateral subquery
// that shifts the 0-based position.
func prestoLateralSQL(g *Generator, e exp.Expression) string {
	if !truthy(e.Arg("view")) {
		return g.lateralSQL(e)
	}
	this := asExpression(e.This())
	alias := asExpression(e.Arg("alias"))
	if this == nil || alias == nil {
		return g.lateralSQL(e)
	}

	columns := exprList(alias.Arg("columns"))
	switch {
	case this.Kind() == exp.KindPosexplode:
		if len(columns) == 0 {
			return g.lateralSQL(e)
		}
		pos := columns[0]
		cols := columns[1:]
		selectExprs := make([]exp.Expression, 0, 1+len(cols))
		selectExprs = append(selectExprs, exp.AliasExpr(
			exp.Sub(exp.Args{"this": pos.Copy(), "expression": exp.LiteralNumber("1")}),
			pos.Name(), false))
		unnestCols := make([]any, 0, len(cols)+1)
		for _, col := range cols {
			selectExprs = append(selectExprs, col.Copy())
			unnestCols = append(unnestCols, col.Copy())
		}
		unnestCols = append(unnestCols, pos.Copy())
		unnest := exp.Unnest(exp.Args{
			"expressions": []exp.Expression{this.This().Copy()},
			"offset":      true,
			"alias": exp.TableAlias(exp.Args{
				"this":    alias.This(),
				"columns": unnestCols,
			}),
		})
		subquery := exp.Select(exp.Args{
			"expressions": selectExprs,
			"from_":       exp.From(exp.Args{"this": unnest}),
		})
		return g.gen(exp.Join(exp.Args{
			"this": exp.Lateral(exp.Args{"this": exp.Subquery(exp.Args{"this": subquery})}),
			"kind": "CROSS",
		}))
	case this.Kind() == exp.KindExplode || isInlineUDTF(this):
		inner := this.This()
		if isInlineUDTF(this) {
			inner = this.Expressions()[0]
		}
		return g.gen(exp.Join(exp.Args{
			"this": exp.Unnest(exp.Args{
				"expressions": []exp.Expression{inner.Copy()},
				"alias":       alias.Copy(),
			}),
			"kind": "CROSS",
		}))
	}
	return g.lateralSQL(e)
}

// isInlineUDTF reports whether the lateral's UDTF is LATERAL VIEW INLINE.
func isInlineUDTF(e exp.Expression) bool {
	return e != nil && e.Kind() == exp.KindAnonymous && strings.EqualFold(e.Text("this"), "INLINE") && len(e.Expressions()) == 1
}

// prestoCastToTimestamp wraps a value in CAST(... AS TIMESTAMP) unless it already is one.
func prestoCastToTimestamp(e exp.Expression) exp.Expression {
	if e == nil {
		return nil
	}
	if dtype, ok := castResultDType(e); ok && dtype == exp.DTypeTimestamp {
		return e
	}
	return exp.Cast(exp.Args{"this": e, "to": exp.DTypeTimestamp.IntoExpr(nil)})
}

// prestoIsSQL ports the IS TRUE/FALSE rewrites required by IS_BOOL_ALLOWED = False
// (generators/presto.py:266).
func prestoIsSQL(g *Generator, e exp.Expression) string {
	this := asExpression(e.This())
	test := asExpression(e.Arg("expression"))
	if this == nil || test == nil || test.Kind() != exp.KindBoolean {
		return g.isSQL(e)
	}
	truth := boolValue(test.Arg("this"))
	if boolValue(e.Arg("negate")) {
		truth = !truth
	}
	if truth {
		return g.gen(this)
	}
	return "NOT " + g.gen(this)
}

// prestoExtractSQL ports extract_sql's epoch handling (generators/presto.py): an epoch
// extraction renders through TO_UNIXTIME.
func prestoExtractSQL(g *Generator, e exp.Expression) string {
	unit := asExpression(e.Arg("this"))
	name := ""
	if unit != nil {
		name = strings.ToUpper(unit.Name())
	}
	switch name {
	case "EPOCH", "EPOCH_SECOND":
		return "TO_UNIXTIME(" + g.gen(prestoCastToTimestamp(asExpression(e.Arg("expression")))) + ")"
	case "EPOCH_MILLISECOND":
		return "TO_UNIXTIME(" + g.gen(prestoCastToTimestamp(asExpression(e.Arg("expression")))) + ") * 1000"
	}
	return g.extractSQL(e)
}

// prestoStrictTimeFormat maps the port's internal "strict" markers straight to Presto
// tokens, mirroring the strict entries of upstream's INVERSE_TIME_MAPPING. Mapping them
// directly avoids collapsing %H:%M:%S into %T.
var prestoStrictTimeFormat = strings.NewReplacer(
	"%mstrict", "%m", "%dstrict", "%d", "%Hstrict", "%H",
	"%Istrict", "%h", "%Mstrict", "%i", "%Sstrict", "%s",
)

// prestoStrToUnixSQL ports strtounix_sql (generators/presto.py:565-582). TO_UNIXTIME needs a
// TIMESTAMP, so the input goes through DATE_PARSE, falling back to PARSE_DATETIME (Hive/Joda
// pattern language) when a time zone makes DATE_PARSE fail.
func prestoStrToUnixSQL(g *Generator, e exp.Expression) string {
	this := asExpression(e.This())
	valueAsText := exp.Cast(exp.Args{"this": this, "to": exp.DTypeText.IntoExpr(nil)})
	var valueAsTimestamp exp.Expression = this
	if this.IsString() {
		valueAsTimestamp = exp.Cast(exp.Args{"this": this, "to": exp.DTypeTimestamp.IntoExpr(nil)})
	}
	prestoFormat, hiveFormat := []any{}, []any{}
	if format := asExpression(e.Arg("format")); format != nil && format.IsString() {
		prestoFormat = []any{prestoFormatTime(e)}
		hiveFormat = []any{exp.LiteralString(dialects.HiveFormatTime(format.Name()))}
	}
	parseWithoutTZ := g.funcCall("DATE_PARSE", append([]any{valueAsText}, prestoFormat...), "(", ")", true)
	formatted := g.funcCall("DATE_FORMAT", append([]any{valueAsTimestamp}, prestoFormat...), "(", ")", true)
	parseWithTZ := g.funcCall("PARSE_DATETIME", append([]any{formatted}, hiveFormat...), "(", ")", true)
	coalesced := g.funcCall("COALESCE", []any{g.funcCall("TRY", []any{parseWithoutTZ}, "(", ")", true), parseWithTZ}, "(", ")", true)
	return g.funcCall("TO_UNIXTIME", []any{coalesced}, "(", ")", true)
}

// sequenceSQL ports sequence_sql (dialects/dialect.py:2396-2441), which Presto uses for
// GenerateSeries and GenerateDateArray: DATE and TIMESTAMP bounds are cast to the type of the
// cast operand, and an exclusive end yields an empty array when the step cannot advance.
func sequenceSQL(g *Generator, e exp.Expression) string {
	start := asExpression(e.Arg("start"))
	end := asExpression(e.Arg("end"))
	step := asExpression(e.Arg("step"))
	var targetType exp.Expression
	if start != nil && start.Kind() == exp.KindCast {
		targetType = asExpression(start.Arg("to"))
	} else if end != nil && end.Kind() == exp.KindCast {
		targetType = asExpression(end.Arg("to"))
	}
	if start != nil && end != nil {
		dt, ok := exp.DType(""), false
		if targetType != nil {
			dt, ok = targetType.Arg("this").(exp.DType)
		}
		if ok && (dt == exp.DTypeDate || dt == exp.DTypeTimestamp) {
			if start.Kind() == exp.KindCast && asExpression(start.Arg("to")) == targetType {
				end = exp.Cast(exp.Args{"this": end, "to": targetType})
			} else {
				start = exp.Cast(exp.Args{"this": start, "to": targetType})
			}
		}
		if truthy(e.Arg("is_end_exclusive")) {
			if folded, ok := foldExclusiveSequence(start, end, step); ok {
				return folded
			}
			stepValue := step
			if stepValue == nil {
				stepValue = exp.LiteralNumber("1")
			}
			end = exp.Paren(exp.Args{"this": exp.Sub(exp.Args{"this": end, "expression": stepValue})})
			sequenceCall := exp.Anonymous(exp.Args{"this": "SEQUENCE", "expressions": nonNilExprs(start, end, step)})
			zero := exp.LiteralNumber("0")
			shouldReturnEmpty := exp.Or(exp.Args{"this": exp.Or(exp.Args{
				"this": exp.EQ(exp.Args{"this": stepValue.Copy(), "expression": zero.Copy()}),
				"expression": exp.And(exp.Args{
					"this":       exp.GT(exp.Args{"this": stepValue.Copy(), "expression": zero.Copy()}),
					"expression": exp.GT(exp.Args{"this": start.Copy(), "expression": end.Copy()}),
				}),
			}), "expression": exp.And(exp.Args{
				"this":       exp.LT(exp.Args{"this": stepValue.Copy(), "expression": zero.Copy()}),
				"expression": exp.LT(exp.Args{"this": start.Copy(), "expression": end.Copy()}),
			})})
			emptyOrSequence := exp.If(exp.Args{"this": shouldReturnEmpty, "true": exp.Array(exp.Args{"expressions": []exp.Expression{}}), "false": sequenceCall})
			return g.gen(emptyOrSequence)
		}
	}
	return g.funcCall("SEQUENCE", anyExprs(start, end, step), "(", ")", true)
}

// nonNilExprs keeps the present operands of a SEQUENCE call.
func nonNilExprs(items ...exp.Expression) []exp.Expression {
	var out []exp.Expression
	for _, item := range items {
		if item != nil {
			out = append(out, item)
		}
	}
	return out
}

// anyExprs is nonNilExprs in the []any shape funcCall takes.
func anyExprs(items ...exp.Expression) []any {
	var out []any
	for _, item := range items {
		if item != nil {
			out = append(out, item)
		}
	}
	return out
}

// foldExclusiveSequence is _simplify_unless_literal (generators/presto.py via sequence_sql)
// for literal integer bounds: the emptiness test and the shortened end are evaluated at
// transpile time, so the result is an empty array or a SEQUENCE over the adjusted end. Only
// literal operands fold; anything else keeps the runtime IF.
func foldExclusiveSequence(start, end, step exp.Expression) (string, bool) {
	toInt := func(e exp.Expression) (int64, bool) {
		if e == nil || e.Kind() != exp.KindLiteral || e.IsString() {
			return 0, false
		}
		v, err := strconv.ParseInt(e.Name(), 10, 64)
		return v, err == nil
	}
	s, okS := toInt(start)
	en, okE := toInt(end)
	if !okS || !okE {
		return "", false
	}
	st, stepGiven := int64(1), step != nil
	if stepGiven {
		v, ok := toInt(step)
		if !ok {
			return "", false
		}
		st = v
	}
	if st == 0 || (st > 0 && s > en) || (st < 0 && s < en) {
		return "ARRAY[]", true
	}
	args := []string{strconv.FormatInt(s, 10), strconv.FormatInt(en-st, 10)}
	if stepGiven {
		args = append(args, strconv.FormatInt(st, 10))
	}
	return "SEQUENCE(" + strings.Join(args, ", ") + ")", true
}
