package optimizer_test

import (
	"testing"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/optimizer"
	"github.com/maltzsama/sqlglot-go/schema"
)

// TestMergeSubqueriesFixtures ports test_merge_subqueries (tests/test_optimizer.py:1505-1516).
// Upstream runs optimize() with rules [qualify_tables, qualify_columns, merge_subqueries]; the
// same three rules run here in order with optimize()'s keyword arguments, since optimize() is
// not ported yet (#32). Upstream's execute=True duckdb comparison is not reproduced.
func TestMergeSubqueriesFixtures(t *testing.T) {
	schema := optimizerTestSchema()
	for i, pair := range loadSQLFixturePairs(t, "merge_subqueries.sql") {
		dialect := pair.Meta["dialect"]
		leaveTablesIsolated := stringToBool(pair.Meta["leave_tables_isolated"])
		canonicalizeTableAliases := stringToBool(pair.Meta["canonicalize_table_aliases"])
		pair := pair
		pair.Meta = map[string]string{"title": pair.Meta["title"], "dialect": dialect}
		checkPassFixturePair(t, i, pair, false, func(e exp.Expression) exp.Expression {
			e = optimizer.QualifyTables(e, "", "", dialect, canonicalizeTableAliases, nil, nil, nil)
			e = optimizer.QualifyColumns(e, schema, true, true, nil, false, dialect)
			return optimizer.MergeSubqueries(e, leaveTablesIsolated)
		})
	}
}

// TestMergeSubqueriesKeepsAliasColumns pins DEVIATIONS §1.25: renaming an inner table source that
// conflicts with an outer one keeps its alias column list. Upstream drops it, producing
// `FROM t1 AS q_2 ... q_2.x` although t1 has no column x.
func TestMergeSubqueriesKeepsAliasColumns(t *testing.T) {
	const sql = "SELECT s.x FROM (SELECT q.x FROM t1 AS q(x, y)) AS s JOIN t2 AS q ON s.x = q.a"
	const want = "SELECT q_2.x AS x FROM t1 AS q_2(x, y) JOIN t2 AS q ON q_2.x = q.a"
	mapping := schema.M("t1", schema.M("a", "INT", "b", "INT"), "t2", schema.M("a", "INT"))

	e := mustParse(t, sql)
	e = optimizer.QualifyTables(e, "", "", "", false, nil, nil, nil)
	e = optimizer.QualifyColumns(e, mapping, true, true, nil, false, "")
	if got := mustSQL(t, optimizer.MergeSubqueries(e, false)); got != want {
		t.Fatalf("MergeSubqueries = %q, want %q", got, want)
	}
}
