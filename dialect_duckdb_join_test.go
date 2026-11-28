package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/generator"
)

// DuckDB join and statement forms from parsers/duckdb.py (_parse_join, STATEMENT_PARSERS)
// and generators/duckdb.py (join_sql). Expected outputs are the pinned upstream v30.35.0
// output.
func TestDuckDBJoinByTypeAndNearest(t *testing.T) {
	cases := []struct{ sql, want string }{
		{"SELECT * FROM a JOIN BY (TYPE inner) b ON a.id = b.id", "SELECT * FROM a INNER JOIN b ON a.id = b.id"},
		{"SELECT * FROM a JOIN BY (TYPE left) b USING (id)", "SELECT * FROM a LEFT JOIN b USING (id)"},
		{"SELECT * FROM a JOIN BY (TYPE mark) b ON a.id = b.id", "SELECT * FROM a JOIN BY (TYPE MARK) b ON a.id = b.id"},
		{"SELECT * FROM a JOIN BY (TYPE right_semi) b ON a.id = b.id", "SELECT * FROM a JOIN BY (TYPE RIGHT_SEMI) b ON a.id = b.id"},
		{"SELECT * FROM a JOIN BY (TYPE single) b ON a.id = b.id", "SELECT * FROM a JOIN BY (TYPE SINGLE) b ON a.id = b.id"},
		{"SELECT * FROM a LEFT SEMI JOIN b ON a.id = b.id", "SELECT * FROM a SEMI JOIN b ON a.id = b.id"},
		{"SELECT * FROM a JOIN b NEAREST 1 BY DISTANCE ABS(a.id - b.id)", "SELECT * FROM a JOIN b NEAREST 1 BY DISTANCE ABS(a.id - b.id)"},
		{"SELECT * FROM a JOIN b NEAREST 1 BY DISTANCE ABS(a.id - b.id) JOIN c NEAREST 2 BY DISTANCE ABS(b.id - c.id)", "SELECT * FROM a JOIN b NEAREST 1 BY DISTANCE ABS(a.id - b.id) JOIN c NEAREST 2 BY DISTANCE ABS(b.id - c.id)"},
		{"SELECT * FROM a LEFT JOIN b NEAREST 1 BY DISTANCE ABS(a.id - b.id)", "SELECT * FROM a LEFT JOIN b NEAREST 1 BY DISTANCE ABS(a.id - b.id)"},
		{`SELECT * FROM a JOIN b AS "nearest" NEAREST 1 BY DISTANCE ABS(a.id - "nearest".id)`, `SELECT * FROM a JOIN b AS "nearest" NEAREST 1 BY DISTANCE ABS(a.id - "nearest".id)`},
	}
	for _, tc := range cases {
		transpileCase(t, "duckdb", "duckdb", tc.sql, tc.want)
	}
}

func TestDuckDBConnectDisconnect(t *testing.T) {
	for _, sql := range []string{`CONNECT "remote db"`, "CONNECT qk", "DISCONNECT"} {
		transpileCase(t, "duckdb", "duckdb", sql, sql)
	}
}

// A quoted "APPROX" is an alias, not the APPROX NEAREST modifier (TEXT_MATCH_EXCLUDED_TOKENS).
func TestDuckDBQuotedApproxIsAlias(t *testing.T) {
	transpileCase(t, "duckdb", "duckdb",
		`SELECT * FROM a JOIN b "APPROX" NEAREST 1 BY DISTANCE a.x`,
		`SELECT * FROM a JOIN b AS "APPROX" NEAREST 1 BY DISTANCE a.x`)
	transpileCase(t, "duckdb", "duckdb",
		"SELECT * FROM a JOIN b APPROX NEAREST 1 BY DISTANCE a.x",
		"SELECT * FROM a JOIN b APPROX NEAREST 1 BY DISTANCE a.x")
}

// A JOIN BY (TYPE with no type name is a parse error, not a nil dereference.
func TestDuckDBJoinByMissingTypeIsParseError(t *testing.T) {
	if _, err := sqlglot.Transpile("SELECT * FROM a JOIN BY (TYPE", "duckdb", "duckdb", generator.Options{}); err == nil {
		t.Fatal("expected a parse error")
	}
}
