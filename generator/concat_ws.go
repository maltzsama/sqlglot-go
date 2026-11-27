package generator

import (
	"strings"

	expressions "github.com/maltzsama/sqlglot-go/expressions"
)

// concatWsSQL ports concatws_sql (generator.py:3726-3744). A dialect whose CONCAT_WS skips NULL
// arguments gets a CASE that returns NULL when any argument, the delimiter included, is NULL.
func (g *Generator) concatWsSQL(e expressions.Expression) string {
	if g.dialect.ConcatWsCoalesce && !boolValue(e.Arg("coalesce")) {
		all := listFromValue(e.Arg("expressions"))
		parts := make([]string, 0, len(all))
		for _, a := range all {
			if arg := asExpression(a); arg != nil {
				rendered := g.gen(arg)
				// Parenthesize a compound argument: `NOT x IS NULL` parses as NOT (x IS NULL) on Trino.
				if !atomicConcatArg(arg) {
					rendered = "(" + rendered + ")"
				}
				parts = append(parts, rendered+" IS NULL")
			}
		}
		e.Set("coalesce", true)
		return "CASE WHEN " + strings.Join(parts, " OR ") + " THEN NULL ELSE " + g.gen(e) + " END"
	}
	args := listFromValue(e.Arg("expressions"))
	if len(args) == 0 {
		return g.funcCall("CONCAT_WS", nil, "(", ")", true)
	}
	// convert_concat_args skips the delimiter (generator.py:3680-3681).
	rest := e.Copy()
	rest.Set("expressions", args[1:])
	converted := g.convertConcatArgs(rest)
	return g.funcCall("CONCAT_WS", append([]any{args[0]}, converted...), "(", ")", true)
}

// atomicConcatArg reports whether an argument renders as one token group, so it needs no
// parentheses before IS NULL.
func atomicConcatArg(e expressions.Expression) bool {
	switch e.Kind() {
	case expressions.KindColumn, expressions.KindLiteral, expressions.KindIdentifier, expressions.KindParen, expressions.KindNull, expressions.KindVar:
		return true
	}
	return false
}
