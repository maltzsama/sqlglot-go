package dialects

import (
	"maps"

	exp "github.com/maltzsama/sqlglot-go/expressions"
	"github.com/maltzsama/sqlglot-go/tokens"
)

// duckDBInverseTimeMapping ports DuckDB.INVERSE_TIME_MAPPING (dialects/duckdb.py:39-51).
var duckDBInverseTimeMapping = map[string]string{
	"%e":       "%-d",
	"%:z":      "%z",
	"%-z":      "%z",
	"%f_zero":  "%n",
	"%f_one":   "%n",
	"%f_two":   "%n",
	"%f_three": "%g",
	"%f_four":  "%n",
	"%f_five":  "%n",
	"%f_seven": "%n",
	"%f_eight": "%n",
	"%f_nine":  "%n",
}

// DuckDB ports dialects/duckdb.py (class DuckDB): the tokenizer, parser flags and FUNCTIONS
// overlay. The parser grammar overrides live in parser/dialect_duckdb.go and the generator in
// generator/dialect_duckdb.go.
func DuckDB() *Dialect {
	d := Base()
	d.Name = "duckdb"

	// dialects/duckdb.py:20-39 class attributes.
	d.NullOrdering = "nulls_are_last"
	d.SupportsUserDefinedTypes = true
	d.IndexOffset = 1
	d.ConcatCoalesce = true
	d.ConcatWsCoalesce = true
	d.SupportsOrderByAll = true
	d.SupportsLimitAll = true
	d.SupportsFixedSizeArrays = true
	d.StrictJSONPathSyntax = false
	d.NumbersCanBeUnderscoreSeparated = true
	d.UUIDIsStringType = false
	d.NormalizationStrategy = CaseInsensitive
	d.DatePartMapping = maps.Clone(d.DatePartMapping)
	if d.DatePartMapping == nil {
		d.DatePartMapping = map[string]string{}
	}
	d.DatePartMapping["DAYOFWEEKISO"] = "ISODOW"
	delete(d.DatePartMapping, "WEEKDAY")
	d.InverseTimeMapping = duckDBInverseTimeMapping

	// dialects/duckdb.py:99-152 class Tokenizer.
	cfg := d.TokenizerConfig
	cfg.FormatStrings = maps.Clone(cfg.FormatStrings)
	if cfg.FormatStrings == nil {
		cfg.FormatStrings = map[string]tokens.FormatString{}
	}
	for _, prefix := range []string{"e'", "E'"} {
		cfg.FormatStrings[prefix] = tokens.FormatString{End: "'", TokenType: tokens.BYTE_STRING}
	}
	cfg.FormatStrings["$"] = tokens.FormatString{End: "$", TokenType: tokens.HEREDOC_STRING}
	cfg.ByteStringEscapes = map[rune]bool{'\'': true, '\\': true}
	cfg.HeredocTagIsIdentifier = true
	cfg.HeredocStringAlternative = tokens.PARAMETER
	cfg.SingleTokens = maps.Clone(cfg.SingleTokens)
	if cfg.SingleTokens == nil {
		cfg.SingleTokens = map[rune]tokens.TokenType{}
	}
	cfg.SingleTokens['$'] = tokens.PARAMETER
	cfg.VarSingleTokens = maps.Clone(cfg.VarSingleTokens)
	if cfg.VarSingleTokens == nil {
		cfg.VarSingleTokens = map[rune]bool{}
	}
	cfg.VarSingleTokens['$'] = true
	cfg.NumbersCanBeUnderscoreSeparated = true
	cfg.Keywords = maps.Clone(cfg.Keywords)
	if cfg.Keywords == nil {
		cfg.Keywords = map[string]tokens.TokenType{}
	}
	for keyword, tokenType := range map[string]tokens.TokenType{
		"//":           tokens.DIV,
		"**":           tokens.DSTAR,
		"^@":           tokens.CARET_AT,
		"@>":           tokens.AT_GT,
		"<@":           tokens.LT_AT,
		"ATTACH":       tokens.ATTACH,
		"BINARY":       tokens.VARBINARY,
		"BITSTRING":    tokens.BIT,
		"BPCHAR":       tokens.TEXT,
		"CHAR":         tokens.TEXT,
		"DATETIME":     tokens.TIMESTAMPNTZ,
		"DETACH":       tokens.DETACH,
		"FORCE":        tokens.FORCE,
		"INSTALL":      tokens.INSTALL,
		"INT8":         tokens.BIGINT,
		"LOGICAL":      tokens.BOOLEAN,
		"MACRO":        tokens.FUNCTION,
		"ONLY":         tokens.ONLY,
		"PIVOT_WIDER":  tokens.PIVOT,
		"POSITIONAL":   tokens.POSITIONAL,
		"RESET":        tokens.COMMAND,
		"ROW":          tokens.STRUCT,
		"SIGNED":       tokens.INT,
		"STRING":       tokens.TEXT,
		"SUMMARIZE":    tokens.SUMMARIZE,
		"TIMESTAMP":    tokens.TIMESTAMPNTZ,
		"TIMESTAMP_S":  tokens.TIMESTAMP_S,
		"TIMESTAMP_MS": tokens.TIMESTAMP_MS,
		"TIMESTAMP_NS": tokens.TIMESTAMP_NS,
		"TIMESTAMP_US": tokens.TIMESTAMP,
		"UBIGINT":      tokens.UBIGINT,
		"UINTEGER":     tokens.UINT,
		"USMALLINT":    tokens.USMALLINT,
		"UTINYINT":     tokens.UTINYINT,
		"VARCHAR":      tokens.TEXT,
	} {
		cfg.Keywords[keyword] = tokenType
	}
	delete(cfg.Keywords, "/*+")
	delete(cfg.Commands, tokens.SHOW)
	cfg.HintStart = ""
	d.TokenizerConfig = tokens.CompileConfig(cfg)

	// parsers/duckdb.py flags that the shared parser reads from the Dialect.
	d.JSONOperatorsAreBinary = true
	d.MapKeysAreArbitraryExpressions = true
	d.PivotColumnNaming = "agg_name_if_aliased_or_multiple"
	d.PrefixAlias = true
	d.ByteStart, d.ByteEnd = "e'", "'"
	// generators/duckdb.py:1560 STRUCT_DELIMITER = ("(", ")")
	d.StructDelimiter = [2]string{"(", ")"}
	// generators/duckdb.py:1554-1581 generator flags.
	d.ParameterToken = "$"
	d.TablesampleKeywords = "USING SAMPLE"
	d.TablesampleSeedKeyword = "REPEATABLE"
	d.CopyHasIntoKeyword = false
	d.RenameTableWithDB = false
	d.LambdaKeyword = true
	d.NormalizeExtractDateParts = true
	d.IgnoreNullsInFunc = true
	d.IgnoreNullsBeforeOrder = false
	d.ExponentOperators = true
	d.RegexpRangeOperators = true
	d.CaretAtStartsWith = true
	d.ArrayOverlapsDAMP = true

	// parsers/duckdb.py:99-176 FUNCTIONS overlay; functions removed from the base table are
	// marked so the lookup falls through instead of using the base builder.
	d.RemovedFunctions = map[string]bool{"DATE_SUB": true, "GLOB": true}
	d.Functions = duckDBFunctions(d)

	return d
}

// duckDBFunctions builds the DuckDBParser.FUNCTIONS overlay (parsers/duckdb.py:99-176).
func duckDBFunctions(d *Dialect) map[string]func([]exp.Expression) exp.Expression {
	out := map[string]func([]exp.Expression) exp.Expression{}

	out["ANY_VALUE"] = func(args []exp.Expression) exp.Expression {
		return exp.IgnoreNulls(exp.Args{"this": exp.FromArgList(exp.KindAnyValue, args)})
	}
	out["ARRAY_PREPEND"] = duckDBBuildArrayPrepend
	out["ARRAY_REVERSE_SORT"] = duckDBBuildSortArrayDesc
	out["ARRAY_INTERSECT"] = func(args []exp.Expression) exp.Expression {
		return exp.ArrayIntersect(exp.Args{"expressions": args})
	}
	out["ARRAY_SORT"] = exp.FromArgListFunc(exp.KindSortArray)
	out["BIT_AND"] = exp.FromArgListFunc(exp.KindBitwiseAndAgg)
	out["BIT_OR"] = exp.FromArgListFunc(exp.KindBitwiseOrAgg)
	out["BIT_XOR"] = exp.FromArgListFunc(exp.KindBitwiseXorAgg)
	out["CURRENT_LOCALTIMESTAMP"] = exp.FromArgListFunc(exp.KindLocaltimestamp)
	out["DATEDIFF"] = duckDBBuildDateDiff
	out["DATE_DIFF"] = duckDBBuildDateDiff
	out["DATE_TRUNC"] = duckDBBuildDateTrunc
	out["DATETRUNC"] = duckDBBuildDateTrunc
	out["DECODE"] = func(args []exp.Expression) exp.Expression {
		return exp.Decode(exp.Args{"this": seqGet(args, 0), "charset": exp.LiteralString("utf-8")})
	}
	out["EDITDIST3"] = exp.FromArgListFunc(exp.KindLevenshtein)
	out["ENCODE"] = func(args []exp.Expression) exp.Expression {
		return exp.Encode(exp.Args{"this": seqGet(args, 0), "charset": exp.LiteralString("utf-8")})
	}
	out["EPOCH"] = exp.FromArgListFunc(exp.KindTimeToUnix)
	out["EPOCH_MS"] = func(args []exp.Expression) exp.Expression {
		return exp.UnixToTime(exp.Args{"this": seqGet(args, 0), "scale": exp.LiteralNumber("3")})
	}
	out["FROM_HEX"] = exp.FromArgListFunc(exp.KindUnhex)
	out["GENERATE_SERIES"] = duckDBBuildGenerateSeries(false)
	out["GET_CURRENT_TIME"] = exp.FromArgListFunc(exp.KindCurrentTime)
	out["LEN"] = func(args []exp.Expression) exp.Expression {
		return exp.Length(exp.Args{"this": seqGet(args, 0)})
	}
	out["GET_BIT"] = func(args []exp.Expression) exp.Expression {
		return exp.Getbit(exp.Args{"this": seqGet(args, 0), "expression": seqGet(args, 1), "zero_is_msb": true})
	}
	out["JARO_WINKLER_SIMILARITY"] = exp.FromArgListFunc(exp.KindJarowinklerSimilarity)
	out["JSON"] = exp.FromArgListFunc(exp.KindParseJSON)
	out["JSON_ARRAY"] = func(args []exp.Expression) exp.Expression {
		return exp.JSONArray(exp.Args{"expressions": args})
	}
	out["JSON_EXTRACT_PATH"] = func(args []exp.Expression) exp.Expression {
		// parser.build_extract_json_with_path: extra path args land in `expressions`, not in
		// the next positional arg slot.
		built := exp.New(exp.KindJSONExtract, exp.Args{"this": seqGet(args, 0), "expression": seqGet(args, 1)})
		if len(args) > 2 {
			built.Set("expressions", args[2:])
		}
		return built
	}
	out["JSON_EXTRACT_STRING"] = func(args []exp.Expression) exp.Expression {
		return exp.FromArgList(exp.KindJSONExtractScalar, args)
	}
	out["LIST"] = exp.FromArgListFunc(exp.KindArrayAgg)
	out["LIST_DISTINCT"] = exp.FromArgListFunc(exp.KindArrayDistinct)
	out["LIST_APPEND"] = exp.FromArgListFunc(exp.KindArrayAppend)
	out["LIST_CONCAT"] = duckDBBuildArrayConcat(d)
	out["LIST_CONTAINS"] = exp.FromArgListFunc(exp.KindArrayContains)
	out["LIST_COSINE_DISTANCE"] = exp.FromArgListFunc(exp.KindCosineDistance)
	out["LIST_DISTANCE"] = exp.FromArgListFunc(exp.KindEuclideanDistance)
	out["LIST_FILTER"] = exp.FromArgListFunc(exp.KindArrayFilter)
	out["LIST_HAS"] = exp.FromArgListFunc(exp.KindArrayContains)
	out["LIST_HAS_ANY"] = exp.FromArgListFunc(exp.KindArrayOverlaps)
	out["LIST_MAX"] = exp.FromArgListFunc(exp.KindArrayMax)
	out["LIST_MIN"] = exp.FromArgListFunc(exp.KindArrayMin)
	out["LIST_PREPEND"] = duckDBBuildArrayPrepend
	out["LIST_REVERSE_SORT"] = duckDBBuildSortArrayDesc
	out["LIST_SORT"] = exp.FromArgListFunc(exp.KindSortArray)
	out["LIST_TRANSFORM"] = exp.FromArgListFunc(exp.KindTransform)
	out["LIST_VALUE"] = func(args []exp.Expression) exp.Expression {
		return exp.Array(exp.Args{"expressions": args})
	}
	out["MAKE_DATE"] = exp.FromArgListFunc(exp.KindDateFromParts)
	out["MAKE_TIME"] = exp.FromArgListFunc(exp.KindTimeFromParts)
	out["MAKE_TIMESTAMP"] = duckDBBuildMakeTimestamp
	out["QUANTILE_CONT"] = exp.FromArgListFunc(exp.KindPercentileCont)
	out["QUANTILE_DISC"] = exp.FromArgListFunc(exp.KindPercentileDisc)
	out["RANGE"] = duckDBBuildGenerateSeries(true)
	out["REGEXP_EXTRACT"] = duckDBBuildRegexpExtract(exp.KindRegexpExtract)
	out["REGEXP_EXTRACT_ALL"] = duckDBBuildRegexpExtract(exp.KindRegexpExtractAll)
	out["REGEXP_MATCHES"] = exp.FromArgListFunc(exp.KindRegexpLike)
	out["REGEXP_REPLACE"] = func(args []exp.Expression) exp.Expression {
		return exp.RegexpReplace(exp.Args{
			"this":           seqGet(args, 0),
			"expression":     seqGet(args, 1),
			"replacement":    seqGet(args, 2),
			"modifiers":      seqGet(args, 3),
			"single_replace": true,
		})
	}
	out["SHA256"] = func(args []exp.Expression) exp.Expression {
		return exp.SHA2(exp.Args{"this": seqGet(args, 0), "length": exp.LiteralNumber("256")})
	}
	out["STRFTIME"] = postgresBuildFormattedTimeClass(d, exp.KindTimeToStr)
	out["STRING_SPLIT"] = exp.FromArgListFunc(exp.KindSplit)
	out["STRING_SPLIT_REGEX"] = exp.FromArgListFunc(exp.KindRegexpSplit)
	out["STRING_TO_ARRAY"] = exp.FromArgListFunc(exp.KindSplit)
	out["STRPTIME"] = postgresBuildFormattedTimeClass(d, exp.KindStrToTime)
	out["STRUCT_PACK"] = exp.FromArgListFunc(exp.KindStruct)
	out["STR_SPLIT"] = exp.FromArgListFunc(exp.KindSplit)
	out["STR_SPLIT_REGEX"] = exp.FromArgListFunc(exp.KindRegexpSplit)
	out["TODAY"] = exp.FromArgListFunc(exp.KindCurrentDate)
	out["TIME_BUCKET"] = exp.FromArgListFunc(exp.KindDateBin)
	out["TO_TIMESTAMP"] = exp.FromArgListFunc(exp.KindUnixToTime)
	out["UNNEST"] = exp.FromArgListFunc(exp.KindExplode)
	out["VERSION"] = exp.FromArgListFunc(exp.KindCurrentVersion)
	out["XOR"] = duckDBBinaryFromFunction(exp.KindBitwiseXor)

	return out
}

// duckDBBuildSortArrayDesc ports _build_sort_array_desc (parsers/duckdb.py:18-19).
func duckDBBuildSortArrayDesc(args []exp.Expression) exp.Expression {
	return exp.SortArray(exp.Args{"this": seqGet(args, 0), "asc": exp.Boolean(exp.Args{"this": false})})
}

// duckDBBuildArrayPrepend ports _build_array_prepend (parsers/duckdb.py:22-23).
func duckDBBuildArrayPrepend(args []exp.Expression) exp.Expression {
	return exp.ArrayPrepend(exp.Args{"this": seqGet(args, 1), "expression": seqGet(args, 0)})
}

// duckDBBuildDateDiff ports _build_date_diff (parsers/duckdb.py:26-27).
func duckDBBuildDateDiff(args []exp.Expression) exp.Expression {
	return exp.DateDiff(exp.Args{
		"this":       seqGet(args, 2),
		"expression": seqGet(args, 1),
		"unit":       seqGet(args, 0),
	})
}

// duckDBBuildGenerateSeries ports _build_generate_series (parsers/duckdb.py:30-43).
func duckDBBuildGenerateSeries(endExclusive bool) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		if len(args) == 1 {
			args = append([]exp.Expression{exp.LiteralNumber("0")}, args...)
		}
		built := exp.FromArgList(exp.KindGenerateSeries, args)
		built.Set("is_end_exclusive", endExclusive)
		return built
	}
}

// duckDBBuildMakeTimestamp ports _build_make_timestamp (parsers/duckdb.py:46-58).
func duckDBBuildMakeTimestamp(args []exp.Expression) exp.Expression {
	if len(args) == 1 {
		return exp.UnixToTime(exp.Args{"this": seqGet(args, 0), "scale": exp.LiteralNumber("6")})
	}
	return exp.TimestampFromParts(exp.Args{
		"year":  seqGet(args, 0),
		"month": seqGet(args, 1),
		"day":   seqGet(args, 2),
		"hour":  seqGet(args, 3),
		"min":   seqGet(args, 4),
		"sec":   seqGet(args, 5),
	})
}

// duckDBBuildDateTrunc ports date_trunc_to_time (dialects/dialect.py:1682-1688).
func duckDBBuildDateTrunc(args []exp.Expression) exp.Expression {
	unit := seqGet(args, 0)
	this := seqGet(args, 1)
	if this != nil && this.Kind() == exp.KindCast && exp.IsType(asExpression(this.Arg("to")), exp.DTypeDate) {
		return exp.DateTrunc(exp.Args{"unit": unit, "this": this})
	}
	return exp.New(exp.KindTimestampTrunc, exp.Args{"this": this, "unit": unit})
}

// duckDBBuildRegexpExtract ports build_regexp_extract (dialects/dialect.py:2459-2478). DuckDB
// keeps the base REGEXP_EXTRACT_DEFAULT_GROUP and null_if_pos_overflow defaults.
func duckDBBuildRegexpExtract(kind exp.Kind) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		group := seqGet(args, 2)
		if group == nil {
			group = exp.LiteralNumber("0")
		}
		built := exp.New(kind, exp.Args{
			"this":       seqGet(args, 0),
			"expression": seqGet(args, 1),
			"group":      group,
			"parameters": seqGet(args, 3),
		})
		if kind == exp.KindRegexpExtract {
			built.Set("null_if_pos_overflow", true)
		}
		return built
	}
}

// duckDBBinaryFromFunction ports binary_from_function (dialects/dialect.py:1898-1899).
func duckDBBinaryFromFunction(kind exp.Kind) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		return exp.New(kind, exp.Args{"this": seqGet(args, 0), "expression": seqGet(args, 1)})
	}
}

// duckDBBuildArrayConcat ports build_array_concat (parser.py:235-250).
func duckDBBuildArrayConcat(d *Dialect) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		var rest []exp.Expression
		if len(args) > 1 {
			rest = args[1:]
		}
		return exp.ArrayConcat(exp.Args{
			"this":             seqGet(args, 0),
			"expressions":      rest,
			"null_propagation": d.ArrayFuncsPropagatesNulls,
		})
	}
}
