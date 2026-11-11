package optimizer_test

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/optimizer"
	"github.com/maltzsama/sqlglot-go/schema"
)

// checkOptimizeFile is check_file (tests/test_optimizer.py:178-247) with optimize as the rule:
// the fixture's leave_tables_isolated, validate_qualify_columns and canonicalize_table_aliases
// metadata become optimize kwargs, on top of kwargs. Known gaps are keyed by fixture title.
// Execution against DuckDB is not ported.
func checkOptimizeFile(t *testing.T, filename string, pretty bool, gaps map[string]string, opts optimizer.OptimizeOpts) {
	t.Helper()
	for i, pair := range loadSQLFixturePairs(t, filename) {
		pairOpts := opts
		kwargs := opts.Kwargs
		if v, ok := pair.Meta["leave_tables_isolated"]; ok {
			b := stringToBool(v)
			kwargs.LeaveTablesIsolated = &b
		}
		if v, ok := pair.Meta["validate_qualify_columns"]; ok {
			b := stringToBool(v)
			kwargs.ValidateQualifyColumns = &b
		}
		if v, ok := pair.Meta["canonicalize_table_aliases"]; ok {
			b := stringToBool(v)
			kwargs.CanonicalizeTableAliases = &b
		}
		pairOpts.Kwargs = kwargs
		checkPassFixturePairWith(t, i, pair, pretty, gaps[pair.Meta["title"]], func(e exp.Expression, dialect string) exp.Expression {
			o := pairOpts
			o.Dialect = dialect
			return optimizer.Optimize(e, o)
		})
	}
}

// loadTSVSchema reads a table/column/type TSV dump of a tests/helpers.py schema dict, keeping
// its table and column order.
func loadTSVSchema(t *testing.T, filename string) *schema.Mapping {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", filename))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	out := schema.NewMapping()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		table, ok := out.Get(parts[0])
		if !ok {
			table = schema.NewMapping()
			out.Set(parts[0], table)
		}
		table.(*schema.Mapping).Set(parts[1], parts[2])
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// TestOptimize ports test_optimize (tests/test_optimizer.py:252-269).
func TestOptimize(t *testing.T) {
	identify := false
	got := mustSQL(t, optimizer.Optimize(mustParse(t, "x = 1 + 1"), optimizer.OptimizeOpts{Kwargs: optimizer.RuleArgs{Identify: &identify}}))
	if got != "x = 2" {
		t.Errorf("optimize = %q, want %q", got, "x = 2")
	}

	infer := true
	checkOptimizeFile(t, "optimizer.sql", true, optimizeGaps, optimizer.OptimizeOpts{
		Schema: schema.M(
			"x", schema.M("a", "INT", "b", "INT"),
			"y", schema.M("b", "INT", "c", "INT"),
			"z", schema.M("a", "INT", "c", "INT"),
			"u", schema.M("f", "INT", "g", "INT", "h", "TEXT"),
		),
		Kwargs: optimizer.RuleArgs{InferSchema: &infer},
	})
}

// TestTPCH ports test_tpch (tests/test_optimizer.py:1822-1823).
func TestTPCH(t *testing.T) {
	checkOptimizeFile(t, "tpc-h/tpc-h.sql", true, nil, optimizer.OptimizeOpts{Schema: loadTSVSchema(t, "tpc-h/schema.tsv")})
}

// TestTPCDS ports test_tpcds (tests/test_optimizer.py:1825-1826).
func TestTPCDS(t *testing.T) {
	checkOptimizeFile(t, "tpc-ds/tpc-ds.sql", true, nil, optimizer.OptimizeOpts{Schema: loadTSVSchema(t, "tpc-ds/schema.tsv")})
}

// optimizeGaps are optimizer.sql fixtures whose SELECT * over a PIVOT or UNPIVOT still expands
// to the pre-pivot columns (#46).
var optimizeGaps = map[string]string{
	"pivoted source with explicit selections":                                                           "star expansion through PIVOT (#46)",
	"pivoted source with explicit selections where one of them is excluded & selected at the same time": "star expansion through PIVOT (#46)",
	"pivoted source with implicit selections":                                                           "star expansion through PIVOT (#46)",
	"unpivoted table source, unpivot has column aliases":                                                "star expansion through UNPIVOT (#46)",
	// #35: merge_subqueries folds the EXPLODE_OUTER projection subquery into the outer SELECT;
	// upstream's eliminate_subqueries keeps it as a top-level CTE.
	"explode_outer": "merge_subqueries folds an explode projection instead of keeping a CTE (#35 follow-up)",
	// #35: fixtures that only run now that spark is registered.
	"selecting all columns from a pivoted source and generating spark":            "star expansion through PIVOT (#46)",
	"decorrelate subquery and transpile ArrayAny correctly when generating spark": "subquery decorrelation for the ArrayAny rewrite is not ported",
	"SELECT TRANSFORM ... Spark clause when schema is provided":                   "qualify aliases a TRANSFORM projection instead of exposing its schema columns",
	"SELECT TRANSFORM ... Spark clause when schema is not provided":               "qualify aliases a TRANSFORM projection instead of exposing its key/value columns",
	// #41: fixtures that only run now that bigquery is registered.
	"selecting all columns from a pivoted CTE source, using alias for the aggregation and generating bigquery": "star expansion through PIVOT (#46)",
	"unpivoted table source with a single value column, unpivot columns can be qualified":                      "star expansion through UNPIVOT (#46)",
	"unpivoted table source with multiple value columns":                                                       "star expansion through UNPIVOT (#46)",
	// #40: fixtures that only run now that snowflake is registered.
	"selecting all columns from a pivoted source and generating snowflake":                  "star expansion through PIVOT (#46)",
	"selecting all columns from a pivoted source, pivot has column aliases":                 "star expansion through PIVOT (#46)",
	"unpivoted table source with a single value column, unpivot columns can't be qualified": "star expansion through UNPIVOT (#46)",
	"unpivoted derived table source with a single value column":                             "star expansion through UNPIVOT (#46)",
	"flatten over object_construct":                                                         "ObjectConstruct/flatten rewriting is not ported (#40)",
	"array_agg within group over":                                                           "ArrayAgg WITHIN GROUP ordering is not ported (#40)",
}

// TestOptimizeFileSchema ports test_file_schema (tests/test_optimizer.py:1828-1835).
func TestOptimizeFileSchema(t *testing.T) {
	onQualify := func(table exp.Expression) {
		bar, err := exp.ToTable("bar", nil, true, nil)
		if err != nil {
			t.Fatal(err)
		}
		table.Replace(bar)
	}
	got := mustSQL(t, optimizer.Optimize(mustParse(t, "SELECT * FROM foo"), optimizer.OptimizeOpts{Kwargs: optimizer.RuleArgs{OnQualify: onQualify}}))
	if want := `SELECT * FROM "bar"`; got != want {
		t.Errorf("optimize = %q, want %q", got, want)
	}
}
