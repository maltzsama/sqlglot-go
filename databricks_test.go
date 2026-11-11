package sqlglot_test

import (
	"testing"
)

// TestDatabricksJSON ports test_json (tests/dialects/test_databricks.py:303-340).
func TestDatabricksJSON(t *testing.T) {
	cases := []struct{ sql, want string }{
		{"SELECT c1:price, c1:price.foo, c1:price.bar[1]", "SELECT c1:price, c1:price.foo, c1:price.bar[1]"},
		{"SELECT TRY_CAST(c1:price AS ARRAY<VARIANT>)", "SELECT TRY_CAST(c1:price AS ARRAY<VARIANT>)"},
		{`SELECT TRY_CAST(c1:["foo bar"]["baz qux"] AS ARRAY<VARIANT>)`, `SELECT TRY_CAST(c1:["foo bar"]["baz qux"] AS ARRAY<VARIANT>)`},
		{`SELECT c1:item[1].price FROM VALUES ('{ "item": [ { "model" : "basic", "price" : 6.12 } ] }') AS T(c1)`, `SELECT c1:item[1].price FROM VALUES ('{ "item": [ { "model" : "basic", "price" : 6.12 } ] }') AS T(c1)`},
		{`SELECT c1:item[*].price FROM VALUES ('{ "item": [ { "model" : "basic", "price" : 6.12 } ] }') AS T(c1)`, `SELECT c1:item[*].price FROM VALUES ('{ "item": [ { "model" : "basic", "price" : 6.12 } ] }') AS T(c1)`},
		{`SELECT FROM_JSON(c1:item[*].price, 'ARRAY<DOUBLE>')[0] FROM VALUES ('{ "item": [ { "model" : "basic", "price" : 6.12 } ] }') AS T(c1)`, `SELECT FROM_JSON(c1:item[*].price, 'ARRAY<DOUBLE>')[0] FROM VALUES ('{ "item": [ { "model" : "basic", "price" : 6.12 } ] }') AS T(c1)`},
		{`SELECT INLINE(FROM_JSON(c1:item[*], 'ARRAY<STRUCT<model STRING, price DOUBLE>>')) FROM VALUES ('{ "item": [ { "model" : "basic", "price" : 6.12 } ] }') AS T(c1)`, `SELECT INLINE(FROM_JSON(c1:item[*], 'ARRAY<STRUCT<model STRING, price DOUBLE>>')) FROM VALUES ('{ "item": [ { "model" : "basic", "price" : 6.12 } ] }') AS T(c1)`},
		{`SELECT c1:['price'] FROM VALUES ('{ "price": 5 }') AS T(c1)`, `SELECT c1:["price"] FROM VALUES ('{ "price": 5 }') AS T(c1)`},
		{`SELECT GET_JSON_OBJECT(c1, '$.price') FROM VALUES ('{ "price": 5 }') AS T(c1)`, `SELECT GET_JSON_OBJECT(c1, '$.price') FROM VALUES ('{ "price": 5 }') AS T(c1)`},
		{"SELECT GET_JSON_OBJECT(col, path_col)", "SELECT GET_JSON_OBJECT(col, path_col)"},
		{"SELECT GET_JSON_OBJECT(col, CONCAT('$.', field_name))", "SELECT GET_JSON_OBJECT(col, CONCAT('$.', field_name))"},
		{"SELECT GET_JSON_OBJECT(GET_JSON_OBJECT(col, '$[0]'), '$.a')", "SELECT GET_JSON_OBJECT(GET_JSON_OBJECT(col, '$[0]'), '$.a')"},
		{`SELECT raw:` + "`zip code`" + `, raw:` + "`fb:testid`" + `, raw:store['bicycle'], raw:store["zip code"]`,
			`SELECT raw:["zip code"], raw:["fb:testid"], raw:store["bicycle"], raw:store["zip code"]`},
		{"SELECT col:`fr'uit`", "SELECT col:[\"fr'uit\"]"},
	}
	for _, c := range cases {
		assertRoundTrip(t, "databricks", c.sql, c.want)
	}
}

// TestDatabricksDatediff ports test_datediff (databricks write arm).
func TestDatabricksDatediff(t *testing.T) {
	for _, unit := range []string{"year", "quarter", "month", "week", "day", "hour", "minute", "second", "millisecond", "microsecond"} {
		sql := "SELECT DATEDIFF(" + unit + ", 'start', 'end')"
		want := "SELECT DATEDIFF(" + upperASCII(unit) + ", 'start', 'end')"
		assertRoundTrip(t, "databricks", sql, want)
	}
	assertRoundTrip(t, "databricks", "SELECT DATEDIFF('end', 'start')", "SELECT DATEDIFF(DAY, 'start', 'end')")
}

// TestDatabricksAddDate ports test_add_date.
func TestDatabricksAddDate(t *testing.T) {
	assertRoundTrip(t, "databricks", "SELECT DATEADD(year, 1, '2020-01-01')", "SELECT DATE_ADD(YEAR, 1, '2020-01-01')")
	assertRoundTrip(t, "databricks", "SELECT DATE_ADD('2020-01-01', 1)", "SELECT DATE_ADD('2020-01-01', 1)")
	assertRoundTrip(t, "databricks", "SELECT DATE_ADD(MONTH, 1, '2020-01-01')", "SELECT DATE_ADD(MONTH, 1, '2020-01-01')")
	assertRoundTrip(t, "databricks", "SELECT DATEADD(e, 24) FROM t", "SELECT DATE_ADD(e, 24) FROM t")
}

// TestDatabricksWithoutAs ports test_without_as.
func TestDatabricksWithoutAs(t *testing.T) {
	assertRoundTrip(t, "databricks", "CREATE TABLE x (SELECT 1)", "CREATE TABLE x AS (SELECT 1)")
	assertRoundTrip(t, "databricks", "WITH x (select 1) SELECT * FROM x", "WITH x AS (SELECT 1) SELECT * FROM x")
}

// TestDatabricksStreamingTables ports test_streaming_tables.
func TestDatabricksStreamingTables(t *testing.T) {
	for _, sql := range []string{
		"CREATE STREAMING TABLE raw_data AS SELECT * FROM STREAM READ_FILES('abfss://container@storageAccount.dfs.core.windows.net/base/path')",
		"CREATE OR REFRESH STREAMING TABLE csv_data (id INT, ts TIMESTAMP, event STRING) AS SELECT * FROM STREAM READ_FILES('s3://bucket/path', format => 'csv', schema => 'id int, ts timestamp, event string')",
	} {
		assertRoundTrip(t, "databricks", sql, sql)
	}
}

// TestDatabricksGrantRevoke ports test_grant and test_revoke.
func TestDatabricksGrantRevoke(t *testing.T) {
	for _, sql := range []string{
		"GRANT CREATE ON SCHEMA my_schema TO `alf@melmak.et`",
		"GRANT SELECT ON TABLE sample_data TO `alf@melmak.et`",
		"GRANT ALL PRIVILEGES ON TABLE forecasts TO finance",
		"GRANT SELECT ON TABLE t TO `fab9e00e-ca35-11ec-9d64-0242ac120002`",
		"REVOKE CREATE ON SCHEMA my_schema FROM `alf@melmak.et`",
		"REVOKE SELECT ON TABLE sample_data FROM `alf@melmak.et`",
		"REVOKE ALL PRIVILEGES ON TABLE forecasts FROM finance",
		"REVOKE SELECT ON TABLE t FROM `fab9e00e-ca35-11ec-9d64-0242ac120002`",
	} {
		assertRoundTrip(t, "databricks", sql, sql)
	}
}

// TestDatabricksAnalyze ports test_analyze.
func TestDatabricksAnalyze(t *testing.T) {
	for _, sql := range []string{
		"ANALYZE TABLE tbl COMPUTE DELTA STATISTICS NOSCAN",
		"ANALYZE TABLE tbl COMPUTE DELTA STATISTICS FOR ALL COLUMNS",
		"ANALYZE TABLE tbl COMPUTE DELTA STATISTICS FOR COLUMNS foo, bar",
		"ANALYZE TABLE ctlg.db.tbl COMPUTE DELTA STATISTICS NOSCAN",
		"ANALYZE TABLES COMPUTE STATISTICS NOSCAN",
		"ANALYZE TABLES FROM db COMPUTE STATISTICS",
		"ANALYZE TABLES IN db COMPUTE STATISTICS",
		"ANALYZE TABLE ctlg.db.tbl PARTITION(foo = 'foo', bar = 'bar') COMPUTE STATISTICS NOSCAN",
	} {
		assertRoundTrip(t, "databricks", sql, sql)
	}
}

// TestDatabricksUDFProperties ports the UDF property tests.
func TestDatabricksUDFProperties(t *testing.T) {
	for _, sql := range []string{
		`CREATE FUNCTION a() ENVIRONMENT (dependencies = '["foo1==1", "foo2==2"]', environment_version = 'None')`,
		`CREATE FUNCTION a() HANDLER 'handler_function'`,
		`CREATE FUNCTION a() PARAMETER STYLE PANDAS`,
	} {
		assertRoundTrip(t, "databricks", sql, sql)
	}
}

// TestDatabricksQdcolon ports test_qdcolon.
func TestDatabricksQdcolon(t *testing.T) {
	assertRoundTrip(t, "databricks", "SELECT '20'?::INTEGER", "SELECT TRY_CAST('20' AS INT)")
}

// TestDatabricksOverlay ports test_overlay.
func TestDatabricksOverlay(t *testing.T) {
	assertRoundTrip(t, "databricks", "SELECT OVERLAY('Spark SQL', 'ANSI ', 7, 0)",
		"SELECT OVERLAY('Spark SQL' PLACING 'ANSI ' FROM 7 FOR 0)")
	assertRoundTrip(t, "databricks", "SELECT OVERLAY('Spark SQL' PLACING 'CORE' FROM 7)",
		"SELECT OVERLAY('Spark SQL' PLACING 'CORE' FROM 7)")
	assertRoundTrip(t, "databricks", "SELECT OVERLAY(ENCODE('Spark SQL', 'utf-8') PLACING ENCODE('_', 'utf-8') FROM 6)",
		"SELECT OVERLAY(ENCODE('Spark SQL', 'utf-8') PLACING ENCODE('_', 'utf-8') FROM 6)")
	assertRoundTrip(t, "databricks", "SELECT OVERLAY('Spark SQL' PLACING 'ANSI ' FROM 7 FOR 0)",
		"SELECT OVERLAY('Spark SQL' PLACING 'ANSI ' FROM 7 FOR 0)")
}

// TestDatabricksSetVariable ports test_set_variable.
func TestDatabricksSetVariable(t *testing.T) {
	cases := []struct{ sql, want string }{
		{"SET VAR v = 5", "SET VARIABLE v = 5"},
		{"SET VARIABLE v = 5", "SET VARIABLE v = 5"},
		{"SET VARIABLE v1 = 1, v2 = '2'", "SET VARIABLE v1 = 1, v2 = '2'"},
		{"SET VARIABLE (v1, v2) = (SELECT 1, 2)", "SET VARIABLE (v1, v2) = (SELECT 1, 2)"},
		{"SET VARIABLE v = (SELECT MAX(c1) FROM VALUES (1), (2) AS T(c1))", "SET VARIABLE v = (SELECT MAX(c1) FROM VALUES (1), (2) AS T(c1))"},
		{"SET VARIABLE v = DEFAULT", "SET VARIABLE v = DEFAULT"},
	}
	for _, c := range cases {
		assertRoundTrip(t, "databricks", c.sql, c.want)
	}
}

// TestDatabricksIff ports test_iff: IFF normalizes to IF.
func TestDatabricksIff(t *testing.T) {
	assertRoundTrip(t, "databricks", "SELECT IFF(x > 0, 'positive', 'non-positive')",
		"SELECT IF(x > 0, 'positive', 'non-positive')")
}

// TestDatabricksTryDivide ports test_try_divide.
func TestDatabricksTryDivide(t *testing.T) {
	assertRoundTrip(t, "databricks", "SELECT TRY_DIVIDE(a, b)", "SELECT TRY_DIVIDE(a, b)")
	assertRoundTrip(t, "spark", "SELECT TRY_DIVIDE(a, b)", "SELECT TRY_DIVIDE(a, b)")
}

// TestDatabricksDeclare ports test_declare.
func TestDatabricksDeclare(t *testing.T) {
	cases := []struct{ sql, want string }{
		{"DECLARE VAR x INT", "DECLARE x INT"},
		{"DECLARE x INT", "DECLARE x INT"},
		{"DECLARE VARIABLE myvar INT DEFAULT 1", "DECLARE myvar INT = 1"},
		{"DECLARE x, y, z INT DEFAULT 1", "DECLARE x, y, z INT = 1"},
		{"DECLARE x INT = 1", "DECLARE x INT = 1"},
		{"DECLARE OR REPLACE x INT = 1", "DECLARE OR REPLACE x INT = 1"},
	}
	for _, c := range cases {
		assertRoundTrip(t, "databricks", c.sql, c.want)
	}
}

// TestDatabricksInsertReplaceUsing ports test_insert_replace_using.
func TestDatabricksInsertReplaceUsing(t *testing.T) {
	sql := "INSERT INTO target REPLACE USING (c1, c2) SELECT c1, c2 FROM source"
	assertRoundTrip(t, "databricks", sql, sql)
}

// upperASCII returns s with its ASCII letters uppercased.
func upperASCII(s string) string {
	out := []byte(s)
	for i, b := range out {
		if b >= 'a' && b <= 'z' {
			out[i] = b - 'a' + 'A'
		}
	}
	return string(out)
}
