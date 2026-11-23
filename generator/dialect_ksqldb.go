package generator

import (
	"strings"

	"github.com/maltzsama/sqlglot-go/expressions"
)

// ksqlDB generator overlays — the beyond-upstream ksqlDB dialect (Confluent ksqldb-parser
// SqlBase.g4). Only the shapes the base generator renders differently are overridden: the
// bare window duration (`1 HOUR` instead of `INTERVAL 1 HOUR`), the SHOW/DESCRIBE/EXPLAIN
// command spellings, the DEFINE/UNDEFINE/UNSET/TERMINATE family, the `->` arrow, and the
// window body itself (the WINDOW keyword is added by ksqlQueryModifiers). The query clause
// order, the join's WITHIN window, INSERT's WITH clause and DROP's DELETE TOPIC are spliced
// into the shared generators behind an isDialect("ksqldb") guard (see generator/sql.go and
// generator/ddl.go).
func init() {
	registerDialectDispatch("ksqldb", map[expressions.Kind]func(*Generator, expressions.Expression) string{
		// windowUnit durations are bare `1 HOUR` in ksqlDB (SqlBase.g4:201-208), never
		// wrapped in INTERVAL.
		expressions.KindInterval: (*Generator).ksqlIntervalSQL,
		// SHOW/LIST share one node; the verb is carried in Show.this alongside its phrase.
		expressions.KindShow: (*Generator).ksqlShowSQL,
		// DESCRIBE/EXPLAIN are suffix-keyword commands in ksqlDB, not prefix ones.
		expressions.KindDescribe: (*Generator).ksqlDescribeSQL,
		// SET/UNSET/DEFINE/UNDEFINE share one node; tag is the DEFINE discriminator.
		expressions.KindSet: (*Generator).ksqlSetSQL,
		// TERMINATE/PAUSE/RESUME share one node; kind carries the verb (not "KILL").
		expressions.KindKill: (*Generator).ksqlKillSQL,
		// ksqlDB spells the JSON arrow `->`, not JSON_EXTRACT(...).
		expressions.KindJSONExtract: (*Generator).ksqlJSONExtractSQL,
		// The WINDOW keyword is added by ksqlQueryModifiers; this renders the body only.
		expressions.KindKsqlWindow: (*Generator).ksqlWindowSQL,
	})
	// ksqlDB has no TEXT type; STRING is the VARCHAR alias (SqlBase.g4:361, baseType).
	registerDialectTypeMapping("ksqldb", map[expressions.DType]string{
		expressions.DTypeText: "STRING",
	})
}

// ksqlIntervalSQL renders a window/duration unit as the bare `1 HOUR` ksqlDB expects
// (windowUnit, SqlBase.g4:201-208), skipping the INTERVAL keyword.
func (g *Generator) ksqlIntervalSQL(e expressions.Expression) string {
	this := g.sqlKey(e, "this")
	if unit := g.sqlKey(e, "unit"); unit != "" {
		if this != "" {
			this += " "
		}
		this += unit
	}
	return strings.TrimSpace(this)
}

// ksqlShowSQL renders `(LIST | SHOW) ... [EXTENDED]` (SqlBase.g4:49-63). The verb and its
// phrase are kept together in Show.this so LIST and SHOW round-trip distinctly.
func (g *Generator) ksqlShowSQL(e expressions.Expression) string {
	sql := g.sqlKey(e, "this")
	if sql == "" {
		return "SHOW"
	}
	// Show.this already carries the LIST/SHOW verb; only a verb-less node gets SHOW.
	upper := strings.ToUpper(sql)
	if upper != "SHOW" && upper != "LIST" &&
		!strings.HasPrefix(upper, "SHOW ") && !strings.HasPrefix(upper, "LIST ") {
		sql = "SHOW " + sql
	}
	if truthy(e.Arg("full")) {
		sql += " EXTENDED"
	}
	return sql
}

// ksqlDescribeSQL renders DESCRIBE / EXPLAIN (SqlBase.g4:58-61, 91). ksqlDB puts EXTENDED
// after the target (`DESCRIBE x EXTENDED`), unlike the prefix `style` the base generator
// renders, and EXPLAIN takes a statement or an identifier.
func (g *Generator) ksqlDescribeSQL(e expressions.Expression) string {
	target := g.sqlKey(e, "this")
	switch e.Text("kind") {
	case "EXPLAIN":
		return "EXPLAIN " + target
	case "FUNCTION":
		return "DESCRIBE FUNCTION " + target
	case "CONNECTOR":
		return "DESCRIBE CONNECTOR " + target
	}
	sql := "DESCRIBE"
	if target != "" {
		sql += " " + target
	}
	if e.Text("style") != "" {
		sql += " " + e.Text("style")
	}
	return sql
}

// ksqlSetSQL renders SET / UNSET / DEFINE / UNDEFINE (SqlBase.g4:70-73) off one node:
// `tag` is the DEFINE discriminator, `unset` the UNSET/UNDEFINE one. A plain SET falls
// through to the base renderer.
func (g *Generator) ksqlSetSQL(e expressions.Expression) string {
	if truthy(e.Arg("tag")) {
		items := g.expressions(exprsOptions{expression: e, flat: true})
		if truthy(e.Arg("unset")) {
			return "UNDEFINE " + items
		}
		return "DEFINE " + items
	}
	if truthy(e.Arg("unset")) {
		return "UNSET " + g.expressions(exprsOptions{expression: e, flat: true})
	}
	return g.setSQL(e)
}

// ksqlKillSQL renders TERMINATE / PAUSE / RESUME (SqlBase.g4:64-69), including the ALL
// form; kind carries the verb.
func (g *Generator) ksqlKillSQL(e expressions.Expression) string {
	verb := e.Text("kind")
	if verb == "" {
		verb = "TERMINATE"
	}
	if this := g.sqlKey(e, "this"); this != "" {
		return verb + " " + this
	}
	return verb
}

// ksqlJSONExtractSQL renders ksqlDB's JSON arrow `a -> b` (SqlBase.g4 primaryExpression),
// where base sqlglot renders the JSON_EXTRACT function form.
func (g *Generator) ksqlJSONExtractSQL(e expressions.Expression) string {
	return g.binary(e, "->")
}

// ksqlWindowSQL renders windowExpression (SqlBase.g4:184-199) — the optional window name
// plus TUMBLING / HOPPING / SESSION. The `WINDOW ` keyword itself is prefixed by
// ksqlQueryModifiers, since the clause is stored as a single node in Select.windows.
func (g *Generator) ksqlWindowSQL(e expressions.Expression) string {
	var b strings.Builder
	if alias := g.sqlKey(e, "alias"); alias != "" {
		b.WriteString(alias)
		b.WriteString(" ")
	}
	b.WriteString(e.Text("kind"))
	b.WriteString(" (")
	first := true
	add := func(label string, key string) {
		if value := g.sqlKey(e, key); value != "" {
			if !first {
				b.WriteString(", ")
			}
			first = false
			if label != "" {
				b.WriteString(label)
				b.WriteString(" ")
			}
			b.WriteString(value)
		}
	}
	// TUMBLING/HOPPING spell SIZE explicitly; SESSION takes the bare duration
	// (SqlBase.g4:186-198).
	if e.Text("kind") != "SESSION" {
		add("SIZE", "size")
	} else {
		add("", "size")
	}
	add("ADVANCE BY", "advance")
	add("RETENTION", "retention")
	add("GRACE PERIOD", "grace")
	b.WriteString(")")
	return b.String()
}

// ksqlQueryModifiers renders the ksqlDB query clause order (SqlBase.g4:117-127):
// `(WINDOW)? (WHERE)? (GROUP BY)? (PARTITION BY)? (HAVING)? (EMIT)? limitClause?`.
// The base order puts WHERE before WINDOW and has no PARTITION BY or EMIT at all.
func (g *Generator) ksqlQueryModifiers(e expressions.Expression, sqls ...string) string {
	limit := asExpression(e.Arg("limit"))
	if g.dialect.LimitFetch == "FETCH" && limit != nil && limit.Kind() == expressions.KindLimit {
		limit = expressions.Fetch(expressions.Args{"direction": "FIRST", "count": limit.Arg("expression")})
	} else if g.dialect.LimitFetch == "LIMIT" && limit != nil && limit.Kind() == expressions.KindFetch {
		limit = expressions.Limit(expressions.Args{"expression": limit.Arg("count")})
	}
	fetch := limit != nil && limit.Kind() == expressions.KindFetch

	parts := make([]string, 0, len(sqls)+12)
	parts = append(parts, sqls...)
	for _, join := range listFromValue(e.Arg("joins")) {
		parts = append(parts, g.gen(join))
	}
	parts = append(parts, g.sqlKey(e, "match"))
	for _, lateral := range listFromValue(e.Arg("laterals")) {
		parts = append(parts, g.gen(lateral))
	}
	if truthy(e.Arg("windows")) {
		parts = append(parts, g.seg("WINDOW ")+g.expressions(exprsOptions{expression: e, key: "windows", flat: true}))
	}
	parts = append(parts, g.sqlKey(e, "prewhere"), g.sqlKey(e, "where"), g.sqlKey(e, "connect"))
	parts = append(parts, g.sqlKey(e, "group"))
	if partitionBy := g.expressions(exprsOptions{expression: e, key: "partition_by", sep: ", ", flat: true}); partitionBy != "" {
		parts = append(parts, g.seg("PARTITION BY ")+partitionBy)
	}
	parts = append(parts, g.sqlKey(e, "having"))
	parts = append(parts, g.sqlKey(e, "cluster"), g.sqlKey(e, "distribute"), g.sqlKey(e, "sort"),
		g.sqlKey(e, "qualify"), g.sqlKey(e, "order"))
	if emit := g.sqlKey(e, "emit"); emit != "" {
		parts = append(parts, g.seg("EMIT ")+emit)
	}
	parts = append(parts, g.offsetLimitModifiers(e, fetch, limit)...)
	parts = append(parts, g.afterLimitModifiers(e)...)
	parts = append(parts, g.optionsModifier(e), g.sqlKey(e, "for_"))
	return joinNonEmpty("", parts...)
}
