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

// TestStarRocksIdentity is the same-dialect round-trip oracle for StarRocks, ported from
// .reference/sqlglot-v30.17.0/tests/dialects/test_starrocks.py. Cases are already in their
// canonical StarRocks spelling.
func TestStarRocksIdentity(t *testing.T) {
	cases := []string{
		"SELECT 1",
		"SELECT ARRAY_JOIN([1, 3, 5, NULL], '_')",
		"SELECT ARRAY_AGG(a) FROM x",
		"SELECT CURRENT_VERSION()",
		"SELECT DATE_ADD(x, INTERVAL '7' DAY)",
		"SELECT DATE_SUB(x, INTERVAL '3' DAY)",
		"SELECT DATE_DIFF('DAY', a, b)",
		"CREATE TABLE t (c1 INT, c2 STRING) PRIMARY KEY (c1) DISTRIBUTED BY HASH (c1) BUCKETS 1",
		"CREATE TABLE t (c1 INT, c2 STRING) UNIQUE KEY (c1) DISTRIBUTED BY HASH (c1) PROPERTIES ('replication_num'='1')",
		"CREATE TABLE t (c1 DATE) PARTITION BY RANGE (c1) (PARTITION p1 VALUES LESS THAN ('2020-01-31'))",
		"CREATE TABLE t (c1 STRING, c2 STRING) PARTITION BY LIST (c1) (PARTITION p1 VALUES IN ('a'))",
		"CREATE MATERIALIZED VIEW mv REFRESH ASYNC AS SELECT 1",
		"CREATE MATERIALIZED VIEW mv REFRESH DEFERRED MANUAL AS SELECT 1",
		"CREATE MATERIALIZED VIEW mv REFRESH IMMEDIATE START ('2025-01-01 00:00:00') EVERY (INTERVAL 5 MINUTE) AS SELECT 1",
		"REFRESH EXTERNAL TABLE hive1",
		"REFRESH TABLE t",
		"ALTER TABLE a SWAP WITH b",
		"ANALYZE TABLE TBL(c1, c2) PROPERTIES ('prop1'=val1)",
		"SELECT * FROM t1 LEFT SEMI JOIN t2 ON t1.a = t2.a",
		"SELECT * FROM t1 LEFT ANTI JOIN t2 ON t1.a = t2.a",
	}
	for _, sql := range cases {
		t.Run(sql, func(t *testing.T) {
			expr, err := sqlglot.ParseOne(sql, "starrocks")
			if err != nil {
				t.Fatalf("ParseOne: %v", err)
			}
			got, err := sqlglot.Generate(expr, "starrocks", generator.Options{})
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if got != sql {
				t.Fatalf("round-trip mismatch\n want: %s\n  got: %s", sql, got)
			}
		})
	}
}

// TestStarRocksASTShapes asserts the structured nodes behind the StarRocks grammar and the
// generator-side preprocess transforms, so degradation to Command or a lost transform fails.
func TestStarRocksASTShapes(t *testing.T) {
	tests := []struct {
		name  string
		sql   string
		kind  exp.Kind
		finds []exp.Kind
	}{
		{
			name:  "primary key moves into schema",
			sql:   "CREATE TABLE t (c1 INT) PRIMARY KEY (c1) DISTRIBUTED BY HASH (c1)",
			kind:  exp.KindCreate,
			finds: []exp.Kind{exp.KindPrimaryKey, exp.KindDistributedByProperty},
		},
		{
			name:  "unique key and rollup",
			sql:   "CREATE TABLE t (c1 INT) UNIQUE KEY (c1) ROLLUP (r1 (c1))",
			kind:  exp.KindCreate,
			finds: []exp.Kind{exp.KindUniqueKeyProperty, exp.KindRollupProperty, exp.KindRollupIndex},
		},
		{
			name:  "refresh trigger",
			sql:   "CREATE MATERIALIZED VIEW mv REFRESH IMMEDIATE ASYNC AS SELECT 1",
			kind:  exp.KindCreate,
			finds: []exp.Kind{exp.KindRefreshTriggerProperty},
		},
		{
			name:  "partition range",
			sql:   "CREATE TABLE t (c1 DATE) PARTITION BY RANGE (c1) (PARTITION p1 VALUES LESS THAN ('2020-01-31'))",
			kind:  exp.KindCreate,
			finds: []exp.Kind{exp.KindPartitionByRangeProperty, exp.KindPartitionRange},
		},
		{
			name:  "dynamic partition",
			sql:   "CREATE TABLE t (c1 DATE) PARTITION BY RANGE (c1) (START ('2020-01-01') END ('2020-02-01') EVERY (INTERVAL 1 DAY))",
			kind:  exp.KindCreate,
			finds: []exp.Kind{exp.KindPartitionByRangePropertyDynamic},
		},
		{
			name:  "refresh statement",
			sql:   "REFRESH EXTERNAL TABLE hive1",
			kind:  exp.KindRefresh,
			finds: []exp.Kind{exp.KindTable},
		},
		{
			name:  "unnest defaults the alias",
			sql:   "SELECT * FROM UNNEST([1, 2])",
			kind:  exp.KindSelect,
			finds: []exp.Kind{exp.KindUnnest},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := sqlglot.ParseOne(tt.sql, "starrocks")
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

// TestStarRocksPreprocessTransforms covers the generator-side Select/Delete rewrites
// (generators/starrocks.py:18-40, 121-126).
func TestStarRocksPreprocessTransforms(t *testing.T) {
	t.Run("delete between expansion", func(t *testing.T) {
		expr, err := sqlglot.ParseOne("DELETE FROM t WHERE x BETWEEN 1 AND 10", "starrocks")
		if err != nil {
			t.Fatalf("ParseOne: %v", err)
		}
		got, err := sqlglot.Generate(expr, "starrocks", generator.Options{})
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		want := "DELETE FROM t WHERE x >= 1 AND x <= 10"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("distinct on elimination", func(t *testing.T) {
		expr, err := sqlglot.ParseOne("SELECT DISTINCT ON (a) a, b FROM x ORDER BY c DESC", "starrocks")
		if err != nil {
			t.Fatalf("ParseOne: %v", err)
		}
		got, err := sqlglot.Generate(expr, "starrocks", generator.Options{})
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		want := "SELECT a, b FROM (SELECT a AS a, b AS b, ROW_NUMBER() OVER (PARTITION BY a ORDER BY c DESC) AS _row_number FROM x) AS _t WHERE _row_number = 1"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("generate date array recursive cte", func(t *testing.T) {
		expr, err := sqlglot.ParseOne(
			"SELECT * FROM UNNEST(GENERATE_DATE_ARRAY(DATE '2020-01-01', DATE '2020-02-01', INTERVAL 1 WEEK)) AS _q(date_week)",
			"starrocks",
		)
		if err != nil {
			t.Fatalf("ParseOne: %v", err)
		}
		got, err := sqlglot.Generate(expr, "starrocks", generator.Options{})
		if err != nil {
			t.Fatalf("Generate: %v", err)
		}
		for _, want := range []string{
			"WITH RECURSIVE _generated_dates(date_week) AS",
			"SELECT CAST('2020-01-01' AS DATE) AS date_week UNION ALL",
			"DATE_ADD(date_week, INTERVAL 1 WEEK)",
			"FROM (SELECT date_week FROM _generated_dates) AS _generated_dates",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("generated SQL %q missing %q", got, want)
			}
		}
	})
}

// TestStarRocksDialectFlags covers the MySQL-derived flags StarRocks overrides.
func TestStarRocksDialectFlags(t *testing.T) {
	d, err := dialects.GetOrRaise("starrocks")
	if err != nil {
		t.Fatalf("GetOrRaise: %v", err)
	}
	if d.IndexOffset != 1 {
		t.Fatalf("IndexOffset = %d, want 1", d.IndexOffset)
	}
	if d.VarcharRequiresSize {
		t.Fatal("VarcharRequiresSize should be false")
	}
	if d.IsBoolAllowed {
		t.Fatal("IsBoolAllowed should be false")
	}
	if d.WithPropertiesPrefix != "PROPERTIES" {
		t.Fatalf("WithPropertiesPrefix = %q, want PROPERTIES", d.WithPropertiesPrefix)
	}
	if d.InsertOverwrite != " OVERWRITE" {
		t.Fatalf("InsertOverwrite = %q, want \" OVERWRITE\"", d.InsertOverwrite)
	}
	if d.RenameTableWithDB {
		t.Fatal("RenameTableWithDB should be false")
	}

	// IS TRUE rewrites to the bare operand when IS_BOOL_ALLOWED is false.
	expr, err := sqlglot.ParseOne("SELECT a IS TRUE FROM t", "starrocks")
	if err != nil {
		t.Fatalf("ParseOne: %v", err)
	}
	out, err := sqlglot.Generate(expr, "starrocks", generator.Options{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out != "SELECT a FROM t" {
		t.Fatalf("IS TRUE rewrote to %q, want %q", out, "SELECT a FROM t")
	}

	// The documented reserved keyword list is active.
	if !d.IsReservedKeyword("select") {
		t.Fatal("select should be a reserved keyword")
	}
}

// TestStarRocksQualifyAndScope checks the optimizer's qualify pass on a StarRocks query.
func TestStarRocksQualifyAndScope(t *testing.T) {
	d := dialects.StarRocks()
	mapping, err := schema.NewMappingSchema(schema.M(
		"db", schema.M("t", schema.M("a", "INT", "b", "STRING")),
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
	out, err := sqlglot.Generate(qualified, "starrocks", generator.Options{})
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
