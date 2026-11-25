package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
)

func TestSnowflakeNthValueFromClause(t *testing.T) {
	for _, sql := range []string{
		"SELECT NTH_VALUE(a, 2) FROM t",
		"SELECT NTH_VALUE(a, 2) FROM FIRST OVER () FROM t",
		"SELECT NTH_VALUE(a, 2) FROM LAST OVER () FROM t",
	} {
		t.Run(sql, func(t *testing.T) {
			assertRoundTrip(t, "snowflake", sql, sql)
		})
	}
}

func TestSnowflakeShow(t *testing.T) {
	for _, tc := range []struct{ sql, want string }{
		{"SHOW VIEWS IN foo", "SHOW VIEWS IN SCHEMA foo"},
		{"SHOW TERSE PRIMARY KEYS IN db.schema.tbl", "SHOW PRIMARY KEYS IN TABLE db.schema.tbl"},
		{"SHOW ICEBERG TABLES IN db.schema", "SHOW ICEBERG TABLES IN SCHEMA db.schema"},
		{"show terse tables in db.schema starts with 'a' limit 10 from 'b'", "SHOW TERSE TABLES IN SCHEMA db.schema STARTS WITH 'a' LIMIT 10 FROM 'b'"},
		{"SHOW COLUMNS IN VIEW", "SHOW COLUMNS IN VIEW"},
	} {
		t.Run(tc.sql, func(t *testing.T) {
			node, err := sqlglot.ParseOne(tc.sql, "snowflake")
			if err != nil || node.Kind() != exp.KindShow {
				t.Fatalf("expected Show, got %v, %v", node, err)
			}
			got, err := sqlglot.Generate(node, "snowflake", generator.Options{})
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestSnowflakeHistoricalData(t *testing.T) {
	for _, sql := range []string{
		"SELECT * FROM t AT (OFFSET => -60 * 5)",
		"SELECT * FROM t BEFORE (STATEMENT => $query_id) AS x",
		"SELECT * FROM t CHANGES (INFORMATION => APPEND_ONLY) AT (STREAM => 's') END (TIMESTAMP => $ts)",
	} {
		t.Run(sql, func(t *testing.T) {
			node, err := sqlglot.ParseOne(sql, "snowflake")
			if err != nil {
				t.Fatal(err)
			}
			var historical bool
			for _, n := range node.Walk() {
				historical = historical || n.Kind() == exp.KindHistoricalData
			}
			if !historical {
				t.Fatal("expected structured historical data")
			}
			got, err := sqlglot.Generate(node, "snowflake", generator.Options{})
			if err != nil || got != sql {
				t.Fatalf("got %q, %v; want %q", got, err, sql)
			}
		})
	}
}

func TestSnowflakeSemanticView(t *testing.T) {
	sql := "SELECT * FROM SEMANTIC_VIEW(tbl DIMENSIONS a.b METRICS a.c WHERE a.c > 5)"
	want := "SELECT * FROM SEMANTIC_VIEW(tbl METRICS a.c DIMENSIONS a.b WHERE a.c > 5)"
	node, err := sqlglot.ParseOne(sql, "snowflake")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, n := range node.Walk() {
		if n.Kind() == exp.KindSemanticView {
			found = true
			if n.Arg("metrics") == nil || n.Arg("dimensions") == nil || n.Arg("where") == nil {
				t.Fatal("semantic view missing clauses")
			}
		}
	}
	if !found {
		t.Fatal("expected structured semantic view")
	}
	got, err := sqlglot.Generate(node, "snowflake", generator.Options{})
	if err != nil || got != want {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
	pretty, err := sqlglot.Generate(node, "snowflake", generator.Options{Pretty: true})
	prettyWant := "SELECT\n  *\nFROM SEMANTIC_VIEW(\n  tbl\n  METRICS a.c\n  DIMENSIONS a.b\n  WHERE a.c > 5\n)"
	if err != nil || pretty != prettyWant {
		t.Fatalf("pretty got %q, %v; want %q", pretty, err, prettyWant)
	}
}

func TestSnowflakeStageSources(t *testing.T) {
	for _, sql := range []string{
		"SELECT * FROM @foo/bar (FILE_FORMAT => ds.test.fmt, PATTERN => 'test') AS x",
		"SELECT * FROM @~/some/path/file.csv",
		"SELECT * FROM '@external/location' (FILE_FORMAT => 'path.to.csv')",
	} {
		t.Run(sql, func(t *testing.T) {
			node, err := sqlglot.ParseOne(sql, "snowflake")
			if err != nil {
				t.Fatal(err)
			}
			var stage exp.Expression
			for _, n := range node.Walk() {
				if n.Kind() == exp.KindTable {
					stage = n
					break
				}
			}
			if stage == nil || stage.This() == nil || (stage.This().Kind() != exp.KindVar && stage.This().Kind() != exp.KindLiteral) {
				t.Fatal("stage source is not structured as a table")
			}
			got, err := sqlglot.Generate(node, "snowflake", generator.Options{})
			if err != nil || got != sql {
				t.Fatalf("got %q, %v; want %q", got, err, sql)
			}
		})
	}
}

func TestSnowflakeDirectoryStage(t *testing.T) {
	sql := "SELECT FILE_URL FROM DIRECTORY(@mystage) WHERE SIZE > 100000"
	node, err := sqlglot.ParseOne(sql, "snowflake")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, n := range node.Walk() {
		found = found || n.Kind() == exp.KindDirectoryStage
	}
	if !found {
		t.Fatal("expected structured directory stage")
	}
	got, err := sqlglot.Generate(node, "snowflake", generator.Options{})
	if err != nil || got != sql {
		t.Fatalf("got %q, %v; want %q", got, err, sql)
	}
}

func TestSnowflakeAutoincrement(t *testing.T) {
	for _, tc := range []struct{ sql, want string }{
		{"CREATE TABLE x (y INT AUTOINCREMENT START 10)", "CREATE TABLE x (y INT AUTOINCREMENT START 10)"},
		{"CREATE TABLE x (y INT AUTOINCREMENT(0, 1) NOORDER)", "CREATE TABLE x (y INT AUTOINCREMENT START 0 INCREMENT 1 NOORDER)"},
		{"CREATE TABLE x (y INT AUTOINCREMENT)", "CREATE TABLE x (y INT AUTOINCREMENT)"},
	} {
		t.Run(tc.sql, func(t *testing.T) { assertRoundTrip(t, "snowflake", tc.sql, tc.want) })
	}
}

func TestSnowflakeFileTransfer(t *testing.T) {
	for _, tc := range []struct {
		sql  string
		kind exp.Kind
	}{
		{"GET @s1/test 'file:///dir/tmp.csv'", exp.KindGet},
		{"PUT 'file:///dir/tmp.csv' @s1/test", exp.KindPut},
		{"PUT file:///dir/tmp.csv @s1/test", exp.KindCommand},
	} {
		t.Run(tc.sql, func(t *testing.T) {
			node, err := sqlglot.ParseOne(tc.sql, "snowflake")
			if err != nil || node.Kind() != tc.kind {
				t.Fatalf("unexpected node %v, %v", node, err)
			}
			got, err := sqlglot.Generate(node, "snowflake", generator.Options{})
			if err != nil || got != tc.sql {
				t.Fatalf("got %q, %v; want %q", got, err, tc.sql)
			}
		})
	}
}

func TestSnowflakeModelAttribute(t *testing.T) {
	for _, sql := range []string{"SELECT model!PREDICT(1)", "SELECT model!mladmin", "x.y.z!PREDICT(foo, bar)"} {
		t.Run(sql, func(t *testing.T) {
			node, err := sqlglot.ParseOne(sql, "snowflake")
			if err != nil {
				t.Fatal(err)
			}
			var found bool
			for _, n := range node.Walk() {
				found = found || n.Kind() == exp.KindModelAttribute
			}
			if !found {
				t.Fatal("expected model attribute")
			}
			got, err := sqlglot.Generate(node, "snowflake", generator.Options{})
			if err != nil || got != sql {
				t.Fatalf("got %q, %v; want %q", got, err, sql)
			}
		})
	}
}

func TestSnowflakeObjectLiterals(t *testing.T) {
	for _, tc := range []struct{ sql, want string }{
		{"SELECT {'a': 'b'}", "SELECT OBJECT_CONSTRUCT('a', 'b')"},
		{"SELECT {*} FROM t", "SELECT {*} FROM t"},
		{"SELECT {* EXCLUDE (a)} FROM t", "SELECT {* EXCLUDE (a)} FROM t"},
		{"SELECT {t.*} FROM t", "SELECT {t.*} FROM t"},
	} {
		t.Run(tc.sql, func(t *testing.T) {
			node, err := sqlglot.ParseOne(tc.sql, "snowflake")
			if err != nil {
				t.Fatal(err)
			}
			if node.Expressions()[0].Kind() != exp.KindStruct {
				t.Fatal("expected structured object")
			}
			got, err := sqlglot.Generate(node, "snowflake", generator.Options{})
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestSnowflakeNamedRoundArguments(t *testing.T) {
	sql := "SELECT ROUND(ROUNDING_MODE => 'HALF_TO_EVEN', EXPR => 2.25, SCALE => 1)"
	node, err := sqlglot.ParseOne(sql, "snowflake")
	if err != nil {
		t.Fatal(err)
	}
	round := node.Expressions()[0]
	if round.Kind() != exp.KindRound || round.Arg("decimals") == nil || round.Arg("truncate") == nil {
		t.Fatal("named arguments were not mapped to the Round AST")
	}
	got, err := sqlglot.Generate(node, "snowflake", generator.Options{})
	want := "SELECT ROUND(2.25, 1, 'HALF_TO_EVEN')"
	if err != nil || got != want {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
}

func TestSnowflakeDynamicFunction(t *testing.T) {
	for _, sql := range []string{"SELECT IDENTIFIER('my_func')(1, 2)", "SELECT IDENTIFIER($my_function_name)()"} {
		t.Run(sql, func(t *testing.T) {
			node, err := sqlglot.ParseOne(sql, "snowflake")
			if err != nil {
				t.Fatal(err)
			}
			if node.Expressions()[0].Kind() != exp.KindDynamicIdentifier {
				t.Fatal("expected dynamic identifier")
			}
			got, err := sqlglot.Generate(node, "snowflake", generator.Options{})
			if err != nil || got != sql {
				t.Fatalf("got %q, %v; want %q", got, err, sql)
			}
		})
	}
}

func TestSnowflakeDefaultDecimalPrecision(t *testing.T) {
	for _, tc := range []struct{ sql, want string }{
		{"SELECT CAST(x AS NUMBER)", "SELECT CAST(x AS DECIMAL(38, 0))"},
		{"SELECT CAST(x AS DECIMAL(10))", "SELECT CAST(x AS DECIMAL(10))"},
		{"CREATE TABLE x (y NUMERIC)", "CREATE TABLE x (y DECIMAL(38, 0))"},
	} {
		t.Run(tc.sql, func(t *testing.T) { assertRoundTrip(t, "snowflake", tc.sql, tc.want) })
	}
}

func TestSnowflakeStageFileFormat(t *testing.T) {
	for _, tc := range []struct{ sql, want string }{
		{"CREATE STAGE stage1 FILE_FORMAT='format1'", "CREATE STAGE stage1 FILE_FORMAT=(FORMAT_NAME='format1')"},
		{"CREATE STAGE stage1 FILE_FORMAT=schema1.format1", "CREATE STAGE stage1 FILE_FORMAT=(FORMAT_NAME=schema1.format1)"},
		{"CREATE TEMPORARY STAGE stage1 FILE_FORMAT=(TYPE=PARQUET)", "CREATE TEMPORARY STAGE stage1 FILE_FORMAT=(TYPE=PARQUET)"},
	} {
		t.Run(tc.sql, func(t *testing.T) {
			node, err := sqlglot.ParseOne(tc.sql, "snowflake")
			if err != nil {
				t.Fatal(err)
			}
			var found bool
			for _, n := range node.Walk() {
				found = found || n.Kind() == exp.KindFileFormatProperty
			}
			if !found {
				t.Fatal("expected file format property")
			}
			got, err := sqlglot.Generate(node, "snowflake", generator.Options{})
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestSnowflakeClusterProperty(t *testing.T) {
	for _, sql := range []string{"ALTER TABLE table1 CLUSTER BY (name)", "CREATE TABLE table1 CLUSTER BY (name1, name2, name3)"} {
		t.Run(sql, func(t *testing.T) {
			node, err := sqlglot.ParseOne(sql, "snowflake")
			if err != nil {
				t.Fatal(err)
			}
			var found bool
			for _, n := range node.Walk() {
				found = found || n.Kind() == exp.KindClusterProperty
			}
			if !found {
				t.Fatal("expected cluster property")
			}
			got, err := sqlglot.Generate(node, "snowflake", generator.Options{})
			if err != nil || got != sql {
				t.Fatalf("got %q, %v; want %q", got, err, sql)
			}
		})
	}
}

func TestSnowflakeSearchNamedArguments(t *testing.T) {
	sql := "SELECT SEARCH(line, 'king', SEARCH_MODE => 'AND', ANALYZER => 'PATTERN_ANALYZER')"
	want := "SELECT SEARCH(line, 'king', ANALYZER => 'PATTERN_ANALYZER', SEARCH_MODE => 'AND')"
	node, err := sqlglot.ParseOne(sql, "snowflake")
	if err != nil {
		t.Fatal(err)
	}
	search := node.Expressions()[0]
	if search.Kind() != exp.KindSearch || search.Arg("analyzer") == nil || search.Arg("search_mode") == nil {
		t.Fatal("named SEARCH arguments were not mapped")
	}
	got, err := sqlglot.Generate(node, "snowflake", generator.Options{})
	if err != nil || got != want {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
}

func TestSnowflakePolicies(t *testing.T) {
	for _, tc := range []struct {
		sql, want string
		kind      exp.Kind
	}{
		{"CREATE TABLE t (id INT WITH MASKING POLICY p USING (c1, c2))", "CREATE TABLE t (id INT MASKING POLICY p USING (c1, c2))", exp.KindMaskingPolicyColumnConstraint},
		{"CREATE TABLE t (id INT WITH PROJECTION POLICY p.q.r)", "CREATE TABLE t (id INT PROJECTION POLICY p.q.r)", exp.KindProjectionPolicyColumnConstraint},
		{"CREATE TABLE t (id INT WITH TAG (k='v'))", "CREATE TABLE t (id INT TAG (k='v'))", exp.KindTags},
		{"CREATE TABLE t (c INT) ROW ACCESS POLICY p ON (c)", "CREATE TABLE t (c INT) WITH ROW ACCESS POLICY p ON (c)", exp.KindRowAccessProperty},
	} {
		t.Run(tc.sql, func(t *testing.T) {
			node, err := sqlglot.ParseOne(tc.sql, "snowflake")
			if err != nil {
				t.Fatal(err)
			}
			var found bool
			for _, n := range node.Walk() {
				found = found || n.Kind() == tc.kind
			}
			if !found {
				t.Fatal("expected structured policy")
			}
			got, err := sqlglot.Generate(node, "snowflake", generator.Options{})
			if err != nil || got != tc.want {
				t.Fatalf("got %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestSnowflakeAlterSetOptions(t *testing.T) {
	for _, sql := range []string{
		"ALTER TABLE t SET TAG a = 'x', b = 'y'",
		"ALTER TABLE t SET STAGE_FILE_FORMAT = (TYPE=CSV FIELD_DELIMITER='|')",
		"ALTER TABLE t SET STAGE_COPY_OPTIONS = (ON_ERROR=SKIP_FILE PURGE=TRUE)",
	} {
		t.Run(sql, func(t *testing.T) { assertRoundTrip(t, "snowflake", sql, sql) })
	}
}

// TestSnowflakeLogNatural ports generators/snowflake.py:778-782: a one-argument
// Log is Snowflake's natural logarithm, so it renders as LN.
func TestSnowflakeLogNatural(t *testing.T) {
	assertRoundTrip(t, "snowflake", "SELECT LN(5)", "SELECT LN(5)")
	got, err := sqlglot.Generate(mustParse(t, "SELECT {fn LOG(5)}", "snowflake"), "snowflake", generator.Options{})
	if err != nil || got != "SELECT LN(5)" {
		t.Fatalf("got %q, %v; want %q", got, err, "SELECT LN(5)")
	}
}

// TestSnowflakeUnnestToFlatten ports unnest_sql (generators/snowflake.py:804-844).
func TestSnowflakeUnnestToFlatten(t *testing.T) {
	assertRoundTrip(t, "snowflake",
		"SELECT * FROM t, UNNEST(x) WITH ORDINALITY",
		"SELECT * FROM t, TABLE(FLATTEN(INPUT => x)) AS _t0(seq, key, path, index, value, this)")
}

// TestSnowflakeCore ports the same-dialect round-trip core of test_snowflake
// (tests/dialects/test_snowflake.py).
func TestSnowflakeCore(t *testing.T) {
	for _, sql := range []string{
		"SELECT 1",
		`SELECT "a" FROM t`,
		`SELECT "a"."b" FROM "t" AS "a"`,
		"SELECT $1 FROM t",
		"SELECT * FROM foo WHERE x > 1",
		"SELECT CAST(x AS VARIANT)",
		"SELECT CAST(x AS VARCHAR)",
		"SELECT CAST(x AS DECIMAL(38, 0))",
		"SELECT TRY_CAST(x AS INT)",
		"SELECT IFF(x, 1, 0)",
		"SELECT BOOLAND(a, b)",
		"SELECT BOOLOR(a, b)",
		"SELECT BITAND(a, b)",
		"SELECT BITANDAGG(x)",
		"SELECT BITORAGG(x)",
		"SELECT BITXORAGG(x)",
		"SELECT BITSHIFTLEFT(a, 1)",
		"SELECT HEX_ENCODE(x)",
		"SELECT HEX_DECODE_BINARY(x)",
		"SELECT UUID_STRING()",
		"SELECT LISTAGG(x, ', ')",
		"SELECT OBJECT_CONSTRUCT('a', 1)",
		"SELECT ARRAY_CONTAINS(1, x)",
		"SELECT ARRAY_INTERSECTION(a, b)",
		"SELECT ARRAYS_OVERLAP(a, b)",
		"SELECT CHARINDEX(a, b)",
		"SELECT STARTSWITH(a, b)",
		"SELECT ENDSWITH(a, b)",
		"SELECT DAYOFWEEKISO(x)",
		"SELECT DAYOFYEAR(x)",
		"SELECT WEEKISO(x)",
		"SELECT YEAROFWEEK(x)",
		"SELECT DATEADD(DAY, 1, x)",
		"SELECT DATEDIFF(DAY, a, b)",
		"SELECT DATE_PART('year', x)",
		"SELECT DATE_PART(YEAR, x)",
		"SELECT TO_DATE(x)",
		"SELECT TO_TIMESTAMP(x)",
		"SELECT TRY_TO_DATE(x)",
		"SELECT MD5_BINARY(x)",
		"SELECT SHA1_BINARY(x)",
		"SELECT SHA2_BINARY(x, 256)",
	} {
		assertRoundTrip(t, "snowflake", sql, sql)
	}
}

// TestSnowflakeCanonicalizations pins the generator's same-dialect normalizations.
func TestSnowflakeCanonicalizations(t *testing.T) {
	cases := map[string]string{
		"SELECT ARRAY_CONSTRUCT(1, 2)":        "SELECT [1, 2]",
		"SELECT DATE(x)":                      "SELECT TO_DATE(x)",
		"SELECT TIMESTAMPADD(HOUR, 1, x)":     "SELECT DATEADD(HOUR, 1, x)",
		"SELECT TIMESTAMPDIFF(HOUR, a, b)":    "SELECT DATEDIFF(HOUR, a, b)",
		"SELECT EXTRACT(YEAR FROM x)":         "SELECT DATE_PART(YEAR, x)",
		"SELECT SQUARE(x)":                    "SELECT POWER(x, 2)",
		"SELECT LEN(x)":                       "SELECT LENGTH(x)",
		"SELECT ARRAY_CONSTRUCT()":            "SELECT []",
		"SELECT CAST(x AS OBJECT)":            "SELECT CAST(x AS OBJECT)",
		"SELECT CAST(x AS ARRAY(INT))":        "SELECT CAST(x AS ARRAY(INT))",
		"SELECT CAST(x AS MAP(VARCHAR, INT))": "SELECT CAST(x AS MAP(VARCHAR, INT))",
	}
	for sql, want := range cases {
		assertRoundTrip(t, "snowflake", sql, want)
	}
}

// TestSnowflakeUndrop ports the UNDROP statement.
func TestSnowflakeUndrop(t *testing.T) {
	for _, sql := range []string{
		"UNDROP TABLE my_table",
		"UNDROP TABLE db.schema.my_table",
		"UNDROP DATABASE my_db",
		"UNDROP SCHEMA my_schema",
		"UNDROP SCHEMA my_schema RENAME TO new_schema",
		"UNDROP TAG my_tag",
	} {
		assertRoundTrip(t, "snowflake", sql, sql)
	}
}

// TestSnowflakeDirectedJoin ports the DIRECTED join modifier.
func TestSnowflakeDirectedJoin(t *testing.T) {
	for _, sql := range []string{
		"SELECT * FROM a LEFT DIRECTED JOIN b USING (id)",
		"SELECT * FROM a RIGHT OUTER DIRECTED JOIN b USING (id)",
	} {
		assertRoundTrip(t, "snowflake", sql, sql)
	}
}

// TestSnowflakeReservedIdentifiers verifies the Snowflake-only ID_VAR keywords are usable as
// column names.
func TestSnowflakeReservedIdentifiers(t *testing.T) {
	for _, sql := range []string{
		"SELECT integration FROM t",
		"SELECT package FROM t",
		"SELECT policy FROM t",
		"SELECT pool FROM t",
		"SELECT role FROM t",
		"SELECT rule FROM t",
		"SELECT volume FROM t",
	} {
		assertRoundTrip(t, "snowflake", sql, sql)
	}
}

// TestSnowflakeDescribe verifies the describe kind is preserved for tables.
func TestSnowflakeDescribe(t *testing.T) {
	assertRoundTrip(t, "snowflake", "DESCRIBE TABLE db.table", "DESCRIBE TABLE db.table")
}

// ensure the generator import is referenced even if a helper changes.
var _ = generator.Options{}

// TestSnowflakeReviewRegressions pins the review-driven fixes: DIV0 null/grouping semantics,
// OBJECT_CONSTRUCT_KEEP_NULL, Boolean XOR/NOT, the bitwise padding argument, TIMEDIFF,
// CURRENT_TIMESTAMP precision and ARRAY_POSITION argument order.
func TestSnowflakeReviewRegressions(t *testing.T) {
	cases := map[string]string{
		"SELECT DIV0(NULL, 0)":                         "SELECT IFF(0 = 0 AND NOT NULL IS NULL, 0, NULL / 0)",
		"SELECT DIV0(a - b, c - d)":                    "SELECT IFF((c - d) = 0 AND NOT (a - b) IS NULL, 0, (a - b) / (c - d))",
		"SELECT OBJECT_CONSTRUCT_KEEP_NULL('k', NULL)": "SELECT OBJECT_CONSTRUCT_KEEP_NULL('k', NULL)",
		"SELECT OBJECT_CONSTRUCT('k', NULL)":           "SELECT OBJECT_CONSTRUCT('k', NULL)",
		"SELECT BOOLXOR(2, 0)":                         "SELECT BOOLXOR(2, 0)",
		"SELECT BOOLNOT(x)":                            "SELECT BOOLNOT(x)",
		"SELECT BITAND(a, b, 'pad')":                   "SELECT BITAND(a, b, 'pad')",
		"SELECT TIMEDIFF(HOUR, a, b)":                  "SELECT DATEDIFF(HOUR, a, b)",
		"SELECT CURRENT_TIMESTAMP(3)":                  "SELECT CURRENT_TIMESTAMP(3)",
		"SELECT ARRAY_CONTAINS('x', arr)":              "SELECT ARRAY_CONTAINS('x', arr)",
		"SELECT ARRAY_POSITION('x', arr)":              "SELECT ARRAY_POSITION('x', arr)",
	}
	for sql, want := range cases {
		assertRoundTrip(t, "snowflake", sql, want)
	}
}

// TestSnowflakeTimeDiffRenderer ensures a TimeDiff node produces Snowflake's DATEDIFF form.
func TestSnowflakeTimeDiffRenderer(t *testing.T) {
	expr := exp.TimeDiff(exp.Args{
		"this":       exp.Column(exp.Args{"this": exp.ToIdentifier("b")}),
		"expression": exp.Column(exp.Args{"this": exp.ToIdentifier("a")}),
		"unit":       exp.Var(exp.Args{"this": "HOUR"}),
	})
	got, err := sqlglot.Generate(expr, "snowflake", generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if want := "DATEDIFF(HOUR, a, b)"; got != want {
		t.Fatalf("TimeDiff = %q, want %q", got, want)
	}
}

// TestSnowflakeClosedParityGaps pins the constructs whose Snowflake rows were the last
// entries in testdata/parity_gaps.txt (issue #92): COPY parameter `=`, staged file
// locations, typed lambda arguments, HANDLER =, DYNAMIC/CHANGE_TRACKING property placement,
// the TO_TIMESTAMP_*_TZ cast builders, and external-table parsing.
func TestSnowflakeClosedParityGaps(t *testing.T) {
	for _, tc := range []struct{ sql, want string }{
		// generators/snowflake.py:390 COPY_PARAMS_EQ_REQUIRED.
		{
			"COPY INTO mytable (col1) FROM 's3://bucket/data' FILES = ('f1', 'f2') PATTERN = 'p'",
			"COPY INTO mytable (col1) FROM 's3://bucket/data' FILES = ('f1', 'f2') PATTERN = 'p'",
		},
		// parsers/snowflake.py:1306-1318 _parse_location_path.
		{
			"COPY INTO load1 FROM @%load1/data1/ CREDENTIALS = (AWS_KEY_ID='id' AWS_SECRET_KEY='key') FILES = ('t.csv')",
			"COPY INTO load1 FROM @%load1/data1/ CREDENTIALS = (AWS_KEY_ID='id' AWS_SECRET_KEY='key') FILES = ('t.csv')",
		},
		{
			"COPY INTO my_db.my_schema.my_table FROM @my_db.my_schema.my_stage/my_path FILE_FORMAT = (FORMAT_NAME=my_db.my_schema.my_format)",
			"COPY INTO my_db.my_schema.my_table FROM @my_db.my_schema.my_stage/my_path FILE_FORMAT = (FORMAT_NAME=my_db.my_schema.my_format)",
		},
		// parsers/snowflake.py:1298-1304 _parse_file_location parsing table parts.
		{
			"COPY INTO 's3://example/data.csv' FROM EXTRA.EXAMPLE.TABLE FILE_FORMAT = (TYPE=CSV)",
			"COPY INTO 's3://example/data.csv' FROM EXTRA.EXAMPLE.TABLE FILE_FORMAT = (TYPE=CSV)",
		},
		// parsers/snowflake.py:1320-1331 _parse_lambda_arg + TYPED_LAMBDA_ARGS.
		{
			"transform(x, a int -> a + a + 1)",
			"TRANSFORM(x, a -> CAST(a AS INT) + CAST(a AS INT) + 1)",
		},
		// generators/snowflake.py:483 `HANDLER = <expr>`.
		{
			"CREATE OR REPLACE FUNCTION f() RETURNS INT LANGUAGE PYTHON HANDLER = 'fn' AS 'x'",
			"CREATE OR REPLACE FUNCTION f() RETURNS INT LANGUAGE PYTHON HANDLER = 'fn' AS 'x'",
		},
		// Column list before POST_SCHEMA/POST_WITH properties (DYNAMIC, CHANGE_TRACKING).
		{
			"CREATE DYNAMIC TABLE dt TARGET_LAG='1 minute' WAREHOUSE=my_wh (id) AS SELECT * FROM bla",
			"CREATE DYNAMIC TABLE dt (id) TARGET_LAG='1 minute' WAREHOUSE=my_wh AS SELECT * FROM bla",
		},
		{
			"CREATE TABLE t CHANGE_TRACKING=TRUE DATA_RETENTION_TIME_IN_DAYS=1 (id INT)",
			"CREATE TABLE t (id INT) CHANGE_TRACKING=TRUE DATA_RETENTION_TIME_IN_DAYS=1",
		},
		{
			"CREATE VIEW my_view CHANGE_TRACKING=TRUE (id) AS SELECT * FROM my_table",
			"CREATE VIEW my_view (id) CHANGE_TRACKING=TRUE AS SELECT * FROM my_table",
		},
		// parsers/snowflake.py:657-689 _build_datetime for the timestamp flavors.
		{
			"SELECT 1 FROM some_table AT (TIMESTAMP => TO_TIMESTAMP_TZ('2024-07-01 00:00:00+00:00'))",
			"SELECT 1 FROM some_table AT (TIMESTAMP => CAST('2024-07-01 00:00:00+00:00' AS TIMESTAMPTZ))",
		},
		// generators/snowflake.py:957-973: the StrToTime function name follows TIMESTAMP_TYPES.
		{
			"SELECT TO_TIMESTAMP_TZ('2024-01-01', 'YYYY-MM-DD')",
			"SELECT TO_TIMESTAMP_TZ('2024-01-01', 'yyyy-mm-DD')",
		},
		{
			"SELECT TO_TIMESTAMP_LTZ('2024-01-01', 'YYYY-MM-DD')",
			"SELECT TO_TIMESTAMP_LTZ('2024-01-01', 'yyyy-mm-DD')",
		},
		{
			"SELECT TO_TIMESTAMP_NTZ('2024-01-01', 'YYYY-MM-DD')",
			"SELECT TO_TIMESTAMP_NTZ('2024-01-01', 'yyyy-mm-DD')",
		},
		// Grammar extensions beyond upstream (testdata/upstream_extensions.jsonl).
		{"COPY INTO t", "COPY INTO t"},
		{"SHOW TABLES IN SCHEMA LIMIT 10", "SHOW TABLES IN SCHEMA LIMIT 10"},
		// EXTERNAL/LOCATION/PARTITION BY properties plus `expr:KEY` -> GET_PATH.
		{
			"CREATE EXTERNAL TABLE et2 (col1 DATE AS (parse_json(m):COL1::date)) PARTITION BY (col1) LOCATION=@s2/logs/ FILE_FORMAT = (TYPE=PARQUET)",
			"CREATE EXTERNAL TABLE et2 (col1 DATE AS (CAST(GET_PATH(PARSE_JSON(m), 'COL1') AS DATE))) PARTITION BY (col1) LOCATION=@s2/logs/ FILE_FORMAT=(TYPE=PARQUET)",
		},
	} {
		t.Run(tc.sql, func(t *testing.T) {
			assertRoundTrip(t, "snowflake", tc.sql, tc.want)
		})
	}
}

// TestSnowflakeStorageIntegrationPretty pins the POST_WITH separator in pretty mode: the
// Snowflake with_properties override (generators/snowflake.py:683-684) prefixes the list
// with self.sep(""), so the first property starts on its own line.
func TestSnowflakeStorageIntegrationPretty(t *testing.T) {
	sql := "CREATE STORAGE INTEGRATION s3_int\nTYPE=EXTERNAL_STAGE\nSTORAGE_PROVIDER='S3'\nENABLED=TRUE"
	node, err := sqlglot.ParseOne(sql, "snowflake")
	if err != nil {
		t.Fatal(err)
	}
	got, err := sqlglot.Generate(node, "snowflake", generator.Options{Pretty: true})
	if err != nil {
		t.Fatal(err)
	}
	if got != sql {
		t.Fatalf("pretty CREATE STORAGE INTEGRATION = %q, want %q", got, sql)
	}
}

// TestSnowflakeExplodeProjectionToUnnest pins transforms.explode_projection_to_unnest
// (transforms.py:402-552) as wired into the Snowflake Select preprocess
// (generators/snowflake.py:549), mirroring upstream's test_flatten expectation.
func TestSnowflakeExplodeProjectionToUnnest(t *testing.T) {
	node := exp.SelectQuery(exp.AliasExpr(
		exp.Explode(exp.Args{"this": exp.Column(exp.Args{"this": exp.ToIdentifier("x", false)})}),
		"y", false,
	))
	got, err := sqlglot.Generate(node, "snowflake", generator.Options{Pretty: true})
	if err != nil {
		t.Fatal(err)
	}
	want := `SELECT
  IFF(_u.pos = _u_2.pos_2, _u_2.y, NULL) AS y
FROM TABLE(FLATTEN(INPUT => ARRAY_GENERATE_RANGE(0, (
  GREATEST(ARRAY_SIZE(x)) - 1
) + 1))) AS _u(seq, key, path, index, pos, this)
CROSS JOIN TABLE(FLATTEN(INPUT => x)) AS _u_2(seq, key, path, pos_2, y, this)
WHERE
  _u.pos = _u_2.pos_2
  OR (
    _u.pos > (
      ARRAY_SIZE(x) - 1
    ) AND _u_2.pos_2 = (
      ARRAY_SIZE(x) - 1
    )
  )`
	if got != want {
		t.Fatalf("explode projection rewrite =\n%s\nwant\n%s", got, want)
	}
}

// TestSnowflakeExplodeProjectionQuotedAlias keeps the alias Identifier's quoted flag when
// explode_projection_to_unnest rebuilds it (transforms.py:437-480 stores the identifier, not
// just its name).
func TestSnowflakeExplodeProjectionQuotedAlias(t *testing.T) {
	node := exp.SelectQuery(exp.AliasNode(exp.Args{
		"this":  exp.Explode(exp.Args{"this": exp.Column(exp.Args{"this": exp.ToIdentifier("x", false)})}),
		"alias": exp.ToIdentifier("MyCol", true),
	}))
	got, err := sqlglot.Generate(node, "snowflake", generator.Options{Pretty: true})
	if err != nil {
		t.Fatal(err)
	}
	want := `SELECT
  IFF(_u.pos = _u_2.pos_2, _u_2."MyCol", NULL) AS "MyCol"
FROM TABLE(FLATTEN(INPUT => ARRAY_GENERATE_RANGE(0, (
  GREATEST(ARRAY_SIZE(x)) - 1
) + 1))) AS _u(seq, key, path, index, pos, this)
CROSS JOIN TABLE(FLATTEN(INPUT => x)) AS _u_2(seq, key, path, pos_2, "MyCol", this)
WHERE
  _u.pos = _u_2.pos_2
  OR (
    _u.pos > (
      ARRAY_SIZE(x) - 1
    ) AND _u_2.pos_2 = (
      ARRAY_SIZE(x) - 1
    )
  )`
	if got != want {
		t.Fatalf("quoted explode alias rewrite =\n%s\nwant\n%s", got, want)
	}
}
