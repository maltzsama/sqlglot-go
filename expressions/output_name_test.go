package expressions_test

import (
	"testing"

	exp "github.com/maltzsama/sqlglot-go/expressions"
)

// TestOutputNameJSONPaths pins the JSON output_name overrides ported alongside
// canonicalize_internal_names: Bracket.output_name (core.py), JSONExtract.output_name
// (json.py:118-120), JSONExtractScalar.output_name (json.py:141-143) and
// JSONPath.output_name (query.py:1984-1987). qualify names an unaliased projection after
// its output_name, which is how `SELECT j['name']` and `SELECT JSON_EXTRACT(j, '$.field')`
// get the column names "name" and "field".
func TestOutputNameJSONPaths(t *testing.T) {
	column := exp.Column(exp.Args{"this": "j"})

	// Bracket: a single subscript names the projection after the key.
	bracket := exp.Bracket(exp.Args{
		"this":        column,
		"expressions": []exp.Expression{exp.LiteralString("name")},
	})
	if got := bracket.OutputName(); got != "name" {
		t.Errorf("Bracket.OutputName() = %q, want %q", got, "name")
	}

	// Bracket with more than one subscript has no single name.
	multi := exp.Bracket(exp.Args{
		"this":        column,
		"expressions": []exp.Expression{exp.LiteralString("a"), exp.LiteralString("b")},
	})
	if got := multi.OutputName(); got != "" {
		t.Errorf("multi-subscript Bracket.OutputName() = %q, want empty", got)
	}

	// JSONPath: the last segment's string is the name.
	path := exp.JSONPath(exp.Args{"expressions": []exp.Expression{
		exp.JSONPathRoot(exp.Args{}),
		exp.JSONPathKey(exp.Args{"this": "field"}),
	}})
	if got := path.OutputName(); got != "field" {
		t.Errorf("JSONPath.OutputName() = %q, want %q", got, "field")
	}

	// JSONExtract: names itself after the path.
	extract := exp.JSONExtract(exp.Args{"this": column, "expression": path})
	if got := extract.OutputName(); got != "field" {
		t.Errorf("JSONExtract.OutputName() = %q, want %q", got, "field")
	}

	// JSONExtract in the var-len form (extra expressions) has no single name.
	varLen := exp.JSONExtract(exp.Args{
		"this":        column,
		"expression":  path,
		"expressions": []exp.Expression{exp.LiteralString("$.a")},
	})
	if got := varLen.OutputName(); got != "" {
		t.Errorf("var-len JSONExtract.OutputName() = %q, want empty", got)
	}

	// JSONExtractScalar follows the path unconditionally.
	scalar := exp.JSONExtractScalar(exp.Args{"this": column, "expression": path})
	if got := scalar.OutputName(); got != "field" {
		t.Errorf("JSONExtractScalar.OutputName() = %q, want %q", got, "field")
	}
}
