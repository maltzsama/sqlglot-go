package generator

import (
	"strconv"
	"strings"

	"github.com/maltzsama/sqlglot-go/dialects"
	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// Teradata generator overrides, ported from
// .reference/sqlglot-v30.17.0/sqlglot/generators/teradata.py.
func init() {
	registerDialectDispatch("teradata", teradataDispatch())
	registerDialectTypeMapping("teradata", map[exp.DType]string{
		exp.DTypeGeometry:    "ST_GEOMETRY",
		exp.DTypeDouble:      "DOUBLE PRECISION",
		exp.DTypeTimestampTz: "TIMESTAMP",
	})
	registerDialectPropertyLocations("teradata", map[exp.Kind]propertyLocation{
		exp.KindOnCommitProperty:      propertyLocationPostIndex,
		exp.KindPartitionedByProperty: propertyLocationPostExpression,
		exp.KindStabilityProperty:     propertyLocationPostCreate,
	})
}

func teradataDispatch() map[exp.Kind]func(*Generator, exp.Expression) string {
	return map[exp.Kind]func(*Generator, exp.Expression) string{
		exp.KindArgMax:           clickhouseRename("MAX_BY"),
		exp.KindArgMin:           clickhouseRename("MIN_BY"),
		exp.KindMax:              teradataMaxOrGreatest,
		exp.KindMin:              teradataMinOrLeast,
		exp.KindPow:              func(g *Generator, e exp.Expression) string { return g.binary(e, "**") },
		exp.KindRand:             teradataRandomSQL,
		exp.KindSelect:           teradataSelect,
		exp.KindStrPosition:      instrStrPositionSQL,
		exp.KindStrToDate:        teradataStrToDateSQL,
		exp.KindToChar:           func(g *Generator, e exp.Expression) string { return g.functionFallbackSQL(e) },
		exp.KindToNumber:         teradataToNumberSQL,
		exp.KindUse:              func(g *Generator, e exp.Expression) string { return "DATABASE " + g.sqlKey(e, "this") },
		exp.KindDateAdd:          teradataDateDeltaSQL("+"),
		exp.KindDateSub:          teradataDateDeltaSQL("-"),
		exp.KindQuarter:          teradataQuarterSQL,
		exp.KindExtract:          teradataExtractSQL,
		exp.KindInterval:         teradataIntervalSQL,
		exp.KindMod:              func(g *Generator, e exp.Expression) string { return g.binary(e, "MOD") },
		exp.KindCast:             teradataCastSQL,
		exp.KindTryCast:          func(g *Generator, e exp.Expression) string { return g.castSQLWithPrefix(e, "TRY") },
		exp.KindCurrentTimestamp: teradataCurrentTimestampSQL,
		exp.KindTableSample:      teradataTableSampleSQL,
		exp.KindPartitionedByProperty: func(g *Generator, e exp.Expression) string {
			return "PARTITION BY " + g.sqlKey(e, "this")
		},
		exp.KindRangeN:   teradataRangeNSQL,
		exp.KindUpdate:   teradataUpdateSQL,
		exp.KindDataType: teradataDataTypeSQL,
		exp.KindLockingStatement: func(g *Generator, e exp.Expression) string {
			return g.sqlKey(e, "this") + " " + g.sqlKey(e, "expression")
		},
	}
}

// teradataMaxOrGreatest ports max_or_greatest (dialects/dialect.py:1807-1809).
func teradataMaxOrGreatest(g *Generator, e exp.Expression) string {
	name := "MAX"
	if len(e.Expressions()) > 0 {
		name = "GREATEST"
	}
	return g.funcCall(name, g.fallbackArgs(e), "(", ")", true)
}

// teradataMinOrLeast ports min_or_least (dialects/dialect.py:1802-1804).
func teradataMinOrLeast(g *Generator, e exp.Expression) string {
	name := "MIN"
	if len(e.Expressions()) > 0 {
		name = "LEAST"
	}
	return g.funcCall(name, g.fallbackArgs(e), "(", ")", true)
}

// teradataRandomSQL ports the Rand transform (generators/teradata.py:94).
func teradataRandomSQL(g *Generator, e exp.Expression) string {
	return g.funcCall("RANDOM", []any{asExpression(e.Arg("lower")), asExpression(e.Arg("upper"))}, "(", ")", true)
}

// teradataSelect applies the generators/teradata.py:95-97 exp.Select preprocess chain,
// transforms.preprocess([eliminate_distinct_on, eliminate_semi_and_anti_joins]), before
// the base SELECT rendering.
func teradataSelect(g *Generator, e exp.Expression) string {
	e = starRocksEliminateDistinctOn(g, e)
	e = eliminateSemiAndAntiJoins(e)
	return g.selectSQL(e)
}

// eliminateSemiAndAntiJoins ports transforms.eliminate_semi_and_anti_joins
// (transforms.py:615-629): every SEMI/ANTI join carrying an ON clause becomes an
// EXISTS / NOT EXISTS subquery over SELECT 1 in the WHERE clause, and the join is
// dropped from the join list.
func eliminateSemiAndAntiJoins(expression exp.Expression) exp.Expression {
	if expression.Kind() != exp.KindSelect {
		return expression
	}
	// Snapshot first: the join list is mutated in place below (upstream iterates a
	// list(...) copy too, transforms.py:619).
	joins := listFromValue(expression.Arg("joins"))
	for _, value := range joins {
		join, ok := value.(exp.Expression)
		if !ok || isNilExpression(join) {
			continue
		}
		kind := join.Text("kind")
		on := asExpression(join.Arg("on"))
		if on == nil || (kind != "SEMI" && kind != "ANTI") {
			continue
		}
		subquery := exp.Select(exp.Args{
			"expressions": []exp.Expression{exp.LiteralNumber(1)},
			"from_":       exp.From(exp.Args{"this": join.This()}),
			"where":       exp.Where(exp.Args{"this": on}),
		})
		condition := exp.Exists(exp.Args{"this": subquery})
		if kind == "ANTI" {
			condition = exp.Not(exp.Args{"this": condition})
		}
		join.Pop()
		// Select.where with append (query.py:265 -> core.py:2735-2758): the new
		// condition is ANDed with the existing WHERE body and re-wrapped in Where.
		if where := asExpression(expression.Arg("where")); where != nil {
			expression.Set("where", exp.Where(exp.Args{"this": exp.And(exp.Args{
				"this":       where.This(),
				"expression": condition,
			})}))
		} else {
			expression.Set("where", exp.Where(exp.Args{"this": condition}))
		}
	}
	return expression
}

// teradataStrToDateSQL ports the StrToDate transform (generators/teradata.py:100-102):
// CAST(this AS DATE FORMAT <format>).
func teradataStrToDateSQL(g *Generator, e exp.Expression) string {
	format := ""
	if f := asExpression(e.Arg("format")); f != nil && f.IsString() {
		if converted, ok := dialects.FormatTime(f.Name(), g.dialect.InverseTimeMapping); ok {
			format = " FORMAT '" + converted + "'"
		}
	}
	return "CAST(" + g.sqlKey(e, "this") + " AS DATE" + format + ")"
}

// teradataToNumberSQL ports to_number_with_nls_param (dialects/dialect.py:2357-2363).
func teradataToNumberSQL(g *Generator, e exp.Expression) string {
	return g.funcCall("TO_NUMBER", []any{
		e.This(),
		asExpression(e.Arg("format")),
		asExpression(e.Arg("nlsparam")),
	}, "(", ")", true)
}

// teradataDateDeltaSQL ports _date_add_sql (generators/teradata.py:19-39): a literal amount
// renders as `<this> +/- INTERVAL '<value>' <unit>`.
func teradataDateDeltaSQL(kind string) func(*Generator, exp.Expression) string {
	return func(g *Generator, e exp.Expression) string {
		value := asExpression(e.Arg("expression"))
		unit := e.Arg("unit")
		op := kind
		if value != nil && value.Kind() == exp.KindNeg {
			value = exp.LiteralString(value.This().Name())
			if kind == "+" {
				op = "-"
			} else {
				op = "+"
			}
		} else if value != nil {
			value = exp.LiteralString(value.Name())
		}
		interval := exp.Interval(exp.Args{"this": value, "unit": unit})
		return g.sqlKey(e, "this") + " " + op + " " + g.gen(interval)
	}
}

// teradataQuarterSQL ports the Quarter transform (generators/teradata.py:93): the node is
// re-emitted as an Extract so it flows through teradata's extract_sql below (a QUARTER
// extract renders as CAST(TO_CHAR(x, 'Q') AS INT)).
func teradataQuarterSQL(g *Generator, e exp.Expression) string {
	return g.gen(exp.Extract(exp.Args{"this": "QUARTER", "expression": e.Arg("this")}))
}

// teradataExtractSQL ports extract_sql (generators/teradata.py:162-168): Teradata's EXTRACT
// has no QUARTER unit, so it renders as CAST(TO_CHAR(expr, 'Q') AS INT); every other unit
// uses the base EXTRACT(... FROM ...) form.
func teradataExtractSQL(g *Generator, e exp.Expression) string {
	if strings.ToUpper(g.sqlKey(e, "this")) != "QUARTER" {
		return g.extractSQL(e)
	}
	toChar := exp.ToChar(exp.Args{"this": e.Arg("expression"), "format": exp.LiteralString("Q")})
	cast := exp.Cast(exp.Args{"this": toChar, "to": exp.DTypeInt.IntoExpr(nil)})
	return g.gen(cast)
}

// teradataIntervalSQL ports interval_sql (generators/teradata.py:170-182): Teradata has no
// WEEK/QUARTER interval units, so those render as a DAY-interval multiplier — the unit text
// is prefix-matched case-sensitively exactly as upstream's str.startswith does.
func teradataIntervalSQL(g *Generator, e exp.Expression) string {
	multiplier := 0
	switch unit := e.Text("unit"); {
	case strings.HasPrefix(unit, "WEEK"):
		multiplier = 7
	case strings.HasPrefix(unit, "QUARTER"):
		multiplier = 90
	}
	if multiplier == 0 {
		return g.intervalSQL(e)
	}
	day := exp.Interval(exp.Args{"this": e.Arg("this"), "unit": exp.Var(exp.Args{"this": "DAY"})})
	return "(" + strconv.Itoa(multiplier) + " * " + g.intervalSQL(day) + ")"
}

// teradataCastSQL ports cast_sql (generators/teradata.py:129-135): a FORMAT-only cast drops
// the (unknown) target type.
func teradataCastSQL(g *Generator, e exp.Expression) string {
	to := asExpression(e.Arg("to"))
	if to != nil && to.Kind() == exp.KindDataType && to.Arg("this") == exp.DTypeUnknown && e.Arg("format") != nil {
		return "CAST(" + g.sqlKey(e, "this") + " AS FORMAT " + g.sqlKey(e, "format") + ")"
	}
	return g.castSQLWithPrefix(e, "")
}

// teradataCurrentTimestampSQL ports currenttimestamp_sql (generators/teradata.py:107-113):
// the bare keyword without a precision, CURRENT_TIMESTAMP(n) with one.
func teradataCurrentTimestampSQL(g *Generator, e exp.Expression) string {
	if e.This() == nil {
		return "CURRENT_TIMESTAMP"
	}
	return g.funcCall("CURRENT_TIMESTAMP", []any{e.This()}, "(", ")", true)
}

// teradataTableSampleSQL ports tablesample_sql (generators/teradata.py:139-144):
// `<this> SAMPLE <expressions>`.
func teradataTableSampleSQL(g *Generator, e exp.Expression) string {
	exprs := g.expressions(exprsOptions{expression: e, flat: true})
	if exprs == "" {
		exprs = strings.TrimSpace(g.sqlKey(e, "percent") + g.sqlKey(e, "size"))
	}
	return g.sqlKey(e, "this") + " SAMPLE " + exprs
}

// teradataRangeNSQL ports rangen_sql (generators/teradata.py:159-166).
func teradataRangeNSQL(g *Generator, e exp.Expression) string {
	expressions := g.expressions(exprsOptions{expression: e, flat: true})
	each := ""
	if eachExpr := asExpression(e.Arg("each")); eachExpr != nil {
		each = " EACH " + g.gen(eachExpr)
	}
	return "RANGE_N(" + g.sqlKey(e, "this") + " BETWEEN " + expressions + each + ")"
}

// teradataUpdateSQL ports update_sql (generators/teradata.py:146-154): FROM before SET.
func teradataUpdateSQL(g *Generator, e exp.Expression) string {
	// from_ and where render with a leading separator (generator.py:2761 from_sql uses
	// seg("FROM")), so they concatenate directly, as in upstream's f-string.
	this := g.sqlKey(e, "this")
	from := g.sqlKey(e, "from_")
	set := g.expressions(exprsOptions{expression: e, flat: true})
	where := g.sqlKey(e, "where")
	return g.prependCtes(e, "UPDATE "+this+from+" SET "+set+where)
}

// teradataDataTypeSQL ports the TZ_TO_WITH_TIME_ZONE branch (generator.py: TZ_TO_WITH_TIME_ZONE):
// a timezone-aware TIMESTAMP renders WITH TIME ZONE.
func teradataDataTypeSQL(g *Generator, e exp.Expression) string {
	if dtype, ok := e.Arg("this").(exp.DType); ok && dtype == exp.DTypeTimestampTz {
		return "TIMESTAMP WITH TIME ZONE"
	}
	return g.dataTypeSQL(e)
}

// teradataCreateable ports TeradataGenerator.createable_sql (generators/teradata.py:148-160):
// a TABLE's POST_NAME properties render between the object name and its column list, each
// comma-prefixed and comma-joined, exactly as upstream's properties(prefix=",") call.
func (g *Generator) teradataCreateable(e exp.Expression, postName []exp.Expression) string {
	thisNode := asExpression(e.Arg("this"))
	name := g.sqlKey(thisNode, "this")
	props := g.renderProperties(g.propertiesExpression(postName, e), ",", ", ", "", false)
	schema := ""
	if thisNode != nil {
		schema = g.schemaColumnsSQL(thisNode)
	}
	// Upstream concatenates self.sep() unconditionally (generators/teradata.py:159), so a
	// name-only creatable keeps the trailing separator there too.
	return name + props + g.sep() + schema
}
