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
