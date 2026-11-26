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
	"github.com/maltzsama/sqlglot-go/tokens"
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

// scriptStatement is one expected top-level statement of a multi-statement script: its root
// Kind and the verbatim source text its span covers.
type scriptStatement struct {
	kind exp.Kind
	span string
}

// assertScriptStatements parses sql under dialect and checks the statement boundaries: the
// statement count, each root Kind, and each statement's verbatim span text.
func assertScriptStatements(t *testing.T, sql, dialect string, want []scriptStatement) {
	t.Helper()
	statements, err := sqlglot.Parse(sql, dialect)
	if err != nil {
		t.Fatalf("%q [%s]: parse: %v", sql, dialect, err)
	}
	if len(statements) != len(want) {
		t.Fatalf("%q [%s]: got %d statements, want %d", sql, dialect, len(statements), len(want))
	}
	for i, stmt := range statements {
		if stmt == nil {
			t.Fatalf("%q [%s] stmt %d: got nil, want %s", sql, dialect, i, exp.ClassName(want[i].kind))
		}
		if stmt.Kind() != want[i].kind {
			t.Errorf("%q [%s] stmt %d: got %s, want %s", sql, dialect, i, exp.ClassName(stmt.Kind()), exp.ClassName(want[i].kind))
		}
		if got, ok := stmt.SpanText(); !ok || got != want[i].span {
			t.Errorf("%q [%s] stmt %d: span %q, want %q", sql, dialect, i, got, want[i].span)
		}
	}
}

// TestTSQLGoBatchSeparator pins the `GO` batch separator (DEVIATIONS §1.22): a line holding
// only `GO [count]` ends the batch, as sqlcmd/SSMS treat it. Upstream lexes GO as a Command,
// so it either becomes a column alias, folds the next batch into a Command, or fails.
func TestTSQLGoBatchSeparator(t *testing.T) {
	sel := exp.KindSelect
	cases := []struct {
		sql  string
		want []scriptStatement
	}{
		// Upstream: one Select aliased GO.
		{"SELECT 1\nGO", []scriptStatement{{sel, "SELECT 1"}}},
		// Upstream: Select + Command "GO\nSELECT 2", hiding the second batch.
		{"SELECT 1;\nGO\nSELECT 2", []scriptStatement{{sel, "SELECT 1"}, {sel, "SELECT 2"}}},
		// Upstream: parse error.
		{"SELECT TOP 1 [a] FROM dbo.t\nGO\nSELECT 2",
			[]scriptStatement{{sel, "SELECT TOP 1 [a] FROM dbo.t"}, {sel, "SELECT 2"}}},
		{"SELECT 1\nGO\nSELECT 2", []scriptStatement{{sel, "SELECT 1"}, {sel, "SELECT 2"}}},
		// A repeat count is accepted and not part of either statement.
		{"SELECT 1\nGO 5\nSELECT 2", []scriptStatement{{sel, "SELECT 1"}, {sel, "SELECT 2"}}},
		// Case-insensitive, surrounding blanks, CRLF line endings.
		{"SELECT 1\n  go\t\nSELECT 2", []scriptStatement{{sel, "SELECT 1"}, {sel, "SELECT 2"}}},
		{"SELECT 1\r\nGO\r\nSELECT 2", []scriptStatement{{sel, "SELECT 1"}, {sel, "SELECT 2"}}},
		// Leading, repeated, and trailing separators never produce empty statements.
		{"GO\nSELECT 1\nGO\nGO\n", []scriptStatement{{sel, "SELECT 1"}}},
		// A Command's packed tail stops at the separator line.
		{"EXEC sp_who\nGO\nSELECT 2", []scriptStatement{{exp.KindExecute, "EXEC sp_who"}, {sel, "SELECT 2"}}},
		// A trailing `--` comment on the GO line is allowed and stripped with the separator:
		// a batch separator is a boundary, never a statement (no orphan Semicolon node).
		{"SELECT 1\nGO -- end of batch\nSELECT 2",
			[]scriptStatement{{sel, "SELECT 1"}, {sel, "SELECT 2"}}},
		{"SELECT 1;\nGO -- end of batch\nSELECT 2",
			[]scriptStatement{{sel, "SELECT 1"}, {sel, "SELECT 2"}}},
		// Not separators: GO inside a string or a block comment, GO not alone on its line,
		// and a longer word that starts with GO.
		{"SELECT 'a\nGO\nb'", []scriptStatement{{sel, "SELECT 'a\nGO\nb'"}}},
		{"SELECT 1 /* x\nGO\n*/ + 1", []scriptStatement{{sel, "SELECT 1 /* x\nGO\n*/ + 1"}}},
		{"SELECT a,\nGOTO\nFROM t", []scriptStatement{{sel, "SELECT a,\nGOTO\nFROM t"}}},
		{"SELECT 1 GO", []scriptStatement{{sel, "SELECT 1 GO"}}},
		// The upstream identity case: an input holding only `GO` has no batch to separate,
		// so it stays upstream's Command.
		{"GO", []scriptStatement{{exp.KindCommand, "GO"}}},
	}
	for _, tc := range cases {
		assertScriptStatements(t, tc.sql, "tsql", tc.want)
	}

	// Other dialects keep upstream's lexing: GO is an ordinary word.
	assertScriptStatements(t, "SELECT 1\nGO", "", []scriptStatement{{sel, "SELECT 1\nGO"}})
}

// TestTSQLGoBatchSeparatorToken pins the token the separator line lexes to: a SEMICOLON
// whose Text and Start/End cover the separator text (`GO 5`), so token consumers split on it
// like `;` and can still recover the original line. The separator is emitted even right after
// a `;` (DEVIATIONS §1.22), so every batch boundary is locatable from the stream.
func TestTSQLGoBatchSeparatorToken(t *testing.T) {
	cases := []struct {
		sql  string
		want []string
	}{
		{"SELECT 1\nGO 5\nSELECT 2", []string{"GO 5"}},
		{"SELECT 1;\nGO\nSELECT 2", []string{";", "GO"}},
	}
	for _, tc := range cases {
		toks, err := sqlglot.Tokenize(tc.sql, "tsql")
		if err != nil {
			t.Fatal(err)
		}
		var separators []string
		for _, tok := range toks {
			if tok.TokenType == tokens.SEMICOLON {
				separators = append(separators, tok.Text)
				got := string([]rune(tc.sql)[tok.Start : tok.End+1])
				if got != tok.Text {
					t.Errorf("%q: separator source slice = %q, want %q", tc.sql, got, tok.Text)
				}
			}
		}
		if len(separators) != len(tc.want) {
			t.Fatalf("%q: separators = %q, want %q", tc.sql, separators, tc.want)
		}
		for i := range separators {
			if separators[i] != tc.want[i] {
				t.Fatalf("%q: separators = %q, want %q", tc.sql, separators, tc.want)
			}
		}
	}
}

// TestTSQLGoEndsRoutineBody pins the batch boundary inside a routine body: a
// single-statement body (`CREATE PROCEDURE p AS SELECT 1`, no BEGIN...END) is one batch, so
// the GO separator must end it instead of letting the body block swallow the next batch's
// statement. The structured Create keeps a one-statement Block body; a plain `;` still folds
// the following statement into the body, as upstream's _parse_block does.
func TestTSQLGoEndsRoutineBody(t *testing.T) {
	statements, err := sqlglot.Parse("CREATE PROCEDURE p AS SELECT 1\ngo\nSELECT 2", "tsql")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(statements) != 2 {
		t.Fatalf("got %d statements, want 2:\n%s", len(statements), statements[0].ToS())
	}
	create := statements[0]
	if create.Kind() != exp.KindCreate {
		t.Fatalf("first statement = %s, want Create", exp.ClassName(create.Kind()))
	}
	body := create.Arg("expression")
	block, ok := body.(*exp.Node)
	if !ok || block.Kind() != exp.KindBlock {
		t.Fatalf("create body = %v, want Block", body)
	}
	if got := len(block.Expressions()); got != 1 {
		t.Fatalf("body has %d expressions, want 1:\n%s", got, block.ToS())
	}
	if statements[1].Kind() != exp.KindSelect {
		t.Fatalf("second statement = %s, want Select", exp.ClassName(statements[1].Kind()))
	}

	// BEGIN...END bodies already stop at the separator; keep that covered alongside the bare
	// form, and confirm a bare body still folds a plain `;`-separated statement.
	assertScriptStatements(t, "CREATE PROCEDURE p AS BEGIN SELECT 1 END\ngo\nSELECT 2", "tsql",
		[]scriptStatement{{exp.KindCommand, "CREATE PROCEDURE p AS BEGIN SELECT 1 END"}, {exp.KindSelect, "SELECT 2"}})
	assertScriptStatements(t, "CREATE PROCEDURE p AS SELECT 1; SELECT 2", "tsql",
		[]scriptStatement{{exp.KindCreate, "CREATE PROCEDURE p AS SELECT 1; SELECT 2"}})
}

// TestTSQLReviewFixes pins the real-engine corrections recorded in DEVIATIONS 1.34.
func TestTSQLReviewFixes(t *testing.T) {
	cases := []struct{ sql, write, want string }{
		{"SELECT CONVERT(DATE, '12-31-24', 10)", "duckdb", "SELECT CAST(STRPTIME('12-31-24', '%m-%d-%y') AS DATE)"},
		{"SELECT CONVERT(DATE, '12-31-24', 110)", "duckdb", "SELECT CAST(STRPTIME('12-31-24', '%m-%d-%Y') AS DATE)"},
		{"SELECT CONVERT(DATE, '2024-01-02', NULL)", "duckdb", "SELECT NULL"},
		{"SELECT CONVERT(VARCHAR(10), CAST('2024-01-02' AS DATE), 101)", "duckdb", "SELECT CAST(STRFTIME(CAST('2024-01-02' AS DATE), '%m/%d/%Y') AS TEXT(10))"},
		{"SELECT DISTINCT x FROM t OFFSET 1 ROWS", "tsql", "SELECT DISTINCT x FROM t ORDER BY 1 OFFSET 1 ROWS"},
		{"SELECT TRY_CONVERT(DATE, 'garbage', 101)", "duckdb", "SELECT CAST(TRY_STRPTIME('garbage', '%m/%d/%Y') AS DATE)"},
	}
	for _, tc := range cases {
		out, err := sqlglot.Transpile(tc.sql, "tsql", tc.write, generator.Options{})
		if err != nil {
			t.Fatalf("%s: %v", tc.sql, err)
		}
		got := ""
		if len(out) == 1 {
			got = out[0]
		}
		if got != tc.want {
			t.Fatalf("%s\n got: %s\nwant: %s", tc.sql, got, tc.want)
		}
	}
}
