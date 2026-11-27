package dialects

import (
	"maps"
	"strconv"
	"strings"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

func init() {
	registerDialectTypeAndParser()
}

// registerDialectTypeAndParser is split out so init order stays explicit.
func registerDialectTypeAndParser() {}

// snowflakeTimeMapping ports Snowflake.TIME_MAPPING (dialects/snowflake.py:78-121).
var snowflakeTimeMapping = map[string]string{
	"YYYY": "%Y", "yyyy": "%Y", "YY": "%y", "yy": "%y",
	"MMMM": "%B", "mmmm": "%B", "MON": "%b", "mon": "%b", "MM": "%m", "mm": "%m",
	"DD": "%d", "dd": "%-d", "DY": "%a", "dy": "%w",
	"HH24": "%H", "hh24": "%H", "HH12": "%I", "hh12": "%I",
	"MI": "%M", "mi": "%M", "SS": "%S", "ss": "%S",
	"FF": "%f_nine", "ff": "%f_nine",
	"FF0": "%f_zero", "ff0": "%f_zero", "FF1": "%f_one", "ff1": "%f_one",
	"FF2": "%f_two", "ff2": "%f_two", "FF3": "%f_three", "ff3": "%f_three",
	"FF4": "%f_four", "ff4": "%f_four", "FF5": "%f_five", "ff5": "%f_five",
	"FF6": "%f", "ff6": "%f", "FF7": "%f_seven", "ff7": "%f_seven",
	"FF8": "%f_eight", "ff8": "%f_eight", "FF9": "%f_nine", "ff9": "%f_nine",
	"TZHTZM": "%z", "tzhtzm": "%z", "TZH:TZM": "%:z", "tzh:tzm": "%:z",
	"TZH": "%-z", "tzh": "%-z", "\"T\"": "T",
	"AM": "%p", "am": "%p", "PM": "%p", "pm": "%p",
}

// Snowflake ports dialects/snowflake.py (class Snowflake): the tokenizer, parser flags and
// FUNCTIONS overlay. The parser grammar overrides live in parser/dialect_snowflake.go and the
// generator in generator/dialect_snowflake.go.
func Snowflake() *Dialect {
	d := Base()
	d.Name = "snowflake"
	// dialects/snowflake.py:27 ARRAY_AGG_INCLUDES_NULLS = None.
	noNulls := false
	d.ArrayAggIncludesNulls = &noNulls
	// generators/snowflake.py:393 ARRAY_CONCAT_IS_VAR_LEN = False: binary ARRAY_CAT nesting.
	d.ArrayConcatIsVarLen = false
	// generators/snowflake.py TRANSFORMS: eliminate_distinct_on.
	d.EliminateDistinctOn = true
	// generators/snowflake.py TRANSFORMS: eliminate_semi_and_anti_joins.
	d.EliminateSemiAntiJoins = true
	// dialects/snowflake.py:130-132 JSONPathTokenizer: SINGLE_TOKENS drops '$'.
	d.JSONPathNoDollar = true
	// generators/redshift.py:33 / snowflake.py:394 / spark.py:70: native CONVERT_TIMEZONE.
	d.SupportsConvertTimezone = true

	// dialects/snowflake.py:20-40 class attributes.
	d.NormalizationStrategy = Uppercase
	d.UnescapedSequences = map[string]string{"\\a": "a", "\\v": "v"}
	d.NullOrdering = "nulls_are_large"
	d.TimeFormat = "'YYYY-MM-DD HH24:MI:SS'"
	d.SupportsUserDefinedTypes = false
	d.PreferCTEAliasColumn = true
	d.TablesampleSizeIsPercent = true
	d.CopyParamsAreCsv = false
	// generators/snowflake.py:390: COPY params use `NAME = value`.
	d.CopyParamsEqRequired = true
	d.ArrayFuncsPropagatesNulls = true
	d.AlterTableAddRequiredForEachColumn = false
	tryCastRequiresString := true
	d.TryCastRequiresString = &tryCastRequiresString
	d.SupportsAliasRefsInJoinConditions = true
	d.LeastGreatestIgnoresNulls = false
	// generators/snowflake.py:402 IS_BOOL_ALLOWED = False.
	d.IsBoolAllowed = false
	d.UUIDIsStringType = true
	d.StarIlikeBackslashEscape = true
	d.JSONExtractRequiresJSONExpression = true
	d.DefaultSamplingMethod = "BERNOULLI"
	d.IdentifyPivotStrings = true
	d.ParameterToken = "$"
	d.StructDelimiter = [2]string{"(", ")"}
	d.JSONKeyValuePairSep = ","
	d.DirectedJoins = true
	d.CopyParamsAreWrapped = false
	d.MapIntervalUnits = true
	d.NormalizeExtractDateParts = true
	d.RlikeFullMatch = true
	d.ExtraIdVarTokens = map[tokens.TokenType]bool{
		tokens.EXCEPT: true, tokens.INTEGRATION: true, tokens.MATCH_CONDITION: true,
		tokens.PACKAGE: true, tokens.POLICY: true, tokens.POOL: true, tokens.ROLE: true,
		tokens.RULE: true, tokens.VOLUME: true,
	}
	d.InitcapDefaultDelimiterChars = " \\t\\n\\r\\f\\v!?@\\\"^#$&~_,.:;+\\\\-*%/|\\\\[\\\\](){}<>"

	// Snowflake's ordered TIME_MAPPING inverse (dialects/dialect.py:261-271) prefers
	// lowercase month/year tokens and uppercase day tokens, matching format_time's
	// canonicalized output (e.g. `%m-%d-%Y` -> `mm-DD-yyyy`).
	d.InverseTimeMapping = withStrictTimeInverse(map[string]string{
		"T": "T", "%Y": "yyyy", "%y": "yy", "%B": "mmmm", "%b": "mon",
		"%m": "mm", "%d": "DD", "%-d": "dd", "%a": "DY", "%w": "dy",
		"%H": "hh24", "%I": "hh12", "%M": "mi", "%S": "ss", "%f": "ff6",
		"%f_zero": "ff0", "%f_one": "ff1", "%f_two": "ff2", "%f_three": "ff3",
		"%f_four": "ff4", "%f_five": "ff5", "%f_seven": "ff7", "%f_eight": "ff8",
		"%f_nine": "ff9", "%z": "tzhtzm", "%:z": "tzh:tzm", "%-z": "tzh", "%p": "pm",
	})
	d.TimeMapping = snowflakeTimeMapping
	d.DatePartMapping = maps.Clone(d.DatePartMapping)
	if d.DatePartMapping == nil {
		d.DatePartMapping = map[string]string{}
	}
	d.DatePartMapping["ISOWEEK"] = "WEEKISO"
	d.DatePartMapping["EPOCH_SECOND"] = "EPOCH_SECOND"
	d.DatePartMapping["EPOCH_SECONDS"] = "EPOCH_SECOND"
	d.Pseudocolumns = map[string]bool{"LEVEL": true}

	// dialects/snowflake.py:151-190 class Tokenizer.
	cfg := d.TokenizerConfig
	cfg.StringEscapes = map[rune]bool{'\\': true, '\'': true}
	// Snowflake HEX_STRINGS is non-empty, so the number scanner accepts bare 0x literals.
	cfg.HasHexStrings = true
	cfg.FormatStrings = maps.Clone(cfg.FormatStrings)
	if cfg.FormatStrings == nil {
		cfg.FormatStrings = map[string]tokens.FormatString{}
	}
	cfg.FormatStrings["x'"] = tokens.FormatString{End: "'", TokenType: tokens.HEX_STRING}
	cfg.FormatStrings["X'"] = tokens.FormatString{End: "'", TokenType: tokens.HEX_STRING}
	cfg.FormatStrings["$$"] = tokens.FormatString{End: "$$", TokenType: tokens.RAW_STRING}
	cfg.Comments = map[string]string{"--": "", "//": "", "/*": "*/"}
	cfg.NestedComments = false
	cfg.SingleTokens = maps.Clone(cfg.SingleTokens)
	if cfg.SingleTokens == nil {
		cfg.SingleTokens = map[rune]tokens.TokenType{}
	}
	cfg.SingleTokens['$'] = tokens.PARAMETER
	cfg.SingleTokens['!'] = tokens.EXCLAMATION
	cfg.VarSingleTokens = maps.Clone(cfg.VarSingleTokens)
	if cfg.VarSingleTokens == nil {
		cfg.VarSingleTokens = map[rune]bool{}
	}
	cfg.VarSingleTokens['$'] = true
	cfg.Keywords = maps.Clone(cfg.Keywords)
	if cfg.Keywords == nil {
		cfg.Keywords = map[string]tokens.TokenType{}
	}
	for keyword, tokenType := range map[string]tokens.TokenType{
		"BYTEINT":             tokens.INT,
		"FILE://":             tokens.URI_START,
		"FILE FORMAT":         tokens.FILE_FORMAT,
		"GET":                 tokens.GET,
		"INTEGRATION":         tokens.INTEGRATION,
		"MATCH_CONDITION":     tokens.MATCH_CONDITION,
		"MATCH_RECOGNIZE":     tokens.MATCH_RECOGNIZE,
		"MINUS":               tokens.EXCEPT,
		"NCHAR VARYING":       tokens.VARCHAR,
		"PACKAGE":             tokens.PACKAGE,
		"POLICY":              tokens.POLICY,
		"POOL":                tokens.POOL,
		"PUT":                 tokens.PUT,
		"UNDROP":              tokens.UNDROP,
		"REMOVE":              tokens.COMMAND,
		"RM":                  tokens.COMMAND,
		"ROLE":                tokens.ROLE,
		"RULE":                tokens.RULE,
		"SAMPLE":              tokens.TABLE_SAMPLE,
		"SEMANTIC VIEW":       tokens.SEMANTIC_VIEW,
		"SQL_DOUBLE":          tokens.DOUBLE,
		"SQL_VARCHAR":         tokens.VARCHAR,
		"STAGE":               tokens.STAGE,
		"STORAGE INTEGRATION": tokens.STORAGE_INTEGRATION,
		"STREAMLIT":           tokens.STREAMLIT,
		"TAG":                 tokens.TAG,
		"TIMESTAMP_TZ":        tokens.TIMESTAMPTZ,
		"TOP":                 tokens.TOP,
		"VOLUME":              tokens.VOLUME,
		"WAREHOUSE":           tokens.WAREHOUSE,
		"FLOAT":               tokens.DOUBLE,
	} {
		cfg.Keywords[keyword] = tokenType
	}
	delete(cfg.Keywords, "/*+")
	delete(cfg.Commands, tokens.SHOW)
	d.TokenizerConfig = tokens.CompileConfig(cfg)

	// parsers/snowflake.py flags that the shared parser reads from the Dialect.
	d.ColonIsVariantExtract = true
	d.Pseudocolumns["LEVEL"] = true
	d.InsertOverwrite = " OVERWRITE INTO"
	d.HexStart, d.HexEnd = "x'", "'"
	d.StringEscape = "\\"
	d.SingleStringInterval = true

	// parsers/snowflake.py:735-744 FUNCTION_PARSERS / FUNCTIONS removals.
	d.RemovedFunctions = map[string]bool{"PREDICT": true}
	d.Functions = snowflakeFunctions(d)

	return d
}

// snowflakeDateUnit ports the unit arg of snowflake's date-delta builders
// (parsers/snowflake.py:506-623): map_date_part first — with the default dialect, so the
// base DATE_PART_MAPPING — then the TimeUnit constructor's normalization
// (expressions/core.py:2055-2066), which turns a single-part Column/Literal/Var unit into
// an uppercase, unabbreviated Var. exp.TimeUnitVar ports that constructor.
func snowflakeDateUnit(unit exp.Expression) exp.Expression {
	return exp.TimeUnitVar(mapDatePart(unit))
}

// snowflakeFunctions builds the SnowflakeParser.FUNCTIONS overlay
// (parsers/snowflake.py:396-733) for the identity-relevant entries.
func snowflakeFunctions(d *Dialect) map[string]func([]exp.Expression) exp.Expression {
	out := map[string]func([]exp.Expression) exp.Expression{}
	// Snowflake sets LEAST_GREATEST_IGNORES_NULLS=False, so plain GREATEST/LEAST carry
	// ignore_nulls=false (the base builder hardcodes true).
	buildGreatestLeast := func(kind exp.Kind) func([]exp.Expression) exp.Expression {
		return func(args []exp.Expression) exp.Expression {
			m := exp.Args{"ignore_nulls": d.LeastGreatestIgnoresNulls}
			if len(args) > 0 {
				m["this"] = args[0]
			}
			if len(args) > 1 {
				m["expressions"] = args[1:]
			}
			return exp.New(kind, m)
		}
	}
	out["GREATEST"] = buildGreatestLeast(exp.KindGreatest)
	out["LEAST"] = buildGreatestLeast(exp.KindLeast)

	// Snowflake alias builders.
	safeFn := func(kind exp.Kind) func([]exp.Expression) exp.Expression {
		return func(args []exp.Expression) exp.Expression {
			e := exp.FromArgList(kind, args)
			e.Set("safe", true)
			return e
		}
	}
	out["REGEXP_SUBSTR"] = exp.FromArgListFunc(exp.KindRegexpExtract)
	out["COLLATE"] = exp.FromArgListFunc(exp.KindCollate)
	out["EDITDISTANCE"] = func(args []exp.Expression) exp.Expression {
		return exp.New(exp.KindLevenshtein, exp.Args{
			"this": seqGet(args, 0), "expression": seqGet(args, 1), "max_dist": seqGet(args, 2),
		})
	}
	out["DECODE"] = func(args []exp.Expression) exp.Expression {
		return exp.New(exp.KindDecodeCase, exp.Args{"expressions": args})
	}
	out["TRY_DECRYPT"] = safeFn(exp.KindDecrypt)
	out["TRY_DECRYPT_RAW"] = safeFn(exp.KindDecryptRaw)
	out["TRY_TO_BOOLEAN"] = safeFn(exp.KindToBoolean)
	out["TO_FILE"] = exp.FromArgListFunc(exp.KindToFile)
	out["TRY_TO_FILE"] = safeFn(exp.KindToFile)
	for name, kind := range map[string]exp.Kind{
		"ARRAY_APPEND":  exp.KindArrayAppend,
		"ARRAY_PREPEND": exp.KindArrayPrepend,
		"ARRAY_REMOVE":  exp.KindArrayRemove,
		"ARRAY_CAT":     exp.KindArrayConcat,
	} {
		kind := kind
		out[name] = func(args []exp.Expression) exp.Expression {
			e := exp.FromArgList(kind, args)
			e.Set("null_propagation", d.ArrayFuncsPropagatesNulls)
			return e
		}
	}
	out["SEARCH"] = func(args []exp.Expression) exp.Expression {
		values := exp.Args{"this": seqGet(args, 0), "expression": seqGet(args, 1)}
		for _, arg := range args[min(2, len(args)):] {
			if arg != nil && arg.Kind() == exp.KindKwarg && arg.This() != nil {
				values[strings.ToLower(arg.This().Name())] = arg
			}
		}
		return exp.New(exp.KindSearch, values)
	}
	out["GENERATOR"] = func(args []exp.Expression) exp.Expression {
		values := exp.Args{}
		positional := []string{"rowcount", "timelimit"}
		for i, arg := range args {
			if arg == nil {
				continue
			}
			if arg.Kind() == exp.KindKwarg {
				switch strings.ToUpper(arg.This().Name()) {
				case "ROWCOUNT":
					values["rowcount"] = arg.Expr()
				case "TIMELIMIT":
					values["timelimit"] = arg.Expr()
				}
				continue
			}
			if i < len(positional) {
				values[positional[i]] = arg
			}
		}
		return exp.New(exp.KindGenerator, values)
	}
	out["TABLE"] = func(args []exp.Expression) exp.Expression {
		return exp.New(exp.KindTableFromRows, exp.Args{"this": seqGet(args, 0)})
	}
	out["IDENTIFIER"] = func(args []exp.Expression) exp.Expression {
		return exp.New(exp.KindDynamicIdentifier, exp.Args{"this": seqGet(args, 0)})
	}
	out["ROUND"] = func(args []exp.Expression) exp.Expression {
		values := exp.Args{"casts_non_integer_decimals": true}
		keys := []string{"this", "decimals", "truncate"}
		pos := 0
		for _, arg := range args {
			if arg != nil && arg.Kind() == exp.KindKwarg {
				switch strings.ToUpper(arg.This().Name()) {
				case "EXPR":
					values["this"] = arg.Expr()
				case "SCALE":
					values["decimals"] = arg.Expr()
				case "ROUNDING_MODE":
					values["truncate"] = arg.Expr()
				}
			} else if pos < len(keys) {
				values[keys[pos]] = arg
				pos++
			}
		}
		return exp.Round(values)
	}

	// parsers/snowflake.py:502 `_build_datetime("DATE", DType.DATE)` handles
	// explicit format strings as TsOrDsToDate and converts the format via TIME_MAPPING.
	dateWithFormat := postgresBuildFormattedTimeClass(d, exp.KindTsOrDsToDate)
	dateBuilder := func(args []exp.Expression) exp.Expression {
		value := seqGet(args, 0)
		if len(args) == 1 && value != nil && value.IsString() {
			if _, err := strconv.ParseInt(value.Name(), 10, 64); err == nil {
				return exp.Anonymous(exp.Args{"this": "DATE", "expressions": args})
			}
			return exp.Cast(exp.Args{
				"this": value,
				"to":   exp.DataType(exp.Args{"this": exp.DTypeDate}),
			})
		}
		if len(args) > 1 && args[1] != nil &&
			(args[1].Kind() != exp.KindLiteral || args[1].IsString()) {
			return dateWithFormat(args)
		}
		return exp.FromArgList(exp.KindDate, args)
	}
	out["DATE"] = dateBuilder
	out["TO_DATE"] = func(args []exp.Expression) exp.Expression {
		value := seqGet(args, 0)
		if len(args) == 1 && value != nil && value.IsString() {
			if _, err := strconv.ParseInt(value.Name(), 10, 64); err == nil {
				return exp.Anonymous(exp.Args{"this": "TO_DATE", "expressions": args})
			}
		}
		return dateBuilder(args)
	}
	timeToStr := postgresBuildFormattedTimeClass(d, exp.KindTimeToStr)
	toChar := func(args []exp.Expression) exp.Expression {
		if len(args) == 2 && snowflakeStaticTemporal(seqGet(args, 0)) {
			return timeToStr(args)
		}
		return exp.FromArgList(exp.KindToChar, args)
	}
	out["TO_CHAR"] = toChar
	out["TO_VARCHAR"] = toChar
	out["GETDATE"] = func([]exp.Expression) exp.Expression {
		return exp.CurrentTimestamp(exp.Args{})
	}
	out["SYSTIMESTAMP"] = exp.FromArgListFunc(exp.KindCurrentTimestamp)
	out["ZEROIFNULL"] = func(args []exp.Expression) exp.Expression {
		value := seqGet(args, 0)
		return exp.If(exp.Args{
			"this":  exp.Is(exp.Args{"this": value, "expression": exp.Null()}),
			"true":  exp.LiteralNumber("0"),
			"false": value,
		})
	}
	toNumber := func(args []exp.Expression, safe bool) exp.Expression {
		second := seqGet(args, 1)
		number := exp.ToNumber(exp.Args{
			"this": seqGet(args, 0), "safe": safe,
		})
		if second != nil && second.IsNumber() {
			number.Set("precision", second)
			number.Set("scale", seqGet(args, 2))
			if number.Arg("scale") == nil {
				number.Set("scale", exp.LiteralNumber("0"))
			}
		} else {
			number.Set("format", second)
			number.Set("precision", seqGet(args, 2))
			if number.Arg("precision") == nil {
				number.Set("precision", exp.LiteralNumber("38"))
			}
			number.Set("scale", seqGet(args, 3))
			if number.Arg("scale") == nil {
				number.Set("scale", exp.LiteralNumber("0"))
			}
		}
		return number
	}
	for _, name := range []string{"TO_DECIMAL", "TO_NUMBER", "TO_NUMERIC"} {
		out[name] = func(args []exp.Expression) exp.Expression { return toNumber(args, false) }
	}
	for _, name := range []string{"TRY_TO_DECIMAL", "TRY_TO_NUMBER", "TRY_TO_NUMERIC"} {
		out[name] = func(args []exp.Expression) exp.Expression { return toNumber(args, true) }
	}
	out["BITSHIFTLEFT"] = exp.FromArgListFunc(exp.KindBitwiseLeftShift)
	out["BITSHIFTRIGHT"] = exp.FromArgListFunc(exp.KindBitwiseRightShift)
	out["LOCALTIMESTAMP"] = func(args []exp.Expression) exp.Expression {
		return exp.CurrentTimestamp(exp.Args{"this": seqGet(args, 0)})
	}
	for name, target := range map[string]exp.DType{
		"TO_TIME": exp.DTypeTime, "TO_TIMESTAMP": exp.DTypeTimestamp,
		"TO_TIMESTAMP_LTZ": exp.DTypeTimestampLtz, "TO_TIMESTAMP_NTZ": exp.DTypeTimestampNtz,
		"TO_TIMESTAMP_TZ": exp.DTypeTimestampTz,
		"TRY_TO_DATE":     exp.DTypeDate, "TRY_TO_TIME": exp.DTypeTime,
		"TRY_TO_TIMESTAMP":     exp.DTypeTimestamp,
		"TRY_TO_TIMESTAMP_LTZ": exp.DTypeTimestampLtz, "TRY_TO_TIMESTAMP_NTZ": exp.DTypeTimestampNtz,
		"TRY_TO_TIMESTAMP_TZ": exp.DTypeTimestampTz,
	} {
		safe := strings.HasPrefix(name, "TRY_")
		// TIMESTAMP_TYPES (parsers/snowflake.py:74-79) covers all four timestamp flavors.
		isTimestampType := target == exp.DTypeTimestamp || target == exp.DTypeTimestampLtz ||
			target == exp.DTypeTimestampNtz || target == exp.DTypeTimestampTz
		out[name] = func(args []exp.Expression) exp.Expression {
			value := seqGet(args, 0)
			if isTimestampType && safe && len(args) == 1 &&
				value != nil && value.IsString() {
				if _, err := strconv.ParseInt(value.Name(), 10, 64); err == nil {
					return exp.Anonymous(exp.Args{"this": name, "expressions": args})
				}
			}
			if isTimestampType && !safe {
				scale := seqGet(args, 1)
				valueIsInteger := false
				if value != nil {
					_, err := strconv.ParseInt(value.Name(), 10, 64)
					valueIsInteger = err == nil
				}
				scaleIsInteger := false
				if scale != nil && scale.IsNumber() {
					_, err := strconv.ParseInt(scale.Name(), 10, 64)
					scaleIsInteger = err == nil
				}
				if value != nil && ((len(args) == 1 && valueIsInteger) ||
					(scale != nil && scaleIsInteger)) {
					return exp.UnixToTime(exp.Args{
						"this": value, "scale": scale,
						"target_type": exp.DataType(exp.Args{"this": target}),
					})
				}
				if scale != nil && !scale.IsNumber() {
					parsed := postgresBuildFormattedTimeClass(d, exp.KindStrToTime)(args)
					parsed.Set("safe", false)
					parsed.Set("target_type", exp.DataType(exp.Args{"this": target}))
					return parsed
				}
			}
			if len(args) == 1 && value != nil && value.IsString() {
				_, integerString := strconv.ParseInt(value.Name(), 10, 64)
				if isTimestampType && !safe && integerString == nil {
					return exp.UnixToTime(exp.Args{
						"this":        value,
						"target_type": exp.DataType(exp.Args{"this": target}),
					})
				}
				kind := exp.KindCast
				if safe {
					kind = exp.KindTryCast
				}
				cast := exp.New(kind, exp.Args{
					"this": value,
					"to":   exp.DataType(exp.Args{"this": target}),
				})
				if safe {
					cast.Set("requires_string", true)
					if target == exp.DTypeDate {
						cast.Set("probe_date_format", true)
					}
				}
				return cast
			}
			if target == exp.DTypeTime && len(args) > 1 {
				parsed := postgresBuildFormattedTimeClass(d, exp.KindTsOrDsToTime)(args)
				parsed.Set("safe", safe)
				return parsed
			}
			if target == exp.DTypeDate && len(args) > 1 {
				parsed := dateWithFormat(args)
				parsed.Set("safe", safe)
				return parsed
			}
			if isTimestampType && len(args) > 1 {
				parsed := postgresBuildFormattedTimeClass(d, exp.KindStrToTime)(args)
				parsed.Set("safe", safe)
				parsed.Set("target_type", exp.DataType(exp.Args{"this": target}))
				return parsed
			}
			if target == exp.DTypeTime && len(args) == 1 {
				return exp.New(exp.KindTsOrDsToTime, exp.Args{"this": value, "safe": safe})
			}
			return exp.Anonymous(exp.Args{"this": name, "expressions": args})
		}
	}

	// parsers/snowflake.py:524: FLATTEN parses as exp.Explode; the lateral parse site then
	// defaults its alias columns to FLATTEN_COLUMNS.
	out["FLATTEN"] = exp.FromArgListFunc(exp.KindExplode)

	// parsers/snowflake.py:630-635: TRUNC/TRUNCATE build a numeric Trunc that records that
	// fractional digits are supported (DuckDB later casts the decimals to INT).
	out["TRUNC"] = snowflakeTruncBuilder
	out["TRUNCATE"] = snowflakeTruncBuilder
	out["DATE_TRUNC"] = snowflakeDateTruncBuilder
	out["DATETRUNC"] = snowflakeDateTruncBuilder
	out["DATEADD"] = func(args []exp.Expression) exp.Expression {
		return exp.DateAdd(exp.Args{"this": seqGet(args, 2), "expression": seqGet(args, 1), "unit": snowflakeDateUnit(seqGet(args, 0))})
	}
	out["TIMESTAMPADD"] = func(args []exp.Expression) exp.Expression {
		return exp.DateAdd(exp.Args{"this": seqGet(args, 2), "expression": seqGet(args, 1), "unit": snowflakeDateUnit(seqGet(args, 0))})
	}
	out["TIMEADD"] = func(args []exp.Expression) exp.Expression {
		return exp.TimeAdd(exp.Args{"this": seqGet(args, 2), "expression": seqGet(args, 1), "unit": snowflakeDateUnit(seqGet(args, 0))})
	}
	out["DATEDIFF"] = func(args []exp.Expression) exp.Expression {
		return exp.DateDiff(exp.Args{"this": seqGet(args, 2), "expression": seqGet(args, 1), "unit": snowflakeDateUnit(seqGet(args, 0)), "date_part_boundary": true})
	}
	out["TIMESTAMPDIFF"] = func(args []exp.Expression) exp.Expression {
		return exp.DateDiff(exp.Args{"this": seqGet(args, 2), "expression": seqGet(args, 1), "unit": snowflakeDateUnit(seqGet(args, 0)), "date_part_boundary": true})
	}
	out["TIMEDIFF"] = func(args []exp.Expression) exp.Expression {
		return exp.DateDiff(exp.Args{"this": seqGet(args, 2), "expression": seqGet(args, 1), "unit": snowflakeDateUnit(seqGet(args, 0))})
	}
	out["CHARINDEX"] = func(args []exp.Expression) exp.Expression {
		return exp.StrPosition(exp.Args{
			"this": seqGet(args, 1), "substr": seqGet(args, 0), "position": seqGet(args, 2), "clamp_position": true,
		})
	}
	out["DIV0"] = func(args []exp.Expression) exp.Expression {
		lhs := snowflakeWrapBinary(seqGet(args, 0))
		rhs := snowflakeWrapBinary(seqGet(args, 1))
		cond := exp.And(exp.Args{
			"this": exp.EQ(exp.Args{"this": rhs, "expression": exp.LiteralNumber("0")}),
			"expression": exp.Not(exp.Args{"this": exp.Is(exp.Args{
				"this": lhs, "expression": exp.Null(),
			})}),
		})
		return exp.If(exp.Args{
			"this":  cond,
			"true":  exp.LiteralNumber("0"),
			"false": exp.Div(exp.Args{"this": lhs, "expression": rhs}),
		})
	}
	out["DIV0NULL"] = func(args []exp.Expression) exp.Expression {
		lhs := snowflakeWrapBinary(seqGet(args, 0))
		rhs := snowflakeWrapBinary(seqGet(args, 1))
		cond := exp.Or(exp.Args{
			"this":       exp.EQ(exp.Args{"this": rhs, "expression": exp.LiteralNumber("0")}),
			"expression": exp.Is(exp.Args{"this": rhs, "expression": exp.Null()}),
		})
		return exp.If(exp.Args{
			"this": cond, "true": exp.LiteralNumber("0"),
			"false": exp.Div(exp.Args{"this": lhs, "expression": rhs}),
		})
	}
	out["MD5_HEX"] = exp.FromArgListFunc(exp.KindMD5)
	out["REPLACE"] = func(args []exp.Expression) exp.Expression {
		if len(args) == 2 {
			args = append(args, exp.LiteralString(""))
		}
		return exp.FromArgList(exp.KindReplace, args)
	}
	out["REGEXP_REPLACE"] = func(args []exp.Expression) exp.Expression {
		if len(args) == 2 {
			args = append(args, exp.LiteralString(""))
		}
		return exp.FromArgList(exp.KindRegexpReplace, args)
	}
	out["REGEXP_SUBSTR_ALL"] = func(args []exp.Expression) exp.Expression {
		return exp.RegexpExtractAll(exp.Args{
			"this": seqGet(args, 0), "expression": seqGet(args, 1),
			"position": seqGet(args, 2), "occurrence": seqGet(args, 3),
			"parameters": seqGet(args, 4), "group": seqGet(args, 5),
		})
	}
	out["STDDEV_SAMP"] = exp.FromArgListFunc(exp.KindStddev)
	out["APPROX_TOP_K"] = func(args []exp.Expression) exp.Expression {
		expression := seqGet(args, 1)
		if expression == nil {
			expression = exp.LiteralNumber("1")
		}
		return exp.ApproxTopK(exp.Args{
			"this": seqGet(args, 0), "expression": expression, "counters": seqGet(args, 2),
		})
	}
	out["APPROXIMATE_JACCARD_INDEX"] = exp.FromArgListFunc(exp.KindApproximateSimilarity)
	out["APPROXIMATE_SIMILARITY"] = exp.FromArgListFunc(exp.KindApproximateSimilarity)
	out["STRTOK"] = func(args []exp.Expression) exp.Expression {
		delimiter := seqGet(args, 1)
		if delimiter == nil {
			delimiter = exp.LiteralString(" ")
		}
		partIndex := seqGet(args, 2)
		if partIndex == nil {
			partIndex = exp.LiteralNumber("1")
		}
		return exp.Strtok(exp.Args{
			"this": seqGet(args, 0), "delimiter": delimiter, "part_index": partIndex,
		})
	}
	out["NULLIFZERO"] = func(args []exp.Expression) exp.Expression {
		value := seqGet(args, 0)
		return exp.If(exp.Args{
			"this": exp.EQ(exp.Args{"this": value, "expression": exp.LiteralNumber("0")}),
			"true": exp.Null(), "false": value,
		})
	}
	// parsers/snowflake.py:570-571 + 232-247 _build_regexp_extract: the position form records
	// that an out-of-range position returns NULL (REGEXP_EXTRACT_POSITION_OVERFLOW_RETURNS_NULL).
	regexpExtract := func(kind exp.Kind) func([]exp.Expression) exp.Expression {
		return func(args []exp.Expression) exp.Expression {
			m := exp.Args{
				"this":       seqGet(args, 0),
				"expression": seqGet(args, 1),
				"position":   seqGet(args, 2),
				"occurrence": seqGet(args, 3),
				"parameters": seqGet(args, 4),
				"group":      seqGet(args, 5),
			}
			if m["group"] == nil {
				m["group"] = exp.LiteralNumber("0")
			}
			if kind == exp.KindRegexpExtract {
				m["null_if_pos_overflow"] = d.RegexpExtractPositionOverflowReturnsNull
			}
			return exp.New(kind, m)
		}
	}
	out["REGEXP_SUBSTR"] = regexpExtract(exp.KindRegexpExtract)
	out["REGEXP_SUBSTR_ALL"] = regexpExtract(exp.KindRegexpExtractAll)
	out["GET_PATH"] = func(args []exp.Expression) exp.Expression {
		return exp.JSONExtract(exp.Args{
			"this":          seqGet(args, 0),
			"expression":    d.ToJSONPath(seqGet(args, 1)),
			"requires_json": true,
		})
	}
	out["IFF"] = exp.FromArgListFunc(exp.KindIf)
	out["LEN"] = func(args []exp.Expression) exp.Expression {
		return exp.Length(exp.Args{"this": seqGet(args, 0)})
	}
	out["OBJECT_CONSTRUCT"] = func(args []exp.Expression) exp.Expression {
		return exp.StarMap(exp.Args{"expressions": args})
	}
	out["OBJECT_CONSTRUCT_KEEP_NULL"] = func(args []exp.Expression) exp.Expression {
		pairs := make([]exp.Expression, 0, len(args)/2)
		for i := 0; i+1 < len(args); i += 2 {
			pairs = append(pairs, exp.JSONKeyValue(exp.Args{"this": args[i], "expression": args[i+1]}))
		}
		return exp.JSONObject(exp.Args{"expressions": pairs})
	}
	out["ARRAY_CONSTRUCT"] = func(args []exp.Expression) exp.Expression {
		return exp.Array(exp.Args{"expressions": args})
	}
	// parsers/snowflake.py:413-418: ARRAY_CONTAINS(value, array) keeps the value's type
	// (ensure_variant=False), so the generator does not cast it to VARIANT when round-tripping.
	out["ARRAY_CONTAINS"] = func(args []exp.Expression) exp.Expression {
		return exp.ArrayContains(exp.Args{
			"this": seqGet(args, 1), "expression": seqGet(args, 0),
			"ensure_variant": false, "check_null": true,
		})
	}
	out["ARRAY_DISTINCT"] = func(args []exp.Expression) exp.Expression {
		return exp.ArrayDistinct(exp.Args{"this": seqGet(args, 0), "check_null": true})
	}
	out["ARRAY_INTERSECTION"] = func(args []exp.Expression) exp.Expression {
		return exp.ArrayIntersect(exp.Args{"expressions": args, "is_multiset": true})
	}
	out["ARRAY_POSITION"] = func(args []exp.Expression) exp.Expression {
		return exp.ArrayPosition(exp.Args{"this": seqGet(args, 1), "expression": seqGet(args, 0), "zero_based": true})
	}
	out["ARRAYS_OVERLAP"] = func(args []exp.Expression) exp.Expression {
		return exp.ArrayOverlaps(exp.Args{"this": seqGet(args, 0), "expression": seqGet(args, 1), "null_safe": true})
	}
	out["BITAND"] = snowflakeBitwiseBuilder(exp.KindBitwiseAnd)
	out["BIT_AND"] = snowflakeBitwiseBuilder(exp.KindBitwiseAnd)
	out["BITOR"] = snowflakeBitwiseBuilder(exp.KindBitwiseOr)
	out["BIT_OR"] = snowflakeBitwiseBuilder(exp.KindBitwiseOr)
	out["BITXOR"] = snowflakeBitwiseBuilder(exp.KindBitwiseXor)
	out["BIT_XOR"] = snowflakeBitwiseBuilder(exp.KindBitwiseXor)
	out["BITNOT"] = func(args []exp.Expression) exp.Expression { return exp.BitwiseNot(exp.Args{"this": seqGet(args, 0)}) }
	out["BIT_NOT"] = func(args []exp.Expression) exp.Expression { return exp.BitwiseNot(exp.Args{"this": seqGet(args, 0)}) }
	out["BITANDAGG"] = exp.FromArgListFunc(exp.KindBitwiseAndAgg)
	out["BITAND_AGG"] = exp.FromArgListFunc(exp.KindBitwiseAndAgg)
	out["BIT_AND_AGG"] = exp.FromArgListFunc(exp.KindBitwiseAndAgg)
	out["BIT_ANDAGG"] = exp.FromArgListFunc(exp.KindBitwiseAndAgg)
	out["BITORAGG"] = exp.FromArgListFunc(exp.KindBitwiseOrAgg)
	out["BITOR_AGG"] = exp.FromArgListFunc(exp.KindBitwiseOrAgg)
	out["BIT_OR_AGG"] = exp.FromArgListFunc(exp.KindBitwiseOrAgg)
	out["BIT_ORAGG"] = exp.FromArgListFunc(exp.KindBitwiseOrAgg)
	out["BITXORAGG"] = exp.FromArgListFunc(exp.KindBitwiseXorAgg)
	out["BITXOR_AGG"] = exp.FromArgListFunc(exp.KindBitwiseXorAgg)
	out["BIT_XOR_AGG"] = exp.FromArgListFunc(exp.KindBitwiseXorAgg)
	out["BIT_XORAGG"] = exp.FromArgListFunc(exp.KindBitwiseXorAgg)
	out["BOOLAND"] = func(args []exp.Expression) exp.Expression {
		return exp.Booland(exp.Args{"this": seqGet(args, 0), "expression": seqGet(args, 1), "round_input": true})
	}
	out["BOOLOR"] = func(args []exp.Expression) exp.Expression {
		return exp.Boolor(exp.Args{"this": seqGet(args, 0), "expression": seqGet(args, 1), "round_input": true})
	}
	out["BOOLNOT"] = func(args []exp.Expression) exp.Expression {
		return exp.Boolnot(exp.Args{"this": seqGet(args, 0), "round_input": true})
	}
	out["BOOLXOR"] = func(args []exp.Expression) exp.Expression {
		return exp.Xor(exp.Args{"this": seqGet(args, 0), "expression": seqGet(args, 1), "round_input": true})
	}
	out["DAYOFWEEKISO"] = exp.FromArgListFunc(exp.KindDayOfWeekIso)
	out["DAYOFYEAR"] = exp.FromArgListFunc(exp.KindDayOfYear)
	out["WEEKISO"] = exp.FromArgListFunc(exp.KindWeekOfYear)
	out["WEEKOFYEAR"] = exp.FromArgListFunc(exp.KindWeek)
	out["YEAROFWEEK"] = exp.FromArgListFunc(exp.KindYearOfWeek)
	out["OBJECT_KEYS"] = exp.FromArgListFunc(exp.KindJSONKeys)
	out["TO_JSON"] = exp.FromArgListFunc(exp.KindJSONFormat)
	out["SQUARE"] = func(args []exp.Expression) exp.Expression {
		return exp.Pow(exp.Args{"this": seqGet(args, 0), "expression": exp.LiteralNumber("2")})
	}
	out["STARTSWITH"] = snowflakeBinaryBuilder(exp.KindStartsWith)
	out["ENDSWITH"] = snowflakeBinaryBuilder(exp.KindEndsWith)
	// parser.py:469-473 records the source dialect's string UUID type.
	out["UUID_STRING"] = func(args []exp.Expression) exp.Expression {
		uuid := exp.FromArgList(exp.KindUuid, args)
		uuid.Set("is_string", true)
		return uuid
	}
	out["HEX_DECODE_BINARY"] = exp.FromArgListFunc(exp.KindUnhex)
	out["HEX_ENCODE"] = exp.FromArgListFunc(exp.KindHex)
	out["STRTOK_TO_ARRAY"] = func(args []exp.Expression) exp.Expression {
		delimiter := seqGet(args, 1)
		if delimiter == nil {
			delimiter = exp.LiteralString(" ")
		}
		return exp.StrtokToArray(exp.Args{"this": seqGet(args, 0), "expression": delimiter})
	}
	out["GET"] = exp.FromArgListFunc(exp.KindGetExtract)
	// parsers/snowflake.py:587-592: Snowflake SPLIT propagates a NULL delimiter as NULL and
	// returns the whole string as one element for an empty delimiter.
	out["SPLIT"] = func(args []exp.Expression) exp.Expression {
		return exp.New(exp.KindSplit, exp.Args{
			"this":                          seqGet(args, 0),
			"expression":                    seqGet(args, 1),
			"null_returns_null":             true,
			"empty_delimiter_returns_whole": true,
		})
	}
	for name, kind := range map[string]exp.Kind{"LIKE": exp.KindLike, "ILIKE": exp.KindILike} {
		out[name] = func(args []exp.Expression) exp.Expression {
			like := exp.New(kind, exp.Args{
				"this": seqGet(args, 0), "expression": seqGet(args, 1),
			})
			if escape := seqGet(args, 2); escape != nil {
				return exp.Escape(exp.Args{"this": like, "expression": escape})
			}
			return like
		}
	}
	timestampFromParts := func(args []exp.Expression) exp.Expression {
		if len(args) == 2 {
			return exp.New(exp.KindTimestampFromParts, exp.Args{
				"this": args[0], "expression": args[1],
			})
		}
		return exp.FromArgList(exp.KindTimestampFromParts, args)
	}
	for _, name := range []string{
		"TIMESTAMPFROMPARTS", "TIMESTAMP_FROM_PARTS", "TIMESTAMPNTZFROMPARTS",
		"TIMESTAMP_NTZ_FROM_PARTS",
	} {
		out[name] = timestampFromParts
	}

	// dialects/snowflake.py:27 ARRAY_AGG_INCLUDES_NULLS = None: the Snowflake ARRAY_AGG
	// excludes NULLs when used as a source, but a Snowflake target keeps the native call.
	out["ARRAY_AGG"] = func(args []exp.Expression) exp.Expression {
		return exp.ArrayAgg(exp.Args{"this": seqGet(args, 0), "nulls_excluded": true})
	}

	// parsers/snowflake.py:396-733: the remaining FUNCTIONS entries that preserve
	// Snowflake's argument semantics/flags for transpilation.
	out["ADD_MONTHS"] = func(args []exp.Expression) exp.Expression {
		return exp.AddMonths(exp.Args{
			"this": seqGet(args, 0), "expression": seqGet(args, 1), "preserve_end_of_month": true,
		})
	}
	out["APPROX_PERCENTILE"] = exp.FromArgListFunc(exp.KindApproxQuantile)
	out["CURRENT_TIME"] = func(args []exp.Expression) exp.Expression {
		return exp.Localtime(exp.Args{"this": seqGet(args, 0)})
	}
	out["ARRAY_GENERATE_RANGE"] = func(args []exp.Expression) exp.Expression {
		return exp.GenerateSeries(exp.Args{
			"start": seqGet(args, 0), "end": seqGet(args, 1), "step": seqGet(args, 2),
			// Snowflake has exclusive end semantics.
			"is_end_exclusive": true,
		})
	}
	out["ARRAY_EXCEPT"] = func(args []exp.Expression) exp.Expression {
		return exp.New(exp.KindArrayExcept, exp.Args{
			"this": seqGet(args, 0), "expression": seqGet(args, 1), "is_multiset": true,
		})
	}
	out["ARRAY_SLICE"] = func(args []exp.Expression) exp.Expression {
		return exp.New(exp.KindArraySlice, exp.Args{
			"this": seqGet(args, 0), "start": seqGet(args, 1), "end": seqGet(args, 2),
			"zero_based": true,
		})
	}
	out["ARRAY_SORT"] = func(args []exp.Expression) exp.Expression {
		asc := seqGet(args, 1)
		nullsFirst := seqGet(args, 2)
		if nullsFirst == nil && asc != nil && asc.Kind() == exp.KindBoolean {
			ascValue, _ := asc.Arg("this").(bool)
			nullsFirst = exp.Boolean(exp.Args{"this": !ascValue})
		}
		return exp.New(exp.KindSortArray, exp.Args{
			"this": seqGet(args, 0), "asc": asc, "nulls_first": nullsFirst,
		})
	}
	out["ARRAY_FLATTEN"] = exp.FromArgListFunc(exp.KindFlatten)
	out["ARRAY_TO_STRING"] = func(args []exp.Expression) exp.Expression {
		return exp.ArrayToString(exp.Args{
			"this": seqGet(args, 0), "expression": seqGet(args, 1),
			"null_is_empty": true, "null_delim_is_null": true,
		})
	}
	out["CORR"] = func(args []exp.Expression) exp.Expression {
		return exp.Corr(exp.Args{
			"this": seqGet(args, 0), "expression": seqGet(args, 1), "null_on_zero_variance": true,
		})
	}
	out["COUNT_IF"] = func(args []exp.Expression) exp.Expression {
		return exp.CountIf(exp.Args{"this": seqGet(args, 0), "zero_on_all_null": true})
	}
	out["DAYNAME"] = func(args []exp.Expression) exp.Expression {
		return exp.New(exp.KindDayname, exp.Args{"this": seqGet(args, 0), "abbreviated": true})
	}
	out["MONTHNAME"] = func(args []exp.Expression) exp.Expression {
		return exp.New(exp.KindMonthname, exp.Args{"this": seqGet(args, 0), "abbreviated": true})
	}
	out["LAST_DAY"] = func(args []exp.Expression) exp.Expression {
		return exp.LastDay(exp.Args{"this": seqGet(args, 0), "unit": snowflakeDateUnit(seqGet(args, 1))})
	}
	out["LEFT"] = func(args []exp.Expression) exp.Expression {
		return exp.Left(exp.Args{
			"this": seqGet(args, 0), "expression": seqGet(args, 1), "negative_length_returns_empty": true,
		})
	}
	out["RIGHT"] = func(args []exp.Expression) exp.Expression {
		return exp.Right(exp.Args{
			"this": seqGet(args, 0), "expression": seqGet(args, 1), "negative_length_returns_empty": true,
		})
	}
	for _, name := range []string{"LEN", "LENGTH"} {
		out[name] = func(args []exp.Expression) exp.Expression {
			return exp.Length(exp.Args{"this": seqGet(args, 0), "binary": true})
		}
	}
	out["OCTET_LENGTH"] = exp.FromArgListFunc(exp.KindByteLength)
	out["PARSE_URL"] = func(args []exp.Expression) exp.Expression {
		return exp.New(exp.KindParseUrl, exp.Args{"this": seqGet(args, 0), "permissive": seqGet(args, 1)})
	}
	out["RANDOM"] = func(args []exp.Expression) exp.Expression {
		return exp.Rand(exp.Args{
			"this":  seqGet(args, 0),
			"lower": exp.LiteralNumber("-9.223372036854776E+18"),
			"upper": exp.LiteralNumber("9.223372036854776e+18"),
		})
	}
	regexpExtractBuilder := func(kind exp.Kind, nullIfOverflow bool) func([]exp.Expression) exp.Expression {
		return func(args []exp.Expression) exp.Expression {
			group := seqGet(args, 5)
			if group == nil {
				group = exp.LiteralNumber("0")
			}
			built := exp.New(kind, exp.Args{
				"this": seqGet(args, 0), "expression": seqGet(args, 1),
				"position": seqGet(args, 2), "occurrence": seqGet(args, 3),
				"parameters": seqGet(args, 4), "group": group,
			})
			if nullIfOverflow {
				built.Set("null_if_pos_overflow", true)
			}
			return built
		}
	}
	out["REGEXP_EXTRACT_ALL"] = regexpExtractBuilder(exp.KindRegexpExtractAll, false)
	regexpLikeBuilder := func(args []exp.Expression) exp.Expression {
		return exp.New(exp.KindRegexpLike, exp.Args{
			"this": seqGet(args, 0), "expression": seqGet(args, 1), "flag": seqGet(args, 2),
			"full_match": true,
		})
	}
	out["REGEXP_LIKE"] = regexpLikeBuilder
	out["RLIKE"] = regexpLikeBuilder
	out["SHA1_BINARY"] = exp.FromArgListFunc(exp.KindSHA1Digest)
	out["SHA1_HEX"] = exp.FromArgListFunc(exp.KindSHA)
	out["SHA2_BINARY"] = exp.FromArgListFunc(exp.KindSHA2Digest)
	out["SHA2_HEX"] = exp.FromArgListFunc(exp.KindSHA2)
	out["MD5_BINARY"] = exp.FromArgListFunc(exp.KindMD5Digest)
	out["MD5_NUMBER_LOWER64"] = exp.FromArgListFunc(exp.KindMD5NumberLower64)
	out["MD5_NUMBER_UPPER64"] = exp.FromArgListFunc(exp.KindMD5NumberUpper64)
	out["BITMAP_OR_AGG"] = exp.FromArgListFunc(exp.KindBitmapOrAgg)
	out["SPLIT_PART"] = func(args []exp.Expression) exp.Expression {
		return exp.SplitPart(exp.Args{
			"this": seqGet(args, 0), "delimiter": seqGet(args, 1), "part_index": seqGet(args, 2),
			"part_index_zero_as_one": true, "empty_delimiter_returns_whole": true,
		})
	}
	out["SYSDATE"] = func(args []exp.Expression) exp.Expression {
		return exp.CurrentTimestamp(exp.Args{"this": seqGet(args, 0), "sysdate": true})
	}
	out["TIME_FROM_PARTS"] = func(args []exp.Expression) exp.Expression {
		return exp.TimeFromParts(exp.Args{
			"hour": seqGet(args, 0), "min": seqGet(args, 1), "sec": seqGet(args, 2),
			"nano": seqGet(args, 3), "overflow": true,
		})
	}
	out["UNICODE"] = func(args []exp.Expression) exp.Expression {
		return exp.Unicode(exp.Args{"this": seqGet(args, 0), "empty_is_zero": true})
	}
	out["JAROWINKLER_SIMILARITY"] = func(args []exp.Expression) exp.Expression {
		return exp.JarowinklerSimilarity(exp.Args{
			"this": seqGet(args, 0), "expression": seqGet(args, 1),
			"case_insensitive": true, "integer_scale": true,
		})
	}
	out["GREATEST_IGNORE_NULLS"] = func(kind exp.Kind) func([]exp.Expression) exp.Expression {
		return func(args []exp.Expression) exp.Expression {
			values := exp.Args{"ignore_nulls": true}
			if len(args) > 0 {
				values["this"] = args[0]
			}
			if len(args) > 1 {
				values["expressions"] = args[1:]
			}
			return exp.New(kind, values)
		}
	}(exp.KindGreatest)
	out["LEAST_IGNORE_NULLS"] = func(kind exp.Kind) func([]exp.Expression) exp.Expression {
		return func(args []exp.Expression) exp.Expression {
			values := exp.Args{"ignore_nulls": true}
			if len(args) > 0 {
				values["this"] = args[0]
			}
			if len(args) > 1 {
				values["expressions"] = args[1:]
			}
			return exp.New(kind, values)
		}
	}(exp.KindLeast)
	out["TRY_PARSE_JSON"] = func(args []exp.Expression) exp.Expression {
		return exp.ParseJSON(exp.Args{"this": seqGet(args, 0), "safe": true})
	}
	out["TRY_TO_DOUBLE"] = func(args []exp.Expression) exp.Expression {
		return exp.ToDouble(exp.Args{
			"this": seqGet(args, 0), "format": seqGet(args, 1), "safe": true,
		})
	}
	out["TRY_TO_BINARY"] = func(args []exp.Expression) exp.Expression {
		return exp.ToBinary(exp.Args{
			"this": seqGet(args, 0), "format": seqGet(args, 1), "safe": true,
		})
	}
	out["TO_GEOGRAPHY"] = snowflakeToGeographyBuilder("TO_GEOGRAPHY", exp.DTypeGeography)
	out["TO_GEOMETRY"] = snowflakeToGeographyBuilder("TO_GEOMETRY", exp.DTypeGeometry)
	out["VECTOR_COSINE_SIMILARITY"] = exp.FromArgListFunc(exp.KindCosineDistance)
	out["VECTOR_INNER_PRODUCT"] = exp.FromArgListFunc(exp.KindDotProduct)
	out["VECTOR_L1_DISTANCE"] = exp.FromArgListFunc(exp.KindManhattanDistance)
	out["VECTOR_L2_DISTANCE"] = exp.FromArgListFunc(exp.KindEuclideanDistance)
	out["SKEW"] = exp.FromArgListFunc(exp.KindSkewness)
	out["BIT_SHIFTLEFT"] = snowflakeShiftBuilder(exp.KindBitwiseLeftShift)
	out["BIT_SHIFTRIGHT"] = snowflakeShiftBuilder(exp.KindBitwiseRightShift)
	out["BITSHIFTLEFT"] = snowflakeShiftBuilder(exp.KindBitwiseLeftShift)
	out["BITSHIFTRIGHT"] = snowflakeShiftBuilder(exp.KindBitwiseRightShift)
	out["DATE_FROM_PARTS"] = func(args []exp.Expression) exp.Expression {
		return exp.DateFromParts(exp.Args{
			"year": seqGet(args, 0), "month": seqGet(args, 1), "day": seqGet(args, 2),
			"allow_overflow": true,
		})
	}
	out["DATEFROMPARTS"] = out["DATE_FROM_PARTS"]

	return out
}

// snowflakeToGeographyBuilder ports the TO_GEOGRAPHY/TO_GEOMETRY lambdas
// (parsers/snowflake.py:690-701): a single argument becomes a cast, otherwise the call is
// kept as an anonymous function.
func snowflakeToGeographyBuilder(name string, dtype exp.DType) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		if len(args) == 1 {
			return exp.Cast(exp.Args{"this": args[0], "to": exp.DataType(exp.Args{"this": dtype})})
		}
		return exp.Anonymous(exp.Args{"this": name, "expressions": args})
	}
}

func snowflakeStaticTemporal(e exp.Expression) bool {
	if e == nil || e.Kind() != exp.KindCast {
		return false
	}
	to, _ := e.Arg("to").(exp.Expression)
	if to == nil {
		return false
	}
	dtype, _ := to.Arg("this").(exp.DType)
	switch dtype {
	case exp.DTypeDate, exp.DTypeDatetime, exp.DTypeTimestamp, exp.DTypeTimestampNtz,
		exp.DTypeTimestampTz, exp.DTypeTimestampLtz, exp.DTypeTime, exp.DTypeTimeTz:
		return true
	default:
		return false
	}
}

// snowflakeBitwiseBuilder ports _build_bitwise (parsers/snowflake.py:133-152).
func snowflakeBitwiseBuilder(kind exp.Kind) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		built := exp.New(kind, exp.Args{"this": seqGet(args, 0), "expression": seqGet(args, 1)})
		if len(args) == 3 {
			switch kind {
			case exp.KindBitwiseAnd, exp.KindBitwiseOr, exp.KindBitwiseXor:
				built.Set("padside", args[2])
			}
		}
		return built
	}
}

// snowflakeWrapBinary ports exp._wrap(expr, exp.Binary): parenthesize a binary operand.
func snowflakeWrapBinary(e exp.Expression) exp.Expression {
	if e != nil && e.Is(exp.TraitBinary) {
		return exp.Paren(exp.Args{"this": e})
	}
	return e
}

// snowflakeBinaryBuilder builds a this/expression binary-function node.
func snowflakeBinaryBuilder(kind exp.Kind) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		return exp.New(kind, exp.Args{"this": seqGet(args, 0), "expression": seqGet(args, 1)})
	}
}

// snowflakeDateTruncBuilder ports date_trunc_to_time (dialects/dialect.py:1682-1688).
func snowflakeDateTruncBuilder(args []exp.Expression) exp.Expression {
	unit := seqGet(args, 0)
	this := seqGet(args, 1)
	isDateUnit := snowflakeIsDateUnit(unit)
	if this != nil && this.Kind() == exp.KindCast && exp.IsType(asExpression(this.Arg("to")), exp.DTypeDate) {
		trunc := exp.DateTrunc(exp.Args{"unit": unit, "this": this})
		if !isDateUnit {
			trunc.Set("input_type_preserved", true)
		}
		return trunc
	}
	trunc := exp.New(exp.KindTimestampTrunc, exp.Args{"this": this, "unit": unit})
	dtype := exp.DTypeUnknown
	if this != nil && this.Kind() == exp.KindCast {
		if to := asExpression(this.Arg("to")); to != nil {
			dtype, _ = to.Arg("this").(exp.DType)
		}
	}
	if isDateUnit || dtype == exp.DTypeTime || dtype == exp.DTypeTimeTz {
		trunc.Set("input_type_preserved", true)
	}
	return trunc
}

// snowflakeIsDateUnit reports whether unit is a calendar date part.
func snowflakeIsDateUnit(unit exp.Expression) bool {
	if unit == nil {
		return false
	}
	name := strings.ToUpper(unit.Name())
	if canonical, ok := datePartMapping[name]; ok {
		name = canonical
	}
	switch name {
	case "YEAR", "QUARTER", "MONTH", "WEEK", "DAY":
		return true
	}
	return false
}

// snowflakeShiftBuilder ports _build_bitwise for shifts: Snowflake specifies INT128.
func snowflakeShiftBuilder(kind exp.Kind) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		built := exp.New(kind, exp.Args{"this": seqGet(args, 0), "expression": seqGet(args, 1), "requires_int128": true})
		return built
	}
}

// snowflakeTruncBuilder ports parsers/snowflake.py:630-635: the numeric TRUNC form records
// fractions_supported so DuckDB casts a non-integer decimals argument to INT.
func snowflakeTruncBuilder(args []exp.Expression) exp.Expression {
	trunc := exp.New(exp.KindTrunc, exp.Args{"this": seqGet(args, 0), "decimals": seqGet(args, 1)})
	if len(args) > 1 {
		trunc.Set("fractions_supported", true)
	}
	return trunc
}
