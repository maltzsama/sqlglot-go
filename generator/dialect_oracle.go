package generator

import (
	"strings"

	"github.com/maltzsama/sqlglot-go/dialects"
	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// Oracle generator overrides, ported from
// .reference/sqlglot-v30.17.0/sqlglot/generators/oracle.py.
func init() {
	registerDialectDispatch("oracle", oracleDispatch())
	registerDialectTypeMapping("oracle", map[exp.DType]string{
		exp.DTypeBlob:         "BLOB",
		exp.DTypeTinyInt:      "SMALLINT",
		exp.DTypeSmallInt:     "SMALLINT",
		exp.DTypeInt:          "INT",
		exp.DTypeBigInt:       "INT",
		exp.DTypeDecimal:      "NUMBER",
		exp.DTypeDouble:       "DOUBLE PRECISION",
		exp.DTypeVarchar:      "VARCHAR2",
		exp.DTypeNVarchar:     "NVARCHAR2",
		exp.DTypeNChar:        "NCHAR",
		exp.DTypeText:         "CLOB",
		exp.DTypeTimeTz:       "TIME",
		exp.DTypeTimestampNtz: "TIMESTAMP",
		exp.DTypeTimestampTz:  "TIMESTAMP",
		exp.DTypeBinary:       "BLOB",
		exp.DTypeVarBinary:    "BLOB",
		exp.DTypeRowVersion:   "BLOB",
	})
	registerDialectPropertyLocations("oracle", map[exp.Kind]propertyLocation{
		exp.KindVolatileProperty: propertyLocationUnsupported,
	})
}

func oracleDispatch() map[exp.Kind]func(*Generator, exp.Expression) string {
	return map[exp.Kind]func(*Generator, exp.Expression) string{
		exp.KindGroupConcat: func(g *Generator, e exp.Expression) string {
			return g.trinoGroupConcatSQL(e)
		},
		exp.KindDateStrToDate: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "TO_DATE", e.This(), exp.LiteralString("YYYY-MM-DD"))
		},
		exp.KindDateTrunc: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "TRUNC", e.This(), asExpression(e.Arg("unit")))
		},
		exp.KindEuclideanDistance: clickhouseRename("L2_DISTANCE"),
		exp.KindILike: func(g *Generator, e exp.Expression) string {
			return "LOWER(" + g.sqlKey(e, "this") + ") LIKE LOWER(" + g.sqlKey(e, "expression") + ")"
		},
		exp.KindLogicalOr: clickhouseRename("MAX"),
		exp.KindMod:       clickhouseRename("MOD"),
		exp.KindRand: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "DBMS_RANDOM.VALUE",
				asExpression(e.Arg("lower")), asExpression(e.Arg("upper")))
		},
		exp.KindForceProperty: func(g *Generator, e exp.Expression) string { return "FORCE" },
		exp.KindConvertToCharset: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "CONVERT", e.This(), asExpression(e.Arg("dest")), asExpression(e.Arg("source")))
		},
		exp.KindStrPosition: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "INSTR",
				e.This(),
				asExpression(e.Arg("substr")),
				asExpression(e.Arg("position")),
				asExpression(e.Arg("occurrence")),
			)
		},
		exp.KindStrToTime: oracleStrToTime,
		exp.KindStrToDate: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "TO_DATE", e.This(), oracleFormatTimeArg(e))
		},
		exp.KindSubquery:  func(g *Generator, e exp.Expression) string { return g.subquerySQLSep(e, " ") },
		exp.KindSubstring: clickhouseRename("SUBSTR"),
		exp.KindTable:     func(g *Generator, e exp.Expression) string { return g.tableSQLSep(e, " ") },
		exp.KindTableSample: func(g *Generator, e exp.Expression) string {
			return g.tablesampleSQL(e)
		},
		exp.KindTemporaryProperty: func(g *Generator, e exp.Expression) string {
			name := e.Name()
			if name == "" {
				name = "GLOBAL"
			}
			return name + " TEMPORARY"
		},
		exp.KindTimeToStr: func(g *Generator, e exp.Expression) string {
			return clickhouseCall(g, "TO_CHAR", e.This(), oracleFormatTimeArg(e))
		},
		exp.KindToChar: func(g *Generator, e exp.Expression) string { return g.functionFallbackSQL(e) },
		exp.KindTrim:   oracleTrim,
		exp.KindUnicode: func(g *Generator, e exp.Expression) string {
			return "ASCII(UNISTR(" + g.sqlKey(e, "this") + "))"
		},
		exp.KindUnixToTime: func(g *Generator, e exp.Expression) string {
			return "TO_DATE('1970-01-01', 'YYYY-MM-DD') + (" + g.sqlKey(e, "this") + " / 86400)"
		},
		exp.KindUtcTimestamp:     clickhouseRename("UTC_TIMESTAMP"),
		exp.KindSystimestamp:     func(g *Generator, e exp.Expression) string { return "SYSTIMESTAMP" },
		exp.KindCurrentTimestamp: oracleCurrentTimestamp,
		exp.KindOffset:           func(g *Generator, e exp.Expression) string { return g.offsetSQL(e) + " ROWS" },
		exp.KindQueryOption:      oracleQueryOption,
		exp.KindCoalesce:         oracleCoalesce,
		exp.KindInto:             oracleInto,
		exp.KindHint:             oracleHint,
		exp.KindIsAscii: func(g *Generator, e exp.Expression) string {
			return "NVL(REGEXP_LIKE(" + g.sqlKey(e, "this") + ", '^[' || CHR(1) || '-' || CHR(127) || ']*$'), TRUE)"
		},
		exp.KindInterval: oracleInterval,
		exp.KindToNumber: oracleToNumber,
		exp.KindPivot:    oraclePivot,
		exp.KindColumnDef: func(g *Generator, e exp.Expression) string {
			return g.columnDefSQL(e)
		},
		// TZ_TO_WITH_TIME_ZONE (generator.py:1790-1792).
		exp.KindDataType: tzToWithTimeZone,
	}
}

// oracleFormatTimeArg converts the stored strftime format back to Oracle's spelling.
func oracleFormatTimeArg(e exp.Expression) exp.Expression {
	format := asExpression(e.Arg("format"))
	if format == nil || !format.IsString() {
		return format
	}
	return exp.LiteralString(dialects.OracleTimeFormat(format.Name()))
}

// oracleCurrentTimestamp ports currenttimestamp_sql (generators/oracle.py:114-119).
func oracleCurrentTimestamp(g *Generator, e exp.Expression) string {
	if boolValue(e.Arg("sysdate")) {
		return "SYSDATE"
	}
	if this := asExpression(e.Arg("this")); this != nil {
		return clickhouseCall(g, "CURRENT_TIMESTAMP", this)
	}
	return "CURRENT_TIMESTAMP"
}

// oracleQueryOption ports queryoption_sql (generators/oracle.py:127-132).
func oracleQueryOption(g *Generator, e exp.Expression) string {
	option := g.sqlKey(e, "this")
	value := g.sqlKey(e, "expression")
	if value != "" {
		value = " CONSTRAINT " + value
	}
	return option + value
}

// oracleCoalesce ports coalesce_sql (generators/oracle.py:134-136).
func oracleCoalesce(g *Generator, e exp.Expression) string {
	name := "COALESCE"
	if boolValue(e.Arg("is_nvl")) {
		name = "NVL"
	}
	return g.funcCall(name, g.fallbackArgs(e), "(", ")", true)
}

// oracleInto ports into_sql (generators/oracle.py:138-143).
func oracleInto(g *Generator, e exp.Expression) string {
	into := "INTO"
	if boolValue(e.Arg("bulk_collect")) {
		into = "BULK COLLECT INTO"
	}
	if this := asExpression(e.Arg("this")); this != nil {
		return g.seg(into) + " " + g.gen(this)
	}
	return g.seg(into) + " " + g.expressions(exprsOptions{expression: e})
}

// oracleHint ports hint_sql (generators/oracle.py:145-155).
func oracleHint(g *Generator, e exp.Expression) string {
	var parts []string
	for _, item := range listFromValue(e.Arg("expressions")) {
		hint, ok := item.(exp.Expression)
		if !ok || isNilExpression(hint) {
			// The fallback form stores the raw hint text as a string.
			if s, isString := item.(string); isString && s != "" {
				parts = append(parts, s)
			}
			continue
		}
		if hint.Kind() == exp.KindAnonymous {
			args := g.formatArgs(exprAnySlice(hint.Expressions()), " ")
			parts = append(parts, g.sqlKey(hint, "this")+"("+args+")")
		} else {
			parts = append(parts, g.gen(hint))
		}
	}
	body := strings.Join(parts, " ")
	if g.pretty && len(parts) > 1 {
		sqls := make([]any, len(parts))
		for i, item := range parts {
			sqls[i] = item
		}
		body = strings.TrimSpace(g.expressions(exprsOptions{sqls: sqls, sep: " "}))
	}
	return " /*+ " + strings.TrimSpace(body) + " */"
}

func exprAnySlice(items []exp.Expression) []any {
	out := make([]any, len(items))
	for i, item := range items {
		out[i] = item
	}
	return out
}

// oracleInterval ports interval_sql (generators/oracle.py:160-161).
func oracleInterval(g *Generator, e exp.Expression) string {
	prefix := ""
	if this := asExpression(e.Arg("this")); this != nil && this.Kind() == exp.KindLiteral {
		prefix = "INTERVAL "
	}
	return prefix + g.sqlKey(e, "this") + " " + g.sqlKey(e, "unit")
}

// oracleToNumber ports tonumber_sql (generators/oracle.py:163-171).
func oracleToNumber(g *Generator, e exp.Expression) string {
	this := g.sqlKey(e, "this")
	if def := g.sqlKey(e, "default"); def != "" {
		this += " DEFAULT " + def + " ON CONVERSION ERROR"
	}
	args := []string{this}
	if format := g.sqlKey(e, "format"); format != "" {
		args = append(args, format)
	}
	if nlsparam := g.sqlKey(e, "nlsparam"); nlsparam != "" {
		args = append(args, nlsparam)
	}
	return "TO_NUMBER(" + strings.Join(args, ", ") + ")"
}

// oracleStrToTime ports the StrToTime transform (generators/oracle.py:90).
func oracleStrToTime(g *Generator, e exp.Expression) string {
	return clickhouseCall(g, "TO_TIMESTAMP", e.This(), oracleFormatTimeArg(e))
}

// oracleTrim ports _trim_sql (generators/oracle.py:14-20). Both branches call the same
// standard trim helper upstream, so the standard form always applies.
func oracleTrim(g *Generator, e exp.Expression) string {
	return g.trimSQLStandard(e)
}

// oraclePivot applies the unqualify_pivot_fields preprocess
// (transforms.py:741-755) before the base PIVOT rendering.
func oraclePivot(g *Generator, e exp.Expression) string {
	if fields, ok := e.Arg("fields").([]exp.Expression); ok {
		unqualified := make([]exp.Expression, len(fields))
		for i, field := range fields {
			unqualified[i] = unqualifyColumnsDeep(field)
		}
		e.Set("fields", unqualified)
	}
	return g.pivotSQL(e)
}

// unqualifyColumnsDeep ports transforms.unqualify_columns: every qualified Column in the
// subtree becomes its bare name.
func unqualifyColumnsDeep(e exp.Expression) exp.Expression {
	if e == nil {
		return e
	}
	if e.Kind() == exp.KindColumn {
		if e.Arg("table") != nil || e.Arg("schema") != nil || e.Arg("catalog") != nil {
			return exp.Column(exp.Args{"this": e.This()})
		}
		return e
	}
	for _, key := range exp.ArgKeys(e.Kind()) {
		switch v := e.Arg(key).(type) {
		case exp.Expression:
			e.Set(key, unqualifyColumnsDeep(v))
		case []exp.Expression:
			items := make([]exp.Expression, len(v))
			for i, item := range v {
				items[i] = unqualifyColumnsDeep(item)
			}
			e.Set(key, items)
		}
	}
	return e
}
