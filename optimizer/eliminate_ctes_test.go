package optimizer_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
	"github.com/maltzsama/sqlglot-go/optimizer"
)

// TestEliminateCTEsFixtures ports test_eliminate_ctes (tests/test_optimizer.py:1413-1418).
func TestEliminateCTEsFixtures(t *testing.T) {
	checkPassFixtures(t, "eliminate_ctes.sql", true, func(e exp.Expression) exp.Expression {
		return optimizer.EliminateCTEs(e, nil)
	})
}

func mustParse(t *testing.T, sql string) exp.Expression {
	t.Helper()
	e, err := sqlglot.ParseOne(sql, "")
	if err != nil {
		t.Fatalf("ParseOne(%q): %v", sql, err)
	}
	return e
}

func mustSQL(t *testing.T, e exp.Expression) string {
	t.Helper()
	sql, err := sqlglot.Generate(e, "", generator.Options{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return sql
}

// TestJournal ports the journal and eliminate_ctes parts of test_journal
// (tests/test_optimizer.py:1420-1462); the pushdown_projections part lands with that pass.
func TestJournal(t *testing.T) {
	var journal optimizer.Journal

	expression := mustParse(t, "SELECT a, b FROM x")
	journal.Record(expression, "expressions")
	expression.Set("expressions", []exp.Expression{expression.Expressions()[0]})
	if got := mustSQL(t, expression); got != "SELECT a FROM x" {
		t.Fatalf("after set: %q", got)
	}
	journal.Revert()
	if got := mustSQL(t, expression); got != "SELECT a, b FROM x" {
		t.Fatalf("after revert: %q", got)
	}
	if len(journal) != 0 {
		t.Fatalf("journal not emptied: %v", journal)
	}
	for i, selection := range expression.Expressions() {
		if selection.Parent() != expression || selection.ArgKey() != "expressions" || selection.Index() != i {
			t.Fatalf("selection %d not reattached: parent=%v key=%q index=%d", i, selection.Parent(), selection.ArgKey(), selection.Index())
		}
	}

	for _, tc := range []struct{ sql, want string }{
		// Removing the last CTE pops the entire WITH clause.
		{"WITH y AS (SELECT a FROM x) SELECT a FROM z", "SELECT a FROM z"},
		// The WITH clause survives when other CTEs remain.
		{"WITH y AS (SELECT a FROM x), z AS (SELECT b FROM x) SELECT b FROM z", "WITH z AS (SELECT b FROM x) SELECT b FROM z"},
		// Chained removals record the same WITH clause multiple times.
		{"WITH y AS (SELECT a FROM x), z AS (SELECT a FROM y) SELECT 1 AS c", "SELECT 1 AS c"},
	} {
		expression := mustParse(t, tc.sql)
		optimizer.EliminateCTEs(expression, &journal)
		if got := mustSQL(t, expression); got != tc.want {
			t.Fatalf("EliminateCTEs(%q) = %q, want %q", tc.sql, got, tc.want)
		}
		journal.Revert()
		if got := mustSQL(t, expression); got != tc.sql {
			t.Fatalf("revert of %q = %q", tc.sql, got)
		}
	}
}
