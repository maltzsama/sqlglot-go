package optimizer_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/dialects"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
	"github.com/maltzsama/sqlglot-go/optimizer"
	"github.com/maltzsama/sqlglot-go/schema"
)

// annotateGapsPath lists the annotate fixture cases that don't pass yet, mostly expressions
// whose upstream class has no Kind in this port. The fixture tests fail on any failure missing
// from it, and on any listed case that now passes, so the list only shrinks. Regenerate with
// ANNOTATE_GAPS_UPDATE=1.
const annotateGapsPath = "testdata/annotate_gaps.txt"

type annotateCase struct {
	id, dialect, sql, expected string
	run                        func() (exp.Expression, error)
}

func typeSQL(t exp.Expression, dialect string) (string, error) {
	if t == nil {
		return "<nil>", nil
	}
	return sqlglot.Generate(t, dialect, generator.Options{})
}

// checkAnnotateCases runs the cases, comparing result.type.sql(dialect) with
// DataType.build(expected, dialect).sql(dialect) as upstream does, against the gaps file.
func checkAnnotateCases(t *testing.T, file string, cases []annotateCase) {
	t.Helper()
	data, _ := os.ReadFile(annotateGapsPath)
	gaps := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			gaps[line] = true
		}
	}

	var failures []string
	passed := 0
	for _, c := range cases {
		key := file + "|" + c.dialect + "|" + c.id
		got, want, err := func() (got, want string, err error) {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("panic: %v", r)
				}
			}()
			result, err := c.run()
			if err != nil {
				return "", "", err
			}
			expected, err := exp.DataTypeBuild(c.expected, c.dialect, false, false, nil)
			if err != nil {
				return "", "", err
			}
			if got, err = typeSQL(result.Type(), c.dialect); err != nil {
				return "", "", err
			}
			want, err = typeSQL(expected, c.dialect)
			return got, want, err
		}()
		ok := err == nil && got == want
		if ok {
			passed++
		} else {
			failures = append(failures, key)
		}
		switch {
		case !ok && !gaps[key]:
			t.Errorf("%s: %q [%s] annotated %q, want %q (err %v)", key, c.sql, c.dialect, got, want, err)
		case ok && gaps[key]:
			t.Errorf("%s: %q [%s] passes now; remove it from %s", key, c.sql, c.dialect, annotateGapsPath)
		}
	}
	t.Logf("%s: %d/%d cases pass", file, passed, len(cases))

	if os.Getenv("ANNOTATE_GAPS_UPDATE") == "1" {
		for key := range gaps {
			if !strings.HasPrefix(key, file+"|") {
				failures = append(failures, key)
			}
		}
		sort.Strings(failures)
		header := "# Annotate fixture cases that don't pass yet: file|dialect|case. See optimizer/annotate_types_test.go.\n"
		if err := os.WriteFile(filepath.Clean(annotateGapsPath), []byte(header+strings.Join(failures, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// TestAnnotateTypesFixtures ports test_annotate_types (tests/test_optimizer.py:2093-2109).
func TestAnnotateTypesFixtures(t *testing.T) {
	var cases []annotateCase
	for i, pair := range loadSQLFixturePairs(t, "annotate_types.sql") {
		dialect := pair.Meta["dialect"]
		if _, err := dialects.GetOrRaise(dialect); err != nil {
			continue
		}
		sql := pair.SQL
		cases = append(cases, annotateCase{
			id: fmt.Sprint(i + 1), dialect: dialect, sql: sql, expected: pair.Expected,
			run: func() (exp.Expression, error) {
				e, err := sqlglot.ParseOne(sql, dialect)
				if err != nil {
					return nil, err
				}
				return optimizer.AnnotateTypes(e, nil, dialect), nil
			},
		})
	}
	checkAnnotateCases(t, "annotate_types.sql", cases)
}

// TestAnnotateFunctionsFixtures ports test_annotate_funcs (tests/test_optimizer.py:2151-2188):
// each case is `SELECT <sql> FROM tbl`, and the first projection's type is checked.
func TestAnnotateFunctionsFixtures(t *testing.T) {
	testSchema := schema.M("tbl", schema.M(
		"bin_col", "BINARY", "str_col", "STRING", "bignum_col", "BIGNUMERIC", "date_col", "DATE",
		"decfloat_col", "DECFLOAT", "float_col", "FLOAT", "timestamp_col", "TIMESTAMP",
		"double_col", "DOUBLE", "bigint_col", "BIGINT", "obj_col", "OBJECT", "int_col", "INT",
		"bool_col", "BOOLEAN", "bytes_col", "BYTES", "interval_col", "INTERVAL",
		"array_col", "ARRAY<STRING>",
	))
	var cases []annotateCase
	for i, pair := range loadSQLFixturePairs(t, "annotate_functions.sql") {
		for _, dialect := range strings.Split(strings.TrimSpace(pair.Meta["dialect"]), ", ") {
			dialect := strings.TrimSpace(dialect)
			if _, err := dialects.GetOrRaise(dialect); err != nil {
				continue
			}
			sql := "SELECT " + pair.SQL + " FROM tbl"
			cases = append(cases, annotateCase{
				id: fmt.Sprint(i + 1), dialect: dialect, sql: pair.SQL, expected: pair.Expected,
				run: func() (exp.Expression, error) {
					e, err := sqlglot.ParseOne(sql, dialect)
					if err != nil {
						return nil, err
					}
					return optimizer.AnnotateTypes(e, testSchema, dialect).Expressions()[0], nil
				},
			})
		}
	}
	checkAnnotateCases(t, "annotate_functions.sql", cases)
}

// TestAnnotateUntypedBinaryOperands ports test_annotate_untyped_binary_operands
// (tests/test_optimizer.py:2111-2120): operands that escaped annotation count as UNKNOWN.
func TestAnnotateUntypedBinaryOperands(t *testing.T) {
	for _, sql := range []string{"a + b", "a / b"} {
		e := optimizer.NewTypeAnnotator(mustMappingSchema(t)).Annotate(mustParse(t, sql), false)
		if !e.IsType(exp.DTypeUnknown) {
			t.Errorf("%q annotated %v, want UNKNOWN", sql, e.Type())
		}
	}
}

func mustMappingSchema(t *testing.T) *schema.MappingSchema {
	t.Helper()
	s, err := schema.NewMappingSchema(nil, "", true)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
