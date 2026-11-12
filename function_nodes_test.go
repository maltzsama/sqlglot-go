package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestLogicalAggregateRenames covers the LogicalAnd generator renames ported from
// generators/postgres.py and generators/mysql.py.
func TestLogicalAggregateRenames(t *testing.T) {
	assertRoundTrip(t, "postgres", "SELECT BOOL_AND(x) FROM t", "SELECT BOOL_AND(x) FROM t")
	assertRoundTrip(t, "mysql", "SELECT BOOL_AND(x) FROM t", "SELECT MIN(x) FROM t")
}

// TestApproxQuantilesNode ensures APPROX_QUANTILES parses to the structured node rather than
// Anonymous, so annotate_types can type it.
func TestApproxQuantilesNode(t *testing.T) {
	assertRoundTrip(t, "postgres", "SELECT APPROX_QUANTILES(x, 5) FROM t", "SELECT APPROX_QUANTILES(x, 5) FROM t")
}

// TestDuckDBArrayAggIgnoreNulls pins DuckDB's ARRAY_AGG IGNORE NULLS rewrite: the modifier
// becomes a FILTER over the non-NULL input instead of a trailing IGNORE NULLS.
func TestDuckDBArrayAggIgnoreNulls(t *testing.T) {
	assertRoundTrip(t, "duckdb", "SELECT ARRAY_AGG(x IGNORE NULLS) FROM t", "SELECT ARRAY_AGG(x) FILTER(WHERE x IS NOT NULL) FROM t")
}

// TestArrayAggNullFilterGrouping pins the ARRAY_AGG null-filter merge: an existing FILTER
// predicate is parenthesized before the null check is AND-ed in, and a compound aggregate
// input is grouped before IS NOT NULL.
func TestArrayAggNullFilterGrouping(t *testing.T) {
	col := func(name string) exp.Expression {
		return exp.Column(exp.Args{"this": exp.ToIdentifier(name)})
	}
	agg := exp.ArrayAgg(exp.Args{"this": col("x"), "nulls_excluded": true})
	cond := exp.Or(exp.Args{"this": col("a"), "expression": col("b")})
	sel := exp.Select(exp.Args{
		"expressions": []exp.Expression{
			exp.Filter(exp.Args{"this": agg, "expression": exp.Where(exp.Args{"this": cond})}),
		},
	})
	got, err := sqlglot.Generate(sel, "duckdb", generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if want := "SELECT ARRAY_AGG(x) FILTER(WHERE (a OR b) AND NOT x IS NULL)"; got != want {
		t.Fatalf("existing filter = %q, want %q", got, want)
	}

	compoundWithFilter := exp.Filter(exp.Args{
		"this": exp.ArrayAgg(exp.Args{
			"this":           exp.Add(exp.Args{"this": col("a"), "expression": col("b")}),
			"nulls_excluded": true,
		}),
		"expression": exp.Where(exp.Args{"this": cond}),
	})
	sel = exp.Select(exp.Args{"expressions": []exp.Expression{compoundWithFilter}})
	got, err = sqlglot.Generate(sel, "duckdb", generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if want := "SELECT ARRAY_AGG(a + b) FILTER(WHERE (a OR b) AND NOT (a + b) IS NULL)"; got != want {
		t.Fatalf("compound null check = %q, want %q", got, want)
	}

	compound := exp.ArrayAgg(exp.Args{
		"this":           exp.Add(exp.Args{"this": col("a"), "expression": col("b")}),
		"nulls_excluded": true,
	})
	sel = exp.Select(exp.Args{"expressions": []exp.Expression{compound}})
	got, err = sqlglot.Generate(sel, "duckdb", generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if want := "SELECT ARRAY_AGG(a + b) FILTER(WHERE (a + b) IS NOT NULL)"; got != want {
		t.Fatalf("compound input = %q, want %q", got, want)
	}
}
