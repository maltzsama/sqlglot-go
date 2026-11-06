package optimizer

import (
	"fmt"
	"strings"
	"unicode"

	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// Ports gen / Gen (optimizer/simplify.py:1593-1880): a bare pseudo-SQL generator whose strings
// simplify sorts and dedupes operands by. The output must match upstream byte for byte, since it
// decides the operand order uniq_sort and sort_comparison produce, so the stack machine is
// ported as is, including Python's str() / repr() of non-expression values.

// genSep is a literal fragment on the Gen stack (a Python str pushed by a handler).
type genSep string

// genList is a Python list on the Gen stack.
type genList []any

// simplifyGen ports gen(expression, comments).
func simplifyGen(expression exp.Expression, comments bool) string {
	stack := []any{expression}
	var sqls strings.Builder

	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		switch v := node.(type) {
		case exp.Expression:
			if v == nil {
				continue
			}
			if comments && len(v.Comments()) > 0 {
				stack = append(stack, genSep(" /*"+strings.Join(v.Comments(), ",")+"*/"))
			}
			stack = genNode(v, stack)
		case genList:
			for i := len(v) - 1; i >= 0; i-- {
				if v[i] != nil && !isNilExpr(v[i]) {
					stack = append(stack, v[i], genSep(","))
				}
			}
			// `if node: self.stack.pop()`: a non-empty list always pops the top, the trailing
			// "," when any element was pushed.
			if len(v) > 0 && len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		case genSep:
			sqls.WriteString(string(v))
		default:
			if node != nil {
				sqls.WriteString(pyStr(node))
			}
		}
	}
	return sqls.String()
}

func isNilExpr(v any) bool {
	e, ok := v.(exp.Expression)
	return ok && e == nil
}

// pyStr ports str() for the scalar arg values an expression holds.
func pyStr(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case bool:
		if x {
			return "True"
		}
		return "False"
	case exp.DType:
		return "DType." + string(x)
	}
	return fmt.Sprint(v)
}

// pyTypeName is type(v).__name__ for a scalar arg value.
func pyTypeName(v any) string {
	switch v.(type) {
	case int, int64:
		return "int"
	case bool:
		return "bool"
	case float64:
		return "float"
	}
	return fmt.Sprintf("%T", v)
}

// pyRepr ports repr() of a Python str.
func pyRepr(s string) string {
	quote := '\''
	if strings.ContainsRune(s, '\'') && !strings.ContainsRune(s, '"') {
		quote = '"'
	}
	var b strings.Builder
	b.WriteRune(quote)
	for _, r := range s {
		switch {
		case r == quote || r == '\\':
			b.WriteRune('\\')
			b.WriteRune(r)
		case r == '\t':
			b.WriteString(`\t`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\x%02x`, r)
		case r > 0x7f && !unicode.IsPrint(r):
			switch {
			case r <= 0xff:
				fmt.Fprintf(&b, `\x%02x`, r)
			case r <= 0xffff:
				fmt.Fprintf(&b, `\u%04x`, r)
			default:
				fmt.Fprintf(&b, `\U%08x`, r)
			}
		default:
			b.WriteRune(r)
		}
	}
	b.WriteRune(quote)
	return b.String()
}

// argValue is node.args.get(key) as a Gen stack value.
func argValue(node exp.Expression, key string) any {
	switch v := node.Arg(key).(type) {
	case nil:
		return nil
	case exp.Expression:
		if v == nil {
			return nil
		}
		return v
	case []exp.Expression:
		out := make(genList, len(v))
		for i, e := range v {
			out[i] = e
		}
		return out
	case []any:
		return genList(v)
	default:
		return v
	}
}

// genArgs ports Gen._args: the args from argIndex in arg_types order, as `:key,value` pairs, a
// plain string value repr()-ed. It reports whether any was pushed.
func genArgs(node exp.Expression, argIndex int, stack []any) ([]any, bool) {
	keys := exp.ArgKeys(node.Kind())
	if argIndex > len(keys) {
		argIndex = len(keys)
	}
	var kvs genList
	for _, k := range keys[argIndex:] {
		v := argValue(node, k)
		if v == nil {
			continue
		}
		if s, ok := v.(string); ok {
			v = genSep(pyRepr(s))
		}
		name := k
		if k == "schema" && (node.Kind() == exp.KindTable || node.Kind() == exp.KindColumn) {
			// The port's renamed qualifier arg (DEVIATIONS §7) keeps upstream's key here.
			name = "db"
		}
		kvs = append(kvs, genList{genSep(":" + name), v})
	}
	if len(kvs) > 0 {
		return append(stack, kvs), true
	}
	return stack, false
}

func genBinary(e exp.Expression, op string, stack []any) []any {
	return append(stack, argValue(e, "expression"), genSep(op), argValue(e, "this"))
}

func genUnary(e exp.Expression, op string, stack []any) []any {
	return append(stack, argValue(e, "this"), genSep(op))
}

func genParts(parts []exp.Expression, stack []any) []any {
	for i := len(parts) - 1; i >= 0; i-- {
		stack = append(stack, parts[i], genSep("."))
	}
	if len(parts) > 0 {
		stack = stack[:len(stack)-1]
	}
	return stack
}

var genBinaryOps = func() map[exp.Kind]string {
	ops := map[exp.Kind]string{
		exp.KindAdd: " + ", exp.KindAnd: " AND ", exp.KindDiv: " / ", exp.KindDot: ".",
		exp.KindEQ: " = ", exp.KindGT: " > ", exp.KindGTE: " >= ",
		exp.KindLT: " < ", exp.KindLTE: " <= ", exp.KindMod: " % ", exp.KindMul: " * ",
		exp.KindNEQ: " <> ", exp.KindOr: " OR ", exp.KindSub: " - ",
	}
	// IntDiv has no Kind yet (#44).
	if k, ok := exp.KindByClassName("IntDiv"); ok {
		ops[k] = " DIV "
	}
	return ops
}()

// genNode dispatches one expression, porting GEN_DISPATCH and Gen.gen's fallbacks.
func genNode(e exp.Expression, stack []any) []any {
	if op, ok := genBinaryOps[e.Kind()]; ok {
		return genBinary(e, op, stack)
	}
	switch e.Kind() {
	case exp.KindAlias:
		return append(stack, argValue(e, "alias"), genSep(" AS "), argValue(e, "this"))
	case exp.KindAnonymous:
		var name string
		switch this := e.Arg("this").(type) {
		case string:
			name = strings.ToUpper(this)
		case exp.Expression:
			if this == nil || this.Kind() != exp.KindIdentifier {
				panic(fmt.Errorf("Anonymous.this expects a str or an Identifier, got '%s'.", exp.ClassName(this.Kind())))
			}
			name = this.Text("this")
			if truthyArg(this.Arg("quoted")) {
				name = `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
			} else {
				name = strings.ToUpper(name)
			}
		default:
			panic(fmt.Errorf("Anonymous.this expects a str or an Identifier, got '%s'.", pyTypeName(this)))
		}
		return append(stack, genSep(")"), argValue(e, "expressions"), genSep("("), genSep(name))
	case exp.KindBetween:
		return append(stack, argValue(e, "high"), genSep(" AND "), argValue(e, "low"), genSep(" BETWEEN "), argValue(e, "this"))
	case exp.KindBoolean:
		if truthyArg(e.Arg("this")) {
			return append(stack, genSep("TRUE"))
		}
		return append(stack, genSep("FALSE"))
	case exp.KindBracket:
		return append(stack, genSep("]"), argValue(e, "expressions"), genSep("["), argValue(e, "this"))
	case exp.KindColumn:
		return genParts(e.Parts(), stack)
	case exp.KindDataType:
		stack, _ = genArgs(e, 1, stack)
		return append(stack, genSep(dtypeName(e)+" "))
	case exp.KindFrom:
		return append(stack, argValue(e, "this"), genSep("FROM "))
	case exp.KindIdentifier:
		if truthyArg(e.Arg("quoted")) {
			return append(stack, genSep(`"`+strings.ReplaceAll(e.Text("this"), `"`, `""`)+`"`))
		}
		return append(stack, genSep(e.Text("this")))
	case exp.KindILike:
		if truthyArg(e.Arg("negate")) {
			return genBinary(e, " NOT ILIKE ", stack)
		}
		return genBinary(e, " ILIKE ", stack)
	case exp.KindIn:
		stack = append(stack, genSep(")"))
		stack, _ = genArgs(e, 1, stack)
		return append(stack, genSep("("), genSep(" IN "), argValue(e, "this"))
	case exp.KindIs:
		if truthyArg(e.Arg("negate")) {
			return genBinary(e, " IS NOT ", stack)
		}
		return genBinary(e, " IS ", stack)
	case exp.KindLike:
		if truthyArg(e.Arg("negate")) {
			return genBinary(e, " NOT Like ", stack)
		}
		return genBinary(e, " Like ", stack)
	case exp.KindLiteral:
		if e.IsString() {
			return append(stack, genSep("'"+strings.ReplaceAll(e.Text("this"), "'", "''")+"'"))
		}
		return append(stack, genSep(e.Text("this")))
	case exp.KindNeg:
		return genUnary(e, "-", stack)
	case exp.KindNot:
		return genUnary(e, "NOT ", stack)
	case exp.KindNull:
		return append(stack, genSep("NULL"))
	case exp.KindParen:
		return append(stack, genSep(")"), argValue(e, "this"), genSep("("))
	case exp.KindSubquery:
		stack, _ = genArgs(e, 2, stack)
		if alias := argValue(e, "alias"); alias != nil {
			stack = append(stack, alias)
		}
		return append(stack, genSep(")"), argValue(e, "this"), genSep("("))
	case exp.KindTable:
		stack, _ = genArgs(e, 4, stack)
		if alias := argValue(e, "alias"); alias != nil {
			stack = append(stack, alias)
		}
		return genParts(e.Parts(), stack)
	case exp.KindTableAlias:
		if columns := argValue(e, "columns"); columns != nil {
			if l, ok := columns.(genList); !ok || len(l) > 0 {
				stack = append(stack, genSep(")"), columns, genSep("("))
			}
		}
		return append(stack, argValue(e, "this"), genSep(" AS "))
	case exp.KindVar:
		return append(stack, genSep(e.Text("this")))
	}

	if e.Is(exp.TraitFunc) {
		// Gen._function: every arg value in insertion order, then the SQL name.
		var values genList
		for _, k := range argOrder(e) {
			values = append(values, argValue(e, k))
		}
		return append(stack, genSep(")"), values, genSep("("), genSep(funcSQLName(e)))
	}

	key := strings.ToUpper(exp.ClassName(e.Kind()))
	var pushed bool
	stack, pushed = genArgs(e, 0, stack)
	if pushed {
		return append(stack, genSep(key+" "))
	}
	return append(stack, genSep(key))
}

// argOrder is the node's args in insertion order (upstream's args dict order), followed by any
// spec keys that hold a value but aren't in that order.
func argOrder(e exp.Expression) []string {
	type ordered interface{ ArgOrder() []string }
	if o, ok := e.(ordered); ok {
		return o.ArgOrder()
	}
	return exp.ArgKeys(e.Kind())
}

func dtypeName(e exp.Expression) string {
	if d, ok := e.Arg("this").(exp.DType); ok {
		return string(d)
	}
	return pyStr(e.Arg("this"))
}

// funcSQLName ports Func.sql_name(), from the generated upstream table.
func funcSQLName(e exp.Expression) string {
	if name, ok := funcSQLNames[exp.ClassName(e.Kind())]; ok {
		return name
	}
	return strings.ToUpper(exp.ClassName(e.Kind()))
}
