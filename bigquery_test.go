package sqlglot_test

import (
	"strings"
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestBigQueryCore ports the same-dialect round-trip core of test_bigquery
// (tests/dialects/test_bigquery.py:26): the constructs the port supports round-trip unchanged.
func TestBigQueryCore(t *testing.T) {
	for _, sql := range []string{
		"SELECT 1",
		"SELECT * FROM foo",
		"SELECT * FROM foo WHERE x > 1",
		"SELECT CAST(x AS INT64) FROM t",
		"SELECT CAST(x AS FLOAT64)",
		"SELECT CAST(x AS BOOL)",
		"SELECT CAST(x AS BYTES)",
		"SELECT CAST(x AS BIGNUMERIC)",
		"SELECT CAST(x AS ANY TYPE)",
		"SELECT [1, 2, 3]",
		"SELECT ARRAY(SELECT 1)",
		"SELECT STRUCT(1 AS a, 'x' AS b)",
		"SELECT SAFE_CAST(x AS INT64)",
		"SELECT IF(x, 1, NULL)",
		"SELECT COUNTIF(x)",
		"SELECT GENERATE_UUID()",
		"SELECT APPROX_COUNT_DISTINCT(x)",
		"SELECT ARRAY_AGG(x IGNORE NULLS)",
		"SELECT ARRAY_AGG(DISTINCT x IGNORE NULLS ORDER BY x LIMIT 1)",
		"SELECT SUM(x) OVER (PARTITION BY y ORDER BY z)",
		"DECLARE x INT64 DEFAULT 1",
		"DECLARE x, y, z INT64",
		"BEGIN TRANSACTION",
		"COMMIT TRANSACTION",
		"ROLLBACK TRANSACTION",
	} {
		assertRoundTrip(t, "bigquery", sql, sql)
	}
}

// TestBigQueryDateFunctions ports the temporal-function round-trips.
func TestBigQueryDateFunctions(t *testing.T) {
	for _, sql := range []string{
		"SELECT DATE_ADD(CURRENT_DATE, INTERVAL '1' DAY)",
		"SELECT DATE_SUB(CURRENT_DATE, INTERVAL '1' WEEK)",
		"SELECT DATETIME_ADD('2023-01-01T00:00:00', INTERVAL '1' MILLISECOND)",
		"SELECT TIMESTAMP_ADD(CAST('2008-12-25 15:30:00+00' AS TIMESTAMP), INTERVAL '10' MINUTE)",
		"SELECT DATE_DIFF('2017-12-18', '2017-12-17', WEEK)",
		"SELECT DATE_DIFF('2017-12-18', '2017-12-17', WEEK(SATURDAY))",
		"SELECT DATE_TRUNC(x, WEEK)",
		"SELECT DATE_TRUNC(x, WEEK)",
		"SELECT DATETIME_TRUNC(CAST('2008-11-10 14:30:00' AS DATETIME), WEEK)",
		"SELECT TIMESTAMP_TRUNC(CAST('2008-12-25 15:30:00+00' AS TIMESTAMP), DAY, 'UTC')",
		"SELECT DATETIME_DIFF(CAST('2021-02-01 00:00:00' AS DATETIME), CAST('2021-01-31 00:00:00' AS DATETIME), MONTH)",
		"SELECT GENERATE_DATE_ARRAY('2016-10-05', '2016-10-08', INTERVAL '1' DAY)",
		"SELECT GENERATE_DATE_ARRAY('2016-10-05', '2016-10-08', INTERVAL '1' MONTH)",
		"SELECT TIMESTAMP_SECONDS(60)",
		"SELECT TIMESTAMP_MILLIS(60)",
		"SELECT TIMESTAMP_MICROS(60)",
		"SELECT UNIX_DATE(x)",
		"SELECT DATE_FROM_UNIX_DATE(x)",
		"SELECT FORMAT_DATE('%F', x)",
		"SELECT PARSE_DATE('%F', '2023-12-25')",
		"SELECT PARSE_TIMESTAMP('%F %T', '2023-12-25 15:30:00', 'UTC')",
		"SELECT PARSE_TIME('%T', '15:30:00')",
		"SELECT FORMAT_DATETIME('%D', '2023-12-25 15:30:00')",
		"SELECT LAST_DAY(x, MONTH)",
		"SELECT CURRENT_DATETIME()",
	} {
		assertRoundTrip(t, "bigquery", sql, sql)
	}
}

// TestBigQueryJSONObject ports test_json_object (tests/dialects/test_bigquery.py:2738).
func TestBigQueryJSONObject(t *testing.T) {
	for _, sql := range []string{
		"SELECT JSON_OBJECT('a', 10, 'a', 'foo')",
		"SELECT JSON_OBJECT('foo', 10, 'bar', TRUE)",
		"SELECT JSON_OBJECT('foo', 10, 'bar', ['a', 'b'])",
		"SELECT JSON_OBJECT('a', 10, 'b', NULL)",
		"SELECT JSON_OBJECT('a', PARSE_JSON('10'), 'b', PARSE_JSON('\"foo\"'))",
		"SELECT JSON_OBJECT('a', STRUCT(10 AS id, 'Red' AS color), 'b', STRUCT(20 AS id, 'Blue' AS color))",
		"SELECT JSON_OBJECT('a', TO_JSON(10), 'b', TO_JSON(['foo', 'bar']))",
	} {
		assertRoundTrip(t, "bigquery", sql, sql)
	}
}

// TestBigQueryJSONFunctions ports the JSON function round-trips.
func TestBigQueryJSONFunctions(t *testing.T) {
	for _, sql := range []string{
		"SELECT JSON_EXTRACT(doc, '$.a')",
		"SELECT JSON_EXTRACT_SCALAR(doc, '$.a')",
		"SELECT JSON_EXTRACT_ARRAY(doc, '$.a')",
		"SELECT JSON_QUERY(doc, '$.a')",
		"SELECT JSON_QUERY_ARRAY(doc, '$.a')",
		"SELECT JSON_VALUE(doc, '$.a')",
		"SELECT JSON_VALUE_ARRAY(doc, '$.a')",
		"SELECT JSON_KEYS(x, 2)",
		"SELECT JSON_STRIP_NULLS(x)",
		"SELECT TO_JSON_STRING(x)",
		"SELECT TO_JSON(x)",
		"SELECT BOOL(x)",
		"SELECT JSON_ARRAY(1, 2, 3)",
	} {
		assertRoundTrip(t, "bigquery", sql, sql)
	}
}

// TestBigQueryStringAgg ports test_string_agg (tests/dialects/test_bigquery.py:3275).
func TestBigQueryStringAgg(t *testing.T) {
	for _, sql := range []string{
		"STRING_AGG(DISTINCT a ORDER BY b DESC, c DESC LIMIT 10)",
		"STRING_AGG(DISTINCT v, sep LIMIT 3)",
		"STRING_AGG(a, ' & ' ORDER BY LENGTH(a))",
		"STRING_AGG(foo, b'|' ORDER BY bar)",
	} {
		assertRoundTrip(t, "bigquery", sql, sql)
	}
}

// TestBigQuerySetOperations ports test_set_operations (tests/dialects/test_bigquery.py:3303).
func TestBigQuerySetOperations(t *testing.T) {
	for _, sql := range []string{
		"SELECT 1 AS foo UNION ALL SELECT 3 AS foo",
		"SELECT 1 AS foo UNION ALL BY NAME SELECT 3 AS foo, 4 AS bar",
		"SELECT 1 AS foo UNION ALL BY NAME ON (foo, bar) SELECT 3 AS foo, 4 AS bar",
		"SELECT 1 AS foo FULL OUTER UNION ALL BY NAME SELECT 3 AS foo, 4 AS bar",
		"SELECT * FROM x EXCEPT DISTINCT SELECT * FROM y",
		"SELECT * FROM x INTERSECT DISTINCT SELECT * FROM y",
	} {
		assertRoundTrip(t, "bigquery", sql, sql)
	}
}

// TestBigQueryWithOffset ports test_with_offset and test_unnest_with_offset
// (tests/dialects/test_bigquery.py:3343, 3497).
func TestBigQueryWithOffset(t *testing.T) {
	for _, sql := range []string{
		"SELECT * FROM UNNEST(x) WITH OFFSET AS offset",
		"SELECT * FROM UNNEST(x) WITH OFFSET AS pos",
		"SELECT * FROM t CROSS JOIN UNNEST(t.arr) AS x WITH OFFSET AS pos",
		"SELECT * FROM t1 CROSS JOIN UNNEST([1, 2]) AS hit WITH OFFSET AS offset LEFT JOIN foo",
	} {
		assertRoundTrip(t, "bigquery", sql, sql)
	}
}

// TestBigQuerySelectAsStruct ports test_select_as_struct (tests/dialects/test_bigquery.py:3453).
func TestBigQuerySelectAsStruct(t *testing.T) {
	for _, sql := range []string{
		"SELECT AS STRUCT 1 AS a, 2 AS b",
		"SELECT AS STRUCT * FROM t",
		"SELECT AS STRUCT x.* FROM t",
	} {
		assertRoundTrip(t, "bigquery", sql, sql)
	}
}

// TestBigQueryUnnest ports test_unnest (tests/dialects/test_bigquery.py:2852).
func TestBigQueryUnnest(t *testing.T) {
	for _, sql := range []string{
		"SELECT * FROM UNNEST([1, 2, 3]) AS x",
		"SELECT * FROM UNNEST([1, 2, 3]) AS x WITH OFFSET AS offset",
		"SELECT * FROM UNNEST([1, 2, 3])",
		"SELECT * FROM t CROSS JOIN UNNEST(t.arr) AS x",
	} {
		assertRoundTrip(t, "bigquery", sql, sql)
	}
}

// TestBigQueryPivotUnpivot ports the pivot/unpivot round-trips.
func TestBigQueryPivotUnpivot(t *testing.T) {
	for _, sql := range []string{
		"SELECT * FROM Produce PIVOT(SUM(sales) FOR quarter IN ('Q1', 'Q2'))",
		"SELECT * FROM Produce UNPIVOT((first_half_sales, second_half_sales) FOR semesters IN ((Q1, Q2) AS 'semester_1', (Q3, Q4) AS 'semester_2'))",
		"SELECT * FROM Produce UNPIVOT((first_half_sales, second_half_sales) FOR semesters IN ((Q1, Q2) AS 1, (Q3, Q4) AS 2))",
	} {
		assertRoundTrip(t, "bigquery", sql, sql)
	}
}

// TestBigQueryInformationSchema ports the INFORMATION_SCHEMA table handling: the view name is
// merged into one quoted identifier and aliased (parsers/bigquery.py:484-514).
func TestBigQueryInformationSchema(t *testing.T) {
	const sql = "SELECT * FROM proj.region_or_dataset.INFORMATION_SCHEMA.TABLES"
	const want = "SELECT * FROM proj.region_or_dataset.`INFORMATION_SCHEMA.TABLES` AS TABLES"
	expr, err := sqlglot.ParseOne(sql, "bigquery")
	if err != nil {
		t.Fatal(err)
	}
	got, err := sqlglot.Generate(expr, "bigquery", generator.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// TestBigQueryQuotedTable ports test_quoted_identifier_meta (tests/dialects/test_bigquery.py:3393):
// a backtick-quoted dotted path is one quoted identifier.
func TestBigQueryQuotedTable(t *testing.T) {
	const sql = "SELECT * FROM `a.b.com:project-id.region-us.INFORMATION_SCHEMA.JOBS`"
	const want = "SELECT * FROM `a.b.com:project-id.region-us.INFORMATION_SCHEMA.JOBS` AS `a.b.com:project-id.region-us.INFORMATION_SCHEMA.JOBS`"
	assertRoundTrip(t, "bigquery", sql, want)

	const qualified = "SELECT * FROM `domain.com:project-id.region-us.INFORMATION_SCHEMA`.JOBS"
	const qualifiedWant = "SELECT * FROM `domain.com:project-id.region-us.INFORMATION_SCHEMA.JOBS` AS JOBS"
	assertRoundTrip(t, "bigquery", qualified, qualifiedWant)
}

// TestBigQueryQuotedColumn ports the quoted column path split (parsers/bigquery.py:518-536).
func TestBigQueryQuotedColumn(t *testing.T) {
	const sql = "SELECT `my-project.mydataset.mytable`.`my-column` FROM `my-project.mydataset.mytable`"
	assertRoundTrip(t, "bigquery", sql, sql)
}

// TestBigQueryWildcardTableParts ports the wildcard table suffix.
func TestBigQueryWildcardTableParts(t *testing.T) {
	for _, sql := range []string{
		"SELECT * FROM x.*",
		"SELECT * FROM x.y*",
		"SELECT * FROM dataset.table_*",
	} {
		assertRoundTrip(t, "bigquery", sql, sql)
	}
}

// TestBigQueryDashedTableNames ports the dashed/numeric table-name grammar
// (parsers/bigquery.py:402-425).
func TestBigQueryDashedTableNames(t *testing.T) {
	for _, sql := range []string{
		"SELECT * FROM my-table",
		"SELECT * FROM my-project.mydataset.mytable",
		"SELECT * FROM abc-def-ghi",
		"SELECT * FROM foo.bar.`25`",
		"SELECT * FROM foo.bar.`25x` AS a",
		"SELECT * FROM x-0.a",
	} {
		assertRoundTrip(t, "bigquery", sql, sql)
	}
}

// TestBigQuerySafeAndNetFuncs ports the SAFE./NET. qualified-call re-parse
// (parsers/bigquery.py:773-799).
func TestBigQuerySafeAndNetFuncs(t *testing.T) {
	for _, sql := range []string{
		"SELECT SAFE.PARSE_DATE('%F', '2024-01-15')",
		"SELECT SAFE.PARSE_DATETIME('%F %T', '2024-01-15 10:30:00')",
		"SELECT SAFE.PARSE_TIMESTAMP('%F %T', '2024-01-15 10:30:00')",
		"SELECT NET.IPV4_TO_INT64(x)",
	} {
		assertRoundTrip(t, "bigquery", sql, sql)
	}
}

// TestBigQueryCastFormat ports test_cast_format_with_parentheses
// (tests/dialects/test_bigquery.py:3249) and the FORMAT/AT TIME ZONE grammar.
func TestBigQueryCastFormat(t *testing.T) {
	cases := map[string]string{
		"CAST(encrypted_value AS STRING FORMAT 'BASE64')":                                        "CAST(encrypted_value AS STRING FORMAT 'BASE64')",
		"SAFE_CAST(some_date AS DATE FORMAT 'DD MONTH YYYY')":                                    "SAFE_CAST(some_date AS DATE FORMAT 'DD MONTH YYYY')",
		"SELECT CAST(date AS STRING FORMAT ('YYYY'))":                                            "SELECT CAST(date AS STRING FORMAT 'YYYY')",
		"SELECT CAST(timestamp AS STRING FORMAT ('YYYY-MM-DD') AT TIME ZONE 'UTC')":              "SELECT CAST(timestamp AS STRING FORMAT 'YYYY-MM-DD' AT TIME ZONE 'UTC')",
		"SELECT CAST('20201225' AS TIMESTAMP FORMAT 'YYYYMMDD' AT TIME ZONE 'America/New_York')": "SELECT PARSE_TIMESTAMP('%Y%m%d', '20201225', 'America/New_York')",
	}
	for sql, want := range cases {
		assertRoundTrip(t, "bigquery", sql, want)
	}
}

// TestBigQueryRemovePrecisionParameterizedTypes ports
// test_remove_precision_parameterized_types (tests/dialects/test_bigquery.py:2502).
func TestBigQueryRemovePrecisionParameterizedTypes(t *testing.T) {
	assertRoundTrip(t, "bigquery", "SELECT CAST(1 AS NUMERIC(10, 2))", "SELECT CAST(1 AS NUMERIC)")
	assertRoundTrip(t, "bigquery", "SELECT CAST(1 AS BIGNUMERIC(10, 2))", "SELECT CAST(1 AS BIGNUMERIC)")
	assertRoundTrip(t, "bigquery", "SELECT CAST(1 AS STRING(10))", "SELECT CAST(1 AS STRING)")
}

// TestBigQueryHavingMax ports the ANY_VALUE/ARRAY_AGG HAVING MAX/MIN grammar.
func TestBigQueryHavingMax(t *testing.T) {
	for _, sql := range []string{
		"SELECT ANY_VALUE(fruit HAVING MAX sold) FROM fruits",
		"SELECT ANY_VALUE(fruit HAVING MIN sold) FROM fruits",
		"SELECT category, ANY_VALUE(product HAVING MAX price), ANY_VALUE(product HAVING MIN cost) FROM t",
		"SELECT ARRAY_AGG(DISTINCT x IGNORE NULLS HAVING MAX x ORDER BY x LIMIT 1)",
	} {
		assertRoundTrip(t, "bigquery", sql, sql)
	}
}

// TestBigQueryGroupByAlias ports the _alias_ordered_group preprocess
// (generators/bigquery.py:91-110).
func TestBigQueryGroupByAlias(t *testing.T) {
	assertRoundTrip(t, "bigquery",
		"SELECT y + 1 z FROM x GROUP BY y + 1 ORDER BY z",
		"SELECT y + 1 AS z FROM x GROUP BY z ORDER BY z")
}

// TestBigQueryGapFill ports test_gap_fill (tests/dialects/test_bigquery.py:2521).
func TestBigQueryGapFill(t *testing.T) {
	for _, sql := range []string{
		"SELECT * FROM GAP_FILL(TABLE t, ts_column => 'time', bucket_width => INTERVAL '1' MINUTE)",
		"SELECT * FROM GAP_FILL(TABLE t, ts_column => 'time', bucket_width => INTERVAL '1' MINUTE) ORDER BY time",
	} {
		assertRoundTrip(t, "bigquery", sql, sql)
	}
}

// TestBigQueryMLFunctions ports test_ml_functions (tests/dialects/test_bigquery.py:2580) and
// test_models (2541).
func TestBigQueryMLFunctions(t *testing.T) {
	for _, sql := range []string{
		"SELECT * FROM ML.FORECAST(MODEL `mydataset.mymodel`, STRUCT(2 AS horizon))",
		"SELECT * FROM ML.FEATURES_AT_TIME(TABLE mydataset.feature_table, time => '2022-06-11 10:00:00+00', num_rows => 1)",
		"SELECT * FROM ML.TRANSLATE(MODEL `mydataset.mymodel`, (SELECT * FROM t), STRUCT('en' AS target_language))",
		"SELECT * FROM AI.FORECAST(TABLE citibike_trips, data_col => 'num_trips', timestamp_col => 'date', horizon => 30)",
		"SELECT AI.GENERATE_BOOL(MODEL `mydataset.gemini_model`, 'Is sky blue?')",
		"SELECT AI.GENERATE_TEXT(MODEL `mydataset.gemini_model`, 'Is sky blue?')",
		"SELECT AI.GENERATE_TEXT(MODEL `mydataset.gemini_model`, TABLE `mydataset.prompt_table`, STRUCT(0.15 AS temperature))",
		"SELECT AI.EMBED('text')",
		"SELECT AI.SIMILARITY('a', 'b')",
		"SELECT * FROM VECTOR_SEARCH(TABLE t, 'emb', TABLE q, top_k => 10)",
	} {
		assertRoundTrip(t, "bigquery", sql, sql)
	}
}

// TestBigQueryExportData ports the EXPORT DATA statement grammar
// (parsers/bigquery.py:762-771).
func TestBigQueryExportData(t *testing.T) {
	for _, sql := range []string{
		"EXPORT DATA OPTIONS (URI='gs://bucket/folder/*.csv') AS (SELECT 1)",
		"EXPORT DATA OPTIONS (URI='gs://path*.csv.gz', FORMAT='CSV') AS SELECT * FROM all_rows",
		"EXPORT DATA WITH CONNECTION myproject.us.myconnection OPTIONS (URI='gs://path*.csv.gz', FORMAT='CSV') AS SELECT * FROM all_rows",
	} {
		assertRoundTrip(t, "bigquery", sql, sql)
	}
}

// TestBigQueryLoadData ports the LOAD DATA ... FROM FILES grammar.
func TestBigQueryLoadData(t *testing.T) {
	for _, sql := range []string{
		"LOAD DATA INTO TABLE mydataset.table1 FROM FILES(FORMAT='AVRO', uris=['gs://bucket/path/file.avro'])",
		"LOAD DATA INTO TEMP TABLE mydataset.table1 FROM FILES(FORMAT='AVRO', uris=['gs://bucket/path/file.avro'])",
		"LOAD DATA OVERWRITE mydataset.table1 FROM FILES(FORMAT='AVRO', uris=['gs://bucket/path/file.avro'])",
	} {
		assertRoundTrip(t, "bigquery", sql, sql)
	}
}

// TestBigQueryDDL ports the CREATE/ALTER round-trips.
func TestBigQueryDDL(t *testing.T) {
	for _, sql := range []string{
		"ALTER TABLE db.t1 RENAME TO t2",
		"ALTER TABLE foo DROP PRIMARY KEY IF EXISTS",
		"CREATE TABLE db.example_table (x INT64) PARTITION BY x CLUSTER BY x",
		"CREATE TABLE IF NOT EXISTS foo AS SELECT * FROM bla EXCEPT DISTINCT (SELECT * FROM bar) LIMIT 0",
		"CREATE SCHEMA x DEFAULT COLLATE 'en'",
		"CREATE TABLE db.example_table (col_a STRUCT<struct_col_a INT64, struct_col_b STRUCT<nested_col_a STRING>>)",
		"CREATE VIEW `d.v` OPTIONS (expiration_timestamp=CAST('2020-01-02T04:05:06.007Z' AS TIMESTAMP)) AS SELECT 1 AS c",
		"CREATE TEMPORARY FUNCTION f(x INT64) RETURNS FLOAT64 LANGUAGE js AS 'return x'",
	} {
		assertRoundTrip(t, "bigquery", sql, sql)
	}
}

// TestBigQueryMiscFunctions ports the remaining function round-trips.
func TestBigQueryMiscFunctions(t *testing.T) {
	for _, sql := range []string{
		"SELECT TO_HEX(x)",
		"SELECT UPPER(TO_HEX(x))",
		"SELECT TO_HEX(SHA1('abc'))",
		"SELECT TO_HEX(MD5(x))",
		"SELECT MD5(x)",
		"SELECT SHA256(x)",
		"SELECT SHA512(x)",
		"SELECT FROM_HEX(x)",
		"SELECT CONTAINS_SUBSTR(a, b)",
		"SELECT CONTAINS_SUBSTR(a, b, 'JSON_KEYS_AND_VALUES')",
		"SELECT MOD(a, b)",
		"SELECT DIV(x, y)",
		"SELECT LOG(n, b)",
		"SELECT LN(x)",
		"SELECT SAFE_DIVIDE(a, b)",
		"SELECT NORMALIZE(x)",
		"SELECT NORMALIZE_AND_CASEFOLD(x, NFKC)",
		"SELECT BYTE_LENGTH(x)",
		"SELECT EDIT_DISTANCE(a, b)",
		"SELECT REGEXP_CONTAINS(a, 'b')",
		"SELECT REGEXP_EXTRACT(a, 'b')",
		"SELECT REGEXP_EXTRACT_ALL(a, 'b')",
		"SELECT REGEXP_REPLACE(a, 'b', c)",
		"SELECT STRPOS(a, b)",
		"SELECT SPLIT(a, ',')",
		"SELECT SPLIT(a, b)",
		"SELECT TRANSLATE(x, y, z)",
		"SELECT ARRAY_CONCAT([1, 2], [3, 4])",
		"SELECT EXISTS(SELECT 1 FROM UNNEST([1, 2, 3]) AS _col WHERE _col = 1)",
		"SELECT GREATEST(1, NULL, 3)",
		"SELECT LEAST(1, NULL, 3)",
		"SELECT GREATEST(a, b)",
		"SELECT LEAST(a, b)",
		"SELECT BIT_AND(x)",
		"SELECT BIT_OR(x)",
		"SELECT BIT_XOR(x)",
		"SELECT BIT_COUNT(x)",
		"SELECT UNIX_SECONDS(x)",
		"SELECT SESSION_USER()",
		"SELECT MOD(a, b)",
	} {
		assertRoundTrip(t, "bigquery", sql, sql)
	}
}

// TestBigQueryPseudoColumns ports test_pseudocolumns (tests/dialects/test_bigquery.py:4029).
func TestBigQueryPseudoColumns(t *testing.T) {
	for _, sql := range []string{
		"SELECT _PARTITIONTIME FROM t",
		"SELECT _PARTITIONDATE FROM t",
		"SELECT _TABLE_SUFFIX FROM t",
		"SELECT _FILE_NAME FROM t",
	} {
		assertRoundTrip(t, "bigquery", sql, sql)
	}
}

// TestBigQueryReservedKeywords verifies a reserved word used as an alias is quoted
// (generators/bigquery.py:484).
func TestBigQueryReservedKeywords(t *testing.T) {
	assertRoundTrip(t, "bigquery", "SELECT 1 AS at", "SELECT 1 AS `at`")
	assertRoundTrip(t, "bigquery", "SELECT 1 AS hash", "SELECT 1 AS `hash`")
}

// TestBigQueryDeclare ports test_declare (tests/dialects/test_bigquery.py:3557).
func TestBigQueryDeclare(t *testing.T) {
	for _, sql := range []string{
		"DECLARE x INT64",
		"DECLARE x INT64 DEFAULT 1",
		"DECLARE x, y, z INT64 DEFAULT 42",
		"DECLARE START_DATE DATE DEFAULT CURRENT_DATE - 1",
	} {
		assertRoundTrip(t, "bigquery", sql, sql)
	}
}

// TestBigQueryQuotedStrings verifies the triple-quote and raw-string tokenizer behavior.
func TestBigQueryQuotedStrings(t *testing.T) {
	cases := map[string]string{
		`SELECT """a\"b"""`:               `SELECT 'a"b'`,
		`SELECT '''a\'b'''`:               `SELECT 'a\'b'`,
		`SELECT r'x\''`:                   `SELECT 'x\\\''`,
		`SELECT """ends with \"word\""""`: `SELECT 'ends with "word"'`,
	}
	for sql, want := range cases {
		assertRoundTrip(t, "bigquery", sql, want)
	}
}

// TestBigQueryUnpivotAliasLiterals verifies UNPIVOT_ALIASES_ARE_IDENTIFIERS = False.
func TestBigQueryUnpivotAliasLiterals(t *testing.T) {
	const sql = "SELECT * FROM Produce UNPIVOT((a, b) FOR s IN ((Q1, Q2) AS 's1', (Q3, Q4) AS 's2'))"
	expr, err := sqlglot.ParseOne(sql, "bigquery")
	if err != nil {
		t.Fatal(err)
	}
	if got := expr.ToS(); !strings.Contains(got, "PivotAlias") {
		t.Fatalf("expected a PivotAlias node, got %s", got)
	}
	assertRoundTrip(t, "bigquery", sql, sql)
}

// TestBigQuerySafeFuncNode verifies SAFE.<fn> parses to a SafeFunc wrapper, not a Dot.
func TestBigQuerySafeFuncNode(t *testing.T) {
	expr, err := sqlglot.ParseOne("SAFE.PARSE_DATE('%F', x)", "bigquery")
	if err != nil {
		t.Fatal(err)
	}
	if expr.Kind() != exp.KindSafeFunc {
		t.Fatalf("expected SafeFunc, got %s", expr.ToS())
	}
}
