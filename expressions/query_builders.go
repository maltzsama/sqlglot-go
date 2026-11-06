package expressions

import "strings"

// Query builder methods, ported from the builder half of upstream expressions/query.py and
// expressions/core.py (sqlglot v30.17.0): the fluent API the optimizer's rewrite passes use to
// build and reshape queries in place.
//
// divergence (Go-forced): upstream builders accept either SQL strings, parsed through
// maybe_parse, or Expr instances. These accept only expressions, because the parser hooks
// (MaybeParseFunc) exist only once the root sqlglot package is imported, and the optimizer
// package must not depend on it. Callers build the nodes the upstream string form would
// parse to. Without parsing, the `into` wrapping of _apply_builder is applied directly.

// JoinOptions carries Select.join's keyword arguments (expressions/query.py:1365-1445).
type JoinOptions struct {
	// On is ANDed into the join's ON condition.
	On []Expression
	// Using lists the join's USING columns.
	Using []Expression
	// Replace resets the existing joins instead of appending (upstream append=False).
	Replace bool
	// JoinType alters the join type: any of the method/side/kind words of a join, e.g.
	// "LEFT", "CROSS", "LEFT OUTER", "NATURAL".
	JoinType string
	// JoinAlias is an optional table alias for the joined source.
	JoinAlias string
	// Copy builds on a copy of the select instead of modifying it in place.
	Copy bool
}

// applyConjunctionBuilder ports _apply_conjunction_builder (expressions/core.py:2735-2758).
// into, when not 0, wraps the combined condition (e.g. KindWhere).
func applyConjunctionBuilder(instance Expression, arg string, into Kind, appendValue, copyValue bool, expressions []Expression) Expression {
	filtered := make([]Expression, 0, len(expressions))
	for _, expression := range expressions {
		if expression != nil {
			filtered = append(filtered, expression)
		}
	}
	if len(filtered) == 0 {
		return instance
	}
	inst := maybeCopy(instance, copyValue)
	if existing := asExpression(inst.Arg(arg)); appendValue && existing != nil {
		if into != 0 {
			existing = existing.This()
		}
		filtered = append([]Expression{existing}, filtered...)
	}
	node := And_(copyAll(filtered, copyValue)...)
	if into != 0 {
		node = New(into, Args{"this": node})
	}
	inst.Set(arg, node)
	return inst
}

// applyListBuilder ports _apply_list_builder (expressions/core.py:2702-2732).
func applyListBuilder(instance Expression, arg string, appendValue, copyValue bool, expressions []Expression) Expression {
	inst := maybeCopy(instance, copyValue)
	parsed := []Expression{}
	for _, expression := range expressions {
		if expression != nil {
			parsed = append(parsed, expression)
		}
	}
	if existing := inst.Arg(arg); appendValue {
		if list := expressionsFromValue(existing, false); len(list) > 0 {
			parsed = append(append([]Expression{}, list...), parsed...)
		}
	}
	inst.Set(arg, parsed)
	return inst
}

// applyChildListBuilder ports _apply_child_list_builder (expressions/core.py:2655-2699): the
// expressions become the `expressions` of a fresh into node (e.g. a Group) set as arg.
func applyChildListBuilder(instance Expression, arg string, into Kind, appendValue, copyValue bool, expressions []Expression) Expression {
	inst := maybeCopy(instance, copyValue)
	var parsed []Expression
	for _, expression := range expressions {
		if expression == nil {
			continue
		}
		// _is_wrong_expression: an expression of another kind is wrapped as into(expressions=[e]),
		// whose args then contribute only that expression.
		if expression.Kind() != into {
			parsed = append(parsed, expression)
			continue
		}
		parsed = append(parsed, expression.Expressions()...)
	}
	if existing := asExpression(inst.Arg(arg)); appendValue && existing != nil {
		parsed = append(append([]Expression{}, existing.Expressions()...), parsed...)
	}
	inst.Set(arg, New(into, Args{"expressions": parsed}))
	return inst
}

func copyAll(expressions []Expression, copyValue bool) []Expression {
	if !copyValue {
		return expressions
	}
	out := make([]Expression, len(expressions))
	for i, expression := range expressions {
		out[i] = maybeCopy(expression, true)
	}
	return out
}

// Subquery ports Query.subquery (expressions/query.py:130-150): it wraps the query in a
// Subquery aliased alias ("" for none).
func (n *Node) Subquery(alias string, copyValue bool) Expression {
	instance := maybeCopy(n, copyValue)
	args := Args{"this": instance}
	if alias != "" {
		args["alias"] = TableAlias(Args{"this": ToIdentifier(alias)})
	}
	return Subquery(args)
}

// Where ports Query.where (expressions/query.py:265-300): it ANDs the conditions into the
// query's WHERE (a Where node's condition is taken as-is), replacing it when appendValue is false.
func (n *Node) Where(appendValue, copyValue bool, expressions ...Expression) Expression {
	conditions := make([]Expression, 0, len(expressions))
	for _, expression := range expressions {
		if expression != nil && expression.Kind() == KindWhere {
			expression = expression.This()
		}
		conditions = append(conditions, expression)
	}
	return applyConjunctionBuilder(n, "where", KindWhere, appendValue, copyValue, conditions)
}

// Having ports Select.having (expressions/query.py:1461-1500): it ANDs the conditions into the
// query's HAVING, replacing it when appendValue is false.
func (n *Node) Having(appendValue, copyValue bool, expressions ...Expression) Expression {
	return applyConjunctionBuilder(n, "having", KindHaving, appendValue, copyValue, expressions)
}

// From_ ports Select.from_ (expressions/query.py:1147-1180): it sets the FROM clause, wrapping
// a source that is not already a From.
func (n *Node) From_(expression Expression, copyValue bool) Expression {
	if expression != nil && expression.Kind() != KindFrom {
		expression = From(Args{"this": expression})
	}
	instance := maybeCopy(n, copyValue)
	instance.Set("from_", expression)
	return instance
}

// GroupBy ports Select.group_by (expressions/query.py:1183-1220).
func (n *Node) GroupBy(appendValue, copyValue bool, expressions ...Expression) Expression {
	if len(expressions) == 0 {
		return maybeCopy(n, copyValue)
	}
	return applyChildListBuilder(n, "group", KindGroup, appendValue, copyValue, expressions)
}

// SelectExprs ports Select.select (expressions/query.py:1307-1340): it appends projections,
// or replaces them when appendValue is false. (Named SelectExprs because Node.Selects already
// returns the projections.)
func (n *Node) SelectExprs(appendValue, copyValue bool, expressions ...Expression) Expression {
	return applyListBuilder(n, "expressions", appendValue, copyValue, expressions)
}

// Join ports Select.join (expressions/query.py:1365-1445) for expression arguments.
func (n *Node) Join(expression Expression, opts JoinOptions) Expression {
	join := expression
	if join.Kind() != KindJoin {
		join = New(KindJoin, Args{"this": expression})
	}
	if this := join.This(); this != nil && this.Kind() == KindSelect {
		this.Replace(this.(*Node).Subquery("", true))
	}

	if opts.JoinType != "" {
		// divergence (Go-forced): upstream parses `FROM _ <join_type> JOIN _` and copies the
		// parsed method/side/kind; the words are classified directly instead.
		for _, word := range strings.Fields(strings.ToUpper(opts.JoinType)) {
			switch word {
			case "NATURAL", "ASOF", "POSITIONAL":
				join.Set("method", word)
			case "LEFT", "RIGHT", "FULL":
				join.Set("side", word)
			case "INNER", "OUTER", "CROSS", "SEMI", "ANTI", "STRAIGHT_JOIN", "ARRAY":
				join.Set("kind", word)
			}
		}
	}

	if on := nonNil(opts.On); len(on) > 0 {
		join.Set("on", And_(copyAll(on, opts.Copy)...))
	}

	if using := nonNil(opts.Using); len(using) > 0 {
		join = applyListBuilder(join, "using", !opts.Replace, opts.Copy, using)
	}

	if opts.JoinAlias != "" {
		join.Set("this", AliasExpr(join.This(), opts.JoinAlias, true))
	}

	return applyListBuilder(n, "joins", !opts.Replace, opts.Copy, []Expression{join})
}

func nonNil(expressions []Expression) []Expression {
	out := make([]Expression, 0, len(expressions))
	for _, expression := range expressions {
		if expression != nil {
			out = append(out, expression)
		}
	}
	return out
}

// Unalias ports Expression.unalias (expressions/core.py:1224-1227).
func (n *Node) Unalias() Expression {
	if n.kind == KindAlias {
		return n.This()
	}
	return n
}

// Transform ports Expression.transform (expressions/core.py:1253-1270): it visits the tree
// depth-first, replacing each node with fun(node) and not descending into a replacement.
func (n *Node) Transform(fun func(Expression) Expression, copyValue bool) Expression {
	var root, newNode Expression
	start := maybeCopy(n, copyValue).(*Node)
	stack := []Expression{start}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		parent, argKey, index := node.Parent(), node.ArgKey(), node.Index()
		newNode = fun(node)
		if root == nil {
			root = newNode
		} else if parent != nil && argKey != "" && newNode != node {
			// parent.set(arg_key, new_node, index): a node outside a list has index -1 here
			// (None upstream), which is a plain set.
			if index < 0 {
				parent.Set(argKey, newNode)
			} else {
				parent.SetAt(argKey, newNode, index, true)
			}
		}
		// dfs(prune=lambda n: n is not new_node): children are only visited when fun kept
		// the node itself.
		if newNode == node {
			if nn, ok := node.(*Node); ok {
				stack = append(stack, nn.iterExpressions(true)...)
			}
		}
	}
	return root
}

// binop ports Expression._binop (expressions/core.py:1399-1409).
func (n *Node) binop(kind Kind, other any) Expression {
	this := n.Copy()
	otherExpr := Convert(other, true)
	if this.Kind() != kind && otherExpr.Kind() != kind {
		this = wrap(this, TraitBinary)
		otherExpr = wrap(otherExpr, TraitBinary)
	}
	return New(kind, Args{"this": this, "expression": otherExpr})
}

// Is_ ports Expression.is_ (expressions/core.py:1467-1474): `this IS other`.
func (n *Node) Is_(other any) Expression { return n.binop(KindIs, other) }

// Eq ports Expression.eq (expressions/core.py:1476-1483): `this = other`.
func (n *Node) Eq(other any) Expression { return n.binop(KindEQ, other) }

// Not_ ports Expression.not_ (expressions/core.py:1356-1366).
func (n *Node) Not_(copyValue bool) Expression { return Not_(n, copyValue) }

// SelectQuery ports the module-level select builder (expressions/builders.py:63-80) for
// expression projections: a new Select with the given projections.
func SelectQuery(expressions ...Expression) Expression {
	return Select(nil).(*Node).SelectExprs(true, false, expressions...)
}

// True_ ports exp.true (expressions/builders.py:1079-1088).
func True_() Expression { return Boolean(Args{"this": true}) }
