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
	d.ArrayFuncsPropagatesNulls = true
	d.AlterTableAddRequiredForEachColumn = false
	tryCastRequiresString := true
	d.TryCastRequiresString = &tryCastRequiresString
	d.SupportsAliasRefsInJoinConditions = true
	d.LeastGreatestIgnoresNulls = false
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
	d.InverseTimeMapping = map[string]string{
		"T": "T", "%Y": "yyyy", "%y": "yy", "%B": "mmmm", "%b": "mon",
		"%m": "mm", "%d": "DD", "%-d": "dd", "%a": "DY", "%w": "dy",
		"%H": "hh24", "%I": "hh12", "%M": "mi", "%S": "ss", "%f": "ff6",
		"%f_zero": "ff0", "%f_one": "ff1", "%f_two": "ff2", "%f_three": "ff3",
		"%f_four": "ff4", "%f_five": "ff5", "%f_seven": "ff7", "%f_eight": "ff8",
		"%f_nine": "ff9", "%z": "tzhtzm", "%:z": "tzh:tzm", "%-z": "tzh", "%p": "pm",
	}
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
		names := []string{"ROWCOUNT", "TIMELIMIT"}
		mapped := make([]exp.Expression, 0, len(args))
		for i, arg := range args {
			if arg == nil {
				continue
			}
			if arg.Kind() == exp.KindKwarg {
				mapped = append(mapped, arg)
				continue
			}
			if i < len(names) {
				mapped = append(mapped, exp.Kwarg(exp.Args{
					"this": exp.ToIdentifier(names[i]), "expression": arg,
				}))
			} else {
				mapped = append(mapped, arg)
			}
		}
		return exp.Anonymous(exp.Args{"this": "GENERATOR", "expressions": mapped})
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
		"TRY_TO_DATE": exp.DTypeDate, "TRY_TO_TIME": exp.DTypeTime,
		"TRY_TO_TIMESTAMP": exp.DTypeTimestamp,
	} {
		safe := strings.HasPrefix(name, "TRY_")
		out[name] = func(args []exp.Expression) exp.Expression {
			value := seqGet(args, 0)
			if target == exp.DTypeTimestamp && safe && len(args) == 1 &&
				value != nil && value.IsString() {
				if _, err := strconv.ParseInt(value.Name(), 10, 64); err == nil {
					return exp.Anonymous(exp.Args{"this": name, "expressions": args})
				}
			}
			if target == exp.DTypeTimestamp && !safe {
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
				if target == exp.DTypeTimestamp && !safe && integerString == nil {
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
			return exp.Anonymous(exp.Args{"this": name, "expressions": args})
		}
	}

	// parsers/snowflake.py:524: FLATTEN parses as exp.Explode; the lateral parse site then
	// defaults its alias columns to FLATTEN_COLUMNS.
	out["FLATTEN"] = exp.FromArgListFunc(exp.KindExplode)

	out["DATE_TRUNC"] = snowflakeDateTruncBuilder
	out["DATETRUNC"] = snowflakeDateTruncBuilder
	out["DATEADD"] = func(args []exp.Expression) exp.Expression {
		return exp.DateAdd(exp.Args{"this": seqGet(args, 2), "expression": seqGet(args, 1), "unit": snowflakeDateUnit(seqGet(args, 0))})
	}
	out["TIMESTAMPADD"] = func(args []exp.Expression) exp.Expression {
		return exp.TimestampAdd(exp.Args{"this": seqGet(args, 2), "expression": seqGet(args, 1), "unit": snowflakeDateUnit(seqGet(args, 0))})
	}
	out["TIMEADD"] = func(args []exp.Expression) exp.Expression {
		return exp.TimeAdd(exp.Args{"this": seqGet(args, 2), "expression": seqGet(args, 1), "unit": snowflakeDateUnit(seqGet(args, 0))})
	}
	out["DATEDIFF"] = func(args []exp.Expression) exp.Expression {
		return exp.DateDiff(exp.Args{"this": seqGet(args, 2), "expression": seqGet(args, 1), "unit": snowflakeDateUnit(seqGet(args, 0))})
	}
	out["TIMESTAMPDIFF"] = func(args []exp.Expression) exp.Expression {
		return exp.TimestampDiff(exp.Args{"this": seqGet(args, 2), "expression": seqGet(args, 1), "unit": snowflakeDateUnit(seqGet(args, 0))})
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
	out["ARRAY_CONTAINS"] = func(args []exp.Expression) exp.Expression {
		return exp.ArrayContains(exp.Args{"this": seqGet(args, 1), "expression": seqGet(args, 0), "check_null": true})
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
	out["UUID_STRING"] = exp.FromArgListFunc(exp.KindUuid)
	out["HEX_DECODE_BINARY"] = exp.FromArgListFunc(exp.KindUnhex)
	out["HEX_ENCODE"] = exp.FromArgListFunc(exp.KindHex)
	out["STRTOK_TO_ARRAY"] = func(args []exp.Expression) exp.Expression {
		delimiter := seqGet(args, 1)
		if delimiter == nil {
			delimiter = exp.LiteralString(" ")
		}
		return exp.StrtokToArray(exp.Args{"this": seqGet(args, 0), "expression": delimiter})
	}
	out["SPLIT"] = exp.FromArgListFunc(exp.KindSplit)
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
		"TIMESTAMPFROMPARTS", "TIMESTAMPNTZFROMPARTS",
		"TIMESTAMP_NTZ_FROM_PARTS",
	} {
		out[name] = timestampFromParts
	}

	return out
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
	if this != nil && this.Kind() == exp.KindCast && exp.IsType(asExpression(this.Arg("to")), exp.DTypeDate) {
		return exp.DateTrunc(exp.Args{"unit": unit, "this": this})
	}
	return exp.New(exp.KindTimestampTrunc, exp.Args{"this": this, "unit": unit})
}
