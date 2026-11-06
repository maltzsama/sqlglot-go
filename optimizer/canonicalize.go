package optimizer

import (
	"github.com/maltzsama/sqlglot-go/dialects"
	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// Canonicalize ports canonicalize (optimizer/canonicalize.py:12-35): it converts a SQL
// expression into a standard form. Many of the conversions rely on annotate_types.
func Canonicalize(expression exp.Expression, dialect dialects.DialectType) exp.Expression {
	d, err := dialects.GetOrRaise(dialect)
	if err != nil {
		panic(err)
	}
	return replaceTree(expression, func(e exp.Expression) exp.Expression {
		if !isCanonicalizeType(e) {
			return e
		}
		e = addTextToConcat(e)
		e = replaceDateFuncs(e, d)
		e = coerceType(e, d.PromoteToInferredDatetimeType)
		e = removeRedundantCasts(e)
		e = ensureBools(e, replaceIntPredicate)
		e = removeAscendingOrder(e)
		return e
	})
}

// replaceTree ports exp.replace_tree (expressions/builders.py:682-705): each node, leaves first
// (reverse DFS), is replaced by fun(node); new nodes are traversed too.
func replaceTree(expression exp.Expression, fun func(exp.Expression) exp.Expression) exp.Expression {
	stack := expression.Walk(false)
	newNode := expression
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		newNode = fun(node)
		if newNode != node {
			node.Replace(newNode)
			if newNode != nil {
				stack = append(stack, newNode)
			}
		}
	}
	return newNode
}

// coercibleDateOps ports COERCIBLE_DATE_OPS (optimizer/canonicalize.py:38-49).
var coercibleDateOps = map[exp.Kind]bool{
	exp.KindAdd: true, exp.KindSub: true, exp.KindEQ: true, exp.KindNEQ: true, exp.KindGT: true,
	exp.KindGTE: true, exp.KindLT: true, exp.KindLTE: true, exp.KindNullSafeEQ: true,
	exp.KindNullSafeNEQ: true,
}

// isCanonicalizeType ports _CANONICALIZE_TYPES (optimizer/canonicalize.py:53-80): the node
// types any canonicalize function acts on.
func isCanonicalizeType(e exp.Expression) bool {
	if coercibleDateOps[e.Kind()] || e.Is(exp.TraitConnector) || isClass(e, "Timestamp") {
		return true
	}
	switch e.Kind() {
	case exp.KindDate, exp.KindTsOrDsToDate, exp.KindBetween, exp.KindExtract, exp.KindDateAdd,
		exp.KindDateSub, exp.KindDateTrunc, exp.KindDateDiff, exp.KindCast, exp.KindNot, exp.KindIf,
		exp.KindWhere, exp.KindHaving, exp.KindOrdered:
		return true
	}
	return false
}

// nodeTypeThis is node.type.this: the DType of node's annotated type, if any.
func nodeTypeThis(node exp.Expression) (exp.DType, bool) {
	if node == nil || node.Type() == nil {
		return "", false
	}
	dt, ok := node.Type().Arg("this").(exp.DType)
	return dt, ok
}

// addTextToConcat ports add_text_to_concat (optimizer/canonicalize.py:83-91).
func addTextToConcat(node exp.Expression) exp.Expression {
	if node.Kind() == exp.KindAdd {
		if dt, ok := nodeTypeThis(node); ok && exp.TextTypes[dt] {
			// All known dialects that concatenate strings with + (Redshift, T-SQL) don't
			// coalesce NULLs.
			return exp.Concat(exp.Args{"expressions": []exp.Expression{node.Left(), node.Right()}, "coalesce": false})
		}
	}
	return node
}

// replaceDateFuncs ports replace_date_funcs (optimizer/canonicalize.py:94-109).
func replaceDateFuncs(node exp.Expression, dialect *dialects.Dialect) exp.Expression {
	if (node.Kind() == exp.KindDate || node.Kind() == exp.KindTsOrDsToDate) &&
		len(node.Expressions()) == 0 && node.Arg("zone") == nil &&
		node.This() != nil && node.This().IsString() && isISODate(node.This().Name()) {
		return buildCast(node.This(), exp.DTypeDate)
	}
	if isClass(node, "Timestamp") && node.Arg("zone") == nil {
		if node.Type() == nil {
			node = AnnotateTypes(node, nil, dialect)
		}
		if node.Type() != nil {
			return buildCast(node.This(), node.Type())
		}
		return buildCast(node.This(), exp.DTypeTimestamp)
	}
	return node
}

// coerceType ports coerce_type (optimizer/canonicalize.py:112-127).
func coerceType(node exp.Expression, promote bool) exp.Expression {
	switch {
	case coercibleDateOps[node.Kind()]:
		canonicalizeCoerceDate(node.Left(), node.Right(), promote)
	case node.Kind() == exp.KindBetween:
		canonicalizeCoerceDate(node.This(), asExpression(node.Arg("low")), promote)
	case node.Kind() == exp.KindExtract:
		if e := node.Expr(); e != nil && !e.IsType(dtypeSet(exp.TemporalTypes)...) {
			replaceCast(e, exp.DTypeDatetime)
		}
	case node.Kind() == exp.KindDateAdd || node.Kind() == exp.KindDateSub || node.Kind() == exp.KindDateTrunc:
		coerceTimeunitArg(node.This(), asExpression(node.Arg("unit")))
	case node.Kind() == exp.KindDateDiff:
		coerceDatediffArgs(node)
	}
	return node
}

// removeRedundantCasts ports remove_redundant_casts (optimizer/canonicalize.py:130-146).
func removeRedundantCasts(expression exp.Expression) exp.Expression {
	this := expression.This()
	if expression.Kind() == exp.KindCast && this != nil && this.Type() != nil {
		if to := asExpression(expression.Arg("to")); to != nil && to.Equal(this.Type()) {
			return this
		}
	}
	if (expression.Kind() == exp.KindDate || expression.Kind() == exp.KindTsOrDsToDate) && this != nil {
		if dt, ok := nodeTypeThis(this); ok && dt == exp.DTypeDate && len(this.Type().Expressions()) == 0 {
			return this
		}
	}
	return expression
}

// ensureBools ports ensure_bools (optimizer/canonicalize.py:149-163).
func ensureBools(expression exp.Expression, replace func(exp.Expression)) exp.Expression {
	switch {
	case expression.Is(exp.TraitConnector):
		replace(expression.Left())
		replace(expression.Right())
	case expression.Kind() == exp.KindNot:
		replace(expression.This())
	case expression.Kind() == exp.KindIf:
		// num in CASE x WHEN num ... can't be replaced: it isn't the full predicate.
		if parent := expression.Parent(); parent == nil || parent.Kind() != exp.KindCase || parent.This() == nil {
			replace(expression.This())
		}
	case expression.Kind() == exp.KindWhere || expression.Kind() == exp.KindHaving:
		replace(expression.This())
	}
	return expression
}

// removeAscendingOrder ports remove_ascending_order (optimizer/canonicalize.py:166-171):
// ORDER BY a ASC becomes ORDER BY a.
func removeAscendingOrder(expression exp.Expression) exp.Expression {
	if expression.Kind() == exp.KindOrdered {
		if desc, ok := expression.Arg("desc").(bool); ok && !desc {
			expression.Set("desc", nil)
		}
	}
	return expression
}

// canonicalizeCoerceDate ports _coerce_date (optimizer/canonicalize.py:174-217).
func canonicalizeCoerceDate(a0, b0 exp.Expression, promote bool) {
	if a0 == nil || b0 == nil {
		return
	}
	// itertools.permutations([a, b]): (a, b) then (b, a), on the original nodes.
	for _, pair := range [][2]exp.Expression{{a0, b0}, {b0, a0}} {
		a, b := pair[0], pair[1]
		if b.Kind() == exp.KindInterval {
			a = coerceTimeunitArg(a, asExpression(b.Arg("unit")))
		}

		aType, aOk := nodeTypeThis(a)
		bType, bOk := nodeTypeThis(b)
		if !aOk || !exp.TemporalTypes[aType] || !bOk || !exp.TextTypes[bType] {
			continue
		}

		// target is either a DType inferred from b, or a's own type; a DType never equals a's
		// DataType upstream, so a is recast exactly when the target comes from b.
		var target any = a.Type()
		recastA := false
		if promote {
			var inferred exp.DType
			if b.IsString() {
				switch text := b.Name(); {
				case isISODate(text):
					inferred = exp.DTypeDate
				case isISODatetime(text):
					inferred = exp.DTypeDatetime
				default:
					inferred = aType
				}
			} else {
				// A non-datetime string b is conservatively promoted to DATETIME, so there are
				// no surprising truncations from downcasting.
				inferred = exp.DTypeDatetime
			}
			if defaultCoercesTo[aType][inferred] {
				target, recastA = inferred, true
			}
		}

		if recastA {
			replaceCast(a, target)
		}
		replaceCast(b, target)
	}
}

// coerceTimeunitArg ports _coerce_timeunit_arg (optimizer/canonicalize.py:220-238).
func coerceTimeunitArg(arg, unit exp.Expression) exp.Expression {
	dt, ok := nodeTypeThis(arg)
	if !ok {
		return arg
	}
	if exp.TextTypes[dt] {
		text := arg.Name()
		isoDate := isISODate(text)
		if isoDate && isDateUnit(unit) {
			return arg.Replace(buildCast(arg.Copy(), exp.DTypeDate))
		}
		// An ISO date is also an ISO datetime, but not vice versa.
		if isoDate || isISODatetime(text) {
			return arg.Replace(buildCast(arg.Copy(), exp.DTypeDatetime))
		}
	} else if dt == exp.DTypeDate && !isDateUnit(unit) {
		return arg.Replace(buildCast(arg.Copy(), exp.DTypeDatetime))
	}
	return arg
}

// coerceDatediffArgs ports _coerce_datediff_args (optimizer/canonicalize.py:241-244).
func coerceDatediffArgs(node exp.Expression) {
	for _, e := range []exp.Expression{node.This(), node.Expr()} {
		if e == nil {
			continue
		}
		if dt, ok := nodeTypeThis(e); !ok || !exp.TemporalTypes[dt] {
			e.Replace(buildCast(e.Copy(), exp.DTypeDatetime))
		}
	}
}

// replaceCast ports _replace_cast (optimizer/canonicalize.py:247-248).
func replaceCast(node exp.Expression, to any) {
	node.Replace(buildCast(node.Copy(), to))
}

// replaceIntPredicate ports _replace_int_predicate (optimizer/canonicalize.py:251-262): an
// integer used as a predicate becomes `x <> 0`. Originally designed for Presto, which has a
// BOOLEAN type (unlike T-SQL's bits), so only integer types are rewritten.
func replaceIntPredicate(expression exp.Expression) {
	if expression == nil {
		return
	}
	if expression.Kind() == exp.KindCoalesce {
		for _, child := range expression.IterExpressions(false) {
			replaceIntPredicate(child)
		}
		return
	}
	if dt, ok := nodeTypeThis(expression); ok && exp.IntegerTypes[dt] {
		expression.Replace(binop(exp.KindNEQ, expression, literalNumber(pyNum{isInt: true, coef: bigZero()})))
	}
}
