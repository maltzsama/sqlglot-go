package dialects

import (
	"maps"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// bigQueryTimeMapping ports BigQuery.TIME_MAPPING (dialects/bigquery.py:58-66).
var bigQueryTimeMapping = map[string]string{
	"%x":   "%m/%d/%y",
	"%D":   "%m/%d/%y",
	"%E6S": "%S.%f",
	"%e":   "%-d",
	"%F":   "%Y-%m-%d",
	"%T":   "%H:%M:%S",
	"%c":   "%a %b %e %H:%M:%S %Y",
}

// bigQueryTimeMappingOrder replays the TIME_MAPPING insertion order so the reverse mapping
// resolves collisions the way upstream's dict comprehension does (%m/%d/%y -> %D).
var bigQueryTimeMappingOrder = []string{"%x", "%D", "%E6S", "%e", "%F", "%T", "%c"}

// bigQueryInverseTimeMapping ports BigQuery.INVERSE_TIME_MAPPING (dialects/bigquery.py:68-72),
// the reverse of TIME_MAPPING with %H:%M:%E6S kept atomic.
var bigQueryInverseTimeMapping = func() map[string]string {
	out := map[string]string{}
	for _, src := range bigQueryTimeMappingOrder {
		dst := bigQueryTimeMapping[src]
		if dst == "%S.%f" {
			continue
		}
		out[dst] = src
	}
	out["%H:%M:%S.%f"] = "%H:%M:%E6S"
	return out
}()

// bigQueryFormatMapping ports BigQuery.FORMAT_MAPPING (dialects/bigquery.py:74-98).
var bigQueryFormatMapping = map[string]string{
	"dd": "%d", "DD": "%d", "mm": "%m", "MM": "%m", "mon": "%b", "MON": "%b",
	"month": "%B", "MONTH": "%B", "yyyy": "%Y", "YYYY": "%Y", "yy": "%y", "YY": "%y",
	"HH": "%I", "HH12": "%I", "hh24": "%H", "HH24": "%H", "mi": "%M", "MI": "%M",
	"ss": "%S", "SS": "%S", "SSSSS": "%f", "tzh": "%z", "TZH": "%z",
}

// bigQueryFormatMappingOrder replays the upstream dict insertion order, which decides which
// spelling the inverse mapping keeps when several map to the same strftime directive
// (e.g. both "month" and "MONTH" map to %B; the later "MONTH" wins).
var bigQueryFormatMappingOrder = []string{
	"dd", "DD", "mm", "MM", "mon", "MON", "month", "MONTH", "yyyy", "YYYY", "yy", "YY",
	"HH", "HH12", "hh24", "HH24", "mi", "MI", "ss", "SS", "SSSSS", "tzh", "TZH",
}

// bigQueryInverseFormatMapping is the reverse of FORMAT_MAPPING.
var bigQueryInverseFormatMapping = func() map[string]string {
	out := map[string]string{}
	for _, src := range bigQueryFormatMappingOrder {
		out[bigQueryFormatMapping[src]] = src
	}
	return out
}()

// BigQuery ports dialects/bigquery.py (class BigQuery): the tokenizer, parser flags and
// FUNCTIONS overlay. The parser grammar overrides live in parser/dialect_bigquery.go and the
// generator in generator/dialect_bigquery.go.
func BigQuery() *Dialect {
	d := Base()
	d.Name = "bigquery"

	// dialects/bigquery.py:24-46 class attributes.
	d.WeekOffset = -1
	d.UnnestColumnOnly = true
	d.SupportsUserDefinedTypes = false
	d.LogBaseFirst = false
	d.HexLowercase = true
	d.HexFunc = "TO_HEX"
	d.LogDefaultsToLn = true
	d.ForceEarlyAliasRefExpansion = true
	d.ExpandOnlyGroupAliasRef = true
	d.PreserveOriginalNames = true
	d.HexStringIsIntegerType = true
	d.ByteStringIsBytesType = true
	d.UUIDIsStringType = true
	d.AnnotateAllScopes = true
	d.ProjectionAliasesShadowSourceNames = true
	d.TablesReferenceableAsColumns = true
	d.SupportsStructStarExpansion = true
	d.ExcludesPseudocolumnsFromStar = true
	// parsers/bigquery.py:190 PREFIXED_PIVOT_COLUMNS: the aggregation alias leads each
	// inferred PIVOT output column name.
	d.PrefixedPivotColumns = true
	d.QueryResultsAreStructs = true
	d.JSONExtractScalarScalarOnly = true
	d.JSONPathSingleDotIsWildcard = true
	d.LeastGreatestIgnoresNulls = false
	d.DefaultNullType = exp.DTypeBigInt
	d.PrioritizeNonLiteralTypes = true
	d.AliasPostVersion = false

	// dialects/bigquery.py:49 INITCAP_DEFAULT_DELIMITER_CHARS.
	d.InitcapDefaultDelimiterChars = " \t\n\r\f\v\\[\\](){}/|<>!?@\"^#$&~_,.:;*%+\\-"

	// dialects/bigquery.py:52-55: case-insensitive identifiers, unnormalized function names.
	d.NormalizationStrategy = CaseInsensitive
	d.NormalizeFunctions = ""

	// dialects/bigquery.py:58-98 time and datetime format tables.
	d.TimeMapping = bigQueryTimeMapping
	d.InverseTimeMapping = bigQueryInverseTimeMapping
	d.FormatMapping = bigQueryFormatMapping
	d.InverseFormatMapping = bigQueryInverseFormatMapping

	// dialects/bigquery.py:104-110 PSEUDOCOLUMNS.
	d.Pseudocolumns = map[string]bool{
		"_PARTITIONTIME":     true,
		"_PARTITIONDATE":     true,
		"_TABLE_SUFFIX":      true,
		"_FILE_NAME":         true,
		"_DBT_MAX_PARTITION": true,
	}

	// dialects/bigquery.py:113 every set operation must state DISTINCT or ALL.
	d.SetOpExplicitDistinct = true

	// generators/bigquery.py:484-... RESERVED_KEYWORDS (85+ words) force identifier quoting.
	d.ReservedKeywords = map[string]bool{
		"all":                  true,
		"and":                  true,
		"any":                  true,
		"array":                true,
		"as":                   true,
		"asc":                  true,
		"assert_rows_modified": true,
		"at":                   true,
		"between":              true,
		"by":                   true,
		"case":                 true,
		"cast":                 true,
		"collate":              true,
		"contains":             true,
		"create":               true,
		"cross":                true,
		"cube":                 true,
		"current":              true,
		"default":              true,
		"define":               true,
		"desc":                 true,
		"distinct":             true,
		"else":                 true,
		"end":                  true,
		"enum":                 true,
		"escape":               true,
		"except":               true,
		"exclude":              true,
		"exists":               true,
		"extract":              true,
		"false":                true,
		"fetch":                true,
		"following":            true,
		"for":                  true,
		"from":                 true,
		"full":                 true,
		"group":                true,
		"grouping":             true,
		"groups":               true,
		"hash":                 true,
		"having":               true,
		"if":                   true,
		"ignore":               true,
		"in":                   true,
		"inner":                true,
		"intersect":            true,
		"interval":             true,
		"into":                 true,
		"is":                   true,
		"join":                 true,
		"lateral":              true,
		"left":                 true,
		"like":                 true,
		"limit":                true,
		"lookup":               true,
		"merge":                true,
		"natural":              true,
		"new":                  true,
		"no":                   true,
		"not":                  true,
		"null":                 true,
		"nulls":                true,
		"of":                   true,
		"on":                   true,
		"or":                   true,
		"order":                true,
		"outer":                true,
		"over":                 true,
		"partition":            true,
		"preceding":            true,
		"proto":                true,
		"qualify":              true,
		"range":                true,
		"recursive":            true,
		"respect":              true,
		"right":                true,
		"rollup":               true,
		"rows":                 true,
		"select":               true,
		"set":                  true,
		"some":                 true,
		"struct":               true,
		"tablesample":          true,
		"then":                 true,
		"to":                   true,
		"treat":                true,
		"true":                 true,
		"unbounded":            true,
		"union":                true,
		"unnest":               true,
		"using":                true,
		"when":                 true,
		"where":                true,
		"window":               true,
		"with":                 true,
		"within":               true,
	}
	d.UnpivotAliasesAreIdentifiers = false
	d.JSONKeyValuePairSep = ","
	d.QualifiedTablesCaseSensitive = true
	d.WithPropertiesPrefix = "OPTIONS"
	d.RenameTableWithDB = false

	// generators/bigquery.py:259-276 generator flags.
	d.IgnoreNullsInFunc = true
	d.UnnestWithOrdinality = false
	d.InUnnestOpBare = true
	d.SupportsTableAliasColumns = false
	d.QualifyBeforeWindow = true
	d.WildcardTableParts = true
	d.IdentifierStart, d.IdentifierEnd = "`", "`"
	d.JoinsHaveEqualPrecedence = true
	d.SupportsImplicitUnnest = true

	// dialects/bigquery.py:174-178 HEX_STRINGS 0x and BYTE_STRINGS b'/b" families: the literal
	// prefixes and delimiters.
	d.HexStart, d.HexEnd = "0x", ""
	d.ByteStart, d.ByteEnd = "b'", "'"

	// generators/bigquery.py:277 DECLARE_DEFAULT_ASSIGNMENT = "DEFAULT".
	d.DeclareDefaultAssignment = "DEFAULT"

	// dialects/bigquery.py:168-205 class Tokenizer.
	cfg := d.TokenizerConfig
	cfg.Quotes = map[string]string{
		"'": "'", "\"": "\"", "\"\"\"": "\"\"\"", "'''": "'''",
	}
	cfg.Comments = map[string]string{"--": "", "#": "", "/*": "*/"}
	cfg.Identifiers = map[rune]string{'`': "`"}
	cfg.StringEscapes = map[rune]bool{'\\': true}
	cfg.ByteStringEscapes = map[rune]bool{'\\': true}
	// HEX_STRINGS [("0x", ""), ("0X", "")]: the 0x prefix is scanned by scanHex.
	cfg.HasHexStrings = true
	cfg.NestedComments = false
	cfg.FormatStrings = maps.Clone(cfg.FormatStrings)
	if cfg.FormatStrings == nil {
		cfg.FormatStrings = map[string]tokens.FormatString{}
	}
	for _, quote := range []string{"'", "\"", "\"\"\"", "'''"} {
		for _, prefix := range []string{"b", "B"} {
			cfg.FormatStrings[prefix+quote] = tokens.FormatString{End: quote, TokenType: tokens.BYTE_STRING}
		}
		for _, prefix := range []string{"r", "R"} {
			cfg.FormatStrings[prefix+quote] = tokens.FormatString{End: quote, TokenType: tokens.RAW_STRING}
		}
	}
	cfg.Keywords = maps.Clone(cfg.Keywords)
	if cfg.Keywords == nil {
		cfg.Keywords = map[string]tokens.TokenType{}
	}
	for keyword, tokenType := range map[string]tokens.TokenType{
		"ANY TYPE":          tokens.VARIANT,
		"BEGIN":             tokens.COMMAND,
		"BEGIN TRANSACTION": tokens.BEGIN,
		"BYTEINT":           tokens.INT,
		"BYTES":             tokens.BINARY,
		"CURRENT_DATETIME":  tokens.CURRENT_DATETIME,
		"DATETIME":          tokens.TIMESTAMP,
		"DECLARE":           tokens.DECLARE,
		"ELSEIF":            tokens.COMMAND,
		"EXCEPTION":         tokens.COMMAND,
		"EXPORT":            tokens.EXPORT,
		"FLOAT64":           tokens.DOUBLE,
		"LOOP":              tokens.COMMAND,
		"MODEL":             tokens.MODEL,
		"RECORD":            tokens.STRUCT,
		"REPEAT":            tokens.COMMAND,
		"TIMESTAMP":         tokens.TIMESTAMPTZ,
		"WHILE":             tokens.COMMAND,
	} {
		cfg.Keywords[keyword] = tokenType
	}
	// dialects/bigquery.py:203-205 KEYWORDS.pop("DIV"), pop("VALUES"), pop("/*+").
	delete(cfg.Keywords, "DIV")
	delete(cfg.Keywords, "VALUES")
	delete(cfg.Keywords, "/*+")
	cfg.HintStart = ""
	d.TokenizerConfig = tokens.CompileConfig(cfg)

	// parsers/bigquery.py:224-323 FUNCTIONS = {**Parser.FUNCTIONS without SEARCH, ...}.
	d.RemovedFunctions = map[string]bool{"SEARCH": true}
	d.Functions = bigQueryFunctions(d)

	return d
}

// bigQueryFunctions builds the BigQueryParser.FUNCTIONS overlay.
func bigQueryFunctions(d *Dialect) map[string]func([]exp.Expression) exp.Expression {
	out := map[string]func([]exp.Expression) exp.Expression{}

	out["APPROX_TOP_COUNT"] = exp.FromArgListFunc(exp.KindApproxTopK)
	out["BIT_AND"] = exp.FromArgListFunc(exp.KindBitwiseAndAgg)
	out["BIT_OR"] = exp.FromArgListFunc(exp.KindBitwiseOrAgg)
	out["BIT_XOR"] = exp.FromArgListFunc(exp.KindBitwiseXorAgg)
	out["BIT_COUNT"] = exp.FromArgListFunc(exp.KindBitwiseCount)
	out["BOOL"] = exp.FromArgListFunc(exp.KindJSONBool)
	out["CONTAINS_SUBSTR"] = bigQueryBuildContainsSubstring
	out["DATE"] = bigQueryBuildDate
	out["DATE_ADD"] = bigQueryBuildDateDeltaWithInterval(exp.KindDateAdd)
	out["DATE_DIFF"] = bigQueryBuildDateDiff(exp.KindDateDiff)
	out["DATE_SUB"] = bigQueryBuildDateDeltaWithInterval(exp.KindDateSub)
	out["DATE_TRUNC"] = bigQueryBuildDateTrunc
	out["DATETIME"] = bigQueryBuildDatetime
	out["DATETIME_ADD"] = bigQueryBuildDateDeltaWithInterval(exp.KindDatetimeAdd)
	out["DATETIME_DIFF"] = bigQueryBuildDateDiff(exp.KindDatetimeDiff)
	out["DATETIME_SUB"] = bigQueryBuildDateDeltaWithInterval(exp.KindDatetimeSub)
	out["DATETIME_TRUNC"] = bigQueryBuildDatetimeTrunc
	out["DIV"] = bigQueryBinaryFromFunction(exp.KindIntDiv)
	out["EDIT_DISTANCE"] = bigQueryBuildLevenshtein
	out["EMBED"] = bigQueryExpressionsBuilder(exp.KindAIEmbed)
	out["FORMAT_DATE"] = bigQueryBuildFormatTime(d, exp.KindTsOrDsToDate)
	out["GENERATE"] = bigQueryExpressionsBuilder(exp.KindAIGenerate)
	out["GENERATE_ARRAY"] = exp.FromArgListFunc(exp.KindGenerateSeries)
	out["JSON_EXTRACT_SCALAR"] = bigQueryBuildExtractJSON(d, exp.KindJSONExtractScalar)
	out["JSON_EXTRACT_ARRAY"] = bigQueryBuildExtractJSON(d, exp.KindJSONExtractArray)
	out["JSON_EXTRACT_STRING_ARRAY"] = bigQueryBuildExtractJSON(d, exp.KindJSONValueArray)
	out["JSON_KEYS"] = exp.FromArgListFunc(exp.KindJSONKeysAtDepth)
	out["JSON_QUERY"] = bigQueryBuildExtractJSON(d, exp.KindJSONExtract)
	out["JSON_QUERY_ARRAY"] = bigQueryBuildExtractJSON(d, exp.KindJSONExtractArray)
	out["JSON_STRIP_NULLS"] = bigQueryBuildJSONStripNulls
	out["JSON_VALUE"] = bigQueryBuildExtractJSON(d, exp.KindJSONExtractScalar)
	out["JSON_VALUE_ARRAY"] = bigQueryBuildExtractJSON(d, exp.KindJSONValueArray)
	out["LAST_DAY"] = bigQueryBuildLastDay
	out["LOG"] = bigQueryBuildLogarithm(d)
	out["LENGTH"] = func(args []exp.Expression) exp.Expression {
		return exp.Length(exp.Args{"this": seqGet(args, 0), "binary": true})
	}
	out["MD5"] = exp.FromArgListFunc(exp.KindMD5Digest)
	out["SHA1"] = exp.FromArgListFunc(exp.KindSHA1Digest)
	out["NORMALIZE_AND_CASEFOLD"] = func(args []exp.Expression) exp.Expression {
		return exp.Normalize(exp.Args{"this": seqGet(args, 0), "form": seqGet(args, 1), "is_casefold": true})
	}
	out["OCTET_LENGTH"] = exp.FromArgListFunc(exp.KindByteLength)
	out["TO_HEX"] = bigQueryBuildToHex
	out["PARSE_DATE"] = bigQueryBuildParseDate(d)
	out["PARSE_TIME"] = func(args []exp.Expression) exp.Expression {
		return postgresBuildFormattedTimeClass(d, exp.KindParseTime)([]exp.Expression{seqGet(args, 1), seqGet(args, 0)})
	}
	out["PARSE_TIMESTAMP"] = bigQueryBuildParseTimestamp(d)
	out["PARSE_DATETIME"] = bigQueryBuildParseDatetime(d)
	out["REGEXP_CONTAINS"] = exp.FromArgListFunc(exp.KindRegexpLike)
	out["REGEXP_EXTRACT"] = bigQueryBuildRegexpExtract(exp.KindRegexpExtract, nil)
	out["REGEXP_SUBSTR"] = bigQueryBuildRegexpExtract(exp.KindRegexpExtract, nil)
	out["REGEXP_EXTRACT_ALL"] = bigQueryBuildRegexpExtract(exp.KindRegexpExtractAll, exp.LiteralNumber("0"))
	out["SHA256"] = func(args []exp.Expression) exp.Expression {
		return exp.SHA2Digest(exp.Args{"this": seqGet(args, 0), "length": exp.LiteralNumber("256")})
	}
	out["SHA512"] = func(args []exp.Expression) exp.Expression {
		return exp.SHA2Digest(exp.Args{"this": seqGet(args, 0), "length": exp.LiteralNumber("512")})
	}
	out["SIMILARITY"] = bigQueryExpressionsBuilder(exp.KindAISimilarity)
	out["SPLIT"] = func(args []exp.Expression) exp.Expression {
		expression := seqGet(args, 1)
		if expression == nil {
			expression = exp.LiteralString(",")
		}
		return exp.Split(exp.Args{"this": seqGet(args, 0), "expression": expression})
	}
	out["STRPOS"] = exp.FromArgListFunc(exp.KindStrPosition)
	out["TIME"] = bigQueryBuildTime
	out["TIME_ADD"] = bigQueryBuildDateDeltaWithInterval(exp.KindTimeAdd)
	out["TIME_SUB"] = bigQueryBuildDateDeltaWithInterval(exp.KindTimeSub)
	out["TIMESTAMP"] = bigQueryBuildTimestamp
	out["TIMESTAMP_ADD"] = bigQueryBuildDateDeltaWithInterval(exp.KindTimestampAdd)
	out["TIMESTAMP_SUB"] = bigQueryBuildDateDeltaWithInterval(exp.KindTimestampSub)
	out["TIMESTAMP_MICROS"] = func(args []exp.Expression) exp.Expression {
		return exp.UnixToTime(exp.Args{"this": seqGet(args, 0), "scale": exp.LiteralNumber("6")})
	}
	out["TIMESTAMP_MILLIS"] = func(args []exp.Expression) exp.Expression {
		return exp.UnixToTime(exp.Args{"this": seqGet(args, 0), "scale": exp.LiteralNumber("3")})
	}
	out["TIMESTAMP_SECONDS"] = func(args []exp.Expression) exp.Expression {
		return exp.UnixToTime(exp.Args{"this": seqGet(args, 0)})
	}
	out["TIMESTAMP_TRUNC"] = bigQueryBuildTimestampTrunc
	out["TO_JSON"] = func(args []exp.Expression) exp.Expression {
		return exp.JSONFormat(exp.Args{"this": seqGet(args, 0), "options": seqGet(args, 1), "to_json": true})
	}
	out["TO_JSON_STRING"] = exp.FromArgListFunc(exp.KindJSONFormat)
	out["FORMAT_DATETIME"] = bigQueryBuildFormatTime(d, exp.KindTsOrDsToDatetime)
	out["FORMAT_TIMESTAMP"] = bigQueryBuildFormatTime(d, exp.KindTsOrDsToTimestamp)
	out["FORMAT_TIME"] = bigQueryBuildFormatTime(d, exp.KindTsOrDsToTime)
	out["FROM_HEX"] = exp.FromArgListFunc(exp.KindUnhex)
	out["WEEK"] = func(args []exp.Expression) exp.Expression {
		return exp.WeekStart(exp.Args{"this": exp.Var(exp.Args{"this": seqGet(args, 0).Name()})})
	}

	return out
}

// bigQueryExpressionsBuilder builds a node whose only arg is the full `expressions` list
// (expressions/functions.py AI/ML nodes, e.g. AISimilarity: {"expressions": True}).
func bigQueryExpressionsBuilder(kind exp.Kind) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		return exp.New(kind, exp.Args{"expressions": args})
	}
}
