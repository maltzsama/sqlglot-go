package optimizer_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
	"github.com/maltzsama/sqlglot-go/optimizer"
	"github.com/maltzsama/sqlglot-go/schema"
)

// canonicalizeInternalNamesSchema is the optimizer test schema plus the two tables the
// upstream canonicalize_internal_names test adds (tests/test_optimizer.py:1521-1526).
func canonicalizeInternalNamesSchema() *schema.Mapping {
	s := optimizerTestSchema()
	s.Set("jtbl", schema.M("j", "JSON"))
	s.Set("pvt", schema.M("c", "TEXT", "v", "INT"))
	return s
}

// qualifyThenCanonicalize ports qualify_then_canonicalize (tests/test_optimizer.py:32-34):
// qualify the expression, then canonicalize its internal names.
func qualifyThenCanonicalize(t *testing.T, sql, dialect string, sch any, defaultSchema, catalog any) exp.Expression {
	t.Helper()
	expression, err := sqlglot.ParseOne(sql, dialect)
	if err != nil {
		t.Fatalf("ParseOne(%q): %v", sql, err)
	}
	opts := optimizer.DefaultQualifyOpts()
	opts.Schema = sch
	opts.Dialect = dialect
	opts.DefaultSchema = defaultSchema
	opts.Catalog = catalog
	return optimizer.CanonicalizeInternalNames(optimizer.Qualify(expression, opts))
}

func canonicalSQL(t *testing.T, e exp.Expression, dialect string) string {
	t.Helper()
	got, err := sqlglot.Generate(e, dialect, generator.Options{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return got
}

// TestCanonicalizeInternalNamesFixtures ports the fixture half of
// test_canonicalize_internal_names (tests/test_optimizer.py:1521-1534).
func TestCanonicalizeInternalNamesFixtures(t *testing.T) {
	sch := canonicalizeInternalNamesSchema()
	checkPassFixturesWith(t, "canonicalize_internal_names.sql", false, nil, func(e exp.Expression, dialect string) exp.Expression {
		opts := optimizer.DefaultQualifyOpts()
		opts.Schema = sch
		opts.Dialect = dialect
		opts.DefaultSchema = "db"
		opts.Catalog = "c"
		return optimizer.CanonicalizeInternalNames(optimizer.Qualify(e, opts))
	})
}

// TestCanonicalizeInternalNamesContract ports the behavioral half of the upstream test:
// the data contract (base tables, base columns, top-level aliases) is preserved while
// internal handles (table aliases, CTE/subquery names, inner aliases) are canonicalized.
func TestCanonicalizeInternalNamesContract(t *testing.T) {
	canon := func(sql, dialect string, sch any, defaultSchema, catalog any) string {
		t.Helper()
		return canonicalSQL(t, qualifyThenCanonicalize(t, sql, dialect, sch, defaultSchema, catalog), dialect)
	}

	// Physical table identity is part of the data contract.
	canonA := canon("SELECT id, name FROM cat.db.users WHERE id > 5", "", schema.M("cat", schema.M("db", schema.M("users", schema.M("id", "INT", "name", "TEXT")))), "", "")
	canonDiffTable := canon("SELECT id, name FROM cat.db.employees WHERE id > 5", "", schema.M("cat", schema.M("db", schema.M("employees", schema.M("id", "INT", "name", "TEXT")))), "", "")
	if canonA == canonDiffTable {
		t.Errorf("different base tables must canonicalize differently: %q", canonA)
	}

	// Renaming a base-table column is likewise a data-contract change.
	canonRename := canon("SELECT emp_id, full_name FROM cat.db.users WHERE emp_id > 5", "", schema.M("cat", schema.M("db", schema.M("users", schema.M("emp_id", "INT", "full_name", "TEXT")))), "", "")
	if canonA == canonRename {
		t.Errorf("different base columns must canonicalize differently: %q", canonA)
	}

	// A user-chosen table alias is an internal handle.
	aliasA := canon("SELECT foo.id FROM users AS foo", "", schema.M("users", schema.M("id", "INT")), "", "")
	aliasB := canon("SELECT bar.id FROM users AS bar", "", schema.M("users", schema.M("id", "INT")), "", "")
	if aliasA != aliasB {
		t.Errorf("table alias must not change the canonical form: %q vs %q", aliasA, aliasB)
	}

	// Unreferenced source aliases are still internal handles.
	unrefA := canon("SELECT 1 FROM users AS foo CROSS JOIN logs AS bar", "", schema.M("users", schema.M("id", "INT"), "logs", schema.M("id", "INT")), "", "")
	unrefB := canon("SELECT 1 FROM users AS baz CROSS JOIN logs AS qux", "", schema.M("users", schema.M("id", "INT"), "logs", schema.M("id", "INT")), "", "")
	if unrefA != unrefB {
		t.Errorf("unreferenced aliases must not change the canonical form: %q vs %q", unrefA, unrefB)
	}

	existsA := canon("SELECT 1 FROM t WHERE EXISTS(SELECT 1 FROM s AS abc)", "", schema.M("t", schema.M("id", "INT"), "s", schema.M("id", "INT")), "", "")
	existsB := canon("SELECT 1 FROM t WHERE EXISTS(SELECT 1 FROM s AS xyz)", "", schema.M("t", schema.M("id", "INT"), "s", schema.M("id", "INT")), "", "")
	if existsA != existsB {
		t.Errorf("EXISTS source alias must not change the canonical form: %q vs %q", existsA, existsB)
	}

	// Physical table identity is preserved for real tables; base columns and top-level
	// output aliases are preserved.
	if got, want := canon("SELECT a FROM x", "", schema.M("x", schema.M("a", "INT")), "mydb", "cat"), `SELECT "_t0"."a" AS "a" FROM "cat"."mydb"."x" AS "_t0"`; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// Top-level output alias is part of the contract.
	named := canon("SELECT a AS alpha FROM x", "", schema.M("x", schema.M("a", "INT")), "", "")
	renamed := canon("SELECT a AS beta FROM x", "", schema.M("x", schema.M("a", "INT")), "", "")
	if named == renamed {
		t.Errorf("top-level alias must change the canonical form: %q", named)
	}

	// Internal alias inside a CTE is self-consistent.
	innerA := canon("WITH t AS (SELECT a AS foo FROM x) SELECT foo AS result FROM t", "", schema.M("x", schema.M("a", "INT")), "", "")
	innerB := canon("WITH t AS (SELECT a AS bar FROM x) SELECT bar AS result FROM t", "", schema.M("x", schema.M("a", "INT")), "", "")
	if innerA != innerB {
		t.Errorf("CTE-internal alias must not change the canonical form: %q vs %q", innerA, innerB)
	}

	// Changing which base-table column a CTE reads is a real change.
	cteColA := canon("WITH t AS (SELECT a AS x FROM src) SELECT x FROM t", "", schema.M("src", schema.M("a", "INT", "b", "INT")), "", "")
	cteColB := canon("WITH t AS (SELECT b AS x FROM src) SELECT x FROM t", "", schema.M("src", schema.M("a", "INT", "b", "INT")), "", "")
	if cteColA == cteColB {
		t.Errorf("different CTE base columns must canonicalize differently: %q", cteColA)
	}

	// For parsed SQL, VALUES operands are wrapped in selects and canonicalized.
	if got, want := canon("VALUES (1) UNION ALL SELECT 2", "", nil, "", ""), `SELECT "_t0"."_c0" AS "_col_0" FROM (VALUES (1)) AS "_t0"("_c0") UNION ALL SELECT 2 AS "_c1"`; got != want {
		t.Errorf("VALUES union: got %q, want %q", got, want)
	}

	// UNION BY NAME: different column-name sets must produce different canonical forms.
	ux := schema.M("x", schema.M("a", "INT"), "y", schema.M("a", "INT", "b", "INT"))
	match := canon("SELECT a FROM x UNION BY NAME SELECT a FROM y", "duckdb", ux, "", "")
	diff := canon("SELECT a FROM x UNION BY NAME SELECT b FROM y", "duckdb", ux, "", "")
	if match == diff {
		t.Errorf("UNION BY NAME with different names must canonicalize differently: %q", match)
	}

	// UNION BY NAME with a nested rhs: swapping the source column must be caught.
	nested := schema.M("x", schema.M("a", "INT"), "y", schema.M("b", "INT"), "z", schema.M("c", "INT", "d", "INT"))
	nestedA := canon("SELECT a + 1 AS shared FROM x UNION BY NAME (SELECT b AS shared FROM y UNION BY NAME SELECT c AS shared FROM z)", "duckdb", nested, "", "")
	nestedB := canon("SELECT a + 1 AS shared FROM x UNION BY NAME (SELECT b AS shared FROM y UNION BY NAME SELECT d AS shared FROM z)", "duckdb", nested, "", "")
	if nestedA == nestedB {
		t.Errorf("nested UNION BY NAME column swap must canonicalize differently: %q", nestedA)
	}

	// Case-folding: in case-insensitive dialects unquoted and quoted refer to the same column.
	pgSchema := schema.M("x", schema.M("a", "INT", "b", "INT"))
	pgA := canon("SELECT a FROM x", "postgres", pgSchema, "", "")
	pgQA := canon(`SELECT "a" FROM x`, "postgres", pgSchema, "", "")
	if pgA != pgQA {
		t.Errorf("postgres unquoted/quoted must canonicalize the same: %q vs %q", pgA, pgQA)
	}

	// Snowflake upper-folds unquoted names, so `a` and `"a"` are different columns.
	sfSchema := schema.M("X", schema.M("A", "INT", `"a"`, "INT"))
	if got, want := canon(`SELECT a, "a" FROM x`, "snowflake", sfSchema, "", ""), `SELECT "_t0"."A" AS "A", "_t0"."a" AS "a" FROM "_t0" AS "_t0"`; got != want {
		t.Errorf("snowflake case folding: got %q, want %q", got, want)
	}
	sfSchema2 := schema.M("X", schema.M("A", "INT"))
	if got, want := canon(`SELECT A, "A" FROM x`, "snowflake", sfSchema2, "", ""), `SELECT "_t0"."A" AS "A", "_t0"."A" AS "A" FROM "_t0" AS "_t0"`; got != want {
		t.Errorf("snowflake same-column coalescing: got %q, want %q", got, want)
	}

	// Multiple references to the same CTE source get distinct per-reference aliases.
	cteSelfJoin := schema.M("src", schema.M("foo", "INT", "bar", "INT"))
	selfJoin := qualifyThenCanonicalize(t, "WITH t AS (SELECT * FROM src) SELECT x.foo AS l, y.bar AS r FROM t AS x JOIN t AS y ON x.foo = y.foo", "", cteSelfJoin, "", "")
	if got, want := canonicalSQL(t, selfJoin, ""), `WITH "_t1" AS (SELECT "_t0"."foo" AS "_c0", "_t0"."bar" AS "_c1" FROM "_t0" AS "_t0") SELECT "_t2"."_c0" AS "l", "_t3"."_c1" AS "r" FROM "_t1" AS "_t2" JOIN "_t1" AS "_t3" ON "_t2"."_c0" = "_t3"."_c0"`; got != want {
		t.Errorf("self-join: got %q, want %q", got, want)
	}

	// Three references to the same CTE: each gets its own alias, and the rewired AST keeps
	// three distinct sources.
	triple := qualifyThenCanonicalize(t, "WITH t AS (SELECT * FROM src) SELECT a.foo, b.foo, c.foo FROM t AS a JOIN t AS b ON a.foo = b.foo JOIN t AS c ON b.foo = c.foo", "", cteSelfJoin, "", "")
	tripleScope := optimizer.BuildScope(triple)
	if tripleScope == nil {
		t.Fatal("BuildScope returned nil for the canonical triple self-join")
	}
	outerTableAliases := map[string]bool{}
	for _, table := range tripleScope.Tables() {
		outerTableAliases[table.Alias()] = true
	}
	if len(outerTableAliases) != 3 {
		t.Errorf("triple join should expose 3 distinct aliases, got %v", outerTableAliases)
	}
	if got := len(tripleScope.SelectedSources()); got != 3 {
		t.Errorf("selected sources = %d, want 3", got)
	}

	// UNNEST whose element type is unresolved after qualify must not crash, and DuckDB
	// renders the comma UNNEST as JOIN ... ON TRUE (generators/duckdb.py:3145-3160).
	t.Run("duckdb unnest", func(t *testing.T) {
		unnest := canon("SELECT v FROM t, UNNEST(t.arr) AS v(v)", "duckdb", schema.M("t", schema.M("arr", "ARRAY<INT>")), "", "")
		if got, want := unnest, `SELECT "_t1"."_c0" AS "v" FROM "_t0" AS "_t0" JOIN UNNEST("_t0"."arr") AS "_t1"("_c0") ON TRUE`; got != want {
			t.Errorf("unnest: got %q, want %q", got, want)
		}
	})
}
