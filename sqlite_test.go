package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestSQLiteCanonicalizations ports the function-form rewrites from
// tests/dialects/test_sqlite.py:107-222: the LIKE/GLOB function spellings become infix
// operators, MIN/MAX with several arguments are LEAST/GREATEST-style calls, JSON aggregates
// rename, and UNICODE stays put.
func TestSQLiteCanonicalizations(t *testing.T) {
	assertRoundTrip(t, "sqlite", "SELECT LIKE(y, x)", "SELECT x LIKE y")
	assertRoundTrip(t, "sqlite", "SELECT GLOB('*y*', 'xyz')", "SELECT 'xyz' GLOB '*y*'")
	assertRoundTrip(t, "sqlite", "SELECT LIKE('%y%', 'xyz', '')", "SELECT 'xyz' LIKE '%y%' ESCAPE ''")
	assertRoundTrip(t, "sqlite", "SELECT MIN(a, b) FROM t", "SELECT MIN(a, b) FROM t")
	assertRoundTrip(t, "sqlite", "SELECT MAX(a, b) FROM t", "SELECT MAX(a, b) FROM t")
	assertRoundTrip(t, "sqlite", "SELECT JSON_GROUP_ARRAY(name) FROM t", "SELECT JSON_GROUP_ARRAY(name) FROM t")
	assertRoundTrip(t, "sqlite", "SELECT JSON_GROUP_OBJECT(name, value) FROM t", "SELECT JSON_GROUP_OBJECT(name, value) FROM t")
	assertRoundTrip(t, "sqlite", "SELECT UNICODE(x)", "SELECT UNICODE(x)")
	assertRoundTrip(t, "sqlite", "SELECT JULIANDAY('now') - JULIANDAY('1776-07-04')", "SELECT JULIANDAY('now') - JULIANDAY('1776-07-04')")
	assertRoundTrip(t, "sqlite", "SELECT 0XCC", "SELECT x'CC'")
	assertRoundTrip(t, "sqlite", "SELECT a FROM t1 WHERE a NOT NULL AND a NOT NULL ORDER BY a",
		"SELECT a FROM t1 WHERE NOT a IS NULL AND NOT a IS NULL ORDER BY a")
	assertRoundTrip(t, "sqlite", "SELECT * FROM t1, t2", "SELECT * FROM t1 CROSS JOIN t2")
	assertRoundTrip(t, "sqlite", "ALTER TABLE t RENAME a TO b", "ALTER TABLE t RENAME COLUMN a TO b")
}

// TestSQLiteJSONArrowPrecedence ports test_json_arrow_precedence
// (tests/dialects/test_sqlite.py:255-290): ||, -> and ->> form a single left-associative
// tier that binds tighter than multiplication and looser than COLLATE.
func TestSQLiteJSONArrowPrecedence(t *testing.T) {
	assertRoundTrip(t, "sqlite", "SELECT a.data ->> b.key ->> a.final_key FROM t",
		"SELECT a.data ->> b.key ->> a.final_key FROM t")
	assertRoundTrip(t, "sqlite", "SELECT a -> b ->> c -> d FROM t", "SELECT a -> b ->> c -> d FROM t")
	assertRoundTrip(t, "sqlite", "SELECT a || b ->> c", "SELECT (a || b) ->> c")
	assertRoundTrip(t, "sqlite", "SELECT 2 * 3 || 4", "SELECT 2 * (3 || 4)")
	assertRoundTrip(t, "sqlite", "SELECT a || b * c FROM t", "SELECT (a || b) * c FROM t")
	assertRoundTrip(t, "sqlite", "SELECT a - b ->> c FROM t", "SELECT a - (b ->> c) FROM t")
	assertRoundTrip(t, "sqlite", "SELECT a || b COLLATE NOCASE FROM t ORDER BY 1",
		"SELECT a || b COLLATE NOCASE FROM t ORDER BY 1")
	assertRoundTrip(t, "sqlite", "SELECT a COLLATE NOCASE || b -> c FROM t",
		"SELECT (a COLLATE NOCASE || b) -> c FROM t")

	// The chained arrows build nested JSONExtractScalar nodes.
	expr, err := sqlglot.ParseOne("SELECT a.data ->> b.key ->> a.final_key FROM t", "sqlite")
	if err != nil {
		t.Fatal(err)
	}
	outer := expr.Expressions()[0]
	if outer.Kind() != expressions.KindJSONExtractScalar {
		t.Fatalf("expected JSONExtractScalar, got %s", outer.ToS())
	}
	if inner := outer.This(); inner == nil || inner.Kind() != expressions.KindJSONExtractScalar {
		t.Fatalf("expected nested JSONExtractScalar, got %s", outer.ToS())
	}
}

// TestSQLiteJSONExtractSubtype ports the json_subtype assertions
// (tests/dialects/test_sqlite.py:63-70): the single-path JSON_EXTRACT function keeps the
// JSON subtype while the ->> operator strips it.
func TestSQLiteJSONExtractSubtype(t *testing.T) {
	fn, err := sqlglot.ParseOne("SELECT JSON_EXTRACT(a, '$.k') FROM t", "sqlite")
	if err != nil {
		t.Fatal(err)
	}
	extract := fn.Expressions()[0]
	if extract.Kind() != expressions.KindJSONExtractScalar {
		t.Fatalf("expected JSONExtractScalar, got %s", extract.ToS())
	}
	if subtype, _ := extract.Arg("json_subtype").(bool); !subtype {
		t.Fatalf("expected json_subtype on the function form, got %s", extract.ToS())
	}

	op, err := sqlglot.ParseOne("SELECT a ->> '$.k' FROM t", "sqlite")
	if err != nil {
		t.Fatal(err)
	}
	scalar := op.Expressions()[0]
	if subtype, _ := scalar.Arg("json_subtype").(bool); subtype {
		t.Fatalf("expected no json_subtype on the ->> form, got %s", scalar.ToS())
	}
}

// TestSQLiteGeneratedColumns ports test_generated_columns
// (tests/dialects/test_sqlite.py:370-398): GENERATED ALWAYS AS (expr) becomes a computed
// column with STORED/VIRTUAL storage, and a true identity column becomes AUTOINCREMENT.
func TestSQLiteGeneratedColumns(t *testing.T) {
	assertRoundTrip(t, "sqlite", "CREATE TABLE t (a INTEGER, b INTEGER AS (a * 2) STORED)",
		"CREATE TABLE t (a INTEGER, b INTEGER AS (a * 2) STORED)")
	assertRoundTrip(t, "sqlite", "CREATE TABLE t (a INTEGER, b AS (a * 2))",
		"CREATE TABLE t (a INTEGER, b AS (a * 2))")
	assertRoundTrip(t, "sqlite", "CREATE TABLE t (a INTEGER, b INTEGER GENERATED ALWAYS AS (a * 2) STORED)",
		"CREATE TABLE t (a INTEGER, b INTEGER AS (a * 2) STORED)")
	assertRoundTrip(t, "sqlite", "CREATE TABLE t (a INTEGER, b INTEGER GENERATED ALWAYS AS (a * 2) VIRTUAL)",
		"CREATE TABLE t (a INTEGER, b INTEGER AS (a * 2))")
	assertRoundTrip(t, "sqlite", "CREATE TABLE t (a INTEGER PRIMARY KEY GENERATED ALWAYS AS IDENTITY)",
		"CREATE TABLE t (a INTEGER PRIMARY KEY AUTOINCREMENT)")
}

// TestSQLiteDDL ports the PRAGMA / ATTACH / DETACH / INSERT conflict grammar from
// tests/dialects/test_sqlite.py:400-503.
func TestSQLiteDDL(t *testing.T) {
	assertRoundTrip(t, "sqlite", "PRAGMA table_info", "PRAGMA table_info")
	assertRoundTrip(t, "sqlite", "PRAGMA main.cache_size = 2000", "PRAGMA main.cache_size = 2000")
	assertRoundTrip(t, "sqlite", "PRAGMA foo(-2000)", "PRAGMA foo = -2000")
	assertRoundTrip(t, "sqlite", "PRAGMA encoding = 'UTF-16'", "PRAGMA encoding = 'UTF-16'")
	assertRoundTrip(t, "sqlite", "ATTACH DATABASE 'foo' AS schema_name", "ATTACH 'foo' AS schema_name")
	assertRoundTrip(t, "sqlite", "DETACH DATABASE schema_name", "DETACH schema_name")
	assertRoundTrip(t, "sqlite", "CREATE VIRTUAL TABLE docs USING fts5(title, content)",
		"CREATE VIRTUAL TABLE docs USING fts5(title, content)")
	assertRoundTrip(t, "sqlite", "CREATE VIRTUAL TABLE t USING module_name",
		"CREATE VIRTUAL TABLE t USING module_name")
	assertRoundTrip(t, "sqlite", "INSERT OR IGNORE INTO foo (x, y) VALUES (1, 2)",
		"INSERT OR IGNORE INTO foo (x, y) VALUES (1, 2)")
	assertRoundTrip(t, "sqlite",
		"INSERT INTO tbl (x, y) VALUES (1, 'a') ON CONFLICT(x, LOWER(y) COLLATE NOCASE ASC, y DESC) DO NOTHING",
		"INSERT INTO tbl (x, y) VALUES (1, 'a') ON CONFLICT(x, LOWER(y) COLLATE NOCASE ASC, y DESC) DO NOTHING")
	assertRoundTrip(t, "sqlite", "ANALYZE schma.tbl", "ANALYZE schma.tbl")
	// Table and column aliases: INDEXED BY round-trips, named alias columns are dropped.
	assertRoundTrip(t, "sqlite", "SELECT * FROM t AS t INDEXED BY s.i", "SELECT * FROM t AS t INDEXED BY s.i")
	assertRoundTrip(t, "sqlite", "SELECT * FROM t NOT INDEXED", "SELECT * FROM t NOT INDEXED")
	assertRoundTrip(t, "sqlite", "SELECT * FROM t AS t(c1, c2)", "SELECT * FROM t AS t")
	// A CTE keeps its named columns even though table aliases don't.
	assertRoundTrip(t, "sqlite", "WITH xyz(x) AS (SELECT 1) SELECT x FROM xyz", "WITH xyz(x) AS (SELECT 1) SELECT x FROM xyz")
}

// TestSQLiteTrunc ports test_trunc (tests/dialects/test_sqlite.py:361-368): SQLite TRUNC
// takes one argument, so a decimals argument is dropped.
func TestSQLiteTrunc(t *testing.T) {
	assertRoundTrip(t, "sqlite", "TRUNC(3.14)", "TRUNC(3.14)")
	assertRoundTrip(t, "sqlite", "TRUNC(3.14, 2)", "TRUNC(3.14)")
}

// TestSQLiteModernStatements ports the statement forms from
// tests/dialects/test_sqlite.py:24-45 and the trigger Command fallback at 509-524.
func TestSQLiteModernStatements(t *testing.T) {
	assertRoundTrip(t, "sqlite", "SELECT DATE()", "SELECT DATE()")
	assertRoundTrip(t, "sqlite", "SELECT DATE('now', 'start of month', '+1 month', '-1 day')",
		"SELECT DATE('now', 'start of month', '+1 month', '-1 day')")
	assertRoundTrip(t, "sqlite", "SELECT DATETIME(1092941466, 'unixepoch')", "SELECT DATETIME(1092941466, 'unixepoch')")
	assertRoundTrip(t, "sqlite", "SELECT UNIXEPOCH()", "SELECT UNIXEPOCH()")
	assertRoundTrip(t, "sqlite", "SELECT * FROM GENERATE_SERIES(1, 5)", "SELECT * FROM GENERATE_SERIES(1, 5)")
	assertRoundTrip(t, "sqlite", "SELECT INSTR(haystack, needle)", "SELECT INSTR(haystack, needle)")
	assertRoundTrip(t, "sqlite", "SELECT SQLITE_VERSION()", "SELECT SQLITE_VERSION()")
	assertRoundTrip(t, "sqlite", "SELECT RANK() OVER (RANGE CURRENT ROW) FROM tbl",
		"SELECT RANK() OVER (RANGE CURRENT ROW) FROM tbl")
	assertRoundTrip(t, "sqlite", "SELECT STRFTIME('%s')", "SELECT STRFTIME('%s', CURRENT_TIMESTAMP)")
	// CREATE TRIGGER bodies are not parsed structurally; they stay verbatim Commands.
	trigger := "CREATE TRIGGER log_insert AFTER INSERT ON users BEGIN INSERT INTO audit_log (user_id, action, created_at) VALUES (NEW.id, 'INSERT', datetime('now')) END"
	assertRoundTrip(t, "sqlite", trigger, trigger)
}

// TestSQLiteDateDiff pins the JULIANDAY conversion from test_datediff
// (tests/dialects/test_sqlite.py:315-327).
func TestSQLiteDateDiff(t *testing.T) {
	cases := map[string]string{
		"DATEDIFF(a, b, 'day')":  "CAST((JULIANDAY(a) - JULIANDAY(b)) AS INTEGER)",
		"DATEDIFF(a, b, 'hour')": "CAST((JULIANDAY(a) - JULIANDAY(b)) * 24.0 AS INTEGER)",
		"DATEDIFF(a, b, 'year')": "CAST((JULIANDAY(a) - JULIANDAY(b)) / 365.0 AS INTEGER)",
	}
	for sql, want := range cases {
		expr, err := sqlglot.ParseOne(sql, "sqlite")
		if err != nil {
			t.Fatalf("parse %q: %v", sql, err)
		}
		got, err := sqlglot.Generate(expr, "sqlite", generator.Options{})
		if err != nil {
			t.Fatalf("generate %q: %v", sql, err)
		}
		if got != want {
			t.Fatalf("datediff mismatch\n  sql:  %s\n  got:  %s\n  want: %s", sql, got, want)
		}
	}
}
