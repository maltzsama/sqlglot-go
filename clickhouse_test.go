package sqlglot_test

import (
	"strings"
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/dialects"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
	"github.com/maltzsama/sqlglot-go/optimizer"
	"github.com/maltzsama/sqlglot-go/schema"
)

// TestClickhouseIdentity is the same-dialect round-trip oracle for ClickHouse, ported from
// .reference/sqlglot-v30.17.0/tests/dialects/test_clickhouse.py. Cases are already in their
// canonical ClickHouse spelling; parse -> generate must be an identity.
func TestClickhouseIdentity(t *testing.T) {
	cases := []string{
		"SELECT 1",
		"SELECT * FROM t FINAL",
		"SELECT a FROM t PREWHERE b > 0 WHERE c < 1",
		"SELECT a FROM t SETTINGS x = 1, y = 'z' FORMAT JSON",
		"SELECT * FROM t GLOBAL LEFT ANY JOIN u ON t.a = u.a",
		"SELECT * FROM t ARRAY JOIN arr1, t.arr2 AS foo, ['a', 'b'] AS elem",
		"SELECT uniq(x), countIf(y), maxMap(k, v), groupUniqArray(2)(a) FROM t",
		"SELECT json.a.:Int64, json.a.b.:\"Array(JSON)\".c",
		"SELECT {abc: UInt32}, {d: Map(String, Array(UInt8))}",
		"SELECT ['a', 'b'], []",
		"SELECT col.^nested, t.col2.^nested.twice FROM t",
		"SELECT * APPLY(sum), COLUMNS('col') APPLY(sum) APPLY(avg) FROM t",
		"SELECT dateTrunc('DAY', ts), dateTrunc('DAY', ts) FROM t",
		"SELECT DATE_ADD(DAY, 1, x), DATE_DIFF(DAY, a, b)",
		"CREATE TABLE t (a UInt32, CONSTRAINT a_constraint CHECK (a < 10)) ENGINE=MergeTree ORDER BY a",
		"CREATE TABLE t (x UInt32 TTL now() + INTERVAL '1' MONTH) ENGINE=MergeTree ORDER BY tuple()",
		"ALTER TABLE t ALTER COLUMN c TYPE Int64",
		"ALTER TABLE t ON CLUSTER c ADD COLUMN x Int32",
		"DROP TABLE test ON CLUSTER test_cluster",
		"DELETE FROM tbl ON CLUSTER test_cluster WHERE x = 1",
		"DETACH TABLE t PERMANENTLY SYNC",
	}
	for _, sql := range cases {
		t.Run(sql, func(t *testing.T) {
			expr, err := sqlglot.ParseOne(sql, "clickhouse")
			if err != nil {
				t.Fatalf("ParseOne: %v", err)
			}
			got, err := sqlglot.Generate(expr, "clickhouse", generator.Options{})
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if got != sql {
				t.Fatalf("round-trip mismatch\n want: %s\n  got: %s", sql, got)
			}
		})
	}
}

// TestClickhouseASTShapes asserts the structured nodes behind the ClickHouse grammar, so a
// degradation to raw-text Command (which would still round-trip) fails the test.
func TestClickhouseASTShapes(t *testing.T) {
	tests := []struct {
		name  string
		sql   string
		kind  exp.Kind
		finds []exp.Kind
	}{
		{
			name:  "nested json select",
			sql:   "SELECT col.^nested FROM t",
			kind:  exp.KindSelect,
			finds: []exp.Kind{exp.KindNestedJSONSelect},
		},
		{
			name:  "json cast shorthand",
			sql:   "SELECT json.a.b[].c",
			kind:  exp.KindSelect,
			finds: []exp.Kind{exp.KindJSONCast},
		},
		{
			name:  "brace map literal",
			sql:   "SELECT {'key1': 1, 'key2': 2}",
			kind:  exp.KindSelect,
			finds: []exp.Kind{exp.KindVarMap},
		},
		{
			name:  "query parameter",
			sql:   "SELECT {abc: UInt32}",
			kind:  exp.KindSelect,
			finds: []exp.Kind{exp.KindPlaceholder},
		},
		{
			name:  "detach with cluster",
			sql:   "DETACH TABLE t ON CLUSTER c PERMANENTLY SYNC",
			kind:  exp.KindDetach,
			finds: []exp.Kind{exp.KindOnCluster},
		},
		{
			name:  "create table ttl and settings",
			sql:   "CREATE TABLE t (a UInt32 TTL now() + INTERVAL '1' MONTH) ENGINE=MergeTree ORDER BY a SETTINGS x = 1",
			kind:  exp.KindCreate,
			finds: []exp.Kind{exp.KindMergeTreeTTL, exp.KindSettingsProperty, exp.KindOrder},
		},
		{
			name:  "create dictionary",
			sql:   "CREATE DICTIONARY d (key UInt64) PRIMARY KEY (key) SOURCE(CLICKHOUSE(TABLE 'x')) LIFETIME(MIN 1 MAX 10) LAYOUT(FLAT())",
			kind:  exp.KindCreate,
			finds: []exp.Kind{exp.KindDictProperty, exp.KindDictRange, exp.KindPrimaryKey},
		},
		{
			name:  "aggregate combinator",
			sql:   "SELECT sumMergeIfState(x) FROM t",
			kind:  exp.KindSelect,
			finds: []exp.Kind{exp.KindCombinedAggFunc},
		},
		{
			name:  "parameterized aggregate",
			sql:   "SELECT groupUniqArray(2)(x) FROM t",
			kind:  exp.KindSelect,
			finds: []exp.Kind{exp.KindParameterizedAgg},
		},
		{
			name:  "quantile function",
			sql:   "SELECT quantile(0.5)(x) FROM t",
			kind:  exp.KindSelect,
			finds: []exp.Kind{exp.KindQuantile},
		},
		{
			name:  "prewhere and final",
			sql:   "SELECT * FROM t FINAL PREWHERE a > 0",
			kind:  exp.KindSelect,
			finds: []exp.Kind{exp.KindFinal, exp.KindPreWhere},
		},
		{
			name:  "apply and columns",
			sql:   "SELECT COLUMNS('a') APPLY(sum) FROM t",
			kind:  exp.KindSelect,
			finds: []exp.Kind{exp.KindColumns, exp.KindApply},
		},
		{
			name:  "with fill",
			sql:   "SELECT a FROM t ORDER BY a WITH FILL FROM 0 TO 5 STEP 1",
			kind:  exp.KindSelect,
			finds: []exp.Kind{exp.KindWithFill},
		},
		{
			name:  "array join converts to column",
			sql:   "SELECT * FROM t ARRAY JOIN arr",
			kind:  exp.KindSelect,
			finds: []exp.Kind{exp.KindJoin, exp.KindColumn},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := sqlglot.ParseOne(tt.sql, "clickhouse")
			if err != nil {
				t.Fatalf("ParseOne: %v", err)
			}
			if expr.Kind() != tt.kind {
				t.Fatalf("root kind = %s, want %s:\n%s", exp.ClassName(expr.Kind()), exp.ClassName(tt.kind), expr.ToS())
			}
			if expr.Find(exp.KindCommand) != nil {
				t.Fatalf("statement degraded to Command:\n%s", expr.ToS())
			}
			for _, k := range tt.finds {
				if expr.Find(k) == nil {
					t.Fatalf("tree is missing %s:\n%s", exp.ClassName(k), expr.ToS())
				}
			}
		})
	}
}

// TestClickhouseDialectFlags covers the parse-time flags that change name resolution and
// identifier quoting for ClickHouse.
func TestClickhouseDialectFlags(t *testing.T) {
	d, err := dialects.GetOrRaise("clickhouse")
	if err != nil {
		t.Fatalf("GetOrRaise: %v", err)
	}
	if d.NormalizationStrategy != dialects.CaseSensitive {
		t.Fatalf("normalization = %v, want CaseSensitive", d.NormalizationStrategy)
	}
	if d.IndexOffset != 1 {
		t.Fatalf("IndexOffset = %d, want 1", d.IndexOffset)
	}
	if d.StructDelimiter != [2]string{"(", ")"} {
		t.Fatalf("StructDelimiter = %v, want ()", d.StructDelimiter)
	}
	if d.AlterSetType != "TYPE" {
		t.Fatalf("AlterSetType = %q, want TYPE", d.AlterSetType)
	}

	// Parameterized function spelling survives (NORMALIZE_FUNCTIONS=False).
	expr, err := sqlglot.ParseOne("SELECT toTypeName(x), lower(x)", "clickhouse")
	if err != nil {
		t.Fatalf("ParseOne: %v", err)
	}
	out, err := sqlglot.Generate(expr, "clickhouse", generator.Options{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out != "SELECT toTypeName(x), lower(x)" {
		t.Fatalf("function casing changed: %q", out)
	}
}

// TestClickhouseQualifyAndScope checks that the optimizer's qualify pass handles a
// ClickHouse query end to end.
func TestClickhouseQualifyAndScope(t *testing.T) {
	d := dialects.Clickhouse()
	mapping, err := schema.NewMappingSchema(schema.M(
		"db", schema.M("t", schema.M("a", "UInt32", "b", "String")),
	), d, true)
	if err != nil {
		t.Fatalf("NewMappingSchema: %v", err)
	}
	expr, err := sqlglot.ParseOne("SELECT a, b FROM t WHERE a > 0", d)
	if err != nil {
		t.Fatalf("ParseOne: %v", err)
	}
	opts := optimizer.DefaultQualifyOpts()
	opts.Dialect = d
	opts.Schema = mapping
	opts.DefaultSchema = "db"
	opts.QuoteIdentifiers = false
	opts.Identify = false
	qualified := optimizer.Qualify(expr, opts)
	out, err := sqlglot.Generate(qualified, "clickhouse", generator.Options{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, want := range []string{"db.t", "t.a", "t.b"} {
		if !strings.Contains(out, want) {
			t.Fatalf("qualified SQL %q missing %q", out, want)
		}
	}
	scopes := optimizer.TraverseScope(qualified)
	if len(scopes) != 1 {
		t.Fatalf("scopes = %d, want 1", len(scopes))
	}
	if _, ok := scopes[0].Sources["t"]; !ok {
		t.Fatalf("scope sources = %v, want t", scopes[0].Sources)
	}
}
