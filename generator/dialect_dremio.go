package generator

import (
	"strings"

	"github.com/maltzsama/sqlglot-go/dialects"
	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// Dremio generator overrides, ported from
// .reference/sqlglot-v30.17.0/sqlglot/generators/dremio.py.
func init() {
	registerDialectDispatch("dremio", dremioDispatch())
	registerDialectTypeMapping("dremio", map[exp.DType]string{
		exp.DTypeSmallInt:     "INT",
		exp.DTypeTinyInt:      "INT",
		exp.DTypeBinary:       "VARBINARY",
		exp.DTypeText:         "VARCHAR",
		exp.DTypeNChar:        "VARCHAR",
		exp.DTypeChar:         "VARCHAR",
		exp.DTypeTimestampNtz: "TIMESTAMP",
		exp.DTypeDatetime:     "TIMESTAMP",
		exp.DTypeArray:        "LIST",
		exp.DTypeBit:          "BOOLEAN",
	})
}

func dremioDispatch() map[exp.Kind]func(*Generator, exp.Expression) string {
	return map[exp.Kind]func(*Generator, exp.Expression) string{
		exp.KindBitwiseAndAgg:  clickhouseRename("BIT_AND"),
		exp.KindBitwiseOrAgg:   clickhouseRename("BIT_OR"),
		exp.KindToChar:         clickhouseRename("TO_CHAR"),
		exp.KindTimeToStr:      dremioTimeToStrSQL,
		exp.KindDateAdd:        dremioDateDeltaSQL("DATE_ADD"),
		exp.KindDateSub:        dremioDateDeltaSQL("DATE_SUB"),
		exp.KindGenerateSeries: clickhouseRename("ARRAY_GENERATE_RANGE"),
		exp.KindVersion:        dremioVersionSQL,
		exp.KindDataType:       dremioDataTypeSQL,
		exp.KindCast:           dremioCastSQL,
	}
}

// dremioTimeToStrSQL ports the TimeToStr transform (generators/dremio.py:85): the format is
// converted back through INVERSE_TIME_MAPPING and rendered as TO_CHAR(this, format).
func dremioTimeToStrSQL(g *Generator, e exp.Expression) string {
	format := asExpression(e.Arg("format"))
	var args []any
	args = append(args, e.This())
	if format != nil && format.IsString() {
		converted, ok := dialects.FormatTime(format.Name(), g.dialect.InverseTimeMapping)
		if ok {
			format = exp.LiteralString(converted)
		}
	}
	if format != nil {
		args = append(args, format)
	}
	return g.funcCall("TO_CHAR", args, "(", ")", true)
}

// dremioDateDeltaSQL ports _date_delta_sql (generators/dremio.py:19-38): a non-DAY unit is
// rendered as CAST(<amount> AS INTERVAL <unit>).
func dremioDateDeltaSQL(name string) func(*Generator, exp.Expression) string {
	return func(g *Generator, e exp.Expression) string {
		unit := strings.ToUpper(e.Text("unit"))
		if unit == "" || unit == "DAY" {
			return g.funcCall(name, []any{e.This(), asExpression(e.Arg("expression"))}, "(", ")", true)
		}
		interval := "CAST(" + g.sqlKey(e, "expression") + " AS INTERVAL " + unit + ")"
		return name + "(" + g.sqlKey(e, "this") + ", " + interval + ")"
	}
}

// dremioVersionSQL ports version_sql (generators/dremio.py:88-95): time travel renders as
// AT SNAPSHOT / AT TIMESTAMP; range time travel is unsupported.
func dremioVersionSQL(g *Generator, e exp.Expression) string {
	if e.Text("kind") == "AS OF" {
		this := e.Name()
		if this == "VERSION" {
			this = "SNAPSHOT"
		}
		expression := g.sqlKey(e, "expression")
		if expression != "" {
			expression = " " + expression
		}
		return "AT " + this + expression
	}
	g.unsupported("Range time travel is not supported in Dremio")
	return g.versionSQL(e)
}

// dremioDataTypeSQL ports datatype_sql (generators/dremio.py:97-108): time-zone-aware
// TIMESTAMP types are unsupported.
func dremioDataTypeSQL(g *Generator, e exp.Expression) string {
	if dtype, ok := e.Arg("this").(exp.DType); ok &&
		(dtype == exp.DTypeTimestampTz || dtype == exp.DTypeTimestampLtz) {
		g.unsupported("Dremio does not support time-zone-aware TIMESTAMP")
	}
	return g.dataTypeSQL(e)
}

// dremioCastSQL ports cast_sql (generators/dremio.py:110-124): a DATE cast of
// CURRENT_TIMESTAMP AT TIME ZONE 'UTC' renders as CURRENT_DATE_UTC.
func dremioCastSQL(g *Generator, e exp.Expression) string {
	to := asExpression(e.Arg("to"))
	if to != nil && to.Kind() == exp.KindDataType && to.Arg("this") == exp.DTypeDate {
		this := asExpression(e.Arg("this"))
		if this != nil && this.Kind() == exp.KindAtTimeZone {
			inner := this.This()
			zone := asExpression(this.Arg("zone"))
			if inner != nil && inner.Kind() == exp.KindCurrentTimestamp && zone != nil &&
				zone.Kind() == exp.KindLiteral && strings.EqualFold(zone.Name(), "UTC") {
				return "CURRENT_DATE_UTC"
			}
		}
	}
	return g.castSQLWithPrefix(e, "")
}
