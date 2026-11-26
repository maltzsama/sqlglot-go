package generator

import (
	"strings"

	"github.com/maltzsama/sqlglot-go/dialects"
	"github.com/maltzsama/sqlglot-go/expressions"
)

// MySQL generator property placements, ported from
// .reference/sqlglot-v30.17.0/sqlglot/generators/mysql.py.
func init() {
	// generators/mysql.py:268 exp.VolatileProperty: UNSUPPORTED. The MySQL subclasses
	// (MariaDB, StarRocks, Doris) inherit this through dialectParents, mirroring upstream's
	// generator class hierarchy.
	registerDialectPropertyLocations("mysql", map[expressions.Kind]propertyLocation{
		expressions.KindVolatileProperty: propertyLocationUnsupported,
	})
}

// MySQL function renames (generators/mysql.py TRANSFORMS), shared with the MySQL subclasses.
func init() {
	registerDialectDispatch("mysql", map[expressions.Kind]func(*Generator, expressions.Expression) string{
		expressions.KindLength: clickhouseRename("CHAR_LENGTH"),
		expressions.KindUnixSeconds: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("UNIX_TIMESTAMP", []any{e.This()}, "(", ")", true)
		},
		expressions.KindUnixToTime: func(g *Generator, e expressions.Expression) string {
			scale := asExpression(e.Arg("scale"))
			if scale != nil {
				ts := expressions.Div(expressions.Args{
					"this":       e.This(),
					"expression": expressions.Anonymous(expressions.Args{"this": "POWER", "expressions": []expressions.Expression{expressions.LiteralNumber("10"), scale}}),
				})
				return g.funcCall("FROM_UNIXTIME", []any{ts}, "(", ")", true)
			}
			return g.funcCall("FROM_UNIXTIME", []any{e.This()}, "(", ")", true)
		},
		expressions.KindBitwiseAndAgg: renameFunc("BIT_AND"),
		expressions.KindBitwiseOrAgg:  renameFunc("BIT_OR"),
		expressions.KindBitwiseXorAgg: renameFunc("BIT_XOR"),
		expressions.KindNumberToStr:   renameFunc("FORMAT"),
		expressions.KindStuff:         renameFunc("INSERT"),
		expressions.KindTimeFromParts: renameFunc("MAKETIME"),
		expressions.KindUtcTimestamp:  renameFunc("UTC_TIMESTAMP"),
	})
}

// generators/mysql.py: VERSION() spells CurrentVersion.
func init() {
	registerDialectDispatch("mysql", map[expressions.Kind]func(*Generator, expressions.Expression) string{
		expressions.KindCurrentVersion: renameFunc("VERSION"),
	})
}

func init() {
	registerDialectDispatch("mysql", map[expressions.Kind]func(*Generator, expressions.Expression) string{
		expressions.KindDateAdd:      mysqlDateAddSQL,
		expressions.KindTsOrDsAdd:    mysqlDateAddSQL,
		expressions.KindDateTrunc:    mysqlDateTruncSQL,
		expressions.KindTsOrDsToDate: mysqlTsOrDsToDateSQL,
		expressions.KindStrToDate:    mysqlStrToDateSQL,
		expressions.KindStrToTime:    mysqlStrToDateSQL,
		expressions.KindDay:          mysqlStripDateCast("DAY"),
		expressions.KindMonth:        mysqlStripDateCast("MONTH"),
		expressions.KindYear:         mysqlStripDateCast("YEAR"),
		expressions.KindILike: func(g *Generator, e expressions.Expression) string {
			op := "LIKE"
			if boolValue(e.Arg("negate")) {
				op = "NOT LIKE"
			}
			return "LOWER(" + g.sqlKey(e, "this") + ") " + op + " LOWER(" + g.sqlKey(e, "expression") + ")"
		},
		expressions.KindSelect: func(g *Generator, e expressions.Expression) string {
			if asExpression(e.Arg("qualify")) != nil {
				e = eliminateQualify(e)
			}
			return g.selectSQL(e)
		},
		expressions.KindNullSafeEQ: func(g *Generator, e expressions.Expression) string {
			return g.binary(e, "<=>")
		},
		expressions.KindNullSafeNEQ: func(g *Generator, e expressions.Expression) string {
			return "NOT " + g.binary(e, "<=>")
		},
		expressions.KindIsAscii: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("REGEXP_LIKE", []any{e.This(), expressions.LiteralString("^[[:ascii:]]*$")}, "(", ")", true)
		},
	})
}

// mysqlDateAddSQL ports date_add_sql for MySQL: DATE_ADD(x, INTERVAL n UNIT).
func mysqlDateAddSQL(g *Generator, e expressions.Expression) string {
	count := asExpression(e.Arg("expression"))
	if count != nil && count.Kind() == expressions.KindInterval {
		return "DATE_ADD(" + g.sqlKey(e, "this") + ", " + g.gen(count) + ")"
	}
	unit := asExpression(e.Arg("unit"))
	unitName := "DAY"
	if unit != nil {
		unitName = strings.ToUpper(unit.Name())
	}
	if count == nil {
		count = expressions.LiteralNumber("1")
	}
	countSQL := g.gen(count)
	if count.Kind() != expressions.KindLiteral {
		countSQL = "(" + countSQL + ")"
	}
	return "DATE_ADD(" + g.sqlKey(e, "this") + ", INTERVAL " + countSQL + " " + unitName + ")"
}

// mysqlDateTruncSQL ports MySQL's DATE_TRUNC emulation through STR_TO_DATE/CONCAT.
func mysqlDateTruncSQL(g *Generator, e expressions.Expression) string {
	if g.dialect.Name != "mysql" && g.dialect.Name != "mariadb" {
		return g.functionFallbackSQL(e)
	}
	unit := asExpression(e.Arg("unit"))
	unitName := ""
	if unit != nil {
		unitName = strings.ToUpper(unit.Name())
	}
	x := g.sqlKey(e, "this")
	build := func(parts []string, format string) string {
		return "STR_TO_DATE(CONCAT(" + strings.Join(parts, ", ") + "), '" + format + "')"
	}
	switch unitName {
	case "DAY":
		return "DATE(" + x + ")"
	case "YEAR":
		return build([]string{"YEAR(" + x + ")", "' 1 1'"}, "%Y %c %e")
	case "MONTH":
		return build([]string{"YEAR(" + x + ")", "' '", "MONTH(" + x + ")", "' 1'"}, "%Y %c %e")
	case "QUARTER":
		return build([]string{"YEAR(" + x + ")", "' '", "QUARTER(" + x + ") * 3 - 2", "' 1'"}, "%Y %c %e")
	case "WEEK":
		return build([]string{"YEAR(" + x + ")", "' '", "WEEK(" + x + ", 1)", "' 1'"}, "%Y %u %w")
	}
	return g.functionFallbackSQL(e)
}

// mysqlConvertFormat converts a format literal through MySQL's inverse time mapping.
func mysqlConvertFormat(g *Generator, format expressions.Expression) expressions.Expression {
	if format != nil && format.IsString() {
		if converted, ok := dialects.FormatTime(format.Name(), g.dialect.InverseTimeMapping); ok {
			return expressions.LiteralString(converted)
		}
	}
	return format
}

func mysqlStrToDateSQL(g *Generator, e expressions.Expression) string {
	return g.funcCall("STR_TO_DATE", []any{e.This(), mysqlConvertFormat(g, asExpression(e.Arg("format")))}, "(", ")", true)
}

func mysqlTsOrDsToDateSQL(g *Generator, e expressions.Expression) string {
	if format := asExpression(e.Arg("format")); format != nil {
		return g.funcCall("STR_TO_DATE", []any{e.This(), mysqlConvertFormat(g, format)}, "(", ")", true)
	}
	return "DATE(" + g.sqlKey(e, "this") + ")"
}

// mysqlStripDateCast removes a redundant TS_OR_DS_TO_DATE around DAY/MONTH/YEAR args.
func mysqlStripDateCast(name string) func(*Generator, expressions.Expression) string {
	return func(g *Generator, e expressions.Expression) string {
		this := asExpression(e.This())
		if this != nil && this.Kind() == expressions.KindTsOrDsToDate {
			return g.funcCall(name, []any{this.This()}, "(", ")", true)
		}
		return g.functionFallbackSQL(e)
	}
}
