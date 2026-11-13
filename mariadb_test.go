package sqlglot_test

import (
	"testing"

	sqlglot "github.com/maltzsama/sqlglot-go"
	"github.com/maltzsama/sqlglot-go/dialects"
	"github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/generator"
)

// TestMariaDBIdentity pins the beyond-upstream MariaDB dialect identity and the MySQL behavior
// it inherits: the override chains treat mariadb as a child of mysql, so the family branches
// (dpipe-as-OR, backtick identifiers, REPLACE, SHOW CREATE, AUTO_INCREMENT) all apply.
func TestMariaDBIdentity(t *testing.T) {
	d, err := dialects.GetOrRaise("mariadb")
	if err != nil || d == nil || d.Name != "mariadb" {
		t.Fatalf("GetOrRaise(mariadb) = %v, err %v; want Name mariadb", d, err)
	}
	// Settings still apply to the MariaDB identity.
	d, err = dialects.GetOrRaise("mariadb, mysql_version=100611")
	if err != nil || d == nil || d.Name != "mariadb" || d.MySQLVersion == nil || *d.MySQLVersion != 100611 {
		t.Fatalf("GetOrRaise(mariadb with settings) = %+v, err %v", d, err)
	}

	assertRoundTrip(t, "mariadb", "SELECT `a` FROM `t`", "SELECT `a` FROM `t`")
	assertRoundTrip(t, "mariadb", "SELECT a || b", "SELECT a OR b")
	assertRoundTrip(t, "mariadb", "REPLACE INTO t VALUES (1)", "REPLACE INTO t VALUES (1)")
	assertRoundTrip(t, "mariadb", "SHOW CREATE TABLE t", "SHOW CREATE TABLE t")
	assertRoundTrip(t, "mariadb", "CREATE TABLE t (a INT AUTO_INCREMENT PRIMARY KEY)",
		"CREATE TABLE t (a INT AUTO_INCREMENT PRIMARY KEY)")
	assertRoundTrip(t, "mariadb", "SELECT CONVERT(x USING utf8mb4)", "SELECT CAST(x AS CHAR CHARACTER SET utf8mb4)")

	// The mysql_ansi_quotes setting applies to the MariaDB identity too: `"` becomes an
	// identifier delimiter instead of a string quote.
	ansi, err := sqlglot.Transpile(`SELECT "x"`, "mariadb, mysql_ansi_quotes=true", "mariadb", generator.Options{})
	if err != nil || ansi[0] != "SELECT `x`" {
		t.Fatalf(`mariadb ansi_quotes parse = %q, err %v; want the identifier form`, ansi, err)
	}
	plain, err := sqlglot.Transpile(`SELECT "x"`, "mariadb", "mariadb", generator.Options{})
	if err != nil || plain[0] != "SELECT 'x'" {
		t.Fatalf(`mariadb default parse = %q, err %v; want the string form`, plain, err)
	}
}

// TestMariaDBSequences pins the MariaDB-only sequence access, which upstream cannot parse under
// mysql (verified: ParseError) and has no dialect for at all.
func TestMariaDBSequences(t *testing.T) {
	cases := map[string]expressions.Kind{
		"SELECT NEXT VALUE FOR seq":      expressions.KindNextValueFor,
		"SELECT NEXT VALUE FOR db.seq":   expressions.KindNextValueFor,
		"SELECT PREVIOUS VALUE FOR seq":  expressions.KindPrevValueFor,
		"SELECT PREVIOUS VALUE FOR db.s": expressions.KindPrevValueFor,
	}
	for sql, kind := range cases {
		expr, err := sqlglot.ParseOne(sql, "mariadb")
		if err != nil {
			t.Fatalf("parse %q: %v", sql, err)
		}
		selected := expr.Expressions()[0]
		if selected.Kind() != kind {
			t.Fatalf("%q parsed to %s, want %s", sql, selected.ToS(), expressions.ClassName(kind))
		}
		got, err := sqlglot.Generate(expr, "mariadb", generator.Options{})
		if err != nil {
			t.Fatalf("generate %q: %v", sql, err)
		}
		if got != sql {
			t.Fatalf("round-trip %q -> %q", sql, got)
		}
	}
	// The sequences are gated to MariaDB: MySQL itself still rejects them.
	if _, err := sqlglot.ParseOne("SELECT NEXT VALUE FOR seq", "mysql"); err == nil {
		t.Fatal("expected mysql to reject NEXT VALUE FOR; the construct is MariaDB-only")
	}
	if _, err := sqlglot.ParseOne("SELECT PREVIOUS VALUE FOR seq", "mysql"); err == nil {
		t.Fatal("expected mysql to reject PREVIOUS VALUE FOR; the construct is MariaDB-only")
	}
}

// TestMariaDBExecutableComments pins `/*M! ... */`, activated alongside `/*!` by the
// mysql_version setting: the optional six-digit gate selects the body when it is <= the
// configured version, and the body is stripped otherwise.
func TestMariaDBExecutableComments(t *testing.T) {
	const dialect = "mariadb, mysql_version=100500"
	cases := map[string]string{
		"SELECT /*M!100100 1 */":   "SELECT 1",
		"SELECT /*M! 2 */":         "SELECT 2",
		"SELECT /*M!50301 3 */":    "SELECT 3",
		"SELECT 0 /*M!110000 4 */": "SELECT 0 /* M!110000 4 */",
		// MariaDB ignores MySQL-style comments gated to MySQL 5.7 through 9.99 ...
		"SELECT 0 /*!80035 5 */": "SELECT 0 /* !80035 5 */",
		// ... while MariaDB-numbered gates and markers execute.
		"SELECT /*!100100 6 */": "SELECT 6",
	}
	for sql, want := range cases {
		got, err := sqlglot.Transpile(sql, dialect, "mariadb", generator.Options{})
		if err != nil {
			t.Fatalf("transpile %q: %v", sql, err)
		}
		if got[0] != want {
			t.Fatalf("transpile %q = %q, want %q", sql, got[0], want)
		}
	}
	// Without the version setting both markers stay ordinary comments: the body is dropped.
	for _, sql := range []string{"SELECT 0 /*M!100100 1 */", "SELECT 0 /*!100100 1 */"} {
		got, err := sqlglot.Transpile(sql, "mariadb", "mariadb", generator.Options{})
		if err != nil {
			t.Fatalf("transpile %q without mysql_version: %v", sql, err)
		}
		if got[0] != "SELECT 0 /* M!100100 1 */" && got[0] != "SELECT 0 /* !100100 1 */" {
			t.Fatalf("expected the comment body to stay inert without mysql_version, got %q", got[0])
		}
	}
}

// TestMariaDBDialectDifferences pins the engine differences that are not plain MySQL
// inheritance: XOR precedence (AND binds tighter than XOR, XOR tighter than OR), the preserved
// SHOW SLAVE HOSTS spelling, and MEMBER OF rendering through MariaDB's JSON_CONTAINS.
func TestMariaDBDialectDifferences(t *testing.T) {
	expr, err := sqlglot.ParseOne("SELECT 1 XOR 1 AND 0", "mariadb")
	if err != nil {
		t.Fatal(err)
	}
	xor := expr.Expressions()[0]
	if xor.Kind() != expressions.KindXor {
		t.Fatalf("mariadb XOR precedence: expected a top-level Xor, got %s", xor.ToS())
	}
	if rhs := asKind(xor.Arg("expression")); rhs != expressions.KindAnd {
		t.Fatalf("mariadb XOR precedence: expected Xor(1, And(1, 0)), got %s", xor.ToS())
	}
	// MySQL keeps upstream's grouping, where XOR shares the AND level.
	mysqlExpr, err := sqlglot.ParseOne("SELECT 1 XOR 1 AND 0", "mysql")
	if err != nil {
		t.Fatal(err)
	}
	if got := mysqlExpr.Expressions()[0].Kind(); got != expressions.KindAnd {
		t.Fatalf("mysql XOR precedence: expected a top-level And, got %s", mysqlExpr.Expressions()[0].ToS())
	}

	assertRoundTrip(t, "mariadb", "SHOW SLAVE HOSTS", "SHOW SLAVE HOSTS")
	// MySQL canonicalizes to SHOW REPLICAS; MariaDB has no such spelling.
	if out, err := sqlglot.Transpile("SHOW SLAVE HOSTS", "mysql", "mysql", generator.Options{}); err != nil || out[0] != "SHOW REPLICAS" {
		t.Fatalf("mysql SHOW SLAVE HOSTS = %q, err %v; want SHOW REPLICAS", out, err)
	}

	out, err := sqlglot.Transpile("SELECT x MEMBER OF(y)", "mysql", "mariadb", generator.Options{})
	if err != nil || out[0] != "SELECT JSON_CONTAINS(y, x)" {
		t.Fatalf("mariadb MEMBER OF = %q, err %v; want JSON_CONTAINS", out, err)
	}
}

// asKind returns the Kind of an expression argument value, or 0 when absent.
func asKind(v any) expressions.Kind {
	expr, ok := v.(expressions.Expression)
	if !ok || expr == nil {
		return 0
	}
	return expr.Kind()
}
