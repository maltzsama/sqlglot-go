package optimizer

import (
	"strings"
	"testing"

	"github.com/maltzsama/sqlglot-go/dialects"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/parser"
)

func parseFor(t *testing.T, sql, dialect string) exp.Expression {
	t.Helper()
	d, err := dialects.GetOrRaise(dialect)
	if err != nil {
		t.Fatal(err)
	}
	toks, err := d.NewTokenizer().Tokenize(sql)
	if err != nil {
		t.Fatal(err)
	}
	es, err := parser.New(d).Parse(toks, sql)
	if err != nil || len(es) == 0 {
		t.Fatalf("parse %q: %v", sql, err)
	}
	return es[0]
}

// TestSimplifyGen ports the gen() assertions of test_simplify (tests/test_optimizer.py:1247-1312).
func TestSimplifyGen(t *testing.T) {
	gen := func(sql string) string { return simplifyGen(parseFor(t, sql, ""), false) }

	if got := gen("anonymous(x, y)"); got != "ANONYMOUS(x,y)" {
		t.Errorf("anonymous unquoted = %q", got)
	}
	query := parseFor(t, "SELECT x FROM t", "")
	if simplifyGen(query, false) != simplifyGen(query.Copy(), false) {
		t.Error("gen differs on a copy")
	}
	anon := exp.Anonymous(exp.Args{"this": exp.ToIdentifier("anonymous"), "expressions": []exp.Expression{
		exp.Column_("x", nil, nil, nil, nil), exp.Column_("y", nil, nil, nil, nil),
	}})
	if got := simplifyGen(anon, false); got != "ANONYMOUS(x,y)" {
		t.Errorf("anonymous identifier = %q", got)
	}
	if got := gen(`"anonymous"(x, y)`); got != `"anonymous"(x,y)` {
		t.Errorf("anonymous quoted = %q", got)
	}
	// Literal and identifier content is escaped so it can't forge structure.
	if gen("v IN ('x'',''y')") == gen("v IN ('x', 'y')") {
		t.Error("escaped literal forges two literals")
	}
	for sql, want := range map[string]string{"'''a'''": "'''a'''", `"a""b"`: `"a""b"`, `"an""on"(x)`: `"an""on"(x)`} {
		if got := gen(sql); got != want {
			t.Errorf("gen(%s) = %q, want %q", sql, got, want)
		}
	}

	func() {
		defer func() {
			r := recover()
			if r == nil || !strings.Contains(r.(error).Error(), "Anonymous.this expects a str or an Identifier, got 'int'.") {
				t.Errorf("invalid Anonymous.this: recovered %v", r)
			}
		}()
		simplifyGen(exp.Anonymous(exp.Args{"this": 5}), false)
	}()

	if got := simplifyGen(parseFor(t, "select item_id /* description */", ""), true); got != "SELECT :expressions,item_id /* description */" {
		t.Errorf("gen with comments = %q", got)
	}
}
