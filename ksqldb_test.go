package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
)

// ksqlDB round-trip cases, one per statement/clause family of SqlBase.g4. Every line must
// regenerate byte-identically under the ksqldb dialect.
var ksqlRoundTrips = []string{
	// CREATE (SqlBase.g4:74-83, 93)
	"CREATE STREAM s (id INT, name STRING) WITH (KAFKA_TOPIC='t', VALUE_FORMAT='JSON')",
	"CREATE OR REPLACE SOURCE STREAM s2 (a STRING) WITH (PARTITION_BY='a')",
	"CREATE TABLE t AS SELECT id, COUNT(*) AS c FROM s GROUP BY id EMIT CHANGES",
	"CREATE OR REPLACE TABLE t2 (a INT) WITH (VALUE_FORMAT='AVRO')",
	"CREATE SINK CONNECTOR c WITH (CONNECTOR_CLASS='x')",
	"CREATE SOURCE CONNECTOR c2 WITH (CONNECTOR_CLASS='y')",
	"CREATE TYPE ty AS STRUCT<a INT>",
	// INSERT (SqlBase.g4:86-87)
	"INSERT INTO t (a, b) VALUES (1, 2)",
	"INSERT INTO t VALUES (1, 2)",
	"INSERT INTO t WITH (KAFKA_TOPIC='x') SELECT a FROM s",
	// DROP (SqlBase.g4:88-90, 93)
	"DROP STREAM s DELETE TOPIC",
	"DROP TABLE IF EXISTS t",
	"DROP CONNECTOR c",
	"DROP TYPE ty",
	// SHOW / LIST (SqlBase.g4:49-63)
	"SHOW STREAMS",
	"SHOW ALL TOPICS",
	"SHOW TOPICS EXTENDED",
	"LIST PROPERTIES",
	"SHOW FUNCTIONS EXTENDED",
	"LIST SOURCE CONNECTORS",
	"SHOW CONNECTOR PLUGINS",
	"LIST QUERIES",
	"SHOW TYPES",
	// DESCRIBE / EXPLAIN (SqlBase.g4:58-61, 91)
	"DESCRIBE s",
	"DESCRIBE s EXTENDED",
	"DESCRIBE STREAMS EXTENDED",
	"DESCRIBE FUNCTION f",
	"DESCRIBE CONNECTOR c",
	"EXPLAIN query1",
	"EXPLAIN SELECT a FROM s",
	// ALTER (SqlBase.g4:94)
	"ALTER STREAM s ADD COLUMN z STRING",
	"ALTER TABLE t ADD COLUMN a INT, ADD COLUMN b STRING",
	// TERMINATE / PAUSE / RESUME (SqlBase.g4:64-69)
	"TERMINATE query1",
	"TERMINATE ALL",
	"PAUSE ALL",
	"RESUME q",
	// SET / UNSET / DEFINE / UNDEFINE (SqlBase.g4:70-73)
	"SET 'k' = 'v'",
	"UNSET 'k'",
	"DEFINE x = 'y'",
	"UNDEFINE x",
	// Query: WINDOW / WHERE / GROUP BY / PARTITION BY / HAVING / EMIT / LIMIT
	"SELECT a FROM s WINDOW TUMBLING (SIZE 1 HOUR) WHERE a > 1 GROUP BY a HAVING COUNT(*) > 2 EMIT CHANGES",
	"SELECT a FROM s WINDOW w HOPPING (SIZE 30 SECONDS, ADVANCE BY 10 SECONDS) PARTITION BY a EMIT FINAL",
	"SELECT a FROM s WINDOW SESSION (5 MINUTES)",
	"SELECT a FROM s WINDOW TUMBLING (SIZE 1 HOUR)",
	"SELECT a FROM s WINDOW TUMBLING (SIZE 1 HOUR, RETENTION 24 HOURS, GRACE PERIOD 5 MINUTES)",
	"SELECT a FROM s PARTITION BY a LIMIT 10",
	// Join window (SqlBase.g4:251-262)
	"SELECT * FROM a JOIN b WITHIN 1 HOUR GRACE PERIOD 10 MINUTES ON a.id = b.id",
	"SELECT * FROM a JOIN b WITHIN (1 HOUR, 2 HOURS) ON a.id = b.id",
	// JSON arrow (SqlBase.g4 primaryExpression)
	"SELECT a, b -> c FROM s",
	// PRINT / ASSERT / RUN (SqlBase.g4:62, 99-109) stay verbatim Commands
	"PRINT t",
	"PRINT t FROM BEGINNING",
	"RUN SCRIPT 'x.sql'",
	"ASSERT NULL VALUES s KEY (1)",
}

// TestKsqlDBRoundTrip is the ksqlDB identity gate: each statement of SqlBase.g4 that the
// port structures must parse and regenerate byte-identically.
func TestKsqlDBRoundTrip(t *testing.T) {
	for _, sql := range ksqlRoundTrips {
		assertRoundTrip(t, "ksqldb", sql, sql)
	}
	// The ksql alias registers the same dialect.
	assertRoundTrip(t, "ksql", "SHOW STREAMS", "SHOW STREAMS")
}

// TestKsqlDBAST asserts the node shape of the constructs that map onto shared sqlglot
// nodes in a ksqlDB-specific way (see DEVIATIONS.md "Beyond-upstream dialects").
func TestKsqlDBAST(t *testing.T) {
	for _, tc := range []struct {
		name  string
		sql   string
		check func(*testing.T, exp.Expression)
	}{
		{
			name: "create stream with properties",
			sql:  "CREATE STREAM s (id INT) WITH (KAFKA_TOPIC='t')",
			check: func(t *testing.T, e exp.Expression) {
				if e.Kind() != exp.KindCreate {
					t.Fatalf("kind = %s, want Create", exp.ClassName(e.Kind()))
				}
				if got := e.Text("kind"); got != "STREAM" {
					t.Errorf("create kind = %q, want STREAM", got)
				}
				schema, ok := e.Arg("this").(exp.Expression)
				if !ok || schema.Kind() != exp.KindSchema {
					t.Errorf("create this = %v, want Schema", e.Arg("this"))
				}
				props, ok := e.Arg("properties").(exp.Expression)
				if !ok || props.Kind() != exp.KindProperties {
					t.Errorf("create properties = %v, want Properties", e.Arg("properties"))
				} else if n := len(props.Expressions()); n != 1 {
					t.Errorf("properties = %d, want 1", n)
				}
			},
		},
		{
			name: "create table as select keeps emit",
			sql:  "CREATE TABLE t AS SELECT a FROM s EMIT CHANGES",
			check: func(t *testing.T, e exp.Expression) {
				query, ok := e.Arg("expression").(exp.Expression)
				if !ok || query.Kind() != exp.KindSelect {
					t.Fatalf("create expression = %v, want Select", e.Arg("expression"))
				}
				emit, ok := query.Arg("emit").(exp.Expression)
				if !ok || emit.Name() != "CHANGES" {
					t.Errorf("select emit = %v, want Var CHANGES", query.Arg("emit"))
				}
			},
		},
		{
			name: "window clause",
			sql:  "SELECT a FROM s WINDOW w TUMBLING (SIZE 1 HOUR) EMIT FINAL",
			check: func(t *testing.T, e exp.Expression) {
				windows := ksqlList(e.Arg("windows"))
				if len(windows) != 1 {
					t.Fatalf("windows = %d, want 1", len(windows))
				}
				w := windows[0]
				if w.Kind() != exp.KindKsqlWindow {
					t.Fatalf("window kind = %s, want KsqlWindow", exp.ClassName(w.Kind()))
				}
				if got := w.Text("kind"); got != "TUMBLING" {
					t.Errorf("window kind = %q, want TUMBLING", got)
				}
				if got := w.Text("alias"); got != "w" {
					t.Errorf("window alias = %q, want w", got)
				}
				size, ok := w.Arg("size").(exp.Expression)
				if !ok || size.Kind() != exp.KindInterval {
					t.Fatalf("window size = %v, want Interval", w.Arg("size"))
				}
				if got := size.Text("unit"); got != "HOUR" {
					t.Errorf("size unit = %q, want HOUR", got)
				}
				emit, ok := e.Arg("emit").(exp.Expression)
				if !ok || emit.Name() != "FINAL" {
					t.Errorf("select emit = %v, want Var FINAL", e.Arg("emit"))
				}
			},
		},
		{
			name: "partition by",
			sql:  "SELECT a FROM s PARTITION BY b",
			check: func(t *testing.T, e exp.Expression) {
				list := ksqlList(e.Arg("partition_by"))
				if len(list) != 1 {
					t.Fatalf("partition_by = %d expressions, want 1", len(list))
				}
			},
		},
		{
			name: "join window",
			sql:  "SELECT * FROM a JOIN b WITHIN 1 HOUR GRACE PERIOD 10 MINUTES ON a.id = b.id",
			check: func(t *testing.T, e exp.Expression) {
				joins := ksqlList(e.Arg("joins"))
				if len(joins) != 1 {
					t.Fatalf("joins = %d, want 1", len(joins))
				}
				join := joins[0]
				within, ok := join.Arg("within").(exp.Expression)
				if !ok || within.Kind() != exp.KindInterval {
					t.Fatalf("join within = %v, want Interval", join.Arg("within"))
				}
				grace, ok := join.Arg("grace").(exp.Expression)
				if !ok || grace.Kind() != exp.KindInterval {
					t.Fatalf("join grace = %v, want Interval", join.Arg("grace"))
				}
				if grace.Text("unit") != "MINUTES" {
					t.Errorf("grace unit = %q, want MINUTES", grace.Text("unit"))
				}
				if join.Arg("on") == nil {
					t.Error("join lost its ON criteria")
				}
			},
		},
		{
			name: "join window tuple",
			sql:  "SELECT * FROM a JOIN b WITHIN (1 HOUR, 2 HOURS) ON a.id = b.id",
			check: func(t *testing.T, e exp.Expression) {
				join := ksqlList(e.Arg("joins"))[0]
				within, ok := join.Arg("within").(exp.Expression)
				if !ok || within.Kind() != exp.KindTuple {
					t.Fatalf("join within = %v, want Tuple", join.Arg("within"))
				}
				if n := len(within.Expressions()); n != 2 {
					t.Errorf("within tuple = %d, want 2", n)
				}
			},
		},
		{
			name: "terminate",
			sql:  "TERMINATE query1",
			check: func(t *testing.T, e exp.Expression) {
				if e.Kind() != exp.KindKill {
					t.Fatalf("kind = %s, want Kill", exp.ClassName(e.Kind()))
				}
				if got := e.Text("kind"); got != "TERMINATE" {
					t.Errorf("kill kind = %q, want TERMINATE", got)
				}
				if got := e.Name(); got != "query1" {
					t.Errorf("kill this = %q, want query1", got)
				}
			},
		},
		{
			name: "show carries its verb",
			sql:  "LIST PROPERTIES",
			check: func(t *testing.T, e exp.Expression) {
				if e.Kind() != exp.KindShow {
					t.Fatalf("kind = %s, want Show", exp.ClassName(e.Kind()))
				}
				if got := e.Name(); got != "LIST PROPERTIES" {
					t.Errorf("show this = %q, want LIST PROPERTIES", got)
				}
			},
		},
		{
			name: "describe extended is a suffix",
			sql:  "DESCRIBE s EXTENDED",
			check: func(t *testing.T, e exp.Expression) {
				if e.Kind() != exp.KindDescribe {
					t.Fatalf("kind = %s, want Describe", exp.ClassName(e.Kind()))
				}
				if got := e.Text("style"); got != "EXTENDED" {
					t.Errorf("describe style = %q, want EXTENDED", got)
				}
			},
		},
		{
			name: "explain wraps a statement",
			sql:  "EXPLAIN SELECT a FROM s",
			check: func(t *testing.T, e exp.Expression) {
				if e.Kind() != exp.KindDescribe || e.Text("kind") != "EXPLAIN" {
					t.Fatalf("kind = %s/%q, want Describe EXPLAIN", exp.ClassName(e.Kind()), e.Text("kind"))
				}
				if inner, ok := e.Arg("this").(exp.Expression); !ok || inner.Kind() != exp.KindSelect {
					t.Errorf("explain this = %v, want Select", e.Arg("this"))
				}
			},
		},
		{
			name: "define",
			sql:  "DEFINE x = 'y'",
			check: func(t *testing.T, e exp.Expression) {
				if e.Kind() != exp.KindSet {
					t.Fatalf("kind = %s, want Set", exp.ClassName(e.Kind()))
				}
				if !truthyArg(e, "tag") {
					t.Error("define is missing the tag discriminator")
				}
				if n := len(e.Expressions()); n != 1 {
					t.Errorf("set items = %d, want 1", n)
				}
			},
		},
		{
			name: "drop delete topic",
			sql:  "DROP STREAM s DELETE TOPIC",
			check: func(t *testing.T, e exp.Expression) {
				if e.Kind() != exp.KindDrop {
					t.Fatalf("kind = %s, want Drop", exp.ClassName(e.Kind()))
				}
				if !truthyArg(e, "delete_topic") {
					t.Error("drop is missing delete_topic")
				}
			},
		},
		{
			name: "insert values with columns",
			sql:  "INSERT INTO t (a, b) VALUES (1, 2)",
			check: func(t *testing.T, e exp.Expression) {
				if e.Kind() != exp.KindInsert {
					t.Fatalf("kind = %s, want Insert", exp.ClassName(e.Kind()))
				}
				if inner, ok := e.Arg("this").(exp.Expression); !ok || inner.Kind() != exp.KindSchema {
					t.Errorf("insert this = %v, want Schema", e.Arg("this"))
				}
				if src, ok := e.Arg("expression").(exp.Expression); !ok || src.Kind() != exp.KindValues {
					t.Errorf("insert expression = %v, want Values", e.Arg("expression"))
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, err := sqlglot.ParseOne(tc.sql, "ksqldb")
			if err != nil {
				t.Fatalf("ParseOne(%q): %v", tc.sql, err)
			}
			tc.check(t, e)
		})
	}
}

// TestKsqlDBFailClosed pins the statements ksqlDB's grammar rejects: they must degrade to a
// verbatim Command (or a parse error), never to a structured node the engine would refuse.
func TestKsqlDBFailClosed(t *testing.T) {
	// SqlBase.g4:74-83 — the tableElements form and the AS-query form are disjoint.
	for _, sql := range []string{
		"CREATE STREAM x (a INT) AS SELECT a FROM y",
		// SqlBase.g4:86-87 — the column list pairs only with VALUES, WITH only with a query.
		"INSERT INTO t (a) SELECT a FROM s",
		"INSERT INTO t WITH (KAFKA_TOPIC='x') VALUES (1, 2)",
		// SqlBase.g4:126 — EMIT needs CHANGES or FINAL; a bare EMIT is a nonReserved alias.
		"SELECT a FROM s EMIT CHANGES TRAILING",
		"SELECT a FROM s EMIT FINAL TRAILING",
	} {
		e, err := sqlglot.ParseOne(sql, "ksqldb")
		if err != nil {
			continue // rejected outright: also acceptable
		}
		if e.Kind() != exp.KindCommand {
			t.Errorf("%q parsed to %s, want a fail-closed Command", sql, exp.ClassName(e.Kind()))
		}
	}
}

// TestKsqlDBPrintStaysCommand pins the clauses the port deliberately does not model
// (printClause, assert timeout): they round-trip verbatim as a raw Command.
func TestKsqlDBPrintStaysCommand(t *testing.T) {
	for _, sql := range []string{
		"PRINT t",
		"PRINT t FROM BEGINNING",
		"PRINT t INTERVAL 10",
		"RUN SCRIPT 'x.sql'",
	} {
		e, err := sqlglot.ParseOne(sql, "ksqldb")
		if err != nil {
			t.Fatalf("ParseOne(%q): %v", sql, err)
		}
		if e.Kind() != exp.KindCommand {
			t.Errorf("%q parsed to %s, want Command", sql, exp.ClassName(e.Kind()))
			continue
		}
		got, err := sqlglot.Generate(e, "ksqldb", generator.Options{})
		if err != nil {
			t.Fatalf("Generate(%q): %v", sql, err)
		}
		if got != sql {
			t.Errorf("%q -> %q, want verbatim", sql, got)
		}
	}
}

// TestKsqlDBTypes pins ksqlDB's type spellings: STRING is the canonical VARCHAR alias
// (base sqlglot would emit TEXT, which ksqlDB has no keyword for).
func TestKsqlDBTypes(t *testing.T) {
	for _, tc := range []struct{ sql, want string }{
		{"CREATE STREAM s (a STRING)", "CREATE STREAM s (a STRING)"},
		{"CREATE STREAM s (a INT, b BIGINT, c DOUBLE, d BOOLEAN)", "CREATE STREAM s (a INT, b BIGINT, c DOUBLE, d BOOLEAN)"},
		{"CREATE STREAM s (a VARCHAR(10), b DECIMAL(10, 2))", "CREATE STREAM s (a VARCHAR(10), b DECIMAL(10, 2))"},
		{"CREATE STREAM s (a ARRAY<INT>, b MAP<STRING, INT>)", "CREATE STREAM s (a ARRAY<INT>, b MAP<STRING, INT>)"},
	} {
		assertRoundTrip(t, "ksqldb", tc.sql, tc.want)
	}
}

// TestKsqlDBBaseDegrades pins that a ksqlDB statement under the base dialect — which has no
// ksql grammar — still round-trips instead of crashing or mis-parsing.
func TestKsqlDBBaseDegrades(t *testing.T) {
	for _, sql := range []string{"SHOW STREAMS"} {
		e, err := sqlglot.ParseOne(sql, "")
		if err != nil {
			continue // a parse error is an acceptable fallback
		}
		if e.Kind() != exp.KindCommand {
			t.Errorf("%q under base parsed to %s, want Command", sql, exp.ClassName(e.Kind()))
		}
	}
}

// ksqlList flattens an args value into its expression list (the tests have no access to the
// generator package's listFromValue helper).
func ksqlList(v any) []exp.Expression {
	switch t := v.(type) {
	case nil:
		return nil
	case exp.Expression:
		return []exp.Expression{t}
	case []exp.Expression:
		return t
	case []any:
		out := make([]exp.Expression, 0, len(t))
		for _, item := range t {
			if e, ok := item.(exp.Expression); ok {
				out = append(out, e)
			}
		}
		return out
	default:
		return nil
	}
}

func truthyArg(e exp.Expression, key string) bool {
	switch v := e.Arg(key).(type) {
	case bool:
		return v
	case exp.Expression:
		return v != nil
	default:
		return v != nil
	}
}
