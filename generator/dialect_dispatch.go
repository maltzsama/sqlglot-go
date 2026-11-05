package generator

import (
	"strings"

	"github.com/ridi-oss/sqlglot-go/dialects"
	"github.com/ridi-oss/sqlglot-go/expressions"
)

// Per-dialect generator overrides — the port of upstream's per-class TRANSFORMS, TYPE_MAPPING
// and PROPERTIES_LOCATION overlays. A key is a dialect name or a routed sub-generator name
// (athena-trino, athena-hive); lookups walk dialectParents so trino inherits presto the way
// TrinoGenerator subclasses PrestoGenerator. Dialects that register nothing (base, mysql,
// postgres) resolve straight to the shared tables, byte-identical to before.
var (
	dialectDispatch     = map[string]map[expressions.Kind]func(*Generator, expressions.Expression) string{}
	dialectTypeMappings = map[string]map[expressions.DType]string{}
	// dialectRemovedTypeMappings records a dialect that removes a base TYPE_MAPPING entry
	// (generators/tsql.py:171-176 drops NCHAR/NVARCHAR), so the type renders with its own
	// DType name instead of the base mapping.
	dialectRemovedTypeMappings = map[string]map[expressions.DType]bool{}
	dialectPropertyLocations   = map[string]map[expressions.Kind]propertyLocation{}
	dialectParents             = map[string]string{
		"starrocks":    "mysql",
		"doris":        "mysql",
		"trino":        "presto",
		"athena-trino": "trino",
		"athena-hive":  "hive",
	}
)

func registerDialectDispatch(name string, table map[expressions.Kind]func(*Generator, expressions.Expression) string) {
	name = strings.ToLower(name)
	if dialectDispatch[name] == nil {
		dialectDispatch[name] = map[expressions.Kind]func(*Generator, expressions.Expression) string{}
	}
	for kind, handler := range table {
		if handler == nil {
			panic("generator: nil dispatch override for dialect " + name)
		}
		if _, exists := dialectDispatch[name][kind]; exists {
			panic("generator: duplicate dispatch override for dialect " + name + " kind " + expressions.ClassName(kind))
		}
		dialectDispatch[name][kind] = handler
	}
}

func registerDialectTypeMapping(name string, delta map[expressions.DType]string) {
	name = strings.ToLower(name)
	if dialectTypeMappings[name] == nil {
		dialectTypeMappings[name] = map[expressions.DType]string{}
	}
	for dtype, mapped := range delta {
		dialectTypeMappings[name][dtype] = mapped
	}
}

// registerDialectRemoveTypeMappings removes base TYPE_MAPPING entries for a dialect: the DType
// then renders as its own name (e.g. NVARCHAR rather than the base's VARCHAR).
func registerDialectRemoveTypeMappings(name string, dtypes ...expressions.DType) {
	name = strings.ToLower(name)
	if dialectRemovedTypeMappings[name] == nil {
		dialectRemovedTypeMappings[name] = map[expressions.DType]bool{}
	}
	for _, dtype := range dtypes {
		dialectRemovedTypeMappings[name][dtype] = true
	}
}

func registerDialectPropertyLocations(name string, table map[expressions.Kind]propertyLocation) {
	name = strings.ToLower(name)
	if dialectPropertyLocations[name] == nil {
		dialectPropertyLocations[name] = map[expressions.Kind]propertyLocation{}
	}
	for kind, location := range table {
		dialectPropertyLocations[name][kind] = location
	}
}

// overrideKey names the override chain this generator renders with: the routed sub-generator
// name when set, else the dialect name.
func (g *Generator) overrideKey() string {
	if g.overrideName != "" {
		return g.overrideName
	}
	return strings.ToLower(g.dialect.Name)
}

// isDialect reports whether name is the generator's dialect or an ancestor in its override
// chain (so "presto" matches trino and athena-trino generators too).
func (g *Generator) isDialect(name string) bool {
	for key := g.overrideKey(); key != ""; key = dialectParents[key] {
		if key == name {
			return true
		}
	}
	return false
}

func (g *Generator) lookupDispatch(kind expressions.Kind) func(*Generator, expressions.Expression) string {
	for key := g.overrideKey(); key != ""; key = dialectParents[key] {
		if handler := dialectDispatch[key][kind]; handler != nil {
			return handler
		}
	}
	return dispatch[kind]
}

func (g *Generator) lookupTypeMapping(dtype expressions.DType) (string, bool) {
	for key := g.overrideKey(); key != ""; key = dialectParents[key] {
		if dialectRemovedTypeMappings[key][dtype] {
			return string(dtype), true
		}
		if mapped, ok := dialectTypeMappings[key][dtype]; ok {
			return mapped, true
		}
	}
	return "", false
}

func (g *Generator) lookupPropertyLocation(kind expressions.Kind) (propertyLocation, bool) {
	for key := g.overrideKey(); key != ""; key = dialectParents[key] {
		if location, ok := dialectPropertyLocations[key][kind]; ok {
			return location, true
		}
	}
	return 0, false
}

// child builds a generator for a routed sub-dialect with the same options, carrying the
// resolve-time settings of the outer dialect (parser/dialect_athena_router.go does the same).
func (g *Generator) child(d *dialects.Dialect, overrideName string) *Generator {
	d.OpaqueFunctions = g.dialect.OpaqueFunctions
	d.NormalizationStrategy = g.dialect.NormalizationStrategy
	c := New(d, g.opts)
	c.overrideName = overrideName
	return c
}
