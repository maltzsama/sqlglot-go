package generator

import (
	"strings"

	"github.com/maltzsama/sqlglot-go/expressions"
)

// Spark registers SparkGenerator's TYPE_MAPPING additions (generators/spark.py:76-83), its
// TRANSFORMS (generators/spark.py:91-147) and its method overrides
// (generators/spark.py:149-207). Databricks inherits this table through dialectParents.
func init() {
	registerDialectTypeMapping("spark", map[expressions.DType]string{
		expressions.DTypeMoney:        "DECIMAL",
		expressions.DTypeSmallMoney:   "DECIMAL",
		expressions.DTypeUUID:         "STRING",
		expressions.DTypeTimestampLtz: "TIMESTAMP_LTZ",
		expressions.DTypeTimestampNtz: "TIMESTAMP_NTZ",
	})

	registerDialectDispatch("spark", map[expressions.Kind]func(*Generator, expressions.Expression) string{
		expressions.KindArrayInsert: sparkArrayInsertSQL,
		// The BIT_* aggregates and BIT_COUNT are plain renames.
		expressions.KindBitwiseAndAgg: clickhouseRename("BIT_AND"),
		expressions.KindBitwiseOrAgg:  clickhouseRename("BIT_OR"),
		expressions.KindBitwiseXorAgg: clickhouseRename("BIT_XOR"),
		expressions.KindBitwiseCount:  clickhouseRename("BIT_COUNT"),
		// generators/spark.py:107-115: the Spark 3 CREATE preprocess chain replaces the
		// Spark 2 PARTITIONED BY schema fold with move_partitioned_by_to_schema_columns.
		expressions.KindCreate:                sparkCreate,
		expressions.KindCurrentVersion:        clickhouseRename("VERSION"),
		expressions.KindDateFromUnixDate:      clickhouseRename("DATE_FROM_UNIX_DATE"),
		expressions.KindGroupConcat:           sparkGroupConcat,
		expressions.KindEndsWith:              clickhouseRename("ENDSWITH"),
		expressions.KindStartsWith:            clickhouseRename("STARTSWITH"),
		expressions.KindJSONKeys:              clickhouseRename("JSON_OBJECT_KEYS"),
		expressions.KindPartitionedByProperty: sparkPartitionedByProperty,
		expressions.KindSafeAdd:               clickhouseRename("TRY_ADD"),
		expressions.KindSafeDivide:            clickhouseRename("TRY_DIVIDE"),
		expressions.KindSafeMultiply:          clickhouseRename("TRY_MULTIPLY"),
		expressions.KindSafeSubtract:          clickhouseRename("TRY_SUBTRACT"),
		expressions.KindTimestampFromParts:    clickhouseRename("MAKE_TIMESTAMP"),
		expressions.KindTsOrDsAdd:             sparkDateAddSQL,
		expressions.KindTimestampAdd:          sparkDateAddSQL,
		expressions.KindTimestampDiff:         sparkTimestampDiffSQL,
		// generators/spark.py:139-141: a safe TryCast renders TRY_CAST, otherwise the Spark 2
		// cast_sql override applies unchanged.
		expressions.KindTryCast: sparkTryCastSQL,
		// generators/spark.py:142-144 removes the inherited mappings so the base behavior
		// applies: AnyValue keeps its own name, DateDiff uses datediff_sql, With the base
		// with_sql.
		expressions.KindAnyValue: func(g *Generator, e expressions.Expression) string {
			return g.functionFallbackSQL(e)
		},
		expressions.KindDateDiff: sparkDateDiffSQL,
		// generators/spark.py:149-157 restores the base IGNORE NULLS rendering and renders a
		// safe bracket back as TRY_ELEMENT_AT.
		expressions.KindIgnoreNulls: (*Generator).ignoreNullsSQL,
		expressions.KindBracket:     sparkBracketSQL,
		// generators/spark.py:159-186.
		expressions.KindComputedColumnConstraint: sparkComputedColumnConstraintSQL,
		expressions.KindPlaceholder:              sparkPlaceholderSQL,
		expressions.KindReadParquet:              sparkReadParquetSQL,
	})
}

// sparkCreate ports the exp.Create preprocess chain (generators/spark.py:107-115).
func sparkCreate(g *Generator, e expressions.Expression) string {
	return g.createSQL(movePartitionedByToSchemaColumns(
		ctasWithTmpTablesToCreateTmpView(removeUniqueConstraints(e), true)))
}

// sparkArrayInsertSQL ports the ArrayInsert lambda (generators/spark.py:98-100).
func sparkArrayInsertSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("ARRAY_INSERT", []any{e.This(), e.Arg("position"), e.Arg("expression")}, "(", ")", true)
}

// sparkGroupConcat ports groupconcat_sql (dialects/dialect.py:2539-2583) for Spark 3+, whose
// default dialect version is >= 4: LISTAGG(this, separator), with a trailing ORDER pushed into
// a WITHIN GROUP clause.
func sparkGroupConcat(g *Generator, e expressions.Expression) string {
	this := asExpression(e.Arg("this"))
	separator := g.sqlKey(e, "separator")
	if separator == "" {
		separator = "','"
	}

	var limit expressions.Expression
	if this != nil && this.Kind() == expressions.KindLimit && this.This() != nil {
		limit = this
		this = this.This().Pop()
	}

	var order expressions.Expression
	if this != nil {
		order = this.Find(expressions.KindOrder)
	}
	if order != nil && order.This() != nil {
		this = order.This().Pop()
	}

	args := []string{g.gen(this), separator}
	listagg := "LISTAGG(" + strings.Join(args, ", ")
	if limit != nil {
		listagg += g.gen(limit)
	}
	listagg += ")"
	if order != nil {
		return listagg + " WITHIN GROUP (" + g.gen(order) + ")"
	}
	return listagg
}

// sparkPartitionedByProperty ports the PartitionedByProperty lambda (generators/spark.py:
// 123-125), including _normalize_partition (generators/spark.py:23-29).
func sparkPartitionedByProperty(g *Generator, e expressions.Expression) string {
	var parts []any
	if this := asExpression(e.Arg("this")); this != nil {
		for _, item := range this.Expressions() {
			if item.Kind() == expressions.KindLiteral {
				parts = append(parts, g.gen(expressions.ToIdentifier(item.Name())))
				continue
			}
			parts = append(parts, g.gen(item))
		}
	}
	return "PARTITIONED BY " + g.wrap(g.expressions(exprsOptions{sqls: parts, skipFirst: true}))
}

// sparkDateAddSQL ports _dateadd_sql (generators/spark.py:32-53).
func sparkDateAddSQL(g *Generator, e expressions.Expression) string {
	unit := asExpression(e.Arg("unit"))
	if unit == nil || (e.Kind() == expressions.KindTsOrDsAdd && strings.ToUpper(e.Text("unit")) == "DAY") {
		return g.funcCall("DATE_ADD", []any{e.This(), e.Arg("expression")}, "(", ")", true)
	}
	dateAdd := g.funcCall("DATE_ADD", []any{unitToVarValue(unit), e.Arg("expression"), e.This()}, "(", ")", true)
	if e.Kind() != expressions.KindTsOrDsAdd {
		return dateAdd
	}
	// TsOrDsAdd.return_type (expressions/temporal.py:147-155) defaults to DATE.
	returnType := asExpression(e.Arg("return_type"))
	if returnType == nil {
		returnType = expressions.DTypeDate.IntoExpr(nil)
	}
	if !returnType.IsType(expressions.DTypeTimestamp, expressions.DTypeDatetime) {
		return "CAST(" + dateAdd + " AS " + g.gen(returnType) + ")"
	}
	return dateAdd
}

// sparkDateDiffSQL ports datediff_sql (generators/spark.py:165-172).
func sparkDateDiffSQL(g *Generator, e expressions.Expression) string {
	end := g.sqlKey(e, "this")
	start := g.sqlKey(e, "expression")
	if unit := asExpression(e.Arg("unit")); unit != nil {
		return g.funcCall("DATEDIFF", []any{unitToVarValue(unit), start, end}, "(", ")", true)
	}
	return g.funcCall("DATEDIFF", []any{end, start}, "(", ")", true)
}

// sparkTimestampDiffSQL ports timestampdiff_sql (dialects/dialect.py:2519-2520):
// TIMESTAMPDIFF(unit, expression, this).
func sparkTimestampDiffSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("TIMESTAMPDIFF", []any{
		asExpression(e.Arg("unit")), e.Arg("expression"), e.This(),
	}, "(", ")", true)
}

// sparkTryCastSQL ports the TryCast lambda (generators/spark.py:139-141): a safe cast renders
// TRY_CAST through the Spark 2 cast_sql override, which in turn hands the prefix to the base.
func sparkTryCastSQL(g *Generator, e expressions.Expression) string {
	if truthy(e.Arg("safe")) {
		return spark2CastSQLWithPrefix(g, e, "TRY_")
	}
	return spark2CastSQL(g, e)
}

// sparkBracketSQL ports bracket_sql (generators/spark.py:152-157): a safe bracket renders back
// as TRY_ELEMENT_AT, everything else defers to the Spark 2 override.
func sparkBracketSQL(g *Generator, e expressions.Expression) string {
	if truthy(e.Arg("safe")) {
		var key any
		if index := g.bracketOffsetExpression(e, 1); index != nil {
			key = index
		}
		return g.funcCall("TRY_ELEMENT_AT", []any{e.This(), key}, "(", ")", true)
	}
	return spark2BracketSQL(g, e)
}

// sparkComputedColumnConstraintSQL ports computedcolumnconstraint_sql
// (generators/spark.py:159-160).
func sparkComputedColumnConstraintSQL(g *Generator, e expressions.Expression) string {
	return "GENERATED ALWAYS AS (" + g.sqlKey(e, "this") + ")"
}

// sparkPlaceholderSQL ports placeholder_sql (generators/spark.py:174-178): a widget parameter
// keeps its curly braces.
func sparkPlaceholderSQL(g *Generator, e expressions.Expression) string {
	if !truthy(e.Arg("widget")) {
		return g.placeholderSQL(e)
	}
	return "{" + e.Name() + "}"
}

// sparkReadParquetSQL ports readparquet_sql (generators/spark.py:180-186).
func sparkReadParquetSQL(g *Generator, e expressions.Expression) string {
	if len(e.Expressions()) != 1 {
		g.unsupported("READ_PARQUET with multiple arguments is not supported")
		return ""
	}
	return "parquet.`" + e.Expressions()[0].Name() + "`"
}
