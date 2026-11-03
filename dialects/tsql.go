package dialects

import (
	"maps"

	"github.com/ridi-oss/sqlglot-go/tokens"
)

// TSQL ports the T-SQL (Microsoft SQL Server) dialect class from
// .reference/sqlglot-v30.17.0/sqlglot/dialects/tsql.py:14-194.
func TSQL() *Dialect {
	d := Base()
	d.Name = "tsql"
	// dialects/tsql.py:22 NORMALIZATION_STRATEGY = CASE_INSENSITIVE.
	d.NormalizationStrategy = CaseInsensitive
	// dialects/tsql.py:19-21 TYPED_DIVISION / CONCAT_COALESCE / CONCAT_WS_COALESCE.
	d.TypedDivision = true
	d.ConcatCoalesce = true
	// parsers/tsql.py:323 LOG_DEFAULTS_TO_LN = True.
	d.LogDefaultsToLn = true
	// generators/tsql.py:141 SUPPORTS_SELECT_INTO = True.
	d.SupportsSelectInto = true
	// generators/tsql.py:145 COPY_PARAMS_EQ_REQUIRED = True.
	d.CopyParamsEqRequired = true
	// dialects/tsql.py:23 ALTER_TABLE_ADD_REQUIRED_FOR_EACH_COLUMN = False.
	d.AlterTableAddRequiredForEachColumn = false
	// generators/tsql.py:129-150.
	d.LimitIsTop = true
	d.LimitFetch = "FETCH"
	d.EnsureBools = true
	d.CteRecursiveKeywordRequired = false
	d.AlterTableIncludeColumnKeyword = false
	d.ComputedColumnWithType = false
	d.TablesampleSeedKeyword = "REPEATABLE"
	d.SupportsAlterColumnNullability = true
	d.AlterSetWrapped = true
	d.AlterSetType = ""
	// parsers/tsql.py:324 STRING_ALIASES = True.
	d.StringAliases = true
	// parsers/tsql.py:348-404 FUNCTIONS overlay.
	d.Functions = tsqlFunctions()

	// T-SQL identifier/quote delimiters: [bracketed] and "quoted" identifiers, '...' strings.
	// dialects/tsql.py:151-153.
	d.IdentifierStart = "["
	d.IdentifierEnd = "]"

	cfg := d.TokenizerConfig
	cfg.Identifiers = maps.Clone(cfg.Identifiers)
	cfg.Identifiers['['] = "]"
	delete(cfg.SingleTokens, '[') // '[' is an identifier delimiter, not a bracket token
	cfg.Identifiers['"'] = "\""
	delete(cfg.Quotes, "\"") // '"' is an identifier, not a string quote

	// dialects/tsql.py:153-155 HEX_STRINGS = [("0x",""), ("0X","")], VAR_SINGLE_TOKENS = @ $ #.
	cfg.HasHexStrings = true
	d.HexStart, d.HexEnd = "0x", ""
	cfg.VarSingleTokens['@'] = true
	cfg.VarSingleTokens['$'] = true
	cfg.VarSingleTokens['#'] = true

	// dialects/tsql.py:157-186 KEYWORDS deltas.
	for keyword, tokenType := range map[string]tokens.TokenType{
		"CLUSTERED INDEX":    tokens.INDEX,
		"DATETIME2":          tokens.DATETIME2,
		"DATETIMEOFFSET":     tokens.TIMESTAMPTZ,
		"DECLARE":            tokens.DECLARE,
		"EXEC":               tokens.EXECUTE,
		"GO":                 tokens.COMMAND,
		"IMAGE":              tokens.IMAGE,
		"MONEY":              tokens.MONEY,
		"NONCLUSTERED INDEX": tokens.INDEX,
		"NTEXT":              tokens.TEXT,
		"OPTION":             tokens.OPTION,
		"OUTPUT":             tokens.RETURNING,
		"PRINT":              tokens.COMMAND,
		"PROC":               tokens.PROCEDURE,
		"REAL":               tokens.FLOAT,
		"ROWVERSION":         tokens.ROWVERSION,
		"SMALLDATETIME":      tokens.SMALLDATETIME,
		"SMALLMONEY":         tokens.SMALLMONEY,
		"SQL_VARIANT":        tokens.VARIANT,
		"SYSTEM_USER":        tokens.CURRENT_USER,
		"TOP":                tokens.TOP,
		"TIMESTAMP":          tokens.ROWVERSION,
		"TINYINT":            tokens.UTINYINT,
		"UNIQUEIDENTIFIER":   tokens.UUID,
		"UPDATE STATISTICS":  tokens.COMMAND,
		"XML":                tokens.XML,
	} {
		cfg.Keywords[keyword] = tokenType
	}
	// dialects/tsql.py:187 KEYWORDS.pop("/*+") and :188 COMMANDS = base + END - EXECUTE.
	delete(cfg.Keywords, "/*+")
	cfg.HintStart = ""
	cfg.TokensPrecedingHint = nil
	cfg.Commands = maps.Clone(cfg.Commands)
	cfg.Commands[tokens.END] = true
	delete(cfg.Commands, tokens.EXECUTE)

	d.TokenizerConfig = tokens.CompileConfig(cfg)

	// dialects/tsql.py:29-48 DATE_PART_MAPPING deltas.
	d.DatePartMapping = maps.Clone(d.DatePartMapping)
	for key, value := range map[string]string{
		"QQ":       "QUARTER",
		"M":        "MONTH",
		"Y":        "DAYOFYEAR",
		"WW":       "WEEK",
		"N":        "MINUTE",
		"SS":       "SECOND",
		"MCS":      "MICROSECOND",
		"TZOFFSET": "TIMEZONE_MINUTE",
		"TZ":       "TIMEZONE_MINUTE",
		"ISO_WEEK": "WEEKISO",
		"ISOWK":    "WEEKISO",
		"ISOWW":    "WEEKISO",
	} {
		d.DatePartMapping[key] = value
	}
	d.ValidIntervalUnits = validIntervalUnits(d.DatePartMapping)

	return d
}
