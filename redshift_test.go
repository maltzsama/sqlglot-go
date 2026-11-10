package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestRedshiftColumnUnnesting ports tests/dialects/test_redshift.py::test_column_unnesting
// (lines 669-695). Redshift's SUPPORTS_IMPLICIT_UNNEST rewrites a `FROM a, a.b` join whose
// leftmost part names an already-introduced relation into an explicit Unnest, while a join
// carrying an ON clause stays a plain Table. The two validate_identity round-trips that open
// the upstream method are asserted by the identity table ported alongside this file.
func TestRedshiftColumnUnnesting(t *testing.T) {
	cases := []struct {
		sql string
		// fromKind is the Kind of from_.this; joinKinds are the Kinds of each join's `this`.
		fromKind  exp.Kind
		joinKinds []exp.Kind
		// dotJoin is the join index whose Unnest expression must be a Dot, or -1 for none.
		dotJoin int
	}{
		{
			// ON makes this a real table join, so no unnest conversion applies.
			sql:      "SELECT * FROM t.t JOIN t.c1 ON c1.c2 = t.c3",
			fromKind: exp.KindTable, joinKinds: []exp.Kind{exp.KindTable}, dotJoin: -1,
		},
		{
			sql:      "SELECT * FROM t AS t CROSS JOIN t.c1",
			fromKind: exp.KindTable, joinKinds: []exp.Kind{exp.KindUnnest}, dotJoin: -1,
		},
		{
			// Three column-unnesting joins; the fourth-part-plus name lands in a Dot chain.
			sql:      "SELECT * FROM x AS a, a.b AS c, c.d.e AS f, f.g.h.i.j.k AS l",
			fromKind: exp.KindTable, joinKinds: []exp.Kind{exp.KindUnnest, exp.KindUnnest, exp.KindUnnest}, dotJoin: 2,
		},
	}

	for _, tc := range cases {
		e, err := sqlglot.ParseOne(tc.sql, "redshift")
		if err != nil {
			t.Errorf("ParseOne(%q, redshift): %v", tc.sql, err)
			continue
		}

		from, _ := e.Arg("from_").(exp.Expression)
		if from == nil {
			t.Errorf("%q: missing from_", tc.sql)
			continue
		}
		if from.This() == nil {
			t.Errorf("%q: from_ has no this", tc.sql)
			continue
		}
		if got := from.This().Kind(); got != tc.fromKind {
			t.Errorf("%q: from_.this = %s, want %s", tc.sql, exp.ClassName(got), exp.ClassName(tc.fromKind))
		}

		joins, _ := e.Arg("joins").([]exp.Expression)
		if len(joins) != len(tc.joinKinds) {
			t.Errorf("%q: %d joins, want %d", tc.sql, len(joins), len(tc.joinKinds))
			continue
		}
		for i, join := range joins {
			joinThis, _ := join.Arg("this").(exp.Expression)
			if joinThis == nil {
				t.Errorf("%q: join[%d] has no this", tc.sql, i)
				continue
			}
			if got := joinThis.Kind(); got != tc.joinKinds[i] {
				t.Errorf("%q: join[%d].this = %s, want %s", tc.sql, i,
					exp.ClassName(got), exp.ClassName(tc.joinKinds[i]))
			}
			if i == tc.dotJoin {
				exprs := joinThis.Expressions()
				if len(exprs) == 0 || exprs[0].Kind() != exp.KindDot {
					t.Errorf("%q: join[%d] expression = %v, want Dot", tc.sql, i, exprs)
				}
			}
		}

		out, err := sqlglot.Generate(e, "redshift", generator.Options{})
		if err != nil {
			t.Errorf("Generate(%q, redshift): %v", tc.sql, err)
			continue
		}
		if out != tc.sql {
			t.Errorf("Generate(%q, redshift) = %q", tc.sql, out)
		}
	}
}
