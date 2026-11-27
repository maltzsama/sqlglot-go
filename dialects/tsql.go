package dialects

import (
	"maps"

	exp "github.com/maltzsama/sqlglot-go/expressions"

	"github.com/maltzsama/sqlglot-go/tokens"
)

// TSQL ports the T-SQL (Microsoft SQL Server) dialect class from
// .reference/sqlglot-v30.17.0/sqlglot/dialects/tsql.py:14-194.
func TSQL() *Dialect {
	d := Base()
	// dialects/tsql.py CONCAT_WS_COALESCE = True.
	d.ConcatWsCoalesce = true
	d.Name = "tsql"
	// generators/tsql.py:152-163 EXPRESSIONS_WITHOUT_NESTED_CTES.
	d.MoveCTEsToTopLevel = map[exp.Kind]bool{
		exp.KindCreate: true, exp.KindDelete: true, exp.KindInsert: true,
		exp.KindIntersect: true, exp.KindExcept: true, exp.KindMerge: true,
		exp.KindSelect: true, exp.KindSubquery: true, exp.KindUnion: true, exp.KindUpdate: true,
	}
	// generators/mysql.py:124 / generators/tsql.py:138 NULL_ORDERING_SUPPORTED = None.
	d.NullOrderingSupported = nil
	// generators/tsql.py TRANSFORMS: eliminate_distinct_on.
	d.EliminateDistinctOn = true
	// generators/tsql.py TRANSFORMS: eliminate_semi_and_anti_joins.
	d.EliminateSemiAntiJoins = true
	// generators/tsql.py NVL2_SUPPORTED = False.
	d.Nvl2Supported = false
	// generators/tsql.py:146 PARSE_JSON_NAME = None (bare value).
	d.ParseJSONName = stringPtr("")
	// parsers/tsql.py:326 UNPIVOT_VALUE_COLUMNS_FIRST.
	d.UnpivotValueColumnsFirst = true
	// dialects/tsql.py:16 WEEK_OFFSET = -1.
	d.WeekOffset = -1
	// dialects/tsql.py:22 NORMALIZATION_STRATEGY = CASE_INSENSITIVE.
	d.NormalizationStrategy = CaseInsensitive
	// dialects/tsql.py:19-21 TYPED_DIVISION / CONCAT_COALESCE / CONCAT_WS_COALESCE.
	d.TypedDivision = true
	d.ConcatCoalesce = true
	// dialects/tsql.py:18 LOG_BASE_FIRST = False.
	d.LogBaseFirst = false
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
	// generators/tsql.py:131 RETURNING_END = False (OUTPUT after the target).
	d.ReturningEnd = false
	// parsers/tsql.py:460 SET_OP_MODIFIERS = {"offset"}.
	d.SetOpModifiers = map[string]bool{"offset": true}
	d.JSONPathBracketedKeySupported = false
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
	// divergence (DEVIATIONS §1.22): a `GO [count]` line ends the batch, as sqlcmd/SSMS do;
	// upstream lexes GO as a Command and folds the next batch into it.
	cfg.BatchSeparator = tokens.BatchSeparatorGo

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

// TSQLConvertFormatMapping ports TSQL.CONVERT_FORMAT_MAPPING (dialects/tsql.py:90-130).
var TSQLConvertFormatMapping = map[string]string{
	"0": "%b %d %Y %-I:%M%p", "1": "%m/%d/%y", "2": "%y.%m.%d", "3": "%d/%m/%y",
	"4": "%d.%m.%y", "5": "%d-%m-%y", "6": "%d %b %y", "7": "%b %d, %y",
	"8": "%H:%M:%S", "9": "%b %d %Y %-I:%M:%S:%f%p", "10": "%m-%d-%y",
	"11": "%y/%m/%d", "12": "%y%m%d", "13": "%d %b %Y %H:%M:%S:%f", "14": "%H:%M:%S:%f",
	"20": "%Y-%m-%d %H:%M:%S", "21": "%Y-%m-%d %H:%M:%S.%f",
	"22": "%m/%d/%y %-I:%M:%S %p", "23": "%Y-%m-%d", "24": "%H:%M:%S",
	"25": "%Y-%m-%d %H:%M:%S.%f", "100": "%b %d %Y %-I:%M%p", "101": "%m/%d/%Y",
	"102": "%Y.%m.%d", "103": "%d/%m/%Y", "104": "%d.%m.%Y", "105": "%d-%m-%Y",
	"106": "%d %b %Y", "107": "%b %d, %Y", "108": "%H:%M:%S",
	"109": "%b %d %Y %-I:%M:%S:%f%p", "110": "%m-%d-%Y", "111": "%Y/%m/%d",
	"112": "%Y%m%d", "113": "%d %b %Y %H:%M:%S:%f", "114": "%H:%M:%S:%f",
	"120": "%Y-%m-%d %H:%M:%S", "121": "%Y-%m-%d %H:%M:%S.%f", "126": "%Y-%m-%dT%H:%M:%S.%f",
}
