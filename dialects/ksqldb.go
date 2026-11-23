package dialects

import (
	"github.com/maltzsama/sqlglot-go/tokens"
)

// KsqlDB ports the ksqlDB dialect (Confluent ksqlDB, ksqldb-parser/SqlBase.g4). ksqlDB has no
// upstream sqlglot dialect, so this is a beyond-upstream dialect tracked in DEVIATIONS.md
// "Beyond-upstream dialects" and testdata/upstream_extensions.jsonl: the tokenizer overlay
// below adds the statement/query-modifier keywords upstream ships no token for, and the
// grammar callbacks live in parser/dialect_ksqldb.go + generator/dialect_ksqldb.go.
//
// The statement surface follows SqlBase.g4 1:1 where the port structures it (CREATE/INSERT/
// DROP/SHOW/LIST/DESCRIBE/ALTER/EXPLAIN/TERMINATE/PAUSE/RESUME/SET/UNSET/DEFINE/UNDEFINE and
// the query's WINDOW/PARTITION BY/EMIT clauses); PRINT / ASSERT / RUN SCRIPT degrade to a
// verbatim Command because their clauses (printClause, timeout) are not modeled.
func KsqlDB() *Dialect {
	d := Base()
	d.Name = "ksqldb"

	// ksqlDB treats identifiers and function names case-insensitively and preserves the
	// written case (no canonical UPPER/lower fold): CaseInsensitive + an empty
	// NormalizeFunctions keeps `SELECT Foo()` emitting `Foo()`.
	d.NormalizationStrategy = CaseInsensitive
	d.NormalizeFunctions = ""

	// parser flags (SqlBase.g4): DIGIT_IDENTIFIER allows `1abc` as an identifier, and the
	// backtick-quoted form is a second quoting style alongside `"..."`.
	d.AlterTableAddRequiredForEachColumn = true
	d.AlterTableIncludeColumnKeyword = true

	cfg := tokens.BaseConfig()
	cfg.IdentifiersCanStartWithDigit = true
	cfg.Identifiers['`'] = "`"
	// StringEscapes stays at the base {'\'': true}: ksqlDB's STRING rule only doubles a
	// quote (''), it has no backslash escape (SqlBase.g4:602-604).

	// Keyword overlay (SqlBase.g4:437-579). These are statement roots and the EMIT query
	// modifier that upstream's base tokenizer maps to nothing (or, for EXPLAIN, to COMMAND
	// which would pack the whole statement as raw text).
	for keyword, tokenType := range map[string]tokens.TokenType{
		"EMIT":      tokens.EMIT,
		"PRINT":     tokens.PRINT,
		"TERMINATE": tokens.TERMINATE,
		"PAUSE":     tokens.PAUSE,
		"RESUME":    tokens.RESUME,
		"UNSET":     tokens.UNSET,
		"DEFINE":    tokens.DEFINE,
		"UNDEFINE":  tokens.UNDEFINE,
		"ASSERT":    tokens.ASSERT,
		"RUN":       tokens.RUN,
		"WITHIN":    tokens.WITHIN,
		// Upstream maps EXPLAIN to COMMAND (raw-text statements); ksqlDB EXPLAIN takes a
		// statement or identifier and is structured (Describe{kind: "EXPLAIN"}).
		"EXPLAIN": tokens.EXPLAIN,
	} {
		cfg.Keywords[keyword] = tokenType
	}
	// Upstream packs a statement-leading SHOW into a raw Command (Commands contains SHOW);
	// ksqlDB SHOW/LIST are structured (Show{this: "<VERB> <PHRASE>"}), so un-pack it the
	// same way DuckDB does (dialects/duckdb.go).
	delete(cfg.Commands, tokens.SHOW)

	d.TokenizerConfig = tokens.CompileConfig(cfg)

	// ID_VAR_TOKENS additions (SqlBase.g4 nonReserved:412-435): these keywords stay usable
	// as identifiers/expression heads and as implicit table aliases — except EMIT, which
	// parseTableAlias rejects so the query modifier is not swallowed (see parser/parser.go).
	// PRINT/TERMINATE/PAUSE/RESUME/UNSET/RUN/WITHIN remain reserved (they are statement/join
	// roots and never appear as identifiers).
	d.ExtraIdVarTokens = map[tokens.TokenType]bool{
		tokens.EMIT:     true,
		tokens.DEFINE:   true,
		tokens.UNDEFINE: true,
		tokens.ASSERT:   true,
		tokens.EXPLAIN:  true,
		// ALTER is nonReserved too (SqlBase.g4:424), so `SELECT alter FROM t` must parse;
		// it stays a statement root only because parseStatement keys on the token type.
		tokens.ALTER: true,
	}
	return d
}
