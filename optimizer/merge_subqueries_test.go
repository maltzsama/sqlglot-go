package optimizer_test

import (
	"testing"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/optimizer"
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
