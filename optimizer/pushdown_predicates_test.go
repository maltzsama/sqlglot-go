package optimizer_test

import (
	"testing"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/optimizer"
)

// TestPushdownPredicatesFixtures ports test_pushdown_predicates (tests/test_optimizer.py:1336-1337).
func TestPushdownPredicatesFixtures(t *testing.T) {
	checkPassFixturesWith(t, "pushdown_predicates.sql", false, nil, func(e exp.Expression, dialect string) exp.Expression {
		return optimizer.PushdownPredicates(e, dialect)
	})
}
