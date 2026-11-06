package optimizer

import (
	"math/big"
	"strings"
)

// pyNum is the Python value Literal.to_py yields for a number literal (expressions/core.py:
// 1774-1783): an int when the text is one, otherwise a decimal.Decimal. simplify folds constants
// with Python's arithmetic on those values and renders the result with str(), so this ports the
// int and Decimal semantics it relies on: exact int arithmetic, Decimal arithmetic under the
// default context (28 significant digits, ROUND_HALF_EVEN), comparisons, and str().
type pyNum struct {
	isInt bool
	neg   bool
	coef  *big.Int // magnitude; the value is ±coef * 10^exp
	exp   int      // always 0 for an int
}

const decimalPrec = 28

var bigTen = big.NewInt(10)

func pow10(n int) *big.Int {
	return new(big.Int).Exp(bigTen, big.NewInt(int64(n)), nil)
}

// parsePyNum ports Literal.to_py for a number: int(text), else Decimal(text). ok is false when
// neither accepts the text.
func parsePyNum(text string) (pyNum, bool) {
	s := strings.TrimSpace(text)
	if n, ok := parsePyInt(s); ok {
		return n, true
	}
	return parsePyDecimal(s)
}

// parsePyInt ports int(text) for base-10 text: an optional sign, then digits with single
// underscores between them.
func parsePyInt(s string) (pyNum, bool) {
	neg := false
	if s != "" && (s[0] == '+' || s[0] == '-') {
		neg = s[0] == '-'
		s = s[1:]
	}
	digits, ok := stripUnderscores(s)
	if !ok || digits == "" {
		return pyNum{}, false
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return pyNum{}, false
		}
	}
	coef, _ := new(big.Int).SetString(digits, 10)
	return pyNum{isInt: true, neg: neg && coef.Sign() != 0, coef: coef}, true
}

// stripUnderscores removes the single underscores Python allows between digits.
func stripUnderscores(s string) (string, bool) {
	if !strings.Contains(s, "_") {
		return s, true
	}
	if strings.HasPrefix(s, "_") || strings.HasSuffix(s, "_") || strings.Contains(s, "__") {
		return "", false
	}
	return strings.ReplaceAll(s, "_", ""), true
}

// parsePyDecimal ports Decimal(text) for finite values: [sign] digits [. digits] [e|E [sign]
// digits], with Python's underscore rule.
func parsePyDecimal(s string) (pyNum, bool) {
	neg := false
	if s != "" && (s[0] == '+' || s[0] == '-') {
		neg = s[0] == '-'
		s = s[1:]
	}
	mantissa, exponent := s, ""
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		mantissa, exponent = s[:i], s[i+1:]
		if exponent == "" {
			return pyNum{}, false
		}
	}
	intPart, fracPart := mantissa, ""
	if i := strings.IndexByte(mantissa, '.'); i >= 0 {
		intPart, fracPart = mantissa[:i], mantissa[i+1:]
	}
	var ok bool
	if intPart, ok = stripUnderscores(intPart); !ok {
		return pyNum{}, false
	}
	if fracPart, ok = stripUnderscores(fracPart); !ok {
		return pyNum{}, false
	}
	digits := intPart + fracPart
	if digits == "" {
		return pyNum{}, false
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return pyNum{}, false
		}
	}
	exp := -len(fracPart)
	if exponent != "" {
		e, ok := parsePyInt(exponent)
		if !ok || !e.coef.IsInt64() {
			return pyNum{}, false
		}
		v := int(e.coef.Int64())
		if e.neg {
			v = -v
		}
		exp += v
	}
	coef, _ := new(big.Int).SetString(digits, 10)
	return pyNum{neg: neg, coef: coef, exp: exp}, true
}

func (n pyNum) toDecimal() pyNum {
	return pyNum{neg: n.neg, coef: n.coef, exp: n.exp}
}

func (n pyNum) isZero() bool { return n.coef.Sign() == 0 }

// signed returns the value's coefficient with its sign, aligned to exponent exp (<= n.exp).
func (n pyNum) signedAt(exp int) *big.Int {
	c := new(big.Int).Mul(n.coef, pow10(n.exp-exp))
	if n.neg {
		c.Neg(c)
	}
	return c
}

// fix ports Decimal._fix for the default context: round to 28 significant digits, half even.
func (n pyNum) fix() pyNum {
	digits := len(n.coef.String())
	if n.coef.Sign() == 0 || digits <= decimalPrec {
		return n
	}
	drop := digits - decimalPrec
	q, r := new(big.Int).QuoRem(n.coef, pow10(drop), new(big.Int))
	half := new(big.Int).Mul(big.NewInt(5), pow10(drop-1))
	if c := r.Cmp(half); c > 0 || (c == 0 && q.Bit(0) == 1) {
		q.Add(q, big.NewInt(1))
	}
	exp := n.exp + drop
	if len(q.String()) > decimalPrec {
		q.Quo(q, bigTen)
		exp++
	}
	return pyNum{neg: n.neg, coef: q, exp: exp}
}

func fromSigned(c *big.Int, exp int) pyNum {
	return pyNum{neg: c.Sign() < 0, coef: new(big.Int).Abs(c), exp: exp}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// add ports int.__add__ / Decimal.__add__ (a zero sum takes the smaller exponent, positive
// unless both operands are negative).
func (n pyNum) add(o pyNum) pyNum {
	if n.isInt && o.isInt {
		r := fromSigned(new(big.Int).Add(n.signedAt(0), o.signedAt(0)), 0)
		r.isInt = true
		return r
	}
	a, b := n.toDecimal(), o.toDecimal()
	exp := minInt(a.exp, b.exp)
	sum := new(big.Int).Add(a.signedAt(exp), b.signedAt(exp))
	r := fromSigned(sum, exp)
	if sum.Sign() == 0 {
		r.neg = a.neg && b.neg
	}
	return r.fix()
}

func (n pyNum) negate() pyNum {
	r := n
	r.neg = !n.neg && !n.isZero()
	if !n.isInt && n.isZero() {
		r.neg = !n.neg
	}
	return r
}

func (n pyNum) sub(o pyNum) pyNum { return n.add(o.negate()) }

// mul ports int.__mul__ / Decimal.__mul__.
func (n pyNum) mul(o pyNum) pyNum {
	coef := new(big.Int).Mul(n.coef, o.coef)
	r := pyNum{isInt: n.isInt && o.isInt, neg: n.neg != o.neg, coef: coef, exp: n.exp + o.exp}
	if r.isInt {
		r.neg = r.neg && coef.Sign() != 0
		return r
	}
	return r.fix()
}

// div ports Decimal.__truediv__ (the caller never divides two ints). ok is false for a zero
// divisor, where Python raises.
func (n pyNum) div(o pyNum) (pyNum, bool) {
	a, b := n.toDecimal(), o.toDecimal()
	if b.isZero() {
		return pyNum{}, false
	}
	neg := a.neg != b.neg
	if a.isZero() {
		return pyNum{neg: neg, coef: new(big.Int), exp: a.exp - b.exp}.fix(), true
	}
	shift := len(b.coef.String()) - len(a.coef.String()) + decimalPrec + 1
	exp := a.exp - b.exp - shift
	var coef, rem *big.Int
	if shift >= 0 {
		coef, rem = new(big.Int).QuoRem(new(big.Int).Mul(a.coef, pow10(shift)), b.coef, new(big.Int))
	} else {
		coef, rem = new(big.Int).QuoRem(a.coef, new(big.Int).Mul(b.coef, pow10(-shift)), new(big.Int))
	}
	five := big.NewInt(5)
	if rem.Sign() != 0 {
		if new(big.Int).Mod(coef, five).Sign() == 0 {
			coef.Add(coef, big.NewInt(1))
		}
	} else {
		idealExp := a.exp - b.exp
		for exp < idealExp && new(big.Int).Mod(coef, bigTen).Sign() == 0 {
			coef.Quo(coef, bigTen)
			exp++
		}
	}
	return pyNum{neg: neg, coef: coef, exp: exp}.fix(), true
}

// cmp compares two numbers by value, as Python does across int and Decimal.
func (n pyNum) cmp(o pyNum) int {
	exp := minInt(n.exp, o.exp)
	return n.signedAt(exp).Cmp(o.signedAt(exp))
}

func (n pyNum) abs() pyNum {
	r := n
	r.neg = false
	return r
}

func (n pyNum) isNegative() bool { return n.neg && !n.isZero() }

// String ports str(int) and Decimal.__str__ (to-scientific-string, capital E).
func (n pyNum) String() string {
	sign := ""
	if n.neg {
		sign = "-"
	}
	digits := n.coef.String()
	if n.isInt {
		return sign + digits
	}
	leftDigits := n.exp + len(digits)
	dotPlace := 1
	if n.exp <= 0 && leftDigits > -6 {
		dotPlace = leftDigits
	}
	var intPart, fracPart string
	switch {
	case dotPlace <= 0:
		intPart = "0"
		fracPart = "." + strings.Repeat("0", -dotPlace) + digits
	case dotPlace >= len(digits):
		intPart = digits + strings.Repeat("0", dotPlace-len(digits))
	default:
		intPart = digits[:dotPlace]
		fracPart = "." + digits[dotPlace:]
	}
	exp := ""
	if leftDigits != dotPlace {
		e := leftDigits - dotPlace
		if e >= 0 {
			exp = "E+" + itoa(e)
		} else {
			exp = "E" + itoa(e)
		}
	}
	return sign + intPart + fracPart + exp
}

func itoa(i int) string { return big.NewInt(int64(i)).String() }
