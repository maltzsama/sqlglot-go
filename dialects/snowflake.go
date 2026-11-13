package dialects

import (
	"maps"

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

	d.InverseTimeMapping = map[string]string{"T": "T"}
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

	// parsers/snowflake.py:735-744 FUNCTION_PARSERS / FUNCTIONS removals.
	d.RemovedFunctions = map[string]bool{"PREDICT": true}
	d.Functions = snowflakeFunctions(d)

	return d
}

// snowflakeFunctions builds the SnowflakeParser.FUNCTIONS overlay
// (parsers/snowflake.py:396-733) for the identity-relevant entries.
func snowflakeFunctions(d *Dialect) map[string]func([]exp.Expression) exp.Expression {
	out := map[string]func([]exp.Expression) exp.Expression{}

	out["DATE_TRUNC"] = snowflakeDateTruncBuilder
	out["DATETRUNC"] = snowflakeDateTruncBuilder
	out["DATEADD"] = func(args []exp.Expression) exp.Expression {
		return exp.DateAdd(exp.Args{"this": seqGet(args, 2), "expression": seqGet(args, 1), "unit": seqGet(args, 0)})
	}
	out["TIMESTAMPADD"] = func(args []exp.Expression) exp.Expression {
		return exp.TimestampAdd(exp.Args{"this": seqGet(args, 2), "expression": seqGet(args, 1), "unit": seqGet(args, 0)})
	}
	out["TIMEADD"] = func(args []exp.Expression) exp.Expression {
		return exp.TimeAdd(exp.Args{"this": seqGet(args, 2), "expression": seqGet(args, 1), "unit": seqGet(args, 0)})
	}
	out["DATEDIFF"] = func(args []exp.Expression) exp.Expression {
		return exp.DateDiff(exp.Args{"this": seqGet(args, 2), "expression": seqGet(args, 1), "unit": seqGet(args, 0)})
	}
	out["TIMESTAMPDIFF"] = func(args []exp.Expression) exp.Expression {
		return exp.TimestampDiff(exp.Args{"this": seqGet(args, 2), "expression": seqGet(args, 1), "unit": seqGet(args, 0)})
	}
	out["TIMEDIFF"] = func(args []exp.Expression) exp.Expression {
		return exp.DateDiff(exp.Args{"this": seqGet(args, 2), "expression": seqGet(args, 1), "unit": seqGet(args, 0)})
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
	out["WEEKOFYEAR"] = exp.FromArgListFunc(exp.KindWeekOfYear)
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
	out["STRTOK_TO_ARRAY"] = exp.FromArgListFunc(exp.KindStrtokToArray)
	out["SPLIT"] = exp.FromArgListFunc(exp.KindSplit)

	return out
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
