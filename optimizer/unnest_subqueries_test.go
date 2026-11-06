package optimizer_test

import (
	"testing"

	"github.com/maltzsama/sqlglot-go/optimizer"
)

// TestUnnestSubqueriesFixtures ports test_unnest_subqueries (tests/test_optimizer.py:1333-1334).
func TestUnnestSubqueriesFixtures(t *testing.T) {
	checkPassFixtures(t, "unnest_subqueries.sql", false, optimizer.UnnestSubqueries)
}

// TestUnnestSubqueriesSetOperationScalar pins DEVIATIONS §1.26: a scalar set-operation subquery
// that would be rewritten under MAX has no parent to replace once rewrapped. Upstream raises
// AttributeError; the port leaves the query unchanged.
func TestUnnestSubqueriesSetOperationScalar(t *testing.T) {
	for _, sql := range []string{
		"SELECT x.a + (SELECT 1 UNION ALL SELECT 2) FROM x GROUP BY x.a",
		"SELECT x.a FROM x GROUP BY x.a HAVING x.a > (SELECT 1 AS a UNION ALL SELECT 2 AS a)",
	} {
		if got := mustSQL(t, optimizer.UnnestSubqueries(mustParse(t, sql))); got != sql {
			t.Errorf("UnnestSubqueries(%q) = %q, want it unchanged", sql, got)
		}
	}
}
