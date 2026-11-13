package dialects

import "github.com/maltzsama/sqlglot-go/tokens"

// MariaDB ports the MariaDB dialect identity. Upstream sqlglot has no MariaDB dialect, so this
// is a beyond-upstream extension: every MySQL behavior is inherited (the parser and generator
// override chains treat "mariadb" as a child of "mysql"), and MariaDB-only grammar is gated on
// this name. The MariaDB-only constructs are:
//   - `NEXT VALUE FOR` / `PREVIOUS VALUE FOR` sequences (parser/dialect_mariadb.go),
//   - `/*M! ... */` executable comments, activated alongside `/*!` under the `mysql_version`
//     setting (DEVIATIONS "Opt-in behavioral extensions").
//
// See DEVIATIONS "Opt-in behavioral extensions" and testdata/upstream_extensions.jsonl.
func MariaDB() *Dialect {
	d := MySQL()
	d.Name = "mariadb"

	// MariaDB executable comments: /*M! ... */ and /*M!100100 ... */ (the version is gated
	// against the same mysql_version setting as MySQL's /*! comments).
	cfg := d.TokenizerConfig
	cfg.MariaDBExecutableComments = true
	d.TokenizerConfig = tokens.CompileConfig(cfg)

	return d
}
