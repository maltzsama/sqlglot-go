package dialects

import (
	"maps"

	"github.com/maltzsama/sqlglot-go/tokens"
)

// Clickhouse ports the ClickHouse dialect class from
// .reference/sqlglot-v30.17.0/sqlglot/dialects/clickhouse.py:15-131.
func Clickhouse() *Dialect {
	d := Base()
	d.Name = "clickhouse"

	// dialects/clickhouse.py:16-28 class flags.
	d.IndexOffset = 1
	// NORMALIZE_FUNCTIONS = False: function names round-trip with their original casing.
	d.NormalizeFunctions = ""
	d.NullOrdering = "nulls_are_last"
	d.SupportsUserDefinedTypes = false
	d.ForceEarlyAliasRefExpansion = true
	d.HexStringIsIntegerType = true
	// NORMALIZATION_STRATEGY = CASE_SENSITIVE.
	d.NormalizationStrategy = CaseSensitive

	// parsers/clickhouse.py:244-247 parser class flags.
	// MODIFIERS_ATTACHED_TO_SET_OP = False: an empty SetOpModifiers map disables the
	// hoist in parser.parseSetOperations (parser.py:5967-5974).
	d.SetOpModifiers = map[string]bool{}
	// INTERVAL_SPANS = False.
	d.IntervalSpans = false
	// JOINS_HAVE_EQUAL_PRECEDENCE = True.
	d.JoinsHaveEqualPrecedence = true
	// parsers/clickhouse.py:442 LOG_DEFAULTS_TO_LN = True.
	d.LogDefaultsToLn = true
	// parsers/clickhouse.py:246 OPTIONAL_ALIAS_TOKEN_CTE = False.
	d.OptionalAliasTokenCTE = false
	// dialects/clickhouse.py:22 PRESERVE_ORIGINAL_NAMES = True.
	d.PreserveOriginalNames = true
	// generators/clickhouse.py:175-182 STRUCT_DELIMITER / ALTER_SET_TYPE /
	// SUPPORTS_ALTER_COLUMN_IF_EXISTS.
	d.StructDelimiter = [2]string{"(", ")"}
	d.AlterSetType = "TYPE"
	d.SupportsAlterColumnIfExists = true
	// generators/clickhouse.py:191 AUTO_REFRESH_BARE_INTERVALS = True.
	d.AutoRefreshBareIntervals = true
	// generators/clickhouse.py:167-191: SELECT_KINDS=(), TRY_SUPPORTED=False, and the
	// table-sample / VALUES-constructor flags.
	d.WrapDerivedValues = false
	d.TablesampleRequiresParens = false
	d.TablesampleSizeIsRows = false
	d.TablesampleKeywords = "SAMPLE"
	// parsers/clickhouse.py:330-336 FUNC_TOKENS adds AND/FILE/OR/SET.
	d.ExtraFuncTokens = map[tokens.TokenType]bool{
		tokens.AND: true, tokens.FILE: true, tokens.OR: true, tokens.SET: true,
	}
	// parsers/clickhouse.py:250-254 removes TRANSFORM and APPROX_TOP_SUM from FUNCTIONS.
	d.RemovedFunctions = map[string]bool{"TRANSFORM": true, "APPROX_TOP_SUM": true}
	// parsers/clickhouse.py:340-343 ID_VAR_TOKENS adds LIKE.
	d.ExtraIdVarTokens = map[tokens.TokenType]bool{tokens.LIKE: true}
	// dialects/clickhouse.py:32-34 UNESCAPED_SEQUENCES = {"\\0": "\0"}, whose inverse is
	// the generator's ESCAPED_SEQUENCES table.
	d.EscapedSequences = map[byte]string{0: `\0`}
	// parsers/clickhouse.py:249-325 FUNCTIONS overlay.
	d.Functions = clickhouseFunctions()

	cfg := d.TokenizerConfig
	// dialects/clickhouse.py:23-24 NUMBERS_CAN_BE_UNDERSCORE_SEPARATED, IDENTIFIERS_CAN_START_WITH_DIGIT.
	cfg.NumbersCanBeUnderscoreSeparated = true
	cfg.IdentifiersCanStartWithDigit = true
	// dialects/clickhouse.py:32-34 UNESCAPED_SEQUENCES.
	cfg.UnescapedSequences = map[string]string{`\0`: "\x00"}
	// dialects/clickhouse.py:71-77 comments, identifiers, escapes, heredocs.
	cfg.Comments = maps.Clone(cfg.Comments)
	cfg.Comments["#"] = ""
	cfg.Comments["#!"] = ""
	cfg.Identifiers = maps.Clone(cfg.Identifiers)
	cfg.Identifiers['"'] = "\""
	cfg.Identifiers['`'] = "`"
	delete(cfg.SingleTokens, '`')
	cfg.IdentifierEscapes = maps.Clone(cfg.IdentifierEscapes)
	cfg.IdentifierEscapes['\\'] = true
	cfg.StringEscapes = maps.Clone(cfg.StringEscapes)
	cfg.StringEscapes['\\'] = true
	cfg.FormatStrings = maps.Clone(cfg.FormatStrings)
	cfg.FormatStrings["$"] = tokens.FormatString{End: "$", TokenType: tokens.HEREDOC_STRING}
	cfg.SingleTokens['$'] = tokens.HEREDOC_STRING

	// dialects/clickhouse.py:75-76 BIT_STRINGS = [("0b","")], HEX_STRINGS = [("0x",""),("0X","")].
	cfg.HasBitStrings = true
	cfg.HasHexStrings = true
	d.BitStart, d.BitEnd = "0b", ""
	d.HexStart, d.HexEnd = "0x", ""

	// dialects/clickhouse.py:79-120 KEYWORDS deltas.
	for keyword, tokenType := range map[string]tokens.TokenType{
		".:":                      tokens.DOTCOLON,
		".^":                      tokens.DOTCARET,
		"ATTACH":                  tokens.COMMAND,
		"DATE32":                  tokens.DATE32,
		"DETACH":                  tokens.DETACH,
		"DATETIME64":              tokens.DATETIME64,
		"DICTIONARY":              tokens.DICTIONARY,
		"DYNAMIC":                 tokens.DYNAMIC,
		"ENUM8":                   tokens.ENUM8,
		"ENUM16":                  tokens.ENUM16,
		"EXCHANGE":                tokens.COMMAND,
		"FINAL":                   tokens.FINAL,
		"FIXEDSTRING":             tokens.FIXEDSTRING,
		"FLOAT32":                 tokens.FLOAT,
		"FLOAT64":                 tokens.DOUBLE,
		"GLOBAL":                  tokens.GLOBAL,
		"LOWCARDINALITY":          tokens.LOWCARDINALITY,
		"MAP":                     tokens.MAP,
		"NESTED":                  tokens.NESTED,
		"NOTHING":                 tokens.NOTHING,
		"SAMPLE":                  tokens.TABLE_SAMPLE,
		"TUPLE":                   tokens.STRUCT,
		"UINT16":                  tokens.USMALLINT,
		"UINT32":                  tokens.UINT,
		"UINT64":                  tokens.UBIGINT,
		"UINT8":                   tokens.UTINYINT,
		"IPV4":                    tokens.IPV4,
		"IPV6":                    tokens.IPV6,
		"POINT":                   tokens.POINT,
		"PROJECTION":              tokens.PROJECTION,
		"RING":                    tokens.RING,
		"LINESTRING":              tokens.LINESTRING,
		"MULTILINESTRING":         tokens.MULTILINESTRING,
		"POLYGON":                 tokens.POLYGON,
		"MULTIPOLYGON":            tokens.MULTIPOLYGON,
		"AGGREGATEFUNCTION":       tokens.AGGREGATEFUNCTION,
		"SIMPLEAGGREGATEFUNCTION": tokens.SIMPLEAGGREGATEFUNCTION,
		"SYSTEM":                  tokens.COMMAND,
		"PREWHERE":                tokens.PREWHERE,
	} {
		cfg.Keywords[keyword] = tokenType
	}
	// dialects/clickhouse.py:122-123 KEYWORDS.pop("/*+"): `/*+` is an ordinary block
	// comment for ClickHouse, so the hint machinery is disabled (tsql does the same).
	delete(cfg.Keywords, "/*+")
	cfg.HintStart = ""
	cfg.TokensPrecedingHint = nil

	d.TokenizerConfig = tokens.CompileConfig(cfg)

	return d
}
