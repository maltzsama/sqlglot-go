package generator

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/maltzsama/sqlglot-go/dialects"
	"github.com/maltzsama/sqlglot-go/expressions"
)

// Hive overrides follow generators/hive.py:260-393 and 474-642.
func init() {
	registerDialectTypeMapping("hive", map[expressions.DType]string{
		expressions.DTypeBit: "BOOLEAN", expressions.DTypeBlob: "BINARY", expressions.DTypeDatetime: "TIMESTAMP", expressions.DTypeRowVersion: "BINARY", expressions.DTypeText: "STRING", expressions.DTypeTime: "TIMESTAMP", expressions.DTypeTimestampNtz: "TIMESTAMP", expressions.DTypeTimestampTz: "TIMESTAMP", expressions.DTypeUTinyInt: "SMALLINT", expressions.DTypeVarBinary: "BINARY",
	})
	registerDialectPropertyLocations("hive", map[expressions.Kind]propertyLocation{
		expressions.KindExternalProperty:       propertyLocationPostCreate,
		expressions.KindLocationProperty:       propertyLocationPostSchema,
		expressions.KindClusteredByProperty:    propertyLocationPostSchema,
		expressions.KindStorageHandlerProperty: propertyLocationPostSchema,
		expressions.KindUsingProperty:          propertyLocationPostExpression,
		expressions.KindFileFormatProperty:     propertyLocationPostSchema,
		expressions.KindPartitionedByProperty:  propertyLocationPostSchema,
		expressions.KindWithDataProperty:       propertyLocationUnsupported,
		// generators/hive.py:401 exp.VolatileProperty: UNSUPPORTED.
		expressions.KindVolatileProperty: propertyLocationUnsupported,
	})
	handlers := map[expressions.Kind]func(*Generator, expressions.Expression) string{
		// generators/hive.py:343-345 exp.StrPosition: strposition_sql(func_name="LOCATE",
		// supports_position=True); spark2/spark/databricks inherit it through dialectParents.
		expressions.KindStrPosition: opaqueStrPosition(locateStrPositionSQL),
		expressions.KindParameter:   (*Generator).hiveParameterSQL,
		expressions.KindVersion: func(g *Generator, e expressions.Expression) string {
			return e.Name() + " " + e.Text("kind") + " " + g.sqlKey(e, "expression")
		},
		expressions.KindProperties: (*Generator).hivePropertiesSQL,
		expressions.KindProperty: func(g *Generator, e expressions.Expression) string {
			// dialect.py:1317-1318 property_sql: a dotted key keeps its raw form, any other key
			// is quoted.
			return g.propertyName(e, true) + "=" + g.sqlKey(e, "value")
		},
		expressions.KindExternalProperty:       func(*Generator, expressions.Expression) string { return "EXTERNAL" },
		expressions.KindLocationProperty:       hivePrefix("LOCATION "),
		expressions.KindStorageHandlerProperty: hivePrefix("STORED BY "),
		expressions.KindSchemaCommentProperty:  hivePrefix("COMMENT "),
		expressions.KindPartitionedByProperty:  hivePrefix("PARTITIONED BY "),
		expressions.KindUsingProperty: func(g *Generator, e expressions.Expression) string {
			return "USING " + e.Text("kind") + " " + g.sqlKey(e, "this")
		},
		expressions.KindFileFormatProperty: hiveFileFormatPropertySQL,
		expressions.KindInputOutputFormat: func(g *Generator, e expressions.Expression) string {
			var p []string
			for _, a := range []struct{ k, p string }{{"input_format", "INPUTFORMAT "}, {"output_format", "OUTPUTFORMAT "}} {
				if s := g.sqlKey(e, a.k); s != "" {
					p = append(p, a.p+s)
				}
			}
			return strings.Join(p, " ")
		},
		expressions.KindClusteredByProperty: func(g *Generator, e expressions.Expression) string {
			s := "CLUSTERED BY (" + g.expressions(exprsOptions{expression: e, flat: true}) + ")"
			if v := g.expressions(exprsOptions{expression: e, key: "sorted_by", flat: true}); v != "" {
				s += " SORTED BY (" + v + ")"
			}
			return s + " INTO " + g.sqlKey(e, "buckets") + " BUCKETS"
		},
		expressions.KindAlterSet:    (*Generator).hiveAlterSetSQL,
		expressions.KindAlterColumn: (*Generator).hiveAlterColumnSQL,
		expressions.KindCreate:      hiveCreateSQL,
		expressions.KindColumnDef:   hiveColumnDefSQL,
		expressions.KindNotNullColumnConstraint: func(g *Generator, e expressions.Expression) string {
			// generators/hive.py:313: Hive omits explicit NULL constraints.
			if boolValue(e.Arg("allow_null")) {
				return ""
			}
			return g.notNullColumnConstraintSQL(e)
		},
		expressions.KindDataType: (*Generator).hiveDataTypeSQL,
		expressions.KindTryCast:  func(g *Generator, e expressions.Expression) string { return g.castSQLWithPrefix(e, "") },
		// generators/hive.py:300 `exp.If: if_sql()` — Hive renders the three-argument IF
		// form rather than a CASE.
		expressions.KindIf: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("IF", []any{e.This(), e.Arg("true"), e.Arg("false")}, "(", ")", true)
		},
		// generators/hive.py:367 `exp.Trim: trim_sql` imports dialect.py:1822's TRIM grammar
		// form (this port's trimSQLStandard), not the LTRIM/RTRIM call form.
		expressions.KindTrim:           (*Generator).trimSQLStandard,
		expressions.KindApproxDistinct: hiveCall("APPROX_COUNT_DISTINCT", "this"),
		expressions.KindIsNan:          hiveCall("ISNAN", "this"),
		expressions.KindRegexpLike:     func(g *Generator, e expressions.Expression) string { return g.binary(e, "RLIKE") },
		expressions.KindRegexpReplace:  hiveCall("REGEXP_REPLACE", "this", "expression", "replacement"),
		expressions.KindMonthsBetween:  hiveCall("MONTHS_BETWEEN", "this", "expression"),
		expressions.KindMD5Digest: func(g *Generator, e expressions.Expression) string {
			digest := g.funcCall("MD5", []any{e.This()}, "(", ")", true)
			return g.funcCall("UNHEX", []any{digest}, "(", ")", true)
		},
		expressions.KindExtract: func(g *Generator, e expressions.Expression) string {
			// generators/hive.py:233 EXTRACT_ALLOWS_QUOTES = False.
			if unit := e.This(); unit != nil && (unit.IsString() || unit.Kind() == expressions.KindIdentifier) {
				e.Set("this", expressions.Var(expressions.Args{"this": unit.Name()}))
			}
			return g.extractSQL(e)
		},
		expressions.KindLeft:  hiveLeftSQL,
		expressions.KindRight: hiveRightSQL,
		expressions.KindTrunc: func(g *Generator, e expressions.Expression) string {
			// generators/hive.py:491-493 trunc_sql.
			if e.Arg("decimals") != nil {
				g.unsupported("'decimals' is not supported")
			}
			return g.gen(expressions.Cast(expressions.Args{"this": e.This(), "to": expressions.DTypeBigInt.IntoExpr(nil)}))
		},
		expressions.KindUuid: func(g *Generator, e expressions.Expression) string {
			// generator.py:6237-6244 uuid_sql.
			uuid := g.funcCall("UUID", nil, "(", ")", true)
			if boolValue(e.Arg("is_string")) {
				return g.gen(expressions.Cast(expressions.Args{"this": uuid, "to": expressions.DTypeVarchar.IntoExpr(nil)}))
			}
			return uuid
		},
		expressions.KindIgnoreNulls: func(g *Generator, e expressions.Expression) string {
			// hive.py:432-437: FIRST/LAST/FIRST_VALUE/LAST_VALUE take IGNORE NULLS as a TRUE argument.
			if this := e.This(); this != nil {
				switch this.Kind() {
				case expressions.KindFirst, expressions.KindLast, expressions.KindFirstValue, expressions.KindLastValue:
					return g.funcCall(g.sqlName(this.Kind()), []any{this.This(), expressions.Boolean(expressions.Args{"this": true})}, "(", ")", true)
				}
			}
			return g.ignoreNullsSQL(e)
		},
		expressions.KindStrToDate: func(g *Generator, e expressions.Expression) string {
			return "CAST(" + hiveStrToTimeInner(g, e) + " AS DATE)"
		},
		expressions.KindStrToTime: func(g *Generator, e expressions.Expression) string {
			return "CAST(" + hiveStrToTimeInner(g, e) + " AS TIMESTAMP)"
		},
		expressions.KindDateAdd: (*Generator).hiveAddDateSQL,
		// generators/hive.py:587-591: EXISTS over an array lambda is a function call.
		expressions.KindExists: func(g *Generator, e expressions.Expression) string {
			if e.Arg("expression") != nil {
				return g.functionFallbackSQL(e)
			}
			return g.existsSQL(e)
		},
		// generators/hive.py:351: GENERATE_SERIES in a FROM becomes an UNNEST.
		expressions.KindTable: hiveTableSQL,
		// generators/hive.py:335-342: the Select preprocess chain (no arrays_zip).
		expressions.KindSelect: func(g *Generator, e expressions.Expression) string {
			if asExpression(e.Arg("qualify")) != nil {
				e = eliminateQualify(e)
			}
			e = unnestToExplode(e, false)
			e = anyToExists(e)
			return g.selectSQL(e)
		},
		expressions.KindDateStrToDate: func(g *Generator, e expressions.Expression) string {
			return "CAST(" + g.sqlKey(e, "this") + " AS DATE)"
		},
		expressions.KindDateToDateStr: func(g *Generator, e expressions.Expression) string {
			return "CAST(" + g.sqlKey(e, "this") + " AS STRING)"
		},
		expressions.KindDateToDi: func(g *Generator, e expressions.Expression) string {
			return "CAST(DATE_FORMAT(" + g.sqlKey(e, "this") + ", 'yyyyMMdd') AS INT)"
		},
		expressions.KindDiToDate: func(g *Generator, e expressions.Expression) string {
			return "TO_DATE(CAST(" + g.sqlKey(e, "this") + " AS STRING), 'yyyyMMdd')"
		},
		expressions.KindTimeStrToTime: func(g *Generator, e expressions.Expression) string {
			return "CAST(" + g.sqlKey(e, "this") + " AS TIMESTAMP)"
		},
		expressions.KindTimeToTimeStr: func(g *Generator, e expressions.Expression) string {
			return "CAST(" + g.sqlKey(e, "this") + " AS STRING)"
		},
		expressions.KindTsOrDiToDi: func(g *Generator, e expressions.Expression) string {
			return "CAST(SUBSTR(REPLACE(CAST(" + g.sqlKey(e, "this") + " AS STRING), '-', ''), 1, 8) AS INT)"
		},
		expressions.KindTsOrDsToDateStr: func(g *Generator, e expressions.Expression) string {
			return "SUBSTRING(CAST(" + g.sqlKey(e, "this") + " AS STRING), 1, 10)"
		},
		expressions.KindUnixToTimeStr: func(g *Generator, e expressions.Expression) string {
			return "FROM_UNIXTIME(" + g.sqlKey(e, "this") + ")"
		},
		expressions.KindArrayFilter: renameFunc("FILTER"),
		// generators/hive.py:281 Array: preprocess with inherit_struct_field_names.
		expressions.KindArray: func(g *Generator, e expressions.Expression) string {
			if truthy(e.Arg("struct_name_inheritance")) {
				applyStructNameInheritance(e.Expressions())
			}
			return g.arraySQL(e)
		},
		expressions.KindAnonymous: func(g *Generator, e expressions.Expression) string {
			name := strings.ToUpper(e.Text("this"))
			args := e.Expressions()
			switch name {
			case "VAR_MAP":
				return g.funcCall("MAP", listToAny(args), "(", ")", true)
			case "MAP":
				if len(args) == 2 && args[0] != nil && args[1] != nil &&
					args[0].Kind() == expressions.KindArray && args[1].Kind() == expressions.KindArray {
					keys := args[0].Expressions()
					values := args[1].Expressions()
					flat := make([]any, 0, len(keys)+len(values))
					for i := range keys {
						if i < len(values) {
							flat = append(flat, keys[i], values[i])
						}
					}
					return g.funcCall("MAP", flat, "(", ")", true)
				}
			case "ARRAY_SORT":
				if len(args) > 0 {
					return g.funcCall("SORT_ARRAY", []any{args[0]}, "(", ")", true)
				}
			}
			return g.anonymousSQL(e)
		},
		expressions.KindArrayToString: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("CONCAT_WS", []any{e.Arg("expression"), e.This()}, "(", ")", true)
		},
		expressions.KindDateDiff:   (*Generator).hiveDateDiffUnits,
		expressions.KindTsOrDsDiff: (*Generator).hiveDateDiffUnits,
		expressions.KindIndex:      hiveIndexSQL,
		expressions.KindExplode:    renameFunc("EXPLODE"),
		expressions.KindILike: func(g *Generator, e expressions.Expression) string {
			if g.dialect.Name != "hive" {
				return g.ilikeSQL(e)
			}
			op := "LIKE"
			if boolValue(e.Arg("negate")) {
				op = "NOT LIKE"
			}
			return "LOWER(" + g.sqlKey(e, "this") + ") " + op + " LOWER(" + g.sqlKey(e, "expression") + ")"
		},
		expressions.KindDateSub:      hiveDateSubSQL,
		expressions.KindConcatWs:     hiveConcatWsSQL,
		expressions.KindTsOrDsAdd:    (*Generator).hiveAddDateSQL,
		expressions.KindTsOrDsToDate: (*Generator).hiveToDateSQL,
		expressions.KindTimeToStr: func(g *Generator, e expressions.Expression) string {
			// hive.py:593-598: DATE_FORMAT(TimeStrToTime(x), f) renders the inner x directly.
			this := e.This()
			if this != nil && this.Kind() == expressions.KindTimeStrToTime {
				this = this.This()
			}
			return g.funcCall("DATE_FORMAT", []any{this, hiveFormatTimeLiteral(e)}, "(", ")", true)
		},
		expressions.KindUnixToStr: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("FROM_UNIXTIME", []any{e.This(), hiveNonDefaultTimeFormat(e)}, "(", ")", true)
		},
		expressions.KindStrToUnix: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("UNIX_TIMESTAMP", []any{e.This(), hiveNonDefaultTimeFormat(e)}, "(", ")", true)
		},
		expressions.KindArrayAgg: func(g *Generator, e expressions.Expression) string {
			this := e.This()
			if this != nil && this.Kind() == expressions.KindOrder {
				this = this.This()
			}
			return g.funcCall("COLLECT_LIST", []any{this}, "(", ")", true)
		},
		expressions.KindStruct: func(g *Generator, e expressions.Expression) string {
			args := []any{}
			for _, x := range e.Expressions() {
				if x.Kind() == expressions.KindPropertyEQ {
					g.unsupported("Hive does not support named structs.")
					args = append(args, x.Arg("expression"))
				} else {
					args = append(args, x)
				}
			}
			return g.funcCall("STRUCT", args, "(", ")", true)
		},
		expressions.KindVarMap:            (*Generator).hiveVarMapSQL,
		expressions.KindMap:               (*Generator).hiveVarMapSQL,
		expressions.KindSplit:             hiveSplitSQL,
		expressions.KindUnixToTime:        hiveUnixToTimeSQL,
		expressions.KindIntDiv:            func(g *Generator, e expressions.Expression) string { return g.binary(e, "DIV") },
		expressions.KindRegexpExtract:     (*Generator).hiveRegexpExtractSQL,
		expressions.KindRegexpExtractAll:  (*Generator).hiveRegexpExtractSQL,
		expressions.KindJSONExtract:       hiveCall("GET_JSON_OBJECT", "this", "expression"),
		expressions.KindJSONExtractScalar: hiveCall("GET_JSON_OBJECT", "this", "expression"),
		expressions.KindGenerateSeries:    hiveSequenceSQL,
		expressions.KindGenerateDateArray: hiveSequenceSQL,
		expressions.KindArraySize:         hiveCall("SIZE", "this"),
		// generators/hive.py:282 exp.ArrayConcat: rename_func("CONCAT").
		expressions.KindArrayConcat: renameFunc("CONCAT"),
		expressions.KindStrToMap:    hiveCall("STR_TO_MAP", "this", "pair_delim", "key_value_delim"),
		expressions.KindTimestampTrunc: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("TRUNC", []any{e.This(), expressions.LiteralString(e.Text("unit"))}, "(", ")", true)
		},
	}
	// Go 1.22+ gives each range iteration its own k/name, so the closure can capture them
	// directly; the previous `name := name` copy is no longer needed.
	for k, name := range map[expressions.Kind]string{expressions.KindAnyValue: "FIRST", expressions.KindFromBase64: "UNBASE64", expressions.KindToBase64: "BASE64", expressions.KindJSONFormat: "TO_JSON", expressions.KindQuantile: "PERCENTILE", expressions.KindApproxQuantile: "PERCENTILE_APPROX", expressions.KindRegexpSplit: "SPLIT", expressions.KindArrayUniqueAgg: "COLLECT_SET", expressions.KindTimeStrToDate: "TO_DATE", expressions.KindTimeStrToUnix: "UNIX_TIMESTAMP", expressions.KindTimeToUnix: "UNIX_TIMESTAMP", expressions.KindWeekOfYear: "WEEKOFYEAR", expressions.KindDayOfMonth: "DAYOFMONTH", expressions.KindDayOfWeek: "DAYOFWEEK", expressions.KindUnnest: "EXPLODE", expressions.KindStarMap: "MAP", expressions.KindUnicode: "ASCII"} {
		handlers[k] = func(g *Generator, e expressions.Expression) string {
			return g.funcCall(name, g.fallbackArgs(e), "(", ")", true)
		}
	}
	registerDialectDispatch("hive", handlers)
}

// hiveCreateSQL ports the CREATE preprocess chain in generators/hive.py:315-321.
func hiveCreateSQL(g *Generator, e expressions.Expression) string {
	return g.createSQL(moveSchemaColumnsToPartitionedBy(
		ctasWithTmpTablesToCreateTmpView(removeUniqueConstraints(e), false)))
}

// hiveColumnDefSQL ports columndef_sql (generators/hive.py:466-472): a struct's field names
// are separated by a colon instead of a space.
func hiveColumnDefSQL(g *Generator, e expressions.Expression) string {
	s := g.columnDefSQL(e)
	if p := e.Parent(); p != nil && p.Kind() == expressions.KindDataType && expressions.IsType(p, expressions.DTypeStruct) {
		column := g.sqlKey(e, "this")
		s = strings.Replace(s, column+" ", column+": ", 1)
	}
	return s
}

func hiveCall(name string, keys ...string) func(*Generator, expressions.Expression) string {
	return func(g *Generator, e expressions.Expression) string {
		args := make([]any, 0, len(keys))
		for _, key := range keys {
			args = append(args, e.Arg(key))
		}
		return g.funcCall(name, args, "(", ")", true)
	}
}

// hiveLeftSQL and hiveRightSQL port the substring transforms in dialect.py:1752-1767.
func hiveLeftSQL(g *Generator, e expressions.Expression) string {
	return g.gen(expressions.Substring(expressions.Args{
		"this": e.This(), "start": expressions.LiteralNumber(1), "length": e.Expr(),
	}))
}

func hiveRightSQL(g *Generator, e expressions.Expression) string {
	length := e.Expr()
	// expressions/core.py:1399-1407 wraps a different binary operator before subtracting.
	if length != nil && length.Kind() != expressions.KindSub && length.Is(expressions.TraitBinary) {
		length = expressions.Paren(expressions.Args{"this": length})
	}
	start := expressions.Sub(expressions.Args{
		"this": expressions.Length(expressions.Args{"this": e.This().Copy()}),
		"expression": expressions.Paren(expressions.Args{"this": expressions.Sub(expressions.Args{
			"this": length, "expression": expressions.LiteralNumber(1),
		})}),
	})
	return g.gen(expressions.Substring(expressions.Args{"this": e.This(), "start": start}))
}

// hiveFileFormatPropertySQL ports fileformatproperty_sql (generators/hive.py:604-611):
// STORED AS <input/output format | uppercase name>. Spark2 reuses it when hive_format is set
// (generators/spark2.py:237-241).
func hiveFileFormatPropertySQL(g *Generator, e expressions.Expression) string {
	this := strings.ToUpper(e.Name())
	if t := e.This(); t != nil && t.Kind() == expressions.KindInputOutputFormat {
		this = g.gen(t)
	}
	return "STORED AS " + this
}

// hiveFormatTimeLiteral is Generator.format_time under Hive's INVERSE_TIME_MAPPING.
func hiveFormatTimeLiteral(e expressions.Expression) any {
	format := asExpression(e.Arg("format"))
	if format == nil {
		return format
	}
	convert := func(part expressions.Expression) expressions.Expression {
		if part == nil || !part.IsString() {
			return part
		}
		name := part.Name()
		switch e.Kind() {
		case expressions.KindStrToTime, expressions.KindStrToDate, expressions.KindStrToUnix, expressions.KindTsOrDsToDate:
			name = hiveLenientParseFormat(name)
		}
		return expressions.LiteralString(dialects.HiveFormatTime(name))
	}
	if format.Kind() == expressions.KindConcat {
		parts := make([]expressions.Expression, 0, len(format.Expressions()))
		for _, part := range format.Expressions() {
			parts = append(parts, convert(part))
		}
		return expressions.Concat(expressions.Args{"expressions": parts})
	}
	if !format.IsString() {
		return format
	}
	return convert(format)
}

// hiveNonDefaultTimeFormat ports dialect.py:1622-1633 time_format("hive"): nil for TIME_FORMAT.
func hiveNonDefaultTimeFormat(e expressions.Expression) any {
	format := hiveFormatTimeLiteral(e)
	if lit, ok := format.(expressions.Expression); ok && lit != nil && lit.Name() == dialects.HiveTimeFormat {
		return nil
	}
	return format
}

// hiveStrToTimeInner ports the shared body of _str_to_date_sql/_str_to_time_sql (hive.py:199-209).
func hiveStrToTimeInner(g *Generator, e expressions.Expression) string {
	this := g.sqlKey(e, "this")
	if format, ok := hiveFormatTimeLiteral(e).(expressions.Expression); ok && format != nil {
		if !hiveIsCastTimeFormat(e, format.Name()) {
			return "FROM_UNIXTIME(UNIX_TIMESTAMP(" + this + ", " + g.gen(format) + "))"
		}
	}
	return this
}

// hiveToDateSQL ports _to_date_sql (hive.py:211-219).
func (g *Generator) hiveToDateSQL(e expressions.Expression) string {
	if format, ok := hiveFormatTimeLiteral(e).(expressions.Expression); ok && format != nil {
		if !hiveIsCastTimeFormat(e, format.Name()) {
			return g.funcCall("TO_DATE", []any{e.This(), format}, "(", ")", true)
		}
	}
	if p := e.Parent(); p != nil {
		switch p.Kind() {
		case expressions.KindDateDiff, expressions.KindDay, expressions.KindMonth, expressions.KindYear:
			return g.sqlKey(e, "this")
		}
		// Spark2Generator.TS_OR_DS_EXPRESSIONS (generators/spark2.py:130-136) extends Hive's
		// list with these four; Spark and Databricks inherit it.
		if g.isDialect("spark2") {
			switch p.Kind() {
			case expressions.KindDayOfMonth, expressions.KindDayOfWeek, expressions.KindDayOfYear, expressions.KindWeekOfYear:
				return g.sqlKey(e, "this")
			}
		}
	}
	return g.funcCall("TO_DATE", []any{e.This()}, "(", ")", true)
}

// hiveVarMapSQL ports var_map_sql (dialect.py:1521-1536).
func (g *Generator) hiveVarMapSQL(e expressions.Expression) string {
	keys, values := asExpression(e.Arg("keys")), asExpression(e.Arg("values"))
	if keys == nil || values == nil || keys.Kind() != expressions.KindArray || values.Kind() != expressions.KindArray {
		g.unsupported("Cannot convert array columns into map.")
		return g.funcCall("MAP", []any{keys, values}, "(", ")", true)
	}
	args := []any{}
	ks, vs := keys.Expressions(), values.Expressions()
	for i := 0; i < len(ks) && i < len(vs); i++ {
		args = append(args, ks[i], vs[i])
	}
	return g.funcCall("MAP", args, "(", ")", true)
}

// hiveRegexpExtractSQL ports regexp_extract_sql (dialect.py:1855-1865).
func (g *Generator) hiveRegexpExtractSQL(e expressions.Expression) string {
	group := asExpression(e.Arg("group"))
	if group != nil && group.Name() == strconv.Itoa(g.dialect.RegexpExtractDefaultGroup) {
		group = nil
	}
	return g.funcCall(g.sqlName(e.Kind()), []any{e.This(), e.Arg("expression"), group}, "(", ")", true)
}

func hivePrefix(prefix string) func(*Generator, expressions.Expression) string {
	return func(g *Generator, e expressions.Expression) string { return prefix + g.sqlKey(e, "this") }
}

// generators/hive.py:235 supplies WITH_PROPERTIES_PREFIX for generator.py:2019-2042.
func (g *Generator) hivePropertiesSQL(e expressions.Expression) string {
	locations := g.locateProperties(e)
	root := g.rootPropertiesSQL(g.propertiesExpression(locations[propertyLocationPostSchema], e.Parent()))
	with := g.renderProperties(g.propertiesExpression(locations[propertyLocationPostWith], e.Parent()), g.seg("TBLPROPERTIES", ""), ", ", "", true)
	if root != "" && with != "" && !g.pretty {
		with = " " + with
	}
	return root + with
}

// generators/hive.py:590-605.
func (g *Generator) hiveAlterSetSQL(e expressions.Expression) string {
	s := "SET"
	if v := g.sqlKey(e, "serde"); v != "" {
		s += " SERDE " + v
	}
	if v := g.expressions(exprsOptions{expression: e, flat: true}); v != "" {
		s += " " + v
	}
	if v := g.sqlKey(e, "location"); v != "" {
		s += " LOCATION " + v
	}
	if v := g.expressions(exprsOptions{expression: e, key: "file_format", flat: true, sep: " "}); v != "" {
		s += " FILEFORMAT " + v
	}
	if v := g.expressions(exprsOptions{expression: e, key: "tag", flat: true, sep: ""}); v != "" {
		s += " TAGS " + v
	}
	return s
}

// generators/hive.py:560-584.
func (g *Generator) hiveAlterColumnSQL(e expressions.Expression) string {
	if boolValue(e.Arg("exists")) {
		g.unsupported("ALTER COLUMN IF EXISTS is not supported by this dialect")
	}
	if truthy(e.Arg("default")) || truthy(e.Arg("drop")) || truthy(e.Arg("visible")) || e.Arg("allow_null") != nil {
		g.unsupported("Unsupported CHANGE COLUMN syntax")
	}
	this := g.sqlKey(e, "this")
	name := g.sqlKey(e, "rename_to")
	if name == "" {
		name = this
	}
	dtype := g.sqlKey(e, "dtype")
	if dtype == "" {
		g.unsupported("CHANGE COLUMN without a type is not supported")
	}
	s := "CHANGE COLUMN " + this + " " + name + " " + dtype
	if c := g.sqlKey(e, "comment"); c != "" {
		s += " COMMENT " + c
	}
	return s
}

// generators/hive.py:502-520.
func (g *Generator) hiveDataTypeSQL(e expressions.Expression) string {
	e = e.Copy()
	params := e.Expressions()
	switch e.Arg("this") {
	case expressions.DTypeVarchar, expressions.DTypeNVarchar, expressions.DTypeChar, expressions.DTypeNChar:
		if len(params) == 0 || params[0].Name() == "MAX" {
			e.Set("this", expressions.DTypeText)
			e.Set("expressions", nil)
		}
	case expressions.DTypeText:
		if len(params) > 0 {
			e.Set("this", expressions.DTypeVarchar)
		}
	case expressions.DTypeFloat:
		if len(params) > 0 {
			if n, err := strconv.Atoi(params[0].Name()); err == nil {
				if n > 32 {
					e.Set("this", expressions.DTypeDouble)
				}
				e.Set("expressions", nil)
			}
		}
	}
	if dtype, ok := e.Arg("this").(expressions.DType); ok && expressions.TemporalTypes[dtype] {
		e.Set("expressions", nil)
	}
	return g.dataTypeSQL(e)
}

// generators/hive.py:118-139.
func (g *Generator) hiveAddDateSQL(e expressions.Expression) string {
	unit := strings.ToUpper(e.Text("unit"))
	name := "DATE_ADD"
	mult := 1
	switch unit {
	case "WEEK":
		mult = 7
	case "MONTH":
		name = "ADD_MONTHS"
	case "QUARTER":
		name = "ADD_MONTHS"
		mult = 3
	case "YEAR":
		name = "ADD_MONTHS"
		mult = 12
	}
	inc := e.Expr()
	sql := g.gen(inc)
	if inc != nil && inc.Kind() == expressions.KindLiteral {
		if _, err := strconv.ParseFloat(inc.Name(), 64); err == nil {
			sql = inc.Name()
		}
	}
	if mult != 1 {
		if inc != nil && inc.Kind() == expressions.KindLiteral {
			if n, err := strconv.ParseFloat(inc.Name(), 64); err == nil {
				sql = strconv.FormatFloat(n*float64(mult), 'f', -1, 64)
			} else {
				g.unsupported("Invalid date increment")
			}
		} else {
			sql = "(" + sql + ") * " + strconv.Itoa(mult)
		}
	}
	return g.funcCall(name, []any{e.This(), sql}, "(", ")", true)
}

// generators/hive.py:449-460 omits braces for SET assignments.
func (g *Generator) hiveParameterSQL(e expressions.Expression) string {
	this := g.sqlKey(e, "this")
	if value := g.sqlKey(e, "expression"); value != "" {
		this += ":" + value
	}
	if parent := e.Parent(); parent != nil && parent.Kind() == expressions.KindEQ {
		if item := parent.Parent(); item != nil && item.Kind() == expressions.KindSetItem {
			return this
		}
	}
	return "${" + this + "}"
}

var hiveCanonicalTimeFormat = regexp.MustCompile(`%(?:[mdHIMS]strict|[-:].|.)`)

// generators/hive.py:67-92 keeps adjacent numeric fields padded.
func hiveLenientParseFormat(format string) string {
	matches := hiveCanonicalTimeFormat.FindAllStringIndex(format, -1)
	var result strings.Builder
	end := 0
	for i, match := range matches {
		start, stop := match[0], match[1]
		result.WriteString(format[end:start])
		token := format[start:stop]
		if len(token) == 2 && strings.ContainsRune("mdHIMS", rune(token[1])) {
			leftAdjacent := i > 0 && matches[i-1][1] == start
			rightAdjacent := i+1 < len(matches) && matches[i+1][0] == stop
			if start > 0 {
				r, _ := utf8.DecodeLastRuneInString(format[:start])
				leftAdjacent = leftAdjacent || unicode.IsDigit(r)
			}
			if stop < len(format) {
				r, _ := utf8.DecodeRuneInString(format[stop:])
				rightAdjacent = rightAdjacent || unicode.IsDigit(r)
			}
			if !leftAdjacent && !rightAdjacent {
				token = "%-" + token[1:]
			}
		}
		result.WriteString(token)
		end = stop
	}
	result.WriteString(format[end:])
	return result.String()
}

func hiveIsCastTimeFormat(e expressions.Expression, format string) bool {
	if format == dialects.HiveTimeFormat || format == dialects.HiveDateFormat {
		return true
	}
	if format == "yyyy-M-d H:m:s" || format == "yyyy-M-d" {
		if original := asExpression(e.Arg("format")); original != nil && original.IsString() {
			padded := dialects.HiveFormatTime(original.Name())
			return padded == dialects.HiveTimeFormat || padded == dialects.HiveDateFormat
		}
	}
	return false
}

// Hive function renames (generators/hive.py TRANSFORMS); Spark/Databricks inherit them.
func init() {
	registerDialectDispatch("hive", map[expressions.Kind]func(*Generator, expressions.Expression) string{
		expressions.KindStructExtract: structExtractSQL,
		expressions.KindArgMax:        renameFunc("MAX_BY"),
		expressions.KindArgMin:        renameFunc("MIN_BY"),
		expressions.KindNumberToStr:   renameFunc("FORMAT_NUMBER"),
	})
}

// hiveSequenceSQL ports sequence_sql (dialects/dialect.py:2396-2436) for GenerateSeries.
func hiveSequenceSQL(g *Generator, e expressions.Expression) string {
	if truthy(e.Arg("exploding")) {
		parent := e.Parent()
		if parent != nil && (parent.Kind() == expressions.KindAlias || parent.Kind() == expressions.KindTableAlias) {
			parent = parent.Parent()
		}
		if parent == nil || (parent.Kind() != expressions.KindTable && parent.Kind() != expressions.KindUnnest) {
			copy := e.Copy()
			copy.Set("exploding", nil)
			return g.gen(expressions.Unnest(expressions.Args{
				"expressions": []expressions.Expression{copy},
			}))
		}
	}
	start := asExpression(e.Arg("start"))
	end := asExpression(e.Arg("end"))
	step := asExpression(e.Arg("step"))
	if step != nil && step.IsString() {
		if interval := postgresIntervalExpr(step.Name()); interval != nil {
			step = interval
		}
	}
	// dialects/dialect.py:2401-2413: when either bound is a date/timestamp cast, cast the
	// other bound to the same type so SEQUENCE sees a uniform range.
	if target := hiveSeriesCastTarget(start, end); target != nil {
		if start.Kind() != expressions.KindCast {
			start = expressions.Cast(expressions.Args{"this": start, "to": target.Copy()})
		}
		if end.Kind() != expressions.KindCast {
			end = expressions.Cast(expressions.Args{"this": end, "to": target.Copy()})
		}
	}
	if start == nil || end == nil || !truthy(e.Arg("is_end_exclusive")) {
		return g.funcCall("SEQUENCE", []any{start, end, step}, "(", ")", true)
	}
	stepValue := step
	if stepValue == nil {
		stepValue = expressions.LiteralNumber("1")
	}
	sequence := func(endExpr expressions.Expression) expressions.Expression {
		if step == nil {
			return expressions.Anonymous(expressions.Args{"this": "SEQUENCE", "expressions": []expressions.Expression{start, endExpr}})
		}
		return expressions.Anonymous(expressions.Args{"this": "SEQUENCE", "expressions": []expressions.Expression{start, endExpr, step}})
	}
	st, hasLiteralStep := hiveIntLiteral(stepValue)
	s, hasStart := hiveIntLiteral(start)
	en, hasEnd := hiveIntLiteral(end)
	if hasLiteralStep && st == 0 {
		return g.gen(expressions.Array(expressions.Args{}))
	}
	if hasLiteralStep && hasStart && hasEnd {
		// SEQUENCE's stop is inclusive: convert the exclusive bound to the last term
		// inside the range (end-1 for ascending, end+1 for descending).
		last := en - 1
		empty := s >= en
		if st < 0 {
			last = en + 1
			empty = s <= en
		}
		if empty {
			return g.gen(expressions.Array(expressions.Args{}))
		}
		return g.gen(sequence(expressions.LiteralNumber(strconv.Itoa(last))))
	}
	if hasLiteralStep {
		var adjusted, cond expressions.Expression
		if st > 0 {
			adjusted = expressions.Paren(expressions.Args{"this": expressions.Sub(expressions.Args{"this": end, "expression": expressions.LiteralNumber("1")})})
			cond = expressions.LT(expressions.Args{"this": adjusted.Copy(), "expression": start.Copy()})
		} else {
			adjusted = expressions.Paren(expressions.Args{"this": expressions.Add(expressions.Args{"this": end, "expression": expressions.LiteralNumber("1")})})
			cond = expressions.GT(expressions.Args{"this": adjusted.Copy(), "expression": start.Copy()})
		}
		return g.gen(expressions.If(expressions.Args{
			"this": cond, "true": expressions.Array(expressions.Args{}), "false": sequence(adjusted),
		}))
	}
	// Dynamic step: SEQUENCE cannot know the last term, so keep the upstream
	// end-step form and its emptiness guard.
	adjustedEnd := expressions.Paren(expressions.Args{"this": expressions.Sub(expressions.Args{"this": end, "expression": stepValue})})
	zero := expressions.LiteralNumber("0")
	shouldReturnEmpty := expressions.Or(expressions.Args{
		"this": expressions.EQ(expressions.Args{"this": stepValue.Copy(), "expression": zero.Copy()}),
		"expression": expressions.Or(expressions.Args{
			"this": expressions.And(expressions.Args{
				"this":       expressions.GT(expressions.Args{"this": stepValue.Copy(), "expression": zero.Copy()}),
				"expression": expressions.LT(expressions.Args{"this": adjustedEnd.Copy(), "expression": start.Copy()}),
			}),
			"expression": expressions.And(expressions.Args{
				"this":       expressions.LT(expressions.Args{"this": stepValue.Copy(), "expression": zero.Copy()}),
				"expression": expressions.GT(expressions.Args{"this": adjustedEnd.Copy(), "expression": start.Copy()}),
			}),
		}),
	})
	return g.gen(expressions.If(expressions.Args{
		"this": shouldReturnEmpty, "true": expressions.Array(expressions.Args{}), "false": sequence(adjustedEnd),
	}))
}

// hiveIntLiteral returns the integer value of a literal, including a leading negation.
func hiveIntLiteral(e expressions.Expression) (int, bool) {
	if e == nil {
		return 0, false
	}
	if e.Kind() == expressions.KindNeg {
		v, ok := hiveIntLiteral(asExpression(e.This()))
		return -v, ok
	}
	if !e.IsInt() {
		return 0, false
	}
	v, err := strconv.Atoi(e.Name())
	return v, err == nil
}

// hiveSplitSQL ports the Hive SPLIT transform: quote the delimiter with \Q...\E.
func hiveSplitSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("SPLIT", []any{e.This(), hiveQuotedRegex(e.Arg("expression"))}, "(", ")", true)
}

func hiveQuotedRegex(arg any) expressions.Expression {
	inner := asExpression(arg)
	if inner != nil && inner.Kind() == expressions.KindLiteral && inner.IsString() {
		// Java's Pattern.quote handles an embedded \E by closing, escaping and reopening the
		// quote: \E -> \E\\E\Q.
		text := strings.ReplaceAll(inner.Name(), `\E`, `\E\\E\Q`)
		inner = expressions.LiteralString(text)
	}
	return expressions.Concat(expressions.Args{"expressions": []expressions.Expression{
		expressions.LiteralString(`\Q`), inner, expressions.LiteralString(`\E`),
	}})
}

// hiveUnixToTimeSQL ports the Hive UNIX_TIMESTAMP/FROM_UNIXTIME transform for UnixToTime.
func hiveUnixToTimeSQL(g *Generator, e expressions.Expression) string {
	if scale := asExpression(e.Arg("scale")); scale != nil {
		ts := expressions.Div(expressions.Args{
			"this": e.This(),
			"expression": expressions.Anonymous(expressions.Args{"this": "POWER", "expressions": []expressions.Expression{
				expressions.LiteralNumber("10"), scale,
			}}),
		})
		return g.funcCall("FROM_UNIXTIME", []any{ts}, "(", ")", true)
	}
	return g.funcCall("FROM_UNIXTIME", []any{e.This()}, "(", ")", true)
}

// hiveDateSubSQL ports the Hive DATE_SUB forms: DATE_ADD(x, -n), or ADD_MONTHS(x, -n).
func hiveDateSubSQL(g *Generator, e expressions.Expression) string {
	count := asExpression(e.Arg("expression"))
	if count != nil {
		if count.IsInt() {
			count = expressions.LiteralNumber("-" + count.Name())
		} else {
			count = expressions.Mul(expressions.Args{
				"this": count, "expression": expressions.LiteralNumber("-1"),
			})
		}
	}
	unit := asExpression(e.Arg("unit"))
	if unit != nil && strings.EqualFold(unit.Name(), "MONTH") {
		return g.funcCall("ADD_MONTHS", []any{e.This(), count}, "(", ")", true)
	}
	return g.funcCall("DATE_ADD", []any{e.This(), count}, "(", ")", true)
}

// hiveConcatWsSQL ports concatws_sql's CONCAT_WS_COALESCE branch (generator.py:3726-3736).
// Hive and its descendants (Spark, Databricks) set the flag.
func hiveConcatWsSQL(g *Generator, e expressions.Expression) string {
	if !g.dialect.ConcatWsCoalesce {
		return g.functionFallbackSQL(e)
	}
	args := e.Expressions()
	base := g.funcCall("CONCAT_WS", listToAny(args), "(", ")", true)
	if truthy(e.Arg("coalesce")) {
		return base
	}
	conds := make([]expressions.Expression, 0, len(args))
	for _, arg := range args {
		if arg != nil {
			conds = append(conds, expressions.Is(expressions.Args{"this": arg.Copy(), "expression": expressions.Null()}))
		}
	}
	cond := duckDBConnectorExpr(expressions.KindOr, conds)
	if cond == nil {
		return base
	}
	return "CASE WHEN " + g.gen(cond) + " THEN NULL ELSE " + base + " END"
}

// hiveIndexSQL ports Hive's index rendering: ON TABLE <name>.
func hiveIndexSQL(g *Generator, e expressions.Expression) string {
	unique := ""
	if boolValue(e.Arg("unique")) {
		unique = "UNIQUE "
	}
	primary := ""
	if boolValue(e.Arg("primary")) {
		primary = "PRIMARY "
	}
	name := g.sqlKey(e, "this")
	if name != "" {
		name += " "
	}
	table := g.sqlKey(e, "table")
	if table != "" {
		table = "ON TABLE " + table
	}
	index := ""
	if table == "" {
		index = "INDEX "
	}
	return unique + primary + index + name + table + g.sqlKey(e, "params")
}

// hiveDateDiffUnits ports _date_diff_sql in generators/hive.py:139-161.
func (g *Generator) hiveDateDiffUnits(e expressions.Expression) string {
	unitName := strings.ToLower(e.Text("unit"))
	factor := ""
	switch unitName {
	case "hour":
		factor = " / 3600"
	case "minute":
		factor = " / 60"
	case "millisecond":
		factor = " * 1000"
	}
	if unitName == "second" || factor != "" {
		y := g.sqlKey(e, "this")
		x := g.sqlKey(e, "expression")
		diff := "UNIX_TIMESTAMP(" + y + ") - UNIX_TIMESTAMP(" + x + ")"
		if factor == "" {
			return diff
		}
		return "(" + diff + ")" + factor
	}
	name := "DATEDIFF"
	divisor := ""
	switch unitName {
	case "year":
		name, divisor = "MONTHS_BETWEEN", " / 12"
	case "quarter":
		name, divisor = "MONTHS_BETWEEN", " / 3"
	case "month":
		name = "MONTHS_BETWEEN"
	case "week":
		divisor = " / 7"
	}
	sql := g.funcCall(name, []any{e.This(), e.Expr()}, "(", ")", true)
	if name == "MONTHS_BETWEEN" || divisor != "" {
		return "CAST(" + sql + divisor + " AS INT)"
	}
	return sql
}

// hiveTableSQL ports the Table preprocess chain (generators/hive.py:351): a GENERATE_SERIES
// table reference becomes an UNNEST so it renders as EXPLODE(SEQUENCE(...)).
func hiveTableSQL(g *Generator, e expressions.Expression) string {
	this := asExpression(e.This())
	if this == nil || this.Kind() != expressions.KindGenerateSeries {
		return g.tableSQL(e)
	}
	args := expressions.Args{"expressions": []expressions.Expression{this}}
	if alias := asExpression(e.Arg("alias")); alias != nil {
		args["alias"] = alias.Copy()
	}
	return g.gen(expressions.Unnest(args))
}

// hiveSeriesCastTarget returns the DATE/TIMESTAMP target when one bound is already a cast,
// mirroring sequence_sql's target_type detection (dialects/dialect.py:2401-2406).
func hiveSeriesCastTarget(start, end expressions.Expression) expressions.Expression {
	var cast expressions.Expression
	switch {
	case start != nil && start.Kind() == expressions.KindCast:
		cast = start
	case end != nil && end.Kind() == expressions.KindCast:
		cast = end
	default:
		return nil
	}
	to := asExpression(cast.Arg("to"))
	if to == nil {
		return nil
	}
	dtype, _ := to.Arg("this").(expressions.DType)
	if dtype != expressions.DTypeDate && dtype != expressions.DTypeTimestamp && dtype != expressions.DTypeTimestampTz {
		return nil
	}
	return to
}
