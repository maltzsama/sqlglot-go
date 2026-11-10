package dialects

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// NormalizeIdentifiers folds unquoted identifiers in the tree per this dialect's normalization
// strategy, mirroring upstream's normalize_identifiers(expression, dialect) walker
// (optimizer/normalize_identifiers.py:34-78).
//
// The walk lives on the Dialect instead of in the optimizer package because the parser needs it
// too: upstream's _implicit_unnests_to_explicit imports it lazily inside the function
// (parser.py:4306) precisely to dodge a module-level cycle. Go has no lazy imports, so keeping
// the walk in the optimizer would make parser -> optimizer cycle against the optimizer's own
// internal tests, which reach the parser through the root package.
func (d *Dialect) NormalizeIdentifiers(expression exp.Expression) exp.Expression {
	if expression == nil {
		return nil
	}
	// Ports normalize_identifiers.py:66-78: prune subtrees under a case_sensitive-marked node
	// and skip such nodes, normalizing only the rest. (store_original_column_identifiers is off
	// by default, so its dot_parts branch is not ported here.) NB: the only upstream producer of
	// the case_sensitive meta is add_comments parsing a `/* sqlglot.meta case_sensitive */`
	// annotation (core.py:1044), which this port does not yet parse — so this guard is currently
	// inert (nothing sets the flag) but kept structurally faithful for when that lands.
	caseSensitive := func(n exp.Expression) bool {
		b, _ := n.MetaGet("case_sensitive").(bool)
		return b
	}
	for _, node := range expression.WalkWithPrune(true, caseSensitive) {
		if caseSensitive(node) {
			continue
		}
		if node.Kind() == exp.KindIdentifier {
			if d.OpaqueFunctions && isOpaqueCallIdentifier(node) {
				// opaque_functions contract: a function call's name and schema qualifier stay
				// spelled as written — the EngineCatalog resolver folds for lookup itself.
				continue
			}
			d.NormalizeIdentifier(node)
		}
	}
	return expression
}

// isOpaqueCallIdentifier reports whether the identifier is an Anonymous call's quoted-name
// ("this") or the schema qualifier of Dot(Identifier, Anonymous).
func isOpaqueCallIdentifier(node exp.Expression) bool {
	parent := node.Parent()
	if parent == nil {
		return false
	}
	if parent.Kind() == exp.KindAnonymous && node.ArgKey() == "this" {
		return true
	}
	if parent.Kind() == exp.KindDot && node.ArgKey() == "this" {
		if e, ok := parent.Arg("expression").(exp.Expression); ok && e != nil && e.Kind() == exp.KindAnonymous {
			return true
		}
	}
	return false
}
