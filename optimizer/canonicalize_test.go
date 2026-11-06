package optimizer_test

import (
	"testing"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/optimizer"
)

// TestCanonicalizeFixtures ports test_canonicalize (tests/test_optimizer.py:1794-1804): optimize
// with the rules qualify, quote_identifiers, annotate_types and canonicalize, each given the
// schema and the fixture's dialect.
func TestCanonicalizeFixtures(t *testing.T) {
	schema := optimizerTestSchema()
	checkPassFixturesWith(t, "canonicalize.sql", false, canonicalizeGaps, func(e exp.Expression, dialect string) exp.Expression {
		opts := optimizer.DefaultQualifyOpts()
		opts.Schema = schema
		opts.Dialect = dialect
		e = optimizer.Qualify(e, opts)
		e = optimizer.QuoteIdentifiers(e, dialect, true)
		e = optimizer.AnnotateTypes(e, schema, dialect)
		return optimizer.Canonicalize(e, dialect)
	})
}

// canonicalizeGaps are fixtures whose input still parses differently from upstream (#44):
// TIMESTAMP and DATE_TRUNC as Anonymous, and a DATEDIFF unit as a Column instead of a Var.
var canonicalizeGaps = map[int]string{
	18: "TIMESTAMP parses as Anonymous (#44)",
	29: "DATE_TRUNC parses as Anonymous (#44)",
	30: "DATE_TRUNC parses as Anonymous (#44)",
	31: "DATEDIFF unit parses as a Column, not a Var (#44)",
	41: "DATE_TRUNC parses as Anonymous (#44)",
	42: "DATE_TRUNC parses as Anonymous (#44)",
}
