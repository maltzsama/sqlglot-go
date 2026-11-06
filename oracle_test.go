package sqlglot_test

import (
	"strings"
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/dialects"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
	"github.com/maltzsama/sqlglot-go/optimizer"
	"github.com/maltzsama/sqlglot-go/schema"
)

// TestOracleIdentity is the same-dialect round-trip oracle for Oracle, ported from
// .reference/sqlglot-v30.17.0/tests/dialects/test_oracle.py. Cases are already in their
// canonical Oracle spelling.
func TestOracleIdentity(t *testing.T) {
	cases := []string{
		"SELECT 1 FROM DUAL",
		"SELECT TRUNC(SYSDATE, 'DD') FROM DUAL",
		"SELECT TO_DATE('2024-12-12', 'YYYY-MM-DD') FROM DUAL",
		"SELECT TO_TIMESTAMP('05 Dec 2000 10:00 A.M.', 'DD Mon YYYY HH12:MI A.M.') FROM DUAL",
		"SELECT TO_NUMBER('dino' DEFAULT 0 ON CONVERSION ERROR, '9999') FROM DUAL",
		"SELECT NVL(NULL, 1) FROM DUAL",
		"SELECT LISTAGG(last_name, '; ' ON OVERFLOW ERROR) WITHIN GROUP (ORDER BY hire_date) FROM employees",
		"SELECT LISTAGG(last_name, '; ' ON OVERFLOW TRUNCATE '...' WITH COUNT) WITHIN GROUP (ORDER BY hire_date) FROM employees",
		"SELECT MIN(column_name) KEEP (DENSE_RANK FIRST ORDER BY column_name DESC) FROM table_name",
		"SELECT last_name, employee_id, manager_id, LEVEL FROM employees START WITH employee_id = 100 CONNECT BY PRIOR employee_id = manager_id ORDER SIBLINGS BY last_name",
		"SELECT * FROM sales UNPIVOT(q FOR p IN (q1 AS 'Prod1', q2 AS 'Prod2'))",
		"SELECT * FROM t PIVOT(SUM(t.val) FOR cat IN ('a' AS a)) p",
		"SELECT * FROM t SAMPLE (0.25)",
		"SELECT * FROM t WHERE JSON_EXISTS(name, '$[1].middle' PASSING 'name1' AS \"var1\" TRUE ON ERROR)",
		"SELECT DBMS_RANDOM.VALUE() FROM DUAL",
		"SELECT department_id BULK COLLECT INTO v_department_ids FROM departments",
		"SELECT department_id, department_name INTO v_department_id, v_department_name FROM departments FETCH FIRST 1 ROWS ONLY",
		"INSERT ALL INTO t1 (id) VALUES (id) INTO t2 (id) VALUES (id) SELECT id FROM src",
		"INSERT FIRST WHEN id <= 3 THEN INTO t1 (id) VALUES (id) ELSE INTO t2 (id) VALUES (id) SELECT id FROM src",
		"SELECT CAST(NULL AS VARCHAR2(2328 BYTE)) AS COL1 FROM DUAL",
		"SELECT CAST(NULL AS VARCHAR2(2328 CHAR)) AS COL1 FROM DUAL",
		"SELECT TO_DATE('January 15, 1989, 11:00 A.M.', 'Month dd, YYYY, HH12:MI A.M.') FROM DUAL",
		"SELECT DISTINCT col1, col2 FROM tbl",
		"SELECT e1.x, e2.x FROM e e1, e e2 WHERE e1.y (+) = e2.y",
		"CREATE OR REPLACE FORCE VIEW foo1.foo2",
	}
	for _, sql := range cases {
		t.Run(sql, func(t *testing.T) {
			expr, err := sqlglot.ParseOne(sql, "oracle")
			if err != nil {
				t.Fatalf("ParseOne: %v", err)
			}
			got, err := sqlglot.Generate(expr, "oracle", generator.Options{})
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if got != sql {
				t.Fatalf("round-trip mismatch\n want: %s\n  got: %s", sql, got)
			}
		})
	}
}

// TestOracleASTShapes asserts the structured nodes behind the Oracle grammar, so a
// degradation to raw-text Command (which would still round-trip) fails the test.
func TestOracleASTShapes(t *testing.T) {
	tests := []struct {
		name  string
		sql   string
		kind  exp.Kind
		finds []exp.Kind
	}{
		{
			name:  "date literal",
			sql:   "SELECT DATE '2022-01-01' FROM DUAL",
			kind:  exp.KindSelect,
			finds: []exp.Kind{exp.KindDateStrToDate},
		},
		{
			name:  "sysdate",
			sql:   "SELECT SYSDATE FROM DUAL",
			kind:  exp.KindSelect,
			finds: []exp.Kind{exp.KindCurrentTimestamp},
		},
		{
			name:  "dbms random",
			sql:   "SELECT DBMS_RANDOM.VALUE(1, 10) FROM DUAL",
			kind:  exp.KindSelect,
			finds: []exp.Kind{exp.KindRand},
		},
		{
			name:  "keep dense rank",
			sql:   "SELECT MIN(x) KEEP (DENSE_RANK FIRST ORDER BY y) FROM t",
			kind:  exp.KindSelect,
			finds: []exp.Kind{exp.KindWindow},
		},
		{
			name:  "bulk collect into",
			sql:   "SELECT id BULK COLLECT INTO ids FROM t",
			kind:  exp.KindSelect,
			finds: []exp.Kind{exp.KindInto},
		},
		{
			name:  "json exists",
			sql:   "SELECT * FROM t WHERE JSON_EXISTS(name, '$.a' TRUE ON ERROR)",
			kind:  exp.KindSelect,
			finds: []exp.Kind{exp.KindJSONExists, exp.KindOnCondition},
		},
		{
			name:  "multi table insert all",
			sql:   "INSERT ALL INTO t1 (id) VALUES (id) INTO t2 (id) VALUES (id) SELECT id FROM src",
			kind:  exp.KindMultitableInserts,
			finds: []exp.Kind{exp.KindConditionalInsert, exp.KindInsert},
		},
		{
			name:  "multi table insert first",
			sql:   "INSERT FIRST WHEN id <= 3 THEN INTO t1 (id) VALUES (id) ELSE INTO t2 (id) VALUES (id) SELECT id FROM src",
			kind:  exp.KindMultitableInserts,
			finds: []exp.Kind{exp.KindConditionalInsert},
		},
		{
			name:  "trunc default unit",
			sql:   "SELECT TRUNC(SYSDATE) FROM DUAL",
			kind:  exp.KindSelect,
			finds: []exp.Kind{exp.KindDateTrunc},
		},
		{
			name:  "connect by root",
			sql:   "SELECT CONNECT_BY_ROOT x AS y FROM t",
			kind:  exp.KindSelect,
			finds: []exp.Kind{exp.KindConnectByRoot},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := sqlglot.ParseOne(tt.sql, "oracle")
			if err != nil {
				t.Fatalf("ParseOne: %v", err)
			}
			if expr.Kind() != tt.kind {
				t.Fatalf("root kind = %s, want %s:\n%s", exp.ClassName(expr.Kind()), exp.ClassName(tt.kind), expr.ToS())
			}
			if expr.Find(exp.KindCommand) != nil {
				t.Fatalf("statement degraded to Command:\n%s", expr.ToS())
			}
			for _, k := range tt.finds {
				if expr.Find(k) == nil {
					t.Fatalf("tree is missing %s:\n%s", exp.ClassName(k), expr.ToS())
				}
			}
		})
	}
}

// TestOracleDialectFlags covers the flags that change Oracle's parse/generate behavior.
func TestOracleDialectFlags(t *testing.T) {
	d, err := dialects.GetOrRaise("oracle")
	if err != nil {
		t.Fatalf("GetOrRaise: %v", err)
	}
	if d.NormalizationStrategy != dialects.Uppercase {
		t.Fatalf("normalization = %v, want Uppercase", d.NormalizationStrategy)
	}
	if !d.SupportsColumnJoinMarks {
		t.Fatal("SupportsColumnJoinMarks should be true")
	}
	if d.LimitFetch != "FETCH" {
		t.Fatalf("LimitFetch = %q, want FETCH", d.LimitFetch)
	}
	if d.TablesampleKeywords != "SAMPLE" {
		t.Fatalf("TablesampleKeywords = %q, want SAMPLE", d.TablesampleKeywords)
	}
	if d.PivotAliasWithAs {
		t.Fatal("PivotAliasWithAs should be false")
	}
	if d.OnConditionEmptyBeforeError {
		t.Fatal("OnConditionEmptyBeforeError should be false")
	}
	if !d.Pseudocolumns["ROWNUM"] || !d.Pseudocolumns["LEVEL"] {
		t.Fatalf("pseudocolumns = %v, want ROWNUM and LEVEL", d.Pseudocolumns)
	}

	// SELECT UNIQUE is old-style Oracle DISTINCT.
	expr, err := sqlglot.ParseOne("SELECT UNIQUE a, b FROM t", "oracle")
	if err != nil {
		t.Fatalf("ParseOne: %v", err)
	}
	out, err := sqlglot.Generate(expr, "oracle", generator.Options{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out != "SELECT DISTINCT a, b FROM t" {
		t.Fatalf("SELECT UNIQUE rewrote to %q", out)
	}
}

// TestOracleQualifyAndScope checks the optimizer's qualify pass on an Oracle query.
func TestOracleQualifyAndScope(t *testing.T) {
	d := dialects.Oracle()
	mapping, err := schema.NewMappingSchema(schema.M(
		"DB", schema.M("T", schema.M("A", "INT", "B", "VARCHAR2")),
	), d, true)
	if err != nil {
		t.Fatalf("NewMappingSchema: %v", err)
	}
	expr, err := sqlglot.ParseOne("SELECT a, b FROM t WHERE a > 0", d)
	if err != nil {
		t.Fatalf("ParseOne: %v", err)
	}
	opts := optimizer.DefaultQualifyOpts()
	opts.Dialect = d
	opts.Schema = mapping
	opts.DefaultSchema = "DB"
	opts.QuoteIdentifiers = false
	opts.Identify = false
	qualified := optimizer.Qualify(expr, opts)
	out, err := sqlglot.Generate(qualified, "oracle", generator.Options{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, want := range []string{"T", "A", "B"} {
		if !strings.Contains(out, want) {
			t.Fatalf("qualified SQL %q missing %q", out, want)
		}
	}
	scopes := optimizer.TraverseScope(qualified)
	if len(scopes) != 1 {
		t.Fatalf("scopes = %d, want 1", len(scopes))
	}
	if _, ok := scopes[0].Sources["T"]; !ok {
		t.Fatalf("scope sources = %v, want T", scopes[0].Sources)
	}
}

// TestOracleSlashTerminator pins the SQL*Plus/SQLcl `/` terminator (DEVIATIONS §1.23): a line
// holding only `/` ends the statement and never parses as division. Upstream lexes it as a
// division operator and fails.
func TestOracleSlashTerminator(t *testing.T) {
	sel := exp.KindSelect
	cases := []struct {
		sql  string
		want []scriptStatement
	}{
		// Upstream: "Invalid expression / Unexpected token".
		{"SELECT 1 FROM dual\n/\nSELECT 2 FROM dual",
			[]scriptStatement{{sel, "SELECT 1 FROM dual"}, {sel, "SELECT 2 FROM dual"}}},
		// Upstream: "Required keyword: 'this' missing for Div". After a `;` it is a no-op.
		{"SELECT 1 FROM dual;\n/", []scriptStatement{{sel, "SELECT 1 FROM dual"}}},
		// Surrounding blanks, a trailing terminator, and CRLF line endings.
		{"SELECT 1 FROM dual\n  /  \nSELECT 2 FROM dual\n/",
			[]scriptStatement{{sel, "SELECT 1 FROM dual"}, {sel, "SELECT 2 FROM dual"}}},
		{"SELECT 1 FROM dual\r\n/\r\nSELECT 2 FROM dual",
			[]scriptStatement{{sel, "SELECT 1 FROM dual"}, {sel, "SELECT 2 FROM dual"}}},
		// Division is untouched when `/` shares its line with an operand.
		{"SELECT a\n/ b FROM t", []scriptStatement{{sel, "SELECT a\n/ b FROM t"}}},
		{"SELECT 10 /\n2 FROM dual", []scriptStatement{{sel, "SELECT 10 /\n2 FROM dual"}}},
	}
	for _, tc := range cases {
		assertScriptStatements(t, tc.sql, "oracle", tc.want)
	}

	// `//` is not a terminator line, so it still parses as (invalid) division.
	if _, err := sqlglot.Parse("SELECT 1 FROM dual\n//\nSELECT 2 FROM dual", "oracle"); err == nil {
		t.Error("`//` line: expected a parse error, it is not a terminator")
	}
	// A block comment opening a line is not the terminator either.
	assertScriptStatements(t, "SELECT 1\n/* c */ + 1 FROM dual", "oracle",
		[]scriptStatement{{sel, "SELECT 1\n/* c */ + 1 FROM dual"}})

	// Other dialects keep upstream's lexing: the line is division.
	assertScriptStatements(t, "SELECT a\n/\nb", "", []scriptStatement{{sel, "SELECT a\n/\nb"}})
}

// TestOraclePLSQLUnitBoundaries pins PL/SQL unit boundaries (DEVIATIONS §1.24): a stored unit or
// an anonymous block is one statement whose span runs through its final `END [name]`, however
// many `;` its body holds. Upstream cuts each unit at the first `;` (a truncated Command plus a
// stray END), fails on anonymous blocks, and mangles a CREATE PROCEDURE body.
func TestOraclePLSQLUnitBoundaries(t *testing.T) {
	create, command, sel := exp.KindCreate, exp.KindCommand, exp.KindSelect
	cases := []struct {
		sql  string
		want []scriptStatement
	}{
		// The issue's inputs.
		{"CREATE OR REPLACE PROCEDURE p IS BEGIN NULL; END;",
			[]scriptStatement{{create, "CREATE OR REPLACE PROCEDURE p IS BEGIN NULL; END"}}},
		{"CREATE OR REPLACE FUNCTION f RETURN NUMBER IS BEGIN RETURN 1; END;",
			[]scriptStatement{{create, "CREATE OR REPLACE FUNCTION f RETURN NUMBER IS BEGIN RETURN 1; END"}}},
		{"CREATE OR REPLACE TRIGGER trg BEFORE INSERT ON t FOR EACH ROW BEGIN :new.id := 1; END;",
			[]scriptStatement{{create, "CREATE OR REPLACE TRIGGER trg BEFORE INSERT ON t FOR EACH ROW BEGIN :new.id := 1; END"}}},
		{"BEGIN NULL; END;", []scriptStatement{{command, "BEGIN NULL; END"}}},
		{"DECLARE x NUMBER; BEGIN x := 1; END;", []scriptStatement{{command, "DECLARE x NUMBER; BEGIN x := 1; END"}}},
		// The `/` terminator after a unit (§1.23) closes it; the next statement stands alone.
		{"CREATE OR REPLACE PROCEDURE p IS BEGIN UPDATE t SET a = 1; COMMIT; END;\n/\nSELECT 1 FROM dual",
			[]scriptStatement{
				{create, "CREATE OR REPLACE PROCEDURE p IS BEGIN UPDATE t SET a = 1; COMMIT; END"},
				{sel, "SELECT 1 FROM dual"},
			}},
		{"BEGIN NULL; END;\n/\nBEGIN NULL; END;\n/",
			[]scriptStatement{{command, "BEGIN NULL; END"}, {command, "BEGIN NULL; END"}}},
		// Nesting: a local subprogram, IF/LOOP/CASE statements with their `END IF`/`END LOOP`/
		// `END CASE`, a CASE expression, and a labeled final END.
		{"CREATE PROCEDURE p IS x NUMBER; PROCEDURE inner IS BEGIN NULL; END; BEGIN " +
			"IF x > 1 THEN inner; ELSIF x = 0 THEN NULL; END IF; " +
			"FOR i IN 1..3 LOOP x := CASE WHEN i = 1 THEN 1 ELSE 2 END; END LOOP; " +
			"CASE x WHEN 1 THEN NULL; END CASE; END p;\nSELECT 2 FROM dual",
			[]scriptStatement{
				{create, "CREATE PROCEDURE p IS x NUMBER; PROCEDURE inner IS BEGIN NULL; END; BEGIN " +
					"IF x > 1 THEN inner; ELSIF x = 0 THEN NULL; END IF; " +
					"FOR i IN 1..3 LOOP x := CASE WHEN i = 1 THEN 1 ELSE 2 END; END LOOP; " +
					"CASE x WHEN 1 THEN NULL; END CASE; END p"},
				{sel, "SELECT 2 FROM dual"},
			}},
		// A package spec has no BEGIN: its AS opens the unit, forward declarations end in `;`.
		{"CREATE OR REPLACE PACKAGE pkg AS PROCEDURE p; FUNCTION f RETURN NUMBER; END pkg;\nSELECT 1 FROM dual",
			[]scriptStatement{
				{create, "CREATE OR REPLACE PACKAGE pkg AS PROCEDURE p; FUNCTION f RETURN NUMBER; END pkg"},
				{sel, "SELECT 1 FROM dual"},
			}},
		{"CREATE OR REPLACE PACKAGE BODY pkg AS PROCEDURE p IS BEGIN NULL; END p; BEGIN NULL; END pkg;",
			[]scriptStatement{{create, "CREATE OR REPLACE PACKAGE BODY pkg AS PROCEDURE p IS BEGIN NULL; END p; BEGIN NULL; END pkg"}}},
		{"CREATE OR REPLACE TYPE BODY t AS MEMBER FUNCTION f RETURN NUMBER IS BEGIN RETURN 1; END; END;",
			[]scriptStatement{{create, "CREATE OR REPLACE TYPE BODY t AS MEMBER FUNCTION f RETURN NUMBER IS BEGIN RETURN 1; END; END"}}},
		{"CREATE OR REPLACE EDITIONABLE PROCEDURE p AS BEGIN NULL; END;",
			[]scriptStatement{{create, "CREATE OR REPLACE EDITIONABLE PROCEDURE p AS BEGIN NULL; END"}}},
		// A compound trigger: each timing point is its own BEGIN...END inside the trigger body.
		{"CREATE OR REPLACE TRIGGER trg FOR INSERT ON t COMPOUND TRIGGER x NUMBER; " +
			"BEFORE STATEMENT IS BEGIN x := 0; END BEFORE STATEMENT; END trg;",
			[]scriptStatement{{create, "CREATE OR REPLACE TRIGGER trg FOR INSERT ON t COMPOUND TRIGGER x NUMBER; " +
				"BEFORE STATEMENT IS BEGIN x := 0; END BEFORE STATEMENT; END trg"}}},
		// A call specification has no PL/SQL body, so its first `;` ends it.
		{"CREATE FUNCTION f RETURN NUMBER AS LANGUAGE C NAME \"f\" LIBRARY lib;\nSELECT 1 FROM dual",
			[]scriptStatement{
				{create, "CREATE FUNCTION f RETURN NUMBER AS LANGUAGE C NAME \"f\" LIBRARY lib"},
				{sel, "SELECT 1 FROM dual"},
			}},
		// `;` inside string literals never counts.
		{"BEGIN x := 'a;b'; y := q'[c;d]'; END;\nSELECT 1 FROM dual",
			[]scriptStatement{{command, "BEGIN x := 'a;b'; y := q'[c;d]'; END"}, {sel, "SELECT 1 FROM dual"}}},
		// Units mixed with ordinary statements.
		{"SELECT 1 FROM dual; BEGIN NULL; END; SELECT 2 FROM dual",
			[]scriptStatement{{sel, "SELECT 1 FROM dual"}, {command, "BEGIN NULL; END"}, {sel, "SELECT 2 FROM dual"}}},
		// A CREATE TYPE spec is not a PL/SQL unit and keeps the ordinary path.
		{"CREATE OR REPLACE TYPE t2 AS OBJECT (a NUMBER); SELECT 1 FROM dual",
			[]scriptStatement{{command, "CREATE OR REPLACE TYPE t2 AS OBJECT (a NUMBER)"}, {sel, "SELECT 1 FROM dual"}}},
	}
	for _, tc := range cases {
		assertScriptStatements(t, tc.sql, "oracle", tc.want)
	}

	// The node names the unit and keeps its text verbatim, so it round-trips byte for byte.
	for _, tc := range []struct {
		sql  string
		root exp.Kind
		kind string
	}{
		{"CREATE OR REPLACE PROCEDURE p IS\n  x NUMBER; -- note\nBEGIN\n  NULL;\nEND p", create, "PROCEDURE"},
		{"CREATE OR REPLACE PACKAGE BODY pkg AS BEGIN NULL; END pkg", create, "PACKAGE BODY"},
		{"CREATE OR REPLACE TYPE BODY t AS MEMBER FUNCTION f RETURN NUMBER IS BEGIN RETURN 1; END; END", create, "TYPE BODY"},
		{"BEGIN NULL; END", command, ""},
	} {
		e, err := sqlglot.ParseOne(tc.sql, "oracle")
		if err != nil {
			t.Fatalf("%q: %v", tc.sql, err)
		}
		if e.Kind() != tc.root || e.Text("kind") != tc.kind {
			t.Errorf("%q: got %s kind=%q, want %s kind=%q", tc.sql, exp.ClassName(e.Kind()), e.Text("kind"), exp.ClassName(tc.root), tc.kind)
		}
		got, err := sqlglot.Generate(e, "oracle", generator.Options{})
		if err != nil || got != tc.sql {
			t.Errorf("%q: round-trip %q (err %v)", tc.sql, got, err)
		}
	}

	// A unit still open at the end of input, or at a `/` line, fails closed instead of becoming
	// a truncated statement.
	for _, sql := range []string{
		"BEGIN NULL;",
		"BEGIN NULL;\n/\nSELECT 1 FROM dual",
		"CREATE OR REPLACE PROCEDURE p IS BEGIN IF x THEN NULL; END;",
	} {
		if _, err := sqlglot.Parse(sql, "oracle"); err == nil || !strings.Contains(err.Error(), "Unterminated PL/SQL block") {
			t.Errorf("%q: got err %v, want an unterminated PL/SQL block error", sql, err)
		}
	}
}
