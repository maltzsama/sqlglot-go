package generator

import (
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
