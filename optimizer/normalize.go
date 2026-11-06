package optimizer

import exp "github.com/maltzsama/sqlglot-go/expressions"

// normalized ports normalized (optimizer/normalize.py:70-91): whether expression is in CNF
// (dnf false) or DNF (dnf true), i.e. no connector of the root kind sits under one of the
// ancestor kind. The rest of normalize lands with #29.
func normalized(expression exp.Expression, dnf bool) bool {
	ancestor, root := exp.KindOr, exp.KindAnd
	if dnf {
		ancestor, root = exp.KindAnd, exp.KindOr
	}
	for _, connector := range findAllInScope(expression, root) {
		if connector.FindAncestor(ancestor) != nil {
			return false
		}
	}
	return true
}
