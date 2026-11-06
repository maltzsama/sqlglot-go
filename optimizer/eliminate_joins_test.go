package optimizer_test

import (
	"testing"

	"github.com/maltzsama/sqlglot-go/optimizer"
)

// TestEliminateJoinsFixtures ports test_eliminate_joins (tests/test_optimizer.py:1406-1411).
func TestEliminateJoinsFixtures(t *testing.T) {
	checkPassFixtures(t, "eliminate_joins.sql", true, optimizer.EliminateJoins)
}
