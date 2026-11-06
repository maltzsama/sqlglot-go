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
