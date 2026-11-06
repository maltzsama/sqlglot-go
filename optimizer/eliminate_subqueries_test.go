package optimizer_test

import (
	"testing"

	"github.com/maltzsama/sqlglot-go/optimizer"
)

// TestEliminateSubqueriesFixtures ports test_eliminate_subqueries
// (tests/test_optimizer.py:1518-1519).
func TestEliminateSubqueriesFixtures(t *testing.T) {
	checkPassFixtures(t, "eliminate_subqueries.sql", false, optimizer.EliminateSubqueries)
}
