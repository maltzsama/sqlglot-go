package optimizer_test

import (
	"testing"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/optimizer"
)

// TestNormalize ports test_normalize (tests/test_optimizer.py:362-388). Its Snowflake BOOLXOR
// case waits for that dialect; the fixture run uses the test file's normalize wrapper (lines
// 55-60): normalize to CNF, annotate_types, simplify.
func TestNormalize(t *testing.T) {
	if got := mustSQL(t, optimizer.Normalize(mustParse(t, "x AND (y OR z)"), true, 128)); got != "(x AND y) OR (x AND z)" {
		t.Errorf("DNF = %q", got)
	}
	if got := mustSQL(t, optimizer.Normalize(mustParse(t, "x AND (y OR z)"), false, 128)); got != "x AND (y OR z)" {
		t.Errorf("CNF = %q", got)
	}

	schema := optimizerTestSchema()
	checkPassFixtures(t, "normalize.sql", false, func(e exp.Expression) exp.Expression {
		e = optimizer.Normalize(e, false, 128)
		e = optimizer.AnnotateTypes(e, schema, "")
		return optimizer.Simplify(e, false, false, "")
	})
}
