package generator

import (
	"strings"

	"github.com/maltzsama/sqlglot-go/dialects"
	"github.com/maltzsama/sqlglot-go/expressions"
)

// Spark2 registers Spark2Generator's TRANSFORMS (generators/spark2.py:133-211), its method
// overrides (generators/spark2.py:216-263) and the PROPERTIES_LOCATION overrides
// (generators/spark2.py:117-123). Spark and Databricks inherit this table through
// dialectParents (generator/dialect_dispatch.go).
func init() {
	registerDialectPropertyLocations("spark2", map[expressions.Kind]propertyLocation{
		expressions.KindEngineProperty:        propertyLocationUnsupported,
		expressions.KindAutoIncrementProperty: propertyLocationUnsupported,
		expressions.KindCharacterSetProperty:  propertyLocationUnsupported,
		expressions.KindCollateProperty:       propertyLocationUnsupported,
	})

	registerDialectDispatch("spark2", map[expressions.Kind]func(*Generator, expressions.Expression) string{
		expressions.KindApproxDistinct: clickhouseRename("APPROX_COUNT_DISTINCT"),
		// generators/spark2.py:138-140: AGGREGATE(arr, 0, (acc, x) -> acc + x, acc -> acc).
		expressions.KindArraySum:          spark2ArraySumSQL,
		expressions.KindArrayToString:     clickhouseRename("ARRAY_JOIN"),
		expressions.KindArraySlice:        clickhouseRename("SLICE"),
		expressions.KindAtTimeZone:        spark2AtTimeZoneSQL,
		expressions.KindBitwiseLeftShift:  clickhouseRename("SHIFTLEFT"),
		expressions.KindBitwiseRightShift: clickhouseRename("SHIFTRIGHT"),
		// generators/spark2.py:148-156: remove_unique_constraints, then the temporary-storage
		// CTAS rewrite, then the PARTITIONED BY schema fold.
		expressions.KindCreate:         spark2Create,
		expressions.KindDateFromParts:  clickhouseRename("MAKE_DATE"),
		expressions.KindDateTrunc:      spark2DateTruncSQL,
		expressions.KindDayOfYear:      clickhouseRename("DAYOFYEAR"),
		expressions.KindDayOfWeekIso:   spark2DayOfWeekIsoSQL,
		expressions.KindFormat:         clickhouseRename("FORMAT_STRING"),
		expressions.KindFrom:           spark2FromSQL,
		expressions.KindFromTimeZone:   spark2FromTimeZoneSQL,
		expressions.KindJSONFormat:     spark2JSONFormatSQL,
		expressions.KindLogicalOr:      clickhouseRename("BOOL_OR"),
		expressions.KindMap:            spark2MapSQL,
		expressions.KindPivot:          oraclePivot,
		expressions.KindReduce:         clickhouseRename("AGGREGATE"),
		expressions.KindRegexpReplace:  spark2RegexpReplaceSQL,
		expressions.KindSHA2Digest:     spark2SHA2SQL,
		expressions.KindStrToDate:      spark2StrToDateSQL,
		expressions.KindStrToTime:      spark2StrToTimeSQL,
		expressions.KindTimestampTrunc: spark2TimestampTruncSQL,
		expressions.KindUnixToTime:     spark2UnixToTimeSQL,
		expressions.KindVariancePop:    clickhouseRename("VAR_POP"),
		expressions.KindWithinGroup:    spark2WithinGroupSQL,
		// generators/spark2.py:216-263 method overrides.
		expressions.KindStruct:             (*Generator).structSQL,
		expressions.KindCast:               spark2CastSQL,
		expressions.KindTryCast:            spark2CastSQL,
		expressions.KindFileFormatProperty: spark2FileFormatPropertySQL,
		expressions.KindAlterColumn:        spark2AlterColumnSQL,
		expressions.KindRenameColumn:       (*Generator).renameColumnSQL,
		expressions.KindBracket:            spark2BracketSQL,
	})
}

// spark2ArraySumSQL ports the ArraySum lambda (generators/spark2.py:138-140).
func spark2ArraySumSQL(g *Generator, e expressions.Expression) string {
	return "AGGREGATE(" + g.sqlKey(e, "this") + ", 0, (acc, x) -> acc + x, acc -> acc)"
}

// spark2AtTimeZoneSQL ports the AtTimeZone lambda (generators/spark2.py:143-145).
func spark2AtTimeZoneSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("FROM_UTC_TIMESTAMP", []any{e.This(), e.Arg("zone")}, "(", ")", true)
}

// spark2FromTimeZoneSQL ports the FromTimeZone lambda (generators/spark2.py:168-170).
func spark2FromTimeZoneSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("TO_UTC_TIMESTAMP", []any{e.This(), e.Arg("zone")}, "(", ")", true)
}

// spark2Create ports the exp.Create preprocess chain (generators/spark2.py:148-156).
func spark2Create(g *Generator, e expressions.Expression) string {
	return g.createSQL(moveSchemaColumnsToPartitionedBy(
		ctasWithTmpTablesToCreateTmpView(removeUniqueConstraints(e), true)))
}

// spark2DateTruncSQL ports the DateTrunc lambda (generators/spark2.py:158-160).
func spark2DateTruncSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("TRUNC", []any{e.This(), starRocksUnitToStr(g, e)}, "(", ")", true)
}

// spark2DayOfWeekIsoSQL ports the DayOfWeekIso lambda (generators/spark2.py:164).
func spark2DayOfWeekIsoSQL(g *Generator, e expressions.Expression) string {
	return "((" + g.funcCall("DAYOFWEEK", []any{e.This()}, "(", ")", true) + " % 7) + 1)"
}

// spark2FromSQL ports `exp.From: preprocess([_unalias_pivot])` (generators/spark2.py:167).
func spark2FromSQL(g *Generator, e expressions.Expression) string {
	return g.fromSQL(unaliasedPivot(e))
}

// unaliasedPivot ports _unalias_pivot (generators/spark2.py:77-100): Spark rejects a PIVOT
// alias, so a pivoted source keeps the alias by wrapping the pivot in a SELECT * subquery.
func unaliasedPivot(e expressions.Expression) expressions.Expression {
	if e.Kind() != expressions.KindFrom {
		return e
	}
	this := asExpression(e.Arg("this"))
	if this == nil {
		return e
	}
	pivots := listFromValue(this.Arg("pivots"))
	if len(pivots) == 0 {
		return e
	}
	pivot := asExpression(pivots[0])
	if pivot == nil {
		return e
	}
	alias := asExpression(pivot.Arg("alias"))
	if alias == nil {
		return e
	}
	alias.Pop()
	subquery := expressions.Subquery(expressions.Args{
		"this": expressions.Select(expressions.Args{
			"expressions": []expressions.Expression{expressions.Star(nil)},
			"from_":       expressions.From(expressions.Args{"this": this.Copy()}),
		}),
		"alias": alias,
	})
	return expressions.From(expressions.Args{"this": subquery})
}

// spark2MapSQL ports _map_sql (generators/spark2.py:43-50).
func spark2MapSQL(g *Generator, e expressions.Expression) string {
	keys := asExpression(e.Arg("keys"))
	values := asExpression(e.Arg("values"))
	if keys == nil || values == nil {
		return g.funcCall("MAP", nil, "(", ")", true)
	}
	return g.funcCall("MAP_FROM_ARRAYS", []any{keys, values}, "(", ")", true)
}

// spark2RegexpReplaceSQL ports the RegexpReplace lambda (generators/spark2.py:177-183).
func spark2RegexpReplaceSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("REGEXP_REPLACE", []any{
		e.This(), e.Arg("expression"), e.Arg("replacement"), e.Arg("position"),
	}, "(", ")", true)
}

// spark2SHA2SQL ports the SHA2Digest lambda (generators/spark2.py:192-194): the bit length
// defaults to 256.
func spark2SHA2SQL(g *Generator, e expressions.Expression) string {
	var length any = e.Arg("length")
	if length == nil {
		length = expressions.LiteralNumber("256")
	}
	return g.funcCall("SHA2", []any{e.This(), length}, "(", ")", true)
}

// spark2StrToDateSQL ports _str_to_date (generators/spark2.py:53-57): Hive's default date
// format collapses to a one-argument TO_DATE.
func spark2StrToDateSQL(g *Generator, e expressions.Expression) string {
	format, _ := hiveFormatTimeLiteral(e).(expressions.Expression)
	if format != nil && format.Name() == dialects.HiveDateFormat {
		return g.funcCall("TO_DATE", []any{e.This()}, "(", ")", true)
	}
	return g.funcCall("TO_DATE", []any{e.This(), format}, "(", ")", true)
}

// spark2StrToTimeSQL ports the StrToTime lambda (generators/spark2.py:196).
func spark2StrToTimeSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("TO_TIMESTAMP", []any{e.This(), hiveFormatTimeLiteral(e)}, "(", ")", true)
}

// spark2TimestampTruncSQL ports timestamptrunc_sql()'s default (dialects/dialect.py:1700-1709):
// DATE_TRUNC(<unit>, <this>).
func spark2TimestampTruncSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("DATE_TRUNC", []any{starRocksUnitToStr(g, e), e.This()}, "(", ")", true)
}

// spark2UnixToTimeSQL ports _unix_to_time_sql (generators/spark2.py:60-74).
func spark2UnixToTimeSQL(g *Generator, e expressions.Expression) string {
	scale := asExpression(e.Arg("scale"))
	timestamp := e.This()
	if scale == nil {
		return g.gen(expressions.Cast(expressions.Args{
			"this": expressions.Anonymous(expressions.Args{
				"this":        "FROM_UNIXTIME",
				"expressions": []expressions.Expression{timestamp},
			}),
			"to": expressions.DTypeTimestamp.IntoExpr(nil),
		}))
	}
	if scale.IsInt() {
		switch scale.Name() {
		case "0":
			return g.funcCall("TIMESTAMP_SECONDS", []any{timestamp}, "(", ")", true)
		case "3":
			return g.funcCall("TIMESTAMP_MILLIS", []any{timestamp}, "(", ")", true)
		case "6":
			return g.funcCall("TIMESTAMP_MICROS", []any{timestamp}, "(", ")", true)
		}
	}
	unixSeconds := expressions.Div(expressions.Args{
		"this":       timestamp,
		"expression": expressions.Pow(expressions.Args{"this": expressions.LiteralNumber("10"), "expression": scale}),
	})
	return g.funcCall("TIMESTAMP_SECONDS", []any{unixSeconds}, "(", ")", true)
}

// spark2WithinGroupSQL ports `exp.WithinGroup: preprocess([remove_within_group_for_percentiles])`
// (generators/spark2.py:201-203). Upstream's preprocess re-dispatches the rewritten node, so a
// percentile that became ApproxQuantile renders through the normal dispatch rather than
// within_group_sql.
func spark2WithinGroupSQL(g *Generator, e expressions.Expression) string {
	if transformed := removeWithinGroupForPercentiles(e); transformed != e {
		return g.gen(transformed)
	}
	return g.withinGroupSQL(e)
}

// spark2JSONFormatSQL ports _json_format_sql (generators/spark2.py:22-40).
func spark2JSONFormatSQL(g *Generator, e expressions.Expression) string {
	this := asExpression(e.Arg("this"))
	if isParseJSON(this) {
		if inner := asExpression(this.Arg("this")); inner != nil && inner.IsString() {
			// FROM_JSON requires a nested type, so the json string is wrapped in a dummy
			// array to keep "naked" strings like 'a' valid; TO_JSON then re-adds the
			// delimiters, which REGEXP_EXTRACT strips again.
			wrapped := g.gen(expressions.LiteralString("[" + inner.Name() + "]"))
			fromJSON := g.funcCall("FROM_JSON", []any{wrapped, g.funcCall("SCHEMA_OF_JSON", []any{wrapped}, "(", ")", true)}, "(", ")", true)
			return g.funcCall("REGEXP_EXTRACT", []any{"TO_JSON(" + fromJSON + ")", "'^.(.*).$'", "1"}, "(", ")", true)
		}
		return g.gen(this)
	}
	return g.funcCall("TO_JSON", []any{this, e.Arg("options")}, "(", ")", true)
}

// spark2CastSQL ports cast_sql (generators/spark2.py:221-235). TryCast shares it: upstream's
// trycast_sql dispatches back into cast_sql with an empty safe prefix, which this port models
// by registering the same handler for KindTryCast.
func spark2CastSQL(g *Generator, e expressions.Expression) string {
	arg := e.This()
	isJSONExtract := arg != nil &&
		(arg.Kind() == expressions.KindJSONExtract || arg.Kind() == expressions.KindJSONExtractScalar) &&
		!truthy(arg.Arg("variant_extract"))

	if to := asExpression(e.Arg("to")); truthy(to.Arg("nested")) && (isParseJSON(arg) || isJSONExtract) {
		target := arg
		if !isJSONExtract {
			target = asExpression(arg.Arg("this"))
		}
		schema := "'" + g.sqlKey(e, "to") + "'"
		return g.funcCall("FROM_JSON", []any{target, schema}, "(", ")", true)
	}
	if isParseJSON(e) {
		return g.funcCall("TO_JSON", []any{arg}, "(", ")", true)
	}
	return g.castSQLWithPrefix(e, "")
}

// isParseJSON ports is_parse_json (dialects/dialect.py:1958-1961).
func isParseJSON(e expressions.Expression) bool {
	if e == nil {
		return false
	}
	if e.Kind() == expressions.KindParseJSON {
		return true
	}
	switch e.Kind() {
	case expressions.KindCast, expressions.KindTryCast, expressions.KindJSONCast:
		if t := e.Type(); t != nil {
			return t.IsType(expressions.DTypeJSON)
		}
	}
	return false
}

// spark2FileFormatPropertySQL ports fileformatproperty_sql (generators/spark2.py:237-241):
// Hive's STORED AS form when hive_format is set, otherwise the datasource USING form.
func spark2FileFormatPropertySQL(g *Generator, e expressions.Expression) string {
	if truthy(e.Arg("hive_format")) {
		return hiveFileFormatPropertySQL(g, e)
	}
	return "USING " + strings.ToUpper(e.Name())
}

// spark2AlterColumnSQL ports altercolumn_sql (generators/spark2.py:243-254); the fall-through
// is the base Generator (upstream calls super(HiveGenerator, self)), so a type change renders
// ALTER COLUMN ... TYPE.
func spark2AlterColumnSQL(g *Generator, e expressions.Expression) string {
	if truthy(e.Arg("exists")) {
		g.unsupported("ALTER COLUMN IF EXISTS is not supported by this dialect")
	}
	this := g.sqlKey(e, "this")
	newName := g.sqlKey(e, "rename_to")
	if newName == "" {
		newName = this
	}
	if newName != this {
		return "RENAME COLUMN " + this + " TO " + newName
	}
	return g.alterColumnSQL(e)
}

// spark2BracketSQL ports bracket_sql (generators/spark2.py:259-263): an ELEMENT_AT bracket
// (safe=False) renders back as the ELEMENT_AT call.
func spark2BracketSQL(g *Generator, e expressions.Expression) string {
	if safe := e.Arg("safe"); safe != nil && !boolValue(safe) {
		return g.bracketToElementAtSQL(e)
	}
	return g.bracketSQL(e)
}

// bracketToElementAtSQL ports bracket_to_element_at_sql (dialects/dialect.py:1218-1227).
// Upstream's apply_index_offset annotates types and simplifies the shifted index; the
// generator cannot import the optimizer, so only a literal integer index is shifted. The sole
// user-reachable safe=False bracket - ELEMENT_AT's offset=1 - cancels the shift, so this is
// output-identical for parsed SQL.
func (g *Generator) bracketToElementAtSQL(e expressions.Expression) string {
	offset := 0
	switch v := e.Arg("offset").(type) {
	case int:
		offset = 1 - v
	case float64:
		offset = 1 - int(v)
	default:
		offset = 1
	}
	exprs := listFromValue(e.Arg("expressions"))
	var index any
	if len(exprs) > 0 {
		index = exprs[0]
		if offset != 0 && len(exprs) == 1 {
			if first := asExpression(exprs[0]); first != nil && first.IsInt() {
				if n, ok := first.ToPy().(int64); ok {
					index = expressions.LiteralNumber(int(n) + offset)
				}
			}
		}
	}
	return g.funcCall("ELEMENT_AT", []any{e.This(), index}, "(", ")", true)
}
