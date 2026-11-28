package sqlglot_test

import "testing"

// IS binds at the dialect's IS_PREDICATE_PRECEDENCE (parsers/mysql.py:65, parsers/sqlite.py:70):
// MySQL places IS below the comparison operators and SQLite places it with =, so `0 = 1 IS NULL`
// means (0 = 1) IS NULL there. Expected outputs are the pinned upstream v30.35.0 output.
func TestIsPredicatePrecedenceMySQLAndSQLite(t *testing.T) {
	for _, dialect := range []string{"mysql", "sqlite"} {
		transpileCase(t, dialect, dialect, "SELECT 0 = 1 IS NULL", "SELECT (0 = 1) IS NULL")
	}
}
