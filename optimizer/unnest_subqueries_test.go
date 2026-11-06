package optimizer_test

import (
	"testing"

	"github.com/maltzsama/sqlglot-go/optimizer"
)

// TestUnnestSubqueriesFixtures ports test_unnest_subqueries (tests/test_optimizer.py:1333-1334).
func TestUnnestSubqueriesFixtures(t *testing.T) {
	checkPassFixtures(t, "unnest_subqueries.sql", false, optimizer.UnnestSubqueries)
}
