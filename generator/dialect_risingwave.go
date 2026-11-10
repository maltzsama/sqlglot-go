package generator

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// RisingWave generator overrides, ported from
// .reference/sqlglot-v30.17.0/sqlglot/generators/risingwave.py. RisingWaveGenerator
// subclasses PostgresGenerator and only adds a few transforms, so the rest falls back to
// the postgres overlay via dialectParents.
func init() {
	registerDialectDispatch("risingwave", risingwaveDispatch())
	// generators/risingwave.py:27-31 PROPERTIES_LOCATION: FileFormatProperty moves from the
	// WITH clause position (base, generators materialize-style `FORMAT=`) to POST_EXPRESSION,
	// right after the CREATE body.
	registerDialectPropertyLocations("risingwave", map[exp.Kind]propertyLocation{
		exp.KindFileFormatProperty: propertyLocationPostExpression,
	})
}

func risingwaveDispatch() map[exp.Kind]func(*Generator, exp.Expression) string {
	return map[exp.Kind]func(*Generator, exp.Expression) string{
		// generators/risingwave.py:24 exp.FileFormatProperty: `FORMAT <fmt>` (the base entry
		// renders the generic `FORMAT=<fmt>` property form).
		exp.KindFileFormatProperty: func(g *Generator, e exp.Expression) string {
			return "FORMAT " + g.sqlKey(e, "this")
		},
		// generators/risingwave.py:23 + 27-28: computedcolumnconstraint_sql explicitly calls
		// the BASE Generator method, bypassing PostgresGenerator's
		// `GENERATED ALWAYS AS (...) STORED` override.
		exp.KindComputedColumnConstraint: (*Generator).baseComputedColumnConstraintSQL,
		// generators/risingwave.py:30-35 datatype_sql: a 2-arg MAP renders `MAP(<k>, <v>)`
		// (postgres/base would render `MAP<k, v>`).
		exp.KindDataType: risingwaveDataType,
	}
}

// risingwaveDataType ports datatype_sql (generators/risingwave.py:30-35).
func risingwaveDataType(g *Generator, e exp.Expression) string {
	if exp.IsType(e, exp.DTypeMap) && len(e.Expressions()) == 2 {
		exprs := e.Expressions()
		return "MAP(" + g.gen(exprs[0]) + ", " + g.gen(exprs[1]) + ")"
	}
	return g.dataTypeSQL(e)
}
