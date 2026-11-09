package dialects

import (
	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

func Postgres() *Dialect {
	d := Base()
	d.Name = "postgres"
	d.QuoteStart = "'"
	d.QuoteEnd = "'"
	d.IdentifierStart = "\""
	d.IdentifierEnd = "\""
	d.IndexOffset = 1
	d.TypedDivision = true
	// dialects/postgres.py:15 CONCAT_COALESCE = True.
	d.ConcatCoalesce = true
	d.NullOrdering = "nulls_are_large"
	d.SupportsLimitAll = true
	d.TablesReferenceableAsColumns = true
	// generator.py:466 SUPPORTS_SELECT_INTO is overridden True for postgres: `SELECT ... INTO x`
	// stays inline instead of being rewritten to `CREATE TABLE x AS SELECT ...`.
	d.SupportsSelectInto = true
	// Real PostgreSQL 17.6 rejects a bare string as a table name (`FROM 'foo'`) or table alias
	// (`FROM t 'x'`); see the field doc.
	d.StringTableIdentifiers = false
	// generators/postgres.py:234 RENAME_TABLE_WITH_DB = False.
	d.RenameTableWithDB = false
	// generators/postgres.py:233 SINGLE_STRING_INTERVAL = True.
	d.SingleStringInterval = true
	// generators/postgres.py:240 PARAMETER_TOKEN = "$".
	d.ParameterToken = "$"
	// generators/postgres.py:235 LOCKING_READS_SUPPORTED = True.
	d.LockingReadsSupported = true
	// generators/postgres.py:280 SUPPORTS_BETWEEN_FLAGS = True.
	d.SupportsBetweenFlags = true
	// dialects/postgres.py:20 TABLESAMPLE_SIZE_IS_PERCENT = True.
	d.TablesampleSizeIsPercent = true
	// generators/postgres.py:242 TABLESAMPLE_SIZE_IS_ROWS = False.
	d.TablesampleSizeIsRows = false
	// generators/postgres.py:243 TABLESAMPLE_SEED_KEYWORD = "REPEATABLE".
	d.TablesampleSeedKeyword = "REPEATABLE"
	// generators/postgres.py:251 COPY_HAS_INTO_KEYWORD = False.
	d.CopyHasIntoKeyword = false
	// parsers/postgres.py:191 JSON_ARROWS_REQUIRE_JSON_TYPE = True.
	d.JSONArrowsRequireJSONType = true
	// parsers/postgres.py:192-219 (v30.17.0): JSON operators are binary-tier, not accessors.
	d.JSONOperatorsAreBinary = true
	// dialects/postgres.py:16 NORMALIZE_NOT_NULL = False: NOTNULL keeps Is(negate=True).
	d.NormalizeNotNull = false
	// generators/postgres.py:245 JSON_TYPE_REQUIRED_FOR_EXTRACTION = True.
	d.JSONTypeRequiredForExtraction = true

	// dialects/postgres.py:30-70 TIME_MAPPING, with the metaclass-generated inverse
	// (dialect.py:261-271). Consumed parse-side by the TO_CHAR builder below
	// (parsers/postgres.py:122 build_formatted_time) and generate-side by
	// generators/postgres.py:399 exp.TimeToStr -> TO_CHAR.
	d.TimeMapping = postgresTimeMapping
	d.InverseTimeMapping = inverseTimeMapping(postgresTimeMapping, postgresTimeMappingOrder)
	// TODO(slice 5b): DEFAULT_FUNCTIONS_COLUMN_NAMES (needs KindExplodingGenerateSeries + FUNCTIONS override).
	// TODO(slice 5b): typing/{mysql,postgres}.py EXPRESSION_METADATA — feeds annotate_types only, off probe's path (ROADMAP 4c).

	// CHAR_LENGTH/CHARACTER_LENGTH close parity_gaps.txt gaps 177-178. Length._sql_names =
	// ["LENGTH", "LEN", "CHAR_LENGTH", "CHARACTER_LENGTH"] (string.py:69-71) is base-scope
	// upstream, but functions.go:100-105 explicitly warns against unifying LENGTH globally
	// (MySQL's LENGTH is byte-length, CHAR_LENGTH is char-length - a real semantic split,
	// deferred to ROADMAP 5b), so only postgres (which has no such split: LENGTH,
	// CHAR_LENGTH and CHARACTER_LENGTH all render identically as LENGTH(...), no generator
	// override needed - see the default className fallback) gets these two spellings here.
	// "LENGTH" itself is left unregistered: it already round-trips correctly as Anonymous.
	d.Functions = map[string]func([]exp.Expression) exp.Expression{
		"CHAR_LENGTH":      exp.FromArgListFunc(exp.KindLength),
		"CHARACTER_LENGTH": exp.FromArgListFunc(exp.KindLength),
		// parsers/postgres.py:120 "NOW": exp.CurrentTimestamp.from_arg_list. Materialize and
		// RisingWave inherit this through their PostgresParser chain, so `NOW()` builds
		// CurrentTimestamp and renders bare CURRENT_TIMESTAMP there (niladicBareDialects).
		"NOW": exp.FromArgListFunc(exp.KindCurrentTimestamp),
		// parsers/postgres.py:111 "DATE_TRUNC": build_timestamp_trunc (dialect.py:1903-1904).
		// Upstream additionally normalizes the unit through TimeUnit.__init__
		// (core.py:2055-2066: an uppercased, unabbreviated Var), which exp.TimeUnitVar ports,
		// so `DATE_TRUNC('month', x)` generates `DATE_TRUNC('MONTH', x)`.
		"DATE_TRUNC": postgresBuildTimestampTrunc,
		// parsers/postgres.py:122 "TO_CHAR": build_formatted_time(exp.TimeToStr). The builder
		// reads the mapping off the Dialect at call time, so subclasses that override
		// TIME_MAPPING after Postgres() returns (Redshift's MON/MONTH) convert with their own
		// table — the same object identity upstream gets from the class-level TIME_TRIE.
		"TO_CHAR": postgresBuildFormattedTime(d),
	}

	cfg := tokens.BaseConfig()
	// has_bit_strings/has_hex_strings = bool(BIT_STRINGS)/bool(HEX_STRINGS) (tokens.py:581-582);
	// postgres BIT_STRINGS/HEX_STRINGS are non-empty (dialects/postgres.py:65-66), which also
	// enables the number scanner's bare `0b`/`0x` forms (`SELECT 0xFF` -> `SELECT x'FF'`).
	cfg.HasBitStrings = true
	cfg.HasHexStrings = true
	cfg.FormatStrings["b'"] = tokens.FormatString{End: "'", TokenType: tokens.BIT_STRING}
	cfg.FormatStrings["B'"] = tokens.FormatString{End: "'", TokenType: tokens.BIT_STRING}
	cfg.FormatStrings["x'"] = tokens.FormatString{End: "'", TokenType: tokens.HEX_STRING}
	cfg.FormatStrings["X'"] = tokens.FormatString{End: "'", TokenType: tokens.HEX_STRING}
	cfg.FormatStrings["e'"] = tokens.FormatString{End: "'", TokenType: tokens.BYTE_STRING}
	cfg.FormatStrings["E'"] = tokens.FormatString{End: "'", TokenType: tokens.BYTE_STRING}
	cfg.ByteStringEscapes = map[rune]bool{'\'': true, '\\': true}
	// SQL-standard Unicode-escaped forms (standard_conforming_strings, on by default in
	// Postgres): `U&'...'` string literals and `U&"..."` quoted identifiers, decoded into the
	// real code points they denote so an AST consumer sees the actual string/identifier the
	// server executes — e.g. `U&"inf\006Frmation_schema"` is the identifier information_schema.
	// DecodeUnicode drives the extract+decode (tokens/unicode_escape.go). Beyond pinned
	// upstream, which mis-tokenizes `U&'...'` as `U & '...'` (bitwise-AND of a column named U)
	// and parse-errors the identifier form; see DEVIATIONS §1.
	for _, prefix := range []string{"U&", "u&"} {
		cfg.FormatStrings[prefix+"'"] = tokens.FormatString{End: "'", TokenType: tokens.STRING, DecodeUnicode: true}
		cfg.FormatStrings[prefix+`"`] = tokens.FormatString{End: `"`, TokenType: tokens.IDENTIFIER, DecodeUnicode: true}
	}
	// dialects/postgres.py:65-67 BIT_STRINGS=[("b'","'"),("B'","'")] / HEX_STRINGS=
	// [("x'","'"),("X'","'")] / BYTE_STRINGS=[("e'","'"),("E'","'")]; *_START take the
	// FIRST tuple of each.
	d.BitStart, d.BitEnd = "b'", "'"
	d.HexStart, d.HexEnd = "x'", "'"
	d.ByteStart, d.ByteEnd = "e'", "'"
	cfg.FormatStrings["$"] = tokens.FormatString{End: "$", TokenType: tokens.HEREDOC_STRING}
	cfg.SingleTokens['$'] = tokens.HEREDOC_STRING
	cfg.VarSingleTokens['$'] = true
	cfg.HeredocTagIsIdentifier = true
	cfg.HeredocStringAlternative = tokens.PARAMETER

	for keyword, tokenType := range map[string]tokens.TokenType{
		"~":     tokens.RLIKE,
		"@@":    tokens.DAT,
		"@?":    tokens.AT_QMARK,
		"@>":    tokens.AT_GT,
		"<@":    tokens.LT_AT,
		"?&":    tokens.QMARK_AMP,
		"?|":    tokens.QMARK_PIPE,
		"#-":    tokens.HASH_DASH,
		"|/":    tokens.PIPE_SLASH,
		"||/":   tokens.DPIPE_SLASH,
		"BEGIN": tokens.BEGIN,
		// START (as in `START TRANSACTION`) is a synonym for BEGIN — standard SQL, and exactly a
		// transaction boundary in Postgres. mysql/presto/oracle already map "START"->BEGIN upstream;
		// postgres does not, so upstream (and this port before) mis-parse `START TRANSACTION` as an
		// Alias and error on its modes. This maps it so it routes through parseTransaction like BEGIN.
		// Matches real PG; a grammar extension (upstream parse-errors it) — see DEVIATIONS
		// 'Grammar extensions beyond upstream' and ledger id pg-start-transaction.
		"START":     tokens.BEGIN,
		"BIGSERIAL": tokens.BIGSERIAL,
		"CSTRING":   tokens.PSEUDO_TYPE,
		"DECLARE":   tokens.COMMAND,
		"DO":        tokens.COMMAND,
		"EXEC":      tokens.COMMAND,
		"EXPLAIN":   tokens.DESCRIBE, // Ledgered pg-explain: non-COMMAND prevents swallowing remaining SQL as raw text.
		"HSTORE":    tokens.HSTORE,
		"INT8":      tokens.BIGINT,
		"MONEY":     tokens.MONEY,
		"NAME":      tokens.NAME,
		"OID":       tokens.OBJECT_IDENTIFIER,
		"ONLY":      tokens.ONLY,
		"POINT":     tokens.POINT,
		"REFRESH":   tokens.COMMAND,
		"REINDEX":   tokens.COMMAND,
		// RESET is deliberately NOT mapped to COMMAND (unlike pinned upstream postgres.py:101 and
		// unlike MySQL, whose RESET MASTER/REPLICA is a privileged admin op that stays a raw Command).
		// Postgres `RESET { name | ALL }` is a session GUC reset; leaving RESET as a plain VAR lets
		// parseResetStatement build a structured exp.Reset from real tokens. Grammar extension, ledger
		// id pg-reset. See DEVIATIONS.
		"SERIAL":        tokens.SERIAL,
		"SMALLSERIAL":   tokens.SMALLSERIAL,
		"TEMP":          tokens.TEMPORARY,
		"TYPE":          tokens.TYPE,
		"REGCLASS":      tokens.OBJECT_IDENTIFIER,
		"REGCOLLATION":  tokens.OBJECT_IDENTIFIER,
		"REGCONFIG":     tokens.OBJECT_IDENTIFIER,
		"REGDICTIONARY": tokens.OBJECT_IDENTIFIER,
		"REGNAMESPACE":  tokens.OBJECT_IDENTIFIER,
		"REGOPER":       tokens.OBJECT_IDENTIFIER,
		"REGOPERATOR":   tokens.OBJECT_IDENTIFIER,
		"REGPROC":       tokens.OBJECT_IDENTIFIER,
		"REGPROCEDURE":  tokens.OBJECT_IDENTIFIER,
		"REGROLE":       tokens.OBJECT_IDENTIFIER,
		"REGTYPE":       tokens.OBJECT_IDENTIFIER,
		"FLOAT":         tokens.DOUBLE,
		"XML":           tokens.XML,
		"VARIADIC":      tokens.VARIADIC,
		"INOUT":         tokens.INOUT,
	} {
		cfg.Keywords[keyword] = tokenType
	}
	delete(cfg.Keywords, "/*+")
	delete(cfg.Keywords, "DIV")
	delete(cfg.Comments, "/*+")
	// Drop SHOW from the Commands set (mirroring MySQL's dialects/mysql.go override) so the
	// tokenizer does not pack the parameter tail into one raw STRING. Postgres `SHOW { name | ALL }`
	// is then parsed from real tokens by parsePostgresShow into a structured exp.Show. Grammar
	// extension, ledger id pg-show-guc. See DEVIATIONS.
	delete(cfg.Commands, tokens.SHOW)
	d.TokenizerConfig = tokens.CompileConfig(cfg)
	return d
}

// seqGet ports sqlglot.helper.seq_get: args[i] or nil when out of range.
func seqGet(args []exp.Expression, i int) exp.Expression {
	if i < 0 || i >= len(args) {
		return nil
	}
	return args[i]
}

// postgresBuildTimestampTrunc ports build_timestamp_trunc (dialects/dialect.py:1903-1904):
// TimestampTrunc(this=args[1], unit=args[0]). The unit additionally runs through
// TimeUnit.__init__ (core.py:2055-2066), which uppercases literal/var units into a Var —
// exp.TimeUnitVar carries that normalization into this port.
func postgresBuildTimestampTrunc(args []exp.Expression) exp.Expression {
	return exp.TimestampTrunc(exp.Args{
		"this": seqGet(args, 1),
		"unit": exp.TimeUnitVar(seqGet(args, 0)),
	})
}

// postgresTimeMappingOrder replays the upstream dict insertion order (dialects/postgres.py:
// 30-70) so the generated INVERSE_TIME_MAPPING resolves duplicate values deterministically
// (last key wins, exactly like the Python dict comprehension over the ordered literal).
var postgresTimeMappingOrder = []string{
	"d", "D", "dd", "DD", "ddd", "DDD", "FMDD", "FMDDD", "FMHH12", "FMHH24", "FMMI", "FMMM",
	"FMSS", "HH12", "HH24", "mi", "MI", "mm", "MM", "OF", "ss", "SS", "TMDay", "TMDy",
	"TMMon", "TMMonth", "day", "dy", "TZ", "US", "ww", "WW", "yy", "YY", "yyy", "YYY",
	"yyyy", "YYYY",
}

// postgresTimeMapping is TIME_MAPPING (dialects/postgres.py:30-70): TO_CHAR-style pattern
// -> Python strftime, in upstream insertion order.
var postgresTimeMapping = map[string]string{
	"d":   "%u", // 1-based day of week
	"D":   "%u", // 1-based day of week
	"dd":  "%d", // day of month
	"DD":  "%d", // day of month
	"ddd": "%j", // zero padded day of year
	"DDD": "%j", // zero padded day of year
	// FMDD is %-d: "- is no leading zero for Python; same for FM in postgres".
	"FMDD":    "%-d",
	"FMDDD":   "%-j",
	"FMHH12":  "%-I", // 9
	"FMHH24":  "%-H", // 9
	"FMMI":    "%-M", // Minute
	"FMMM":    "%-m", // 1
	"FMSS":    "%-S", // Second
	"HH12":    "%I",  // 09
	"HH24":    "%H",  // 09
	"mi":      "%M",  // zero padded minute
	"MI":      "%M",  // zero padded minute
	"mm":      "%m",  // 01
	"MM":      "%m",  // 01
	"OF":      "%z",  // utc offset
	"ss":      "%S",  // zero padded second
	"SS":      "%S",  // zero padded second
	"TMDay":   "%A",  // TM is locale dependent
	"TMDy":    "%a",
	"TMMon":   "%b",        // Sep
	"TMMonth": "%B",        // September
	"day":     "%Aenlower", // tuesday
	"dy":      "%aenlower", // tue
	"TZ":      "%Z",        // uppercase timezone name
	"US":      "%f",        // zero padded microsecond
	"ww":      "%U",        // 1-based week of year
	"WW":      "%U",        // 1-based week of year
	"yy":      "%y",        // 15
	"YY":      "%y",        // 15
	"yyy":     "%Ythree",   // 015
	"YYY":     "%Ythree",   // 015
	"yyyy":    "%Y",        // 2015
	"YYYY":    "%Y",        // 2015
}

// postgresBuildFormattedTime ports build_formatted_time (dialects/dialect.py:1592-1618) for
// the parsers/postgres.py:122 TO_CHAR entry: no dialect_override and no default, so a present
// format argument converts through this Dialect's TIME_MAPPING (Dialect.format_time,
// dialect.py:1014-1024: only string literals convert; anything else passes through).
func postgresBuildFormattedTime(d *Dialect) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		this := seqGet(args, 0)
		format := seqGet(args, 1)
		if format == nil {
			// fmt stays None upstream (default=None) and format_time(None) returns it unchanged.
			return exp.TimeToStr(exp.Args{"this": this})
		}
		if format.IsString() {
			converted, ok := FormatTime(format.Name(), d.TimeMapping)
			if !ok {
				// time.py:27 returns None for an empty format; Literal.string(None) stores
				// "None" — the same handling hiveFormatTime applies (dialects/hive.go:412-417).
				converted = "None"
			}
			format = exp.LiteralString(converted)
		}
		return exp.TimeToStr(exp.Args{"this": this, "format": format})
	}
}
