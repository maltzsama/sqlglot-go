package generator

import "github.com/maltzsama/sqlglot-go/expressions"

// unnestToExplode ports transforms.unnest_to_explode (transforms.py:297-399): a cross join
// UNNEST becomes LATERAL VIEW EXPLODE / POSEXPLODE / INLINE(ARRAYS_ZIP(...)), and a FROM
// UNNEST becomes a table over the same UDTF.
func unnestToExplode(e expressions.Expression, unnestUsingArraysZip bool) expressions.Expression {
	if e.Kind() != expressions.KindSelect {
		return e
	}
	unnestToExplodeFrom(e, unnestUsingArraysZip)
	unnestToExplodeJoins(e, unnestUsingArraysZip)
	return e
}

func unnestToExplodeFrom(e expressions.Expression, unnestUsingArraysZip bool) {
	from := asExpression(e.Arg("from_"))
	if from == nil {
		return
	}
	unnest := asExpression(from.This())
	if unnest == nil || unnest.Kind() != expressions.KindUnnest {
		return
	}

	alias := asExpression(unnest.Arg("alias"))
	unnestExprs := unnest.Expressions()
	hasMulti := len(unnestExprs) > 1
	unnestExprs = unnestZipExprs(unnest, unnestExprs, hasMulti, unnestUsingArraysZip)
	if len(unnestExprs) == 0 {
		return
	}

	var columns []any
	if alias != nil {
		columns = listFromValue(alias.Arg("columns"))
	}
	if unnestOffsetIsExpression(unnest) {
		columns = append([]any{unnestOffsetColumn(unnest)}, columns...)
	}

	var aliasNode expressions.Expression
	if alias != nil {
		aliasNode = expressions.TableAlias(expressions.Args{
			"this":    alias.This(),
			"columns": columns,
		})
	}

	args := expressions.Args{"this": unnestUDTF(unnest, hasMulti, unnestExprs[0])}
	if aliasNode != nil {
		args["alias"] = aliasNode
	}
	unnest.Replace(expressions.Table(args))
}

func unnestToExplodeJoins(e expressions.Expression, unnestUsingArraysZip bool) {
	joins := listFromValue(e.Arg("joins"))
	if len(joins) == 0 {
		return
	}

	remaining := make([]any, 0, len(joins))
	for _, item := range joins {
		join := asExpression(item)
		if join == nil {
			continue
		}

		joinExpr := join.This()
		isLateral := joinExpr != nil && joinExpr.Kind() == expressions.KindLateral
		unnest := joinExpr
		if isLateral {
			unnest = joinExpr.This()
		}
		if unnest == nil || unnest.Kind() != expressions.KindUnnest {
			remaining = append(remaining, item)
			continue
		}

		alias := asExpression(unnest.Arg("alias"))
		if isLateral {
			alias = asExpression(joinExpr.Arg("alias"))
		}
		if alias == nil {
			// Upstream raises UnsupportedError; keep the join so nothing is silently lost.
			remaining = append(remaining, item)
			continue
		}

		unnestExprs := unnest.Expressions()
		hasMulti := len(unnestExprs) > 1
		unnestExprs = unnestZipExprs(unnest, unnestExprs, hasMulti, unnestUsingArraysZip)
		if len(unnestExprs) == 0 {
			remaining = append(remaining, item)
			continue
		}

		aliasCols := listFromValue(alias.Arg("columns"))
		if !hasMulti && len(aliasCols) != 1 && len(aliasCols) != 2 {
			// Upstream raises UnsupportedError when the aliases are missing or ambiguous.
			remaining = append(remaining, item)
			continue
		}
		if unnestOffsetIsExpression(unnest) {
			aliasCols = append([]any{unnestOffsetColumn(unnest)}, aliasCols...)
		}

		laterals := listFromValue(e.Arg("laterals"))
		for i := 0; i < len(unnestExprs) && i < len(aliasCols); i++ {
			laterals = append(laterals, expressions.Lateral(expressions.Args{
				"this": unnestUDTF(unnest, hasMulti, unnestExprs[i]),
				"view": true,
				"alias": expressions.TableAlias(expressions.Args{
					"this":    alias.This(),
					"columns": aliasCols,
				}),
			}))
		}
		e.Set("laterals", laterals)
	}

	if len(remaining) == 0 {
		e.Set("joins", nil)
	} else {
		e.Set("joins", remaining)
	}
}

// unnestZipExprs ports _unnest_zip_exprs: multiple UNNEST inputs become one ARRAYS_ZIP call
// when the target supports it. A target without arrays_zip returns nil (upstream raises).
func unnestZipExprs(unnest expressions.Expression, exprs []expressions.Expression, hasMulti, usingArraysZip bool) []expressions.Expression {
	if !hasMulti {
		return exprs
	}
	if !usingArraysZip {
		return nil
	}
	zipped := []expressions.Expression{
		expressions.Anonymous(expressions.Args{"this": "ARRAYS_ZIP", "expressions": exprs}),
	}
	unnest.Set("expressions", zipped)
	return zipped
}

// unnestUDTF ports _udtf_type: POSEXPLODE when an offset is present, INLINE for multiple
// inputs, EXPLODE otherwise.
func unnestUDTF(unnest expressions.Expression, hasMulti bool, expr expressions.Expression) expressions.Expression {
	name := "EXPLODE"
	switch {
	case unnestOffsetIsExpression(unnest):
		name = "POSEXPLODE"
	case hasMulti:
		name = "INLINE"
	}
	args := expressions.Args{"this": name}
	if expr != nil {
		args["expressions"] = []expressions.Expression{expr}
	}
	return expressions.Anonymous(args)
}

// unnestOffsetIsExpression reports whether UNNEST carries an offset (a bool true or the
// WITH OFFSET alias identifier; the parser stores false when there is none).
func unnestOffsetIsExpression(unnest expressions.Expression) bool {
	offset := unnest.Arg("offset")
	if offset == nil {
		return false
	}
	return truthy(offset)
}

func unnestOffsetColumn(unnest expressions.Expression) expressions.Expression {
	if offset, ok := unnest.Arg("offset").(expressions.Expression); ok && offset.Kind() == expressions.KindIdentifier {
		return offset
	}
	return expressions.ToIdentifier("pos")
}

// anyToExists ports transforms.any_to_exists (transforms.py:974-998): `x < ANY(arr)` becomes
// Spark's `EXISTS(arr, y -> x < y)`.
func anyToExists(e expressions.Expression) expressions.Expression {
	if e.Kind() != expressions.KindSelect {
		return e
	}
	for _, anyExpr := range e.FindAll(expressions.KindAny) {
		this := asExpression(anyExpr.This())
		if this == nil || this.Kind() == expressions.KindSubquery {
			continue
		}
		parent := anyExpr.Parent()
		if parent == nil {
			continue
		}
		if parent.Kind() == expressions.KindLike || parent.Kind() == expressions.KindILike {
			continue
		}
		if !parent.Is(expressions.TraitBinary) {
			continue
		}

		lambdaArg := expressions.ToIdentifier("x")
		anyExpr.Replace(lambdaArg)
		lambda := expressions.Lambda(expressions.Args{
			"this":        parent.Copy(),
			"expressions": []expressions.Expression{lambdaArg},
		})
		parent.Replace(expressions.Exists(expressions.Args{
			"this":       unnestParens(this),
			"expression": lambda,
		}))
	}
	return e
}

// unnestParens mirrors exp.Expr.unnest(): strip wrapping parentheses.
func unnestParens(e expressions.Expression) expressions.Expression {
	for e != nil && e.Kind() == expressions.KindParen {
		e = asExpression(e.This())
	}
	return e
}
