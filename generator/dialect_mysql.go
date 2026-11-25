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
