package optimizer

import (
	"github.com/maltzsama/sqlglot-go/dialects"
	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// NormalizeIdentifiers folds unquoted identifiers per the dialect's normalization strategy,
// mirroring upstream sqlglot's normalize_identifiers(expression, dialect: DialectType).
//
// The walker itself lives on dialects.Dialect so the parser can share it without an import
// cycle — see the note on Dialect.NormalizeIdentifiers (dialects/normalize_identifiers.go).
func NormalizeIdentifiers(expression exp.Expression, dialect dialects.DialectType) exp.Expression {
	d, err := dialects.GetOrRaise(dialect)
	if err != nil {
		panic(err)
	}
	return d.NormalizeIdentifiers(expression)
}

func NormalizeIdentifiersString(name string, dialect dialects.DialectType) exp.Expression {
	// ParseIdentifier only needs the dialect's tokenizer/quoting (strategy-independent), so
	// resolve any->name for it; NormalizeIdentifiers still applies the full strategy.
	d, err := dialects.GetOrRaise(dialect)
	if err != nil {
		panic(err)
	}
	return NormalizeIdentifiers(exp.ParseIdentifier(name, d.Name), dialect)
}
