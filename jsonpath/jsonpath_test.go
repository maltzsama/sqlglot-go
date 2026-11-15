package jsonpath_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
	"github.com/maltzsama/sqlglot-go/jsonpath"
)

// TestJSONPath ports test_jsonpath (tests/test_jsonpath.py:12-28): the tokenizer and parser
// produce the canonical AST for a path with wildcards, subscripts, unions, a filter, slices
// and a script.
func TestJSONPath(t *testing.T) {
	expected := exp.JSONPath(exp.Args{"expressions": []exp.Expression{
		exp.JSONPathRoot(exp.Args{}),
		exp.JSONPathKey(exp.Args{"this": exp.JSONPathWildcard(exp.Args{})}),
		exp.JSONPathKey(exp.Args{"this": "a"}),
		exp.JSONPathSubscript(exp.Args{"this": 0}),
		exp.JSONPathKey(exp.Args{"this": "x"}),
		exp.JSONPathUnion(exp.Args{"expressions": []any{exp.JSONPathWildcard(exp.Args{}), "y", 1}}),
		exp.JSONPathKey(exp.Args{"this": "z"}),
		exp.JSONPathSelector(exp.Args{"this": exp.JSONPathFilter(exp.Args{"this": "(@.a == 'b'), 1:"})}),
		exp.JSONPathSubscript(exp.Args{"this": exp.JSONPathSlice(exp.Args{"start": 1, "end": 5, "step": nil})}),
		exp.JSONPathUnion(exp.Args{"expressions": []any{1, exp.JSONPathFilter(exp.Args{"this": "@.a"})}}),
		exp.JSONPathSelector(exp.Args{"this": exp.JSONPathScript(exp.Args{"this": "@.x)"})}),
	}})

	got, err := jsonpath.Parse("$.*.a[0]['x'][*, 'y', 1].z[?(@.a == 'b'), 1:][1:5][1,?@.a][(@.x)]", jsonpath.Config{})
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.ToS() != expected.ToS() {
		t.Fatalf("got:\n%s\nwant:\n%s", got.ToS(), expected.ToS())
	}
}

// TestUnclosedFilterRaisesParseError ports test_unclosed_filter_raises_parse_error
// (tests/test_jsonpath.py:30-37): a filter/script that runs off the end without a closing
// bracket is a ParseError.
func TestUnclosedFilterRaisesParseError(t *testing.T) {
	for _, selector := range []string{"$[?(@.x", "$[(@.x", "$[?(1"} {
		if _, err := jsonpath.Parse(selector, jsonpath.Config{}); err == nil {
			t.Errorf("%q: expected a parse error, got none", selector)
		}
	}
}

// TestIdentity ports test_identity (tests/test_jsonpath.py:39-47): the parsed path round-trips
// through the generator inside a quoted string literal.
func TestIdentity(t *testing.T) {
	cases := map[string]string{
		"$.select":          "$.select",
		"$[(@.length-1)]":   "$[(@.length-1)]",
		"$[((@.length-1))]": "$[((@.length-1))]",
	}
	for selector, want := range cases {
		path, err := jsonpath.Parse(selector, jsonpath.Config{})
		if err != nil {
			t.Fatalf("Parse(%q): %v", selector, err)
		}
		got, err := sqlglot.Generate(path, "", generator.Options{})
		if err != nil {
			t.Fatalf("Generate(%q): %v", selector, err)
		}
		if got != "'"+want+"'" {
			t.Errorf("%q -> %q, want %q", selector, got, "'"+want+"'")
		}
	}
}

// TestUnionPreservesFalseyMembers ports test_union_preserves_falsey_members
// (tests/test_jsonpath.py:49-62): 0 and "" survive in a union, while a single index is a
// subscript.
func TestUnionPreservesFalseyMembers(t *testing.T) {
	cases := []struct {
		selector string
		want     string
	}{
		{"$[1,0]", exp.JSONPathUnion(exp.Args{"expressions": []any{1, 0}}).ToS()},
		{`$["a",""]`, exp.JSONPathUnion(exp.Args{"expressions": []any{"a", ""}}).ToS()},
		{`$["",1]`, exp.JSONPathUnion(exp.Args{"expressions": []any{"", 1}}).ToS()},
	}
	for _, c := range cases {
		path, err := jsonpath.Parse(c.selector, jsonpath.Config{})
		if err != nil {
			t.Fatalf("Parse(%q): %v", c.selector, err)
		}
		parts := path.Expressions()
		if len(parts) == 0 {
			t.Fatalf("%q: empty JSONPath", c.selector)
		}
		if got := parts[len(parts)-1].ToS(); got != c.want {
			t.Errorf("%q last part = %s, want %s", c.selector, got, c.want)
		}
	}

	path, err := jsonpath.Parse("$[0]", jsonpath.Config{})
	if err != nil {
		t.Fatalf("Parse($[0]): %v", err)
	}
	parts := path.Expressions()
	if got := parts[len(parts)-1].ToS(); got != exp.JSONPathSubscript(exp.Args{"this": 0}).ToS() {
		t.Errorf("$[0] last part = %s, want a JSONPathSubscript", got)
	}
}
