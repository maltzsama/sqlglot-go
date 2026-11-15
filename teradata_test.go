package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
)

// assertTeradataRead transpiles sql from readDialect into teradata, expecting want — the
// upstream validate_all read= direction (parse with the read dialect, generate with the
// test dialect, must equal the main sql).
func assertTeradataRead(t *testing.T, readDialect, sql, want string) {
	t.Helper()
	got, err := sqlglot.Transpile(sql, readDialect, "teradata", generator.Options{})
	if err != nil {
		t.Fatalf("read %s: Transpile(%q): %v", readDialect, sql, err)
	}
	if got[0] != want {
		t.Fatalf("read %s: %q -> %q, want %q", readDialect, sql, got[0], want)
	}
}

// assertTeradataWrite transpiles sql from teradata into writeDialect, expecting want — the
// upstream validate_all write= direction (parse with the test dialect, generate with the
// write dialect, must equal the recorded output).
func assertTeradataWrite(t *testing.T, writeDialect, sql, want string) {
	t.Helper()
	got, err := sqlglot.Transpile(sql, "teradata", writeDialect, generator.Options{})
	if err != nil {
		t.Fatalf("write %s: Transpile(%q): %v", writeDialect, sql, err)
	}
	if got[0] != want {
		t.Fatalf("write %s: %q -> %q, want %q", writeDialect, sql, got[0], want)
	}
}

// assertCommand checks that sql parses to a Command node and round-trips — the upstream
// validate_identity(..., check_command_warning=True).assert_is(exp.Command) pattern.
func assertCommand(t *testing.T, dialect, sql string) {
	t.Helper()
	expr, err := sqlglot.ParseOne(sql, dialect)
	if err != nil {
		t.Fatalf("%s: ParseOne(%q): %v", dialect, sql, err)
	}
	if expr.Kind() != expressions.KindCommand {
		t.Fatalf("%s: %q parsed to %s, want Command", dialect, sql, expr.ToS())
	}
	assertRoundTrip(t, dialect, sql, sql)
}

// TestTeradata ports test_teradata (tests/dialects/test_teradata.py:8-41).
func TestTeradata(t *testing.T) {
	// write={"": "(u - l) * RAND() + l", "teradata": "RANDOM(l, u)"}.
	assertTeradataWrite(t, "", "RANDOM(l, u)", "(u - l) * RAND() + l")
	assertRoundTrip(t, "teradata", "RANDOM(l, u)", "RANDOM(l, u)")

	assertRoundTrip(t, "teradata", "TO_NUMBER(expr, fmt, nlsparam)", "TO_NUMBER(expr, fmt, nlsparam)")
	assertRoundTrip(t, "teradata", "SELECT TOP 10 * FROM tbl", "SELECT TOP 10 * FROM tbl")
	assertRoundTrip(t, "teradata", "SELECT * FROM tbl SAMPLE 5", "SELECT * FROM tbl SAMPLE 5")
	assertRoundTrip(t, "teradata", "SELECT * FROM tbl SAMPLE 0.33, .25, .1",
		"SELECT * FROM tbl SAMPLE 0.33, 0.25, 0.1")

	// read={"databricks": "USE tduser"}, write databricks + teradata.
	assertTeradataRead(t, "databricks", "USE tduser", "DATABASE tduser")
	assertTeradataWrite(t, "databricks", "DATABASE tduser", "USE tduser")
	assertRoundTrip(t, "teradata", "DATABASE tduser", "DATABASE tduser")

	assertRoundTrip(t, "teradata", "SELECT 0x1d", "SELECT X'1d'")
	assertRoundTrip(t, "teradata", "SELECT X'1D'", "SELECT X'1D'")
	assertRoundTrip(t, "teradata", "SELECT x'1d'", "SELECT X'1d'")

	assertCommand(t, "teradata", "RENAME TABLE emp TO employee")
}

// TestTeradataTranslate ports test_translate (tests/dialects/test_teradata.py:43-45).
func TestTeradataTranslate(t *testing.T) {
	assertRoundTrip(t, "teradata", "TRANSLATE(x USING LATIN_TO_UNICODE)", "TRANSLATE(x USING LATIN_TO_UNICODE)")
	assertRoundTrip(t, "teradata", "TRANSLATE(x USING LATIN_TO_UNICODE WITH ERROR)",
		"TRANSLATE(x USING LATIN_TO_UNICODE WITH ERROR)")
}

// TestTeradataLocking ports test_locking (tests/dialects/test_teradata.py:47-66).
func TestTeradataLocking(t *testing.T) {
	for _, sql := range []string{
		"LOCKING ROW FOR ACCESS SELECT * FROM table1",
		"LOCKING TABLE table1 FOR ACCESS SELECT col1, col2 FROM table1",
		"LOCKING ROW FOR SHARE SELECT * FROM table1",
		"LOCKING DATABASE db1 FOR READ SELECT * FROM table1",
		"LOCKING ROW FOR EXCLUSIVE SELECT * FROM table1",
		"LOCKING VIEW view1 FOR ACCESS SELECT * FROM view1",
		"LOCKING ROW FOR ACCESS SELECT col1, col2 FROM table1 WHERE col1 > 10",
		"LOCKING TABLE table1 FOR ACCESS SELECT * FROM table1 JOIN table2 ON table1.id = table2.id",
		// Regression test: LOCKING still works in CREATE VIEW context.
		"CREATE VIEW view_b AS LOCKING ROW FOR ACCESS SELECT COL1, COL2 FROM table_b",
	} {
		assertRoundTrip(t, "teradata", sql, sql)
	}
}

// TestTeradataUpdate ports test_update (tests/dialects/test_teradata.py:68-75): mysql has
// no UPDATE ... FROM, so the parsed FROM is rewritten into JOIN ... ON TRUE.
func TestTeradataUpdate(t *testing.T) {
	sql := "UPDATE A FROM schema.tableA AS A, (SELECT col1 FROM schema.tableA GROUP BY col1) AS B SET col2 = '' WHERE A.col1 = B.col1"
	assertRoundTrip(t, "teradata", sql, sql)
	assertTeradataWrite(t, "mysql", sql,
		"UPDATE A JOIN `schema`.tableA AS A ON TRUE JOIN (SELECT col1 FROM `schema`.tableA GROUP BY col1) AS B ON TRUE SET A.col2 = '' WHERE A.col1 = B.col1")
}

// TestTeradataStatistics ports test_statistics (tests/dialects/test_teradata.py:77-84).
func TestTeradataStatistics(t *testing.T) {
	for _, sql := range []string{
		"COLLECT STATISTICS ON tbl INDEX(col)",
		"COLLECT STATS ON tbl COLUMNS(col)",
		"COLLECT STATS COLUMNS(col) ON tbl",
		"HELP STATISTICS personel.employee",
		"HELP STATISTICS personnel.employee FROM my_qcd",
	} {
		assertCommand(t, "teradata", sql)
	}
}

// TestTeradataCreate ports test_create (tests/dialects/test_teradata.py:86-186).
func TestTeradataCreate(t *testing.T) {
	// Upstream lists this REPLACE VIEW case twice; one copy covers it.
	replaceView := "REPLACE VIEW view_b (COL1, COL2) AS LOCKING ROW FOR ACCESS SELECT COL1, COL2 FROM table_b"
	assertRoundTrip(t, "teradata", replaceView,
		"CREATE OR REPLACE VIEW view_b (COL1, COL2) AS LOCKING ROW FOR ACCESS SELECT COL1, COL2 FROM table_b")

	for _, sql := range []string{
		"CREATE TABLE x (y INT) PRIMARY INDEX (y) PARTITION BY y INDEX (y)",
		"CREATE TABLE x (y INT) PARTITION BY y INDEX (y)",
		"CREATE MULTISET VOLATILE TABLE my_table (id INT) PRIMARY INDEX (id) ON COMMIT PRESERVE ROWS",
		"CREATE SET VOLATILE TABLE my_table (id INT) PRIMARY INDEX (id) ON COMMIT DELETE ROWS",
		"CREATE TABLE a (b INT) PRIMARY INDEX (y) PARTITION BY RANGE_N(b BETWEEN 'a', 'b' AND 'c' EACH '1')",
		"CREATE TABLE a (b INT) PARTITION BY RANGE_N(b BETWEEN 0, 1 AND 2 EACH 1)",
		"CREATE TABLE a (b INT) PARTITION BY RANGE_N(b BETWEEN *, 1 AND * EACH b) INDEX (a)",
		"CREATE TABLE a, NO FALLBACK PROTECTION, NO LOG, NO JOURNAL, CHECKSUM=ON, NO MERGEBLOCKRATIO, BLOCKCOMPRESSION=ALWAYS (a INT)",
		"CREATE TABLE a, WITH JOURNAL TABLE=x.y.z, CHECKSUM=OFF, MERGEBLOCKRATIO=1, DATABLOCKSIZE=10 KBYTES (a INT)",
		"CREATE TABLE a, BEFORE JOURNAL, AFTER JOURNAL, FREESPACE=1, DEFAULT DATABLOCKSIZE, BLOCKCOMPRESSION=DEFAULT (a INT)",
		"CREATE TABLE a, DUAL JOURNAL, DUAL AFTER JOURNAL, MERGEBLOCKRATIO=1 PERCENT, DATABLOCKSIZE=10 KILOBYTES (a INT)",
		"CREATE TABLE a, DUAL BEFORE JOURNAL, LOCAL AFTER JOURNAL, MAXIMUM DATABLOCKSIZE, BLOCKCOMPRESSION=AUTOTEMP(c1 INT) (a INT)",
		"CREATE VOLATILE MULTISET TABLE a, NOT LOCAL AFTER JOURNAL, FREESPACE=1 PERCENT, DATABLOCKSIZE=10 BYTES, WITH NO CONCURRENT ISOLATED LOADING FOR ALL (a INT)",
		"CREATE VOLATILE SET TABLE example1 AS (SELECT col1, col2, col3 FROM table1) WITH DATA PRIMARY INDEX (col1) ON COMMIT PRESERVE ROWS",
		"CREATE SET GLOBAL TEMPORARY TABLE a, NO BEFORE JOURNAL, NO AFTER JOURNAL, MINIMUM DATABLOCKSIZE, BLOCKCOMPRESSION=NEVER (a INT)",
		"CREATE TABLE db.foo (id INT NOT NULL, valid_date DATE FORMAT 'YYYY-MM-DD', measurement INT COMPRESS)",
		"CREATE TABLE db.foo (id INT NOT NULL, valid_date DATE FORMAT 'YYYY-MM-DD', measurement INT COMPRESS (1, 2, 3))",
		"CREATE TABLE db.foo (id INT NOT NULL, valid_date DATE FORMAT 'YYYY-MM-DD' COMPRESS (CAST('9999-09-09' AS DATE)), measurement INT)",
	} {
		assertRoundTrip(t, "teradata", sql, sql)
	}

	// The multi-line input normalizes to a single line.
	assertTeradataWrite(t, "teradata", `
            CREATE SET TABLE test, NO FALLBACK, NO BEFORE JOURNAL, NO AFTER JOURNAL,
            CHECKSUM = DEFAULT (x INT, y INT, z CHAR(30), a INT, b DATE, e INT)
            PRIMARY INDEX (a),
            INDEX(x, y)
            `,
		"CREATE SET TABLE test, NO FALLBACK, NO BEFORE JOURNAL, NO AFTER JOURNAL, CHECKSUM=DEFAULT (x INT, y INT, z CHAR(30), a INT, b DATE, e INT) PRIMARY INDEX (a) INDEX (x, y)")

	assertTeradataWrite(t, "teradata", "REPLACE VIEW a AS (SELECT b FROM c)",
		"CREATE OR REPLACE VIEW a AS (SELECT b FROM c)")

	// write={"bigquery": ..., ...}: the VOLATILE clause is dropped everywhere. drill and
	// tableau are not registered dialects in this port, so they are skipped.
	assertTeradataWrite(t, "teradata", "CREATE VOLATILE TABLE a", "CREATE VOLATILE TABLE a")
	for _, writeDialect := range []string{
		"bigquery", "clickhouse", "databricks", "duckdb", "hive", "mysql", "oracle",
		"postgres", "presto", "redshift", "snowflake", "spark", "sqlite", "starrocks",
		"trino", "tsql",
	} {
		assertTeradataWrite(t, writeDialect, "CREATE VOLATILE TABLE a", "CREATE TABLE a")
	}
}

// TestTeradataInsert ports test_insert (tests/dialects/test_teradata.py:188-191).
func TestTeradataInsert(t *testing.T) {
	assertTeradataWrite(t, "teradata", "INS INTO x SELECT * FROM y", "INSERT INTO x SELECT * FROM y")
}

// TestTeradataMod ports test_mod (tests/dialects/test_teradata.py:193-194).
func TestTeradataMod(t *testing.T) {
	assertRoundTrip(t, "teradata", "a MOD b", "a MOD b")
	assertTeradataWrite(t, "mysql", "a MOD b", "a % b")
}

// TestTeradataPower ports test_power (tests/dialects/test_teradata.py:196-197).
func TestTeradataPower(t *testing.T) {
	assertRoundTrip(t, "teradata", "a ** b", "a ** b")
	assertTeradataWrite(t, "mysql", "a ** b", "POWER(a, b)")
	// parsers/teradata.py:99-101 EXPONENT only lists DSTAR, so Teradata's `^` stays
	// BitwiseXor while duckdb's {CARET, DSTAR} table (parsers/duckdb.py:106-110) turns
	// both spellings into Pow — which teradata re-renders as `**`.
	assertRoundTrip(t, "teradata", "2 ^ 3", "2 ^ 3")
	assertTeradataRead(t, "duckdb", "2 ^ 3", "2 ** 3")
	assertTeradataRead(t, "duckdb", "2 ** 3", "2 ** 3")
}

// TestTeradataAbbrev ports test_abbrev (tests/dialects/test_teradata.py:199-214).
func TestTeradataAbbrev(t *testing.T) {
	cases := map[string]string{
		"a LT b":       "a < b",
		"a LE b":       "a <= b",
		"a GT b":       "a > b",
		"a GE b":       "a >= b",
		"a ^= b":       "a <> b",
		"a NE b":       "a <> b",
		"a NOT= b":     "a <> b",
		"a EQ b":       "a = b",
		"SEL a FROM b": "SELECT a FROM b",
		"SELECT col1, col2 FROM dbc.table1 WHERE col1 EQ 'value1' MINUS SELECT col1, col2 FROM dbc.table2": "SELECT col1, col2 FROM dbc.table1 WHERE col1 = 'value1' EXCEPT SELECT col1, col2 FROM dbc.table2",
		"UPD a SET b = 1": "UPDATE a SET b = 1",
		"DEL FROM a":      "DELETE FROM a",
	}
	for sql, want := range cases {
		assertRoundTrip(t, "teradata", sql, want)
	}
}

// TestTeradataDatatype ports test_datatype (tests/dialects/test_teradata.py:216-225).
func TestTeradataDatatype(t *testing.T) {
	assertRoundTrip(t, "teradata", "CREATE TABLE z (a ST_GEOMETRY(1))", "CREATE TABLE z (a ST_GEOMETRY(1))")
	assertTeradataWrite(t, "redshift", "CREATE TABLE z (a ST_GEOMETRY(1))", "CREATE TABLE z (a GEOMETRY(1))")
	assertRoundTrip(t, "teradata", "CREATE TABLE z (a SYSUDTLIB.INT)", "CREATE TABLE z (a SYSUDTLIB.INT)")
}

// TestTeradataCast ports test_cast (tests/dialects/test_teradata.py:227-253).
func TestTeradataCast(t *testing.T) {
	// read={"bigquery": ...}, write={teradata, bigquery, databricks, mysql, spark, ""}.
	assertTeradataRead(t, "bigquery", "CAST('1992-01' AS DATE FORMAT 'YYYY-DD')",
		"CAST('1992-01' AS DATE FORMAT 'YYYY-DD')")
	assertRoundTrip(t, "teradata", "CAST('1992-01' AS DATE FORMAT 'YYYY-DD')",
		"CAST('1992-01' AS DATE FORMAT 'YYYY-DD')")
	assertTeradataWrite(t, "bigquery", "CAST('1992-01' AS DATE FORMAT 'YYYY-DD')", "PARSE_DATE('%Y-%d', '1992-01')")
	assertTeradataWrite(t, "databricks", "CAST('1992-01' AS DATE FORMAT 'YYYY-DD')", "TO_DATE('1992-01', 'yyyy-d')")
	assertTeradataWrite(t, "mysql", "CAST('1992-01' AS DATE FORMAT 'YYYY-DD')", "STR_TO_DATE('1992-01', '%Y-%d')")
	assertTeradataWrite(t, "spark", "CAST('1992-01' AS DATE FORMAT 'YYYY-DD')", "TO_DATE('1992-01', 'yyyy-d')")
	assertTeradataWrite(t, "", "CAST('1992-01' AS DATE FORMAT 'YYYY-DD')", "STR_TO_DATE('1992-01', '%Y-%d')")

	assertRoundTrip(t, "teradata", "CAST('1992-01' AS FORMAT 'YYYY-DD')", "CAST('1992-01' AS FORMAT 'YYYY-DD')")

	// read={"snowflake": TRY_CAST...}, write={snowflake, teradata}.
	assertTeradataRead(t, "snowflake", "TRY_CAST('-2.5' AS DECIMAL(5, 2))", "TRYCAST('-2.5' AS DECIMAL(5, 2))")
	assertTeradataWrite(t, "snowflake", "TRYCAST('-2.5' AS DECIMAL(5, 2))", "TRY_CAST('-2.5' AS DECIMAL(5, 2))")
	assertRoundTrip(t, "teradata", "TRYCAST('-2.5' AS DECIMAL(5, 2))", "TRYCAST('-2.5' AS DECIMAL(5, 2))")
}

// TestTeradataFormatOverride ports test_format_override (tests/dialects/test_teradata.py:255-264).
func TestTeradataFormatOverride(t *testing.T) {
	for _, sql := range []string{
		"SELECT ('a' || 'b') (FORMAT '...')",
		"SELECT Col1 (FORMAT '+9999') FROM Test1",
		"SELECT date_col (FORMAT 'YYYY-MM-DD') FROM t",
	} {
		assertRoundTrip(t, "teradata", sql, sql)
	}
	assertRoundTrip(t, "teradata", "SELECT CAST(Col1 AS INTEGER) FROM Test1", "SELECT CAST(Col1 AS INT) FROM Test1")
}

// TestTeradataTime ports test_time (tests/dialects/test_teradata.py:266-355).
func TestTeradataTime(t *testing.T) {
	assertRoundTrip(t, "teradata", "CAST(CURRENT_TIMESTAMP(6) AS TIMESTAMP WITH TIME ZONE)",
		"CAST(CURRENT_TIMESTAMP(6) AS TIMESTAMP WITH TIME ZONE)")

	// TZ_TO_WITH_TIME_ZONE (generators/teradata.py:54): the timezone-aware types render
	// WITH TIME ZONE after their parameters — precision is kept, not dropped.
	assertRoundTrip(t, "teradata", "CAST(x AS TIMESTAMP(3) WITH TIME ZONE)",
		"CAST(x AS TIMESTAMP(3) WITH TIME ZONE)")
	assertRoundTrip(t, "teradata", "CAST(x AS TIMETZ)", "CAST(x AS TIMETZ WITH TIME ZONE)")

	assertRoundTrip(t, "teradata", "CURRENT_TIMESTAMP", "CURRENT_TIMESTAMP")
	assertTeradataRead(t, "snowflake", "CURRENT_TIMESTAMP()", "CURRENT_TIMESTAMP")

	// INTERVAL arithmetic: teradata identity, snowflake DATEADD and sqlite DATE_SUB reads.
	assertRoundTrip(t, "teradata", "SELECT '2023-01-01' + INTERVAL '5' YEAR", "SELECT '2023-01-01' + INTERVAL '5' YEAR")
	assertTeradataRead(t, "snowflake", "SELECT DATEADD(YEAR, 5, '2023-01-01')", "SELECT '2023-01-01' + INTERVAL '5' YEAR")
	assertRoundTrip(t, "teradata", "SELECT '2023-01-01' - INTERVAL '5' YEAR", "SELECT '2023-01-01' - INTERVAL '5' YEAR")
	assertTeradataRead(t, "snowflake", "SELECT DATEADD(YEAR, -5, '2023-01-01')", "SELECT '2023-01-01' - INTERVAL '5' YEAR")
	assertTeradataRead(t, "sqlite", "SELECT DATE_SUB('2023-01-01', 5, YEAR)", "SELECT '2023-01-01' - INTERVAL '5' YEAR")
	assertTeradataRead(t, "sqlite", "SELECT DATE_SUB('2023-01-01', -5, YEAR)", "SELECT '2023-01-01' + INTERVAL '5' YEAR")

	// WEEK/QUARTER intervals have no Teradata unit, so they render as DAY multipliers.
	assertRoundTrip(t, "teradata", "SELECT (90 * INTERVAL '1' DAY)", "SELECT (90 * INTERVAL '1' DAY)")
	assertTeradataRead(t, "snowflake", "SELECT INTERVAL '1' QUARTER", "SELECT (90 * INTERVAL '1' DAY)")
	assertRoundTrip(t, "teradata", "SELECT (7 * INTERVAL '1' DAY)", "SELECT (7 * INTERVAL '1' DAY)")
	assertTeradataRead(t, "snowflake", "SELECT INTERVAL '1' WEEK", "SELECT (7 * INTERVAL '1' DAY)")
	assertRoundTrip(t, "teradata", "SELECT '2023-01-01' + (90 * INTERVAL '5' DAY)", "SELECT '2023-01-01' + (90 * INTERVAL '5' DAY)")
	assertTeradataRead(t, "snowflake", "SELECT DATEADD(QUARTER, 5, '2023-01-01')", "SELECT '2023-01-01' + (90 * INTERVAL '5' DAY)")
	assertRoundTrip(t, "teradata", "SELECT '2023-01-01' + (7 * INTERVAL '5' DAY)", "SELECT '2023-01-01' + (7 * INTERVAL '5' DAY)")
	assertTeradataRead(t, "snowflake", "SELECT DATEADD(WEEK, 5, '2023-01-01')", "SELECT '2023-01-01' + (7 * INTERVAL '5' DAY)")

	// QUARTER renders as CAST(TO_CHAR(x, 'Q') AS INT) on generate.
	assertRoundTrip(t, "teradata", "CAST(TO_CHAR(x, 'Q') AS INT)", "CAST(TO_CHAR(x, 'Q') AS INT)")
	assertTeradataRead(t, "snowflake", "DATE_PART(QUARTER, x)", "CAST(TO_CHAR(x, 'Q') AS INT)")
	assertTeradataRead(t, "bigquery", "EXTRACT(QUARTER FROM x)", "CAST(TO_CHAR(x, 'Q') AS INT)")
	assertRoundTrip(t, "teradata", "EXTRACT(MONTH FROM x)", "EXTRACT(MONTH FROM x)")
	assertTeradataRead(t, "snowflake", "DATE_PART(MONTH, x)", "EXTRACT(MONTH FROM x)")
	assertTeradataRead(t, "bigquery", "EXTRACT(MONTH FROM x)", "EXTRACT(MONTH FROM x)")
	assertTeradataRead(t, "snowflake", "quarter(x)", "CAST(TO_CHAR(x, 'Q') AS INT)")
}

// TestTeradataQueryBand ports test_query_band (tests/dialects/test_teradata.py:357-369).
func TestTeradataQueryBand(t *testing.T) {
	for _, sql := range []string{
		"SET QUERY_BAND = 'app=myapp;' FOR SESSION",
		"SET QUERY_BAND = 'app=myapp;user=john;' FOR TRANSACTION",
		"SET QUERY_BAND = 'priority=high;' UPDATE FOR SESSION",
		"SET QUERY_BAND = 'workload=batch;' UPDATE FOR TRANSACTION",
		"SET QUERY_BAND = 'org=Finance;report=Fin123;' FOR SESSION",
		"SET QUERY_BAND = NONE FOR SESSION",
		"SET QUERY_BAND = NONE FOR SESSION VOLATILE",
		"SET QUERY_BAND = 'priority=high;' UPDATE FOR SESSION VOLATILE",
		// Quoted NONE should remain quoted.
		"SET QUERY_BAND = 'NONE' FOR SESSION",
		"SET QUERY_BAND = '' FOR SESSION",
	} {
		assertRoundTrip(t, "teradata", sql, sql)
	}
}

// TestTeradataSetScopedForms guards the SET_PARSERS init order (parsers/teradata.py:135-140
// extends the base table; the copy must run after stmt_set.go builds it): a scoped
// assignment parses to a structured Set, exactly as upstream, instead of degrading to a
// raw Command when only QUERY_BAND is in the trie.
func TestTeradataSetScopedForms(t *testing.T) {
	for _, sql := range []string{"SET SESSION x = 1", "SET LOCAL x = 1", "SET x = 1"} {
		e, err := sqlglot.ParseOne(sql, "teradata")
		if err != nil {
			t.Fatalf("ParseOne(%q): %v", sql, err)
		}
		if got := expressions.ClassName(e.Kind()); got != "Set" {
			t.Errorf("%q parsed to %s, want Set", sql, got)
		}
		assertRoundTrip(t, "teradata", sql, sql)
	}
}

// TestTeradataSemiAntiWhere guards the SEMI/ANTI -> EXISTS rewrite's WHERE merge: the new
// EXISTS condition is combined through and_, which parenthesizes a Connector operand, so an
// existing OR body cannot re-associate past the AND (transforms.py:616-631 ->
// core.py:2775-2783).
func TestTeradataSemiAntiWhere(t *testing.T) {
	cases := map[string]string{
		"SELECT * FROM t1 SEMI JOIN t2 ON t1.id = t2.id":                "SELECT * FROM t1 WHERE EXISTS(SELECT 1 FROM t2 WHERE t1.id = t2.id)",
		"SELECT * FROM t1 WHERE t1.y > 0 SEMI JOIN t2 ON t1.id = t2.id": "SELECT * FROM t1 WHERE t1.y > 0 AND EXISTS(SELECT 1 FROM t2 WHERE t1.id = t2.id)",
		"SELECT * FROM t1 WHERE a OR b SEMI JOIN t2 ON t1.id = t2.id":   "SELECT * FROM t1 WHERE (a OR b) AND EXISTS(SELECT 1 FROM t2 WHERE t1.id = t2.id)",
		"SELECT * FROM t1 WHERE a AND b SEMI JOIN t2 ON t1.id = t2.id":  "SELECT * FROM t1 WHERE (a AND b) AND EXISTS(SELECT 1 FROM t2 WHERE t1.id = t2.id)",
		"SELECT * FROM t1 WHERE a OR b ANTI JOIN t2 ON t1.id = t2.id":   "SELECT * FROM t1 WHERE (a OR b) AND NOT EXISTS(SELECT 1 FROM t2 WHERE t1.id = t2.id)",
	}
	for sql, want := range cases {
		assertTeradataRead(t, "teradata", sql, want)
	}
}

// TestTeradataStrToDateFormat guards the FORMAT literal rendering (generators/teradata.py:
// 84-86): format_time runs on the SQL-rendered format, so quote doubling survives the
// mapping (a quote inside the format cannot break the literal) and a non-literal format
// renders as its converted SQL instead of being dropped.
func TestTeradataStrToDateFormat(t *testing.T) {
	assertTeradataRead(t, "", "SELECT STR_TO_DATE(x, '%Y''%m')",
		"SELECT CAST(x AS DATE FORMAT 'YYYY''MM')")
	assertTeradataRead(t, "hive", "SELECT STR_TO_DATE(x, '%Y-%m')",
		"SELECT CAST(x AS DATE FORMAT 'YYYY-MM')")
	assertTeradataRead(t, "hive", "SELECT STR_TO_DATE(x, '%Y''%m')",
		"SELECT CAST(x AS DATE FORMAT CONCAT('YYYY', 'MM'))")
}

// TestTeradataDateAdd guards _date_add_sql (generators/teradata.py:19-39): a literal amount
// renders quoted, a non-literal amount stays bare (reported unsupported) instead of being
// frozen into a string literal, and a Neg operand flips the operator.
func TestTeradataDateAdd(t *testing.T) {
	assertTeradataRead(t, "bigquery", "SELECT DATE_ADD(d, INTERVAL 5 DAY)",
		"SELECT d + INTERVAL '5' DAY")
	assertTeradataRead(t, "bigquery", "SELECT DATE_ADD(d, INTERVAL -5 DAY)",
		"SELECT d + INTERVAL '-5' DAY")
	assertTeradataRead(t, "bigquery", "SELECT DATE_SUB(d, INTERVAL -5 DAY)",
		"SELECT d - INTERVAL '-5' DAY")
	assertTeradataRead(t, "bigquery", "SELECT DATE_ADD(d, INTERVAL n DAY)",
		"SELECT d + INTERVAL n DAY")
	// Upstream crashes here (ValueError from to_py); the port keeps the operand bare with
	// the flipped operator (DEVIATIONS.md §1.31).
	assertTeradataRead(t, "bigquery", "SELECT DATE_ADD(d, INTERVAL -n DAY)",
		"SELECT d - INTERVAL n DAY")
	// Teradata's own parser keeps the whole INTERVAL node as the amount, so it nests.
	assertTeradataRead(t, "teradata", "SELECT DATE_ADD(d, INTERVAL 5 DAY)",
		"SELECT d + INTERVAL (INTERVAL '5' DAY)")
	assertTeradataRead(t, "teradata", "SELECT DATE_ADD(d, INTERVAL n DAY)",
		"SELECT d + INTERVAL (INTERVAL n DAY)")
	// A foldable amount stays unfolded: the port cannot call the optimizer's simplify
	// from the generator (import cycle), where upstream folds 1 + 1 to 2 (DEVIATIONS.md
	// §1.31).
	assertTeradataRead(t, "bigquery", "SELECT DATE_ADD(d, INTERVAL 1 + 1 DAY)",
		"SELECT d + INTERVAL (1 + 1) DAY")
}
