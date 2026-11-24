package generator_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
)

// Expected output matches pinned v30.17.0 generators/duckdb.py and generator.py.
func TestDuckDBPreservesTextTypeParameters(t *testing.T) {
	for _, typeName := range []string{
		"CHARACTER VARYING", "VARCHAR", "NVARCHAR", "NCHAR", "CHAR", "TEXT",
	} {
		t.Run(typeName, func(t *testing.T) {
			sql := "CAST(a AS " + typeName + "(3))"
			expression, err := sqlglot.ParseOne(sql, "")
			if err != nil {
				t.Fatalf("ParseOne(%q): %v", sql, err)
			}
			before := expression.ToS()
			got, err := sqlglot.Generate(expression, "duckdb", generator.Options{})
			if err != nil {
				t.Fatalf("Generate(%q, duckdb): %v", sql, err)
			}
			if want := "CAST(a AS TEXT(3))"; got != want {
				t.Errorf("Generate(%q, duckdb) = %q, want %q", sql, got, want)
			}
			if after := expression.ToS(); after != before {
				t.Errorf("Generate(%q, duckdb) mutated input AST: before %q, after %q", sql, before, after)
			}
		})
	}
}

func TestDuckDBParserDropsTextTypeParameters(t *testing.T) {
	expression, err := sqlglot.ParseOne("CAST(x AS VARCHAR(5))", "duckdb")
	if err != nil {
		t.Fatalf("ParseOne(CAST(x AS VARCHAR(5)), duckdb): %v", err)
	}
	dtype, ok := expression.Arg("to").(exp.Expression)
	if !ok || dtype.Arg("this") != exp.DTypeText || len(dtype.Expressions()) != 0 {
		t.Fatalf("DuckDB parsed VARCHAR(5) type = %v, want TEXT with no parameters", expression.Arg("to"))
	}
	got, err := sqlglot.Generate(expression, "duckdb", generator.Options{})
	if err != nil {
		t.Fatalf("Generate(CAST(x AS VARCHAR(5)), duckdb): %v", err)
	}
	if want := "CAST(x AS TEXT)"; got != want {
		t.Errorf("Generate(CAST(x AS VARCHAR(5)), duckdb) = %q, want %q", got, want)
	}
}

func TestDuckDBPreservesArraySizeDimension(t *testing.T) {
	for _, tc := range []struct {
		read      string
		dimension string
	}{
		{read: "", dimension: "1"},
		{read: "duckdb", dimension: "2"},
	} {
		t.Run("read="+tc.read+"/dimension="+tc.dimension, func(t *testing.T) {
			sql := "SELECT ARRAY_LENGTH([0], " + tc.dimension + ") AS x"
			expression, err := sqlglot.ParseOne(sql, tc.read)
			if err != nil {
				t.Fatalf("ParseOne(%q, %q): %v", sql, tc.read, err)
			}
			got, err := sqlglot.Generate(expression, "duckdb", generator.Options{})
			if err != nil {
				t.Fatalf("Generate(%q, duckdb): %v", sql, err)
			}
			if got != sql {
				t.Errorf("Generate(%q, duckdb) = %q, want %q", sql, got, sql)
			}
		})
	}
}
