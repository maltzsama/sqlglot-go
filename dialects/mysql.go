package dialects

import (
	"fmt"
	"maps"
	"strings"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// mysqlTimeMapping ports MySQL.TIME_MAPPING (dialects/mysql.py:37-48): the MySQL
// %-specifiers that differ from strftime, shared by parse-time format conversion
// (DATE_FORMAT) and the dialect's TimeMapping.
var mysqlTimeMapping = map[string]string{
	"%M": "%B", "%c": "%-m", "%e": "%-d", "%h": "%I", "%i": "%M", "%s": "%S",
	"%u": "%W", "%k": "%-H", "%l": "%-I", "%T": "%H:%M:%S", "%W": "%A",
}

// applyMySQLAnsiQuotes rewrites the dialect's tokenizer config for MySQL `sql_mode=ANSI_QUOTES`: `"`
// stops being a string quote and becomes a QUOTED IDENTIFIER delimiter (alongside the backtick),
// exactly as the real server does (verified MySQL 8.0.33: under ANSI_QUOTES `SELECT "x"` reads column
// `x`, and a string literal must use `'`). Only the PARSE side changes — the generator already emits
// strings with `'` (QuoteStart) and identifiers with the backtick, both valid under ANSI_QUOTES, so a
// re-generated statement is correct in that mode with no generator change. No-op for a non-MySQL base
// (base/postgres already treat `"` as a quoted identifier). See DEVIATIONS.
func applyMySQLAnsiQuotes(d *Dialect) {
	if d.Name != "mysql" && d.Name != "mariadb" {
		return
	}
	cfg := d.TokenizerConfig
	// Clone the maps we mutate so a shared/cached config can never be disturbed.
	cfg.Quotes = maps.Clone(cfg.Quotes)
	cfg.Identifiers = maps.Clone(cfg.Identifiers)
	cfg.FormatStrings = maps.Clone(cfg.FormatStrings)
	cfg.StringEscapes = maps.Clone(cfg.StringEscapes)

	delete(cfg.Quotes, `"`)         // `"` is no longer a string literal,
	delete(cfg.FormatStrings, `n"`) // nor the base for the N"…" national-string forms
	delete(cfg.FormatStrings, `N"`) // that compileConfig derives from a `"` quote,
	cfg.Identifiers['"'] = `"`      // it is a quoted-identifier delimiter instead;
	// and it is no longer a string escape either — MySQL adds `"` to StringEscapes for `""`-doubling
	// INSIDE a `"`-string (default mode), but under ANSI_QUOTES there are no `"`-strings, so leaving it
	// would make a `"` before a closing `'` wrongly escape the terminator (`SELECT '\n"'` fails to
	// tokenize). Doubling inside a `"`-IDENTIFIER (`"a""b"`) is handled by the identifier scanner, not
	// this map.
	delete(cfg.StringEscapes, '"')

	d.TokenizerConfig = tokens.CompileConfig(cfg) // rebuild the keyword trie + re-derive
}

// parseMySQLVersion parses the mysql_version setting into MySQL's MYSQL_VERSION_ID —
// the comparable integer major*10000 + minor*100 + patch (e.g. 80035 for MySQL 8.0.35).
// This is exactly the /*!NNNNN version-comment gate form and what the C API
// mysql_get_server_version() returns, so a client passes the integer it already has.
// The value must be a plain decimal integer; a dotted version string is not accepted.
func parseMySQLVersion(value string) (int, error) {
	value = strings.TrimSpace(value)
	invalid := func(reason string) (int, error) {
		return 0, fmt.Errorf("invalid mysql_version %q: %s", value, reason)
	}
	if value == "" {
		return invalid("expected a MYSQL_VERSION_ID integer")
	}

	maxInt := int(^uint(0) >> 1)
	id := 0
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return invalid("expected a MYSQL_VERSION_ID integer, e.g. 80035 for MySQL 8.0.35")
		}
		digit := int(value[i] - '0')
		if id > (maxInt-digit)/10 {
			return invalid("MYSQL_VERSION_ID overflows int")
		}
		id = id*10 + digit
	}
	return id, nil
}

func MySQL() *Dialect {
	d := Base()
	d.Name = "mysql"
	// generators/mysql.py:124 / generators/tsql.py:138 NULL_ORDERING_SUPPORTED = None.
	d.NullOrderingSupported = nil
	// generators/mysql.py PAD_FILL_PATTERN_IS_REQUIRED = True.
	d.PadFillPatternIsRequired = true
	// generators/mysql.py TRANSFORMS: eliminate_distinct_on.
	d.EliminateDistinctOn = true
	// generators/mysql.py TRANSFORMS: eliminate_semi_and_anti_joins.
	d.EliminateSemiAntiJoins = true
	// generators/mysql.py LIMIT_FETCH = "LIMIT": render FETCH as LIMIT.
	d.LimitFetch = "LIMIT"
	// generators/mysql.py NVL2_SUPPORTED = False.
	d.Nvl2Supported = false
	// parsers/mysql.py:298 LOG_DEFAULTS_TO_LN.
	d.LogDefaultsToLn = true
	// dialects/mysql.py:31 SAFE_TO_ELIMINATE_DOUBLE_NEGATION = False.
	d.SafeToEliminateDoubleNegation = false
	d.PromoteToInferredDatetimeType = true
	// generators/mysql.py:118 SUPPORTS_ALTER_COLUMN_NULLABILITY = True.
	d.SupportsAlterColumnNullability = true
	d.QuoteStart = "'"
	d.QuoteEnd = "'"
	d.IdentifierStart = "`"
	d.IdentifierEnd = "`"
	// mysql.py:25 NORMALIZATION_STRATEGY = CASE_SENSITIVE (upstream default, kept for
	// faithfulness — no folding). For correct+safe identifier folding (columns are
	// case-insensitive on every MySQL platform), a consumer opts into a MySQL strategy via the
	// settings string, e.g.
	//   GetOrRaise("mysql, normalization_strategy=mysql_case_sensitive_table_names")   (lctn=0)
	//   GetOrRaise("mysql, normalization_strategy=mysql_case_insensitive")             (lctn=1/2)
	// Those strategies fold with MySQL's exact .tolower map (exported as MySQLLower).
	d.NormalizationStrategy = CaseSensitive
	d.DPipeIsStringConcat = false
	d.SupportsUserDefinedTypes = false
	d.SafeDivision = true
	// generators/mysql.py:216 TRY_SUPPORTED = False.
	d.TrySupported = false
	// generators/mysql.py:141 UPDATE_STATEMENT_SUPPORTS_FROM = False: a parsed UPDATE ... FROM
	// is rewritten into JOIN ... ON TRUE by the generator's updateFromJoinsSQL.
	d.UpdateStatementSupportsFrom = false
	// parsers/mysql.py:68 FUNC_TOKENS adds TokenType.VALUES (see ValuesIsFunction doc).
	d.ValuesIsFunction = true
	// parsers/mysql.py:69 FUNC_TOKENS adds TokenType.CHARACTER_SET (see CharsetIsFunction doc).
	d.CharsetIsFunction = true
	// generators/mysql.py:137 DUPLICATE_KEY_UPDATE_WITH_SET = False.
	d.DuplicateKeyUpdateWithSet = false
	// generators/mysql.py:136 PARSE_JSON_NAME = None (bare value).
	d.ParseJSONName = stringPtr("")
	// generators/mysql.py:148 WRAP_DERIVED_VALUES = False: aliased VALUES is emitted bare.
	d.WrapDerivedValues = false
	// generators/mysql.py:134 JSON_KEY_VALUE_PAIR_SEP = "," (JSON_OBJECT pairs join with a comma).
	d.JSONKeyValuePairSep = ","
	// generators/mysql.py:139 VALUES_AS_TABLE = False: a table-source VALUES is rewritten
	// into `(SELECT ... UNION ALL ...) AS t` instead of a bare VALUES constructor.
	d.ValuesAsTable = false
	// generators/mysql.py:127-128 SUPPORTS_MODIFY_COLUMN/SUPPORTS_CHANGE_COLUMN = True.
	d.SupportsModifyColumn = true
	d.SupportsChangeColumn = true
	// generators/mysql.py:149 VARCHAR_REQUIRES_SIZE = True.
	d.VarcharRequiresSize = true
	// generators/mysql.py:132 INTERVAL_ALLOWS_PLURAL_FORM = False.
	d.IntervalAllowsPluralForm = false
	// generators/mysql.py:276 LIMIT_ONLY_LITERALS = True.
	d.LimitOnlyLiterals = true
	// generators/mysql.py:133 LOCKING_READS_SUPPORTED = True.
	d.LockingReadsSupported = true
	// generators/mysql.py:142 JSON_TYPE_REQUIRED_FOR_EXTRACTION = True.
	d.JSONTypeRequiredForExtraction = true
	// parsers/mysql.py:303 VALUES_FOLLOWED_BY_PAREN = False.
	d.ValuesFollowedByParen = false
	// parsers/mysql.py:304 SUPPORTS_PARTITION_SELECTION = True: allow `FROM t PARTITION(p0)`.
	d.SupportsPartitionSelection = true
	// parsers/mysql.py:302 STRING_ALIASES = True: fold a trailing string into an identifier
	// alias in parseAlias, e.g. `SELECT 1 'x'` -> SELECT 1 AS `x` (base/postgres reject it).
	d.StringAliases = true
	// Real MySQL 8.0.33 rejects a bare string as a table name (`FROM 'foo'`) or table alias
	// (`FROM t 'x'`), even though it accepts the projection string alias above; see the field doc.
	d.StringTableIdentifiers = false
	// generators/mysql.py:339 RESERVED_KEYWORDS: quote reserved words used as identifiers
	// (e.g. `SELECT 1 AS row` -> SELECT 1 AS `row`). See dialects/mysql_reserved.go.
	d.ReservedKeywords = mysqlReservedKeywords

	// parsers/mysql.py:106-148 FUNCTIONS overlay. MySQL date-part functions
	// coerce their first argument to a date, including WEEK with an optional mode.
	d.Functions = map[string]func([]exp.Expression) exp.Expression{
		"CHAR_LENGTH":      exp.FromArgListFunc(exp.KindLength),
		"STR_TO_DATE":      mysqlBuildStrToDate,
		"CHARACTER_LENGTH": exp.FromArgListFunc(exp.KindLength),
		"CURDATE":          exp.FromArgListFunc(exp.KindCurrentDate),
		"CURTIME":          exp.FromArgListFunc(exp.KindCurrentTime),
		"DATABASE":         exp.FromArgListFunc(exp.KindCurrentSchema),
		"DAY":              mysqlDatePart(exp.KindDay),
		"DAYOFMONTH":       mysqlDatePart(exp.KindDayOfMonth),
		"DAYOFWEEK":        mysqlDatePart(exp.KindDayOfWeek),
		"DAYOFYEAR":        mysqlDatePart(exp.KindDayOfYear),
		"MONTH":            mysqlDatePart(exp.KindMonth),
		"SCHEMA":           exp.FromArgListFunc(exp.KindCurrentSchema),
		"TIME_STR_TO_UNIX": exp.FromArgListFunc(exp.KindTimeStrToUnix),
		"WEEK":             mysqlDatePart(exp.KindWeek),
		"WEEKOFYEAR":       mysqlDatePart(exp.KindWeekOfYear),
		"YEAR":             mysqlDatePart(exp.KindYear),
		// parsers/mysql.py:122: FROM_UNIXTIME parses with the MySQL time format.
		"FROM_UNIXTIME": func(args []exp.Expression) exp.Expression {
			format := hiveSeqGet(args, 1)
			if format != nil && format.IsString() {
				if converted, ok := FormatTime(format.Name(), mysqlTimeMapping); ok {
					format = exp.LiteralString(converted)
				}
			}
			return exp.New(exp.KindUnixToTime, exp.Args{"this": hiveSeqGet(args, 0), "format": format})
		},
		// generators/mysql.py:177 exp.NumberToStr: FORMAT(value, decimals).
		"FORMAT":  exp.FromArgListFunc(exp.KindNumberToStr),
		"VERSION": exp.FromArgListFunc(exp.KindCurrentVersion),
		"CONVERT_TZ": func(args []exp.Expression) exp.Expression {
			return exp.New(exp.KindConvertTimezone, exp.Args{
				"source_tz": hiveSeqGet(args, 1), "target_tz": hiveSeqGet(args, 2), "timestamp": hiveSeqGet(args, 0),
			})
		},
		// parsers/mysql.py:123: ISNULL becomes a parenthesized IS NULL.
		"ISNULL": func(args []exp.Expression) exp.Expression {
			return exp.Paren(exp.Args{"this": exp.Is(exp.Args{
				"this":       hiveSeqGet(args, 0),
				"expression": exp.Null(),
			})})
		},
		// parsers/mysql.py:112-115: DATE_FORMAT(value, format) is a TimeToStr over a
		// timestamp cast, with the format converted from MySQL's %-specifiers.
		"DATE_FORMAT": func(args []exp.Expression) exp.Expression {
			format := hiveSeqGet(args, 1)
			if format != nil && format.IsString() {
				if converted, ok := FormatTime(format.Name(), mysqlTimeMapping); ok {
					format = exp.LiteralString(converted)
				}
			}
			return exp.TimeToStr(exp.Args{
				"this":   exp.TsOrDsToTimestamp(exp.Args{"this": hiveSeqGet(args, 0)}),
				"format": format,
			})
		},
		// parsers/mysql.py:127-130: MONTHNAME is DATE_FORMAT(x, '%M').
		"MONTHNAME": func(args []exp.Expression) exp.Expression {
			return exp.TimeToStr(exp.Args{
				"this":   exp.TsOrDsToDate(exp.Args{"this": hiveSeqGet(args, 0)}),
				"format": exp.LiteralString("%B"),
			})
		},
		// parsers/mysql.py:134: TIMESTAMPDIFF(unit, expression, this).
		"TIMESTAMPDIFF": sparkBuildDateDelta(exp.KindTimestampDiff),
		// parsers/mysql.py:135-141: TO_DAYS counts from year zero.
		"TO_DAYS": func(args []exp.Expression) exp.Expression {
			diff := exp.DateDiff(exp.Args{
				"this":       exp.TsOrDsToDate(exp.Args{"this": hiveSeqGet(args, 0)}),
				"expression": exp.TsOrDsToDate(exp.Args{"this": exp.LiteralString("0000-01-01")}),
				"unit":       exp.Var(exp.Args{"this": "DAY"}),
			})
			return exp.Paren(exp.Args{"this": exp.Add(exp.Args{
				"this":       diff,
				"expression": exp.LiteralNumber("1"),
			})})
		},
		// parsers/mysql.py:108-110 (v30.17.0): MySQL DATEDIFF counts date-part boundaries.
		"DATEDIFF": func(args []exp.Expression) exp.Expression {
			return exp.New(exp.KindDateDiff, exp.Args{
				"this": hiveSeqGet(args, 0), "expression": hiveSeqGet(args, 1), "date_part_boundary": true,
			})
		},
	}

	for _, unit := range []string{
		"SECOND_MICROSECOND",
		"MINUTE_MICROSECOND",
		"MINUTE_SECOND",
		"HOUR_MICROSECOND",
		"HOUR_SECOND",
		"HOUR_MINUTE",
		"DAY_MICROSECOND",
		"DAY_SECOND",
		"DAY_MINUTE",
		"DAY_HOUR",
		"YEAR_MONTH",
	} {
		d.ValidIntervalUnits[unit] = true
	}

	cfg := tokens.BaseConfig()
	cfg.Quotes["\""] = "\""
	cfg.Identifiers = map[rune]string{'`': "`"}
	cfg.Comments["#"] = ""
	cfg.MySQLExecutableComments = true
	// MySQL requires `--` to be followed by whitespace/control (or EOF) to start a line
	// comment; otherwise it is two `-` operators (`1--2` == `1 - -2`). Upstream sqlglot
	// mis-tokenizes this — see DEVIATIONS §1.4. `#` needs no such guard (`#comment` is a
	// full-line comment in MySQL).
	cfg.LineCommentRequiresSpace = map[string]bool{"--": true}
	cfg.StringEscapes['"'] = true
	cfg.StringEscapes['\\'] = true
	for _, r := range []rune{'0', 'b', 'n', 'r', 't', 'Z', '%', '_'} {
		cfg.EscapeFollowChars[r] = true
	}
	cfg.HasBitStrings = true
	cfg.HasHexStrings = true
	cfg.FormatStrings["b'"] = tokens.FormatString{End: "'", TokenType: tokens.BIT_STRING}
	cfg.FormatStrings["B'"] = tokens.FormatString{End: "'", TokenType: tokens.BIT_STRING}
	cfg.FormatStrings["x'"] = tokens.FormatString{End: "'", TokenType: tokens.HEX_STRING}
	cfg.FormatStrings["X'"] = tokens.FormatString{End: "'", TokenType: tokens.HEX_STRING}
	// dialects/mysql.py:71-72 BIT_STRINGS=[("b'","'"),("B'","'"),("0b","")] / HEX_STRINGS=
	// [("x'","'"),("X'","'"),("0x","")]; BIT_START/HEX_START take the FIRST tuple.
	d.BitStart, d.BitEnd = "b'", "'"
	d.HexStart, d.HexEnd = "x'", "'"
	// generators/mysql.py JSON_PATH_BRACKETED_KEY_SUPPORTED = False.
	d.JSONPathBracketedKeySupported = false
	// dialects/mysql.py:37-48 TIME_MAPPING and its inverse.
	d.TimeMapping = mysqlTimeMapping
	d.InverseTimeMapping = map[string]string{
		"%B": "%M", "%-m": "%c", "%-d": "%e", "%I": "%h", "%M": "%i", "%S": "%s",
		"%W": "%u", "%-H": "%k", "%-I": "%l", "%H:%M:%S": "%T", "%A": "%W",
	}
	cfg.NestedComments = false
	cfg.IdentifiersCanStartWithDigit = true

	for keyword, tokenType := range map[string]tokens.TokenType{
		"BLOB":        tokens.BLOB,
		"CHARSET":     tokens.CHARACTER_SET,
		"DISTINCTROW": tokens.DISTINCT,
		"EXPLAIN":     tokens.DESCRIBE,
		"FORCE":       tokens.FORCE,
		"IGNORE":      tokens.IGNORE,
		"KEY":         tokens.KEY,
		"LOCK TABLES": tokens.COMMAND,
		"LONGBLOB":    tokens.LONGBLOB,
		"LONGTEXT":    tokens.LONGTEXT,
		"MEDIUMBLOB":  tokens.MEDIUMBLOB,
		"MEDIUMINT":   tokens.MEDIUMINT,
		"MEDIUMTEXT":  tokens.MEDIUMTEXT,
		"MEMBER OF":   tokens.MEMBER_OF,
		"MOD":         tokens.MOD,
		// RESET (RESET MASTER / RESET REPLICA / RESET BINARY LOGS AND GTIDS) is a MySQL
		// admin statement. Without this, RESET falls into the generic expression path and
		// mis-parses as an Alias (`RESET AS MASTER`) — a semantically wrong structural claim.
		// Tokenizing it as COMMAND degrades the whole statement to a raw exp.Command, matching
		// how Postgres already handles RESET (postgres.go). See DEVIATIONS §1. divergence:
		// upstream MySQL leaves RESET unmapped and produces the bogus Alias.
		"RESET":            tokens.COMMAND,
		"SEPARATOR":        tokens.SEPARATOR,
		"SERIAL":           tokens.SERIAL,
		"SIGNED":           tokens.BIGINT,
		"SIGNED INTEGER":   tokens.BIGINT,
		"SOUNDS LIKE":      tokens.SOUNDS_LIKE,
		"START":            tokens.BEGIN,
		"TIMESTAMP":        tokens.TIMESTAMPTZ,
		"TINYBLOB":         tokens.TINYBLOB,
		"TINYTEXT":         tokens.TINYTEXT,
		"UNLOCK TABLES":    tokens.COMMAND,
		"UNSIGNED":         tokens.UBIGINT,
		"UNSIGNED INTEGER": tokens.UBIGINT,
		"YEAR":             tokens.YEAR,
		"_ARMSCII8":        tokens.INTRODUCER,
		"_ASCII":           tokens.INTRODUCER,
		"_BIG5":            tokens.INTRODUCER,
		"_BINARY":          tokens.INTRODUCER,
		"_CP1250":          tokens.INTRODUCER,
		"_CP1251":          tokens.INTRODUCER,
		"_CP1256":          tokens.INTRODUCER,
		"_CP1257":          tokens.INTRODUCER,
		"_CP850":           tokens.INTRODUCER,
		"_CP852":           tokens.INTRODUCER,
		"_CP866":           tokens.INTRODUCER,
		"_CP932":           tokens.INTRODUCER,
		"_DEC8":            tokens.INTRODUCER,
		"_EUCJPMS":         tokens.INTRODUCER,
		"_EUCKR":           tokens.INTRODUCER,
		"_GB18030":         tokens.INTRODUCER,
		"_GB2312":          tokens.INTRODUCER,
		"_GBK":             tokens.INTRODUCER,
		"_GEOSTD8":         tokens.INTRODUCER,
		"_GREEK":           tokens.INTRODUCER,
		"_HEBREW":          tokens.INTRODUCER,
		"_HP8":             tokens.INTRODUCER,
		"_KEYBCS2":         tokens.INTRODUCER,
		"_KOI8R":           tokens.INTRODUCER,
		"_KOI8U":           tokens.INTRODUCER,
		"_LATIN1":          tokens.INTRODUCER,
		"_LATIN2":          tokens.INTRODUCER,
		"_LATIN5":          tokens.INTRODUCER,
		"_LATIN7":          tokens.INTRODUCER,
		"_MACCE":           tokens.INTRODUCER,
		"_MACROMAN":        tokens.INTRODUCER,
		"_SJIS":            tokens.INTRODUCER,
		"_SWE7":            tokens.INTRODUCER,
		"_TIS620":          tokens.INTRODUCER,
		"_UCS2":            tokens.INTRODUCER,
		"_UJIS":            tokens.INTRODUCER,
		"_UTF8":            tokens.INTRODUCER,
		"_UTF16":           tokens.INTRODUCER,
		"_UTF16LE":         tokens.INTRODUCER,
		"_UTF32":           tokens.INTRODUCER,
		"_UTF8MB3":         tokens.INTRODUCER,
		"_UTF8MB4":         tokens.INTRODUCER,
		"@@":               tokens.SESSION_PARAMETER,
	} {
		cfg.Keywords[keyword] = tokenType
	}
	// The pinned .reference/sqlglot-v30.12.0/sqlglot/dialects/mysql.py:154 adds
	// REPLACE as a Command, but the mysql-replace extension preserves its suffix
	// tokens for statement parsing.
	delete(cfg.Commands, tokens.REPLACE)
	delete(cfg.Commands, tokens.SHOW)
	d.TokenizerConfig = tokens.CompileConfig(cfg)
	return d
}

func mysqlDatePart(kind exp.Kind) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		part := exp.FromArgList(kind, args)
		part.Set("this", exp.TsOrDsToDate(exp.Args{"this": part.This()}))
		return part
	}
}

// mysqlTimeSpecifiers is TIME_SPECIFIERS (parsers/mysql.py:20): the MySQL format codes that
// carry a time of day.
const mysqlTimeSpecifiers = "fHhIiklpSsT"

// mysqlBuildStrToDate ports _str_to_date (parsers/mysql.py:36-44): the format is converted
// through MySQL's TIME_MAPPING, and a format with a time specifier yields StrToTime.
func mysqlBuildStrToDate(args []exp.Expression) exp.Expression {
	this := seqGet(args, 0)
	format := seqGet(args, 1)
	kind := exp.KindStrToDate
	var converted exp.Expression
	if format != nil {
		if format.IsString() {
			if forward, ok := FormatTime(format.Name(), mysqlTimeMapping); ok {
				converted = exp.LiteralString(forward)
			} else {
				converted = exp.LiteralString(format.Name())
			}
			if hasTimeSpecifier(format.Name()) {
				kind = exp.KindStrToTime
			}
		} else {
			converted = format
		}
	}
	return exp.New(kind, exp.Args{"this": this, "format": converted})
}

// hasTimeSpecifier ports _has_time_specifier (parsers/mysql.py:23-34).
func hasTimeSpecifier(format string) bool {
	for i := 0; i+1 < len(format); i++ {
		if format[i] == '%' && strings.ContainsRune(mysqlTimeSpecifiers, rune(format[i+1])) {
			return true
		}
	}
	return false
}
