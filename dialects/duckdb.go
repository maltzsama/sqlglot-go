package dialects

import (
	"maps"
	"strings"

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
	// generators/duckdb.py: QUERY_HINTS = False.
	d.NoQueryHints = true
	d.SupportsNearestJoin = true
	d.SemiAntiJoinNoSide = true
	// generators/duckdb.py PAD_FILL_PATTERN_IS_REQUIRED = True.
	d.PadFillPatternIsRequired = true
	// generators/duckdb.py LIMIT_FETCH = "LIMIT": render FETCH as LIMIT.
	d.LimitFetch = "LIMIT"
	// generators/duckdb.py NVL2_SUPPORTED = False.
	d.Nvl2Supported = false
	// generators/duckdb.py:1588 SET_ASSIGNMENT_REQUIRES_VARIABLE_KEYWORD = True.
	d.SetAssignmentRequiresVariableKeyword = true
	// generators/duckdb.py:1580 SUPPORTS_WINDOW_EXCLUDE = True.
	d.SupportsWindowExclude = true
	// generators/duckdb.py:1571 SUPPORTS_CREATE_TABLE_LIKE = False: CREATE TABLE ... LIKE is
	// rewritten to a zero-row CTAS.
	d.SupportsCreateTableLike = false
	// generators/duckdb.py:1575 SELECT_KINDS = (): SELECT AS STRUCT becomes a struct literal.
	d.SelectKinds = []string{}
	// generators/duckdb.py:1570 JSON_PATH_BRACKETED_KEY_SUPPORTED = False: an unsafe JSON
	// path key renders as `."key"`, not `["key"]`.
	d.JSONPathBracketedKeySupported = false
	// generators/duckdb.py:1852-1935 RESERVED_KEYWORDS: DuckDB quotes these as identifiers.
	d.ReservedKeywords = duckDBReservedKeywords()
	// generators/duckdb.py UNWRAPPED_INTERVAL_VALUES = (Literal, Paren), narrower than the
	// base set: a column or unary-minus interval value is parenthesized.
	d.IntervalUnwrappedLiteralsOnly = true

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
	// dialects/duckdb.py:55-65 to_json_path keeps JSON-pointer paths ("/..." or "[#...") literal.
	d.JSONPathPointerSyntax = true
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
		"CONNECT":      tokens.CONNECT,
		"BINARY":       tokens.VARBINARY,
		"BITSTRING":    tokens.BIT,
		"BPCHAR":       tokens.TEXT,
		"CHAR":         tokens.TEXT,
		"DATETIME":     tokens.TIMESTAMPNTZ,
		"DETACH":       tokens.DETACH,
		"DISCONNECT":   tokens.DISCONNECT,
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
	// parsers/duckdb.py:106-110 EXPONENT: `**` and `^` both parse as Pow.
	d.ExponentOperators = map[tokens.TokenType]bool{tokens.CARET: true, tokens.DSTAR: true}
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

	out["JSON_ARRAY"] = exp.FromArgListFunc(exp.KindJSONArray)
	out["EPOCH_MS"] = func(args []exp.Expression) exp.Expression {
		return exp.New(exp.KindUnixToTime, exp.Args{"this": seqGet(args, 0), "scale": exp.LiteralNumber("3")})
	}
	out["STRING_TO_ARRAY"] = exp.FromArgListFunc(exp.KindSplit)
	out["STR_SPLIT"] = exp.FromArgListFunc(exp.KindSplit)
	out["STR_SPLIT_REGEX"] = exp.FromArgListFunc(exp.KindRegexpSplit)
	out["ANY_VALUE"] = func(args []exp.Expression) exp.Expression {
		return exp.IgnoreNulls(exp.Args{"this": exp.FromArgList(exp.KindAnyValue, args)})
	}
	out["ARRAY_PREPEND"] = duckDBBuildArrayPrepend
	out["ARRAY_REVERSE_SORT"] = duckDBBuildListSort("ARRAY_REVERSE_SORT", true)
	out["ARRAY_INTERSECT"] = func(args []exp.Expression) exp.Expression {
		return exp.ArrayIntersect(exp.Args{"expressions": args})
	}
	// parsers/duckdb.py:234 LIST_INTERSECT and :186-195 ARRAY_SLICE / LIST_SLICE.
	out["LIST_INTERSECT"] = out["ARRAY_INTERSECT"]
	out["ARRAY_SLICE"] = duckDBBuildArraySlice
	out["LIST_SLICE"] = duckDBBuildArraySlice
	// parsers/duckdb.py:213 FORMAT is the is_fmt form; :247 PRINTF is the plain Format.
	out["FORMAT"] = func(args []exp.Expression) exp.Expression {
		return exp.Format(exp.Args{"this": seqGet(args, 0), "expressions": seqGetRest(args), "is_fmt": true})
	}
	out["PRINTF"] = exp.FromArgListFunc(exp.KindFormat)
	out["ARRAY_SORT"] = duckDBBuildListSort("ARRAY_SORT", false)
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
	out["LIST_REVERSE_SORT"] = duckDBBuildListSort("LIST_REVERSE_SORT", true)
	out["LIST_SORT"] = duckDBBuildListSort("LIST_SORT", false)
	out["LIST_TRANSFORM"] = exp.FromArgListFunc(exp.KindTransform)
	out["LIST_VALUE"] = func(args []exp.Expression) exp.Expression {
		// parsers/duckdb.py:242: value_constructor when the only argument is a query.
		return exp.Array(exp.Args{"expressions": args, "value_constructor": len(args) == 1 && args[0] != nil && args[0].Is(exp.TraitQuery)})
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

// duckDBBuildListSort ports _build_list_sort (parsers/duckdb.py:_build_list_sort): the order
// and NULLS arguments are case-insensitive literal strings, NULLs sort last by default, and a
// call with any other order falls back to a plain function call under the same name.
func duckDBBuildListSort(name string, desc bool) func([]exp.Expression) exp.Expression {
	return func(args []exp.Expression) exp.Expression {
		orders := []exp.Expression{}
		if desc {
			orders = append(orders, exp.LiteralString("DESC"))
		}
		if len(args) > 1 {
			orders = append(orders, args[1:]...)
		}
		orderName := duckDBListSortOrderName(seqGet(orders, 0), "ASC")
		nullOrderName := duckDBListSortOrderName(seqGet(orders, 1), "NULLS LAST")
		valid := (orderName == "ASC" || orderName == "DESC") && (nullOrderName == "NULLS FIRST" || nullOrderName == "NULLS LAST")
		if !valid {
			return exp.Anonymous(exp.Args{"this": name, "expressions": args})
		}
		var asc exp.Expression
		if orderName == "DESC" {
			asc = exp.Boolean(exp.Args{"this": false})
		}
		return exp.SortArray(exp.Args{
			"this":        seqGet(args, 0),
			"asc":         asc,
			"nulls_first": exp.Boolean(exp.Args{"this": nullOrderName == "NULLS FIRST"}),
		})
	}
}

// duckDBListSortOrderName ports _list_sort_order_name: the upper-cased string literal, the
// default when the argument is absent, and "" for a non-literal string.
func duckDBListSortOrderName(order exp.Expression, def string) string {
	if order == nil {
		return def
	}
	if order.IsString() {
		return strings.ToUpper(order.Name())
	}
	return ""
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
	unit := exp.TimeUnitVar(seqGet(args, 0))
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

// duckDBReservedKeywords ports RESERVED_KEYWORDS (generators/duckdb.py:1853-1935), taken from
// DuckDB's libpg_query reserved_keywords.list. The odd "_p" entries are upstream's internal
// token names and are ported verbatim.
func duckDBReservedKeywords() map[string]bool {
	words := []string{
		"array", "analyse", "union", "all", "when", "in_p", "default", "create_p", "window", //nolint:misspell // upstream spelling
		"asymmetric", "to", "else", "localtime", "from", "end_p", "select", "current_date",
		"foreign", "with", "grant", "session_user", "or", "except", "references", "fetch",
		"limit", "group_p", "leading", "into", "collate", "offset", "do", "then",
		"localtimestamp", "check_p", "lateral_p", "current_role", "where", "asc_p", "placing",
		"desc_p", "user", "unique", "initially", "column", "both", "some", "as", "any", "only",
		"deferrable", "null_p", "current_time", "true_p", "table", "case", "trailing",
		"variadic", "for", "on", "distinct", "false_p", "not", "constraint", "current_timestamp",
		"returning", "primary", "intersect", "having", "analyze", "current_user", "and", "cast",
		"symmetric", "using", "order", "current_catalog",
	}
	out := make(map[string]bool, len(words))
	for _, w := range words {
		out[w] = true
	}
	return out
}

// duckDBBuildArraySlice ports the ARRAY_SLICE / LIST_SLICE builder (parsers/duckdb.py:186-195).
func duckDBBuildArraySlice(args []exp.Expression) exp.Expression {
	return exp.ArraySlice(exp.Args{
		"this":  seqGet(args, 0),
		"start": seqGet(args, 1),
		"end":   seqGet(args, 2),
		"step":  seqGet(args, 3),
	})
}

// seqGetRest returns args[1:] or nil (Python's args[1:] on a short list).
func seqGetRest(args []exp.Expression) []exp.Expression {
	if len(args) <= 1 {
		return nil
	}
	return args[1:]
}
