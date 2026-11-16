package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
)

// Statement-level source spans (Go-only, DEVIATIONS §6.2): every expression Parse returns is
// stamped with the rune span + verbatim text of its statement, so a batch consumer can split a
// multi-statement input into the exact submitted bytes — no Generate() rewrite. A statement
// whose body consumed several semicolon chunks (a procedure BEGIN...END, spanning its inner
// semicolons) is one statement with one span.
func TestStatementSpans(t *testing.T) {
	cases := []struct {
		sql     string
		dialect string
		want    []string // per-statement verbatim span text
	}{
		{"SELECT 'a;b' FROM t; SELECT 2", "postgres",
			[]string{"SELECT 'a;b' FROM t", "SELECT 2"}},
		{"select id,   ssn from t -- keep me", "postgres",
			[]string{"select id,   ssn from t -- keep me"}},
		{"CREATE PROCEDURE p() BEGIN UPDATE t SET a = 1; SELECT 1; END", "mysql",
			[]string{"CREATE PROCEDURE p() BEGIN UPDATE t SET a = 1; SELECT 1; END"}},
		// divergence (DEVIATIONS §1.18): END terminates the block, so a statement after it is
		// its own statement — upstream folds SELECT 9 into the procedure's Block, but real
		// PostgreSQL 16 (BEGIN ATOMIC) runs the trailing statement separately.
		{"CREATE PROCEDURE p() BEGIN SELECT 1; END; SELECT 9", "mysql",
			[]string{"CREATE PROCEDURE p() BEGIN SELECT 1; END", "SELECT 9"}},
		{"CREATE FUNCTION f() RETURNS INT AS $fn$ SELECT '$$'; $fn$ LANGUAGE plpgsql; SELECT 9", "postgres",
			[]string{"CREATE FUNCTION f() RETURNS INT AS $fn$ SELECT '$$'; $fn$ LANGUAGE plpgsql", "SELECT 9"}},
		// The trailing-comment `;` chunk parses to exp.Semicolon; the comment rides on the
		// chunk's `;` and stays inside its span.
		{"SELECT 1; -- note", "postgres",
			[]string{"SELECT 1", "; -- note"}},
		// A comment-bearing `;` mid-batch is its own chunk; the following bare `;` leaves an
		// empty chunk = a nil slot ("" below), matching upstream's [Select, Semicolon, None,
		// Select].
		{"SELECT 1; /* c */ ; SELECT 2", "postgres",
			[]string{"SELECT 1", "; /* c */", "", "SELECT 2"}},
		// Rune (not byte) offsets: multibyte text before and inside a statement.
		{"SELECT '한글' FROM t; SELECT 'β'", "postgres",
			[]string{"SELECT '한글' FROM t", "SELECT 'β'"}},
		// The span covers the statement's tokens plus its attached comments: leading trivia
		// (whitespace) and inter-statement whitespace sit in gaps outside every span, but a
		// leading comment is part of the statement.
		{"  /* c */  SELECT 1;   \n  SELECT 2", "postgres",
			[]string{"/* c */  SELECT 1", "SELECT 2"}},
		// An activated MySQL executable comment splices body tokens into the statement; the
		// span is widened back to the `/*!NNNNN ... */` delimiters (Token.WrapStart/WrapEnd) so
		// the slice stays executable MySQL — both trailing and leading wrapper positions.
		{"SELECT 1 /*!40101 + 2 */; SELECT 3", "mysql, mysql_version=80035",
			[]string{"SELECT 1 /*!40101 + 2 */", "SELECT 3"}},
		{"/*!40101 SELECT */ 1", "mysql, mysql_version=80035",
			[]string{"/*!40101 SELECT */ 1"}},
	}
	for _, tc := range cases {
		statements, err := sqlglot.Parse(tc.sql, tc.dialect)
		if err != nil {
			t.Errorf("%q [%s]: parse: %v", tc.sql, tc.dialect, err)
			continue
		}
		if len(statements) != len(tc.want) {
			t.Errorf("%q [%s]: got %d statements, want %d", tc.sql, tc.dialect, len(statements), len(tc.want))
			continue
		}
		runes := []rune(tc.sql)
		for i, stmt := range statements {
			if tc.want[i] == "" {
				if stmt != nil {
					t.Errorf("%q [%s] stmt %d: got %s, want nil (empty chunk)", tc.sql, tc.dialect, i, exp.ClassName(stmt.Kind()))
				}
				continue
			}
			start, end, ok := stmt.Span()
			if !ok {
				t.Errorf("%q [%s] stmt %d: Span() not set", tc.sql, tc.dialect, i)
				continue
			}
			if got := string(runes[start:end]); got != tc.want[i] {
				t.Errorf("%q [%s] stmt %d: span slice %q, want %q", tc.sql, tc.dialect, i, got, tc.want[i])
			}
			if text, ok := stmt.SpanText(); !ok || text != tc.want[i] {
				t.Errorf("%q [%s] stmt %d: SpanText()=%q,%v, want %q", tc.sql, tc.dialect, i, text, ok, tc.want[i])
			}
		}
	}
}

// A `;` inside an activated executable comment splits the body across two statements; no
// source slice can represent either half with its wrapper, so both spans stay ABSENT (never
// a wrong slice like `/*!40101 SELECT 1` or `SELECT 2 */`).
func TestExecutableCommentSplitStatementSpansAbsent(t *testing.T) {
	statements, err := sqlglot.Parse("/*!40101 SELECT 1; SELECT 2 */", "mysql, mysql_version=80035")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(statements) != 2 {
		t.Fatalf("got %d statements, want 2", len(statements))
	}
	for i, stmt := range statements {
		if _, _, ok := stmt.Span(); ok {
			text, _ := stmt.SpanText()
			t.Errorf("stmt %d: span set (%q), want absent", i, text)
		}
	}
}

// Wrap widening is statement-level ONLY: a projection whose boundary token closes the
// executable comment must keep its own narrow span, not inherit the comment's full extent.
func TestExecutableCommentProjectionSpansNotWidened(t *testing.T) {
	sql := "/*!40101 SELECT a, b */"
	e, err := sqlglot.ParseOne(sql, "mysql, mysql_version=80035")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if text, _ := e.SpanText(); text != sql {
		t.Errorf("statement SpanText()=%q, want %q", text, sql)
	}
	projections := e.Expressions()
	if len(projections) != 2 {
		t.Fatalf("got %d projections, want 2", len(projections))
	}
	for i, want := range []string{"a", "b"} {
		if text, ok := projections[i].SpanText(); !ok || text != want {
			t.Errorf("projection %d SpanText()=%q,%v, want %q", i, text, ok, want)
		}
	}
}

// A trailing `;` whose chunk carries only a comment parses to exp.Semicolon holding the
// comment (parser.py:1110, 2159-2161) — upstream parity; previously this parse-errored.
func TestTrailingCommentSemicolon(t *testing.T) {
	for _, dialect := range []string{"base", "mysql", "postgres"} {
		statements, err := sqlglot.Parse("SELECT 1; -- note", dialect)
		if err != nil {
			t.Fatalf("[%s] parse: %v", dialect, err)
		}
		if len(statements) != 2 {
			t.Fatalf("[%s] got %d statements, want 2", dialect, len(statements))
		}
		if statements[1].Kind() != exp.KindSemicolon {
			t.Errorf("[%s] statements[1] Kind=%s, want Semicolon", dialect, exp.ClassName(statements[1].Kind()))
		}
		if comments := statements[1].Comments(); len(comments) != 1 || comments[0] != " note" {
			t.Errorf("[%s] comments=%q, want [\" note\"]", dialect, comments)
		}
		// generator.py:5421-5422: the node body renders empty; only the comment survives.
		if got, err := sqlglot.Generate(statements[1], dialect, generator.Options{}); err != nil || got != "/* note */" {
			t.Errorf("[%s] Generate(Semicolon)=%q,%v, want \"/* note */\"", dialect, got, err)
		}
	}
}

// TestStatementSpansCoverCommentsAndOracleUnits pins the issue #74 contract: a statement span
// covers its attached comments (leading on the first token, trailing on the same line, before a
// `;`, or at EOF), and an Oracle PL/SQL unit's terminating `;` is part of the unit. A `;` inside
// a comment or a string is never a boundary, and a call specification with no END keeps its `;`
// outside the span, as an ordinary statement terminator.
func TestStatementSpansCoverCommentsAndOracleUnits(t *testing.T) {
	cases := []struct {
		sql     string
		dialect string
		want    []string
	}{
		{"/* lead */ SELECT 1", "postgres", []string{"/* lead */ SELECT 1"}},
		{"-- lead\nSELECT 1", "postgres", []string{"-- lead\nSELECT 1"}},
		{"SELECT 1 /* tail */", "postgres", []string{"SELECT 1 /* tail */"}},
		{"SELECT 'a;b' -- c;d", "postgres", []string{"SELECT 'a;b' -- c;d"}},
		// The next statement's leading comment starts a fresh run: the `;` chunk keeps
		// `-- tail`, and the following `-- lead` belongs only to SELECT 2.
		{"SELECT 1; -- tail\n-- lead\nSELECT 2", "postgres",
			[]string{"SELECT 1", "; -- tail", "-- lead\nSELECT 2"}},
		// Consecutive trailing comments on one statement are one run, so both stay inside it.
		{"SELECT 1 -- a\n-- b\nFROM t", "postgres", []string{"SELECT 1 -- a\n-- b\nFROM t"}},
		{"SELECT 1 /* t */; SELECT 2", "postgres", []string{"SELECT 1 /* t */", "SELECT 2"}},
		// The `;` inside the comment is not a boundary; the comment trails the first statement.
		{"SELECT 1 -- a;b\n; SELECT 2", "postgres", []string{"SELECT 1 -- a;b", "SELECT 2"}},
		{"BEGIN NULL; END;", "oracle", []string{"BEGIN NULL; END;"}},
		{"CREATE PROCEDURE p AS BEGIN NULL; END;", "oracle", []string{"CREATE PROCEDURE p AS BEGIN NULL; END;"}},
		{`CREATE FUNCTION f RETURN NUMBER AS LANGUAGE C NAME "f" LIBRARY lib; SELECT 1 FROM dual`, "oracle",
			[]string{`CREATE FUNCTION f RETURN NUMBER AS LANGUAGE C NAME "f" LIBRARY lib`, "SELECT 1 FROM dual"}},
	}
	for _, tc := range cases {
		statements, err := sqlglot.Parse(tc.sql, tc.dialect)
		if err != nil {
			t.Fatalf("%q [%s]: parse: %v", tc.sql, tc.dialect, err)
		}
		if len(statements) != len(tc.want) {
			t.Fatalf("%q [%s]: got %d statements, want %d", tc.sql, tc.dialect, len(statements), len(tc.want))
		}
		for i, stmt := range statements {
			if stmt == nil {
				t.Fatalf("%q [%s] stmt %d: got nil, want %q", tc.sql, tc.dialect, i, tc.want[i])
			}
			if text, ok := stmt.SpanText(); !ok || text != tc.want[i] {
				t.Fatalf("%q [%s] stmt %d: span %q, want %q", tc.sql, tc.dialect, i, text, tc.want[i])
			}
			start, end, ok := stmt.Span()
			if !ok {
				t.Fatalf("%q [%s] stmt %d: no span", tc.sql, tc.dialect, i)
			}
			runes := []rune(tc.sql)
			if start < 0 || end > len(runes) {
				t.Fatalf("%q [%s] stmt %d: span %d..%d out of range", tc.sql, tc.dialect, i, start, end)
			}
			if got := string(runes[start:end]); got != tc.want[i] {
				t.Fatalf("%q [%s] stmt %d: span slice %q, want %q", tc.sql, tc.dialect, i, got, tc.want[i])
			}
		}
	}
}
