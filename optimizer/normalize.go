package optimizer

import (
	"fmt"

	sqlerrors "github.com/maltzsama/sqlglot-go/errors"
	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// Normalize ports normalize (optimizer/normalize.py:15-67): it rewrites the AST into conjunctive
// normal form, or disjunctive normal form when dnf is true. A connector whose estimated
// conversion distance exceeds maxDistance is left alone.
//
//	(x AND y) OR z  ->  (x OR z) AND (y OR z)
func Normalize(expression exp.Expression, dnf bool, maxDistance int) exp.Expression {
	simplifier := NewSimplifier(nil, false)

	nodes := expression.WalkWithPrune(true, func(e exp.Expression) bool { return e.Is(exp.TraitConnector) })
	for _, node := range nodes {
		if !node.Is(exp.TraitConnector) || normalized(node, dnf) {
			continue
		}
		root := node == expression
		original := node.Copy()

		node.Transform(simplifier.rewriteBetween, false)
		if distance := normalizationDistance(node, dnf, maxDistance); distance > maxDistance {
			// Skipping normalization because the distance exceeds max_distance.
			return expression
		}

		normalizedNode, failed := func() (result exp.Expression, failed bool) {
			defer func() {
				if r := recover(); r != nil {
					if _, ok := r.(*sqlerrors.OptimizeError); ok {
						failed = true
						return
					}
					panic(r)
				}
			}()
			return simplifier.whileChanging(node, func(e exp.Expression) exp.Expression {
				return distributiveLaw(e, dnf, maxDistance, simplifier)
			}), false
		}()
		if failed {
			node.Replace(original)
			if root {
				return original
			}
			return expression
		}
		node = node.Replace(normalizedNode)
		if root {
			expression = node
		}
	}
	return expression
}

// normalized ports normalized (optimizer/normalize.py:70-91): whether expression is in CNF
// (dnf false) or DNF (dnf true), i.e. no connector of the root kind sits under one of the
// ancestor kind.
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

// normalizationDistance ports normalization_distance (optimizer/normalize.py:94-121): the
// difference in predicate count between expression and its normal form, an estimate of the
// conversion's (exponential) cost. It stops early once the count exceeds max.
//
//	(a AND b) OR (c AND d)  ->  4
func normalizationDistance(expression exp.Expression, dnf bool, max int) int {
	total := -(len(expression.FindAll(exp.TraitConnector)) + 1)
	stop := false
	predicateLengths(expression, dnf, max, 0, func(length int) bool {
		total += length
		if total > max {
			stop = true
		}
		return !stop
	})
	return total
}

// predicateLengths ports _predicate_lengths (optimizer/normalize.py:124-149): the predicate
// lengths when expanded to normal form, (A AND B) OR C -> [2, 2]. yield returns false to stop.
func predicateLengths(expression exp.Expression, dnf bool, max, depth int, yield func(int) bool) bool {
	if depth > max {
		return yield(depth)
	}
	expression = expression.Unnest()
	if !expression.Is(exp.TraitConnector) {
		return yield(1)
	}
	depth++
	left, right := expression.Left(), expression.Right()

	distributing := exp.KindOr
	if dnf {
		distributing = exp.KindAnd
	}
	if expression.Kind() == distributing {
		// The nested generators of upstream: the right side is walked again for each left value.
		return predicateLengths(left, dnf, max, depth, func(a int) bool {
			return predicateLengths(right, dnf, max, depth, func(b int) bool { return yield(a + b) })
		})
	}
	if !predicateLengths(left, dnf, max, depth, yield) {
		return false
	}
	return predicateLengths(right, dnf, max, depth, yield)
}

// replaceChildren ports exp.replace_children (expressions/builders.py:654-680): each child
// expression is replaced by fun(child).
func replaceChildren(expression exp.Expression, fun func(exp.Expression) exp.Expression) {
	for _, key := range argOrder(expression) {
		switch v := expression.Arg(key).(type) {
		case []exp.Expression:
			out := make([]exp.Expression, 0, len(v))
			for _, child := range v {
				if child != nil {
					out = append(out, fun(child))
				}
			}
			expression.Set(key, out)
		case exp.Expression:
			if v != nil {
				expression.Set(key, fun(v))
			}
		}
	}
}

// distributiveLaw ports distributive_law (optimizer/normalize.py:152-188):
//
//	x OR (y AND z) -> (x OR y) AND (x OR z)
//	(x AND y) OR (y AND z) -> (x OR y) AND (x OR z) AND (y OR y) AND (y OR z)
func distributiveLaw(expression exp.Expression, dnf bool, maxDistance int, simplifier *Simplifier) exp.Expression {
	if normalized(expression, dnf) {
		return expression
	}
	if distance := normalizationDistance(expression, dnf, maxDistance); distance > maxDistance {
		panic(&sqlerrors.OptimizeError{Msg: fmt.Sprintf("Normalization distance %d exceeds max %d", distance, maxDistance)})
	}

	replaceChildren(expression, func(e exp.Expression) exp.Expression {
		return distributiveLaw(e, dnf, maxDistance, nil)
	})
	toKind, fromKind := exp.KindAnd, exp.KindOr
	if dnf {
		toKind, fromKind = exp.KindOr, exp.KindAnd
	}

	if expression.Kind() == fromKind {
		a, b := expression.Left().Unnest(), expression.Right().Unnest()
		if simplifier == nil {
			simplifier = NewSimplifier(nil, false)
		}
		switch {
		case a.Kind() == toKind && b.Kind() == toKind:
			if len(a.FindAll(exp.TraitConnector)) > len(b.FindAll(exp.TraitConnector)) {
				return distribute(a, b, fromKind, toKind, simplifier)
			}
			return distribute(b, a, fromKind, toKind, simplifier)
		case a.Kind() == toKind:
			return distribute(b, a, fromKind, toKind, simplifier)
		case b.Kind() == toKind:
			return distribute(a, b, fromKind, toKind, simplifier)
		}
	}
	return expression
}

// copyingCombine is exp.and_ / exp.or_ with their default copy=True: the operands are copied.
func copyingCombine(kind exp.Kind, a, b exp.Expression) exp.Expression {
	return combine(kind, a.Copy(), b.Copy())
}

// distribute ports _distribute (optimizer/normalize.py:191-228).
func distribute(a, b exp.Expression, fromKind, toKind exp.Kind, simplifier *Simplifier) exp.Expression {
	// When a and b are connectors of the same polarity (e.g. both AND in CNF mode), b is
	// distributed across a's children so the cross-product handling can simplify them.
	if a.Is(exp.TraitConnector) && a.Kind() == b.Kind() {
		replaceChildren(a, func(c exp.Expression) exp.Expression {
			return combine(toKind,
				simplifier.uniqSort(flattenConnector(copyingCombine(fromKind, c, b.Left())), true),
				simplifier.uniqSort(flattenConnector(copyingCombine(fromKind, c, b.Right())), true),
			)
		})
		return a
	}

	// Otherwise apply the textbook rule a OR (b.left AND b.right) -> (a OR b.left) AND
	// (a OR b.right), or its AND/OR dual.
	return combine(toKind,
		simplifier.uniqSort(flattenConnector(copyingCombine(fromKind, a, b.Left())), true),
		simplifier.uniqSort(flattenConnector(copyingCombine(fromKind, a, b.Right())), true),
	)
}
