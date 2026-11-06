package optimizer_test

import (
	"testing"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/optimizer"
)

// TestPushdownProjectionsFixtures ports test_pushdown_projection (tests/test_optimizer.py:
// 1191-1192), with the test file's pushdown_projections wrapper (lines 48-52): qualify_tables,
// qualify_columns with infer_schema, then pushdown_projections.
func TestPushdownProjectionsFixtures(t *testing.T) {
	schema := optimizerTestSchema()
	infer := true
	// EXPLODE, POSEXPLODE and INLINE still parse as Anonymous under the base dialect (#44), so
	// the set-returning projection isn't recognized.
	gaps := map[int]string{
		52: "EXPLODE parses as Anonymous (#44)",
		53: "POSEXPLODE parses as Anonymous (#44)",
		54: "INLINE parses as Anonymous (#44)",
	}
	checkPassFixturesWith(t, "pushdown_projections.sql", false, gaps, func(e exp.Expression, _ string) exp.Expression {
		e = optimizer.QualifyTables(e, "", "", "", false, nil, nil, nil)
		e = optimizer.QualifyColumns(e, schema, true, true, &infer, false, "")
		return optimizer.PushdownProjections(e, schema, true, "", nil)
	})
}

// TestJournalPushdownProjections ports the pushdown_projections part of test_journal
// (tests/test_optimizer.py:1464-1478).
func TestJournalPushdownProjections(t *testing.T) {
	var journal optimizer.Journal
	schema := optimizerTestSchema()
	opts := optimizer.DefaultQualifyOpts()
	opts.Schema = schema
	opts.QuoteIdentifiers = false
	expression := optimizer.Qualify(mustParse(t, "SELECT a FROM (SELECT a, b FROM x) AS t"), opts)
	sql := mustSQL(t, expression)

	optimizer.PushdownProjections(expression, schema, true, "", &journal)
	if got := mustSQL(t, expression); got != "SELECT t.a AS a FROM (SELECT x.a AS a FROM x AS x) AS t" {
		t.Fatalf("pushdown = %q", got)
	}
	journal.Revert()
	if got := mustSQL(t, expression); got != sql {
		t.Fatalf("revert = %q, want %q", got, sql)
	}
}
