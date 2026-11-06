package optimizer

import (
	"sort"
	"strings"

	"github.com/maltzsama/sqlglot-go/dialects"
	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// Ports optimizer/simplify.py (sqlglot v30.17.0): boolean, arithmetic, string and date
// simplification. Constant folding follows upstream's Python value semantics (pynum.go,
// pydate.go), and operand ordering follows its gen() pseudo-SQL (simplify_gen.go).

// simplifyFinal ports FINAL (simplify.py:33): meta marking an expression not to simplify.
const simplifyFinal = "final"

func kindIfExists(name string) (exp.Kind, bool) { return exp.KindByClassName(name) }

// isSimplifiable ports SIMPLIFIABLE (simplify.py:35-41).
func isSimplifiable(e exp.Expression) bool {
	return e.Is(exp.TraitBinary) || e.Is(exp.TraitFunc) || e.Kind() == exp.KindLambda ||
		e.Is(exp.TraitPredicate) || e.Is(exp.TraitUnary)
}

// Simplify ports simplify (simplify.py:44-72).
//
//	TRUE AND TRUE  ->  TRUE
func Simplify(expression exp.Expression, constantPropagation, coalesceSimplification bool, dialect dialects.DialectType) exp.Expression {
	return NewSimplifier(dialect, true).Simplify(expression, constantPropagation, coalesceSimplification)
}

// Simplifier ports Simplifier (simplify.py:541-1590).
type Simplifier struct {
	dialect                *dialects.Dialect
	annotateNewExpressions bool
	annotator              *TypeAnnotator
}

// NewSimplifier ports Simplifier.__init__ (simplify.py:542-548).
func NewSimplifier(dialect dialects.DialectType, annotateNewExpressions bool) *Simplifier {
	d, err := dialects.GetOrRaise(dialect)
	if err != nil {
		panic(err)
	}
	annotator := NewTypeAnnotator(nil)
	annotator.dialect = d
	annotator.metadata = expressionMetadataFor(d.Name)
	annotator.coercesTo = coercesToFor(d.Name)
	annotator.overwriteTypes = false
	return &Simplifier{dialect: d, annotateNewExpressions: annotateNewExpressions, annotator: annotator}
}

// Byte-sized integer ranges (simplify.py:550-554).
const (
	tinyintMin  = -128
	tinyintMax  = 127
	utinyintMin = 0
	utinyintMax = 255
)

var complementComparisons = map[exp.Kind]exp.Kind{
	exp.KindLT: exp.KindGTE, exp.KindGT: exp.KindLTE, exp.KindLTE: exp.KindGT,
	exp.KindGTE: exp.KindLT, exp.KindEQ: exp.KindNEQ, exp.KindNEQ: exp.KindEQ,
}

var complementSubqueryPredicates = map[exp.Kind]exp.Kind{exp.KindAll: exp.KindAny, exp.KindAny: exp.KindAll}

var inverseComparisons = map[exp.Kind]exp.Kind{
	exp.KindLT: exp.KindGT, exp.KindGT: exp.KindLT, exp.KindLTE: exp.KindGTE, exp.KindGTE: exp.KindLTE,
}

func isLtLte(k exp.Kind) bool { return k == exp.KindLT || k == exp.KindLTE }
func isGtGte(k exp.Kind) bool { return k == exp.KindGT || k == exp.KindGTE }

// isComparison ports COMPARISONS (simplify.py:573-579).
func isComparison(k exp.Kind) bool {
	return isLtLte(k) || isGtGte(k) || k == exp.KindEQ || k == exp.KindNEQ || k == exp.KindIs
}

// inverseDateOps / inverseOps port INVERSE_DATE_OPS / INVERSE_OPS (simplify.py:596-607);
// DatetimeAdd and DatetimeSub join once they have kinds (#44).
var inverseDateOps = func() map[exp.Kind]exp.Kind {
	ops := map[exp.Kind]exp.Kind{exp.KindDateAdd: exp.KindSub, exp.KindDateSub: exp.KindAdd}
	if k, ok := kindIfExists("DatetimeAdd"); ok {
		ops[k] = exp.KindSub
	}
	if k, ok := kindIfExists("DatetimeSub"); ok {
		ops[k] = exp.KindAdd
	}
	return ops
}()

var inverseOps = func() map[exp.Kind]exp.Kind {
	ops := map[exp.Kind]exp.Kind{exp.KindAdd: exp.KindSub, exp.KindSub: exp.KindAdd}
	for k, v := range inverseDateOps {
		ops[k] = v
	}
	return ops
}()

func isNondeterministic(e exp.Expression) bool {
	if e.Kind() == exp.KindRand {
		return true
	}
	k, ok := kindIfExists("Randn")
	return ok && e.Kind() == k
}

func isConcat(e exp.Expression) bool { return e.Kind() == exp.KindConcat || e.Kind() == exp.KindDPipe }

func isDateTrunc(e exp.Expression) bool {
	return e.Kind() == exp.KindDateTrunc || e.Kind() == exp.KindTimestampTrunc
}

// simplifyJoins ports JOINS (simplify.py:629-639): the (side, kind) pairs a join with an
// always-true ON can become CROSS from.
var simplifyJoins = map[[2]string]bool{{"", ""}: true, {"", "INNER"}: true, {"RIGHT", ""}: true, {"RIGHT", "OUTER"}: true}

// onChange ports the annotate_types_on_change decorator (simplify.py:94-117): a new
// expression gets annotated and keeps the original's type, so the schema can't change.
func (s *Simplifier) onChange(expression, newExpression exp.Expression) exp.Expression {
	if newExpression == nil {
		return nil
	}
	if s.annotateNewExpressions && !expression.Equal(newExpression) {
		s.annotator.Clear()
		newExpression = s.annotator.Annotate(newExpression, false)
		newExpression.SetType(expression.Type())
	}
	return newExpression
}

// catchUnsupportedUnit ports @catch(ModuleNotFoundError, UnsupportedUnit): the rule returns the
// expression unchanged when it hits an unsupported unit.
func catchUnsupportedUnit(expression exp.Expression, rule func() exp.Expression) (out exp.Expression) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(errUnsupportedUnit); ok {
				out = expression
				return
			}
			panic(r)
		}
	}()
	return rule()
}

// Simplify ports Simplifier.simplify (simplify.py:642-710).
func (s *Simplifier) Simplify(expression exp.Expression, constantPropagation, coalesceSimplification bool) exp.Expression {
	var wheres, joins []exp.Expression

	// expression.walk(prune=...) is a lazy BFS: a node's children are queued only after the
	// loop body ran for it, so a FINAL marked during the walk prunes that node's subtree.
	queue := []exp.Expression{expression}
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]

		if node.MetaGet(simplifyFinal) != true {
			// GROUP BY expressions cannot be simplified, e.g. SELECT x + 1 + 1 FROM y GROUP BY
			// x + 1 + 1: the projection must exactly match the group key.
			if group := asExpression(node.Arg("group")); group != nil && node.Is(exp.TraitQuery) {
				groups := group.Expressions()
				group.Meta()[simplifyFinal] = true

				for _, sel := range node.Selects() {
					for _, n := range sel.Walk() {
						if indexOfExpr(groups, n) >= 0 {
							sel.Meta()[simplifyFinal] = true
							break
						}
					}
				}
				if having := asExpression(node.Arg("having")); having != nil {
					for _, n := range having.Walk() {
						if indexOfExpr(groups, n) >= 0 {
							having.Meta()[simplifyFinal] = true
							break
						}
					}
				}
			}

			switch {
			case node.Is(exp.TraitCondition):
				simplified := s.whileChanging(node, func(e exp.Expression) exp.Expression {
					return s.simplifyNode(e, constantPropagation, coalesceSimplification)
				})
				if node == expression {
					expression = simplified
				}
			case node.Kind() == exp.KindWhere:
				wheres = append(wheres, node)
			case node.Kind() == exp.KindJoin:
				// Snowflake match_conditions have very strict ordering rules.
				if match := asExpression(node.Arg("match_condition")); match != nil {
					match.Meta()[simplifyFinal] = true
				}
				joins = append(joins, node)
			}
		}

		if node.Is(exp.TraitCondition) || node.MetaGet(simplifyFinal) == true {
			continue
		}
		queue = append(queue, node.IterExpressions(false)...)
	}

	for _, where := range wheres {
		if alwaysTrue(where.This()) {
			where.Pop()
		}
	}
	for _, join := range joins {
		on := asExpression(join.Arg("on"))
		if alwaysTrue(on) && !truthyArg(join.Arg("using")) && !truthyArg(join.Arg("method")) &&
			simplifyJoins[[2]string{joinText(join, "side"), joinText(join, "kind")}] {
			on.Pop()
			join.Set("side", nil)
			join.Set("kind", "CROSS")
		}
	}
	return expression
}

// whileChanging ports helper.while_changing (helper.py:164-184): apply fn until the
// expression's hash stops changing.
func (s *Simplifier) whileChanging(expression exp.Expression, fn func(exp.Expression) exp.Expression) exp.Expression {
	for {
		start := expression.HashKey()
		expression = fn(expression)
		if expression.HashKey() == start {
			return expression
		}
	}
}

type postFrame struct {
	node, parent exp.Expression
}

// simplifyNode ports Simplifier._simplify (simplify.py:712-784).
func (s *Simplifier) simplifyNode(expression exp.Expression, constantPropagation, coalesceSimplification bool) exp.Expression {
	pre := []exp.Expression{expression}
	var post []postFrame

	for len(pre) > 0 {
		original := pre[len(pre)-1]
		pre = pre[:len(pre)-1]
		node := original

		if !isSimplifiable(node) {
			if node.Is(exp.TraitQuery) {
				s.Simplify(node, constantPropagation, coalesceSimplification)
			}
			continue
		}

		parent := node.Parent()
		root := node == expression

		node = s.rewriteBetween(node)
		node = s.uniqSort(node, root)
		node = s.absorbAndEliminate(node, root)
		node = s.simplifyConcat(node)
		node = s.simplifyConditionals(node)

		if constantPropagation {
			node = propagateConstants(node, root)
		}

		if node != original {
			original.Replace(node)
		}

		for _, n := range node.IterExpressions(true) {
			if n.MetaGet(simplifyFinal) != true {
				pre = append(pre, n)
			}
		}
		post = append(post, postFrame{node, parent})
	}

	node := expression
	for len(post) > 0 {
		frame := post[len(post)-1]
		post = post[:len(post)-1]
		original, parent := frame.node, frame.parent
		root := original == expression

		// Resets parent / arg_key / index pointers, since earlier transformations may have
		// left them inconsistent; the values themselves are unchanged.
		for _, key := range argOrder(original) {
			switch v := original.Arg(key).(type) {
			case nil:
				original.Set(key, nil)
			case exp.Expression:
				if v == nil {
					original.Set(key, nil)
				} else {
					v.SetParent(original, key, -1)
				}
			case []exp.Expression:
				for i, child := range v {
					if child != nil {
						child.SetParent(original, key, i)
					}
				}
			}
		}

		// Post-order transformations.
		node = s.simplifyNot(original)
		node = flattenConnector(node)
		node = s.simplifyConnectors(node, root)
		node = s.removeComplements(node, root)

		if coalesceSimplification {
			node = s.simplifyCoalesce(node)
		}
		// node.parent = parent, even when it is None (the root): simplify_parens then unwraps
		// a parenthesized root.
		node.SetParent(parent, node.ArgKey(), node.Index())

		node = s.simplifyLiterals(node, root)
		node = s.simplifyEquality(node)
		node = SimplifyParens(node, s.dialect)
		node = s.simplifyDatetrunc(node)
		node = s.sortComparison(node)
		node = s.simplifyStartswith(node)

		if node != original {
			original.Replace(node)
		}
	}
	return node
}

// flattenConnector ports flatten (simplify.py:120-130): A AND (B AND C) -> A AND B AND C.
func flattenConnector(expression exp.Expression) exp.Expression {
	if !expression.Is(exp.TraitConnector) {
		return expression
	}
	for _, key := range argOrder(expression) {
		node, ok := expression.Arg(key).(exp.Expression)
		if !ok || node == nil {
			continue
		}
		if child := node.Unnest(); child != nil && child.Kind() == expression.Kind() {
			node.Replace(child)
		}
	}
	return expression
}

// SimplifyParens ports simplify_parens (simplify.py:133-177).
func SimplifyParens(expression exp.Expression, dialect *dialects.Dialect) exp.Expression {
	if expression == nil || expression.Kind() != exp.KindParen {
		return expression
	}
	if dialect == nil {
		dialect = dialects.Base()
	}

	this := expression.This()
	parent := expression.Parent()
	parentIsPredicate := parent != nil && parent.Is(exp.TraitPredicate)

	if this != nil && this.Kind() == exp.KindSelect {
		return expression
	}
	if parent != nil && (isSubqueryPredicate(parent) || parent.Kind() == exp.KindBracket) {
		return expression
	}
	if dialect.RequiresParenthesizedStructAccess && parent != nil && parent.Kind() == exp.KindDot {
		if right := parent.Right(); right != nil && (right.Kind() == exp.KindIdentifier || right.Kind() == exp.KindStar) {
			return expression
		}
	}

	// Unary operators that bind tighter than the predicate, unlike NOT.
	tightUnary := parent != nil && (parent.Kind() == exp.KindNeg || isClass(parent, "BitwiseNot"))
	if this != nil && this.Is(exp.TraitPredicate) && !parentIsPredicate && !tightUnary &&
		(parent == nil || !parent.Is(exp.TraitBinary) || parent.Is(exp.TraitConnector)) {
		return this
	}

	if parent == nil || (!parent.Is(exp.TraitCondition) && !parent.Is(exp.TraitBinary)) || parent.Kind() == exp.KindParen ||
		(this != nil && !this.Is(exp.TraitBinary) && ((this.Kind() != exp.KindNot && this.Kind() != exp.KindIs) || !parentIsPredicate)) ||
		(this != nil && this.Kind() == exp.KindAdd && parent.Kind() == exp.KindAdd) ||
		(this != nil && this.Kind() == exp.KindMul && parent.Kind() == exp.KindMul) ||
		(this != nil && this.Kind() == exp.KindMul && (parent.Kind() == exp.KindAdd || parent.Kind() == exp.KindSub)) {
		return this
	}
	return expression
}

// isSubqueryPredicate ports isinstance(e, exp.SubqueryPredicate): All, Any and Exists
// (expressions/core.py:1611-1622, functions.py:187).
func isSubqueryPredicate(expression exp.Expression) bool {
	if expression == nil {
		return false
	}
	switch expression.Kind() {
	case exp.KindExists, exp.KindAny, exp.KindAll:
		return true
	}
	return false
}

// propagateConstants ports propagate_constants (simplify.py:180-216): in a DNF conjunction,
// a column equal to a literal is replaced by the literal elsewhere.
func propagateConstants(expression exp.Expression, root bool) exp.Expression {
	if expression.Kind() != exp.KindAnd || (!root && expression.SameParent()) || !normalized(expression, true) {
		return expression
	}
	type constant struct {
		column, value exp.Expression
	}
	var mapping []constant
	for _, expr := range walkInScope(expression, func(n exp.Expression) bool { return n.Kind() == exp.KindIf }) {
		if expr.Kind() != exp.KindEQ {
			continue
		}
		l, r := expr.Left(), expr.Right()
		if l != nil && r != nil && l.Kind() == exp.KindColumn && r.Kind() == exp.KindLiteral {
			replaced := false
			for i := range mapping {
				if mapping[i].column.Equal(l) {
					mapping[i] = constant{l, r}
					replaced = true
				}
			}
			if !replaced {
				mapping = append(mapping, constant{l, r})
			}
		}
	}
	if len(mapping) == 0 {
		return expression
	}
	for _, col := range findAllInScope(expression, exp.KindColumn) {
		parent := col.Parent()
		for _, c := range mapping {
			if !c.column.Equal(col) {
				continue
			}
			if col != c.column && !(parent != nil && parent.Kind() == exp.KindIs && parent.Expr() != nil && parent.Expr().Kind() == exp.KindNull) {
				col.Replace(c.value.Copy())
			}
			break
		}
	}
	return expression
}

// pyNumOf is expression.to_py() for a number literal (a Neg negates).
func pyNumOf(e exp.Expression) (pyNum, bool) {
	if e == nil || !e.IsNumber() {
		return pyNum{}, false
	}
	if e.Kind() == exp.KindNeg {
		n, ok := pyNumOf(e.This())
		if !ok {
			return pyNum{}, false
		}
		return n.negate(), true
	}
	return parsePyNum(e.Text("this"))
}

func isPyInt(e exp.Expression) bool {
	n, ok := pyNumOf(e)
	return ok && n.isInt
}

// literalNumber ports exp.Literal.number: a negative value becomes Neg(Literal(abs)).
func literalNumber(n pyNum) exp.Expression {
	if n.isNegative() {
		return exp.Neg(exp.Args{"this": exp.Literal(exp.Args{"this": n.abs().String(), "is_string": false})})
	}
	return exp.Literal(exp.Args{"this": n.String(), "is_string": false})
}

func literalString(s string) exp.Expression {
	return exp.Literal(exp.Args{"this": s, "is_string": true})
}

// isSimplifyConstant ports _is_constant (simplify.py:231-233).
func isSimplifyConstant(e exp.Expression) bool {
	if e == nil {
		return false
	}
	if e.Kind() == exp.KindNeg {
		e = e.This()
		if e == nil {
			return false
		}
	}
	switch e.Kind() {
	case exp.KindLiteral, exp.KindBoolean, exp.KindNull:
		return true
	}
	return isDateLiteral(e)
}

// isNonnullConstant ports _is_nonnull_constant (simplify.py:227-228).
func isNonnullConstant(e exp.Expression) bool {
	return e != nil && (e.Kind() == exp.KindLiteral || e.Kind() == exp.KindBoolean || isDateLiteral(e))
}

// alwaysTrue ports always_true (simplify.py:312-315).
func alwaysTrue(e exp.Expression) bool {
	if e == nil {
		return false
	}
	if e.Kind() == exp.KindBoolean {
		return truthyArg(e.Arg("this"))
	}
	return e.Kind() == exp.KindLiteral && e.IsNumber() && !isZero(e)
}

// alwaysFalse ports always_false (simplify.py:318-319).
func alwaysFalse(e exp.Expression) bool { return isFalse(e) || isNull(e) || isZero(e) }

// isZero ports is_zero (simplify.py:322-323).
func isZero(e exp.Expression) bool {
	if e == nil || e.Kind() != exp.KindLiteral {
		return false
	}
	if n, ok := pyNumOf(e); ok {
		return n.isZero()
	}
	return false
}

func isFalse(e exp.Expression) bool {
	return e != nil && e.Kind() == exp.KindBoolean && !truthyArg(e.Arg("this"))
}

func isNull(e exp.Expression) bool { return e != nil && e.Kind() == exp.KindNull }

func booleanLiteral(b bool) exp.Expression { return exp.Boolean(exp.Args{"this": b}) }

func nullLiteral() exp.Expression { return exp.Null() }

// evalBoolean ports eval_boolean (simplify.py:347-364) for a comparison result c (<0, 0, >0)
// of a and b.
func evalBoolean(expression exp.Expression, c int) exp.Expression {
	switch expression.Kind() {
	case exp.KindEQ:
		return booleanLiteral(c == 0)
	case exp.KindIs:
		if truthyArg(expression.Arg("negate")) {
			return booleanLiteral(c != 0)
		}
		return booleanLiteral(c == 0)
	case exp.KindNEQ:
		return booleanLiteral(c != 0)
	case exp.KindGT:
		return booleanLiteral(c > 0)
	case exp.KindGTE:
		return booleanLiteral(c >= 0)
	case exp.KindLT:
		return booleanLiteral(c < 0)
	case exp.KindLTE:
		return booleanLiteral(c <= 0)
	}
	return nil
}

// evalDates compares two Python dates the way eval_boolean does: a date and a datetime are never
// equal, and ordering them (or an aware and a naive datetime) raises in Python, which skips the
// simplification here.
func evalDates(expression exp.Expression, a, b pyDate) exp.Expression {
	if a.isDatetime != b.isDatetime {
		switch expression.Kind() {
		case exp.KindEQ:
			return booleanLiteral(false)
		case exp.KindNEQ:
			return booleanLiteral(true)
		case exp.KindIs:
			return booleanLiteral(truthyArg(expression.Arg("negate")))
		}
		return nil
	}
	if !a.comparable(b) {
		return nil
	}
	return evalBoolean(expression, a.cmp(b))
}

// castAsDate / castAsDatetime / castValue port simplify.py:367-398 for a string value.
func castAsDate(value string) (pyDate, bool) {
	d, ok := pyFromISODatetime(value)
	if !ok {
		return pyDate{}, false
	}
	return d.asDate(), true
}

func castValue(value any, to exp.Expression) (pyDate, bool) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return pyDate{}, false
		}
		if to.IsType(exp.DTypeDate) {
			return castAsDate(v)
		}
		if to.IsType(dtypeSet(exp.TemporalTypes)...) {
			return pyFromISODatetime(v)
		}
	case pyDate:
		if to.IsType(exp.DTypeDate) {
			return v.asDate(), true
		}
		if to.IsType(dtypeSet(exp.TemporalTypes)...) {
			return v.asDatetime(), true
		}
	}
	return pyDate{}, false
}

// extractDate ports extract_date (simplify.py:401-415).
func extractDate(cast exp.Expression) (pyDate, bool) {
	if cast == nil {
		return pyDate{}, false
	}
	var to exp.Expression
	switch {
	case cast.Kind() == exp.KindCast:
		to = asExpression(cast.Arg("to"))
	case cast.Kind() == exp.KindTsOrDsToDate && cast.Arg("format") == nil:
		to = exp.DataType(exp.Args{"this": exp.DTypeDate})
	default:
		return pyDate{}, false
	}
	if to == nil {
		return pyDate{}, false
	}
	this := cast.This()
	if this == nil {
		return pyDate{}, false
	}
	var value any
	switch this.Kind() {
	case exp.KindLiteral:
		value = this.Name()
	case exp.KindCast, exp.KindTsOrDsToDate:
		d, ok := extractDate(this)
		if !ok {
			return pyDate{}, false
		}
		value = d
	default:
		return pyDate{}, false
	}
	return castValue(value, to)
}

func isDateLiteral(e exp.Expression) bool {
	_, ok := extractDate(e)
	return ok
}

// extractInterval ports extract_interval (simplify.py:422-428).
func extractInterval(e exp.Expression) (relativedelta, bool) {
	this := e.This()
	if this == nil {
		return relativedelta{}, false
	}
	var n pyNum
	var ok bool
	if this.IsNumber() {
		n, ok = pyNumOf(this)
	} else {
		n, ok = parsePyInt(strings.TrimSpace(this.Text("this")))
	}
	if !ok || !n.isInt || !n.coef.IsInt64() {
		return relativedelta{}, false
	}
	count := int(n.coef.Int64())
	if n.neg {
		count = -count
	}
	var rd relativedelta
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, isUnit := r.(errUnsupportedUnit); isUnit {
					ok = false
					return
				}
				panic(r)
			}
		}()
		rd = interval(strings.ToLower(e.Text("unit")), count)
	}()
	return rd, ok
}

func isInterval(e exp.Expression) bool {
	if e == nil || e.Kind() != exp.KindInterval {
		return false
	}
	_, ok := extractInterval(e)
	return ok
}

// extractType ports extract_type (simplify.py:431-438).
func extractType(expressions ...exp.Expression) exp.Expression {
	for _, e := range expressions {
		var t exp.Expression
		if e.Kind() == exp.KindCast {
			t = asExpression(e.Arg("to"))
		} else {
			t = e.Type()
		}
		if t != nil {
			return t
		}
	}
	return nil
}

// buildCast ports exp.cast(expression, to): Cast(this, to) annotated with the target type.
func buildCast(expression exp.Expression, to any) exp.Expression {
	var dataType exp.Expression
	switch v := to.(type) {
	case exp.DType:
		dataType = exp.DataType(exp.Args{"this": v})
	case exp.Expression:
		dataType = v.Copy()
	}
	if expression.IsType(dataType) {
		return expression
	}
	cast := exp.Cast(exp.Args{"this": expression, "to": dataType})
	cast.SetType(dataType)
	return cast
}

// dateLiteral ports date_literal (simplify.py:441-445).
func dateLiteral(d pyDate, targetType exp.Expression) exp.Expression {
	var to any = targetType
	if targetType == nil || !targetType.IsType(dtypeSet(exp.TemporalTypes)...) {
		if d.isDatetime {
			to = exp.DTypeDatetime
		} else {
			to = exp.DTypeDate
		}
	}
	return buildCast(literalString(d.String()), to)
}

// truncUnit ports _trunc_unit (simplify.py:517-525); WeekStart joins once it has a kind.
func truncUnit(unit exp.Expression) string {
	if isClass(unit, "WeekStart") {
		panic(errUnsupportedUnit{unit.Name()})
	}
	return strings.ToLower(unit.Name())
}

// binop ports Expression._binop for `l <op> r` (core.py:1399-1409): both operands copied, and
// each wrapped in parentheses when it is a binary of another class.
func binop(kind exp.Kind, l, r exp.Expression) exp.Expression {
	this, other := l.Copy(), r.Copy()
	if this.Kind() != kind && other.Kind() != kind {
		if this.Is(exp.TraitBinary) {
			this = exp.Paren(exp.Args{"this": this})
		}
		if other.Is(exp.TraitBinary) {
			other = exp.Paren(exp.Args{"this": other})
		}
	}
	return exp.New(kind, exp.Args{"this": this, "expression": other})
}

// parenthesizeNestedConnector ports _parenthesize_nested_connector (simplify.py:297-309).
func parenthesizeNestedConnector(expression, parent exp.Expression) exp.Expression {
	if expression.Is(exp.TraitConnector) && parent != nil &&
		(parent.Kind() == exp.KindNot || (parent.Is(exp.TraitConnector) && parent.Kind() != expression.Kind())) {
		return exp.Paren(exp.Args{"this": expression})
	}
	return expression
}

// combine ports exp._combine with wrap=True for kind (And, Or or Xor).
func combine(kind exp.Kind, expressions ...exp.Expression) exp.Expression {
	var filtered []exp.Expression
	for _, e := range expressions {
		if e != nil {
			filtered = append(filtered, e)
		}
	}
	if len(filtered) == 0 {
		return nil
	}
	wrapConnector := func(e exp.Expression) exp.Expression {
		if e.Is(exp.TraitConnector) {
			return exp.Paren(exp.Args{"this": e})
		}
		return e
	}
	this := filtered[0]
	if len(filtered) > 1 {
		this = wrapConnector(this)
	}
	for _, e := range filtered[1:] {
		this = exp.New(kind, exp.Args{"this": this, "expression": wrapConnector(e)})
	}
	return this
}

// rewriteBetween ports rewrite_between (simplify.py:786-803): x BETWEEN y AND z becomes
// x >= y AND x <= z, since comparison simplification only handles lt/lte/gt/gte.
func (s *Simplifier) rewriteBetween(expression exp.Expression) exp.Expression {
	out := expression
	if expression.Kind() == exp.KindBetween {
		negate := expression.Parent() != nil && expression.Parent().Kind() == exp.KindNot
		out = combine(exp.KindAnd,
			exp.New(exp.KindGTE, exp.Args{"this": expression.This().Copy(), "expression": asExpression(expression.Arg("low"))}),
			exp.New(exp.KindLTE, exp.Args{"this": expression.This().Copy(), "expression": asExpression(expression.Arg("high"))}),
		)
		if negate {
			out = exp.Paren(exp.Args{"this": out})
		}
	}
	return s.onChange(expression, out)
}

// simplifyNot ports simplify_not (simplify.py:806-861): De Morgan's laws and double negation.
func (s *Simplifier) simplifyNot(expression exp.Expression) exp.Expression {
	return s.onChange(expression, s.simplifyNotInner(expression))
}

func (s *Simplifier) simplifyNotInner(expression exp.Expression) exp.Expression {
	if expression.Kind() != exp.KindNot {
		return expression
	}
	this := expression.This()
	if this == nil {
		return expression
	}
	if isNull(this) {
		return parenthesizeNestedConnector(combine(exp.KindAnd, nullLiteral(), booleanLiteral(true)), expression.Parent())
	}
	if complement, ok := complementComparisons[this.Kind()]; ok {
		right := this.Expr()
		if right != nil {
			if sub, ok := complementSubqueryPredicates[right.Kind()]; ok {
				right = exp.New(sub, exp.Args{"this": right.This()})
			}
		}
		return exp.New(complement, exp.Args{"this": this.This(), "expression": right})
	}
	if this.Kind() == exp.KindParen {
		condition := this.Unnest()
		switch {
		case condition.Kind() == exp.KindAnd:
			return exp.Paren(exp.Args{"this": combine(exp.KindOr,
				exp.Not_(condition.Left(), false), exp.Not_(condition.Right(), false))})
		case condition.Kind() == exp.KindOr:
			return exp.Paren(exp.Args{"this": combine(exp.KindAnd,
				exp.Not_(condition.Left(), false), exp.Not_(condition.Right(), false))})
		case isNull(condition):
			return parenthesizeNestedConnector(combine(exp.KindAnd, nullLiteral(), booleanLiteral(true)), expression.Parent())
		}
	}
	if alwaysTrue(this) {
		return booleanLiteral(false)
	}
	if isFalse(this) {
		return booleanLiteral(true)
	}
	if this.Kind() == exp.KindNot && s.dialect.SafeToEliminateDoubleNegation {
		// NOT NOT x -> x, if x is BOOLEAN.
		if inner := this.This(); inner != nil && inner.IsType(exp.DTypeBoolean) {
			return inner
		}
	}
	return expression
}

// simplifyConnectors ports simplify_connectors (simplify.py:863-921).
func (s *Simplifier) simplifyConnectors(expression exp.Expression, root bool) exp.Expression {
	if !expression.Is(exp.TraitConnector) {
		return s.onChange(expression, expression)
	}
	pairwise := func(e, left, right exp.Expression) exp.Expression {
		switch e.Kind() {
		case exp.KindAnd:
			if isFalse(left) || isFalse(right) || isZero(left) || isZero(right) {
				return booleanLiteral(false)
			}
			if (isNull(left) && isNull(right)) || (isNull(left) && alwaysTrue(right)) || (alwaysTrue(left) && isNull(right)) {
				return nullLiteral()
			}
			if alwaysTrue(left) && alwaysTrue(right) {
				return booleanLiteral(true)
			}
			if alwaysTrue(left) {
				return right
			}
			if alwaysTrue(right) {
				return left
			}
			return s.simplifyComparison(e, left, right, false)
		case exp.KindOr:
			if alwaysTrue(left) || alwaysTrue(right) {
				return booleanLiteral(true)
			}
			if (isNull(left) && isNull(right)) || (isNull(left) && alwaysFalse(right)) || (alwaysFalse(left) && isNull(right)) {
				return nullLiteral()
			}
			if isFalse(left) {
				return right
			}
			if isFalse(right) {
				return left
			}
			return s.simplifyComparison(e, left, right, true)
		}
		return nil
	}

	originalParent := expression.Parent()
	out := s.flatSimplify(expression, pairwise, root)

	// If a connector reduced to, e.g., a column (t1 AND ... AND tn -> tk), make sure the result
	// stays boolean: only connectors, booleans, and operands of a further connector are safe.
	//
	//   A AND (((B)))  -- safe to keep, it will be part of another connector
	if !out.Is(exp.TraitConnector) && out.Kind() != exp.KindBoolean && !out.IsType(exp.DTypeBoolean) {
		for originalParent == nil || !originalParent.Is(exp.TraitConnector) {
			if originalParent == nil || originalParent.Kind() != exp.KindParen {
				out = combine(exp.KindAnd, out, booleanLiteral(true))
				break
			}
			originalParent = originalParent.Parent()
		}
	}
	return s.onChange(expression, out)
}

// comparisonValue is one side of a comparison folded to a Python value.
type comparisonValue struct {
	num  pyNum
	str  string
	date pyDate
	kind int // 0 number, 1 string, 2 date
}

func (v comparisonValue) cmp(o comparisonValue) (int, bool) {
	switch v.kind {
	case 0:
		return v.num.cmp(o.num), true
	case 1:
		return strings.Compare(v.str, o.str), true
	default:
		if !v.date.comparable(o.date) {
			return 0, false
		}
		return v.date.cmp(o.date), true
	}
}

// simplifyComparison ports _simplify_comparison (simplify.py:923-990), including its
// annotate_types_on_change wrapper; it returns nil when nothing combines.
func (s *Simplifier) simplifyComparison(expression, left, right exp.Expression, or bool) exp.Expression {
	result := s.simplifyComparisonInner(expression, left, right, or)
	if result == nil {
		return nil
	}
	return s.onChange(expression, result)
}

func (s *Simplifier) simplifyComparisonInner(expression, left, right exp.Expression, or bool) exp.Expression {
	if !isComparison(left.Kind()) || !isComparison(right.Kind()) {
		return nil
	}
	for _, e := range []exp.Expression{left, right} {
		if e.Kind() == exp.KindIs && truthyArg(e.Arg("negate")) {
			return nil
		}
	}

	largs := uniqueExprs(left.This(), left.Expr())
	rargs := uniqueExprs(right.This(), right.Expr())

	var matching, columns []exp.Expression
	for _, m := range largs {
		if indexOfExpr(rargs, m) >= 0 {
			matching = append(matching, m)
			if !isSimplifyConstant(m) && !hasNondeterministic(m) {
				columns = append(columns, m)
			}
		}
	}
	if len(matching) == 0 || len(columns) == 0 {
		return nil
	}

	l := firstNotIn(largs, columns)
	r := firstNotIn(rargs, columns)
	if l == nil || r == nil {
		return expression
	}

	var lv, rv comparisonValue
	switch {
	case l.IsNumber() && r.IsNumber():
		ln, ok1 := pyNumOf(l)
		rn, ok2 := pyNumOf(r)
		if !ok1 || !ok2 {
			return nil
		}
		lv, rv = comparisonValue{num: ln}, comparisonValue{num: rn}
	case l.IsString() && r.IsString():
		lv, rv = comparisonValue{str: l.Name(), kind: 1}, comparisonValue{str: r.Name(), kind: 1}
	default:
		ld, ok := extractDate(l)
		if !ok {
			return nil
		}
		rd, ok := extractDate(r)
		if !ok {
			return nil
		}
		// Python won't compare date and datetime, but many engines will upcast.
		lv, rv = comparisonValue{date: ld.asDatetime(), kind: 2}, comparisonValue{date: rd.asDatetime(), kind: 2}
	}

	type side struct {
		e exp.Expression
		v comparisonValue
	}
	for _, p := range [][2]side{{{left, lv}, {right, rv}}, {{right, rv}, {left, lv}}} {
		a, b := p[0], p[1]
		c, ok := a.v.cmp(b.v)
		if !ok {
			return nil
		}
		ak, bk := a.e.Kind(), b.e.Kind()
		if isLtLte(ak) && isLtLte(bk) {
			if (or && c > 0) || (!or && c <= 0) {
				return left
			}
			return right
		}
		if isGtGte(ak) && isGtGte(bk) {
			if (or && c < 0) || (!or && c >= 0) {
				return left
			}
			return right
		}
		// We can't ever shortcut to true because the column could be null.
		if !or {
			switch {
			case ak == exp.KindLT && isGtGte(bk):
				if c <= 0 {
					return booleanLiteral(false)
				}
			case ak == exp.KindGT && isLtLte(bk):
				if c >= 0 {
					return booleanLiteral(false)
				}
			case ak == exp.KindEQ:
				switch bk {
				case exp.KindLT:
					if c >= 0 {
						return booleanLiteral(false)
					}
					return a.e
				case exp.KindLTE:
					if c > 0 {
						return booleanLiteral(false)
					}
					return a.e
				case exp.KindGT:
					if c <= 0 {
						return booleanLiteral(false)
					}
					return a.e
				case exp.KindGTE:
					if c < 0 {
						return booleanLiteral(false)
					}
					return a.e
				case exp.KindNEQ:
					if c == 0 {
						return booleanLiteral(false)
					}
					return a.e
				}
			}
		}
	}
	return nil
}

func uniqueExprs(exprs ...exp.Expression) []exp.Expression {
	var out []exp.Expression
	for _, e := range exprs {
		if e != nil && indexOfExpr(out, e) < 0 {
			out = append(out, e)
		}
	}
	return out
}

func firstNotIn(exprs, exclude []exp.Expression) exp.Expression {
	for _, e := range exprs {
		if indexOfExpr(exclude, e) < 0 {
			return e
		}
	}
	return nil
}

func hasNondeterministic(e exp.Expression) bool {
	for _, n := range e.Walk() {
		if isNondeterministic(n) {
			return true
		}
	}
	return false
}

// removeComplements ports remove_complements (simplify.py:992-1007): A AND NOT A -> FALSE and
// A OR NOT A -> TRUE, for non-NULL A.
func (s *Simplifier) removeComplements(expression exp.Expression, root bool) exp.Expression {
	out := expression
	if (expression.Kind() == exp.KindAnd || expression.Kind() == exp.KindOr) && (root || !expression.SameParent()) {
		ops := flatten(expression, true)
		for _, op := range ops {
			if op.Kind() == exp.KindNot && indexOfExpr(ops, op.This()) >= 0 && expression.MetaGet("nonnull") == true {
				if expression.Kind() == exp.KindAnd {
					out = booleanLiteral(false)
				} else {
					out = booleanLiteral(true)
				}
				break
			}
		}
	}
	return s.onChange(expression, out)
}

// uniqSort ports uniq_sort (simplify.py:1009-1044): dedupe and sort a connector's operands,
// C AND A AND B AND B -> A AND B AND C.
func (s *Simplifier) uniqSort(expression exp.Expression, root bool) exp.Expression {
	out := expression
	if expression.Is(exp.TraitConnector) && (root || !expression.SameParent()) {
		flattened := flatten(expression, true)

		type entry struct {
			sql string
			e   exp.Expression
		}
		var arr []entry
		var dedupedKeys []string
		deduped := map[string]exp.Expression{}
		isXor := expression.Kind() == exp.KindXor
		resultKind := exp.KindOr
		if expression.Kind() == exp.KindAnd {
			resultKind = exp.KindAnd
		}
		if isXor {
			resultKind = exp.KindXor
			// Don't dedupe XOR, as A XOR A != A when A is TRUE.
			for _, e := range flattened {
				arr = append(arr, entry{simplifyGen(e, false), e})
			}
		} else {
			for _, e := range flattened {
				key := simplifyGen(e, false)
				if _, ok := deduped[key]; !ok {
					dedupedKeys = append(dedupedKeys, key)
				}
				deduped[key] = e
			}
			for _, key := range dedupedKeys {
				arr = append(arr, entry{key, deduped[key]})
			}
		}

		sorted := true
		for i := 1; i < len(arr); i++ {
			if arr[i].sql < arr[i-1].sql {
				sorted = false
				break
			}
		}
		if !sorted {
			ordered := append([]entry(nil), arr...)
			sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].sql < ordered[j].sql })
			var operands []exp.Expression
			for _, e := range ordered {
				operands = append(operands, e.e)
			}
			out = combine(resultKind, operands...)
		} else if !isXor && len(deduped) > 0 && len(deduped) < len(flattened) {
			// We didn't have to sort, but maybe we need to dedupe.
			if len(deduped) == 1 {
				out = combine(exp.KindAnd, flattened[0], booleanLiteral(true))
			} else {
				var operands []exp.Expression
				for _, key := range dedupedKeys {
					operands = append(operands, deduped[key])
				}
				out = combine(resultKind, operands...)
			}
		}
	}
	return s.onChange(expression, out)
}

// exprSetOf is Python's set(...) of expressions.
func exprSetOf(exprs []exp.Expression) []exp.Expression { return uniqueExprs(exprs...) }

// isProperSubset reports a < b for two expression sets.
func isProperSubset(a, b []exp.Expression) bool {
	if len(a) >= len(b) {
		return false
	}
	for _, e := range a {
		if indexOfExpr(b, e) < 0 {
			return false
		}
	}
	return true
}

// absorbAndEliminate ports absorb_and_eliminate (simplify.py:1046-1126).
//
//	absorption:  A AND (A OR B) -> A;  A OR (A AND B) -> A
//	             A AND (NOT A OR B) -> A AND B;  A OR (NOT A AND B) -> A OR B  (non-NULL A)
//	elimination: (A AND B) OR (A AND NOT B) -> A;  (A OR B) AND (A OR NOT B) -> A  (non-NULL B)
func (s *Simplifier) absorbAndEliminate(expression exp.Expression, root bool) exp.Expression {
	if (expression.Kind() != exp.KindAnd && expression.Kind() != exp.KindOr) || !(root || !expression.SameParent()) {
		return s.onChange(expression, expression)
	}
	kind := exp.KindAnd
	if expression.Kind() == exp.KindAnd {
		kind = exp.KindOr
	}
	ops := flatten(expression, true)

	// Lookup tables: all operands, sub-operand sets for absorption, complement pairs for
	// elimination (keyed by the unordered pair, as a frozenset).
	opSet := exprSetOf(ops)
	type subopsEntry struct {
		key  exp.Expression
		sets [][]exp.Expression
	}
	var subops []*subopsEntry
	subopsFor := func(e exp.Expression) *subopsEntry {
		for _, entry := range subops {
			if entry.key.Equal(e) {
				return entry
			}
		}
		entry := &subopsEntry{key: e}
		subops = append(subops, entry)
		return entry
	}
	type pairEntry struct {
		key   []exp.Expression
		items [][2]exp.Expression
	}
	var pairs []*pairEntry
	sameSet := func(a, b []exp.Expression) bool {
		return len(a) == len(b) && !isProperSubset(a, b) && func() bool {
			for _, e := range a {
				if indexOfExpr(b, e) < 0 {
					return false
				}
			}
			return true
		}()
	}
	pairFor := func(a, b exp.Expression, create bool) *pairEntry {
		key := uniqueExprs(a, b)
		for _, entry := range pairs {
			if sameSet(entry.key, key) {
				return entry
			}
		}
		if !create {
			return nil
		}
		entry := &pairEntry{key: key}
		pairs = append(pairs, entry)
		return entry
	}

	for _, op := range ops {
		if op.Kind() != kind {
			// In cases like A OR (A AND B), the subop is A.
			entry := subopsFor(op)
			entry.sets = append(entry.sets, []exp.Expression{op})
			continue
		}
		// In cases like (A AND B) OR (A AND B AND C), the subops are A and B.
		subset := exprSetOf(flatten(op, true))
		for _, i := range subset {
			entry := subopsFor(i)
			entry.sets = append(entry.sets, subset)
		}

		a, b := op.Left().Unnest(), op.Right().Unnest()
		if a.Kind() == exp.KindNot && a.This().MetaGet("nonnull") == true {
			p := pairFor(a.This(), b, true)
			p.items = append(p.items, [2]exp.Expression{op, b})
		}
		if b.Kind() == exp.KindNot && b.This().MetaGet("nonnull") == true {
			p := pairFor(a, b.This(), true)
			p.items = append(p.items, [2]exp.Expression{op, a})
		}
	}

	absorbed := func() exp.Expression {
		if kind == exp.KindAnd {
			return booleanLiteral(true)
		}
		return booleanLiteral(false)
	}
	eliminated := func() exp.Expression {
		if kind == exp.KindAnd {
			return booleanLiteral(false)
		}
		return booleanLiteral(true)
	}

	for _, op := range ops {
		if op.Kind() != kind {
			continue
		}
		a, b := op.Left().Unnest(), op.Right().Unnest()

		// Absorb.
		if a.Kind() == exp.KindNot && indexOfExpr(opSet, a.This()) >= 0 && a.This().MetaGet("nonnull") == true {
			a.Replace(absorbed())
			continue
		}
		if b.Kind() == exp.KindNot && indexOfExpr(opSet, b.This()) >= 0 && b.This().MetaGet("nonnull") == true {
			b.Replace(absorbed())
			continue
		}
		superset := exprSetOf(flatten(op, true))
		found := false
		for _, i := range superset {
			for _, set := range subopsFor(i).sets {
				if isProperSubset(set, superset) {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if found {
			op.Replace(eliminated())
			continue
		}

		// Eliminate.
		if p := pairFor(a, b, false); p != nil {
			for _, item := range p.items {
				op.Replace(item[1])
				item[0].Replace(item[1])
			}
		}
	}
	return s.onChange(expression, expression)
}

// simplifyEquality ports simplify_equality (simplify.py:1127-1185): the subtraction and
// addition properties of equality, x + 1 = 3 -> x = 2; with the variable as the subtrahend the
// comparison is inverted, 5 - x < 2 -> x > 3.
func (s *Simplifier) simplifyEquality(expression exp.Expression) exp.Expression {
	out := catchUnsupportedUnit(expression, func() exp.Expression { return s.simplifyEqualityInner(expression) })
	return s.onChange(expression, out)
}

// intervalOf ports IntervalOp.interval (core.py:2083-2090).
func intervalOf(e exp.Expression) exp.Expression {
	args := exp.Args{}
	if expr := e.Expr(); expr != nil {
		args["this"] = expr.Copy()
	}
	if unit := asExpression(e.Arg("unit")); unit != nil {
		args["unit"] = unit.Copy()
	}
	return exp.Interval(args)
}

func (s *Simplifier) simplifyEqualityInner(expression exp.Expression) exp.Expression {
	if !isComparison(expression.Kind()) {
		return expression
	}
	l, r := expression.Left(), expression.Right()
	if l == nil || r == nil {
		return expression
	}
	inverse, ok := inverseOps[l.Kind()]
	if !ok {
		return expression
	}

	var aPredicate, bPredicate func(exp.Expression) bool
	switch {
	case r.IsNumber():
		aPredicate, bPredicate = isNumberExpr, isNumberExpr
	case isDateLiteral(r):
		aPredicate, bPredicate = isDateLiteral, isInterval
	default:
		return expression
	}

	var a, b exp.Expression
	if _, isDateOp := inverseDateOps[l.Kind()]; isDateOp {
		a, b = l.This(), intervalOf(l)
	} else {
		a, b = l.Left(), l.Right()
	}
	if a == nil || b == nil {
		return expression
	}

	switch {
	case !aPredicate(a) && bPredicate(b):
	case !aPredicate(b) && bPredicate(a):
		if l.Kind() == exp.KindSub {
			k := expression.Kind()
			if inv, ok := inverseComparisons[k]; ok {
				k = inv
			}
			return exp.New(k, exp.Args{"this": b, "expression": exp.New(exp.KindSub, exp.Args{"this": a, "expression": r})})
		}
		a, b = b, a
	default:
		return expression
	}
	return exp.New(expression.Kind(), exp.Args{"this": a, "expression": exp.New(inverse, exp.Args{"this": r, "expression": b})})
}

func isNumberExpr(e exp.Expression) bool { return e.IsNumber() }

// simplifyLiterals ports simplify_literals (simplify.py:1190-1203).
func (s *Simplifier) simplifyLiterals(expression exp.Expression, root bool) exp.Expression {
	out := expression
	switch {
	case expression.Is(exp.TraitBinary) && !expression.Is(exp.TraitConnector):
		out = s.flatSimplify(expression, s.simplifyBinary, root)
	case expression.Kind() == exp.KindNeg && expression.This() != nil && expression.This().Kind() == exp.KindNeg:
		out = expression.This().This()
	default:
		if _, isDateOp := inverseDateOps[expression.Kind()]; isDateOp {
			if folded := s.simplifyBinary(expression, expression.This(), intervalOf(expression)); folded != nil {
				out = folded
			}
		}
	}
	return s.onChange(expression, out)
}

// simplifyIntegerCast ports _simplify_integer_cast (simplify.py:1205-1226): an upcast of a
// byte-sized integer in a predicate is side-effect free and can go.
func (s *Simplifier) simplifyIntegerCast(expr exp.Expression) exp.Expression {
	if expr == nil || expr.Kind() != exp.KindCast {
		return expr
	}
	this := expr.This()
	if this != nil && this.Kind() == exp.KindCast {
		this = s.simplifyIntegerCast(this)
	}
	if this == nil || !isPyInt(this) {
		return expr
	}
	n, _ := pyNumOf(this)
	if !n.coef.IsInt64() {
		return expr
	}
	num := n.coef.Int64()
	if n.neg {
		num = -num
	}
	to := asExpression(expr.Arg("to"))
	if to == nil {
		return expr
	}
	toType := typeThis(to)
	if (num >= tinyintMin && num <= tinyintMax && exp.SignedIntegerTypes[toType]) ||
		(num >= utinyintMin && num <= utinyintMax && exp.UnsignedIntegerTypes[toType]) {
		return this
	}
	return expr
}

// simplifyBinary ports _simplify_binary (simplify.py:1228-1301); nil means no simplification.
func (s *Simplifier) simplifyBinary(expression, a, b exp.Expression) exp.Expression {
	if isComparison(expression.Kind()) {
		a = s.simplifyIntegerCast(a)
		b = s.simplifyIntegerCast(b)
	}

	switch {
	case expression.Kind() == exp.KindIs:
		c, not := b, false
		if b.Kind() == exp.KindNot {
			c, not = b.This(), true
		}
		if truthyArg(expression.Arg("negate")) {
			not = !not
		}
		if isNull(c) {
			if a.Kind() == exp.KindLiteral {
				return booleanLiteral(not)
			}
			if isNull(a) {
				return booleanLiteral(!not)
			}
		}
	case expression.Kind() == exp.KindNullSafeEQ || expression.Kind() == exp.KindNullSafeNEQ || expression.Kind() == exp.KindPropertyEQ:
		return nil
	case (isNull(a) || isNull(b)) && expression.Parent() != nil && expression.Parent().Kind() == exp.KindIf:
		return nullLiteral()
	}

	switch {
	case a.IsNumber() && b.IsNumber():
		numA, ok1 := pyNumOf(a)
		numB, ok2 := pyNumOf(b)
		if !ok1 || !ok2 {
			return nil
		}
		switch expression.Kind() {
		case exp.KindAdd:
			return literalNumber(numA.add(numB))
		case exp.KindMul:
			return literalNumber(numA.mul(numB))
		case exp.KindSub:
			// Sub and Div only fold operands of the same parent, since they aren't associative.
			if a.Parent() == b.Parent() {
				return literalNumber(numA.sub(numB))
			}
			return nil
		case exp.KindDiv:
			// Engines differ on integer division, so it is never folded.
			if (numA.isInt && numB.isInt) || a.Parent() != b.Parent() {
				return nil
			}
			q, ok := numA.div(numB)
			if !ok {
				return nil
			}
			return literalNumber(q)
		}
		if boolean := evalBoolean(expression, numA.cmp(numB)); boolean != nil {
			return boolean
		}
	case a.IsString() && b.IsString():
		if boolean := evalBoolean(expression, strings.Compare(a.Text("this"), b.Text("this"))); boolean != nil {
			return boolean
		}
	case isDateLiteral(a) && b.Kind() == exp.KindInterval:
		date, _ := extractDate(a)
		if rd, ok := extractInterval(b); ok && !rd.isZero() {
			switch expression.Kind() {
			case exp.KindAdd, exp.KindDateAdd:
				return dateLiteral(rd.addTo(date), extractType(a))
			case exp.KindSub, exp.KindDateSub:
				return dateLiteral(rd.negate().addTo(date), extractType(a))
			}
			if k, ok := kindIfExists("DatetimeAdd"); ok && expression.Kind() == k {
				return dateLiteral(rd.addTo(date), extractType(a))
			}
			if k, ok := kindIfExists("DatetimeSub"); ok && expression.Kind() == k {
				return dateLiteral(rd.negate().addTo(date), extractType(a))
			}
		}
	case a.Kind() == exp.KindInterval && isDateLiteral(b):
		// You cannot subtract a date from an interval.
		rd, ok := extractInterval(a)
		date, _ := extractDate(b)
		if ok && !rd.isZero() && expression.Kind() == exp.KindAdd {
			return dateLiteral(rd.addTo(date), extractType(b))
		}
	case isDateLiteral(a) && isDateLiteral(b):
		if expression.Is(exp.TraitPredicate) {
			da, _ := extractDate(a)
			db, _ := extractDate(b)
			if boolean := evalDates(expression, da, db); boolean != nil {
				return boolean
			}
		}
	}
	return nil
}

// simplifyCoalesce ports simplify_coalesce (simplify.py:1303-1363).
func (s *Simplifier) simplifyCoalesce(expression exp.Expression) exp.Expression {
	return s.onChange(expression, s.simplifyCoalesceInner(expression))
}

func (s *Simplifier) simplifyCoalesceInner(expression exp.Expression) exp.Expression {
	// COALESCE(x) -> x; COALESCE is also used as a Spark partitioning hint.
	if expression.Kind() == exp.KindCoalesce &&
		(len(expression.Expressions()) == 0 || isNonnullConstant(expression.This())) &&
		!(expression.Parent() != nil && expression.Parent().Kind() == exp.KindHint) {
		return expression.This()
	}
	if s.dialect.CoalesceComparisonNonStandard || !isComparison(expression.Kind()) {
		return expression
	}

	var coalesce, other exp.Expression
	switch {
	case expression.Left() != nil && expression.Left().Kind() == exp.KindCoalesce:
		coalesce, other = expression.Left(), expression.Right()
	case expression.Right() != nil && expression.Right().Kind() == exp.KindCoalesce:
		coalesce, other = expression.Right(), expression.Left()
	default:
		return expression
	}

	// Valid for non-constants, but it only does anything if both are constants.
	if !isSimplifyConstant(other) {
		return expression
	}

	// Find the first constant arg.
	argIndex := -1
	var arg exp.Expression
	for i, e := range coalesce.Expressions() {
		if isSimplifyConstant(e) {
			argIndex, arg = i, e
			break
		}
	}
	if argIndex < 0 {
		return expression
	}

	coalesce.Set("expressions", append([]exp.Expression(nil), coalesce.Expressions()[:argIndex]...))

	// Remove the COALESCE function, skipping a simplify iteration.
	this := coalesce
	if len(coalesce.Expressions()) == 0 {
		this = coalesce.This()
	}

	// More complex than when we started, but it gets simplified further.
	return exp.Paren(exp.Args{"this": combine(exp.KindOr,
		combine(exp.KindAnd, this.Is_(exp.Null()).Not_(false), expression.Copy()),
		combine(exp.KindAnd, this.Is_(exp.Null()), exp.New(expression.Kind(), exp.Args{"this": arg.Copy(), "expression": other.Copy()})),
	)})
}

// simplifyConcat ports simplify_concat (simplify.py:1365-1405): groups of string literals
// are concatenated.
func (s *Simplifier) simplifyConcat(expression exp.Expression) exp.Expression {
	return s.onChange(expression, s.simplifyConcatInner(expression))
}

func (s *Simplifier) simplifyConcatInner(expression exp.Expression) exp.Expression {
	concatWs := isClass(expression, "ConcatWs")
	if !isConcat(expression) && !concatWs {
		return expression
	}
	// We can't reduce a CONCAT_WS call if we don't statically know the separator.
	if concatWs && (len(expression.Expressions()) == 0 || !expression.Expressions()[0].IsString()) {
		return expression
	}

	var sepExpr exp.Expression
	var expressions []exp.Expression
	sep := ""
	concatKind := exp.KindConcat
	if concatWs {
		sepExpr = expression.Expressions()[0]
		expressions = expression.Expressions()[1:]
		sep = sepExpr.Name()
		concatKind = expression.Kind()
	} else {
		expressions = expression.Expressions()
	}

	safe, coalesce := expression.Arg("safe"), expression.Arg("coalesce")

	operands := expressions
	if len(operands) == 0 {
		operands = flatten(expression, true)
	}
	var newArgs []exp.Expression
	for i := 0; i < len(operands); {
		j := i
		for j < len(operands) && operands[j].IsString() == operands[i].IsString() {
			j++
		}
		if operands[i].IsString() {
			var names []string
			for _, str := range operands[i:j] {
				names = append(names, str.Name())
			}
			newArgs = append(newArgs, literalString(strings.Join(names, sep)))
		} else {
			newArgs = append(newArgs, operands[i:j]...)
		}
		i = j
	}

	if len(newArgs) == 1 && newArgs[0].IsString() {
		return newArgs[0]
	}

	if concatWs {
		newArgs = append([]exp.Expression{sepExpr}, newArgs...)
	} else if expression.Kind() == exp.KindDPipe {
		out := newArgs[0]
		for _, arg := range newArgs[1:] {
			args := exp.Args{"this": out, "expression": arg}
			if safe != nil {
				args["safe"] = safe
			}
			out = exp.DPipe(args)
		}
		return out
	}

	args := exp.Args{"expressions": newArgs}
	if safe != nil {
		args["safe"] = safe
	}
	if coalesce != nil {
		args["coalesce"] = coalesce
	}
	return exp.New(concatKind, args)
}

// simplifyConditionals ports simplify_conditionals (simplify.py:1407-1431): IF and CASE with
// a statically known condition.
func (s *Simplifier) simplifyConditionals(expression exp.Expression) exp.Expression {
	return s.onChange(expression, s.simplifyConditionalsInner(expression))
}

func (s *Simplifier) simplifyConditionalsInner(expression exp.Expression) exp.Expression {
	switch {
	case expression.Kind() == exp.KindCase:
		this := expression.This()
		// Python iterates the live ifs list while popping from it, so the element after a
		// popped one is skipped; the index walk below does the same.
		for i := 0; i < len(expressionsFor(expression, "ifs")); i++ {
			caseNode := expressionsFor(expression, "ifs")[i]
			cond := caseNode.This()
			if this != nil {
				// CASE x WHEN v ... -> CASE WHEN x = v ...
				cond = cond.Replace(this.Pop().Eq(cond))
			}
			if alwaysTrue(cond) {
				return asExpression(caseNode.Arg("true"))
			}
			if alwaysFalse(cond) {
				caseNode.Pop()
				if len(expressionsFor(expression, "ifs")) == 0 {
					if def := asExpression(expression.Arg("default")); def != nil {
						return def
					}
					return nullLiteral()
				}
			}
		}
	case expression.Kind() == exp.KindIf && !(expression.Parent() != nil && expression.Parent().Kind() == exp.KindCase):
		if alwaysTrue(expression.This()) {
			return asExpression(expression.Arg("true"))
		}
		if alwaysFalse(expression.This()) {
			if f := asExpression(expression.Arg("false")); f != nil {
				return f
			}
			return nullLiteral()
		}
	}
	return expression
}

// simplifyStartswith ports simplify_startswith (simplify.py:1433-1450).
func (s *Simplifier) simplifyStartswith(expression exp.Expression) exp.Expression {
	out := expression
	if expression.Kind() == exp.KindStartsWith && expression.This() != nil && expression.This().IsString() &&
		expression.Expr() != nil && expression.Expr().IsString() {
		out = booleanLiteral(strings.HasPrefix(expression.Name(), expression.Expr().Name()))
	}
	return s.onChange(expression, out)
}

func (s *Simplifier) isDatetruncPredicate(left, right exp.Expression) bool {
	return isDateTrunc(left) && isDateLiteral(right)
}

// datetruncRange ports _datetrunc_range (simplify.py:236-251): [floor, floor + unit), or
// false when no value can equal date for unit.
func (s *Simplifier) datetruncRange(date pyDate, unit string) ([2]pyDate, bool) {
	floor := datetimeFloor(date, unit, s.dialect.WeekOffset)
	if floor.cmp(date) != 0 || floor.isDatetime != date.isDatetime {
		// Always False, except for NULL values.
		return [2]pyDate{}, false
	}
	return [2]pyDate{floor, interval(unit, 1).addTo(floor)}, true
}

func datetruncEqExpression(left exp.Expression, drange [2]pyDate, targetType exp.Expression) exp.Expression {
	return combine(exp.KindAnd,
		binop(exp.KindGTE, left, dateLiteral(drange[0], targetType)),
		binop(exp.KindLT, left, dateLiteral(drange[1], targetType)),
	)
}

// datetruncBinary ports DATETRUNC_BINARY_COMPARISONS (simplify.py:609-623).
func (s *Simplifier) datetruncBinary(kind exp.Kind, l exp.Expression, dt pyDate, unit string, t exp.Expression) exp.Expression {
	wo := s.dialect.WeekOffset
	floor := func() pyDate { return datetimeFloor(dt, unit, wo) }
	switch kind {
	case exp.KindLT:
		target := dt
		if f := floor(); !(f.cmp(dt) == 0 && f.isDatetime == dt.isDatetime) {
			target = interval(unit, 1).addTo(f)
		}
		return binop(exp.KindLT, l, dateLiteral(target, t))
	case exp.KindGT:
		return binop(exp.KindGTE, l, dateLiteral(interval(unit, 1).addTo(floor()), t))
	case exp.KindLTE:
		return binop(exp.KindLT, l, dateLiteral(interval(unit, 1).addTo(floor()), t))
	case exp.KindGTE:
		return binop(exp.KindGTE, l, dateLiteral(dateCeil(dt, unit, wo), t))
	case exp.KindEQ:
		drange, ok := s.datetruncRange(dt, unit)
		if !ok {
			return nil
		}
		return datetruncEqExpression(l, drange, t)
	case exp.KindNEQ:
		drange, ok := s.datetruncRange(dt, unit)
		if !ok {
			return nil
		}
		return combine(exp.KindOr,
			binop(exp.KindLT, l, dateLiteral(drange[0], t)),
			binop(exp.KindGTE, l, dateLiteral(drange[1], t)),
		)
	}
	return nil
}

// simplifyDatetrunc ports simplify_datetrunc (simplify.py:1452-1529), e.g.
// DATE_TRUNC('year', x) >= CAST('2021-01-01' AS DATE).
func (s *Simplifier) simplifyDatetrunc(expression exp.Expression) exp.Expression {
	out := catchUnsupportedUnit(expression, func() exp.Expression { return s.simplifyDatetruncInner(expression) })
	return s.onChange(expression, out)
}

func (s *Simplifier) simplifyDatetruncInner(expression exp.Expression) exp.Expression {
	kind := expression.Kind()
	if isDateTrunc(expression) {
		this := expression.This()
		if this == nil {
			return expression
		}
		truncType := extractType(this)
		date, ok := extractDate(this)
		if unit := asExpression(expression.Arg("unit")); ok && unit != nil {
			return dateLiteral(datetimeFloor(date, truncUnit(unit), s.dialect.WeekOffset), truncType)
		}
		return expression
	}
	switch kind {
	case exp.KindIn, exp.KindLT, exp.KindGT, exp.KindLTE, exp.KindGTE, exp.KindEQ, exp.KindNEQ:
	default:
		return expression
	}

	if expression.Is(exp.TraitBinary) {
		l, r := expression.Left(), expression.Right()
		if l == nil || r == nil || !s.isDatetruncPredicate(l, r) {
			return expression
		}
		truncArg := l.This()
		unit := truncUnit(asExpression(l.Arg("unit")))
		date, ok := extractDate(r)
		if !ok {
			return expression
		}
		simplified := s.datetruncBinary(kind, truncArg, date, unit, extractType(r))
		if simplified == nil {
			return expression
		}
		return parenthesizeNestedConnector(simplified, expression.Parent())
	}

	if kind == exp.KindIn {
		l := expression.This()
		rs := expression.Expressions()
		if l == nil || len(rs) == 0 || !isDateTrunc(l) {
			return expression
		}
		for _, r := range rs {
			if !s.isDatetruncPredicate(l, r) {
				return expression
			}
		}
		unit := truncUnit(asExpression(l.Arg("unit")))
		var ranges [][2]pyDate
		for _, r := range rs {
			date, ok := extractDate(r)
			if !ok {
				return expression
			}
			if drange, ok := s.datetruncRange(date, unit); ok {
				ranges = append(ranges, drange)
			}
		}
		if len(ranges) == 0 {
			return expression
		}
		ranges = mergeRanges(ranges)
		targetType := extractType(rs...)
		var parts []exp.Expression
		for _, drange := range ranges {
			parts = append(parts, datetruncEqExpression(l, drange, targetType))
		}
		return parenthesizeNestedConnector(combine(exp.KindOr, parts...), expression.Parent())
	}
	return expression
}

// mergeRanges ports helper.merge_ranges (helper.py:405-430).
func mergeRanges(ranges [][2]pyDate) [][2]pyDate {
	if len(ranges) == 0 {
		return nil
	}
	sorted := append([][2]pyDate(nil), ranges...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if c := sorted[i][0].cmp(sorted[j][0]); c != 0 {
			return c < 0
		}
		return sorted[i][1].cmp(sorted[j][1]) < 0
	})
	merged := [][2]pyDate{sorted[0]}
	for _, r := range sorted[1:] {
		last := &merged[len(merged)-1]
		if r[0].cmp(last[1]) <= 0 {
			if r[1].cmp(last[1]) > 0 {
				last[1] = r[1]
			}
		} else {
			merged = append(merged, r)
		}
	}
	return merged
}

// sortComparison ports sort_comparison (simplify.py:1531-1549): a constant goes on the right,
// a column on the left, and otherwise operands order by gen().
func (s *Simplifier) sortComparison(expression exp.Expression) exp.Expression {
	out := expression
	if _, ok := complementComparisons[expression.Kind()]; ok {
		l, r := expression.This(), expression.Expr()
		if l != nil && r != nil {
			lColumn, rColumn := l.Kind() == exp.KindColumn, r.Kind() == exp.KindColumn
			lConst, rConst := isSimplifyConstant(l), isSimplifyConstant(r)
			switch {
			case (lColumn && !rColumn) || (rConst && !lConst) || isSubqueryPredicate(r):
			case (rColumn && !lColumn) || (lConst && !rConst) || simplifyGen(l, false) > simplifyGen(r, false):
				k := expression.Kind()
				if inv, ok := inverseComparisons[k]; ok {
					k = inv
				}
				out = exp.New(k, exp.Args{"this": r, "expression": l})
			}
		}
	}
	return s.onChange(expression, out)
}

// connectorCombinable ports CONNECTOR_COMBINABLE (simplify.py:581-583).
func connectorCombinable(e exp.Expression) bool {
	switch e.Kind() {
	case exp.KindBoolean, exp.KindLiteral, exp.KindNull:
		return true
	}
	return isComparison(e.Kind())
}

// flatSimplify ports _flat_simplify (simplify.py:1551-1590).
func (s *Simplifier) flatSimplify(expression exp.Expression, simplifier func(e, a, b exp.Expression) exp.Expression, root bool) exp.Expression {
	if !(root || !expression.SameParent()) {
		return expression
	}
	queue := flatten(expression, false)
	size := len(queue)

	// The pairwise scan is O(n^2), and a connector pair only combines when one side is a
	// constant or both are comparisons, so skip connectors with no combinable operand.
	if expression.Is(exp.TraitConnector) {
		combinable := false
		for _, op := range queue {
			if connectorCombinable(op) {
				combinable = true
				break
			}
		}
		if !combinable {
			return expression
		}
	}

	var operands []exp.Expression
	for len(queue) > 0 {
		a := queue[0]
		queue = queue[1:]
		merged := false
		for _, b := range queue {
			result := simplifier(expression, a, b)
			if result != nil && result != expression {
				// queue.remove(b) drops the first operand equal to b.
				idx := indexOfExpr(queue, b)
				queue = append(queue[:idx:idx], queue[idx+1:]...)
				queue = append([]exp.Expression{result}, queue...)
				merged = true
				break
			}
		}
		if !merged {
			operands = append(operands, a)
		}
	}

	if len(operands) < size {
		out := operands[0]
		for _, b := range operands[1:] {
			out = exp.New(expression.Kind(), exp.Args{"this": out, "expression": b})
		}
		return out
	}
	return expression
}
