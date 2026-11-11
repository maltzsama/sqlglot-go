package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
)

// roundTrip parses sql under dialect and regenerates it, failing when the output differs from
// want. Shared with databricks_test.go.
func assertRoundTrip(t *testing.T, dialect, sql, want string) {
	t.Helper()
	expr, err := sqlglot.ParseOne(sql, dialect)
	if err != nil {
		t.Fatalf("%s: ParseOne(%q): %v", dialect, sql, err)
	}
	got, err := sqlglot.Generate(expr, dialect, generator.Options{})
	if err != nil {
		t.Fatalf("%s: Generate(%q): %v", dialect, sql, err)
	}
	if got != want {
		t.Fatalf("%s: %q -> %q, want %q", dialect, sql, got, want)
	}
}

// assertRoundTrips checks the same round-trip for several dialects.
func assertRoundTrips(t *testing.T, dialects []string, sql, want string) {
	t.Helper()
	for _, dialect := range dialects {
		assertRoundTrip(t, dialect, sql, want)
	}
}

// TestSparkHint ports test_hint (tests/dialects/test_spark.py:200-272): every Spark join hint
// round-trips under spark.
func TestSparkHint(t *testing.T) {
	for _, sql := range []string{
		"SELECT /*+ COALESCE(3) */ * FROM x",
		"SELECT /*+ COALESCE(3), REPARTITION(1) */ * FROM x",
		"SELECT /*+ BROADCAST(table) */ cola FROM table",
		"SELECT /*+ BROADCASTJOIN(table) */ cola FROM table",
		"SELECT /*+ MAPJOIN(table) */ cola FROM table",
		"SELECT /*+ MERGE(table) */ cola FROM table",
		"SELECT /*+ SHUFFLEMERGE(table) */ cola FROM table",
		"SELECT /*+ MERGEJOIN(table) */ cola FROM table",
		"SELECT /*+ SHUFFLE_HASH(table) */ cola FROM table",
		"SELECT /*+ SHUFFLE_REPLICATE_NL(table) */ cola FROM table",
	} {
		assertRoundTrip(t, "spark", sql, sql)
	}
}

// TestSparkNamedStruct ports test_named_struct.
func TestSparkNamedStruct(t *testing.T) {
	assertRoundTrips(t, []string{"spark", "databricks"},
		"SELECT named_struct('a', 1, 'b', 'x')", "SELECT STRUCT(1 AS a, 'x' AS b)")
}

// TestSparkBoolOr ports test_bool_or.
func TestSparkBoolOr(t *testing.T) {
	assertRoundTrip(t, "spark", "SELECT a, LOGICAL_OR(b) FROM table GROUP BY a",
		"SELECT a, BOOL_OR(b) FROM table GROUP BY a")
}

// TestSparkCurrentUser ports test_current_user.
func TestSparkCurrentUser(t *testing.T) {
	assertRoundTrips(t, []string{"spark", "spark2", "databricks"}, "CURRENT_USER", "CURRENT_USER()")
	assertRoundTrips(t, []string{"spark", "spark2", "databricks"}, "CURRENT_USER()", "CURRENT_USER()")
}

// TestSparkTransformQuery ports test_transform_query.
func TestSparkTransformQuery(t *testing.T) {
	for _, sql := range []string{
		"SELECT TRANSFORM(x) USING 'x' AS (x INT) FROM t",
		"SELECT TRANSFORM(zip_code, name, age) USING 'cat' AS (a, b, c) FROM person WHERE zip_code > 94511",
		"SELECT TRANSFORM(zip_code, name, age) USING 'cat' AS (a STRING, b STRING, c STRING) FROM person WHERE zip_code > 94511",
		"SELECT TRANSFORM(name, age) ROW FORMAT DELIMITED FIELDS TERMINATED BY ',' LINES TERMINATED BY '\\n' NULL DEFINED AS 'NULL' USING 'cat' AS (name_age STRING) ROW FORMAT DELIMITED FIELDS TERMINATED BY '@' LINES TERMINATED BY '\\n' NULL DEFINED AS 'NULL' FROM person",
		"SELECT TRANSFORM(zip_code, name, age) ROW FORMAT SERDE 'org.apache.hadoop.hive.serde2.lazy.LazySimpleSerDe' WITH SERDEPROPERTIES ('field.delim'='\\t') USING 'cat' AS (a STRING, b STRING, c STRING) ROW FORMAT SERDE 'org.apache.hadoop.hive.serde2.lazy.LazySimpleSerDe' WITH SERDEPROPERTIES ('field.delim'='\\t') FROM person WHERE zip_code > 94511",
		"SELECT TRANSFORM(zip_code, name, age) USING 'cat' FROM person WHERE zip_code > 94500",
	} {
		assertRoundTrip(t, "spark", sql, sql)
	}
}

// TestSparkInsertCTE ports test_insert_cte: the CTE is hoisted above INSERT for the whole
// Hive family.
func TestSparkInsertCTE(t *testing.T) {
	assertRoundTrips(t, []string{"spark", "spark2", "databricks", "hive"},
		"INSERT OVERWRITE TABLE table WITH cte AS (SELECT cola FROM other_table) SELECT cola FROM cte",
		"WITH cte AS (SELECT cola FROM other_table) INSERT OVERWRITE TABLE table SELECT cola FROM cte")
}

// TestSparkDeclare ports test_declare.
func TestSparkDeclare(t *testing.T) {
	cases := []struct{ sql, want string }{
		{"DECLARE VAR x INT", "DECLARE x INT"},
		{"DECLARE x INT", "DECLARE x INT"},
		{"DECLARE VARIABLE myvar INT DEFAULT 5", "DECLARE myvar INT = 5"},
		{"DECLARE x, y, z INT DEFAULT 1", "DECLARE x, y, z INT = 1"},
		{"DECLARE x INT = 5", "DECLARE x INT = 5"},
		{"DECLARE five = 5", "DECLARE five = 5"},
		{"DECLARE OR REPLACE five = 55", "DECLARE OR REPLACE five = 55"},
		{"DECLARE VARIABLE size DEFAULT 6", "DECLARE size = 6"},
		{"DECLARE some_var STRING", "DECLARE some_var STRING"},
	}
	for _, c := range cases {
		assertRoundTrip(t, "spark", c.sql, c.want)
	}
}

// TestSparkSetVariable ports test_set_variable.
func TestSparkSetVariable(t *testing.T) {
	cases := []struct{ sql, want string }{
		{"SET VAR v = 5", "SET VARIABLE v = 5"},
		{"SET VARIABLE v = 5", "SET VARIABLE v = 5"},
		{"SET VARIABLE v = (SELECT MAX(c1) FROM VALUES (1), (2) AS T(c1))", "SET VARIABLE v = (SELECT MAX(c1) FROM VALUES (1), (2) AS T(c1))"},
		{"SET VARIABLE v = DEFAULT", "SET VARIABLE v = DEFAULT"},
		{"SET VARIABLE v1 = 1, v2 = '2'", "SET VARIABLE v1 = 1, v2 = '2'"},
		{"SET VARIABLE (v1, v2) = (SELECT 1, 2)", "SET VARIABLE (v1, v2) = (SELECT 1, 2)"},
	}
	for _, c := range cases {
		assertRoundTrips(t, []string{"spark", "databricks"}, c.sql, c.want)
	}
}

// TestSparkApproxPercentile ports test_approx_percentile, including the AST assertions.
func TestSparkApproxPercentile(t *testing.T) {
	for _, c := range []struct{ sql, want string }{
		{"PERCENTILE_APPROX(DISTINCT col, 0.3)", "PERCENTILE_APPROX(DISTINCT col, 0.3)"},
		{"PERCENTILE_APPROX(DISTINCT col, 0.3, 200)", "PERCENTILE_APPROX(DISTINCT col, 0.3, 200)"},
		{"APPROX_PERCENTILE(DISTINCT col, 0.3)", "PERCENTILE_APPROX(DISTINCT col, 0.3)"},
		{"APPROX_PERCENTILE(DISTINCT col, 0.3, 200)", "PERCENTILE_APPROX(DISTINCT col, 0.3, 200)"},
	} {
		assertRoundTrips(t, []string{"spark", "databricks"}, c.sql, c.want)
	}

	for _, sql := range []string{"PERCENTILE_APPROX(DISTINCT col, 0.3)", "PERCENTILE_APPROX(DISTINCT col, 0.3, 200)"} {
		expr, err := sqlglot.ParseOne(sql, "spark")
		if err != nil {
			t.Fatal(err)
		}
		quantile := expr
		if quantile.Kind() != exp.KindApproxQuantile {
			t.Fatalf("%s: got %s, want ApproxQuantile", sql, exp.ClassName(quantile.Kind()))
		}
		if this := quantile.This(); this == nil || this.Kind() != exp.KindDistinct {
			t.Fatalf("%s: this is not DISTINCT", sql)
		}
		if q := quantile.Arg("quantile"); q == nil || q.(exp.Expression).Kind() != exp.KindLiteral {
			t.Fatalf("%s: quantile is not a Literal", sql)
		}
		if sql != "PERCENTILE_APPROX(DISTINCT col, 0.3)" {
			if a := quantile.Arg("accuracy"); a == nil || a.(exp.Expression).Kind() != exp.KindLiteral {
				t.Fatalf("%s: accuracy is not a Literal", sql)
			}
		}
	}
}

// TestSparkArrayInsert ports test_array_insert.
func TestSparkArrayInsert(t *testing.T) {
	assertRoundTrips(t, []string{"spark", "databricks"},
		"SELECT ARRAY_INSERT(ARRAY('a', 'b', 'c'), 1, 'z')", "SELECT ARRAY_INSERT(ARRAY('a', 'b', 'c'), 1, 'z')")
}

// TestSparkTryDivide ports test_try_divide.
func TestSparkTryDivide(t *testing.T) {
	assertRoundTrips(t, []string{"spark", "databricks"}, "SELECT TRY_DIVIDE(a, b)", "SELECT TRY_DIVIDE(a, b)")
}

// TestSparkStringAndBinaryString ports test_string and test_binary_string.
func TestSparkStringAndBinaryString(t *testing.T) {
	assertRoundTrips(t, []string{"hive", "spark2", "spark", "databricks"}, "STRING(a)", "CAST(a AS STRING)")
	assertRoundTrips(t, []string{"spark2", "spark", "databricks"}, "X'ab'", "X'ab'")
	assertRoundTrips(t, []string{"spark2", "spark", "databricks"}, "X''", "X''")
}

// TestSparkSchemaBindingOptions ports test_schema_binding_options.
func TestSparkSchemaBindingOptions(t *testing.T) {
	for _, schemaBinding := range []string{"BINDING", "COMPENSATION", "TYPE EVOLUTION", "EVOLUTION"} {
		sql := "CREATE VIEW emp_v WITH SCHEMA " + schemaBinding + " AS SELECT * FROM emp"
		assertRoundTrip(t, "spark", sql, sql)
	}
}

// TestSparkMinus ports test_minus.
func TestSparkMinus(t *testing.T) {
	assertRoundTrips(t, []string{"spark", "databricks"},
		"SELECT * FROM db.table1 MINUS SELECT * FROM db.table2",
		"SELECT * FROM db.table1 EXCEPT SELECT * FROM db.table2")
}

// TestSparkAnalyze ports test_analyze.
func TestSparkAnalyze(t *testing.T) {
	for _, sql := range []string{
		"ANALYZE TABLE tbl COMPUTE STATISTICS NOSCAN",
		"ANALYZE TABLE tbl COMPUTE STATISTICS FOR ALL COLUMNS",
		"ANALYZE TABLE tbl COMPUTE STATISTICS FOR COLUMNS foo, bar",
		"ANALYZE TABLE ctlg.db.tbl COMPUTE STATISTICS NOSCAN",
		"ANALYZE TABLES COMPUTE STATISTICS NOSCAN",
		"ANALYZE TABLES FROM db COMPUTE STATISTICS",
		"ANALYZE TABLES IN db COMPUTE STATISTICS",
		"ANALYZE TABLE ctlg.db.tbl PARTITION(foo = 'foo', bar = 'bar') COMPUTE STATISTICS NOSCAN",
	} {
		assertRoundTrip(t, "spark", sql, sql)
	}
}

// TestSparkStripModifiers ports test_strip_modifiers: the Hive family keeps CLUSTER BY /
// DISTRIBUTE BY / SORT BY.
func TestSparkStripModifiers(t *testing.T) {
	sql := "SELECT * FROM t CLUSTER BY y DISTRIBUTE BY x SORT BY z"
	assertRoundTrips(t, []string{"hive", "spark2", "spark", "databricks"}, sql, sql)
}

// TestSparkExplodeProjection ports the spark arm of test_explode_projection_to_unnest.
func TestSparkExplodeProjection(t *testing.T) {
	assertRoundTrip(t, "spark", "SELECT EXPLODE(x) FROM tbl", "SELECT EXPLODE(x) FROM tbl")
	assertRoundTrip(t, "spark", "SELECT EXPLODE(col) FROM _u", "SELECT EXPLODE(col) FROM _u")
	assertRoundTrip(t, "spark", "SELECT EXPLODE(col) AS exploded FROM schema.tbl",
		"SELECT EXPLODE(col) AS exploded FROM schema.tbl")
}

// TestSparkToDate ports test_to_date for the registered dialects.
func TestSparkToDate(t *testing.T) {
	assertRoundTrips(t, []string{"hive", "spark", "databricks"}, "TO_DATE(x, 'yyyy-MM-dd')", "TO_DATE(x)")
	assertRoundTrips(t, []string{"hive", "spark", "databricks"}, "TO_DATE(x, 'yyyy')", "TO_DATE(x, 'yyyy')")
}
