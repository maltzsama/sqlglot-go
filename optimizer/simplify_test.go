package optimizer_test

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/dialects"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
	"github.com/maltzsama/sqlglot-go/optimizer"
)

// simplifyGapsPath lists the simplify.sql cases that don't pass yet. The fixture test fails on
// any failure missing from it and on any listed case that now passes, so the list only shrinks.
// Regenerate with SIMPLIFY_GAPS_UPDATE=1.
const simplifyGapsPath = "testdata/simplify_gaps.txt"

// TestSimplifyFixtures ports test_simplify's fixture run (tests/test_optimizer.py:1194-1195,
// with its simplify wrapper at lines 63-70): annotate_types, then simplify with constant
// propagation and coalesce simplification.
func TestSimplifyFixtures(t *testing.T) {
	data, _ := os.ReadFile(simplifyGapsPath)
	gaps := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			gaps[line] = true
		}
	}
	schema := optimizerTestSchema()
	var failures []string
	passed, total := 0, 0
	for i, pair := range loadSQLFixturePairs(t, "simplify.sql") {
		dialect := pair.Meta["dialect"]
		if _, err := dialects.GetOrRaise(dialect); err != nil {
			continue
		}
		total++
		key := fmt.Sprintf("%d|%s", i+1, pair.SQL)
		got, err := func() (got string, err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic: %v", r)
				}
			}()
			e, err := sqlglot.ParseOne(pair.SQL, dialect)
			if err != nil {
				return "", err
			}
			e = optimizer.AnnotateTypes(e, schema, dialect)
			e = optimizer.Simplify(e, true, true, dialect)
			return sqlglot.Generate(e, dialect, generator.Options{})
		}()
		ok := err == nil && got == pair.Expected
		if ok {
			passed++
		} else {
			failures = append(failures, key)
		}
		switch {
		case !ok && !gaps[key]:
			t.Errorf("%s: got %q, want %q (err %v)", key, got, pair.Expected, err)
		case ok && gaps[key]:
			t.Errorf("%s passes now; remove it from %s", key, simplifyGapsPath)
		}
	}
	t.Logf("simplify.sql: %d/%d cases pass", passed, total)
	if os.Getenv("SIMPLIFY_GAPS_UPDATE") == "1" {
		sort.Strings(failures)
		header := "# simplify.sql cases that don't pass yet: index|sql. See optimizer/simplify_test.go.\n"
		if err := os.WriteFile(simplifyGapsPath, []byte(header+strings.Join(failures, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// simplifyWrapper is test_optimizer.py's simplify helper (lines 63-70).
func simplifyWrapper(e exp.Expression, dialect string) exp.Expression {
	e = optimizer.AnnotateTypes(e, nil, dialect)
	return optimizer.Simplify(e, true, true, dialect)
}

// TestSimplifyBehavior ports the inline assertions of test_simplify
// (tests/test_optimizer.py:1197-1245).
func TestSimplifyBehavior(t *testing.T) {
	// Stress test with a huge union query.
	union := strings.Repeat("SELECT 1 UNION ALL ", 1000) + "SELECT 1"
	if got := mustSQL(t, optimizer.Simplify(mustParse(t, union), false, false, "")); got != union {
		t.Error("simplify changed a huge UNION ALL")
	}

	// simplify mutates the AST in place.
	e := mustParse(t, "SELECT 1 + 2")
	simplifyWrapper(e.Selects()[0], "")
	if got := mustSQL(t, e); got != "SELECT 3" {
		t.Errorf("in-place simplify = %q, want SELECT 3", got)
	}

	where := mustParse(t, "SELECT a, c, b FROM table1 WHERE 1 = 1").Find(exp.KindWhere)
	if got := mustSQL(t, simplifyWrapper(simplifyWrapper(where, ""), "")); got != "WHERE TRUE" {
		t.Errorf("simplify(WHERE 1 = 1) = %q, want WHERE TRUE", got)
	}

	if got := optimizer.Simplify(mustParse(t, "TRUE AND TRUE AND TRUE"), false, false, ""); !got.Equal(exp.Boolean(exp.Args{"this": true})) {
		t.Errorf("TRUE AND TRUE AND TRUE = %s", mustSQL(t, got))
	}

	// simplify_concat keeps the CONCAT's safe flag: Presto's CONCAT parses as an unsafe Concat.
	for _, tc := range []struct {
		dialect string
		safe    bool
	}{{"presto", false}, {"", true}} {
		concat, err := sqlglot.ParseOne("CONCAT('a', x, 'b', 'c')", tc.dialect)
		if err != nil {
			t.Fatal(err)
		}
		simplified := optimizer.Simplify(concat, false, false, "")
		if got, _ := simplified.Arg("safe").(bool); got != tc.safe {
			t.Errorf("[%s] simplified CONCAT safe = %v, want %v", tc.dialect, got, tc.safe)
		}
		if got, _ := sqlglot.Generate(simplified, tc.dialect, generator.Options{}); got != "CONCAT('a', x, 'bc')" {
			t.Errorf("[%s] simplified CONCAT = %q", tc.dialect, got)
		}
	}
}

// TestSimplifyNested ports test_simplify_nested (tests/test_optimizer.py:1313-1332): a GROUP BY
// key, and the projections that match it, are left alone.
func TestSimplifyNested(t *testing.T) {
	sql := `SELECT x, 1 + 1 FROM foo WHERE x > (((select x + 1 + 1, sum(y + 1 + 1) FROM bar GROUP BY x + 1 + 1)))`
	want := `SELECT x, 2 FROM foo WHERE x > (((select x + 1 + 1, sum(y + 2) FROM bar GROUP BY x + 1 + 1)))`
	got, _ := sqlglot.Generate(optimizer.Simplify(mustParse(t, sql), false, false, ""), "", generator.Options{Pretty: true})
	expected, _ := sqlglot.Generate(mustParse(t, want), "", generator.Options{Pretty: true})
	if got != expected {
		t.Errorf("got:\n%s\nwant:\n%s", got, expected)
	}
}
