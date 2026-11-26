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
		expressions.KindBitwiseCount: renameFunc("BIT_COUNT"),
		expressions.KindArrayAgg:     renameFunc("GROUP_CONCAT"),
		expressions.KindIgnoreNulls: func(g *Generator, e expressions.Expression) string {
			// generators/mysql.py:799-802 drops IGNORE NULLS.
			return g.sqlKey(e, "this")
		},
		expressions.KindTimestampTrunc: mysqlTimestampTruncSQL,
		expressions.KindMakeInterval:   mysqlMakeIntervalSQL,
		expressions.KindDateDiff: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("DATEDIFF", []any{mysqlStripDateCastArg(e.This()), mysqlStripDateCastArg(asExpression(e.Arg("expression")))}, "(", ")", true)
		},
		expressions.KindTsOrDsDiff: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("DATEDIFF", []any{mysqlStripDateCastArg(e.This()), mysqlStripDateCastArg(asExpression(e.Arg("expression")))}, "(", ")", true)
		},
		expressions.KindCurrentDate: func(g *Generator, e expressions.Expression) string {
			if zone := g.sqlKey(e, "this"); zone != "" {
				return "CURRENT_DATE AT TIME ZONE " + zone
			}
			return "CURRENT_DATE"
		},
		expressions.KindLastDay: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("LAST_DAY", []any{e.This()}, "(", ")", true)
		},
		expressions.KindTimestampAdd: mysqlTimestampIntervalSQL("DATE_ADD"),
		expressions.KindTimestampSub: mysqlTimestampIntervalSQL("DATE_SUB"),
		expressions.KindConvertTimezone: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("CONVERT_TZ", []any{e.Arg("timestamp"), e.Arg("source_tz"), e.Arg("target_tz")}, "(", ")", true)
		},
		expressions.KindExtract: func(g *Generator, e expressions.Expression) string {
			if unit := asExpression(e.Arg("this")); unit != nil && strings.EqualFold(unit.Name(), "epoch") {
				return g.funcCall("UNIX_TIMESTAMP", []any{e.Arg("expression")}, "(", ")", true)
			}
			return g.extractSQL(e)
		},
		expressions.KindDPipe: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("CONCAT", listToAny(flattenDPipe(e)), "(", ")", true)
		},
		expressions.KindUnicode: func(g *Generator, e expressions.Expression) string {
			return "ORD(CONVERT(" + g.sqlKey(e, "this") + " USING utf32))"
		},
		expressions.KindDateStrToDate: func(g *Generator, e expressions.Expression) string {
			return g.gen(expressions.Cast(expressions.Args{
				"this": e.This(),
				"to":   expressions.DataType(expressions.Args{"this": expressions.DTypeDate}),
			}))
		},
		expressions.KindTimeToStr: func(g *Generator, e expressions.Expression) string {
			return g.funcCall("DATE_FORMAT", []any{mysqlStripDateCastArg(e.This()), mysqlConvertFormat(g, asExpression(e.Arg("format")))}, "(", ")", true)
		},
		expressions.KindTimestampDiff: func(g *Generator, e expressions.Expression) string {
			// generators/mysql.py:199 TIMESTAMPDIFF(unit, expression, this).
			return g.funcCall("TIMESTAMPDIFF", []any{
				unitToVarValue(asExpression(e.Arg("unit"))), e.Arg("expression"), e.This(),
			}, "(", ")", true)
		},
		expressions.KindDateAdd:      mysqlDateAddSQL,
		expressions.KindTsOrDsAdd:    mysqlDateAddSQL,
		expressions.KindDateTrunc:    mysqlDateTruncSQL,
		expressions.KindTsOrDsToDate: mysqlTsOrDsToDateSQL,
		expressions.KindStrToDate:    mysqlStrToDateSQL,
		expressions.KindStrToTime:    mysqlStrToDateSQL,
		expressions.KindDay:          mysqlStripDateCast("DAY"),
		expressions.KindMonth:        mysqlStripDateCast("MONTH"),
		expressions.KindYear:         mysqlStripDateCast("YEAR"),
		expressions.KindAnonymous: func(g *Generator, e expressions.Expression) string {
			switch strings.ToUpper(e.Text("this")) {
			case "TRUNC":
				return g.funcCall("TRUNCATE", listToAny(e.Expressions()), "(", ")", true)
			case "DATE_FORMAT":
				if args := e.Expressions(); len(args) == 2 {
					format := args[1]
					if format.IsString() {
						format = expressions.LiteralString(mysqlNormalizeFormat(g, format.Name()))
					}
					return g.funcCall("DATE_FORMAT", []any{args[0], format}, "(", ")", true)
				}
			case "FORMATDATETIME":
				// ClickHouse's FORMATDATETIME(value, format) is MySQL's
				// DATE_FORMAT(value, format).
				if args := e.Expressions(); len(args) == 2 {
					return g.funcCall("DATE_FORMAT", []any{args[0], args[1]}, "(", ")", true)
				}
			}
			return g.anonymousSQL(e)
		},
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
			if joined := eliminateFullOuterJoin(e); joined != e {
				return g.gen(joined)
			}
			starRocksUnnestGenerateDateArray(g, e)
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
		return "DATE_ADD(" + g.gen(mysqlStripDateCastArg(asExpression(e.This()))) + ", " + g.gen(count) + ")"
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
	if count.Kind() != expressions.KindLiteral && count.Kind() != expressions.KindNeg {
		countSQL = "(" + countSQL + ")"
	}
	return "DATE_ADD(" + g.gen(mysqlStripDateCastArg(asExpression(e.This()))) + ", INTERVAL " + countSQL + " " + unitName + ")"
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

// mysqlNormalizeFormat converts a MySQL DATE_FORMAT token string to strftime and back, so
// equivalent spellings collapse to the canonical MySQL form (%H:%i:%s -> %T).
func mysqlNormalizeFormat(g *Generator, format string) string {
	if forward, ok := dialects.FormatTime(format, g.dialect.TimeMapping); ok {
		format = forward
	}
	if converted, ok := dialects.FormatTime(format, g.dialect.InverseTimeMapping); ok {
		format = converted
	}
	return format
}

// mysqlStripDateCastArg ports remove_ts_or_ds_to_date: a nested TsOrDsToDate without a
// format is replaced by its input.
func mysqlStripDateCastArg(e expressions.Expression) expressions.Expression {
	if e == nil || e.Kind() != expressions.KindTsOrDsToDate || e.Arg("format") != nil {
		return e
	}
	return e.This()
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

// mysqlTimestampTruncSQL ports timestamptrunc_sql (generators/mysql.py:770-783): the
// truncation is expressed as a difference from a very old timestamp plus an interval.
func mysqlTimestampTruncSQL(g *Generator, e expressions.Expression) string {
	unit := e.Arg("unit")
	start := expressions.LiteralString("0000-01-01 00:00:00")
	diff := expressions.New(expressions.KindTimestampDiff, expressions.Args{
		"this":       e.This(),
		"expression": start.Copy(),
		"unit":       unit,
	})
	interval := expressions.Interval(expressions.Args{
		"this": diff,
		"unit": unit,
	})
	return g.gen(expressions.New(expressions.KindDateAdd, expressions.Args{
		"this":       start.Copy(),
		"expression": interval,
		"unit":       unit,
	}))
}

// makeIntervalUnitAliases ports _MAKE_INTERVAL_UNIT_ALIASES (generators/mysql.py:93-103).
var makeIntervalUnitAliases = map[string]string{
	"years": "year", "months": "month", "weeks": "week", "days": "day",
	"hours": "hour", "minutes": "minute", "mins": "minute",
	"seconds": "second", "secs": "second",
}

// mysqlMakeIntervalSQL ports makeinterval_sql (generators/mysql.py:596-620): one INTERVAL
// per named argument, joined by plus or minus depending on the parent subtraction.
func mysqlMakeIntervalSQL(g *Generator, e expressions.Expression) string {
	keys := []string{"year", "month", "week", "day", "hour", "minute", "second"}
	intervals := make([]string, 0, len(keys))
	for _, key := range keys {
		value := asExpression(e.Arg(key))
		if value == nil {
			continue
		}
		unitName := key
		if value.Kind() == expressions.KindKwarg {
			if this := asExpression(value.This()); this != nil {
				name := strings.ToLower(this.Name())
				if alias, ok := makeIntervalUnitAliases[name]; ok {
					name = alias
				}
				unitName = name
			}
			value = value.Expr()
		}
		intervals = append(intervals, g.gen(expressions.Interval(expressions.Args{
			"this": value.Copy(),
			"unit": expressions.Var(expressions.Args{"this": strings.ToUpper(unitName)}),
		})))
	}
	if len(intervals) == 0 {
		return g.functionFallbackSQL(e)
	}
	sep := " + "
	if parent := e.Parent(); parent != nil && parent.Kind() == expressions.KindSub && asExpression(parent.Arg("expression")) == e {
		sep = " - "
	}
	return strings.Join(intervals, sep)
}

// mysqlTimestampIntervalSQL ports date_add_interval_sql (dialects/dialect.py:1691-1697) for
// the TIMESTAMP_ADD/TIMESTAMP_SUB forms.
func mysqlTimestampIntervalSQL(name string) func(*Generator, expressions.Expression) string {
	return func(g *Generator, e expressions.Expression) string {
		interval := expressions.Interval(expressions.Args{
			"this": e.Arg("expression"),
			"unit": unitToVarValue(asExpression(e.Arg("unit"))),
		})
		return name + "(" + g.sqlKey(e, "this") + ", " + g.gen(interval) + ")"
	}
}
