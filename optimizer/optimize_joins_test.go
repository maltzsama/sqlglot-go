package optimizer_test

import (
	"testing"

	"github.com/maltzsama/sqlglot-go/optimizer"
)

// TestOptimizeJoinsFixtures ports test_optimize_joins (tests/test_optimizer.py:1400-1404).
func TestOptimizeJoinsFixtures(t *testing.T) {
	checkPassFixtures(t, "optimize_joins.sql", false, optimizer.OptimizeJoins)
}
