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
