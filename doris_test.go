package sqlglot_test

import (
	"strings"
	"testing"

	sqlglot "github.com/ridi-oss/sqlglot-go"
	"github.com/ridi-oss/sqlglot-go/dialects"
	exp "github.com/ridi-oss/sqlglot-go/expressions"
	"github.com/ridi-oss/sqlglot-go/generator"
	"github.com/ridi-oss/sqlglot-go/optimizer"
	"github.com/ridi-oss/sqlglot-go/schema"
)

// TestDorisIdentity is the same-dialect round-trip oracle for Apache Doris, ported from
// .reference/sqlglot-v30.17.0/tests/dialects/test_doris.py. Cases are already in their
// canonical Doris spelling.
func TestDorisIdentity(t *testing.T) {
	cases := []string{
		"SELECT 1",
		"SELECT DATE_ADD(x, INTERVAL '7' DAY)",
		"SELECT DATE_SUB(x, INTERVAL '3' DAY)",
		"SELECT DATE_TRUNC(event_date, 'DAY')",
		"SELECT REGEXP(abc, '%foo%')",
		"SELECT L2_DISTANCE(x, y)",
		"SELECT MONTHS_ADD(d, n)",
		"SELECT TO_DATE('2020-02-02 00:00:00')",
		"SELECT JSON_EXTRACT(CAST('{\"key\": 1}' AS JSONB), '$.key')",
		"CREATE TABLE t (c1 INT, c2 INT) UNIQUE KEY (c1) DISTRIBUTED BY HASH (c1) BUCKETS 1",
		"CREATE TABLE t (c1 INT, c2 INT) DUPLICATE KEY (c1) DISTRIBUTED BY HASH (c1)",
		"CREATE TABLE t (c1 INT, c2 INT) DISTRIBUTED BY RANDOM BUCKETS 1",
		"CREATE TABLE t (c1 INT) PARTITION BY (c1)",
		"CREATE TABLE t (c1 INT, c2 DATE) PARTITION BY RANGE (`c2`) (FROM ('2000-11-14') TO ('2021-11-14') INTERVAL 2 YEAR)",
		"CREATE TABLE t (c1 INT, c2 DATE) PARTITION BY RANGE (`c2`) (PARTITION `p1` VALUES LESS THAN ('2017-02-01'))",
		"CREATE TABLE t (c1 INT) PROPERTIES ('x'='y')",
		"CREATE TABLE t (c1 INT) COMMENT 'c'",
		"CREATE MATERIALIZED VIEW mv BUILD DEFERRED AS SELECT 1",
		"CREATE MATERIALIZED VIEW mv REFRESH AUTO ON COMMIT AS SELECT 1",
		"CREATE MATERIALIZED VIEW mv REFRESH AUTO ON SCHEDULE EVERY 5 MINUTE STARTS '2025-01-01 00:00:00' AS SELECT 1",
		"CREATE MATERIALIZED VIEW mv REFRESH COMPLETE ON MANUAL AS SELECT 1",
		"ALTER TABLE db.t1 RENAME t2",
		"INSERT OVERWRITE TABLE test PARTITION(p1, p2) VALUES (1, 2)",
		"UPDATE accounts a SET a.balance = a.balance + 100 WHERE a.account_type = 'savings'",
		"DELETE FROM sales s WHERE s.id = 1",
	}
	for _, sql := range cases {
		t.Run(sql, func(t *testing.T) {
			expr, err := sqlglot.ParseOne(sql, "doris")
			if err != nil {
				t.Fatalf("ParseOne: %v", err)
			}
			got, err := sqlglot.Generate(expr, "doris", generator.Options{})
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if got != sql {
				t.Fatalf("round-trip mismatch\n want: %s\n  got: %s", sql, got)
			}
		})
	}
}

// TestDorisASTShapes asserts the structured nodes behind the Doris DDL grammar, so a
// degradation to Command (which would still round-trip) fails.
func TestDorisASTShapes(t *testing.T) {
	tests := []struct {
		name  string
		sql   string
		kind  exp.Kind
		finds []exp.Kind
	}{
		{
			name:  "unique key and distribution",
			sql:   "CREATE TABLE t (c1 INT) UNIQUE KEY (c1) DISTRIBUTED BY HASH (c1) BUCKETS 1",
			kind:  exp.KindCreate,
			finds: []exp.Kind{exp.KindUniqueKeyProperty, exp.KindDistributedByProperty},
		},
		{
			name:  "duplicate key",
			sql:   "CREATE TABLE t (c1 INT) DUPLICATE KEY (c1)",
			kind:  exp.KindCreate,
			finds: []exp.Kind{exp.KindDuplicateKeyProperty},
		},
		{
			name:  "build property",
			sql:   "CREATE MATERIALIZED VIEW mv BUILD IMMEDIATE AS SELECT 1",
			kind:  exp.KindCreate,
			finds: []exp.Kind{exp.KindBuildProperty},
		},
		{
			name:  "refresh trigger",
			sql:   "CREATE MATERIALIZED VIEW mv REFRESH COMPLETE ON MANUAL AS SELECT 1",
			kind:  exp.KindCreate,
			finds: []exp.Kind{exp.KindRefreshTriggerProperty},
		},
		{
			name:  "dynamic partition range",
			sql:   "CREATE TABLE t (c1 INT, c2 DATE) PARTITION BY RANGE (`c2`) (FROM ('2000-11-14') TO ('2021-11-14') INTERVAL 2 YEAR)",
			kind:  exp.KindCreate,
			finds: []exp.Kind{exp.KindPartitionByRangeProperty, exp.KindPartitionByRangePropertyDynamic},
		},
		{
			name:  "static partition range",
			sql:   "CREATE TABLE t (c1 INT, c2 DATE) PARTITION BY RANGE (`c2`) (PARTITION `p1` VALUES LESS THAN ('2017-02-01'))",
			kind:  exp.KindCreate,
			finds: []exp.Kind{exp.KindPartitionByRangeProperty, exp.KindPartitionRange},
		},
		{
			name:  "bracket partition values",
			sql:   "CREATE TABLE t (c1 INT, c2 DATE) PARTITION BY RANGE (`c2`) (PARTITION `p1` VALUES [('2017-01-01'), ('2017-02-01')), PARTITION `other` VALUES LESS THAN (MAXVALUE))",
			kind:  exp.KindCreate,
			finds: []exp.Kind{exp.KindPartitionRange},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := sqlglot.ParseOne(tt.sql, "doris")
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

// TestDorisDialectFlags covers the MySQL-derived flags Doris overrides.
func TestDorisDialectFlags(t *testing.T) {
	d, err := dialects.GetOrRaise("doris")
	if err != nil {
		t.Fatalf("GetOrRaise: %v", err)
	}
	if d.VarcharRequiresSize {
		t.Fatal("VarcharRequiresSize should be false")
	}
	if d.SupportsAlterColumnNullability {
		t.Fatal("SupportsAlterColumnNullability should be false")
	}
	if d.WithPropertiesPrefix != "PROPERTIES" {
		t.Fatalf("WithPropertiesPrefix = %q, want PROPERTIES", d.WithPropertiesPrefix)
	}
	if d.RenameTableWithDB {
		t.Fatal("RenameTableWithDB should be false")
	}
	if !d.IsReservedKeyword("select") {
		t.Fatal("select should be a reserved keyword")
	}
	if !d.IsReservedKeyword("doris_internal_table_id") {
		t.Fatal("doris_internal_table_id should be a reserved keyword")
	}
}

// TestDorisQualifyAndScope checks the optimizer's qualify pass on a Doris query.
func TestDorisQualifyAndScope(t *testing.T) {
	d := dialects.Doris()
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
	out, err := sqlglot.Generate(qualified, "doris", generator.Options{})
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
