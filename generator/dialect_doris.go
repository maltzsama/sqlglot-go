package generator

import (
	"strings"

	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// Doris generator overrides, ported from
// .reference/sqlglot-v30.17.0/sqlglot/generators/doris.py.
func init() {
	registerDialectDispatch("doris", dorisDispatch())
	registerDialectTypeMapping("doris", map[exp.DType]string{
		exp.DTypeText:        "STRING",
		exp.DTypeTimestamp:   "DATETIME",
		exp.DTypeTimestampTz: "DATETIME",
	})
	registerDialectPropertyLocations("doris", map[exp.Kind]propertyLocation{
		exp.KindUniqueKeyProperty:     propertyLocationPostSchema,
		exp.KindPartitionedByProperty: propertyLocationPostSchema,
		exp.KindBuildProperty:         propertyLocationPostSchema,
	})
}

func dorisDispatch() map[exp.Kind]func(*Generator, exp.Expression) string {
	return map[exp.Kind]func(*Generator, exp.Expression) string{
		exp.KindAddMonths: clickhouseRename("MONTHS_ADD"),
		exp.KindApproxDistinct: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "APPROX_COUNT_DISTINCT", e.This())
		},
		exp.KindArgMax:           clickhouseRename("MAX_BY"),
		exp.KindArgMin:           clickhouseRename("MIN_BY"),
		exp.KindArrayAgg:         clickhouseRename("COLLECT_LIST"),
		exp.KindArrayToString:    clickhouseRename("ARRAY_JOIN"),
		exp.KindArrayUniqueAgg:   clickhouseRename("COLLECT_SET"),
		exp.KindDateSub:          func(g *Generator, e exp.Expression) string { return g.mysqlDateAddSQL("SUB", e) },
		exp.KindCurrentDate:      func(g *Generator, e exp.Expression) string { return "CURRENT_DATE()" },
		exp.KindCurrentTimestamp: func(g *Generator, e exp.Expression) string { return "NOW()" },
		exp.KindDateTrunc: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "DATE_TRUNC", e.This(), starRocksUnitToStr(g, e))
		},
		exp.KindEuclideanDistance: clickhouseRename("L2_DISTANCE"),
		exp.KindGroupConcat: func(g *Generator, e exp.Expression) string {
			separator := asExpression(e.Arg("separator"))
			if separator == nil {
				separator = exp.LiteralString(",")
			}
			return clickhouseCall(g, "GROUP_CONCAT", e.This(), separator)
		},
		exp.KindJSONExtractScalar: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "JSON_EXTRACT", e.This(), asExpression(e.Arg("expression")))
		},
		exp.KindLag:         dorisLagLead("LAG"),
		exp.KindLead:        dorisLagLead("LEAD"),
		exp.KindMap:         clickhouseRename("ARRAY_MAP"),
		exp.KindProperty:    starRocksProperty,
		exp.KindRegexpLike:  clickhouseRename("REGEXP"),
		exp.KindRegexpSplit: clickhouseRename("SPLIT_BY_STRING"),
		exp.KindSchemaCommentProperty: func(g *Generator, e exp.Expression) string {
			return "COMMENT " + g.sqlKey(e, "this")
		},
		exp.KindSplit:         clickhouseRename("SPLIT_BY_STRING"),
		exp.KindStringToArray: clickhouseRename("SPLIT_BY_STRING"),
		exp.KindStrToUnix: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "UNIX_TIMESTAMP", e.This(), asExpression(e.Arg("format")))
		},
		exp.KindTimeStrToDate: clickhouseRename("TO_DATE"),
		exp.KindTsOrDsAdd: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "DATE_ADD", e.This(), asExpression(e.Arg("expression")))
		},
		exp.KindTsOrDsToDate: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "TO_DATE", e.This())
		},
		exp.KindTimeToUnix: clickhouseRename("UNIX_TIMESTAMP"),
		exp.KindTimestampTrunc: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "DATE_TRUNC", e.This(), starRocksUnitToStr(g, e))
		},
		exp.KindUnixToStr: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "FROM_UNIXTIME", e.This(), asExpression(e.Arg("format")))
		},
		exp.KindUnixToTime: clickhouseRename("FROM_UNIXTIME"),

		// Doris DDL nodes and the UPDATE/DELETE table alias.
		exp.KindBuildProperty: func(g *Generator, e exp.Expression) string { return "BUILD " + g.sqlKey(e, "this") },
		exp.KindDuplicateKeyProperty: func(g *Generator, e exp.Expression) string {
			return "DUPLICATE KEY (" + g.expressions(exprsOptions{expression: e, flat: true}) + ")"
		},
		exp.KindUniqueKeyProperty:               dorisUniqueKeyProperty,
		exp.KindRefreshTriggerProperty:          dorisRefreshTrigger,
		exp.KindPartitionRange:                  dorisPartitionRange,
		exp.KindPartitionByRangePropertyDynamic: dorisPartitionByRangeDynamic,
		exp.KindPartitionedByProperty:           dorisPartitionedBy,
		exp.KindTable:                           dorisTable,
	}
}

// dorisLagLead ports _lag_lead_sql (generators/doris.py:16-22).
func dorisLagLead(name string) func(*Generator, exp.Expression) string {
	return func(g *Generator, e exp.Expression) string {
		offset := asExpression(e.Arg("offset"))
		if offset == nil {
			offset = exp.LiteralNumber("1")
		}
		def := asExpression(e.Arg("default"))
		if def == nil {
			def = exp.Null()
		}
		return clickhouseCall(g, name, e.This(), offset, def)
	}
}

// dorisUniqueKeyProperty ports uniquekeyproperty_sql (generators/doris.py:566-573): a
// materialized view renders `KEY (...)` instead of `UNIQUE KEY (...)`.
func dorisUniqueKeyProperty(g *Generator, e exp.Expression) string {
	prefix := "UNIQUE KEY"
	if create := e.FindAncestor(exp.KindCreate); create != nil {
		if props := asExpression(create.Arg("properties")); props != nil && props.Find(exp.KindMaterializedProperty) != nil {
			prefix = "KEY"
		}
	}
	return prefix + " (" + g.expressions(exprsOptions{expression: e, flat: true}) + ")"
}

// dorisRefreshTrigger ports refreshtriggerproperty_sql (generator.py:6216-6227).
func dorisRefreshTrigger(g *Generator, e exp.Expression) string {
	method := g.sqlKey(e, "method")
	kind := g.sqlKey(e, "kind")
	if kind == "" {
		return "REFRESH " + method
	}
	every := ""
	if everySQL := g.sqlKey(e, "every"); everySQL != "" {
		every = " EVERY " + everySQL
		if unit := g.sqlKey(e, "unit"); unit != "" {
			every += " " + unit
		}
	}
	starts := ""
	if startsSQL := g.sqlKey(e, "starts"); startsSQL != "" {
		starts = " STARTS " + startsSQL
	}
	return "REFRESH " + method + " ON " + kind + every + starts
}

// dorisPartitionRange ports partitionrange_sql (generators/doris.py:575-590).
func dorisPartitionRange(g *Generator, e exp.Expression) string {
	name := g.sqlKey(e, "this")
	values := e.Expressions()
	if len(values) == 1 && values[0].Kind() != exp.KindTuple {
		return "PARTITION " + name + " VALUES LESS THAN (" + g.gen(values[0]) + ")"
	}
	rendered := make([]string, 0, len(values))
	for _, value := range values {
		if value.Kind() == exp.KindTuple {
			parts := make([]string, 0, len(value.Expressions()))
			for _, inner := range value.Expressions() {
				parts = append(parts, g.gen(inner))
			}
			rendered = append(rendered, "("+strings.Join(parts, ", ")+")")
		} else {
			rendered = append(rendered, "("+g.gen(value)+")")
		}
	}
	return "PARTITION " + name + " VALUES [" + strings.Join(rendered, ", ") + ")"
}

// dorisPartitionByRangeDynamic ports partitionbyrangepropertydynamic_sql
// (generators/doris.py:592-606).
func dorisPartitionByRangeDynamic(g *Generator, e exp.Expression) string {
	start := g.sqlKey(e, "start")
	end := g.sqlKey(e, "end")
	interval := ""
	if every := asExpression(e.Arg("every")); every != nil {
		interval = "INTERVAL " + g.sqlKey(every, "this") + " " + g.sqlKey(every, "unit")
	}
	return "FROM (" + start + ") TO (" + end + ") " + interval
}

// dorisPartitionedBy ports partitionedbyproperty_sql (generators/doris.py:608-612).
func dorisPartitionedBy(g *Generator, e exp.Expression) string {
	this := asExpression(e.Arg("this"))
	if this != nil && this.Kind() == exp.KindSchema {
		return "PARTITION BY (" + g.expressions(exprsOptions{expression: this, flat: true}) + ")"
	}
	return "PARTITION BY (" + g.gen(this) + ")"
}

// dorisTable ports table_sql (generators/doris.py:614-619): UPDATE/DELETE aliases omit AS.
func dorisTable(g *Generator, e exp.Expression) string {
	ancestor := e.FindAncestor(exp.KindUpdate, exp.KindDelete, exp.KindSelect)
	if ancestor == nil || ancestor.Kind() != exp.KindSelect {
		return g.tableSQLSep(e, " ")
	}
	return g.tableSQLSep(e, " AS ")
}
