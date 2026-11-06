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

// TestTSQLIdentity is the same-dialect round-trip oracle for the T-SQL dialect, ported
// from .reference/sqlglot-v30.17.0/tests/dialects/test_tsql.py. Each case is already in
// its canonical T-SQL spelling, so parse → generate must be an identity.
func TestTSQLIdentity(t *testing.T) {
	cases := []string{
		"SELECT 1",
		"SELECT TOP 10 * FROM [dbo].[Users]",
		"SELECT [a], N'b' FROM t",
		"SELECT * FROM t WHERE x = 1 ORDER BY y OFFSET 5 ROWS FETCH NEXT 10 ROWS ONLY",
		"SELECT ISNULL(a, b) FROM t",
		"SELECT LEN(a), LEFT(a, 2), RIGHT(a, 2) FROM t",
		"SELECT * FROM #temp",
		"SELECT * FROM ##temp",
		"GO",
		"SELECT GETDATE(), SYSDATETIMEOFFSET(), NEWID()",
		"SELECT CONVERT(INTEGER, x) FROM t",
		"SELECT * FROM t CROSS APPLY F(t.x)",
		"SELECT * FROM t FOR JSON PATH",
		"SELECT * FROM t FOR BROWSE",
		"CREATE TABLE #t (id INTEGER IDENTITY(1, 1) PRIMARY KEY, name NVARCHAR(100))",
		"DECLARE @x INTEGER = 1",
		"DECLARE @v1 INTEGER = 1, @v2 CHAR(1) = 'c'",
		"SELECT [x] FROM [a].[b] FOR SYSTEM_TIME ALL AS alias",
		"SELECT [x] FROM [a].[b] FOR SYSTEM_TIME AS OF 'foo'",
		"SELECT DATEDIFF(DAY, CAST(a AS DATETIME2), CAST(b AS DATETIME2)) AS x FROM foo",
		"SELECT DATEADD(DAY, 1, x) FROM t",
		"SELECT TRIM('.,! ' FROM ' # test .') AS Result",
		"SELECT STRING_AGG(x, '|') WITHIN GROUP (ORDER BY z ASC)",
		"SELECT JSON_ARRAYAGG(c ORDER BY c NULL ON NULL)",
		"SELECT a FROM t1 UNION ALL SELECT b FROM t2",
		"SELECT [x] FROM [a].[b] WITH (NOLOCK)",
	}
	for _, sql := range cases {
		t.Run(sql, func(t *testing.T) {
			expr, err := sqlglot.ParseOne(sql, "tsql")
			if err != nil {
				t.Fatalf("ParseOne: %v", err)
			}
			got, err := sqlglot.Generate(expr, "tsql", generator.Options{})
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if got != sql {
				t.Fatalf("round-trip mismatch\n want: %s\n  got: %s", sql, got)
			}
		})
	}
}

// TestTSQLASTShapes asserts that T-SQL constructs build the upstream node shapes rather
// than degrading to raw-text Commands.
func TestTSQLASTShapes(t *testing.T) {
	tests := []struct {
		sql  string
		want exp.Kind
	}{
		{"SELECT TOP 10 * FROM t", exp.KindSelect},
		{"SELECT CONVERT(INTEGER, x) FROM t", exp.KindConvert},
		{"SELECT ISNULL(a, b)", exp.KindCoalesce},
		{"SELECT HASHBYTES('MD5', 'x')", exp.KindMD5},
		{"SELECT EOMONTH(GETDATE())", exp.KindLastDay},
		{"SELECT DATEPART(year, x)", exp.KindExtract},
		{"DECLARE @x INTEGER = 1", exp.KindDeclare},
		{"SELECT CAST(x AS INTEGER)::dbo.f", exp.KindScopeResolution},
		{"SELECT NEXT VALUE FOR s", exp.KindNextValueFor},
	}
	for _, tt := range tests {
		expr, err := sqlglot.ParseOne(tt.sql, "tsql")
		if err != nil {
			t.Fatalf("%s: ParseOne: %v", tt.sql, err)
		}
		node := expr
		if node.Kind() != tt.want {
			if selects := node.Selects(); len(selects) > 0 {
				node = selects[0]
				if node.Kind() == exp.KindAlias {
					node = node.This()
				}
			}
		}
		if node.Kind() != tt.want {
			t.Fatalf("%s: kind = %s, want %s", tt.sql, exp.ClassName(node.Kind()), exp.ClassName(tt.want))
		}
	}
}

// TestTSQLTokenizer asserts the T-SQL-specific lexer deltas.
func TestTSQLTokenizer(t *testing.T) {
	toks, err := sqlglot.Tokenize("SELECT [a], N'b', @v, #t, ##g, 0xFF", "tsql")
	if err != nil {
		t.Fatalf("Tokenize: %v", err)
	}
	var texts []string
	for _, tok := range toks {
		texts = append(texts, tok.Text)
	}
	joined := strings.Join(texts, "\x00")
	for _, want := range []string{"a", "b", "v", "t", "g", "FF"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("token stream %q missing %q", texts, want)
		}
	}
}

// TestTSQLQualifyAndScope verifies the pipeline Baleia consumes: parse → qualify →
// traverse_scope resolves a T-SQL query end to end.
func TestTSQLQualifyAndScope(t *testing.T) {
	d := dialects.TSQL()
	mapping, err := schema.NewMappingSchema(schema.M(
		"dbo", schema.M("users", schema.M("id", "INT")),
	), d, true)
	if err != nil {
		t.Fatalf("NewMappingSchema: %v", err)
	}
	expr, err := sqlglot.ParseOne("SELECT id FROM users", d)
	if err != nil {
		t.Fatalf("ParseOne: %v", err)
	}
	opts := optimizer.DefaultQualifyOpts()
	opts.Dialect = d
	opts.Schema = mapping
	opts.DefaultSchema = "dbo"
	opts.QuoteIdentifiers = false
	opts.Identify = false
	qualified := optimizer.Qualify(expr, opts)
	out, err := sqlglot.Generate(qualified, "tsql", generator.Options{})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	for _, want := range []string{"dbo.users", "users.id"} {
		if !strings.Contains(out, want) {
			t.Fatalf("qualified SQL %q missing %q", out, want)
		}
	}
	scopes := optimizer.TraverseScope(qualified)
	if len(scopes) != 1 {
		t.Fatalf("scopes = %d, want 1", len(scopes))
	}
	if _, ok := scopes[0].Sources["users"]; !ok {
		t.Fatalf("scope sources = %v, want users", scopes[0].Sources)
	}
}

// TestTSQLParserGeneratorGaps covers the seven parser/generator gaps closed for issue
// maltzsama/sqlglot-go#3 (TSQL identity corpus 606 → 613/613). Each case asserts the
// construct builds its structured upstream node — a raw-text Command round-trips too and
// would hide a broken AST — plus its exact generated spelling. SQL/want pairs are taken
// from testdata/dialect_identity.jsonl, recorded from pinned upstream v30.17.0.
func TestTSQLParserGeneratorGaps(t *testing.T) {
	compactMerge := "INSERT INTO Production.UpdatedInventory SELECT ProductID, LocationID, NewQty, PreviousQty FROM (MERGE INTO Production.ProductInventory AS pi USING (SELECT ProductID, SUM(OrderQty) FROM Sales.SalesOrderDetail AS sod INNER JOIN Sales.SalesOrderHeader AS soh ON sod.SalesOrderID = soh.SalesOrderID AND soh.OrderDate BETWEEN '20030701' AND '20030731' GROUP BY ProductID) AS src(ProductID, OrderQty) ON pi.ProductID = src.ProductID WHEN MATCHED AND pi.Quantity - src.OrderQty >= 0 THEN UPDATE SET pi.Quantity = pi.Quantity - src.OrderQty WHEN MATCHED AND pi.Quantity - src.OrderQty <= 0 THEN DELETE OUTPUT $action, Inserted.ProductID, Inserted.LocationID, Inserted.Quantity AS NewQty, Deleted.Quantity AS PreviousQty) AS Changes(Action, ProductID, LocationID, NewQty, PreviousQty) WHERE Action = 'UPDATE'"
	mergePrettyWant := `INSERT INTO Production.UpdatedInventory
SELECT
  ProductID,
  LocationID,
  NewQty,
  PreviousQty
FROM (
  MERGE INTO Production.ProductInventory AS pi
  USING (
    SELECT
      ProductID,
      SUM(OrderQty)
    FROM Sales.SalesOrderDetail AS sod
    INNER JOIN Sales.SalesOrderHeader AS soh
      ON sod.SalesOrderID = soh.SalesOrderID
      AND soh.OrderDate BETWEEN '20030701' AND '20030731'
    GROUP BY
      ProductID
  ) AS src(ProductID, OrderQty)
  ON pi.ProductID = src.ProductID
  WHEN MATCHED AND pi.Quantity - src.OrderQty >= 0 THEN UPDATE SET
    pi.Quantity = pi.Quantity - src.OrderQty
  WHEN MATCHED AND pi.Quantity - src.OrderQty <= 0 THEN DELETE
  OUTPUT $action, Inserted.ProductID, Inserted.LocationID, Inserted.Quantity AS NewQty, Deleted.Quantity AS PreviousQty
) AS Changes(Action, ProductID, LocationID, NewQty, PreviousQty)
WHERE
  Action = 'UPDATE'`
	udfPrettyWant := `CREATE FUNCTION udfProductInYear(
    @model_year INTEGER
)
RETURNS TABLE AS
RETURN SELECT
  product_name,
  model_year,
  list_price
FROM production.products
WHERE
  model_year = @model_year`

	tests := []struct {
		name   string
		sql    string
		want   string // "" means identity (want == sql)
		kind   exp.Kind
		finds  []exp.Kind // kinds that must appear somewhere in the tree
		pretty bool
	}{
		// Gap 4/4b + 1: PERIOD FOR SYSTEM_TIME moved to the base constraint registry and
		// the WITH(...) system-versioning properties added to propertyLocations (they were
		// failing closed as "Unsupported property" once the parse stopped degrading to
		// Command), including the optional-args None-not-"" rendering.
		{
			name:  "create table with period and system versioning",
			sql:   "CREATE TABLE test ([data] CHAR(7), [valid_from] DATETIME2(2) GENERATED ALWAYS AS ROW START NOT NULL, [valid_to] DATETIME2(2) GENERATED ALWAYS AS ROW END NOT NULL, PERIOD FOR SYSTEM_TIME ([valid_from], [valid_to])) WITH(SYSTEM_VERSIONING=ON)",
			kind:  exp.KindCreate,
			finds: []exp.Kind{exp.KindPeriodForSystemTimeConstraint, exp.KindWithSystemVersioningProperty},
		},
		{
			name:  "create table with bare system versioning",
			sql:   "CREATE TABLE t1 ([a] INTEGER) WITH(SYSTEM_VERSIONING=ON)",
			kind:  exp.KindCreate,
			finds: []exp.Kind{exp.KindWithSystemVersioningProperty},
		},
		{
			name:  "alter set system versioning with all options",
			sql:   "ALTER TABLE tbl SET (SYSTEM_VERSIONING=ON(HISTORY_TABLE=db.tbl, DATA_CONSISTENCY_CHECK=OFF, HISTORY_RETENTION_PERIOD=5 DAYS))",
			kind:  exp.KindAlter,
			finds: []exp.Kind{exp.KindWithSystemVersioningProperty},
		},
		{
			name:  "alter set system versioning off",
			sql:   "ALTER TABLE tbl SET (SYSTEM_VERSIONING=OFF)",
			kind:  exp.KindAlter,
			finds: []exp.Kind{exp.KindWithSystemVersioningProperty},
		},
		{
			name:  "alter set data deletion with options",
			sql:   "ALTER TABLE tbl SET (DATA_DELETION=ON(FILTER_COLUMN=col, RETENTION_PERIOD=5 MONTHS))",
			kind:  exp.KindAlter,
			finds: []exp.Kind{exp.KindDataDeletionProperty},
		},
		{
			name:  "alter set data deletion off",
			sql:   "ALTER TABLE tbl SET (DATA_DELETION=OFF)",
			kind:  exp.KindAlter,
			finds: []exp.Kind{exp.KindDataDeletionProperty},
		},
		// Gap 5: bare `ON <schema>` after CREATE TABLE's property list.
		{
			name:  "create table with on property",
			sql:   "CREATE TABLE x (CONSTRAINT [pk_mytable] UNIQUE NONCLUSTERED (a DESC)) ON b (c)",
			kind:  exp.KindCreate,
			finds: []exp.Kind{exp.KindOnProperty},
		},
		// Gap 2: COPY CREDENTIAL = (...) parses its paren body as a property list; the
		// comma-between-properties split yields an empty SequenceProperties node that the
		// generator renders as "" (flat expressions() skips it).
		{
			name:  "copy with credential options",
			sql:   "COPY INTO test_1 FROM 'path' WITH (FORMAT_NAME = test, FILE_TYPE = 'CSV', CREDENTIAL = (IDENTITY='Shared Access Signature', SECRET='token'), FIELDTERMINATOR = ';', ROWTERMINATOR = '0X0A', ENCODING = 'UTF8', DATEFORMAT = 'ymd', MAXERRORS = 10, ERRORFILE = 'errorsfolder', IDENTITY_INSERT = 'ON')",
			kind:  exp.KindCopy,
			finds: []exp.Kind{exp.KindCopyParameter, exp.KindSequenceProperties},
		},
		// Gap 7: statement-leading `::` is a T-SQL RANGE_PARSERS entry producing
		// ScopeResolution (postfix x::y stays on the column-operator path).
		{
			name: "leading scope resolution",
			sql:  "::FOO(a, b)",
			kind: exp.KindScopeResolution,
		},
		// Gap 3b + ensure-bools: TSQL's ENSURE_BOOLS rewrites non-boolean condition
		// operands at generate time (upstream does it as an AST preprocess).
		{
			name: "ensure bool in where",
			sql:  "SELECT * FROM t WHERE NOT c",
			want: "SELECT * FROM t WHERE NOT c <> 0",
			kind: exp.KindSelect,
		},
		{
			name: "ensure bool on connector operands",
			sql:  "CAST(x AS int) OR y",
			want: "CAST(x AS INTEGER) <> 0 OR y <> 0",
			kind: exp.KindOr,
		},
		{
			name: "ensure bool parenthesizes comparison RHS",
			sql:  "1 AND true",
			want: "1 <> 0 AND (1 = 1)",
			kind: exp.KindAnd,
		},
		{
			name: "ensure bool rewrites inequality literal",
			sql:  "a != FALSE",
			want: "a <> 0",
			kind: exp.KindNEQ,
		},
		// OPENJSON + WITH schema (structured, not Command).
		{
			name:  "openjson",
			sql:   "SELECT * FROM OPENJSON(@json)",
			kind:  exp.KindSelect,
			finds: []exp.Kind{exp.KindOpenJSON},
		},
		// Gap 6: T-SQL allows MERGE inside a wrapped select (`INSERT ... FROM (MERGE
		// ...)`); pretty output must also break a too-wide ON connector across lines
		// (connector_sql's pretty too_wide sep).
		{
			name:   "merge inside wrapped select",
			sql:    compactMerge,
			want:   mergePrettyWant,
			kind:   exp.KindInsert,
			finds:  []exp.Kind{exp.KindMerge},
			pretty: true,
		},
		// Gap 3a: pretty CREATE FUNCTION indents the parameter list through wrap's
		// level=1/pad=0 on top of expressions' own pad=2 (4 spaces).
		{
			name:   "pretty udf parameter indent",
			sql:    "CREATE FUNCTION udfProductInYear (@model_year INT) RETURNS TABLE AS RETURN SELECT product_name, model_year, list_price FROM production.products WHERE model_year = @model_year",
			want:   udfPrettyWant,
			kind:   exp.KindCreate,
			pretty: true,
		},
		{
			name: "top level merge",
			sql:  "MERGE INTO mytable WITH (HOLDLOCK) AS T USING mytable_merge AS S ON (T.user_id = S.user_id) WHEN NOT MATCHED THEN INSERT (c1, c2) VALUES (S.c1, S.c2)",
			kind: exp.KindMerge,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr, err := sqlglot.ParseOne(tt.sql, "tsql")
			if err != nil {
				t.Fatalf("ParseOne: %v", err)
			}
			if expr.Kind() != tt.kind {
				t.Fatalf("root kind = %s, want %s:\n%s", exp.ClassName(expr.Kind()), exp.ClassName(tt.kind), expr.ToS())
			}
			for _, k := range tt.finds {
				if expr.Find(k) == nil {
					t.Fatalf("tree is missing %s:\n%s", exp.ClassName(k), expr.ToS())
				}
			}
			want := tt.want
			if want == "" {
				want = tt.sql
			}
			got, err := sqlglot.Generate(expr, "tsql", generator.Options{Pretty: tt.pretty})
			if err != nil {
				t.Fatalf("Generate: %v", err)
			}
			if got != want {
				t.Fatalf("round-trip mismatch\n want: %s\n  got: %s", want, got)
			}
		})
	}
}
